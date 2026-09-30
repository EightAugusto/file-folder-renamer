package ui

import (
	"testing"

	"github.com/eightaugusto/file-folder-renamer/internal/pattern"
	"github.com/eightaugusto/file-folder-renamer/internal/rename"
	"github.com/eightaugusto/file-folder-renamer/internal/rules"
)

func TestStudioDraftRuleAndTestTransitions(t *testing.T) {
	state := newStudioDraft()
	state.load(pattern.Spec{Name: "draft", Rules: []pattern.RuleSpec{{Kind: rules.KindCase}}}, false)
	state.appendRule(pattern.RuleSpec{Kind: rules.KindReplaceRunes})
	if state.selectedRule != 1 || !state.moveRule(1, -1) || state.selectedRule != 0 {
		t.Fatalf("unexpected rule selection: %+v", state)
	}
	if !state.removeRule(0) || len(state.draft.Rules) != 1 || state.selectedRule != 0 {
		t.Fatalf("unexpected rule deletion: %+v", state)
	}
	state.appendTest(pattern.TestCase{Input: "a", Kind: rename.NodeKindFile, Expected: "A"})
	state.selectedTest = 0
	if !state.removeSelectedTest() || len(state.draft.Tests) != 0 || state.selectedTest != -1 {
		t.Fatalf("unexpected test deletion: %+v", state)
	}
}

func TestStudioDraftSnapshotIsDeepCopy(t *testing.T) {
	state := newStudioDraft()
	state.load(pattern.Spec{Rules: []pattern.RuleSpec{{Remove: []string{"-"}, ExcludeMatches: []string{"x"}}}}, false)
	snapshot := state.snapshot()
	snapshot.Rules[0].Remove[0] = "_"
	snapshot.Rules[0].ExcludeMatches[0] = "y"
	if state.draft.Rules[0].Remove[0] != "-" || state.draft.Rules[0].ExcludeMatches[0] != "x" {
		t.Fatal("snapshot mutated draft state")
	}
}
