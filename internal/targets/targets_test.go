package targets

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRepositoryVersions(t *testing.T) {
	root, err := FindRoot()
	if err != nil {
		t.Fatal(err)
	}
	got, err := Versions(root, "medusa")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "targets", "medusa", "versions"))
	if err != nil {
		t.Fatal(err)
	}
	var folders []string
	for _, e := range entries {
		if e.IsDir() {
			folders = append(folders, e.Name())
		}
	}
	if len(got) != len(folders) || !slices.IsSortedFunc(got, compareVersions) {
		t.Errorf("versions = %v, folders %v", got, folders)
	}
	for _, v := range []string{"2.19.0", "2.21.2", "2.22.0"} {
		if !slices.Contains(got, v) {
			t.Errorf("versions %v are missing %s, which has published results", got, v)
		}
	}
}

const goodPackage = `{
  "version": "2.30.1",
  "dependencies": {"@medusajs/cli": "2.30.1", "@medusajs/medusa": "2.30.1", "pg": "8.16.0"},
  "devDependencies": {"typescript": "5.9.3"}
}`

const goodLock = `{
  "lockfileVersion": 3,
  "packages": {
    "": {
      "version": "2.30.1",
      "dependencies": {"@medusajs/cli": "2.30.1", "@medusajs/medusa": "2.30.1", "pg": "8.16.0"},
      "devDependencies": {"typescript": "5.9.3"}
    },
    "node_modules/@medusajs/cli": {"version": "2.30.1"},
    "node_modules/@medusajs/medusa": {"version": "2.30.1"},
    "node_modules/@medusajs/ui": {"version": "4.9.0"}
  }
}`

func writeVersion(t *testing.T, root, version, pkg, lock string) {
	t.Helper()
	dir := filepath.Join(root, "targets", "medusa", "versions", version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if pkg != "" {
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if lock != "" {
		if err := os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte(lock), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestVersionsReadsFoldersInOrder(t *testing.T) {
	root := t.TempDir()
	for _, v := range []string{"2.30.1", "2.4.0", "2.10.0"} {
		writeVersion(t, root, v, strings.ReplaceAll(goodPackage, "2.30.1", v), strings.ReplaceAll(goodLock, "2.30.1", v))
	}
	if err := os.WriteFile(filepath.Join(root, "targets", "medusa", "versions", ".DS_Store"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Versions(root, "medusa")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"2.4.0", "2.10.0", "2.30.1"}; !slices.Equal(got, want) {
		t.Errorf("versions = %v, want %v", got, want)
	}
}

func TestCheckIsStrict(t *testing.T) {
	tests := []struct {
		name    string
		version string
		pkg     string
		lock    string
		want    string
	}{
		{name: "good", version: "2.30.1", pkg: goodPackage, lock: goodLock},
		{name: "folder name", version: "next", pkg: goodPackage, lock: goodLock, want: "folder name is not a version"},
		{name: "no package.json", version: "2.30.1", lock: goodLock, want: "package.json: missing"},
		{name: "no lockfile", version: "2.30.1", pkg: goodPackage, want: "package-lock.json: missing"},
		{name: "range", version: "2.30.1", pkg: strings.Replace(goodPackage, `"@medusajs/cli": "2.30.1"`, `"@medusajs/cli": "^2.30.1"`, 1), lock: goodLock, want: `@medusajs/cli is "^2.30.1", want exactly 2.30.1`},
		{name: "other version", version: "2.30.1", pkg: strings.Replace(goodPackage, `"@medusajs/cli": "2.30.1"`, `"@medusajs/cli": "2.30.0"`, 1), lock: goodLock, want: `@medusajs/cli is "2.30.0"`},
		{name: "package version", version: "2.30.1", pkg: strings.Replace(goodPackage, `"version": "2.30.1"`, `"version": "2.30.0"`, 1), lock: goodLock, want: `version is "2.30.0"`},
		{name: "no medusa", version: "2.30.1", pkg: strings.Replace(goodPackage, `"@medusajs/medusa": "2.30.1", `, "", 1), lock: goodLock, want: "@medusajs/medusa is not a dependency"},
		{name: "stale lockfile", version: "2.30.1", pkg: strings.Replace(goodPackage, `"pg": "8.16.0"`, `"pg": "8.17.0"`, 1), lock: goodLock, want: "does not match package.json"},
		{name: "lock resolves elsewhere", version: "2.30.1", pkg: goodPackage, lock: strings.Replace(goodLock, `"node_modules/@medusajs/cli": {"version": "2.30.1"}`, `"node_modules/@medusajs/cli": {"version": "2.29.0"}`, 1), want: `@medusajs/cli resolves to "2.29.0"`},
		{name: "old lockfile", version: "2.30.1", pkg: goodPackage, lock: strings.Replace(goodLock, `"lockfileVersion": 3`, `"lockfileVersion": 1`, 1), want: "lockfileVersion 1"},
		{name: "bad json", version: "2.30.1", pkg: "{", lock: goodLock, want: "package.json: unexpected end of JSON input"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeVersion(t, root, tt.version, tt.pkg, tt.lock)
			err := Check(root, "medusa", tt.version)
			if tt.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
			if _, verr := Versions(root, "medusa"); verr == nil {
				t.Error("Versions should refuse a folder that fails Check")
			}
		})
	}
}

func TestCheckRefusesMissingFolderAndEngine(t *testing.T) {
	root := t.TempDir()
	if err := Check(root, "medusa", "2.18.0"); err == nil || !strings.Contains(err.Error(), "medusa 2.18.0 has no folder targets/medusa/versions/2.18.0") {
		t.Errorf("err = %v", err)
	}
	if err := Check(root, "shopware", "6.6.0"); err == nil || !strings.Contains(err.Error(), "unknown engine") {
		t.Errorf("err = %v", err)
	}
	if _, err := Versions(root, "medusa"); err == nil {
		t.Error("a root without a versions folder should be refused")
	}
}
