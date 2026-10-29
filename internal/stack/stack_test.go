package stack

import (
	"strings"
	"testing"

	"github.com/akallam04/oversold/internal/envfile"
	"github.com/akallam04/oversold/internal/history"
	"github.com/akallam04/oversold/internal/targets"
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
		s, err := New("../..", envfile.Env{"MEDUSA_VERSION": "2.21.2"}, v, nil)
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
	if _, err := New("../..", envfile.Env{}, "2.18.0", nil); err == nil {
		t.Error("a version without build files should be refused")
	}
	if _, err := New("../..", envfile.Env{}, "2.21.2; drop", nil); err == nil {
		t.Error("a malformed version should be refused")
	}
}

func TestNamesDefaults(t *testing.T) {
	clearNameEnv(t)
	s, err := New("../..", envfile.Env{}, "2.21.2", nil)
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
	s, err := New("../..", env, "2.22.0", nil)
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

const statsBefore = "# Commandstats\r\n" +
	"cmdstat_ping:calls=12,usec=20,usec_per_call=1.67,rejected_calls=0,failed_calls=0\r\n" +
	"cmdstat_evalsha:calls=40,usec=900,usec_per_call=22.50,rejected_calls=0,failed_calls=1\r\n" +
	"cmdstat_eval:calls=3,usec=100,usec_per_call=33.33,rejected_calls=0,failed_calls=0\r\n" +
	"cmdstat_set:calls=43,usec=80,usec_per_call=1.86,rejected_calls=0,failed_calls=0\r\n" +
	"cmdstat_info:calls=5,usec=300,usec_per_call=60.00,rejected_calls=0,failed_calls=0\r\n" +
	"cmdstat_client|list:calls=1,usec=10,usec_per_call=10.00,rejected_calls=0,failed_calls=0\r\n"

const statsAfter = "# Commandstats\r\n" +
	"cmdstat_ping:calls=12,usec=20,usec_per_call=1.67,rejected_calls=0,failed_calls=0\r\n" +
	"cmdstat_evalsha:calls=58,usec=1300,usec_per_call=22.41,rejected_calls=0,failed_calls=1\r\n" +
	"cmdstat_eval:calls=3,usec=100,usec_per_call=33.33,rejected_calls=0,failed_calls=0\r\n" +
	"cmdstat_set:calls=61,usec=110,usec_per_call=1.80,rejected_calls=0,failed_calls=0\r\n" +
	"cmdstat_get:calls=6,usec=6,usec_per_call=1.00,rejected_calls=0,failed_calls=0\r\n" +
	"cmdstat_info:calls=6,usec=360,usec_per_call=60.00,rejected_calls=0,failed_calls=0\r\n" +
	"cmdstat_client|list:calls=1,usec=10,usec_per_call=10.00,rejected_calls=0,failed_calls=0\r\n"

func TestLockCallsFromCommandStats(t *testing.T) {
	before, err := ParseCommandStats(statsBefore)
	if err != nil {
		t.Fatal(err)
	}
	after, err := ParseCommandStats(statsAfter)
	if err != nil {
		t.Fatal(err)
	}
	if got := after["evalsha"]; got != (history.CommandStat{Calls: 58, FailedCalls: 1}) {
		t.Errorf("evalsha = %+v", got)
	}
	if _, ok := after["client|list"]; !ok || len(after) != 7 {
		t.Errorf("after has %d commands: %v", len(after), after)
	}
	n, err := history.LockCalls(before, after)
	if err != nil {
		t.Fatal(err)
	}
	if n != 18 {
		t.Errorf("lock calls = %d, want 18 (evalsha 40 to 58, eval 3 to 3)", n)
	}

	empty, err := ParseCommandStats("# Commandstats\r\n")
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty stats = %v, %v", empty, err)
	}
	if n, err := history.LockCalls(empty, after); err != nil || n != 61 {
		t.Errorf("lock calls from a fresh Redis = %d, %v, want 61", n, err)
	}
	if n, err := history.LockCalls(before, before); err != nil || n != 0 {
		t.Errorf("lock calls with no change = %d, %v, want 0", n, err)
	}
	if _, err := history.LockCalls(after, before); err == nil || !strings.Contains(err.Error(), "went down from 58 to 40") {
		t.Errorf("stats that went down: err = %v", err)
	}
}

func TestParseCommandStatsRefusesBadInput(t *testing.T) {
	for _, in := range []string{
		"cmdstat_evalsha:calls=x,usec=1",
		"cmdstat_evalsha:usec=1",
		"cmdstat_:calls=1",
		"cmdstat_evalsha",
		"ERR unknown command",
		"cmdstat_eval:calls=-1",
	} {
		if _, err := ParseCommandStats(in); err == nil {
			t.Errorf("ParseCommandStats(%q) should fail", in)
		}
	}
}

func TestDockerEnvAlwaysSetsLocking(t *testing.T) {
	clearNameEnv(t)
	t.Setenv("OVERSOLD_LOCKING", "redis")
	s, err := New("../..", envfile.Env{}, "2.22.0", nil)
	if err != nil {
		t.Fatal(err)
	}
	last := func() string {
		v := ""
		for _, kv := range s.environ() {
			if rest, ok := strings.CutPrefix(kv, "OVERSOLD_LOCKING="); ok {
				v = rest
			}
		}
		return v
	}
	if s.Redis() || last() != "in-memory" {
		t.Errorf("default locking: Redis() = %v, docker gets OVERSOLD_LOCKING=%q, want in-memory over the shell's redis", s.Redis(), last())
	}
	s.Locking = "redis"
	if !s.Redis() || last() != "redis" {
		t.Errorf("redis locking: Redis() = %v, docker gets OVERSOLD_LOCKING=%q", s.Redis(), last())
	}
}

func TestVariantImageAndTemplate(t *testing.T) {
	clearNameEnv(t)
	root, err := targets.FindRoot()
	if err != nil {
		t.Fatal(err)
	}
	v, err := targets.LoadVariant(root, "medusa", "pr-16961")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := New("../..", envfile.Env{}, "2.22.0", nil)
	if err != nil {
		t.Fatal(err)
	}
	s, err := New("../..", envfile.Env{}, "2.22.0", v)
	if err != nil {
		t.Fatal(err)
	}
	if s.ImageName() != "oversold-medusa:2.22.0-pr-16961" || s.TemplateName() != "medusa_template_2_22_0_pr_16961" {
		t.Errorf("variant image %s, template %s", s.ImageName(), s.TemplateName())
	}
	if plain.ImageName() != "oversold-medusa:2.22.0" || plain.TemplateName() != "medusa_template_2_22_0" {
		t.Errorf("plain image %s, template %s", plain.ImageName(), plain.TemplateName())
	}
	if len(s.Inputs) != 64 || len(plain.Inputs) != 64 || s.Inputs == plain.Inputs {
		t.Errorf("inputs plain %q, variant %q", plain.Inputs, s.Inputs)
	}
	env := func(st *Stack) map[string]string {
		out := map[string]string{}
		for _, kv := range st.environ() {
			k, val, _ := strings.Cut(kv, "=")
			out[k] = val
		}
		return out
	}
	t.Setenv("OVERSOLD_BUILD_TARGET", "variant")
	t.Setenv("MEDUSA_IMAGE_TAG", "elsewhere")
	pe, ve := env(plain), env(s)
	if pe["MEDUSA_IMAGE_TAG"] != "2.22.0" || pe["OVERSOLD_BUILD_TARGET"] != "release" || pe["OVERSOLD_VARIANT"] != "" || pe["OVERSOLD_IMAGE_INPUTS"] != plain.Inputs {
		t.Errorf("plain env %v", pe)
	}
	if ve["MEDUSA_IMAGE_TAG"] != "2.22.0-pr-16961" || ve["OVERSOLD_BUILD_TARGET"] != "variant" || ve["OVERSOLD_VARIANT"] != "pr-16961" || ve["OVERSOLD_IMAGE_INPUTS"] != s.Inputs {
		t.Errorf("variant env %v", ve)
	}
	if _, err := New("../..", envfile.Env{}, "2.21.2", v); err == nil || !strings.Contains(err.Error(), "built on medusa 2.22.0") {
		t.Errorf("variant on 2.21.2: err = %v", err)
	}
}

func TestParseSums(t *testing.T) {
	got, err := parseSums("8e150f6b9de7bc64f431e91a803716646a782379d3cc993a31f301056d5ac919  /server/node_modules/a.js\n2af8432c719836364a143d9fe7f4ee6f9aba17062d555e2fdace47315c11c569  /server/node_modules/b.js\n")
	if err != nil || len(got) != 2 || got["/server/node_modules/b.js"] != "2af8432c719836364a143d9fe7f4ee6f9aba17062d555e2fdace47315c11c569" {
		t.Errorf("parseSums = %v, %v", got, err)
	}
	if _, err := parseSums("sha256sum: /x: No such file or directory"); err == nil {
		t.Error("an error line should not parse")
	}
	if cleanLabel("<no value>") != "" || cleanLabel(" abc ") != "abc" {
		t.Error("cleanLabel")
	}
}
