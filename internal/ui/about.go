package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/eightaugusto/file-folder-renamer/internal/version"
)

func (application *Application) showAbout() {
	about := dialog.NewCustom(
		application.text("about.title", "About {{.Name}}", map[string]any{"Name": application.text("app.title", "File & Folder Renamer")}),
		application.text("common.close", "Close"),
		aboutContent(application),
		application.window,
	)
	about.Resize(fyne.NewSize(440, 220))
	application.showDialog(about)
}

func aboutContent(application *Application) fyne.CanvasObject {
	name := widget.NewLabelWithStyle(application.text("app.title", "File & Folder Renamer"), fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	currentVersion := widget.NewLabelWithStyle(
		application.text("about.version", "Version {{.Version}}", map[string]any{"Version": version.Display()}),
		fyne.TextAlignCenter,
		fyne.TextStyle{},
	)
	license := widget.NewLabel(application.text("about.license", "Licensed under the Apache License, Version 2.0."))
	license.Alignment = fyne.TextAlignCenter
	license.Wrapping = fyne.TextWrapWord

	return container.NewCenter(newVertical(spaceMD, name, currentVersion, widget.NewSeparator(), license))
}
