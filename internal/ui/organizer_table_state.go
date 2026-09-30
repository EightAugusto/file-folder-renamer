package ui

import (
	"sort"

	"github.com/eightaugusto/file-folder-renamer/internal/rename"
)

// proposalTableState is UI-toolkit independent. It owns copied proposal data,
// stable filters, and sorting while the Fyne table acts only as an adapter.
type proposalTableState struct {
	proposals     []rename.Proposal
	kindFilter    map[string]struct{}
	statusFilter  map[string]struct{}
	actionFilter  map[string]struct{}
	sortColumn    int
	sortAscending bool
}

func newProposalTableState() proposalTableState {
	return proposalTableState{
		kindFilter: make(map[string]struct{}), statusFilter: make(map[string]struct{}),
		actionFilter: make(map[string]struct{}), sortColumn: proposalColumnLocation, sortAscending: true,
	}
}

func (state *proposalTableState) setProposals(proposals []rename.Proposal) {
	state.proposals = append([]rename.Proposal(nil), proposals...)
}

func (state proposalTableState) visibleIndexes() []int {
	indexes := make([]int, 0, len(state.proposals))
	for index, item := range state.proposals {
		if len(state.kindFilter) > 0 && !containsProposalFilter(state.kindFilter, enumLabel(string(item.Kind))) {
			continue
		}
		if len(state.statusFilter) > 0 && !containsProposalFilter(state.statusFilter, proposalStatus(item.Changed)) {
			continue
		}
		if len(state.actionFilter) > 0 && !containsProposalFilter(state.actionFilter, proposalAction(item)) {
			continue
		}
		indexes = append(indexes, index)
	}
	if state.sortColumn < 0 || state.sortColumn >= len(proposalTableHeaders()) {
		return indexes
	}
	sort.SliceStable(indexes, func(left, right int) bool {
		leftValue := proposalSortValue(state.proposals[indexes[left]], state.sortColumn)
		rightValue := proposalSortValue(state.proposals[indexes[right]], state.sortColumn)
		if state.sortAscending {
			return leftValue < rightValue
		}
		return leftValue > rightValue
	})
	return indexes
}

func (state *proposalTableState) toggleSort(column int) {
	if state.sortColumn == column {
		state.sortAscending = !state.sortAscending
		return
	}
	state.sortColumn = column
	state.sortAscending = true
}
