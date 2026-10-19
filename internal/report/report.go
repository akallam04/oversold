package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/akallam04/oversold/internal/checks"
	"github.com/akallam04/oversold/internal/history"
	"github.com/akallam04/oversold/internal/scenario"
)

const Schema = "oversold.report/v1"

const (
	ExitPass         = 0
	ExitFail         = 1
	ExitUsage        = 2
	ExitInconclusive = 3

	MaxHarnessErrorsInARow = 3
)

type ScenarioRef struct {
	Name   string `json:"name"`
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}

type Server struct {
	Service       string `json:"service"`
	MedusaVersion string `json:"medusa_version"`
	ImageID       string `json:"image_id"`
}

type CheckCount struct {
	Check        string `json:"check"`
	Version      int    `json:"version"`
	Runs         int    `json:"runs"`
	Fail         int    `json:"fail"`
	Pass         int    `json:"pass"`
	Inconclusive int    `json:"inconclusive"`
}

type RunVerdict struct {
	Check   string         `json:"check"`
	Version int            `json:"version"`
	Result  history.Result `json:"result"`
	Reason  string         `json:"reason"`
}

type Run struct {
	Index        int            `json:"index"`
	File         string         `json:"file"`
	Result       history.Result `json:"result"`
	HarnessError string         `json:"harness_error,omitempty"`
	Verdicts     []RunVerdict   `json:"verdicts"`
}

type Report struct {
	Schema          string            `json:"schema"`
	Scenario        ScenarioRef       `json:"scenario"`
	Oversold        []history.Build   `json:"oversold"`
	CheckedBy       history.Build     `json:"checked_by"`
	Servers         []Server          `json:"servers"`
	PostgresVersion string            `json:"postgres_version"`
	RunsPlanned     int               `json:"runs_planned"`
	RunsRecorded    int               `json:"runs_recorded"`
	StoppedEarly    string            `json:"stopped_early,omitempty"`
	Checks          []CheckCount      `json:"checks"`
	Runs            []Run             `json:"runs"`
	ExitCode        int               `json:"exit_code"`
	Generated       history.Timestamp `json:"generated"`
}

type Entry struct {
	File    string
	History *history.History
}

func Build(file *scenario.File, entries []Entry, planned int, stoppedEarly string) *Report {
	sc := file.Scenario
	r := &Report{
		Schema:       Schema,
		Scenario:     ScenarioRef{Name: sc.Name, File: filepath.Base(file.Path), SHA256: file.SHA256},
		CheckedBy:    history.CurrentBuild(),
		RunsPlanned:  planned,
		RunsRecorded: len(entries),
		StoppedEarly: stoppedEarly,
		Generated:    history.Timestamp{Time: time.Now().UTC()},
	}

	counts := map[string]*CheckCount{}
	for _, name := range sc.SelectedChecks() {
		c, _ := checks.Lookup(name)
		counts[name] = &CheckCount{Check: name, Version: c.Version}
	}

	builds := map[string]bool{}
	servers := map[string]bool{}
	for _, e := range entries {
		h := e.History
		if key := fmt.Sprintf("%s/%v", h.Oversold.Commit, h.Oversold.Dirty); !builds[key] {
			builds[key] = true
			r.Oversold = append(r.Oversold, h.Oversold)
		}
		for _, s := range h.Target.Servers {
			key := s.Service + "/" + s.MedusaVersion + "/" + s.ImageID
			if !servers[key] {
				servers[key] = true
				r.Servers = append(r.Servers, Server{Service: s.Service, MedusaVersion: s.MedusaVersion, ImageID: s.ImageID})
			}
		}
		if r.PostgresVersion == "" {
			r.PostgresVersion = h.Target.PostgresVersion
		}

		run := Run{Index: h.Run.Index, File: e.File, Result: checks.Summarize(h.Verdicts), HarnessError: h.HarnessError}
		for _, v := range h.Verdicts {
			run.Verdicts = append(run.Verdicts, RunVerdict{Check: v.Check, Version: v.Version, Result: v.Result, Reason: v.Reason})
			cc := counts[v.Check]
			if cc == nil {
				continue
			}
			cc.Runs++
			switch v.Result {
			case history.Pass:
				cc.Pass++
			case history.Fail:
				cc.Fail++
			default:
				cc.Inconclusive++
			}
		}
		r.Runs = append(r.Runs, run)
	}
	sort.Slice(r.Runs, func(i, j int) bool { return r.Runs[i].Index < r.Runs[j].Index })
	sort.Slice(r.Servers, func(i, j int) bool { return r.Servers[i].Service < r.Servers[j].Service })
	for _, name := range sc.SelectedChecks() {
		r.Checks = append(r.Checks, *counts[name])
	}
	r.ExitCode = exitCode(r)
	return r
}

func exitCode(r *Report) int {
	inconclusive := len(r.Runs) == 0
	for _, c := range r.Checks {
		if c.Fail > 0 {
			return ExitFail
		}
		if c.Inconclusive > 0 || c.Runs < len(r.Runs) {
			inconclusive = true
		}
	}
	if inconclusive {
		return ExitInconclusive
	}
	return ExitPass
}

func (r *Report) Write(dir string) (string, string, error) {
	jsonPath := filepath.Join(dir, "report.json")
	mdPath := filepath.Join(dir, "report.md")
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", "", err
	}
	if err := os.WriteFile(jsonPath, append(b, '\n'), 0o644); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(mdPath, []byte(r.Markdown()), 0o644); err != nil {
		return "", "", err
	}
	return jsonPath, mdPath, nil
}

func (r *Report) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", r.Scenario.Name)
	fmt.Fprintf(&b, "- Scenario file: `%s`, sha256 `%s`\n", r.Scenario.File, r.Scenario.SHA256)
	for _, o := range r.Oversold {
		state := "clean"
		if o.Dirty {
			state = "with uncommitted changes"
		}
		fmt.Fprintf(&b, "- Oversold commit that ran it: `%s` (%s)\n", o.Commit, state)
	}
	checkedState := "clean"
	if r.CheckedBy.Dirty {
		checkedState = "with uncommitted changes"
	}
	fmt.Fprintf(&b, "- Verdicts computed by Oversold commit: `%s` (%s)\n", r.CheckedBy.Commit, checkedState)
	for _, s := range r.Servers {
		fmt.Fprintf(&b, "- Server `%s`: Medusa %s, image `%s`\n", s.Service, s.MedusaVersion, s.ImageID)
	}
	if r.PostgresVersion != "" {
		fmt.Fprintf(&b, "- Postgres %s\n", r.PostgresVersion)
	}
	fmt.Fprintf(&b, "- Runs: %d of %d recorded\n", r.RunsRecorded, r.RunsPlanned)
	if r.StoppedEarly != "" {
		fmt.Fprintf(&b, "- Stopped early: %s\n", r.StoppedEarly)
	}
	fmt.Fprintf(&b, "- Exit code: %d\n\n", r.ExitCode)

	b.WriteString("## Checks\n\n| Check | Version | Fail | Pass | Inconclusive |\n|---|---|---|---|---|\n")
	for _, c := range r.Checks {
		n := len(r.Runs)
		fmt.Fprintf(&b, "| %s | %d | %d of %d | %d of %d | %d of %d |\n", c.Check, c.Version, c.Fail, n, c.Pass, n, c.Inconclusive, n)
	}

	b.WriteString("\n## Runs\n\n| Run |")
	for _, c := range r.Checks {
		fmt.Fprintf(&b, " %s |", c.Check)
	}
	b.WriteString(" History |\n|---|")
	for range r.Checks {
		b.WriteString("---|")
	}
	b.WriteString("---|\n")
	for _, run := range r.Runs {
		fmt.Fprintf(&b, "| %d |", run.Index)
		for _, c := range r.Checks {
			fmt.Fprintf(&b, " %s |", resultOf(run, c.Check))
		}
		fmt.Fprintf(&b, " [%s](%s) |\n", run.File, run.File)
	}

	var notes []string
	for _, run := range r.Runs {
		for _, v := range run.Verdicts {
			if v.Result != history.Pass {
				notes = append(notes, fmt.Sprintf("- Run %d, %s %s: %s", run.Index, v.Check, v.Result, escape(v.Reason)))
			}
		}
	}
	if len(notes) > 0 {
		b.WriteString("\n## Failed and inconclusive verdicts\n\n")
		b.WriteString(strings.Join(notes, "\n"))
		b.WriteString("\n")
	}
	return b.String()
}

func resultOf(run Run, check string) string {
	for _, v := range run.Verdicts {
		if v.Check == check {
			return string(v.Result)
		}
	}
	return "missing"
}

func escape(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

func (r *Report) CountLine(c CheckCount) string {
	n := len(r.Runs)
	return fmt.Sprintf("%s v%d: fail %d of %d, pass %d of %d, inconclusive %d of %d", c.Check, c.Version, c.Fail, n, c.Pass, n, c.Inconclusive, n)
}

func StoppedEarly(entries []Entry, planned int) string {
	if len(entries) >= planned {
		return ""
	}
	inARow := 0
	for i := len(entries) - 1; i >= 0 && entries[i].History.HarnessError != ""; i-- {
		inARow++
	}
	if inARow >= MaxHarnessErrorsInARow {
		return fmt.Sprintf("stopped after %d harness errors in a row, %d of %d runs recorded", inARow, len(entries), planned)
	}
	return fmt.Sprintf("only %d of %d runs recorded", len(entries), planned)
}

func LoadFolder(dir string) (*scenario.File, []Entry, []string, error) {
	yamls, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, nil, nil, err
	}
	if len(yamls) != 1 {
		return nil, nil, nil, fmt.Errorf("%s: want exactly one scenario .yaml file, found %d", dir, len(yamls))
	}
	file, problems := scenario.Load(yamls[0])
	if len(problems) > 0 {
		return file, nil, problems, nil
	}
	paths, err := filepath.Glob(filepath.Join(dir, "run-*.json"))
	if err != nil {
		return nil, nil, nil, err
	}
	if len(paths) == 0 {
		return nil, nil, nil, fmt.Errorf("%s: no run-*.json histories", dir)
	}
	sort.Strings(paths)
	var entries []Entry
	for _, p := range paths {
		h, err := history.Read(p)
		if err != nil {
			return nil, nil, nil, err
		}
		if h.Scenario.SHA256 != file.SHA256 {
			return nil, nil, nil, fmt.Errorf("%s: scenario sha256 %s does not match %s (%s)", p, h.Scenario.SHA256, filepath.Base(file.Path), file.SHA256)
		}
		entries = append(entries, Entry{File: filepath.Base(p), History: h})
	}
	return file, entries, nil, nil
}
