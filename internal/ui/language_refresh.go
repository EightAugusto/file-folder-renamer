package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/widget"
	"github.com/eightaugusto/file-folder-renamer/internal/localization"
)

// Called on Fyne's UI thread after settings have been saved successfully.
// Controls and callbacks stay alive; only presentation is changed.
func (application *Application) refreshLanguage() {
	locale := localization.Resolve(application.settings.Language, lang.SystemLocale().String())
	if locale == application.texts().Preference() {
		return
	}
	application.translator = localization.New(locale, "")
	application.window.SetTitle(application.text("app.title", "File & Folder Renamer"))
	if application.shell != nil {
		application.shell.refreshLanguage()
	}
	application.organizer.refreshLanguage()
	application.studio.refreshLanguage()
	application.settingsUI.refreshLanguage()
}

func (view *OrganizerView) refreshLanguage() {
	view.labels.refresh()
	view.path.SetPlaceHolder(view.application.text("organizer.choose_folder", "Choose a folder"))
	view.filterLabels.refresh()
	if session := view.session; session != nil {
		view.summary.SetText(view.application.text("organizer.summary", "{{.Changed}} changed  •  {{.Unchanged}} unchanged  •  {{.Total}} total", map[string]any{"Changed": session.ChangedCount(), "Unchanged": session.UnchangedCount(), "Total": len(session.ProposalSnapshot())}))
	}
	view.refreshProposalTable()
	view.content.Refresh()
	if view.filterPopup != nil && view.filterPopup.Visible() {
		if anchor := view.headerFilters[view.filterColumn]; anchor != nil {
			view.filterPopup.ShowAtRelativePosition(fyne.NewPos(0, anchor.Size().Height), anchor)
		}
	}
}

func (view *StudioView) refreshLanguage() {
	view.labels.refresh()
	view.ruleLabels.refresh()
	view.editorLabels.refresh()
	relabelSelect(view.ruleKind, ruleKindOptions(view.application))
	relabelSelect(view.testKind, nodeKindOptions(view.application))
	if view.caseMode != nil {
		relabelSelect(view.caseMode, caseModeOptions(view.application))
	}
	view.testInput.SetPlaceHolder(view.application.text("studio.input_name", "Input name"))
	view.testExpected.SetPlaceHolder(view.application.text("studio.calculated", "Calculated automatically"))
	for index := range view.draft.Rules {
		row := view.rulesHost.Objects[index].(*fyne.Container)
		for _, object := range row.Objects {
			if button, ok := object.(*widget.Button); ok {
				button.SetText(view.ruleRowLabel(index))
			}
		}
	}
	if view.excludedTable != nil {
		view.excludedTable.refreshLanguage(view.application, view.application.text("studio.regex", "Regex"))
	}

	view.resizeTestColumns()
	view.testsTable.Refresh()
	view.content.Refresh()
}

func (view *StudioView) testHeaders() []string {
	return []string{view.application.text("common.kind", "Kind"), view.application.text("common.input", "Input"), view.application.text("common.expected", "Expected"), view.application.text("studio.actual", "Actual"), view.application.text("studio.result", "Result")}
}

func (view *StudioView) resizeTestColumns() {
	headers := view.testHeaders()
	view.testsTable.SetColumnWidth(0, maxFloat32(70, widestTableLabel(append(nodeKindOptions(view.application), headers[0]))))
	view.testsTable.SetColumnWidth(4, maxFloat32(70, widestTableLabel([]string{headers[4], testResultLabel("PASS", view.application), testResultLabel("FAIL", view.application), testResultLabel("ERROR", view.application)})))
}

func (view *SettingsView) refreshLanguage() {
	view.labels.refresh()
	options := append([]string(nil), view.language.Options...)
	options[0] = view.application.text("language.system", "System default")
	relabelSelect(view.language, options)
	view.ignoreInput.SetPlaceHolder(view.application.text("settings.ignored_placeholder", "File name or wildcard, such as *.tmp"))
	view.ignoredTable.refreshLanguage(view.application, view.application.text("settings.ignored_name", "Ignored name"))
	view.content.Refresh()
}
