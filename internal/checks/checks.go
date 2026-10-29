package checks

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/akallam04/oversold/internal/dbstate"
	"github.com/akallam04/oversold/internal/history"
	"github.com/akallam04/oversold/internal/scenario"
)

type Check struct {
	Name    string
	Version int
	eval    func(*state) (history.Result, string, []history.Evidence)
}

var all = []Check{
	{Name: "no_oversell", Version: 1, eval: noOversell},
	{Name: "reserved_matches_reservations", Version: 1, eval: reservedMatchesReservations},
	{Name: "one_order_per_cart", Version: 1, eval: oneOrderPerCart},
	{Name: "no_orphaned_orders", Version: 1, eval: noOrphanedOrders},
}

func All() []Check {
	out := make([]Check, len(all))
	copy(out, all)
	return out
}

func Lookup(name string) (Check, bool) {
	for _, c := range all {
		if c.Name == name {
			return c, true
		}
	}
	return Check{}, false
}

func Evaluate(h *history.History, names []string) ([]history.Verdict, error) {
	st, why := prepare(h)
	out := make([]history.Verdict, 0, len(names))
	for _, name := range names {
		c, ok := Lookup(name)
		if !ok {
			return nil, fmt.Errorf("unknown check %q", name)
		}
		v := history.Verdict{Check: c.Name, Version: c.Version}
		if why != "" {
			v.Result = history.Inconclusive
			v.Reason = why
		} else {
			v.Result, v.Reason, v.Evidence = c.eval(st)
		}
		out = append(out, v)
	}
	return out, nil
}

type levelKey struct {
	item     string
	location string
}

type need struct {
	item     string
	sku      string
	required *big.Rat
}

type state struct {
	h        *history.History
	before   *dbstate.Snapshot
	after    *dbstate.Snapshot
	itemSKU  map[string]string
	needs    map[string][]need
	live     map[levelKey]*big.Rat
	orders   []dbstate.Order
	carts    map[string]string
	complete map[string][]history.Operation
}

func prepare(h *history.History) (*state, string) {
	if h == nil {
		return nil, "no history"
	}
	if h.HarnessError != "" {
		return nil, "harness error: " + firstLine(h.HarnessError, 200)
	}
	if h.Before == nil {
		return nil, "before snapshot is missing"
	}
	if h.After == nil {
		return nil, "after snapshot is missing"
	}

	st := &state{
		h:        h,
		before:   h.Before,
		after:    h.After,
		itemSKU:  map[string]string{},
		needs:    map[string][]need{},
		carts:    map[string]string{},
		complete: map[string][]history.Operation{},
	}
	var bad []string

	for _, s := range []*dbstate.Snapshot{h.Before, h.After} {
		for _, l := range s.InventoryLevels {
			st.itemSKU[l.InventoryItemID] = l.SKU
		}
	}

	if problems := cleanStart(h); len(problems) > 0 {
		return nil, "before snapshot is not a clean start: " + strings.Join(problems, "; ")
	}
	if len(h.After.InventoryLevels) == 0 {
		return nil, "after snapshot has no inventory levels"
	}

	variants := h.After.Variants
	if len(variants) == 0 {
		variants = h.Before.Variants
	}
	for _, v := range variants {
		req, ok := rat(v.RequiredQuantity)
		if !ok {
			bad = append(bad, fmt.Sprintf("variant %s has unreadable required_quantity %q", v.VariantSKU, v.RequiredQuantity))
			continue
		}
		n := need{item: v.InventoryItemID, sku: v.InventorySKU, required: req}
		st.needs[v.VariantID] = append(st.needs[v.VariantID], n)
		if v.VariantSKU != "" {
			st.needs["sku:"+v.VariantSKU] = append(st.needs["sku:"+v.VariantSKU], n)
		}
		if v.InventorySKU != "" {
			st.itemSKU[v.InventoryItemID] = v.InventorySKU
		}
	}

	live, err := liveSums(h.After)
	if err != "" {
		bad = append(bad, err)
	}
	st.live = live

	for _, o := range h.After.Orders {
		if o.CanceledAt != nil || o.Status == "canceled" {
			continue
		}
		st.orders = append(st.orders, o)
		for _, l := range o.Lines {
			if len(st.lineNeeds(l)) == 0 {
				bad = append(bad, fmt.Sprintf("after snapshot has no variants entry for order %s line %s", o.ID, l.LineItemID))
			}
			if _, ok := rat(l.Quantity); !ok {
				bad = append(bad, fmt.Sprintf("order %s line %s has unreadable quantity %q", o.ID, l.LineItemID, l.Quantity))
			}
		}
	}
	if len(bad) > 0 {
		return nil, "after snapshot is missing data: " + strings.Join(bad, "; ")
	}

	for name, id := range h.Carts {
		st.carts[id] = name
	}
	for _, op := range h.Operations {
		if op.Phase == history.PhaseRace && op.Step == history.StepComplete {
			st.complete[op.Cart] = append(st.complete[op.Cart], op)
		}
	}
	return st, ""
}

func cleanStart(h *history.History) []string {
	var p []string
	b := h.Before
	if n := len(b.Orders); n > 0 {
		p = append(p, fmt.Sprintf("%d orders", n))
	}
	if n := len(b.Reservations); n > 0 {
		p = append(p, fmt.Sprintf("%d live reservations", n))
	}
	live, err := liveSums(b)
	if err != "" {
		p = append(p, err)
	}
	seen := map[string]bool{}
	for _, l := range b.InventoryLevels {
		seen[l.SKU] = true
		reserved, ok1 := rat(l.ReservedQuantity)
		stocked, ok2 := rat(l.StockedQuantity)
		if !ok1 || !ok2 {
			p = append(p, fmt.Sprintf("%s has unreadable quantities", l.SKU))
			continue
		}
		sum := live[levelKey{l.InventoryItemID, l.LocationID}]
		if sum == nil {
			sum = new(big.Rat)
		}
		if reserved.Cmp(sum) != 0 {
			p = append(p, fmt.Sprintf("%s reserved_quantity %s but live reservations sum to %s", l.SKU, num(reserved), num(sum)))
		}
		want, ok := expectedStock(h, l.SKU)
		if !ok {
			p = append(p, fmt.Sprintf("no expected stock for %s", l.SKU))
		} else if stocked.Cmp(big.NewRat(int64(want), 1)) != 0 {
			p = append(p, fmt.Sprintf("%s stocked %s but scenario and seed give %d", l.SKU, num(stocked), want))
		}
	}
	for _, sku := range scenario.InventorySKUs {
		if !seen[sku] {
			p = append(p, fmt.Sprintf("no inventory level for %s", sku))
		}
	}
	return p
}

func expectedStock(h *history.History, sku string) (int, bool) {
	if v, ok := h.StockBefore[sku]; ok {
		return v, true
	}
	if v, ok := h.Stock[sku]; ok {
		return v, true
	}
	v, ok := scenario.SeededStock[sku]
	return v, ok
}

func liveSums(s *dbstate.Snapshot) (map[levelKey]*big.Rat, string) {
	out := map[levelKey]*big.Rat{}
	for _, r := range s.Reservations {
		q, ok := rat(r.Quantity)
		if !ok {
			return out, fmt.Sprintf("reservation %s has unreadable quantity %q", r.ID, r.Quantity)
		}
		k := levelKey{r.InventoryItemID, r.LocationID}
		if out[k] == nil {
			out[k] = new(big.Rat)
		}
		out[k].Add(out[k], q)
	}
	return out, ""
}

func (st *state) lineNeeds(l dbstate.OrderLine) []need {
	if l.VariantID != nil {
		if n := st.needs[*l.VariantID]; len(n) > 0 {
			return n
		}
	}
	if l.VariantSKU != nil {
		return st.needs["sku:"+*l.VariantSKU]
	}
	return nil
}

func (st *state) liveAt(k levelKey) *big.Rat {
	if v := st.live[k]; v != nil {
		return v
	}
	return new(big.Rat)
}

func (st *state) orderUnits() map[string]*big.Rat {
	out := map[string]*big.Rat{}
	for _, o := range st.orders {
		for _, l := range o.Lines {
			qty, _ := rat(l.Quantity)
			for _, n := range st.lineNeeds(l) {
				if out[n.item] == nil {
					out[n.item] = new(big.Rat)
				}
				out[n.item].Add(out[n.item], new(big.Rat).Mul(qty, n.required))
			}
		}
	}
	return out
}

func (st *state) sortedAfterLevels() []dbstate.InventoryLevel {
	levels := make([]dbstate.InventoryLevel, len(st.after.InventoryLevels))
	copy(levels, st.after.InventoryLevels)
	sort.Slice(levels, func(i, j int) bool {
		if levels[i].SKU != levels[j].SKU {
			return levels[i].SKU < levels[j].SKU
		}
		return levels[i].LocationID < levels[j].LocationID
	})
	return levels
}

func noOversell(st *state) (history.Result, string, []history.Evidence) {
	units := st.orderUnits()
	stockedBefore := map[string]*big.Rat{}
	beforeLevel := map[levelKey]*big.Rat{}
	for _, l := range st.before.InventoryLevels {
		q, _ := rat(l.StockedQuantity)
		if stockedBefore[l.InventoryItemID] == nil {
			stockedBefore[l.InventoryItemID] = new(big.Rat)
		}
		stockedBefore[l.InventoryItemID].Add(stockedBefore[l.InventoryItemID], q)
		beforeLevel[levelKey{l.InventoryItemID, l.LocationID}] = q
	}

	var problems, summary []string
	var ev []history.Evidence
	for _, l := range st.sortedAfterLevels() {
		k := levelKey{l.InventoryItemID, l.LocationID}
		stockedAfter, _ := rat(l.StockedQuantity)
		live := st.liveAt(k)
		need := zeroIfNil(units[l.InventoryItemID])
		before := zeroIfNil(beforeLevel[k])
		ev = append(ev, history.Evidence{
			Subject: st.levelName(l),
			Numbers: map[string]json.Number{
				"stocked_before":    num(before),
				"stocked_after":     num(stockedAfter),
				"live_reservations": num(live),
				"order_units":       num(need),
			},
		})
		if live.Cmp(stockedAfter) > 0 {
			problems = append(problems, fmt.Sprintf("%s live reservations %s exceed stocked %s", l.SKU, num(live), num(stockedAfter)))
		}
		summary = append(summary, fmt.Sprintf("%s live %s of %s stocked", l.SKU, num(live), num(stockedAfter)))
	}

	items := make([]string, 0, len(units))
	for item := range units {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return st.itemSKU[items[i]] < st.itemSKU[items[j]] })
	for _, item := range items {
		have := zeroIfNil(stockedBefore[item])
		if units[item].Cmp(have) > 0 {
			problems = append(problems, fmt.Sprintf("%s orders need %s units but %s were stocked at the start of the race", st.skuOf(item), num(units[item]), num(have)))
		}
		summary = append(summary, fmt.Sprintf("%s orders need %s of %s", st.skuOf(item), num(units[item]), num(have)))
	}

	if len(problems) > 0 {
		return history.Fail, strings.Join(problems, "; "), ev
	}
	return history.Pass, strings.Join(summary, "; "), ev
}

func reservedMatchesReservations(st *state) (history.Result, string, []history.Evidence) {
	var problems, summary []string
	var ev []history.Evidence
	for _, l := range st.sortedAfterLevels() {
		reserved, _ := rat(l.ReservedQuantity)
		live := st.liveAt(levelKey{l.InventoryItemID, l.LocationID})
		ev = append(ev, history.Evidence{
			Subject: st.levelName(l),
			Numbers: map[string]json.Number{
				"reserved_quantity": num(reserved),
				"live_reservations": num(live),
			},
		})
		if reserved.Cmp(live) != 0 {
			problems = append(problems, fmt.Sprintf("%s reserved_quantity %s but live reservations sum to %s", l.SKU, num(reserved), num(live)))
		}
		summary = append(summary, fmt.Sprintf("%s reserved %s = live %s", l.SKU, num(reserved), num(live)))
	}
	if len(problems) > 0 {
		return history.Fail, strings.Join(problems, "; "), ev
	}
	return history.Pass, strings.Join(summary, "; "), ev
}

func oneOrderPerCart(st *state) (history.Result, string, []history.Evidence) {
	byCart := st.linksByCart()
	var problems []string
	var ev []history.Evidence

	cartIDs := make([]string, 0, len(byCart))
	for id := range byCart {
		cartIDs = append(cartIDs, id)
	}
	sort.Strings(cartIDs)
	for _, id := range cartIDs {
		if orders := byCart[id]; len(orders) > 1 {
			problems = append(problems, fmt.Sprintf("cart %s is linked to %d orders: %s", st.cartName(id), len(orders), strings.Join(orders, ", ")))
		}
	}

	names := st.cartNames()
	for _, name := range names {
		id := st.h.Carts[name]
		returned := map[string]bool{}
		var okCalls []string
		for _, op := range st.complete[name] {
			if op.Outcome != history.OK {
				continue
			}
			oid := responseOrderID(op.ResponseBody)
			returned[oid] = true
			okCalls = append(okCalls, fmt.Sprintf("%s#%d=%s", op.ActionID, op.ID, oid))
		}
		ids := sortedKeys(returned)
		ev = append(ev, history.Evidence{
			Subject: "cart " + name,
			Numbers: map[string]json.Number{
				"linked_orders":      json.Number(fmt.Sprint(len(byCart[id]))),
				"ok_complete_calls":  json.Number(fmt.Sprint(len(okCalls))),
				"distinct_order_ids": json.Number(fmt.Sprint(len(ids))),
			},
			Values: map[string]string{
				"cart_id":       id,
				"linked_orders": strings.Join(byCart[id], ","),
				"ok_calls":      strings.Join(okCalls, ","),
			},
		})
		if len(ids) > 1 {
			problems = append(problems, fmt.Sprintf("cart %s: ok complete calls returned %d different orders (%s)", name, len(ids), strings.Join(okCalls, ", ")))
		}
	}

	if len(problems) > 0 {
		return history.Fail, strings.Join(problems, "; "), ev
	}
	var summary []string
	for _, name := range names {
		summary = append(summary, fmt.Sprintf("cart %s: %d order", name, len(byCart[st.h.Carts[name]])))
	}
	return history.Pass, strings.Join(summary, "; "), ev
}

func noOrphanedOrders(st *state) (history.Result, string, []history.Evidence) {
	byOrder := map[string][]string{}
	for _, l := range st.after.CartOrders {
		if l.DeletedAt == nil {
			byOrder[l.OrderID] = append(byOrder[l.OrderID], l.CartID)
		}
	}
	byCart := st.linksByCart()

	var problems []string
	var ev []history.Evidence
	for _, o := range st.orders {
		carts := byOrder[o.ID]
		if len(carts) == 0 {
			problems = append(problems, fmt.Sprintf("order %s is not linked to any cart", o.ID))
		}
		for _, c := range carts {
			if _, ok := st.carts[c]; !ok {
				problems = append(problems, fmt.Sprintf("order %s is linked to cart %s, which is not a scenario cart", o.ID, c))
			}
		}

		totalNeed, totalHeld := new(big.Rat), new(big.Rat)
		for _, l := range o.Lines {
			qty, _ := rat(l.Quantity)
			cartLines := st.cartLinesFor(carts, l)
			for _, n := range st.lineNeeds(l) {
				want := new(big.Rat).Mul(qty, n.required)
				held := new(big.Rat)
				for _, r := range st.after.Reservations {
					if r.InventoryItemID != n.item || r.LineItemID == nil {
						continue
					}
					if *r.LineItemID == l.LineItemID || cartLines[*r.LineItemID] {
						q, _ := rat(r.Quantity)
						held.Add(held, q)
					}
				}
				totalNeed.Add(totalNeed, want)
				totalHeld.Add(totalHeld, held)
				if want.Cmp(held) != 0 {
					problems = append(problems, fmt.Sprintf("order %s line %s (%s) needs %s %s but holds %s", o.ID, l.LineItemID, deref(l.VariantSKU), num(want), st.skuOf(n.item), num(held)))
				}
			}
		}
		ev = append(ev, history.Evidence{
			Subject: "order " + o.ID,
			Numbers: map[string]json.Number{
				"lines":          json.Number(fmt.Sprint(len(o.Lines))),
				"units_needed":   num(totalNeed),
				"units_reserved": num(totalHeld),
			},
			Values: map[string]string{"carts": strings.Join(carts, ",")},
		})
	}

	for _, name := range st.cartNames() {
		ops := st.complete[name]
		if len(ops) == 0 {
			continue
		}
		var outcomes []string
		failed, unknown := 0, 0
		for _, op := range ops {
			outcomes = append(outcomes, fmt.Sprintf("%s#%d=%s", op.ActionID, op.ID, op.Outcome))
			switch op.Outcome {
			case history.Failed:
				failed++
			case history.Unknown:
				unknown++
			}
		}
		id := st.h.Carts[name]
		status := "has an ok call"
		switch {
		case unknown > 0:
			status = "skipped, has an unknown call"
		case failed == len(ops):
			status = "every call failed"
			if orders := byCart[id]; len(orders) > 0 {
				problems = append(problems, fmt.Sprintf("cart %s: every complete call failed but it is linked to order %s", name, strings.Join(orders, ", ")))
			}
		}
		ev = append(ev, history.Evidence{
			Subject: "cart " + name,
			Values: map[string]string{
				"complete_calls": strings.Join(outcomes, ","),
				"linked_orders":  strings.Join(byCart[id], ","),
				"failed_rule":    status,
			},
		})
	}

	if len(problems) > 0 {
		return history.Fail, strings.Join(problems, "; "), ev
	}
	return history.Pass, fmt.Sprintf("%d orders, each linked to a scenario cart and holding the reservations its lines need", len(st.orders)), ev
}

func (st *state) linksByCart() map[string][]string {
	sets := map[string]map[string]bool{}
	for _, l := range st.after.CartOrders {
		if l.DeletedAt != nil {
			continue
		}
		if sets[l.CartID] == nil {
			sets[l.CartID] = map[string]bool{}
		}
		sets[l.CartID][l.OrderID] = true
	}
	out := map[string][]string{}
	for c, set := range sets {
		out[c] = sortedKeys(set)
	}
	return out
}

func (st *state) cartLinesFor(carts []string, l dbstate.OrderLine) map[string]bool {
	in := map[string]bool{}
	for _, c := range carts {
		in[c] = true
	}
	out := map[string]bool{}
	for _, cl := range st.after.CartLines {
		if !in[cl.CartID] {
			continue
		}
		sameVariant := l.VariantID != nil && cl.VariantID != nil && *l.VariantID == *cl.VariantID
		sameSKU := l.VariantSKU != nil && cl.VariantSKU != nil && *l.VariantSKU == *cl.VariantSKU
		if sameVariant || sameSKU {
			out[cl.ID] = true
		}
	}
	return out
}

func (st *state) cartNames() []string {
	names := make([]string, 0, len(st.h.Carts))
	for name := range st.h.Carts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (st *state) cartName(id string) string {
	if name, ok := st.carts[id]; ok {
		return name + " (" + id + ")"
	}
	return id
}

func (st *state) skuOf(item string) string {
	if sku := st.itemSKU[item]; sku != "" {
		return sku
	}
	return item
}

func (st *state) levelName(l dbstate.InventoryLevel) string {
	return st.skuOf(l.InventoryItemID) + " at " + l.LocationID
}

func responseOrderID(body json.RawMessage) string {
	var r struct {
		Order struct {
			ID string `json:"id"`
		} `json:"order"`
	}
	if json.Unmarshal(body, &r) != nil {
		return ""
	}
	return r.Order.ID
}

func rat(n json.Number) (*big.Rat, bool) {
	if n == "" {
		return nil, false
	}
	return new(big.Rat).SetString(string(n))
}

func num(r *big.Rat) json.Number {
	if r == nil {
		return "0"
	}
	if r.IsInt() {
		return json.Number(r.Num().String())
	}
	return json.Number(r.FloatString(6))
}

func zeroIfNil(r *big.Rat) *big.Rat {
	if r == nil {
		return new(big.Rat)
	}
	return r
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func firstLine(s string, n int) string {
	s, _, _ = strings.Cut(s, "\n")
	if len(s) > n {
		s = s[:n] + "..."
	}
	return s
}

func Summarize(verdicts []history.Verdict) history.Result {
	result := history.Pass
	for _, v := range verdicts {
		switch v.Result {
		case history.Fail:
			return history.Fail
		case history.Inconclusive:
			result = history.Inconclusive
		}
	}
	return result
}
