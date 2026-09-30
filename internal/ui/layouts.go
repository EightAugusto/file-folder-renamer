package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	spaceXS float32 = 4
	spaceSM float32 = 8
	spaceMD float32 = 12
	spaceLG float32 = 16
	spaceXL float32 = 24
)

// Width-aware measurement is performed by the parent before laying out its
// children. A wrapped toolbar must reserve its full height on the first frame.
type widthMeasured interface {
	heightAt([]fyne.CanvasObject, float32) float32
}

func heightAt(object fyne.CanvasObject, width float32) float32 {
	if !object.Visible() {
		return 0
	}
	if s, ok := object.(*surface); ok {
		return heightAt(s.content, width-s.left-s.right) + s.top + s.bottom
	}
	if c, ok := object.(*fyne.Container); ok {
		if layout, ok := c.Layout.(widthMeasured); ok {
			return layout.heightAt(c.Objects, width)
		}
	}
	if label, ok := object.(*widget.Label); ok && label.Wrapping == fyne.TextWrapWord {
		return wrappedTextHeight(label.Text, label.TextStyle, width-theme.InnerPadding()*2) + theme.InnerPadding()*2
	}
	return object.MinSize().Height
}
func wrappedTextHeight(text string, style fyne.TextStyle, width float32) float32 {
	size := theme.TextSize()
	lineHeight := fyne.MeasureText("Mg", size, style).Height
	lines := 0
	for _, paragraph := range strings.Split(text, "\n") {
		line := ""
		lines++
		for _, word := range strings.Fields(paragraph) {
			next := word
			if line != "" {
				next = line + " " + word
			}
			if line != "" && fyne.MeasureText(next, size, style).Width <= width {
				line = next
				continue
			}
			if line != "" {
				lines++
				line = ""
			}
			remaining := []rune(word)
			for len(remaining) > 1 && fyne.MeasureText(string(remaining), size, style).Width > width {
				low, high := 1, len(remaining)
				for low < high {
					mid := (low + high + 1) / 2
					if fyne.MeasureText(string(remaining[:mid]), size, style).Width <= width {
						low = mid
					} else {
						high = mid - 1
					}
				}
				remaining = remaining[low:]
				lines++
			}
			line = string(remaining)
		}
	}
	return float32(lines) * lineHeight
}

// verticalLayout is also used for sections containing responsive forms.
type verticalLayout struct{ gap, inset float32 }

func newVertical(gap float32, objects ...fyne.CanvasObject) *fyne.Container {
	return container.New(&verticalLayout{gap: gap}, objects...)
}
func (l *verticalLayout) heightAt(objects []fyne.CanvasObject, width float32) float32 {
	h := l.inset * 2
	count := 0
	for _, o := range objects {
		if o.Visible() {
			h += heightAt(o, width-l.inset*2)
			count++
		}
	}
	if count > 1 {
		h += float32(count-1) * l.gap
	}
	return h
}
func (l *verticalLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	y := l.inset
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		h := heightAt(o, size.Width-l.inset*2)
		o.Move(fyne.NewPos(l.inset, y))
		o.Resize(fyne.NewSize(maxFloat32(0, size.Width-l.inset*2), h))
		y += h + l.gap
	}
}
func (l *verticalLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, l.heightAt(objects, 600))
}

type flowLayout struct{ gap float32 }

func newFlow(objects ...fyne.CanvasObject) *fyne.Container {
	return container.New(&flowLayout{gap: spaceSM}, objects...)
}
func (l *flowLayout) arrange(objects []fyne.CanvasObject, width float32, place bool) float32 {
	x, y, row := float32(0), float32(0), float32(0)
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		w := o.MinSize().Width
		if w > width {
			w = width
		}
		h := heightAt(o, w)
		if x > 0 && x+w > width {
			x = 0
			y += row + l.gap
			row = 0
		}
		if place {
			o.Move(fyne.NewPos(x, y))
			o.Resize(fyne.NewSize(w, h))
		}
		x += w + l.gap
		row = maxFloat32(row, h)
	}
	return y + row
}
func (l *flowLayout) heightAt(objects []fyne.CanvasObject, width float32) float32 {
	return l.arrange(objects, width, false)
}
func (l *flowLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	l.arrange(objects, size.Width, true)
}
func (l *flowLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	w, h := float32(0), float32(0)
	for _, o := range objects {
		if o.Visible() {
			w += o.MinSize().Width + l.gap
			h = maxFloat32(h, o.MinSize().Height)
		}
	}
	return fyne.NewSize(maxFloat32(0, w-l.gap), h)
}

// Adaptive border reserves measured heights instead of VBox's single-row minima.
type workspaceLayout struct {
	top, bottom, body fyne.CanvasObject
	inset             float32
}

func newWorkspace(top, bottom, body fyne.CanvasObject) *fyne.Container {
	l := &workspaceLayout{body: body, inset: spaceLG}
	if top != nil {
		l.top = newSurface(top)
	}
	if bottom != nil {
		l.bottom = newSurface(bottom)
	}
	objects := []fyne.CanvasObject{body}
	if top != nil {
		objects = append(objects, l.top)
	}
	if bottom != nil {
		objects = append(objects, l.bottom)
	}
	return container.New(l, objects...)
}
func (l *workspaceLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	w := maxFloat32(0, size.Width-l.inset*2)
	y := l.inset
	bottom := size.Height - l.inset
	if l.top != nil && l.top.Visible() {
		panel := l.top.(*surface)
		panel.left, panel.right, panel.top, panel.bottom = l.inset, l.inset, l.inset, spaceMD
		h := heightAt(panel, size.Width)
		panel.Move(fyne.Position{})
		panel.Resize(fyne.NewSize(size.Width, h))
		y = h
	}
	if l.bottom != nil && l.bottom.Visible() {
		panel := l.bottom.(*surface)
		panel.left, panel.right, panel.top, panel.bottom = l.inset, l.inset, spaceMD, l.inset
		h := heightAt(panel, size.Width)
		bottom = size.Height - h
		panel.Move(fyne.NewPos(0, bottom))
		panel.Resize(fyne.NewSize(size.Width, h))
	}

	l.body.Move(fyne.NewPos(l.inset, y))
	l.body.Resize(fyne.NewSize(w, maxFloat32(0, bottom-y)))
}
func (l *workspaceLayout) MinSize(_ []fyne.CanvasObject) fyne.Size { return fyne.NewSize(320, 240) }

// Form controls and FormItems stay alive when labels move above fields.
type responsiveFormLayout struct {
	items  []*widget.FormItem
	labels []*widget.Label
}

func newResponsiveForm(items ...*widget.FormItem) *fyne.Container {
	l := &responsiveFormLayout{items: items}
	objects := []fyne.CanvasObject{}
	for _, item := range items {
		label := widget.NewLabel(item.Text)
		l.labels = append(l.labels, label)
		objects = append(objects, label, item.Widget)
	}
	return container.New(l, objects...)
}
func (l *responsiveFormLayout) measure(width float32, place bool) float32 {
	labelWidth := float32(0)
	for i, item := range l.items {
		if l.labels[i].Text != item.Text {
			l.labels[i].SetText(item.Text)
		}
		labelWidth = maxFloat32(labelWidth, l.labels[i].MinSize().Width)
	}
	stacked := width < labelWidth+220+spaceMD
	y := float32(0)
	for i, item := range l.items {
		label := l.labels[i]
		field := item.Widget
		if !field.Visible() {
			continue
		}
		lh := label.MinSize().Height
		fw := maxFloat32(0, width-labelWidth-spaceMD)
		if stacked {
			fw = width
		}
		fh := heightAt(field, fw)
		if stacked {
			if item.Text != "" {
				if place {
					label.Move(fyne.NewPos(0, y))
					label.Resize(fyne.NewSize(width, lh))
				}
				y += lh
			}
			if place {
				field.Move(fyne.NewPos(0, y))
				field.Resize(fyne.NewSize(width, fh))
			}
			y += fh + spaceSM
		} else {
			h := maxFloat32(lh, fh)
			if place {
				label.Move(fyne.NewPos(0, y))
				label.Resize(fyne.NewSize(labelWidth, h))
				field.Move(fyne.NewPos(labelWidth+spaceMD, y))
				field.Resize(fyne.NewSize(fw, h))
			}
			y += h + spaceSM
		}
	}
	return maxFloat32(0, y-spaceSM)
}
func (l *responsiveFormLayout) heightAt(_ []fyne.CanvasObject, width float32) float32 {
	return l.measure(width, false)
}
func (l *responsiveFormLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	l.measure(size.Width, true)
}
func (l *responsiveFormLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(220, l.measure(600, false))
}

// A scroll's content needs a width-dependent height without rebuilding it.
type documentLayout struct{}

func (*documentLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Move(fyne.Position{})
		o.Resize(fyne.NewSize(size.Width, heightAt(o, size.Width)))
	}
}
func (*documentLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	h := float32(0)
	for _, o := range objects {
		w := o.Size().Width
		if w <= 0 {
			w = 600
		}
		h = maxFloat32(h, heightAt(o, w))
	}
	return fyne.NewSize(0, h)
}
func newDocumentScroll(content fyne.CanvasObject) *container.Scroll {
	return container.NewVScroll(container.New(&documentLayout{}, content))
}

// Compact section headings share a line with their explanation when it fits.
// In a narrow rule editor the explanation wraps onto its own lines.
type sectionHeadingLayout struct{}

func (l *sectionHeadingLayout) arrange(objects []fyne.CanvasObject, width float32, place bool) float32 {
	title, description := objects[0], objects[1]
	th := title.MinSize().Height
	if !description.Visible() {
		if place {
			title.Move(fyne.Position{})
			title.Resize(fyne.NewSize(width, th))
		}
		return th
	}
	label := description.(*widget.Label)
	dw := fyne.MeasureText(label.Text, theme.TextSize(), label.TextStyle).Width + theme.InnerPadding()*2
	tw := title.MinSize().Width
	if tw+spaceMD+dw <= width {
		dh := heightAt(description, dw)
		h := maxFloat32(th, dh)
		if place {
			title.Move(fyne.Position{})
			title.Resize(fyne.NewSize(tw, h))
			description.Move(fyne.NewPos(tw+spaceMD, 0))
			description.Resize(fyne.NewSize(width-tw-spaceMD, h))
		}
		return h
	}
	dh := heightAt(description, width)
	if place {
		title.Move(fyne.Position{})
		title.Resize(fyne.NewSize(width, th))
		description.Move(fyne.NewPos(0, th+spaceXS))
		description.Resize(fyne.NewSize(width, dh))
	}
	return th + spaceXS + dh
}
func (l *sectionHeadingLayout) heightAt(objects []fyne.CanvasObject, width float32) float32 {
	return l.arrange(objects, width, false)
}
func (l *sectionHeadingLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	l.arrange(objects, size.Width, true)
}
func (l *sectionHeadingLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, l.heightAt(objects, 600))
}

// Feedback wraps beside the transient Cancel action and reserves its full height.
type feedbackLayout struct{}

func (*feedbackLayout) heightAt(objects []fyne.CanvasObject, width float32) float32 {
	action := fyne.Size{}
	if objects[1].Visible() {
		action = objects[1].MinSize()
		width -= action.Width + spaceSM
	}
	return maxFloat32(heightAt(objects[0], width), action.Height)
}
func (l *feedbackLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	width := size.Width
	if objects[1].Visible() {
		action := objects[1].MinSize()
		width -= action.Width + spaceSM
		objects[1].Move(fyne.NewPos(width+spaceSM, 0))
		objects[1].Resize(action)
	}
	objects[0].Move(fyne.Position{})
	objects[0].Resize(fyne.NewSize(maxFloat32(0, width), size.Height))
}
func (l *feedbackLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, l.heightAt(objects, 600))
}
