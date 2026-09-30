package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// Bind only application-owned text. Entry values and domain data never pass
// through these callbacks. Each replaceable editor owns its own binding group.
type textBindings []func()

func (bindings *textBindings) add(update func()) { *bindings = append(*bindings, update) }
func (bindings *textBindings) refresh() {
	for _, update := range *bindings {
		update()
	}
}
func (bindings *textBindings) label(text func() string) *widget.Label {
	w := widget.NewLabel(text())
	bindings.add(func() { w.SetText(text()) })
	return w
}
func (bindings *textBindings) button(text func() string, tapped func()) *widget.Button {
	w := widget.NewButton(text(), tapped)
	bindings.add(func() { w.SetText(text()) })
	return w
}
func (bindings *textBindings) buttonIcon(text func() string, icon fyne.Resource, tapped func()) *widget.Button {
	w := widget.NewButtonWithIcon(text(), icon, tapped)
	bindings.add(func() { w.SetText(text()) })
	return w
}
func (bindings *textBindings) check(text func() string, changed func(bool)) *widget.Check {
	w := widget.NewCheck(text(), changed)
	bindings.add(func() { w.Text = text(); w.Refresh() })
	return w
}
func (bindings *textBindings) section(title, subtitle func() string, content fyne.CanvasObject) *fyne.Container {
	heading := bindings.label(title)
	heading.TextStyle = fyne.TextStyle{Bold: true}
	description := bindings.label(subtitle)
	description.Wrapping = fyne.TextWrapWord
	bindings.add(func() {
		if subtitle() == "" {
			description.Hide()
		} else {
			description.Show()
		}
	})
	if subtitle() == "" {
		description.Hide()
	}
	return newVertical(spaceSM, container.New(&sectionHeadingLayout{}, heading, description), content)
}
func (bindings *textBindings) formItem(text func() string, content fyne.CanvasObject) *widget.FormItem {
	w := widget.NewFormItem(text(), content)
	bindings.add(func() { w.Text = text() })
	return w
}

func relabelSelect(control *widget.Select, options []string) {
	index := control.SelectedIndex()
	callback := control.OnChanged
	control.OnChanged = nil
	control.Options = options
	if index >= 0 && index < len(options) {
		control.SetSelectedIndex(index)
	}
	control.Refresh()
	control.OnChanged = callback
}

func (bindings *textBindings) panel(title, subtitle func() string, body fyne.CanvasObject) *fyne.Container {
	heading := bindings.label(title)
	heading.TextStyle = fyne.TextStyle{Bold: true}
	description := bindings.label(subtitle)
	description.Wrapping = fyne.TextWrapWord
	bindings.add(func() {
		if subtitle() == "" {
			description.Hide()
		} else {
			description.Show()
		}
	})
	if subtitle() == "" {
		description.Hide()
	}
	top := container.New(&sectionHeadingLayout{}, heading, description)
	panel := newWorkspace(top, nil, body)
	panel.Layout.(*workspaceLayout).inset = 0
	return panel
}
