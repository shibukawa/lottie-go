package main

import (
	"fmt"
	"image"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// timelineView sits under the stage and shows where playback is inside the
// animation: the whole document as a track, the range actually being played
// as a band, and every marker as a tick. Markers are what a state's segment
// names, so seeing them is how you tell a segment was cut where you meant.
//
// Dragging scrubs.
type timelineView struct {
	guigui.DefaultWidget

	readout basicwidget.Text
	labels  guigui.WidgetSlice[*basicwidget.Text]

	dragging bool
}

func (t *timelineView) model(context *guigui.Context) *Model {
	v, ok := context.Env(t, envKeyModel)
	if !ok {
		return nil
	}
	m, _ := v.(*Model)
	return m
}

func (t *timelineView) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m := t.model(context)
	if m == nil {
		return nil
	}
	adder.AddWidget(&t.readout)

	markers := m.PreviewMarkers()
	t.labels.SetLen(len(markers))
	for i := range markers {
		adder.AddWidget(t.labels.At(i))
	}
	for i, mk := range markers {
		l := t.labels.At(i)
		l.SetValue(mk.Name)
		l.SetScale(0.75)
		l.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
		context.SetPassthrough(l, true)
	}

	t.syncReadout(m)
	t.readout.SetScale(0.8)
	t.readout.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	t.readout.SetHorizontalAlign(basicwidget.HorizontalAlignEnd)
	context.SetPassthrough(&t.readout, true)
	return nil
}

func (t *timelineView) readoutText(m *Model) string {
	p := m.PreviewPlayer()
	anim := m.PreviewAnimation()
	if p == nil || anim == nil {
		return ""
	}
	start, end := p.Range()
	fps := anim.FrameRate()
	if fps <= 0 {
		fps = 60
	}
	return fmt.Sprintf("frame %.0f   %.2fs / %.2fs",
		p.Frame(), (p.Frame()-start)/fps, (end-start)/fps)
}

// docRange is the whole animation, which the track represents. The chart
// shares the same extent so its bars line up with this ruler.
func (t *timelineView) docRange(m *Model) (float64, float64, bool) {
	return stageDocRange(m)
}

// stageDocRange is the frame extent of the animation on stage.
func stageDocRange(m *Model) (float64, float64, bool) {
	anim := m.PreviewAnimation()
	p := m.PreviewPlayer()
	if anim == nil || p == nil {
		return 0, 0, false
	}
	// The player's range may be a segment; the document is the full extent,
	// so a segment reads as a band inside it.
	lo, hi := p.Range()
	for _, mk := range anim.Markers() {
		lo = min(lo, mk.Start)
		hi = max(hi, mk.End)
	}
	total := anim.Duration().Seconds() * anim.FrameRate()
	lo = min(lo, 0)
	hi = max(hi, total)
	if hi <= lo {
		return 0, 0, false
	}
	return lo, hi, true
}

// trackRect shares the chart's horizontal extent (gutter to right pad),
// so a frame sits at the same x on both rulers.
func (t *timelineView) trackRect(context *guigui.Context, bounds image.Rectangle) image.Rectangle {
	u := basicwidget.UnitSize(context)
	return image.Rect(bounds.Min.X+chartGutter(u), bounds.Min.Y+u/3,
		bounds.Max.X-u/2, bounds.Min.Y+u/3+u/2)
}

func (t *timelineView) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	m := t.model(context)
	if m == nil {
		return
	}
	lo, hi, ok := t.docRange(m)
	if !ok {
		return
	}
	pal := paletteFor(context)
	tr := t.trackRect(context, widgetBounds.Bounds())
	x := func(frame float64) float32 {
		return float32(tr.Min.X) + float32(tr.Dx())*float32((frame-lo)/(hi-lo))
	}

	vector.DrawFilledRect(dst, float32(tr.Min.X), float32(tr.Min.Y),
		float32(tr.Dx()), float32(tr.Dy()), pal.track, false)

	// The band is the part actually being played, which is the segment when
	// a state names one.
	p := m.PreviewPlayer()
	start, end := p.Range()
	vector.DrawFilledRect(dst, x(start), float32(tr.Min.Y),
		x(end)-x(start), float32(tr.Dy()), pal.band, false)

	tickW := float32(max(1, basicwidget.UnitSize(context)/16))
	for _, mk := range m.PreviewMarkers() {
		vector.StrokeLine(dst, x(mk.Start), float32(tr.Min.Y)-tickW*2,
			x(mk.Start), float32(tr.Max.Y)+tickW*2, tickW, pal.tick, false)
	}

	head := x(p.Frame())
	vector.StrokeLine(dst, head, float32(tr.Min.Y)-tickW*3,
		head, float32(tr.Max.Y)+tickW*3, tickW*2, pal.playhead, false)
}

func (t *timelineView) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	m := t.model(context)
	if m == nil {
		return
	}
	u := basicwidget.UnitSize(context)
	b := widgetBounds.Bounds()
	layouter.LayoutWidget(&t.readout, image.Rect(b.Min.X, b.Max.Y-u, b.Max.X-u/2, b.Max.Y))

	lo, hi, ok := t.docRange(m)
	if !ok {
		return
	}
	tr := t.trackRect(context, b)
	markers := m.PreviewMarkers()
	for i, mk := range markers {
		if i >= t.labels.Len() {
			break
		}
		x := tr.Min.X + int(float64(tr.Dx())*(mk.Start-lo)/(hi-lo))
		layouter.LayoutWidget(t.labels.At(i),
			image.Rect(x+u/8, tr.Max.Y, x+4*u, tr.Max.Y+u))
	}
}

func (t *timelineView) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w := 8 * u
	if cw, ok := constraints.FixedWidth(); ok {
		w = cw
	}
	return image.Pt(w, 5*u/2)
}

// WriteStateKey keeps the readout and the marker labels current. The
// playhead itself moves every tick and is a redraw, requested by the stage.
func (t *timelineView) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	m := t.model(context)
	if m == nil {
		return
	}
	w.WriteInt(m.Generation())
	w.WriteString(m.PreviewClip().Anim + "/" + m.PreviewClip().Segment)
	w.WriteString(m.ActiveState())
}

// syncReadout writes the current position into the label. It runs from Tick
// as well as Build: the playhead is redrawn every frame, and a readout that
// only refreshed on a rebuild would disagree with it.
func (t *timelineView) syncReadout(m *Model) {
	t.readout.SetValue(t.readoutText(m))
}

// Tick redraws so the playhead follows playback without rebuilding the tree.
func (t *timelineView) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	if m := t.model(context); m != nil {
		t.syncReadout(m)
	}
	guigui.RequestRedraw(t)
	return nil
}

func (t *timelineView) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if widgetBounds.IsHitAtCursor() || t.dragging {
		return ebiten.CursorShapeEWResize, true
	}
	return 0, false
}

func (t *timelineView) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	m := t.model(context)
	if m == nil {
		return guigui.HandleInputResult{}
	}
	if t.dragging && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		t.dragging = false
		return guigui.HandleInputByWidget(t)
	}
	if !t.dragging {
		if !widgetBounds.IsHitAtCursor() || !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			return guigui.HandleInputResult{}
		}
		t.dragging = true
	}
	lo, hi, ok := t.docRange(m)
	if !ok {
		return guigui.HandleInputByWidget(t)
	}
	tr := t.trackRect(context, widgetBounds.Bounds())
	cx, _ := ebiten.CursorPosition()
	u := float64(cx-tr.Min.X) / float64(tr.Dx())
	m.PreviewSeek(lo + u*(hi-lo))
	return guigui.HandleInputByWidget(t)
}
