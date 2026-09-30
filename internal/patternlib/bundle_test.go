package patternlib

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadJSONConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{"pattern":"default","files":true,"folders":false,"patterns":[{"name":"slug","rules":[{"kind":"case","mode":"lower","by_word":true}]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(configPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	configuration, err := DecodeBundle(file)
	if err != nil {
		t.Fatal(err)
	}
	if configuration.Pattern != "default" || !configuration.Files || configuration.Folders {
		t.Fatalf("unexpected config: %+v", configuration)
	}
	if len(configuration.Patterns) != 1 {
		t.Fatalf("expected custom patterns to be loaded, got %d", len(configuration.Patterns))
	}
	if !configuration.Patterns[0].Rules[0].ByWord {
		t.Fatal("expected by_word to be loaded")
	}
}
