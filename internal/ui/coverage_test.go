package ui

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/eightaugusto/file-folder-renamer/internal/localization"
	"github.com/eightaugusto/file-folder-renamer/internal/organizer"
	"github.com/eightaugusto/file-folder-renamer/internal/pattern"
	"github.com/eightaugusto/file-folder-renamer/internal/patternlib"
	"github.com/eightaugusto/file-folder-renamer/internal/rename"
	"github.com/eightaugusto/file-folder-renamer/internal/rules"
	"github.com/eightaugusto/file-folder-renamer/internal/settings"
)

func dismissTop(app *Application) {
	app.dismissTopModal(nil)
}

func TestApplicationAndDialogCoverage(t *testing.T) {
	app := newTestApplication(t)
	app.shell = newApplicationShell(app)
	app.window.SetContent(app.shell.content)
	app.showAbout()
	dismissTop(app)
	app.showError(errors.New("failure"))
	dismissTop(app)
	app.installShortcuts()
	app.openShortcut(nil)
	dismissTop(app)
	app.saveShortcut(nil)
	if patternName, files, folders := (&Application{}).organizerConfiguration(); patternName != "" || files || folders {
		t.Fatal("nil organizer configuration")
	}
	(&Application{}).setOrganizerConfiguration("x", true, true)
	app.organizer.OpenPath()
	dismissTop(app)
	app.settingsUI.chooseSettingsFolder()
	dismissTop(app)
	app.studio.promptNewPattern()
	dismissTop(app)
	app.studio.duplicatePattern()
	dismissTop(app)
	app.studio.showHelp()
	dismissTop(app)
}

func TestRunCompositionRoot(t *testing.T) {
	previousNew, previousBootstrap := newDesktopApp, bootstrapApp
	previousOpen, previousRun := openPatternLibrary, runDesktopApp
	t.Cleanup(func() {
		newDesktopApp, bootstrapApp = previousNew, previousBootstrap
		openPatternLibrary, runDesktopApp = previousOpen, previousRun
	})
	runs := 0
	runDesktopApp = func(fyne.App) { runs++ }
	defaultTestApp := test.NewTempApp(t)
	previousRun(defaultTestApp)
	created := previousNew()
	created.Quit()
	bootstrapErr := errors.New("bootstrap")
	bootstrapApp = func() (settings.Layout, error) { return settings.Layout{}, bootstrapErr }
	newDesktopApp = func() fyne.App { return test.NewTempApp(t) }
	Run()

	root := t.TempDir()
	configuration := settings.DefaultConfig(filepath.Join(root, "patterns"))
	bootstrapApp = func() (settings.Layout, error) {
		return settings.Layout{PatternsRoot: configuration.PatternsPath, ConfigPath: filepath.Join(root, "config.json"), Config: configuration}, nil
	}
	openPatternLibrary = func(string) (*patternlib.Store, error) { return nil, errors.New("library") }
	newDesktopApp = func() fyne.App { return test.NewTempApp(t) }
	Run()
	openPatternLibrary = func(string) (*patternlib.Store, error) { return nil, nil }
	newDesktopApp = func() fyne.App { return test.NewTempApp(t) }
	Run()

	store, err := patternlib.Open(configuration.PatternsPath)
	if err != nil {
		t.Fatal(err)
	}
	openPatternLibrary = func(string) (*patternlib.Store, error) { return store, errors.New("recoverable library warning") }
	newDesktopApp = func() fyne.App { return test.NewTempApp(t) }
	Run()
	if runs != 4 {
		t.Fatalf("event loop runs = %d", runs)
	}
}

func TestOrganizerAsyncOperationCoverage(t *testing.T) {
	app := newTestApplication(t)
	view := app.organizer
	root := t.TempDir()
	proposal := rename.Proposal{
		SourcePath: filepath.Join(root, "old.txt"), ParentDir: root, RelativePath: "old.txt",
		OriginalName: "old.txt", ProposedName: "new.txt", Kind: rename.NodeKindFile, Changed: true,
	}
	session := organizer.NewPreviewSession(root, "default", []rename.Proposal{proposal})

	app.service = immediateService{session: session}
	view.path.SetText(root)
	view.StartPreview()
	<-view.previewDone
	if view.session == nil {
		t.Fatal("preview session was not delivered")
	}

	for _, failure := range []error{context.Canceled, errors.New("preview failure")} {
		app.service = immediateService{err: failure}
		view.StartPreview()
		<-view.previewDone
		dismissTop(app)
	}

	view.path.SetText("")
	view.session = session
	view.tableState.setProposals(session.ProposalSnapshot())
	app.service = immediateService{result: organizer.ApplyResult{AppliedCount: 1}}
	view.startApply()
	<-view.applyDone
	dismissTop(app)
	view.startApplyOne(0)
	<-view.applyDone
	dismissTop(app)

	for _, single := range []bool{false, true} {
		view.session = session
		view.tableState.setProposals(session.ProposalSnapshot())
		app.service = immediateService{err: errors.New("apply failure")}
		if single {
			view.startApplyOne(0)
		} else {
			view.startApply()
		}
		<-view.applyDone
		dismissTop(app)
	}

	view.session = nil
	view.startApply()
	view.startApplyOne(0)
	view.session = session
	view.applying = true
	view.startApply()
	view.startApplyOne(0)
	view.applying = false
	view.startApplyOne(-1)
}

func TestApplicationCloseCoverage(t *testing.T) {
	app := newTestApplication(t)
	app.organizer.applying = true
	app.closeNow()
	if app.closing {
		t.Fatal("close began while a rename was in progress")
	}
	app.requestClose()
	dismissTop(app)
	app.organizer.applying = false
	app.studio.dirty = true
	app.requestClose()
	tapDialogButton(app, app.text("common.cancel", "Cancel"))
	app.requestClose()
	tapDialogButton(app, app.text("common.save", "Save"))
	dismissTop(app)

	app.allowClose = true
	app.requestClose()
	app.closeNow()

	app = newTestApplication(t)
	done := make(chan struct{})
	app.organizer.previewDone = done
	app.organizer.cancelScan = func() {}
	app.closeNow()
	close(done)
	fyne.DoAndWait(func() {})
}

func TestApplicationCloseDecisionCallbacks(t *testing.T) {
	clean := newTestApplication(t)
	clean.requestClose()

	save := newTestApplication(t)
	if err := save.studio.newPattern("close-save"); err != nil {
		t.Fatal(err)
	}
	save.requestClose()
	if !tapDialogButton(save, save.text("common.save", "Save")) {
		t.Fatal("save close action not found")
	}

	discard := newTestApplication(t)
	if err := discard.studio.newPattern("close-discard"); err != nil {
		t.Fatal(err)
	}
	discard.requestClose()
	if !tapDialogButton(discard, discard.text("common.discard", "Discard")) {
		t.Fatal("discard close action not found")
	}
}

func TestShortcutAndConfirmationCallbacks(t *testing.T) {
	app := newTestApplication(t)
	view := app.studio
	view.builtIn = true
	view.dirty = true
	app.saveShortcut(nil)
	dismissTop(app)
	view.builtIn = false
	if err := view.newPattern("shortcut-save"); err != nil {
		t.Fatal(err)
	}
	app.saveShortcut(nil)

	organizerView := app.organizer
	proposal := rename.Proposal{SourcePath: "/tmp/a", ParentDir: "/tmp", OriginalName: "a", ProposedName: "b", Kind: rename.NodeKindFile, Changed: true}
	organizerView.session = organizer.NewPreviewSession("/tmp", "default", []rename.Proposal{proposal})
	organizerView.tableState.setProposals([]rename.Proposal{proposal})
	organizerView.confirmApply()
	tapDialogButton(app, app.text("common.cancel", "Cancel"))
	organizerView.confirmApplyOne(0)
	tapDialogButton(app, app.text("common.cancel", "Cancel"))
}

func TestSettingsRemainingBranches(t *testing.T) {
	app := newTestApplication(t)
	view := app.settingsUI
	view.ignoreInput.SetText("")
	view.addIgnoredName()
	view.ignoreInput.SetText("*.coverage")
	view.addIgnoredName()
	view.removeIgnoredName(-1)
	view.removeIgnoredName(len(view.ignoredNames) - 1)

	view.settingsFolderSelected(nil, errors.New("picker"))
	dismissTop(app)
	view.settingsFolderSelected(nil, nil)
	missing := filepath.Join(t.TempDir(), "missing")
	view.settingsFolderSelected(listableURI{URI: storage.NewFileURI(missing), path: missing}, nil)
	dismissTop(app)
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := view.loadSettingsFolder(file); err == nil {
		t.Fatal("file accepted as settings folder")
	}
	root := t.TempDir()
	if err := view.loadSettingsFolder(root); err != nil {
		t.Fatal(err)
	}
	configuration := settings.DefaultConfig(filepath.Join(root, "patterns"))
	configuration.IncludeFolders = false
	if err := settings.Save(filepath.Join(root, "config.json"), configuration); err != nil {
		t.Fatal(err)
	}
	if err := view.loadSettingsFolder(root); err != nil {
		t.Fatal(err)
	}
	view.language.SetSelectedIndex(-1)
	if view.selectedLanguage() != localization.PreferenceSystem {
		t.Fatal("invalid language did not fall back")
	}
	view.ignoredTable = nil
	view.refreshIgnoredNames()
	app.organizer.applying = true
	if err := view.Save(); err == nil {
		t.Fatal("settings changed during apply")
	}
	app.organizer.applying = false
	view.languageValues = nil
	_ = view.selectedLanguage()
	view.files.SetChecked(false)
	view.folders.SetChecked(false)
	if err := view.Save(); err == nil {
		t.Fatal("invalid include settings saved")
	}
	view.files.SetChecked(true)
	view.selectedConfig = filepath.Join(t.TempDir(), "config.json")
	app.studio.dirty = true
	if err := view.Save(); err == nil {
		t.Fatal("settings changed with dirty draft")
	}
	app.studio.dirty = false
	brokenRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(brokenRoot, "patterns"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	view.selectedConfig = filepath.Join(brokenRoot, "config.json")
	if err := view.Save(); err == nil {
		t.Fatal("invalid patterns directory accepted")
	}
	saveRoot := t.TempDir()
	view.selectedConfig = saveRoot
	if err := view.Save(); err == nil {
		t.Fatal("directory accepted as config file")
	}
}

func TestStudioEditorAndPersistenceCoverage(t *testing.T) {
	app := newTestApplication(t)
	view := app.studio
	view.dirty = false
	if !view.requireClean() {
		t.Fatal("clean draft rejected")
	}
	view.dirty = true
	if view.requireClean() {
		t.Fatal("dirty draft accepted")
	}
	view.dirty = false
	if err := view.duplicatePatternAs("coverage copy"); err != nil {
		t.Fatal(err)
	}
	view.baseName = ""
	view.discardChanges()

	view.draft = pattern.Spec{Name: "case editor", Rules: []pattern.RuleSpec{{Kind: rules.KindCase, Mode: rules.CaseModeLower}}}
	view.selectedRule = 0
	caseEditor := view.caseEditor()
	for _, entry := range canvasEntries(caseEditor) {
		entry.SetText("invalid")
		entry.SetText("0, 1")
	}
	if view.caseMode != nil {
		view.caseMode.SetSelected(caseModeLabel(rules.CaseModeUpper, app))
	}
	for _, check := range canvasChecks(caseEditor) {
		check.SetChecked(!check.Checked)
	}

	view.draft = pattern.Spec{Name: "replace editor", Rules: []pattern.RuleSpec{{Kind: rules.KindReplaceRunes, ExcludeMatches: []string{"x"}}}}
	view.selectedRule = 0
	replaceEditor := view.replaceEditor()
	for _, entry := range canvasEntries(replaceEditor) {
		entry.SetText(entry.Text + "z")
	}
	for _, check := range canvasChecks(replaceEditor) {
		check.SetChecked(!check.Checked)
	}
	if err := view.addExcludeMatch(0, "x"); err == nil {
		t.Fatal("duplicate exclusion accepted")
	}
	if err := view.addExcludeMatch(0, "["); err == nil {
		t.Fatal("invalid exclusion accepted")
	}
	if err := view.addExcludeMatch(0, ""); err == nil {
		t.Fatal("empty exclusion accepted")
	}
	if err := view.addExcludeMatch(-1, "x"); err == nil {
		t.Fatal("invalid rule accepted")
	}
	view.builtIn = true
	if err := view.addExcludeMatch(0, "y"); !errors.Is(err, patternlib.ErrReadOnly) {
		t.Fatal("built-in exclusion changed")
	}
	if view.removeExcludeMatch(0, 0) {
		t.Fatal("removed built-in exclusion")
	}
	view.builtIn = false
	if err := view.addExcludeMatch(0, "y"); err != nil {
		t.Fatal(err)
	}
	if view.removeExcludeMatch(-1, 0) || view.removeExcludeMatch(0, -1) || !view.removeExcludeMatch(0, 0) {
		t.Fatal("exclude removal branches")
	}

	view.draft = pattern.Spec{Name: "failed test", Tests: []pattern.TestCase{{Input: "a", Expected: "b", Kind: rename.NodeKindFile}}}
	view.baseName = ""
	if err := view.SaveCurrent(); err == nil {
		t.Fatal("failing characterization test was saved")
	}
	view.builtIn = true
	if !errors.Is(view.SaveCurrent(), patternlib.ErrReadOnly) {
		t.Fatal("built-in pattern saved")
	}
	view.builtIn = false
	view.validationErr = errors.New("validation")
	if !errors.Is(view.SaveCurrent(), view.validationErr) {
		t.Fatal("validation failure lost")
	}
	view.validationErr = nil
	view.draft = pattern.Spec{Name: "runtime test error", Tests: []pattern.TestCase{{Input: "a", Expected: "a", Kind: rename.NodeKind("unknown")}}}
	if err := view.SaveCurrent(); err == nil {
		t.Fatal("test execution error was saved")
	}
	view.draft = pattern.Spec{Name: "invalid", Rules: []pattern.RuleSpec{{Kind: rules.KindUnknown}}}
	if err := view.SaveCurrent(); err == nil {
		t.Fatal("invalid rule saved")
	}
	view.draft = pattern.Spec{Name: "cannot overwrite default"}
	view.baseName = "default"
	if err := view.SaveCurrent(); err == nil {
		t.Fatal("built-in source was overwritten")
	}
	view.baseName = "default"
	view.discardChanges()
	view.draft = pattern.Spec{Name: "moves", Rules: []pattern.RuleSpec{{Kind: rules.KindCase}, {Kind: rules.KindCase}}}
	view.selectedRule = 0
	view.moveRule(0, 1)
	view.removeRule(1)
	view.selectedTest = 0
	view.draft.Tests = []pattern.TestCase{{Input: "a", Expected: "a"}}
	view.removeSelectedTest()

	view.confirmDelete()
	view.baseName = "custom"
	view.builtIn = false
	view.confirmDelete()
	dismissTop(app)
	view.importConfig()
	dismissTop(app)
	view.exportConfig()
	dismissTop(app)
}

func TestStudioImportExportCoverage(t *testing.T) {
	app := newTestApplication(t)
	view := app.studio
	view.importSelected(nil, errors.New("read"))
	dismissTop(app)
	view.importSelected(nil, nil)
	view.importSelected(&uriReader{ReadCloser: io.NopCloser(bytes.NewBufferString("not json")), uri: storage.NewFileURI("bad.json")}, nil)
	dismissTop(app)
	bundle := `{"version":1,"pattern":"default","files":true,"folders":true,"patterns":[]}`
	view.importSelected(&uriReader{ReadCloser: io.NopCloser(bytes.NewBufferString(bundle)), uri: storage.NewFileURI("ok.json")}, nil)
	dismissTop(app)

	view.resolveImport(patternlib.Bundle{Patterns: []pattern.Spec{{Name: "default"}}}, 0, map[string]patternlib.ConflictAction{})
	tapDialogButton(app, app.text("studio.keep", "Keep existing"))
	dismissTop(app)
	view.resolveImport(patternlib.Bundle{Patterns: []pattern.Spec{{Name: "broken", Rules: []pattern.RuleSpec{{Kind: rules.KindUnknown}}}}}, 0, map[string]patternlib.ConflictAction{})
	dismissTop(app)

	configuration := patternlib.Bundle{Pattern: "default", Files: true, Folders: true}
	view.exportSelected(configuration, nil, errors.New("write"))
	dismissTop(app)
	view.exportSelected(configuration, nil, nil)
	writer := &uriWriter{uri: storage.NewFileURI("out.json")}
	view.exportSelected(configuration, writer, nil)
	if writer.Len() == 0 || !writer.closed {
		t.Fatal("bundle was not written and closed")
	}
	view.exportSelected(configuration, &errorURIWriter{uri: storage.NewFileURI("error.json")}, nil)
	dismissTop(app)
}

func TestRemainingLayoutAndValueBranches(t *testing.T) {
	app := newTestApplication(t)
	if _, ok := ruleKindFromLabel(ruleKindLabel(rules.KindCase, app), app); !ok {
		t.Fatal("case label did not round trip")
	}
	if _, ok := ruleKindFromLabel(ruleKindLabel(rules.KindReplaceRunes, app), app); !ok {
		t.Fatal("replace label did not round trip")
	}
	if _, ok := ruleKindFromLabel("unknown", app); ok {
		t.Fatal("unknown label accepted")
	}
	_ = nodeKindLabel("", app)

	hidden := widget.NewLabel("hidden")
	hidden.Hide()
	if heightAt(hidden, 100) != 0 {
		t.Fatal("hidden object has height")
	}
	visible := widget.NewLabel("visible")
	objects := []fyne.CanvasObject{hidden, visible}
	(&verticalLayout{gap: 2}).Layout(objects, fyne.NewSize(100, 100))
	(&flowLayout{gap: 2}).arrange(objects, 20, true)

	field := widget.NewEntry()
	form := newResponsiveForm(widget.NewFormItem("Original", field))
	form.Layout.(*responsiveFormLayout).items[0].Text = "Updated"
	field.Hide()
	form.Layout.MinSize(form.Objects)
	field.Show()
	form.Layout.Layout(form.Objects, fyne.NewSize(100, 100))
	form.Layout.MinSize(form.Objects)

	title, description := widget.NewLabel("Title"), widget.NewLabel("Description")
	description.Wrapping = fyne.TextWrapWord
	section := &sectionHeadingLayout{}
	section.MinSize([]fyne.CanvasObject{title, description})
	description.Hide()
	section.Layout([]fyne.CanvasObject{title, description}, fyne.NewSize(100, 100))
	section.MinSize([]fyne.CanvasObject{title, description})
	feedback := &feedbackLayout{}
	action := widget.NewButton("Action", nil)
	feedback.MinSize([]fyne.CanvasObject{visible, action})
	action.Hide()
	feedback.Layout([]fyne.CanvasObject{visible, action}, fyne.NewSize(100, 100))
	feedback.MinSize([]fyne.CanvasObject{visible, action})

	document := &documentLayout{}
	visible.Resize(fyne.Size{})
	document.MinSize([]fyne.CanvasObject{visible})
	testLayout := &testEntryLayout{}
	testObjects := []fyne.CanvasObject{widget.NewLabel("a"), widget.NewLabel("b"), widget.NewLabel("c"), widget.NewButton("go", nil)}
	testLayout.MinSize(testObjects)
	(&proposalTableLayout{view: app.organizer}).MinSize([]fyne.CanvasObject{visible})

	panel := NewTablePanel([]TableColumn{{Title: "A"}, {Title: "B", Width: 20}, {Title: "C"}}, 1)
	panel.NewRow(widget.NewLabel("one"))
	panel.NewRow(widget.NewLabel("one"), widget.NewLabel("two"))
	panel.NewRow(widget.NewLabel("one"), widget.NewLabel("two"), widget.NewLabel("three"))

	state := newProposalTableState()
	state.setProposals([]rename.Proposal{{Kind: rename.NodeKindFile}})
	state.sortColumn = -1
	state.visibleIndexes()
	state.kindFilter = map[string]struct{}{"Folder": {}}
	state.visibleIndexes()
	state.kindFilter = nil
	state.statusFilter = map[string]struct{}{"Changed": {}}
	state.visibleIndexes()
	state.statusFilter = nil
	state.actionFilter = map[string]struct{}{"Apply": {}}
	state.visibleIndexes()

	r := canvas.NewRectangle(nil)
	r.SetMinSize(fyne.NewSize(2, 3))
	_ = r
}

func TestRemainingOrganizerWidgetBranches(t *testing.T) {
	app := newTestApplication(t)
	view := app.organizer
	view.path.OnSubmitted("")
	view.RefreshPatterns("")
	view.patterns.Selected = "not present"
	view.RefreshPatterns("")
	view.folderSelected(nil, errors.New("folder"))
	dismissTop(app)
	view.folderSelected(nil, nil)
	view.folderSelected(listableURI{URI: storage.NewFileURI("")}, nil)
	view.cancelScan = func() {}
	view.StartPreview()
	view.cancelScan = nil
	view.path.SetText(t.TempDir())
	delete(view.patternByID, strings.ToLower(view.patterns.Selected))
	view.inputsChanged()
	view.updating = true
	view.invalidatePreview()
	view.updating = false

	app.window.Resize(fyne.Size{})
	_ = folderPickerSize(app.window)
	view.table = nil
	view.refreshProposalTable()

	proposal := rename.Proposal{SourcePath: "/tmp/a", ParentDir: "/tmp", OriginalName: "a", ProposedName: "b", Kind: rename.NodeKindFile, Changed: true}
	view.tableState.setProposals([]rename.Proposal{proposal})
	table := view.newProposalTable()
	cell := table.CreateCell()
	table.UpdateCell(widget.TableCellID{Row: -1, Col: 0}, cell)
	table.UpdateCell(widget.TableCellID{Row: 0, Col: proposalColumnAction}, cell)
	action := cell.(*fyne.Container).Objects[1].(*fyne.Container).Objects[0].(*widget.Button)
	action.OnTapped()
	dismissTop(app)
	view.applying = true
	table.UpdateCell(widget.TableCellID{Row: 0, Col: proposalColumnAction}, cell)
	view.applying = false
	header := table.CreateHeader()
	table.UpdateHeader(widget.TableCellID{Row: 0, Col: 0}, header)
	table.UpdateHeader(widget.TableCellID{Row: -1, Col: 0}, header)

	view.tableState.kindFilter = map[string]struct{}{"File": {}}
	view.openProposalFilter(proposalColumnKind, widget.NewButton("anchor", nil))
	for _, check := range canvasChecks(view.filterPopup.Content) {
		check.SetChecked(!check.Checked)
	}
	tapPopupButton(view.filterPopup, app.text("table.clear", "Clear"))
	tapPopupButton(view.filterPopup, app.text("common.apply", "Apply"))
	view.filterPopup.Hide()
}

func TestOrganizerRenameRootAndRestartBranches(t *testing.T) {
	app := newTestApplication(t)
	view := app.organizer
	root := t.TempDir()
	proposal := rename.Proposal{SourcePath: root, ParentDir: filepath.Dir(root), OriginalName: filepath.Base(root), ProposedName: "renamed-root", Kind: rename.NodeKindFolder, Changed: true}
	session := organizer.NewPreviewSession(root, "default", []rename.Proposal{proposal})
	app.service = immediateService{session: session, result: organizer.ApplyResult{AppliedCount: 1}}
	view.path.SetText("")
	view.session = session
	view.tableState.setProposals([]rename.Proposal{proposal})
	view.startApply()
	<-view.applyDone
	if view.previewDone != nil {
		<-view.previewDone
	}
	dismissTop(app)
	view.path.SetText("")
	view.session = session
	view.tableState.setProposals([]rename.Proposal{proposal})
	view.startApplyOne(0)
	<-view.applyDone
	if view.previewDone != nil {
		<-view.previewDone
	}
	dismissTop(app)

	restarter := &restartPreviewService{release: make(chan struct{}), session: session}
	app.service = restarter
	view.path.SetText(root)
	view.RefreshPatterns("default")
	view.StartPreview()
	firstDone := view.previewDone
	view.previewNext = true
	close(restarter.release)
	<-firstDone
	if secondDone := view.previewDone; secondDone != nil && secondDone != firstDone {
		<-secondDone
	}
}

func TestOrganizerIgnoresPreviewInvalidatedBeforeDelivery(t *testing.T) {
	app := newTestApplication(t)
	view := app.organizer
	root := t.TempDir()
	proposal := rename.Proposal{SourcePath: filepath.Join(root, "old.txt"), ParentDir: root, OriginalName: "old.txt", ProposedName: "new.txt", Kind: rename.NodeKindFile, Changed: true}
	release := make(chan struct{})
	app.service = &restartPreviewService{release: release, session: organizer.NewPreviewSession(root, "default", []rename.Proposal{proposal})}
	view.path.SetText(root)
	view.StartPreview()
	done := view.previewDone
	view.invalidatePreview()
	close(release)
	<-done
	if view.session != nil || len(view.tableState.proposals) != 0 {
		t.Fatal("invalidated preview result was delivered")
	}
}

func TestRemainingStudioCallbacks(t *testing.T) {
	app := newTestApplication(t)
	view := app.studio
	view.builtIn = true
	view.dirty = true
	view.save.OnTapped()
	dismissTop(app)
	view.dirty = false
	view.builtIn = false
	view.ruleKind.Selected = "unknown"
	view.addRuleButton.OnTapped()
	view.ruleKind.SetSelected(ruleKindLabel(rules.KindCase, app))
	view.addRuleButton.OnTapped()

	view.dirty = true
	view.RefreshPatterns("default")
	view.promptNewPattern()
	view.duplicatePattern()
	view.baseName = ""
	view.draft.Name = "draft"
	view.selectPattern("default")
	view.dirty = false
	view.patternPick.Selected = ""
	view.RefreshPatterns("")
	view.RefreshPatterns("missing")
	view.selectDraftName("default")

	view.promptNewPattern()
	view.namePromptInput.SetText("")
	tapDialogButton(app, app.text("common.create", "Create"))
	dismissTop(app)
	view.dirty = false
	view.promptNewPattern()
	view.namePromptInput.SetText("prompt-created")
	tapDialogButton(app, app.text("common.create", "Create"))
	view.dirty = false
	view.duplicatePattern()
	view.namePromptInput.SetText("prompt-copy")
	tapDialogButton(app, app.text("common.create", "Create"))

	view.builtIn = false
	view.selectedTest = 0
	view.applyReadOnlyState()
	view.draft = pattern.Spec{Name: "rows", Rules: []pattern.RuleSpec{{Kind: rules.KindCase}, {Kind: rules.KindReplaceRunes}}}
	view.selectedRule = 0
	view.refreshRules()
	for _, button := range canvasButtons(view.rulesHost) {
		if !button.Disabled() {
			button.OnTapped()
			break
		}
	}
	view.builtIn = true
	view.refreshRules()

	view.draft = pattern.Spec{Name: "case", Rules: []pattern.RuleSpec{{Kind: rules.KindCase, Positions: []int{0, 2}}}}
	view.selectedRule = 0
	view.caseEditor()

	view.draft = pattern.Spec{Name: "excluded", Rules: []pattern.RuleSpec{{Kind: rules.KindReplaceRunes, ExcludeMatches: []string{"x"}}}}
	view.selectedRule = 0
	view.builtIn = true
	view.excludeMatchesEditor()
	view.builtIn = false
	editor := view.excludeMatchesEditor()
	entries := canvasEntries(editor)
	for _, button := range canvasButtons(editor) {
		if button.Text == app.text("common.add", "Add") {
			button.OnTapped()
			entries[len(entries)-1].SetText("z")
			button.OnTapped()
			break
		}
	}

	view.dirty = true
	view.importConfig()
	view.dirty = false
	view.patternPick.Selected = ""
	view.exportConfig()
	dismissTop(app)
}

func TestStudioTableCallbackBranches(t *testing.T) {
	app := newTestApplication(t)
	view := app.studio
	view.testResults = []pattern.TestResult{
		{Case: pattern.TestCase{Input: "a", Expected: "a"}, Actual: "a", Passed: true},
		{Case: pattern.TestCase{Input: "a", Expected: "b", Kind: rename.NodeKindFile}, Actual: "a"},
		{Case: pattern.TestCase{Input: "a", Expected: "a", Kind: rename.NodeKindFile}, Error: "failure"},
	}
	table := view.newTestsTable()
	for row := 0; row <= len(view.testResults); row++ {
		for _, column := range []int{0, 4} {
			cell := table.CreateCell()
			table.UpdateCell(widget.TableCellID{Row: row, Col: column}, cell)
		}
	}
	table.OnSelected(widget.TableCellID{Row: 0, Col: 0})
	view.builtIn = true
	table.OnSelected(widget.TableCellID{Row: 1, Col: 0})
	view.builtIn = false
	table.OnSelected(widget.TableCellID{Row: 1, Col: 0})
}

func TestFinalUIBranches(t *testing.T) {
	app := newTestApplication(t)
	view := app.organizer
	view.session = nil
	view.confirmApply()
	unchanged := rename.Proposal{SourcePath: "/tmp/a", ParentDir: "/tmp", OriginalName: "a", ProposedName: "a", Kind: rename.NodeKindFile}
	view.session = organizer.NewPreviewSession("/tmp", "default", []rename.Proposal{unchanged})
	view.confirmApply()
	changed := rename.Proposal{SourcePath: "/tmp/a", ParentDir: "/tmp", OriginalName: "a", ProposedName: "b", Kind: rename.NodeKindFile, Changed: true}
	view.session = organizer.NewPreviewSession("/tmp", "default", []rename.Proposal{changed})
	view.tableState.setProposals([]rename.Proposal{changed})
	view.path.SetText("")
	app.service = immediateService{result: organizer.ApplyResult{AppliedCount: 1}}
	view.confirmApply()
	tapDialogButton(app, app.text("common.apply", "Apply"))
	<-view.applyDone
	dismissTop(app)
	view.session = organizer.NewPreviewSession("/tmp", "default", []rename.Proposal{changed})
	view.tableState.setProposals([]rename.Proposal{changed})
	view.confirmApplyOne(0)
	tapDialogButton(app, app.text("common.apply", "Apply"))
	<-view.applyDone
	dismissTop(app)

	app.studio.excludedTable = NewTablePanel([]TableColumn{{Title: "Regex"}, {Title: "Action"}}, 1)
	app.studio.refreshLanguage()
	app.shell.selectIndex(-1)

	settingsView := app.settingsUI
	app.organizer.applying = true
	for _, button := range canvasButtons(settingsView.content) {
		if button.Text == app.text("settings.save", "Save settings") {
			button.OnTapped()
			break
		}
	}
	dismissTop(app)
	previousAbs := resolveAbsolutePath
	resolveAbsolutePath = func(string) (string, error) { return "", errors.New("absolute") }
	if err := settingsView.loadSettingsFolder("relative"); err == nil {
		t.Fatal("absolute path error lost")
	}
	resolveAbsolutePath = previousAbs

	studio := app.studio
	studio.baseName = "default"
	studio.builtIn = false
	studio.confirmDelete()
	tapDialogButton(app, app.text("common.delete", "Delete"))
	dismissTop(app)
	if err := studio.newPattern("delete-success"); err != nil {
		t.Fatal(err)
	}
	if err := studio.SaveCurrent(); err != nil {
		t.Fatal(err)
	}
	studio.confirmDelete()
	tapDialogButton(app, app.text("common.cancel", "Cancel"))
	studio.confirmDelete()
	tapDialogButton(app, app.text("common.delete", "Delete"))

	for action := 0; action < 3; action++ {
		studio.draft = pattern.Spec{Name: "rules", Rules: []pattern.RuleSpec{{Kind: rules.KindCase}, {Kind: rules.KindCase}}}
		studio.selectedRule = 1
		studio.builtIn = false
		studio.refreshRules()
		buttons := canvasButtons(studio.rulesHost)
		blank := make([]*widget.Button, 0)
		for _, button := range buttons {
			if button.Text == "" {
				blank = append(blank, button)
			}
		}
		if action < len(blank) {
			blank[action].OnTapped()
		}
	}

	studio.draft = pattern.Spec{Name: "excluded", Rules: []pattern.RuleSpec{{Kind: rules.KindReplaceRunes, ExcludeMatches: []string{"x"}}}}
	studio.selectedRule = 0
	studio.builtIn = false
	studio.excludeMatchesEditor()
	studio.editorLabels.refresh()
	studio.excludedTable.rows.Objects[0].(*ignoredNameRow).remove.OnTapped()
	studio.draft.Rules[0].Kind = rules.KindCase
	if err := studio.addExcludeMatch(0, "x"); err == nil {
		t.Fatal("case rule accepted exclusion")
	}
	studio.draft.Tests = []pattern.TestCase{{Input: "a", Expected: "a"}}
	studio.selectedTest = 0
	studio.builtIn = false
	studio.removeSelectedTest()

	conflict := pattern.Spec{Name: "conflict"}
	if err := app.store.SaveAs("", conflict); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{app.text("studio.replace", "Replace"), app.text("studio.copy", "Import as copy")} {
		studio.resolveImport(patternlib.Bundle{Patterns: []pattern.Spec{conflict}}, 0, map[string]patternlib.ConflictAction{})
		tapDialogButton(app, action)
		dismissTop(app)
	}
	configuration := patternlib.Bundle{Pattern: "default"}
	studio.exportCallback(configuration)(&uriWriter{uri: storage.NewFileURI("callback.json")}, nil)

	app.window.Resize(fyne.Size{})
	_ = patternStudioHelpSize(app.window)
}

func tapDialogButton(app *Application, text string) bool {
	return tapPopupButton(app.window.Canvas().Overlays().Top(), text)
}

func tapPopupButton(popup fyne.CanvasObject, text string) bool {
	if popup == nil {
		return false
	}
	var selected *widget.Button
	walkCanvas(popup, func(item fyne.CanvasObject) {
		if button, ok := item.(*widget.Button); ok && button.Text == text && selected == nil {
			selected = button
		}
	})
	if selected == nil {
		return false
	}
	selected.OnTapped()
	return true
}

func canvasEntries(object fyne.CanvasObject) []*widget.Entry {
	var result []*widget.Entry
	walkCanvas(object, func(item fyne.CanvasObject) {
		if entry, ok := item.(*widget.Entry); ok {
			result = append(result, entry)
		}
	})
	return result
}

func canvasChecks(object fyne.CanvasObject) []*widget.Check {
	var result []*widget.Check
	walkCanvas(object, func(item fyne.CanvasObject) {
		if check, ok := item.(*widget.Check); ok {
			result = append(result, check)
		}
	})
	return result
}

func canvasButtons(object fyne.CanvasObject) []*widget.Button {
	var result []*widget.Button
	walkCanvas(object, func(item fyne.CanvasObject) {
		if button, ok := item.(*widget.Button); ok {
			result = append(result, button)
		}
	})
	return result
}

func walkCanvas(object fyne.CanvasObject, visit func(fyne.CanvasObject)) {
	visited := make(map[fyne.CanvasObject]struct{})
	var walk func(fyne.CanvasObject)
	walk = func(current fyne.CanvasObject) {
		if current == nil {
			return
		}
		if _, found := visited[current]; found {
			return
		}
		visited[current] = struct{}{}
		visit(current)
		if group, ok := current.(*fyne.Container); ok {
			for _, child := range group.Objects {
				walk(child)
			}
		}
		if scroll, ok := current.(*container.Scroll); ok {
			walk(scroll.Content)
		}
		if surface, ok := current.(*surface); ok {
			walk(surface.content)
		}
		if item, ok := current.(fyne.Widget); ok {
			renderer := item.CreateRenderer()
			for _, child := range renderer.Objects() {
				walk(child)
			}
		}
	}
	walk(object)
}

type listableURI struct {
	fyne.URI
	path string
}

func (uri listableURI) List() ([]fyne.URI, error) { return nil, nil }
func (uri listableURI) Path() string              { return uri.path }

type uriReader struct {
	io.ReadCloser
	uri fyne.URI
}

func (reader *uriReader) URI() fyne.URI { return reader.uri }

type uriWriter struct {
	bytes.Buffer
	uri    fyne.URI
	closed bool
}

func (writer *uriWriter) Close() error  { writer.closed = true; return nil }
func (writer *uriWriter) URI() fyne.URI { return writer.uri }

type errorURIWriter struct{ uri fyne.URI }

func (*errorURIWriter) Write([]byte) (int, error) { return 0, errors.New("write") }
func (*errorURIWriter) Close() error              { return nil }
func (writer *errorURIWriter) URI() fyne.URI      { return writer.uri }

func TestOrganizerRemainingControllerBranches(t *testing.T) {
	app := newTestApplication(t)
	view := app.organizer
	view.path.SetText("")
	view.StartPreview()
	view.path.SetText(filepath.Join(t.TempDir(), "missing"))
	view.StartPreview()
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	view.path.SetText(file)
	view.StartPreview()
	root := t.TempDir()
	view.path.SetText(root)
	view.updating = true
	view.files.SetChecked(false)
	view.folders.SetChecked(false)
	view.updating = false
	view.StartPreview()
	view.updating = true
	view.files.SetChecked(true)
	view.updating = false
	delete(view.patternByID, strings.ToLower(view.patterns.Selected))
	view.StartPreview()
	canceled := false
	view.cancelScan = func() { canceled = true }
	view.CancelPreview()
	if !canceled {
		t.Fatal("preview not canceled")
	}
	view.cancelScan = nil
	view.path.SetText("")
	view.previewNext = true
	view.inputsChanged()
	view.previewNext = false
	if got := rootAfterAppliedProposals(root, []rename.Proposal{{SourcePath: root, ParentDir: filepath.Dir(root), ProposedName: "new", Changed: true}}); got == root {
		t.Fatal("renamed root unchanged")
	}
	if got := rootAfterAppliedProposals(root, nil); got != root {
		t.Fatal("empty root changed")
	}
	view.tableState.proposals = []rename.Proposal{{Kind: rename.NodeKindFile, Changed: true}}
	view.session = organizer.NewPreviewSession(root, "default", view.tableState.proposals)
	view.confirmApply()
	dismissTop(app)
	view.confirmApplyOne(-1)
	view.confirmApplyOne(0)
	dismissTop(app)
	view.setControlsEnabled(false)
	view.setControlsEnabled(true)
	if view.proposalTableHeaderText(-1) != "" || view.proposalFilter(-1) != nil || proposalFilterOptions(-1) != nil {
		t.Fatal("invalid column helpers")
	}
	view.setProposalFilters(-1, nil)
	view.openProposalFilter(-1, view.table)
	view.toggleProposalSort(-1)
	if enumLabel("other") != "Other" || enumLabel("") != "" || truncateRunes("short", 10) != "short" || truncateRunes("long value", 4) == "long value" || truncateRunes("long", 1) != "…" {
		t.Fatal("format helpers")
	}
}

func TestStudioRemainingDraftAndValidationBranches(t *testing.T) {
	app := newTestApplication(t)
	view := app.studio
	view.switching = true
	view.selectPattern("default")
	view.switching = false
	view.dirty = true
	view.selectPattern("default")
	view.dirty = false
	view.selectPattern("missing")
	view.loadPattern("missing")
	view.builtIn = true
	view.markDirty()
	view.builtIn = false
	if err := view.newPattern(""); err == nil {
		t.Fatal("empty name accepted")
	}
	if err := view.newPattern("default"); err == nil {
		t.Fatal("duplicate name accepted")
	}
	if err := view.duplicatePatternAs(""); err == nil {
		t.Fatal("empty duplicate accepted")
	}
	if err := view.duplicatePatternAs("default"); err == nil {
		t.Fatal("duplicate target accepted")
	}
	view.newPattern("coverage")
	view.addRule(rules.KindUnknown)
	view.addRule(rules.KindCase)
	view.moveRule(0, -1)
	view.moveRule(0, 1)
	view.removeRule(-1)
	view.removeRule(0)
	view.testInput.SetText("")
	view.addTest()
	view.draft = pattern.Spec{}
	view.testInput.SetText("input")
	view.refreshPendingTestExpected()
	view.testInput = nil
	view.refreshPendingTestExpected()
	view.testInput = widget.NewEntry()
	view.testsTable = nil
	view.refreshTests()
	view.testsTable = view.newTestsTable()
	view.builtIn = true
	view.removeSelectedTest()
	view.builtIn = false
	if _, err := parsePositions("bad", app); err == nil {
		t.Fatal("invalid positions accepted")
	}
	if positions, err := parsePositions("", app); err != nil || positions != nil {
		t.Fatal("empty positions")
	}
	if got := stringList("ab"); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("string list: %v", got)
	}
	if stringList("") != nil {
		t.Fatal("empty string list")
	}
	_ = (pattern.Spec{}).Clone()
}

func TestEnumAndSmallWidgetBranches(t *testing.T) {
	app := newTestApplication(t, localization.PreferenceEnglish)
	for _, kind := range []rules.Kind{rules.KindCase, rules.KindReplaceRunes, rules.KindUnknown} {
		_ = ruleKindLabel(kind, app)
	}
	for _, mode := range []rules.CaseMode{rules.CaseModeLower, rules.CaseModeUpper, rules.CaseModeUnknown} {
		label := caseModeLabel(mode, app)
		_, _ = caseModeFromLabel(label, app)
	}
	for _, kind := range []rename.NodeKind{rename.NodeKindFile, rename.NodeKindFolder, rename.NodeKind("other")} {
		_ = nodeKindLabel(kind, app)
	}
	for _, status := range []string{"PASS", "FAIL", "ERROR", "other"} {
		_ = testResultLabel(status, app)
	}
	feedback := localizedFeedback{}
	label := widget.NewLabel("")
	feedback.setLiteral(label, "literal", label)
	feedback.refresh(app, label)
	row := newIgnoredNameRow(app)
	row.set(0, "", nil)
	row.set(1, "x", func(int) {})
	header := newProposalTableHeaderGuard(app.organizer)
	header.Dragged(&fyne.DragEvent{})
	header.DragEnd()
	_ = header.MinSize()
	surface := newSurface(widget.NewLabel("x"))
	surface.Tapped(nil)
	renderer := surface.CreateRenderer()
	renderer.Destroy()
	_ = renderer.MinSize()
	renderer.Layout(fyne.NewSize(10, 10))
	renderer.Refresh()
	panel := NewTablePanel(nil, 0)
	_ = panel.NewRow()
	_ = test.Tap
}

type immediateService struct {
	session *organizer.PreviewSession
	result  organizer.ApplyResult
	err     error
}

type restartPreviewService struct {
	release chan struct{}
	session *organizer.PreviewSession
	calls   int
}

func (service *restartPreviewService) Preview(context.Context, organizer.PreviewRequest) (*organizer.PreviewSession, error) {
	service.calls++
	if service.calls == 1 {
		<-service.release
	}
	return service.session, nil
}
func (*restartPreviewService) Apply(context.Context, *organizer.PreviewSession) (organizer.ApplyResult, error) {
	return organizer.ApplyResult{}, nil
}
func (*restartPreviewService) ApplyOne(context.Context, *organizer.PreviewSession, string) (organizer.ApplyResult, error) {
	return organizer.ApplyResult{}, nil
}

func (service immediateService) Preview(context.Context, organizer.PreviewRequest) (*organizer.PreviewSession, error) {
	return service.session, service.err
}
func (service immediateService) Apply(context.Context, *organizer.PreviewSession) (organizer.ApplyResult, error) {
	return service.result, service.err
}
func (service immediateService) ApplyOne(context.Context, *organizer.PreviewSession, string) (organizer.ApplyResult, error) {
	return service.result, service.err
}
