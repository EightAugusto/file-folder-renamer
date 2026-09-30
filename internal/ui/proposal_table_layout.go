package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// proposalTableLayout recalculates widths whenever its available screen space
// changes. The Location column receives any remaining width after compact
// action, enum, and status columns have been sized.
type proposalTableLayout struct {
	view        *OrganizerView
	headerGuard *proposalTableHeaderGuard
}

func (layout *proposalTableLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	layout.view.updateProposalTableColumns(size.Width)
	for _, object := range objects {
		if object == layout.headerGuard {
			object.Move(fyne.Position{})
			object.Resize(fyne.NewSize(size.Width, newProposalHeader().MinSize().Height))
			continue
		}
		object.Move(fyne.Position{})
		object.Resize(size)
	}
}

// proposalTableHeaderGuard absorbs header drags so the user cannot resize
// Action, Kind, Status (nor leave a manual width that fights the layout).
// It is transparent and covers only the table header; table rows keep their
// normal selection and individual Apply-button behavior.
type proposalTableHeaderGuard struct {
	widget.BaseWidget
	view *OrganizerView
}

func newProposalTableHeaderGuard(view *OrganizerView) *proposalTableHeaderGuard {
	guard := &proposalTableHeaderGuard{view: view}
	guard.ExtendBaseWidget(guard)
	return guard
}

func (guard *proposalTableHeaderGuard) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

// Only drags are intercepted. Taps and keyboard focus reach the actual header
// buttons, which Fyne positions correctly even when descriptive columns scroll.
func (*proposalTableHeaderGuard) Dragged(*fyne.DragEvent) {}
func (*proposalTableHeaderGuard) DragEnd()                {}

func (layout *proposalTableLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	minimum := fyne.Size{}
	for _, object := range objects {
		minimum = minimum.Max(object.MinSize())
	}
	return minimum
}

func newProposalHeader() *fyne.Container {
	sort := widget.NewButtonWithIcon("", theme.MenuDropDownIcon(), nil)
	sort.Alignment = widget.ButtonAlignLeading
	sort.IconPlacement = widget.ButtonIconTrailingText
	filter := widget.NewButtonWithIcon("", theme.MenuIcon(), nil)
	return container.New(&proposalHeaderLayout{}, sort, filter)
}

type proposalHeaderLayout struct{}

func (*proposalHeaderLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	w := float32(0)
	if objects[1].Visible() {
		w = objects[1].MinSize().Width
		objects[1].Move(fyne.NewPos(size.Width-w, 0))
		objects[1].Resize(fyne.NewSize(w, size.Height))
		w += theme.Padding()
	}
	objects[0].Move(fyne.Position{})
	objects[0].Resize(fyne.NewSize(maxFloat32(0, size.Width-w), size.Height))
}
func (*proposalHeaderLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	size := objects[0].MinSize()
	if objects[1].Visible() {
		size.Width += objects[1].MinSize().Width + theme.Padding()
		size.Height = maxFloat32(size.Height, objects[1].MinSize().Height)
	}
	return size
}
