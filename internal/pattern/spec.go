package pattern

import (
	"github.com/eightaugusto/file-folder-renamer/internal/rename"
	"github.com/eightaugusto/file-folder-renamer/internal/rules"
)

type Spec struct {
	Name  string     `json:"name"`
	Rules []RuleSpec `json:"rules"`
	Tests []TestCase `json:"tests,omitempty"`
}

type TestCase struct {
	Input    string          `json:"input"`
	Kind     rename.NodeKind `json:"kind,omitempty"`
	Expected string          `json:"expected"`
}

type TraceStep struct {
	Index       int        `json:"index"`
	Kind        rules.Kind `json:"kind"`
	Description string     `json:"description,omitempty"`
	Input       string     `json:"input"`
	Output      string     `json:"output"`
	Error       string     `json:"error,omitempty"`
}

type TraceResult struct {
	Input     string          `json:"input"`
	Kind      rename.NodeKind `json:"kind"`
	Stem      string          `json:"stem,omitempty"`
	Extension string          `json:"extension,omitempty"`
	Protected bool            `json:"protected"`
	Steps     []TraceStep     `json:"steps"`
	Output    string          `json:"output"`
}

type TestResult struct {
	Case   TestCase `json:"case"`
	Actual string   `json:"actual"`
	Passed bool     `json:"passed"`
	Error  string   `json:"error,omitempty"`
}

type RuleSpec struct {
	Kind           rules.Kind     `json:"kind"`
	Mode           rules.CaseMode `json:"mode,omitempty"`
	Positions      []int          `json:"positions,omitempty"`
	ByWord         bool           `json:"by_word,omitempty"`
	Deduplicate    bool           `json:"deduplicate,omitempty"`
	Trim           bool           `json:"trim,omitempty"`
	Numeric        bool           `json:"numeric,omitempty"`
	Alphabetical   bool           `json:"alphabetical,omitempty"`
	Space          bool           `json:"space,omitempty"`
	Special        bool           `json:"special,omitempty"`
	Latin          bool           `json:"latin,omitempty"`
	Remove         []string       `json:"remove,omitempty"`
	Preserve       []string       `json:"preserve,omitempty"`
	ExcludeMatches []string       `json:"exclude_matches,omitempty"`
	Replacement    string         `json:"replacement,omitempty"`
	Description    string         `json:"_description,omitempty"`
}

type Pattern struct {
	Name  string
	Rules []rules.Rule
}

// Clone returns an independent copy suitable for editing or returning across
// package boundaries.
func (spec Spec) Clone() Spec {
	cloned := Spec{Name: spec.Name, Tests: append([]TestCase(nil), spec.Tests...)}
	cloned.Rules = make([]RuleSpec, len(spec.Rules))
	for index, rule := range spec.Rules {
		cloned.Rules[index] = rule
		cloned.Rules[index].Positions = append([]int(nil), rule.Positions...)
		cloned.Rules[index].Remove = append([]string(nil), rule.Remove...)
		cloned.Rules[index].Preserve = append([]string(nil), rule.Preserve...)
		cloned.Rules[index].ExcludeMatches = append([]string(nil), rule.ExcludeMatches...)
	}
	return cloned
}
