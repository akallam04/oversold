package main

import (
	"fmt"
	"io"
	"os"
)

var version = "dev"

const usage = `usage: oversold <command>

commands:
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
