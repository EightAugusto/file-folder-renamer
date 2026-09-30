package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// Operational feedback survives relabeling without resetting a scan or draft.
type localizedFeedback struct {
	key, fallback, literal string
	data                   []any
}

func (f *localizedFeedback) set(app *Application, label *widget.Label, key, fallback string, data ...any) {
	f.key, f.fallback, f.literal, f.data = key, fallback, "", data
	f.refresh(app, label)
}
func (f *localizedFeedback) setLiteral(label *widget.Label, text string, content fyne.CanvasObject) {
	f.key = ""
	f.literal = text
	label.SetText(text)
	refreshFeedbackLayout(content)
}
func (f *localizedFeedback) refresh(app *Application, label *widget.Label) {
	if f.key == "" {
		label.SetText(f.literal)
	} else {
		label.SetText(app.text(f.key, f.fallback, f.data...))
	}
}
func (v *OrganizerView) setStatus(key, fallback string, data ...any) {
	v.feedback.set(v.application, v.status, key, fallback, data...)
	refreshFeedbackLayout(v.content)
}
func (v *StudioView) setStatus(key, fallback string, data ...any) {
	v.feedback.set(v.application, v.status, key, fallback, data...)
	refreshFeedbackLayout(v.content)
}
func (v *SettingsView) setStatus(key, fallback string, data ...any) {
	v.feedback.set(v.application, v.status, key, fallback, data...)
	refreshFeedbackLayout(v.content)
}

func refreshFeedbackLayout(content fyne.CanvasObject) {
	if content != nil {
		content.Refresh()
	}
}
