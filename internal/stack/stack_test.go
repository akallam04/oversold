package stack

import (
	"strings"
	"testing"

	"github.com/akallam04/oversold/internal/envfile"
)

func TestLocalEndpoint(t *testing.T) {
	tests := []struct {
		host string
		ok   bool
	}{
		{"unix:///var/run/docker.sock", true},
		{"unix:///Users/someone/.docker/run/docker.sock", true},
		{"npipe:////./pipe/docker_engine", true},
		{"tcp://127.0.0.1:2375", true},
		{"tcp://localhost:2376", true},
		{"tcp://[::1]:2375", true},
		{"tcp://10.0.0.8:2375", false},
		{"tcp://docker.example.com:2376", false},
		{"ssh://user@buildbox", false},
		{"tcp://0.0.0.0:2375", false},
		{"", false},
	}
	for _, tt := range tests {
		err := LocalEndpoint(tt.host)
		if tt.ok != (err == nil) {
			t.Errorf("LocalEndpoint(%q) = %v, want ok=%v", tt.host, err, tt.ok)
		}
	}
}

func TestTemplateName(t *testing.T) {
	s := &Stack{Version: "2.21.2"}
	if got := s.TemplateName(); got != "medusa_template_2_21_2" {
		t.Errorf("template name = %q", got)
	}
}

func clearNameEnv(t *testing.T) {
	for _, k := range []string{"COMPOSE_PROJECT_NAME", "OVERSOLD_IMAGE_PREFIX", "POSTGRES_PORT", "MEDUSA_PORT", "MEDUSA_2_PORT"} {
		t.Setenv(k, "")
	}
}

func TestNewPicksImageAndTemplateFromVersion(t *testing.T) {
	clearNameEnv(t)
	for _, v := range []string{"2.19.0", "2.21.2", "2.22.0"} {
		s, err := New("../..", envfile.Env{"MEDUSA_VERSION": "2.21.2"}, v)
		if err != nil {
			t.Fatalf("New(%s): %v", v, err)
		}
		if s.ImageName() != "oversold-medusa:"+v {
			t.Errorf("image = %s", s.ImageName())
		}
		if s.TemplateName() != "medusa_template_"+strings.ReplaceAll(v, ".", "_") {
			t.Errorf("template = %s", s.TemplateName())
		}
	}
	if _, err := New("../..", envfile.Env{}, "2.18.0"); err == nil {
		t.Error("a version without build files should be refused")
	}
	if _, err := New("../..", envfile.Env{}, "2.21.2; drop"); err == nil {
		t.Error("a malformed version should be refused")
	}
}

func TestNamesDefaults(t *testing.T) {
	clearNameEnv(t)
	s, err := New("../..", envfile.Env{}, "2.21.2")
	if err != nil {
		t.Fatal(err)
	}
	want := Names{Project: "oversold", ImagePrefix: "oversold-medusa", PostgresPort: "55432", MedusaPort: "9000", Medusa2Port: "9001"}
	if s.Names != want {
		t.Errorf("names = %+v, want %+v", s.Names, want)
	}
	if got := s.ImageName(); got != "oversold-medusa:2.21.2" {
		t.Errorf("image = %s", got)
	}
	if got := s.Servers(2); got[0].URL != "http://127.0.0.1:9000" || got[1].URL != "http://127.0.0.1:9001" {
		t.Errorf("servers = %+v", got)
	}
	if got := s.DatabaseURL("medusa"); !strings.Contains(got, "@127.0.0.1:55432/medusa") {
		t.Errorf("database url = %s", got)
	}
}

func TestNamesOverrides(t *testing.T) {
	clearNameEnv(t)
	env := envfile.Env{
		"COMPOSE_PROJECT_NAME":  "oversold-b",
		"OVERSOLD_IMAGE_PREFIX": "oversold-b-medusa",
		"POSTGRES_PORT":         "56432",
		"MEDUSA_PORT":           "9100",
		"MEDUSA_2_PORT":         "9101",
	}
	s, err := New("../..", env, "2.22.0")
	if err != nil {
		t.Fatal(err)
	}
	if got := s.ImageName(); got != "oversold-b-medusa:2.22.0" {
		t.Errorf("image = %s", got)
	}
	if got := s.Servers(2); got[0].URL != "http://127.0.0.1:9100" || got[1].URL != "http://127.0.0.1:9101" {
		t.Errorf("servers = %+v", got)
	}
	if got := s.DatabaseURL("medusa"); !strings.Contains(got, "@127.0.0.1:56432/medusa") {
		t.Errorf("database url = %s", got)
	}
	environ := strings.Join(s.Names.Environ(), " ")
	for _, kv := range []string{"COMPOSE_PROJECT_NAME=oversold-b", "OVERSOLD_IMAGE_PREFIX=oversold-b-medusa", "POSTGRES_PORT=56432", "MEDUSA_PORT=9100", "MEDUSA_2_PORT=9101"} {
		if !strings.Contains(environ, kv) {
			t.Errorf("compose environment %q is missing %s", environ, kv)
		}
	}

	t.Setenv("MEDUSA_PORT", "9200")
	names, err := NamesFrom(env)
	if err != nil {
		t.Fatal(err)
	}
	if names.MedusaPort != "9200" {
		t.Errorf("the shell environment should win over .env, got %s", names.MedusaPort)
	}
}

func TestNamesRefuseBadValues(t *testing.T) {
	clearNameEnv(t)
	tests := []struct {
		env  envfile.Env
		want string
	}{
		{envfile.Env{"COMPOSE_PROJECT_NAME": "Oversold"}, "COMPOSE_PROJECT_NAME"},
		{envfile.Env{"COMPOSE_PROJECT_NAME": "-oversold"}, "COMPOSE_PROJECT_NAME"},
		{envfile.Env{"OVERSOLD_IMAGE_PREFIX": "registry.example.com/medusa"}, "OVERSOLD_IMAGE_PREFIX"},
		{envfile.Env{"OVERSOLD_IMAGE_PREFIX": "medusa:2"}, "OVERSOLD_IMAGE_PREFIX"},
		{envfile.Env{"POSTGRES_PORT": "0"}, "POSTGRES_PORT"},
		{envfile.Env{"MEDUSA_PORT": "70000"}, "MEDUSA_PORT"},
		{envfile.Env{"MEDUSA_2_PORT": "09001"}, "MEDUSA_2_PORT"},
		{envfile.Env{"MEDUSA_2_PORT": "9000"}, "MEDUSA_PORT and MEDUSA_2_PORT are both 9000"},
	}
	for _, tt := range tests {
		if _, err := NamesFrom(tt.env); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("NamesFrom(%v) = %v, want error with %q", tt.env, err, tt.want)
		}
	}
}
