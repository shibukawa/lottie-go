package main

import (
	"image/color"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// The pose overlay on the stage: the selected part outlined, and the joint
// it turns about marked. Dragging inside the outline swings the part around
// that joint, which is the gesture the rig is built for — the parts are a
// skeleton of rotations, and everything else about them is fixed.
//
// The overlay only offers an edit while the playhead is parked on a
// keyframe. Between keys the outline is drawn faintly and nothing responds,
// because a value written there would land at a frame the clip's other
// tracks have no key at.

// poseColor marks the part being posed. It sits outside the collision tag
// palette on purpose: this is not a judgement volume, it is the artwork.
var (
	poseColor     = color.NRGBA{0xf0, 0x8a, 0x3c, 0xff}
	poseIdleColor = color.NRGBA{0xf0, 0x8a, 0x3c, 0x66}
	posePivotFill = color.NRGBA{0xff, 0xff, 0xff, 0xff}
	posePivotEdge = color.NRGBA{0x20, 0x20, 0x28, 0xff}
)

// drawPoseOverlay outlines the selected part and marks its joint.
func drawPoseOverlay(dst *ebiten.Image, m *Model, tr stageTransform, u float32) {
	part := m.SelectedPosePart()
	if part < 0 {
		return
	}
	q, ok := m.PosePartQuad(part)
	if !ok {
		return
	}
	_, editable := m.SelectedPoseKey()
	clr := poseIdleColor
	stroke := max(1, u/24)
	if editable {
		clr = poseColor
		stroke = max(2, u/12)
	}
	for i := range q.pts {
		a, b := q.pts[i], q.pts[(i+1)%4]
		ax, ay := tr.toScreen(a[0], a[1])
		bx, by := tr.toScreen(b[0], b[1])
		vector.StrokeLine(dst, ax, ay, bx, by, stroke, clr, true)
	}
	if !editable {
		return
	}
	// The joint is also the grip that moves the part, the way the collision
	// shapes carry a resize grip: one mark, one alternative gesture.
	px, py := tr.toScreen(q.pivot[0], q.pivot[1])
	r := handleSize(u) / 2
	vector.StrokeLine(dst, px-r, py, px+r, py, max(1, u/16), clr, true)
	vector.StrokeLine(dst, px, py-r, px, py+r, max(1, u/16), clr, true)
	vector.DrawFilledCircle(dst, px, py, r/2, posePivotFill, true)
	vector.StrokeCircle(dst, px, py, r/2, max(1, u/16), posePivotEdge, true)
}

// posePivotAt returns the joint grip of the selected part in screen
// coordinates, when there is one to grab.
func posePivotAt(m *Model, tr stageTransform) (float32, float32, bool) {
	part := m.SelectedPosePart()
	if part < 0 {
		return 0, 0, false
	}
	if _, ok := m.SelectedPoseKey(); !ok {
		return 0, 0, false
	}
	q, ok := m.PosePartQuad(part)
	if !ok {
		return 0, 0, false
	}
	x, y := tr.toScreen(q.pivot[0], q.pivot[1])
	return x, y, true
}

// handlePoseInput is the stage's pose branch: the joint grip moves the part,
// the outline swings it, empty stage deselects. Picking works whether or not
// a key is selected — seeing which part is which is worth a click on its own
// — but only a parked playhead starts a drag.
func (s *previewStage) handlePoseInput(context *guigui.Context, m *Model, tr stageTransform, cx, cy int) guigui.HandleInputResult {
	u := float32(basicwidget.UnitSize(context))
	// The grip sits inside the outline it belongs to, so it is tested first
	// or it would never be reachable.
	if px, py, ok := posePivotAt(m, tr); ok && m.PosePartDraggable() {
		half := handleSize(u)/2 + 2
		if abs32(float32(cx)-px) <= half && abs32(float32(cy)-py) <= half {
			s.dragMode, s.dragKind = dragMove, dragPoseJoint
			m.BeginPoseEdit()
			return guigui.HandleInputByWidget(s)
		}
	}
	ax, ay := tr.toAnim(cx, cy)
	if i, ok := m.PosePartAt(ax, ay); ok {
		m.SelectPosePart(i)
		// Picking always works, so the pane can explain a part it cannot
		// drag; the drag itself needs the part and its parent to be
		// addressable by name, or it would write into the wrong space.
		_, editable := m.SelectedPoseKey()
		if editable && m.PosePartDraggable() {
			s.dragMode, s.dragKind = dragMove, dragPosePart
			m.BeginPoseEdit()
		}
		return guigui.HandleInputByWidget(s)
	}
	return s.beginPan()
}

// poseCursorShape reports the cursor over the pose overlay: a move cursor on
// the joint grip, a grab elsewhere on a part that is ready to swing.
func poseCursorShape(m *Model, tr stageTransform, u float32, cx, cy int) (ebiten.CursorShapeType, bool) {
	if px, py, ok := posePivotAt(m, tr); ok {
		half := handleSize(u)/2 + 2
		if abs32(float32(cx)-px) <= half && abs32(float32(cy)-py) <= half {
			return ebiten.CursorShapeMove, true
		}
	}
	ax, ay := tr.toAnim(cx, cy)
	if _, ok := m.PosePartAt(ax, ay); ok {
		return ebiten.CursorShapePointer, true
	}
	return 0, false
}

// rig colours follow the onion skin's: the frame being edited in the pose
// colour, its neighbours cool behind and warm ahead, so a joint that
// travelled reads as a line between two dots.
var (
	rigPrevColor = color.NRGBA{0x4a, 0x8a, 0xe0, 0xcc}
	rigNextColor = color.NRGBA{0xe0, 0x82, 0x3a, 0xcc}
)

// drawRigOverlay paints the skeleton the artwork hides: a dot per joint and
// a bone from each to its parent's.
func drawRigOverlay(dst *ebiten.Image, m *Model, tr stageTransform, u float32) {
	for _, g := range m.OnionGhosts() {
		clr := rigPrevColor
		if g.next {
			clr = rigNextColor
		}
		drawRigAt(dst, m, tr, u, g.frame, clr, false)
	}
	drawRigAt(dst, m, tr, u, m.stageFrame(), poseColor, true)
}

func drawRigAt(dst *ebiten.Image, m *Model, tr stageTransform, u float32, frame float64, clr color.NRGBA, solid bool) {
	joints := m.RigJoints(frame)
	if len(joints) == 0 {
		return
	}
	bone := max(1, u/20)
	r := max(2, u/10)
	if !solid {
		bone = max(1, u/28)
		r = max(1.5, u/14)
	}
	// A bone is drawn only where the parent has one child, which is what a
	// limb segment is: shoulder to elbow, hip to knee. The torso has five
	// children hanging off it at five different points, and joining each to
	// its single hip joint draws a starburst that says nothing — a hub is
	// honestly drawn as a hub, which is to say as its joint alone.
	//
	// The ghosts get dots and no bones. Three skeletons of lines over one
	// drawing is a thicket; three sets of dots still say where each joint
	// was, which is the whole question the ghosts are there to answer.
	if solid {
		kids := make([]int, len(joints))
		for _, j := range joints {
			if j.parent >= 0 {
				kids[j.parent]++
			}
		}
		for _, j := range joints {
			if j.parent < 0 || kids[j.parent] != 1 {
				continue
			}
			p := joints[j.parent]
			ax, ay := tr.toScreen(p.at[0], p.at[1])
			bx, by := tr.toScreen(j.at[0], j.at[1])
			vector.StrokeLine(dst, ax, ay, bx, by, bone, clr, true)
		}
	}
	sel := m.SelectedPosePart()
	for _, j := range joints {
		x, y := tr.toScreen(j.at[0], j.at[1])
		vector.DrawFilledCircle(dst, x, y, r, clr, true)
		// The selected part's joint is ringed, so the thing being posed is
		// findable in a skeleton of a dozen identical dots.
		if solid && j.layer == sel {
			vector.StrokeCircle(dst, x, y, r*2, max(1, u/16), clr, true)
		}
	}
}
