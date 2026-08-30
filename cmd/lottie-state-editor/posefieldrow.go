package main

import (
	"image"
	"image/color"
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// poseFieldRow is one numeric field of the pose form: the text input and, at
// its right, two buttons that copy the neighbouring keys' values into it.
// Posing is mostly done in differences — this frame is a neighbour with a few
// numbers nudged — so the value most often wanted in a field is the one the
// same member holds one key away, and that is worth a button instead of a
// number read off another frame and retyped. Both directions exist because
// both edits happen: building forward from the last pose, and pulling an
// in-between back toward where the motion lands.
type poseFieldRow struct {
	guigui.DefaultWidget

	text basicwidget.TextInput
	prev basicwidget.Button
	next basicwidget.Button
	// The buttons are icons, and which key an icon means is exactly the kind
	// of thing a tooltip is for; the inspector fills in the frame numbers.
	prevTip basicwidget.TooltipArea
	nextTip basicwidget.TooltipArea

	items     []guigui.LinearLayoutItem
	boundsArr []image.Rectangle
}

func (r *poseFieldRow) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&r.text)
	adder.AddWidget(&r.prev)
	adder.AddWidget(&r.next)
	adder.AddWidget(&r.prevTip)
	adder.AddWidget(&r.nextTip)
	// A clipboard with the arrow pointing along the timeline at the key the
	// value comes from. A bare arrow here read as a nudge button — the
	// clipboard is what says "copy", not "decrement". Each button is
	// disabled whenever there is nothing to fetch — no key on that side, or
	// the field already matches it — so enabled also means "this frame
	// differs from that one here".
	dark := context.ColorMode() == ebiten.ColorModeDark
	r.prev.SetIcon(poseCopyIcon(dark, -1))
	r.next.SetIcon(poseCopyIcon(dark, +1))
	return nil
}

func (r *poseFieldRow) layout(context *guigui.Context) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)
	r.items = slices.Delete(r.items, 0, len(r.items))
	r.items = append(r.items,
		guigui.LinearLayoutItem{Widget: &r.text, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &r.prev, Size: guigui.FixedSize(5 * u / 4)},
		guigui.LinearLayoutItem{Widget: &r.next, Size: guigui.FixedSize(5 * u / 4)},
	)
	return guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     r.items,
		Gap:       u / 8,
	}
}

func (r *poseFieldRow) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layout := r.layout(context)
	layout.LayoutWidgets(context, widgetBounds.Bounds(), layouter)
	// The tooltip areas sit exactly over their buttons.
	r.boundsArr = layout.AppendItemBounds(r.boundsArr[:0], context, widgetBounds.Bounds())
	layouter.LayoutWidget(&r.prevTip, r.boundsArr[1])
	layouter.LayoutWidget(&r.nextTip, r.boundsArr[2])
}

func (r *poseFieldRow) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	// The row reports the bare input's size and the buttons take their share
	// out of it in Layout — the numbers are short, so the input can spare
	// the width. Reporting the summed width instead would overflow the
	// form's value column and push every row onto two lines.
	return r.text.Measure(context, constraints)
}

// poseCopyIconCache holds the four copy-button icons: per direction and per
// color mode, drawn once at first use. dir is -1 or +1, matching the model's
// adjacent-key direction.
var poseCopyIconCache = map[[2]bool]*ebiten.Image{}

// poseCopyIcon draws a clipboard carrying a triangle that points at the key
// the value would come from: left for the previous key, right for the next.
func poseCopyIcon(dark bool, dir int) *ebiten.Image {
	key := [2]bool{dark, dir > 0}
	if img, ok := poseCopyIconCache[key]; ok {
		return img
	}
	// Drawn oversized and scaled down by the button, so the strokes stay
	// crisp at any UI scale.
	const s = 48
	img := ebiten.NewImage(s, s)
	ink := color.NRGBA{0x30, 0x30, 0x38, 0xff}
	if dark {
		ink = color.NRGBA{0xe6, 0xe6, 0xea, 0xff}
	}
	// The clipboard: a board with a small clip tab on top.
	vector.StrokeRect(img, 9, 8, 30, 37, 4.5, ink, true)
	vector.DrawFilledRect(img, 16, 2, 16, 11, ink, true)
	// The triangle points at the neighbour being copied from.
	cx, cy, w, h := float32(24), float32(29), float32(9), float32(10)
	var p vector.Path
	p.MoveTo(cx+float32(dir)*w, cy)
	p.LineTo(cx-float32(dir)*w, cy-h)
	p.LineTo(cx-float32(dir)*w, cy+h)
	p.Close()
	op := pathColor(ink)
	vector.FillPath(img, &p, &vector.FillOptions{FillRule: vector.FillRuleNonZero}, &op)
	poseCopyIconCache[key] = img
	return img
}
