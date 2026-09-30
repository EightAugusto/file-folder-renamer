package settings

import (
	"github.com/eightaugusto/file-folder-renamer/internal/atomicfile"
	"github.com/eightaugusto/file-folder-renamer/internal/localization"
	"os"
	"path/filepath"
	"testing"
)

func TestLanguageConfigDefaultsAndRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.json")
	data := []byte(`{"patterns_path":"` + filepath.ToSlash(filepath.Join(root, "patterns")) + `","include_files":true,"ignored_names":[]}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Language != localization.PreferenceSystem {
		t.Fatalf("default language: %q", loaded.Language)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(data) {
		t.Fatal("Load rewrote existing configuration")
	}
	for _, language := range []localization.Preference{localization.PreferenceSystem, localization.PreferenceEnglish, localization.PreferenceSpanish} {
		loaded.Language = language
		if err := Save(path, loaded); err != nil {
			t.Fatal(err)
		}
		roundtrip, err := Load(path)
		if err != nil || roundtrip.Language != language {
			t.Fatalf("language %s: %+v, %v", language, roundtrip, err)
		}
	}
	loaded.Language = "fr-FR"
	if err := Save(path, loaded); err == nil {
		t.Fatal("unsupported language saved")
	}
	if err := atomicfile.WriteJSON(path, loaded); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("unsupported language loaded")
	}
	if _, err := Open(root); err == nil {
		t.Fatal("unsupported language bootstrapped")
	}
}
