package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"time"

	"github.com/akallam04/oversold/internal/dbstate"
	"github.com/akallam04/oversold/internal/envfile"
	"github.com/akallam04/oversold/internal/history"
	"github.com/akallam04/oversold/internal/medusa"
	"github.com/akallam04/oversold/internal/runner"
	"github.com/akallam04/oversold/internal/scenario"
	"github.com/akallam04/oversold/internal/stack"
)

var labelPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

type checkoutRun struct {
	Format       string                `json:"format"`
	Label        string                `json:"label"`
	Oversold     history.Build         `json:"oversold"`
	Target       history.Target        `json:"target"`
	Start        history.Timestamp     `json:"start"`
	End          history.Timestamp     `json:"end"`
	Lines        []medusa.Line         `json:"lines"`
	Before       *dbstate.Snapshot     `json:"before,omitempty"`
	Operations   []history.Operation   `json:"operations"`
	Result       medusa.CheckoutResult `json:"result"`
	After        *dbstate.Snapshot     `json:"after,omitempty"`
	HarnessError string                `json:"harness_error,omitempty"`
}

func checkoutCmd(args []string, stdout, stderr io.Writer) int {
	env := envfile.Load(".env")

	fs := flag.NewFlagSet("checkout", flag.ContinueOnError)
	fs.SetOutput(stderr)
	linesFlag := fs.String("lines", "", "comma separated SKU:QUANTITY pairs, for example OVS-PIECE:2,OVS-TWOPACK:1")
	label := fs.String("label", "", "short name for this run, used in the run file name")
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

	start := time.Now()
	masker := history.NewMasker(
		env.Get("POSTGRES_PASSWORD", ""),
		env.Get("MEDUSA_JWT_SECRET", ""),
		env.Get("MEDUSA_COOKIE_SECRET", ""),
		env.Get("MEDUSA_ADMIN_PASSWORD", ""),
	)
	rec := history.NewRecorder(start, masker)
	run := &checkoutRun{
		Format:   "oversold.checkout/v0",
		Label:    *label,
		Oversold: history.CurrentBuild(),
		Start:    history.Timestamp{Time: start.UTC()},
		Lines:    lines,
	}
	path := filepath.Join(*outDir, start.UTC().Format("20060102T150405.000000000Z")+"-"+*label+".json")

	harnessErr := doCheckout(ctx, run, env, rec, *timeout)
	run.Operations = rec.Operations()
	run.End = history.Timestamp{Time: time.Now().UTC()}
	if harnessErr != nil {
		run.HarnessError = harnessErr.Error()
	}

	if err := history.WriteJSON(path, run, masker); err != nil {
		fmt.Fprintf(stderr, "checkout: write run file: %v\n", err)
		return 1
	}
	printCheckout(stdout, run, path)
	if harnessErr != nil {
		fmt.Fprintf(stderr, "checkout: %s\n", masker.String(harnessErr.Error()))
		return 1
	}
	return 0
}

func doCheckout(ctx context.Context, run *checkoutRun, env envfile.Env, rec *history.Recorder, timeout time.Duration) error {
	if _, err := stack.CheckDocker(ctx); err != nil {
		return err
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	st, err := stack.New(dir, env, env.Get("MEDUSA_VERSION", ""))
	if err != nil {
		return err
	}
	run.Target = history.Target{Engine: "medusa", ScenarioVersion: st.Version}
	if run.Target.Servers, err = st.Facts(ctx, 1); err != nil {
		return err
	}

	db, err := dbstate.Open(ctx, st.AppDatabaseURL())
	if err != nil {
		return err
	}
	defer db.Close(context.Background())
	if run.Target.PostgresVersion, err = db.ServerVersion(ctx); err != nil {
		return err
	}
	key, err := db.PublishableKey(ctx)
	if err != nil {
		return err
	}
	rec.Masker().Add(key)

	client, err := medusa.NewClient(st.Servers(1)[0].URL, key, &http.Client{Timeout: timeout}, rec)
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
	for i := range run.Lines {
		run.Lines[i].VariantID = target.Variants[run.Lines[i].SKU]
	}

	if run.Before, err = db.Snapshot(ctx, scenario.VariantSKUs()); err != nil {
		return err
	}
	run.Result = client.Checkout(ctx, medusa.CartInput{
		RegionID: target.RegionID,
		Lines:    run.Lines,
		Email:    "buyer+" + run.Label + "@example.com",
		Address:  medusa.BuyerAddress(),
	})
	if run.After, err = db.Snapshot(context.Background(), scenario.VariantSKUs()); err != nil {
		return err
	}
	return nil
}

func printCheckout(w io.Writer, run *checkoutRun, path string) {
	switch {
	case run.Result.Completed:
		fmt.Fprintf(w, "%s: completed, order %s\n", run.Label, run.Result.OrderID)
	case run.Result.FailedStep != "":
		fmt.Fprintf(w, "%s: rejected at %s (%s): %s\n", run.Label, run.Result.FailedStep, run.Result.Outcome, run.Result.Error)
	default:
		fmt.Fprintf(w, "%s: not run\n", run.Label)
	}
	for _, op := range run.Operations {
		fmt.Fprintf(w, "  %-4s %-60s %d %s\n", op.Method, op.Path, op.Status, op.Outcome)
	}
	if run.Before != nil && run.After != nil {
		h := &history.History{Before: run.Before, After: run.After}
		fmt.Fprintf(w, "  %s\n", runner.Summary(h))
	}
	fmt.Fprintf(w, "  run file: %s\n", path)
}
