package expect

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"

	"go.yaml.in/yaml/v3"

	"github.com/akallam04/oversold/internal/checks"
	"github.com/akallam04/oversold/internal/history"
	"github.com/akallam04/oversold/internal/report"
	"github.com/akallam04/oversold/internal/scenario"
	"github.com/akallam04/oversold/internal/targets"
)

const (
	Schema        = "oversold.expect/v1"
	SummarySchema = "oversold.expect-summary/v1"

	PassEveryRun    = "pass_every_run"
	FailEveryRun    = "fail_every_run"
	FailAtLeastOnce = "fail_at_least_once"

	MinRuns    = 1
	MaxRuns    = 1000
	MaxNoteLen = 80

	ExitMet          = 0
	ExitUnmet        = 1
	ExitUsage        = 2
	ExitInconclusive = 3
)

var Kinds = []string{PassEveryRun, FailEveryRun, FailAtLeastOnce}

type Expectations struct {
	Schema      string `yaml:"schema"`
	Description string `yaml:"description"`
	Runs        int    `yaml:"runs"`
	Cases       []Case `yaml:"cases"`
}

type Case struct {
	Scenario string            `yaml:"scenario"`
	Note     string            `yaml:"note"`
	Expect   map[string]string `yaml:"expect"`

	File *scenario.File `yaml:"-"`
}

type File struct {
	Path         string
	Bytes        []byte
	SHA256       string
	Expectations *Expectations
}

func Load(path string) (*File, []string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, []string{err.Error()}
	}
	root, err := targets.FindRoot()
	if err != nil {
		return nil, []string{err.Error()}
	}
	e, problems := Parse(b, root)
	sum := sha256.Sum256(b)
	return &File{Path: path, Bytes: b, SHA256: hex.EncodeToString(sum[:]), Expectations: e}, problems
}

func Parse(b []byte, root string) (*Expectations, []string) {
	var problems []string
	e := &Expectations{}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	err := dec.Decode(e)
	switch {
	case errors.Is(err, io.EOF):
		return e, []string{"file is empty"}
	case err != nil:
		var te *yaml.TypeError
		if !errors.As(err, &te) {
			return e, []string{err.Error()}
		}
		problems = append(problems, te.Errors...)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		problems = append(problems, "file must hold exactly one YAML document")
	}
	return e, append(problems, e.validate(root)...)
}

func (e *Expectations) validate(root string) []string {
	var p []string
	add := func(format string, args ...any) { p = append(p, fmt.Sprintf(format, args...)) }

	switch e.Schema {
	case Schema:
	case "":
		add("schema: required, must be %q", Schema)
	default:
		add("schema: got %q, want %q", e.Schema, Schema)
	}
	if e.Runs < MinRuns || e.Runs > MaxRuns {
		add("runs: got %d, want %d to %d", e.Runs, MinRuns, MaxRuns)
	}
	if len(e.Cases) == 0 {
		add("cases: at least one case")
	}

	seen := map[string]int{}
	for i := range e.Cases {
		c := &e.Cases[i]
		at := fmt.Sprintf("cases[%d]", i)
		if strings.ContainsAny(c.Note, "\n\r") || len(c.Note) > MaxNoteLen {
			add("%s.note: one line of at most %d characters", at, MaxNoteLen)
		}
		if c.Scenario == "" {
			add("%s.scenario: required, a scenario file relative to the repository root", at)
			continue
		}
		if filepath.IsAbs(c.Scenario) || filepath.Clean(c.Scenario) != c.Scenario || strings.HasPrefix(c.Scenario, "..") || filepath.Ext(c.Scenario) != ".yaml" {
			add("%s.scenario: %q must be a clean .yaml path relative to the repository root", at, c.Scenario)
			continue
		}
		if j, ok := seen[c.Scenario]; ok {
			add("%s.scenario: %s is already cases[%d]", at, c.Scenario, j)
			continue
		}
		seen[c.Scenario] = i

		f, problems := scenario.Load(filepath.Join(root, c.Scenario))
		if f == nil {
			add("%s.scenario: %s", at, strings.Join(problems, "; "))
			continue
		}
		if len(problems) > 0 {
			for _, sp := range problems {
				add("%s.scenario: %s is not a valid scenario: %s", at, c.Scenario, sp)
			}
			continue
		}
		f.Path = c.Scenario
		c.File = f

		selected := f.Scenario.SelectedChecks()
		if len(c.Expect) == 0 {
			add("%s.expect: required, one expectation for each check %s runs: %s", at, f.Scenario.Name, strings.Join(selected, ", "))
			continue
		}
		for _, name := range sortedKeys(c.Expect) {
			kind := c.Expect[name]
			if _, ok := checks.Lookup(name); !ok {
				add("%s.expect.%s: unknown check", at, name)
				continue
			}
			if !slices.Contains(selected, name) {
				add("%s.expect.%s: %s does not run this check", at, name, f.Scenario.Name)
				continue
			}
			if !slices.Contains(Kinds, kind) {
				add("%s.expect.%s: got %q, want one of %s", at, name, kind, strings.Join(Kinds, ", "))
			}
		}
		for _, name := range selected {
			if _, ok := c.Expect[name]; !ok {
				add("%s.expect.%s: missing, %s runs this check", at, name, f.Scenario.Name)
			}
		}
	}
	return p
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

type CheckResult struct {
	Check        string `json:"check"`
	Expected     string `json:"expected"`
	Runs         int    `json:"runs"`
	Fail         int    `json:"fail"`
	Pass         int    `json:"pass"`
	Inconclusive int    `json:"inconclusive"`
	Met          bool   `json:"met"`
}

func Met(kind string, runs, fail, pass int) bool {
	switch kind {
	case PassEveryRun:
		return runs > 0 && pass == runs
	case FailEveryRun:
		return runs > 0 && fail == runs
	case FailAtLeastOnce:
		return fail > 0
	}
	return false
}

func expectedText(kind string) string {
	switch kind {
	case PassEveryRun:
		return "no run"
	case FailEveryRun:
		return "every run"
	case FailAtLeastOnce:
		return "at least one run"
	}
	return kind
}

type CaseResult struct {
	Scenario         string        `json:"scenario"`
	File             string        `json:"file"`
	SHA256           string        `json:"sha256"`
	Note             string        `json:"note,omitempty"`
	Folder           string        `json:"folder,omitempty"`
	RunsPlanned      int           `json:"runs_planned"`
	RunsRecorded     int           `json:"runs_recorded"`
	HarnessErrors    int           `json:"harness_errors"`
	InconclusiveRuns int           `json:"inconclusive_runs"`
	ExitCode         int           `json:"exit_code"`
	Error            string        `json:"error,omitempty"`
	Checks           []CheckResult `json:"checks"`
}

func CaseFrom(c Case, runs int, rep *report.Report, folder string, err error, masker *history.Masker) CaseResult {
	r := CaseResult{Scenario: c.File.Scenario.Name, File: c.Scenario, SHA256: c.File.SHA256, Note: c.Note, Folder: folder, RunsPlanned: runs}
	if err != nil {
		r.Error = masker.String(err.Error())
		r.ExitCode = ExitInconclusive
	}
	counts := map[string]report.CheckCount{}
	if rep != nil {
		r.RunsRecorded = rep.RunsRecorded
		r.ExitCode = rep.ExitCode
		for _, run := range rep.Runs {
			if run.HarnessError != "" {
				r.HarnessErrors++
			}
			if slices.ContainsFunc(run.Verdicts, func(v report.RunVerdict) bool { return v.Result == history.Inconclusive }) {
				r.InconclusiveRuns++
			}
		}
		for _, cc := range rep.Checks {
			counts[cc.Check] = cc
		}
	}
	for _, name := range c.File.Scenario.SelectedChecks() {
		cc := counts[name]
		kind := c.Expect[name]
		r.Checks = append(r.Checks, CheckResult{
			Check:        name,
			Expected:     kind,
			Runs:         runs,
			Fail:         cc.Fail,
			Pass:         cc.Pass,
			Inconclusive: cc.Inconclusive,
			Met:          Met(kind, runs, cc.Fail, cc.Pass),
		})
	}
	return r
}

type Prepare struct {
	ImageBuilt      bool    `json:"image_built"`
	ImageSeconds    float64 `json:"image_seconds"`
	TemplateCreated bool    `json:"template_created"`
	TemplateSeconds float64 `json:"template_seconds"`
	Error           string  `json:"error,omitempty"`
}

type FileRef struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}

type Summary struct {
	Schema        string            `json:"schema"`
	Expectations  FileRef           `json:"expectations"`
	MedusaVersion string            `json:"medusa_version"`
	ImageIDs      []string          `json:"image_ids"`
	Oversold      history.Build     `json:"oversold"`
	Runs          int               `json:"runs"`
	Started       history.Timestamp `json:"started"`
	Finished      history.Timestamp `json:"finished"`
	Seconds       float64           `json:"seconds"`
	Prepare       Prepare           `json:"prepare"`
	Cases         []CaseResult      `json:"cases"`
	Unmet         []string          `json:"unmet"`
	Problems      []string          `json:"problems"`
	ExitCode      int               `json:"exit_code"`
}

func (s *Summary) AddImage(id string) {
	if id != "" && !slices.Contains(s.ImageIDs, id) {
		s.ImageIDs = append(s.ImageIDs, id)
	}
}

func (s *Summary) Finish() {
	s.Unmet = []string{}
	s.Problems = []string{}
	if s.Prepare.Error != "" {
		s.Problems = append(s.Problems, "prepare: "+s.Prepare.Error)
	}
	for _, c := range s.Cases {
		if c.Error != "" {
			s.Problems = append(s.Problems, fmt.Sprintf("%s: %s", c.Scenario, c.Error))
		}
		if c.HarnessErrors > 0 {
			s.Problems = append(s.Problems, fmt.Sprintf("%s: %d of %d runs had a harness error", c.Scenario, c.HarnessErrors, c.RunsPlanned))
		}
		if c.InconclusiveRuns > 0 {
			s.Problems = append(s.Problems, fmt.Sprintf("%s: %d of %d runs were inconclusive", c.Scenario, c.InconclusiveRuns, c.RunsPlanned))
		}
		if c.Error == "" && c.RunsRecorded < c.RunsPlanned {
			s.Problems = append(s.Problems, fmt.Sprintf("%s: only %d of %d runs recorded", c.Scenario, c.RunsRecorded, c.RunsPlanned))
		}
		for _, r := range c.Checks {
			if !r.Met {
				s.Unmet = append(s.Unmet, fmt.Sprintf("%s on %s: %s failed in %d of %d runs, expected %s. Medusa may have changed this; run the full scenario set before saying so.",
					c.Scenario, s.MedusaVersion, r.Check, r.Fail, r.Runs, expectedText(r.Expected)))
			}
		}
	}
	s.ExitCode = ExitCode(len(s.Problems) > 0, len(s.Unmet) > 0)
}

func ExitCode(troubled, unmet bool) int {
	switch {
	case troubled:
		return ExitInconclusive
	case unmet:
		return ExitUnmet
	}
	return ExitMet
}

func (s *Summary) Write(dir string) (string, string, error) {
	jsonPath := filepath.Join(dir, "summary.json")
	mdPath := filepath.Join(dir, "summary.md")
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", "", err
	}
	if err := os.WriteFile(jsonPath, append(b, '\n'), 0o644); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(mdPath, []byte(s.Markdown()), 0o644); err != nil {
		return "", "", err
	}
	return jsonPath, mdPath, nil
}

func cell(r CheckResult) string {
	s := fmt.Sprintf("fail %d of %d", r.Fail, r.Runs)
	if r.Inconclusive > 0 {
		s += fmt.Sprintf(", inconclusive %d", r.Inconclusive)
	}
	return s
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func took(done bool, seconds float64) string {
	if !done {
		return "already present"
	}
	return fmt.Sprintf("built in %.1fs", seconds)
}

func (s *Summary) outcome() string {
	switch s.ExitCode {
	case ExitMet:
		return "every expectation met"
	case ExitUnmet:
		return fmt.Sprintf("%d expectations not met", len(s.Unmet))
	}
	return "a run was inconclusive or the harness failed, so the counts cannot be trusted"
}

func (s *Summary) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# oversold expect on Medusa %s\n\n", s.MedusaVersion)
	fmt.Fprintf(&b, "- Expectations file: `%s`, sha256 `%s`\n", s.Expectations.File, s.Expectations.SHA256)
	fmt.Fprintf(&b, "- Medusa %s, image `%s`\n", s.MedusaVersion, strings.Join(s.ImageIDs, "`, `"))
	state := "clean"
	if s.Oversold.Dirty {
		state = "with uncommitted changes"
	}
	fmt.Fprintf(&b, "- Oversold commit: `%s` (%s)\n", s.Oversold.Commit, state)
	fmt.Fprintf(&b, "- Runs per case: %d\n", s.Runs)
	fmt.Fprintf(&b, "- Medusa image: %s. Template database: %s\n", took(s.Prepare.ImageBuilt, s.Prepare.ImageSeconds), took(s.Prepare.TemplateCreated, s.Prepare.TemplateSeconds))
	fmt.Fprintf(&b, "- Started %s, took %.1fs\n", s.Started.Format("2006-01-02T15:04:05Z"), s.Seconds)
	fmt.Fprintf(&b, "- Exit code: %d, %s\n\n", s.ExitCode, s.outcome())

	b.WriteString("| Case | Note | Check | Result | Expected | Met |\n|---|---|---|---|---|---|\n")
	for _, c := range s.Cases {
		for _, r := range c.Checks {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", c.Scenario, escape(c.Note), r.Check, cell(r), r.Expected, yesNo(r.Met))
		}
	}
	if len(s.Unmet) > 0 {
		b.WriteString("\n## Not met\n\n")
		for _, u := range s.Unmet {
			fmt.Fprintf(&b, "- %s\n", u)
		}
	}
	if len(s.Problems) > 0 {
		b.WriteString("\n## Problems\n\n")
		for _, p := range s.Problems {
			fmt.Fprintf(&b, "- %s\n", escape(p))
		}
	}
	b.WriteString("\n## Run sets\n\n")
	for _, c := range s.Cases {
		if c.Folder == "" {
			fmt.Fprintf(&b, "- %s: no run set\n", c.Scenario)
			continue
		}
		fmt.Fprintf(&b, "- [%s](%s/report.md), exit code %d\n", c.Scenario, c.Folder, c.ExitCode)
	}
	return b.String()
}

func escape(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

func (s *Summary) Table(w io.Writer) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "case\tnote\tcheck\tresult\texpected\tmet")
	for _, c := range s.Cases {
		for _, r := range c.Checks {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", c.Scenario, c.Note, r.Check, cell(r), r.Expected, yesNo(r.Met))
		}
	}
	tw.Flush()
}
