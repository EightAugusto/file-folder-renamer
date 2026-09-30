package rename

import (
	"testing"

	"github.com/eightaugusto/file-folder-renamer/internal/rules"
)

func TestPipelineAppliesRulesInOrder(t *testing.T) {
	renamePipeline := New("default", []rules.Rule{
		rules.ReplaceRunesRule{Remove: map[rune]struct{}{':': {}}, Replacement: "-"},
		rules.CaseRule{Mode: rules.CaseModeLower},
	})
	transformedName, err := renamePipeline.Apply("HELLO: WORLD")
	if err != nil {
		t.Fatal(err)
	}
	if transformedName != "hello- world" {
		t.Fatalf("got %q", transformedName)
	}
}

func TestPipelineOwnsMutableRuleState(t *testing.T) {
	positions := []int{0}
	remove := map[rune]struct{}{':': {}}
	rulesList := []rules.Rule{
		rules.CaseRule{Mode: rules.CaseModeUpper, Positions: positions},
		rules.ReplaceRunesRule{Remove: remove, Replacement: "-"},
	}
	renamePipeline := New("immutable", rulesList)
	positions[0] = 1
	delete(remove, ':')
	rulesList[0] = rules.CaseRule{Mode: rules.CaseModeLower}

	actual, err := renamePipeline.Apply("a:b")
	if err != nil {
		t.Fatal(err)
	}
	if actual != "A-b" {
		t.Fatalf("pipeline changed through caller-owned state: %q", actual)
	}
}
