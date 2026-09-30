package rules

import (
	"regexp"
	"testing"
)

func TestKindTextAndCatalogBranches(t *testing.T) {
	for _, testCase := range []struct {
		text string
		kind Kind
	}{{"", KindUnknown}, {"unknown", KindUnknown}, {"case", KindCase}, {" replace_RUNES ", KindReplaceRunes}, {" remove_DIACRITICS ", KindRemoveDiacritics}} {
		kind, err := ParseKind(testCase.text)
		if err != nil || kind != testCase.kind {
			t.Fatalf("ParseKind(%q)=%v,%v", testCase.text, kind, err)
		}
		encoded, err := kind.MarshalText()
		if err != nil || string(encoded) != kind.String() {
			t.Fatalf("marshal kind: %q %v", encoded, err)
		}
		var decoded Kind
		if err := decoded.UnmarshalText(encoded); err != nil || decoded != kind {
			t.Fatalf("unmarshal kind: %v %v", decoded, err)
		}
	}
	if _, err := ParseKind("bad"); err == nil {
		t.Fatal("invalid kind accepted")
	}
	if _, err := ParseKind("remove_latin_diacritics"); err == nil {
		t.Fatal("unreleased remove_latin_diacritics alias accepted")
	}
	var kind Kind
	if err := kind.UnmarshalText([]byte("bad")); err == nil {
		t.Fatal("invalid kind text accepted")
	}
	if Kind(99).String() != "unknown" {
		t.Fatal("invalid kind string")
	}
}

func TestCaseModeTextAndRemainingRuleBranches(t *testing.T) {
	for _, testCase := range []struct {
		text string
		mode CaseMode
	}{{"", CaseModeUnknown}, {"unknown", CaseModeUnknown}, {"lower", CaseModeLower}, {" UPPER ", CaseModeUpper}} {
		mode, err := ParseCaseMode(testCase.text)
		if err != nil || mode != testCase.mode {
			t.Fatalf("ParseCaseMode(%q)=%v,%v", testCase.text, mode, err)
		}
		encoded, err := mode.MarshalText()
		if err != nil || string(encoded) != mode.String() {
			t.Fatalf("marshal mode: %q %v", encoded, err)
		}
		var decoded CaseMode
		if err := decoded.UnmarshalText(encoded); err != nil || decoded != mode {
			t.Fatalf("unmarshal mode: %v %v", decoded, err)
		}
	}
	if _, err := ParseCaseMode("bad"); err == nil {
		t.Fatal("invalid mode accepted")
	}
	var mode CaseMode
	if err := mode.UnmarshalText([]byte("bad")); err == nil {
		t.Fatal("invalid mode text accepted")
	}
	if CaseMode(99).String() != "unknown" || transformRune('A', CaseModeUnknown) != 'A' {
		t.Fatal("unknown mode behavior")
	}
	if _, err := (CaseRule{}).Apply("value"); err == nil {
		t.Fatal("unknown case mode accepted")
	}
	if _, err := (CaseRule{Mode: CaseModeLower, Positions: []int{-1}}).Apply("value"); err == nil {
		t.Fatal("negative position accepted")
	}
	if (CaseRule{}).Kind() != KindCase || (ReplaceRunesRule{}).Kind() != KindReplaceRunes || (RemoveDiacriticsRule{}).Kind() != KindRemoveDiacritics {
		t.Fatal("rule kind mismatch")
	}
}

func TestCloneCopiesMutableBuiltInRuleState(t *testing.T) {
	positions := []int{0}
	caseClone := Clone(CaseRule{Mode: CaseModeUpper, Positions: positions}).(CaseRule)
	positions[0] = 1
	if caseClone.Positions[0] != 0 {
		t.Fatal("case rule clone shares positions")
	}

	remove := map[rune]struct{}{'-': {}}
	preserve := map[rune]struct{}{'.': {}}
	expressions := []*regexp.Regexp{regexp.MustCompile("date")}
	replaceClone := Clone(ReplaceRunesRule{Remove: remove, Preserve: preserve, ExcludeMatches: expressions}).(ReplaceRunesRule)
	delete(remove, '-')
	delete(preserve, '.')
	expressions[0] = regexp.MustCompile("other")
	if _, found := replaceClone.Remove['-']; !found {
		t.Fatal("replace rule clone shares remove set")
	}
	if _, found := replaceClone.Preserve['.']; !found {
		t.Fatal("replace rule clone shares preserve set")
	}
	if replaceClone.ExcludeMatches[0].String() != "date" {
		t.Fatal("replace rule clone shares expression slice")
	}
	if cloned := Clone(ReplaceRunesRule{}).(ReplaceRunesRule); cloned.Remove != nil || cloned.Preserve != nil {
		t.Fatal("nil rune sets changed during cloning")
	}
	diacriticsRule := RemoveDiacriticsRule{Latin: true}
	if cloned := Clone(diacriticsRule); cloned != diacriticsRule {
		t.Fatal("immutable diacritics rule changed during cloning")
	}
	unknown := immutableTestRule{}
	if Clone(unknown) != unknown {
		t.Fatal("unknown immutable rule changed during cloning")
	}
}

type immutableTestRule struct{}

func (immutableTestRule) Kind() Kind                         { return KindUnknown }
func (immutableTestRule) Apply(input string) (string, error) { return input, nil }
