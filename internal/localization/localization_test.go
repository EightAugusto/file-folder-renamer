package localization

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"text/template"
)

func TestResolve(t *testing.T) {
	for _, tc := range []struct {
		preference Preference
		system     string
		want       Preference
	}{
		{PreferenceSystem, "en-US", PreferenceEnglish}, {PreferenceSystem, "en-GB", PreferenceEnglish},
		{PreferenceSystem, "es-MX", PreferenceSpanish}, {PreferenceSystem, "es-ES", PreferenceSpanish},
		{PreferenceSystem, "es-AR", PreferenceSpanish}, {PreferenceSystem, "fr-FR", PreferenceEnglish},
		{PreferenceSystem, "", PreferenceEnglish}, {PreferenceSystem, "C", PreferenceEnglish},
		{PreferenceEnglish, "es-MX", PreferenceEnglish}, {PreferenceSpanish, "en-US", PreferenceSpanish},
	} {
		if got := Resolve(tc.preference, tc.system); got != tc.want {
			t.Errorf("Resolve(%q, %q) = %q, want %q", tc.preference, tc.system, got, tc.want)
		}
	}
}

func TestPreferenceJSONAndMetadata(t *testing.T) {
	if Preference("unsupported").Valid() || !PreferenceSystem.Valid() || !PreferenceEnglish.Valid() || !PreferenceSpanish.Valid() {
		t.Fatal("preference validation mismatch")
	}
	var preference Preference
	if err := json.Unmarshal([]byte(`"es-MX"`), &preference); err != nil || preference != PreferenceSpanish {
		t.Fatalf("valid preference: %q %v", preference, err)
	}
	for _, data := range []string{`"unsupported"`, `42`} {
		if err := json.Unmarshal([]byte(data), &preference); err == nil {
			t.Fatalf("expected %s to fail", data)
		}
	}
	translator := New(PreferenceEnglish, "")
	if translator.Preference() != PreferenceEnglish {
		t.Fatal("translator did not retain resolved preference")
	}
}

func TestTranslatorEmbeddedCatalogFailuresPanic(t *testing.T) {
	for _, testCase := range []struct {
		name string
		read func(string) ([]byte, error)
	}{
		{name: "read", read: func(string) ([]byte, error) { return nil, errors.New("read failure") }},
		{name: "parse", read: func(string) ([]byte, error) { return []byte(`{`), nil }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected catalog failure to panic")
				}
			}()
			newTranslator(PreferenceEnglish, "", testCase.read)
		})
	}
}

func TestFallbackRenderingFailures(t *testing.T) {
	if got := renderFallback("{{", nil); got != "{{" {
		t.Fatalf("parse failure changed fallback: %q", got)
	}
	if got := renderFallback("{{.Missing}}", func() {}); got != "{{.Missing}}" {
		t.Fatalf("execute failure changed fallback: %q", got)
	}
	translator := New(PreferenceEnglish, "")
	if got := translator.Text("missing.empty", ""); got != "" {
		t.Fatalf("empty fallback: %q", got)
	}
	if got := translator.Plural("missing.empty.plural", "", "", 1); got != "" {
		t.Fatalf("empty one fallback: %q", got)
	}
	if got := translator.Plural("missing.empty.plural", "", "", 2); got != "" {
		t.Fatalf("empty other fallback: %q", got)
	}
}

func TestTranslationsPluralAndFallback(t *testing.T) {
	for _, tc := range []struct {
		preference       Preference
		label, one, many string
	}{
		{PreferenceEnglish, "Settings", "Applied 1 rename.", "Applied 2 renames."},
		{PreferenceSpanish, "Configuración", "Se aplicó 1 cambio de nombre.", "Se aplicaron 2 cambios de nombre."},
	} {
		translator := New(tc.preference, "")
		if got := translator.Text("tab.settings", "Settings"); got != tc.label {
			t.Fatal(got)
		}
		for count, want := range map[int]string{1: tc.one, 2: tc.many} {
			if got := translator.Plural("organizer.complete_message", "", "", count, map[string]any{"Count": count}); got != want {
				t.Fatalf("plural %s: %q, want %q", tc.preference, got, want)
			}
		}
		if got := translator.Text("missing.key", "Hello {{.Name}}", map[string]any{"Name": "Ana"}); got != "Hello Ana" {
			t.Fatal(got)
		}
		if got := translator.Plural("missing.plural", "{{.Count}} item", "{{.Count}} items", 0, map[string]any{"Count": 0}); got != "0 items" {
			t.Fatal(got)
		}
	}
}

func catalog(t *testing.T, preference Preference) map[string]json.RawMessage {
	t.Helper()
	data, err := translations.ReadFile("translations/" + string(preference) + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var messages map[string]json.RawMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		t.Fatal(err)
	}
	return messages
}

func forms(t *testing.T, raw json.RawMessage) map[string]string {
	t.Helper()
	var plain string
	if json.Unmarshal(raw, &plain) == nil {
		return map[string]string{"other": plain}
	}
	var values map[string]string
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Fatal(err)
	}
	return values
}

func TestCatalogCompletenessAndTemplateArguments(t *testing.T) {
	baseline := catalog(t, PreferenceEnglish)
	parameters := regexp.MustCompile(`\{\{\.[A-Za-z]+\}\}|%[qsdwv]`)
	for _, entry := range Languages() {
		messages := catalog(t, entry.Preference)
		if len(messages) != len(baseline) {
			t.Fatalf("%s has %d keys; want %d", entry.Preference, len(messages), len(baseline))
		}
		for id, source := range baseline {
			raw, found := messages[id]
			if !found {
				t.Errorf("%s missing %s", entry.Preference, id)
				continue
			}
			originalForms, translatedForms := forms(t, source), forms(t, raw)
			if translatedForms["other"] == "" {
				t.Errorf("%s %s lacks other form", entry.Preference, id)
			}
			for form, text := range translatedForms {
				if _, err := template.New(id).Parse(text); err != nil {
					t.Errorf("%s %s: %v", entry.Preference, id, err)
				}
				expected := originalForms[form]
				if expected == "" {
					expected = originalForms["other"]
				}
				gotArgs, wantArgs := parameters.FindAllString(text, -1), parameters.FindAllString(expected, -1)
				sort.Strings(gotArgs)
				sort.Strings(wantArgs)
				if !reflect.DeepEqual(gotArgs, wantArgs) {
					t.Errorf("%s %s/%s template arguments differ: %v vs %v", entry.Preference, id, form, gotArgs, wantArgs)
				}
			}
		}
	}
}

// Prevent new UI messages from silently falling back to English in Spanish.
func TestUICatalogReferencesExist(t *testing.T) {
	messages := catalog(t, PreferenceEnglish)
	paths, err := filepath.Glob("../ui/*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (selector.Sel.Name != "text" && selector.Sel.Name != "Plural" && selector.Sel.Name != "setStatus") {
				return true
			}
			literal, ok := call.Args[0].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			id, _ := strconv.Unquote(literal.Value)
			if _, found := messages[id]; !found {
				t.Errorf("%s references missing message %q", path, id)
			}
			return true
		})
	}
}
