package main

import (
	"fmt"
	"io"
	"os"
)

var version = "dev"

const usage = `usage: oversold <command>

commands:
  validate  check a scenario file and print every problem
            oversold validate scenarios/medusa/normal-checkout.yaml
  run       run a scenario, each run on a fresh database, and write one history per run
            oversold run scenarios/medusa/normal-checkout.yaml [--runs N]
  checkout  run one checkout against the running Medusa without a reset
            oversold checkout --lines OVS-PIECE:2,OVS-TWOPACK:1 --label normal
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
	case "checkout":
		return checkoutCmd(args[1:], stdout, stderr)
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
