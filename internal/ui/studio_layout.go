package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

func (v *StudioView) pipelineWidth() float32 {
	w := v.rulesHost.MinSize().Width
	w = maxFloat32(w, v.ruleKind.MinSize().Width+v.addRuleButton.MinSize().Width+theme.Padding())
	return w * 1.25
}

type pipelineWorkspaceLayout struct{ view *StudioView }

func (l *pipelineWorkspaceLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	w := l.view.pipelineWidth()
	objects[0].Move(fyne.Position{})
	objects[0].Resize(fyne.NewSize(w, size.Height))
	objects[1].Move(fyne.NewPos(w+spaceLG, 0))
	objects[1].Resize(fyne.NewSize(maxFloat32(0, size.Width-w-spaceLG), size.Height))
}
func (*pipelineWorkspaceLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(320, 160) }

// Test input, kind, and calculated output share a row; the action moves below
// when necessary. Existing entries remain mounted through every resize.
type testEntryLayout struct{}

func (l *testEntryLayout) heightAt(objects []fyne.CanvasObject, width float32) float32 {
	_, h := l.measure(objects, width, false)
	return h
}
func (l *testEntryLayout) measure(objects []fyne.CanvasObject, width float32, place bool) (float32, float32) {
	action := objects[3].MinSize()
	kind := float32(150)
	inline := width >= 760
	available := width - kind - spaceSM*2
	if inline {
		available -= action.Width + spaceSM
	}
	field := maxFloat32(80, available/2)
	widths := []float32{field, kind, field}
	x, h := float32(0), float32(0)
	for i, w := range widths {
		ch := heightAt(objects[i], w)
		h = maxFloat32(h, ch)
		if place {
			objects[i].Move(fyne.NewPos(x, 0))
			objects[i].Resize(fyne.NewSize(w, ch))
		}
		x += w + spaceSM
	}
	if inline {
		if place {
			objects[3].Move(fyne.NewPos(x, h-action.Height))
			objects[3].Resize(action)
		}
	} else {
		if place {
			objects[3].Move(fyne.NewPos(0, h+spaceXS))
			objects[3].Resize(action)
		}
		h += action.Height + spaceXS
	}
	return width, h
}
func (l *testEntryLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	l.measure(objects, size.Width, true)
}
func (l *testEntryLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(320, l.heightAt(objects, 800))
}
