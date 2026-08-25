package main

import (
	"image"
	"image/color"
	"math"
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	lottie "github.com/shibukawa/lottie-go"
)

// The state graph has no basicwidget equivalent, so it is drawn here: the
// view paints the transition edges and hosts one child widget per state,
// which guigui draws on top.

type palette struct {
	edge, node, nodeBorder, selected, initial, global color.Color
	// active marks the state the preview is in right now; traced marks the
	// transitions that read the selected input.
	active, activeBorder, traced color.Color
	// frame outlines the graph and the stage.
	frame color.Color
	// timeline parts.
	track, band, playhead, tick color.Color
}

func paletteFor(context *guigui.Context) palette {
	if context.ColorMode() == ebiten.ColorModeDark {
		return palette{
			edge:       color.NRGBA{0x8a, 0x8a, 0x96, 0xff},
			node:       color.NRGBA{0x33, 0x36, 0x42, 0xff},
			nodeBorder: color.NRGBA{0x55, 0x59, 0x68, 0xff},
			selected:   color.NRGBA{0x5b, 0x9d, 0xf0, 0xff},
			initial:    color.NRGBA{0x4c, 0xc2, 0x8a, 0xff},
			global:     color.NRGBA{0xd0, 0x9a, 0x3c, 0xff},

			active:       color.NRGBA{0x1f, 0x5c, 0x40, 0xff},
			activeBorder: color.NRGBA{0x4c, 0xc2, 0x8a, 0xff},
			traced:       color.NRGBA{0xf0, 0x8a, 0x3c, 0xff},

			frame:    color.NRGBA{0x4a, 0x4e, 0x5c, 0xff},
			track:    color.NRGBA{0x2a, 0x2d, 0x36, 0xff},
			band:     color.NRGBA{0x3d, 0x4a, 0x63, 0xff},
			playhead: color.NRGBA{0x6f, 0xb0, 0xff, 0xff},
			tick:     color.NRGBA{0x9a, 0x9a, 0xa8, 0xff},
		}
	}
	return palette{
		edge:       color.NRGBA{0x77, 0x77, 0x84, 0xff},
		node:       color.NRGBA{0xff, 0xff, 0xff, 0xff},
		nodeBorder: color.NRGBA{0xb4, 0xb8, 0xc4, 0xff},
		selected:   color.NRGBA{0x1f, 0x6f, 0xd0, 0xff},
		initial:    color.NRGBA{0x1e, 0x9c, 0x63, 0xff},
		global:     color.NRGBA{0xb5, 0x7c, 0x1a, 0xff},

		active:       color.NRGBA{0xd4, 0xf3, 0xe2, 0xff},
		activeBorder: color.NRGBA{0x1e, 0x9c, 0x63, 0xff},
		traced:       color.NRGBA{0xd4, 0x6a, 0x10, 0xff},

		frame:    color.NRGBA{0xc2, 0xc6, 0xd0, 0xff},
		track:    color.NRGBA{0xe6, 0xe8, 0xee, 0xff},
		band:     color.NRGBA{0xc3, 0xd6, 0xf2, 0xff},
		playhead: color.NRGBA{0x1f, 0x6f, 0xd0, 0xff},
		tick:     color.NRGBA{0x6a, 0x6e, 0x7a, 0xff},
	}
}

type graphView struct {
	guigui.DefaultWidget

	nodes guigui.WidgetSlice[*graphNode]

	dragIndex  int
	generation int
}

func (g *graphView) model(context *guigui.Context) *Model {
	v, ok := context.Env(g, envKeyModel)
	if !ok {
		return nil
	}
	m, _ := v.(*Model)
	return m
}

func nodeSize(context *guigui.Context) image.Point {
	u := basicwidget.UnitSize(context)
	return image.Pt(6*u, 3*u/2)
}

// nodeRect is where state i sits, in screen coordinates.
func (g *graphView) nodeRect(context *guigui.Context, origin image.Point, m *Model, i int) image.Rectangle {
	p := m.NodePos(i).Add(origin)
	return image.Rectangle{Min: p, Max: p.Add(nodeSize(context))}
}

func (g *graphView) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m := g.model(context)
	if m == nil || m.Machine() == nil {
		g.nodes.SetLen(0)
		return nil
	}
	states := m.Machine().States
	g.nodes.SetLen(len(states))
	for i := range states {
		adder.AddWidget(g.nodes.At(i))
	}
	active := m.ActiveState()
	for i := range states {
		st := &states[i]
		n := g.nodes.At(i)
		n.SetState(st.Name, st.Type == lottie.StateGlobal,
			st.Name == m.Machine().Initial, st.Name == m.SelectedStateName(),
			st.Name == active)
		n.OnSelected(func(context *guigui.Context) {
			m.SelectState(st.Name)
			// Working on a state means watching the machine, not a clip.
			m.ShowMachine()
		})
		n.OnDragged(func(context *guigui.Context, delta image.Point) {
			m.SetNodePos(i, m.NodePos(i).Add(delta))
		})
	}
	g.generation = m.Generation()
	return nil
}

func (g *graphView) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	m := g.model(context)
	if m == nil || m.Machine() == nil {
		return
	}
	origin := widgetBounds.Bounds().Min
	for i := range g.nodes.Len() {
		layouter.LayoutWidget(g.nodes.At(i), g.nodeRect(context, origin, m, i))
	}
}

// WriteStateKey rebuilds the graph when the document changes. The model is
// mutated from handlers that already rebuild, but the generation also moves
// on drags, which must relayout.
func (g *graphView) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	if m := g.model(context); m != nil {
		w.WriteInt(m.Generation())
	}
}

// Measure spans every node, so the edges this widget draws are not clipped
// to the default box and the enclosing panel can scroll to reach a node
// dragged past the viewport.
func (g *graphView) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	size := nodeSize(context)
	w, h := 0, 0
	if m := g.model(context); m != nil && m.Machine() != nil {
		for i := range m.Machine().States {
			p := m.NodePos(i)
			w = max(w, p.X+size.X)
			h = max(h, p.Y+size.Y)
		}
	}
	w, h = w+u, h+u
	// Fill the viewport when the content is smaller, but keep the larger
	// content size when it overflows so the panel scrolls instead of
	// cropping.
	if cw, ok := constraints.FixedWidth(); ok {
		w = max(w, cw)
	}
	if ch, ok := constraints.FixedHeight(); ok {
		h = max(h, ch)
	}
	return image.Pt(w, h)
}

func (g *graphView) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	m := g.model(context)
	if m == nil || m.Machine() == nil {
		return
	}
	pal := paletteFor(context)
	origin := widgetBounds.Bounds().Min
	states := m.Machine().States
	index := map[string]int{}
	for i, st := range states {
		index[st.Name] = i
	}
	u := float32(basicwidget.UnitSize(context))
	traced := m.SelectedInputName()
	// Traced edges are drawn last so they sit on top of the plain ones.
	for _, highlight := range []bool{false, true} {
		for i, st := range states {
			from := g.nodeRect(context, origin, m, i)
			for _, tr := range st.Transitions {
				if TransitionUsesInput(tr, traced) != highlight {
					continue
				}
				j, ok := index[tr.ToState]
				if !ok {
					continue
				}
				clr, width := pal.edge, u/8
				if highlight {
					clr, width = pal.traced, u/5
				}
				if j == i {
					drawSelfLoop(dst, from, width, clr)
					continue
				}
				drawEdge(dst, from, g.nodeRect(context, origin, m, j), width, clr)
			}
		}
	}
}

// drawEdge connects two node rectangles along their centre line, stopping at
// each boundary so the line does not run under the nodes.
func drawEdge(dst *ebiten.Image, from, to image.Rectangle, width float32, clr color.Color) {
	c0, c1 := center(from), center(to)
	p0 := borderPoint(from, c0, c1)
	p1 := borderPoint(to, c1, c0)
	vector.StrokeLine(dst, p0.X, p0.Y, p1.X, p1.Y, width, clr, true)
	drawArrowHead(dst, p0, p1, width, clr)
}

type ptf struct{ X, Y float32 }

func center(r image.Rectangle) ptf {
	return ptf{float32(r.Min.X+r.Max.X) / 2, float32(r.Min.Y+r.Max.Y) / 2}
}

// borderPoint walks from c towards other and returns where it leaves r.
func borderPoint(r image.Rectangle, c, other ptf) ptf {
	dx, dy := other.X-c.X, other.Y-c.Y
	if dx == 0 && dy == 0 {
		return c
	}
	hw := float32(r.Dx()) / 2
	hh := float32(r.Dy()) / 2
	// Scale the direction so it just touches the nearer pair of edges.
	t := float32(math.MaxFloat32)
	if dx != 0 {
		if v := hw / abs32(dx); v < t {
			t = v
		}
	}
	if dy != 0 {
		if v := hh / abs32(dy); v < t {
			t = v
		}
	}
	return ptf{c.X + dx*t, c.Y + dy*t}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// drawArrowHead marks the target end with two short strokes.
func drawArrowHead(dst *ebiten.Image, from, to ptf, width float32, clr color.Color) {
	dx, dy := to.X-from.X, to.Y-from.Y
	l := float32(math.Hypot(float64(dx), float64(dy)))
	if l == 0 {
		return
	}
	dx, dy = dx/l, dy/l
	size := width * 5
	// Rotate the reversed direction by roughly +/-30 degrees.
	const cos, sin = 0.866, 0.5
	bx, by := -dx*size, -dy*size
	vector.StrokeLine(dst, to.X, to.Y, to.X+bx*cos-by*sin, to.Y+bx*sin+by*cos, width, clr, true)
	vector.StrokeLine(dst, to.X, to.Y, to.X+bx*cos+by*sin, to.Y-bx*sin+by*cos, width, clr, true)
}

// drawSelfLoop marks a transition back into the same state.
func drawSelfLoop(dst *ebiten.Image, r image.Rectangle, width float32, clr color.Color) {
	x := float32(r.Max.X)
	y := float32(r.Min.Y)
	s := float32(r.Dy()) / 2
	vector.StrokeLine(dst, x-s, y, x+s/2, y-s, width, clr, true)
	vector.StrokeLine(dst, x+s/2, y-s, x+s, y+s/2, width, clr, true)
	drawArrowHead(dst, ptf{x + s/2, y - s}, ptf{x + s, y + s/2}, width, clr)
}

// ---- node ----

var (
	eventNodeSelected = guigui.GenerateEventKey()
	eventNodeDragged  = guigui.GenerateEventKey()
)

type graphNode struct {
	guigui.DefaultWidget

	label basicwidget.Text

	name     string
	global   bool
	initial  bool
	selected bool
	active   bool

	dragging   bool
	lastCursor image.Point

	items []guigui.LinearLayoutItem
}

func (n *graphNode) SetState(name string, global, initial, selected, active bool) {
	n.name, n.global, n.initial, n.selected = name, global, initial, selected
	n.active = active
}

func (n *graphNode) OnSelected(f func(context *guigui.Context)) {
	guigui.SetEventHandler(n, eventNodeSelected, f)
}

func (n *graphNode) OnDragged(f func(context *guigui.Context, delta image.Point)) {
	guigui.SetEventHandler(n, eventNodeDragged, f)
}

func (n *graphNode) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&n.label)
	text := n.name
	if n.initial {
		text = "▶ " + text
	}
	n.label.SetValue(text)
	n.label.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	n.label.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	// The label must not swallow the click meant for the node.
	context.SetPassthrough(&n.label, true)
	return nil
}

func (n *graphNode) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	n.items = slices.Delete(n.items, 0, len(n.items))
	n.items = append(n.items, guigui.LinearLayoutItem{
		Widget: &n.label,
		Size:   guigui.FlexibleSize(1),
	})
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     n.items,
		Padding:   guigui.Padding{Start: u / 4, End: u / 4},
	}).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (n *graphNode) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	pal := paletteFor(context)
	b := widgetBounds.Bounds()
	x, y := float32(b.Min.X), float32(b.Min.Y)
	w, h := float32(b.Dx()), float32(b.Dy())
	fillColor := pal.node
	if n.active {
		fillColor = pal.active
	}
	vector.DrawFilledRect(dst, x, y, w, h, fillColor, true)

	border := pal.nodeBorder
	width := float32(basicwidget.UnitSize(context)) / 12
	switch {
	// Selection is what you are editing; active is what is playing. Both
	// can be true, and the selection border wins so editing stays legible.
	case n.selected:
		border, width = pal.selected, width*2.5
	case n.active:
		border, width = pal.activeBorder, width*2
	case n.initial:
		border = pal.initial
	case n.global:
		border = pal.global
	}
	vector.StrokeRect(dst, x, y, w, h, width, border, true)
}

func (n *graphNode) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return nodeSize(context)
}

func (n *graphNode) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if widgetBounds.IsHitAtCursor() || n.dragging {
		return ebiten.CursorShapeMove, true
	}
	return 0, false
}

func (n *graphNode) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if n.dragging {
		if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			n.dragging = false
			return guigui.HandleInputByWidget(n)
		}
		cx, cy := ebiten.CursorPosition()
		cur := image.Pt(cx, cy)
		if d := cur.Sub(n.lastCursor); d.X != 0 || d.Y != 0 {
			n.lastCursor = cur
			guigui.DispatchEvent(n, eventNodeDragged, d)
		}
		return guigui.HandleInputByWidget(n)
	}
	if widgetBounds.IsHitAtCursor() && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		n.dragging = true
		cx, cy := ebiten.CursorPosition()
		n.lastCursor = image.Pt(cx, cy)
		guigui.DispatchEvent(n, eventNodeSelected)
		return guigui.HandleInputByWidget(n)
	}
	return guigui.HandleInputResult{}
}
