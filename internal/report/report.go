package report

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/akallam04/oversold/internal/checks"
	"github.com/akallam04/oversold/internal/history"
	"github.com/akallam04/oversold/internal/scenario"
	"github.com/akallam04/oversold/internal/targets"
)

const Schema = "oversold.report/v1"

const (
	ExitPass         = 0
	ExitFail         = 1
	ExitUsage        = 2
	ExitInconclusive = 3
	ExitMismatch     = 1

	MaxHarnessErrorsInARow = 3
)

type ScenarioRef struct {
	Name    string `json:"name"`
	File    string `json:"file"`
	SHA256  string `json:"sha256"`
	Version string `json:"version"`
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
	LockCalls    *int64         `json:"lock_calls,omitempty"`
	Verdicts     []RunVerdict   `json:"verdicts"`
}

type Report struct {
	Schema          string            `json:"schema"`
	Scenario        ScenarioRef       `json:"scenario"`
	Oversold        []history.Build   `json:"oversold"`
	CheckedBy       history.Build     `json:"checked_by"`
	VersionsUsed    []string          `json:"versions_used"`
	Servers         []Server          `json:"servers"`
	PostgresVersion string            `json:"postgres_version"`
	Locking         string            `json:"locking,omitempty"`
	RedisVersions   []string          `json:"redis_versions,omitempty"`
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
		Scenario:     ScenarioRef{Name: sc.Name, File: filepath.Base(file.Path), SHA256: file.SHA256, Version: sc.Target.Version},
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
		if r.Locking == "" {
			r.Locking = h.Target.Locking
		}
		if v := h.Target.RedisVersion; v != "" && !slices.Contains(r.RedisVersions, v) {
			r.RedisVersions = append(r.RedisVersions, v)
		}
		used := h.Target.Version
		if used == "" {
			used = h.Target.ScenarioVersion
		}
		if used != "" && !slices.Contains(r.VersionsUsed, used) {
			r.VersionsUsed = append(r.VersionsUsed, used)
		}

		run := Run{Index: h.Run.Index, File: e.File, Result: checks.Summarize(h.Verdicts), HarnessError: h.HarnessError, LockCalls: h.Race.LockCalls}
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
	fmt.Fprintf(&b, "- Medusa version in the file: %s. Version used: %s\n", r.Scenario.Version, strings.Join(r.VersionsUsed, ", "))
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
	fmt.Fprintf(&b, "- Locking: %s\n", r.LockingMode())
	if len(r.RedisVersions) > 0 {
		fmt.Fprintf(&b, "- Redis %s\n", strings.Join(r.RedisVersions, ", "))
	}
	if lo, hi, n := r.LockCallRange(); n > 0 {
		fmt.Fprintf(&b, "- Scripted Redis lock calls during the race: %d to %d per run, recorded in %d runs\n", lo, hi, n)
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

	lockColumn := r.Locking != ""
	b.WriteString("\n## Runs\n\n| Run |")
	for _, c := range r.Checks {
		fmt.Fprintf(&b, " %s |", c.Check)
	}
	if lockColumn {
		b.WriteString(" Lock calls |")
	}
	b.WriteString(" History |\n|---|")
	for range r.Checks {
		b.WriteString("---|")
	}
	if lockColumn {
		b.WriteString("---|")
	}
	b.WriteString("---|\n")
	for _, run := range r.Runs {
		fmt.Fprintf(&b, "| %d |", run.Index)
		for _, c := range r.Checks {
			fmt.Fprintf(&b, " %s |", resultOf(run, c.Check))
		}
		if lockColumn {
			if run.LockCalls != nil {
				fmt.Fprintf(&b, " %d |", *run.LockCalls)
			} else {
				b.WriteString(" none |")
			}
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

func (r *Report) LockingMode() string {
	if r.Locking == "" {
		return targets.LockingInMemory
	}
	return r.Locking
}

func (r *Report) LockCallRange() (lo, hi int64, n int) {
	for _, run := range r.Runs {
		if run.LockCalls == nil {
			continue
		}
		c := *run.LockCalls
		if n == 0 || c < lo {
			lo = c
		}
		if n == 0 || c > hi {
			hi = c
		}
		n++
	}
	return lo, hi, n
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
	paths, err := historyPaths(dir)
	if err != nil {
		return nil, nil, nil, err
	}
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

func historyPaths(dir string) ([]string, error) {
	plain, err := filepath.Glob(filepath.Join(dir, "run-*.json"))
	if err != nil {
		return nil, err
	}
	zipped, err := filepath.Glob(filepath.Join(dir, "run-*.json.gz"))
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, p := range plain {
		seen[p] = true
	}
	for _, p := range zipped {
		if seen[strings.TrimSuffix(p, ".gz")] {
			return nil, fmt.Errorf("%s: both %s and %s exist", dir, filepath.Base(strings.TrimSuffix(p, ".gz")), filepath.Base(p))
		}
	}
	paths := append(plain, zipped...)
	if len(paths) == 0 {
		return nil, fmt.Errorf("%s: no run-*.json or run-*.json.gz histories", dir)
	}
	sort.Strings(paths)
	return paths, nil
}

func Export(src, dst string) ([]string, error) {
	file, entries, problems, err := LoadFolder(src)
	if err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("%s: invalid scenario: %s", file.Path, strings.Join(problems, "; "))
	}
	b, err := os.ReadFile(filepath.Join(src, "report.json"))
	if err != nil {
		return nil, fmt.Errorf("read report: %w", err)
	}
	var rep Report
	if err := json.Unmarshal(b, &rep); err != nil {
		return nil, fmt.Errorf("read report: %w", err)
	}
	if rep.Scenario.SHA256 != file.SHA256 {
		return nil, fmt.Errorf("report.json is for scenario sha256 %s, folder has %s", rep.Scenario.SHA256, file.SHA256)
	}

	if existing, err := os.ReadDir(dst); err == nil && len(existing) > 0 {
		return nil, fmt.Errorf("%s exists and is not empty", dst)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return nil, err
	}

	var written []string
	scenarioName := filepath.Base(file.Path)
	if err := os.WriteFile(filepath.Join(dst, scenarioName), file.Bytes, 0o644); err != nil {
		return nil, err
	}
	written = append(written, scenarioName)

	renamed := map[string]string{}
	for _, e := range entries {
		raw, err := rawHistory(filepath.Join(src, e.File))
		if err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(e.File, ".gz") + ".gz"
		if err := writeGzip(filepath.Join(dst, name), raw); err != nil {
			return nil, err
		}
		renamed[e.File] = name
		written = append(written, name)
	}
	for i := range rep.Runs {
		if n, ok := renamed[rep.Runs[i].File]; ok {
			rep.Runs[i].File = n
		}
	}
	if _, _, err := rep.Write(dst); err != nil {
		return nil, err
	}
	written = append(written, "report.json", "report.md")
	return written, nil
}

func rawHistory(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil || !strings.HasSuffix(path, ".gz") {
		return b, err
	}
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

func writeGzip(path string, data []byte) error {
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return err
	}
	if _, err := zw.Write(data); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func Verify(dir string) ([]string, error) {
	file, entries, problems, err := LoadFolder(dir)
	if err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("%s: invalid scenario: %s", file.Path, strings.Join(problems, "; "))
	}
	b, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err != nil {
		return nil, fmt.Errorf("read report: %w", err)
	}
	var saved Report
	if err := json.Unmarshal(b, &saved); err != nil {
		return nil, fmt.Errorf("read report: %w", err)
	}

	var mismatches []string
	selected := file.Scenario.SelectedChecks()
	planned := 0
	recomputedEntries := make([]Entry, len(entries))
	for i, e := range entries {
		planned = max(planned, e.History.Run.Runs)
		recomputed, err := checks.Evaluate(e.History, selected)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.File, err)
		}
		for _, m := range lockCallMismatches(e.History) {
			mismatches = append(mismatches, fmt.Sprintf("%s: %s", e.File, m))
		}
		if len(e.History.Verdicts) == 0 {
			mismatches = append(mismatches, fmt.Sprintf("%s: no stored verdicts", e.File))
		} else if !sameJSON(e.History.Verdicts, recomputed) {
			mismatches = append(mismatches, fmt.Sprintf("%s: recomputed verdicts differ from the stored ones", e.File))
		}
		h := *e.History
		h.Verdicts = recomputed
		recomputedEntries[i] = Entry{File: e.File, History: &h}
	}

	want := Build(file, recomputedEntries, planned, StoppedEarly(recomputedEntries, planned))
	for _, d := range reportDiffs(&saved, want) {
		mismatches = append(mismatches, "report.json: "+d)
	}
	return mismatches, nil
}

func reportDiffs(saved, want *Report) []string {
	var d []string
	field := func(name string, a, b any) {
		if !sameJSON(a, b) {
			d = append(d, name+" does not match the recomputed value")
		}
	}
	field("schema", saved.Schema, want.Schema)
	field("scenario", saved.Scenario, want.Scenario)
	field("oversold", saved.Oversold, want.Oversold)
	field("versions_used", saved.VersionsUsed, want.VersionsUsed)
	field("servers", saved.Servers, want.Servers)
	field("postgres_version", saved.PostgresVersion, want.PostgresVersion)
	field("locking", saved.Locking, want.Locking)
	field("redis_versions", saved.RedisVersions, want.RedisVersions)
	field("runs_planned", saved.RunsPlanned, want.RunsPlanned)
	field("runs_recorded", saved.RunsRecorded, want.RunsRecorded)
	field("stopped_early", saved.StoppedEarly, want.StoppedEarly)
	field("checks", saved.Checks, want.Checks)
	field("runs", saved.Runs, want.Runs)
	field("exit_code", saved.ExitCode, want.ExitCode)
	return d
}

func lockCallMismatches(h *history.History) []string {
	stats, calls := h.Race.RedisCommandStats, h.Race.LockCalls
	switch {
	case stats == nil && calls == nil:
		return nil
	case stats == nil:
		return []string{"lock_calls is set without redis_command_stats"}
	case calls == nil:
		if h.HarnessError == "" {
			return []string{"redis_command_stats is set without lock_calls"}
		}
		return nil
	}
	n, err := history.LockCalls(stats.Before, stats.After)
	if err != nil {
		return []string{err.Error()}
	}
	if n != *calls {
		return []string{fmt.Sprintf("lock_calls is %d, the stored redis command stats give %d", *calls, n)}
	}
	return nil
}

func sameJSON(a, b any) bool {
	ja, err1 := json.Marshal(a)
	jb, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && bytes.Equal(ja, jb)
}
