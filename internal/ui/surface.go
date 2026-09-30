package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Fyne 2.8 replaces, rather than intersects, nested scroll clip bounds. Entry
// text inside a document can consequently paint/hit-test beyond its viewport.
// Opaque neighbouring panels cover that overflow and absorb background taps;
// actual controls remain ordinary Fyne widgets above the background.
type surface struct {
	widget.BaseWidget
	content                  fyne.CanvasObject
	left, top, right, bottom float32
}

func newSurface(content fyne.CanvasObject) *surface {
	s := &surface{content: content}
	s.ExtendBaseWidget(s)
	return s
}
func (s *surface) Tapped(*fyne.PointEvent) {}
func (s *surface) CreateRenderer() fyne.WidgetRenderer {
	return &surfaceRenderer{surface: s, background: canvas.NewRectangle(theme.Color(theme.ColorNameBackground))}
}

type surfaceRenderer struct {
	surface    *surface
	background *canvas.Rectangle
}

func (r *surfaceRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.background, r.surface.content}
}
func (r *surfaceRenderer) Destroy() {}
func (r *surfaceRenderer) MinSize() fyne.Size {
	s := r.surface
	return s.content.MinSize().AddWidthHeight(s.left+s.right, s.top+s.bottom)
}
func (r *surfaceRenderer) Layout(size fyne.Size) {
	s := r.surface
	r.background.Resize(size)
	s.content.Move(fyne.NewPos(s.left, s.top))
	s.content.Resize(fyne.NewSize(maxFloat32(0, size.Width-s.left-s.right), maxFloat32(0, size.Height-s.top-s.bottom)))
}
func (r *surfaceRenderer) Refresh() {
	r.background.FillColor = theme.Color(theme.ColorNameBackground)
	r.background.Refresh()
	r.Layout(r.surface.Size())
	r.surface.content.Refresh()
}
