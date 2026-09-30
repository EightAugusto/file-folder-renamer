package pattern

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/eightaugusto/file-folder-renamer/internal/rules"
)

func Build(spec Spec) (Pattern, error) {
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return Pattern{}, fmt.Errorf("pattern name must not be empty")
	}

	rulesList := make([]rules.Rule, 0, len(spec.Rules))
	for _, ruleSpec := range spec.Rules {
		ruleDefinition, err := buildRule(ruleSpec)
		if err != nil {
			return Pattern{}, err
		}
		rulesList = append(rulesList, ruleDefinition)
	}

	return Pattern{Name: name, Rules: rulesList}, nil
}

func buildRule(spec RuleSpec) (rules.Rule, error) {
	switch spec.Kind {
	case rules.KindCase:
		if spec.Mode == rules.CaseModeUnknown {
			return nil, fmt.Errorf("case rule mode must be lower or upper")
		}
		for _, position := range spec.Positions {
			if position < 0 {
				return nil, fmt.Errorf("case rule positions must be non-negative")
			}
		}
		positions := append([]int(nil), spec.Positions...)
		return rules.CaseRule{Mode: spec.Mode, Positions: positions, ByWord: spec.ByWord}, nil
	case rules.KindReplaceRunes:
		remove, err := runeSet(spec.Remove, "remove")
		if err != nil {
			return nil, err
		}
		preserve, err := runeSet(spec.Preserve, "preserve")
		if err != nil {
			return nil, err
		}
		for ch := range remove {
			if _, found := preserve[ch]; found {
				return nil, fmt.Errorf("replace_runes rule cannot select and preserve %q", ch)
			}
		}
		excludeMatches, err := compileExcludeMatches(spec.ExcludeMatches)
		if err != nil {
			return nil, err
		}
		return rules.ReplaceRunesRule{
			Numeric:        spec.Numeric,
			Alphabetical:   spec.Alphabetical,
			Space:          spec.Space,
			Special:        spec.Special,
			Deduplicate:    spec.Deduplicate,
			Trim:           spec.Trim,
			Remove:         remove,
			Preserve:       preserve,
			ExcludeMatches: excludeMatches,
			Replacement:    spec.Replacement,
		}, nil
	case rules.KindRemoveDiacritics:
		if !spec.Latin {
			return nil, fmt.Errorf("remove_diacritics rule must enable at least one script")
		}
		return rules.RemoveDiacriticsRule{Latin: spec.Latin}, nil
	default:
		return nil, fmt.Errorf("unknown rule kind %q", spec.Kind.String())
	}
}

func compileExcludeMatches(expressions []string) ([]*regexp.Regexp, error) {
	if len(expressions) == 0 {
		return nil, nil
	}
	compiled := make([]*regexp.Regexp, 0, len(expressions))
	seen := make(map[string]struct{}, len(expressions))
	for index, expression := range expressions {
		if strings.TrimSpace(expression) == "" {
			return nil, fmt.Errorf("replace_runes exclude_matches[%d] must not be empty", index)
		}
		if _, found := seen[expression]; found {
			return nil, fmt.Errorf("replace_runes exclude_matches[%d] duplicates %q", index, expression)
		}
		pattern, err := regexp.Compile(expression)
		if err != nil {
			return nil, fmt.Errorf("replace_runes exclude_matches[%d] %q: %w", index, expression, err)
		}
		seen[expression] = struct{}{}
		compiled = append(compiled, pattern)
	}
	return compiled, nil
}

func runeSet(entries []string, field string) (map[rune]struct{}, error) {
	set := make(map[rune]struct{})
	for _, entry := range entries {
		characters := []rune(entry)
		if len(characters) != 1 {
			return nil, fmt.Errorf("replace_runes %s entries must contain exactly one character", field)
		}
		set[characters[0]] = struct{}{}
	}
	return set, nil
}
