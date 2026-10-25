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
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/akallam04/oversold/internal/envfile"
	"github.com/akallam04/oversold/internal/history"
	"github.com/akallam04/oversold/internal/localguard"
	"github.com/akallam04/oversold/internal/targets"
)

const (
	healthTimeout  = 120 * time.Second
	healthInterval = 100 * time.Millisecond
	templatePrefix = "oversold template image="
	appDatabase    = "medusa"

	DefaultProject      = "oversold"
	DefaultImagePrefix  = "oversold-medusa"
	DefaultPostgresPort = "55432"
	DefaultMedusaPort   = "9000"
	DefaultMedusa2Port  = "9001"
)

var (
	projectName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	imagePrefix = regexp.MustCompile(`^[a-z0-9]+([._-][a-z0-9]+)*$`)
)

type Names struct {
	Project      string
	ImagePrefix  string
	PostgresPort string
	MedusaPort   string
	Medusa2Port  string
}

func NamesFrom(env envfile.Env) (Names, error) {
	n := Names{
		Project:      env.Get("COMPOSE_PROJECT_NAME", DefaultProject),
		ImagePrefix:  env.Get("OVERSOLD_IMAGE_PREFIX", DefaultImagePrefix),
		PostgresPort: env.Get("POSTGRES_PORT", DefaultPostgresPort),
		MedusaPort:   env.Get("MEDUSA_PORT", DefaultMedusaPort),
		Medusa2Port:  env.Get("MEDUSA_2_PORT", DefaultMedusa2Port),
	}
	if !projectName.MatchString(n.Project) {
		return n, fmt.Errorf("COMPOSE_PROJECT_NAME %q: use lowercase letters, digits, dashes and underscores, starting with a letter or digit", n.Project)
	}
	if !imagePrefix.MatchString(n.ImagePrefix) || len(n.ImagePrefix) > 128 {
		return n, fmt.Errorf("OVERSOLD_IMAGE_PREFIX %q: use lowercase letters and digits, separated by single dots, dashes or underscores", n.ImagePrefix)
	}
	seen := map[string]string{}
	for _, p := range []struct{ key, value string }{
		{"POSTGRES_PORT", n.PostgresPort},
		{"MEDUSA_PORT", n.MedusaPort},
		{"MEDUSA_2_PORT", n.Medusa2Port},
	} {
		port, err := strconv.Atoi(p.value)
		if err != nil || port < 1 || port > 65535 || strconv.Itoa(port) != p.value {
			return n, fmt.Errorf("%s %q: want a port from 1 to 65535", p.key, p.value)
		}
		if other, ok := seen[p.value]; ok {
			return n, fmt.Errorf("%s and %s are both %s, each needs its own port", other, p.key, p.value)
		}
		seen[p.value] = p.key
	}
	return n, nil
}

func (n Names) Environ() []string {
	return []string{
		"COMPOSE_PROJECT_NAME=" + n.Project,
		"OVERSOLD_IMAGE_PREFIX=" + n.ImagePrefix,
		"POSTGRES_PORT=" + n.PostgresPort,
		"MEDUSA_PORT=" + n.MedusaPort,
		"MEDUSA_2_PORT=" + n.Medusa2Port,
	}
}

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
	Names   Names
}

func New(dir string, env envfile.Env, version string) (*Stack, error) {
	if _, err := os.Stat(dir + "/compose.yaml"); err != nil {
		return nil, fmt.Errorf("compose.yaml not found in %s, run oversold from the repository root", dir)
	}
	if err := targets.Check(dir, "medusa", version); err != nil {
		return nil, err
	}
	names, err := NamesFrom(env)
	if err != nil {
		return nil, err
	}
	return &Stack{Dir: dir, Env: env, Version: version, Names: names}, nil
}

func (s *Stack) Servers(n int) []Server {
	all := []Server{
		{Index: 1, Service: "medusa", URL: "http://127.0.0.1:" + s.Names.MedusaPort},
		{Index: 2, Service: "medusa-2", URL: "http://127.0.0.1:" + s.Names.Medusa2Port},
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
	return s.Names.ImagePrefix + ":" + s.Version
}

func (s *Stack) DatabaseURL(db string) string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(s.Env.Get("POSTGRES_USER", "oversold"), s.Env.Get("POSTGRES_PASSWORD", "")),
		Host:     "127.0.0.1:" + s.Names.PostgresPort,
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
	cmd.Env = append(append(os.Environ(), "MEDUSA_VERSION="+s.Version), s.Names.Environ()...)
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
	ImageBuilt      bool
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

	imageID, built, err := s.ensureImage(ctx)
	res.ImageBuilt = built
	if err != nil {
		return res, err
	}

	conn, err := s.connectAdmin(ctx)
	if err != nil {
		return res, err
	}
	defer conn.Close(context.Background())

	tmpl := s.TemplateName()
	created, err := s.ensureTemplate(ctx, conn, imageID)
	res.TemplateCreated = created
	if err != nil {
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

type PrepareResult struct {
	ImageID         string
	ImageBuilt      bool
	ImageTime       time.Duration
	TemplateCreated bool
	TemplateTime    time.Duration
}

func (s *Stack) Prepare(ctx context.Context) (PrepareResult, error) {
	var res PrepareResult
	if _, err := s.compose(ctx, "stop", "medusa", "medusa-2"); err != nil {
		return res, err
	}
	start := time.Now()
	imageID, built, err := s.ensureImage(ctx)
	res.ImageID, res.ImageBuilt, res.ImageTime = imageID, built, time.Since(start)
	if err != nil {
		return res, err
	}
	conn, err := s.connectAdmin(ctx)
	if err != nil {
		return res, err
	}
	defer conn.Close(context.Background())
	start = time.Now()
	created, err := s.ensureTemplate(ctx, conn, imageID)
	res.TemplateCreated, res.TemplateTime = created, time.Since(start)
	return res, err
}

func (s *Stack) ensureImage(ctx context.Context) (string, bool, error) {
	imageID, err := s.ImageID(ctx)
	if err == nil {
		return imageID, false, nil
	}
	if _, err := s.compose(ctx, "build", "medusa-migrate"); err != nil {
		return "", false, fmt.Errorf("build medusa image %s: %w", s.ImageName(), err)
	}
	if imageID, err = s.ImageID(ctx); err != nil {
		return "", true, fmt.Errorf("medusa image %s not found after build: %w", s.ImageName(), err)
	}
	return imageID, true, nil
}

func (s *Stack) connectAdmin(ctx context.Context) (*pgx.Conn, error) {
	adminURL := s.DatabaseURL(s.Env.Get("POSTGRES_DB", "oversold"))
	if err := localguard.Check(adminURL); err != nil {
		return nil, err
	}
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres, run make up first: %w", err)
	}
	return conn, nil
}

func (s *Stack) ensureTemplate(ctx context.Context, conn *pgx.Conn, imageID string) (bool, error) {
	tmpl := s.TemplateName()
	var comment *string
	err := conn.QueryRow(ctx, `select shobj_description(oid, 'pg_database') from pg_database where datname = $1`, tmpl).Scan(&comment)
	switch {
	case errors.Is(err, pgx.ErrNoRows), err == nil && (comment == nil || *comment != templatePrefix+imageID):
		if err := s.buildTemplate(ctx, conn, tmpl, imageID); err != nil {
			return false, fmt.Errorf("build template %s: %w", tmpl, err)
		}
		return true, nil
	case err != nil:
		return false, err
	}
	return false, nil
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
