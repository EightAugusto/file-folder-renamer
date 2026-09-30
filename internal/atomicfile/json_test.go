package atomicfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteJSONReplacesCompleteDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"old":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteJSON(path, map[string]any{"new": true}); err != nil {
		t.Fatal(err)
	}
	var value map[string]bool
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &value); err != nil || !value["new"] || value["old"] {
		t.Fatalf("unexpected document %q: %v", data, err)
	}
}

func TestReplaceJSONSetCommitsEveryDocument(t *testing.T) {
	root := t.TempDir()
	definition := filepath.Join(root, "pattern.json")
	tests := filepath.Join(root, "pattern.tests.json")
	if err := ReplaceJSONSet(map[string]any{
		definition: map[string]int{"version": 1},
		tests:      map[string]string{"pattern": "example"},
	}, nil); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{definition, tests} {
		if data, err := os.ReadFile(path); err != nil || !json.Valid(data) {
			t.Fatalf("invalid committed file %q: %q %v", path, data, err)
		}
	}
}

func TestReplaceJSONSetStagesBeforeReplacing(t *testing.T) {
	root := t.TempDir()
	unchanged := filepath.Join(root, "a.json")
	blocked := filepath.Join(root, "z.json")
	if err := os.WriteFile(unchanged, []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	err := ReplaceJSONSet(map[string]any{unchanged: map[string]bool{"changed": true}, blocked: struct{}{}}, nil)
	if err == nil {
		t.Fatal("expected directory destination to reject the transaction")
	}
	data, readErr := os.ReadFile(unchanged)
	if readErr != nil || string(data) != "original\n" {
		t.Fatalf("first destination changed after failed transaction: %q %v", data, readErr)
	}
	if info, statErr := os.Stat(blocked); statErr != nil || !info.IsDir() {
		t.Fatalf("blocked destination was replaced: %v %v", info, statErr)
	}
}
