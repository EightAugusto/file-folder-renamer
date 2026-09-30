package rules

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// RemoveDiacriticsRule removes decomposable diacritics attached to letters in
// enabled scripts. Characters requiring transliteration are preserved.
type RemoveDiacriticsRule struct {
	Latin bool
}

func (RemoveDiacriticsRule) Kind() Kind { return KindRemoveDiacritics }

func (rule RemoveDiacriticsRule) Apply(input string) (string, error) {
	if !rule.hasEnabledScript() {
		return input, nil
	}

	decomposed := norm.NFD.String(input)
	var output strings.Builder
	output.Grow(len(decomposed))

	enabledBase := false
	for _, character := range decomposed {
		if unicode.IsMark(character) {
			if enabledBase {
				continue
			}
			output.WriteRune(character)
			continue
		}

		enabledBase = rule.matchesEnabledScript(character)
		output.WriteRune(character)
	}

	return norm.NFC.String(output.String()), nil
}

func (rule RemoveDiacriticsRule) hasEnabledScript() bool {
	return rule.Latin
}

func (rule RemoveDiacriticsRule) matchesEnabledScript(character rune) bool {
	return rule.Latin && unicode.Is(unicode.Latin, character)
}
