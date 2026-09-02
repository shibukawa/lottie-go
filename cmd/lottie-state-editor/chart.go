package main

import (
	"image"
	"math"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	lottieresolv "github.com/shibukawa/lottie-go/plugin/physics/resolv"
)

// chartView is the After-Effects-style time chart under the timeline: one
// row per hitbox or window of the stage clip, its spans drawn as
// tag-colored bars on the frame axis. Bars drag to move, their edges drag
// to retime, the header scrubs, and any touch pauses playback first —
// placing a span under a moving playhead is near impossible. The selected
// bar's parameters are edited in the inspector; the stage still owns
// geometry. Transport (play/pause, ±1 frame) lives in the header gutter.
type chartView struct {
	guigui.DefaultWidget

	playBtn basicwidget.Button
	backBtn basicwidget.Button
	fwdBtn  basicwidget.Button
	labels  guigui.WidgetSlice[*basicwidget.Text]

	drag      chartDrag
	dragBox   int
	dragSpan  int
	dragStart float64 // frame under the cursor at press
	origFrom  float64 // dragged span's bounds at press
	origTo    float64
	watch     frameWatch
}

type chartDrag int

const (
	chartDragNone chartDrag = iota
	chartDragScrub
	chartDragMove
	chartDragLeft
	chartDragRight
)

func (c *chartView) model(context *guigui.Context) *Model {
	v, ok := context.Env(c, envKeyModel)
	if !ok {
		return nil
	}
	m, _ := v.(*Model)
	return m
}

// chartRows are the boxes charted: the stage track's, in order, or none
// while resolv tooling is configured away.
func chartRows(m *Model) []lottieresolv.Box {
	if !m.ResolvEnabled() {
		return nil
	}
	if t := m.StageTrack(); t != nil {
		return t.Boxes
	}
	return nil
}

func chartGutter(u int) int { return 4 * u }
func chartRowH(u int) int   { return u / 2 }

// plotRect is where frames map to pixels: right of the gutter.
func (c *chartView) plotRect(context *guigui.Context, b image.Rectangle) image.Rectangle {
	u := basicwidget.UnitSize(context)
	return image.Rect(b.Min.X+chartGutter(u), b.Min.Y, b.Max.X-u/2, b.Max.Y)
}

func (c *chartView) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
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

	rows := chartRows(m)
	c.labels.SetLen(len(rows))
	for i := range rows {
		adder.AddWidget(c.labels.At(i))
		l := c.labels.At(i)
		l.SetValue(rows[i].Name)
		l.SetScale(0.75)
		l.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
		context.SetPassthrough(l, true)
	}
	return nil
}

func (c *chartView) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	m := c.model(context)
	if m == nil {
		return
	}
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

func (c *chartView) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w := 8 * u
	if cw, ok := constraints.FixedWidth(); ok {
		w = cw
	}
	rows := 0
	if m := c.model(context); m != nil {
		rows = len(chartRows(m))
	}
	return image.Pt(w, u+rows*chartRowH(u)+u/8)
}

func (c *chartView) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
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
	rows := chartRows(m)
	rowTop := func(i int) float32 { return float32(b.Min.Y + u + i*rowH) }

	// The band is the range actually being played — the segment when one
	// is on stage — mirroring the scrub timeline above, whose ruler this
	// chart shares pixel for pixel. Spans outside it belong to other
	// segments of the same file.
	ruleY := float32(b.Min.Y + u*3/4)
	if p := m.PreviewPlayer(); p != nil {
		start, end := p.Range()
		vector.DrawFilledRect(dst, x(start), float32(b.Min.Y)+float32(u)/4,
			x(end)-x(start), ruleY-float32(b.Min.Y)-float32(u)/4, pal.band, false)
	}
	// Header ruler with a tick every 10 frames while they stay legible.
	vector.StrokeLine(dst, float32(plot.Min.X), ruleY, float32(plot.Max.X), ruleY,
		1, pal.tick, false)
	if step := 10.0; x(step)-x(0) > 4 {
		for f := math.Ceil(lo/step) * step; f <= hi; f += step {
			vector.StrokeLine(dst, x(f), ruleY-float32(u)/6, x(f), ruleY, 1, pal.tick, false)
		}
	}

	sel := m.SelectedHitboxIndex()
	for i, box := range rows {
		top := rowTop(i)
		if i == sel {
			vector.DrawFilledRect(dst, float32(b.Min.X), top,
				float32(b.Dx()), float32(rowH), pal.band, false)
		}
		vector.StrokeLine(dst, float32(plot.Min.X), top+float32(rowH),
			float32(plot.Max.X), top+float32(rowH), 1, pal.track, false)

		clr := tagColor(box.Tags)
		barTop := top + float32(rowH)/6
		barH := float32(rowH) * 2 / 3
		for _, sp := range box.Spans {
			w := x(sp.To) - x(sp.From)
			if box.Kind == lottieresolv.KindWindow {
				// Windows have no geometry on stage; hollow bars keep them
				// visually lighter than the solid judgement boxes.
				vector.StrokeRect(dst, x(sp.From), barTop, w, barH, 1.5, withAlpha(clr, 0xe6), true)
			} else {
				vector.DrawFilledRect(dst, x(sp.From), barTop, w, barH, withAlpha(clr, 0xc0), true)
			}
		}
	}

	if p := m.PreviewPlayer(); p != nil {
		head := x(p.Frame())
		tickW := float32(max(1, u/16))
		vector.StrokeLine(dst, head, float32(b.Min.Y)+float32(u)/4, head, float32(b.Max.Y),
			tickW*2, pal.playhead, false)
	}
}

// frameAt converts a cursor x to a frame, snapped to whole frames.
func (c *chartView) frameAt(m *Model, plot image.Rectangle, cx int) (float64, bool) {
	lo, hi, ok := stageDocRange(m)
	if !ok || plot.Dx() <= 0 {
		return 0, false
	}
	f := lo + (hi-lo)*float64(cx-plot.Min.X)/float64(plot.Dx())
	return math.Round(min(max(f, lo), hi)), true
}

func (c *chartView) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	m := c.model(context)
	if m == nil {
		return guigui.HandleInputResult{}
	}
	u := basicwidget.UnitSize(context)
	b := widgetBounds.Bounds()
	plot := c.plotRect(context, b)

	if c.drag != chartDragNone {
		cx, _ := ebiten.CursorPosition()
		frame, ok := c.frameAt(m, plot, cx)
		if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			if c.drag == chartDragMove || c.drag == chartDragLeft || c.drag == chartDragRight {
				m.NormalizeSpans(c.dragBox)
			}
			c.drag = chartDragNone
			return guigui.HandleInputByWidget(c)
		}
		if !ok {
			return guigui.HandleInputByWidget(c)
		}
		switch c.drag {
		case chartDragScrub:
			m.PreviewSeek(frame)
		case chartDragMove:
			_, sp := m.boxSpan(c.dragBox, c.dragSpan)
			if sp != nil {
				want := c.origFrom + (frame - c.dragStart)
				m.ShiftSpan(c.dragBox, c.dragSpan, want-sp.From)
			}
		case chartDragLeft:
			m.SetSpanEdge(c.dragBox, c.dragSpan, false, frame)
		case chartDragRight:
			m.SetSpanEdge(c.dragBox, c.dragSpan, true, frame)
		}
		return guigui.HandleInputByWidget(c)
	}

	if !widgetBounds.IsHitAtCursor() || !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return guigui.HandleInputResult{}
	}
	cx, cy := ebiten.CursorPosition()
	if cx < plot.Min.X {
		// The gutter belongs to the transport buttons and row labels; a
		// label click still selects its row.
		if row := (cy - b.Min.Y - u) / chartRowH(u); cy >= b.Min.Y+u && row >= 0 && row < len(chartRows(m)) {
			m.PausePreview()
			m.SelectHitbox(row)
			return guigui.HandleInputByWidget(c)
		}
		return guigui.HandleInputResult{}
	}
	frame, ok := c.frameAt(m, plot, cx)
	if !ok {
		return guigui.HandleInputResult{}
	}
	// Touching the chart pauses at once: the whole point is a playhead
	// that holds still while you work.
	m.PausePreview()

	rows := chartRows(m)
	row := (cy - b.Min.Y - u) / chartRowH(u)
	if cy >= b.Min.Y+u && row >= 0 && row < len(rows) {
		lo, hi, _ := stageDocRange(m)
		edge := (hi - lo) * 4 / float64(max(plot.Dx(), 1)) // ~4px in frames
		for si, sp := range rows[row].Spans {
			fc := float64(cx-plot.Min.X)/float64(plot.Dx())*(hi-lo) + lo
			switch {
			case math.Abs(fc-sp.From) <= edge:
				c.drag, c.dragBox, c.dragSpan = chartDragLeft, row, si
			case math.Abs(fc-sp.To) <= edge:
				c.drag, c.dragBox, c.dragSpan = chartDragRight, row, si
			case fc >= sp.From && fc < sp.To:
				c.drag, c.dragBox, c.dragSpan = chartDragMove, row, si
				c.dragStart, c.origFrom, c.origTo = frame, sp.From, sp.To
			default:
				continue
			}
			m.SelectHitbox(row)
			return guigui.HandleInputByWidget(c)
		}
		// Empty row area: select the row and scrub from here.
		m.SelectHitbox(row)
		c.drag = chartDragScrub
		m.PreviewSeek(frame)
		return guigui.HandleInputByWidget(c)
	}
	// Header (or below the rows): scrub.
	c.drag = chartDragScrub
	m.PreviewSeek(frame)
	return guigui.HandleInputByWidget(c)
}

func (c *chartView) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	switch c.drag {
	case chartDragLeft, chartDragRight:
		return ebiten.CursorShapeEWResize, true
	case chartDragMove, chartDragScrub:
		return ebiten.CursorShapeEWResize, true
	}
	return 0, false
}

// Tick redraws so the playhead follows playback without rebuilding, only
// while it moves; the state key covers everything else.
func (c *chartView) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	m := c.model(context)
	if m == nil {
		return nil
	}
	if c.watch.moved(m) || c.drag != chartDragNone {
		guigui.RequestRedraw(c)
	}
	return nil
}

func (c *chartView) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	m := c.model(context)
	if m == nil {
		return
	}
	w.WriteInt(m.Generation())
	w.WriteString(m.StageAnimID())
	// Playback can stop on its own (loop count, clip end); the play
	// button's glyph must follow.
	w.WriteBool(m.PreviewPlaying())
}
