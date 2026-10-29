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
	"github.com/akallam04/oversold/internal/targets"
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

func variantCmd(args []string, stdout, stderr io.Writer) int {
	if len(args) > 1 || (len(args) == 1 && strings.HasPrefix(args[0], "-")) {
		fmt.Fprintln(stderr, "usage: oversold variant [name]")
		return expect.ExitUsage
	}
	root, err := targets.FindRoot()
	if err != nil {
		fmt.Fprintf(stderr, "oversold variant: %v\n", err)
		return expect.ExitUsage
	}
	if len(args) == 0 {
		names, err := targets.Variants(root, "medusa")
		if err != nil {
			fmt.Fprintf(stderr, "oversold variant: %v\n", err)
			return expect.ExitUsage
		}
		code := 0
		for _, name := range names {
			v, err := targets.LoadVariant(root, "medusa", name)
			if err != nil {
				fmt.Fprintf(stderr, "oversold variant: %v\n", err)
				code = expect.ExitUsage
				continue
			}
			fmt.Fprintf(stdout, "%s  medusa %s  %s by %s\n", v.Name, v.BaseVersion, v.Source.PR, v.Source.Author)
		}
		return code
	}
	v, problems := targets.ReadVariant(root, "medusa", args[0])
	if v != nil {
		names, err := stack.NamesFrom(envfile.Load(".env"))
		if err != nil {
			fmt.Fprintf(stderr, "oversold variant: %v\n", err)
			return expect.ExitUsage
		}
		dash := func(s string) string {
			if s == "" {
				return "-"
			}
			return s
		}
		merged := "-"
		if v.Source.Merged != nil {
			merged = fmt.Sprint(*v.Source.Merged)
		}
		fmt.Fprintf(stdout, "name %s\nbase_version %s\nbase_image %s:%s\npr %s\nauthor %s\nstate %s\nmerged %s\nhead_sha %s\nbase_sha %s\n",
			dash(v.Name), dash(v.BaseVersion), names.ImagePrefix, dash(v.BaseVersion), dash(v.Source.PR), dash(v.Source.Author), dash(v.Source.State), merged, dash(v.Source.HeadSHA), dash(v.Source.BaseSHA))
		for _, f := range v.SourceFiles {
			fmt.Fprintf(stdout, "source_file %s\n", f)
		}
		for _, f := range v.Files {
			fmt.Fprintf(stdout, "file %s %s %s\n", dash(f.Path), dash(f.SHA256), dash(f.ReplacesSHA256))
		}
	}
	for _, p := range problems {
		fmt.Fprintf(stderr, "oversold variant %s: %s\n", args[0], p)
	}
	if len(problems) > 0 {
		return expect.ExitUsage
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
	if flagSet(fs, "runs") && (*runs < expect.MinRuns || *runs > expect.MaxRuns) {
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
	if flagSet(fs, "runs") {
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
	st, err := stack.New(dir, env, version, nil)
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
