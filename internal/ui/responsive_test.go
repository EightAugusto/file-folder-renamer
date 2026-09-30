package ui

import (
	"errors"
	"fmt"
	"image/color"
	"math"
	"reflect"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/eightaugusto/file-folder-renamer/internal/localization"
	"github.com/eightaugusto/file-folder-renamer/internal/organizer"
	"github.com/eightaugusto/file-folder-renamer/internal/rename"
	"github.com/eightaugusto/file-folder-renamer/internal/rules"
)

func mountShell(app *Application, size fyne.Size) {
	app.shell = newApplicationShell(app)
	app.window.SetContent(app.shell.content)
	app.window.Resize(size)
}
func absolute(app *Application, o fyne.CanvasObject) fyne.Position {
	return app.app.Driver().AbsolutePositionForObject(o)
}
func assertReachable(t *testing.T, app *Application, o fyne.CanvasObject) {
	t.Helper()
	p, s := absolute(app, o), o.Size()
	bounds := app.window.Canvas().Size()
	if p.X < 0 || p.Y < 0 || p.X+s.Width > bounds.Width+1 || p.Y+s.Height > bounds.Height+1 || s.Width <= 0 || s.Height <= 0 {
		t.Fatalf("%T outside canvas: %v + %v in %v", o, p, s, bounds)
	}
}

func TestResponsiveWorkspacesFitRequestedWindows(t *testing.T) {
	for _, locale := range []localization.Preference{localization.PreferenceEnglish, localization.PreferenceSpanish} {
		for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
			for _, size := range []fyne.Size{fyne.NewSize(1024, 768), fyne.NewSize(1280, 800), fyne.NewSize(1920, 1080)} {
				t.Run(fmt.Sprintf("%s/%d/%v", locale, variant, size), func(t *testing.T) {
					app := newThemedTestApplication(t, fixtureTheme{newOrganizerTheme(), variant}, locale)
					mountShell(app, size)
					if app.window.Canvas().Size() != size {
						t.Fatalf("window minimum inflated requested size: %v", app.window.Canvas().Size())
					}
					if app.shell.compact != (size.Width < navigationBreakpoint) {
						t.Fatal("wrong initial navigation layout")
					}
					for i, b := range app.shell.buttons {
						assertReachable(t, app, b)
						if b.Size().Width < b.MinSize().Width || b.Size().Height < 32 {
							t.Fatalf("navigation %d clipped or too short", i)
						}
					}
					assertReachable(t, app, app.shell.about)
					views := append([]fyne.CanvasObject(nil), app.shell.views...)
					for i := range views {
						app.shell.selectIndex(i)
						switch i {
						case 0:
							assertReachable(t, app, app.organizer.apply)
							if app.organizer.tableHost.Size().Height < 300 {
								t.Fatalf("table is not dominant: %v", app.organizer.tableHost.Size())
							}
						case 1:
							for _, b := range []*widget.Button{app.studio.newButton, app.studio.helpButton, app.studio.save, app.studio.discard, app.studio.delete, app.studio.addTestButton, app.studio.deleteTest} {
								assertReachable(t, app, b)
								if b.Size().Width < b.MinSize().Width {
									t.Fatalf("toolbar button %q clipped", b.Text)
								}
							}
							if app.studio.testsTable.Size().Height < 70 {
								t.Fatalf("saved results not visible: %v", app.studio.testsTable.Size())
							}
							if math.Abs(float64(app.studio.pipelinePanel.Size().Width-app.studio.pipelineWidth())) > 0.1 {
								t.Fatal("pipeline allowance lost")
							}
						case 2:
							assertReachable(t, app, app.settingsUI.language)
							assertReachable(t, app, app.settingsUI.ignoreInput)
						}
					}
					if !reflect.DeepEqual(views, app.shell.views) {
						t.Fatal("navigation recreated views")
					}
				})
			}
		}
	}
}

func TestAboutIsNeutralAndAnchoredAwayFromWorkspaceNavigation(t *testing.T) {
	app := newTestApplication(t)
	mountShell(app, fyne.NewSize(1024, 768))

	if app.shell.about.Importance != widget.LowImportance {
		t.Fatal("About is an action and must not look like the selected workspace")
	}
	if got, want := app.shell.about.Position().X+app.shell.about.Size().Width, app.shell.navigation.Size().Width-spaceLG; got != want {
		t.Fatalf("horizontal About action must be right-aligned: got %v, want %v", got, want)
	}
	for index := range app.shell.buttons {
		app.shell.selectIndex(index)
		if app.shell.about.Importance != widget.LowImportance {
			t.Fatalf("selecting workspace %d made About look selected", index)
		}
	}

	app.window.Resize(fyne.NewSize(1280, 800))
	if got, want := app.shell.about.Position().Y+app.shell.about.Size().Height, app.shell.navigation.Size().Height-spaceLG; got != want {
		t.Fatalf("sidebar About action must remain bottom-aligned: got %v, want %v", got, want)
	}
}

func TestNavigationResizeAndLanguagePreserveLiveState(t *testing.T) {
	app := newTestApplication(t)
	mountShell(app, fyne.NewSize(1280, 800))
	studio := app.studio
	if err := studio.newPattern("Draft"); err != nil {
		t.Fatal(err)
	}
	studio.addRule(rules.KindCase)
	positions := findRuleEntry(studio.editorHost, "Positions")
	positions.SetText("-1")
	studio.testInput.SetText("unfinished file.txt")
	editor := studio.editorHost.Objects[0].(*container.Scroll)
	editor.Offset = fyne.NewPos(0, 10)
	editor.Refresh()
	app.organizer.tableState.proposals = make([]rename.Proposal, 80)
	for i := range app.organizer.tableState.proposals {
		app.organizer.tableState.proposals[i] = rename.Proposal{Kind: rename.NodeKindFile, RelativePath: fmt.Sprint(i), OriginalName: "source", ProposedName: "Target", Changed: true}
	}
	app.organizer.session = organizer.NewPreviewSession("", "", app.organizer.tableState.proposals)
	session, service, store := app.organizer.session, app.service, app.store
	app.organizer.setProposalFilters(proposalColumnKind, []string{"File"})
	app.organizer.tableState.sortColumn = proposalColumnOriginal
	canceled := false
	app.organizer.cancelScan = func() { canceled = true }
	app.organizer.setScanning(true)
	app.settingsUI.ignoreInput.SetText("unfinished*.tmp")
	app.shell.selectIndex(1)
	app.window.Canvas().Focus(positions)
	studio.workspaceSplit.SetOffset(0.55)
	controls := []fyne.CanvasObject{positions, studio.testInput, editor, app.organizer.table}
	for _, width := range []float32{1024, 1920, 1179, 1180, 1280} {
		app.window.Resize(fyne.NewSize(width, 800))
		if app.window.Canvas().Focused() != positions {
			t.Fatal("resize lost keyboard focus")
		}
		for _, locale := range []localization.Preference{localization.PreferenceSpanish, localization.PreferenceEnglish} {
			app.settings.Language = locale
			app.refreshLanguage()
			if positions.Text != "-1" || studio.testInput.Text != "unfinished file.txt" || app.settingsUI.ignoreInput.Text != "unfinished*.tmp" {
				t.Fatal("unfinished input lost")
			}
			if !studio.dirty || studio.validationErr == nil || studio.workspaceSplit.Offset != 0.55 || app.shell.selected != 1 {
				t.Fatal("draft or navigation state lost")
			}
			if app.organizer.session != session || app.service != service || app.store != store || canceled || app.organizer.previewNext {
				t.Fatal("language or resize restarted services or preview")
			}
			if !reflect.DeepEqual(controls, []fyne.CanvasObject{positions, studio.testInput, studio.editorHost.Objects[0], app.organizer.table}) {
				t.Fatal("live controls recreated")
			}
			if app.organizer.status.Text != app.text("organizer.status_scanning", "Scanning selection…") {
				t.Fatal("scan feedback reset during translation")
			}
			if !reflect.DeepEqual(app.organizer.tableState.kindFilter, map[string]struct{}{"File": {}}) || app.organizer.tableState.sortColumn != proposalColumnOriginal {
				t.Fatal("filter or sorting lost")
			}
			if editor.Offset.Y != 10 {
				t.Fatalf("editor scroll reset: %v", editor.Offset)
			}
		}
	}
}

func TestHeaderKeyboardFilterAndDragInteraction(t *testing.T) {
	app := newTestApplication(t)
	mountShell(app, fyne.NewSize(1024, 768))
	v := app.organizer
	v.path.OnChanged = nil
	v.path.SetText("/example")
	v.tableState.proposals = []rename.Proposal{{Kind: rename.NodeKindFile, OriginalName: "one", ProposedName: "One", Changed: true}}
	v.refreshProposalTable()
	// Use actual rendered headers: a transparent drag guard must not swallow taps.
	filter := v.headerFilters[proposalColumnKind].(*widget.Button)
	pos := absolute(app, filter).Add(fyne.NewPos(5, 5))
	test.TapCanvas(app.window.Canvas(), pos)
	if v.filterPopup == nil || !v.filterPopup.Visible() {
		t.Fatal("header filter did not receive pointer tap")
	}
	checks := v.filterPopup.Content.(*fyne.Container).Objects[1].(*fyne.Container)
	check := checks.Objects[0].(*widget.Check)
	check.SetChecked(true)
	popup := v.filterPopup
	app.settings.Language = localization.PreferenceSpanish
	app.refreshLanguage()
	if v.filterPopup != popup || !check.Checked || check.Text != "Archivo" {
		t.Fatal("language refresh discarded pending filter selection")
	}
	actions := popup.Content.(*fyne.Container).Objects[2].(*fyne.Container)
	apply := actions.Objects[1].(*widget.Button)
	app.window.Canvas().Focus(apply)
	apply.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	if v.headerFilters[proposalColumnKind].(*widget.Button).Text != "1" {
		t.Fatal("active filter needs a non-color marker")
	}
	if len(v.tableState.kindFilter) != 1 {
		t.Fatal("filter Apply is not keyboard operable")
	}
	widths := v.columnWidths
	hostPos := absolute(app, v.tableHost)
	test.Drag(app.window.Canvas(), hostPos.Add(fyne.NewPos(widths[0]-1, 15)), 80, 0)
	if widths != v.columnWidths {
		t.Fatal("header drag resized fixed columns")
	}
	// Locate the real sort button through its sibling filter.
	sortButton := v.headerSorts[proposalColumnKind]
	if sortButton == nil {
		t.Fatal("sort control missing")
	}
	app.window.Canvas().Focus(sortButton)
	sortButton.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	if v.tableState.sortColumn != proposalColumnKind {
		t.Fatal("sort did not respond to Space")
	}
	if app.window.Canvas().Focused() != v.headerSorts[proposalColumnKind] {
		t.Fatalf("sort refresh lost focus: focused %p want %p old %p", app.window.Canvas().Focused(), v.headerSorts[proposalColumnKind], sortButton)
	}
}

func TestToolbarWrappingPreservesOrderAndMeasuredHeight(t *testing.T) {
	app := newTestApplication(t, localization.PreferenceSpanish)
	mountShell(app, fyne.NewSize(700, 768))
	app.shell.selectIndex(1)
	help, save := app.studio.helpButton, app.studio.save
	if absolute(app, help).Y < absolute(app, save).Y {
		t.Fatal("Help must remain last after wrapping")
	}
	assertReachable(t, app, help)
	splitPos := absolute(app, app.studio.workspaceSplit)
	if absolute(app, help).Y+help.Size().Height > splitPos.Y {
		t.Fatal("wrapped toolbar overlaps workspace")
	}
}

func TestNavigationAndHelpKeyboardInteraction(t *testing.T) {
	app := newTestApplication(t)
	app.window = &shortcutTestWindow{Window: app.window, canvas: &shortcutTestCanvas{Canvas: app.window.Canvas()}}
	mountShell(app, fyne.NewSize(1024, 768))
	app.installShortcuts()
	c := app.window.Canvas()
	c.Focus(app.shell.buttons[0])
	c.FocusNext()
	if c.Focused() != app.shell.buttons[1] {
		t.Fatal("Tab cannot reach next navigation item")
	}
	c.Focused().TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	if app.shell.selected != 1 || !strings.Contains(app.shell.buttons[1].Text, "•") {
		t.Fatal("keyboard navigation or non-color selection marker missing")
	}
	c.Focus(app.studio.helpButton)
	app.studio.helpButton.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	if c.Overlays().Top() == nil {
		t.Fatal("Help did not open")
	}
	for _, o := range visibleObjects(c.Overlays().Top()) {
		if p, ok := o.(*widget.PopUp); ok {
			assertReachable(t, app, p)
		}
	}
	c.(interface{ TypedShortcut(fyne.Shortcut) }).TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyEscape})
	if c.Overlays().Top() != nil {
		t.Fatal("Escape did not dismiss Help")
	}
}

func TestEscapeDismissesTopmostApplicationModal(t *testing.T) {
	app := newTestApplication(t)
	app.window = &shortcutTestWindow{Window: app.window, canvas: &shortcutTestCanvas{Canvas: app.window.Canvas()}}
	mountShell(app, fyne.NewSize(1024, 768))
	app.installShortcuts()
	canvas := app.window.Canvas()

	app.showAbout()
	app.showError(errors.New("failure"))
	if canvas.Overlays().Top() == nil {
		t.Fatal("expected stacked application modals")
	}

	escape := &desktop.CustomShortcut{KeyName: fyne.KeyEscape}
	canvas.(interface{ TypedShortcut(fyne.Shortcut) }).TypedShortcut(escape)
	if canvas.Overlays().Top() == nil {
		t.Fatal("Escape must dismiss only the topmost modal")
	}
	canvas.(interface{ TypedShortcut(fyne.Shortcut) }).TypedShortcut(escape)
	if canvas.Overlays().Top() != nil {
		t.Fatal("second Escape did not dismiss the remaining modal")
	}

	proposal := rename.Proposal{
		SourcePath: "/tmp/source", ParentDir: "/tmp", OriginalName: "source", ProposedName: "target", Kind: rename.NodeKindFile, Changed: true,
	}
	app.organizer.session = organizer.NewPreviewSession("/tmp", "default", []rename.Proposal{proposal})
	app.organizer.tableState.setProposals([]rename.Proposal{proposal})

	modalKinds := []struct {
		name string
		open func()
	}{
		{name: "Help", open: app.studio.showHelp},
		{name: "confirmation", open: app.organizer.confirmApply},
		{name: "form", open: func() { app.studio.promptForPatternName("Create", "", func(string) error { return nil }) }},
		{name: "folder chooser", open: app.organizer.OpenPath},
		{name: "settings folder chooser", open: app.settingsUI.chooseSettingsFolder},
		{name: "unsaved changes", open: func() {
			app.studio.dirty = true
			app.requestClose()
		}},
	}
	for _, modal := range modalKinds {
		t.Run(modal.name, func(t *testing.T) {
			modal.open()
			if canvas.Overlays().Top() == nil || len(app.modals) != 1 {
				t.Fatalf("%s modal was not registered", modal.name)
			}
			canvas.(interface{ TypedShortcut(fyne.Shortcut) }).TypedShortcut(escape)
			if canvas.Overlays().Top() != nil || len(app.modals) != 0 {
				t.Fatalf("Escape did not dismiss the %s modal", modal.name)
			}
			app.studio.dirty = false
		})
	}
}

// The software test canvas exposes registration but hides dispatch behind its
// interface. Record the same registrations while forwarding to the real canvas.
type shortcutTestCanvas struct {
	fyne.Canvas
	handler fyne.ShortcutHandler
}

func (c *shortcutTestCanvas) AddShortcut(s fyne.Shortcut, f func(fyne.Shortcut)) {
	c.handler.AddShortcut(s, f)
	c.Canvas.AddShortcut(s, f)
}
func (c *shortcutTestCanvas) RemoveShortcut(s fyne.Shortcut) {
	c.handler.RemoveShortcut(s)
	c.Canvas.RemoveShortcut(s)
}
func (c *shortcutTestCanvas) TypedShortcut(s fyne.Shortcut) { c.handler.TypedShortcut(s) }

type shortcutTestWindow struct {
	fyne.Window
	canvas fyne.Canvas
}

func (w *shortcutTestWindow) Canvas() fyne.Canvas { return w.canvas }

func TestTranslatedHeadersFitAndKeepActionsAccessible(t *testing.T) {
	for _, locale := range []localization.Preference{localization.PreferenceEnglish, localization.PreferenceSpanish} {
		t.Run(string(locale), func(t *testing.T) {
			app := newTestApplication(t, locale)
			mountShell(app, fyne.NewSize(1024, 768))
			v := app.organizer
			v.path.OnChanged = nil
			v.path.SetText("/example")
			v.tableState.proposals = make([]rename.Proposal, 60)
			for i := range v.tableState.proposals {
				v.tableState.proposals[i] = rename.Proposal{Kind: rename.NodeKindFile, RelativePath: fmt.Sprintf("%03d", i), OriginalName: "old", ProposedName: "New", Changed: true}
			}
			v.session = organizer.NewPreviewSession("", "", v.tableState.proposals)
			v.refreshProposalTable()
			for column := 0; column < proposalStickyColumnCount; column++ {
				header := v.table.CreateHeader()
				v.table.UpdateHeader(widget.TableCellID{Row: -1, Col: column}, header)
				if v.columnWidths[column] < header.MinSize().Width {
					t.Fatalf("translated header %d clips: %f < %f", column, v.columnWidths[column], header.MinSize().Width)
				}
			}
			apply := widget.NewButton(app.text("common.apply", "Apply"), nil)
			if v.columnWidths[proposalColumnAction] < apply.MinSize().Width+theme.Padding()*4 {
				t.Fatal("row Apply lost comfortable padding")
			}
			// Exercise the widths supplied by real workspace resizing, keeping all
			// descriptive headers reachable alongside the three fixed columns.
			for _, width := range []float32{1280, 1920, 1024} {
				app.window.Resize(fyne.NewSize(width, 768))
				for column := 0; column < proposalStickyColumnCount; column++ {
					assertReachable(t, app, v.headerFilters[column])
				}
				proposed := v.headerSorts[proposalColumnProposed]
				assertReachable(t, app, proposed)
				test.TapCanvas(app.window.Canvas(), absolute(app, proposed).Add(fyne.NewPos(10, 10)))
				if v.tableState.sortColumn != proposalColumnProposed {
					t.Fatal("resized header sorted the wrong column")
				}
			}

			v.table.ScrollToOffset(fyne.NewPos(0, 200))
			var scroll *container.Scroll
			for _, o := range visibleObjects(v.table) {
				if s, ok := o.(*container.Scroll); ok && s.Direction == container.ScrollBoth {
					scroll = s
					break
				}
			}
			if scroll == nil {
				t.Fatal("table scroller missing")
			}
			before := scroll.Offset
			if before.Y <= 0 {
				t.Fatal("fixture did not scroll")
			}
			app.settings.Language = localization.PreferenceSpanish
			if locale == localization.PreferenceSpanish {
				app.settings.Language = localization.PreferenceEnglish
			}
			app.refreshLanguage()
			if scroll.Offset.Y != before.Y {
				t.Fatalf("language refresh reset table scroll: %v -> %v", before, scroll.Offset)
			}
		})
	}
}

func visibleObjects(root fyne.CanvasObject) []fyne.CanvasObject {
	if !root.Visible() {
		return nil
	}
	out := []fyne.CanvasObject{root}
	var children []fyne.CanvasObject
	switch o := root.(type) {
	case *fyne.Container:
		children = o.Objects
	case fyne.Widget:
		children = test.WidgetRenderer(o).Objects()
	}
	for _, child := range children {
		out = append(out, visibleObjects(child)...)
	}
	return out
}

func TestThemeTextContrast(t *testing.T) {
	th := newOrganizerTheme()
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		for _, pair := range [][2]fyne.ThemeColorName{
			{theme.ColorNameForeground, theme.ColorNameBackground},
			{theme.ColorNameForeground, theme.ColorNameInputBackground},
			{theme.ColorNameForegroundOnPrimary, theme.ColorNamePrimary},
			{theme.ColorNameForegroundOnError, theme.ColorNameError},
			{theme.ColorNameForegroundOnSuccess, theme.ColorNameSuccess},
		} {
			a, b := luminance(th.Color(pair[0], variant)), luminance(th.Color(pair[1], variant))
			if a < b {
				a, b = b, a
			}
			if ratio := (a + 0.05) / (b + 0.05); ratio < 4.5 {
				t.Fatalf("%s on %s in variant %d has contrast %.2f", pair[0], pair[1], variant, ratio)
			}
		}
	}
}
func luminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	linear := func(v uint32) float64 {
		x := float64(v) / 65535
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(r) + 0.7152*linear(g) + 0.0722*linear(b)
}

func TestLongOperationalFeedbackReservesItsWrappedHeight(t *testing.T) {
	app := newTestApplication(t, localization.PreferenceSpanish)
	mountShell(app, fyne.NewSize(1024, 768))
	v := app.organizer
	v.setStatus("organizer.status_unavailable", "Folder is unavailable: {{.Error}}", map[string]any{"Error": strings.Repeat("/very-long-folder-name", 18)})
	if v.status.Size().Height+1 < v.status.MinSize().Height {
		t.Fatalf("wrapped feedback clipped: allocated %v, needs %v", v.status.Size(), v.status.MinSize())
	}
}

func TestScanningFeedbackRelayoutsImmediately(t *testing.T) {
	app := newTestApplication(t, localization.PreferenceSpanish)
	mountShell(app, fyne.NewSize(1024, 768))
	v := app.organizer
	v.setScanning(true)
	assertReachable(t, app, v.cancel)
	if v.cancel.Size().Height < 32 || v.cancel.Size().Width < v.cancel.MinSize().Width {
		t.Fatalf("Cancel was not laid out: %v", v.cancel.Size())
	}
	if absolute(app, v.cancel).Y != absolute(app, v.status).Y {
		t.Fatal("Cancel must appear beside scanning feedback")
	}
	if absolute(app, v.cancel).Y+v.cancel.Size().Height > absolute(app, v.apply).Y {
		t.Fatal("Cancel overlaps preview toolbar")
	}
	v.setScanning(false)
	if v.cancel.Visible() {
		t.Fatal("Cancel remains visible after scan")
	}
}
