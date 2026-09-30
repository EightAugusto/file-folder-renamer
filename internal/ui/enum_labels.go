package ui

import (
	"sort"

	"github.com/eightaugusto/file-folder-renamer/internal/rename"
	"github.com/eightaugusto/file-folder-renamer/internal/rules"
)

// These labels are intentionally UI-only. Pattern JSON continues to use the
// stable machine values (for example, "replace_runes" and "lower").
const (
	ruleKindCaseLabel             = "Case"
	ruleKindReplaceRunesLabel     = "Replace Runes"
	ruleKindRemoveDiacriticsLabel = "Remove Diacritics"
	caseModeLowerLabel            = "Lower case"
	caseModeUpperLabel            = "Upper case"
	nodeKindFileLabel             = "File"
	nodeKindFolderLabel           = "Folder"
)

func ruleKindOptions(application *Application) []string {
	options := []string{
		application.text("studio.case", "Case"),
		application.text("studio.replace_runes", "Replace Runes"),
		application.text("studio.remove_diacritics", "Remove Diacritics"),
	}
	sort.Strings(options)
	return options
}

func ruleKindLabel(kind rules.Kind, application *Application) string {
	switch kind {
	case rules.KindCase:
		return application.text("studio.case", "Case")
	case rules.KindReplaceRunes:
		return application.text("studio.replace_runes", "Replace Runes")
	case rules.KindRemoveDiacritics:
		return application.text("studio.remove_diacritics", "Remove Diacritics")
	default:
		return application.text("studio.unknown_rule", "Unknown rule")
	}
}

func ruleKindFromLabel(value string, application *Application) (rules.Kind, bool) {
	switch value {
	case application.text("studio.case", "Case"):
		return rules.KindCase, true
	case application.text("studio.replace_runes", "Replace Runes"):
		return rules.KindReplaceRunes, true
	case application.text("studio.remove_diacritics", "Remove Diacritics"):
		return rules.KindRemoveDiacritics, true
	default:
		return rules.KindUnknown, false
	}
}

func caseModeOptions(application *Application) []string {
	return []string{application.text("studio.lower_case", "Lower case"), application.text("studio.upper_case", "Upper case")}
}

func caseModeLabel(mode rules.CaseMode, application *Application) string {
	switch mode {
	case rules.CaseModeLower:
		return application.text("studio.lower_case", "Lower case")
	case rules.CaseModeUpper:
		return application.text("studio.upper_case", "Upper case")
	default:
		return ""
	}
}

func caseModeFromLabel(value string, application *Application) (rules.CaseMode, bool) {
	switch value {
	case application.text("studio.lower_case", "Lower case"):
		return rules.CaseModeLower, true
	case application.text("studio.upper_case", "Upper case"):
		return rules.CaseModeUpper, true
	default:
		return rules.CaseModeUnknown, false
	}
}

func nodeKindOptions(application *Application) []string {
	return []string{application.text("node.file", "File"), application.text("node.folder", "Folder")}
}

func nodeKindLabel(kind rename.NodeKind, application *Application) string {
	if kind == "" {
		kind = rename.NodeKindFile
	}
	switch kind {
	case rename.NodeKindFile:
		return application.text("node.file", "File")
	case rename.NodeKindFolder:
		return application.text("node.folder", "Folder")
	default:
		return application.text("node.unknown", "Unknown")
	}
}

func nodeKindFromLabel(value string, application *Application) rename.NodeKind {
	switch value {
	case application.text("node.folder", "Folder"):
		return rename.NodeKindFolder
	default:
		return rename.NodeKindFile
	}
}

func testResultLabel(status string, application *Application) string {
	switch status {
	case "PASS":
		return application.text("studio.passed", "Passed")
	case "FAIL":
		return application.text("studio.failed", "Failed")
	case "ERROR":
		return application.text("common.error", "Error")
	default:
		return status
	}
}
