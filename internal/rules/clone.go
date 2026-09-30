package rules

import "regexp"

// Clone returns an independent copy of the built-in mutable rule values.
// Unknown Rule implementations are treated as immutable values.
func Clone(rule Rule) Rule {
	switch value := rule.(type) {
	case CaseRule:
		value.Positions = append([]int(nil), value.Positions...)
		return value
	case ReplaceRunesRule:
		value.Remove = cloneRuneSet(value.Remove)
		value.Preserve = cloneRuneSet(value.Preserve)
		value.ExcludeMatches = append([]*regexp.Regexp(nil), value.ExcludeMatches...)
		return value
	default:
		return rule
	}
}

func cloneRuneSet(source map[rune]struct{}) map[rune]struct{} {
	if source == nil {
		return nil
	}
	clone := make(map[rune]struct{}, len(source))
	for character := range source {
		clone[character] = struct{}{}
	}
	return clone
}
