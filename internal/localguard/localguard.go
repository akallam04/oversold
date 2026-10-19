package localguard

import (
	"fmt"
	"net/url"
)

var allowedHosts = map[string]bool{
	"localhost": true,
	"127.0.0.1": true,
	"::1":       true,
}

func Check(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse %q: %w", redact(raw), err)
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("url %q has no scheme or host", redact(raw))
	}
	host := u.Hostname()
	if !allowedHosts[host] {
		return fmt.Errorf("refusing non-local host %q in %q: only localhost, 127.0.0.1 and ::1 are allowed", host, redact(raw))
	}
	return nil
}

func redact(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	if _, ok := u.User.Password(); ok {
		u.User = url.UserPassword(u.User.Username(), "xxxxx")
	}
	return u.String()
}
