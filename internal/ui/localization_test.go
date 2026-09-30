package ui

import (
	"encoding/json"
	"fyne.io/fyne/v2/container"
	"github.com/eightaugusto/file-folder-renamer/internal/localization"
	organizerpkg "github.com/eightaugusto/file-folder-renamer/internal/organizer"
	"github.com/eightaugusto/file-folder-renamer/internal/rename"
	"github.com/eightaugusto/file-folder-renamer/internal/rules"
	settingspkg "github.com/eightaugusto/file-folder-renamer/internal/settings"
	"os"
	"reflect"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

func TestLocalizedStudioPreservesPatternValues(t *testing.T) {
	var englishJSON []byte
	for _, tc := range []struct {
		language                    localization.Preference
		lower, folder, help, remove string
	}{
		{localization.PreferenceEnglish, "Lower case", "Folder", "Case", "Remove Diacritics"},
		{localization.PreferenceSpanish, "Minúsculas", "Carpeta", "Mayúsculas y minúsculas", "Quitar diacríticos"},
	} {
		t.Run(string(tc.language), func(t *testing.T) {
			app := newTestApplication(t, tc.language)
			studio := app.studio
			if err := studio.newPattern("Sample"); err != nil {
				t.Fatal(err)
			}
			studio.addRule(rules.KindCase)
			kind, ok := ruleKindFromLabel(ruleKindLabel(rules.KindCase, app), app)
			if !ok || kind != rules.KindCase {
				t.Fatal("translated rule changed its identity")
			}
			kind, ok = ruleKindFromLabel(tc.remove, app)
			if !ok || kind != rules.KindRemoveDiacritics {
				t.Fatal("translated diacritics rule changed its identity")
			}
			mode, ok := caseModeFromLabel(tc.lower, app)
			if !ok || mode != rules.CaseModeLower {
				t.Fatal("translated case mode changed its identity")
			}
			studio.testKind.SetSelected(tc.folder)
			studio.testInput.SetText("MY FOLDER")
			if studio.testExpected.Text != "my folder" {
				t.Fatal(studio.testExpected.Text)
			}
			studio.addTest()
			if studio.draft.Tests[0].Kind != rename.NodeKindFolder {
				t.Fatal("translated Folder was not saved as folder")
			}
			data, err := json.Marshal(studio.draft)
			if err != nil {
				t.Fatal(err)
			}
			if englishJSON == nil {
				englishJSON = data
			} else if string(data) != string(englishJSON) {
				t.Fatalf("language changed pattern JSON:\n%s\n%s", englishJSON, data)
			}
			accordion := patternStudioHelpAccordion(app)
			if accordion.Items[0].Title != tc.help || accordion.MultiOpen {
				t.Fatal("help localization or accordion behavior changed")
			}
			if err := studio.SaveCurrent(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSpanishOrganizerKeepsStableFiltersAndFixedColumns(t *testing.T) {
	app := newTestApplication(t, localization.PreferenceSpanish)
	view := app.organizer
	view.tableState.proposals = []rename.Proposal{
		{Kind: rename.NodeKindFile, RelativePath: "z/file", OriginalName: "MY FILE", ProposedName: "My File", Changed: true},
		{Kind: rename.NodeKindFolder, RelativePath: "a", OriginalName: "a", ProposedName: "a"},
	}
	if view.apply.Text != "Aplicar todos los cambios" || view.proposalTableHeaderText(proposalColumnStatus) != "Estado" {
		t.Fatal("organizer not translated")
	}
	if view.tableState.proposals[view.visibleProposalIndexes()[0]].RelativePath != "a" {
		t.Fatal("default Location sort changed")
	}
	view.setProposalFilters(proposalColumnKind, []string{"File"})
	view.setProposalFilters(proposalColumnStatus, []string{"Changed"})
	view.setProposalFilters(proposalColumnAction, []string{"Apply"})
	if len(view.visibleProposalIndexes()) != 1 {
		t.Fatal("Spanish filters changed matching")
	}
	cell := view.table.CreateCell()
	view.table.UpdateCell(widget.TableCellID{Row: 0, Col: proposalColumnKind}, cell)
	if cell.(*fyne.Container).Objects[0].(*widget.Label).Text != "Archivo" {
		t.Fatal("kind not translated")
	}
	view.updateProposalTableColumns(1300)
	widths := view.columnWidths
	view.updateProposalTableColumns(1900)
	for _, col := range []int{proposalColumnAction, proposalColumnKind, proposalColumnStatus} {
		if widths[col] != view.columnWidths[col] {
			t.Fatal("fixed column resized")
		}
	}
	if widths[proposalColumnAction] < widget.NewButton("Aplicar", nil).MinSize().Width {
		t.Fatal("translated Apply does not fit")
	}
	if app.appliedMessage(1) != "Se aplicó 1 cambio de nombre." || app.appliedMessage(0) != "Se aplicaron 0 cambios de nombre." {
		t.Fatal("pluralization failed")
	}
}

func TestLanguageRefreshPreservesDraftPreviewAndControls(t *testing.T) {
	app := newTestApplication(t, localization.PreferenceEnglish)
	studio, organizer, settings := app.studio, app.organizer, app.settingsUI
	app.shell = newApplicationShell(app)
	app.window.SetContent(app.shell.content)
	app.shell.selectIndex(2)
	if err := studio.newPattern("Settings"); err != nil {
		t.Fatal(err)
	}
	studio.addRule(rules.KindCase)
	studio.testKind.SetSelected("Folder")
	studio.testInput.SetText("MY FOLDER")
	studio.addTest()
	studio.testsTable.Select(widget.TableCellID{Row: 1, Col: 0})
	studio.testInput.SetText("ANOTHER FOLDER")
	positions := findRuleEntry(studio.editorHost, "Positions")
	if positions == nil {
		t.Fatal("positions entry missing")
	}
	positions.SetText("-1")
	studio.testsTable.Select(widget.TableCellID{Row: 1, Col: 0})
	validationErr := studio.validationErr
	before, _ := json.Marshal(studio.draft)
	editor, table, mode := studio.editorHost.Objects[0], studio.testsTable, studio.caseMode
	studio.workspaceSplit.SetOffset(0.63)
	organizer.tableState.proposals = []rename.Proposal{{Kind: rename.NodeKindFile, RelativePath: "Settings", OriginalName: "Settings", ProposedName: "Settings", Changed: false}}
	organizer.session = organizerpkg.NewPreviewSession("", "", organizer.tableState.proposals)
	session := organizer.session
	organizer.tableState.sortColumn, organizer.tableState.sortAscending = proposalColumnOriginal, false
	organizer.setProposalFilters(proposalColumnKind, []string{"File"})
	canceled := false
	organizer.cancelScan = func() { canceled = true }
	settings.ignoreInput.SetText("Settings")
	store := app.store
	for _, preference := range []localization.Preference{localization.PreferenceSpanish, localization.PreferenceEnglish, localization.PreferenceSpanish} {
		settings.selectLanguage(preference)
		if err := settings.Save(); err != nil {
			t.Fatal(err)
		}
		if app.texts().Preference() != preference {
			t.Fatal("language did not switch")
		}
		if app.store != store || organizer.session != session || canceled || organizer.previewNext {
			t.Fatal("language change reloaded library or restarted scan")
		}
		after, _ := json.Marshal(studio.draft)
		if string(before) != string(after) || !studio.dirty || studio.validationErr != validationErr {
			t.Fatal("draft or validation state changed")
		}
		if studio.editorHost.Objects[0] != editor || positions.Text != "-1" || studio.testsTable != table || studio.caseMode != mode {
			t.Fatal("live controls were replaced")
		}
		if studio.selectedTest != 0 || studio.testInput.Text != "ANOTHER FOLDER" || nodeKindFromLabel(studio.testKind.Selected, app) != rename.NodeKindFolder {
			t.Fatal("test selection or pending input changed")
		}
		if studio.workspaceSplit.Offset != 0.63 || app.shell.selected != 2 {
			t.Fatal("layout or tab selection changed")
		}
		if organizer.tableState.sortColumn != proposalColumnOriginal || organizer.tableState.sortAscending || !reflect.DeepEqual(organizer.tableState.kindFilter, map[string]struct{}{"File": {}}) {
			t.Fatal("sort or filters changed")
		}
		if settings.ignoreInput.Text != "Settings" || studio.patternPick.Selected != "Settings" {
			t.Fatal("user data was translated")
		}
		if _, ok := caseModeFromLabel(mode.Selected, app); !ok {
			t.Fatal("case dropdown lost enum mapping")
		}
	}
	if organizer.apply.Text != "Aplicar todos los cambios" || app.shell.buttons[2].Text != "• Configuración" || app.shell.buttons[0].Text != "Organizador" {
		t.Fatal("visible UI did not refresh")
	}
	if strings.Contains(settings.status.Text, "Restart") {
		t.Fatal("obsolete restart notice")
	}
	loaded, err := settingspkg.Load(app.configPath)
	if err != nil || loaded.Language != localization.PreferenceSpanish {
		t.Fatalf("language not saved: %v", err)
	}
}

func findRuleEntry(object fyne.CanvasObject, label string) *widget.Entry {
	var children []fyne.CanvasObject
	switch value := object.(type) {
	case *fyne.Container:
		if form, ok := value.Layout.(*responsiveFormLayout); ok {
			for _, item := range form.items {
				if item.Text == label {
					if entry, ok := item.Widget.(*widget.Entry); ok {
						return entry
					}
				}
			}
		}
		children = value.Objects
	case *container.Scroll:
		children = []fyne.CanvasObject{value.Content}
	case *widget.Card:
		children = []fyne.CanvasObject{value.Content}
	case *widget.Form:
		for _, item := range value.Items {
			if item.Text == label {
				if entry, ok := item.Widget.(*widget.Entry); ok {
					return entry
				}
			}
		}
	}
	for _, child := range children {
		if result := findRuleEntry(child, label); result != nil {
			return result
		}
	}
	return nil
}

func TestFailedLanguageSaveDoesNotRefresh(t *testing.T) {
	app := newTestApplication(t, localization.PreferenceEnglish)
	if err := os.Mkdir(app.configPath, 0700); err != nil {
		t.Fatal(err)
	}
	app.settingsUI.selectLanguage(localization.PreferenceSpanish)
	if err := app.settingsUI.Save(); err == nil {
		t.Fatal("expected failed config write")
	}
	if app.texts().Preference() != localization.PreferenceEnglish || app.organizer.apply.Text != "Apply all changes" {
		t.Fatal("failed save switched language")
	}
}

func TestLanguageSaveWaitsForRename(t *testing.T) {
	app := newTestApplication(t, localization.PreferenceEnglish)
	app.organizer.applying = true
	app.settingsUI.selectLanguage(localization.PreferenceSpanish)
	if err := app.settingsUI.Save(); err == nil {
		t.Fatal("settings changed during rename")
	}
	if app.texts().Preference() != localization.PreferenceEnglish {
		t.Fatal("language changed during rename")
	}
}

func TestRenamedApplicationTitles(t *testing.T) {
	for _, tc := range []struct {
		language localization.Preference
		title    string
	}{
		{localization.PreferenceEnglish, "File & Folder Renamer"},
		{localization.PreferenceSpanish, "Renombrador de archivos y carpetas"},
	} {
		app := newTestApplication(t, tc.language)
		if got := app.text("app.title", ""); got != tc.title {
			t.Fatalf("product title: got %q, want %q", got, tc.title)
		}
		app.window.SetTitle(app.text("app.title", ""))
		if app.window.Title() != tc.title {
			t.Fatal("window title changed product text")
		}
	}
}
