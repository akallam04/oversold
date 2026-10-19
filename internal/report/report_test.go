package report

import (
	"strings"
	"testing"

	"github.com/akallam04/oversold/internal/history"
	"github.com/akallam04/oversold/internal/scenario"
)

func file(t *testing.T) *scenario.File {
	t.Helper()
	f, problems := scenario.Load("../checks/testdata/runs/normal-checkout/normal-checkout.yaml")
	if len(problems) > 0 {
		t.Fatal(problems)
	}
	return f
}

func entry(index int, harness string, results ...history.Result) Entry {
	h := &history.History{Run: history.RunInfo{Index: index, Runs: 5}, HarnessError: harness}
	for i, r := range results {
		h.Verdicts = append(h.Verdicts, history.Verdict{Check: scenario.Checks[i], Version: 1, Result: r, Reason: "r"})
	}
	return Entry{File: "run.json", History: h}
}

func pass4(index int) Entry {
	return entry(index, "", history.Pass, history.Pass, history.Pass, history.Pass)
}

func TestExitCodes(t *testing.T) {
	f := file(t)
	tests := []struct {
		name    string
		entries []Entry
		want    int
	}{
		{"all pass", []Entry{pass4(1), pass4(2)}, ExitPass},
		{"one fail", []Entry{pass4(1), entry(2, "", history.Fail, history.Pass, history.Pass, history.Pass)}, ExitFail},
		{"inconclusive only", []Entry{pass4(1), entry(2, "boom", history.Inconclusive, history.Inconclusive, history.Inconclusive, history.Inconclusive)}, ExitInconclusive},
		{"fail beats inconclusive", []Entry{entry(1, "", history.Inconclusive, history.Inconclusive, history.Inconclusive, history.Inconclusive), entry(2, "", history.Pass, history.Pass, history.Pass, history.Fail)}, ExitFail},
		{"no runs", nil, ExitInconclusive},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Build(f, tt.entries, 5, "")
			if r.ExitCode != tt.want {
				t.Errorf("exit code = %d, want %d", r.ExitCode, tt.want)
			}
		})
	}
}

func TestCountsAndMarkdown(t *testing.T) {
	r := Build(file(t), []Entry{pass4(1), entry(2, "", history.Fail, history.Pass, history.Pass, history.Inconclusive), pass4(3)}, 3, "")
	c := r.Checks[0]
	if c.Check != "no_oversell" || c.Fail != 1 || c.Pass != 2 || c.Inconclusive != 0 || c.Runs != 3 {
		t.Errorf("no_oversell counts = %+v", c)
	}
	if got := r.CountLine(r.Checks[3]); got != "no_orphaned_orders v1: fail 0 of 3, pass 2 of 3, inconclusive 1 of 3" {
		t.Errorf("count line = %q", got)
	}
	md := r.Markdown()
	for _, want := range []string{
		"| no_oversell | 1 | 1 of 3 | 2 of 3 | 0 of 3 |",
		"| 2 | fail | pass | pass | inconclusive | [run.json](run.json) |",
		"- Run 2, no_oversell fail: r",
		"- Exit code: 1",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown lacks %q:\n%s", want, md)
		}
	}
}

func TestStoppedEarly(t *testing.T) {
	bad := func(i int) Entry { return entry(i, "boom") }
	if got := StoppedEarly([]Entry{pass4(1), bad(2), bad(3), bad(4)}, 5); !strings.Contains(got, "3 harness errors in a row") {
		t.Errorf("got %q", got)
	}
	if got := StoppedEarly([]Entry{pass4(1), bad(2)}, 5); got != "only 2 of 5 runs recorded" {
		t.Errorf("got %q", got)
	}
	if got := StoppedEarly([]Entry{pass4(1), pass4(2)}, 2); got != "" {
		t.Errorf("got %q", got)
	}
}
