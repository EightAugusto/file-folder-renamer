package pattern

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eightaugusto/file-folder-renamer/internal/rename"
	"github.com/eightaugusto/file-folder-renamer/internal/rules"
)

const minimumDefaultPatternCases = 100

type patternTestSuite struct {
	Pattern string            `json:"pattern"`
	Cases   []patternTestCase `json:"cases"`
}

type patternTestCase struct {
	Input             string `json:"input"`
	ExpectedName      string `json:"expected_name"`
	ExpectedExtension string `json:"expected_extension"`
}

func TestDefaultPatternFixture(t *testing.T) {
	fixturePath := filepath.Join("assets", "test_pattern_default.json")
	fixtureData, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read required default pattern fixture: %v", err)
	}

	var fixture patternTestSuite
	if err := json.Unmarshal(fixtureData, &fixture); err != nil {
		t.Fatalf("parse default pattern fixture: %v", err)
	}
	if fixture.Pattern != "default" {
		t.Fatalf("unexpected fixture pattern: %q", fixture.Pattern)
	}
	if len(fixture.Cases) < minimumDefaultPatternCases {
		t.Fatalf("expected at least %d cases, got %d", minimumDefaultPatternCases, len(fixture.Cases))
	}

	patternDefinition := builtInPattern(t, fixture.Pattern)
	patternPipeline := rename.New(patternDefinition.Name, patternDefinition.Rules)
	testRoot := t.TempDir()

	for caseIndex, testCase := range fixture.Cases {
		expectedFilename := testCase.ExpectedName + testCase.ExpectedExtension
		caseLabel := fmt.Sprintf("%q -> %q", testCase.Input, expectedFilename)
		if err := validatePortableFilename(testCase.Input); err != nil {
			t.Errorf("case %s has an invalid input filename: %v", caseLabel, err)
			continue
		}
		if err := validatePortableFilename(expectedFilename); err != nil {
			t.Errorf("case %s has an invalid expected filename: %v", caseLabel, err)
			continue
		}

		nameParts := rename.Parse(testCase.Input)
		normalizedExtension := rename.NormalizeExtension(nameParts.Extension)
		if normalizedExtension != testCase.ExpectedExtension {
			t.Errorf("case %s extension: got %q, want %q", caseLabel, normalizedExtension, testCase.ExpectedExtension)
			continue
		}

		transformedName := ""
		if !nameParts.Protected {
			var err error
			transformedName, err = patternPipeline.Apply(nameParts.Stem)
			if err != nil {
				t.Fatalf("apply pattern to %q: %v", testCase.Input, err)
			}
		}
		if transformedName != testCase.ExpectedName {
			t.Errorf("case %s transformed name: got %q, want %q", caseLabel, transformedName, testCase.ExpectedName)
			continue
		}
		caseRoot := filepath.Join(testRoot, fmt.Sprintf("%03d", caseIndex))
		if err := os.Mkdir(caseRoot, 0o755); err != nil {
			t.Fatalf("create fixture directory for case %s: %v", caseLabel, err)
		}
		sourcePath := filepath.Join(caseRoot, testCase.Input)
		err = os.WriteFile(sourcePath, []byte("fixture"), 0o644)
		if err != nil {
			t.Errorf("case %s could not be created as a real filename: %v", caseLabel, err)
			continue
		}

		proposals, err := rename.Build([]rename.Entry{{
			SourcePath: sourcePath,
			Name:       testCase.Input,
			Kind:       rename.NodeKindFile,
			Depth:      1,
		}}, rename.BuildOptions{
			Pipeline: patternPipeline,
			Files:    true,
		})
		if err != nil {
			t.Fatalf("build proposal for case %s: %v", caseLabel, err)
		}
		if len(proposals) != 1 {
			t.Fatalf("case %s: expected one proposal, got %d", caseLabel, len(proposals))
		}
		if proposals[0].ProposedName != expectedFilename {
			t.Errorf("case %s proposal: got %q, want %q", caseLabel, proposals[0].ProposedName, expectedFilename)
		}
	}
}

func validatePortableFilename(name string) error {
	if name == "" || name == "." || name == ".." {
		return fmt.Errorf("name must not be empty, . or ..")
	}
	if strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return fmt.Errorf("name must not end with a period or space")
	}
	for _, character := range name {
		if character < 32 || strings.ContainsRune(`<>:"/\\|?*`, character) {
			return fmt.Errorf("contains unsupported character %q", character)
		}
	}

	baseName := rename.Parse(name).Stem
	switch strings.ToUpper(baseName) {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return fmt.Errorf("uses reserved Windows base name %q", baseName)
	}
	if utf16CodeUnits(name) > 255 {
		return fmt.Errorf("name exceeds 255 UTF-16 code units")
	}
	return nil
}

func utf16CodeUnits(value string) int {
	codeUnits := 0
	for _, character := range value {
		codeUnits++
		if character > 0xFFFF {
			codeUnits++
		}
	}
	return codeUnits
}

func TestEmbeddedSpecsLoadDefaultPattern(t *testing.T) {
	embeddedSpecs, err := EmbeddedSpecs()
	if err != nil {
		t.Fatal(err)
	}

	var defaultSpec *Spec
	for index := range embeddedSpecs {
		if embeddedSpecs[index].Name == "default" {
			defaultSpec = &embeddedSpecs[index]
			break
		}
	}
	if defaultSpec == nil {
		t.Fatal("expected an embedded default pattern")
	}

	defaultPattern, err := Build(*defaultSpec)
	if err != nil {
		t.Fatal(err)
	}
	if len(defaultPattern.Rules) != 10 {
		t.Fatalf("expected 10 rules, got %d", len(defaultPattern.Rules))
	}
	if len(defaultSpec.Tests) < minimumDefaultPatternCases {
		t.Fatalf("expected embedded default tests, got %d", len(defaultSpec.Tests))
	}

	transformedName, err := rename.New(defaultPattern.Name, defaultPattern.Rules).Apply("FILE-document darwin")
	if err != nil {
		t.Fatal(err)
	}
	if transformedName != "File - Document Darwin" {
		t.Fatalf("unexpected transformed name: %q", transformedName)
	}
}

func TestDefaultPatternDateExclusionVariants(t *testing.T) {
	definition := builtInPattern(t, "default")
	patternPipeline := rename.New(definition.Name, definition.Rules)
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "two digit year first", input: "24-01", expected: "24-01"},
		{name: "four digit year first", input: "2024-01", expected: "2024-01"},
		{name: "two digit year last", input: "01-24", expected: "01-24"},
		{name: "four digit year last", input: "01-2024", expected: "01-2024"},
		{name: "four digit year first in three parts", input: "2024-01-31", expected: "2024-01-31"},
		{name: "two digit year first in three parts", input: "24-01-31", expected: "24-01-31"},
		{name: "four digit year middle", input: "01-2024-31", expected: "01-2024-31"},
		{name: "two digit year middle", input: "01-24-31", expected: "01-24-31"},
		{name: "four digit year last in three parts", input: "01-31-2024", expected: "01-31-2024"},
		{name: "two digit year last in three parts", input: "01-31-24", expected: "01-31-24"},
		{name: "date within a filename", input: "invoice-2024-01-31-final", expected: "Invoice - 2024-01-31 - Final"},
		{name: "repeated separators", input: "invoice---2024-01---final", expected: "Invoice - 2024-01 - Final"},
		{name: "multiple dates", input: "2024-01-and-01-31-24", expected: "2024-01 - And - 01-31-24"},
		{name: "three digit year", input: "202-01", expected: "202 - 01"},
		{name: "one digit middle component", input: "2024-1-31", expected: "2024 - 1 - 31"},
		{name: "one digit final component", input: "2024-01-1", expected: "2024-01 - 1"},
		{name: "three digit middle component", input: "01-202-31", expected: "01 - 202 - 31"},
		{name: "double hyphen normalizes into a date", input: "2024--01", expected: "2024-01"},
		{name: "numeric prefix prevents a partial match", input: "024-01", expected: "024 - 01"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual, err := patternPipeline.Apply(testCase.input)
			if err != nil {
				t.Fatal(err)
			}
			if actual != testCase.expected {
				t.Fatalf("got %q, want %q", actual, testCase.expected)
			}
		})
	}
}

func TestBuildRejectsInvalidOrDuplicateExcludeMatches(t *testing.T) {
	testCases := []struct {
		name    string
		matches []string
	}{
		{name: "empty", matches: []string{""}},
		{name: "invalid", matches: []string{"["}},
		{name: "duplicate", matches: []string{"date", "date"}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := Build(Spec{Name: "excluded", Rules: []RuleSpec{{
				Kind: rules.KindReplaceRunes, Remove: []string{"-"}, ExcludeMatches: testCase.matches,
			}}})
			if err == nil {
				t.Fatal("expected build error")
			}
			if !strings.Contains(err.Error(), "exclude_matches") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestBuildReplaceRunesRuleDefaultsToNoSelections(t *testing.T) {
	patternDefinition, err := Build(Spec{
		Name: "empty-replace",
		Rules: []RuleSpec{{
			Kind: rules.KindReplaceRunes,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	actual, err := patternDefinition.Rules[0].Apply("File-Name")
	if err != nil {
		t.Fatal(err)
	}
	if actual != "File-Name" {
		t.Fatalf("got %q", actual)
	}
}

func TestBuildReplaceRunesRuleAppliesPreserveAndRejectsConflicts(t *testing.T) {
	patternDefinition, err := Build(Spec{
		Name: "safe-characters",
		Rules: []RuleSpec{{
			Kind:     rules.KindReplaceRunes,
			Special:  true,
			Preserve: []string{"-"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	actual, err := patternDefinition.Rules[0].Apply("File-Name_")
	if err != nil {
		t.Fatal(err)
	}
	if actual != "File-Name" {
		t.Fatalf("got %q", actual)
	}

	_, err = Build(Spec{
		Name: "conflicting-overrides",
		Rules: []RuleSpec{{
			Kind:     rules.KindReplaceRunes,
			Special:  true,
			Remove:   []string{"-"},
			Preserve: []string{"-"},
		}},
	})
	if err == nil {
		t.Fatal("expected conflicting remove and preserve entries to return an error")
	}
}

func TestBuildRejectsMultiCharacterReplaceRuneSelectors(t *testing.T) {
	for _, testCase := range []struct {
		name string
		rule RuleSpec
	}{
		{name: "remove", rule: RuleSpec{Kind: rules.KindReplaceRunes, Remove: []string{"ab"}}},
		{name: "preserve", rule: RuleSpec{Kind: rules.KindReplaceRunes, Preserve: []string{"()"}}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := Build(Spec{Name: "one-rune-selectors", Rules: []RuleSpec{testCase.rule}})
			if err == nil || !strings.Contains(err.Error(), "exactly one character") {
				t.Fatalf("expected one-character selector validation error, got %v", err)
			}
		})
	}
}

func TestParseEmbeddedSpecRejectsInvalidJSON(t *testing.T) {
	if _, err := parseEmbeddedSpec("assets/invalid.json", []byte(`{"name":`)); err == nil {
		t.Fatal("expected invalid embedded pattern JSON to return an error")
	}
}

func TestBuildRejectsInvalidEmbeddedPattern(t *testing.T) {
	spec, err := parseEmbeddedSpec("assets/invalid.json", []byte(`{"name":"default","rules":[{"kind":"case"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Build(spec); err == nil {
		t.Fatal("expected invalid embedded pattern to return an error")
	}
}

func TestBuildCustomPatternFromJSONSpec(t *testing.T) {
	patternDefinition, err := Build(Spec{
		Name: "slug",
		Rules: []RuleSpec{
			{Kind: rules.KindCase, Mode: rules.CaseModeLower},
			{
				Kind:        rules.KindReplaceRunes,
				Remove:      []string{" "},
				Replacement: "-",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if patternDefinition.Name != "slug" {
		t.Fatalf("unexpected pattern name: %s", patternDefinition.Name)
	}
	if ruleCount := len(patternDefinition.Rules); ruleCount != 2 {
		t.Fatalf("unexpected rule count: %d", ruleCount)
	}
}

func TestBuildCaseRuleWithPositions(t *testing.T) {
	patternDefinition, err := Build(Spec{
		Name: "selective",
		Rules: []RuleSpec{
			{Kind: rules.KindCase, Mode: rules.CaseModeUpper, Positions: []int{0, 2}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	transformedName, err := patternDefinition.Rules[0].Apply("abcd")
	if err != nil {
		t.Fatal(err)
	}
	if transformedName != "AbCd" {
		t.Fatalf("unexpected transformed name: %q", transformedName)
	}
}

func TestBuildCaseRuleByWordWithPositionZero(t *testing.T) {
	patternDefinition, err := Build(Spec{
		Name: "title",
		Rules: []RuleSpec{
			{Kind: rules.KindCase, Mode: rules.CaseModeUpper, Positions: []int{0}, ByWord: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	transformedName, err := patternDefinition.Rules[0].Apply("file - document - darwin")
	if err != nil {
		t.Fatal(err)
	}
	if transformedName != "File - Document - Darwin" {
		t.Fatalf("unexpected transformed name: %q", transformedName)
	}
}

func TestBuildCaseRuleByWordDisabledUsesFullString(t *testing.T) {
	patternDefinition, err := Build(Spec{
		Name: "single",
		Rules: []RuleSpec{
			{Kind: rules.KindCase, Mode: rules.CaseModeUpper, Positions: []int{0}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	transformedName, err := patternDefinition.Rules[0].Apply("file - document - darwin")
	if err != nil {
		t.Fatal(err)
	}
	if transformedName != "File - document - darwin" {
		t.Fatalf("unexpected transformed name: %q", transformedName)
	}
}

func TestBuildCaseRuleRejectsUnknownMode(t *testing.T) {
	_, err := Build(Spec{
		Name: "invalid",
		Rules: []RuleSpec{
			{Kind: rules.KindCase},
		},
	})
	if err == nil {
		t.Fatal("expected error for unknown case mode")
	}
}

func builtInPattern(t *testing.T, name string) Pattern {
	t.Helper()
	specs, err := EmbeddedSpecs()
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range specs {
		if spec.Name == name {
			definition, err := Build(spec)
			if err != nil {
				t.Fatal(err)
			}
			return definition
		}
	}
	t.Fatalf("built-in pattern %q not found", name)
	return Pattern{}
}

func TestPatternSpecUnmarshalsFromJSON(t *testing.T) {
	var patternSpec Spec
	if err := json.Unmarshal([]byte(`{
		"name":"slug",
		"rules":[
			{"kind":"case","mode":"lower","by_word":true},
			{"_description":"Replace spaces with hyphens.","kind":"replace_runes","remove":[" "],"replacement":"-","deduplicate":true},
			{"_description":"Normalize Latin letters.","kind":"remove_diacritics","latin":true}
		]
	}`), &patternSpec); err != nil {
		t.Fatal(err)
	}

	if patternSpec.Name != "slug" {
		t.Fatalf("unexpected spec name: %s", patternSpec.Name)
	}
	if len(patternSpec.Rules) != 3 {
		t.Fatalf("unexpected rule count: %d", len(patternSpec.Rules))
	}
	if patternSpec.Rules[0].Kind != rules.KindCase {
		t.Fatalf("unexpected parsed kind: %v", patternSpec.Rules[0].Kind)
	}
	if patternSpec.Rules[0].Mode != rules.CaseModeLower {
		t.Fatalf("unexpected parsed mode: %v", patternSpec.Rules[0].Mode)
	}
	if !patternSpec.Rules[0].ByWord {
		t.Fatal("expected by_word to be parsed")
	}
	if patternSpec.Rules[1].Kind != rules.KindReplaceRunes {
		t.Fatalf("unexpected parsed kind: %v", patternSpec.Rules[1].Kind)
	}
	if patternSpec.Rules[1].Description != "Replace spaces with hyphens." {
		t.Fatalf("unexpected rule description: %q", patternSpec.Rules[1].Description)
	}
	if !patternSpec.Rules[1].Deduplicate {
		t.Fatal("expected deduplicate to be parsed")
	}
	if patternSpec.Rules[2].Kind != rules.KindRemoveDiacritics || !patternSpec.Rules[2].Latin || patternSpec.Rules[2].Description != "Normalize Latin letters." {
		t.Fatalf("unexpected parsed diacritics rule: %+v", patternSpec.Rules[2])
	}
}
