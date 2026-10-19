package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/akallam04/oversold/internal/dbstate"
	"github.com/akallam04/oversold/internal/medusa"
)

var seededSKUs = []string{"OVS-PIECE", "OVS-TWOPACK", "OVS-SOLO"}

var labelPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

type checkoutRun struct {
	Format         string                `json:"format"`
	Label          string                `json:"label"`
	OversoldCommit string                `json:"oversold_commit"`
	MedusaVersion  string                `json:"medusa_version"`
	MedusaURL      string                `json:"medusa_url"`
	StartedAt      time.Time             `json:"started_at"`
	FinishedAt     time.Time             `json:"finished_at"`
	Lines          []medusa.Line         `json:"lines"`
	Target         *medusa.Target        `json:"target,omitempty"`
	Before         *dbstate.Snapshot     `json:"before,omitempty"`
	Calls          []medusa.Call         `json:"calls"`
	Result         medusa.CheckoutResult `json:"result"`
	After          *dbstate.Snapshot     `json:"after,omitempty"`
	HarnessError   string                `json:"harness_error,omitempty"`
}

func checkoutCmd(args []string, stdout, stderr io.Writer) int {
	env := loadEnv(".env")

	fs := flag.NewFlagSet("checkout", flag.ContinueOnError)
	fs.SetOutput(stderr)
	linesFlag := fs.String("lines", "", "comma separated SKU:QUANTITY pairs, for example OVS-PIECE:2,OVS-TWOPACK:1")
	label := fs.String("label", "", "short name for this run, used in the run file name")
	medusaURL := fs.String("medusa-url", "http://127.0.0.1:"+env.get("MEDUSA_PORT", "9000"), "Medusa base URL")
	dbURL := fs.String("db-url", defaultDBURL(env), "Medusa database URL")
	outDir := fs.String("out", "runs", "directory for run files")
	timeout := fs.Duration("timeout", 30*time.Second, "timeout for each HTTP request")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "checkout: unexpected arguments %q\n", fs.Args())
		return 2
	}
	if !labelPattern.MatchString(*label) {
		fmt.Fprintln(stderr, "checkout: --label must be lowercase letters, digits and dashes")
		return 2
	}
	lines, err := medusa.ParseLines(*linesFlag)
	if err != nil {
		fmt.Fprintf(stderr, "checkout: --lines: %v\n", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	run := &checkoutRun{
		Format:         "oversold.checkout.v0",
		Label:          *label,
		OversoldCommit: oversoldCommit(),
		MedusaVersion:  env.get("MEDUSA_VERSION", "unknown"),
		MedusaURL:      *medusaURL,
		StartedAt:      time.Now().UTC(),
		Lines:          lines,
		Calls:          []medusa.Call{},
	}
	path := filepath.Join(*outDir, run.StartedAt.Format("20060102T150405.000000000Z")+"-"+*label+".json")

	harnessErr := doCheckout(ctx, run, *dbURL, *timeout)
	run.FinishedAt = time.Now().UTC()
	if harnessErr != nil {
		run.HarnessError = harnessErr.Error()
	}

	if err := writeRun(path, run); err != nil {
		fmt.Fprintf(stderr, "checkout: write run file: %v\n", err)
		return 1
	}
	printSummary(stdout, run, path)
	if harnessErr != nil {
		fmt.Fprintf(stderr, "checkout: %v\n", harnessErr)
		return 1
	}
	return 0
}

func doCheckout(ctx context.Context, run *checkoutRun, dbURL string, timeout time.Duration) error {
	db, err := dbstate.Open(ctx, dbURL)
	if err != nil {
		return err
	}
	defer db.Close(context.Background())

	key, err := db.PublishableKey(ctx)
	if err != nil {
		return err
	}

	rec := medusa.NewRecorder(run.OversoldCommit, run.MedusaVersion)
	defer func() { run.Calls = rec.Calls() }()
	client, err := medusa.NewClient(run.MedusaURL, key, &http.Client{Timeout: timeout}, rec)
	if err != nil {
		return err
	}

	skus := make([]string, len(run.Lines))
	for i, l := range run.Lines {
		skus[i] = l.SKU
	}
	target, err := client.FindTarget(ctx, "usd", "us", skus)
	if err != nil {
		return err
	}
	run.Target = &target
	for i := range run.Lines {
		run.Lines[i].VariantID = target.Variants[run.Lines[i].SKU]
	}

	if run.Before, err = db.Snapshot(ctx, seededSKUs); err != nil {
		return err
	}

	run.Result = client.Checkout(ctx, medusa.CheckoutInput{
		RegionID: target.RegionID,
		Lines:    run.Lines,
		Email:    "buyer+" + run.Label + "@example.com",
		Address: medusa.Address{
			FirstName:   "Test",
			LastName:    "Buyer",
			Address1:    "1 Main St",
			City:        "Austin",
			Province:    "tx",
			PostalCode:  "78701",
			CountryCode: "us",
		},
	})

	if run.After, err = db.Snapshot(context.Background(), seededSKUs); err != nil {
		return err
	}
	return nil
}

func writeRun(path string, run *checkoutRun) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func printSummary(w io.Writer, run *checkoutRun, path string) {
	switch {
	case run.Result.Completed:
		fmt.Fprintf(w, "%s: completed, order %s\n", run.Label, run.Result.OrderID)
	case run.Result.FailedStep != "":
		fmt.Fprintf(w, "%s: rejected at %s: %s\n", run.Label, run.Result.FailedStep, run.Result.Error)
	default:
		fmt.Fprintf(w, "%s: not run\n", run.Label)
	}
	for _, c := range run.Calls {
		status := fmt.Sprint(c.Status)
		if c.Timeout {
			status = "timeout"
		}
		fmt.Fprintf(w, "  %-4s %-60s %s\n", c.Method, c.Path, status)
	}
	if run.Before != nil && run.After != nil {
		for _, sku := range inventorySKUs(run.After) {
			before := findLevel(run.Before, sku)
			after := findLevel(run.After, sku)
			sum, n := liveReservations(run.After, sku)
			fmt.Fprintf(w, "  %s: stocked %s, reserved %s -> %s, live reservations %s in %d rows\n",
				sku, after.StockedQuantity, before.ReservedQuantity, after.ReservedQuantity, sum, n)
		}
		fmt.Fprintf(w, "  orders with seeded SKUs: %d -> %d\n", len(run.Before.Orders), len(run.After.Orders))
	}
	fmt.Fprintf(w, "  run file: %s\n", path)
}

func inventorySKUs(s *dbstate.Snapshot) []string {
	var out []string
	for _, l := range s.InventoryLevels {
		out = append(out, l.SKU)
	}
	return out
}

func findLevel(s *dbstate.Snapshot, sku string) dbstate.InventoryLevel {
	for _, l := range s.InventoryLevels {
		if l.SKU == sku {
			return l
		}
	}
	return dbstate.InventoryLevel{StockedQuantity: "?", ReservedQuantity: "?"}
}

func liveReservations(s *dbstate.Snapshot, sku string) (string, int) {
	sum := new(big.Rat)
	n := 0
	for _, r := range s.Reservations {
		if r.InventorySKU != sku {
			continue
		}
		q, ok := new(big.Rat).SetString(string(r.Quantity))
		if !ok {
			return "?", n
		}
		sum.Add(sum, q)
		n++
	}
	return sum.RatString(), n
}

func oversoldCommit() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	var rev, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	if rev == "" {
		return "unknown"
	}
	if modified == "true" {
		rev += "-dirty"
	}
	return rev
}

type envFile map[string]string

func loadEnv(path string) envFile {
	env := envFile{}
	f, err := os.Open(path)
	if err != nil {
		return env
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if ok {
			env[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return env
}

func (e envFile) get(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	if v := e[key]; v != "" {
		return v
	}
	return fallback
}

func defaultDBURL(env envFile) string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(env.get("POSTGRES_USER", "oversold"), env.get("POSTGRES_PASSWORD", "")),
		Host:     "127.0.0.1:" + env.get("POSTGRES_PORT", "55432"),
		Path:     "/medusa",
		RawQuery: "sslmode=disable",
	}
	return u.String()
}
