package history

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/akallam04/oversold/internal/dbstate"
	"github.com/akallam04/oversold/internal/targets"
)

const Schema = "oversold.history/v1"

const TimestampFormat = "2006-01-02T15:04:05.000000000Z"

type Timestamp struct {
	time.Time
}

func (t Timestamp) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + t.UTC().Format(TimestampFormat) + `"`), nil
}

func (t *Timestamp) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		t.Time = time.Time{}
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return err
	}
	t.Time = v.UTC()
	return nil
}

type Outcome string

const (
	OK      Outcome = "ok"
	Failed  Outcome = "failed"
	Unknown Outcome = "unknown"
)

const (
	PhaseSetup = "setup"
	PhaseRace  = "race"
)

const StepComplete = "complete_cart"

func Classify(step string, status int, body []byte, transportErr error) Outcome {
	if transportErr != nil {
		return Unknown
	}
	switch {
	case status >= 500:
		return Unknown
	case status >= 400:
		return Failed
	case status < 200 || status > 299:
		return Unknown
	}
	if step != StepComplete {
		return OK
	}
	var r struct {
		Type  string `json:"type"`
		Order *struct {
			ID string `json:"id"`
		} `json:"order"`
		Error json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return Unknown
	}
	switch {
	case r.Type == "order" && r.Order != nil && r.Order.ID != "":
		return OK
	case r.Type == "cart" && len(r.Error) > 0 && string(r.Error) != "null":
		return Failed
	default:
		return Unknown
	}
}

type Operation struct {
	ID           int             `json:"id"`
	Phase        string          `json:"phase"`
	Step         string          `json:"step"`
	ActionID     string          `json:"action_id,omitempty"`
	Cart         string          `json:"cart,omitempty"`
	Server       int             `json:"server"`
	Method       string          `json:"method"`
	Path         string          `json:"path"`
	RequestBody  json.RawMessage `json:"request_body,omitempty"`
	Status       int             `json:"status"`
	ResponseBody json.RawMessage `json:"response_body,omitempty"`
	Start        Timestamp       `json:"start"`
	End          Timestamp       `json:"end"`
	StartNS      int64           `json:"start_ns"`
	EndNS        int64           `json:"end_ns"`
	Outcome      Outcome         `json:"outcome"`
	Error        string          `json:"error,omitempty"`
	Timeout      bool            `json:"timeout"`
	ConnReused   *bool           `json:"conn_reused,omitempty"`
}

type Recorder struct {
	runStart time.Time
	masker   *Masker

	mu   sync.Mutex
	next int
	ops  []Operation
}

func NewRecorder(runStart time.Time, masker *Masker) *Recorder {
	if masker == nil {
		masker = NewMasker()
	}
	return &Recorder{runStart: runStart, masker: masker, next: 1}
}

func (r *Recorder) Masker() *Masker {
	return r.masker
}

type Pending struct {
	rec   *Recorder
	id    int
	start time.Time
}

func (r *Recorder) Begin() *Pending {
	r.mu.Lock()
	id := r.next
	r.next++
	r.mu.Unlock()
	return &Pending{rec: r, id: id, start: time.Now()}
}

func (p *Pending) Finish(op Operation, reqBody, respBody []byte) Operation {
	end := time.Now()
	r := p.rec
	op.ID = p.id
	op.Start = Timestamp{p.start.UTC()}
	op.End = Timestamp{end.UTC()}
	op.StartNS = p.start.Sub(r.runStart).Nanoseconds()
	op.EndNS = end.Sub(r.runStart).Nanoseconds()
	op.RequestBody = r.masker.Body(reqBody)
	op.ResponseBody = r.masker.Body(respBody)
	op.Error = r.masker.String(op.Error)
	r.mu.Lock()
	r.ops = append(r.ops, op)
	r.mu.Unlock()
	return op
}

func (r *Recorder) Operations() []Operation {
	r.mu.Lock()
	out := make([]Operation, len(r.ops))
	copy(out, r.ops)
	r.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (r *Recorder) Since(t time.Time) int64 {
	return t.Sub(r.runStart).Nanoseconds()
}

type ScenarioRef struct {
	Name   string `json:"name"`
	Schema string `json:"schema"`
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}

type RunInfo struct {
	Index int   `json:"index"`
	Runs  int   `json:"runs"`
	Seed  int64 `json:"seed"`
}

type Build struct {
	Commit string `json:"commit"`
	Dirty  bool   `json:"dirty"`
}

type Server struct {
	Index         int    `json:"index"`
	Service       string `json:"service"`
	URL           string `json:"url"`
	ContainerID   string `json:"container_id"`
	ImageID       string `json:"image_id"`
	StartedAt     string `json:"started_at"`
	MedusaVersion string `json:"medusa_version"`
}

type Target struct {
	Engine           string   `json:"engine"`
	ScenarioVersion  string   `json:"scenario_version"`
	Version          string   `json:"version"`
	Servers          []Server `json:"servers"`
	PostgresVersion  string   `json:"postgres_version"`
	TemplateDatabase string   `json:"template_database"`
	Locking          string   `json:"locking,omitempty"`
	RedisVersion     string   `json:"redis_version,omitempty"`
}

func (t Target) LockingMode() string {
	if t.Locking == "" {
		return targets.LockingInMemory
	}
	return t.Locking
}

type Reset struct {
	Start           Timestamp `json:"start"`
	Healthy         Timestamp `json:"healthy"`
	StartNS         int64     `json:"start_ns"`
	HealthyNS       int64     `json:"healthy_ns"`
	TemplateCreated bool      `json:"template_created"`
	ImageBuilt      bool      `json:"image_built"`
}

type RaceAction struct {
	ID          string  `json:"id"`
	Do          string  `json:"do"`
	Cart        string  `json:"cart"`
	Server      int     `json:"server"`
	JitterNS    int64   `json:"jitter_ns"`
	OperationID int     `json:"operation_id,omitempty"`
	Outcome     Outcome `json:"outcome,omitempty"`
}

type Race struct {
	Release           string        `json:"release"`
	ReleaseNS         int64         `json:"release_ns"`
	SettleMS          int64         `json:"settle_ms"`
	Actions           []RaceAction  `json:"actions"`
	LockCalls         *int64        `json:"lock_calls,omitempty"`
	RedisCommandStats *CommandStats `json:"redis_command_stats,omitempty"`
}

type CommandStat struct {
	Calls         int64 `json:"calls"`
	FailedCalls   int64 `json:"failed_calls"`
	RejectedCalls int64 `json:"rejected_calls"`
}

type CommandStats struct {
	Before map[string]CommandStat `json:"before"`
	After  map[string]CommandStat `json:"after"`
}

var LockCommands = []string{"eval", "evalsha"}

func LockCalls(before, after map[string]CommandStat) (int64, error) {
	var n int64
	for _, cmd := range LockCommands {
		d := after[cmd].Calls - before[cmd].Calls
		if d < 0 {
			return 0, fmt.Errorf("redis %s calls went down from %d to %d, the stats were reset during the race", cmd, before[cmd].Calls, after[cmd].Calls)
		}
		n += d
	}
	return n, nil
}

type History struct {
	Schema       string            `json:"schema"`
	Scenario     ScenarioRef       `json:"scenario"`
	Run          RunInfo           `json:"run"`
	Oversold     Build             `json:"oversold"`
	Target       Target            `json:"target"`
	Start        Timestamp         `json:"start"`
	End          Timestamp         `json:"end"`
	Reset        *Reset            `json:"reset,omitempty"`
	Stock        map[string]int    `json:"stock,omitempty"`
	Carts        map[string]string `json:"carts,omitempty"`
	Race         Race              `json:"race"`
	Operations   []Operation       `json:"operations"`
	Before       *dbstate.Snapshot `json:"before"`
	After        *dbstate.Snapshot `json:"after"`
	HarnessError string            `json:"harness_error,omitempty"`
	Verdicts     []Verdict         `json:"verdicts,omitempty"`
}

type Result string

const (
	Pass         Result = "pass"
	Fail         Result = "fail"
	Inconclusive Result = "inconclusive"
)

type Evidence struct {
	Subject string                 `json:"subject"`
	Numbers map[string]json.Number `json:"numbers,omitempty"`
	Values  map[string]string      `json:"values,omitempty"`
}

type Verdict struct {
	Check    string     `json:"check"`
	Version  int        `json:"version"`
	Result   Result     `json:"result"`
	Reason   string     `json:"reason"`
	Evidence []Evidence `json:"evidence,omitempty"`
}

func Read(path string) (*History, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(path, ".gz") {
		zr, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if b, err = io.ReadAll(zr); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if err := zr.Close(); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	h := &History{}
	if err := json.Unmarshal(b, h); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if h.Schema != Schema {
		return nil, fmt.Errorf("%s: schema %q, want %q", path, h.Schema, Schema)
	}
	return h, nil
}

func Write(path string, h *History, m *Masker) error {
	if h == nil {
		return errors.New("history: nil")
	}
	if h.Operations == nil {
		h.Operations = []Operation{}
	}
	return WriteJSON(path, h, m)
}

func WriteJSON(path string, v any, m *Masker) error {
	if m == nil {
		m = NewMasker()
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = m.Document(b)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func CurrentBuild() Build {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return Build{Commit: "unknown"}
	}
	b := Build{Commit: "unknown"}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			b.Commit = s.Value
		case "vcs.modified":
			b.Dirty = s.Value == "true"
		}
	}
	return b
}
