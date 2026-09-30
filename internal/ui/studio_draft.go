package ui

import (
	"github.com/eightaugusto/file-folder-renamer/internal/pattern"
)

// studioDraft is a Fyne-independent editor model. It owns selection, dirty,
// save/discard, and deep-copy boundaries; widgets only render and edit it.
type studioDraft struct {
	selectedTest int
	draft        pattern.Spec
	baseName     string
	selectedRule int
	dirty        bool
	builtIn      bool
}

func newStudioDraft() studioDraft {
	return studioDraft{selectedRule: -1, selectedTest: -1}
}

func (state *studioDraft) load(spec pattern.Spec, builtIn bool) {
	state.draft = spec.Clone()
	state.baseName = spec.Name
	state.builtIn = builtIn
	state.selectedRule = -1
	state.selectedTest = -1
	state.dirty = false
}

func (state *studioDraft) saved() {
	state.baseName = state.draft.Name
	state.builtIn = false
	state.dirty = false
}

func (state *studioDraft) appendRule(rule pattern.RuleSpec) {
	state.draft.Rules = append(state.draft.Rules, rule)
	state.selectedRule = len(state.draft.Rules) - 1
}

func (state *studioDraft) moveRule(index, delta int) bool {
	target := index + delta
	if index < 0 || target < 0 || index >= len(state.draft.Rules) || target >= len(state.draft.Rules) {
		return false
	}
	state.draft.Rules[index], state.draft.Rules[target] = state.draft.Rules[target], state.draft.Rules[index]
	state.selectedRule = target
	return true
}

func (state *studioDraft) removeRule(index int) bool {
	if index < 0 || index >= len(state.draft.Rules) {
		return false
	}
	state.draft.Rules = append(state.draft.Rules[:index], state.draft.Rules[index+1:]...)
	switch {
	case len(state.draft.Rules) == 0:
		state.selectedRule = -1
	case index >= len(state.draft.Rules):
		state.selectedRule = len(state.draft.Rules) - 1
	default:
		state.selectedRule = index
	}
	return true
}

func (state *studioDraft) appendTest(test pattern.TestCase) {
	state.draft.Tests = append(state.draft.Tests, test)
}

func (state *studioDraft) removeSelectedTest() bool {
	if state.builtIn || state.selectedTest < 0 || state.selectedTest >= len(state.draft.Tests) {
		return false
	}
	index := state.selectedTest
	state.draft.Tests = append(state.draft.Tests[:index], state.draft.Tests[index+1:]...)
	state.selectedTest = -1
	return true
}

func (state studioDraft) snapshot() pattern.Spec { return state.draft.Clone() }
