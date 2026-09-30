package pattern

import (
	"testing"

	"github.com/eightaugusto/file-folder-renamer/internal/rename"
	"github.com/eightaugusto/file-folder-renamer/internal/rules"
)

func TestTraceAndRunTests(t *testing.T) {
	spec := Spec{Name: "lower", Rules: []RuleSpec{{Kind: rules.KindCase, Mode: rules.CaseModeLower}}, Tests: []TestCase{{Input: "FILE.TXT", Kind: rename.NodeKindFile, Expected: "file.txt"}}}
	trace, err := Trace(spec, "FILE.TXT", rename.NodeKindFile)
	if err != nil {
		t.Fatal(err)
	}
	if trace.Output != "file.txt" || len(trace.Steps) != 1 || trace.Extension != ".txt" {
		t.Fatalf("unexpected trace: %+v", trace)
	}
	results := RunTests(spec)
	if len(results) != 1 || !results[0].Passed {
		t.Fatalf("unexpected test results: %+v", results)
	}
}

func TestTraceIncludesRemoveDiacriticsRule(t *testing.T) {
	spec := Spec{Name: "latin", Rules: []RuleSpec{{Kind: rules.KindRemoveDiacritics, Latin: true}}}
	trace, err := Trace(spec, "Pokémon.txt", rename.NodeKindFile)
	if err != nil {
		t.Fatal(err)
	}
	if trace.Output != "Pokemon.txt" || len(trace.Steps) != 1 || trace.Steps[0].Kind != rules.KindRemoveDiacritics || trace.Steps[0].Output != "Pokemon" {
		t.Fatalf("unexpected diacritics trace: %+v", trace)
	}
}
