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

	lottie "github.com/shibukawa/lottie-go"
)

// canvasView is the working surface: it draws the scene through the real
// runtime and either arranges nodes (edit mode) or drives the scene with
// real input (preview mode).
type canvasView struct {
	guigui.DefaultWidget

	dragging   bool
	dragIndex  int
	lastCursor image.Point
}

func (c *canvasView) model(context *guigui.Context) *Model {
	v, ok := context.Env(c, envKeyModel)
	if !ok {
		return nil
	}
	m, _ := v.(*Model)
	return m
}

// canvasTransform maps scene coordinates to this widget's pixels: the
// design box fitted and centered, mirroring what a Contain screen mapping
// would do at runtime.
type canvasTransform struct {
	scale, ox, oy float64
}

func (t canvasTransform) toScene(cx, cy int) (float64, float64) {
	return (float64(cx) - t.ox) / t.scale, (float64(cy) - t.oy) / t.scale
}

func (c *canvasView) transform(m *Model, b image.Rectangle) (canvasTransform, bool) {
	w, h := m.Scene().Size.W, m.Scene().Size.H
	if w <= 0 || h <= 0 {
		return canvasTransform{}, false
	}
	scale := min(float64(b.Dx())/float64(w), float64(b.Dy())/float64(h))
	return canvasTransform{
		scale: scale,
		ox:    float64(b.Min.X) + (float64(b.Dx())-float64(w)*scale)/2,
		oy:    float64(b.Min.Y) + (float64(b.Dy())-float64(h)*scale)/2,
	}, true
}

func (c *canvasView) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m := c.model(context)
	if m == nil {
		return nil
	}
	// Preview mode reads the keyboard without requiring a click-to-focus
	// first; the canvas is the only widget that wants raw keys.
	context.SetButtonInputReceptive(c, m.PreviewMode())
	return nil
}

func (c *canvasView) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	if m := c.model(context); m != nil {
		w.WriteInt(m.Generation())
		w.WriteBool(m.PreviewMode())
	}
}

// Tick advances the scene and, in preview mode, feeds the pointer. The
// animation is a redraw, not a rebuild.
func (c *canvasView) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	m := c.model(context)
	if m == nil {
		return nil
	}
	sp := m.Player()
	if sp == nil {
		return nil
	}
	if m.PreviewMode() {
		if tr, ok := c.transform(m, widgetBounds.Bounds()); ok {
			cx, cy := ebiten.CursorPosition()
			if image.Pt(cx, cy).In(widgetBounds.Bounds()) {
				x, y := tr.toScene(cx, cy)
				sp.Pointer(x, y, ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft))
			}
		}
		sp.Update()
	} else {
		// Edit mode runs on the timeline's transport, which pauses itself
		// when the choreography finishes.
		m.EditTick()
	}
	guigui.RequestRedraw(c)
	return nil
}

// HandleButtonInput drives focus and activation in preview mode, the way
// a game would map its keys: Tab and arrows move, Enter activates, Esc
// cancels.
func (c *canvasView) HandleButtonInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	m := c.model(context)
	if m == nil || !m.PreviewMode() {
		return guigui.HandleInputResult{}
	}
	sp := m.Player()
	if sp == nil {
		return guigui.HandleInputResult{}
	}
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyTab):
		if ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight) {
			sp.MoveFocus(lottie.FocusPrev)
		} else {
			sp.MoveFocus(lottie.FocusNext)
		}
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowUp):
		sp.MoveFocus(lottie.FocusUp)
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowDown):
		sp.MoveFocus(lottie.FocusDown)
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft):
		sp.MoveFocus(lottie.FocusLeft)
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowRight):
		sp.MoveFocus(lottie.FocusRight)
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter), inpututil.IsKeyJustPressed(ebiten.KeySpace):
		sp.Activate()
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		sp.Cancel()
	default:
		return guigui.HandleInputResult{}
	}
	return guigui.HandleInputByWidget(c)
}

// HandlePointingInput selects and drags nodes in edit mode; preview-mode
// pointing is fed from Tick so hover works without button activity.
func (c *canvasView) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	m := c.model(context)
	if m == nil || m.PreviewMode() {
		return guigui.HandleInputResult{}
	}
	tr, ok := c.transform(m, widgetBounds.Bounds())
	if !ok {
		return guigui.HandleInputResult{}
	}
	if c.dragging {
		if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			c.dragging = false
			return guigui.HandleInputByWidget(c)
		}
		cx, cy := ebiten.CursorPosition()
		cur := image.Pt(cx, cy)
		if d := cur.Sub(c.lastCursor); d.X != 0 || d.Y != 0 {
			c.lastCursor = cur
			m.DragNode(c.dragIndex, float64(d.X)/tr.scale, float64(d.Y)/tr.scale)
		}
		return guigui.HandleInputByWidget(c)
	}
	if !widgetBounds.IsHitAtCursor() || !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return guigui.HandleInputResult{}
	}
	cx, cy := ebiten.CursorPosition()
	c.lastCursor = image.Pt(cx, cy)
	sp := m.Player()
	if sp == nil {
		return guigui.HandleInputResult{}
	}
	x, y := tr.toScene(cx, cy)
	if n, ok := editHit(sp, x, y); ok {
		m.SelectNodeByName(n.Name())
		c.dragging, c.dragIndex = true, m.SelectedNodeIndex()
		return guigui.HandleInputByWidget(c)
	}
	m.SelectNode(-1)
	return guigui.HandleInputResult{}
}

// inViewedPhase reports whether the node shows in the phase the canvas
// previews; nodes of other phases stay out of editing entirely.
func inViewedPhase(sp *lottie.ScenePlayer, n *lottie.SceneNodePlayer) bool {
	p := n.Definition().Phase
	return p == "" || p == sp.Phase()
}

// nodeGeoM is the node's local-to-scene transform, the editor-side twin
// of what the runtime draws with.
func nodeGeoM(n *lottie.SceneNodePlayer) ebiten.GeoM {
	tf := n.Transform()
	sx, sy := tf.ScaleX, tf.ScaleY
	if sx == 0 {
		sx = 1
	}
	if sy == 0 {
		sy = 1
	}
	var g ebiten.GeoM
	g.Scale(sx, sy)
	g.Rotate(tf.Rotation * math.Pi / 180)
	g.Translate(tf.X, tf.Y)
	return g
}

// editHit is the edit-mode picking test: unlike the runtime's NodeAt it
// also hits nodes whose entrance has not come yet, so a delayed node can
// still be arranged.
func editHit(sp *lottie.ScenePlayer, x, y float64) (*lottie.SceneNodePlayer, bool) {
	nodes := sp.Nodes()
	for i := len(nodes) - 1; i >= 0; i-- {
		n := nodes[i]
		if !inViewedPhase(sp, n) {
			continue
		}
		x0, y0, w, h := n.LocalRect()
		if w <= 0 || h <= 0 {
			continue
		}
		g := nodeGeoM(n)
		if !g.IsInvertible() {
			continue
		}
		g.Invert()
		lx, ly := g.Apply(x, y)
		if lx >= x0 && lx < x0+w && ly >= y0 && ly < y0+h {
			return n, true
		}
	}
	return nil, false
}

func (c *canvasView) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if c.dragging {
		return ebiten.CursorShapeMove, true
	}
	m := c.model(context)
	if m == nil || m.PreviewMode() || !widgetBounds.IsHitAtCursor() {
		return 0, false
	}
	sp := m.Player()
	tr, ok := c.transform(m, widgetBounds.Bounds())
	if sp == nil || !ok {
		return 0, false
	}
	cx, cy := ebiten.CursorPosition()
	x, y := tr.toScene(cx, cy)
	if _, ok := editHit(sp, x, y); ok {
		return ebiten.CursorShapeMove, true
	}
	return 0, false
}

func (c *canvasView) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	m := c.model(context)
	if m == nil {
		return
	}
	pal := paletteFor(context)
	b := widgetBounds.Bounds()
	vector.DrawFilledRect(dst, float32(b.Min.X), float32(b.Min.Y),
		float32(b.Dx()), float32(b.Dy()), pal.canvasBack, false)
	tr, ok := c.transform(m, b)
	if !ok {
		return
	}
	// The design box, so what lies outside the screen is visible as such.
	w, h := m.Scene().Size.W, m.Scene().Size.H
	vector.StrokeRect(dst, float32(tr.ox), float32(tr.oy),
		float32(float64(w)*tr.scale), float32(float64(h)*tr.scale),
		1, pal.designBox, true)

	sp := m.Player()
	if sp == nil {
		return
	}
	var op lottie.DrawOptions
	op.GeoM.Scale(tr.scale, tr.scale)
	op.GeoM.Translate(tr.ox, tr.oy)
	sp.Draw(dst, &op)

	if m.PreviewMode() {
		return
	}
	// Edit mode arranges in plain scene coordinates (the runtime camera is
	// neutralized), so the camera shows as a framing overlay instead: the
	// region a depth-1 node must occupy to be on screen under the camera.
	c.strokeCameraFrame(dst, m, tr, paletteFor(context).camera)
	// Edit overlays: nodes whose entrance is still to come in grey — the
	// runtime skips them, but they must stay arrangeable — focusable
	// nodes in green, the selection in blue on top.
	u := float32(basicwidget.UnitSize(context))
	sel := m.SelectedNode()
	for _, n := range sp.Nodes() {
		if !inViewedPhase(sp, n) {
			continue
		}
		def := n.Definition()
		if !n.Started() {
			c.strokeNode(dst, n, tr, u/16, pal.designBox)
		}
		if def.Focus.Focusable {
			c.strokeNode(dst, n, tr, u/12, pal.focusable)
		}
		if sel != nil && def.Name == sel.Name {
			c.strokeNode(dst, n, tr, u/8, pal.selected)
		}
	}
}

// strokeCameraFrame outlines what the viewed phase's camera sees: the
// design box pulled back through the inverse camera at depth 1. An
// identity camera draws nothing — the design box already is the frame.
func (c *canvasView) strokeCameraFrame(dst *ebiten.Image, m *Model, tr canvasTransform, clr color.Color) {
	s := m.Scene()
	cam := s.CameraFor(m.ViewPhase())
	if cam.X == 0 && cam.Y == 0 && cam.ZoomFactor() == 1 && cam.Rotation == 0 {
		return
	}
	g := cam.GeoM(s.Size.W, s.Size.H, 1)
	if !g.IsInvertible() {
		return
	}
	g.Invert()
	g.Scale(tr.scale, tr.scale)
	g.Translate(tr.ox, tr.oy)
	w, h := float64(s.Size.W), float64(s.Size.H)
	pts := [4][2]float64{{0, 0}, {w, 0}, {w, h}, {0, h}}
	var xs, ys [4]float32
	for i, pt := range pts {
		x, y := g.Apply(pt[0], pt[1])
		xs[i], ys[i] = float32(x), float32(y)
	}
	for i := range 4 {
		j := (i + 1) % 4
		vector.StrokeLine(dst, xs[i], ys[i], xs[j], ys[j], 1.5, clr, true)
	}
}

// strokeNode outlines a node's hit box on the canvas, rotation included.
func (c *canvasView) strokeNode(dst *ebiten.Image, n *lottie.SceneNodePlayer, tr canvasTransform, width float32, clr color.Color) {
	x0, y0, w, h := n.LocalRect()
	if w <= 0 || h <= 0 {
		return
	}
	g := nodeGeoM(n)
	g.Scale(tr.scale, tr.scale)
	g.Translate(tr.ox, tr.oy)
	pts := [4][2]float64{{x0, y0}, {x0 + w, y0}, {x0 + w, y0 + h}, {x0, y0 + h}}
	var xs, ys [4]float32
	for i, pt := range pts {
		x, y := g.Apply(pt[0], pt[1])
		xs[i], ys[i] = float32(x), float32(y)
	}
	for i := range 4 {
		j := (i + 1) % 4
		vector.StrokeLine(dst, xs[i], ys[i], xs[j], ys[j], width, clr, true)
	}
}

func (c *canvasView) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	return image.Pt(8*u, 6*u)
}
