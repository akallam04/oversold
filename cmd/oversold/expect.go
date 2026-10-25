package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/akallam04/oversold/internal/envfile"
	"github.com/akallam04/oversold/internal/expect"
	"github.com/akallam04/oversold/internal/history"
	"github.com/akallam04/oversold/internal/runner"
	"github.com/akallam04/oversold/internal/scenario"
	"github.com/akallam04/oversold/internal/stack"
)

func versionsCmd(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "usage: oversold versions")
		return expect.ExitUsage
	}
	versions, err := scenario.Versions("medusa")
	if err != nil {
		fmt.Fprintf(stderr, "oversold versions: %v\n", err)
		return expect.ExitUsage
	}
	for _, v := range versions {
		fmt.Fprintln(stdout, v)
	}
	return 0
}

func expectCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("expect", flag.ContinueOnError)
	fs.SetOutput(stderr)
	runs := fs.Int("runs", 0, "runs per case, overrides the file")
	version := fs.String("version", "", "medusa version to run every case on, default the newest folder under targets/medusa/versions")

	var file string
	rest := args
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		file, rest = rest[0], rest[1:]
	}
	if err := fs.Parse(rest); err != nil {
		return expect.ExitUsage
	}
	if file == "" && fs.NArg() == 1 {
		file = fs.Arg(0)
	} else if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "oversold expect: unexpected arguments %q\n", fs.Args())
		return expect.ExitUsage
	}
	if file == "" {
		fmt.Fprintln(stderr, "usage: oversold expect <expectations.yaml> [--version X] [--runs N]")
		return expect.ExitUsage
	}
	if *runs != 0 && (*runs < expect.MinRuns || *runs > expect.MaxRuns) {
		fmt.Fprintf(stderr, "oversold expect: --runs %d out of range %d to %d\n", *runs, expect.MinRuns, expect.MaxRuns)
		return expect.ExitUsage
	}

	f, problems := expect.Load(file)
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintf(stderr, "%s: %s\n", file, p)
		}
		fmt.Fprintf(stderr, "%s: invalid, %d problems\n", file, len(problems))
		return expect.ExitUsage
	}
	versions, err := scenario.Versions("medusa")
	if err != nil {
		fmt.Fprintf(stderr, "oversold expect: %v\n", err)
		return expect.ExitUsage
	}
	used := *version
	if used == "" {
		used = versions[len(versions)-1]
	} else if !slices.Contains(versions, used) {
		fmt.Fprintf(stderr, "oversold expect: --version %s: medusa versions Oversold builds are %s\n", used, strings.Join(versions, ", "))
		return expect.ExitUsage
	}
	perCase := f.Expectations.Runs
	if *runs != 0 {
		perCase = *runs
	}

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "oversold expect: %v\n", err)
		return expect.ExitInconclusive
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return runExpect(ctx, f, used, perCase, dir, envfile.Load(".env"), "runs", stdout, stderr)
}

func runExpect(ctx context.Context, f *expect.File, version string, runs int, dir string, env envfile.Env, outRoot string, stdout, stderr io.Writer) int {
	started := time.Now().UTC()
	sum := &expect.Summary{
		Schema:        expect.SummarySchema,
		Expectations:  expect.FileRef{File: filepath.ToSlash(f.Path), SHA256: f.SHA256},
		MedusaVersion: version,
		Oversold:      history.CurrentBuild(),
		Runs:          runs,
		Started:       history.Timestamp{Time: started},
	}
	masker := history.NewMasker(
		env.Get("POSTGRES_PASSWORD", ""),
		env.Get("MEDUSA_JWT_SECRET", ""),
		env.Get("MEDUSA_COOKIE_SECRET", ""),
		env.Get("MEDUSA_ADMIN_PASSWORD", ""),
	)

	outDir := filepath.Join(outRoot, started.Format("20060102T150405Z")+"-expect-"+version)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "oversold expect: %v\n", err)
		return expect.ExitInconclusive
	}
	fmt.Fprintf(stdout, "oversold expect: %s, %d cases, %d runs each, medusa %s, out %s\n", f.Path, len(f.Expectations.Cases), runs, version, outDir)

	prepared := prepare(ctx, dir, env, version, sum, masker)
	for _, c := range f.Expectations.Cases {
		if !prepared {
			sum.Cases = append(sum.Cases, expect.CaseFrom(c, runs, nil, "", fmt.Errorf("not run, the stack could not be prepared"), masker))
			continue
		}
		rep, setDir, err := runner.RunSet(ctx, runner.Options{
			File:    c.File,
			Runs:    runs,
			Version: version,
			OutRoot: outDir,
			Dir:     dir,
			Env:     env,
			Stdout:  stdout,
			Stderr:  stderr,
		})
		if err != nil {
			fmt.Fprintf(stderr, "oversold expect: %s: %v\n", c.Scenario, err)
		}
		folder := ""
		if setDir != "" {
			folder = filepath.Base(setDir)
		}
		if rep != nil {
			for _, s := range rep.Servers {
				sum.AddImage(s.ImageID)
			}
		}
		sum.Cases = append(sum.Cases, expect.CaseFrom(c, runs, rep, folder, err, masker))
	}

	sum.Finished = history.Timestamp{Time: time.Now().UTC()}
	sum.Seconds = sum.Finished.Sub(started).Seconds()
	sum.Finish()
	jsonPath, mdPath, err := sum.Write(outDir)
	if err != nil {
		fmt.Fprintf(stderr, "oversold expect: %v\n", err)
		return expect.ExitInconclusive
	}

	fmt.Fprintln(stdout)
	sum.Table(stdout)
	for _, u := range sum.Unmet {
		fmt.Fprintln(stdout, u)
	}
	for _, p := range sum.Problems {
		fmt.Fprintln(stdout, p)
	}
	fmt.Fprintf(stdout, "medusa %s, image %s, oversold %s (dirty %v), %.1fs, exit code %d\n", version, strings.Join(sum.ImageIDs, ", "), sum.Oversold.Commit, sum.Oversold.Dirty, sum.Seconds, sum.ExitCode)
	fmt.Fprintln(stdout, jsonPath)
	fmt.Fprintln(stdout, mdPath)
	return sum.ExitCode
}

func prepare(ctx context.Context, dir string, env envfile.Env, version string, sum *expect.Summary, masker *history.Masker) bool {
	fail := func(err error) bool {
		sum.Prepare.Error = masker.String(err.Error())
		return false
	}
	if _, err := stack.CheckDocker(ctx); err != nil {
		return fail(err)
	}
	st, err := stack.New(dir, env, version)
	if err != nil {
		return fail(err)
	}
	res, err := st.Prepare(ctx)
	sum.Prepare.ImageBuilt = res.ImageBuilt
	sum.Prepare.ImageSeconds = res.ImageTime.Seconds()
	sum.Prepare.TemplateCreated = res.TemplateCreated
	sum.Prepare.TemplateSeconds = res.TemplateTime.Seconds()
	sum.AddImage(res.ImageID)
	if err != nil {
		return fail(err)
	}
	return true
}
