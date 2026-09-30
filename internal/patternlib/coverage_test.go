package patternlib

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/eightaugusto/file-folder-renamer/internal/pattern"
	"github.com/eightaugusto/file-folder-renamer/internal/rules"
)

func validBundle() Bundle {
	return Bundle{Pattern: "x", Files: true, Patterns: []pattern.Spec{{Name: "x"}}}
}

func TestBundleEveryEncodingAndValidationBranch(t *testing.T) {
	configuration := validBundle()
	var output bytes.Buffer
	if err := EncodeBundle(&output, configuration); err != nil {
		t.Fatal(err)
	}
	if decoded, err := DecodeBundle(bytes.NewReader(output.Bytes())); err != nil || decoded.Pattern != "x" {
		t.Fatalf("decode: %+v %v", decoded, err)
	}
	for _, data := range []string{`{`, `{"pattern":"x"}`, `{"pattern":"","files":true}`} {
		if _, err := DecodeBundle(strings.NewReader(data)); err == nil {
			t.Fatalf("invalid reader accepted: %s", data)
		}
	}
	if err := EncodeBundle(&output, Bundle{}); err == nil {
		t.Fatal("invalid bundle encoded")
	}
}

func TestStoreQueriesCopiesAndSaveAsBranches(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defaultPattern, err := store.Lookup("default")
	if err != nil || !defaultPattern.BuiltIn || !store.Has("default") {
		t.Fatal("store metadata mismatch")
	}
	if _, err := store.Lookup("missing"); err == nil {
		t.Fatal("missing lookup succeeded")
	}
	spec := pattern.Spec{Name: "custom", Rules: []pattern.RuleSpec{{Kind: rules.KindCase, Mode: rules.CaseModeLower}}}
	if err := store.Save(spec); err != nil {
		t.Fatal(err)
	}
	if !store.Has("custom") || len(store.CustomSpecs()) != 1 || len(store.Entries()) < 2 {
		t.Fatal("saved entry missing")
	}
	entries := store.Entries()
	entries[len(entries)-1].Spec.Name = "mutated"
	if _, err := store.Lookup("custom"); err != nil {
		t.Fatal("entry copy mutated store")
	}
	if err := store.SaveAs("custom", pattern.Spec{Name: "renamed", Rules: spec.Rules}); err != nil {
		t.Fatal(err)
	}
	if store.Has("custom") || !store.Has("renamed") {
		t.Fatal("rename did not update maps")
	}
	for _, name := range []string{"custom.json", "custom.tests.json"} {
		if _, err := os.Stat(filepath.Join(root, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("obsolete file remains: %s", name)
		}
	}
	if err := store.SaveAs("renamed", pattern.Spec{}); err == nil {
		t.Fatal("invalid save accepted")
	}
	if err := store.SaveAs("default", spec); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("built-in previous name: %v", err)
	}
	if err := store.Save(pattern.Spec{Name: "default"}); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("built-in target: %v", err)
	}
	if err := store.Save(pattern.Spec{Name: "other", Rules: spec.Rules}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAs("renamed", pattern.Spec{Name: "other", Rules: spec.Rules}); err == nil {
		t.Fatal("duplicate target accepted")
	}
}

func TestOpenInjectedAndSavedFileBranches(t *testing.T) {
	sentinel := errors.New("failure")
	if _, err := openStore("/patterns", func() ([]pattern.Spec, error) { return nil, sentinel }, os.MkdirAll, os.ReadDir); !errors.Is(err, sentinel) {
		t.Fatalf("embedded: %v", err)
	}
	loader := func() ([]pattern.Spec, error) { return []pattern.Spec{{Name: "default"}}, nil }
	if _, err := openStore("/patterns", loader, func(string, os.FileMode) error { return sentinel }, os.ReadDir); !errors.Is(err, sentinel) {
		t.Fatalf("mkdir: %v", err)
	}
	if _, err := openStore("/patterns", loader, func(string, os.FileMode) error { return nil }, func(string) ([]os.DirEntry, error) { return nil, sentinel }); !errors.Is(err, sentinel) {
		t.Fatalf("read dir: %v", err)
	}

	root := t.TempDir()
	files := map[string]string{
		"default.json":         `{"version":1,"name":"default","rules":[]}`,
		"bad-tests.json":       `{"version":1,"name":"bad tests","rules":[]}`,
		"bad-tests.tests.json": `{`,
		"invalid-rule.json":    `{"version":1,"name":"invalid rule","rules":[{"kind":"case"}]}`,
		"ignored.txt":          `ignored`,
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(root, "directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := Open(root)
	if store == nil || err == nil {
		t.Fatalf("expected usable store with load errors: %v", err)
	}
}

func TestStoreDeleteAndImportRemainingBranches(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("missing"); err == nil {
		t.Fatal("missing delete accepted")
	}
	base := pattern.Spec{Name: "one", Rules: []pattern.RuleSpec{{Kind: rules.KindCase, Mode: rules.CaseModeLower}}}
	if err := store.Import([]pattern.Spec{base}, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.Import([]pattern.Spec{base}, map[string]ConflictAction{"one": ConflictKeep}); err != nil {
		t.Fatal(err)
	}
	if err := store.Import([]pattern.Spec{base}, map[string]ConflictAction{"one": ConflictCopy}); err != nil {
		t.Fatal(err)
	}
	if !store.Has("one copy 2") {
		t.Fatal("copy conflict missing")
	}
	replacement := base.Clone()
	replacement.Rules[0].Mode = rules.CaseModeUpper
	if err := store.Import([]pattern.Spec{replacement}, map[string]ConflictAction{"one": ConflictReplace}); err != nil {
		t.Fatal(err)
	}
	entry, _ := store.Lookup("one")
	if entry.Spec.Rules[0].Mode != rules.CaseModeUpper {
		t.Fatal("replacement missing")
	}
	if err := store.Import([]pattern.Spec{base}, map[string]ConflictAction{}); err == nil {
		t.Fatal("missing decision accepted")
	}
	if err := store.Import([]pattern.Spec{{}}, nil); err == nil {
		t.Fatal("invalid import accepted")
	}
	builtin := pattern.Spec{Name: "default"}
	if err := store.Import([]pattern.Spec{builtin}, map[string]ConflictAction{"default": ConflictReplace}); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("built-in replacement: %v", err)
	}
	if err := store.Delete("one"); err != nil {
		t.Fatal(err)
	}
	blockedPath := filepath.Join(store.directory, "blocked.json")
	if err := os.Mkdir(blockedPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blockedPath, "child"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	blocked := pattern.Spec{Name: "blocked", Rules: base.Rules}
	if err := store.Import([]pattern.Spec{blocked}, nil); err == nil {
		t.Fatal("import persistence failure accepted")
	}
}

func TestPatternFileHelpersRemainingBranches(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "example.json")
	spec := pattern.Spec{Name: "example", Rules: []pattern.RuleSpec{{Kind: rules.KindCase, Mode: rules.CaseModeLower}}}
	if err := writePatternFilesReplacing(path, spec, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := readPatternFile(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing pattern read")
	}
	if err := os.WriteFile(path, []byte(`{"version":2,"name":"example"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readPatternFile(path); err == nil {
		t.Fatal("unsupported pattern version accepted")
	}
	testsPath := testPath(path)
	if err := os.WriteFile(testsPath, []byte(`{`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readTestFile(testsPath, "example"); err == nil {
		t.Fatal("bad test JSON accepted")
	}
	if _, err := readTestFile(filepath.Join(root, "missing.tests.json"), "example"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing tests: %v", err)
	}
	if err := os.WriteFile(testsPath, []byte(`{"version":2,"pattern":"example"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readTestFile(testsPath, "example"); err == nil {
		t.Fatal("unsupported test version accepted")
	}
	if err := os.WriteFile(testsPath, []byte(`{"version":1,"pattern":"other"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readTestFile(testsPath, "example"); err == nil {
		t.Fatal("mismatched tests accepted")
	}
	if err := removePatternFiles(path); err != nil {
		t.Fatal(err)
	}
	if err := removePatternFiles(path); err != nil {
		t.Fatal("missing removals should succeed")
	}
	if patternFileName(" --- ") != "pattern.json" {
		t.Fatal("empty sanitized filename fallback")
	}
	if got := patternFileName("A / B"); got != "a-b.json" {
		t.Fatalf("sanitized filename: %q", got)
	}
	if !reflect.DeepEqual(cloneSpec(spec), spec) {
		t.Fatal("clone mismatch")
	}
}

func TestStoreFilenameCollisionValidationAndRemovalErrors(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	rule := []pattern.RuleSpec{{Kind: rules.KindCase, Mode: rules.CaseModeLower}}
	if err := store.Save(pattern.Spec{Name: "a b", Rules: rule}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(pattern.Spec{Name: "a-b", Rules: rule}); err != nil {
		t.Fatal(err)
	}
	if store.customFiles[key("a b")] == store.customFiles[key("a-b")] {
		t.Fatal("filename collision not resolved")
	}
	if len(store.CustomSpecs()) != 2 {
		t.Fatal("custom specs missing")
	}
	if err := validateSpec(pattern.Spec{}); err == nil {
		t.Fatal("invalid spec accepted")
	}
	if err := validateSpec(pattern.Spec{Name: "x", Tests: []pattern.TestCase{{Input: "x", Kind: "device"}}}); err == nil {
		t.Fatal("invalid test accepted")
	}
	if err := validateSpec(pattern.Spec{Name: "x", Tests: []pattern.TestCase{{Input: "x", Kind: "file", Expected: "different"}}}); err == nil {
		t.Fatal("failing test accepted")
	}

	blocked := pattern.Spec{Name: "blocked", Rules: rule}
	if err := os.Mkdir(filepath.Join(root, "blocked.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(blocked); err == nil {
		t.Fatal("blocked write accepted")
	}
	if err := store.Save(pattern.Spec{Name: "remove", Rules: rule}); err != nil {
		t.Fatal(err)
	}
	removePath := filepath.Join(root, store.customFiles[key("remove")])
	if err := os.Remove(removePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(removePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(removePath, "child"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("remove"); err == nil {
		t.Fatal("remove failure accepted")
	}
}

func TestStoreNeverReusesHashedCollisionFilename(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	rule := []pattern.RuleSpec{{Kind: rules.KindCase, Mode: rules.CaseModeLower}}
	digest := sha256.Sum256([]byte(key("a-b")))
	occupier := "a-b-" + hex.EncodeToString(digest[:4])
	for _, name := range []string{occupier, "a b", "a-b"} {
		if err := store.Save(pattern.Spec{Name: name, Rules: rule}); err != nil {
			t.Fatalf("save %q: %v", name, err)
		}
	}
	seen := make(map[string]struct{})
	for _, fileName := range store.customFiles {
		if _, duplicate := seen[fileName]; duplicate {
			t.Fatalf("custom patterns share %q", fileName)
		}
		seen[fileName] = struct{}{}
	}
	if len(seen) != 3 {
		t.Fatalf("got %d custom files, want 3", len(seen))
	}
	reloaded, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{occupier, "a b", "a-b"} {
		if !reloaded.Has(name) {
			t.Fatalf("reloaded store lost %q", name)
		}
	}
}
