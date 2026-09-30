package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/eightaugusto/file-folder-renamer/internal/organizer"
	"github.com/eightaugusto/file-folder-renamer/internal/pattern"
	"github.com/eightaugusto/file-folder-renamer/internal/rename"
)

type OrganizerView struct {
	filterPopup    *widget.PopUp
	filterLabels   textBindings
	filterColumn   int
	feedback       localizedFeedback
	labels         textBindings
	application    *Application
	content        fyne.CanvasObject
	path           *widget.Entry
	files          *widget.Check
	folders        *widget.Check
	patterns       *widget.Select
	cancel         *widget.Button
	apply          *widget.Button
	status         *widget.Label
	summary        *widget.Label
	table          *widget.Table
	tableHost      *fyne.Container
	emptyState     fyne.CanvasObject
	tableState     proposalTableState
	session        *organizer.PreviewSession
	cancelScan     context.CancelFunc
	previewDone    chan struct{}
	applyDone      chan struct{}
	previewNext    bool
	previewVersion uint64
	applying       bool
	patternByID    map[string]pattern.Spec
	headerFilters  map[int]fyne.CanvasObject
	headerSorts    map[int]*widget.Button
	updating       bool
	columnWidths   [6]float32
}

// Keep the interactive state columns together at the left side of the table.
// They remain visible while the descriptive columns scroll horizontally.
const (
	proposalColumnAction = iota
	proposalColumnKind
	proposalColumnStatus
	proposalColumnLocation
	proposalColumnOriginal
	proposalColumnProposed
	proposalStickyColumnCount = 3
)

func NewOrganizerView(application *Application) *OrganizerView {
	view := &OrganizerView{application: application, patternByID: make(map[string]pattern.Spec), headerFilters: make(map[int]fyne.CanvasObject), headerSorts: make(map[int]*widget.Button), tableState: newProposalTableState()}
	view.path = widget.NewEntry()
	view.path.SetPlaceHolder(application.text("organizer.choose_folder", "Choose a folder"))
	view.path.OnChanged = func(string) { view.invalidatePreview() }
	view.path.OnSubmitted = func(string) { view.StartPreview() }
	view.files = view.labels.check(func() string { return application.text("organizer.files", "Files") }, func(bool) { view.inputsChanged() })
	view.files.SetChecked(application.settings.IncludeFiles)
	view.folders = view.labels.check(func() string { return application.text("organizer.folders", "Folders") }, func(bool) { view.inputsChanged() })
	view.folders.SetChecked(application.settings.IncludeFolders)
	view.patterns = widget.NewSelect(nil, func(string) { view.inputsChanged() })
	view.cancel = view.labels.buttonIcon(func() string { return application.text("common.cancel", "Cancel") }, theme.CancelIcon(), view.CancelPreview)
	view.cancel.Hide()
	view.status = widget.NewLabel("")
	view.setStatus("organizer.status_choose_folder", "Choose a folder to preview proposed names automatically.")
	view.labels.add(func() { view.feedback.refresh(application, view.status) })
	view.status.Wrapping = fyne.TextWrapWord
	view.summary = view.labels.label(func() string { return application.text("organizer.no_preview", "No current preview") })
	view.apply = view.labels.buttonIcon(func() string { return application.text("organizer.apply_all", "Apply all changes") }, theme.ConfirmIcon(), view.confirmApply)
	view.apply.Importance = widget.HighImportance
	view.apply.Disable()
	view.table = view.newProposalTable()
	headerGuard := newProposalTableHeaderGuard(view)

	choosePath := view.labels.buttonIcon(func() string { return application.text("common.choose", "Choose…") }, theme.FolderOpenIcon(), view.OpenPath)
	folderControl := container.NewBorder(nil, nil, nil, choosePath, view.path)
	controls := newVertical(spaceSM,
		newResponsiveForm(
			view.labels.formItem(func() string { return application.text("organizer.folder", "Folder") }, folderControl),
			view.labels.formItem(func() string { return application.text("common.pattern", "Pattern") }, view.patterns),
		),
		newFlow(view.labels.label(func() string { return application.text("organizer.include", "Include") }), view.files, view.folders),
		container.New(&feedbackLayout{}, view.status, view.cancel),
	)
	view.emptyState = container.NewCenter(container.NewVBox(
		widget.NewIcon(theme.FolderOpenIcon()),
		view.labels.label(func() string { return application.text("organizer.empty_title", "Choose a folder to get started") }),
		view.labels.label(func() string {
			return application.text("organizer.empty_detail", "Review proposed names here before applying changes.")
		}),
	))
	view.tableHost = container.New(&proposalTableLayout{view: view, headerGuard: headerGuard}, view.table, headerGuard)
	tableArea := container.NewStack(view.tableHost, view.emptyState)
	previewHeading := view.labels.label(func() string { return application.text("organizer.preview_title", "Rename preview") })
	previewHeading.TextStyle = fyne.TextStyle{Bold: true}
	previewBar := newFlow(previewHeading, view.summary, view.apply)
	view.summary.Wrapping = fyne.TextWrapOff
	top := newVertical(spaceSM, controls, widget.NewSeparator(), previewBar)
	view.content = newWorkspace(top, nil, tableArea)

	view.RefreshPatterns("default")
	return view
}

func (view *OrganizerView) Content() fyne.CanvasObject { return view.content }
func (view *OrganizerView) IsApplying() bool           { return view.applying }

func (view *OrganizerView) Configuration() (string, bool, bool) {
	return view.patterns.Selected, view.files.Checked, view.folders.Checked
}

func (view *OrganizerView) RefreshPatterns(prefer string) {
	selected := prefer
	if selected == "" {
		selected = view.patterns.Selected
	}
	view.patternByID = make(map[string]pattern.Spec)
	names := make([]string, 0)
	for _, entry := range view.application.store.Entries() {
		names = append(names, entry.Spec.Name)
		view.patternByID[strings.ToLower(entry.Spec.Name)] = entry.Spec
	}
	view.updating = true
	view.patterns.Options = names
	view.patterns.Refresh()
	if _, found := view.patternByID[strings.ToLower(selected)]; found {
		view.patterns.SetSelected(selected)
	} else if len(names) > 0 {
		view.patterns.SetSelected(names[0])
	}
	view.updating = false
	view.inputsChanged()
}

func (view *OrganizerView) SetConfiguration(patternName string, files, folders bool) {
	view.updating = true
	view.files.SetChecked(files)
	view.folders.SetChecked(folders)
	view.updating = false
	view.RefreshPatterns(patternName)
}

func (view *OrganizerView) OpenPath() {
	picker := dialog.NewFolderOpen(view.folderSelected, view.application.window)
	view.application.showDialog(picker)
	// FileDialog creates its internal popup during Show; Resize before Show
	// panics in Fyne v2.8.1 because MinSize dereferences that popup.
	picker.Resize(folderPickerSize(view.application.window))
	if overlay := view.application.window.Canvas().Overlays().Top(); overlay != nil {
		overlay.Refresh()
	}
}

func (view *OrganizerView) folderSelected(uri fyne.ListableURI, err error) {
	if err != nil {
		view.application.showError(err)
		return
	}
	if uri != nil {
		view.selectFolder(uri.Path())
	}
}

func folderPickerSize(window fyne.Window) fyne.Size {
	size := window.Canvas().Size()
	if size.Width <= 0 || size.Height <= 0 {
		size = fyne.NewSize(defaultWindowWidth, defaultWindowHeight)
	}
	return fyne.NewSize(size.Width*0.5, size.Height*0.5)
}

func (view *OrganizerView) StartPreview() {
	// A new request cancels the active scan and starts after its callback exits.
	if view.cancelScan != nil {
		view.previewNext = true
		view.cancelScan()
		return
	}
	view.previewNext = false
	view.invalidatePreview()
	root := strings.TrimSpace(view.path.Text)
	if root == "" {
		return
	}
	info, err := os.Stat(root)
	if err != nil {
		view.setStatus("organizer.status_unavailable", "Folder is unavailable: {{.Error}}", map[string]any{"Error": err.Error()})
		return
	}
	if !info.IsDir() {
		view.setStatus("organizer.status_file_not_allowed", "Choose a folder; individual files are not supported in Organizer.")
		return
	}
	if !view.files.Checked && !view.folders.Checked {
		view.setStatus("organizer.status_include_required", "Include files, folders, or both to preview.")
		return
	}
	spec, found := view.patternByID[strings.ToLower(view.patterns.Selected)]
	if !found {
		view.setStatus("organizer.status_pattern_required", "Choose a saved pattern to preview.")
		return
	}
	request := organizer.PreviewRequest{
		Root: root, IncludeFiles: view.files.Checked, IncludeFolders: view.folders.Checked,
		IgnoredNames: append([]string(nil), view.application.settings.IgnoredNames...), Pattern: spec,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	view.cancelScan = cancel
	view.previewDone = done
	version := view.previewVersion
	view.setScanning(true)
	go func() {
		session, err := view.application.service.Preview(ctx, request)
		fyne.Do(func() {
			defer close(done)
			view.cancelScan = nil
			view.setScanning(false)
			restart := view.previewNext
			view.previewNext = false
			if restart {
				view.StartPreview()
				return
			}
			// Ignore a result if folder or options changed during the scan.
			if version != view.previewVersion {
				return
			}
			if err != nil {
				if errors.Is(err, context.Canceled) {
					view.setStatus("organizer.status_canceled", "Preview canceled.")
					return
				}
				view.setStatus("organizer.status_preview_failed", "Preview failed: {{.Error}}", map[string]any{"Error": err.Error()})
				view.application.showError(err)
				return
			}
			view.session = session
			view.tableState.setProposals(session.ProposalSnapshot())
			view.refreshProposalTable()
			view.summary.SetText(view.application.text("organizer.summary", "{{.Changed}} changed  •  {{.Unchanged}} unchanged  •  {{.Total}} total", map[string]any{"Changed": session.ChangedCount(), "Unchanged": session.UnchangedCount(), "Total": len(view.tableState.proposals)}))
			view.setStatus("organizer.status_previewed", "Previewed {{.Root}} with pattern {{.Pattern}}.", map[string]any{"Root": session.Root(), "Pattern": fmt.Sprintf("%q", session.PatternName())})
			if session.ChangedCount() > 0 {
				view.apply.Enable()
			}
		})
	}()
}

func (view *OrganizerView) CancelPreview() {
	if view.cancelScan != nil {
		view.previewNext = false
		view.cancelScan()
	}
}

func (view *OrganizerView) confirmApply() {
	if view.session == nil || view.session.ChangedCount() == 0 {
		return
	}
	message := view.application.texts().Plural("organizer.confirm_apply_message", "Rename {{.Count}} item under:\n{{.Root}}\n\nPattern: {{.Pattern}}\n\nThe selected folder is checked again before changes begin.", "Rename {{.Count}} items under:\n{{.Root}}\n\nPattern: {{.Pattern}}\n\nThe selected folder is checked again before changes begin.", view.session.ChangedCount(), map[string]any{"Count": view.session.ChangedCount(), "Root": view.session.Root(), "Pattern": view.session.PatternName()})
	prompt := dialog.NewConfirm(view.application.text("organizer.confirm_apply_title", "Apply proposed renames?"), message, func(confirmed bool) {
		if confirmed {
			view.startApply()
		}
	}, view.application.window)
	prompt.SetConfirmText(view.application.text("common.apply", "Apply"))
	prompt.SetDismissText(view.application.text("common.cancel", "Cancel"))
	view.application.showDialog(prompt)
}

func (view *OrganizerView) startApply() {
	session := view.session
	if session == nil {
		return
	}
	if view.applying {
		return
	}
	view.applying = true
	done := make(chan struct{})
	view.applyDone = done
	nextRoot := rootAfterAppliedProposals(session.Root(), session.ProposalSnapshot())
	view.setControlsEnabled(false)
	view.refreshProposalTable()
	view.setStatus("organizer.status_applying_all", "Applying renames…")
	// Apply runs off the UI thread; its callback replaces the stale preview.
	go func() {
		result, err := view.application.service.Apply(context.Background(), session)
		fyne.Do(func() {
			defer close(done)
			view.applying = false
			view.setControlsEnabled(true)
			if err != nil {
				view.invalidatePreview()
				view.setStatus("organizer.status_apply_failed", "Apply failed: {{.Error}}", map[string]any{"Error": err.Error()})
				view.application.showError(err)
				view.StartPreview()
				return
			}
			view.invalidatePreview()
			view.application.showInformation(view.application.text("organizer.complete_title", "Renames complete"), view.application.appliedMessage(result.AppliedCount))
			if nextRoot != session.Root() {
				view.path.SetText(nextRoot)
			}
			view.StartPreview()
		})
	}()
}

func (view *OrganizerView) confirmApplyOne(index int) {
	if view.session == nil || index < 0 || index >= len(view.tableState.proposals) || !view.tableState.proposals[index].Changed {
		return
	}
	item := view.tableState.proposals[index]
	message := view.application.text("organizer.confirm_apply_one_message", "Rename {{.Original}} to {{.Proposed}}?\n\nThe selected path is checked again before this change begins.", map[string]any{"Original": fmt.Sprintf("%q", item.OriginalName), "Proposed": fmt.Sprintf("%q", item.ProposedName)})
	prompt := dialog.NewConfirm(view.application.text("organizer.confirm_apply_one_title", "Apply this rename?"), message, func(confirmed bool) {
		if confirmed {
			view.startApplyOne(index)
		}
	}, view.application.window)
	prompt.SetConfirmText(view.application.text("common.apply", "Apply"))
	prompt.SetDismissText(view.application.text("common.cancel", "Cancel"))
	view.application.showDialog(prompt)
}

func (view *OrganizerView) startApplyOne(index int) {
	session := view.session
	if session == nil || view.applying {
		return
	}
	view.applying = true
	done := make(chan struct{})
	view.applyDone = done
	if index < 0 || index >= len(view.tableState.proposals) {
		view.applying = false
		close(done)
		return
	}
	selected := view.tableState.proposals[index]
	nextRoot := rootAfterAppliedProposals(session.Root(), []rename.Proposal{selected})
	sourcePath := selected.SourcePath
	view.setControlsEnabled(false)
	view.refreshProposalTable()
	view.setStatus("organizer.status_applying_one", "Applying selected rename…")
	go func() {
		result, err := view.application.service.ApplyOne(context.Background(), session, sourcePath)
		fyne.Do(func() {
			defer close(done)
			view.applying = false
			view.setControlsEnabled(true)
			if err != nil {
				view.invalidatePreview()
				view.setStatus("organizer.status_apply_one_failed", "Selected rename failed: {{.Error}}", map[string]any{"Error": err.Error()})
				view.application.showError(err)
				view.StartPreview()
				return
			}
			view.invalidatePreview()
			view.application.showInformation(view.application.text("organizer.complete_one_title", "Rename complete"), view.application.appliedMessage(result.AppliedCount))
			if nextRoot != session.Root() {
				view.path.SetText(nextRoot)
			}
			view.StartPreview()
		})
	}()
}

func rootAfterAppliedProposals(root string, proposals []rename.Proposal) string {
	cleanRoot := filepath.Clean(root)
	for _, item := range proposals {
		if item.Changed && filepath.Clean(item.SourcePath) == cleanRoot {
			return filepath.Join(item.ParentDir, item.ProposedName)
		}
	}
	return root
}

func (view *OrganizerView) selectFolder(path string) {
	view.path.SetText(path)
	view.StartPreview()
}

func (view *OrganizerView) inputsChanged() {
	if view.updating {
		return
	}
	view.invalidatePreview()
	if strings.TrimSpace(view.path.Text) != "" {
		view.StartPreview()
	}
}

func (view *OrganizerView) invalidatePreview() {
	if view.updating {
		return
	}
	// The version also invalidates an in-flight scan result.
	view.previewVersion++
	if view.status != nil {
		if strings.TrimSpace(view.path.Text) != "" {
			view.setStatus("organizer.status_stale", "Folder or options changed. Waiting for a current preview; press Enter after editing the path.")
		} else {
			view.setStatus("organizer.status_choose_folder", "Choose a folder to preview proposed names automatically.")
		}
	}
	view.session = nil
	view.tableState.setProposals(nil)
	if view.apply != nil {
		view.apply.Disable()
	}
	if view.table != nil {
		view.refreshProposalTable()
	}
	if view.summary != nil {
		view.summary.SetText(view.application.text("organizer.no_preview", "No current preview"))
	}
}

func (view *OrganizerView) setScanning(scanning bool) {
	if scanning {
		view.path.Disable()
		view.files.Disable()
		view.folders.Disable()
		view.patterns.Disable()
		view.cancel.Show()
		view.setStatus("organizer.status_scanning", "Scanning selection…")
		return
	}
	view.path.Enable()
	view.files.Enable()
	view.folders.Enable()
	view.patterns.Enable()
	view.cancel.Hide()
	view.content.Refresh()
}

func (view *OrganizerView) setControlsEnabled(enabled bool) {
	controls := []fyne.Disableable{view.path, view.files, view.folders, view.patterns, view.apply}
	for _, control := range controls {
		if enabled {
			control.Enable()
		} else {
			control.Disable()
		}
	}
}

func (view *OrganizerView) newProposalTable() *widget.Table {
	headers := proposalTableHeaders()
	table := widget.NewTable(
		func() (int, int) { return len(view.visibleProposalIndexes()), len(headers) },
		func() fyne.CanvasObject { return newProposalTableCell() },
		func(id widget.TableCellID, object fyne.CanvasObject) {
			cell := object.(*fyne.Container)
			label := cell.Objects[0].(*widget.Label)
			action := cell.Objects[1].(*fyne.Container).Objects[0].(*widget.Button)
			action.Hide()
			label.Show()
			indexes := view.visibleProposalIndexes()
			if id.Row < 0 || id.Row >= len(indexes) {
				return
			}
			index := indexes[id.Row]
			item := view.tableState.proposals[index]
			values := proposalTableValues(item)
			values[proposalColumnKind] = nodeKindLabel(item.Kind, view.application)
			values[proposalColumnStatus] = view.filterLabel(proposalStatus(item.Changed))
			if id.Col == proposalColumnAction && item.Changed {
				label.Hide()
				action.SetText(view.application.text("common.apply", "Apply"))
				action.OnTapped = func() { view.confirmApplyOne(index) }
				if view.applying {
					action.Disable()
				} else {
					action.Enable()
				}
				action.Show()
				return
			}
			label.SetText(values[id.Col])
		},
	)
	table.ShowHeaderRow = true
	table.StickyColumnCount = proposalStickyColumnCount
	table.CreateHeader = func() fyne.CanvasObject { return newProposalHeader() }
	table.UpdateHeader = func(id widget.TableCellID, object fyne.CanvasObject) {
		if id.Row >= 0 || id.Col < 0 || id.Col >= len(headers) {
			return
		}
		header := object.(*fyne.Container)
		sortButton := header.Objects[0].(*widget.Button)
		view.headerSorts[id.Col] = sortButton
		filterButton := header.Objects[1].(*widget.Button)
		sortButton.SetText(view.proposalTableHeaderText(id.Col))
		sortButton.SetIcon(view.proposalTableHeaderIcon(id.Col))
		sortButton.OnTapped = func() {
			view.toggleProposalSort(id.Col)
			view.application.window.Canvas().Focus(view.headerSorts[id.Col])
		}
		sortButton.Importance = widget.LowImportance
		if view.tableState.sortColumn == id.Col {
			sortButton.Importance = widget.MediumImportance
		}
		if len(proposalFilterOptions(id.Col)) == 0 {
			filterButton.Hide()
			delete(view.headerFilters, id.Col)
		} else {
			filterButton.Show()
			filterButton.OnTapped = func() { view.openProposalFilter(id.Col, filterButton) }
			filterButton.Importance = widget.LowImportance
			filterButton.SetText("")
			if count := len(view.proposalFilter(id.Col)); count > 0 {
				filterButton.Importance = widget.MediumImportance
				filterButton.SetText(strconv.Itoa(count))
			}
			view.headerFilters[id.Col] = filterButton
		}
		header.Refresh()
	}

	return table
}

func (view *OrganizerView) proposalTableHeaderText(column int) string {
	headers := proposalTableHeaders()
	if column < 0 || column >= len(headers) {
		return ""
	}
	keys := []string{"common.action", "common.kind", "table.status", "table.location", "table.original", "table.proposed"}
	return view.application.text(keys[column], headers[column])
}

func (view *OrganizerView) proposalTableHeaderIcon(column int) fyne.Resource {
	if column != view.tableState.sortColumn {
		return nil
	}
	if view.tableState.sortColumn == column && view.tableState.sortAscending {
		return theme.MenuDropUpIcon()
	}
	return theme.MenuDropDownIcon()
}

func proposalFilterOptions(column int) []string {
	switch column {
	case proposalColumnKind:
		return []string{"File", "Folder"}
	case proposalColumnStatus:
		return []string{"Changed", "Unchanged"}
	case proposalColumnAction:
		return []string{"Apply", "No action"}
	default:
		return nil
	}
}

func (view *OrganizerView) proposalFilter(column int) map[string]struct{} {
	switch column {
	case proposalColumnKind:
		return view.tableState.kindFilter
	case proposalColumnStatus:
		return view.tableState.statusFilter
	case proposalColumnAction:
		return view.tableState.actionFilter
	default:
		return nil
	}
}

// Filter values remain stable regardless of the display language.
func (view *OrganizerView) filterLabel(value string) string {
	keys := map[string]string{"File": "node.file", "Folder": "node.folder", "Changed": "table.changed", "Unchanged": "table.unchanged", "Apply": "common.apply", "No action": "table.no_action"}
	return view.application.text(keys[value], value)
}

func (view *OrganizerView) setProposalFilters(column int, values []string) {
	allowed := make(map[string]struct{})
	for _, option := range proposalFilterOptions(column) {
		allowed[option] = struct{}{}
	}
	selected := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, valid := allowed[value]; valid {
			selected[value] = struct{}{}
		}
	}
	switch column {
	case proposalColumnKind:
		view.tableState.kindFilter = selected
	case proposalColumnStatus:
		view.tableState.statusFilter = selected
	case proposalColumnAction:
		view.tableState.actionFilter = selected
	default:
		return
	}
	view.refreshProposalTable()
}

func (view *OrganizerView) openProposalFilter(column int, anchor fyne.CanvasObject) {
	options := proposalFilterOptions(column)
	view.filterLabels = nil
	view.filterColumn = column
	if len(options) == 0 {
		return
	}
	selected := make(map[string]struct{}, len(view.proposalFilter(column)))
	for value := range view.proposalFilter(column) {
		selected[value] = struct{}{}
	}
	checks := make([]fyne.CanvasObject, 0, len(options))
	for _, option := range options {
		option := option
		check := view.filterLabels.check(func() string { return view.filterLabel(option) }, func(checked bool) {
			if checked {
				selected[option] = struct{}{}
			} else {
				delete(selected, option)
			}
		})
		_, checked := selected[option]
		check.SetChecked(checked)
		checks = append(checks, check)
	}

	var popup *widget.PopUp
	clear := view.filterLabels.button(func() string { return view.application.text("table.clear", "Clear") }, func() {
		for value := range selected {
			delete(selected, value)
		}
		for _, object := range checks {
			object.(*widget.Check).SetChecked(false)
		}
	})
	apply := view.filterLabels.button(func() string { return view.application.text("common.apply", "Apply") }, func() {
		values := make([]string, 0, len(selected))
		for _, option := range options {
			if _, checked := selected[option]; checked {
				values = append(values, option)
			}
		}
		view.setProposalFilters(column, values)
		popup.Hide()
		if button, ok := view.headerFilters[column].(*widget.Button); ok {
			view.application.window.Canvas().Focus(button)
		}
	})
	apply.Importance = widget.HighImportance
	help := view.filterLabels.label(func() string {
		return view.application.text("table.filter_help", "Select one or more values. Clear removes this filter.")
	})
	help.Wrapping = fyne.TextWrapWord
	content := newVertical(spaceSM, help, container.NewVBox(checks...), newFlow(clear, apply))
	popup = widget.NewPopUp(content, view.application.window.Canvas())
	view.filterPopup = popup
	popup.Resize(fyne.NewSize(320, heightAt(content, 320)+spaceLG))
	popup.ShowAtRelativePosition(fyne.NewPos(0, anchor.Size().Height), anchor)
	view.application.window.Canvas().Focus(checks[0].(*widget.Check))
}

func (view *OrganizerView) visibleProposalIndexes() []int {
	return view.tableState.visibleIndexes()
}

func containsProposalFilter(filters map[string]struct{}, value string) bool {
	_, found := filters[value]
	return found
}

func (view *OrganizerView) toggleProposalSort(column int) {
	if column < 0 || column >= 6 {
		return
	}
	view.tableState.toggleSort(column)
	view.refreshProposalTable()
}

func proposalTableHeaders() []string {
	return []string{"Action", "Kind", "Status", "Location", "Original", "Proposed"}
}

func proposalSortValue(item rename.Proposal, column int) string {
	values := []string{
		proposalAction(item),
		enumLabel(string(item.Kind)),
		proposalStatus(item.Changed),
		item.RelativePath,
		item.OriginalName,
		item.ProposedName,
	}
	return strings.ToLower(values[column])
}

func proposalAction(item rename.Proposal) string {
	if item.Changed {
		return "Apply"
	}
	return "No action"
}

func newProposalTableCell() *fyne.Container {
	label := widget.NewLabel(" ")
	label.Truncation = fyne.TextTruncateEllipsis
	action := widget.NewButton("Apply", nil)
	return container.NewStack(label, container.New(layout.NewCustomPaddedLayout(spaceXS, spaceXS, spaceSM, spaceSM), action))
}

func proposalTableValues(item rename.Proposal) []string {
	return []string{
		"",
		enumLabel(string(item.Kind)),
		proposalStatus(item.Changed),
		truncateRunes(item.RelativePath, 120),
		truncateRunes(item.OriginalName, 60),
		truncateRunes(item.ProposedName, 60),
	}
}

func enumLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	runes := []rune(value)
	return strings.ToUpper(string(runes[0])) + string(runes[1:])
}

func proposalStatus(changed bool) string {
	if changed {
		return "Changed"
	}
	return "Unchanged"
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 1 {
		return "…"
	}
	return string(runes[:limit-1]) + "…"
}

func (view *OrganizerView) refreshProposalTable() {
	if view.table == nil {
		return
	}
	focused := view.application.window.Canvas().Focused()
	focusColumn := -1
	for column, button := range view.headerSorts {
		if button == focused {
			focusColumn = column
			break
		}
	}

	// The host is the viewport supplied by the Organizer card. The table can be
	// wider than that viewport because it is scrollable, so using table.Size()
	// would prevent Location from consuming the actual available screen width.
	availableWidth := view.table.Size().Width
	if view.tableHost != nil && view.tableHost.Size().Width > 0 {
		availableWidth = view.tableHost.Size().Width
	}
	view.updateProposalTableColumns(availableWidth)
	if view.emptyState != nil {
		if strings.TrimSpace(view.path.Text) == "" && view.session == nil {
			view.emptyState.Show()
			view.tableHost.Hide()
		} else {
			view.emptyState.Hide()
			view.tableHost.Show()
		}
	}
	view.table.Refresh()
	if focusColumn >= 0 {
		view.application.window.Canvas().Focus(view.headerSorts[focusColumn])
	}
}

func (view *OrganizerView) updateProposalTableColumns(availableWidth float32) {
	// Action, Kind, and Status are fixed. Location, Original, and Proposed use
	// 30-character defaults; they are the only columns allowed to shrink when
	// necessary, and Location receives any extra viewport width.
	widths := [6]float32{
		maxFloat32(proposalActionColumnWidth(view.application), proposalFilterColumnWidth([]string{view.application.text("common.action", "Action")}, proposalFilterOptions(proposalColumnAction))),
		proposalFilterColumnWidth([]string{view.application.text("common.kind", "Kind"), view.application.text("node.file", "File"), view.application.text("organizer.folder", "Folder")}, proposalFilterOptions(proposalColumnKind)),
		proposalFilterColumnWidth([]string{view.application.text("table.status", "Status"), view.application.text("table.changed", "Changed"), view.application.text("table.unchanged", "Unchanged")}, proposalFilterOptions(proposalColumnStatus)),
		defaultTableTextWidth(30),
		defaultTableTextWidth(30),
		defaultTableTextWidth(30),
	}
	fixedWidth := float32(0)
	for index, width := range widths {
		if index < proposalStickyColumnCount {
			fixedWidth += width
		}
	}
	flexibleWidth := availableWidth - fixedWidth - theme.Padding()*float32(len(widths)-1)
	widths[proposalColumnLocation], widths[proposalColumnOriginal], widths[proposalColumnProposed] = fitProposalFlexibleWidths(flexibleWidth, widths[proposalColumnLocation], widths[proposalColumnOriginal], widths[proposalColumnProposed])
	for index, width := range widths {
		if view.columnWidths[index] != width {
			view.columnWidths[index] = width
			view.table.SetColumnWidth(index, width)
		}
	}
}

func fitProposalFlexibleWidths(available, location, original, proposed float32) (float32, float32, float32) {
	defaultWidth := location + original + proposed
	if available <= 0 {
		return 0, 0, 0
	}
	if available >= defaultWidth {
		return location + available - defaultWidth, original, proposed
	}
	scale := available / defaultWidth
	return location * scale, original * scale, proposed * scale
}

func defaultTableTextWidth(characters int) float32 {
	return widget.NewLabel(strings.Repeat("M", characters)).MinSize().Width + theme.Padding()*2
}

func proposalActionColumnWidth(application *Application) float32 {
	// The button itself uses Fyne's normal compact padding; reserve additional
	// cell space so it does not visually touch the column separators.
	return widget.NewButton(application.text("common.apply", "Apply"), nil).MinSize().Width + theme.Padding()*4
}

func proposalFilterColumnWidth(labels, options []string) float32 {
	width := widestTableLabel(labels)
	if len(labels) > 0 {
		header := widget.NewButtonWithIcon(labels[0], theme.MenuDropUpIcon(), nil)
		width = maxFloat32(width, header.MinSize().Width+widget.NewButtonWithIcon(strconv.Itoa(len(options)), theme.MenuIcon(), nil).MinSize().Width+theme.Padding())
	}
	return width
}

func maxFloat32(left, right float32) float32 {
	if left > right {
		return left
	}
	return right
}

func widestTableLabel(values []string) float32 {
	width := float32(0)
	for _, value := range values {
		labelWidth := widget.NewLabel(value).MinSize().Width + theme.Padding()*2
		if labelWidth > width {
			width = labelWidth
		}
	}
	return width
}
