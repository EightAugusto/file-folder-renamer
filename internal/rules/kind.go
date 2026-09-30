package rules

import (
	"fmt"
	"strings"
)

type Kind uint8

const (
	KindUnknown Kind = iota
	KindCase
	KindReplaceRunes
	KindRemoveDiacritics
)

func (k Kind) String() string {
	switch k {
	case KindCase:
		return "case"
	case KindReplaceRunes:
		return "replace_runes"
	case KindRemoveDiacritics:
		return "remove_diacritics"
	default:
		return "unknown"
	}
}

func ParseKind(value string) (Kind, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "case":
		return KindCase, nil
	case "replace_runes":
		return KindReplaceRunes, nil
	case "remove_diacritics":
		return KindRemoveDiacritics, nil
	case "", "unknown":
		return KindUnknown, nil
	default:
		return KindUnknown, fmt.Errorf("unknown rule kind %q", value)
	}
}

func (k Kind) MarshalText() ([]byte, error) {
	return []byte(k.String()), nil
}

func (k *Kind) UnmarshalText(text []byte) error {
	parsed, err := ParseKind(string(text))
	if err != nil {
		return err
	}
	*k = parsed
	return nil
}
