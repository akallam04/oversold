package report

import (
	"encoding/json"
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

func redisEntry(index int, calls int64) Entry {
	e := pass4(index)
	e.History.Target = history.Target{Locking: "redis", RedisVersion: "7.4.11"}
	e.History.Race.LockCalls = &calls
	e.History.Race.RedisCommandStats = &history.CommandStats{
		Before: map[string]history.CommandStat{"evalsha": {Calls: 10}},
		After:  map[string]history.CommandStat{"evalsha": {Calls: 10 + calls}},
	}
	return e
}

func TestLockingInReport(t *testing.T) {
	r := Build(file(t), []Entry{redisEntry(1, 14), redisEntry(2, 9)}, 2, "")
	if r.Locking != "redis" || len(r.RedisVersions) != 1 || r.RedisVersions[0] != "7.4.11" {
		t.Errorf("locking = %q, redis versions = %q", r.Locking, r.RedisVersions)
	}
	if r.Runs[0].LockCalls == nil || *r.Runs[0].LockCalls != 14 {
		t.Errorf("run 1 lock calls = %v", r.Runs[0].LockCalls)
	}
	md := r.Markdown()
	for _, want := range []string{
		"- Locking: redis\n",
		"- Redis 7.4.11\n",
		"- Scripted Redis lock calls during the race: 9 to 14 per run, recorded in 2 runs\n",
		"| Run | no_oversell | reserved_matches_reservations | one_order_per_cart | no_orphaned_orders | Lock calls | History |",
		"| 2 | pass | pass | pass | pass | 9 | [run.json](run.json) |",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown lacks %q:\n%s", want, md)
		}
	}

	r = Build(file(t), []Entry{pass4(1)}, 1, "")
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"locking"`, `"redis_versions"`, `"lock_calls"`} {
		if strings.Contains(string(b), key) {
			t.Errorf("in-memory report written with %s", key)
		}
	}
	if md := r.Markdown(); !strings.Contains(md, "- Locking: in-memory\n") || strings.Contains(md, "Lock calls") {
		t.Errorf("in-memory markdown:\n%s", md)
	}
}

func TestLockCallMismatches(t *testing.T) {
	good := redisEntry(1, 14).History
	if m := lockCallMismatches(good); len(m) != 0 {
		t.Errorf("matching history: %q", m)
	}
	edited := redisEntry(1, 14).History
	n := int64(20)
	edited.Race.LockCalls = &n
	if m := lockCallMismatches(edited); len(m) != 1 || !strings.Contains(m[0], "lock_calls is 20, the stored redis command stats give 14") {
		t.Errorf("edited history: %q", m)
	}
	if m := lockCallMismatches(pass4(1).History); len(m) != 0 {
		t.Errorf("in-memory history: %q", m)
	}
}
