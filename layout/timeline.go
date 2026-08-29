package main

import (
	"fmt"
	"image"
	"math"
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// timelinePane is the node manager and the choreography editor in one:
// one row per participating node — the front layer on top, like a layer
// panel — with a bar from the node's entrance (Start) over its clip's
// duration. Dragging a bar moves the entrance, dragging a row's name
// reorders the overlap, dragging the ruler scrubs the playhead, and the
// transport starts stopped and pauses itself when the choreography's
// last element finishes.
type timelinePane struct {
	guigui.DefaultWidget

	title     basicwidget.Text
	phaseSel  basicwidget.Select[string]
	timeLabel basicwidget.Text
	playBtn   basicwidget.Button
	replayBtn basicwidget.Button
	delBtn    basicwidget.Button
	names     guigui.WidgetSlice[*basicwidget.Text]

	rows []int // document indices, display order (front first)

	drag      timelineDrag
	dragIndex int // document index of the dragged node (bar drag)
	dragRow   int // display row being reordered (row drag)
	dragSpan  float64
	lastX     int

	hdrItems []guigui.LinearLayoutItem
}

type timelineDrag int

const (
	dragNone    timelineDrag = iota
	dragBar                  // moving a node's entrance time
	dragReorder              // reordering draw order
	dragRuler                // scrubbing the playhead
)

func (t *timelinePane) model(context *guigui.Context) *Model {
	v, ok := context.Env(t, envKeyModel)
	if !ok {
		return nil
	}
	m, _ := v.(*Model)
	return m
}

// span is the seconds the bar area covers: at least five, and enough for
// every entrance plus one pass of its clip. Held fixed during a drag so
// the bar under the cursor does not rescale itself away.
func (t *timelinePane) span(m *Model) float64 {
	if t.drag == dragBar || t.drag == dragRuler {
		return t.dragSpan
	}
	end := 5.0
	for _, i := range t.rows {
		n := &m.Scene().Nodes[i]
		d, _ := m.NodeDuration(n)
		if d == 0 {
			d = 1
		}
		end = max(end, n.Start+d)
	}
	return math.Ceil(end)
}

// syncRows recomputes which nodes show: the viewed phase's participants,
// front layer first.
func (t *timelinePane) syncRows(m *Model) {
	idx := m.TimelineNodes()
	t.rows = t.rows[:0]
	for i := len(idx) - 1; i >= 0; i-- {
		t.rows = append(t.rows, idx[i])
	}
}

// geometry: a header row of unit height, a ruler lane for scrubbing,
// then node rows, names on the left and bars on the right.
func (t *timelinePane) nameW(context *guigui.Context) int {
	return 6 * basicwidget.UnitSize(context)
}

func (t *timelinePane) rulerBounds(context *guigui.Context, b image.Rectangle) image.Rectangle {
	u := basicwidget.UnitSize(context)
	r := b.Inset(u / 4)
	r.Min.Y += u + u/4
	r.Max.Y = r.Min.Y + u/2
	return r
}

func (t *timelinePane) rowsBounds(context *guigui.Context, b image.Rectangle) image.Rectangle {
	u := basicwidget.UnitSize(context)
	r := b.Inset(u / 4)
	r.Min.Y += u + u/4 + u/2 + u/8
	return r
}

func (t *timelinePane) rowH(context *guigui.Context, rows image.Rectangle, count int) int {
	if count == 0 {
		return 0
	}
	u := basicwidget.UnitSize(context)
	return min(u, rows.Dy()/count)
}

// rowAt maps a cursor position to a displayed row.
func (t *timelinePane) rowAt(context *guigui.Context, b image.Rectangle, y int) (int, bool) {
	rows := t.rowsBounds(context, b)
	h := t.rowH(context, rows, len(t.rows))
	if h <= 0 || y < rows.Min.Y {
		return 0, false
	}
	row := (y - rows.Min.Y) / h
	if row < 0 || row >= len(t.rows) {
		return 0, false
	}
	return row, true
}

func (t *timelinePane) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m := t.model(context)
	if m == nil {
		return nil
	}
	t.syncRows(m)
	adder.AddWidget(&t.title)
	if len(m.PhaseNames()) > 0 {
		adder.AddWidget(&t.phaseSel)
	}
	adder.AddWidget(&t.timeLabel)
	adder.AddWidget(&t.playBtn)
	adder.AddWidget(&t.replayBtn)
	adder.AddWidget(&t.delBtn)

	setBold(&t.title, "Timeline  (top is the front; drag names to reorder)")
	setOptions(&t.phaseSel, m.PhaseNames()...)
	t.phaseSel.SelectItemByValue(m.ViewPhase())
	t.phaseSel.OnItemSelected(func(context *guigui.Context, index int) {
		it, ok := t.phaseSel.ItemByIndex(index)
		if ok && it.Value != m.ViewPhase() {
			m.SetViewPhase(it.Value)
		}
	})
	t.timeLabel.SetValue(t.timeText(m))
	t.timeLabel.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	if m.ScenePlaying() {
		t.playBtn.SetText("Pause")
	} else {
		t.playBtn.SetText("Play")
	}
	t.playBtn.OnDown(func(context *guigui.Context) { m.TogglePlayback() })
	context.SetEnabled(&t.playBtn, !m.PreviewMode())
	t.replayBtn.SetText("Replay")
	t.replayBtn.OnDown(func(context *guigui.Context) { m.ReplayScene() })
	t.delBtn.SetText("Delete")
	t.delBtn.OnDown(func(context *guigui.Context) { m.DeleteNode(m.SelectedNodeIndex()) })
	context.SetEnabled(&t.delBtn, m.SelectedNode() != nil)

	nodes := m.Scene().Nodes
	t.names.SetLen(len(t.rows))
	for row, i := range t.rows {
		w := t.names.At(row)
		adder.AddWidget(w)
		w.SetValue(nodes[i].Name)
		w.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
		w.SetScale(0.85)
		// The name column is this pane's own hit area: select and drag
		// go through HandlePointingInput.
		context.SetPassthrough(w, true)
	}
	return nil
}

func (t *timelinePane) timeText(m *Model) string {
	clock := 0.0
	if sp := m.Player(); sp != nil {
		clock = sp.Time()
	}
	return fmt.Sprintf("%5.2fs", clock)
}

// Tick keeps the clock readout current; the playhead itself is drawn, so
// a redraw is enough.
func (t *timelinePane) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	m := t.model(context)
	if m == nil {
		return nil
	}
	t.timeLabel.SetValue(t.timeText(m))
	guigui.RequestRedraw(t)
	return nil
}

func (t *timelinePane) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	if m := t.model(context); m != nil {
		w.WriteInt(m.Generation())
	}
}

func (t *timelinePane) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	b := widgetBounds.Bounds()
	m := t.model(context)

	t.hdrItems = slices.Delete(t.hdrItems, 0, len(t.hdrItems))
	t.hdrItems = append(t.hdrItems,
		guigui.LinearLayoutItem{Widget: &t.title, Size: guigui.FlexibleSize(1)})
	if m != nil && len(m.PhaseNames()) > 0 {
		t.hdrItems = append(t.hdrItems,
			guigui.LinearLayoutItem{Widget: &t.phaseSel, Size: guigui.FixedSize(5 * u)})
	}
	t.hdrItems = append(t.hdrItems,
		guigui.LinearLayoutItem{Widget: &t.timeLabel, Size: guigui.FixedSize(3 * u)},
		guigui.LinearLayoutItem{Widget: &t.playBtn, Size: guigui.FixedSize(3 * u)},
		guigui.LinearLayoutItem{Widget: &t.replayBtn, Size: guigui.FixedSize(3 * u)},
		guigui.LinearLayoutItem{Widget: &t.delBtn, Size: guigui.FixedSize(3 * u)},
	)
	hdr := image.Rect(b.Min.X+u/4, b.Min.Y+u/4, b.Max.X-u/4, b.Min.Y+u/4+u)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal, Items: t.hdrItems, Gap: u / 4,
	}).LayoutWidgets(context, hdr, layouter)

	rows := t.rowsBounds(context, b)
	h := t.rowH(context, rows, t.names.Len())
	for i := range t.names.Len() {
		y := rows.Min.Y + i*h
		layouter.LayoutWidget(t.names.At(i),
			image.Rect(rows.Min.X, y, rows.Min.X+t.nameW(context), y+h))
	}
}

func (t *timelinePane) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	m := t.model(context)
	if m == nil {
		return
	}
	pal := paletteFor(context)
	b := widgetBounds.Bounds()
	nodes := m.Scene().Nodes
	rows := t.rowsBounds(context, b)
	h := t.rowH(context, rows, len(t.rows))
	barX := rows.Min.X + t.nameW(context)
	barW := rows.Max.X - barX
	if barW <= 0 {
		return
	}
	span := t.span(m)
	pps := float64(barW) / span // pixels per second
	u := float32(basicwidget.UnitSize(context))
	bottom := float32(rows.Min.Y + len(t.rows)*h)

	// The ruler: the scrubbing surface, with a tick per second.
	ruler := t.rulerBounds(context, b)
	vector.DrawFilledRect(dst, float32(barX), float32(ruler.Min.Y),
		float32(barW), float32(ruler.Dy()), pal.track, false)
	// Widen the tick step when a long span would pack them closer than a
	// few pixels: denser is unreadable and the loop runs every frame.
	tick := 1.0
	if pps < 4 {
		tick = math.Ceil(4 / pps)
	}
	for s := 0.0; s <= span; s += tick {
		x := float32(float64(barX) + s*pps)
		vector.StrokeLine(dst, x, float32(ruler.Min.Y), x, float32(ruler.Max.Y), 1, pal.tick, false)
		if h > 0 {
			vector.StrokeLine(dst, x, float32(rows.Min.Y), x, bottom, 1, pal.tick, false)
		}
	}

	sel := m.SelectedNodeIndex()
	for row, i := range t.rows {
		n := &nodes[i]
		y := float32(rows.Min.Y + row*h)
		vector.DrawFilledRect(dst, float32(barX), y+float32(h)/4, float32(barW), float32(h)/2, pal.track, false)

		start := float64(barX) + n.Start*pps
		d, closed := m.NodeDuration(n)
		// Solid for the running-through part (the chain, one pass each),
		// faded past the point the node parks open-ended: a loop, a
		// machine, an image, a text block.
		solidEnd := float64(barX) + math.Min(span, n.Start+d)*pps
		if d > 0 {
			vector.DrawFilledRect(dst, float32(start), y+float32(h)/4, float32(solidEnd-start), float32(h)/2, pal.bar, false)
		}
		if !closed {
			vector.DrawFilledRect(dst, float32(solidEnd), y+float32(h)/4, float32(float64(rows.Max.X)-solidEnd), float32(h)/2, pal.barOpen, false)
		}
		// The entrance edge, the part a bar drag moves.
		vector.StrokeLine(dst, float32(start), y+2, float32(start), y+float32(h)-2, u/8, pal.playhead, false)
		if i == sel {
			vector.StrokeRect(dst, float32(rows.Min.X), y+1,
				float32(rows.Max.X-rows.Min.X), float32(h)-2, 1, pal.selected, false)
		}
	}

	// Content end: where playback parks itself.
	if end := m.ContentEnd(); end > 0 && end <= span {
		x := float32(float64(barX) + end*pps)
		vector.StrokeLine(dst, x, float32(ruler.Min.Y), x, bottom, 1, pal.hovered, true)
	}

	// Playhead over the whole strip, with a grip on the ruler.
	if sp := m.Player(); sp != nil {
		x := float32(float64(barX) + math.Min(sp.Time(), span)*pps)
		vector.StrokeLine(dst, x, float32(ruler.Min.Y), x, bottom, u/8, pal.playhead, true)
		vector.DrawFilledRect(dst, x-u/8, float32(ruler.Min.Y), u/4, float32(ruler.Dy()), pal.playhead, true)
	}
}

// HandlePointingInput routes the three drags: the ruler scrubs, a name
// reorders, a bar moves its entrance. A press anywhere in a row selects
// its node.
func (t *timelinePane) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	m := t.model(context)
	if m == nil {
		return guigui.HandleInputResult{}
	}
	b := widgetBounds.Bounds()
	rows := t.rowsBounds(context, b)
	barX := rows.Min.X + t.nameW(context)
	barW := rows.Max.X - barX
	if barW <= 0 {
		return guigui.HandleInputResult{}
	}
	pps := float64(barW) / t.span(m)

	if t.drag != dragNone {
		if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			if t.drag == dragBar {
				m.CommitNodeStart()
			}
			t.drag = dragNone
			return guigui.HandleInputByWidget(t)
		}
		cx, cy := ebiten.CursorPosition()
		switch t.drag {
		case dragBar:
			if dx := cx - t.lastX; dx != 0 {
				t.lastX = cx
				nodes := m.Scene().Nodes
				if t.dragIndex >= 0 && t.dragIndex < len(nodes) {
					m.SetNodeStart(t.dragIndex, nodes[t.dragIndex].Start+float64(dx)/pps)
				}
			}
		case dragRuler:
			m.SeekScene(float64(cx-barX) / pps)
		case dragReorder:
			if row, ok := t.rowAt(context, b, cy); ok && row != t.dragRow {
				// Walk one swap at a time so a fast drag still passes
				// through every slot in order.
				step := 1
				if row < t.dragRow {
					step = -1
				}
				for t.dragRow != row {
					m.SwapNodes(t.rows[t.dragRow], t.rows[t.dragRow+step])
					t.dragRow += step
					t.syncRows(m)
				}
			}
		}
		return guigui.HandleInputByWidget(t)
	}
	if !widgetBounds.IsHitAtCursor() || !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return guigui.HandleInputResult{}
	}
	cx, cy := ebiten.CursorPosition()
	if p := image.Pt(cx, cy); p.In(t.rulerBounds(context, b)) && cx >= barX {
		t.drag = dragRuler
		t.dragSpan = t.span(m)
		m.SeekScene(float64(cx-barX) / pps)
		return guigui.HandleInputByWidget(t)
	}
	row, ok := t.rowAt(context, b, cy)
	if !ok {
		return guigui.HandleInputResult{}
	}
	m.SelectNode(t.rows[row])
	if cx < barX {
		t.drag = dragReorder
		t.dragRow = row
	} else {
		t.drag = dragBar
		t.dragIndex = t.rows[row]
		t.dragSpan = t.span(m)
		t.lastX = cx
	}
	return guigui.HandleInputByWidget(t)
}

func (t *timelinePane) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	switch t.drag {
	case dragBar, dragRuler:
		return ebiten.CursorShapeEWResize, true
	case dragReorder:
		return ebiten.CursorShapeMove, true
	}
	return 0, false
}

func (t *timelinePane) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	return image.Pt(8*u, 7*u)
}
