package rules

import (
	"regexp"
	"testing"
)

const defaultDateExclusionExpression = `\b(?:(?:[0-9]{2}|[0-9]{4})-[0-9]{2}-[0-9]{2}|[0-9]{2}-(?:[0-9]{2}|[0-9]{4})-[0-9]{2}|[0-9]{2}-[0-9]{2}-(?:[0-9]{2}|[0-9]{4})|(?:[0-9]{2}|[0-9]{4})-[0-9]{2}|[0-9]{2}-(?:[0-9]{2}|[0-9]{4}))\b`

func TestReplaceRunesRuleReplacesMappedRunes(t *testing.T) {
	replaceRule := ReplaceRunesRule{
		Remove:      map[rune]struct{}{':': {}, '_': {}},
		Replacement: "-",
	}

	transformedName, err := replaceRule.Apply("a:b_c")
	if err != nil {
		t.Fatal(err)
	}
	if transformedName != "a-b-c" {
		t.Fatalf("got %q", transformedName)
	}
}

func TestReplaceRunesRulesDeduplicateMixedSeparators(t *testing.T) {
	normalizeRule := ReplaceRunesRule{
		Remove:      map[rune]struct{}{'-': {}, ':': {}, '_': {}},
		Deduplicate: true,
		Replacement: "-",
	}
	formatRule := ReplaceRunesRule{
		Remove:      map[rune]struct{}{'-': {}},
		Deduplicate: true,
		Replacement: " - ",
	}

	normalizedName, err := normalizeRule.Apply("file-_:document")
	if err != nil {
		t.Fatal(err)
	}
	transformedName, err := formatRule.Apply(normalizedName)
	if err != nil {
		t.Fatal(err)
	}
	if transformedName != "file - document" {
		t.Fatalf("got %q", transformedName)
	}
}

func TestReplaceRunesRulesNormalizeAndDeduplicateWhitespace(t *testing.T) {
	normalizeRule := ReplaceRunesRule{Space: true, Replacement: " "}
	deduplicateRule := ReplaceRunesRule{
		Remove:      map[rune]struct{}{' ': {}},
		Deduplicate: true,
		Trim:        true,
		Replacement: " ",
	}

	normalizedName, err := normalizeRule.Apply("\thello\t \nworld ")
	if err != nil {
		t.Fatal(err)
	}
	transformedName, err := deduplicateRule.Apply(normalizedName)
	if err != nil {
		t.Fatal(err)
	}
	if transformedName != "hello world" {
		t.Fatalf("got %q", transformedName)
	}
}

func TestReplaceRunesRuleReplacesConfiguredCategories(t *testing.T) {
	testCases := []struct {
		name     string
		rule     ReplaceRunesRule
		input    string
		expected string
	}{
		{
			name:     "numeric",
			rule:     ReplaceRunesRule{Numeric: true},
			input:    "A1٢-",
			expected: "A-",
		},
		{
			name:     "alphabetical",
			rule:     ReplaceRunesRule{Alphabetical: true},
			input:    "Aé1-",
			expected: "1-",
		},
		{
			name:     "space",
			rule:     ReplaceRunesRule{Space: true},
			input:    "A B\tC\n",
			expected: "ABC",
		},
		{
			name:     "special replacement",
			rule:     ReplaceRunesRule{Special: true, Replacement: " "},
			input:    "A1-_😀\x00",
			expected: "A1    ",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual, err := testCase.rule.Apply(testCase.input)
			if err != nil {
				t.Fatal(err)
			}
			if actual != testCase.expected {
				t.Fatalf("got %q, want %q", actual, testCase.expected)
			}
		})
	}
}

func TestReplaceRunesRuleAppliesRemoveAndPreserveSelectors(t *testing.T) {
	rule := ReplaceRunesRule{
		Remove:      map[rune]struct{}{'_': {}},
		Preserve:    map[rune]struct{}{'-': {}, '.': {}},
		Replacement: " ",
	}

	actual, err := rule.Apply("A-B_C.txt")
	if err != nil {
		t.Fatal(err)
	}
	if actual != "A-B C.txt" {
		t.Fatalf("got %q", actual)
	}
}

func TestReplaceRunesRuleExcludesRegexMatches(t *testing.T) {
	rule := ReplaceRunesRule{
		Remove:         map[rune]struct{}{'-': {}},
		Deduplicate:    true,
		Replacement:    " - ",
		ExcludeMatches: []*regexp.Regexp{regexp.MustCompile(defaultDateExclusionExpression)},
	}
	for input, expected := range map[string]string{
		"2025-07":                   "2025-07",
		"2025-12-05":                "2025-12-05",
		"25-07":                     "25-07",
		"07-2025":                   "07-2025",
		"05-12-2025":                "05-12-2025",
		"05-2025-12":                "05-2025-12",
		"05-12-25":                  "05-12-25",
		"202-07":                    "202 - 07",
		"report-2025-12-05-final":   "report - 2025-12-05 - final",
		"report---final":            "report - final",
		"report-2025-07---final":    "report - 2025-07 - final",
		"report-2025-7-final":       "report - 2025 - 7 - final",
		"report-2025-12-05-2025-07": "report - 2025-12-05 - 2025-07",
		"réport-2025-12-05-fïnäl":   "réport - 2025-12-05 - fïnäl",
	} {
		actual, err := rule.Apply(input)
		if err != nil {
			t.Fatalf("apply %q: %v", input, err)
		}
		if actual != expected {
			t.Errorf("%q: got %q, want %q", input, actual, expected)
		}
	}
}

func TestDefaultDateExclusionPattern(t *testing.T) {
	rule := ReplaceRunesRule{
		Remove:         map[rune]struct{}{'-': {}},
		Deduplicate:    true,
		Replacement:    " - ",
		ExcludeMatches: []*regexp.Regexp{regexp.MustCompile(defaultDateExclusionExpression)},
	}
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "two digit year first", input: "24-01", expected: "24-01"},
		{name: "four digit year first", input: "2024-01", expected: "2024-01"},
		{name: "two digit year last", input: "01-24", expected: "01-24"},
		{name: "four digit year last", input: "01-2024", expected: "01-2024"},
		{name: "four digit year first in three parts", input: "2024-01-31", expected: "2024-01-31"},
		{name: "two digit year first in three parts", input: "24-01-31", expected: "24-01-31"},
		{name: "four digit year middle", input: "01-2024-31", expected: "01-2024-31"},
		{name: "two digit year middle", input: "01-24-31", expected: "01-24-31"},
		{name: "four digit year last in three parts", input: "01-31-2024", expected: "01-31-2024"},
		{name: "two digit year last in three parts", input: "01-31-24", expected: "01-31-24"},
		{name: "date inside a name", input: "invoice-2024-01-31-final", expected: "invoice - 2024-01-31 - final"},
		{name: "date with repeated surrounding separators", input: "invoice---2024-01---final", expected: "invoice - 2024-01 - final"},
		{name: "two dates", input: "2024-01-and-01-31-24", expected: "2024-01 - and - 01-31-24"},
		{name: "existing spaces are preserved for later whitespace normalization", input: "invoice - 2024-01-31 - final", expected: "invoice  -  2024-01-31  -  final"},
		{name: "three digit year is not a date", input: "202-01", expected: "202 - 01"},
		{name: "one digit month is not a date", input: "2024-1", expected: "2024 - 1"},
		{name: "one digit leading component is not a date", input: "1-2024", expected: "1 - 2024"},
		{name: "three digit month is not a date", input: "2024-001", expected: "2024 - 001"},
		{name: "short final component only preserves the valid date prefix", input: "2024-01-1", expected: "2024-01 - 1"},
		{name: "long final component only preserves the valid date prefix", input: "2024-01-312", expected: "2024-01 - 312"},
		{name: "double separator is not a date", input: "2024--01", expected: "2024 - 01"},
		{name: "leading zero creates a non-date token", input: "024-01", expected: "024 - 01"},
		{name: "word prefix prevents a match", input: "x2024-01", expected: "x2024 - 01"},
		{name: "word suffix prevents a match", input: "2024-01x", expected: "2024 - 01x"},
		{name: "unicode words around separated date", input: "réport-2024-01-fïnäl", expected: "réport - 2024-01 - fïnäl"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual, err := rule.Apply(testCase.input)
			if err != nil {
				t.Fatal(err)
			}
			if actual != testCase.expected {
				t.Fatalf("got %q, want %q", actual, testCase.expected)
			}
		})
	}
}

func TestReplaceRunesRuleMergesOverlappingExcludedMatches(t *testing.T) {
	rule := ReplaceRunesRule{
		Remove:         map[rune]struct{}{'-': {}},
		Replacement:    " ",
		ExcludeMatches: []*regexp.Regexp{regexp.MustCompile(`2025-12`), regexp.MustCompile(`12-05`)},
	}
	actual, err := rule.Apply("2025-12-05-next")
	if err != nil {
		t.Fatal(err)
	}
	if actual != "2025-12-05 next" {
		t.Fatalf("got %q", actual)
	}
}

func TestCaseRules(t *testing.T) {
	wholeLowerName, err := CaseRule{Mode: CaseModeLower}.Apply("AbC")
	if err != nil {
		t.Fatal(err)
	}
	if wholeLowerName != "abc" {
		t.Fatal(wholeLowerName)
	}

	wholeUpperName, err := CaseRule{Mode: CaseModeUpper}.Apply("AbC")
	if err != nil {
		t.Fatal(err)
	}
	if wholeUpperName != "ABC" {
		t.Fatal(wholeUpperName)
	}
}

func TestRemoveDiacriticsRuleWithLatinEnabled(t *testing.T) {
	rule := RemoveDiacriticsRule{Latin: true}
	for _, testCase := range []struct {
		name     string
		input    string
		expected string
	}{
		{name: "single marks", input: "ÀÁÂÃÄÅĀĂĄǍǞǠǺ ÇĆĈĊČ ÈÉÊËĒĔĖĘĚ ÌÍÎÏĨĪĬĮǏ ÑŃŅŇ ÒÓÔÕÖŌŎŐǑ ÙÚÛÜŨŪŬŮŰŲǓ ÝŶŸ", expected: "AAAAAAAAAAAAA CCCCC EEEEEEEEE IIIIIIIII NNNN OOOOOOOOO UUUUUUUUUUU YYY"},
		{name: "multiple marks", input: "ǜ ậ ắ", expected: "u a a"},
		{name: "decomposed", input: "Cafe\u0301 Espan\u0303a A\u0308", expected: "Cafe Espana A"},
		{name: "mixed words", input: "Crème brûlée São Tomé Český Krumlov Dvořák Tiếng Việt", expected: "Creme brulee Sao Tome Cesky Krumlov Dvorak Tieng Viet"},
		{name: "transliteration remains explicit", input: "ø ł æ ß", expected: "ø ł æ ß"},
		{name: "non latin marks remain", input: "Αθη\u0301να Приве\u0301т", expected: "Αθήνα Приве́т"},
		{name: "unattached mark remains", input: "\u0301start", expected: "\u0301start"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			actual, err := rule.Apply(testCase.input)
			if err != nil {
				t.Fatal(err)
			}
			if actual != testCase.expected {
				t.Fatalf("got %q, want %q", actual, testCase.expected)
			}
		})
	}
}

func TestRemoveDiacriticsRuleWithNoScriptsIsANoOp(t *testing.T) {
	input := "Pokémon Αθήνα Cafe\u0301"
	actual, err := (RemoveDiacriticsRule{}).Apply(input)
	if err != nil {
		t.Fatal(err)
	}
	if actual != input {
		t.Fatalf("got %q", actual)
	}
}

func TestCaseRuleAppliesToSelectedPositions(t *testing.T) {
	caseRule := CaseRule{Mode: CaseModeUpper, Positions: []int{0, 2}}
	transformedName, err := caseRule.Apply("abcd")
	if err != nil {
		t.Fatal(err)
	}
	if transformedName != "AbCd" {
		t.Fatalf("got %q", transformedName)
	}
}

func TestCaseRuleAppliesByWord(t *testing.T) {
	caseRule := CaseRule{Mode: CaseModeUpper, Positions: []int{0}, ByWord: true}
	transformedName, err := caseRule.Apply("file - document - darwin")
	if err != nil {
		t.Fatal(err)
	}
	if transformedName != "File - Document - Darwin" {
		t.Fatalf("got %q", transformedName)
	}
}

func TestCaseRuleAppliesToFullStringWhenByWordIsDisabled(t *testing.T) {
	caseRule := CaseRule{Mode: CaseModeUpper, Positions: []int{0}}
	transformedName, err := caseRule.Apply("file - document - darwin")
	if err != nil {
		t.Fatal(err)
	}
	if transformedName != "File - document - darwin" {
		t.Fatalf("got %q", transformedName)
	}
}

func TestCaseRuleRejectsUnknownMode(t *testing.T) {
	caseRule := CaseRule{}
	if _, err := caseRule.Apply("AbC"); err == nil {
		t.Fatal("expected error")
	}
}
