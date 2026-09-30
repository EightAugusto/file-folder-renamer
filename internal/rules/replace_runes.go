package rules

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

type ReplaceRunesRule struct {
	Numeric        bool
	Alphabetical   bool
	Space          bool
	Special        bool
	Deduplicate    bool
	Trim           bool
	Remove         map[rune]struct{}
	Preserve       map[rune]struct{}
	ExcludeMatches []*regexp.Regexp
	Replacement    string
}

func (ReplaceRunesRule) Kind() Kind { return KindReplaceRunes }

func (rule ReplaceRunesRule) Apply(input string) (string, error) {
	excluded := excludedByteRanges(input, rule.ExcludeMatches)
	var output strings.Builder
	output.Grow(len(input))
	previousWasSelected := false
	var previous rune
	rangeIndex := 0
	for byteIndex, ch := range input {
		for rangeIndex < len(excluded) && byteIndex >= excluded[rangeIndex].end {
			rangeIndex++
		}
		if rangeIndex < len(excluded) && byteIndex >= excluded[rangeIndex].start {
			output.WriteRune(ch)
			previousWasSelected = false
			continue
		}
		_, selected := rule.Remove[ch]
		if rule.Deduplicate && selected && previousWasSelected && ch == previous {
			continue
		}
		if _, found := rule.Preserve[ch]; found {
			output.WriteRune(ch)
		} else if rule.shouldReplace(ch) {
			output.WriteString(rule.Replacement)
		} else {
			output.WriteRune(ch)
		}
		previous = ch
		previousWasSelected = selected
	}
	result := output.String()
	if rule.Trim {
		result = strings.TrimSpace(result)
	}
	return result, nil
}

type byteRange struct{ start, end int }

func excludedByteRanges(input string, expressions []*regexp.Regexp) []byteRange {
	ranges := make([]byteRange, 0)
	for _, expression := range expressions {
		for _, match := range expression.FindAllStringIndex(input, -1) {
			if match[0] != match[1] {
				ranges = append(ranges, byteRange{start: match[0], end: match[1]})
			}
		}
	}
	if len(ranges) < 2 {
		return ranges
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].start < ranges[j].start })
	merged := ranges[:1]
	for _, candidate := range ranges[1:] {
		last := &merged[len(merged)-1]
		if candidate.start <= last.end {
			if candidate.end > last.end {
				last.end = candidate.end
			}
			continue
		}
		merged = append(merged, candidate)
	}
	return merged
}

func (rule ReplaceRunesRule) matchesCategory(ch rune) bool {
	if rule.Numeric && unicode.IsNumber(ch) {
		return true
	}
	if rule.Alphabetical && unicode.IsLetter(ch) {
		return true
	}
	if rule.Space && unicode.IsSpace(ch) {
		return true
	}
	return rule.Special && !unicode.IsNumber(ch) && !unicode.IsLetter(ch) && !unicode.IsSpace(ch)
}

func (rule ReplaceRunesRule) shouldReplace(ch rune) bool {
	if _, found := rule.Remove[ch]; found {
		return true
	}
	return rule.matchesCategory(ch)
}
