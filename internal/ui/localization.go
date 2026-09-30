package ui

import (
	"fyne.io/fyne/v2/dialog"
	"github.com/eightaugusto/file-folder-renamer/internal/localization"
)

var english = localization.New(localization.PreferenceEnglish, "")

func (application *Application) texts() *localization.Translator {
	if application.translator == nil {
		return english
	}
	return application.translator
}

func (application *Application) text(id, fallback string, data ...any) string {
	return application.texts().Text(id, fallback, data...)
}

func (application *Application) showError(err error) {
	prompt := dialog.NewInformation(application.text("common.error", "Error"), application.text("error.operation", "The operation could not be completed.\n\n{{.Error}}", map[string]any{"Error": err.Error()}), application.window)
	prompt.SetDismissText(application.text("common.close", "Close"))
	application.showDialog(prompt)
}

func (application *Application) showInformation(title, message string) {
	application.showDialog(dialog.NewInformation(title, message, application.window))
}

func (application *Application) appliedMessage(count int) string {
	return application.texts().Plural("organizer.complete_message", "Applied {{.Count}} rename.", "Applied {{.Count}} renames.", count, map[string]any{"Count": count})
}
