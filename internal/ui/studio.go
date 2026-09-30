package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/eightaugusto/file-folder-renamer/internal/pattern"
	"github.com/eightaugusto/file-folder-renamer/internal/patternlib"
	"github.com/eightaugusto/file-folder-renamer/internal/rename"
	"github.com/eightaugusto/file-folder-renamer/internal/rules"
)

type StudioView struct {
	studioDraft
	editorLabels    textBindings
	ruleLabels      textBindings
	pipelinePanel   *fyne.Container
	caseMode        *widget.Select
	latin           *widget.Check
	excludedTable   *TablePanel
	feedback        localizedFeedback
	labels          textBindings
	application     *Application
	content         fyne.CanvasObject
	patternPick     *widget.Select
	newButton       *widget.Button
	helpButton      *widget.Button
	namePrompt      dialog.Dialog
	namePromptInput *widget.Entry
	status          *widget.Label
	save            *widget.Button
	discard         *widget.Button
	delete          *widget.Button
	ruleKind        *widget.Select
	addRuleButton   *widget.Button
	rulesHost       *fyne.Container
	editorHost      *fyne.Container
	workspaceSplit  *container.Split
	testInput       *widget.Entry
	testExpected    *widget.Entry
	testKind        *widget.Select
	addTestButton   *widget.Button
	deleteTest      *widget.Button
	testsTable      *widget.Table
	testResults     []pattern.TestResult
	switching       bool
	validationErr   error
}

func NewStudioView(application *Application) *StudioView {
	view := &StudioView{application: application, studioDraft: newStudioDraft()}
	view.patternPick = widget.NewSelect(nil, view.selectPattern)
	view.status = widget.NewLabel("")
	view.setStatus("studio.initial_status", "Build a pattern and verify it with examples.")
	view.labels.add(func() { view.feedback.refresh(application, view.status) })
	view.status.Wrapping = fyne.TextWrapWord
	view.save = view.labels.buttonIcon(func() string { return application.text("common.save", "Save") }, theme.DocumentSaveIcon(), func() {
		if err := view.SaveCurrent(); err != nil {
			application.showError(err)
		}
	})
	view.save.Importance = widget.HighImportance
	view.save.Disable()
	view.discard = view.labels.button(func() string { return application.text("common.discard", "Discard") }, view.discardChanges)
	view.discard.Disable()
	view.delete = view.labels.buttonIcon(func() string { return application.text("common.delete", "Delete") }, theme.DeleteIcon(), view.confirmDelete)

	view.newButton = view.labels.buttonIcon(func() string { return application.text("studio.new", "New") }, theme.ContentAddIcon(), view.promptNewPattern)
	view.helpButton = view.labels.buttonIcon(func() string { return application.text("studio.help", "Help") }, theme.HelpIcon(), view.showHelp)
	duplicateButton := view.labels.buttonIcon(func() string { return application.text("studio.duplicate", "Duplicate") }, theme.ContentCopyIcon(), view.duplicatePattern)
	importButton := view.labels.buttonIcon(func() string { return application.text("studio.import", "Import") }, theme.DownloadIcon(), view.importConfig)
	exportButton := view.labels.buttonIcon(func() string { return application.text("studio.export", "Export") }, theme.UploadIcon(), view.exportConfig)
	management := newFlow(view.newButton, duplicateButton, view.delete)
	transfer := newFlow(importButton, exportButton)
	editing := newFlow(view.save, view.discard)
	header := newVertical(spaceSM,
		newResponsiveForm(view.labels.formItem(func() string { return application.text("common.pattern", "Pattern") }, view.patternPick)),
		newFlow(management, transfer, editing, view.helpButton),
		view.status, widget.NewSeparator(),
	)

	view.rulesHost = container.NewVBox()
	view.ruleKind = widget.NewSelect(ruleKindOptions(view.application), nil)
	view.ruleKind.SetSelected(ruleKindLabel(rules.KindReplaceRunes, application))
	view.addRuleButton = widget.NewButtonWithIcon("", theme.ContentAddIcon(), func() {
		if kind, ok := ruleKindFromLabel(view.ruleKind.Selected, view.application); ok {
			view.addRule(kind)
		}
	})
	ruleControls := container.NewBorder(nil, nil, nil, view.addRuleButton, view.ruleKind)
	rulePanel := view.labels.panel(func() string { return application.text("studio.rule_pipeline", "Rule pipeline") }, func() string {
		return application.text("studio.rule_pipeline_subtitle", "Rules run from top to bottom.")
	}, container.NewBorder(ruleControls, nil, nil, nil, container.NewVScroll(view.rulesHost)))

	view.editorHost = container.NewStack(view.labels.label(func() string { return application.text("studio.select_rule", "Select a rule to edit its settings.") }))
	editorPanel := view.labels.panel(func() string { return application.text("studio.rule_settings", "Rule settings") }, func() string { return "" }, view.editorHost)
	// The pipeline needs only its content width. Keeping it compact leaves the
	// rest of the top workspace for longer rule forms such as regex exclusions.
	view.pipelinePanel = rulePanel
	ruleWorkspace := container.New(&pipelineWorkspaceLayout{view: view}, rulePanel, editorPanel)

	view.testInput = widget.NewEntry()
	view.testInput.SetPlaceHolder(application.text("studio.input_name", "Input name"))
	view.testInput.OnChanged = func(string) { view.refreshPendingTestExpected() }
	view.testExpected = widget.NewEntry()
	view.testExpected.SetPlaceHolder(application.text("studio.calculated", "Calculated automatically"))
	view.testExpected.Disable()
	view.testKind = widget.NewSelect(nodeKindOptions(view.application), func(string) { view.refreshPendingTestExpected() })
	view.testKind.SetSelected(nodeKindLabel(rename.NodeKindFile, application))
	view.addTestButton = view.labels.buttonIcon(func() string { return application.text("studio.add_test", "Add test") }, theme.ContentAddIcon(), view.addTest)
	view.deleteTest = view.labels.buttonIcon(func() string { return application.text("studio.delete_selected", "Delete selected") }, theme.DeleteIcon(), view.removeSelectedTest)
	view.deleteTest.Disable()
	view.testsTable = view.newTestsTable()
	testFields := container.New(&testEntryLayout{},
		view.labels.section(func() string { return application.text("common.input", "Input") }, func() string { return "" }, view.testInput),
		view.labels.section(func() string { return application.text("common.kind", "Kind") }, func() string { return "" }, view.testKind),
		view.labels.section(func() string { return application.text("common.expected", "Expected") }, func() string { return "" }, view.testExpected),
		container.NewVBox(view.addTestButton, view.deleteTest),
	)
	testBody := newWorkspace(testFields, nil, view.testsTable)
	testBody.Layout.(*workspaceLayout).inset = 0
	testsPanel := view.labels.panel(func() string { return application.text("studio.tests_title", "Saved test cases") }, func() string {
		return application.text("studio.tests_subtitle", "Saved cases are the pattern validation source and all must pass before saving.")
	}, testBody)

	view.workspaceSplit = container.NewVSplit(ruleWorkspace, newSurface(testsPanel))
	view.workspaceSplit.Offset = 0.5
	view.content = newWorkspace(header, nil, view.workspaceSplit)
	view.RefreshPatterns("default")
	return view
}

func (view *StudioView) Content() fyne.CanvasObject { return view.content }
func (view *StudioView) IsDirty() bool              { return view.dirty }

func (view *StudioView) RefreshPatterns(prefer string) {
	if view.dirty && !view.switching {
		return
	}
	options := make([]string, 0)
	for _, entry := range view.application.store.Entries() {
		options = append(options, entry.Spec.Name)
	}
	view.switching = true
	view.patternPick.Options = options
	view.patternPick.Refresh()
	selected := prefer
	if selected == "" {
		selected = view.patternPick.Selected
	}
	found := false
	for _, option := range options {
		if strings.EqualFold(option, selected) {
			selected = option
			found = true
			break
		}
	}
	if !found && len(options) > 0 {
		selected = options[0]
	}
	view.patternPick.SetSelected(selected)
	view.switching = false
	if selected != "" {
		view.loadPattern(selected)
	}
}

func (view *StudioView) selectPattern(name string) {
	if view.switching || name == "" {
		return
	}
	if view.dirty {
		view.setStatus("studio.dirty_switch", "Save or discard the current changes before switching patterns.")
		view.switching = true
		selected := view.baseName
		if selected == "" {
			selected = view.draft.Name
		}
		view.patternPick.SetSelected(selected)
		view.switching = false
		return
	}
	view.loadPattern(name)
}

func (view *StudioView) loadPattern(name string) {
	entry, err := view.application.store.Lookup(name)
	if err != nil {
		view.feedback.setLiteral(view.status, err.Error(), view.content)
		return
	}
	view.switching = true
	view.studioDraft.load(entry.Spec, entry.BuiltIn)
	view.switching = false
	view.validationErr = nil
	view.save.Disable()
	view.discard.Disable()
	view.applyReadOnlyState()
	if entry.BuiltIn {
		view.delete.Disable()
		view.setStatus("studio.builtin", "Built-in pattern. It is read-only; duplicate it to create an editable pattern.")
	} else {
		view.delete.Enable()
		view.setStatus("studio.custom", "Custom editable pattern.")
	}
	view.refreshAll()
}

func (view *StudioView) markDirty() {
	if view.switching || view.builtIn {
		return
	}
	view.dirty = true
	view.save.Enable()
	view.discard.Enable()
	view.setStatus("studio.unsaved", "Unsaved changes")
	view.refreshPendingTestExpected()
	view.refreshTests()
}

func (view *StudioView) SaveCurrent() error {
	if view.builtIn {
		return patternlib.ErrReadOnly
	}
	if view.validationErr != nil {
		return view.validationErr
	}
	if _, err := pattern.Build(view.draft); err != nil {
		return err
	}
	for _, result := range pattern.RunTests(view.draft) {
		if result.Error != "" {
			return errors.New(result.Error)
		}
		if !result.Passed {
			return fmt.Errorf(view.application.text("studio.test_failed", "test %q failed: got %q, want %q"), result.Case.Input, result.Actual, result.Case.Expected)
		}
	}
	if err := view.application.store.SaveAs(view.baseName, view.draft); err != nil {
		return err
	}
	view.studioDraft.saved()
	view.applyReadOnlyState()
	view.save.Disable()
	view.discard.Disable()
	view.setStatus("studio.saved", "Pattern saved. Organizer previews now use this version.")
	view.application.RefreshPatterns(view.draft.Name)
	return nil
}

func (view *StudioView) discardChanges() {
	if view.baseName == "" {
		view.dirty = false
		view.validationErr = nil
		view.RefreshPatterns("default")
		return
	}
	view.dirty = false
	view.loadPattern(view.baseName)
}

func (view *StudioView) promptNewPattern() {
	if !view.requireClean() {
		return
	}
	view.promptForPatternName(view.application.text("studio.new_pattern", "New pattern"), "", func(name string) error {
		return view.newPattern(name)
	})
}

func (view *StudioView) newPattern(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New(view.application.text("studio.name_required", "pattern name must not be empty"))
	}
	if view.application.store.Has(name) {
		return fmt.Errorf(view.application.text("studio.name_exists", "a pattern named %q already exists"), name)
	}
	view.draft = pattern.Spec{Name: name}
	view.baseName = ""
	view.builtIn = false
	view.applyReadOnlyState()
	view.selectedRule = -1
	view.selectDraftName(name)
	view.markDirty()
	view.refreshAll()
	return nil
}

func (view *StudioView) duplicatePattern() {
	if !view.requireClean() {
		return
	}
	view.promptForPatternName(view.application.text("studio.duplicate_pattern", "Duplicate pattern"), view.draft.Name+" copy", func(name string) error {
		return view.duplicatePatternAs(name)
	})
}

func (view *StudioView) duplicatePatternAs(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New(view.application.text("studio.name_required", "pattern name must not be empty"))
	}
	if view.application.store.Has(name) {
		return fmt.Errorf(view.application.text("studio.name_exists", "a pattern named %q already exists"), name)
	}
	view.draft = view.draft.Clone()
	view.draft.Name = name
	view.baseName = ""
	view.builtIn = false
	view.applyReadOnlyState()
	view.selectDraftName(name)
	view.markDirty()
	view.refreshAll()
	return nil
}

func (view *StudioView) promptForPatternName(title, suggested string, accept func(string) error) {
	input := widget.NewEntry()
	input.SetPlaceHolder(view.application.text("studio.pattern_name", "Pattern name"))
	input.SetText(suggested)
	view.namePromptInput = input
	view.namePrompt = dialog.NewForm(title, view.application.text("common.create", "Create"), view.application.text("common.cancel", "Cancel"), []*widget.FormItem{
		widget.NewFormItem(view.application.text("common.name", "Name"), input),
	}, func(confirmed bool) {
		if !confirmed {
			return
		}
		if err := accept(input.Text); err != nil {
			view.application.showError(err)
		}
	}, view.application.window)
	view.application.showDialog(view.namePrompt)
}

func (view *StudioView) selectDraftName(name string) {
	view.switching = true
	options := append([]string(nil), view.patternPick.Options...)
	found := false
	for _, option := range options {
		if strings.EqualFold(option, name) {
			found = true
			break
		}
	}
	if !found {
		options = append(options, name)
	}
	view.patternPick.Options = options
	view.patternPick.Refresh()
	view.patternPick.SetSelected(name)
	view.switching = false
}

func (view *StudioView) requireClean() bool {
	if view.dirty {
		view.setStatus("studio.clean_required", "Save or discard the current changes first.")
		return false
	}
	return true
}

func (view *StudioView) applyReadOnlyState() {
	controls := []fyne.Disableable{view.ruleKind, view.addRuleButton, view.addTestButton}
	for _, control := range controls {
		if view.builtIn {
			control.Disable()
		} else {
			control.Enable()
		}
	}
	// Built-in patterns are read-only, but their input controls remain useful
	// for trying a name and seeing its calculated output without saving it.
	view.testInput.Enable()
	view.testKind.Enable()
	view.testExpected.Disable()
	if view.builtIn || view.selectedTest < 0 {
		view.deleteTest.Disable()
	} else {
		view.deleteTest.Enable()
	}
}

func (view *StudioView) confirmDelete() {
	if view.builtIn || view.baseName == "" {
		return
	}
	prompt := dialog.NewConfirm(view.application.text("studio.delete_pattern_title", "Delete pattern?"), view.application.text("studio.delete_pattern_message", "Delete custom pattern {{.Name}} and its saved test cases?", map[string]any{"Name": fmt.Sprintf("%q", view.baseName)}), func(ok bool) {
		if !ok {
			return
		}
		if err := view.application.store.Delete(view.baseName); err != nil {
			view.application.showError(err)
			return
		}
		view.dirty = false
		view.application.RefreshPatterns("default")
	}, view.application.window)
	prompt.SetConfirmText(view.application.text("common.delete", "Delete"))
	prompt.SetDismissText(view.application.text("common.cancel", "Cancel"))
	prompt.SetConfirmImportance(widget.DangerImportance)
	view.application.showDialog(prompt)
}

func (view *StudioView) addRule(kind rules.Kind) {
	var rule pattern.RuleSpec
	switch kind {
	case rules.KindCase:
		rule.Kind = rules.KindCase
		rule.Mode = rules.CaseModeLower
	case rules.KindReplaceRunes:
		rule.Kind = rules.KindReplaceRunes
	case rules.KindRemoveDiacritics:
		rule.Kind = rules.KindRemoveDiacritics
	default:
		return
	}
	view.studioDraft.appendRule(rule)
	view.markDirty()
	view.refreshRules()
	view.refreshEditor()
}

func (view *StudioView) moveRule(index, delta int) {
	if !view.studioDraft.moveRule(index, delta) {
		return
	}
	view.markDirty()
	view.refreshRules()
	view.refreshEditor()
}

func (view *StudioView) removeRule(index int) {
	if !view.studioDraft.removeRule(index) {
		return
	}
	view.markDirty()
	view.refreshRules()
	view.refreshEditor()
}

func (view *StudioView) refreshAll() {
	view.refreshRules()
	view.refreshEditor()
	view.refreshPendingTestExpected()
	view.refreshTests()
}

func (view *StudioView) refreshRules() {
	view.ruleLabels = nil
	view.rulesHost.RemoveAll()
	if len(view.draft.Rules) == 0 {
		view.rulesHost.Add(view.ruleLabels.label(func() string { return view.application.text("studio.no_rules", "No rules yet. Add one above.") }))
		return
	}
	for index := range view.draft.Rules {
		index := index
		label := view.ruleRowLabel(index)
		selectButton := widget.NewButton(label, func() {
			view.selectedRule = index
			view.refreshRules()
			view.refreshEditor()
		})
		if index == view.selectedRule {
			selectButton.Importance = widget.MediumImportance
		}
		up := widget.NewButtonWithIcon("", theme.MoveUpIcon(), func() { view.moveRule(index, -1) })
		down := widget.NewButtonWithIcon("", theme.MoveDownIcon(), func() { view.moveRule(index, 1) })
		remove := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() { view.removeRule(index) })
		if view.builtIn {
			up.Disable()
			down.Disable()
			remove.Disable()
		}
		if index == 0 {
			up.Disable()
		}
		if index == len(view.draft.Rules)-1 {
			down.Disable()
		}
		view.rulesHost.Add(container.NewBorder(nil, nil, nil, container.NewHBox(up, down, remove), selectButton))
	}
}

func (view *StudioView) refreshEditor() {
	view.editorLabels = nil
	view.caseMode = nil
	view.latin = nil
	view.excludedTable = nil
	view.validationErr = nil
	var editor fyne.CanvasObject = view.editorLabels.label(func() string {
		return view.application.text("studio.select_rule", "Select a rule to edit its settings.")
	})
	if view.selectedRule >= 0 && view.selectedRule < len(view.draft.Rules) {
		switch view.draft.Rules[view.selectedRule].Kind {
		case rules.KindCase:
			editor = view.caseEditor()
		case rules.KindReplaceRunes:
			editor = view.replaceEditor()
		case rules.KindRemoveDiacritics:
			editor = view.removeDiacriticsEditor()
		}
	}
	view.editorHost.Objects = []fyne.CanvasObject{newDocumentScroll(editor)}
	view.editorHost.Refresh()
}

func (view *StudioView) removeDiacriticsEditor() fyne.CanvasObject {
	rule := view.draft.Rules[view.selectedRule]
	behavior := widget.NewLabel(view.application.text(
		"studio.remove_diacritics_behavior",
		"Removes decomposable marks from letters in enabled scripts. Characters that require transliteration are preserved.",
	))
	behavior.Wrapping = fyne.TextWrapWord
	latin := view.editorLabels.check(func() string { return view.application.text("studio.latin", "Latin") }, nil)
	view.latin = latin
	latin.SetChecked(rule.Latin)
	setValidation := func(enabled bool) {
		if enabled {
			view.validationErr = nil
			return
		}
		view.validationErr = errors.New(view.application.text("studio.script_required", "at least one script must be enabled"))
	}
	setValidation(rule.Latin)
	latin.OnChanged = func(value bool) {
		view.draft.Rules[view.selectedRule].Latin = value
		setValidation(value)
		view.markDirty()
		if view.validationErr != nil {
			view.feedback.setLiteral(view.status, view.validationErr.Error(), view.content)
		}
	}
	description := view.boundEntry(rule.Description, func(value string) { view.draft.Rules[view.selectedRule].Description = value })
	view.disableForBuiltIn(latin, description)
	if view.validationErr != nil {
		view.feedback.setLiteral(view.status, view.validationErr.Error(), view.content)
	}
	return ruleSettingsSections(
		view.editorLabels.section(func() string {
			return view.application.text("studio.scripts", "Scripts")
		}, func() string {
			return view.application.text("studio.scripts_subtitle", "Choose at least one writing system whose diacritics should be removed.")
		}, newResponsiveForm(
			view.editorLabels.formItem(func() string { return view.application.text("studio.scripts", "Scripts") }, latin),
		)),
		view.editorLabels.section(func() string {
			return view.application.text("studio.behavior", "Behavior")
		}, func() string {
			return view.application.text("studio.remove_diacritics_subtitle", "Normalize enabled scripts while preserving other writing systems.")
		}, behavior),
		view.editorLabels.section(func() string { return view.application.text("common.description", "Description") }, func() string {
			return view.application.text("studio.description_help", "Optional note saved with this rule.")
		}, newResponsiveForm(
			view.editorLabels.formItem(func() string { return view.application.text("common.description", "Description") }, description),
		)),
	)
}

func (view *StudioView) caseEditor() fyne.CanvasObject {
	rule := view.draft.Rules[view.selectedRule]
	mode := widget.NewSelect(caseModeOptions(view.application), nil)
	view.caseMode = mode
	mode.SetSelected(caseModeLabel(rule.Mode, view.application))
	mode.OnChanged = func(value string) {
		if parsed, ok := caseModeFromLabel(value, view.application); ok {
			view.draft.Rules[view.selectedRule].Mode = parsed
			view.markDirty()
		}
	}
	positions := make([]string, len(rule.Positions))
	for i, position := range rule.Positions {
		positions[i] = strconv.Itoa(position)
	}
	positionEntry := widget.NewEntry()
	positionEntry.SetText(strings.Join(positions, ", "))
	positionEntry.OnChanged = func(value string) {
		parsed, err := parsePositions(value, view.application)
		if err != nil {
			view.validationErr = err
			view.markDirty()
			view.setStatus("studio.positions_invalid", "positions must be non-negative numbers separated by commas")
			return
		}
		view.validationErr = nil
		view.draft.Rules[view.selectedRule].Positions = parsed
		view.markDirty()
	}
	byWord := view.editorLabels.check(func() string { return view.application.text("studio.by_word", "Apply positions to each word") }, nil)
	byWord.SetChecked(rule.ByWord)
	byWord.OnChanged = func(value bool) {
		view.draft.Rules[view.selectedRule].ByWord = value
		view.markDirty()
	}
	description := view.boundEntry(rule.Description, func(value string) { view.draft.Rules[view.selectedRule].Description = value })
	view.disableForBuiltIn(mode, positionEntry, byWord, description)
	return ruleSettingsSections(
		view.editorLabels.section(func() string { return view.application.text("studio.case_conversion", "Case conversion") }, func() string {
			return view.application.text("studio.case_conversion_subtitle", "Choose how selected letters change.")
		}, newResponsiveForm(
			view.editorLabels.formItem(func() string { return view.application.text("studio.mode", "Mode") }, mode),
		)),
		view.editorLabels.section(func() string { return view.application.text("studio.position_scope", "Position scope") }, func() string {
			return view.application.text("studio.position_scope_subtitle", "Choose which character positions are affected.")
		}, newResponsiveForm(
			view.editorLabels.formItem(func() string { return view.application.text("studio.positions", "Positions") }, positionEntry),
			view.editorLabels.formItem(func() string { return view.application.text("studio.scope", "Scope") }, byWord),
		)),
		view.editorLabels.section(func() string { return view.application.text("common.description", "Description") }, func() string {
			return view.application.text("studio.description_help", "Optional note saved with this rule.")
		}, newResponsiveForm(
			view.editorLabels.formItem(func() string { return view.application.text("common.description", "Description") }, description),
		)),
	)
}

func (view *StudioView) replaceEditor() fyne.CanvasObject {
	rule := view.draft.Rules[view.selectedRule]
	numeric := view.boundCheck(func() string { return view.application.text("studio.numbers", "Numbers") }, rule.Numeric, func(v bool) { view.draft.Rules[view.selectedRule].Numeric = v })
	alphabetical := view.boundCheck(func() string { return view.application.text("studio.letters", "Letters") }, rule.Alphabetical, func(v bool) { view.draft.Rules[view.selectedRule].Alphabetical = v })
	spaces := view.boundCheck(func() string { return view.application.text("studio.whitespace", "Whitespace") }, rule.Space, func(v bool) { view.draft.Rules[view.selectedRule].Space = v })
	special := view.boundCheck(func() string { return view.application.text("studio.special", "Special") }, rule.Special, func(v bool) { view.draft.Rules[view.selectedRule].Special = v })
	deduplicate := view.boundCheck(func() string { return view.application.text("studio.deduplicate", "Deduplicate matches") }, rule.Deduplicate, func(v bool) { view.draft.Rules[view.selectedRule].Deduplicate = v })
	trim := view.boundCheck(func() string { return view.application.text("studio.trim", "Trim result") }, rule.Trim, func(v bool) { view.draft.Rules[view.selectedRule].Trim = v })
	remove := view.boundEntry(strings.Join(rule.Remove, ""), func(v string) { view.draft.Rules[view.selectedRule].Remove = stringList(v) })
	preserve := view.boundEntry(strings.Join(rule.Preserve, ""), func(v string) { view.draft.Rules[view.selectedRule].Preserve = stringList(v) })
	replacement := view.boundEntry(rule.Replacement, func(v string) { view.draft.Rules[view.selectedRule].Replacement = v })
	description := view.boundEntry(rule.Description, func(v string) { view.draft.Rules[view.selectedRule].Description = v })
	view.disableForBuiltIn(numeric, alphabetical, spaces, special, deduplicate, trim, remove, preserve, replacement, description)
	return ruleSettingsSections(
		view.editorLabels.section(func() string { return view.application.text("studio.character_selection", "Character selection") }, func() string {
			return view.application.text("studio.character_selection_subtitle", "Choose the characters this rule should replace or preserve.")
		}, newResponsiveForm(
			view.editorLabels.formItem(func() string { return view.application.text("studio.categories", "Categories") }, container.NewGridWithColumns(2, numeric, alphabetical, spaces, special)),
			view.editorLabels.formItem(func() string { return view.application.text("studio.remove_runes", "Remove Runes") }, remove),
			view.editorLabels.formItem(func() string { return view.application.text("studio.preserve_runes", "Preserve runes") }, preserve),
			view.editorLabels.formItem(func() string { return view.application.text("studio.replacement", "Replacement") }, replacement),
		)),
		view.editorLabels.section(func() string { return view.application.text("studio.formatting", "Formatting") }, func() string {
			return view.application.text("studio.formatting_subtitle", "Control repeated replacements and surrounding whitespace.")
		}, newResponsiveForm(
			view.editorLabels.formItem(func() string { return view.application.text("studio.options", "Options") }, newVertical(spaceXS, deduplicate, trim)),
		)),
		view.editorLabels.section(func() string { return view.application.text("studio.excluded_matches", "Excluded matches") }, func() string {
			return view.application.text("studio.excluded_matches_subtitle", "Regex matches are preserved by this Replace Runes rule.")
		}, view.excludeMatchesEditor()),
		view.editorLabels.section(func() string { return view.application.text("common.description", "Description") }, func() string {
			return view.application.text("studio.description_help", "Optional note saved with this rule.")
		}, newResponsiveForm(
			view.editorLabels.formItem(func() string { return view.application.text("common.description", "Description") }, description),
		)),
	)
}

func ruleSettingsSections(sections ...fyne.CanvasObject) fyne.CanvasObject {
	return newVertical(spaceXL, sections...)
}

func (view *StudioView) excludeMatchesEditor() fyne.CanvasObject {
	ruleIndex := view.selectedRule
	table := NewTablePanel([]TableColumn{{Title: view.application.text("studio.regex", "Regex")}, {Title: view.application.text("common.action", "Action"), Width: deleteTableActionWidth(view.application)}}, 2)
	view.excludedTable = table
	rows := make([]fyne.CanvasObject, 0, len(view.draft.Rules[ruleIndex].ExcludeMatches))
	for index, expression := range view.draft.Rules[ruleIndex].ExcludeMatches {
		row := newIgnoredNameRow(view.application)
		row.name.Truncation = fyne.TextTruncateEllipsis
		if view.builtIn {
			row.set(index, expression, nil)
		} else {
			row.set(index, expression, func(matchIndex int) {
				if view.removeExcludeMatch(ruleIndex, matchIndex) {
					view.refreshEditor()
				}
			})
		}
		rows = append(rows, row)
	}
	table.SetRows(rows)

	input := widget.NewEntry()
	input.SetPlaceHolder(view.application.text("studio.regex_placeholder", `Regular expression, e.g. \b[0-9]{4}-[0-9]{2}\b`))
	view.editorLabels.add(func() {
		input.SetPlaceHolder(view.application.text("studio.regex_placeholder", `Regular expression, e.g. \b[0-9]{4}-[0-9]{2}\b`))
	})
	add := view.editorLabels.buttonIcon(func() string { return view.application.text("common.add", "Add") }, theme.ContentAddIcon(), func() {
		if err := view.addExcludeMatch(ruleIndex, input.Text); err != nil {
			view.feedback.setLiteral(view.status, err.Error(), view.content)
			return
		}
		input.SetText("")
		view.refreshEditor()
	})
	view.disableForBuiltIn(input, add)
	return container.NewBorder(nil, container.NewBorder(nil, nil, nil, add, input), nil, nil, table.Content())
}

func (view *StudioView) addExcludeMatch(ruleIndex int, expression string) error {
	if view.builtIn {
		return patternlib.ErrReadOnly
	}
	if ruleIndex < 0 || ruleIndex >= len(view.draft.Rules) || view.draft.Rules[ruleIndex].Kind != rules.KindReplaceRunes {
		return errors.New(view.application.text("studio.select_replace", "select a replace runes rule first"))
	}
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return errors.New(view.application.text("studio.exclude_empty", "excluded match must not be empty"))
	}
	if _, err := regexp.Compile(expression); err != nil {
		return fmt.Errorf(view.application.text("studio.exclude_invalid", "invalid excluded-match expression %q: %w"), expression, err)
	}
	for _, existing := range view.draft.Rules[ruleIndex].ExcludeMatches {
		if existing == expression {
			return fmt.Errorf(view.application.text("studio.exclude_duplicate", "excluded match %q is already listed"), expression)
		}
	}
	view.draft.Rules[ruleIndex].ExcludeMatches = append(view.draft.Rules[ruleIndex].ExcludeMatches, expression)
	view.markDirty()
	return nil
}

func (view *StudioView) removeExcludeMatch(ruleIndex, index int) bool {
	if view.builtIn || ruleIndex < 0 || ruleIndex >= len(view.draft.Rules) {
		return false
	}
	matches := view.draft.Rules[ruleIndex].ExcludeMatches
	if index < 0 || index >= len(matches) {
		return false
	}
	view.draft.Rules[ruleIndex].ExcludeMatches = append(matches[:index], matches[index+1:]...)
	view.markDirty()
	return true
}

func (view *StudioView) disableForBuiltIn(controls ...fyne.Disableable) {
	if !view.builtIn {
		return
	}
	for _, control := range controls {
		control.Disable()
	}
}

func (view *StudioView) boundEntry(initial string, set func(string)) *widget.Entry {
	entry := widget.NewEntry()
	entry.SetText(initial)
	entry.OnChanged = func(value string) {
		set(value)
		view.markDirty()
	}
	return entry
}

func (view *StudioView) boundCheck(label func() string, initial bool, set func(bool)) *widget.Check {
	check := view.editorLabels.check(label, nil)
	check.SetChecked(initial)
	check.OnChanged = func(value bool) {
		set(value)
		view.markDirty()
	}
	return check
}

func (view *StudioView) addTest() {
	input := strings.TrimSpace(view.testInput.Text)
	expected := view.testExpected.Text
	if input == "" || expected == "" {
		view.setStatus("studio.invalid_input", "Enter a valid input so the expected output can be calculated.")
		return
	}
	view.studioDraft.appendTest(pattern.TestCase{Input: input, Kind: nodeKindFromLabel(view.testKind.Selected, view.application), Expected: expected})
	view.testInput.SetText("")
	view.markDirty()
	view.refreshTests()
}

func (view *StudioView) refreshPendingTestExpected() {
	if view.testInput == nil || view.testExpected == nil || view.testKind == nil {
		return
	}
	input := strings.TrimSpace(view.testInput.Text)
	if input == "" {
		view.testExpected.SetText("")
		return
	}
	trace, err := pattern.Trace(view.draft, input, nodeKindFromLabel(view.testKind.Selected, view.application))
	if err != nil {
		view.testExpected.SetText("")
		return
	}
	view.testExpected.SetText(trace.Output)
}

func (view *StudioView) refreshTests() {
	if view.testsTable == nil {
		return
	}
	view.testResults = pattern.RunTests(view.draft)
	view.selectedTest = -1
	view.testsTable.UnselectAll()
	view.deleteTest.Disable()
	view.testsTable.Refresh()
}

func (view *StudioView) removeSelectedTest() {
	if !view.studioDraft.removeSelectedTest() {
		return
	}
	view.markDirty()
}

func (view *StudioView) newTestsTable() *widget.Table {
	headers := []string{view.application.text("common.kind", "Kind"), view.application.text("common.input", "Input"), view.application.text("common.expected", "Expected"), view.application.text("studio.actual", "Actual"), view.application.text("studio.result", "Result")}
	table := widget.NewTable(
		func() (int, int) { return len(view.testResults) + 1, len(headers) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.TableCellID, object fyne.CanvasObject) {
			label := object.(*widget.Label)
			if id.Row == 0 {
				label.TextStyle = fyne.TextStyle{Bold: true}
				label.SetText(view.testHeaders()[id.Col])
				return
			}
			label.TextStyle = fyne.TextStyle{}
			result := view.testResults[id.Row-1]
			kind := result.Case.Kind
			if kind == "" {
				kind = rename.NodeKindFile
			}
			status := "FAIL"
			if result.Passed {
				status = "PASS"
			} else if result.Error != "" {
				status = "ERROR"
			}
			values := []string{nodeKindLabel(kind, view.application), strconv.QuoteToGraphic(result.Case.Input), strconv.QuoteToGraphic(result.Case.Expected), strconv.QuoteToGraphic(result.Actual), testResultLabel(status, view.application)}
			label.SetText(values[id.Col])
		},
	)
	table.OnSelected = func(id widget.TableCellID) {
		if id.Row == 0 {
			table.Unselect(id)
			return
		}
		view.selectedTest = id.Row - 1
		if !view.builtIn {
			view.deleteTest.Enable()
		}
	}
	table.SetColumnWidth(0, maxFloat32(70, widestTableLabel(append(nodeKindOptions(view.application), headers[0]))))
	table.SetColumnWidth(1, 190)
	table.SetColumnWidth(2, 190)
	table.SetColumnWidth(3, 190)
	table.SetColumnWidth(4, maxFloat32(70, widestTableLabel([]string{headers[4], testResultLabel("PASS", view.application), testResultLabel("FAIL", view.application), testResultLabel("ERROR", view.application)})))
	return table
}

func (view *StudioView) importConfig() {
	if !view.requireClean() {
		return
	}
	picker := dialog.NewFileOpen(view.importSelected, view.application.window)
	picker.SetFilter(storage.NewExtensionFileFilter([]string{".json"}))
	view.application.showDialog(picker)
}

func (view *StudioView) importSelected(reader fyne.URIReadCloser, err error) {
	if err != nil {
		view.application.showError(err)
		return
	}
	if reader == nil {
		return
	}
	defer reader.Close()
	configuration, err := patternlib.DecodeBundle(reader)
	if err != nil {
		view.application.showError(err)
		return
	}
	view.resolveImport(configuration, 0, make(map[string]patternlib.ConflictAction))
}

func (view *StudioView) resolveImport(configuration patternlib.Bundle, index int, decisions map[string]patternlib.ConflictAction) {
	for index < len(configuration.Patterns) && !view.application.store.Has(configuration.Patterns[index].Name) {
		index++
	}
	if index >= len(configuration.Patterns) {
		if err := view.application.store.Import(configuration.Patterns, decisions); err != nil {
			view.application.showError(err)
			return
		}
		view.application.reloadPatternsAndConfiguration(configuration.Pattern, configuration.Files, configuration.Folders)
		view.application.showInformation(view.application.text("studio.import_complete", "Import complete"), view.application.texts().Plural("studio.import_count", "Imported {{.Count}} pattern definition.", "Imported {{.Count}} pattern definitions.", len(configuration.Patterns), map[string]any{"Count": len(configuration.Patterns)}))
		return
	}
	candidate := configuration.Patterns[index]
	key := strings.ToLower(strings.TrimSpace(candidate.Name))
	existing, _ := view.application.store.Lookup(candidate.Name)
	existingJSON, _ := json.MarshalIndent(existing.Spec, "", "  ")
	candidateJSON, _ := json.MarshalIndent(candidate, "", "  ")
	comparison := widget.NewMultiLineEntry()
	comparison.SetText(view.application.text("studio.import_existing", "Existing:\n") + string(existingJSON) + view.application.text("studio.import_candidate", "\n\nImported:\n") + string(candidateJSON))
	comparison.Disable()
	comparison.SetMinRowsVisible(10)
	var prompt dialog.Dialog
	choose := func(action patternlib.ConflictAction) {
		decisions[key] = action
		prompt.Hide()
		view.resolveImport(configuration, index+1, decisions)
	}
	replace := widget.NewButton(view.application.text("studio.replace", "Replace"), func() { choose(patternlib.ConflictReplace) })
	message := fmt.Sprintf(view.application.text("studio.conflict_message", "A pattern named %q already exists. Choose how to import it."), candidate.Name)
	if existing.BuiltIn {
		replace.Disable()
		message = fmt.Sprintf(view.application.text("studio.conflict_builtin", "The built-in pattern %q is read-only. Keep it or import the new definition as a copy."), candidate.Name)
	}
	content := container.NewVBox(
		widget.NewLabel(message),
		comparison,
		container.NewHBox(
			replace,
			widget.NewButton(view.application.text("studio.keep", "Keep existing"), func() { choose(patternlib.ConflictKeep) }),
			widget.NewButton(view.application.text("studio.copy", "Import as copy"), func() { choose(patternlib.ConflictCopy) }),
		),
	)
	prompt = dialog.NewCustomWithoutButtons(view.application.text("studio.conflict_title", "Pattern name conflict"), content, view.application.window)
	view.application.showDialog(prompt)
}

func (view *StudioView) exportConfig() {
	selected := view.patternPick.Selected
	if selected == "" {
		selected = "default"
	}
	_, files, folders := view.application.organizerConfiguration()
	configuration := patternlib.Bundle{
		Pattern: selected, Files: files, Folders: folders,
		Patterns: view.application.store.CustomSpecs(),
	}
	picker := dialog.NewFileSave(view.exportCallback(configuration), view.application.window)
	picker.SetFileName("file-folder-renamer-config.json")
	picker.SetFilter(storage.NewExtensionFileFilter([]string{".json"}))
	view.application.showDialog(picker)
}

func (view *StudioView) exportCallback(configuration patternlib.Bundle) func(fyne.URIWriteCloser, error) {
	return func(writer fyne.URIWriteCloser, err error) {
		view.exportSelected(configuration, writer, err)
	}
}

func (view *StudioView) exportSelected(configuration patternlib.Bundle, writer fyne.URIWriteCloser, err error) {
	if err != nil {
		view.application.showError(err)
		return
	}
	if writer == nil {
		return
	}
	defer writer.Close()
	if err := patternlib.EncodeBundle(writer, configuration); err != nil {
		view.application.showError(err)
	}
}

func parsePositions(value string, application *Application) ([]int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' })
	positions := make([]int, 0, len(parts))
	for _, part := range parts {
		position, err := strconv.Atoi(part)
		if err != nil || position < 0 {
			return nil, errors.New(application.text("studio.positions_invalid", "positions must be non-negative numbers separated by commas"))
		}
		positions = append(positions, position)
	}
	return positions, nil
}

func stringList(value string) []string {
	if value == "" {
		return nil
	}
	characters := []rune(value)
	values := make([]string, len(characters))
	for index, character := range characters {
		values[index] = string(character)
	}
	return values
}

func (view *StudioView) ruleRowLabel(index int) string {
	label := fmt.Sprintf("%d. %s", index+1, ruleKindLabel(view.draft.Rules[index].Kind, view.application))
	if index == view.selectedRule {
		return "• " + label
	}
	return label
}
