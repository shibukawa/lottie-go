package main

import (
	"image/color"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// The shape overlay on the stage: the selected geometry outlined, its
// vertices and bezier handles as grips, and — for a gradient — the Flash
// transform gizmo. The pen draws its in-progress path here too.
//
// Like the pose overlay, grips only respond while an edit would land:
// static values anywhere, keyed ones on a parked key.

var (
	shapeColor     = color.NRGBA{0x3c, 0x8a, 0xf0, 0xff}
	shapeIdleColor = color.NRGBA{0x3c, 0x8a, 0xf0, 0x66}
	shapeVertFill  = color.NRGBA{0xff, 0xff, 0xff, 0xff}
	shapeVertEdge  = color.NRGBA{0x20, 0x20, 0x28, 0xff}
	shapeHandleClr = color.NRGBA{0x8a, 0x64, 0xd8, 0xff}
	penColor       = color.NRGBA{0xe0, 0x58, 0x40, 0xff}
)

// shapeVertexScreen maps the selected path's control points to screen
// coordinates: the vertices, and each vertex's in/out handle ends.
func shapeVertexScreen(m *Model, tr stageTransform) (v, in, out [][2]float32, ok bool) {
	n, okN := m.SelectedShapeNode()
	if !okN || n.ty != "sh" {
		return nil, nil, nil, false
	}
	p, okP := m.ShapePath()
	if !okP {
		return nil, nil, nil, false
	}
	g, okG := m.shapeSpaceMatrix(n.layer, n.path, m.stageFrame())
	if !okG {
		return nil, nil, nil, false
	}
	conv := func(x, y float64) [2]float32 {
		ax, ay := g.Apply(x, y)
		sx, sy := tr.toScreen(ax, ay)
		return [2]float32{sx, sy}
	}
	for i := range p.v {
		v = append(v, conv(p.v[i][0], p.v[i][1]))
		in = append(in, conv(p.v[i][0]+p.i[i][0], p.v[i][1]+p.i[i][1]))
		out = append(out, conv(p.v[i][0]+p.o[i][0], p.v[i][1]+p.o[i][1]))
	}
	return v, in, out, true
}

// shapeEditLive reports whether grips should respond — and show at all:
// the members a drag writes are static, or hold a key under the parked
// playhead. Between the keys of an animated shape the corner markers stay
// hidden rather than offering a drag that would only be refused.
func shapeEditLive(m *Model) bool {
	n, ok := m.SelectedShapeNode()
	if !ok || m.Viewer() {
		return false
	}
	switch n.ty {
	case "sh":
		return m.ShapePathWritable()
	case "rc", "el":
		return m.ShapeMemberWritable("p") && m.ShapeMemberWritable("s")
	case "sr":
		return m.ShapeMemberWritable("p") && m.ShapeMemberWritable("or")
	case "gf", "gs":
		return m.ShapeMemberWritable("s") && m.ShapeMemberWritable("e")
	}
	return true
}

// drawShapeOverlay paints the selected layer's selected item and the pen
// preview.
func drawShapeOverlay(dst *ebiten.Image, m *Model, tr stageTransform, u float32) {
	drawPenPreview(dst, m, tr, u)
	n, ok := m.SelectedShapeNode()
	if !ok {
		return
	}
	live := shapeEditLive(m)
	clr := shapeIdleColor
	stroke := max(1, u/24)
	if live {
		clr = shapeColor
		stroke = max(2, u/16)
	}
	// The outline says what is selected even when the item is a style: a
	// fill or gradient outlines the geometry of its own group.
	outlinePath := n.path
	if !isShapeGeometry(n.ty) {
		if sib, ok := groupGeometryPath(m, n); ok {
			outlinePath = sib
		} else {
			outlinePath = nil
		}
	}
	if outlinePath != nil {
		if poly, ok := m.shapeOutline(n.layer, outlinePath, m.stageFrame()); ok && len(poly) >= 2 {
			for i := range poly {
				if i == len(poly)-1 && !polyClosed(m, n, outlinePath) {
					break
				}
				a, b := poly[i], poly[(i+1)%len(poly)]
				ax, ay := tr.toScreen(a[0], a[1])
				bx, by := tr.toScreen(b[0], b[1])
				vector.StrokeLine(dst, ax, ay, bx, by, stroke, clr, true)
			}
		}
	}
	if isShapeGeometry(n.ty) {
		drawShapeBox(dst, m, tr, u, live)
	}
	if n.ty == "sh" {
		drawShapeVertices(dst, m, tr, u, live)
	}
	if m.ShapeItemIsGradient() {
		drawGradGizmo(dst, m, tr, u, live)
	}
}

// shapeBoxScreen maps the selected geometry's box corners to the screen.
func shapeBoxScreen(m *Model, tr stageTransform) ([4][2]float32, bool) {
	n, okN := m.SelectedShapeNode()
	lo, hi, ok := m.ShapeBounds()
	if !okN || !ok {
		return [4][2]float32{}, false
	}
	g, okG := m.shapeSpaceMatrix(n.layer, n.path, m.stageFrame())
	if !okG {
		return [4][2]float32{}, false
	}
	var out [4][2]float32
	for c := range 4 {
		p := shapeCornerPoint(lo, hi, c)
		ax, ay := g.Apply(p[0], p[1])
		sx, sy := tr.toScreen(ax, ay)
		out[c] = [2]float32{sx, sy}
	}
	return out, true
}

// drawShapeBox frames the selected geometry and marks its four corner
// grips: dragging a corner resizes about the opposite one, dragging
// inside the shape moves it whole.
func drawShapeBox(dst *ebiten.Image, m *Model, tr stageTransform, u float32, live bool) {
	box, ok := shapeBoxScreen(m, tr)
	if !ok {
		return
	}
	clr := shapeIdleColor
	if live {
		clr = shapeColor
	}
	for c := range 4 {
		a, b := box[c], box[(c+1)%4]
		vector.StrokeLine(dst, a[0], a[1], b[0], b[1], max(1, u/28), withAlpha(clr, 0x99), true)
	}
	if !live {
		return
	}
	r := handleSize(u) / 2
	for c := range 4 {
		vector.DrawFilledRect(dst, box[c][0]-r, box[c][1]-r, r*2, r*2, shapeVertFill, true)
		vector.StrokeRect(dst, box[c][0]-r, box[c][1]-r, r*2, r*2, max(1, u/20), shapeVertEdge, true)
	}
}

// polyClosed reports whether the outline being drawn should wrap. Only an
// open path leaves the last edge undrawn.
func polyClosed(m *Model, n shapeNode, path []int) bool {
	d := m.StageClipDoc()
	if d == nil {
		return true
	}
	item, ok := d.shapeItem(n.layer, path)
	if !ok {
		return true
	}
	if ty, _ := item["ty"].(string); ty == "sh" {
		p, ok := pathAt(item, m.stageFrame(), true)
		return !ok || p.closed
	}
	return true
}

// groupGeometryPath finds the first geometry sibling of a style item, so
// selecting a fill still shows what it paints.
func groupGeometryPath(m *Model, n shapeNode) ([]int, bool) {
	d := m.StageClipDoc()
	if d == nil || len(n.path) == 0 {
		return nil, false
	}
	prefix := n.path[:len(n.path)-1]
	for _, sib := range d.shapeTree(n.layer) {
		if len(sib.path) != len(prefix)+1 {
			continue
		}
		if !slicePrefixEq(sib.path, prefix) {
			continue
		}
		if isShapeGeometry(sib.ty) {
			return sib.path, true
		}
	}
	return nil, false
}

func slicePrefixEq(path, prefix []int) bool {
	for i := range prefix {
		if path[i] != prefix[i] {
			return false
		}
	}
	return true
}

// drawShapeVertices marks the selected path's grips: square vertices, round
// handle pins on their tangent arms.
func drawShapeVertices(dst *ebiten.Image, m *Model, tr stageTransform, u float32, live bool) {
	v, in, out, ok := shapeVertexScreen(m, tr)
	if !ok {
		return
	}
	sel := m.SelectedShapeVert()
	r := handleSize(u) / 2
	hr := max(2, r/2)
	for i := range v {
		if live && i == sel {
			// Handles show for the selected vertex only; every vertex's arms
			// at once is a thicket.
			for _, h := range [][2]float32{in[i], out[i]} {
				vector.StrokeLine(dst, v[i][0], v[i][1], h[0], h[1], max(1, u/24), shapeHandleClr, true)
				vector.DrawFilledCircle(dst, h[0], h[1], hr, shapeHandleClr, true)
			}
		}
		fill := shapeVertFill
		if i == sel {
			fill = shapeColor
		}
		vector.DrawFilledRect(dst, v[i][0]-r/1.5, v[i][1]-r/1.5, r*4/3, r*4/3, fill, true)
		vector.StrokeRect(dst, v[i][0]-r/1.5, v[i][1]-r/1.5, r*4/3, r*4/3, max(1, u/24), shapeVertEdge, true)
	}
}

// drawGradGizmo is the Flash gradient transform gizmo: a square at the
// start point, a circle at the end, a diamond midway that carries both,
// and — for a radial — a cross at the highlight.
func drawGradGizmo(dst *ebiten.Image, m *Model, tr stageTransform, u float32, live bool) {
	pts, ok := gradGizmoScreen(m, tr)
	if !ok {
		return
	}
	clr := shapeIdleColor
	if live {
		clr = shapeColor
	}
	stroke := max(1, u/16)
	vector.StrokeLine(dst, pts.s[0], pts.s[1], pts.e[0], pts.e[1], stroke, clr, true)
	r := handleSize(u) / 2
	// s: square (the center of a radial, one end of a linear).
	vector.DrawFilledRect(dst, pts.s[0]-r, pts.s[1]-r, r*2, r*2, shapeVertFill, true)
	vector.StrokeRect(dst, pts.s[0]-r, pts.s[1]-r, r*2, r*2, stroke, clr, true)
	// e: circle, Flash's rotation-and-length handle in one point.
	vector.DrawFilledCircle(dst, pts.e[0], pts.e[1], r, shapeVertFill, true)
	vector.StrokeCircle(dst, pts.e[0], pts.e[1], r, stroke, clr, true)
	// mid: diamond, the whole-gradient move grip.
	var pth vector.Path
	pth.MoveTo(pts.mid[0], pts.mid[1]-r)
	pth.LineTo(pts.mid[0]+r, pts.mid[1])
	pth.LineTo(pts.mid[0], pts.mid[1]+r)
	pth.LineTo(pts.mid[0]-r, pts.mid[1])
	pth.Close()
	op := pathColor(shapeVertFill)
	vector.FillPath(dst, &pth, &vector.FillOptions{FillRule: vector.FillRuleNonZero}, &op)
	vector.StrokeLine(dst, pts.mid[0], pts.mid[1]-r, pts.mid[0]+r, pts.mid[1], stroke, clr, true)
	vector.StrokeLine(dst, pts.mid[0]+r, pts.mid[1], pts.mid[0], pts.mid[1]+r, stroke, clr, true)
	vector.StrokeLine(dst, pts.mid[0], pts.mid[1]+r, pts.mid[0]-r, pts.mid[1], stroke, clr, true)
	vector.StrokeLine(dst, pts.mid[0]-r, pts.mid[1], pts.mid[0], pts.mid[1]-r, stroke, clr, true)
}

type gradGizmoPts struct {
	s, e, mid [2]float32
}

func gradGizmoScreen(m *Model, tr stageTransform) (gradGizmoPts, bool) {
	n, ok := m.SelectedShapeNode()
	if !ok || !m.ShapeItemIsGradient() {
		return gradGizmoPts{}, false
	}
	s, e, ok := m.ShapeGradPoints()
	if !ok {
		return gradGizmoPts{}, false
	}
	g, ok := m.shapeSpaceMatrix(n.layer, n.path, m.stageFrame())
	if !ok {
		return gradGizmoPts{}, false
	}
	conv := func(p [2]float64) [2]float32 {
		ax, ay := g.Apply(p[0], p[1])
		sx, sy := tr.toScreen(ax, ay)
		return [2]float32{sx, sy}
	}
	ss, ee := conv(s), conv(e)
	return gradGizmoPts{
		s: ss, e: ee,
		mid: [2]float32{(ss[0] + ee[0]) / 2, (ss[1] + ee[1]) / 2},
	}, true
}

// drawPenPreview shows the path being drawn: its points joined, the first
// one ringed as the close target.
func drawPenPreview(dst *ebiten.Image, m *Model, tr stageTransform, u float32) {
	pts := m.PenPoints()
	if len(pts) == 0 {
		return
	}
	g, ok := m.penTargetMatrix()
	if !ok {
		return
	}
	var scr [][2]float32
	for _, p := range pts {
		ax, ay := g.Apply(p[0], p[1])
		sx, sy := tr.toScreen(ax, ay)
		scr = append(scr, [2]float32{sx, sy})
	}
	for i := 0; i+1 < len(scr); i++ {
		vector.StrokeLine(dst, scr[i][0], scr[i][1], scr[i+1][0], scr[i+1][1], max(1, u/16), penColor, true)
	}
	r := handleSize(u) / 2
	for i, p := range scr {
		vector.DrawFilledCircle(dst, p[0], p[1], max(2, r/2), penColor, true)
		if i == 0 && len(scr) >= 3 {
			vector.StrokeCircle(dst, p[0], p[1], r, max(1, u/16), penColor, true)
		}
	}
}

// ---- stage input ----

// shapeGripAt tests the grips of the current selection in priority order:
// gradient gizmo, then handles, then vertices. Screen coordinates.
func shapeGripAt(m *Model, tr stageTransform, u float32, cx, cy int) (stageDragKind, int, bool) {
	half := handleSize(u)/2 + 2
	hit := func(p [2]float32) bool {
		return abs32(float32(cx)-p[0]) <= half && abs32(float32(cy)-p[1]) <= half
	}
	if pts, ok := gradGizmoScreen(m, tr); ok {
		switch {
		case hit(pts.mid):
			return dragShapeGradBoth, 0, true
		case hit(pts.s):
			return dragShapeGradS, 0, true
		case hit(pts.e):
			return dragShapeGradE, 0, true
		}
	}
	if v, in, out, ok := shapeVertexScreen(m, tr); ok {
		sel := m.SelectedShapeVert()
		if sel >= 0 && sel < len(v) {
			if hit(in[sel]) {
				return dragShapeHandleIn, sel, true
			}
			if hit(out[sel]) {
				return dragShapeHandleOut, sel, true
			}
		}
		for i := range v {
			if hit(v[i]) {
				return dragShapeVertex, i, true
			}
		}
	}
	// The box corners come after the vertices: on a path the vertex is the
	// finer control and wins where they coincide.
	if box, ok := shapeBoxScreen(m, tr); ok {
		for c := range 4 {
			if hit(box[c]) {
				return dragShapeCorner, c, true
			}
		}
	}
	return 0, 0, false
}

// handleShapeInput is the stage's Shapes branch: the pen and primitive
// tools place things, the select tool grabs grips and picks geometry.
func (s *previewStage) handleShapeInput(context *guigui.Context, m *Model, tr stageTransform, cx, cy int) guigui.HandleInputResult {
	u := float32(basicwidget.UnitSize(context))
	ax, ay := tr.toAnim(cx, cy)
	switch m.ShapeTool() {
	case toolPen:
		// On the outline of the selected path, the pen splits the segment —
		// the classic add-a-point gesture. Anywhere else it draws.
		if !m.PenActive() && shapeEditLive(m) {
			if seg, t, ok := m.ShapeSegmentAt(ax, ay, float64(handleSize(u))/tr.scale); ok {
				m.InsertShapeVertex(seg, t)
				return guigui.HandleInputByWidget(s)
			}
		}
		m.PenClick(ax, ay, float64(handleSize(u))/tr.scale)
		return guigui.HandleInputByWidget(s)
	case toolRect, toolEllipse, toolStar:
		m.DropShapePrimitive(m.ShapeTool(), ax, ay)
		return guigui.HandleInputByWidget(s)
	}
	if kind, idx, ok := shapeGripAt(m, tr, u, cx, cy); ok && shapeEditLive(m) {
		if kind == dragShapeVertex {
			m.SelectShapeVert(idx)
		}
		s.dragMode, s.dragKind, s.shapeDragIdx = dragMove, kind, idx
		m.BeginPoseEdit()
		return guigui.HandleInputByWidget(s)
	}
	if layer, path, ok := m.ShapeAt(ax, ay, float64(handleSize(u))/tr.scale); ok {
		m.SelectShape(layer, path)
		// Selecting and moving are one gesture: the press that picked a
		// shape starts carrying it, the way every drawing tool works.
		if shapeEditLive(m) {
			s.dragMode, s.dragKind = dragMove, dragShapeMoveGeom
			m.BeginPoseEdit()
		}
		return guigui.HandleInputByWidget(s)
	}
	return s.beginPan()
}

// dragShapeStep applies one mouse-move step of a shape drag. dx, dy are in
// animation space.
func (s *previewStage) dragShapeStep(m *Model, dx, dy float64) {
	ix, iy, ok := m.animToItemDelta(dx, dy)
	if !ok {
		return
	}
	switch s.dragKind {
	case dragShapeVertex:
		m.MoveShapeVertex(s.shapeDragIdx, ix, iy)
	case dragShapeHandleIn:
		m.MoveShapeHandle(s.shapeDragIdx, false, ix, iy)
	case dragShapeHandleOut:
		m.MoveShapeHandle(s.shapeDragIdx, true, ix, iy)
	case dragShapeGradS:
		m.MoveShapeGradPoint("s", ix, iy)
	case dragShapeGradE:
		m.MoveShapeGradPoint("e", ix, iy)
	case dragShapeGradBoth:
		m.MoveShapeGradPoint("both", ix, iy)
	case dragShapeCorner:
		m.ResizeShapeGeometry(s.shapeDragIdx, ix, iy)
	case dragShapeMoveGeom:
		m.MoveShapeGeometry(ix, iy)
	}
}

// shapeCursorShape reports the cursor for the Shapes tab.
func shapeCursorShape(m *Model, tr stageTransform, u float32, cx, cy int) (ebiten.CursorShapeType, bool) {
	switch m.ShapeTool() {
	case toolPen, toolRect, toolEllipse, toolStar:
		return ebiten.CursorShapeCrosshair, true
	}
	if kind, _, ok := shapeGripAt(m, tr, u, cx, cy); ok && shapeEditLive(m) {
		if kind == dragShapeCorner {
			return ebiten.CursorShapeNWSEResize, true
		}
		return ebiten.CursorShapeMove, true
	}
	ax, ay := tr.toAnim(cx, cy)
	if _, _, ok := m.ShapeAt(ax, ay, float64(handleSize(u))/tr.scale); ok {
		if shapeEditLive(m) {
			return ebiten.CursorShapeMove, true
		}
		return ebiten.CursorShapePointer, true
	}
	return 0, false
}

// ---- panel ----

// buildShapePanel is the Shapes tab's strip: only the tool row — the
// layer picker, the tree and the structure buttons live at the top of the
// inspector pane (shapeinspector.go), the way the Parts list does for
// poses, so the strip keeps its height for the chart and the stage.
func (c *collisionPanel) buildShapePanel(context *guigui.Context, m *Model, adder *guigui.ChildAdder) {
	adder.AddWidget(&c.shapeTools)
	adder.AddWidget(&c.shapeFinish)
	editable := m.StageClipDoc() != nil && !m.Viewer()

	c.shapeToolItems = c.shapeToolItems[:0]
	for _, e := range []struct {
		text string
		tool shapeTool
	}{
		{"Select", toolSelect}, {"Pen", toolPen},
		{"Rect", toolRect}, {"Ellipse", toolEllipse}, {"Star", toolStar},
	} {
		c.shapeToolItems = append(c.shapeToolItems,
			basicwidget.SegmentedControlItem[shapeTool]{Text: e.text, Value: e.tool})
	}
	c.shapeTools.SetItems(c.shapeToolItems)
	c.shapeTools.SelectItemByValue(m.ShapeTool())
	c.shapeTools.OnItemSelected(func(context *guigui.Context, index int) {
		if it, ok := c.shapeTools.ItemByIndex(index); ok && it.Value != m.ShapeTool() {
			m.SetShapeTool(it.Value)
		}
	})
	context.SetEnabled(&c.shapeTools, editable)

	// An open path has no first-vertex click to end on; this commits it.
	c.shapeFinish.SetText("Finish")
	c.shapeFinish.OnDown(func(context *guigui.Context) { m.CommitPen(false) })
	context.SetEnabled(&c.shapeFinish, m.PenActive() && len(m.PenPoints()) >= 2)
}

func slicesEqualInt(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
