package targets

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
)

var versionName = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

var packageScope = map[string]string{"medusa": "@medusajs/"}

const (
	LockingInMemory = "in-memory"
	LockingRedis    = "redis"
)

var LockingModes = []string{LockingInMemory, LockingRedis}

func Engines() []string {
	return slices.Sorted(maps.Keys(packageScope))
}

func FindRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if isRoot(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no compose.yaml with a targets folder in the working directory or above it, run oversold from the repository root")
		}
		dir = parent
	}
}

func isRoot(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "compose.yaml")); err != nil {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, "targets"))
	return err == nil && info.IsDir()
}

type cached struct {
	versions []string
	err      error
}

var (
	cacheMu sync.Mutex
	cache   = map[string]cached{}
)

func Versions(root, engine string) ([]string, error) {
	key := root + "\x00" + engine
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if c, ok := cache[key]; ok {
		return slices.Clone(c.versions), c.err
	}
	versions, err := readVersions(root, engine)
	cache[key] = cached{versions: versions, err: err}
	return slices.Clone(versions), err
}

func readVersions(root, engine string) ([]string, error) {
	if _, ok := packageScope[engine]; !ok {
		return nil, fmt.Errorf("unknown engine %q", engine)
	}
	dir := filepath.Join(root, "targets", engine, "versions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %s versions: %w", engine, err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if err := Check(root, engine, e.Name()); err != nil {
			return nil, err
		}
		out = append(out, e.Name())
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no %s versions under %s", engine, filepath.ToSlash(filepath.Join("targets", engine, "versions")))
	}
	slices.SortFunc(out, compareVersions)
	return out, nil
}

func compareVersions(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := range pa {
		x, _ := strconv.Atoi(pa[i])
		y, _ := strconv.Atoi(pb[i])
		if x != y {
			return x - y
		}
	}
	return 0
}

type packageJSON struct {
	Version         string            `json:"version"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

type lockEntry struct {
	Version         string            `json:"version"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

type lockfile struct {
	LockfileVersion int                  `json:"lockfileVersion"`
	Packages        map[string]lockEntry `json:"packages"`
}

func Check(root, engine, version string) error {
	scope, ok := packageScope[engine]
	if !ok {
		return fmt.Errorf("unknown engine %q", engine)
	}
	rel := filepath.ToSlash(filepath.Join("targets", engine, "versions", version))
	if !versionName.MatchString(version) {
		return fmt.Errorf("%s: folder name is not a version like 2.21.2", rel)
	}
	dir := filepath.Join(root, "targets", engine, "versions", version)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("%s %s has no folder %s", engine, version, rel)
	}

	var pkg packageJSON
	if err := readJSON(filepath.Join(dir, "package.json"), &pkg); err != nil {
		return fmt.Errorf("%s/package.json: %w", rel, err)
	}
	if pkg.Version != version {
		return fmt.Errorf("%s/package.json: version is %q, want %q", rel, pkg.Version, version)
	}
	var pinned []string
	for _, deps := range []map[string]string{pkg.Dependencies, pkg.DevDependencies} {
		for _, name := range slices.Sorted(maps.Keys(deps)) {
			if !strings.HasPrefix(name, scope) {
				continue
			}
			if deps[name] != version {
				return fmt.Errorf("%s/package.json: %s is %q, want exactly %s", rel, name, deps[name], version)
			}
			pinned = append(pinned, name)
		}
	}
	if core := scope + engine; !slices.Contains(pinned, core) {
		return fmt.Errorf("%s/package.json: %s is not a dependency", rel, core)
	}

	var lock lockfile
	if err := readJSON(filepath.Join(dir, "package-lock.json"), &lock); err != nil {
		return fmt.Errorf("%s/package-lock.json: %w", rel, err)
	}
	if lock.LockfileVersion < 2 {
		return fmt.Errorf("%s/package-lock.json: lockfileVersion %d, want 2 or 3", rel, lock.LockfileVersion)
	}
	top, ok := lock.Packages[""]
	if !ok || !maps.Equal(top.Dependencies, pkg.Dependencies) || !maps.Equal(top.DevDependencies, pkg.DevDependencies) {
		return fmt.Errorf("%s/package-lock.json: does not match package.json, regenerate it", rel)
	}
	for _, name := range pinned {
		got := lock.Packages["node_modules/"+name].Version
		if got != version {
			return fmt.Errorf("%s/package-lock.json: %s resolves to %q, want %s", rel, name, got, version)
		}
	}
	return nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return errors.New("missing")
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
