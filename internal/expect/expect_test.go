package expect

import (
	"errors"
	"strings"
	"testing"

	"github.com/akallam04/oversold/internal/history"
	"github.com/akallam04/oversold/internal/report"
	"github.com/akallam04/oversold/internal/targets"
)

func root(t *testing.T) string {
	t.Helper()
	r, err := targets.FindRoot()
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestShippedExpectations(t *testing.T) {
	f, problems := Load("../../expectations/medusa.yaml")
	if len(problems) != 0 {
		t.Fatalf("%q", problems)
	}
	e := f.Expectations
	if e.Runs != 3 || len(e.Cases) != 6 {
		t.Fatalf("runs %d, cases %d", e.Runs, len(e.Cases))
	}
	want := map[string]map[string]string{
		"single-cart-over-demand": {"no_oversell": FailEveryRun, "reserved_matches_reservations": PassEveryRun, "one_order_per_cart": PassEveryRun, "no_orphaned_orders": PassEveryRun},
		"normal-checkout":         {"no_oversell": PassEveryRun, "reserved_matches_reservations": PassEveryRun, "one_order_per_cart": PassEveryRun, "no_orphaned_orders": PassEveryRun},
		"last-unit-two-servers":   {"no_oversell": FailAtLeastOnce, "reserved_matches_reservations": FailAtLeastOnce, "one_order_per_cart": PassEveryRun, "no_orphaned_orders": PassEveryRun},
		"last-unit-one-server":    {"no_oversell": PassEveryRun, "reserved_matches_reservations": PassEveryRun, "one_order_per_cart": PassEveryRun, "no_orphaned_orders": PassEveryRun},
		"counter-two-servers":     {"no_oversell": PassEveryRun, "reserved_matches_reservations": FailAtLeastOnce, "one_order_per_cart": PassEveryRun, "no_orphaned_orders": PassEveryRun},
		"counter-one-server":      {"no_oversell": PassEveryRun, "reserved_matches_reservations": PassEveryRun, "one_order_per_cart": PassEveryRun, "no_orphaned_orders": PassEveryRun},
	}
	for _, c := range e.Cases {
		name := c.File.Scenario.Name
		w, ok := want[name]
		if !ok {
			t.Errorf("unexpected case %s", name)
			continue
		}
		if len(c.Expect) != len(w) {
			t.Errorf("%s: %v", name, c.Expect)
		}
		for k, v := range w {
			if c.Expect[k] != v {
				t.Errorf("%s %s = %q, want %q", name, k, c.Expect[k], v)
			}
		}
	}
}

const good = `schema: oversold.expect/v1
description: one bug and its control
runs: 3
cases:
  - scenario: scenarios/medusa/single-cart-over-demand.yaml
    note: "#16502"
    expect:
      no_oversell: fail_every_run
      reserved_matches_reservations: pass_every_run
      one_order_per_cart: pass_every_run
      no_orphaned_orders: pass_every_run
  - scenario: scenarios/medusa/normal-checkout.yaml
    expect:
      no_oversell: pass_every_run
      reserved_matches_reservations: pass_every_run
      one_order_per_cart: pass_every_run
      no_orphaned_orders: pass_every_run
`

func TestGoodFile(t *testing.T) {
	e, problems := Parse([]byte(good), root(t))
	if len(problems) != 0 {
		t.Fatalf("%q", problems)
	}
	if e.Cases[0].File == nil || e.Cases[0].File.Scenario.Name != "single-cart-over-demand" || e.Cases[1].Note != "" {
		t.Errorf("cases = %+v", e.Cases)
	}
	if e.Cases[0].File.Path != "scenarios/medusa/single-cart-over-demand.yaml" {
		t.Errorf("path = %s", e.Cases[0].File.Path)
	}
}

func TestBadFiles(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"empty", "", "file is empty"},
		{"schema", strings.Replace(good, "oversold.expect/v1", "oversold.expect/v2", 1), `schema: got "oversold.expect/v2"`},
		{"no schema", strings.Replace(good, "schema: oversold.expect/v1\n", "", 1), "schema: required"},
		{"unknown field", strings.Replace(good, "runs: 3", "runs: 3\nretries: 2", 1), "field retries not found"},
		{"unknown case field", strings.Replace(good, `    note: "#16502"`, `    note: "#16502"`+"\n    owner: someone", 1), "field owner not found"},
		{"runs zero", strings.Replace(good, "runs: 3", "runs: 0", 1), "runs: got 0, want 1 to 1000"},
		{"runs too many", strings.Replace(good, "runs: 3", "runs: 1001", 1), "runs: got 1001"},
		{"no runs", strings.Replace(good, "runs: 3\n", "", 1), "runs: got 0"},
		{"no cases", "schema: oversold.expect/v1\nruns: 3\ncases: []\n", "cases: at least one case"},
		{"unknown check", strings.Replace(good, "      no_oversell: fail_every_run", "      no_oversell: fail_every_run\n      orders_paid: pass_every_run", 1), "cases[0].expect.orders_paid: unknown check"},
		{"missing check", strings.Replace(good, "      no_oversell: fail_every_run\n", "", 1), "cases[0].expect.no_oversell: missing"},
		{"bad kind", strings.Replace(good, "no_oversell: fail_every_run", "no_oversell: fail_sometimes", 1), `cases[0].expect.no_oversell: got "fail_sometimes"`},
		{"no expect", "schema: oversold.expect/v1\nruns: 3\ncases:\n  - scenario: scenarios/medusa/normal-checkout.yaml\n", "cases[0].expect: required"},
		{"no scenario", "schema: oversold.expect/v1\nruns: 3\ncases:\n  - note: x\n    expect:\n      no_oversell: pass_every_run\n", "cases[0].scenario: required"},
		{"missing scenario file", strings.Replace(good, "single-cart-over-demand.yaml", "no-such-scenario.yaml", 1), "no such file"},
		{"outside the repository", strings.Replace(good, "scenarios/medusa/single-cart-over-demand.yaml", "../elsewhere/x.yaml", 1), "must be a clean .yaml path"},
		{"absolute path", strings.Replace(good, "scenarios/medusa/single-cart-over-demand.yaml", "/tmp/x.yaml", 1), "must be a clean .yaml path"},
		{"invalid scenario", strings.Replace(good, "scenarios/medusa/single-cart-over-demand.yaml", "internal/scenario/testdata/bad.yaml", 1), "is not a valid scenario"},
		{"duplicate scenario", strings.Replace(good, "scenarios/medusa/normal-checkout.yaml", "scenarios/medusa/single-cart-over-demand.yaml", 1), "is already cases[0]"},
		{"duplicate check", strings.Replace(good, "      no_oversell: fail_every_run", "      no_oversell: fail_every_run\n      no_oversell: pass_every_run", 1), "already defined"},
		{"long note", strings.Replace(good, `note: "#16502"`, `note: "`+strings.Repeat("x", 81)+`"`, 1), "cases[0].note: one line"},
		{"two documents", good + "---\nschema: oversold.expect/v1\n", "exactly one YAML document"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, problems := Parse([]byte(tt.src), root(t))
			if joined := strings.Join(problems, "\n"); !strings.Contains(joined, tt.want) {
				t.Errorf("want problem containing %q, got:\n%s", tt.want, joined)
			}
		})
	}
}

func TestCheckNotRunByScenario(t *testing.T) {
	src := "schema: oversold.expect/v1\nruns: 1\ncases:\n  - scenario: internal/scenario/testdata/good.yaml\n    expect:\n      no_oversell: pass_every_run\n      one_order_per_cart: pass_every_run\n      no_orphaned_orders: pass_every_run\n"
	_, problems := Parse([]byte(src), root(t))
	if len(problems) != 1 || !strings.Contains(problems[0], "cases[0].expect.no_orphaned_orders: good-example does not run this check") {
		t.Errorf("problems = %q", problems)
	}
}

func TestMet(t *testing.T) {
	tests := []struct {
		kind             string
		runs, fail, pass int
		want             bool
	}{
		{PassEveryRun, 3, 0, 3, true},
		{PassEveryRun, 3, 1, 2, false},
		{PassEveryRun, 3, 0, 2, false},
		{PassEveryRun, 0, 0, 0, false},
		{FailEveryRun, 3, 3, 0, true},
		{FailEveryRun, 3, 2, 1, false},
		{FailEveryRun, 3, 2, 0, false},
		{FailEveryRun, 0, 0, 0, false},
		{FailAtLeastOnce, 3, 1, 2, true},
		{FailAtLeastOnce, 3, 3, 0, true},
		{FailAtLeastOnce, 3, 0, 3, false},
		{"fail_sometimes", 3, 3, 0, false},
	}
	for _, tt := range tests {
		if got := Met(tt.kind, tt.runs, tt.fail, tt.pass); got != tt.want {
			t.Errorf("Met(%s, runs %d, fail %d, pass %d) = %v, want %v", tt.kind, tt.runs, tt.fail, tt.pass, got, tt.want)
		}
	}
}

func TestExitCodeOrder(t *testing.T) {
	tests := []struct {
		troubled, unmet bool
		want            int
	}{
		{false, false, 0},
		{false, true, 1},
		{true, false, 3},
		{true, true, 3},
	}
	for _, tt := range tests {
		if got := ExitCode(tt.troubled, tt.unmet); got != tt.want {
			t.Errorf("ExitCode(troubled %v, unmet %v) = %d, want %d", tt.troubled, tt.unmet, got, tt.want)
		}
	}
}

func fakeReport(runs int, counts map[string][2]int, harness, inconclusive int) *report.Report {
	r := &report.Report{RunsRecorded: runs}
	for _, name := range []string{"no_oversell", "reserved_matches_reservations", "one_order_per_cart", "no_orphaned_orders"} {
		c := counts[name]
		r.Checks = append(r.Checks, report.CheckCount{Check: name, Runs: runs, Fail: c[0], Pass: c[1], Inconclusive: runs - c[0] - c[1]})
	}
	for i := 1; i <= runs; i++ {
		run := report.Run{Index: i}
		if i <= harness {
			run.HarnessError = "reset: boom"
		}
		result := history.Pass
		if i <= inconclusive {
			result = history.Inconclusive
		}
		run.Verdicts = []report.RunVerdict{{Check: "no_oversell", Result: result}}
		r.Runs = append(r.Runs, run)
	}
	return r
}

func shippedCase(t *testing.T, name string) Case {
	t.Helper()
	f, problems := Load("../../expectations/medusa.yaml")
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	for _, c := range f.Expectations.Cases {
		if c.File.Scenario.Name == name {
			return c
		}
	}
	t.Fatalf("no case %s", name)
	return Case{}
}

func TestSummaryAllMet(t *testing.T) {
	m := history.NewMasker()
	bug := shippedCase(t, "single-cart-over-demand")
	control := shippedCase(t, "normal-checkout")
	s := &Summary{MedusaVersion: "2.22.0"}
	s.Cases = append(s.Cases,
		CaseFrom(bug, 3, fakeReport(3, map[string][2]int{"no_oversell": {3, 0}, "reserved_matches_reservations": {0, 3}, "one_order_per_cart": {0, 3}, "no_orphaned_orders": {0, 3}}, 0, 0), "a", nil, m),
		CaseFrom(control, 3, fakeReport(3, map[string][2]int{"no_oversell": {0, 3}, "reserved_matches_reservations": {0, 3}, "one_order_per_cart": {0, 3}, "no_orphaned_orders": {0, 3}}, 0, 0), "b", nil, m),
	)
	s.Finish()
	if s.ExitCode != 0 || len(s.Unmet) != 0 || len(s.Problems) != 0 {
		t.Fatalf("exit %d, unmet %q, problems %q", s.ExitCode, s.Unmet, s.Problems)
	}
	md := s.Markdown()
	if !strings.Contains(md, "| single-cart-over-demand | #16502 | no_oversell | fail 3 of 3 | fail_every_run | yes |") {
		t.Errorf("markdown:\n%s", md)
	}
}

func TestSummaryUnmet(t *testing.T) {
	m := history.NewMasker()
	bug := shippedCase(t, "single-cart-over-demand")
	s := &Summary{MedusaVersion: "2.23.0"}
	s.Cases = append(s.Cases, CaseFrom(bug, 3, fakeReport(3, map[string][2]int{"no_oversell": {0, 3}, "reserved_matches_reservations": {0, 3}, "one_order_per_cart": {0, 3}, "no_orphaned_orders": {0, 3}}, 0, 0), "a", nil, m))
	s.Finish()
	want := "single-cart-over-demand on 2.23.0: no_oversell failed in 0 of 3 runs, expected every run. Medusa may have changed this; run the full scenario set before saying so."
	if s.ExitCode != 1 || len(s.Unmet) != 1 || s.Unmet[0] != want {
		t.Fatalf("exit %d, unmet %q", s.ExitCode, s.Unmet)
	}
}

func TestSummaryInconclusiveWins(t *testing.T) {
	m := history.NewMasker("hunter2-secret-value")
	bug := shippedCase(t, "single-cart-over-demand")
	control := shippedCase(t, "normal-checkout")
	s := &Summary{MedusaVersion: "2.22.0"}
	s.Cases = append(s.Cases,
		CaseFrom(bug, 3, fakeReport(3, map[string][2]int{"no_oversell": {0, 2}, "reserved_matches_reservations": {0, 2}, "one_order_per_cart": {0, 2}, "no_orphaned_orders": {0, 2}}, 1, 1), "a", nil, m),
		CaseFrom(control, 3, nil, "", errors.New("connect: password hunter2-secret-value rejected"), m),
	)
	s.Finish()
	if s.ExitCode != 3 {
		t.Fatalf("exit %d, want 3 when a run was inconclusive even with unmet expectations", s.ExitCode)
	}
	if len(s.Unmet) == 0 {
		t.Error("unmet expectations should still be listed")
	}
	joined := strings.Join(s.Problems, "\n")
	for _, want := range []string{"single-cart-over-demand: 1 of 3 runs had a harness error", "single-cart-over-demand: 1 of 3 runs were inconclusive", "normal-checkout: connect"} {
		if !strings.Contains(joined, want) {
			t.Errorf("want %q in %q", want, joined)
		}
	}
	if strings.Contains(joined, "hunter2-secret-value") {
		t.Error("a secret reached the summary")
	}
}

func TestSummaryShortRunSet(t *testing.T) {
	m := history.NewMasker()
	control := shippedCase(t, "normal-checkout")
	s := &Summary{MedusaVersion: "2.22.0"}
	s.Cases = append(s.Cases, CaseFrom(control, 3, fakeReport(2, map[string][2]int{"no_oversell": {0, 2}, "reserved_matches_reservations": {0, 2}, "one_order_per_cart": {0, 2}, "no_orphaned_orders": {0, 2}}, 0, 0), "a", nil, m))
	s.Finish()
	if s.ExitCode != 3 || !strings.Contains(strings.Join(s.Problems, "\n"), "only 2 of 3 runs recorded") {
		t.Fatalf("exit %d, problems %q", s.ExitCode, s.Problems)
	}
}
