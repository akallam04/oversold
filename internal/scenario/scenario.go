package scenario

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/akallam04/oversold/internal/targets"
)

const Schema = "oversold.scenario/v1"

const (
	ReleaseTogether   = "together"
	ReleaseSequential = "sequential"

	DefaultRequestTimeoutMS = 30000
	MinRequestTimeoutMS     = 100
	MaxRequestTimeoutMS     = 120000
	MaxJitterMS             = 10000
	MinRuns                 = 1
	MaxRuns                 = 1000
	MaxLineQty              = 1000
	MaxCartCount            = 100
	MaxRaceActions          = 200
	MaxStock                = 1000000
)

type Variant struct {
	SKU          string
	InventorySKU string
	Units        int
}

var Catalog = []Variant{
	{SKU: "OVS-PIECE", InventorySKU: "WIDGET", Units: 1},
	{SKU: "OVS-TWOPACK", InventorySKU: "WIDGET", Units: 2},
	{SKU: "OVS-SOLO", InventorySKU: "SOLO", Units: 1},
}

var InventorySKUs = []string{"WIDGET", "SOLO"}

var SeededStock = map[string]int{"WIDGET": 10, "SOLO": 1}

var Checks = []string{
	"no_oversell",
	"reserved_matches_reservations",
	"one_order_per_cart",
	"no_orphaned_orders",
}

var Actions = []string{"complete"}

func VariantSKUs() []string {
	out := make([]string, len(Catalog))
	for i, v := range Catalog {
		out[i] = v.SKU
	}
	return out
}

func LookupVariant(sku string) (Variant, bool) {
	for _, v := range Catalog {
		if v.SKU == sku {
			return v, true
		}
	}
	return Variant{}, false
}

type Scenario struct {
	Schema      string          `yaml:"schema"`
	Name        string          `yaml:"name"`
	Description string          `yaml:"description"`
	Target      Target          `yaml:"target"`
	Stock       map[string]int  `yaml:"stock"`
	Carts       map[string]Cart `yaml:"carts"`
	Race        []Action        `yaml:"race"`
	Timing      Timing          `yaml:"timing"`
	Faults      []any           `yaml:"faults"`
	Runs        int             `yaml:"runs"`
	Seed        *int64          `yaml:"seed"`
	Checks      *[]string       `yaml:"checks"`

	CartOrder   []string `yaml:"-"`
	checksIsNil bool
}

type Target struct {
	Engine  string `yaml:"engine"`
	Version string `yaml:"version"`
	Servers int    `yaml:"servers"`
}

type Cart struct {
	Server int    `yaml:"server"`
	Count  *int   `yaml:"count"`
	Lines  []Line `yaml:"lines"`
}

func (c Cart) Copies() int {
	if c.Count == nil {
		return 1
	}
	return *c.Count
}

type PlannedCart struct {
	Name   string
	Base   string
	Server int
	Lines  []Line
}

type PlannedAction struct {
	ID     string
	Base   string
	Server int
	Do     string
	Cart   string
}

func copyNames(base string, n int) []string {
	if n <= 1 {
		return []string{base}
	}
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("%s-%d", base, i+1)
	}
	return out
}

func (s *Scenario) Plan() ([]PlannedCart, []PlannedAction) {
	var carts []PlannedCart
	for _, name := range s.CartOrder {
		c := s.Carts[name]
		for _, n := range copyNames(name, c.Copies()) {
			carts = append(carts, PlannedCart{Name: n, Base: name, Server: c.Server, Lines: c.Lines})
		}
	}
	var actions []PlannedAction
	for _, a := range s.Race {
		c, ok := s.Carts[a.Cart]
		if !ok {
			continue
		}
		n := c.Copies()
		ids := copyNames(a.ID, n)
		names := copyNames(a.Cart, n)
		for i := range ids {
			actions = append(actions, PlannedAction{ID: ids[i], Base: a.ID, Server: a.Server, Do: a.Do, Cart: names[i]})
		}
	}
	return carts, actions
}

type Line struct {
	SKU string `yaml:"sku"`
	Qty int    `yaml:"qty"`
}

type Action struct {
	ID     string `yaml:"id"`
	Server int    `yaml:"server"`
	Do     string `yaml:"do"`
	Cart   string `yaml:"cart"`
}

type Timing struct {
	Release          string `yaml:"release"`
	JitterMS         []int  `yaml:"jitter_ms"`
	RequestTimeoutMS *int   `yaml:"request_timeout_ms"`
}

func (s *Scenario) ResolveVersion(override string) (string, error) {
	if override == "" {
		return s.Target.Version, nil
	}
	versions, err := Versions(s.Target.Engine)
	if err != nil {
		return "", fmt.Errorf("--version %s: %w", override, err)
	}
	if !slices.Contains(versions, override) {
		return "", fmt.Errorf("--version %s: %s versions Oversold builds are %s", override, s.Target.Engine, strings.Join(versions, ", "))
	}
	return override, nil
}

func Versions(engine string) ([]string, error) {
	root, err := targets.FindRoot()
	if err != nil {
		return nil, err
	}
	return targets.Versions(root, engine)
}

func (s *Scenario) SelectedChecks() []string {
	if s.Checks == nil {
		out := make([]string, len(Checks))
		copy(out, Checks)
		return out
	}
	out := make([]string, len(*s.Checks))
	copy(out, *s.Checks)
	return out
}

func (s *Scenario) RequestTimeout() time.Duration {
	ms := DefaultRequestTimeoutMS
	if s.Timing.RequestTimeoutMS != nil {
		ms = *s.Timing.RequestTimeoutMS
	}
	return time.Duration(ms) * time.Millisecond
}

func (s *Scenario) JitterRange() (min, max time.Duration) {
	if len(s.Timing.JitterMS) != 2 {
		return 0, 0
	}
	return time.Duration(s.Timing.JitterMS[0]) * time.Millisecond, time.Duration(s.Timing.JitterMS[1]) * time.Millisecond
}

type File struct {
	Path     string
	Bytes    []byte
	SHA256   string
	Scenario *Scenario
}

func Load(path string) (*File, []string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, []string{err.Error()}
	}
	s, problems := Parse(b)
	sum := sha256.Sum256(b)
	return &File{Path: path, Bytes: b, SHA256: hex.EncodeToString(sum[:]), Scenario: s}, problems
}

func Parse(b []byte) (*Scenario, []string) {
	var problems []string
	s := &Scenario{}

	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	err := dec.Decode(s)
	switch {
	case errors.Is(err, io.EOF):
		return s, []string{"file is empty"}
	case err != nil:
		var te *yaml.TypeError
		if !errors.As(err, &te) {
			return s, []string{err.Error()}
		}
		problems = append(problems, te.Errors...)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		problems = append(problems, "file must hold exactly one YAML document")
	}

	s.CartOrder = cartOrder(b, s)
	s.checksIsNil = nullField(b, "checks")
	problems = append(problems, s.validate()...)
	return s, problems
}

func cartOrder(b []byte, s *Scenario) []string {
	var doc yaml.Node
	var order []string
	if yaml.Unmarshal(b, &doc) == nil && len(doc.Content) == 1 && doc.Content[0].Kind == yaml.MappingNode {
		root := doc.Content[0]
		for i := 0; i+1 < len(root.Content); i += 2 {
			if root.Content[i].Value != "carts" || root.Content[i+1].Kind != yaml.MappingNode {
				continue
			}
			carts := root.Content[i+1]
			for j := 0; j+1 < len(carts.Content); j += 2 {
				order = append(order, carts.Content[j].Value)
			}
		}
	}
	if len(order) != len(s.Carts) {
		order = order[:0]
		for name := range s.Carts {
			order = append(order, name)
		}
		sort.Strings(order)
	}
	return order
}

func nullField(b []byte, field string) bool {
	var doc yaml.Node
	if yaml.Unmarshal(b, &doc) != nil || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return false
	}
	root := doc.Content[0]
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == field {
			v := root.Content[i+1]
			return v.Kind == yaml.ScalarNode && v.Tag == "!!null"
		}
	}
	return false
}

var kebab = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func (s *Scenario) validate() []string {
	var p []string
	add := func(format string, args ...any) { p = append(p, fmt.Sprintf(format, args...)) }

	switch s.Schema {
	case Schema:
	case "":
		add("schema: required, must be %q", Schema)
	default:
		add("schema: got %q, want %q", s.Schema, Schema)
	}

	if s.Name == "" {
		add("name: required")
	} else if !kebab.MatchString(s.Name) || len(s.Name) > 64 {
		add("name: %q must be kebab-case (lowercase letters, digits and single dashes, at most 64 characters)", s.Name)
	}
	if strings.TrimSpace(s.Description) == "" {
		add("description: required")
	}

	engines := targets.Engines()
	engineOK := slices.Contains(engines, s.Target.Engine)
	switch {
	case s.Target.Engine == "":
		add("target.engine: required, one of %s", strings.Join(engines, ", "))
	case !engineOK:
		add("target.engine: unknown engine %q, want one of %s", s.Target.Engine, strings.Join(engines, ", "))
	}
	if s.Target.Version == "" {
		add("target.version: required")
	} else if engineOK {
		versions, err := Versions(s.Target.Engine)
		switch {
		case err != nil:
			add("target.version: %v", err)
		case !slices.Contains(versions, s.Target.Version):
			add("target.version: %s %s is not a version Oversold builds, want one of %s", s.Target.Engine, s.Target.Version, strings.Join(versions, ", "))
		}
	}
	if s.Target.Servers < 1 || s.Target.Servers > 2 {
		add("target.servers: got %d, want 1 or 2", s.Target.Servers)
	}
	servers := s.Target.Servers

	for _, sku := range sortedKeys(s.Stock) {
		qty := s.Stock[sku]
		if !slices.Contains(InventorySKUs, sku) {
			add("stock.%s: unknown inventory SKU, want one of %s", sku, strings.Join(InventorySKUs, ", "))
		}
		if qty < 0 || qty > MaxStock {
			add("stock.%s: quantity %d out of range 0 to %d", sku, qty, MaxStock)
		}
	}

	if len(s.Carts) == 0 {
		add("carts: at least one cart is required")
	}
	for _, name := range sortedKeys(s.Carts) {
		c := s.Carts[name]
		if !kebab.MatchString(name) {
			add("carts.%s: cart name must be kebab-case", name)
		}
		if c.Server < 1 || (servers >= 1 && c.Server > servers) {
			add("carts.%s.server: got %d, want 1 to %d", name, c.Server, max(servers, 1))
		}
		if c.Count != nil && (*c.Count < 1 || *c.Count > MaxCartCount) {
			add("carts.%s.count: got %d, want 1 to %d", name, *c.Count, MaxCartCount)
		}
		if len(c.Lines) == 0 {
			add("carts.%s.lines: at least one line is required", name)
		}
		seen := map[string]bool{}
		for i, l := range c.Lines {
			if _, ok := LookupVariant(l.SKU); !ok {
				add("carts.%s.lines[%d].sku: unknown SKU %q, want one of %s", name, i, l.SKU, strings.Join(VariantSKUs(), ", "))
			}
			if seen[l.SKU] {
				add("carts.%s.lines[%d].sku: %s appears twice in the cart", name, i, l.SKU)
			}
			seen[l.SKU] = true
			if l.Qty < 1 || l.Qty > MaxLineQty {
				add("carts.%s.lines[%d].qty: got %d, want 1 to %d", name, i, l.Qty, MaxLineQty)
			}
		}
	}

	if len(s.Race) == 0 {
		add("race: at least one action is required")
	}
	ids := map[string]bool{}
	for i, a := range s.Race {
		switch {
		case a.ID == "":
			add("race[%d].id: required", i)
		case !kebab.MatchString(a.ID):
			add("race[%d].id: %q must be kebab-case", i, a.ID)
		case ids[a.ID]:
			add("race[%d].id: %q is used twice", i, a.ID)
		}
		ids[a.ID] = true
		if a.Server < 1 || (servers >= 1 && a.Server > servers) {
			add("race[%d].server: got %d, want 1 to %d", i, a.Server, max(servers, 1))
		}
		if !slices.Contains(Actions, a.Do) {
			add("race[%d].do: got %q, want one of %s", i, a.Do, strings.Join(Actions, ", "))
		}
		if a.Cart == "" {
			add("race[%d].cart: required", i)
		} else if _, ok := s.Carts[a.Cart]; !ok {
			add("race[%d].cart: no cart named %q", i, a.Cart)
		}
	}

	for _, x := range s.planProblems() {
		add("%s", x)
	}

	switch s.Timing.Release {
	case ReleaseTogether, ReleaseSequential:
	case "":
		add("timing.release: required, %q or %q", ReleaseTogether, ReleaseSequential)
	default:
		add("timing.release: got %q, want %q or %q", s.Timing.Release, ReleaseTogether, ReleaseSequential)
	}
	if j := s.Timing.JitterMS; j != nil {
		if len(j) != 2 {
			add("timing.jitter_ms: want [min, max], got %d values", len(j))
		} else if j[0] < 0 || j[1] < j[0] || j[1] > MaxJitterMS {
			add("timing.jitter_ms: want 0 <= min <= max <= %d, got [%d, %d]", MaxJitterMS, j[0], j[1])
		}
	}
	if t := s.Timing.RequestTimeoutMS; t != nil && (*t < MinRequestTimeoutMS || *t > MaxRequestTimeoutMS) {
		add("timing.request_timeout_ms: got %d, want %d to %d", *t, MinRequestTimeoutMS, MaxRequestTimeoutMS)
	}

	if len(s.Faults) > 0 {
		add("faults: must be empty, no fault kinds are supported yet")
	}

	if s.Runs < MinRuns || s.Runs > MaxRuns {
		add("runs: got %d, want %d to %d", s.Runs, MinRuns, MaxRuns)
	}
	if s.Seed == nil {
		add("seed: required integer")
	}

	if s.checksIsNil {
		add("checks: has no value; list check names or leave the field out to run all four")
	}
	if s.Checks != nil && len(*s.Checks) == 0 {
		add("checks: an empty list is not allowed; list check names or leave the field out to run all four")
	}
	var chosen []string
	if s.Checks != nil {
		chosen = *s.Checks
	}
	seenCheck := map[string]bool{}
	for i, c := range chosen {
		if !slices.Contains(Checks, c) {
			add("checks[%d]: unknown check %q, want one of %s", i, c, strings.Join(Checks, ", "))
		}
		if seenCheck[c] {
			add("checks[%d]: %q is listed twice", i, c)
		}
		seenCheck[c] = true
	}
	return p
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (s *Scenario) planProblems() []string {
	var p []string
	for _, c := range s.Carts {
		if c.Count != nil && (*c.Count < 1 || *c.Count > MaxCartCount) {
			return nil
		}
	}
	carts, actions := s.Plan()
	seen := map[string]string{}
	for _, c := range carts {
		if other, dup := seen[c.Name]; dup && other != c.Base {
			p = append(p, fmt.Sprintf("carts.%s: copy %s has the same name as a copy of cart %s", c.Base, c.Name, other))
		}
		seen[c.Name] = c.Base
	}
	seenAction := map[string]string{}
	for _, a := range actions {
		if other, dup := seenAction[a.ID]; dup && other != a.Base {
			p = append(p, fmt.Sprintf("race: action %s from %s has the same id as an action from %s", a.ID, a.Base, other))
		}
		seenAction[a.ID] = a.Base
	}
	if len(actions) > MaxRaceActions {
		p = append(p, fmt.Sprintf("race: %d actions after expanding cart counts, at most %d", len(actions), MaxRaceActions))
	}
	return p
}
