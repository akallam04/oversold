package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "version", args: []string{"version"}, wantCode: 0, wantStdout: "oversold dev\n"},
		{name: "help", args: []string{"help"}, wantCode: 0, wantStdout: "usage: oversold"},
		{name: "no args", args: nil, wantCode: 2, wantStderr: "usage: oversold"},
		{name: "unknown", args: []string{"bogus"}, wantCode: 2, wantStderr: `unknown command "bogus"`},
		{name: "validate good", args: []string{"validate", "../../internal/scenario/testdata/good.yaml"}, wantCode: 0, wantStdout: "valid, scenario good-example"},
		{name: "validate bad", args: []string{"validate", "../../internal/scenario/testdata/bad.yaml"}, wantCode: 2, wantStderr: "invalid, 15 problems"},
		{name: "validate missing file", args: []string{"validate", "nope.yaml"}, wantCode: 2, wantStderr: "no such file"},
		{name: "validate no file", args: []string{"validate"}, wantCode: 2, wantStderr: "usage: oversold validate"},
		{name: "run no file", args: []string{"run"}, wantCode: 2, wantStderr: "usage: oversold run"},
		{name: "run bad runs", args: []string{"run", "../../internal/scenario/testdata/good.yaml", "--runs", "5000"}, wantCode: 2, wantStderr: "--runs 5000 out of range"},
		{name: "run unbuilt version", args: []string{"run", "../../internal/scenario/testdata/good.yaml", "--version", "2.18.0"}, wantCode: 2, wantStderr: "--version 2.18.0: medusa versions Oversold builds are 2.19.0, 2.21.2, 2.22.0"},
		{name: "run invalid scenario", args: []string{"run", "../../internal/scenario/testdata/bad.yaml"}, wantCode: 2, wantStderr: "faults: must be empty"},
		{name: "versions", args: []string{"versions"}, wantCode: 0, wantStdout: "2.19.0\n2.21.2\n2.22.0\n"},
		{name: "versions extra args", args: []string{"versions", "2.22.0"}, wantCode: 2, wantStderr: "usage: oversold versions"},
		{name: "expect no file", args: []string{"expect"}, wantCode: 2, wantStderr: "usage: oversold expect"},
		{name: "expect invalid file", args: []string{"expect", "../../internal/scenario/testdata/good.yaml"}, wantCode: 2, wantStderr: "field name not found"},
		{name: "expect missing file", args: []string{"expect", "nope.yaml"}, wantCode: 2, wantStderr: "no such file"},
		{name: "expect bad runs", args: []string{"expect", "../../expectations/medusa.yaml", "--runs", "0", "--runs", "1001"}, wantCode: 2, wantStderr: "--runs 1001 out of range 1 to 1000"},
		{name: "expect unbuilt version", args: []string{"expect", "../../expectations/medusa.yaml", "--version", "2.18.0"}, wantCode: 2, wantStderr: "--version 2.18.0: medusa versions Oversold builds are 2.19.0, 2.21.2, 2.22.0"},
		{name: "expect extra args", args: []string{"expect", "../../expectations/medusa.yaml", "more"}, wantCode: 2, wantStderr: "unexpected arguments"},
		{name: "run zero runs", args: []string{"run", "../../internal/scenario/testdata/good.yaml", "--runs", "0"}, wantCode: 2, wantStderr: "--runs 0 out of range 1 to 1000"},
		{name: "run negative runs", args: []string{"run", "../../internal/scenario/testdata/good.yaml", "--runs", "-3"}, wantCode: 2, wantStderr: "--runs -3 out of range 1 to 1000"},
		{name: "expect zero runs", args: []string{"expect", "../../expectations/medusa.yaml", "--runs", "0"}, wantCode: 2, wantStderr: "--runs 0 out of range 1 to 1000"},
		{name: "expect negative runs", args: []string{"expect", "../../expectations/medusa.yaml", "--runs=-1"}, wantCode: 2, wantStderr: "--runs -1 out of range 1 to 1000"},
		{name: "run bad variant name", args: []string{"run", "../../internal/scenario/testdata/good.yaml", "--variant", "PR_1"}, wantCode: 2, wantStderr: `--variant "PR_1" must be kebab-case`},
		{name: "run unknown variant", args: []string{"run", "../../internal/scenario/testdata/good.yaml", "--version", "2.22.0", "--variant", "pr-1"}, wantCode: 2, wantStderr: "--variant pr-1: "},
		{name: "run variant on other version", args: []string{"run", "../../internal/scenario/testdata/good.yaml", "--variant", "pr-16961"}, wantCode: 2, wantStderr: "variant pr-16961 is built on medusa 2.22.0, this run uses 2.21.2; add --version 2.22.0"},
		{name: "variant list", args: []string{"variant"}, wantCode: 0, wantStdout: "pr-16961  medusa 2.22.0  https://github.com/medusajs/medusa/pull/16961 by pqa-labs\n"},
		{name: "variant show", args: []string{"variant", "pr-16961"}, wantCode: 0, wantStdout: "head_sha 5ac8ae437ee0b7771a945b5b30f265b76f214172\n"},
		{name: "variant unknown", args: []string{"variant", "pr-1"}, wantCode: 2, wantStderr: "has no variant pr-1"},
		{name: "variant extra args", args: []string{"variant", "a", "b"}, wantCode: 2, wantStderr: "usage: oversold variant"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
			check(t, "stdout", stdout.String(), tt.wantStdout)
			check(t, "stderr", stderr.String(), tt.wantStderr)
		})
	}
}

func check(t *testing.T, stream, got, want string) {
	t.Helper()
	if want == "" {
		if got != "" {
			t.Errorf("%s = %q, want empty", stream, got)
		}
		return
	}
	if !strings.Contains(got, want) {
		t.Errorf("%s = %q, want it to contain %q", stream, got, want)
	}
}
