package ui

import (
	"reflect"
	"testing"

	"github.com/eightaugusto/file-folder-renamer/internal/rename"
)

func TestProposalTableStateFiltersSortsAndCopies(t *testing.T) {
	input := []rename.Proposal{
		{Kind: rename.NodeKindFolder, RelativePath: "z", Changed: false},
		{Kind: rename.NodeKindFile, RelativePath: "b", Changed: true},
		{Kind: rename.NodeKindFile, RelativePath: "a", Changed: true},
	}
	state := newProposalTableState()
	state.setProposals(input)
	input[2].RelativePath = "mutated"
	state.kindFilter = map[string]struct{}{"File": {}}
	if got, want := state.visibleIndexes(), []int{2, 1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("indexes: got %v, want %v", got, want)
	}
	state.toggleSort(proposalColumnLocation)
	if got, want := state.visibleIndexes(), []int{1, 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("descending indexes: got %v, want %v", got, want)
	}
}
