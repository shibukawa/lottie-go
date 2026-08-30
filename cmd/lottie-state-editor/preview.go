package main

import (
	"image"
	"math"
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
// at the pixel you see. What shows follows the strip's active tab — the
// Segment tab is the clean, undecorated preview — so the stage carries no
// toggles of its own.
type previewStage struct {
	guigui.DefaultWidget

	dragMode   stageDrag
	dragKind   stageDragKind
	lastCursor image.Point

	// The vertex or handle a shape drag is carrying (shapeui.go).
	shapeDragIdx int

	// panMoved separates a pan from a click that hit nothing: both start the
	// same way, and only the release can tell them apart.
	panMoved bool

	// The onion skin renders through its own player. Borrowing the stage's
	// would mean moving the playhead mid-draw, and SetFrame past the out
	// point stops playback as a side effect — a display option must not be
	// able to pause the clip.
	ghost     *lottie.Player
	ghostAnim *lottie.Animation
}

// ghostPlayer is the paused, non-looping player the onion skin draws with.
// It is rebuilt only when the animation behind the stage is replaced, which
// an edit does on every drag step.
func (s *previewStage) ghostPlayer(anim *lottie.Animation) *lottie.Player {
	if s.ghostAnim != anim {
		s.ghost = anim.NewPlayer()
		s.ghost.Pause()
		s.ghostAnim = anim
	}
	return s.ghost
}

// drawOnionSkin paints the neighbouring keyframes under the current one.
// Earlier keys go cool and later ones warm, so a limb's direction of travel
// reads at a glance instead of having to be worked out.
func (s *previewStage) drawOnionSkin(dst *ebiten.Image, m *Model, anim *lottie.Animation, base *lottie.DrawOptions) {
	ghosts := m.OnionGhosts()
	if len(ghosts) == 0 {
		return
	}
	p := s.ghostPlayer(anim)
	for _, g := range ghosts {
		op := *base
		// Pushed harder than looks right on paper: at this alpha the result
		// is mostly the white stage, so a gentle tint composites away to
		// nothing and both ghosts read as the same grey.
		if g.next {
			op.ColorScale.Scale(1, 0.58, 0.28, 1)
		} else {
			op.ColorScale.Scale(0.38, 0.6, 1, 1)
		}
		op.ColorScale.ScaleAlpha(onionAlpha)
		p.SetFrame(g.frame)
		p.Draw(dst, &op)
	}
}

// stageDragKind says what the stage drag is editing.
type stageDragKind int

const (
	dragHitbox stageDragKind = iota
	dragBody
	dragSocket
	dragPosePart
	dragPoseJoint
	dragShapeVertex
	dragShapeHandleIn
	dragShapeHandleOut
	dragShapeGradS
	dragShapeGradE
	dragShapeGradBoth
)

type stageDrag int

const (
	dragNone stageDrag = iota
	dragMove
	dragResize
	dragPan
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
	scale := stageFitScale(b, aw, ah) * m.StageZoom()
	panX, panY := m.StagePan()
	return stageTransform{
		scale: scale,
		ox:    float64(b.Min.X) + (float64(b.Dx())-float64(aw)*scale)/2 + panX,
		oy:    float64(b.Min.Y) + (float64(b.Dy())-float64(ah)*scale)/2 + panY,
	}, true
}

// stageFitScale is the magnification that shows the whole clip, which is
// what a zoom of 1 means.
func stageFitScale(b image.Rectangle, aw, ah int) float64 {
	return min(float64(b.Dx())/float64(aw), float64(b.Dy())/float64(ah))
}

// zoomAt magnifies about a point on screen, so whatever is under the cursor
// stays under it. Zooming about the centre instead would push the joint
// being worked on off the pane the moment it got close enough to see.
func (s *previewStage) zoomAt(m *Model, b image.Rectangle, cx, cy int, factor float64) {
	anim := m.PreviewAnimation()
	if anim == nil {
		return
	}
	aw, ah := anim.Size()
	if aw <= 0 || ah <= 0 {
		return
	}
	tr, ok := s.transform(m, b)
	if !ok {
		return
	}
	next := min(max(m.StageZoom()*factor, stageZoomMin), stageZoomMax)
	if next == m.StageZoom() {
		return
	}
	// The animation point under the cursor, and where the new scale would
	// otherwise put it.
	ax, ay := tr.toAnim(cx, cy)
	scale := stageFitScale(b, aw, ah) * next
	baseX := float64(b.Min.X) + (float64(b.Dx())-float64(aw)*scale)/2
	baseY := float64(b.Min.Y) + (float64(b.Dy())-float64(ah)*scale)/2
	m.SetStageView(next, float64(cx)-ax*scale-baseX, float64(cy)-ay*scale-baseY)
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
	// Ghosts go under: where the current pose covers one, the current pose
	// is what should be visible.
	s.drawOnionSkin(dst, m, anim, &op)
	m.PreviewDraw(dst, &op)
	u := float32(basicwidget.UnitSize(context))
	if m.ShowRig() {
		// Over the artwork, not under it: the rig is a diagram of what the
		// drawing is doing, and a diagram hidden behind its subject is no
		// use.
		drawRigOverlay(dst, m, tr, u)
	}
	switch {
	case m.PosesVisible():
		drawPoseOverlay(dst, m, tr, u)
	case m.ShapesVisible():
		drawShapeOverlay(dst, m, tr, u)
	case m.OverlayVisible():
		drawCollisionOverlay(dst, m, tr, u)
	}
}

// HandlePointingInput selects and drags collision shapes on the stage:
// inside a shape moves it, the white grip resizes, empty stage deselects.
func (s *previewStage) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	m := s.model(context)
	if m == nil {
		return guigui.HandleInputResult{}
	}
	b := widgetBounds.Bounds()
	// Zooming and panning are how the stage is looked at, not what it edits,
	// so they work on every tab — the undecorated Segment one included.
	if widgetBounds.IsHitAtCursor() && s.dragMode == dragNone {
		if _, wy := adjustedWheel(); wy != 0 {
			cx, cy := ebiten.CursorPosition()
			s.zoomAt(m, b, cx, cy, math.Pow(stageZoomStep, wy))
			return guigui.HandleInputByWidget(s)
		}
	}
	if s.dragMode == dragPan {
		if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			s.dragMode = dragNone
			// A press that never moved was a click on empty stage, which is
			// still how a selection is dropped.
			if !s.panMoved {
				s.deselectAll(m)
			}
			return guigui.HandleInputByWidget(s)
		}
		cx, cy := ebiten.CursorPosition()
		if d := image.Pt(cx, cy).Sub(s.lastCursor); d.X != 0 || d.Y != 0 {
			s.lastCursor = image.Pt(cx, cy)
			s.panMoved = true
			px, py := m.StagePan()
			m.SetStageView(m.StageZoom(), px+float64(d.X), py+float64(d.Y))
		}
		return guigui.HandleInputByWidget(s)
	}
	if !m.OverlayVisible() {
		// No overlay to hit-test, but the view still drags.
		return s.pressToPan(widgetBounds)
	}
	tr, ok := s.transform(m, b)
	if !ok {
		return guigui.HandleInputResult{}
	}
	if s.dragMode != dragNone {
		if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			s.dragMode = dragNone
			// A whole swing is one thing to take back.
			m.EndPoseEdit()
			return guigui.HandleInputByWidget(s)
		}
		cx, cy := ebiten.CursorPosition()
		cur := image.Pt(cx, cy)
		d := cur.Sub(s.lastCursor)
		if d.X == 0 && d.Y == 0 {
			return guigui.HandleInputByWidget(s)
		}
		prev := s.lastCursor
		s.lastCursor = cur
		dx, dy := float64(d.X)/tr.scale, float64(d.Y)/tr.scale
		switch s.dragKind {
		case dragPosePart:
			// Rotation needs where the cursor was and is, not how far it
			// moved: the angle is measured about the joint.
			fx, fy := tr.toAnim(prev.X, prev.Y)
			tx, ty := tr.toAnim(cur.X, cur.Y)
			m.RotatePosePart(fx, fy, tx, ty)
			return guigui.HandleInputByWidget(s)
		case dragPoseJoint:
			m.MovePosePart(dx, dy)
			return guigui.HandleInputByWidget(s)
		case dragShapeVertex, dragShapeHandleIn, dragShapeHandleOut,
			dragShapeGradS, dragShapeGradE, dragShapeGradBoth:
			s.dragShapeStep(m, dx, dy)
			return guigui.HandleInputByWidget(s)
		}
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
	if m.PosesVisible() {
		return s.handlePoseInput(context, m, tr, cx, cy)
	}
	if m.ShapesVisible() {
		return s.handleShapeInput(context, m, tr, cx, cy)
	}
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
	return s.beginPan()
}

// pressToPan starts a pan from a press anywhere on the stage. It is the
// whole input story for a tab with no overlay of its own.
func (s *previewStage) pressToPan(widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if !widgetBounds.IsHitAtCursor() || !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return guigui.HandleInputResult{}
	}
	cx, cy := ebiten.CursorPosition()
	s.lastCursor = image.Pt(cx, cy)
	return s.beginPan()
}

// beginPan arms a pan drag on a press that hit nothing. Whether it turns out
// to be a pan or a click that clears the selection is decided on release, by
// whether the cursor moved at all.
func (s *previewStage) beginPan() guigui.HandleInputResult {
	s.dragMode, s.panMoved = dragPan, false
	return guigui.HandleInputByWidget(s)
}

// deselectAll drops whatever the stage had selected, which is what a click
// on empty stage means in every tab.
func (s *previewStage) deselectAll(m *Model) {
	if m.SelectedHitboxIndex() >= 0 || m.SelectedCPShapeIndex() >= 0 {
		m.SelectHitbox(-1)
		m.SelectCPShape(-1)
	}
	if m.SelectedPosePart() >= 0 {
		m.SelectPosePart(-1)
	}
	if _, ok := m.SelectedShapeNode(); ok {
		m.SelectShapeNode(nil)
	}
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
	if m.PosesVisible() {
		return poseCursorShape(m, tr, float32(basicwidget.UnitSize(context)), cx, cy)
	}
	if m.ShapesVisible() {
		return shapeCursorShape(m, tr, float32(basicwidget.UnitSize(context)), cx, cy)
	}
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
