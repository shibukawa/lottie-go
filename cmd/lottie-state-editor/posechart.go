package main

import (
	"image"
	"image/color"
	"math"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// poseChartView marks where a clip's keyframes are. Without it the timeline
// says a clip is 20 frames long but not that only frames 0, 4, 7, 12 and 20
// hold anything — and those are the only frames an edit can land on.
//
// A preset clip keeps every animated property on one set of times, so a tick
// is a whole-body pose and there is a single row. When the times disagree
// the tick set is per property, so the chart falls back to a row per
// animated layer. Ticks select, drag retimes, everything else scrubs, and
// any touch pauses first.
type poseChartView struct {
	guigui.DefaultWidget

	playBtn basicwidget.Button
	backBtn basicwidget.Button
	fwdBtn  basicwidget.Button
	labels  guigui.WidgetSlice[*basicwidget.Text]

	drag     poseDrag
	dragRow  int     // layer index, or -1 for the pose row
	dragFrom float64 // the key time being dragged, updated as it lands
}

type poseDrag int

const (
	poseDragNone poseDrag = iota
	poseDragScrub
	poseDragKey
)

func (c *poseChartView) model(context *guigui.Context) *Model {
	v, ok := context.Env(c, envKeyModel)
	if !ok {
		return nil
	}
	m, _ := v.(*Model)
	return m
}

// poseChartRows is what the chart draws: the layer indices of the fallback
// rows, or a single -1 meaning the pose row. An empty result is a clip with
// no keyframes at all, which draws as a bare ruler.
func poseChartRows(m *Model) []int {
	if m.StageClipDoc() == nil {
		return nil
	}
	if len(m.PoseTimes()) > 0 {
		return []int{-1}
	}
	return m.PoseRows()
}

func (c *poseChartView) plotRect(context *guigui.Context, b image.Rectangle) image.Rectangle {
	u := basicwidget.UnitSize(context)
	return image.Rect(b.Min.X+chartGutter(u), b.Min.Y, b.Max.X-u/2, b.Max.Y)
}

func (c *poseChartView) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m := c.model(context)
	if m == nil {
		return nil
	}
	adder.AddWidget(&c.playBtn)
	adder.AddWidget(&c.backBtn)
	adder.AddWidget(&c.fwdBtn)

	if m.PreviewPlaying() {
		c.playBtn.SetText("❚❚")
	} else {
		c.playBtn.SetText("▶")
	}
	c.playBtn.OnDown(func(context *guigui.Context) { m.TogglePreviewPlaying() })
	c.backBtn.SetText("−1")
	c.backBtn.OnDown(func(context *guigui.Context) { m.StepPreviewFrame(-1) })
	c.fwdBtn.SetText("+1")
	c.fwdBtn.OnDown(func(context *guigui.Context) { m.StepPreviewFrame(1) })
	enabled := m.PreviewPlayer() != nil
	for _, w := range []guigui.Widget{&c.playBtn, &c.backBtn, &c.fwdBtn} {
		context.SetEnabled(w, enabled)
	}

	rows := poseChartRows(m)
	c.labels.SetLen(len(rows))
	for i, row := range rows {
		adder.AddWidget(c.labels.At(i))
		l := c.labels.At(i)
		if row < 0 {
			l.SetValue("Poses")
		} else {
			l.SetValue(m.PoseLayerName(row))
		}
		l.SetScale(0.75)
		l.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
		context.SetPassthrough(l, true)
	}
	return nil
}

func (c *poseChartView) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	b := widgetBounds.Bounds()

	btnW := (chartGutter(u) - u/4) / 3
	for i, w := range []guigui.Widget{&c.playBtn, &c.backBtn, &c.fwdBtn} {
		x := b.Min.X + i*(btnW+u/16)
		layouter.LayoutWidget(w, image.Rect(x, b.Min.Y, x+btnW, b.Min.Y+u*3/4))
	}
	rowH := chartRowH(u)
	for i := range c.labels.Len() {
		y := b.Min.Y + u + i*rowH
		layouter.LayoutWidget(c.labels.At(i),
			image.Rect(b.Min.X+u/8, y, b.Min.X+chartGutter(u)-u/8, y+rowH))
	}
}

func (c *poseChartView) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w := 8 * u
	if cw, ok := constraints.FixedWidth(); ok {
		w = cw
	}
	rows := 0
	if m := c.model(context); m != nil {
		rows = len(poseChartRows(m))
	}
	// A clip with no keyframes still shows its ruler, so the row area never
	// collapses to nothing and the tab does not look broken.
	return image.Pt(w, u+max(rows, 1)*chartRowH(u)+u/8)
}

func (c *poseChartView) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	m := c.model(context)
	if m == nil {
		return
	}
	lo, hi, ok := stageDocRange(m)
	if !ok {
		return
	}
	pal := paletteFor(context)
	u := basicwidget.UnitSize(context)
	b := widgetBounds.Bounds()
	plot := c.plotRect(context, b)
	x := func(frame float64) float32 {
		return float32(plot.Min.X) + float32(plot.Dx())*float32((frame-lo)/(hi-lo))
	}
	rowH := chartRowH(u)

	// Header: the played range as a band and a ruler every 10 frames, the
	// same extent the scrub timeline and the span chart use, so one frame
	// sits at one x wherever it is drawn.
	ruleY := float32(b.Min.Y + u*3/4)
	if p := m.PreviewPlayer(); p != nil {
		start, end := p.Range()
		vector.DrawFilledRect(dst, x(start), float32(b.Min.Y)+float32(u)/4,
			x(end)-x(start), ruleY-float32(b.Min.Y)-float32(u)/4, pal.band, false)
	}
	vector.StrokeLine(dst, float32(plot.Min.X), ruleY, float32(plot.Max.X), ruleY,
		1, pal.tick, false)
	if step := 10.0; x(step)-x(0) > 4 {
		for f := math.Ceil(lo/step) * step; f <= hi; f += step {
			vector.StrokeLine(dst, x(f), ruleY-float32(u)/6, x(f), ruleY, 1, pal.tick, false)
		}
	}

	selFrame, hasSel := m.SelectedPoseKey()
	selRow := m.SelectedPoseRow()
	for i, row := range poseChartRows(m) {
		top := float32(b.Min.Y + u + i*rowH)
		if hasSel && row == selRow {
			vector.DrawFilledRect(dst, float32(b.Min.X), top,
				float32(b.Dx()), float32(rowH), pal.band, false)
		}
		mid := top + float32(rowH)/2
		vector.StrokeLine(dst, float32(plot.Min.X), mid, float32(plot.Max.X), mid,
			1, pal.track, false)
		for _, t := range m.poseTimesFor(row) {
			selected := hasSel && row == selRow && t == selFrame
			drawKeyTick(dst, x(t), mid, float32(rowH), pal,
				m.PoseKeyIsHold(t, row), selected)
		}
	}

	if p := m.PreviewPlayer(); p != nil {
		head := x(p.Frame())
		tickW := float32(max(1, u/16))
		vector.StrokeLine(dst, head, float32(b.Min.Y)+float32(u)/4, head, float32(b.Max.Y),
			tickW*2, pal.playhead, false)
	}
}

// drawKeyTick marks one keyframe. A hold key is square and an interpolated
// one is a diamond, so a discrete swap — a limb trading sides — never looks
// like a value there is something to nudge between.
func drawKeyTick(dst *ebiten.Image, cx, cy, rowH float32, pal palette, hold, selected bool) {
	r := rowH / 3
	clr := pal.tick
	if selected {
		clr = pal.playhead
	}
	if hold {
		if selected {
			vector.DrawFilledRect(dst, cx-r, cy-r, r*2, r*2, clr, false)
		} else {
			vector.StrokeRect(dst, cx-r, cy-r, r*2, r*2, 1.5, clr, false)
		}
		return
	}
	var p vector.Path
	p.MoveTo(cx, cy-r)
	p.LineTo(cx+r, cy)
	p.LineTo(cx, cy+r)
	p.LineTo(cx-r, cy)
	p.Close()
	op := pathColor(clr)
	if selected {
		vector.FillPath(dst, &p, &vector.FillOptions{FillRule: vector.FillRuleNonZero}, &op)
	} else {
		vector.StrokePath(dst, &p, &vector.StrokeOptions{Width: 1.5}, &op)
	}
}

// pathColor turns a palette entry into the draw options the path functions
// take, which carry a color scale rather than a color. Color.RGBA is already
// alpha-premultiplied, which is the form the scale wants.
func pathColor(c color.Color) vector.DrawPathOptions {
	var op vector.DrawPathOptions
	op.AntiAlias = true
	r, g, b, a := c.RGBA()
	op.ColorScale.Scale(float32(r)/0xffff, float32(g)/0xffff,
		float32(b)/0xffff, float32(a)/0xffff)
	return op
}

func (c *poseChartView) frameAt(m *Model, plot image.Rectangle, cx int) (float64, bool) {
	lo, hi, ok := stageDocRange(m)
	if !ok || plot.Dx() <= 0 {
		return 0, false
	}
	f := lo + (hi-lo)*float64(cx-plot.Min.X)/float64(plot.Dx())
	return math.Round(min(max(f, lo), hi)), true
}

// keyAt finds the tick under the cursor, within a few pixels so a diamond is
// grabbable without being pixel-perfect.
func (c *poseChartView) keyAt(m *Model, plot image.Rectangle, row int, cx int) (float64, bool) {
	lo, hi, ok := stageDocRange(m)
	if !ok || plot.Dx() <= 0 {
		return 0, false
	}
	fc := lo + (hi-lo)*float64(cx-plot.Min.X)/float64(plot.Dx())
	slack := (hi - lo) * 5 / float64(plot.Dx())
	best, bestD := 0.0, math.MaxFloat64
	for _, t := range m.poseTimesFor(row) {
		if d := math.Abs(fc - t); d <= slack && d < bestD {
			best, bestD = t, d
		}
	}
	return best, bestD < math.MaxFloat64
}

func (c *poseChartView) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	m := c.model(context)
	if m == nil {
		return guigui.HandleInputResult{}
	}
	u := basicwidget.UnitSize(context)
	b := widgetBounds.Bounds()
	plot := c.plotRect(context, b)

	if c.drag != poseDragNone {
		if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			c.drag = poseDragNone
			m.EndPoseEdit()
			return guigui.HandleInputByWidget(c)
		}
		cx, _ := ebiten.CursorPosition()
		frame, ok := c.frameAt(m, plot, cx)
		if !ok {
			return guigui.HandleInputByWidget(c)
		}
		switch c.drag {
		case poseDragScrub:
			m.PreviewSeek(frame)
		case poseDragKey:
			if frame != c.dragFrom {
				m.RetimePoseKey(c.dragFrom, frame, c.dragRow)
				// The key stops one frame short of its neighbours, so where
				// it landed is not always where the cursor is; follow the
				// key, not the cursor, or the next step drags the wrong one.
				if f, ok := m.SelectedPoseKey(); ok {
					c.dragFrom = f
				}
			}
		}
		return guigui.HandleInputByWidget(c)
	}

	if !widgetBounds.IsHitAtCursor() || !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return guigui.HandleInputResult{}
	}
	cx, cy := ebiten.CursorPosition()
	rows := poseChartRows(m)
	row := (cy - b.Min.Y - u) / chartRowH(u)
	inRows := cy >= b.Min.Y+u && row >= 0 && row < len(rows)

	if cx < plot.Min.X {
		// The gutter is transport buttons and row labels; a label click
		// selects that row's first key, which is how a row becomes the edit
		// target without hunting for a tick.
		if inRows {
			if times := m.poseTimesFor(rows[row]); len(times) > 0 {
				m.SelectPoseKey(times[0], rows[row])
			}
			return guigui.HandleInputByWidget(c)
		}
		return guigui.HandleInputResult{}
	}
	frame, ok := c.frameAt(m, plot, cx)
	if !ok {
		return guigui.HandleInputResult{}
	}
	// Touching the chart pauses at once: placing anything under a moving
	// playhead is hopeless.
	m.PausePreview()

	if inRows {
		if key, ok := c.keyAt(m, plot, rows[row], cx); ok {
			m.SelectPoseKey(key, rows[row])
			c.drag, c.dragRow, c.dragFrom = poseDragKey, rows[row], key
			// A retime drag steps frame by frame; collapse it into one
			// undo step the way a stage swing is collapsed.
			m.BeginPoseEdit()
			return guigui.HandleInputByWidget(c)
		}
	}
	c.drag = poseDragScrub
	m.PreviewSeek(frame)
	return guigui.HandleInputByWidget(c)
}

func (c *poseChartView) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if c.drag != poseDragNone {
		return ebiten.CursorShapeEWResize, true
	}
	return 0, false
}

// Tick redraws so the playhead follows playback without rebuilding.
func (c *poseChartView) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	guigui.RequestRedraw(c)
	return nil
}

func (c *poseChartView) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	m := c.model(context)
	if m == nil {
		return
	}
	w.WriteInt(m.Generation())
	w.WriteString(m.StageAnimID())
	w.WriteBool(m.PreviewPlaying())
}
