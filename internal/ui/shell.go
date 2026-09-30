package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const navigationBreakpoint float32 = 1180

type applicationShell struct {
	application *Application
	content     *fyne.Container
	navigation  *fyne.Container
	navSurface  *surface
	buttons     []*widget.Button
	about       *widget.Button
	views       []fyne.CanvasObject
	selected    int
	compact     bool
	background  *canvas.Rectangle
	divider     *widget.Separator
}

func newApplicationShell(application *Application) *applicationShell {
	s := &applicationShell{application: application, views: []fyne.CanvasObject{application.organizer.Content(), application.studio.Content(), application.settingsUI.Content()}}
	icons := []fyne.Resource{theme.FolderOpenIcon(), theme.DocumentCreateIcon(), theme.SettingsIcon()}
	objects := []fyne.CanvasObject{}
	for i, icon := range icons {
		index := i
		b := widget.NewButtonWithIcon("", icon, func() { s.selectIndex(index); application.window.Canvas().Focus(s.buttons[index]) })
		b.Alignment = widget.ButtonAlignLeading
		s.buttons = append(s.buttons, b)
		objects = append(objects, b)
	}
	s.about = widget.NewButtonWithIcon("", theme.InfoIcon(), application.showAbout)
	s.about.Alignment = widget.ButtonAlignLeading
	s.about.Importance = widget.LowImportance
	objects = append(objects, s.about)
	s.background = canvas.NewRectangle(theme.Color("sidebarBackground"))
	s.divider = widget.NewSeparator()
	s.navigation = container.NewWithoutLayout(append([]fyne.CanvasObject{s.background, s.divider}, objects...)...)
	s.navSurface = newSurface(s.navigation)
	s.content = container.New(s, append(append([]fyne.CanvasObject{}, s.views...), s.navSurface)...)
	s.refreshLanguage()
	s.selectIndex(0)
	return s
}
func (s *applicationShell) selectIndex(index int) {
	if index < 0 || index >= len(s.views) {
		return
	}
	s.selected = index
	for i, view := range s.views {
		if i == index {
			view.Show()
			s.buttons[i].Importance = widget.MediumImportance
		} else {
			view.Hide()
			s.buttons[i].Importance = widget.LowImportance
		}
		s.buttons[i].Refresh()
	}
	s.refreshLanguage()
}
func (s *applicationShell) refreshLanguage() {
	for i, key := range []string{"tab.organizer", "tab.studio", "tab.settings"} {
		label := s.application.text(key, []string{"Organizer", "Pattern Studio", "Settings"}[i])
		if i == s.selected {
			label = "• " + label
		}
		s.buttons[i].SetText(label)
	}
	s.about.SetText(s.application.text("about.button", "About"))
	if s.content != nil {
		s.content.Refresh()
	}
}
func (s *applicationShell) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	s.compact = size.Width < navigationBreakpoint
	s.background.FillColor = theme.Color("sidebarBackground")
	s.background.Refresh()
	navWidth := float32(176)
	for _, b := range s.buttons {
		navWidth = maxFloat32(navWidth, b.MinSize().Width+spaceLG*2)
	}
	navWidth = maxFloat32(navWidth, s.about.MinSize().Width+spaceLG*2)
	if s.compact {
		x, h := spaceLG, float32(0)
		controls := append(append([]*widget.Button{}, s.buttons...), s.about)
		for _, b := range controls {
			h = maxFloat32(h, b.MinSize().Height)
		}
		s.navigation.Move(fyne.Position{})
		s.navigation.Resize(fyne.NewSize(size.Width, h+spaceSM*2))
		for _, b := range s.buttons {
			w := b.MinSize().Width
			b.Move(fyne.NewPos(x, spaceSM))
			b.Resize(fyne.NewSize(w, h))
			x += w + spaceSM
		}
		aboutWidth := s.about.MinSize().Width
		s.about.Move(fyne.NewPos(size.Width-spaceLG-aboutWidth, spaceSM))
		s.about.Resize(fyne.NewSize(aboutWidth, h))
		for _, v := range s.views {
			v.Move(fyne.NewPos(0, h+spaceSM*2))
			v.Resize(fyne.NewSize(size.Width, maxFloat32(0, size.Height-h-spaceSM*2)))
		}
	} else {
		s.navigation.Move(fyne.Position{})
		s.navigation.Resize(fyne.NewSize(navWidth, size.Height))
		y := spaceLG
		for _, b := range s.buttons {
			h := maxFloat32(32, b.MinSize().Height)
			b.Move(fyne.NewPos(spaceSM, y))
			b.Resize(fyne.NewSize(navWidth-spaceLG, h))
			y += h + spaceXS
		}
		aboutHeight := maxFloat32(32, s.about.MinSize().Height)
		s.about.Move(fyne.NewPos(spaceSM, size.Height-spaceLG-aboutHeight))
		s.about.Resize(fyne.NewSize(navWidth-spaceLG, aboutHeight))
		for _, v := range s.views {
			v.Move(fyne.NewPos(navWidth, 0))
			v.Resize(fyne.NewSize(maxFloat32(0, size.Width-navWidth), size.Height))
		}
	}
	s.navSurface.Move(fyne.Position{})
	s.navSurface.Resize(s.navigation.Size())
	s.background.Move(fyne.Position{})
	s.background.Resize(s.navigation.Size())
	if s.compact {
		s.divider.Move(fyne.NewPos(0, s.navigation.Size().Height-1))
		s.divider.Resize(fyne.NewSize(size.Width, 1))
	} else {
		s.divider.Move(fyne.NewPos(navWidth-1, 0))
		s.divider.Resize(fyne.NewSize(1, size.Height))
	}
}
func (*applicationShell) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(640, 480) }
