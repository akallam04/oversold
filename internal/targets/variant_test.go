package targets

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShippedVariant(t *testing.T) {
	root, err := FindRoot()
	if err != nil {
		t.Fatal(err)
	}
	names, err := Variants(root, "medusa")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatal("no variants")
	}
	for _, name := range names {
		v, err := LoadVariant(root, "medusa", name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if v != nil && !strings.Contains(v.Description, v.Source.Author) {
			t.Errorf("%s: description %q does not credit %s", name, v.Description, v.Source.Author)
		}
	}
	v, err := LoadVariant(root, "medusa", "pr-16961")
	if err != nil {
		t.Fatal(err)
	}
	if v.Tag() != "2.22.0-pr-16961" || v.PRNumber() != "16961" || v.PRRepo() != "medusajs/medusa" || v.Source.Author != "pqa-labs" || v.Source.Merged == nil || *v.Source.Merged {
		t.Errorf("pr-16961 = %+v", v)
	}
}

const variantFile = "built.js"

func writeVariant(t *testing.T, yaml string) string {
	t.Helper()
	root := t.TempDir()
	writeVersion(t, root, "2.30.1", goodPackage, goodLock)
	dir := filepath.Join(root, "targets", "medusa", "variants", "pr-7")
	if err := os.MkdirAll(filepath.Join(dir, "files", "@medusajs", "cart", "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "files", "@medusajs", "cart", "dist", "a.js"), []byte(variantFile), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "variant.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func goodVariantYAML() string {
	sum := sha256.Sum256([]byte(variantFile))
	return `schema: oversold.variant/v1
name: pr-7
base_version: 2.30.1
description: "Medusa 2.30.1 with medusajs/medusa#7 by someone"
source:
  pr: https://github.com/medusajs/medusa/pull/7
  author: someone
  state: open
  merged: false
  head_sha: 1111111111111111111111111111111111111111
  base_sha: 2222222222222222222222222222222222222222
source_files:
  - packages/modules/cart/src/a.ts
files:
  - path: "@medusajs/cart/dist/a.js"
    replaces_sha256: 3333333333333333333333333333333333333333333333333333333333333333
    sha256: ` + hex.EncodeToString(sum[:]) + "\n"
}

func TestVariantIsStrict(t *testing.T) {
	good := goodVariantYAML()
	if _, err := LoadVariant(writeVariant(t, good), "medusa", "pr-7"); err != nil {
		t.Fatalf("good variant: %v", err)
	}
	tests := []struct {
		name, yaml, want string
	}{
		{"unknown field", good + "notes: x\n", "field notes not found"},
		{"schema", strings.Replace(good, "oversold.variant/v1", "oversold.variant/v2", 1), `schema: got "oversold.variant/v2"`},
		{"name", strings.Replace(good, "name: pr-7", "name: pr-8", 1), `name: got "pr-8", want the folder name "pr-7"`},
		{"base version", strings.Replace(good, "base_version: 2.30.1", "base_version: 2.30.2", 1), "base_version: medusa 2.30.2 has no folder"},
		{"two line description", strings.Replace(good, `description: "Medusa 2.30.1 with medusajs/medusa#7 by someone"`, "description: >\n  Medusa 2.30.1\n", 1), "description: one line"},
		{"pr url", strings.Replace(good, "https://github.com/medusajs/medusa/pull/7", "https://example.com/pull/7", 1), "source.pr: got"},
		{"author", strings.Replace(good, "author: someone", "author: some one", 1), "source.author: got"},
		{"state", strings.Replace(good, "state: open", "state: draft", 1), `source.state: got "draft"`},
		{"merged missing", strings.Replace(good, "  merged: false\n", "", 1), "source.merged: required"},
		{"merged but open", strings.Replace(good, "merged: false", "merged: true", 1), "a merged pull request is closed"},
		{"short head", strings.Replace(good, "head_sha: 1111111111111111111111111111111111111111", "head_sha: 1111111", 1), "source.head_sha: got"},
		{"test source", strings.Replace(good, "packages/modules/cart/src/a.ts", "packages/modules/cart/src/__tests__/a.spec.ts", 1), "is a test file"},
		{"source outside src", strings.Replace(good, "packages/modules/cart/src/a.ts", "packages/modules/cart/a.ts", 1), "must be a .ts file under packages/*/src/"},
		{"file path", strings.Replace(good, `"@medusajs/cart/dist/a.js"`, `"../cart/dist/a.js"`, 1), "must be a .js file under @medusajs/<package>/dist/"},
		{"bad sha", strings.Replace(good, "replaces_sha256: 3333333333333333333333333333333333333333333333333333333333333333", "replaces_sha256: abc", 1), "files[0].replaces_sha256: want 64"},
		{"wrong sha", strings.Replace(good, "sha256: "+good[strings.LastIndex(good, " ")+1:len(good)-1], "sha256: "+strings.Repeat("4", 64), 1), "variant.yaml says 4444"},
		{"two documents", good + "---\nname: x\n", "exactly one YAML document"},
		{"no credit", strings.Replace(good, " by someone", "", 1), "description: name the pull request as #7 and its author someone"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadVariant(writeVariant(t, tt.yaml), "medusa", "pr-7")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want %q", err, tt.want)
			}
		})
	}

	root := writeVariant(t, good)
	extra := filepath.Join(root, "targets", "medusa", "variants", "pr-7", "files", "@medusajs", "cart", "dist", "b.js")
	if err := os.WriteFile(extra, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadVariant(root, "medusa", "pr-7"); err == nil || !strings.Contains(err.Error(), "files/@medusajs/cart/dist/b.js is not listed") {
		t.Errorf("unlisted file: err = %v", err)
	}
	if _, err := LoadVariant(root, "medusa", "pr-9"); err == nil || !strings.Contains(err.Error(), "has no variant pr-9") {
		t.Errorf("missing variant: err = %v", err)
	}
	if _, err := LoadVariant(root, "medusa", "../pr-7"); err == nil || !strings.Contains(err.Error(), "must be kebab-case") {
		t.Errorf("bad name: err = %v", err)
	}
}

func TestInputsHash(t *testing.T) {
	root := writeVariant(t, goodVariantYAML())
	base := filepath.Join(root, "targets", "medusa")
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(base, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Dockerfile", "FROM node\n")
	write(".dockerignore", "**/node_modules\n")
	write("app/medusa-config.ts", "export {}\n")
	write("app/src/scripts/seed.ts", "seed\n")
	write("db/create-database.sql", "create database medusa;\n")

	plain, lines, err := InputsHash(root, "medusa", "2.30.1", "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".dockerignore", "Dockerfile", "app/medusa-config.ts", "app/src/scripts/seed.ts", "versions/2.30.1/package-lock.json", "versions/2.30.1/package.json"}
	if len(lines) != len(want) {
		t.Fatalf("lines = %q", lines)
	}
	for i, w := range want {
		if !strings.HasSuffix(lines[i], "  "+w) {
			t.Errorf("line %d = %q, want %s", i, lines[i], w)
		}
	}
	again, _, _ := InputsHash(root, "medusa", "2.30.1", "")
	if again != plain || len(plain) != 64 {
		t.Errorf("hash %q then %q", plain, again)
	}

	write("app/node_modules/x/index.js", "ignored\n")
	write("app/.medusa/server/index.js", "ignored\n")
	write("app/.env", "SECRET=1\n")
	write("db/create-database.sql", "changed\n")
	if h, _, _ := InputsHash(root, "medusa", "2.30.1", ""); h != plain {
		t.Error("files the image build ignores changed the hash")
	}

	withVariant, lines, err := InputsHash(root, "medusa", "2.30.1", "pr-7")
	if err != nil {
		t.Fatal(err)
	}
	if withVariant == plain || len(lines) != len(want)+2 {
		t.Errorf("variant hash %q, lines %q", withVariant, lines)
	}

	write("app/medusa-config.ts", "export default {}\n")
	if h, _, _ := InputsHash(root, "medusa", "2.30.1", ""); h == plain {
		t.Error("a changed app file kept the hash")
	}
}
