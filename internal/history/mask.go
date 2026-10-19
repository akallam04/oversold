package history

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
)

const Masked = "[masked]"

var sensitiveKeys = map[string]bool{
	"password":              true,
	"password_hash":         true,
	"token":                 true,
	"access_token":          true,
	"refresh_token":         true,
	"id_token":              true,
	"secret":                true,
	"client_secret":         true,
	"jwt":                   true,
	"jwt_secret":            true,
	"cookie_secret":         true,
	"api_key":               true,
	"apikey":                true,
	"publishable_key":       true,
	"x-publishable-api-key": true,
	"authorization":         true,
	"cookie":                true,
	"set-cookie":            true,
	"redacted":              true,
	"salt":                  true,
}

var tokenPattern = regexp.MustCompile(`\b(?:pk|sk)_[0-9A-Za-z]{16,}\b|eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`)

const minSecretLen = 4

type Masker struct {
	mu      sync.RWMutex
	secrets []string
}

func NewMasker(secrets ...string) *Masker {
	m := &Masker{}
	m.Add(secrets...)
	return m
}

func (m *Masker) Add(secrets ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range secrets {
		if len(s) >= minSecretLen {
			m.secrets = append(m.secrets, s)
		}
	}
}

func (m *Masker) list() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, len(m.secrets))
	copy(out, m.secrets)
	return out
}

func (m *Masker) String(s string) string {
	if s == "" {
		return s
	}
	for _, secret := range m.list() {
		s = strings.ReplaceAll(s, secret, Masked)
	}
	return tokenPattern.ReplaceAllString(s, Masked)
}

func (m *Masker) Body(b []byte) json.RawMessage {
	if len(b) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil || dec.More() {
		out, _ := json.Marshal(m.String(string(b)))
		return out
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(m.walk(v)); err != nil {
		out, _ := json.Marshal(Masked)
		return out
	}
	return json.RawMessage(bytes.TrimRight(buf.Bytes(), "\n"))
}

func (m *Masker) walk(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if sensitiveKeys[strings.ToLower(k)] && val != nil {
				t[k] = Masked
				continue
			}
			t[k] = m.walk(val)
		}
		return t
	case []any:
		for i := range t {
			t[i] = m.walk(t[i])
		}
		return t
	case string:
		return m.String(t)
	default:
		return v
	}
}

func (m *Masker) Document(b []byte) []byte {
	for _, secret := range m.list() {
		b = bytes.ReplaceAll(b, []byte(secret), []byte(Masked))
		if enc, err := json.Marshal(secret); err == nil {
			escaped := enc[1 : len(enc)-1]
			b = bytes.ReplaceAll(b, escaped, []byte(Masked))
		}
	}
	return tokenPattern.ReplaceAll(b, []byte(Masked))
}
