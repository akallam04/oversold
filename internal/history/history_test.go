package history

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/akallam04/oversold/internal/dbstate"
)

func TestClassify(t *testing.T) {
	timeout := &net.OpError{Op: "read", Err: context.DeadlineExceeded}
	tests := []struct {
		name   string
		step   string
		status int
		body   string
		err    error
		want   Outcome
	}{
		{"complete gives order", StepComplete, 200, `{"type":"order","order":{"id":"order_1"}}`, nil, OK},
		{"complete gives cart with error", StepComplete, 200, `{"type":"cart","cart":{"id":"cart_1"},"error":{"message":"out of stock"}}`, nil, Failed},
		{"complete gives cart without error", StepComplete, 200, `{"type":"cart","cart":{"id":"cart_1"}}`, nil, Unknown},
		{"complete gives order without id", StepComplete, 200, `{"type":"order","order":{}}`, nil, Unknown},
		{"complete gives unreadable body", StepComplete, 200, `not json`, nil, Unknown},
		{"complete conflict", StepComplete, 409, `{"type":"conflict","message":"busy"}`, nil, Failed},
		{"complete bad request", StepComplete, 400, `{"type":"invalid_data"}`, nil, Failed},
		{"complete server error", StepComplete, 500, `{"type":"unknown_error"}`, nil, Unknown},
		{"complete bad gateway", StepComplete, 502, ``, nil, Unknown},
		{"complete timeout", StepComplete, 0, ``, timeout, Unknown},
		{"complete connection refused", StepComplete, 0, ``, errors.New("dial tcp 127.0.0.1:9000: connection refused"), Unknown},
		{"setup ok", "create_cart", 200, `{"cart":{"id":"cart_1"}}`, nil, OK},
		{"setup created", "create_cart", 201, `{}`, nil, OK},
		{"setup not found", "add_line_item", 404, `{}`, nil, Failed},
		{"setup server error", "add_line_item", 503, `{}`, nil, Unknown},
		{"setup redirect", "list_regions", 302, ``, nil, Unknown},
		{"setup timeout", "list_regions", 0, ``, timeout, Unknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.step, tt.status, []byte(tt.body), tt.err); got != tt.want {
				t.Errorf("Classify = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRecorderOffsets(t *testing.T) {
	start := time.Now()
	rec := NewRecorder(start, nil)
	p1 := rec.Begin()
	time.Sleep(2 * time.Millisecond)
	p2 := rec.Begin()
	p2.Finish(Operation{Step: "b"}, nil, nil)
	p1.Finish(Operation{Step: "a"}, nil, nil)

	ops := rec.Operations()
	if len(ops) != 2 || ops[0].Step != "a" || ops[0].ID != 1 || ops[1].ID != 2 {
		t.Fatalf("ops = %+v", ops)
	}
	if ops[0].StartNS < 0 || ops[1].StartNS-ops[0].StartNS < int64(2*time.Millisecond) {
		t.Errorf("offsets %d %d", ops[0].StartNS, ops[1].StartNS)
	}
	if ops[0].EndNS < ops[0].StartNS || ops[0].Start.Location() != time.UTC {
		t.Errorf("op = %+v", ops[0])
	}
	b, _ := json.Marshal(ops[0])
	var enc struct {
		Start string `json:"start"`
	}
	json.Unmarshal(b, &enc)
	if _, err := time.Parse(TimestampFormat, enc.Start); err != nil || len(enc.Start) != len(TimestampFormat) {
		t.Errorf("start %q not in %s", enc.Start, TimestampFormat)
	}
}

func TestMaskBody(t *testing.T) {
	m := NewMasker("hunter2-secret")
	got := string(m.Body([]byte(`{"email":"a@example.com","password":"hunter2-secret","nested":{"Token":"abc"},"note":"pw is hunter2-secret","keys":["pk_0123456789abcdef0123"]}`)))
	for _, leak := range []string{"hunter2-secret", "abc", "pk_0123456789abcdef0123"} {
		if strings.Contains(got, leak) {
			t.Errorf("body leaks %q: %s", leak, got)
		}
	}
	if !strings.Contains(got, `"email":"a@example.com"`) || strings.Count(got, Masked) != 4 {
		t.Errorf("masked body = %s", got)
	}
	if got := string(m.Body([]byte("plain hunter2-secret text"))); got != `"plain [masked] text"` {
		t.Errorf("text body = %s", got)
	}
}

func TestWriteMasksSnapshotAndErrors(t *testing.T) {
	key := "pk_" + strings.Repeat("ab12", 16)
	m := NewMasker("db-password-value")
	h := &History{
		Schema: Schema,
		Before: &dbstate.Snapshot{
			VariantSKUs: []string{"OVS-PIECE"},
			Orders:      []dbstate.Order{{ID: "order_1", Status: key}},
		},
		HarnessError: "connect postgres://oversold:db-password-value@127.0.0.1/medusa failed",
	}
	path := filepath.Join(t.TempDir(), "run-001.json")
	if err := Write(path, h, m); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	for _, leak := range []string{key, "db-password-value"} {
		if strings.Contains(string(b), leak) {
			t.Errorf("history leaks %q", leak)
		}
	}
	if !strings.Contains(string(b), Masked) {
		t.Error("nothing was masked")
	}
}

func TestSnapshotHasNoSecretFields(t *testing.T) {
	seen := map[reflect.Type]bool{}
	var walk func(reflect.Type, string)
	walk = func(rt reflect.Type, path string) {
		for rt.Kind() == reflect.Pointer || rt.Kind() == reflect.Slice || rt.Kind() == reflect.Map {
			rt = rt.Elem()
		}
		if rt.Kind() != reflect.Struct || seen[rt] {
			return
		}
		seen[rt] = true
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if sensitiveKeys[strings.ToLower(name)] || strings.Contains(strings.ToLower(name), "token") {
				t.Errorf("snapshot field %s.%s would hold a secret", path, name)
			}
			walk(f.Type, path+"."+name)
		}
	}
	walk(reflect.TypeOf(dbstate.Snapshot{}), "snapshot")
	if len(seen) < 5 {
		t.Errorf("walked only %d struct types", len(seen))
	}
}
