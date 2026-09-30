package rules

import (
	"fmt"
	"strings"
	"unicode"
)

type CaseMode uint8

const (
	CaseModeUnknown CaseMode = iota
	CaseModeLower
	CaseModeUpper
)

func (mode CaseMode) String() string {
	switch mode {
	case CaseModeLower:
		return "lower"
	case CaseModeUpper:
		return "upper"
	default:
		return "unknown"
	}
}

func ParseCaseMode(value string) (CaseMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "lower":
		return CaseModeLower, nil
	case "upper":
		return CaseModeUpper, nil
	case "", "unknown":
		return CaseModeUnknown, nil
	default:
		return CaseModeUnknown, fmt.Errorf("unknown case mode %q", value)
	}
}

func (mode CaseMode) MarshalText() ([]byte, error) {
	return []byte(mode.String()), nil
}

func (mode *CaseMode) UnmarshalText(text []byte) error {
	parsedMode, err := ParseCaseMode(string(text))
	if err != nil {
		return err
	}
	*mode = parsedMode
	return nil
}

type CaseRule struct {
	Mode      CaseMode
	Positions []int
	ByWord    bool
}

func (CaseRule) Kind() Kind { return KindCase }

func (rule CaseRule) Apply(input string) (string, error) {
	if rule.Mode == CaseModeUnknown {
		return "", fmt.Errorf("case rule mode must be lower or upper")
	}
	for _, position := range rule.Positions {
		if position < 0 {
			return "", fmt.Errorf("case rule positions must be non-negative")
		}
	}

	if rule.ByWord {
		return applyByWord(input, rule.Mode, rule.Positions), nil
	}

	return applyByRunePositions(input, rule.Mode, rule.Positions), nil
}

func applyByRunePositions(input string, mode CaseMode, positions []int) string {
	runes := []rune(input)
	if len(positions) == 0 {
		for index, ch := range runes {
			runes[index] = transformRune(ch, mode)
		}
		return string(runes)
	}

	positionSet := make(map[int]struct{}, len(positions))
	for _, position := range positions {
		positionSet[position] = struct{}{}
	}

	for index, ch := range runes {
		if _, selected := positionSet[index]; !selected {
			continue
		}
		runes[index] = transformRune(ch, mode)
	}

	return string(runes)
}

func applyByWord(input string, mode CaseMode, positions []int) string {
	runes := []rune(input)
	var transformed strings.Builder
	transformed.Grow(len(input))

	for index := 0; index < len(runes); {
		if !isWordRune(runes[index]) {
			transformed.WriteRune(runes[index])
			index++
			continue
		}

		wordStart := index
		for index < len(runes) && isWordRune(runes[index]) {
			index++
		}
		word := runes[wordStart:index]
		transformed.WriteString(applyByRunePositions(string(word), mode, positions))
	}

	return transformed.String()
}

func transformRune(ch rune, mode CaseMode) rune {
	switch mode {
	case CaseModeLower:
		return unicode.ToLower(ch)
	case CaseModeUpper:
		return unicode.ToUpper(ch)
	default:
		return ch
	}
}

func isWordRune(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch)
}
