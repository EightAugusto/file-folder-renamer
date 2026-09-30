package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ignoredNameRow is a value and its explicit Delete action. It is shared by
// the ignored-names and excluded-regex tables.
type ignoredNameRow struct {
	widget.BaseWidget
	name      *widget.Label
	remove    *widget.Button
	index     int
	onDelete  func(int)
	deletable bool
}

func newIgnoredNameRow(application *Application) *ignoredNameRow {
	row := &ignoredNameRow{
		name: widget.NewLabel(""),
	}
	row.remove = widget.NewButtonWithIcon(application.text("common.delete", "Delete"), theme.DeleteIcon(), func() {
		if row.onDelete != nil {
			row.onDelete(row.index)
		}
	})
	row.ExtendBaseWidget(row)
	return row
}

func deleteTableActionWidth(application *Application) float32 {
	return widget.NewButtonWithIcon(application.text("common.delete", "Delete"), theme.DeleteIcon(), nil).MinSize().Width
}

func (row *ignoredNameRow) CreateRenderer() fyne.WidgetRenderer {
	// The action belongs in a stable right-side table column, instead of relying
	// on a hover-only control that is easy to miss while scrolling.
	action := container.NewBorder(nil, nil, widget.NewSeparator(), nil, row.remove)
	return widget.NewSimpleRenderer(container.NewBorder(nil, nil, nil, action, row.name))
}

func (row *ignoredNameRow) set(index int, name string, onDelete func(int)) {
	row.index = index
	row.name.SetText(name)
	row.onDelete = onDelete
	row.deletable = onDelete != nil
	if row.deletable {
		row.remove.Enable()
	} else {
		row.remove.Disable()
	}
}

func (row *ignoredNameRow) MinSize() fyne.Size {
	nameSize := row.name.MinSize()
	deleteSize := row.remove.MinSize()
	return fyne.NewSize(nameSize.Width+deleteSize.Width, fyne.Max(nameSize.Height, deleteSize.Height))
}
