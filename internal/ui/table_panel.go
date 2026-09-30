package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// TableColumn defines a visual column for TablePanel. TablePanel deliberately
// uses ordinary canvas rows instead of widget.Table virtualization so a row can
// have independent hover controls, read-only state, and inline actions.
type TableColumn struct {
	Title string
	// Width reserves a fixed width for compact action columns. A zero width
	// means that the column should use the remaining row space.
	Width float32
}

// TablePanel provides the visual structure of a table (header, separators,
// scrollable rows) while leaving row behavior to the supplied canvas objects.
// It is suited to small editable collections such as ignored names and regex
// exclusions; large sortable proposal sets continue to use widget.Table.
type TablePanel struct {
	content    fyne.CanvasObject
	rows       *fyne.Container
	columns    []TableColumn
	headerHost *fyne.Container
}

func NewTablePanel(columns []TableColumn, visibleRows int) *TablePanel {
	if visibleRows < 1 {
		visibleRows = 1
	}
	panel := &TablePanel{rows: container.NewVBox(), columns: append([]TableColumn(nil), columns...)}
	headers := make([]fyne.CanvasObject, 0, len(columns))
	for _, column := range columns {
		alignment := fyne.TextAlignLeading
		if column.Width > 0 {
			alignment = fyne.TextAlignCenter
		}
		headers = append(headers, widget.NewLabelWithStyle(column.Title, alignment, fyne.TextStyle{Bold: true}))
	}
	headerRow := container.NewStack(panel.NewRow(headers...))
	panel.headerHost = headerRow
	header := container.NewBorder(nil, widget.NewSeparator(), nil, nil, headerRow)

	rowHeight := widget.NewButton("", nil).MinSize().Height
	minimum := canvas.NewRectangle(color.Transparent)
	minimum.SetMinSize(fyne.NewSize(0, rowHeight*float32(visibleRows)+theme.Padding()*float32(visibleRows-1)))
	viewport := container.NewStack(minimum, panel.rows)
	panel.content = container.NewBorder(header, nil, nil, nil, viewport)
	return panel
}

func (panel *TablePanel) Content() fyne.CanvasObject { return panel.content }

func (panel *TablePanel) refreshLanguage(application *Application, title string) {
	panel.columns[0].Title = title
	panel.columns[1].Title = application.text("common.action", "Action")
	panel.columns[1].Width = deleteTableActionWidth(application)
	panel.headerHost.Objects = []fyne.CanvasObject{panel.NewRow(
		widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle(panel.columns[1].Title, fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
	)}
	panel.headerHost.Refresh()
	for _, object := range panel.rows.Objects {
		if row, ok := object.(*ignoredNameRow); ok {
			row.remove.SetText(application.text("common.delete", "Delete"))
			row.Refresh()
		}
	}
	panel.content.Refresh()
}

// NewRow lays out cells using the same columns as the table header. Two-column
// tables reserve a compact right-hand action column and give all remaining
// space to the value column.
func (panel *TablePanel) NewRow(cells ...fyne.CanvasObject) fyne.CanvasObject {
	if len(cells) == 0 {
		return container.NewWithoutLayout()
	}
	decorated := make([]fyne.CanvasObject, len(cells))
	for index, cell := range cells {
		if index < len(panel.columns) && panel.columns[index].Width > 0 {
			minimum := canvas.NewRectangle(color.Transparent)
			minimum.SetMinSize(fyne.NewSize(panel.columns[index].Width, 0))
			cell = container.NewStack(minimum, cell)
		}
		decorated[index] = cell
	}
	if len(decorated) == 1 {
		return decorated[0]
	}
	if len(decorated) == 2 {
		action := container.NewBorder(nil, nil, widget.NewSeparator(), nil, decorated[1])
		return container.NewBorder(nil, nil, nil, action, decorated[0])
	}
	return container.NewGridWithColumns(len(decorated), decorated...)
}

func (panel *TablePanel) SetRows(rows []fyne.CanvasObject) {
	objects := make([]fyne.CanvasObject, 0, len(rows)*2)
	for index, row := range rows {
		objects = append(objects, row)
		if index < len(rows)-1 {
			objects = append(objects, widget.NewSeparator())
		}
	}
	panel.rows.Objects = objects
	panel.rows.Refresh()
}
