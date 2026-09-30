package settings

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/eightaugusto/file-folder-renamer/internal/localization"
)

func TestDefaultRootAndBootstrapInjectedBranches(t *testing.T) {
	sentinel := errors.New("failure")
	if _, err := resolveDefaultRoot(func() (string, error) { return "", sentinel }, func() (string, error) { return "", nil }); !errors.Is(err, sentinel) {
		t.Fatalf("home: %v", err)
	}
	if _, err := resolveDefaultRoot(func() (string, error) { return "/home", nil }, func() (string, error) { return "", sentinel }); !errors.Is(err, sentinel) {
		t.Fatalf("executable: %v", err)
	}
	if root, err := resolveDefaultRoot(func() (string, error) { return "/home", nil }, func() (string, error) { return "/bin/app", nil }); err != nil || root != "/home/.eightaugusto/app" {
		t.Fatalf("root: %q %v", root, err)
	}
	if _, err := DefaultRoot(); err != nil {
		t.Fatal(err)
	}
	if _, err := bootstrap(func() (string, error) { return "", sentinel }, Open); !errors.Is(err, sentinel) {
		t.Fatalf("bootstrap root: %v", err)
	}
	called := false
	if _, err := bootstrap(func() (string, error) { return "/root", nil }, func(root string) (Layout, error) { called = root == "/root"; return Layout{}, sentinel }); !called || !errors.Is(err, sentinel) {
		t.Fatalf("bootstrap open: %v", err)
	}
	previousRoot, previousOpen := bootstrapDefaultRoot, bootstrapOpen
	t.Cleanup(func() { bootstrapDefaultRoot, bootstrapOpen = previousRoot, previousOpen })
	bootstrapDefaultRoot = func() (string, error) { return "/root", nil }
	bootstrapOpen = func(string) (Layout, error) { return Layout{}, sentinel }
	if _, err := Bootstrap(); !errors.Is(err, sentinel) {
		t.Fatalf("Bootstrap: %v", err)
	}
	if got := defaultRootFor("/home", "/"); got != "/home/.eightaugusto/file-folder-renamer" {
		t.Fatalf("fallback root: %q", got)
	}
}

func TestOpenInjectedOperationFailures(t *testing.T) {
	sentinel := errors.New("failure")
	valid := DefaultConfig("/patterns")
	base := settingsOperations{
		abs:      func(path string) (string, error) { return path, nil },
		mkdirAll: func(string, os.FileMode) error { return nil },
		stat:     func(string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		load:     func(string, string) (Config, error) { return valid, nil },
		write:    func(string, any) error { return nil },
	}
	mutations := []func(*settingsOperations){
		func(ops *settingsOperations) { ops.abs = func(string) (string, error) { return "", sentinel } },
		func(ops *settingsOperations) { ops.mkdirAll = func(string, os.FileMode) error { return sentinel } },
		func(ops *settingsOperations) { ops.stat = func(string) (os.FileInfo, error) { return nil, sentinel } },
		func(ops *settingsOperations) {
			ops.load = func(string, string) (Config, error) { return Config{}, sentinel }
		},
		func(ops *settingsOperations) { ops.write = func(string, any) error { return sentinel } },
		func(ops *settingsOperations) {
			calls := 0
			ops.mkdirAll = func(string, os.FileMode) error {
				calls++
				if calls == 2 {
					return sentinel
				}
				return nil
			}
		},
	}
	for _, mutate := range mutations {
		ops := base
		mutate(&ops)
		if _, err := openWithOperations("/root", ops); !errors.Is(err, sentinel) {
			t.Fatalf("operation failure: %v", err)
		}
	}
}

func TestLoadForRootRemainingBranches(t *testing.T) {
	root := t.TempDir()
	if _, err := loadForRoot(root, root); err == nil {
		t.Fatal("directory read accepted")
	}
	path := filepath.Join(root, "config.json")
	if err := os.WriteFile(path, []byte(`null`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadForRoot(path, root); err == nil {
		t.Fatal("null config accepted")
	}
	patterns := filepath.Join(root, "patterns")
	data := []byte(`{"patterns_path":"` + patterns + `","include_files":true,"ignored_names":[]}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	configuration, err := loadForRoot(path, root)
	if err != nil || configuration.Language != localization.PreferenceSystem {
		t.Fatalf("language default: %+v %v", configuration, err)
	}
}

func TestConfigurationRemainingFailureBranches(t *testing.T) {
	root := t.TempDir()
	patterns := filepath.Join(root, "patterns")
	valid := DefaultConfig(patterns)
	invalid := []Config{
		{Language: localization.Preference("bad"), PatternsPath: patterns, IncludeFiles: true},
		{PatternsPath: "", IncludeFiles: true},
		{PatternsPath: "relative", IncludeFiles: true},
		{PatternsPath: patterns},
		{PatternsPath: patterns, IncludeFiles: true, IgnoredNames: []string{""}},
		{PatternsPath: patterns, IncludeFiles: true, IgnoredNames: []string{"dir/name"}},
		{PatternsPath: patterns, IncludeFiles: true, IgnoredNames: []string{"["}},
		{PatternsPath: patterns, IncludeFiles: true, IgnoredNames: []string{"A", "a"}},
	}
	for _, configuration := range invalid {
		if err := configuration.Validate(); err == nil {
			t.Fatalf("invalid config accepted: %+v", configuration)
		}
	}
	valid.Language = ""
	if err := Save(filepath.Join(root, "saved", "config.json"), valid); err != nil {
		t.Fatal(err)
	}
	if err := Save(filepath.Join(root, "bad.json"), Config{}); err == nil {
		t.Fatal("invalid save accepted")
	}
	parentFile := filepath.Join(root, "parent")
	if err := os.WriteFile(parentFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Save(filepath.Join(parentFile, "config.json"), DefaultConfig(patterns)); err == nil {
		t.Fatal("save mkdir failure accepted")
	}
	if _, err := Load(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing load accepted")
	}
	badJSON := filepath.Join(root, "bad-json")
	if err := os.WriteFile(badJSON, []byte(`{`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(badJSON); err == nil {
		t.Fatal("invalid JSON accepted")
	}
	invalidJSON := filepath.Join(root, "invalid-config")
	if err := os.WriteFile(invalidJSON, []byte(`{"patterns_path":"relative","include_files":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(invalidJSON); err == nil {
		t.Fatal("invalid loaded config accepted")
	}
}

func TestOpenRemainingFilesystemBranches(t *testing.T) {
	rootFile := filepath.Join(t.TempDir(), "root-file")
	if err := os.WriteFile(rootFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(rootFile); err == nil {
		t.Fatal("file root accepted")
	}
	root := t.TempDir()
	config := filepath.Join(root, "config.json")
	if err := os.WriteFile(config, []byte(`{`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(root); err == nil {
		t.Fatal("invalid config accepted")
	}
	patternsFile := filepath.Join(root, "patterns-file")
	if err := os.WriteFile(patternsFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"patterns_path":"` + patternsFile + `","include_files":true,"include_folders":false,"ignored_names":[]}`)
	if err := os.WriteFile(config, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(root); err == nil {
		t.Fatal("patterns file accepted as directory")
	}
}
