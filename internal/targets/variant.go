package targets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

const VariantSchema = "oversold.variant/v1"

const MaxVariantDescription = 200

var (
	variantName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	githubLogin = regexp.MustCompile(`^[A-Za-z0-9](-?[A-Za-z0-9]){0,38}$`)
	commitSHA   = regexp.MustCompile(`^[0-9a-f]{40}$`)
	fileSHA     = regexp.MustCompile(`^[0-9a-f]{64}$`)
	pullURL     = regexp.MustCompile(`^https://github\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+/pull/[1-9][0-9]*$`)
)

type Variant struct {
	Schema      string        `yaml:"schema"`
	Name        string        `yaml:"name"`
	BaseVersion string        `yaml:"base_version"`
	Description string        `yaml:"description"`
	Source      VariantSource `yaml:"source"`
	SourceFiles []string      `yaml:"source_files"`
	Files       []VariantFile `yaml:"files"`
}

type VariantSource struct {
	PR      string `yaml:"pr"`
	Author  string `yaml:"author"`
	State   string `yaml:"state"`
	Merged  *bool  `yaml:"merged"`
	HeadSHA string `yaml:"head_sha"`
	BaseSHA string `yaml:"base_sha"`
}

type VariantFile struct {
	Path           string `yaml:"path"`
	ReplacesSHA256 string `yaml:"replaces_sha256"`
	SHA256         string `yaml:"sha256"`
}

func ValidVariantName(name string) bool {
	return variantName.MatchString(name) && len(name) <= 40
}

func VariantDir(root, engine, name string) string {
	return filepath.Join(root, "targets", engine, "variants", name)
}

func Variants(root, engine string) ([]string, error) {
	if _, ok := packageScope[engine]; !ok {
		return nil, fmt.Errorf("unknown engine %q", engine)
	}
	entries, err := os.ReadDir(filepath.Join(root, "targets", engine, "variants"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

func LoadVariant(root, engine, name string) (*Variant, error) {
	v, problems := ReadVariant(root, engine, name)
	if len(problems) > 0 {
		return v, fmt.Errorf("%s: %s", filepath.ToSlash(filepath.Join("targets", engine, "variants", name, "variant.yaml")), strings.Join(problems, "; "))
	}
	return v, nil
}

func ReadVariant(root, engine, name string) (*Variant, []string) {
	if _, ok := packageScope[engine]; !ok {
		return nil, []string{fmt.Sprintf("unknown engine %q", engine)}
	}
	if !ValidVariantName(name) {
		return nil, []string{fmt.Sprintf("variant name %q must be kebab-case, at most 40 characters", name)}
	}
	dir := VariantDir(root, engine, name)
	b, err := os.ReadFile(filepath.Join(dir, "variant.yaml"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, []string{fmt.Sprintf("%s has no variant %s, see targets/%s/variants/", engine, name, engine)}
	}
	if err != nil {
		return nil, []string{err.Error()}
	}
	v := &Variant{}
	var p []string
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	switch err := dec.Decode(v); {
	case errors.Is(err, io.EOF):
		return v, []string{"file is empty"}
	case err != nil:
		var te *yaml.TypeError
		if !errors.As(err, &te) {
			return v, []string{err.Error()}
		}
		p = append(p, te.Errors...)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		p = append(p, "file must hold exactly one YAML document")
	}
	p = append(p, v.validate(root, engine, name)...)
	return v, p
}

func (v *Variant) validate(root, engine, name string) []string {
	var p []string
	add := func(format string, args ...any) { p = append(p, fmt.Sprintf(format, args...)) }

	if v.Schema != VariantSchema {
		add("schema: got %q, want %q", v.Schema, VariantSchema)
	}
	if v.Name != name {
		add("name: got %q, want the folder name %q", v.Name, name)
	}
	if err := Check(root, engine, v.BaseVersion); err != nil {
		add("base_version: %v", err)
	}
	switch d := v.Description; {
	case strings.TrimSpace(d) == "":
		add("description: required")
	case strings.ContainsAny(d, "\r\n") || len(d) > MaxVariantDescription:
		add("description: one line of at most %d characters", MaxVariantDescription)
	case pullURL.MatchString(v.Source.PR) && (!strings.Contains(d, "#"+v.PRNumber()) || !strings.Contains(d, v.Source.Author)):
		add("description: name the pull request as #%s and its author %s", v.PRNumber(), v.Source.Author)
	}

	s := v.Source
	if !pullURL.MatchString(s.PR) {
		add("source.pr: got %q, want a pull request URL like https://github.com/owner/repo/pull/123", s.PR)
	}
	if !githubLogin.MatchString(s.Author) {
		add("source.author: got %q, want a GitHub login", s.Author)
	}
	if s.State != "open" && s.State != "closed" {
		add("source.state: got %q, want open or closed", s.State)
	}
	if s.Merged == nil {
		add("source.merged: required, true or false")
	} else if *s.Merged && s.State != "closed" {
		add("source.merged: a merged pull request is closed")
	}
	if !commitSHA.MatchString(s.HeadSHA) {
		add("source.head_sha: got %q, want a full 40-character commit SHA", s.HeadSHA)
	}
	if !commitSHA.MatchString(s.BaseSHA) {
		add("source.base_sha: got %q, want a full 40-character commit SHA", s.BaseSHA)
	}

	if len(v.SourceFiles) == 0 {
		add("source_files: at least one file is required")
	}
	seenSource := map[string]bool{}
	for i, f := range v.SourceFiles {
		switch {
		case !cleanRelative(f) || !strings.HasPrefix(f, "packages/") || !strings.Contains(f, "/src/") || path.Ext(f) != ".ts":
			add("source_files[%d]: %q must be a .ts file under packages/*/src/", i, f)
		case isTestPath(f):
			add("source_files[%d]: %q is a test file, only non-test source files are applied", i, f)
		case seenSource[f]:
			add("source_files[%d]: %q is listed twice", i, f)
		}
		seenSource[f] = true
	}

	if len(v.Files) == 0 {
		add("files: at least one file is required")
	}
	listed := map[string]bool{}
	for i, f := range v.Files {
		if !cleanRelative(f.Path) || !strings.HasPrefix(f.Path, "@medusajs/") || !strings.Contains(f.Path, "/dist/") || path.Ext(f.Path) != ".js" {
			add("files[%d].path: %q must be a .js file under @medusajs/<package>/dist/", i, f.Path)
			continue
		}
		if listed[f.Path] {
			add("files[%d].path: %q is listed twice", i, f.Path)
		}
		listed[f.Path] = true
		if !fileSHA.MatchString(f.ReplacesSHA256) {
			add("files[%d].replaces_sha256: want 64 lowercase hex characters", i)
		}
		if !fileSHA.MatchString(f.SHA256) {
			add("files[%d].sha256: want 64 lowercase hex characters", i)
			continue
		}
		got, err := fileSum(filepath.Join(VariantDir(root, engine, name), "files", filepath.FromSlash(f.Path)))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			add("files[%d]: files/%s is missing", i, f.Path)
		case err != nil:
			add("files[%d]: %v", i, err)
		case got != f.SHA256:
			add("files[%d]: files/%s has sha256 %s, variant.yaml says %s", i, f.Path, got, f.SHA256)
		}
	}

	dir := VariantDir(root, engine, name)
	entries, err := os.ReadDir(dir)
	if err != nil {
		add("%v", err)
		return p
	}
	for _, e := range entries {
		if e.Name() != "variant.yaml" && e.Name() != "files" {
			add("%s: only variant.yaml and files/ belong in a variant folder", e.Name())
		}
	}
	filesDir := filepath.Join(dir, "files")
	_ = filepath.WalkDir(filesDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(filesDir, p)
		if !listed[filepath.ToSlash(rel)] {
			add("files/%s is not listed in files", filepath.ToSlash(rel))
		}
		return nil
	})
	return p
}

func (v *Variant) Tag() string {
	return v.BaseVersion + "-" + v.Name
}

func (v *Variant) PRNumber() string {
	return v.Source.PR[strings.LastIndex(v.Source.PR, "/")+1:]
}

func (v *Variant) PRRepo() string {
	rest := strings.TrimPrefix(v.Source.PR, "https://github.com/")
	return rest[:strings.Index(rest, "/pull/")]
}

func cleanRelative(p string) bool {
	return p != "" && path.Clean(p) == p && !path.IsAbs(p) && !strings.HasPrefix(p, "../") && p != ".." && !strings.Contains(p, "\\")
}

func isTestPath(p string) bool {
	for _, part := range strings.Split(p, "/") {
		if part == "__tests__" || part == "__fixtures__" || part == "__mocks__" || part == "integration-tests" {
			return true
		}
	}
	base := path.Base(p)
	return strings.Contains(base, ".spec.") || strings.Contains(base, ".test.")
}

func fileSum(p string) (string, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func InputsHash(root, engine, version, variant string) (string, []string, error) {
	base := filepath.Join(root, "targets", engine)
	parts := []string{"Dockerfile", ".dockerignore", "app", filepath.Join("versions", version)}
	if variant != "" {
		parts = append(parts, filepath.Join("variants", variant))
	}
	var lines []string
	for _, part := range parts {
		start := filepath.Join(base, part)
		info, err := os.Stat(start)
		if err != nil {
			return "", nil, err
		}
		if !info.IsDir() {
			sum, err := fileSum(start)
			if err != nil {
				return "", nil, err
			}
			lines = append(lines, sum+"  "+filepath.ToSlash(part))
			continue
		}
		err = filepath.WalkDir(start, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := d.Name()
			if d.IsDir() {
				if name == "node_modules" || name == ".medusa" {
					return filepath.SkipDir
				}
				return nil
			}
			if name == ".env" || !d.Type().IsRegular() {
				return nil
			}
			rel, err := filepath.Rel(base, p)
			if err != nil {
				return err
			}
			sum, err := fileSum(p)
			if err != nil {
				return err
			}
			lines = append(lines, sum+"  "+filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			return "", nil, err
		}
	}
	slices.SortFunc(lines, func(a, b string) int { return strings.Compare(a[66:], b[66:]) })
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n") + "\n"))
	return hex.EncodeToString(sum[:]), lines, nil
}
