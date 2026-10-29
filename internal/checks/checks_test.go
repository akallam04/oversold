package checks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/akallam04/oversold/internal/dbstate"
	"github.com/akallam04/oversold/internal/history"
	"github.com/akallam04/oversold/internal/scenario"
)

func load(t *testing.T, name string) *history.History {
	t.Helper()
	h, err := history.Read(filepath.Join("testdata", "runs", name, "run-001.json"))
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func verdicts(t *testing.T, h *history.History) map[string]history.Verdict {
	t.Helper()
	vs, err := Evaluate(h, scenario.Checks)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]history.Verdict{}
	for _, v := range vs {
		out[v.Check] = v
	}
	return out
}

func expect(t *testing.T, got map[string]history.Verdict, want map[string]history.Result, reason string) {
	t.Helper()
	for name, res := range want {
		v := got[name]
		if v.Result != res {
			t.Errorf("%s = %s (%s), want %s", name, v.Result, v.Reason, res)
			continue
		}
		if reason != "" && res != history.Pass && !strings.Contains(v.Reason, reason) {
			t.Errorf("%s reason %q does not mention %q", name, v.Reason, reason)
		}
	}
}

func allResults(r history.Result) map[string]history.Result {
	out := map[string]history.Result{}
	for _, c := range scenario.Checks {
		out[c] = r
	}
	return out
}

func widget(s *dbstate.Snapshot) *dbstate.InventoryLevel {
	for i := range s.InventoryLevels {
		if s.InventoryLevels[i].SKU == "WIDGET" {
			return &s.InventoryLevels[i]
		}
	}
	return nil
}

func raceOps(h *history.History) []*history.Operation {
	var out []*history.Operation
	for i := range h.Operations {
		if h.Operations[i].Phase == history.PhaseRace {
			out = append(out, &h.Operations[i])
		}
	}
	return out
}

func TestRealNormalCheckoutPasses(t *testing.T) {
	got := verdicts(t, load(t, "normal-checkout"))
	expect(t, got, allResults(history.Pass), "")
	if v := got["no_oversell"]; !strings.Contains(v.Reason, "WIDGET live 4 of 10 stocked") || len(v.Evidence) != 2 {
		t.Errorf("no_oversell = %+v", v)
	}
}

func TestRealOverDemandFailsNoOversell(t *testing.T) {
	got := verdicts(t, load(t, "single-cart-over-demand"))
	expect(t, got, map[string]history.Result{
		"no_oversell":                   history.Fail,
		"reserved_matches_reservations": history.Pass,
		"one_order_per_cart":            history.Pass,
		"no_orphaned_orders":            history.Pass,
	}, "")
	reason := got["no_oversell"].Reason
	for _, want := range []string{"WIDGET live reservations 8 exceed stocked 6", "WIDGET orders need 8 units but 6 were stocked at the start of the race"} {
		if !strings.Contains(reason, want) {
			t.Errorf("no_oversell reason %q lacks %q", reason, want)
		}
	}
	var found bool
	for _, e := range got["no_oversell"].Evidence {
		if strings.HasPrefix(e.Subject, "WIDGET") {
			found = true
			if e.Numbers["stocked_before"] != "6" || e.Numbers["live_reservations"] != "8" || e.Numbers["order_units"] != "8" {
				t.Errorf("WIDGET evidence = %+v", e.Numbers)
			}
		}
	}
	if !found {
		t.Error("no WIDGET evidence")
	}
}

func TestRealDuplicateCompletePasses(t *testing.T) {
	got := verdicts(t, load(t, "duplicate-complete"))
	expect(t, got, allResults(history.Pass), "")
	ev := got["one_order_per_cart"].Evidence[0]
	if ev.Numbers["ok_complete_calls"] != "2" || ev.Numbers["distinct_order_ids"] != "1" {
		t.Errorf("one_order_per_cart evidence = %+v", ev)
	}
}

func TestCounterDoesNotMatchReservations(t *testing.T) {
	h := load(t, "normal-checkout")
	widget(h.After).ReservedQuantity = "5"
	expect(t, verdicts(t, h), map[string]history.Result{
		"reserved_matches_reservations": history.Fail,
		"no_oversell":                   history.Pass,
	}, "WIDGET reserved_quantity 5 but live reservations sum to 4")
}

func TestTwoOrdersForOneCart(t *testing.T) {
	h := load(t, "normal-checkout")
	first := h.After.Orders[0]
	second := first
	second.ID = "order_second"
	second.Lines = nil
	for _, l := range first.Lines {
		l.LineItemID = "ordli_second_" + l.LineItemID
		second.Lines = append(second.Lines, l)
	}
	h.After.Orders = append(h.After.Orders, second)
	h.After.CartOrders = append(h.After.CartOrders, dbstate.CartOrder{ID: "ordercart_second", CartID: h.Carts["main"], OrderID: second.ID})
	expect(t, verdicts(t, h), map[string]history.Result{"one_order_per_cart": history.Fail}, "is linked to 2 orders")
}

func TestOkCallsReturnDifferentOrders(t *testing.T) {
	h := load(t, "duplicate-complete")
	ops := raceOps(h)
	if len(ops) != 2 {
		t.Fatalf("want 2 race ops, got %d", len(ops))
	}
	ops[1].ResponseBody = json.RawMessage(`{"type":"order","order":{"id":"order_other"}}`)
	expect(t, verdicts(t, h), map[string]history.Result{"one_order_per_cart": history.Fail}, "returned 2 different orders")
}

func TestOrderMissingReservation(t *testing.T) {
	h := load(t, "normal-checkout")
	h.After.Reservations = h.After.Reservations[1:]
	expect(t, verdicts(t, h), map[string]history.Result{"no_orphaned_orders": history.Fail}, "holds 0")
}

func TestReservationsTiedThroughCartLines(t *testing.T) {
	h := load(t, "normal-checkout")
	cart := h.Carts["main"]
	for i := range h.After.Reservations {
		r := &h.After.Reservations[i]
		id := "cali_" + fmt.Sprint(i)
		h.After.CartLines = append(h.After.CartLines, dbstate.CartLine{ID: id, CartID: cart, VariantSKU: r.LineItemSKU, Quantity: "1"})
		r.LineItemID = &id
	}
	expect(t, verdicts(t, h), map[string]history.Result{"no_orphaned_orders": history.Pass}, "")
}

func TestOrderFromNonScenarioCart(t *testing.T) {
	h := load(t, "normal-checkout")
	h.Carts = map[string]string{"main": "cart_somebody_else"}
	expect(t, verdicts(t, h), map[string]history.Result{"no_orphaned_orders": history.Fail}, "not a scenario cart")
}

func TestOrderAfterOnlyCompleteFailed(t *testing.T) {
	h := load(t, "normal-checkout")
	op := raceOps(h)[0]
	op.Outcome = history.Failed
	op.Status = 409
	op.ResponseBody = json.RawMessage(`{"type":"conflict","message":"busy"}`)
	expect(t, verdicts(t, h), map[string]history.Result{
		"no_orphaned_orders": history.Fail,
		"one_order_per_cart": history.Pass,
	}, "every complete call failed but it is linked to order")
}

func TestUnknownCompleteDoesNotCount(t *testing.T) {
	h := load(t, "normal-checkout")
	op := raceOps(h)[0]
	op.Outcome = history.Unknown
	op.Status = 0
	op.Timeout = true
	op.ResponseBody = nil
	got := verdicts(t, h)
	expect(t, got, allResults(history.Pass), "")
	found := false
	for _, e := range got["no_orphaned_orders"].Evidence {
		if e.Values["failed_rule"] == "skipped, has an unknown call" {
			found = true
		}
	}
	if !found {
		t.Errorf("unknown call was not skipped: %+v", got["no_orphaned_orders"].Evidence)
	}
}

func TestOversellByOrdersOnly(t *testing.T) {
	h := load(t, "normal-checkout")
	h.Stock = map[string]int{"WIDGET": 3}
	widget(h.Before).StockedQuantity = "3"
	widget(h.After).StockedQuantity = "4"
	expect(t, verdicts(t, h), map[string]history.Result{"no_oversell": history.Fail}, "orders need 4 units but 3 were stocked at the start of the race")
}

func TestInconclusive(t *testing.T) {
	tests := []struct {
		name   string
		edit   func(*history.History)
		reason string
	}{
		{"harness error", func(h *history.History) { h.HarnessError = "reset: medusa not healthy" }, "harness error: reset: medusa not healthy"},
		{"before missing", func(h *history.History) { h.Before = nil }, "before snapshot is missing"},
		{"after missing", func(h *history.History) { h.After = nil }, "after snapshot is missing"},
		{"order before race", func(h *history.History) { h.Before.Orders = h.After.Orders }, "not a clean start: 1 orders"},
		{"reservation before race", func(h *history.History) {
			h.Before.Reservations = h.After.Reservations[:1]
			widget(h.Before).ReservedQuantity = "2"
		}, "not a clean start: 1 live reservations"},
		{"counter mismatch before race", func(h *history.History) { widget(h.Before).ReservedQuantity = "1" }, "WIDGET reserved_quantity 1 but live reservations sum to 0"},
		{"stock differs from scenario", func(h *history.History) { widget(h.Before).StockedQuantity = "9" }, "WIDGET stocked 9 but scenario and seed give 10"},
		{"seed stock differs", func(h *history.History) {
			for i := range h.Before.InventoryLevels {
				if h.Before.InventoryLevels[i].SKU == "SOLO" {
					h.Before.InventoryLevels[i].StockedQuantity = "2"
				}
			}
		}, "SOLO stocked 2 but scenario and seed give 1"},
		{"after without levels", func(h *history.History) { h.After.InventoryLevels = nil }, "after snapshot has no inventory levels"},
		{"after without variants", func(h *history.History) {
			h.After.Variants = nil
			h.Before.Variants = nil
		}, "no variants entry"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := load(t, "normal-checkout")
			tt.edit(h)
			expect(t, verdicts(t, h), allResults(history.Inconclusive), tt.reason)
		})
	}
}

func TestUnknownCheckName(t *testing.T) {
	if _, err := Evaluate(load(t, "normal-checkout"), []string{"no_free_lunch"}); err == nil {
		t.Error("want error for unknown check")
	}
}

func TestChecksMatchScenarioList(t *testing.T) {
	if len(all) != len(scenario.Checks) {
		t.Fatalf("checks %d, scenario names %d", len(all), len(scenario.Checks))
	}
	for i, c := range all {
		if c.Name != scenario.Checks[i] {
			t.Errorf("check %d is %s, scenario lists %s", i, c.Name, scenario.Checks[i])
		}
	}
}

func TestDocsMatchVersions(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "docs", "checks.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	for _, c := range all {
		heading := fmt.Sprintf("## %s (version %d)", c.Name, c.Version)
		if !strings.Contains(doc, heading) {
			t.Errorf("docs/checks.md has no heading %q", heading)
		}
		line := regexp.MustCompile(`(?m)^- ` + c.Name + ` ` + fmt.Sprint(c.Version) + `: `)
		if !line.MatchString(doc) {
			t.Errorf("docs/checks.md version history has no line for %s %d", c.Name, c.Version)
		}
	}
}

func TestVerdictsAreStableAcrossTime(t *testing.T) {
	h := load(t, "normal-checkout")
	a, _ := Evaluate(h, scenario.Checks)
	time.Sleep(time.Millisecond)
	b, _ := Evaluate(h, scenario.Checks)
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Error("verdicts changed between two evaluations of the same history")
	}
}

func TestCleanStartUsesStockBeforeRace(t *testing.T) {
	h := load(t, "single-cart-over-demand")
	h.Stock = map[string]int{"WIDGET": 10}
	expect(t, verdicts(t, h), allResults(history.Inconclusive), "WIDGET stocked 6 but scenario and seed give 10")

	h.StockBefore = map[string]int{"WIDGET": 6}
	got := verdicts(t, h)
	expect(t, got, map[string]history.Result{
		"no_oversell":                   history.Fail,
		"reserved_matches_reservations": history.Pass,
		"one_order_per_cart":            history.Pass,
		"no_orphaned_orders":            history.Pass,
	}, "")
	if reason := got["no_oversell"].Reason; !strings.Contains(reason, "WIDGET live reservations 8 exceed stocked 6") {
		t.Errorf("no_oversell reason %q", reason)
	}

	h.Stock = map[string]int{"WIDGET": 6}
	h.StockBefore = map[string]int{"WIDGET": 10}
	expect(t, verdicts(t, h), allResults(history.Inconclusive), "WIDGET stocked 6 but scenario and seed give 10")
}
