package ui

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/eightaugusto/file-folder-renamer/internal/localization"
	"github.com/eightaugusto/file-folder-renamer/internal/organizer"
	"github.com/eightaugusto/file-folder-renamer/internal/pattern"
	"github.com/eightaugusto/file-folder-renamer/internal/patternlib"
	"github.com/eightaugusto/file-folder-renamer/internal/rename"
	"github.com/eightaugusto/file-folder-renamer/internal/rules"
	"github.com/eightaugusto/file-folder-renamer/internal/settings"
	"github.com/eightaugusto/file-folder-renamer/internal/version"
)

func newTestApplication(t *testing.T, languages ...localization.Preference) *Application {
	t.Helper()
	return newThemedTestApplication(t, newOrganizerTheme(), languages...)
}

func newThemedTestApplication(t *testing.T, selectedTheme fyne.Theme, languages ...localization.Preference) *Application {
	t.Helper()
	fyneApp := test.NewTempApp(t)
	fyneApp.Settings().SetTheme(selectedTheme)
	patternsPath := filepath.Join(t.TempDir(), "patterns")
	store, err := patternlib.Open(patternsPath)
	if err != nil {
		t.Fatal(err)
	}
	application := &Application{
		app: fyneApp, window: fyneApp.NewWindow("test"), service: organizer.New(), store: store,
		settings: settings.DefaultConfig(patternsPath), configPath: filepath.Join(filepath.Dir(patternsPath), "config.json"),
	}
	preference := localization.PreferenceEnglish
	if len(languages) > 0 {
		preference = languages[0]
		application.settings.Language = preference
	}
	application.translator = localization.New(preference, "en-US")
	application.organizer = NewOrganizerView(application)
	application.studio = NewStudioView(application)
	application.settingsUI = NewSettingsView(application)
	t.Cleanup(func() {
		// Fyne's test driver executes dispatched callbacks on their caller's
		// goroutine. Drain application work before its global test app is closed.
		if done := application.organizer.previewDone; done != nil {
			<-done
		}
		if done := application.organizer.applyDone; done != nil {
			<-done
		}
	})
	return application
}

func TestOrganizerStartsInSafePreviewState(t *testing.T) {
	application := newTestApplication(t)
	if !application.organizer.apply.Disabled() {
		t.Fatal("apply must be disabled before a preview")
	}
	if application.organizer.patterns.Selected != "default" {
		t.Fatalf("unexpected default pattern: %q", application.organizer.patterns.Selected)
	}
	if !application.organizer.files.Checked || !application.organizer.folders.Checked {
		t.Fatal("files and folders must both be included by default")
	}
	if !containsString(application.settings.IgnoredNames, ".DS_Store") {
		t.Fatal("default ignored names must include .DS_Store")
	}
}

func TestWindowConfigurationUsesLargeDesktopLayout(t *testing.T) {
	application := newTestApplication(t)
	configureWindow(application.window)
	if application.window.FullScreen() {
		t.Fatal("application window must not start full-screen")
	}
	size := application.window.Canvas().Size()
	if size.Width < defaultWindowWidth || size.Height < defaultWindowHeight {
		t.Fatalf("window is too small for all components: %v", size)
	}
}

func TestAboutDisplaysApplicationVersionAndLicense(t *testing.T) {
	application := newTestApplication(t)
	application.shell = newApplicationShell(application)
	if application.shell.about == nil || application.shell.about.Text != "About" {
		t.Fatal("Application navigation must expose the localized About dialog")
	}

	wantVersion := "Version " + version.Display()
	seenVersion, seenLicense := false, false
	for _, object := range visibleObjects(aboutContent(application)) {
		label, ok := object.(*widget.Label)
		if !ok {
			continue
		}
		seenVersion = seenVersion || label.Text == wantVersion
		seenLicense = seenLicense || label.Text == "Licensed under the Apache License, Version 2.0."
	}
	if !seenVersion || !seenLicense {
		t.Fatalf("About content missing version or license: version=%v license=%v", seenVersion, seenLicense)
	}
}

func TestFolderPickerUsesHalfOfApplicationWindow(t *testing.T) {
	application := newTestApplication(t)
	application.window.Resize(fyne.NewSize(1500, 1000))
	size := folderPickerSize(application.window)
	if math.Abs(float64(size.Width-750)) > 0.01 || math.Abs(float64(size.Height-500)) > 0.01 {
		t.Fatalf("unexpected folder picker size: %v", size)
	}
}

func TestOrganizerSelectingFolderStartsPreview(t *testing.T) {
	application := newTestApplication(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "My File.TXT"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".DS_Store"), []byte("metadata"), 0o644); err != nil {
		t.Fatal(err)
	}

	application.organizer.selectFolder(root)
	done := application.organizer.previewDone
	if done == nil {
		t.Fatal("selecting a folder did not start a preview")
	}
	<-done

	if application.organizer.session == nil {
		t.Fatal("selecting a folder should create a preview automatically")
	}
	if application.organizer.session.Root() != root {
		t.Fatalf("previewed %q, want %q", application.organizer.session.Root(), root)
	}
	for _, item := range application.organizer.session.ProposalSnapshot() {
		if item.OriginalName == ".DS_Store" {
			t.Fatal("ignored file appeared in Organizer preview")
		}
	}
	_, columns := application.organizer.table.Length()
	if columns != 6 {
		t.Fatalf("rename preview must include an individual action column, got %d columns", columns)
	}
}

func TestOrganizerRejectsSingleFilePath(t *testing.T) {
	application := newTestApplication(t)
	file := filepath.Join(t.TempDir(), "one file.TXT")
	if err := os.WriteFile(file, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	application.organizer.selectFolder(file)
	if application.organizer.session != nil {
		t.Fatal("Organizer must not preview a single selected file")
	}
	if !strings.Contains(application.organizer.status.Text, "Choose a folder") {
		t.Fatalf("unexpected file-path validation message: %q", application.organizer.status.Text)
	}
}

func TestOrganizerProposalTableRendersHeadersAndValues(t *testing.T) {
	application := newTestApplication(t)
	view := application.organizer
	view.tableState.proposals = []rename.Proposal{{Kind: rename.NodeKindFile, RelativePath: "folder/example.txt", OriginalName: "example.txt", ProposedName: "Example.txt", Changed: true}}
	rows, columns := view.table.Length()
	if rows != 1 || columns != 6 {
		t.Fatalf("unexpected proposal-table dimensions: %d rows, %d columns", rows, columns)
	}
	if got := proposalTableHeaders()[proposalColumnAction]; got != "Action" {
		t.Fatalf("Action must be the first organizer column, got %q", got)
	}
	header := view.table.CreateHeader()
	view.table.UpdateHeader(widget.TableCellID{Row: -1, Col: proposalColumnOriginal}, header)
	if got := header.(*fyne.Container).Objects[0].(*widget.Button).Text; got != "Original" {
		t.Fatalf("proposal table header did not render: %q", got)
	}
	locationHeader := view.table.CreateHeader()
	view.table.UpdateHeader(widget.TableCellID{Row: -1, Col: proposalColumnLocation}, locationHeader)
	if got := locationHeader.(*fyne.Container).Objects[0].(*widget.Button).Icon; got != theme.MenuDropUpIcon() {
		t.Fatalf("default Location sort must invert its header triangle upward, got %v", got)
	}
	view.toggleProposalSort(proposalColumnLocation)
	view.table.UpdateHeader(widget.TableCellID{Row: -1, Col: proposalColumnLocation}, locationHeader)
	if got := locationHeader.(*fyne.Container).Objects[0].(*widget.Button).Icon; got != theme.MenuDropDownIcon() {
		t.Fatalf("descending Location sort must invert its header triangle downward, got %v", got)
	}
	filterHeader := view.table.CreateHeader()
	view.table.UpdateHeader(widget.TableCellID{Row: -1, Col: proposalColumnStatus}, filterHeader)
	filter := filterHeader.(*fyne.Container).Objects[1].(*widget.Button)
	if !filter.Visible() || filter.Icon != theme.MenuIcon() {
		t.Fatalf("Status filter must use a compact header filter arrow: %+v", filter)
	}
	cell := view.table.CreateCell()
	view.table.UpdateCell(widget.TableCellID{Row: 0, Col: proposalColumnProposed}, cell)
	if got := cell.(*fyne.Container).Objects[0].(*widget.Label).Text; got != "Example.txt" {
		t.Fatalf("proposal table value did not render: %q", got)
	}
	actionCell := view.table.CreateCell()
	view.table.UpdateCell(widget.TableCellID{Row: 0, Col: proposalColumnAction}, actionCell)
	actionHost := actionCell.(*fyne.Container).Objects[1].(*fyne.Container)
	if !actionHost.Objects[0].(*widget.Button).Visible() {
		t.Fatal("changed proposal did not render its individual Apply button")
	}
	kindCell := view.table.CreateCell()
	view.table.UpdateCell(widget.TableCellID{Row: 0, Col: proposalColumnKind}, kindCell)
	if got := kindCell.(*fyne.Container).Objects[0].(*widget.Label).Text; got != "File" {
		t.Fatalf("kind must use a capitalized enum value, got %q", got)
	}
	kindWidth := proposalFilterColumnWidth([]string{"Kind", "File", "Folder"}, proposalFilterOptions(proposalColumnKind))
	statusWidth := proposalFilterColumnWidth([]string{"Status", "Changed", "Unchanged"}, proposalFilterOptions(proposalColumnStatus))
	actionWidth := maxFloat32(proposalActionColumnWidth(application), proposalFilterColumnWidth([]string{"Action"}, proposalFilterOptions(proposalColumnAction)))
	fixedWidth := kindWidth + statusWidth + actionWidth + theme.Padding()*5
	defaultFlexibleWidth := defaultTableTextWidth(30) * 3
	view.updateProposalTableColumns(fixedWidth + defaultFlexibleWidth)
	defaultAction := view.columnWidths[proposalColumnAction]
	defaultKind := view.columnWidths[proposalColumnKind]
	defaultStatus := view.columnWidths[proposalColumnStatus]
	if view.table.StickyColumnCount != proposalStickyColumnCount {
		t.Fatalf("expected %d frozen organizer columns, got %d", proposalStickyColumnCount, view.table.StickyColumnCount)
	}
	if view.columnWidths[proposalColumnKind] != kindWidth {
		t.Fatal("Kind width must fit its enum values and in-header filter")
	}
	if view.columnWidths[proposalColumnStatus] != statusWidth {
		t.Fatal("Status width must fit its enum values and in-header filter")
	}
	if view.columnWidths[proposalColumnAction] != actionWidth {
		t.Fatal("Action width must fit its Apply button and in-header filter")
	}
	if view.columnWidths[proposalColumnLocation] != defaultTableTextWidth(30) || view.columnWidths[proposalColumnOriginal] != defaultTableTextWidth(30) || view.columnWidths[proposalColumnProposed] != defaultTableTextWidth(30) {
		t.Fatal("Location, Original, and Proposed must use their 30-character default widths")
	}
	view.updateProposalTableColumns(fixedWidth + defaultFlexibleWidth + 500)
	if view.columnWidths[proposalColumnKind] != defaultKind || view.columnWidths[proposalColumnStatus] != defaultStatus || view.columnWidths[proposalColumnAction] != defaultAction {
		t.Fatal("Kind, Status, and Action must remain fixed while Location expands")
	}
	if view.columnWidths[proposalColumnLocation] <= defaultTableTextWidth(30) || view.columnWidths[proposalColumnOriginal] != defaultTableTextWidth(30) || view.columnWidths[proposalColumnProposed] != defaultTableTextWidth(30) {
		t.Fatal("Location must receive unused table width")
	}
	view.updateProposalTableColumns(fixedWidth + defaultFlexibleWidth/2)
	if view.columnWidths[proposalColumnKind] != defaultKind || view.columnWidths[proposalColumnStatus] != defaultStatus || view.columnWidths[proposalColumnAction] != defaultAction {
		t.Fatal("Kind, Status, and Action must remain fixed while other columns shrink")
	}
	if math.Abs(float64(view.columnWidths[proposalColumnLocation]+view.columnWidths[proposalColumnOriginal]+view.columnWidths[proposalColumnProposed]-defaultFlexibleWidth/2)) > 0.01 {
		t.Fatal("Location, Original, and Proposed must absorb all width reductions")
	}
}

func TestOrganizerPreviewSortsAndFiltersAllColumns(t *testing.T) {
	application := newTestApplication(t)
	view := application.organizer
	view.tableState.proposals = []rename.Proposal{
		{Kind: rename.NodeKindFolder, RelativePath: "zeta", OriginalName: "Zoo", ProposedName: "Zoo", Changed: false},
		{Kind: rename.NodeKindFile, RelativePath: "alpha", OriginalName: "Beta", ProposedName: "Alpha", Changed: true},
		{Kind: rename.NodeKindFile, RelativePath: "middle", OriginalName: "Alpha", ProposedName: "Middle", Changed: false},
	}
	if view.tableState.sortColumn != proposalColumnLocation || !view.tableState.sortAscending {
		t.Fatal("Organizer must initially sort rename proposals by Location ascending")
	}
	indexes := view.visibleProposalIndexes()
	if got := view.tableState.proposals[indexes[0]].RelativePath; got != "alpha" {
		t.Fatalf("default Location sort started with %q, want alpha", got)
	}

	view.setProposalFilters(proposalColumnKind, []string{"File"})
	if rows, _ := view.table.Length(); rows != 2 {
		t.Fatalf("kind filter showed %d rows, want 2", rows)
	}
	view.setProposalFilters(proposalColumnKind, []string{"File", "Folder"})
	if rows, _ := view.table.Length(); rows != 3 {
		t.Fatalf("multi-value kind filter showed %d rows, want 3", rows)
	}
	view.setProposalFilters(proposalColumnKind, []string{"File"})
	view.setProposalFilters(proposalColumnStatus, []string{"Changed"})
	if rows, _ := view.table.Length(); rows != 1 {
		t.Fatalf("status filter showed %d rows, want 1", rows)
	}
	view.setProposalFilters(proposalColumnStatus, nil)
	view.setProposalFilters(proposalColumnAction, []string{"No action"})
	if rows, _ := view.table.Length(); rows != 1 {
		t.Fatalf("action filter showed %d rows, want 1", rows)
	}
	view.setProposalFilters(proposalColumnKind, nil)
	view.setProposalFilters(proposalColumnAction, nil)

	for column := range proposalTableHeaders() {
		view.tableState.sortColumn = column
		view.tableState.sortAscending = true
		indexes := view.visibleProposalIndexes()
		for index := 1; index < len(indexes); index++ {
			previous := proposalSortValue(view.tableState.proposals[indexes[index-1]], column)
			current := proposalSortValue(view.tableState.proposals[indexes[index]], column)
			if previous > current {
				t.Fatalf("column %q was not sorted ascending: %q before %q", proposalTableHeaders()[column], previous, current)
			}
		}
		view.tableState.sortAscending = false
		indexes = view.visibleProposalIndexes()
		for index := 1; index < len(indexes); index++ {
			previous := proposalSortValue(view.tableState.proposals[indexes[index-1]], column)
			current := proposalSortValue(view.tableState.proposals[indexes[index]], column)
			if previous < current {
				t.Fatalf("column %q was not sorted descending: %q before %q", proposalTableHeaders()[column], previous, current)
			}
		}
	}
	view.status.SetText("Scanning selection…")
	view.toggleProposalSort(proposalColumnOriginal)
	if view.status.Text != "Scanning selection…" {
		t.Fatal("sorting must not replace the operational status message")
	}
}

func TestPatternStudioCreatesTestsAndSaves(t *testing.T) {
	application := newTestApplication(t)
	studio := application.studio
	if err := studio.newPattern("lowercase"); err != nil {
		t.Fatal(err)
	}
	studio.addRule(rules.KindCase)
	studio.testInput.SetText("FOLDER")
	studio.testKind.SetSelected(nodeKindFolderLabel)
	studio.addTest()
	if err := studio.SaveCurrent(); err != nil {
		t.Fatal(err)
	}
	if studio.IsDirty() {
		t.Fatal("successful save should clear dirty state")
	}
	entry, err := application.store.Lookup("lowercase")
	if err != nil {
		t.Fatal(err)
	}
	if len(entry.Spec.Tests) != 1 || entry.Spec.Tests[0].Kind != rename.NodeKindFolder {
		t.Fatalf("folder test kind was not saved: %+v", entry.Spec.Tests)
	}
	if _, err := os.Stat(filepath.Join(application.settings.PatternsPath, "lowercase.tests.json")); err != nil {
		t.Fatalf("separate test-case JSON was not created: %v", err)
	}
}

func TestPatternStudioBuiltinsAreReadOnly(t *testing.T) {
	application := newTestApplication(t)
	studio := application.studio
	if !studio.builtIn || !studio.addRuleButton.Disabled() || studio.testInput.Disabled() || !studio.addTestButton.Disabled() || !studio.delete.Disabled() {
		t.Fatal("built-in pattern editing controls must be disabled")
	}
	if err := studio.SaveCurrent(); !errors.Is(err, patternlib.ErrReadOnly) {
		t.Fatalf("expected read-only save error, got %v", err)
	}
	if len(studio.testResults) < 100 {
		t.Fatalf("default pattern should expose its embedded saved tests, got %d", len(studio.testResults))
	}
	studio.testInput.SetText("preview only.txt")
	if studio.testExpected.Text == "" {
		t.Fatal("built-in test input must calculate an output without saving")
	}
	studio.selectedRule = 1
	studio.refreshEditor()
	scroll, ok := studio.editorHost.Objects[0].(*container.Scroll)
	if !ok {
		t.Fatalf("built-in rule settings should use the normal form container, got %T", studio.editorHost.Objects[0])
	}
	editor, ok := scroll.Content.(*fyne.Container).Objects[0].(*fyne.Container)
	if !ok {
		t.Fatalf("built-in replace-runes settings should render as grouped sections, got %T", scroll.Content)
	}
	if len(editor.Objects) != 4 {
		t.Fatalf("replace-runes settings must separate regex from surrounding fields, got %d sections", len(editor.Objects))
	}
	regexSection, ok := editor.Objects[2].(*fyne.Container)
	if !ok || regexSection.Objects[0].(*fyne.Container).Objects[0].(*widget.Label).Text != "Excluded matches" {
		t.Fatalf("replace-runes regex section must be a distinct section, got %T (%+v)", editor.Objects[2], regexSection)
	}
	if count := assertDisabledFormControls(t, editor); count == 0 {
		t.Fatal("read-only rule form did not contain any disabled controls")
	}
}

func assertDisabledFormControls(t *testing.T, object fyne.CanvasObject) int {
	t.Helper()
	count := 0
	switch typed := object.(type) {
	case *fyne.Container:
		for _, child := range typed.Objects {
			count += assertDisabledFormControls(t, child)
		}
		return count
	case *widget.Form:
		for _, item := range typed.Items {
			count += assertDisabledFormControls(t, item.Widget)
		}
		return count
	case *widget.Card:
		return assertDisabledFormControls(t, typed.Content)
	}
	if control, ok := object.(fyne.Disableable); ok {
		count++
		if !control.Disabled() {
			t.Fatalf("read-only form control is enabled: %T", object)
		}
	}
	return count
}

func TestPatternStudioUsesSavedTestsAsValidationTable(t *testing.T) {
	application := newTestApplication(t)
	studio := application.studio
	if !reflect.DeepEqual(studio.ruleKind.Options, []string{ruleKindCaseLabel, ruleKindRemoveDiacriticsLabel, ruleKindReplaceRunesLabel}) || studio.ruleKind.Selected != ruleKindReplaceRunesLabel {
		t.Fatalf("rule selector must use friendly labels: options=%v selected=%q", studio.ruleKind.Options, studio.ruleKind.Selected)
	}
	if !reflect.DeepEqual(studio.testKind.Options, []string{nodeKindFileLabel, nodeKindFolderLabel}) || studio.testKind.Selected != nodeKindFileLabel {
		t.Fatalf("test-kind selector must use friendly labels: options=%v selected=%q", studio.testKind.Options, studio.testKind.Selected)
	}
	if err := studio.newPattern("table test"); err != nil {
		t.Fatal(err)
	}
	studio.addRule(rules.KindCase)
	if got := studio.rulesHost.Objects[0].(*fyne.Container).Objects[0].(*widget.Button).Text; got != "• 1. Case" {
		t.Fatalf("rule pipeline must use friendly labels, got %q", got)
	}
	studio.testInput.SetText("FILE.TXT")
	studio.addTest()
	testRows, testColumns := studio.testsTable.Length()
	if testRows != 2 || testColumns != 5 {
		t.Fatalf("unexpected tests table dimensions: %dx%d", testRows, testColumns)
	}
	if studio.workspaceSplit.Offset != 0.5 {
		t.Fatalf("Pattern Studio should start with equal top/bottom workspaces: top=%v", studio.workspaceSplit.Offset)
	}
	if studio.addRuleButton.Text != "" || studio.addRuleButton.MinSize() != widget.NewButtonWithIcon("", theme.MoveUpIcon(), nil).MinSize() {
		t.Fatal("Add rule must use the same compact icon-button size as rule actions")
	}
}

func TestPatternStudioHelpDocumentsEveryAvailableRule(t *testing.T) {
	application := newTestApplication(t)
	studio := application.studio
	if studio.helpButton == nil || studio.helpButton.Text != "Help" {
		t.Fatal("Pattern Studio must expose a Help button")
	}
	documentation := patternStudioPipelineHelpMarkdown + caseRuleHelpMarkdown + replaceRunesRuleHelpMarkdown + removeDiacriticsRuleHelpMarkdown
	for _, expected := range []string{
		"rule pipeline", "ordered list of rules", ruleKindReplaceRunesLabel, ruleKindCaseLabel, ruleKindRemoveDiacriticsLabel,
		"Excluded matches", "Deduplicate matches", "Positions", "Apply positions to each word",
	} {
		if !strings.Contains(strings.ToLower(documentation), strings.ToLower(expected)) {
			t.Fatalf("Pattern Studio help is missing %q", expected)
		}
	}
	if strings.Contains(documentation, "replace_runes") {
		t.Fatal("Help must use the user-facing Replace Runes name, not the enum value")
	}
	accordion := patternStudioHelpAccordion(application)
	if accordion.MultiOpen || len(accordion.Items) != 3 {
		t.Fatalf("Pattern Studio help must have three exclusive rule sections: %+v", accordion)
	}
	if got := []string{accordion.Items[0].Title, accordion.Items[1].Title, accordion.Items[2].Title}; !reflect.DeepEqual(got, []string{ruleKindCaseLabel, ruleKindRemoveDiacriticsLabel, ruleKindReplaceRunesLabel}) {
		t.Fatalf("Pattern Studio help rules must be alphabetical: %v", got)
	}
	accordion.Open(0)
	accordion.Open(1)
	if accordion.Items[0].Open || !accordion.Items[1].Open {
		t.Fatal("opening a rule section must close the previously open section")
	}
	content := patternStudioHelpContent(application)
	if content.MinSize().Width > 760 || content.MinSize().Height > 560 {
		t.Fatalf("Pattern Studio help must fit a narrow window without imposing a large minimum: %v", content.MinSize())
	}
}

func TestRuleEditorsUseConsistentSections(t *testing.T) {
	application := newTestApplication(t)
	studio := application.studio
	if err := studio.newPattern("section cards"); err != nil {
		t.Fatal(err)
	}
	studio.addRule(rules.KindCase)
	assertRuleSettingsSectionTitles(t, studio.caseEditor(), []string{"Case conversion", "Position scope", "Description"})
	studio.addRule(rules.KindReplaceRunes)
	assertRuleSettingsSectionTitles(t, studio.replaceEditor(), []string{"Character selection", "Formatting", "Excluded matches", "Description"})
	studio.addRule(rules.KindRemoveDiacritics)
	assertRuleSettingsSectionTitles(t, studio.removeDiacriticsEditor(), []string{"Scripts", "Behavior", "Description"})
}

func TestRemoveDiacriticsRequiresAndAppliesSelectedScript(t *testing.T) {
	application := newTestApplication(t)
	studio := application.studio
	if err := studio.newPattern("diacritics"); err != nil {
		t.Fatal(err)
	}
	studio.addRule(rules.KindRemoveDiacritics)
	if studio.draft.Rules[0].Latin || studio.latin == nil || studio.latin.Checked {
		t.Fatal("new diacritics rule must not enable a script by default")
	}
	if studio.validationErr == nil || studio.SaveCurrent() == nil {
		t.Fatal("diacritics rule without an enabled script must fail validation")
	}

	studio.latin.SetChecked(true)
	if !studio.draft.Rules[0].Latin || studio.validationErr != nil {
		t.Fatalf("enabling Latin did not restore validity: %+v", studio.validationErr)
	}
	studio.testInput.SetText("Pokémon.txt")
	if studio.testExpected.Text != "Pokemon.txt" {
		t.Fatalf("unexpected Latin preview: %q", studio.testExpected.Text)
	}
	if err := studio.SaveCurrent(); err != nil {
		t.Fatalf("save with Latin enabled: %v", err)
	}
}

func assertRuleSettingsSectionTitles(t *testing.T, editor fyne.CanvasObject, expected []string) {
	t.Helper()
	sections, ok := editor.(*fyne.Container)
	if !ok || len(sections.Objects) != len(expected) {
		t.Fatalf("expected %d rule-setting sections, got %T with %d objects", len(expected), editor, len(sections.Objects))
	}
	for index, title := range expected {
		section, ok := sections.Objects[index].(*fyne.Container)
		if !ok || section.Objects[0].(*fyne.Container).Objects[0].(*widget.Label).Text != title {
			t.Fatalf("section %d: got %T (%+v), want section %q", index, sections.Objects[index], section, title)
		}
	}
}

func TestSavedTestExpectedIsCalculatedAndActualRefreshesWithRules(t *testing.T) {
	application := newTestApplication(t)
	studio := application.studio
	if err := studio.newPattern("calculated tests"); err != nil {
		t.Fatal(err)
	}
	studio.testInput.SetText("FILE.TXT")
	if !studio.testExpected.Disabled() {
		t.Fatal("expected output field must always be read-only")
	}
	if studio.testExpected.Text != "FILE.txt" {
		t.Fatalf("unexpected calculated expected output: %q", studio.testExpected.Text)
	}
	studio.addTest()
	if len(studio.draft.Tests) != 1 || studio.draft.Tests[0].Expected != "FILE.txt" {
		t.Fatalf("calculated expected output was not saved: %+v", studio.draft.Tests)
	}

	studio.addRule(rules.KindCase)
	if len(studio.testResults) != 1 || studio.testResults[0].Actual != "file.txt" || studio.testResults[0].Passed {
		t.Fatalf("saved test result was not recalculated after adding a rule: %+v", studio.testResults)
	}
}

func TestSettingsUISavesPatternsBesideSettingsFile(t *testing.T) {
	application := newTestApplication(t)
	patternsPath := filepath.Join(filepath.Dir(application.configPath), "patterns")
	application.settingsUI.files.SetChecked(true)
	application.settingsUI.folders.SetChecked(true)
	application.settingsUI.ignoreInput.SetText("*.tmp")
	application.settingsUI.addIgnoredName()
	if err := application.settingsUI.Save(); err != nil {
		t.Fatal(err)
	}
	assertStoreWritesTo(t, application.store, patternsPath)
	data, err := os.ReadFile(application.configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" {
		t.Fatal("settings UI did not write config.json")
	}
	if !containsString(application.settings.IgnoredNames, "*.tmp") {
		t.Fatal("settings UI did not persist the added ignored-name pattern")
	}
}

func TestIgnoredNameRowDeletesFromActionColumn(t *testing.T) {
	application := newTestApplication(t)
	view := application.settingsUI
	if rows, columns := len(view.ignoredTable.rows.Objects), len(view.ignoredTable.columns); rows != len(view.ignoredNames)*2-1 || columns != 2 {
		t.Fatalf("ignored names must use a two-column table, got %dx%d", rows, columns)
	}
	initial := len(view.ignoredNames)
	row := newIgnoredNameRow(application)
	row.Resize(fyne.NewSize(400, 40))
	row.set(0, view.ignoredNames[0], view.removeIgnoredName)
	if !row.remove.Visible() || row.remove.Text != "Delete" {
		t.Fatal("delete action must always be visible and explicitly labelled")
	}
	row.CreateRenderer().Layout(fyne.NewSize(400, 40))
	if row.remove.Position().X <= row.name.Position().X {
		t.Fatalf("delete control must be on the right: name=%v delete=%v", row.name.Position(), row.remove.Position())
	}
	row.remove.OnTapped()
	if len(view.ignoredNames) != initial-1 {
		t.Fatal("delete action did not remove the ignored name")
	}
}

func TestIgnoredNamesAreAlwaysVisibleAsATable(t *testing.T) {
	application := newTestApplication(t)
	table := application.settingsUI.ignoredTable
	if table == nil {
		t.Fatal("ignored names table was not created")
	}
	if len(table.columns) != 2 || table.columns[0].Title != "Ignored name" || table.columns[1].Title != "Action" {
		t.Fatal("ignored names must be shown in a two-column non-collapsible table")
	}
}

func TestSettingsLoadsReselectedFolder(t *testing.T) {
	application := newTestApplication(t)
	view := application.settingsUI
	originalPath := application.configPath
	selectedFolder := filepath.Join(t.TempDir(), "alternate")
	if err := os.MkdirAll(selectedFolder, 0o755); err != nil {
		t.Fatal(err)
	}
	selectedPath := filepath.Join(selectedFolder, "config.json")
	patternsPath := filepath.Join(t.TempDir(), "alternate-patterns")
	configuration := settings.DefaultConfig(patternsPath)
	configuration.IgnoredNames = []string{"*.cache"}
	if err := settings.Save(selectedPath, configuration); err != nil {
		t.Fatal(err)
	}
	if err := view.loadSettingsFolder(selectedFolder); err != nil {
		t.Fatal(err)
	}
	if view.selectedConfig != selectedPath || view.settingsFolder.Text != selectedFolder || !containsString(view.ignoredNames, "*.cache") {
		t.Fatalf("selected config was not loaded into the form: path=%q ignored=%v", view.selectedConfig, view.ignoredNames)
	}
	if got, want := view.patternsPath(), filepath.Join(filepath.Dir(selectedPath), "patterns"); got != want {
		t.Fatalf("patterns must be derived from the selected settings file: got %q, want %q", got, want)
	}
	if application.configPath != originalPath {
		t.Fatal("selected config must not become active before Save settings")
	}
	if err := view.Save(); err != nil {
		t.Fatal(err)
	}
	if application.configPath != selectedPath {
		t.Fatalf("saved selected config did not become active: %q", application.configPath)
	}
	assertStoreWritesTo(t, application.store, filepath.Join(filepath.Dir(selectedPath), "patterns"))
}

func assertStoreWritesTo(t *testing.T, store *patternlib.Store, directory string) {
	t.Helper()
	spec := pattern.Spec{Name: "location probe", Rules: []pattern.RuleSpec{{Kind: rules.KindCase, Mode: rules.CaseModeLower}}}
	if err := store.Save(spec); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(directory, "location-probe.json")); err != nil {
		t.Fatalf("pattern store did not write to %q: %v", directory, err)
	}
}

func TestSettingsRejectsInvalidFolderSelection(t *testing.T) {
	application := newTestApplication(t)
	view := application.settingsUI
	originalSelection := view.selectedConfig
	filePath := filepath.Join(t.TempDir(), "not-a-folder")
	if err := os.WriteFile(filePath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := view.loadSettingsFolder(filePath); err == nil {
		t.Fatal("file selection must be rejected")
	}
	invalidFolder := filepath.Join(t.TempDir(), "invalid")
	if err := os.MkdirAll(invalidFolder, 0o755); err != nil {
		t.Fatal(err)
	}
	invalidPath := filepath.Join(invalidFolder, "config.json")
	if err := os.WriteFile(invalidPath, []byte(`{"patterns_path":"relative","include_files":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := view.loadSettingsFolder(invalidFolder); err == nil {
		t.Fatal("folder with invalid config.json must be rejected")
	}
	if view.selectedConfig != originalSelection {
		t.Fatal("invalid selection changed the active config draft")
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func TestPatternStudioRuleSelectionAndDiscardAreNotDirty(t *testing.T) {
	application := newTestApplication(t)
	studio := application.studio
	studio.selectedRule = 0
	studio.refreshEditor()
	if studio.IsDirty() {
		t.Fatal("opening a rule editor must not change the pattern")
	}
	if err := studio.newPattern("discarded pattern"); err != nil {
		t.Fatal(err)
	}
	if !studio.IsDirty() {
		t.Fatal("new pattern should be dirty")
	}
	studio.discardChanges()
	if studio.IsDirty() || studio.draft.Name != "default" {
		t.Fatalf("discard should restore the default pattern: dirty=%v name=%q", studio.IsDirty(), studio.draft.Name)
	}
}

func TestPatternStudioNewPromptsForName(t *testing.T) {
	application := newTestApplication(t)
	studio := application.studio
	test.Tap(studio.newButton)
	if studio.namePrompt == nil || studio.namePromptInput == nil {
		t.Fatal("New must open a pattern-name prompt")
	}
	if studio.draft.Name != "default" {
		t.Fatal("pattern draft must not be created before the name is confirmed")
	}
	studio.namePrompt.Hide()
}

func TestReplaceRuneSelectorsAreStoredAsOneCharacterEntries(t *testing.T) {
	values := stringList("-()")
	expected := []string{"-", "(", ")"}
	if !reflect.DeepEqual(values, expected) {
		t.Fatalf("unexpected selector entries: got %#v, want %#v", values, expected)
	}
}

func TestPatternStudioExcludedMatchesValidateAndRefreshSavedTests(t *testing.T) {
	application := newTestApplication(t)
	studio := application.studio
	if err := studio.newPattern("date spacing"); err != nil {
		t.Fatal(err)
	}
	studio.addRule(rules.KindReplaceRunes)
	ruleIndex := studio.selectedRule
	studio.draft.Rules[ruleIndex].Remove = []string{"-"}
	studio.draft.Rules[ruleIndex].Replacement = " - "
	studio.testInput.SetText("report-2025-12-05-final.txt")
	studio.addTest()

	if err := studio.addExcludeMatch(ruleIndex, `\b[0-9]{4}-[0-9]{2}(?:-[0-9]{2})?\b`); err != nil {
		t.Fatal(err)
	}
	if got, want := studio.draft.Rules[ruleIndex].ExcludeMatches, []string{`\b[0-9]{4}-[0-9]{2}(?:-[0-9]{2})?\b`}; !reflect.DeepEqual(got, want) {
		t.Fatalf("excluded matches mismatch: got %#v, want %#v", got, want)
	}
	if len(studio.testResults) != 1 || studio.testResults[0].Actual != "report - 2025-12-05 - final.txt" {
		t.Fatalf("saved tests did not refresh after adding exclusion: %+v", studio.testResults)
	}
	if err := studio.addExcludeMatch(ruleIndex, "["); err == nil {
		t.Fatal("invalid regular expression was accepted")
	}
	if err := studio.addExcludeMatch(ruleIndex, `\b[0-9]{4}-[0-9]{2}(?:-[0-9]{2})?\b`); err == nil {
		t.Fatal("duplicate regular expression was accepted")
	}
	if !studio.removeExcludeMatch(ruleIndex, 0) || len(studio.draft.Rules[ruleIndex].ExcludeMatches) != 0 {
		t.Fatalf("excluded match was not removed: %#v", studio.draft.Rules[ruleIndex].ExcludeMatches)
	}

	studio.loadPattern("default")
	if err := studio.addExcludeMatch(1, "date"); !errors.Is(err, patternlib.ErrReadOnly) {
		t.Fatalf("built-in pattern must reject excluded-match edits, got %v", err)
	}
}
