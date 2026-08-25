package main

import (
	"image"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// framedPane draws a border around its content. The graph and the stage are
// the two places you actually work, so they get an outline that separates
// them from the panes of controls around them.
type framedPane struct {
	guigui.DefaultWidget

	content guigui.Widget
}

func (f *framedPane) SetContent(w guigui.Widget) { f.content = w }

func frameWidth(context *guigui.Context) int {
	return max(1, basicwidget.UnitSize(context)/16)
}

func (f *framedPane) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	if f.content != nil {
		adder.AddWidget(f.content)
	}
	return nil
}

func (f *framedPane) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	if f.content == nil {
		return
	}
	// Inset by the border so the content never paints over it: children
	// draw after this widget does.
	w := frameWidth(context)
	layouter.LayoutWidget(f.content, widgetBounds.Bounds().Inset(w))
}

func (f *framedPane) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	pal := paletteFor(context)
	w := float32(frameWidth(context))
	b := widgetBounds.Bounds()
	// A stroke straddles its path, so inset by half the width to keep the
	// whole border inside the widget.
	vector.StrokeRect(dst,
		float32(b.Min.X)+w/2, float32(b.Min.Y)+w/2,
		float32(b.Dx())-w, float32(b.Dy())-w,
		w, pal.frame, false)
}

func (f *framedPane) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	if f.content == nil {
		return image.Point{}
	}
	s := f.content.Measure(context, constraints)
	w := 2 * frameWidth(context)
	return image.Pt(s.X+w, s.Y+w)
}
