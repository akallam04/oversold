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
	expect(t, verdicts(t, h), map[string]history.Result{"no_orphaned_orders": history.Fail}, "needs 4 WIDGET but holds 2")
	expect(t, versionOne(t, h), map[string]history.Result{"no_orphaned_orders": history.Fail}, "(OVS-TWOPACK) needs 2 WIDGET but holds 0")
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

func TestVersionList(t *testing.T) {
	next := map[string]int{}
	for _, c := range Versions() {
		next[c.Name]++
		if c.Version != next[c.Name] {
			t.Errorf("%s version %d is out of order", c.Name, c.Version)
		}
	}
	for _, c := range All() {
		if c.Version != next[c.Name] {
			t.Errorf("All() gives %s version %d, newest is %d", c.Name, c.Version, next[c.Name])
		}
	}
	for _, sp := range versionOneSpecs {
		if c, ok := LookupVersion(sp.Name, sp.Version); !ok || c.strict {
			t.Errorf("%s version 1 must stay without the stricter preconditions", sp.Name)
		}
	}
}

func TestChecksMatchScenarioList(t *testing.T) {
	if len(All()) != len(scenario.Checks) {
		t.Fatalf("checks %d, scenario names %d", len(All()), len(scenario.Checks))
	}
	for i, c := range All() {
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
	for _, c := range All() {
		heading := fmt.Sprintf("## %s (version %d)", c.Name, c.Version)
		if !strings.Contains(doc, heading) {
			t.Errorf("docs/checks.md has no heading %q", heading)
		}
	}
	for _, c := range Versions() {
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

var versionOneSpecs = []Spec{
	{Name: "no_oversell", Version: 1},
	{Name: "reserved_matches_reservations", Version: 1},
	{Name: "one_order_per_cart", Version: 1},
	{Name: "no_orphaned_orders", Version: 1},
}

func versionOne(t *testing.T, h *history.History) map[string]history.Verdict {
	t.Helper()
	vs, err := EvaluateVersions(h, versionOneSpecs)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]history.Verdict{}
	for _, v := range vs {
		out[v.Check] = v
	}
	return out
}

func TestMissingOrders(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		edit    func(*history.History)
		want    history.Result
		reason  string
	}{
		{"ok call with no saved order", "normal-checkout", func(h *history.History) {
			h.After.Orders = nil
			h.After.CartOrders = nil
		}, history.Fail, "cart main: ok complete call complete#15 returned order order_01M59D4FPAMQQ4V8RAJA5A0JXR, which is not in the after snapshot"},
		{"ok call returns order X while only order Y is linked", "normal-checkout", func(h *history.History) {
			raceOps(h)[0].ResponseBody = json.RawMessage(`{"type":"order","order":{"id":"order_x"}}`)
		}, history.Fail, "returned order order_x, which is not in the after snapshot"},
		{"ok call returns a saved order linked to another cart", "normal-checkout", func(h *history.History) {
			raceOps(h)[0].ResponseBody = json.RawMessage(`{"type":"order","order":{"id":"order_x"}}`)
			h.After.Orders = append(h.After.Orders, dbstate.Order{ID: "order_x", Status: "pending"})
			h.After.CartOrders = append(h.After.CartOrders, dbstate.CartOrder{ID: "ordercart_x", CartID: "cart_other", OrderID: "order_x"})
		}, history.Fail, "returned order order_x, which is not linked to this cart"},
		{"ok call returns a canceled order", "normal-checkout", func(h *history.History) {
			now := time.Now()
			h.After.Orders[0].CanceledAt = &now
		}, history.Fail, "which is canceled"},
		{"two ok calls for the same valid order", "duplicate-complete", func(h *history.History) {},
			history.Pass, "every ok complete call returned a saved order linked to its cart (2 checked)"},
		{"unknown call skips the cart", "duplicate-complete", func(h *history.History) {
			ops := raceOps(h)
			ops[0].Outcome = history.Unknown
			ops[0].Status = 0
			ops[0].Timeout = true
			ops[0].ResponseBody = nil
			ops[1].ResponseBody = json.RawMessage(`{"type":"order","order":{"id":"order_gone"}}`)
		}, history.Pass, "every ok complete call returned a saved order linked to its cart (0 checked); carts skipped for an unknown call: main"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := load(t, tt.fixture)
			tt.edit(h)
			got := verdicts(t, h)
			v := got["no_missing_orders"]
			if v.Result != tt.want || !strings.Contains(v.Reason, tt.reason) {
				t.Errorf("no_missing_orders = %s (%s), want %s with %q", v.Result, v.Reason, tt.want, tt.reason)
			}
			if v.Version != 1 {
				t.Errorf("no_missing_orders version = %d", v.Version)
			}
		})
	}
}

func TestMissingOrderPassesTheOlderChecks(t *testing.T) {
	h := load(t, "normal-checkout")
	h.After.Orders = nil
	h.After.CartOrders = nil
	expect(t, versionOne(t, h), allResultsOf(versionOneSpecs, history.Pass), "")
	expect(t, verdicts(t, h), map[string]history.Result{
		"no_oversell":                   history.Pass,
		"reserved_matches_reservations": history.Pass,
		"one_order_per_cart":            history.Pass,
		"no_orphaned_orders":            history.Pass,
		"no_missing_orders":             history.Fail,
	}, "which is not in the after snapshot")
}

func allResultsOf(specs []Spec, r history.Result) map[string]history.Result {
	out := map[string]history.Result{}
	for _, sp := range specs {
		out[sp.Name] = r
	}
	return out
}

func twoPieceLines(t *testing.T, reserved string) *history.History {
	t.Helper()
	h := load(t, "normal-checkout")
	o := &h.After.Orders[0]
	var piece *dbstate.OrderLine
	for i := range o.Lines {
		if deref(o.Lines[i].VariantSKU) == "OVS-PIECE" {
			piece = &o.Lines[i]
		}
	}
	if piece == nil || piece.Quantity != "2" {
		t.Fatalf("fixture order has no OVS-PIECE line of 2: %+v", o.Lines)
	}
	oldLine := piece.LineItemID
	a, b := *piece, *piece
	a.LineItemID, a.OrderItemID, a.Quantity = "ordli_piece_a", "orditem_piece_a", "1"
	b.LineItemID, b.OrderItemID, b.Quantity = "ordli_piece_b", "orditem_piece_b", "1"
	lines := []dbstate.OrderLine{a, b}
	for _, l := range o.Lines {
		if l.LineItemID != oldLine {
			lines = append(lines, l)
		}
	}
	o.Lines = lines

	cartLine := "cali_piece"
	h.After.CartLines = append(h.After.CartLines, dbstate.CartLine{ID: cartLine, CartID: h.Carts["main"], VariantID: a.VariantID, VariantSKU: a.VariantSKU, Quantity: "2"})
	found := false
	for i := range h.After.Reservations {
		r := &h.After.Reservations[i]
		if r.LineItemID != nil && *r.LineItemID == oldLine {
			r.LineItemID = &cartLine
			r.Quantity = json.Number(reserved)
			found = true
		}
	}
	if !found {
		t.Fatal("fixture has no reservation for the OVS-PIECE line")
	}
	live := 2
	if reserved == "1" {
		live = 1
	}
	widget(h.After).ReservedQuantity = json.Number(fmt.Sprint(live + 2))
	return h
}

func TestReservationCountedOncePerOrder(t *testing.T) {
	t.Run("one reservation of 1 for two lines that need 1 each", func(t *testing.T) {
		h := twoPieceLines(t, "1")
		expect(t, versionOne(t, h), map[string]history.Result{"no_orphaned_orders": history.Pass}, "")
		expect(t, verdicts(t, h), map[string]history.Result{
			"no_orphaned_orders":            history.Fail,
			"reserved_matches_reservations": history.Pass,
			"no_oversell":                   history.Pass,
		}, "needs 4 WIDGET but holds 3")
	})
	t.Run("one reservation of 2 for two lines that need 1 each", func(t *testing.T) {
		h := twoPieceLines(t, "2")
		expect(t, versionOne(t, h), map[string]history.Result{"no_orphaned_orders": history.Fail}, "(OVS-PIECE) needs 1 WIDGET but holds 2")
		got := verdicts(t, h)
		expect(t, got, allResults(history.Pass), "")
		for _, e := range got["no_orphaned_orders"].Evidence {
			if strings.HasPrefix(e.Subject, "order ") && (e.Numbers["units_needed"] != "4" || e.Numbers["units_reserved"] != "4" || e.Numbers["lines"] != "3") {
				t.Errorf("order evidence = %+v", e.Numbers)
			}
		}
	})
}

func TestMissingLevels(t *testing.T) {
	without := func(h *history.History, sku string) string {
		var keep []dbstate.InventoryLevel
		loc := ""
		for _, l := range h.After.InventoryLevels {
			if l.SKU == sku {
				loc = l.LocationID
				continue
			}
			keep = append(keep, l)
		}
		h.After.InventoryLevels = keep
		return loc
	}
	tests := []struct {
		name    string
		edit    func(*history.History) []string
		v1Clean bool
	}{
		{"level gone without a live reservation", func(h *history.History) []string {
			loc := without(h, "SOLO")
			return []string{"after snapshot is missing rows: SOLO at " + loc + " is in the before snapshot but not in the after snapshot"}
		}, true},
		{"level gone with a live reservation", func(h *history.History) []string {
			loc := without(h, "WIDGET")
			return []string{
				"after snapshot is missing rows: WIDGET at " + loc + " is in the before snapshot but not in the after snapshot",
				"live reservation " + h.After.Reservations[0].ID + " for WIDGET at " + loc + " has no inventory level",
			}
		}, false},
		{"live reservation at a location with no level", func(h *history.History) []string {
			r := h.After.Reservations[0]
			r.ID = "resitem_elsewhere"
			r.LocationID = "sloc_elsewhere"
			h.After.Reservations = append(h.After.Reservations, r)
			return []string{"after snapshot is missing rows: live reservation resitem_elsewhere for WIDGET at sloc_elsewhere has no inventory level"}
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := load(t, "normal-checkout")
			reasons := tt.edit(h)
			got := verdicts(t, h)
			for _, reason := range reasons {
				expect(t, got, allResults(history.Inconclusive), reason)
			}
			old := versionOne(t, h)
			for _, sp := range versionOneSpecs {
				if v := old[sp.Name]; v.Version != 1 || v.Result == history.Inconclusive {
					t.Errorf("%s version 1 = %+v, want the old verdict without the new precondition", sp.Name, v)
				}
			}
			if tt.v1Clean {
				expect(t, old, allResultsOf(versionOneSpecs, history.Pass), "")
			}
		})
	}
}

func TestUnreadableQuantity(t *testing.T) {
	for _, field := range []string{"stocked_quantity", "reserved_quantity"} {
		t.Run(field, func(t *testing.T) {
			h := load(t, "normal-checkout")
			w := widget(h.After)
			if field == "stocked_quantity" {
				w.StockedQuantity = ""
			} else {
				w.ReservedQuantity = ""
			}
			vs, err := Evaluate(h, scenario.Checks)
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]history.Verdict{}
			for _, v := range vs {
				got[v.Check] = v
			}
			expect(t, got, allResults(history.Inconclusive), "after snapshot has unreadable quantities: WIDGET at "+w.LocationID+" has unreadable "+field+` ""`)

			_, err = EvaluateVersions(h, versionOneSpecs)
			if err == nil || !strings.Contains(err.Error(), "could not read this history") {
				t.Errorf("version 1 error = %v, want an error instead of a panic", err)
			}
		})
	}
}

func TestRecheckUsesStoredVersions(t *testing.T) {
	h := load(t, "single-cart-over-demand")
	stored, err := EvaluateVersions(h, versionOneSpecs)
	if err != nil {
		t.Fatal(err)
	}
	h.Verdicts = stored
	again, err := Recheck(h)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(stored)
	b, _ := json.Marshal(again)
	if string(a) != string(b) {
		t.Errorf("recheck changed the stored verdicts:\n%s\n%s", a, b)
	}
	for _, v := range again {
		if v.Version != 1 {
			t.Errorf("%s rechecked at version %d", v.Check, v.Version)
		}
	}

	h.Verdicts = []history.Verdict{{Check: "no_oversell", Version: 9}}
	if _, err := Recheck(h); err == nil || !strings.Contains(err.Error(), `check "no_oversell" has no version 9`) {
		t.Errorf("unknown version error = %v", err)
	}
	h.Verdicts = []history.Verdict{{Check: "no_free_lunch", Version: 1}}
	if _, err := Recheck(h); err == nil || !strings.Contains(err.Error(), "unknown check") {
		t.Errorf("unknown check error = %v", err)
	}
	h.Verdicts = nil
	if _, err := Recheck(h); err == nil {
		t.Error("want an error for a history without stored verdicts")
	}
}
