package main

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// Shape editing is the model side of the Shapes tab: which shape layer and
// which item of its tree is selected, what the stage overlay shows, and
// what a drag or a typed value writes. Everything goes through the same
// clip document and undo stack as pose editing — a shape edit is a clip
// edit like any other.

// shapeTool is the active gesture of the Shapes tab. Every other tab is
// single-gesture; drawing needs a mode, so the mode row lives here alone.
type shapeTool int

const (
	toolSelect shapeTool = iota
	toolPen
	toolRect
	toolEllipse
	toolStar
)

func (m *Model) ShapesVisible() bool {
	return m.CollisionTab() == colShapes
}

func (m *Model) ShapeTool() shapeTool { return m.shapeTool }

func (m *Model) SetShapeTool(t shapeTool) {
	if t != m.shapeTool {
		m.cancelPen()
	}
	m.shapeTool = t
	m.generation++
}

func (m *Model) clearShapeSelection() {
	m.selShapeLayer = -1
	m.selShapePath = nil
	m.selShapeVert = -1
	m.selGradStop = -1
	m.cancelPen()
}

// ShapeLayers lists the stage clip's shape layers.
func (m *Model) ShapeLayers() []int {
	d := m.StageClipDoc()
	if d == nil {
		return nil
	}
	return d.shapeLayerIndices()
}

// SelectedShapeLayer is the layer whose tree the panel shows, or -1.
func (m *Model) SelectedShapeLayer() int {
	d := m.StageClipDoc()
	if d == nil {
		return -1
	}
	if l := d.layer(m.selShapeLayer); l == nil || l.ty != 4 {
		return -1
	}
	return m.selShapeLayer
}

func (m *Model) SelectShapeLayer(i int) {
	m.selShapeLayer = i
	m.selShapePath = nil
	m.selShapeVert = -1
	m.selGradStop = -1
	m.setInspect(inspectShape)
	m.generation++
}

// ShapeNodes is the selected layer's tree, paint order first.
func (m *Model) ShapeNodes() []shapeNode {
	d := m.StageClipDoc()
	layer := m.SelectedShapeLayer()
	if d == nil || layer < 0 {
		return nil
	}
	return d.shapeTree(layer)
}

// SelectShapeNode picks one item of the tree.
func (m *Model) SelectShapeNode(path []int) {
	m.selShapePath = slices.Clone(path)
	m.selShapeVert = -1
	m.selGradStop = -1
	m.setInspect(inspectShape)
	m.generation++
}

// SelectedShapeNode resolves the selected item fresh from the document, so
// a stale path after an undo or a delete reads as no selection.
func (m *Model) SelectedShapeNode() (shapeNode, bool) {
	d := m.StageClipDoc()
	layer := m.SelectedShapeLayer()
	if d == nil || layer < 0 || len(m.selShapePath) == 0 {
		return shapeNode{}, false
	}
	item, ok := d.shapeItem(layer, m.selShapePath)
	if !ok {
		return shapeNode{}, false
	}
	ty, _ := item["ty"].(string)
	name, _ := item["nm"].(string)
	return shapeNode{
		layer: layer, path: slices.Clone(m.selShapePath),
		depth: len(m.selShapePath) - 1, ty: ty, name: name,
	}, true
}

// SelectedShapeItem is the selected item's map, or nil.
func (m *Model) SelectedShapeItem() (map[string]any, bool) {
	d := m.StageClipDoc()
	n, ok := m.SelectedShapeNode()
	if !ok {
		return nil, false
	}
	return d.shapeItem(n.layer, n.path)
}

// shapeEditFrame is where a shape edit lands: the parked key when one is
// selected, the playhead otherwise. Static members are writable anywhere —
// they apply to the whole clip — while keyed members need the park, which
// setPropObj enforces by refusing a frame with no key.
func (m *Model) shapeEditFrame() float64 {
	if frame, ok := m.SelectedPoseKey(); ok {
		return frame
	}
	return m.stageFrame()
}

// ---- style and parameter members ----

// ShapeMemberValue reads one {a, k} member of the selected item at the edit
// frame: the stored value at a key, or the static value.
func (m *Model) ShapeMemberValue(member string) ([]float64, bool) {
	item, ok := m.SelectedShapeItem()
	if !ok {
		return nil, false
	}
	if v, ok := propValueObj(item, member, m.shapeEditFrame()); ok {
		return v, true
	}
	// Between keys the stored value does not exist; show the last one so
	// the pane never goes blank, the way valueNear backs the readouts.
	return propValueNearObj(item, member, m.shapeEditFrame())
}

// ShapeMemberWritable reports whether an edit to the member would land: it
// is static, or it holds a key at the edit frame.
func (m *Model) ShapeMemberWritable(member string) bool {
	item, ok := m.SelectedShapeItem()
	if !ok || m.Viewer() {
		return false
	}
	if !propAnimatedObj(item, member) {
		return true
	}
	return propIsKeyedObj(item, member, m.shapeEditFrame())
}

// SetShapeMemberValue writes one member at the edit frame, promoting a
// static member when the clip has a pose set to key against.
func (m *Model) SetShapeMemberValue(member string, v []float64) {
	if m.blockEdit() {
		return
	}
	d := m.StageClipDoc()
	item, ok := m.SelectedShapeItem()
	if d == nil || !ok {
		return
	}
	animated := propAnimatedObj(item, member)
	pushed := m.snapshotClip()
	if !d.setPropObj(item, member, m.shapeEditFrame(), v) {
		if pushed {
			m.dropLastSnapshot()
		}
		if animated {
			m.setStatus("park on a key to edit an animated value")
			m.generation++
		}
		return
	}
	m.touchClipDoc()
}

// SetShapeMemberComponent writes one component of a member, keeping the
// others as they are.
func (m *Model) SetShapeMemberComponent(member string, comp int, v float64) {
	cur, ok := m.ShapeMemberValue(member)
	if !ok || comp >= len(cur) {
		return
	}
	next := slices.Clone(cur)
	next[comp] = v
	m.SetShapeMemberValue(member, next)
}

// ShapeColorHex reads a fill or stroke color as #rrggbb.
func (m *Model) ShapeColorHex() (string, bool) {
	v, ok := m.ShapeMemberValue("c")
	if !ok || len(v) < 3 {
		return "", false
	}
	return rgbToHex(v[0], v[1], v[2]), true
}

// SetShapeColorHex writes a fill or stroke color, keeping a fourth (alpha)
// component when the document carries one.
func (m *Model) SetShapeColorHex(hex string) {
	r, g, b, ok := hexToRGB(hex)
	if !ok {
		m.setStatus("color must be #rrggbb")
		m.generation++
		return
	}
	cur, okCur := m.ShapeMemberValue("c")
	next := []float64{r, g, b}
	if okCur && len(cur) >= 4 {
		next = append(next, cur[3])
	}
	m.SetShapeMemberValue("c", next)
}

// ShapeItemIsGradient reports whether the selected item carries a ramp.
func (m *Model) ShapeItemIsGradient() bool {
	n, ok := m.SelectedShapeNode()
	return ok && (n.ty == "gf" || n.ty == "gs")
}

// ShapeGradientRadial reports the gradient's type; Set switches it. The
// type is a plain member, not animatable, which matches the renderer.
func (m *Model) ShapeGradientRadial() bool {
	item, ok := m.SelectedShapeItem()
	if !ok {
		return false
	}
	t, _ := jsonNum(item["t"])
	return t == 2
}

func (m *Model) SetShapeGradientRadial(radial bool) {
	if m.blockEdit() {
		return
	}
	item, ok := m.SelectedShapeItem()
	if !ok || !m.ShapeItemIsGradient() || m.ShapeGradientRadial() == radial {
		return
	}
	m.snapshotClip()
	if radial {
		item["t"] = 2
	} else {
		item["t"] = 1
	}
	m.touchClipDoc()
}

// ShapePlainInt reads a bare (non-{a,k}) numeric member, like a stroke's
// line cap or a gradient's type.
func (m *Model) ShapePlainInt(member string) (int, bool) {
	item, ok := m.SelectedShapeItem()
	if !ok {
		return 0, false
	}
	v, ok := jsonNum(item[member])
	if !ok {
		return 0, false
	}
	return int(v), true
}

func (m *Model) SetShapePlainInt(member string, v int) {
	if m.blockEdit() {
		return
	}
	item, ok := m.SelectedShapeItem()
	if !ok {
		return
	}
	if cur, ok := jsonNum(item[member]); ok && int(cur) == v {
		return
	}
	m.snapshotClip()
	item[member] = v
	m.touchClipDoc()
}

// ---- gradient ramp, the Flash way ----

// The ramp is the color bar of the Flash color panel: stops slide along it,
// a click in empty space adds one with the color the ramp already shows
// there, and a stop dragged off the bar is deleted. The stage carries the
// other half of the idiom, the transform gizmo (shapeui.go).

func (m *Model) ShapeGradientStops() []gradStop {
	item, ok := m.SelectedShapeItem()
	if !ok {
		return nil
	}
	stops, _, ok := gradientRamp(item, m.shapeEditFrame())
	if !ok {
		return nil
	}
	return stops
}

func (m *Model) SelectedGradStop() int { return m.selGradStop }

func (m *Model) SelectGradStop(i int) {
	m.selGradStop = i
	m.generation++
}

// writeGradStops stores an edited stop list back, sorted by position, and
// keeps the selection on the stop that moved.
func (m *Model) writeGradStops(stops []gradStop, follow int) {
	if m.blockEdit() {
		return
	}
	d := m.StageClipDoc()
	item, ok := m.SelectedShapeItem()
	if d == nil || !ok {
		return
	}
	_, alphas, _ := gradientRamp(item, m.shapeEditFrame())
	var followStop gradStop
	if follow >= 0 && follow < len(stops) {
		followStop = stops[follow]
	}
	slices.SortStableFunc(stops, func(a, b gradStop) int {
		switch {
		case a.pos < b.pos:
			return -1
		case a.pos > b.pos:
			return 1
		}
		return 0
	})
	pushed := m.snapshotClip()
	if !d.setGradientRamp(item, m.shapeEditFrame(), stops, alphas) {
		if pushed {
			m.dropLastSnapshot()
		}
		m.setStatus("park on a key to edit an animated gradient")
		m.generation++
		return
	}
	if follow >= 0 {
		m.selGradStop = slices.Index(stops, followStop)
	}
	m.touchClipDoc()
}

// SetGradStopPos slides one stop along the ramp.
func (m *Model) SetGradStopPos(i int, pos float64) {
	stops := m.ShapeGradientStops()
	if i < 0 || i >= len(stops) {
		return
	}
	pos = min(max(pos, 0), 1)
	if stops[i].pos == pos {
		return
	}
	stops[i].pos = pos
	m.writeGradStops(stops, i)
}

// SetGradStopColorHex recolors the selected stop.
func (m *Model) SetGradStopColorHex(hex string) {
	r, g, b, ok := hexToRGB(hex)
	if !ok {
		m.setStatus("color must be #rrggbb")
		m.generation++
		return
	}
	stops := m.ShapeGradientStops()
	i := m.selGradStop
	if i < 0 || i >= len(stops) {
		return
	}
	stops[i].r, stops[i].g, stops[i].b = r, g, b
	m.writeGradStops(stops, i)
}

// GradStopColorHex reads the selected stop's color.
func (m *Model) GradStopColorHex() (string, bool) {
	stops := m.ShapeGradientStops()
	i := m.selGradStop
	if i < 0 || i >= len(stops) {
		return "", false
	}
	return rgbToHex(stops[i].r, stops[i].g, stops[i].b), true
}

// AddGradStopAt adds a stop at pos carrying the color the ramp shows
// there, so adding never visibly changes the gradient — the Flash gesture.
func (m *Model) AddGradStopAt(pos float64) {
	stops := m.ShapeGradientStops()
	if len(stops) < 2 {
		return
	}
	pos = min(max(pos, 0), 1)
	r, g, b := gradColorAt(stops, pos)
	stops = append(stops, gradStop{pos, r, g, b})
	m.writeGradStops(stops, len(stops)-1)
}

// DeleteGradStop removes one; the last two stay — a gradient needs both
// ends.
func (m *Model) DeleteGradStop(i int) {
	stops := m.ShapeGradientStops()
	if i < 0 || i >= len(stops) || len(stops) <= 2 {
		return
	}
	stops = slices.Delete(stops, i, i+1)
	m.selGradStop = -1
	m.writeGradStops(stops, -1)
}

// gradColorAt interpolates the ramp's color at pos.
func gradColorAt(stops []gradStop, pos float64) (r, g, b float64) {
	sorted := slices.Clone(stops)
	slices.SortStableFunc(sorted, func(a, b gradStop) int {
		switch {
		case a.pos < b.pos:
			return -1
		case a.pos > b.pos:
			return 1
		}
		return 0
	})
	if pos <= sorted[0].pos {
		s := sorted[0]
		return s.r, s.g, s.b
	}
	for i := 1; i < len(sorted); i++ {
		if pos <= sorted[i].pos {
			a, b2 := sorted[i-1], sorted[i]
			span := b2.pos - a.pos
			t := 0.0
			if span > 0 {
				t = (pos - a.pos) / span
			}
			return a.r + (b2.r-a.r)*t, a.g + (b2.g-a.g)*t, a.b + (b2.b-a.b)*t
		}
	}
	s := sorted[len(sorted)-1]
	return s.r, s.g, s.b
}

// ---- path editing ----

// SelectedShapeVert is the picked vertex of the selected path, or -1.
func (m *Model) SelectedShapeVert() int { return m.selShapeVert }

func (m *Model) SelectShapeVert(i int) {
	m.selShapeVert = i
	m.generation++
}

// ShapePath reads the selected sh item's path at the edit frame — the
// stored value on a key or static, the nearest key between them.
func (m *Model) ShapePath() (pathData, bool) {
	item, ok := m.SelectedShapeItem()
	if !ok {
		return pathData{}, false
	}
	return pathAt(item, m.shapeEditFrame(), true)
}

// ShapePathWritable reports whether a vertex drag would land.
func (m *Model) ShapePathWritable() bool {
	item, ok := m.SelectedShapeItem()
	if !ok || m.Viewer() {
		return false
	}
	if !pathAnimated(item) {
		return true
	}
	_, ok = pathAt(item, m.shapeEditFrame(), false)
	return ok
}

// setShapePath writes the whole path at the edit frame.
func (m *Model) setShapePath(p pathData) {
	if m.blockEdit() {
		return
	}
	d := m.StageClipDoc()
	item, ok := m.SelectedShapeItem()
	if d == nil || !ok {
		return
	}
	pushed := m.snapshotClip()
	if !d.setPathAt(item, m.shapeEditFrame(), p) {
		if pushed {
			m.dropLastSnapshot()
		}
		m.setStatus("park on a key to edit an animated path")
		m.generation++
		return
	}
	m.touchClipDoc()
}

// MoveShapeVertex drags one vertex by a delta in the item's own space; the
// tangents ride along, which is what moving a point means.
func (m *Model) MoveShapeVertex(idx int, dx, dy float64) {
	p, ok := m.ShapePath()
	if !ok || idx < 0 || idx >= len(p.v) {
		return
	}
	p.v[idx][0] = round2(p.v[idx][0] + dx)
	p.v[idx][1] = round2(p.v[idx][1] + dy)
	m.setShapePath(p)
}

// MoveShapeHandle drags one bezier handle. out is the handle leaving the
// vertex; the other is the one arriving.
func (m *Model) MoveShapeHandle(idx int, out bool, dx, dy float64) {
	p, ok := m.ShapePath()
	if !ok || idx < 0 || idx >= len(p.v) {
		return
	}
	h := &p.i[idx]
	if out {
		h = &p.o[idx]
	}
	h[0] = round2(h[0] + dx)
	h[1] = round2(h[1] + dy)
	m.setShapePath(p)
}

// SetShapeVertexValue types one number of the selected vertex: comp 0/1 is
// the point, 2/3 the in handle, 4/5 the out handle.
func (m *Model) SetShapeVertexValue(comp int, v float64) {
	p, ok := m.ShapePath()
	idx := m.selShapeVert
	if !ok || idx < 0 || idx >= len(p.v) {
		return
	}
	switch comp {
	case 0:
		p.v[idx][0] = v
	case 1:
		p.v[idx][1] = v
	case 2:
		p.i[idx][0] = v
	case 3:
		p.i[idx][1] = v
	case 4:
		p.o[idx][0] = v
	case 5:
		p.o[idx][1] = v
	default:
		return
	}
	m.setShapePath(p)
}

// SmoothShapeVertex toggles a vertex between corner and smooth: corner
// zeroes the tangents, smooth grows a level pair from the neighbouring
// segment directions.
func (m *Model) SmoothShapeVertex(idx int) {
	p, ok := m.ShapePath()
	if !ok || idx < 0 || idx >= len(p.v) {
		return
	}
	isCorner := p.i[idx] == [2]float64{} && p.o[idx] == [2]float64{}
	if !isCorner {
		p.i[idx] = [2]float64{}
		p.o[idx] = [2]float64{}
		m.setShapePath(p)
		return
	}
	n := len(p.v)
	prev := p.v[(idx-1+n)%n]
	next := p.v[(idx+1)%n]
	dx, dy := next[0]-prev[0], next[1]-prev[1]
	l := math.Hypot(dx, dy)
	if l == 0 {
		return
	}
	// A third of the chord each way is the classic smoothing length.
	k := l / 6
	p.o[idx] = [2]float64{round2(dx / l * k), round2(dy / l * k)}
	p.i[idx] = [2]float64{round2(-dx / l * k), round2(-dy / l * k)}
	m.setShapePath(p)
}

// InsertShapeVertex splits a segment at t on every key of the path, so the
// topology every key must share changes everywhere at once.
func (m *Model) InsertShapeVertex(seg int, t float64) {
	if m.blockEdit() {
		return
	}
	item, ok := m.SelectedShapeItem()
	if !ok {
		return
	}
	pushed := m.snapshotClip()
	if !insertPathVertex(item, seg, t) {
		if pushed {
			m.dropLastSnapshot()
		}
		return
	}
	m.selShapeVert = seg + 1
	m.touchClipDoc()
}

// DeleteShapeVertex removes the selected vertex from every key.
func (m *Model) DeleteShapeVertex() {
	if m.blockEdit() || m.selShapeVert < 0 {
		return
	}
	item, ok := m.SelectedShapeItem()
	if !ok {
		return
	}
	pushed := m.snapshotClip()
	if !deletePathVertex(item, m.selShapeVert) {
		if pushed {
			m.dropLastSnapshot()
		}
		m.setStatus("a path keeps at least two vertices")
		m.generation++
		return
	}
	m.selShapeVert = -1
	m.touchClipDoc()
}

// ShapePathClosed reports and sets the path's closure, on every key.
func (m *Model) ShapePathClosed() bool {
	p, ok := m.ShapePath()
	return ok && p.closed
}

func (m *Model) SetShapePathClosed(closed bool) {
	if m.blockEdit() {
		return
	}
	item, ok := m.SelectedShapeItem()
	if !ok {
		return
	}
	pushed := m.snapshotClip()
	if !setPathClosed(item, closed) {
		if pushed {
			m.dropLastSnapshot()
		}
		return
	}
	m.touchClipDoc()
}

// ---- structure: layers and items ----

// AddShapeLayerAction creates a fresh, empty shape layer in front and
// selects it, which is where the pen draws next.
func (m *Model) AddShapeLayerAction() {
	if m.blockEdit() {
		return
	}
	d := m.StageClipDoc()
	if d == nil {
		return
	}
	var names []string
	for i := range d.layers {
		names = append(names, d.layers[i].name)
	}
	name := uniqueID("shapes", names)
	pushed := m.snapshotClip()
	i, ok := d.addShapeLayer(name)
	if !ok {
		if pushed {
			m.dropLastSnapshot()
		}
		return
	}
	m.selShapeLayer = i
	m.selShapePath = nil
	m.selShapeVert = -1
	m.setStatus("added shape layer %q", name)
	m.touchClipDoc()
}

// DeleteShapeLayerAction removes the selected shape layer whole.
func (m *Model) DeleteShapeLayerAction() {
	if m.blockEdit() {
		return
	}
	d := m.StageClipDoc()
	layer := m.SelectedShapeLayer()
	if d == nil || layer < 0 {
		return
	}
	name := d.layers[layer].name
	pushed := m.snapshotClip()
	if err := m.deletableShapeLayerErr(d, layer); err != nil {
		if pushed {
			m.dropLastSnapshot()
		}
		m.setStatus("cannot delete layer: %v", err)
		m.generation++
		return
	}
	m.clearShapeSelection()
	m.setStatus("deleted layer %q", name)
	m.touchClipDoc()
}

func (m *Model) deletableShapeLayerErr(d *clipDoc, layer int) error {
	return d.deleteLayer(layer)
}

// shapeInsertTarget is the group new items land in: the selected group, or
// the group the selected item lives in, or the layer root.
func (m *Model) shapeInsertTarget() []int {
	n, ok := m.SelectedShapeNode()
	if !ok {
		return nil
	}
	if n.ty == "gr" {
		return n.path
	}
	if len(n.path) > 1 {
		return n.path[:len(n.path)-1]
	}
	return nil
}

// AddShapeItemAction inserts one item of the given kind into the current
// group. Geometry arrives at the origin of its group's space, styled by
// whatever fills and strokes that group already carries.
func (m *Model) AddShapeItemAction(kind string) {
	if m.blockEdit() {
		return
	}
	d := m.StageClipDoc()
	layer := m.SelectedShapeLayer()
	if d == nil || layer < 0 {
		return
	}
	var item map[string]any
	switch kind {
	case "gr":
		item = newGroupItem("group")
	case "fl":
		item = newFillItem(0.5, 0.5, 0.5)
	case "st":
		item = newStrokeItem(0.1, 0.1, 0.12, 2)
	case "gf":
		item = newGradientFillItem(false)
	case "rc":
		item = newRectItem(0, 0, 100, 100)
	case "el":
		item = newEllipseItem(0, 0, 100, 100)
	case "sr":
		item = newStarItem(0, 0, 50)
	case "tm":
		item = map[string]any{"ty": "tm", "s": staticProp(0.0), "e": staticProp(100.0), "o": staticProp(0.0), "m": 1}
	case "rd":
		item = map[string]any{"ty": "rd", "r": staticProp(10.0)}
	default:
		return
	}
	target := m.shapeInsertTarget()
	pushed := m.snapshotClip()
	if !d.insertShapeItem(layer, target, item) {
		if pushed {
			m.dropLastSnapshot()
		}
		return
	}
	m.selShapePath = append(slices.Clone(target), 0)
	m.selShapeVert = -1
	m.touchClipDoc()
}

// DeleteShapeItemAction removes the selected item, its subtree included.
func (m *Model) DeleteShapeItemAction() {
	if m.blockEdit() {
		return
	}
	d := m.StageClipDoc()
	n, ok := m.SelectedShapeNode()
	if !ok {
		return
	}
	pushed := m.snapshotClip()
	if !d.deleteShapeItem(n.layer, n.path) {
		if pushed {
			m.dropLastSnapshot()
		}
		return
	}
	m.selShapePath = nil
	m.selShapeVert = -1
	m.touchClipDoc()
}

// MoveShapeItemAction shifts the selected item within its group: -1 toward
// the front of the paint order, +1 toward the back.
func (m *Model) MoveShapeItemAction(delta int) {
	if m.blockEdit() {
		return
	}
	d := m.StageClipDoc()
	n, ok := m.SelectedShapeNode()
	if !ok {
		return
	}
	pushed := m.snapshotClip()
	if !d.moveShapeItem(n.layer, n.path, delta) {
		if pushed {
			m.dropLastSnapshot()
		}
		return
	}
	m.selShapePath[len(m.selShapePath)-1] += delta
	m.touchClipDoc()
}

// RenameShapeItem names the selected item; item names are labels, not
// addresses, so blanks and duplicates are allowed here.
func (m *Model) RenameShapeItem(name string) {
	if m.blockEdit() {
		return
	}
	item, ok := m.SelectedShapeItem()
	if !ok {
		return
	}
	name = strings.TrimSpace(name)
	old, _ := item["nm"].(string)
	if name == old {
		return
	}
	m.snapshotClip()
	if name == "" {
		delete(item, "nm")
	} else {
		item["nm"] = name
	}
	m.touchClipDoc()
}

// ---- coordinate spaces ----

// ShapeLayerNameProblem reports why the stage cannot place the selected
// shape layer, or "". The overlay asks the core for the layer's matrix by
// name, exactly as pose editing does, and the same failure modes apply.
func (m *Model) ShapeLayerNameProblem() string {
	d := m.StageClipDoc()
	layer := m.SelectedShapeLayer()
	if d == nil || layer < 0 {
		return ""
	}
	return d.nameProblem(layer)
}

// shapeGroupMatrix composes the transforms of the groups enclosing an item
// path, innermost first, at a frame. Group transforms are read with the
// nearest-key rule, which is exact wherever an edit can land.
func (m *Model) shapeGroupMatrix(layer int, path []int, frame float64) ebiten.GeoM {
	var g ebiten.GeoM
	d := m.StageClipDoc()
	if d == nil {
		return g
	}
	// Ancestor groups are every prefix of the path except the item itself.
	for depth := len(path) - 1; depth >= 1; depth-- {
		group, ok := d.shapeItem(layer, path[:depth])
		if !ok {
			continue
		}
		if ty, _ := group["ty"].(string); ty != "gr" {
			continue
		}
		tr := groupTransformItem(group)
		if tr == nil {
			continue
		}
		g.Concat(trGeoM(tr, frame))
	}
	return g
}

// groupTransformItem finds a group's tr item.
func groupTransformItem(group map[string]any) map[string]any {
	it, _ := group["it"].([]any)
	for _, iv := range it {
		im, ok := iv.(map[string]any)
		if !ok {
			continue
		}
		if ty, _ := im["ty"].(string); ty == "tr" {
			return im
		}
	}
	return nil
}

// trGeoM is one transform item's matrix at a frame: translate(-a), scale,
// rotate, translate(p) — the order every Lottie renderer applies.
func trGeoM(tr map[string]any, frame float64) ebiten.GeoM {
	var g ebiten.GeoM
	if a, ok := propValueNearObj(tr, "a", frame); ok && len(a) >= 2 {
		g.Translate(-a[0], -a[1])
	}
	if s, ok := propValueNearObj(tr, "s", frame); ok && len(s) >= 2 {
		g.Scale(s[0]/100, s[1]/100)
	}
	if r, ok := propValueNearObj(tr, "r", frame); ok && len(r) > 0 {
		g.Rotate(r[0] * math.Pi / 180)
	}
	if p, ok := propValueNearObj(tr, "p", frame); ok && len(p) >= 2 {
		g.Translate(p[0], p[1])
	}
	return g
}

// shapeSpaceMatrix maps an item's own coordinates to animation space: the
// enclosing groups, then the layer's world matrix.
func (m *Model) shapeSpaceMatrix(layer int, path []int, frame float64) (ebiten.GeoM, bool) {
	d := m.StageClipDoc()
	anim := m.PreviewAnimation()
	if d == nil || anim == nil {
		return ebiten.GeoM{}, false
	}
	l := d.layer(layer)
	if l == nil || l.name == "" {
		return ebiten.GeoM{}, false
	}
	lg, ok := anim.LayerTransform(l.name, frame)
	if !ok {
		return ebiten.GeoM{}, false
	}
	g := m.shapeGroupMatrix(layer, path, frame)
	g.Concat(lg)
	return g, true
}

// selectedShapeSpace is the matrix of the selected item's space.
func (m *Model) selectedShapeSpace() (ebiten.GeoM, bool) {
	n, ok := m.SelectedShapeNode()
	if !ok {
		return ebiten.GeoM{}, false
	}
	return m.shapeSpaceMatrix(n.layer, n.path, m.stageFrame())
}

// animToItemDelta converts a drag step from animation space into the
// selected item's space, linear part only.
func (m *Model) animToItemDelta(dx, dy float64) (float64, float64, bool) {
	g, ok := m.selectedShapeSpace()
	if !ok || det(g) == 0 {
		return 0, 0, false
	}
	inv := g
	inv.SetElement(0, 2, 0)
	inv.SetElement(1, 2, 0)
	inv.Invert()
	x, y := inv.Apply(dx, dy)
	return x, y, true
}

// animToItemPoint converts an animation-space point into the selected
// item's space.
func (m *Model) animToItemPoint(ax, ay float64) (float64, float64, bool) {
	g, ok := m.selectedShapeSpace()
	if !ok || det(g) == 0 {
		return 0, 0, false
	}
	inv := g
	inv.Invert()
	x, y := inv.Apply(ax, ay)
	return x, y, true
}

// ---- gradient gizmo ----

// ShapeGradPoints reads the gradient's s and e in item space.
func (m *Model) ShapeGradPoints() (s, e [2]float64, ok bool) {
	sv, okS := m.ShapeMemberValue("s")
	ev, okE := m.ShapeMemberValue("e")
	if !okS || !okE || len(sv) < 2 || len(ev) < 2 {
		return s, e, false
	}
	return [2]float64{sv[0], sv[1]}, [2]float64{ev[0], ev[1]}, true
}

// MoveShapeGradPoint drags one gizmo handle by an item-space delta: "s" is
// the start (the center, for a radial), "e" the end — Flash's rotation and
// length handles collapsed into the point they both move. "both" carries
// the whole gradient, the center handle of the Flash gizmo.
func (m *Model) MoveShapeGradPoint(which string, dx, dy float64) {
	s, e, ok := m.ShapeGradPoints()
	if !ok {
		return
	}
	move := func(member string, p [2]float64) {
		m.SetShapeMemberValue(member, []float64{round2(p[0] + dx), round2(p[1] + dy)})
	}
	switch which {
	case "s":
		move("s", s)
	case "e":
		move("e", e)
	case "both":
		move("s", s)
		move("e", e)
	}
}

// ---- pen tool ----

// PenActive reports whether a path is being drawn.
func (m *Model) PenActive() bool { return m.penActive }

// PenPoints is the path drawn so far, in the target layer's space.
func (m *Model) PenPoints() [][2]float64 {
	return m.penPts
}

func (m *Model) cancelPen() {
	m.penActive = false
	m.penPts = nil
}

// penTargetMatrix is the space the pen draws in: the selected shape
// layer's, at the current frame.
func (m *Model) penTargetMatrix() (ebiten.GeoM, bool) {
	d := m.StageClipDoc()
	anim := m.PreviewAnimation()
	layer := m.SelectedShapeLayer()
	if d == nil || anim == nil || layer < 0 {
		return ebiten.GeoM{}, false
	}
	l := d.layer(layer)
	if l == nil || l.name == "" {
		return ebiten.GeoM{}, false
	}
	return anim.LayerTransform(l.name, m.stageFrame())
}

// PenClick adds a vertex at an animation-space point. A click near the
// first vertex closes the path and commits — the way a pen ends in every
// vector tool.
func (m *Model) PenClick(ax, ay float64, closeRadius float64) {
	if m.blockEdit() {
		return
	}
	if m.SelectedShapeLayer() < 0 {
		m.setStatus("pick or add a shape layer to draw into")
		m.generation++
		return
	}
	g, ok := m.penTargetMatrix()
	if !ok || det(g) == 0 {
		m.setStatus("name the shape layer to draw on the stage")
		m.generation++
		return
	}
	inv := g
	inv.Invert()
	x, y := inv.Apply(ax, ay)
	if m.penActive && len(m.penPts) >= 3 {
		fx, fy := g.Apply(m.penPts[0][0], m.penPts[0][1])
		if math.Hypot(fx-ax, fy-ay) <= closeRadius {
			m.CommitPen(true)
			return
		}
	}
	m.penActive = true
	m.penPts = append(m.penPts, [2]float64{round2(x), round2(y)})
	m.generation++
}

// CommitPen turns the drawn points into a corner path inside a new group
// with a default fill, selected and ready to restyle. Curves come after:
// vertices are dragged smooth with the select tool.
func (m *Model) CommitPen(closed bool) {
	if m.blockEdit() {
		return
	}
	d := m.StageClipDoc()
	layer := m.SelectedShapeLayer()
	if d == nil || layer < 0 || len(m.penPts) < 2 {
		m.cancelPen()
		m.generation++
		return
	}
	p := pathData{closed: closed}
	p.v = slices.Clone(m.penPts)
	p.i = make([][2]float64, len(p.v))
	p.o = make([][2]float64, len(p.v))
	group := newGroupItem("path", newPathItem(p), newFillItem(0.5, 0.5, 0.5))
	pushed := m.snapshotClip()
	if !d.insertShapeItem(layer, nil, group) {
		if pushed {
			m.dropLastSnapshot()
		}
		m.cancelPen()
		return
	}
	m.cancelPen()
	m.shapeTool = toolSelect
	m.selShapePath = []int{0, 0}
	m.selShapeVert = -1
	m.touchClipDoc()
}

// DropShapePrimitive places a rect, ellipse or star centered on an
// animation-space point, in a new group at the layer root.
func (m *Model) DropShapePrimitive(tool shapeTool, ax, ay float64) {
	if m.blockEdit() {
		return
	}
	d := m.StageClipDoc()
	layer := m.SelectedShapeLayer()
	if d == nil || layer < 0 {
		m.setStatus("pick or add a shape layer to draw into")
		m.generation++
		return
	}
	g, ok := m.penTargetMatrix()
	if !ok || det(g) == 0 {
		m.setStatus("name the shape layer to draw on the stage")
		m.generation++
		return
	}
	inv := g
	inv.Invert()
	x, y := inv.Apply(ax, ay)
	x, y = round2(x), round2(y)
	var item map[string]any
	name := ""
	switch tool {
	case toolRect:
		item, name = newRectItem(x, y, 100, 100), "rect"
	case toolEllipse:
		item, name = newEllipseItem(x, y, 100, 100), "ellipse"
	case toolStar:
		item, name = newStarItem(x, y, 50), "star"
	default:
		return
	}
	group := newGroupItem(name, item, newFillItem(0.5, 0.5, 0.5))
	pushed := m.snapshotClip()
	if !d.insertShapeItem(layer, nil, group) {
		if pushed {
			m.dropLastSnapshot()
		}
		return
	}
	m.shapeTool = toolSelect
	m.selShapePath = []int{0, 0}
	m.selShapeVert = -1
	m.touchClipDoc()
}

// ---- picking geometry on the stage ----

// shapeOutline flattens one geometry item into an animation-space polygon,
// for the overlay and the hit test. ok is false when the item is not
// geometry or its layer cannot be placed.
func (m *Model) shapeOutline(layer int, path []int, frame float64) ([][2]float64, bool) {
	d := m.StageClipDoc()
	if d == nil {
		return nil, false
	}
	item, ok := d.shapeItem(layer, path)
	if !ok {
		return nil, false
	}
	g, ok := m.shapeSpaceMatrix(layer, path, frame)
	if !ok {
		return nil, false
	}
	pts, ok := shapeItemPolygon(item, frame)
	if !ok {
		return nil, false
	}
	out := make([][2]float64, len(pts))
	for i, p := range pts {
		x, y := g.Apply(p[0], p[1])
		out[i] = [2]float64{x, y}
	}
	return out, true
}

// shapeItemPolygon flattens a geometry item in its own space.
func shapeItemPolygon(item map[string]any, frame float64) ([][2]float64, bool) {
	ty, _ := item["ty"].(string)
	switch ty {
	case "sh":
		p, ok := pathAt(item, frame, true)
		if !ok {
			return nil, false
		}
		return flattenPath(p, 8), true
	case "rc":
		pos, okP := propValueNearObj(item, "p", frame)
		size, okS := propValueNearObj(item, "s", frame)
		if !okP || !okS || len(pos) < 2 || len(size) < 2 {
			return nil, false
		}
		w, h := size[0]/2, size[1]/2
		return [][2]float64{
			{pos[0] - w, pos[1] - h}, {pos[0] + w, pos[1] - h},
			{pos[0] + w, pos[1] + h}, {pos[0] - w, pos[1] + h},
		}, true
	case "el":
		pos, okP := propValueNearObj(item, "p", frame)
		size, okS := propValueNearObj(item, "s", frame)
		if !okP || !okS || len(pos) < 2 || len(size) < 2 {
			return nil, false
		}
		var out [][2]float64
		for i := range 24 {
			a := float64(i) / 24 * 2 * math.Pi
			out = append(out, [2]float64{
				pos[0] + math.Cos(a)*size[0]/2,
				pos[1] + math.Sin(a)*size[1]/2,
			})
		}
		return out, true
	case "sr":
		pos, okP := propValueNearObj(item, "p", frame)
		or, okR := propValueNearObj(item, "or", frame)
		if !okP || !okR || len(pos) < 2 || len(or) == 0 {
			return nil, false
		}
		pt, _ := propValueNearObj(item, "pt", frame)
		n := 5.0
		if len(pt) > 0 && pt[0] >= 3 {
			n = pt[0]
		}
		ir := or
		if v, ok := propValueNearObj(item, "ir", frame); ok && len(v) > 0 {
			ir = v
		}
		sy, _ := jsonNum(item["sy"])
		var out [][2]float64
		steps := int(n)
		for i := range steps {
			a := float64(i)/n*2*math.Pi - math.Pi/2
			out = append(out, [2]float64{pos[0] + math.Cos(a)*or[0], pos[1] + math.Sin(a)*or[0]})
			if sy != 2 { // a star has inner points; a polygon does not
				a2 := (float64(i)+0.5)/n*2*math.Pi - math.Pi/2
				out = append(out, [2]float64{pos[0] + math.Cos(a2)*ir[0], pos[1] + math.Sin(a2)*ir[0]})
			}
		}
		return out, true
	}
	return nil, false
}

// flattenPath samples each bezier segment into line points.
func flattenPath(p pathData, steps int) [][2]float64 {
	n := len(p.v)
	if n == 0 {
		return nil
	}
	segs := n - 1
	if p.closed {
		segs = n
	}
	var out [][2]float64
	for s := range segs {
		j, k := s, (s+1)%n
		p0 := p.v[j]
		p1 := [2]float64{p.v[j][0] + p.o[j][0], p.v[j][1] + p.o[j][1]}
		p2 := [2]float64{p.v[k][0] + p.i[k][0], p.v[k][1] + p.i[k][1]}
		p3 := p.v[k]
		for i := range steps {
			t := float64(i) / float64(steps)
			out = append(out, cubicAt(p0, p1, p2, p3, t))
		}
	}
	if !p.closed {
		out = append(out, p.v[n-1])
	}
	return out
}

func cubicAt(p0, p1, p2, p3 [2]float64, t float64) [2]float64 {
	u := 1 - t
	var r [2]float64
	for c := range 2 {
		r[c] = u*u*u*p0[c] + 3*u*u*t*p1[c] + 3*u*t*t*p2[c] + t*t*t*p3[c]
	}
	return r
}

// ShapeSegmentAt finds the segment of the selected path nearest an
// animation-space point, within tol, and where along it the point falls —
// which is where the Pen inserts a vertex into an existing path.
func (m *Model) ShapeSegmentAt(ax, ay, tol float64) (seg int, t float64, ok bool) {
	n, okN := m.SelectedShapeNode()
	if !okN || n.ty != "sh" {
		return 0, 0, false
	}
	p, okP := m.ShapePath()
	if !okP {
		return 0, 0, false
	}
	g, okG := m.shapeSpaceMatrix(n.layer, n.path, m.stageFrame())
	if !okG {
		return 0, 0, false
	}
	const steps = 24
	best := tol * tol
	found := false
	nv := len(p.v)
	segs := nv - 1
	if p.closed {
		segs = nv
	}
	for s := range segs {
		j, k := s, (s+1)%nv
		p0 := p.v[j]
		p1 := [2]float64{p.v[j][0] + p.o[j][0], p.v[j][1] + p.o[j][1]}
		p2 := [2]float64{p.v[k][0] + p.i[k][0], p.v[k][1] + p.i[k][1]}
		p3 := p.v[k]
		for i := 1; i < steps; i++ {
			tt := float64(i) / steps
			pt := cubicAt(p0, p1, p2, p3, tt)
			x, y := g.Apply(pt[0], pt[1])
			d := (x-ax)*(x-ax) + (y-ay)*(y-ay)
			if d < best {
				best, seg, t, found = d, s, tt, true
			}
		}
	}
	return seg, t, found
}

// ShapeAt picks the topmost geometry item under an animation-space point,
// searching every shape layer front to back and each tree in paint order —
// the pick must not depend on which layer the panel happens to have
// selected, or a click on the other layer's artwork reads as a miss. tol
// (animation units) also picks a shape by its outline, which is the only
// area a stroke-without-fill has.
func (m *Model) ShapeAt(ax, ay, tol float64) (int, []int, bool) {
	d := m.StageClipDoc()
	if d == nil {
		return 0, nil, false
	}
	frame := m.stageFrame()
	for _, layer := range d.shapeLayerIndices() {
		// A layer switched off by opacity is not on the stage, same rule as
		// the pose pick.
		if o, ok := d.valueNear(layer, "o", frame); ok && len(o) > 0 && o[0] <= 0 {
			continue
		}
		for _, n := range d.shapeTree(layer) {
			if !isShapeGeometry(n.ty) {
				continue
			}
			poly, ok := m.shapeOutline(layer, n.path, frame)
			if !ok || len(poly) < 2 {
				continue
			}
			if len(poly) >= 3 && pointInPolygonEvenOdd(poly, ax, ay) {
				return layer, n.path, true
			}
			if tol > 0 && distToPolyEdge(poly, ax, ay) <= tol {
				return layer, n.path, true
			}
		}
	}
	return 0, nil, false
}

// SelectShape picks one item, switching the panel's layer with it when the
// pick landed on another layer's artwork.
func (m *Model) SelectShape(layer int, path []int) {
	m.selShapeLayer = layer
	m.SelectShapeNode(path)
}

// distToPolyEdge is the distance from a point to the nearest edge of a
// polygon's outline.
func distToPolyEdge(poly [][2]float64, x, y float64) float64 {
	best := math.MaxFloat64
	n := len(poly)
	for i := range n {
		a, b := poly[i], poly[(i+1)%n]
		best = min(best, distToSegment(a, b, x, y))
	}
	return best
}

func distToSegment(a, b [2]float64, x, y float64) float64 {
	dx, dy := b[0]-a[0], b[1]-a[1]
	l2 := dx*dx + dy*dy
	t := 0.0
	if l2 > 0 {
		t = min(max(((x-a[0])*dx+(y-a[1])*dy)/l2, 0), 1)
	}
	return math.Hypot(x-(a[0]+t*dx), y-(a[1]+t*dy))
}

// pointInPolygonEvenOdd is the even-odd crossing test, which matches how a
// self-intersecting path fills often enough for a pick.
func pointInPolygonEvenOdd(poly [][2]float64, x, y float64) bool {
	in := false
	n := len(poly)
	for i := range n {
		a, b := poly[i], poly[(i+1)%n]
		if (a[1] > y) != (b[1] > y) {
			t := (y - a[1]) / (b[1] - a[1])
			if x < a[0]+(b[0]-a[0])*t {
				in = !in
			}
		}
	}
	return in
}

// ---- colors ----

func rgbToHex(r, g, b float64) string {
	c := func(v float64) int {
		return int(math.Round(min(max(v, 0), 1) * 255))
	}
	return fmt.Sprintf("#%02x%02x%02x", c(r), c(g), c(b))
}

func hexToRGB(s string) (r, g, b float64, ok bool) {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "#"))
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return 0, 0, 0, false
	}
	var v [3]float64
	for i := range 3 {
		var n int
		if _, err := fmt.Sscanf(s[i*2:i*2+2], "%02x", &n); err != nil {
			return 0, 0, 0, false
		}
		v[i] = float64(n) / 255
	}
	return v[0], v[1], v[2], true
}
