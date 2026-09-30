package ui

import (
	"fmt"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/eightaugusto/file-folder-renamer/internal/localization"
	"github.com/eightaugusto/file-folder-renamer/internal/organizer"
	"github.com/eightaugusto/file-folder-renamer/internal/rename"
	"github.com/eightaugusto/file-folder-renamer/internal/rules"
)

// Opt-in rendered fixtures use only temporary settings and synthetic proposals.
// UI_CAPTURE_DIR=... go test ./internal/ui -run TestUIVisualFixtures -count=1
func TestUIVisualFixtures(t *testing.T) {
	root := os.Getenv("UI_CAPTURE_DIR")
	if root == "" {
		t.Skip("set UI_CAPTURE_DIR to export rendered fixtures")
	}
	for _, locale := range []localization.Preference{localization.PreferenceEnglish, localization.PreferenceSpanish} {
		for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
			for _, size := range []fyne.Size{fyne.NewSize(1024, 768), fyne.NewSize(1280, 800), fyne.NewSize(1920, 1080)} {
				name := fmt.Sprintf("%s-%d-%dx%d", locale, variant, int(size.Width), int(size.Height))
				t.Run(name, func(t *testing.T) {
					app := newThemedTestApplication(t, fixtureTheme{newOrganizerTheme(), variant}, locale)
					app.shell = newApplicationShell(app)
					app.window.SetContent(app.shell.content)
					app.window.Resize(size)
					captureAt := func(scene string, expectedSize fyne.Size) {
						if app.window.Canvas().Size() != expectedSize {
							t.Fatalf("%s inflated canvas: %v", scene, app.window.Canvas().Size())
						}
						if overlay := app.window.Canvas().Overlays().Top(); overlay != nil {
							for _, o := range visibleObjects(overlay) {
								if popup, ok := o.(*widget.PopUp); ok {
									assertReachable(t, app, popup)
								}
							}
						}
						dir := filepath.Join(root, name)
						if err := os.MkdirAll(dir, 0755); err != nil {
							t.Fatal(err)
						}
						file, err := os.Create(filepath.Join(dir, scene+".png"))
						if err != nil {
							t.Fatal(err)
						}
						defer file.Close()
						if err := png.Encode(file, app.window.Canvas().Capture()); err != nil {
							t.Fatal(err)
						}
						t.Logf("%s canvas %v", scene, app.window.Canvas().Size())
					}
					capture := func(scene string) { captureAt(scene, size) }
					capture("organizer-empty")
					view := app.organizer
					view.path.OnChanged = nil
					view.path.SetText("/Example/Project files")
					view.tableState.proposals = []rename.Proposal{
						{Kind: rename.NodeKindFolder, RelativePath: "project images", OriginalName: "project images", ProposedName: "Project Images", Changed: true},
						{Kind: rename.NodeKindFile, RelativePath: "project images/summer holiday.JPG", OriginalName: "summer holiday.JPG", ProposedName: "Summer Holiday.jpg", Changed: true},
						{Kind: rename.NodeKindFile, RelativePath: "Readme.txt", OriginalName: "Readme.txt", ProposedName: "Readme.txt", Changed: false},
					}
					view.session = organizer.NewPreviewSession(view.path.Text, "default", view.tableState.proposals)
					view.setStatus("organizer.status_previewed", "Previewed {{.Root}} with pattern {{.Pattern}}.", map[string]any{"Root": view.path.Text, "Pattern": "default"})
					view.apply.Enable()
					view.refreshLanguage()
					capture("organizer-preview")
					if locale == localization.PreferenceEnglish && variant == theme.VariantLight && size == fyne.NewSize(1280, 800) {
						compactSize := fyne.NewSize(1280, 480)
						app.window.Resize(compactSize)
						captureAt("organizer-preview-compact", compactSize)
						app.window.Resize(size)
					}
					session := view.session
					view.invalidatePreview()
					view.setScanning(true)
					capture("organizer-scanning")
					view.setScanning(false)
					view.session, view.tableState.proposals = session, session.ProposalSnapshot()
					view.setStatus("organizer.status_previewed", "Previewed {{.Root}} with pattern {{.Pattern}}.", map[string]any{"Root": view.path.Text, "Pattern": "default"})
					view.apply.Enable()
					view.refreshLanguage()
					view.openProposalFilter(proposalColumnKind, view.headerFilters[proposalColumnKind])
					capture("organizer-filter")
					view.filterPopup.Hide()
					test.Tap(view.patterns)
					capture("organizer-dropdown")
					app.window.Canvas().Overlays().Top().Hide()
					view.confirmApply()
					capture("organizer-confirmation")
					app.window.Canvas().Overlays().Top().Hide()
					app.shell.selectIndex(1)
					app.studio.selectedRule = 0
					app.studio.refreshRules()
					app.studio.refreshEditor()
					capture("studio")
					if err := app.studio.duplicatePatternAs("Sample draft"); err != nil {
						t.Fatal(err)
					}
					replaceRule := -1
					for index, rule := range app.studio.draft.Rules {
						if rule.Kind == rules.KindReplaceRunes {
							replaceRule = index
							break
						}
					}
					if replaceRule < 0 {
						t.Fatal("the sample pattern has no replace runes rule")
					}
					app.studio.selectedRule = replaceRule
					for _, expression := range []string{`\bKEEP\b`, `\b2026\b`} {
						if err := app.studio.addExcludeMatch(replaceRule, expression); err != nil {
							t.Fatal(err)
						}
					}
					app.studio.refreshEditor()
					editor := app.studio.editorHost.Objects[0].(*container.Scroll)
					regexY := absolute(app, app.studio.excludedTable.Content()).Y - absolute(app, editor).Y - 60
					editor.ScrollToOffset(fyne.NewPos(0, regexY))
					capture("studio-regex")
					app.studio.addRule(rules.KindCase)
					app.studio.testKind.SetSelected(nodeKindLabel(rename.NodeKindFolder, app))
					app.studio.testInput.SetText("MY FOLDER")
					capture("studio-case")
					app.studio.showHelp()
					capture("studio-help")
					helpOverlay := app.window.Canvas().Overlays().Top()
					for _, object := range visibleObjects(helpOverlay) {
						if accordion, ok := object.(*widget.Accordion); ok {
							accordion.Open(0)
							helpOverlay.Refresh()
							capture("studio-help-case")
							accordion.Open(1)
							helpOverlay.Refresh()
							capture("studio-help-replace")
							break
						}
					}
					for _, object := range visibleObjects(helpOverlay) {
						if scroll, ok := object.(*container.Scroll); ok {
							scroll.ScrollToBottom()
							capture("studio-help-end")
							break
						}
					}
					for _, o := range visibleObjects(app.window.Canvas().Overlays().Top()) {
						if b, ok := o.(*widget.Button); ok && b.Text == app.text("common.close", "Close") {
							test.Tap(b)
							break
						}
					}
					app.shell.selectIndex(2)
					capture("settings")
					test.Tap(app.settingsUI.language)
					capture("settings-dropdown")
					app.window.Canvas().Overlays().Top().Hide()
				})
			}
		}
	}
}

type fixtureTheme struct {
	fyne.Theme
	variant fyne.ThemeVariant
}

func (t fixtureTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return t.Theme.Color(name, t.variant)
}
