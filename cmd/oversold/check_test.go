package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func copyRunFolder(t *testing.T, name string) string {
	t.Helper()
	src := filepath.Join("..", "..", "internal", "checks", "testdata", "runs", name)
	dst := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}

type savedReport struct {
	ExitCode int `json:"exit_code"`
	Checks   []struct {
		Check        string `json:"check"`
		Fail         int    `json:"fail"`
		Pass         int    `json:"pass"`
		Inconclusive int    `json:"inconclusive"`
	} `json:"checks"`
}

func TestCheckCopiedHistories(t *testing.T) {
	tests := []struct {
		name     string
		wantCode int
		wantFail map[string]int
		wantLine string
	}{
		{"normal-checkout", 0, map[string]int{}, "no_oversell v1: fail 0 of 1, pass 1 of 1, inconclusive 0 of 1"},
		{"single-cart-over-demand", 1, map[string]int{"no_oversell": 1}, "no_oversell v1: fail 1 of 1, pass 0 of 1, inconclusive 0 of 1"},
		{"duplicate-complete", 0, map[string]int{}, "one_order_per_cart v1: fail 0 of 1, pass 1 of 1, inconclusive 0 of 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := copyRunFolder(t, tt.name)
			before, _ := os.ReadFile(filepath.Join(dir, "run-001.json"))

			var stdout, stderr bytes.Buffer
			code := run([]string{"check", dir}, &stdout, &stderr)
			if code != tt.wantCode {
				t.Fatalf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, tt.wantCode, stdout.String(), stderr.String())
			}
			out := stdout.String()
			if !strings.Contains(out, tt.wantLine) {
				t.Errorf("stdout lacks %q:\n%s", tt.wantLine, out)
			}
			lines := strings.Split(strings.TrimSpace(out), "\n")
			if last := lines[len(lines)-1]; last != filepath.Join(dir, "report.md") {
				t.Errorf("last line = %q, want the report path", last)
			}

			b, err := os.ReadFile(filepath.Join(dir, "report.json"))
			if err != nil {
				t.Fatal(err)
			}
			var rep savedReport
			if err := json.Unmarshal(b, &rep); err != nil {
				t.Fatal(err)
			}
			if rep.ExitCode != tt.wantCode || len(rep.Checks) != 4 {
				t.Errorf("report = %+v", rep)
			}
			for _, c := range rep.Checks {
				if c.Fail != tt.wantFail[c.Check] || c.Inconclusive != 0 || c.Pass+c.Fail != 1 {
					t.Errorf("%s counts = %+v", c.Check, c)
				}
			}
			md, _ := os.ReadFile(filepath.Join(dir, "report.md"))
			if !strings.Contains(string(md), "[run-001.json](run-001.json)") {
				t.Errorf("report.md does not link the history:\n%s", md)
			}
			after, _ := os.ReadFile(filepath.Join(dir, "run-001.json"))
			if !bytes.Equal(before, after) {
				t.Error("check modified the history file")
			}
		})
	}
}

func TestCheckHarnessErrorIsInconclusive(t *testing.T) {
	dir := copyRunFolder(t, "normal-checkout")
	path := filepath.Join(dir, "run-001.json")
	var h map[string]any
	b, _ := os.ReadFile(path)
	json.Unmarshal(b, &h)
	h["harness_error"] = "reset: medusa not healthy"
	b, _ = json.Marshal(h)
	os.WriteFile(path, b, 0o644)

	var stdout, stderr bytes.Buffer
	if code := run([]string{"check", dir}, &stdout, &stderr); code != 3 {
		t.Fatalf("exit code = %d, want 3\n%s%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "inconclusive 1 of 1") {
		t.Errorf("stdout:\n%s", stdout.String())
	}
}

func TestCheckBadFolder(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"check", t.TempDir()}, &stdout, &stderr); code != 2 {
		t.Errorf("empty folder exit code = %d, want 2", code)
	}
	dir := copyRunFolder(t, "normal-checkout")
	os.WriteFile(filepath.Join(dir, "normal-checkout.yaml"), []byte("schema: oversold.scenario/v1\n"), 0o644)
	if code := run([]string{"check", dir}, &stdout, &stderr); code != 2 {
		t.Errorf("invalid scenario exit code = %d, want 2", code)
	}
	if code := run([]string{"check"}, &stdout, &stderr); code != 2 {
		t.Errorf("no folder exit code = %d, want 2", code)
	}
}

func TestCheckRejectsChangedScenario(t *testing.T) {
	dir := copyRunFolder(t, "normal-checkout")
	p := filepath.Join(dir, "normal-checkout.yaml")
	b, _ := os.ReadFile(p)
	os.WriteFile(p, append(b, []byte("\n")...), 0o644)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"check", dir}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "does not match") {
		t.Errorf("exit code %d, stderr %s", code, stderr.String())
	}
}

func TestExportRoundTrip(t *testing.T) {
	for _, tt := range []struct {
		name string
		code int
	}{{"normal-checkout", 0}, {"single-cart-over-demand", 1}} {
		t.Run(tt.name, func(t *testing.T) {
			src := copyRunFolder(t, tt.name)
			var stdout, stderr bytes.Buffer
			if code := run([]string{"check", src}, &stdout, &stderr); code != tt.code {
				t.Fatalf("check source = %d\n%s", code, stderr.String())
			}
			srcReport, _ := os.ReadFile(filepath.Join(src, "report.json"))

			dst := filepath.Join(t.TempDir(), "export")
			if code := run([]string{"export", src, dst}, &stdout, &stderr); code != 0 {
				t.Fatalf("export = %d\n%s", code, stderr.String())
			}
			entries, _ := os.ReadDir(dst)
			var names []string
			for _, e := range entries {
				names = append(names, e.Name())
			}
			want := []string{tt.name + ".yaml", "report.json", "report.md", "run-001.json.gz"}
			sort.Strings(want)
			if strings.Join(names, ",") != strings.Join(want, ",") {
				t.Errorf("export holds %q, want %q", names, want)
			}
			md, _ := os.ReadFile(filepath.Join(dst, "report.md"))
			if !strings.Contains(string(md), "[run-001.json.gz](run-001.json.gz)") {
				t.Errorf("exported report.md does not link the gzipped history:\n%s", md)
			}

			stdout.Reset()
			if code := run([]string{"check", dst}, &stdout, &stderr); code != tt.code {
				t.Fatalf("check export = %d, want %d\n%s", code, tt.code, stderr.String())
			}
			if !strings.Contains(stdout.String(), "stored verdicts matched in 0 of 0") {
				t.Logf("check output:\n%s", stdout.String())
			}
			dstReport, _ := os.ReadFile(filepath.Join(dst, "report.json"))
			if !sameVerdicts(t, srcReport, dstReport) {
				t.Error("check on the export gave different verdicts than on the source")
			}

			if code := run([]string{"export", src, dst}, &stdout, &stderr); code != 2 {
				t.Errorf("export into a non-empty folder = %d, want 2", code)
			}
		})
	}
}

func TestCheckRefusesPlainAndGzipTwins(t *testing.T) {
	dir := copyRunFolder(t, "normal-checkout")
	b, _ := os.ReadFile(filepath.Join(dir, "run-001.json"))
	os.WriteFile(filepath.Join(dir, "run-001.json.gz"), b, 0o644)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"check", dir}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "both run-001.json and run-001.json.gz") {
		t.Errorf("exit %d, stderr %s", code, stderr.String())
	}
}

func sameVerdicts(t *testing.T, a, b []byte) bool {
	t.Helper()
	type run struct {
		Index    int `json:"index"`
		Verdicts []struct {
			Check  string `json:"check"`
			Result string `json:"result"`
			Reason string `json:"reason"`
		} `json:"verdicts"`
	}
	var ra, rb struct {
		ExitCode int               `json:"exit_code"`
		Checks   []json.RawMessage `json:"checks"`
		Runs     []run             `json:"runs"`
	}
	if json.Unmarshal(a, &ra) != nil || json.Unmarshal(b, &rb) != nil {
		return false
	}
	ja, _ := json.Marshal([]any{ra.ExitCode, ra.Checks, ra.Runs})
	jb, _ := json.Marshal([]any{rb.ExitCode, rb.Checks, rb.Runs})
	return bytes.Equal(ja, jb)
}
