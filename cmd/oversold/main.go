package main

import (
	"fmt"
	"io"
	"os"
)

var version = "dev"

const usage = `usage: oversold <command>

exit codes: 0 every check passed, 1 a check failed, 2 invalid scenario or bad usage,
3 nothing failed but a run was inconclusive

commands:
  validate  check a scenario file and print every problem
            oversold validate scenarios/medusa/normal-checkout.yaml
  run       run a scenario, each run on a fresh database, then check every run
            oversold run scenarios/medusa/normal-checkout.yaml [--runs N] [--version X]
  check     recompute every verdict and the report from a run folder, no Docker needed
            oversold check runs/<folder>
            with --verify, write nothing and exit 0 only if the stored verdicts and
            report match the recomputed ones, 1 otherwise, 2 if the folder cannot be read
  export    copy a run folder's scenario, report and gzipped histories to dest
            oversold export runs/<folder> <dest>
  version   print the version
  help      print this help
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "validate":
		return validateCmd(args[1:], stdout, stderr)
	case "run":
		return runCmd(args[1:], stdout, stderr)
	case "check":
		return checkCmd(args[1:], stdout, stderr)
	case "export":
		return exportCmd(args[1:], stdout, stderr)
	case "version":
		fmt.Fprintf(stdout, "oversold %s\n", version)
		return 0
	case "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "oversold: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}
