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

	"github.com/akallam04/oversold/internal/checks"
	"github.com/akallam04/oversold/internal/dbstate"
	"github.com/akallam04/oversold/internal/envfile"
	"github.com/akallam04/oversold/internal/history"
	"github.com/akallam04/oversold/internal/medusa"
	"github.com/akallam04/oversold/internal/report"
	"github.com/akallam04/oversold/internal/scenario"
	"github.com/akallam04/oversold/internal/stack"
)

const SettleWait = 2 * time.Second

type Options struct {
	File    *scenario.File
	Runs    int
	Version string
	OutRoot string
	Dir     string
	Env     envfile.Env
	Stdout  io.Writer
	Stderr  io.Writer
}

func Run(ctx context.Context, opts Options) int {
	rep, _, err := RunSet(ctx, opts)
	if err != nil {
		fmt.Fprintf(opts.Stderr, "oversold run: %v\n", err)
		return report.ExitInconclusive
	}
	return rep.ExitCode
}

func RunSet(ctx context.Context, opts Options) (*report.Report, string, error) {
	sc := opts.File.Scenario
	runs := sc.Runs
	if opts.Runs > 0 {
		runs = opts.Runs
	}

	host, err := stack.CheckDocker(ctx)
	if err != nil {
		return nil, "", err
	}
	fmt.Fprintf(opts.Stdout, "docker endpoint %s\n", host)
	used, err := sc.ResolveVersion(opts.Version)
	if err != nil {
		return nil, "", err
	}
	st, err := stack.New(opts.Dir, opts.Env, used)
	if err != nil {
		return nil, "", err
	}
	st.Locking = sc.Target.LockingMode()

	started := time.Now().UTC()
	outDir := filepath.Join(opts.OutRoot, started.Format("20060102T150405Z")+"-"+sc.Name+"-"+used)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, "", err
	}
	if err := os.WriteFile(filepath.Join(outDir, filepath.Base(opts.File.Path)), opts.File.Bytes, 0o644); err != nil {
		return nil, outDir, err
	}
	selected := sc.SelectedChecks()
	fmt.Fprintf(opts.Stdout, "scenario %s: medusa %s (file says %s), %d runs, %d servers, %s locking, release %s, checks %s\n", sc.Name, used, sc.Target.Version, runs, sc.Target.Servers, st.Locking, sc.Timing.Release, strings.Join(selected, ", "))

	entries, stopped, err := runAll(ctx, runs, func(i int) (report.Entry, error, error) {
		h, masker, runErr := runOne(ctx, st, opts, runs, i)
		var err error
		if h.Verdicts, err = checks.Evaluate(h, selected); err != nil {
			return report.Entry{}, nil, err
		}
		name := fmt.Sprintf("run-%03d.json", i)
		path := filepath.Join(outDir, name)
		if err := history.Write(path, h, masker); err != nil {
			return report.Entry{}, nil, fmt.Errorf("write %s: %w", path, err)
		}
		saved, err := history.Read(path)
		if err != nil {
			return report.Entry{}, nil, err
		}
		fmt.Fprintln(opts.Stdout, Summary(saved))
		if runErr != nil {
			fmt.Fprintf(opts.Stderr, "oversold run: run %d: harness error: %s\n", i, saved.HarnessError)
		}
		return report.Entry{File: name, History: saved}, runErr, nil
	})
	if err != nil {
		return nil, outDir, err
	}

	rep := report.Build(opts.File, entries, runs, stopped)
	jsonPath, mdPath, err := rep.Write(outDir)
	if err != nil {
		return nil, outDir, err
	}
	for _, c := range rep.Checks {
		fmt.Fprintln(opts.Stdout, rep.CountLine(c))
	}
	if stopped != "" {
		fmt.Fprintln(opts.Stdout, stopped)
	}
	fmt.Fprintf(opts.Stdout, "%d runs in %.1fs, exit code %d, report %s\n", len(entries), time.Since(started).Seconds(), rep.ExitCode, jsonPath)
	fmt.Fprintln(opts.Stdout, mdPath)
	return rep, outDir, nil
}

func runAll(ctx context.Context, runs int, one func(int) (report.Entry, error, error)) ([]report.Entry, string, error) {
	var entries []report.Entry
	inARow := 0
	for i := 1; i <= runs; i++ {
		e, harnessErr, err := one(i)
		if err != nil {
			return entries, "", err
		}
		entries = append(entries, e)
		if harnessErr == nil {
			inARow = 0
			continue
		}
		inARow++
		if ctx.Err() != nil && i < runs {
			return entries, fmt.Sprintf("interrupted after %d of %d runs", i, runs), nil
		}
		if inARow >= report.MaxHarnessErrorsInARow && i < runs {
			return entries, fmt.Sprintf("stopped after %d harness errors in a row, %d of %d runs recorded", inARow, i, runs), nil
		}
	}
	return entries, "", nil
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
			Version:          st.Version,
			TemplateDatabase: st.TemplateName(),
		},
		Start: history.Timestamp{Time: runStart.UTC()},
		Stock: sc.Stock,
		Carts: map[string]string{},
		Race:  history.Race{Release: sc.Timing.Release, SettleMS: SettleWait.Milliseconds()},
	}
	if st.Redis() {
		h.Target.Locking = st.Locking
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
		ImageBuilt:      rr.ImageBuilt,
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
		if s.MedusaVersion != st.Version {
			return fmt.Errorf("%s runs medusa %s, this run set uses %s", s.Service, s.MedusaVersion, st.Version)
		}
	}
	if st.Redis() {
		if h.Target.RedisVersion, err = st.RedisVersion(ctx); err != nil {
			return fmt.Errorf("redis version: %w", err)
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

	plannedCarts, plannedActions := sc.Plan()
	for _, cart := range plannedCarts {
		name := cart.Name
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

	var redisBefore map[string]history.CommandStat
	if st.Redis() {
		if redisBefore, err = st.RedisCommandStats(ctx); err != nil {
			return fmt.Errorf("redis command stats before the race: %w", err)
		}
	}

	if err := race(ctx, sc, plannedActions, index, key, servers, rec, h); err != nil {
		return fmt.Errorf("race: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(SettleWait):
	}

	if st.Redis() {
		redisAfter, err := st.RedisCommandStats(context.Background())
		if err != nil {
			return fmt.Errorf("redis command stats after the race: %w", err)
		}
		h.Race.RedisCommandStats = &history.CommandStats{Before: redisBefore, After: redisAfter}
		n, err := history.LockCalls(redisBefore, redisAfter)
		if err != nil {
			return err
		}
		h.Race.LockCalls = &n
	}

	if h.After, err = db.Snapshot(context.Background(), scenario.VariantSKUs()); err != nil {
		return err
	}
	if h.Race.LockCalls != nil && *h.Race.LockCalls == 0 {
		return fmt.Errorf("redis locking: Medusa made no scripted lock calls during the race, so the servers did not lock through Redis; if %s was built before the locking option existed, remove it and run again to rebuild it", st.ImageName())
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

func race(ctx context.Context, sc *scenario.Scenario, planned []scenario.PlannedAction, index int, key string, servers []stack.Server, rec *history.Recorder, h *history.History) error {
	lo, hi := sc.JitterRange()
	jitter := Jitter(*sc.Seed, index, len(planned), lo, hi)
	acts := make([]*raceAction, len(planned))
	for i, a := range planned {
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
	if h.Race.LockCalls != nil {
		fmt.Fprintf(&b, " | lock calls %d", *h.Race.LockCalls)
	}
	if h.Reset != nil && !h.Reset.Healthy.IsZero() {
		fmt.Fprintf(&b, " | reset %.1fs", h.Reset.Healthy.Sub(h.Reset.Start.Time).Seconds())
		if h.Reset.TemplateCreated {
			b.WriteString(" (template built)")
		}
	}
	if len(h.Verdicts) > 0 {
		counts := map[history.Result]int{}
		for _, v := range h.Verdicts {
			counts[v.Result]++
		}
		fmt.Fprintf(&b, " | checks %d pass %d fail %d inconclusive", counts[history.Pass], counts[history.Fail], counts[history.Inconclusive])
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
