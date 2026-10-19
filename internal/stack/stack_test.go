package stack

import "testing"

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
