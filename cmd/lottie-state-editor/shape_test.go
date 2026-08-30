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
	path, ok := m.ShapeAt(100, 100)
	if !ok || !slices.Equal(path, []int{0, 0}) {
		t.Fatalf("pick at the blob = %v ok=%v", path, ok)
	}
	// The card rect is at layer (0, 60) → animation (100, 160).
	path, ok = m.ShapeAt(100, 160)
	if !ok || !slices.Equal(path, []int{1, 0}) {
		t.Fatalf("pick at the card = %v ok=%v", path, ok)
	}
	if _, ok := m.ShapeAt(5, 5); ok {
		t.Fatalf("empty stage must not pick")
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
