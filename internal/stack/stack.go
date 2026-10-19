package stack

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/akallam04/oversold/internal/envfile"
	"github.com/akallam04/oversold/internal/history"
	"github.com/akallam04/oversold/internal/localguard"
)

const (
	healthTimeout  = 120 * time.Second
	healthInterval = 100 * time.Millisecond
	templatePrefix = "oversold template image="
	appDatabase    = "medusa"
)

var localDockerHosts = map[string]bool{"localhost": true, "127.0.0.1": true, "::1": true}

func LocalEndpoint(host string) error {
	if host == "" {
		return errors.New("docker endpoint is empty")
	}
	u, err := url.Parse(host)
	if err != nil {
		return fmt.Errorf("docker endpoint %q: %w", host, err)
	}
	switch u.Scheme {
	case "unix", "npipe":
		return nil
	case "tcp", "http", "https":
		if localDockerHosts[u.Hostname()] {
			return nil
		}
	}
	return fmt.Errorf("refusing docker endpoint %q: only local unix or npipe sockets and tcp on localhost, 127.0.0.1 or ::1 are allowed", host)
}

func CheckDocker(ctx context.Context) (string, error) {
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		return host, LocalEndpoint(host)
	}
	out, err := exec.CommandContext(ctx, "docker", "context", "inspect", "--format", "{{.Endpoints.docker.Host}}").Output()
	if err != nil {
		return "", fmt.Errorf("read current docker context: %w", err)
	}
	host := strings.TrimSpace(string(out))
	return host, LocalEndpoint(host)
}

type Server struct {
	Index   int
	Service string
	URL     string
}

type Stack struct {
	Dir     string
	Env     envfile.Env
	Version string
}

func New(dir string, env envfile.Env, version string) (*Stack, error) {
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(version) {
		return nil, fmt.Errorf("bad medusa version %q", version)
	}
	if _, err := os.Stat(dir + "/compose.yaml"); err != nil {
		return nil, fmt.Errorf("compose.yaml not found in %s, run oversold from the repository root", dir)
	}
	if got := env.Get("MEDUSA_VERSION", ""); got != version {
		return nil, fmt.Errorf("scenario targets medusa %s but MEDUSA_VERSION in .env is %q", version, got)
	}
	return &Stack{Dir: dir, Env: env, Version: version}, nil
}

func (s *Stack) Servers(n int) []Server {
	all := []Server{
		{Index: 1, Service: "medusa", URL: "http://127.0.0.1:" + s.Env.Get("MEDUSA_PORT", "9000")},
		{Index: 2, Service: "medusa-2", URL: "http://127.0.0.1:" + s.Env.Get("MEDUSA_2_PORT", "9001")},
	}
	if n < 1 || n > len(all) {
		return nil
	}
	return all[:n]
}

func (s *Stack) TemplateName() string {
	return "medusa_template_" + strings.ReplaceAll(s.Version, ".", "_")
}

func (s *Stack) ImageName() string {
	return "oversold-medusa:" + s.Version
}

func (s *Stack) DatabaseURL(db string) string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(s.Env.Get("POSTGRES_USER", "oversold"), s.Env.Get("POSTGRES_PASSWORD", "")),
		Host:     "127.0.0.1:" + s.Env.Get("POSTGRES_PORT", "55432"),
		Path:     "/" + db,
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

func (s *Stack) AppDatabaseURL() string {
	return s.DatabaseURL(appDatabase)
}

func (s *Stack) docker(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = s.Dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("docker %s: %w: %s", strings.Join(args, " "), err, lastLines(out.String(), 5))
	}
	return strings.TrimSpace(out.String()), nil
}

func (s *Stack) compose(ctx context.Context, args ...string) (string, error) {
	return s.docker(ctx, append([]string{"compose"}, args...)...)
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " | ")
}

func (s *Stack) ImageID(ctx context.Context) (string, error) {
	return s.docker(ctx, "image", "inspect", "--format", "{{.Id}}", s.ImageName())
}

type ResetResult struct {
	Start           time.Time
	Healthy         time.Time
	TemplateCreated bool
}

func (s *Stack) Reset(ctx context.Context, servers int) (ResetResult, error) {
	res := ResetResult{Start: time.Now()}
	want := s.Servers(servers)
	if want == nil {
		return res, fmt.Errorf("unsupported server count %d", servers)
	}

	if _, err := s.compose(ctx, "stop", "medusa", "medusa-2"); err != nil {
		return res, err
	}

	imageID, err := s.ImageID(ctx)
	if err != nil {
		return res, fmt.Errorf("medusa image %s not found, run make up first: %w", s.ImageName(), err)
	}

	adminURL := s.DatabaseURL(s.Env.Get("POSTGRES_DB", "oversold"))
	if err := localguard.Check(adminURL); err != nil {
		return res, err
	}
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return res, fmt.Errorf("connect to postgres, run make up first: %w", err)
	}
	defer conn.Close(context.Background())

	tmpl := s.TemplateName()
	var comment *string
	err = conn.QueryRow(ctx, `select shobj_description(oid, 'pg_database') from pg_database where datname = $1`, tmpl).Scan(&comment)
	switch {
	case errors.Is(err, pgx.ErrNoRows), err == nil && (comment == nil || *comment != templatePrefix+imageID):
		if err := s.buildTemplate(ctx, conn, tmpl, imageID); err != nil {
			return res, fmt.Errorf("build template %s: %w", tmpl, err)
		}
		res.TemplateCreated = true
	case err != nil:
		return res, err
	}

	app := pgx.Identifier{appDatabase}.Sanitize()
	if _, err := conn.Exec(ctx, "drop database if exists "+app+" with (force)"); err != nil {
		return res, err
	}
	if _, err := conn.Exec(ctx, "create database "+app+" template "+pgx.Identifier{tmpl}.Sanitize()); err != nil {
		return res, err
	}

	services := []string{"up", "-d", "--no-deps"}
	for _, srv := range want {
		services = append(services, srv.Service)
	}
	if _, err := s.compose(ctx, services...); err != nil {
		return res, err
	}
	for _, srv := range want {
		if err := waitHealthy(ctx, srv.URL); err != nil {
			return res, fmt.Errorf("%s: %w", srv.Service, err)
		}
	}
	res.Healthy = time.Now()
	return res, nil
}

func (s *Stack) buildTemplate(ctx context.Context, conn *pgx.Conn, tmpl, imageID string) error {
	app := pgx.Identifier{appDatabase}.Sanitize()
	t := pgx.Identifier{tmpl}.Sanitize()
	steps := []string{
		"drop database if exists " + app + " with (force)",
		"create database " + app,
	}
	for _, q := range steps {
		if _, err := conn.Exec(ctx, q); err != nil {
			return err
		}
	}
	if _, err := s.compose(ctx, "run", "--rm", "--no-deps", "-T", "medusa-seed", "node_modules/.bin/medusa", "db:migrate"); err != nil {
		return err
	}
	if _, err := s.compose(ctx, "run", "--rm", "--no-deps", "-T", "medusa-seed"); err != nil {
		return err
	}
	steps = []string{
		"drop database if exists " + t + " with (force)",
		"create database " + t + " template " + app,
		"comment on database " + t + " is " + quoteLiteral(templatePrefix+imageID),
	}
	for _, q := range steps {
		if _, err := conn.Exec(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func waitHealthy(ctx context.Context, base string) error {
	if err := localguard.Check(base); err != nil {
		return err
	}
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(healthTimeout)
	var last error
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/health", nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			last = fmt.Errorf("health returned %d", resp.StatusCode)
		} else {
			last = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(healthInterval):
		}
	}
	return fmt.Errorf("not healthy after %s: %v", healthTimeout, last)
}

func (s *Stack) Facts(ctx context.Context, servers int) ([]history.Server, error) {
	var out []history.Server
	for _, srv := range s.Servers(servers) {
		id, err := s.compose(ctx, "ps", "-q", srv.Service)
		if err != nil {
			return nil, err
		}
		if id == "" {
			return nil, fmt.Errorf("%s has no running container", srv.Service)
		}
		inspect, err := s.docker(ctx, "inspect", "--format", "{{.Image}} {{.State.StartedAt}}", id)
		if err != nil {
			return nil, err
		}
		image, startedAt, _ := strings.Cut(inspect, " ")
		version, err := s.docker(ctx, "exec", id, "node", "-p", `require("@medusajs/medusa/package.json").version`)
		if err != nil {
			return nil, err
		}
		out = append(out, history.Server{
			Index:         srv.Index,
			Service:       srv.Service,
			URL:           srv.URL,
			ContainerID:   id,
			ImageID:       image,
			StartedAt:     startedAt,
			MedusaVersion: version,
		})
	}
	return out, nil
}
