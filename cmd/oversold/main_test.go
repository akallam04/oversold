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
