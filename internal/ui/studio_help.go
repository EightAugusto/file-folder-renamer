package ui

import (
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// Keep the pipeline overview separate from each rule reference so Help can
// present a compact overview before its exclusive-open rule accordion.
const patternStudioPipelineHelpMarkdown = `
# How a rule pipeline works

A pattern is an ordered list of rules. The first rule receives the original name, then each following rule receives the output of the rule before it. The final output is the proposed rename.

Create a pipeline by choosing a rule and selecting Add Rule. Select a rule in the Rule pipeline list to configure it. Move rules up or down to change their order; order is significant because a later rule can refine, undo, or format the output of an earlier one.

For example, a pipeline can first normalize separators with Replace Runes, then lower-case the result with Case, and finally upper-case the first character of each word with another Case rule. Use a saved test case whenever you change a pipeline so the intended final name remains clear.
`

const caseRuleHelpMarkdown = `
## Case

**Purpose:** Convert all or selected character positions to lower case or upper case.

**Arguments:**

- **Mode:** Choose **Lower case** or **Upper case**. This is required.
- **Positions:** A comma-separated list of zero-based character positions, such as "0, 2, 5". Leave it blank to transform every character. Positions must be zero or greater; positions outside the name are ignored.
- **Apply positions to each word:** Repeats the selected positions for every word. A word is a continuous run of letters or digits; spaces, hyphens, parentheses, and punctuation separate words.

**Examples:**

- Lower case with no Positions: "My FILE Name" -> "my file name".
- Upper case with Positions "0,2": "abcd" -> "AbCd".
- Upper case with Position "0" and **Apply positions to each word**: "file - document 2025" -> "File - Document 2025".
`

const replaceRunesRuleHelpMarkdown = `
## Replace Runes

**Purpose:** Select individual characters and replace every selected character with the **Replacement** text. An empty Replacement deletes selected characters.

### Categories

- **Numbers:** Selects Unicode numeric characters.
- **Letters:** Selects Unicode letters.
- **Whitespace:** Selects spaces, tabs, and other Unicode whitespace.
- **Special:** Selects any character that is not a letter, number, or whitespace. This includes underscore, hyphen, parentheses, and most punctuation.

### Explicit selectors

- **Remove Runes:** Adds individual characters to the selected set. Every entry must contain exactly one character; typing "_-" selects both underscore and hyphen.
- **Preserve runes:** Explicitly keeps individual characters even when a selected category would otherwise replace them. Every entry must contain exactly one character.
- A character cannot be in both **Remove Runes** and **Preserve runes**.
- **Replacement:** The text emitted for every selected character. Use a single space to normalize separators, " - " to format hyphens, or leave it empty to remove selected characters.

**Example:** Select **Special**, set Replacement to one space, and put "()" in **Preserve runes**. "hello(world)_notes" becomes "hello(world) notes".

### Options

- **Deduplicate matches:** Suppresses consecutive duplicate characters selected explicitly through **Remove Runes**. For example, with hyphen in Remove, "file---name" can produce one replacement instead of three.
- **Trim result:** Removes leading and trailing whitespace after this rule runs.

### Excluded matches

The **Excluded matches** table accepts Go regular expressions (RE2 syntax). Every full match is copied unchanged by this one Replace Runes rule, even when its characters are selected by a category or explicit selector. Expressions are validated when **Add** is selected and cannot be empty, invalid, or duplicated.

**Examples:**

- Preserve ISO-like dates while formatting other hyphens: "\\b(?:[0-9]{2}|[0-9]{4})-[0-9]{2}(?:-[0-9]{2})?\\b". With Special selected and Replacement " - ", "report-2025-07.pdf" can become "report - 2025-07.pdf".
- Preserve a simple parenthesized segment: "\\([^)]*\\)" protects "(draft)".
- Preserve a version token: "\\bv[0-9]+(?:\\.[0-9]+)*\\b" protects "v2.10.3".

Excluded matches apply only to the rule that contains them. Add the expression again if a later Replace Runes rule must also leave the same text unchanged.
`

const removeDiacriticsRuleHelpMarkdown = `
## Remove Diacritics

**Purpose:** Remove decomposable Unicode diacritics from letters in selected scripts.

### Scripts

- **Latin:** Removes marks from Latin letters. For example, "Pokémon", "España", and "Český" become "Pokemon", "Espana", and "Cesky".

At least one script must be enabled. The rule supports both precomposed and decomposed Unicode text.

Characters such as "ø", "ł", "æ", and "ß" are preserved because changing them requires transliteration rather than diacritic removal. Marks in non-Latin writing systems are also preserved.
`

func (view *StudioView) showHelp() {
	help := dialog.NewCustom(view.application.text("studio.help_title", "Pattern Studio help"), view.application.text("common.close", "Close"), patternStudioHelpContent(view.application), view.application.window)
	help.Resize(patternStudioHelpSize(view.application.window))
	view.application.showDialog(help)
}

func patternStudioHelpContent(application *Application) fyne.CanvasObject {
	overview := helpMarkdown(application.text("help.pipeline", patternStudioPipelineHelpMarkdown))
	rules := patternStudioHelpAccordion(application)
	var labels textBindings
	content := newVertical(spaceLG,
		overview,
		widget.NewSeparator(),
		labels.section(func() string { return application.text("help.rules", "Rules") }, func() string { return "" }, rules),
	)
	return container.NewVScroll(container.New(layout.NewCustomPaddedLayout(spaceLG, spaceLG, spaceLG, spaceLG), content))
}

func patternStudioHelpAccordion(application *Application) *widget.Accordion {
	items := []*widget.AccordionItem{
		widget.NewAccordionItem(application.text("studio.case", ruleKindCaseLabel), helpRuleMarkdown(application.text("help.case", caseRuleHelpMarkdown))),
		widget.NewAccordionItem(application.text("studio.replace_runes", ruleKindReplaceRunesLabel), helpRuleMarkdown(application.text("help.replace_runes", replaceRunesRuleHelpMarkdown))),
		widget.NewAccordionItem(application.text("studio.remove_diacritics", ruleKindRemoveDiacriticsLabel), helpRuleMarkdown(application.text("help.remove_diacritics", removeDiacriticsRuleHelpMarkdown))),
	}
	sort.Slice(items, func(left, right int) bool { return items[left].Title < items[right].Title })
	accordion := widget.NewAccordion(items...)
	// false is Fyne's single-open mode: opening a rule always closes the one
	// that was previously expanded.
	accordion.MultiOpen = false
	return accordion
}

func patternStudioHelpSize(window fyne.Window) fyne.Size {
	size := window.Canvas().Size()
	if size.Width <= 0 || size.Height <= 0 {
		return fyne.NewSize(900, 680)
	}
	return fyne.NewSize(fyne.Min(900, size.Width*0.8), fyne.Min(680, size.Height*0.8))
}

// The accordion already names the rule. Omit its repeated document heading.
func helpRuleMarkdown(text string) fyne.CanvasObject {
	text = strings.TrimSpace(text)
	if _, body, found := strings.Cut(text, "\n"); found {
		text = body
	}
	return helpMarkdown(text)
}

// Keep localized documentation intact while presenting its headings through
// the same restrained sections used by the rule editors and Settings.
func helpMarkdown(text string) *fyne.Container {
	var labels textBindings
	sections := []fyne.CanvasObject{}
	title := ""
	body := []string{}
	flush := func() {
		markdown := strings.TrimSpace(strings.Join(body, "\n"))
		if title == "" && markdown == "" {
			return
		}
		rich := widget.NewRichTextFromMarkdown(markdown)
		rich.Wrapping = fyne.TextWrapWord
		heading := title
		if heading == "" {
			sections = append(sections, rich)
		} else {
			sections = append(sections, labels.section(func() string { return heading }, func() string { return "" }, rich))
		}
		body = nil
	}
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		trimmed := strings.TrimSpace(line)
		heading := ""
		if strings.HasPrefix(trimmed, "#") {
			heading = strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		} else if strings.HasPrefix(trimmed, "**") && strings.HasSuffix(trimmed, "**") && strings.Count(trimmed, "**") == 2 {
			heading = strings.TrimSuffix(strings.Trim(trimmed, "*"), ":")
		}
		if heading != "" {
			flush()
			title = heading
		} else {
			body = append(body, line)
		}
	}
	flush()
	return newVertical(spaceLG, sections...)
}
