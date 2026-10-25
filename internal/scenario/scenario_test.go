package scenario

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestGoodFile(t *testing.T) {
	f, problems := Load("testdata/good.yaml")
	if len(problems) != 0 {
		t.Fatalf("problems: %q", problems)
	}
	s := f.Scenario
	if s.Name != "good-example" || s.Target.Servers != 2 || s.Runs != 3 || *s.Seed != 42 {
		t.Errorf("decoded = %+v", s)
	}
	if !slices.Equal(s.CartOrder, []string{"zeta", "alpha"}) {
		t.Errorf("cart order = %q, want file order", s.CartOrder)
	}
	if s.RequestTimeout() != 5*time.Second {
		t.Errorf("timeout = %v", s.RequestTimeout())
	}
	if lo, hi := s.JitterRange(); lo != 0 || hi != 5*time.Millisecond {
		t.Errorf("jitter = %v %v", lo, hi)
	}
	if len(f.SHA256) != 64 {
		t.Errorf("sha256 = %q", f.SHA256)
	}
}

func TestDefaultTimeout(t *testing.T) {
	s, problems := Parse([]byte(minimal("")))
	if len(problems) != 0 {
		t.Fatalf("problems: %q", problems)
	}
	if s.RequestTimeout() != 30*time.Second {
		t.Errorf("default timeout = %v", s.RequestTimeout())
	}
}

func TestBadFileReportsEveryProblem(t *testing.T) {
	_, problems := Load("testdata/bad.yaml")
	want := []string{
		"field colour not found",
		`schema: got "oversold.scenario/v2"`,
		"name: \"Bad_Name\" must be kebab-case",
		"target.version: medusa 2.18.0 is not a version",
		"stock.GADGET: unknown inventory SKU",
		`carts.main.lines[0].sku: unknown SKU "OVS-NOPE"`,
		"race[0].server: got 2, want 1 to 1",
		`race[0].do: got "refund"`,
		`race[0].cart: no cart named "missing"`,
		`timing.release: got "whenever"`,
		"timing.jitter_ms: want 0 <= min <= max",
		"timing.request_timeout_ms: got 10",
		"faults: must be empty",
		"runs: got 1001",
		`checks[1]: unknown check "no_free_lunch"`,
	}
	joined := strings.Join(problems, "\n")
	for _, w := range want {
		if !strings.Contains(joined, w) {
			t.Errorf("missing problem %q in:\n%s", w, joined)
		}
	}
	if len(problems) != len(want) {
		t.Errorf("got %d problems, want %d:\n%s", len(problems), len(want), joined)
	}
}

func TestSingleProblems(t *testing.T) {
	tests := []struct {
		name string
		edit func(string) string
		want string
	}{
		{"unknown top-level field", func(s string) string { return s + "retries: 3\n" }, "field retries not found"},
		{"unknown cart field", func(s string) string {
			return strings.Replace(s, "    server: 1\n    lines:", "    server: 1\n    coupon: SAVE10\n    lines:", 1)
		}, "field coupon not found"},
		{"unknown SKU", func(s string) string { return strings.Replace(s, "OVS-PIECE", "OVS-GIZMO", 1) }, `unknown SKU "OVS-GIZMO"`},
		{"unknown check", func(s string) string { return strings.Replace(s, "- no_oversell", "- no_oversale", 1) }, `unknown check "no_oversale"`},
		{"wrong schema", func(s string) string {
			return strings.Replace(s, "oversold.scenario/v1", "oversold.scenario/v0", 1)
		}, `schema: got "oversold.scenario/v0"`},
		{"missing schema", func(s string) string { return strings.Replace(s, "schema: oversold.scenario/v1\n", "", 1) }, "schema: required"},
		{"faults not empty", func(s string) string { return strings.Replace(s, "faults: []", "faults:\n  - kind: kill-server", 1) }, "faults: must be empty"},
		{"runs zero", func(s string) string { return strings.Replace(s, "runs: 3", "runs: 0", 1) }, "runs: got 0, want 1 to 1000"},
		{"runs too many", func(s string) string { return strings.Replace(s, "runs: 3", "runs: 5000", 1) }, "runs: got 5000, want 1 to 1000"},
		{"seed missing", func(s string) string { return strings.Replace(s, "seed: 1\n", "", 1) }, "seed: required"},
		{"seed not an integer", func(s string) string { return strings.Replace(s, "seed: 1", "seed: one", 1) }, "cannot unmarshal"},
		{"three servers", func(s string) string { return strings.Replace(s, "servers: 1", "servers: 3", 1) }, "target.servers: got 3"},
		{"unknown engine", func(s string) string { return strings.Replace(s, "engine: medusa", "engine: saleor", 1) }, `unknown engine "saleor"`},
		{"duplicate action id", func(s string) string {
			return strings.Replace(s, "timing:", "  - id: a\n    server: 1\n    do: complete\n    cart: main\ntiming:", 1)
		}, `race[1].id: "a" is used twice`},
		{"two documents", func(s string) string { return s + "---\nname: other\n" }, "exactly one YAML document"},
		{"empty checks", func(s string) string { return strings.Replace(s, "checks:\n  - no_oversell\n", "checks: []\n", 1) }, "checks: an empty list is not allowed"},
		{"null checks", func(s string) string { return strings.Replace(s, "checks:\n  - no_oversell\n", "checks:\n", 1) }, "checks: has no value"},
		{"duplicate check", func(s string) string { return s + "  - no_oversell\n" }, `checks[1]: "no_oversell" is listed twice`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, problems := Parse([]byte(tt.edit(minimal(""))))
			joined := strings.Join(problems, "\n")
			if !strings.Contains(joined, tt.want) {
				t.Errorf("want problem containing %q, got:\n%s", tt.want, joined)
			}
		})
	}
}

func TestEmptyFile(t *testing.T) {
	if _, problems := Parse(nil); len(problems) == 0 {
		t.Error("empty file should be invalid")
	}
}

func TestVersionsComeFromTargetFolders(t *testing.T) {
	versions, err := Versions("medusa")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join("..", "..", "targets", "medusa", "versions"))
	if err != nil {
		t.Fatal(err)
	}
	var folders []string
	for _, e := range entries {
		if e.IsDir() {
			folders = append(folders, e.Name())
		}
	}
	if len(versions) != len(folders) {
		t.Fatalf("versions %v, folders %v", versions, folders)
	}
	for _, v := range []string{"2.19.0", "2.21.2", "2.22.0"} {
		if _, problems := Parse([]byte(strings.Replace(minimal(""), "version: 2.21.2", "version: "+v, 1))); len(problems) != 0 {
			t.Errorf("version %s: %q", v, problems)
		}
	}
	_, problems := Parse([]byte(strings.Replace(minimal(""), "version: 2.21.2", "version: 2.18.0", 1)))
	if want := "medusa 2.18.0 is not a version Oversold builds, want one of 2.19.0, 2.21.2, 2.22.0"; !strings.Contains(strings.Join(problems, "\n"), want) {
		t.Errorf("want %q in %q", want, problems)
	}
}

func TestShippedScenariosAreValid(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "scenarios", "*", "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no scenario files found")
	}
	for _, f := range files {
		file, problems := Load(f)
		if len(problems) != 0 {
			t.Errorf("%s: %q", f, problems)
			continue
		}
		if want := strings.TrimSuffix(filepath.Base(f), ".yaml"); file.Scenario.Name != want {
			t.Errorf("%s: name %q does not match the file name", f, file.Scenario.Name)
		}
	}
}

func minimal(extra string) string {
	return `schema: oversold.scenario/v1
name: minimal
description: Smallest valid scenario.
target:
  engine: medusa
  version: 2.21.2
  servers: 1
carts:
  main:
    server: 1
    lines:
      - sku: OVS-PIECE
        qty: 1
race:
  - id: a
    server: 1
    do: complete
    cart: main
timing:
  release: sequential
faults: []
runs: 3
seed: 1
checks:
  - no_oversell
` + extra
}

func TestDocsExampleIsValid(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "docs", "scenario-format.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	start := strings.Index(doc, "```yaml\n")
	if start < 0 {
		t.Fatal("no yaml example in docs/scenario-format.md")
	}
	rest := doc[start+len("```yaml\n"):]
	end := strings.Index(rest, "```")
	if _, problems := Parse([]byte(rest[:end])); len(problems) != 0 {
		t.Errorf("docs example is invalid: %q", problems)
	}
}

func TestChecksDefaultToAll(t *testing.T) {
	src := strings.Replace(minimal(""), "checks:\n  - no_oversell\n", "", 1)
	s, problems := Parse([]byte(src))
	if len(problems) != 0 {
		t.Fatalf("problems: %q", problems)
	}
	if got := s.SelectedChecks(); !slices.Equal(got, Checks) {
		t.Errorf("selected = %q, want all four", got)
	}
	s, _ = Parse([]byte(minimal("  - one_order_per_cart\n")))
	if got := s.SelectedChecks(); !slices.Equal(got, []string{"no_oversell", "one_order_per_cart"}) {
		t.Errorf("selected = %q", got)
	}
}

func TestSeededStockMatchesSeedScript(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "targets", "medusa", "app", "src", "scripts", "seed.ts"))
	if err != nil {
		t.Fatal(err)
	}
	src := strings.Join(strings.Fields(string(b)), " ")
	for sku, qty := range SeededStock {
		want := fmt.Sprintf(`sku: "%s", title: "%s", location_levels: [{ location_id: location.id, stocked_quantity: %d }]`, sku, sku[:1]+strings.ToLower(sku[1:]), qty)
		if !strings.Contains(src, want) {
			t.Errorf("seed.ts does not stock %s with %d", sku, qty)
		}
	}
}

func countScenario(extraCarts, race string) string {
	return `schema: oversold.scenario/v1
name: counted
description: Counted carts.
target:
  engine: medusa
  version: 2.21.2
  servers: 2
carts:
  buyer:
    server: 2
    count: 3
    lines:
      - sku: OVS-PIECE
        qty: 1
  solo:
    server: 1
    lines:
      - sku: OVS-SOLO
        qty: 1
` + extraCarts + `race:
` + race + `timing:
  release: together
runs: 1
seed: 1
`
}

const countRace = `  - id: buy
    server: 1
    do: complete
    cart: buyer
  - id: last
    server: 1
    do: complete
    cart: solo
`

func TestCountExpansion(t *testing.T) {
	s, problems := Parse([]byte(countScenario("", countRace)))
	if len(problems) != 0 {
		t.Fatalf("problems: %q", problems)
	}
	carts, actions := s.Plan()
	var names []string
	for _, c := range carts {
		names = append(names, c.Name)
		if c.Base == "buyer" && (c.Server != 2 || len(c.Lines) != 1 || c.Lines[0].SKU != "OVS-PIECE") {
			t.Errorf("copy %s = %+v", c.Name, c)
		}
	}
	if !slices.Equal(names, []string{"buyer-1", "buyer-2", "buyer-3", "solo"}) {
		t.Errorf("carts = %q", names)
	}
	var got []string
	for _, a := range actions {
		got = append(got, a.ID+"->"+a.Cart)
		if a.Server != 1 || a.Do != "complete" {
			t.Errorf("action %+v", a)
		}
	}
	if !slices.Equal(got, []string{"buy-1->buyer-1", "buy-2->buyer-2", "buy-3->buyer-3", "last->solo"}) {
		t.Errorf("actions = %q", got)
	}
}

func TestCountOneKeepsName(t *testing.T) {
	src := strings.Replace(countScenario("", countRace), "count: 3", "count: 1", 1)
	s, problems := Parse([]byte(src))
	if len(problems) != 0 {
		t.Fatalf("problems: %q", problems)
	}
	carts, actions := s.Plan()
	if carts[0].Name != "buyer" || actions[0].ID != "buy" || actions[0].Cart != "buyer" {
		t.Errorf("carts %+v actions %+v", carts, actions)
	}
}

func TestCountValidation(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"zero", strings.Replace(countScenario("", countRace), "count: 3", "count: 0", 1), "carts.buyer.count: got 0, want 1 to 100"},
		{"too many", strings.Replace(countScenario("", countRace), "count: 3", "count: 101", 1), "carts.buyer.count: got 101, want 1 to 100"},
		{"not a number", strings.Replace(countScenario("", countRace), "count: 3", "count: lots", 1), "cannot unmarshal"},
		{"cart name collision", countScenario("  buyer-2:\n    server: 1\n    lines:\n      - sku: OVS-PIECE\n        qty: 1\n", countRace), "copy buyer-2 has the same name as a copy of cart"},
		{"action id collision", countScenario("", countRace+"  - id: buy-2\n    server: 1\n    do: complete\n    cart: solo\n"), "action buy-2 from"},
		{"too many actions", strings.Replace(countScenario("", countRace+"  - id: again\n    server: 2\n    do: complete\n    cart: buyer\n  - id: third\n    server: 1\n    do: complete\n    cart: buyer\n"), "count: 3", "count: 100", 1), "race: 301 actions after expanding cart counts, at most 200"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, problems := Parse([]byte(tt.src))
			if joined := strings.Join(problems, "\n"); !strings.Contains(joined, tt.want) {
				t.Errorf("want %q in:\n%s", tt.want, joined)
			}
		})
	}
}

func TestResolveVersion(t *testing.T) {
	s, _ := Parse([]byte(minimal("")))
	if v, err := s.ResolveVersion(""); err != nil || v != "2.21.2" {
		t.Errorf("no override = %q, %v", v, err)
	}
	if v, err := s.ResolveVersion("2.19.0"); err != nil || v != "2.19.0" {
		t.Errorf("override = %q, %v", v, err)
	}
	if _, err := s.ResolveVersion("2.18.0"); err == nil {
		t.Error("unbuilt version should be refused")
	}
}
