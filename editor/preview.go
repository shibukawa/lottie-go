package main

import (
	"image"
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	lottie "github.com/shibukawa/lottie-go"
)

// previewPane shows either the machine, driven by the same interpreter a
// game uses, or one clip on its own. Everything that drives it lives in the
// Inputs table, so this pane is only the stage and its status.
type previewPane struct {
	guigui.DefaultWidget

	stage         previewStage
	timeline      timelineView
	collision     collisionPanel
	stateLabel    basicwidget.Text
	hint          basicwidget.Text
	backToMachine basicwidget.Button

	items []guigui.LinearLayoutItem
}

func (p *previewPane) model(context *guigui.Context) *Model {
	v, ok := context.Env(p, envKeyModel)
	if !ok {
		return nil
	}
	m, _ := v.(*Model)
	return m
}

func (p *previewPane) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m := p.model(context)
	if m == nil {
		return nil
	}
	adder.AddWidget(&p.stage)
	adder.AddWidget(&p.timeline)
	adder.AddWidget(&p.collision)
	adder.AddWidget(&p.stateLabel)
	adder.AddWidget(&p.hint)
	// Only offered while a clip has taken the stage.
	if m.PreviewClip().Anim != "" {
		adder.AddWidget(&p.backToMachine)
	}

	p.stateLabel.SetValue(m.PreviewLabel())
	p.stateLabel.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	hint := ""
	switch {
	case m.PreviewClip().Anim != "":
		hint = "playing one clip; the machine is paused"
	case m.PreviewErr() != nil:
		hint = m.PreviewErr().Error()
	case m.Preview() == nil:
		hint = "Select or create a state machine."
	default:
		if m.PreviewStale() {
			hint = "edited — Restart to apply"
		}
		if u := m.Preview().UnsupportedFeatures(); len(u) > 0 {
			if hint != "" {
				hint += "   "
			}
			hint += "skipped: " + u[0]
		}
	}
	p.hint.SetValue(hint)
	p.hint.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	p.hint.SetScale(0.85)

	p.backToMachine.SetText("Back to machine")
	p.backToMachine.OnDown(func(context *guigui.Context) { m.ShowMachine() })
	return nil
}

// WriteStateKey rebuilds when the machine moves to another state, so the
// label and the graph highlight follow playback. The per-frame animation is
// a redraw, not a rebuild; previewStage requests that itself.
func (p *previewPane) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	m := p.model(context)
	if m == nil {
		return
	}
	w.WriteInt(m.Generation())
	w.WriteString(m.ActiveState())
	w.WriteString(m.PreviewClip().Anim + "/" + m.PreviewClip().Segment)
}

func (p *previewPane) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	m := p.model(context)

	p.items = slices.Delete(p.items, 0, len(p.items))
	p.items = append(p.items,
		guigui.LinearLayoutItem{Widget: &p.stage, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &p.timeline},
		guigui.LinearLayoutItem{Widget: &p.collision},
		guigui.LinearLayoutItem{Widget: &p.stateLabel, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &p.hint, Size: guigui.FixedSize(u)},
	)
	if m != nil && m.PreviewClip().Anim != "" {
		p.items = append(p.items, guigui.LinearLayoutItem{
			Widget: &p.backToMachine, Size: guigui.FixedSize(u)})
	}
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     p.items,
		Gap:       u / 4,
		Padding:   guigui.Padding{Start: u / 2, Top: u / 2, End: u / 2, Bottom: u / 2},
	}).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

// previewStage renders whatever the preview is showing, scaled to fit. The
// collision overlay is drawn and dragged here: what you grab is the shape
// at the pixel you see. The overlay visibility toggles sit in the stage's
// own top-right corner — they change what this widget shows, so they live
// on it, apart from the authoring strip below.
type previewStage struct {
	guigui.DefaultWidget

	hitLbl    basicwidget.Text
	hitCheck  basicwidget.Checkbox
	bodyLbl   basicwidget.Text
	bodyCheck basicwidget.Checkbox
	sockLbl   basicwidget.Text
	sockCheck basicwidget.Checkbox

	dragMode   stageDrag
	dragKind   stageDragKind
	lastCursor image.Point
}

// stageDragKind says what the stage drag is editing.
type stageDragKind int

const (
	dragHitbox stageDragKind = iota
	dragBody
	dragSocket
)

func (s *previewStage) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m := s.model(context)
	if m == nil {
		return nil
	}
	if m.ResolvEnabled() {
		adder.AddWidget(&s.hitLbl)
		adder.AddWidget(&s.hitCheck)
	}
	if m.CPEnabled() {
		adder.AddWidget(&s.bodyLbl)
		adder.AddWidget(&s.bodyCheck)
	}
	adder.AddWidget(&s.sockLbl)
	adder.AddWidget(&s.sockCheck)

	label(&s.hitLbl, "hit")
	label(&s.bodyLbl, "body")
	label(&s.sockLbl, "sock")
	for _, l := range []*basicwidget.Text{&s.hitLbl, &s.bodyLbl, &s.sockLbl} {
		l.SetScale(0.8)
		l.SetHorizontalAlign(basicwidget.HorizontalAlignEnd)
		context.SetPassthrough(l, true)
	}
	s.hitCheck.SetValue(m.ShowHitboxes())
	s.hitCheck.OnValueChanged(func(context *guigui.Context, v bool) { m.SetShowHitboxes(v) })
	s.bodyCheck.SetValue(m.ShowBody())
	s.bodyCheck.OnValueChanged(func(context *guigui.Context, v bool) { m.SetShowBody(v) })
	s.sockCheck.SetValue(m.ShowSockets())
	s.sockCheck.OnValueChanged(func(context *guigui.Context, v bool) { m.SetShowSockets(v) })
	return nil
}

// Layout stacks the toggle groups in from the top-right corner.
func (s *previewStage) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	m := s.model(context)
	if m == nil {
		return
	}
	u := basicwidget.UnitSize(context)
	b := widgetBounds.Bounds()
	x := b.Max.X - u/4
	y := b.Min.Y + u/4
	place := func(lbl *basicwidget.Text, chk *basicwidget.Checkbox, lblW int) {
		x -= u
		layouter.LayoutWidget(chk, image.Rect(x, y, x+u, y+u))
		x -= lblW
		layouter.LayoutWidget(lbl, image.Rect(x, y, x+lblW, y+u))
		x -= u / 4
	}
	place(&s.sockLbl, &s.sockCheck, 3*u/2)
	if m.CPEnabled() {
		place(&s.bodyLbl, &s.bodyCheck, 3*u/2)
	}
	if m.ResolvEnabled() {
		place(&s.hitLbl, &s.hitCheck, u)
	}
}

func (s *previewStage) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	if m := s.model(context); m != nil {
		w.WriteInt(m.Generation())
	}
}

type stageDrag int

const (
	dragNone stageDrag = iota
	dragMove
	dragResize
)

// transform maps animation coordinates to this widget's pixels, mirroring
// Draw's fit-and-center math so overlays and hit tests agree with the
// rendering.
func (s *previewStage) transform(m *Model, b image.Rectangle) (stageTransform, bool) {
	anim := m.PreviewAnimation()
	if anim == nil {
		return stageTransform{}, false
	}
	aw, ah := anim.Size()
	if aw <= 0 || ah <= 0 {
		return stageTransform{}, false
	}
	scale := min(float64(b.Dx())/float64(aw), float64(b.Dy())/float64(ah))
	return stageTransform{
		scale: scale,
		ox:    float64(b.Min.X) + (float64(b.Dx())-float64(aw)*scale)/2,
		oy:    float64(b.Min.Y) + (float64(b.Dy())-float64(ah)*scale)/2,
	}, true
}

func (s *previewStage) model(context *guigui.Context) *Model {
	v, ok := context.Env(s, envKeyModel)
	if !ok {
		return nil
	}
	m, _ := v.(*Model)
	return m
}

// Tick advances playback. The frame changes without any tree change, so it
// asks for a redraw rather than a rebuild.
func (s *previewStage) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	m := s.model(context)
	if m == nil {
		return nil
	}
	m.PreviewUpdate()
	guigui.RequestRedraw(s)
	return nil
}

func (s *previewStage) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	m := s.model(context)
	if m == nil {
		return
	}
	anim := m.PreviewAnimation()
	if anim == nil {
		return
	}
	aw, ah := anim.Size()
	if aw <= 0 || ah <= 0 {
		return
	}
	b := widgetBounds.Bounds()
	tr, ok := s.transform(m, b)
	if !ok {
		return
	}
	var op lottie.DrawOptions
	op.GeoM.Scale(tr.scale, tr.scale)
	op.GeoM.Translate(tr.ox, tr.oy)
	m.PreviewDraw(dst, &op)
	if m.OverlayVisible() {
		drawCollisionOverlay(dst, m, tr, float32(basicwidget.UnitSize(context)))
	}
}

// HandlePointingInput selects and drags collision shapes on the stage:
// inside a shape moves it, the white grip resizes, empty stage deselects.
func (s *previewStage) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	m := s.model(context)
	if m == nil || !m.OverlayVisible() {
		return guigui.HandleInputResult{}
	}
	tr, ok := s.transform(m, widgetBounds.Bounds())
	if !ok {
		return guigui.HandleInputResult{}
	}
	if s.dragMode != dragNone {
		if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			s.dragMode = dragNone
			return guigui.HandleInputByWidget(s)
		}
		cx, cy := ebiten.CursorPosition()
		cur := image.Pt(cx, cy)
		d := cur.Sub(s.lastCursor)
		if d.X == 0 && d.Y == 0 {
			return guigui.HandleInputByWidget(s)
		}
		s.lastCursor = cur
		dx, dy := float64(d.X)/tr.scale, float64(d.Y)/tr.scale
		switch {
		case s.dragMode == dragResize && s.dragKind == dragBody:
			m.DragCPShapeHandle(dx, dy)
		case s.dragMode == dragResize:
			m.DragHitboxHandle(dx, dy)
		case s.dragKind == dragBody:
			m.DragCPShape(dx, dy)
		case s.dragKind == dragSocket:
			m.DragSocket(dx, dy)
		default:
			m.DragHitbox(dx, dy)
		}
		return guigui.HandleInputByWidget(s)
	}
	if !widgetBounds.IsHitAtCursor() || !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return guigui.HandleInputResult{}
	}
	cx, cy := ebiten.CursorPosition()
	s.lastCursor = image.Pt(cx, cy)
	// The grip belongs to the current selection, so it is tested before the
	// shapes: a tiny handle would be unreachable under a big sibling.
	if hx, hy, ok := handleAt(m, tr); ok {
		half := handleSize(float32(basicwidget.UnitSize(context)))/2 + 2
		if abs32(float32(cx)-hx) <= half && abs32(float32(cy)-hy) <= half {
			s.dragMode = dragResize
			s.dragKind = dragHitbox
			if m.SelectedCPShape() != nil {
				s.dragKind = dragBody
			}
			return guigui.HandleInputByWidget(s)
		}
	}
	ax, ay := tr.toAnim(cx, cy)
	// Sockets draw topmost, so they hit-test first; their crosses are
	// small enough that a generous radius costs nothing.
	if i, ok := hitTestSockets(m, ax, ay, float64(basicwidget.UnitSize(context))/2/tr.scale); ok {
		m.SelectSocket(i)
		s.dragMode, s.dragKind = dragMove, dragSocket
		return guigui.HandleInputByWidget(s)
	}
	if i, ok := hitTestBoxes(m, ax, ay); ok {
		m.SelectHitbox(i)
		s.dragMode, s.dragKind = dragMove, dragHitbox
		return guigui.HandleInputByWidget(s)
	}
	if i, ok := hitTestCPShapes(m, ax, ay); ok {
		m.SelectCPShape(i)
		s.dragMode, s.dragKind = dragMove, dragBody
		return guigui.HandleInputByWidget(s)
	}
	if m.SelectedHitboxIndex() >= 0 || m.SelectedCPShapeIndex() >= 0 {
		m.SelectHitbox(-1)
		m.SelectCPShape(-1)
	}
	return guigui.HandleInputResult{}
}

func (s *previewStage) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	switch s.dragMode {
	case dragMove:
		return ebiten.CursorShapeMove, true
	case dragResize:
		return ebiten.CursorShapeNWSEResize, true
	}
	m := s.model(context)
	if m == nil || !m.OverlayVisible() || !widgetBounds.IsHitAtCursor() {
		return 0, false
	}
	tr, ok := s.transform(m, widgetBounds.Bounds())
	if !ok {
		return 0, false
	}
	cx, cy := ebiten.CursorPosition()
	if hx, hy, ok := handleAt(m, tr); ok {
		half := handleSize(float32(basicwidget.UnitSize(context)))/2 + 2
		if abs32(float32(cx)-hx) <= half && abs32(float32(cy)-hy) <= half {
			return ebiten.CursorShapeNWSEResize, true
		}
	}
	ax, ay := tr.toAnim(cx, cy)
	if _, ok := hitTestSockets(m, ax, ay, float64(basicwidget.UnitSize(context))/2/tr.scale); ok {
		return ebiten.CursorShapeMove, true
	}
	if _, ok := hitTestBoxes(m, ax, ay); ok {
		return ebiten.CursorShapeMove, true
	}
	if _, ok := hitTestCPShapes(m, ax, ay); ok {
		return ebiten.CursorShapeMove, true
	}
	return 0, false
}

func (s *previewStage) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	return image.Pt(8*u, 6*u)
}
