package main

import (
	"slices"
	"strings"
	"testing"
)

// Shape edits go all the way to the bundle bytes, so these tests re-parse
// the stored clip instead of trusting the model's cache, the way the pose
// tests do.

func shapeModel(t *testing.T) *Model {
	t.Helper()
	m := NewModel()
	if err := m.Bundle().SetAnimation("vec", []byte(vectorClipJSON)); err != nil {
		t.Fatalf("fixture clip rejected: %v", err)
	}
	m.ShowClip(clipRef{Anim: "vec"})
	if m.PreviewPlayer() == nil {
		t.Fatalf("clip did not go on stage: %s", m.Status())
	}
	m.PausePreview()
	m.SetCollisionTab(colShapes)
	return m
}

// storedDoc re-parses the clip as saved, so assertions read what a save
// would write.
func storedDoc(t *testing.T, m *Model) *clipDoc {
	t.Helper()
	data, ok := m.Bundle().AnimationJSON("vec")
	if !ok {
		t.Fatalf("no stored JSON")
	}
	d, err := newClipDoc("vec", data)
	if err != nil {
		t.Fatalf("stored clip unparsable: %v", err)
	}
	return d
}

func TestShapesTabPicksALayerAndListsTheTree(t *testing.T) {
	m := shapeModel(t)
	if !m.ShapesVisible() {
		t.Fatalf("shapes tab not the working context")
	}
	if m.SelectedShapeLayer() < 0 {
		t.Fatalf("no layer picked on entering the tab")
	}
	if m.InspectTarget() != inspectShape {
		t.Fatalf("inspector did not follow the tab")
	}
	nodes := m.ShapeNodes()
	if len(nodes) != 9 {
		t.Fatalf("tree = %d items", len(nodes))
	}
}

func TestShapeFillColorEditReachesTheBundle(t *testing.T) {
	m := shapeModel(t)
	m.SelectShapeNode([]int{0, 1}) // the blob's fill
	if hex, ok := m.ShapeColorHex(); !ok || hex != "#ff0000" {
		t.Fatalf("fill hex = %q ok=%v", hex, ok)
	}
	m.SelectPoseKey(0, -1)
	m.SetShapeColorHex("#0080ff")
	d := storedDoc(t, m)
	fill, _ := d.shapeItem(0, []int{0, 1})
	// The clip is a pose sequence, so the static fill was promoted and the
	// edit landed at the selected key only.
	v0, ok0 := propValueAtObj(fill, "c", 0)
	v20, ok20 := propValueAtObj(fill, "c", 20)
	if !ok0 || !ok20 {
		t.Fatalf("fill was not promoted to keys")
	}
	if v0[2] < 0.99 || v0[0] > 0.01 {
		t.Fatalf("key 0 = %v, want blue-ish", v0)
	}
	if v20[0] < 0.99 {
		t.Fatalf("key 20 should keep the old red, got %v", v20)
	}
	// And the edit is undoable.
	m.UndoClipEdit()
	d = storedDoc(t, m)
	fill, _ = d.shapeItem(0, []int{0, 1})
	if v, ok := propStaticObj(fill, "c"); !ok || v[0] != 1 {
		t.Fatalf("undo did not restore the static red fill: %v ok=%v", v, ok)
	}
}

func TestShapeMemberRefusesBetweenKeys(t *testing.T) {
	m := shapeModel(t)
	m.SelectShapeNode([]int{0, 0}) // the animated path
	m.PreviewSeek(10)              // between the keys at 0 and 20
	if m.ShapePathWritable() {
		t.Fatalf("animated path must not be writable between keys")
	}
	before := storedDoc(t, m)
	pBefore, _ := before.shapeItem(0, []int{0, 0})
	v0, _ := pathAt(pBefore, 0, false)
	m.MoveShapeVertex(0, 10, 10)
	after := storedDoc(t, m)
	pAfter, _ := after.shapeItem(0, []int{0, 0})
	vAfter, _ := pathAt(pAfter, 0, false)
	if v0.v[0] != vAfter.v[0] {
		t.Fatalf("a refused edit still reached the bundle")
	}
	if !strings.Contains(m.Status(), "park on a key") {
		t.Fatalf("no hint about parking, status = %q", m.Status())
	}
}

func TestShapeVertexDragWritesTheSelectedKeyOnly(t *testing.T) {
	m := shapeModel(t)
	m.SelectShapeNode([]int{0, 0})
	m.SelectPoseKey(20, -1)
	m.MoveShapeVertex(0, 5, -5)
	d := storedDoc(t, m)
	sh, _ := d.shapeItem(0, []int{0, 0})
	p20, _ := pathAt(sh, 20, false)
	p0, _ := pathAt(sh, 0, false)
	if p20.v[0] != [2]float64{-45, -5} {
		t.Fatalf("key 20 vertex = %v", p20.v[0])
	}
	if p0.v[0] != [2]float64{-40, 0} {
		t.Fatalf("key 0 vertex moved too: %v", p0.v[0])
	}
}

func TestGradientRampEditing(t *testing.T) {
	m := shapeModel(t)
	m.SelectShapeNode([]int{1, 1}) // the card's gradient
	if !m.ShapeItemIsGradient() {
		t.Fatalf("gradient not recognized")
	}
	m.SelectPoseKey(0, -1)
	m.AddGradStopAt(0.5)
	stops := m.ShapeGradientStops()
	if len(stops) != 3 || stops[1].pos != 0.5 {
		t.Fatalf("stops after add = %v", stops)
	}
	// The added stop carries the ramp's own color at its position, halfway
	// between black and white.
	if stops[1].r < 0.49 || stops[1].r > 0.51 {
		t.Fatalf("added stop color = %v, want mid grey", stops[1])
	}
	if m.SelectedGradStop() != 1 {
		t.Fatalf("selection did not follow the added stop")
	}
	m.SetGradStopColorHex("#ff8000")
	m.SetGradStopPos(1, 0.25)
	stops = m.ShapeGradientStops()
	if stops[1].pos != 0.25 || stops[1].r < 0.99 {
		t.Fatalf("stop after recolor+move = %v", stops)
	}
	m.DeleteGradStop(1)
	if got := len(m.ShapeGradientStops()); got != 2 {
		t.Fatalf("stops after delete = %d", got)
	}
	m.DeleteGradStop(0)
	m.SelectGradStop(0)
	m.DeleteGradStop(0)
	if got := len(m.ShapeGradientStops()); got != 2 {
		t.Fatalf("the last two stops must survive, got %d", got)
	}
}

func TestGradientGizmoMovesEndpoints(t *testing.T) {
	m := shapeModel(t)
	m.SelectShapeNode([]int{1, 1})
	m.SelectPoseKey(0, -1)
	s0, e0, ok := m.ShapeGradPoints()
	if !ok {
		t.Fatalf("no gradient points")
	}
	m.MoveShapeGradPoint("e", 10, 5)
	m.MoveShapeGradPoint("both", -5, 0)
	s1, e1, _ := m.ShapeGradPoints()
	if s1 != [2]float64{s0[0] - 5, s0[1]} {
		t.Fatalf("s = %v", s1)
	}
	if e1 != [2]float64{e0[0] + 5, e0[1] + 5} {
		t.Fatalf("e = %v", e1)
	}
}

func TestPenDrawsAGroupIntoTheLayer(t *testing.T) {
	m := shapeModel(t)
	m.SetShapeTool(toolPen)
	// The layer sits at (100, 100) in the composition, so animation-space
	// clicks land back in layer space shifted by that much.
	m.PenClick(120, 120, 6)
	m.PenClick(160, 120, 6)
	m.PenClick(140, 160, 6)
	if !m.PenActive() || len(m.PenPoints()) != 3 {
		t.Fatalf("pen points = %v", m.PenPoints())
	}
	// Clicking back on the first vertex closes and commits.
	m.PenClick(120, 120, 6)
	if m.PenActive() {
		t.Fatalf("pen still active after close")
	}
	if m.ShapeTool() != toolSelect {
		t.Fatalf("tool did not return to select")
	}
	d := storedDoc(t, m)
	nodes := d.shapeTree(0)
	if nodes[0].ty != "gr" || nodes[1].ty != "sh" || nodes[2].ty != "fl" {
		t.Fatalf("committed tree head = %v %v %v", nodes[0].ty, nodes[1].ty, nodes[2].ty)
	}
	sh, _ := d.shapeItem(0, []int{0, 0})
	p, ok := pathAt(sh, 0, false)
	if !ok || !p.closed || len(p.v) != 3 {
		t.Fatalf("committed path = %+v ok=%v", p, ok)
	}
	if p.v[0] != [2]float64{20, 20} {
		t.Fatalf("vertex not converted into layer space: %v", p.v[0])
	}
}

func TestPrimitiveDropAndStructureOps(t *testing.T) {
	m := shapeModel(t)
	m.SetShapeTool(toolRect)
	m.DropShapePrimitive(toolRect, 150, 150)
	d := storedDoc(t, m)
	nodes := d.shapeTree(0)
	if nodes[0].ty != "gr" || nodes[1].ty != "rc" {
		t.Fatalf("dropped tree head = %+v", nodes[:2])
	}
	rc, _ := d.shapeItem(0, []int{0, 0})
	if pos, ok := propStaticObj(rc, "p"); !ok || pos[0] != 50 || pos[1] != 50 {
		t.Fatalf("rect center = %v", pos)
	}
	// The selection followed the new geometry; add a stroke beside it.
	m.AddShapeItemAction("st")
	d = storedDoc(t, m)
	nodes = d.shapeTree(0)
	kinds := []string{nodes[1].ty, nodes[2].ty}
	if !slices.Equal(kinds, []string{"st", "rc"}) {
		t.Fatalf("after stroke add: %v", kinds)
	}
	// Move it behind the rect, then delete it.
	m.MoveShapeItemAction(1)
	d = storedDoc(t, m)
	nodes = d.shapeTree(0)
	if nodes[1].ty != "rc" || nodes[2].ty != "st" {
		t.Fatalf("after move: %v %v", nodes[1].ty, nodes[2].ty)
	}
	m.DeleteShapeItemAction()
	d = storedDoc(t, m)
	// The original 9 items plus the dropped group (gr + rc + fl + tr).
	if got := len(d.shapeTree(0)); got != 13 {
		t.Fatalf("after delete: %d items", got)
	}
}

func TestAddAndDeleteShapeLayerFromModel(t *testing.T) {
	m := shapeModel(t)
	m.AddShapeLayerAction()
	if m.SelectedShapeLayer() != 0 {
		t.Fatalf("new layer not selected: %d", m.SelectedShapeLayer())
	}
	d := storedDoc(t, m)
	if len(d.shapeLayerIndices()) != 2 {
		t.Fatalf("layer count = %d", len(d.shapeLayerIndices()))
	}
	m.DeleteShapeLayerAction()
	d = storedDoc(t, m)
	if len(d.shapeLayerIndices()) != 1 {
		t.Fatalf("layer not deleted")
	}
}

func TestShapePickFindsTopmostGeometry(t *testing.T) {
	m := shapeModel(t)
	// The blob is centred on the layer origin, which sits at (100, 100).
	layer, path, ok := m.ShapeAt(100, 100, 0)
	if !ok || layer != 0 || !slices.Equal(path, []int{0, 0}) {
		t.Fatalf("pick at the blob = %d %v ok=%v", layer, path, ok)
	}
	// The card rect is at layer (0, 60) → animation (100, 160).
	_, path, ok = m.ShapeAt(100, 160, 0)
	if !ok || !slices.Equal(path, []int{1, 0}) {
		t.Fatalf("pick at the card = %v ok=%v", path, ok)
	}
	if _, _, ok := m.ShapeAt(5, 5, 0); ok {
		t.Fatalf("empty stage must not pick")
	}
	// The open zig path has next to no fill area; the outline tolerance is
	// what makes it reachable. Its first vertex sits at layer (-60, -60) →
	// animation (40, 40).
	_, path, ok = m.ShapeAt(40, 43, 4)
	if !ok || !slices.Equal(path, []int{2}) {
		t.Fatalf("outline pick at the zig = %v ok=%v", path, ok)
	}
}

func TestShapePickCrossesLayersAndParksStay(t *testing.T) {
	m := NewModel()
	m.Open("../../examples/state-editor/character/character.lottie")
	if m.Bundle() == nil {
		t.Fatalf("open: %s", m.Status())
	}
	m.ShowClip(clipRef{Anim: "walk-anim"})
	m.PausePreview()
	m.SetCollisionTab(colShapes)
	if m.SelectedShapeLayer() != 0 {
		t.Fatalf("expected the body layer selected, got %d", m.SelectedShapeLayer())
	}
	// Park on one of the body layer's keys, then pick the shadow — a shape
	// on the *other* layer. The pick must land and switch the panel's layer
	// instead of reading as a click on empty stage.
	rows := m.PoseRows()
	if len(rows) == 0 {
		t.Fatalf("no fallback rows on the walk clip")
	}
	m.SelectPoseKey(m.PoseRowTimes(rows[0])[0], rows[0])
	if _, ok := m.SelectedPoseKey(); !ok {
		t.Fatalf("park did not hold")
	}
	d := m.StageClipDoc()
	shadow := -1
	for _, i := range d.shapeLayerIndices() {
		if d.layers[i].name == "shadow" {
			shadow = i
		}
	}
	if shadow < 0 {
		t.Fatalf("no shadow layer in the sample")
	}
	// The shadow ellipse sits under the feet; probe its layer position.
	found := false
	for y := 150.0; y < 200 && !found; y += 2 {
		for x := 60.0; x < 140 && !found; x += 2 {
			if layer, _, ok := m.ShapeAt(x, y, 0); ok && layer == shadow {
				m.SelectShape(layer, []int{0})
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("shadow never picked")
	}
	if m.SelectedShapeLayer() != shadow {
		t.Fatalf("pick did not switch the layer: %d", m.SelectedShapeLayer())
	}
	if _, ok := m.SelectedPoseKey(); !ok {
		t.Fatalf("picking a shape ended the park")
	}
}

func TestShapeSegmentAtFindsTheOutline(t *testing.T) {
	m := shapeModel(t)
	m.SelectShapeNode([]int{0, 0})
	// The blob's top vertex sits at layer (0, -40) → animation (100, 60);
	// halfway along segment 1 the curve passes right of that. Probe a point
	// on the outline: the segment from (0,-40) to (40,0) passes near
	// (30, -30) in layer space → (130, 70) on stage.
	seg, tt, ok := m.ShapeSegmentAt(130, 70, 8)
	if !ok || seg != 1 {
		t.Fatalf("segment at outline = %d t=%v ok=%v", seg, tt, ok)
	}
	if _, _, ok := m.ShapeSegmentAt(100, 100, 8); ok {
		t.Fatalf("center of the blob is not on the outline")
	}
}

func TestParkingFromMachinePreviewKeepsShapePicking(t *testing.T) {
	m := NewModel()
	m.Open("../../examples/state-editor/character/character.lottie")
	if m.Bundle() == nil {
		t.Fatalf("open: %s", m.Status())
	}
	// The machine is on stage (Open selects it); enter the Shapes tab and
	// select something in the active state's clip.
	if m.Preview() == nil {
		t.Fatalf("no machine preview")
	}
	m.SetCollisionTab(colShapes)
	layer := m.SelectedShapeLayer()
	if layer < 0 {
		t.Fatalf("no layer selected on entering the tab")
	}
	m.SelectShapeNode([]int{0, 0})

	// Parking a key switches the stage from the machine to the clip
	// itself. The document is the same, so the selection must survive and
	// stage picking must keep working — this was the reported break.
	if times := m.PoseTimes(); len(times) > 0 {
		m.SelectPoseKey(times[0], -1)
	} else if rows := m.PoseRows(); len(rows) > 0 {
		m.SelectPoseKey(m.PoseRowTimes(rows[0])[0], rows[0])
	} else {
		t.Fatalf("no keys at all for the stage clip")
	}
	if _, ok := m.SelectedPoseKey(); !ok {
		t.Fatalf("park did not hold")
	}
	if m.PreviewClip().Anim == "" {
		t.Fatalf("parking did not take the clip on stage")
	}
	if m.SelectedShapeLayer() < 0 {
		t.Fatalf("layer selection was dropped by the park")
	}
	if _, ok := m.SelectedShapeNode(); !ok {
		t.Fatalf("shape selection was dropped by the park")
	}
	found := false
	for y := 20.0; y < 200 && !found; y += 4 {
		for x := 20.0; x < 200 && !found; x += 4 {
			if _, _, ok := m.ShapeAt(x, y, 2); ok {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("nothing picks after parking on a key")
	}
}

func TestMoveAndResizeGeometry(t *testing.T) {
	m := shapeModel(t)

	// The card rect: center (0, 60), size 80×40 in its item space.
	m.SelectShapeNode([]int{1, 0})
	lo, hi, ok := m.ShapeBounds()
	if !ok || lo != [2]float64{-40, 40} || hi != [2]float64{40, 80} {
		t.Fatalf("rect bounds = %v %v ok=%v", lo, hi, ok)
	}
	m.MoveShapeGeometry(10, -5)
	if p, _ := m.ShapeMemberValue("p"); p[0] != 10 || p[1] != 55 {
		t.Fatalf("rect center after move = %v", p)
	}
	// Drag the bottom-right corner out by (40, 20): the top-left corner
	// (-30, 35) holds still, so the size grows by the same amount.
	m.ResizeShapeGeometry(2, 40, 20)
	s, _ := m.ShapeMemberValue("s")
	p, _ := m.ShapeMemberValue("p")
	if s[0] != 120 || s[1] != 60 {
		t.Fatalf("rect size after resize = %v", s)
	}
	if p[0] != 30 || p[1] != 65 {
		t.Fatalf("rect center after resize = %v", p)
	}
	d := storedDoc(t, m)
	rc, _ := d.shapeItem(0, []int{1, 0})
	// The clip is a pose sequence, so the first write promoted the static
	// members to keys; the edit landed at the stage frame's key.
	if v, ok := propValueObj(rc, "s", 0); !ok || v[0] != 120 {
		t.Fatalf("resize did not reach the bundle: %v ok=%v", v, ok)
	}

	// The animated blob path moves whole, on the selected key only.
	m.SelectShapeNode([]int{0, 0})
	m.SelectPoseKey(0, -1)
	m.MoveShapeGeometry(5, 5)
	d = storedDoc(t, m)
	sh, _ := d.shapeItem(0, []int{0, 0})
	p0, _ := pathAt(sh, 0, false)
	p20, _ := pathAt(sh, 20, false)
	if p0.v[0] != [2]float64{-35, 5} {
		t.Fatalf("path vertex after move = %v", p0.v[0])
	}
	if p20.v[0] != [2]float64{-50, 0} {
		t.Fatalf("the other key moved too: %v", p20.v[0])
	}
	// Scaling the path doubles it about the fixed corner; the tangents
	// scale with it or the curve would flatten.
	lo, hi, _ = m.ShapeBounds()
	m.ResizeShapeGeometry(2, hi[0]-lo[0], hi[1]-lo[1])
	d = storedDoc(t, m)
	sh, _ = d.shapeItem(0, []int{0, 0})
	after, _ := pathAt(sh, 0, false)
	w := func(p pathData) float64 {
		lo, hi := p.v[0][0], p.v[0][0]
		for _, v := range p.v {
			lo, hi = min(lo, v[0]), max(hi, v[0])
		}
		return hi - lo
	}
	if got := w(after); got < 155 || got > 165 {
		t.Fatalf("path width after doubling = %v", got)
	}
	if after.o[0][1] > -35 {
		t.Fatalf("tangents did not scale: %v", after.o[0])
	}
}

func TestGeometryInsertsIntoTheSelectedGroup(t *testing.T) {
	m := shapeModel(t)
	m.SelectShapeNode([]int{0}) // the blob group
	m.AddShapeItemAction("el")
	d := storedDoc(t, m)
	nodes := d.shapeTree(0)
	if nodes[1].ty != "el" {
		t.Fatalf("ellipse not inserted into the group: %v", nodes[1].ty)
	}
	// A path placeholder inserts the same way and is a real diamond.
	m.SelectShapeNode([]int{0, 0})
	m.AddShapeItemAction("sh")
	d = storedDoc(t, m)
	item, _ := d.shapeItem(0, []int{0, 0})
	if ty, _ := item["ty"].(string); ty != "sh" {
		t.Fatalf("path not inserted: %v", ty)
	}
	if p, ok := pathAt(item, 0, true); !ok || len(p.v) != 4 || !p.closed {
		t.Fatalf("placeholder path = %+v ok=%v", p, ok)
	}
}

func TestCopyPasteAndDuplicateShapeItems(t *testing.T) {
	m := shapeModel(t)

	// Copy the blob group, paste it into the card group.
	m.SelectShapeNode([]int{0})
	m.CopyShapeItem()
	if !m.CanPasteShapeItem() {
		t.Fatalf("clipboard empty after copy")
	}
	m.SelectShapeNode([]int{1})
	m.PasteShapeItem()
	d := storedDoc(t, m)
	pasted, ok := d.shapeItem(0, []int{1, 0})
	if !ok {
		t.Fatalf("nothing pasted into the card group")
	}
	if ty, _ := pasted["ty"].(string); ty != "gr" {
		t.Fatalf("pasted item = %v", ty)
	}
	if nm, _ := pasted["nm"].(string); nm != "blob" {
		t.Fatalf("pasted name = %q", nm)
	}
	// The paste is a deep copy: editing it leaves the original alone.
	m.SelectShapeNode([]int{1, 0, 1}) // the pasted blob's fill
	m.SelectPoseKey(0, -1)
	m.SetShapeColorHex("#00ff00")
	d = storedDoc(t, m)
	orig, _ := d.shapeItem(0, []int{0, 1})
	if v, ok := propStaticObj(orig, "c"); !ok || v[0] != 1 {
		t.Fatalf("editing the paste changed the original: %v ok=%v", v, ok)
	}

	// Duplicate lands beside its source in the same group.
	m.SelectShapeNode([]int{1, 1}) // the card's rect (shifted by the paste)
	before := len(storedDoc(t, m).shapeTree(0))
	m.DuplicateShapeItem()
	d = storedDoc(t, m)
	if got := len(d.shapeTree(0)); got != before+1 {
		t.Fatalf("tree grew by %d", got-before)
	}
	twin, _ := d.shapeItem(0, []int{1, 1})
	next, _ := d.shapeItem(0, []int{1, 2})
	if ty, _ := twin["ty"].(string); ty != "rc" {
		t.Fatalf("twin = %v", ty)
	}
	if ty, _ := next["ty"].(string); ty != "rc" {
		t.Fatalf("original not beside the twin: %v", ty)
	}

	// The clipboard survives switching clips, so a group copied here
	// pastes into another clip.
	if err := m.Bundle().SetAnimation("other", []byte(vectorClipJSON)); err != nil {
		t.Fatalf("second clip: %v", err)
	}
	m.ShowClip(clipRef{Anim: "other"})
	m.PausePreview()
	m.SelectShapeLayer(0)
	if !m.CanPasteShapeItem() {
		t.Fatalf("clipboard lost across clips")
	}
	m.SelectShapeNode(nil)
	m.PasteShapeItem()
	data, _ := m.Bundle().AnimationJSON("other")
	d2, err := newClipDoc("other", data)
	if err != nil {
		t.Fatalf("other clip unparsable: %v", err)
	}
	if item, ok := d2.shapeItem(0, []int{0}); !ok {
		t.Fatalf("nothing pasted into the other clip")
	} else if nm, _ := item["nm"].(string); nm != "blob" {
		t.Fatalf("pasted into other clip = %q", nm)
	}
}

func TestPrimitiveGripsHideBetweenKeys(t *testing.T) {
	m := shapeModel(t)
	// Animate the card rect's size by parking and editing at a key, so the
	// member gains keys at 0 and 20.
	m.SelectShapeNode([]int{1, 0})
	m.SelectPoseKey(0, -1)
	m.SetShapeMemberComponent("s", 0, 90)
	if !shapeEditLive(m) {
		t.Fatalf("parked on a key must be live")
	}
	// Between the keys nothing is writable, so the grips must not offer a
	// drag.
	m.PreviewSeek(10)
	if shapeEditLive(m) {
		t.Fatalf("between keys must not be live")
	}
}

func TestCopyShapeValuesFromNeighbourKeys(t *testing.T) {
	m := shapeModel(t)

	// The path's vertex copies from the key next door: at key 20 vertex 0
	// sits at (-50, 0), at key 0 at (-40, 0).
	m.SelectShapeNode([]int{0, 0})
	m.SelectPoseKey(20, -1)
	m.SelectShapeVert(0)
	v, at, ok := m.ShapeVertexAdjacentValue(0, -1)
	if !ok || at != 0 || v != -40 {
		t.Fatalf("vertex neighbour = %v at %v ok=%v", v, at, ok)
	}
	if _, _, ok := m.ShapeVertexAdjacentValue(0, +1); ok {
		t.Fatalf("there is no key after 20")
	}
	m.CopyShapeVertexFromAdjacent(0, -1)
	d := storedDoc(t, m)
	sh, _ := d.shapeItem(0, []int{0, 0})
	p20, _ := pathAt(sh, 20, false)
	if p20.v[0][0] != -40 {
		t.Fatalf("vertex after copy = %v", p20.v[0])
	}

	// A member copies the same way once it animates: keying the rect's
	// size promotes it, then key 20 pulls key 0's value back.
	m.SelectShapeNode([]int{1, 0})
	m.SelectPoseKey(0, -1)
	m.SetShapeMemberComponent("s", 0, 90)
	m.SelectPoseKey(20, -1)
	adj, at, ok := m.ShapeAdjacentValue("s", -1)
	if !ok || at != 0 || adj[0] != 90 {
		t.Fatalf("member neighbour = %v at %v ok=%v", adj, at, ok)
	}
	m.CopyShapeValueFromAdjacent("s", 0, -1)
	d = storedDoc(t, m)
	rc, _ := d.shapeItem(0, []int{1, 0})
	if v, ok := propValueAtObj(rc, "s", 20); !ok || v[0] != 90 {
		t.Fatalf("member after copy = %v ok=%v", v, ok)
	}
	// A static member reports no neighbours: it already equals them.
	m.SelectShapeNode([]int{0, 1})
	if _, _, ok := m.ShapeAdjacentValue("c", -1); ok {
		t.Fatalf("a static member has no neighbour keys")
	}
}

func TestDragAtKeyOnUnposedClipKeysTheMemberThere(t *testing.T) {
	// The walk clip is not a pose sequence — its chart shows a per-layer
	// row — and its rect is static. Dragging the rect while parked on one
	// of the body row's keys used to write the static value, moving every
	// keyframe at once; it must key the member instead and move that
	// keyframe alone.
	m := NewModel()
	m.Open("../../examples/state-editor/character/character.lottie")
	if m.Bundle() == nil {
		t.Fatalf("open: %s", m.Status())
	}
	m.ShowClip(clipRef{Anim: "walk-anim"})
	m.PausePreview()
	m.SetCollisionTab(colShapes)
	rows := m.PoseRows()
	if len(rows) == 0 {
		t.Fatalf("walk clip should fall back to layer rows")
	}
	times := m.PoseRowTimes(rows[0])
	m.SelectPoseKey(times[1], rows[0])
	if _, ok := m.SelectedPoseKey(); !ok {
		t.Fatalf("park failed")
	}
	m.SelectShapeNode([]int{0, 0}) // the inner rect
	before, _ := m.ShapeMemberValue("p")
	m.MoveShapeGeometry(10, 0)

	data, _ := m.Bundle().AnimationJSON("walk-anim")
	d, err := newClipDoc("walk-anim", data)
	if err != nil {
		t.Fatalf("stored clip: %v", err)
	}
	rc, _ := d.shapeItem(0, []int{0, 0})
	moved, okM := propValueAtObj(rc, "p", times[1])
	if !okM || moved[0] != before[0]+10 {
		t.Fatalf("no key with the move at frame %v: %v ok=%v", times[1], moved, okM)
	}
	still, okS := propValueAtObj(rc, "p", times[0])
	if !okS || still[0] != before[0] {
		t.Fatalf("the other keyframe moved too: %v ok=%v", still, okS)
	}
	if got := shapeMemberKeyTimes(rc, "p"); !slices.Equal(got, times) {
		t.Fatalf("promoted key times = %v, want the row's %v", got, times)
	}
}

func TestVertexDragAtKeyPromotesAStaticPath(t *testing.T) {
	m := shapeModel(t)
	m.SelectShapeNode([]int{2}) // the static zig path
	m.SelectPoseKey(0, -1)
	m.MoveShapeVertex(0, 3, 0)
	d := storedDoc(t, m)
	zig, _ := d.shapeItem(0, []int{2})
	if !pathAnimated(zig) {
		t.Fatalf("static path not promoted by an edit at a key")
	}
	p0, _ := pathAt(zig, 0, false)
	p20, _ := pathAt(zig, 20, false)
	if p0.v[0][0] != -57 {
		t.Fatalf("vertex at the edited key = %v", p0.v[0])
	}
	if p20.v[0][0] != -60 {
		t.Fatalf("the other key moved too: %v", p20.v[0])
	}
}

func TestShapeVertexGhostsFollowOnionSkin(t *testing.T) {
	m := shapeModel(t)
	m.SelectShapeNode([]int{0, 0})
	m.SelectPoseKey(0, -1)
	if got := m.ShapeVertexGhosts(); len(got) != 0 {
		t.Fatalf("ghosts without the toggle: %d", len(got))
	}
	m.SetOnionSkin(true)
	ghosts := m.ShapeVertexGhosts()
	if len(ghosts) != 1 || !ghosts[0].next || ghosts[0].frame != 20 {
		t.Fatalf("ghosts = %+v", ghosts)
	}
	// Key 20's first vertex sits at layer (-50, 0), the layer at (100, 100).
	if len(ghosts[0].pts) != 4 || ghosts[0].pts[0] != [2]float64{50, 100} {
		t.Fatalf("ghost points = %v", ghosts[0].pts)
	}
}

func TestVertexInsertDeleteFromModel(t *testing.T) {
	m := shapeModel(t)
	m.SelectShapeNode([]int{0, 0})
	m.SelectPoseKey(0, -1)
	m.InsertShapeVertex(0, 0.5)
	d := storedDoc(t, m)
	sh, _ := d.shapeItem(0, []int{0, 0})
	for _, frame := range []float64{0, 20} {
		if p, _ := pathAt(sh, frame, false); len(p.v) != 5 {
			t.Fatalf("frame %v: %d vertices", frame, len(p.v))
		}
	}
	if m.SelectedShapeVert() != 1 {
		t.Fatalf("selection did not follow the inserted vertex")
	}
	m.DeleteShapeVertex()
	d = storedDoc(t, m)
	sh, _ = d.shapeItem(0, []int{0, 0})
	if p, _ := pathAt(sh, 0, false); len(p.v) != 4 {
		t.Fatalf("delete did not land: %d vertices", len(p.v))
	}
}
