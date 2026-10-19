package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/akallam04/oversold/internal/envfile"
	"github.com/akallam04/oversold/internal/runner"
	"github.com/akallam04/oversold/internal/scenario"
)

func validateCmd(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(stderr, "usage: oversold validate <scenario.yaml>")
		return 2
	}
	f, problems := scenario.Load(args[0])
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintf(stderr, "%s: %s\n", args[0], p)
		}
		fmt.Fprintf(stderr, "%s: invalid, %d problems\n", args[0], len(problems))
		return 2
	}
	s := f.Scenario
	fmt.Fprintf(stdout, "%s: valid, scenario %s, %s %s on %d servers, %d carts, %d race actions, %d runs\n",
		args[0], s.Name, s.Target.Engine, s.Target.Version, s.Target.Servers, len(s.Carts), len(s.Race), s.Runs)
	return 0
}

func runCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	runs := fs.Int("runs", 0, "number of runs, overrides the scenario")
	outRoot := fs.String("out", "runs", "directory for run folders")

	var file string
	rest := args
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		file, rest = rest[0], rest[1:]
	}
	if err := fs.Parse(rest); err != nil {
		return 2
	}
	if file == "" && fs.NArg() > 0 {
		file = fs.Arg(0)
		if fs.NArg() > 1 {
			fmt.Fprintf(stderr, "oversold run: unexpected arguments %q\n", fs.Args()[1:])
			return 2
		}
	} else if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "oversold run: unexpected arguments %q\n", fs.Args())
		return 2
	}
	if file == "" {
		fmt.Fprintln(stderr, "usage: oversold run <scenario.yaml> [--runs N]")
		return 2
	}
	if *runs != 0 && (*runs < scenario.MinRuns || *runs > scenario.MaxRuns) {
		fmt.Fprintf(stderr, "oversold run: --runs %d out of range %d to %d\n", *runs, scenario.MinRuns, scenario.MaxRuns)
		return 2
	}

	f, problems := scenario.Load(file)
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintf(stderr, "%s: %s\n", file, p)
		}
		return 2
	}

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "oversold run: %v\n", err)
		return 3
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return runner.Run(ctx, runner.Options{
		File:    f,
		Runs:    *runs,
		OutRoot: *outRoot,
		Dir:     dir,
		Env:     envfile.Load(".env"),
		Stdout:  stdout,
		Stderr:  stderr,
	})
}
