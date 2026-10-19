package localguard

import (
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	tests := []struct {
		url string
		ok  bool
	}{
		{"http://localhost:9000", true},
		{"http://127.0.0.1:9000/store", true},
		{"http://[::1]:9000", true},
		{"postgres://u:p@127.0.0.1:55432/medusa?sslmode=disable", true},
		{"postgres://u:p@localhost/medusa", true},
		{"http://LOCALHOST:9000", false},
		{"http://127.0.0.2:9000", false},
		{"http://localhost.example.com", false},
		{"http://example.com", false},
		{"https://shop.example.com@127.0.0.1.evil.test", false},
		{"http://10.0.0.5:9000", false},
		{"postgres://u:p@db.internal:5432/medusa", false},
		{"http://0.0.0.0:9000", false},
		{"localhost:9000", false},
		{"", false},
		{"/store/carts", false},
	}
	for _, tt := range tests {
		err := Check(tt.url)
		if tt.ok && err != nil {
			t.Errorf("Check(%q) = %v, want nil", tt.url, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("Check(%q) = nil, want error", tt.url)
		}
	}
}

func TestCheckRedactsPassword(t *testing.T) {
	err := Check("postgres://user:s3cret@db.example.com/medusa")
	if err == nil {
		t.Fatal("want error")
	}
	if strings.Contains(err.Error(), "s3cret") {
		t.Errorf("error leaks password: %v", err)
	}
}
