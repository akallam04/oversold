package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/akallam04/oversold/internal/dbstate"
	"github.com/akallam04/oversold/internal/envfile"
	"github.com/akallam04/oversold/internal/history"
	"github.com/akallam04/oversold/internal/medusa"
	"github.com/akallam04/oversold/internal/scenario"
	"github.com/akallam04/oversold/internal/stack"
)

const SettleWait = 2 * time.Second

const (
	ExitOK      = 0
	ExitUsage   = 2
	ExitHarness = 3
)

type Options struct {
	File    *scenario.File
	Runs    int
	OutRoot string
	Dir     string
	Env     envfile.Env
	Stdout  io.Writer
	Stderr  io.Writer
}

func Run(ctx context.Context, opts Options) int {
	sc := opts.File.Scenario
	runs := sc.Runs
	if opts.Runs > 0 {
		runs = opts.Runs
	}

	host, err := stack.CheckDocker(ctx)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "oversold run: %v\n", err)
		return ExitHarness
	}
	fmt.Fprintf(opts.Stdout, "docker endpoint %s\n", host)
	st, err := stack.New(opts.Dir, opts.Env, sc.Target.Version)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "oversold run: %v\n", err)
		return ExitHarness
	}

	started := time.Now().UTC()
	outDir := filepath.Join(opts.OutRoot, started.Format("20060102T150405Z")+"-"+sc.Name)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(opts.Stderr, "oversold run: %v\n", err)
		return ExitHarness
	}
	if err := os.WriteFile(filepath.Join(outDir, filepath.Base(opts.File.Path)), opts.File.Bytes, 0o644); err != nil {
		fmt.Fprintf(opts.Stderr, "oversold run: %v\n", err)
		return ExitHarness
	}
	fmt.Fprintf(opts.Stdout, "scenario %s: %d runs, %d servers, release %s\n", sc.Name, runs, sc.Target.Servers, sc.Timing.Release)

	code := ExitOK
	for i := 1; i <= runs; i++ {
		h, masker, runErr := runOne(ctx, st, opts, runs, i)
		path := filepath.Join(outDir, fmt.Sprintf("run-%03d.json", i))
		if err := history.Write(path, h, masker); err != nil {
			fmt.Fprintf(opts.Stderr, "oversold run: write %s: %v\n", path, err)
			return ExitHarness
		}
		fmt.Fprintln(opts.Stdout, Summary(h))
		if runErr != nil {
			fmt.Fprintf(opts.Stderr, "oversold run: run %d: harness error: %s\n", i, masker.String(runErr.Error()))
			code = ExitHarness
			break
		}
	}
	fmt.Fprintf(opts.Stdout, "%s in %.1fs\n", outDir, time.Since(started).Seconds())
	return code
}

func runOne(ctx context.Context, st *stack.Stack, opts Options, runs, index int) (*history.History, *history.Masker, error) {
	sc := opts.File.Scenario
	runStart := time.Now()
	masker := history.NewMasker(
		opts.Env.Get("POSTGRES_PASSWORD", ""),
		opts.Env.Get("MEDUSA_JWT_SECRET", ""),
		opts.Env.Get("MEDUSA_COOKIE_SECRET", ""),
		opts.Env.Get("MEDUSA_ADMIN_PASSWORD", ""),
	)
	rec := history.NewRecorder(runStart, masker)
	h := &history.History{
		Schema: history.Schema,
		Scenario: history.ScenarioRef{
			Name:   sc.Name,
			Schema: sc.Schema,
			File:   filepath.Base(opts.File.Path),
			SHA256: opts.File.SHA256,
		},
		Run:      history.RunInfo{Index: index, Runs: runs, Seed: *sc.Seed},
		Oversold: history.CurrentBuild(),
		Target: history.Target{
			Engine:           sc.Target.Engine,
			ScenarioVersion:  sc.Target.Version,
			TemplateDatabase: st.TemplateName(),
		},
		Start: history.Timestamp{Time: runStart.UTC()},
		Stock: sc.Stock,
		Carts: map[string]string{},
		Race:  history.Race{Release: sc.Timing.Release, SettleMS: SettleWait.Milliseconds()},
	}
	err := execute(ctx, st, sc, opts.Env, index, rec, h)
	h.Operations = rec.Operations()
	h.End = history.Timestamp{Time: time.Now().UTC()}
	if err != nil {
		h.HarnessError = err.Error()
	}
	return h, masker, err
}

func execute(ctx context.Context, st *stack.Stack, sc *scenario.Scenario, env envfile.Env, index int, rec *history.Recorder, h *history.History) error {
	rr, err := st.Reset(ctx, sc.Target.Servers)
	h.Reset = &history.Reset{
		Start:           history.Timestamp{Time: rr.Start.UTC()},
		StartNS:         rec.Since(rr.Start),
		TemplateCreated: rr.TemplateCreated,
	}
	if err != nil {
		return fmt.Errorf("reset: %w", err)
	}
	h.Reset.Healthy = history.Timestamp{Time: rr.Healthy.UTC()}
	h.Reset.HealthyNS = rec.Since(rr.Healthy)

	if h.Target.Servers, err = st.Facts(ctx, sc.Target.Servers); err != nil {
		return fmt.Errorf("read target facts: %w", err)
	}
	for _, s := range h.Target.Servers {
		if s.MedusaVersion != sc.Target.Version {
			return fmt.Errorf("%s runs medusa %s, scenario wants %s", s.Service, s.MedusaVersion, sc.Target.Version)
		}
	}

	db, err := dbstate.Open(ctx, st.AppDatabaseURL())
	if err != nil {
		return err
	}
	defer db.Close(context.Background())
	if h.Target.PostgresVersion, err = db.ServerVersion(ctx); err != nil {
		return fmt.Errorf("postgres version: %w", err)
	}
	key, err := db.PublishableKey(ctx)
	if err != nil {
		return err
	}
	rec.Masker().Add(key)

	timeout := sc.RequestTimeout()
	setupTransport := newTransport()
	defer setupTransport.CloseIdleConnections()
	setupHTTP := &http.Client{Timeout: timeout, Transport: setupTransport}
	servers := st.Servers(sc.Target.Servers)
	clients := make([]*medusa.Client, len(servers))
	for i, srv := range servers {
		c, err := medusa.NewClient(srv.URL, key, setupHTTP, rec)
		if err != nil {
			return err
		}
		clients[i] = c.With(medusa.Labels{Phase: history.PhaseSetup, Server: srv.Index})
	}

	if err := setStock(ctx, clients[0], env, sc.Stock); err != nil {
		return fmt.Errorf("set stock: %w", err)
	}

	target, err := clients[0].FindTarget(ctx, "usd", "us", scenario.VariantSKUs())
	if err != nil {
		return fmt.Errorf("find target: %w", err)
	}

	for _, name := range sc.CartOrder {
		cart := sc.Carts[name]
		lines := make([]medusa.Line, len(cart.Lines))
		for i, l := range cart.Lines {
			lines[i] = medusa.Line{SKU: l.SKU, VariantID: target.Variants[l.SKU], Quantity: l.Qty}
		}
		c := clients[cart.Server-1].With(medusa.Labels{Cart: name})
		cartID, err := c.PrepareCart(ctx, medusa.CartInput{
			RegionID: target.RegionID,
			Lines:    lines,
			Email:    fmt.Sprintf("buyer+%s-%s@example.com", sc.Name, name),
			Address:  medusa.BuyerAddress(),
		})
		if cartID != "" {
			h.Carts[name] = cartID
		}
		if err != nil {
			return fmt.Errorf("prepare cart %s: %w", name, err)
		}
	}

	if h.Before, err = db.Snapshot(ctx, scenario.VariantSKUs()); err != nil {
		return err
	}

	if err := race(ctx, sc, index, key, servers, rec, h); err != nil {
		return fmt.Errorf("race: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(SettleWait):
	}

	if h.After, err = db.Snapshot(context.Background(), scenario.VariantSKUs()); err != nil {
		return err
	}
	return nil
}

func setStock(ctx context.Context, c *medusa.Client, env envfile.Env, stock map[string]int) error {
	if len(stock) == 0 {
		return nil
	}
	token, err := c.AdminLogin(ctx, env.Get("MEDUSA_ADMIN_EMAIL", ""), env.Get("MEDUSA_ADMIN_PASSWORD", ""))
	if err != nil {
		return err
	}
	admin := c.WithAdminToken(token)
	skus := make([]string, 0, len(stock))
	for sku := range stock {
		skus = append(skus, sku)
	}
	sort.Strings(skus)
	for _, sku := range skus {
		items, err := admin.ListInventoryItems(ctx, sku)
		if err != nil {
			return err
		}
		var match []medusa.InventoryItem
		for _, it := range items {
			if it.SKU == sku {
				match = append(match, it)
			}
		}
		if len(match) != 1 || len(match[0].LocationLevels) != 1 {
			return fmt.Errorf("want one %s inventory item with one location level, got %d items", sku, len(match))
		}
		if err := admin.SetStockedQuantity(ctx, match[0].ID, match[0].LocationLevels[0].LocationID, stock[sku]); err != nil {
			return err
		}
	}
	return nil
}

type raceAction struct {
	action history.RaceAction
	cartID string
	warm   *medusa.Client
	run    *medusa.Client
	tr     *http.Transport
	warmed error
}

func race(ctx context.Context, sc *scenario.Scenario, index int, key string, servers []stack.Server, rec *history.Recorder, h *history.History) error {
	lo, hi := sc.JitterRange()
	jitter := Jitter(*sc.Seed, index, len(sc.Race), lo, hi)
	acts := make([]*raceAction, len(sc.Race))
	for i, a := range sc.Race {
		tr := newTransport()
		c, err := medusa.NewClient(servers[a.Server-1].URL, key, &http.Client{Timeout: sc.RequestTimeout(), Transport: tr}, rec)
		if err != nil {
			return err
		}
		labels := medusa.Labels{ActionID: a.ID, Cart: a.Cart, Server: a.Server}
		warmLabels, runLabels := labels, labels
		warmLabels.Phase = history.PhaseSetup
		runLabels.Phase = history.PhaseRace
		acts[i] = &raceAction{
			action: history.RaceAction{ID: a.ID, Do: a.Do, Cart: a.Cart, Server: a.Server, JitterNS: jitter[i].Nanoseconds()},
			cartID: h.Carts[a.Cart],
			warm:   c.With(warmLabels),
			run:    c.With(runLabels),
			tr:     tr,
		}
	}
	defer func() {
		for _, a := range acts {
			a.tr.CloseIdleConnections()
			h.Race.Actions = append(h.Race.Actions, a.action)
		}
	}()

	fire := func(a *raceAction) {
		if d := time.Duration(a.action.JitterNS); d > 0 {
			time.Sleep(d)
		}
		_, op, _ := a.run.CompleteCart(ctx, a.cartID)
		a.action.OperationID = op.ID
		a.action.Outcome = op.Outcome
	}

	switch sc.Timing.Release {
	case scenario.ReleaseTogether:
		b := NewBarrier(len(acts))
		var wg sync.WaitGroup
		for _, a := range acts {
			wg.Add(1)
			go func() {
				defer wg.Done()
				a.warmed = a.warm.Health(ctx)
				if b.Wait() {
					fire(a)
				}
			}()
		}
		b.WaitAll()
		var warmErr error
		for _, a := range acts {
			warmErr = errors.Join(warmErr, a.warmed)
		}
		h.Race.ReleaseNS = rec.Since(time.Now())
		b.Release(warmErr == nil)
		wg.Wait()
		return warmErr
	default:
		for _, a := range acts {
			if err := a.warm.Health(ctx); err != nil {
				return err
			}
		}
		h.Race.ReleaseNS = rec.Since(time.Now())
		for _, a := range acts {
			fire(a)
		}
		return nil
	}
}

func newTransport() *http.Transport {
	return &http.Transport{
		DialContext:         (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConns:        1,
		MaxIdleConnsPerHost: 1,
		MaxConnsPerHost:     1,
		IdleConnTimeout:     60 * time.Second,
		DisableCompression:  true,
	}
}

func Summary(h *history.History) string {
	var b strings.Builder
	fmt.Fprintf(&b, "run %03d:", h.Run.Index)
	for _, a := range h.Race.Actions {
		out := string(a.Outcome)
		if out == "" {
			out = "not-sent"
		}
		fmt.Fprintf(&b, " %s=%s", a.ID, out)
	}
	if h.Before != nil && h.After != nil {
		before := level(h.Before, "WIDGET")
		after := level(h.After, "WIDGET")
		fmt.Fprintf(&b, " | WIDGET stocked %s reserved %s->%s live %s | orders %d",
			after.StockedQuantity, before.ReservedQuantity, after.ReservedQuantity, liveSum(h.After, "WIDGET"), len(h.After.Orders))
	}
	if h.Reset != nil && !h.Reset.Healthy.IsZero() {
		fmt.Fprintf(&b, " | reset %.1fs", h.Reset.Healthy.Sub(h.Reset.Start.Time).Seconds())
		if h.Reset.TemplateCreated {
			b.WriteString(" (template built)")
		}
	}
	if h.HarnessError != "" {
		fmt.Fprintf(&b, " | harness error")
	}
	return b.String()
}

func level(s *dbstate.Snapshot, sku string) dbstate.InventoryLevel {
	for _, l := range s.InventoryLevels {
		if l.SKU == sku {
			return l
		}
	}
	return dbstate.InventoryLevel{StockedQuantity: "?", ReservedQuantity: "?"}
}

func liveSum(s *dbstate.Snapshot, sku string) string {
	sum := new(big.Rat)
	for _, r := range s.Reservations {
		if r.InventorySKU != sku {
			continue
		}
		q, ok := new(big.Rat).SetString(string(r.Quantity))
		if !ok {
			return "?"
		}
		sum.Add(sum, q)
	}
	return sum.RatString()
}
