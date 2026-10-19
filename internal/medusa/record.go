package medusa

import (
	"encoding/json"
	"sync"
	"time"
)

type Call struct {
	Index          int             `json:"index"`
	Step           string          `json:"step"`
	Method         string          `json:"method"`
	Path           string          `json:"path"`
	RequestBody    json.RawMessage `json:"request_body,omitempty"`
	Status         int             `json:"status"`
	ResponseBody   json.RawMessage `json:"response_body,omitempty"`
	Start          Timestamp       `json:"start"`
	End            Timestamp       `json:"end"`
	Error          string          `json:"error,omitempty"`
	Timeout        bool            `json:"timeout"`
	OversoldCommit string          `json:"oversold_commit"`
	MedusaVersion  string          `json:"medusa_version"`
}

const TimestampFormat = "2006-01-02T15:04:05.000000000Z"

type Timestamp struct {
	time.Time
}

func (t Timestamp) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.UTC().Format(TimestampFormat) + `"`), nil
}

func (t *Timestamp) UnmarshalJSON(b []byte) error {
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

type Recorder struct {
	OversoldCommit string
	MedusaVersion  string

	mu    sync.Mutex
	calls []Call
}

func NewRecorder(oversoldCommit, medusaVersion string) *Recorder {
	return &Recorder{OversoldCommit: oversoldCommit, MedusaVersion: medusaVersion}
}

func (r *Recorder) now() Timestamp {
	return Timestamp{time.Now().UTC()}
}

func (r *Recorder) add(c Call) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c.Index = len(r.calls)
	c.OversoldCommit = r.OversoldCommit
	c.MedusaVersion = r.MedusaVersion
	r.calls = append(r.calls, c)
}

func (r *Recorder) Calls() []Call {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Call, len(r.calls))
	copy(out, r.calls)
	return out
}

func rawBody(b []byte) json.RawMessage {
	if len(b) == 0 {
		return nil
	}
	if json.Valid(b) {
		return json.RawMessage(b)
	}
	s, _ := json.Marshal(string(b))
	return json.RawMessage(s)
}
