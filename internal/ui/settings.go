package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/eightaugusto/file-folder-renamer/internal/localization"
	"github.com/eightaugusto/file-folder-renamer/internal/patternlib"
	"github.com/eightaugusto/file-folder-renamer/internal/settings"
)

type SettingsView struct {
	feedback       localizedFeedback
	labels         textBindings
	application    *Application
	content        fyne.CanvasObject
	files          *widget.Check
	folders        *widget.Check
	ignoredNames   []string
	ignoreInput    *widget.Entry
	ignoredTable   *TablePanel
	settingsFolder *widget.Entry
	selectedConfig string
	status         *widget.Label
	language       *widget.Select
	languageValues []localization.Preference
}

var resolveAbsolutePath = filepath.Abs

func NewSettingsView(application *Application) *SettingsView {
	view := &SettingsView{
		application: application, ignoredNames: append([]string(nil), application.settings.IgnoredNames...), selectedConfig: application.configPath,
	}
	view.files = view.labels.check(func() string { return application.text("settings.files_default", "Include files by default") }, nil)
	view.files.SetChecked(application.settings.IncludeFiles)
	view.folders = view.labels.check(func() string { return application.text("settings.folders_default", "Include folders by default") }, nil)
	view.folders.SetChecked(application.settings.IncludeFolders)
	options := []string{application.text("language.system", "System default")}
	view.languageValues = []localization.Preference{localization.PreferenceSystem}
	for _, entry := range localization.Languages() {
		options = append(options, entry.Name)
		view.languageValues = append(view.languageValues, entry.Preference)
	}
	view.language = widget.NewSelect(options, nil)
	view.selectLanguage(application.settings.Language)
	view.ignoreInput = widget.NewEntry()
	view.ignoreInput.SetPlaceHolder(application.text("settings.ignored_placeholder", "File name or wildcard, such as *.tmp"))
	view.ignoredTable = NewTablePanel([]TableColumn{{Title: application.text("settings.ignored_name", "Ignored name")}, {Title: application.text("common.action", "Action"), Width: deleteTableActionWidth(view.application)}}, 4)
	view.refreshIgnoredNames()
	addIgnored := view.labels.buttonIcon(func() string { return application.text("common.add", "Add") }, theme.ContentAddIcon(), view.addIgnoredName)
	view.status = widget.NewLabel("")
	view.setStatus("settings.initial_status", "Changes are written to config.json when you select Save settings.")
	view.labels.add(func() { view.feedback.refresh(application, view.status) })
	view.status.Wrapping = fyne.TextWrapWord

	save := view.labels.buttonIcon(func() string { return application.text("settings.save", "Save settings") }, theme.DocumentSaveIcon(), func() {
		if err := view.Save(); err != nil {
			view.feedback.setLiteral(view.status, err.Error(), view.content)
			application.showError(err)
		}
	})
	save.Importance = widget.HighImportance
	view.settingsFolder = widget.NewEntry()
	view.settingsFolder.SetText(filepath.Dir(application.configPath))
	view.settingsFolder.Disable()
	chooseSettingsFolder := view.labels.buttonIcon(func() string { return application.text("common.choose", "Choose…") }, theme.FolderOpenIcon(), view.chooseSettingsFolder)
	settingsControl := container.NewBorder(nil, nil, nil, chooseSettingsFolder, view.settingsFolder)

	locationCard := view.labels.section(func() string { return application.text("settings.location_title", "Settings location") }, func() string {
		return application.text("settings.location_subtitle", "Choose one folder. Settings use config.json and custom patterns use patterns/ inside it.")
	}, newResponsiveForm(
		view.labels.formItem(func() string { return application.text("settings.folder", "Settings folder") }, settingsControl),
	))
	preferencesCard := view.labels.section(func() string { return application.text("settings.defaults_title", "Organizer defaults") }, func() string {
		return application.text("settings.defaults_subtitle", "Choose which entries are included when a folder is selected.")
	}, newFlow(
		view.files,
		view.folders,
	))
	ignoredCard := view.labels.section(func() string { return application.text("settings.ignored_title", "Ignored names") }, func() string {
		return application.text("settings.ignored_subtitle", "Names are skipped during previews. Delete removes an entry from this list.")
	}, container.NewBorder(
		container.NewBorder(nil, nil, nil, addIgnored, view.ignoreInput), nil, nil, nil, view.ignoredTable.Content(),
	))
	languageCard := view.labels.section(func() string { return application.text("settings.language", "Language") }, func() string {
		return application.text("settings.language_help", "Language changes take effect when you save settings.")
	}, view.language)
	body := newDocumentScroll(newVertical(spaceLG, languageCard, widget.NewSeparator(), locationCard, widget.NewSeparator(), preferencesCard, widget.NewSeparator(), ignoredCard))
	footer := newVertical(spaceXS, widget.NewSeparator(), newFlow(save), view.status)
	view.content = newWorkspace(nil, footer, body)

	return view
}

func (view *SettingsView) Content() fyne.CanvasObject { return view.content }

func (view *SettingsView) addIgnoredName() {
	candidate := strings.TrimSpace(view.ignoreInput.Text)
	proposed := append(append([]string(nil), view.ignoredNames...), candidate)
	configuration := settings.Config{
		PatternsPath: view.patternsPath(), IncludeFiles: true, IgnoredNames: proposed,
	}
	if err := configuration.Validate(); err != nil {
		view.feedback.setLiteral(view.status, err.Error(), view.content)
		return
	}
	view.ignoredNames = proposed
	view.ignoreInput.SetText("")
	view.refreshIgnoredNames()
}

func (view *SettingsView) removeIgnoredName(index int) {
	if index < 0 || index >= len(view.ignoredNames) {
		return
	}
	view.ignoredNames = append(view.ignoredNames[:index], view.ignoredNames[index+1:]...)
	view.refreshIgnoredNames()
}

func (view *SettingsView) chooseSettingsFolder() {
	picker := dialog.NewFolderOpen(view.settingsFolderSelected, view.application.window)
	view.application.showDialog(picker)
	// Match Organizer's folder chooser: FileDialog must be shown before it can
	// be resized safely in Fyne, then it uses half of the current app window.
	picker.Resize(folderPickerSize(view.application.window))
	if overlay := view.application.window.Canvas().Overlays().Top(); overlay != nil {
		overlay.Refresh()
	}
}

func (view *SettingsView) settingsFolderSelected(uri fyne.ListableURI, err error) {
	if err != nil {
		view.application.showError(err)
		return
	}
	if uri == nil {
		return
	}
	if err := view.loadSettingsFolder(uri.Path()); err != nil {
		view.feedback.setLiteral(view.status, err.Error(), view.content)
		view.application.showError(err)
	}
}

func (view *SettingsView) loadSettingsFolder(path string) error {
	absolute, err := resolveAbsolutePath(path)
	if err != nil {
		return fmt.Errorf(view.application.text("settings.resolve_folder", "resolve settings folder: %w"), err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return fmt.Errorf(view.application.text("settings.open_folder", "open settings folder: %w"), err)
	}
	if !info.IsDir() {
		return errors.New(view.application.text("settings.folder_required", "settings location must be a folder"))
	}
	configPath := filepath.Join(absolute, "config.json")
	configuration, err := settings.Load(configPath)
	if errors.Is(err, os.ErrNotExist) {
		configuration = settings.DefaultConfig(filepath.Join(absolute, "patterns"))
	} else if err != nil {
		return err
	}

	view.selectedConfig = filepath.Clean(configPath)
	view.settingsFolder.SetText(absolute)
	view.files.SetChecked(configuration.IncludeFiles)
	view.folders.SetChecked(configuration.IncludeFolders)
	view.selectLanguage(configuration.Language)
	view.ignoredNames = append([]string(nil), configuration.IgnoredNames...)
	view.refreshIgnoredNames()
	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		view.setStatus("settings.new_folder", "New settings folder selected. Select Save settings to create config.json.")
	} else {
		view.setStatus("settings.valid_folder", "Settings folder is valid. Select Save settings to use it.")
	}
	return nil
}

func (view *SettingsView) Save() error {
	if view.application.renameInProgress() {
		return errors.New(view.application.text("settings.wait_apply", "wait for the rename operation to finish before changing settings"))
	}
	patternsPath := view.patternsPath()
	configuration := settings.Config{
		PatternsPath: patternsPath, IncludeFiles: view.files.Checked, IncludeFolders: view.folders.Checked,
		Language:     view.selectedLanguage(),
		IgnoredNames: append([]string(nil), view.ignoredNames...),
	}
	if err := configuration.Validate(); err != nil {
		return err
	}
	previous := view.application.settings
	previous.Language = configuration.Language
	languageOnly := view.selectedConfig == view.application.configPath && reflect.DeepEqual(previous, configuration)
	if languageOnly {
		if err := settings.Save(view.selectedConfig, configuration); err != nil {
			return err
		}
		view.application.settings = configuration
		view.application.refreshLanguage()
		view.setStatus("settings.language_saved", "Settings saved. Language updated.")
		return nil
	}
	if view.application.hasDirtyStudioDraft() {
		return errors.New(view.application.text("settings.clean_required", "save or discard the Pattern Studio draft before changing application settings"))
	}
	store, err := patternlib.Open(patternsPath)
	if err != nil {
		return err
	}
	if err := settings.Save(view.selectedConfig, configuration); err != nil {
		return err
	}

	preferredPattern := "default"
	if selected, _, _ := view.application.organizerConfiguration(); selected != "" {
		preferredPattern = selected
	}
	view.application.settings = configuration
	view.application.configPath = view.selectedConfig
	view.application.store = store
	view.application.reloadPatternsAndConfiguration(preferredPattern, configuration.IncludeFiles, configuration.IncludeFolders)
	view.application.refreshLanguage()
	view.setStatus("settings.saved", "Settings saved and the custom pattern library reloaded.")
	return nil
}

func (view *SettingsView) selectedLanguage() localization.Preference {
	index := view.language.SelectedIndex()
	if index < 0 || index >= len(view.languageValues) {
		return localization.PreferenceSystem
	}
	return view.languageValues[index]
}

func (view *SettingsView) selectLanguage(preference localization.Preference) {
	view.language.SetSelectedIndex(0)
	for index, value := range view.languageValues {
		if value == preference {
			view.language.SetSelectedIndex(index)
			return
		}
	}
}

// patternsPath is intentionally derived from the selected settings file. This
// gives the user one portable configuration location instead of two unrelated
// paths while retaining the absolute patterns_path stored in JSON.
func (view *SettingsView) patternsPath() string {
	return filepath.Join(filepath.Dir(view.selectedConfig), "patterns")
}

// refreshIgnoredNames preserves the visible table and updates its rows without
// turning ignored names into a selection organizer.
func (view *SettingsView) refreshIgnoredNames() {
	if view.ignoredTable == nil {
		return
	}
	rows := make([]fyne.CanvasObject, 0, len(view.ignoredNames))
	for index, name := range view.ignoredNames {
		row := newIgnoredNameRow(view.application)
		row.name.Truncation = fyne.TextTruncateEllipsis
		row.set(index, name, view.removeIgnoredName)
		rows = append(rows, row)
	}
	view.ignoredTable.SetRows(rows)
}
