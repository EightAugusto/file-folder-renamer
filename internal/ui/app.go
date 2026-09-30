package ui

import (
	"context"
	"sync"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/eightaugusto/file-folder-renamer/internal/localization"
	"github.com/eightaugusto/file-folder-renamer/internal/organizer"
	"github.com/eightaugusto/file-folder-renamer/internal/patternlib"
	"github.com/eightaugusto/file-folder-renamer/internal/settings"
)

const applicationID = "com.eightaugusto.filefolderrenamer"

const (
	// A large requested starting size leaves enough room for every desktop
	// workspace. Fyne and the host window manager clamp it to the usable display
	// area while retaining a normal, non-maximized window.
	defaultWindowWidth  = 1920
	defaultWindowHeight = 1080
)

type Application struct {
	translator *localization.Translator
	shell      *applicationShell
	app        fyne.App
	window     fyne.Window
	service    organizerService
	store      *patternlib.Store
	settings   settings.Config
	configPath string
	organizer  *OrganizerView
	studio     *StudioView
	settingsUI *SettingsView
	allowClose bool
	closing    bool
	modalMu    sync.Mutex
	modals     []dialog.Dialog
}

// organizerService is owned by the UI consumer and contains only the use cases
// the Organizer view invokes.
type organizerService interface {
	Preview(context.Context, organizer.PreviewRequest) (*organizer.PreviewSession, error)
	Apply(context.Context, *organizer.PreviewSession) (organizer.ApplyResult, error)
	ApplyOne(context.Context, *organizer.PreviewSession, string) (organizer.ApplyResult, error)
}

var (
	newDesktopApp      = func() fyne.App { return fyneapp.NewWithID(applicationID) }
	bootstrapApp       = settings.Bootstrap
	openPatternLibrary = patternlib.Open
	runDesktopApp      = func(application fyne.App) { application.Run() }
)

func Run() {
	desktopApp := newDesktopApp()
	desktopApp.SetIcon(appIcon)
	desktopApp.Settings().SetTheme(newOrganizerTheme())
	window := desktopApp.NewWindow("File & Folder Renamer")
	configureWindow(window)
	application := &Application{app: desktopApp, window: window}

	layout, resourceErr := bootstrapApp()
	if resourceErr != nil {
		window.SetContent(widget.NewLabel(resourceErr.Error()))
		window.Show()
		application.installModalShortcut()
		application.showDialog(dialog.NewError(resourceErr, window))
		runDesktopApp(desktopApp)
		return
	}
	settings := layout.Config
	store, libraryErr := openPatternLibrary(layout.PatternsRoot)
	if store == nil {
		message := "pattern library is unavailable"
		if libraryErr != nil {
			message = libraryErr.Error()
		}
		window.SetContent(widget.NewLabel(message))
		window.Show()
		runDesktopApp(desktopApp)
		return
	}
	configPath := layout.ConfigPath

	application.service = organizer.New()
	application.store = store
	application.settings = settings
	application.configPath = configPath
	application.translator = localization.New(settings.Language, lang.SystemLocale().String())
	window.SetTitle(application.text("app.title", "File & Folder Renamer"))
	application.organizer = NewOrganizerView(application)
	application.studio = NewStudioView(application)
	application.settingsUI = NewSettingsView(application)
	application.shell = newApplicationShell(application)
	window.SetContent(application.shell.content)
	window.SetCloseIntercept(application.requestClose)
	application.installShortcuts()

	window.Show()
	if libraryErr != nil {
		application.showError(libraryErr)
	}
	runDesktopApp(desktopApp)
}

func configureWindow(window fyne.Window) {
	window.SetFullScreen(false)
	window.Resize(fyne.NewSize(defaultWindowWidth, defaultWindowHeight))
	window.SetFixedSize(false)
	window.SetIcon(appIcon)
	window.CenterOnScreen()
}

func (application *Application) RefreshPatterns(prefer string) {
	application.organizer.RefreshPatterns(prefer)
	application.studio.RefreshPatterns(prefer)
}

func (application *Application) organizerConfiguration() (string, bool, bool) {
	if application.organizer == nil {
		return "", false, false
	}
	return application.organizer.Configuration()
}

func (application *Application) setOrganizerConfiguration(patternName string, files, folders bool) {
	if application.organizer != nil {
		application.organizer.SetConfiguration(patternName, files, folders)
	}
}

func (application *Application) reloadPatternsAndConfiguration(patternName string, files, folders bool) {
	application.setOrganizerConfiguration(patternName, files, folders)
	if application.studio != nil {
		application.studio.RefreshPatterns(patternName)
	}
}

func (application *Application) hasDirtyStudioDraft() bool {
	return application.studio != nil && application.studio.IsDirty()
}

func (application *Application) renameInProgress() bool {
	return application.organizer != nil && application.organizer.IsApplying()
}

func (application *Application) installShortcuts() {
	application.installModalShortcut()
	application.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyO, Modifier: fyne.KeyModifierControl}, application.openShortcut)
	application.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyO, Modifier: fyne.KeyModifierSuper}, application.openShortcut)
	application.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierControl}, application.saveShortcut)
	application.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyS, Modifier: fyne.KeyModifierSuper}, application.saveShortcut)
}

func (application *Application) installModalShortcut() {
	application.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyEscape}, application.dismissTopModal)
}

func (application *Application) showDialog(prompt dialog.Dialog) {
	application.modalMu.Lock()
	application.modals = append(application.modals, prompt)
	application.modalMu.Unlock()
	prompt.SetOnClosed(func() { application.removeDialog(prompt) })
	prompt.Show()
}

func (application *Application) removeDialog(prompt dialog.Dialog) {
	application.modalMu.Lock()
	defer application.modalMu.Unlock()
	for index := len(application.modals) - 1; index >= 0; index-- {
		if application.modals[index] == prompt {
			application.modals = append(application.modals[:index], application.modals[index+1:]...)
			return
		}
	}
}

func (application *Application) dismissTopModal(fyne.Shortcut) {
	application.modalMu.Lock()
	count := len(application.modals)
	if count > 0 {
		prompt := application.modals[count-1]
		application.modals = application.modals[:count-1]
		application.modalMu.Unlock()
		prompt.Dismiss()
		return
	}
	application.modalMu.Unlock()
	if overlay := application.window.Canvas().Overlays().Top(); overlay != nil {
		overlay.Hide()
	}
}

func (application *Application) openShortcut(fyne.Shortcut) {
	application.organizer.OpenPath()
}

func (application *Application) saveShortcut(fyne.Shortcut) {
	if application.studio.IsDirty() {
		if err := application.studio.SaveCurrent(); err != nil {
			application.showError(err)
		}
	}
}

func (application *Application) requestClose() {
	if application.allowClose {
		application.closeNow()
		return
	}
	if application.renameInProgress() {
		application.showInformation(application.text("app.rename_title", "Rename in progress"), application.text("app.rename_message", "Wait for the rename operation and any rollback to finish before closing."))
		return
	}
	if !application.studio.IsDirty() {
		application.closeNow()
		return
	}

	message := widget.NewLabel(application.text("app.unsaved_message", "The current pattern has unsaved changes."))
	var prompt dialog.Dialog
	save := widget.NewButtonWithIcon(application.text("common.save", "Save"), theme.DocumentSaveIcon(), func() {
		if err := application.studio.SaveCurrent(); err != nil {
			application.showError(err)
			return
		}
		prompt.Hide()
		application.closeNow()
	})
	discard := widget.NewButton(application.text("common.discard", "Discard"), func() {
		prompt.Hide()
		application.closeNow()
	})
	cancel := widget.NewButton(application.text("common.cancel", "Cancel"), func() { prompt.Hide() })
	prompt = dialog.NewCustomWithoutButtons(application.text("app.unsaved_title", "Unsaved pattern"), container.NewVBox(message, container.NewHBox(save, discard, cancel)), application.window)
	application.showDialog(prompt)
}

func (application *Application) closeNow() {
	if application.closing {
		return
	}
	if application.renameInProgress() {
		return
	}
	application.closing = true
	application.allowClose = true
	application.organizer.CancelPreview()
	if done := application.organizer.previewDone; done != nil {
		go func() {
			<-done
			fyne.Do(application.finishClose)
		}()
		return
	}
	application.finishClose()
}

func (application *Application) finishClose() {
	application.window.SetCloseIntercept(nil)
	application.window.Close()
}
