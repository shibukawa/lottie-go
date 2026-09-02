package main

import (
	"image"
	"image/color"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Shared widget helpers and the color palette, following the state machine
// editor's conventions.

type palette struct {
	frame, designBox, selected, focusable, hovered color.Color
	canvasBack                                     color.Color
	// camera outlines the scene camera's framing on the edit canvas.
	camera color.Color
	// timeline parts: the row track, a clip's bar, the open-ended bar of
	// a machine or a looping clip, and the playhead.
	track, bar, barOpen, playhead, tick color.Color
}

func paletteFor(context *guigui.Context) palette {
	if context.ColorMode() == ebiten.ColorModeDark {
		return palette{
			frame:      color.NRGBA{0x4a, 0x4e, 0x5c, 0xff},
			designBox:  color.NRGBA{0x6a, 0x6e, 0x7c, 0xff},
			selected:   color.NRGBA{0x5b, 0x9d, 0xf0, 0xff},
			focusable:  color.NRGBA{0x4c, 0xc2, 0x8a, 0xff},
			hovered:    color.NRGBA{0xd0, 0x9a, 0x3c, 0xff},
			canvasBack: color.NRGBA{0x20, 0x22, 0x28, 0xff},
			camera:     color.NRGBA{0xc8, 0x6f, 0xd6, 0xff},
			track:      color.NRGBA{0x2a, 0x2d, 0x36, 0xff},
			bar:        color.NRGBA{0x3d, 0x62, 0x93, 0xff},
			barOpen:    color.NRGBA{0x3d, 0x62, 0x93, 0x66},
			playhead:   color.NRGBA{0x6f, 0xb0, 0xff, 0xff},
			tick:       color.NRGBA{0x9a, 0x9a, 0xa8, 0xff},
		}
	}
	return palette{
		frame:      color.NRGBA{0xc2, 0xc6, 0xd0, 0xff},
		designBox:  color.NRGBA{0x9a, 0x9e, 0xaa, 0xff},
		selected:   color.NRGBA{0x1f, 0x6f, 0xd0, 0xff},
		focusable:  color.NRGBA{0x1e, 0x9c, 0x63, 0xff},
		hovered:    color.NRGBA{0xb5, 0x7c, 0x1a, 0xff},
		canvasBack: color.NRGBA{0xf2, 0xf3, 0xf6, 0xff},
		camera:     color.NRGBA{0x9b, 0x3f, 0xb0, 0xff},
		track:      color.NRGBA{0xe6, 0xe8, 0xee, 0xff},
		bar:        color.NRGBA{0x9d, 0xbc, 0xe6, 0xff},
		barOpen:    color.NRGBA{0x9d, 0xbc, 0xe6, 0x77},
		playhead:   color.NRGBA{0x1f, 0x6f, 0xd0, 0xff},
		tick:       color.NRGBA{0x6a, 0x6e, 0x7a, 0xff},
	}
}

func setBold(t *basicwidget.Text, s string) {
	t.SetValue(s)
	var style basicwidget.TextStyle
	style.SetBold(true)
	t.SetBaseStyle(&style)
}

func label(t *basicwidget.Text, s string) {
	t.SetValue(s)
	t.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
}

// setOptions fills a dropdown whose values are their own labels.
func setOptions(s *basicwidget.Select[string], values ...string) {
	items := make([]basicwidget.SelectItem[string], len(values))
	for i, v := range values {
		items[i] = basicwidget.SelectItem[string]{Text: v, Value: v}
	}
	s.SetItems(items)
}

// setOptionsWithDefault leads the list with a labeled empty value, for
// fields where "" means "resolve automatically".
func setOptionsWithDefault(s *basicwidget.Select[string], emptyLabel string, values ...string) {
	items := make([]basicwidget.SelectItem[string], 0, len(values)+1)
	items = append(items, basicwidget.SelectItem[string]{Text: emptyLabel, Value: ""})
	for _, v := range values {
		items = append(items, basicwidget.SelectItem[string]{Text: v, Value: v})
	}
	s.SetItems(items)
}

// framedPane draws a border around its content; the canvas is the working
// surface, so it gets an outline separating it from the control panes.
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
	w := frameWidth(context)
	layouter.LayoutWidget(f.content, widgetBounds.Bounds().Inset(w))
}

func (f *framedPane) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	pal := paletteFor(context)
	w := float32(frameWidth(context))
	b := widgetBounds.Bounds()
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
