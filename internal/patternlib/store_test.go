package patternlib

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/eightaugusto/file-folder-renamer/internal/pattern"
	"github.com/eightaugusto/file-folder-renamer/internal/rename"
	"github.com/eightaugusto/file-folder-renamer/internal/rules"
)

func TestStoreKeepsBuiltinsInBinaryAndSavesCustomPatternAndTestsSeparately(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "patterns")
	store, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"default.json", "default.tests.json"} {
		if _, err := os.Stat(filepath.Join(directory, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("compiled built-in %s must not be written to resources: %v", name, err)
		}
	}
	if _, err := store.Lookup("default"); err != nil {
		t.Fatalf("compiled built-in must remain available: %v", err)
	}

	spec := pattern.Spec{
		Name: "slug", Rules: []pattern.RuleSpec{
			{Kind: rules.KindCase, Mode: rules.CaseModeLower},
			{Kind: rules.KindReplaceRunes, Remove: []string{"-"}, Replacement: " - ", ExcludeMatches: []string{`\b[0-9]{4}-[0-9]{2}\b`}},
		},
		Tests: []pattern.TestCase{
			{Input: "FILE.TXT", Kind: rename.NodeKindFile, Expected: "file.txt"},
			{Input: "FOLDER", Kind: rename.NodeKindFolder, Expected: "folder"},
		},
	}
	if err := store.Save(spec); err != nil {
		t.Fatal(err)
	}
	patternData, err := os.ReadFile(filepath.Join(directory, "slug.json"))
	if err != nil {
		t.Fatal(err)
	}
	var storedPattern definitionFile
	if err := json.Unmarshal(patternData, &storedPattern); err != nil {
		t.Fatal(err)
	}
	if storedPattern.Version != fileVersion || storedPattern.Name != "slug" {
		t.Fatalf("unexpected versioned pattern JSON: %+v", storedPattern)
	}
	if got, want := storedPattern.Rules[1].ExcludeMatches, []string{`\b[0-9]{4}-[0-9]{2}\b`}; !reflect.DeepEqual(got, want) {
		t.Fatalf("excluded matches were not persisted: got %#v, want %#v", got, want)
	}
	testData, err := os.ReadFile(filepath.Join(directory, "slug.tests.json"))
	if err != nil {
		t.Fatal(err)
	}
	var storedTests testFile
	if err := json.Unmarshal(testData, &storedTests); err != nil {
		t.Fatal(err)
	}
	if storedTests.Version != fileVersion || storedTests.Pattern != "slug" || len(storedTests.Cases) != 2 || storedTests.Cases[1].Kind != rename.NodeKindFolder {
		t.Fatalf("unexpected tests JSON: %+v", storedTests)
	}

	reloaded, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := reloaded.Lookup("slug")
	if err != nil || len(entry.Spec.Tests) != 2 || !reflect.DeepEqual(entry.Spec.Rules[1].ExcludeMatches, []string{`\b[0-9]{4}-[0-9]{2}\b`}) {
		t.Fatalf("saved pattern or cases missing: %+v %v", entry, err)
	}
	if err := reloaded.Delete("slug"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"slug.json", "slug.tests.json"} {
		if _, err := os.Stat(filepath.Join(directory, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected %s to be deleted, got %v", name, err)
		}
	}
}

func TestBuiltinsCannotBeUpdatedOrDeleted(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "patterns"))
	if err != nil {
		t.Fatal(err)
	}
	replacement := pattern.Spec{Name: "default", Rules: []pattern.RuleSpec{{Kind: rules.KindCase, Mode: rules.CaseModeLower}}}
	if err := store.Save(replacement); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("expected read-only save error, got %v", err)
	}
	if err := store.Delete("default"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("expected read-only delete error, got %v", err)
	}
}

func TestCorruptCustomFileReturnsUsableBuiltinsWithoutOverwrite(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "patterns")
	if _, err := Open(directory); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "broken.json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := Open(directory)
	if err == nil || store == nil {
		t.Fatalf("expected usable store and parse error: %v", err)
	}
	if _, lookupErr := store.Lookup("default"); lookupErr != nil {
		t.Fatal(lookupErr)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "{" {
		t.Fatal("corrupt pattern was overwritten")
	}
}

func TestImportCannotReplaceBuiltinButCanCopyIt(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "patterns"))
	if err != nil {
		t.Fatal(err)
	}
	replacement := pattern.Spec{Name: "default", Rules: []pattern.RuleSpec{{Kind: rules.KindCase, Mode: rules.CaseModeLower}}}
	if err := store.Import([]pattern.Spec{replacement}, map[string]ConflictAction{"default": ConflictReplace}); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("expected read-only import error, got %v", err)
	}
	if err := store.Import([]pattern.Spec{replacement}, map[string]ConflictAction{"default": ConflictCopy}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Lookup("default copy 2"); err != nil {
		t.Fatal(err)
	}
}
