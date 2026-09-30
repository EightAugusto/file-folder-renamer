package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/eightaugusto/file-folder-renamer/internal/atomicfile"
)

func TestOpenCreatesDefaultResourceLayout(t *testing.T) {
	root := filepath.Join(t.TempDir(), "resources")
	layout, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if layout.PatternsRoot != filepath.Join(root, "patterns") {
		t.Fatalf("unexpected patterns root: %q", layout.PatternsRoot)
	}
	data, err := os.ReadFile(filepath.Join(root, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var configuration Config
	if err := json.Unmarshal(data, &configuration); err != nil {
		t.Fatal(err)
	}
	if configuration.PatternsPath != filepath.Join(root, "patterns") || !filepath.IsAbs(configuration.PatternsPath) || !configuration.IncludeFiles || !configuration.IncludeFolders || len(configuration.IgnoredNames) == 0 {
		t.Fatalf("unexpected defaults: %+v", configuration)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	wantKeys := []string{"language", "patterns_path", "include_files", "include_folders", "ignored_names"}
	if len(raw) != len(wantKeys) {
		t.Fatalf("unexpected settings fields: %v", raw)
	}
	for _, key := range wantKeys {
		if _, found := raw[key]; !found {
			t.Fatalf("missing settings field %q", key)
		}
	}

}

func TestDefaultRootUsesHomeAndBuildName(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	if got, want := defaultRootFor(home, filepath.Join("build", "file-folder-renamer")), filepath.Join(home, ".eightaugusto", "file-folder-renamer"); got != want {
		t.Fatalf("unexpected default root: got %q, want %q", got, want)
	}
	if got, want := defaultRootFor(home, filepath.Join("build", "File & Folder Renamer.exe")), filepath.Join(home, ".eightaugusto", "File & Folder Renamer"); got != want {
		t.Fatalf("unexpected Windows default root: got %q, want %q", got, want)
	}
}

func TestOpenUsesConfiguredAbsolutePatternPath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "resources")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	patternsPath := filepath.Join(t.TempDir(), "custom-patterns")
	configuration := DefaultConfig(patternsPath)
	if err := atomicfile.WriteJSON(filepath.Join(root, "config.json"), configuration); err != nil {
		t.Fatal(err)
	}
	layout, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if layout.PatternsRoot != patternsPath {
		t.Fatalf("unexpected configured patterns root: %q", layout.PatternsRoot)
	}
}

func TestOpenDoesNotRewriteExistingConfig(t *testing.T) {
	root := filepath.Join(t.TempDir(), "resources")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	patternsPath := filepath.Join(root, "patterns")
	original := []byte(`{"patterns_path":"` + patternsPath + `","include_files":true,"include_folders":false,"language":"system","ignored_names":[]}`)
	path := filepath.Join(root, "config.json")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(root); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatal("opening existing settings rewrote config.json")
	}
}

func TestSaveRejectsRelativePatternPath(t *testing.T) {
	err := Save(filepath.Join(t.TempDir(), "config.json"), Config{PatternsPath: "patterns", IncludeFiles: true})
	if err == nil {
		t.Fatal("relative patterns_path must be rejected")
	}
}

func TestLoadValidatesWithoutModifyingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "selected.json")
	configuration := DefaultConfig(filepath.Join(t.TempDir(), "patterns"))
	if err := atomicfile.WriteJSON(path, configuration); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.PatternsPath != configuration.PatternsPath {
		t.Fatalf("unexpected loaded config: %+v", loaded)
	}
	if string(after) != string(before) {
		t.Fatal("loading a selected config must not rewrite it")
	}
}

func TestLoadRejectsInvalidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(path, []byte(`{"patterns_path":"relative","include_files":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("invalid selected config must be rejected")
	}
}

func TestOpenRejectsRelativePatternPathInCurrentConfig(t *testing.T) {
	root := filepath.Join(t.TempDir(), "resources")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	current := []byte(`{"patterns_path":"patterns","include_files":true,"include_folders":false}`)
	if err := os.WriteFile(filepath.Join(root, "config.json"), current, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(root); err == nil {
		t.Fatal("current config with relative patterns_path must be rejected")
	}
}

func TestConfigRejectsInvalidOrDuplicateIgnoredNames(t *testing.T) {
	configuration := DefaultConfig(filepath.Join(t.TempDir(), "patterns"))
	configuration.IgnoredNames = []string{".DS_Store", ".ds_store"}
	if err := configuration.Validate(); err == nil {
		t.Fatal("case-insensitive duplicate ignored names must be rejected")
	}
	configuration.IgnoredNames = []string{"folder/file"}
	if err := configuration.Validate(); err == nil {
		t.Fatal("ignored names containing directories must be rejected")
	}
}
