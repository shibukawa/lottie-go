package main

import (
	"math"
	"slices"
	"testing"
)

// vectorClipJSON is a small but real vector clip: one named shape layer
// holding a group (animated path + fill + transform), a rect group with a
// gradient fill, and a second, static path whose vertices are corners.
const vectorClipJSON = `{
 "v": "5.7.1", "fr": 30, "ip": 0, "op": 30, "w": 200, "h": 200,
 "layers": [
  {
   "ty": 4, "nm": "vec", "ind": 1, "ip": 0, "op": 30, "st": 0, "sr": 1,
   "ks": {
    "a": {"a": 0, "k": [0, 0, 0]}, "p": {"a": 0, "k": [100, 100, 0]},
    "s": {"a": 0, "k": [100, 100, 100]}, "r": {"a": 0, "k": 0}, "o": {"a": 0, "k": 100}
   },
   "shapes": [
    {
     "ty": "gr", "nm": "blob", "it": [
      {"ty": "sh", "ks": {"a": 1, "k": [
       {"t": 0, "s": [{"c": true,
        "v": [[-40, 0], [0, -40], [40, 0], [0, 40]],
        "i": [[0, -20], [-20, 0], [0, -20], [20, 0]],
        "o": [[0, -20], [20, 0], [0, 20], [-20, 0]]}],
        "i": {"x": [0.5], "y": [1]}, "o": {"x": [0.5], "y": [0]}},
       {"t": 20, "s": [{"c": true,
        "v": [[-50, 0], [0, -30], [50, 0], [0, 30]],
        "i": [[0, -15], [-25, 0], [0, -15], [25, 0]],
        "o": [[0, -15], [25, 0], [0, 15], [-25, 0]]}],
        "i": {"x": [0.5], "y": [1]}, "o": {"x": [0.5], "y": [0]}}
      ]}},
      {"ty": "fl", "c": {"a": 0, "k": [1, 0, 0]}, "o": {"a": 0, "k": 100}, "r": 1},
      {"ty": "tr", "p": {"a": 0, "k": [0, 0]}, "a": {"a": 0, "k": [0, 0]},
       "s": {"a": 0, "k": [100, 100]}, "r": {"a": 0, "k": 0}, "o": {"a": 0, "k": 100}}
     ]
    },
    {
     "ty": "gr", "nm": "card", "it": [
      {"ty": "rc", "p": {"a": 0, "k": [0, 60]}, "s": {"a": 0, "k": [80, 40]}, "r": {"a": 0, "k": 4}},
      {"ty": "gf", "o": {"a": 0, "k": 100}, "r": 1, "t": 1,
       "s": {"a": 0, "k": [-40, 60]}, "e": {"a": 0, "k": [40, 60]},
       "g": {"p": 2, "k": {"a": 0, "k": [0, 0, 0, 1, 1, 1, 1, 0]}}},
      {"ty": "tr", "p": {"a": 0, "k": [0, 0]}, "a": {"a": 0, "k": [0, 0]},
       "s": {"a": 0, "k": [100, 100]}, "r": {"a": 0, "k": 0}, "o": {"a": 0, "k": 100}}
     ]
    },
    {"ty": "sh", "nm": "zig", "ks": {"a": 0, "k": {"c": false,
      "v": [[-60, -60], [-20, -80], [20, -60]],
      "i": [[0, 0], [0, 0], [0, 0]],
      "o": [[0, 0], [0, 0], [0, 0]]}}}
   ]
  }
 ]
}`

func vectorDoc(t *testing.T) *clipDoc {
	t.Helper()
	d, err := newClipDoc("vec", []byte(vectorClipJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return d
}

func TestShapeTreeFlattensInPaintOrder(t *testing.T) {
	d := vectorDoc(t)
	nodes := d.shapeTree(0)
	var kinds []string
	for _, n := range nodes {
		kinds = append(kinds, n.ty)
	}
	want := []string{"gr", "sh", "fl", "tr", "gr", "rc", "gf", "tr", "sh"}
	if !slices.Equal(kinds, want) {
		t.Fatalf("tree kinds = %v, want %v", kinds, want)
	}
	if nodes[1].depth != 1 || len(nodes[1].path) != 2 {
		t.Fatalf("nested path wrong: %+v", nodes[1])
	}
	if _, ok := d.shapeItem(0, nodes[6].path); !ok {
		t.Fatalf("gradient not resolvable by its path")
	}
}

func TestShapeKeyTimesJoinTheClip(t *testing.T) {
	d := vectorDoc(t)
	if want := []float64{0, 20}; !slices.Equal(d.times, want) {
		t.Fatalf("times = %v, want %v", d.times, want)
	}
	if len(d.animatedLayers()) != 1 {
		t.Fatalf("animated layers = %v", d.animatedLayers())
	}
	if !slices.Equal(d.layers[0].shapeTimes, []float64{0, 20}) {
		t.Fatalf("shapeTimes = %v", d.layers[0].shapeTimes)
	}
}

func TestSetPropObjStaticAndKeyed(t *testing.T) {
	d := vectorDoc(t)
	fill, ok := d.shapeItem(0, []int{0, 1})
	if !ok {
		t.Fatalf("no fill item")
	}
	if !d.setPropObj(fill, "c", 0, []float64{0, 0.5, 1}) {
		t.Fatalf("static color write refused")
	}
	// The clip has a pose set (0, 20), so the static write was promoted to
	// keys holding the new value at frame 0.
	if v, ok := propValueAtObj(fill, "c", 0); !ok || v[2] != 1 {
		t.Fatalf("keyed color after promote = %v ok=%v", v, ok)
	}
	if v, ok := propValueAtObj(fill, "c", 20); !ok || v[0] != 1 {
		t.Fatalf("promotion should hold the old value at other keys, got %v ok=%v", v, ok)
	}
	// Between keys nothing accepts a write.
	if d.setPropObj(fill, "c", 10, []float64{0, 0, 0}) {
		t.Fatalf("write between keys must refuse")
	}
}

func TestGradientRampReadWrite(t *testing.T) {
	d := vectorDoc(t)
	gf, _ := d.shapeItem(0, []int{1, 1})
	stops, alphas, ok := gradientRamp(gf, 0)
	if !ok || len(stops) != 2 || len(alphas) != 0 {
		t.Fatalf("ramp = %v %v ok=%v", stops, alphas, ok)
	}
	stops = append(stops, gradStop{0.5, 0.2, 0.4, 0.6})
	slices.SortStableFunc(stops, func(a, b gradStop) int {
		if a.pos < b.pos {
			return -1
		}
		if a.pos > b.pos {
			return 1
		}
		return 0
	})
	if !d.setGradientRamp(gf, 0, stops, alphas) {
		t.Fatalf("ramp write refused")
	}
	back, _, _ := gradientRamp(gf, 0)
	if len(back) != 3 || back[1].pos != 0.5 || back[1].b != 0.6 {
		t.Fatalf("ramp after write = %v", back)
	}
	if p, _ := jsonNum(gf["g"].(map[string]any)["p"]); p != 3 {
		t.Fatalf("stop count not updated: %v", p)
	}
}

func TestInsertPathVertexKeepsEveryKeyAndTheCurve(t *testing.T) {
	d := vectorDoc(t)
	sh, _ := d.shapeItem(0, []int{0, 0})
	before, _ := pathAt(sh, 0, false)
	// Where the curve passes at the split parameter, before the split.
	p0 := before.v[1]
	p1 := [2]float64{p0[0] + before.o[1][0], p0[1] + before.o[1][1]}
	p3 := before.v[2]
	p2 := [2]float64{p3[0] + before.i[2][0], p3[1] + before.i[2][1]}
	want := cubicAt(p0, p1, p2, p3, 0.5)

	if !insertPathVertex(sh, 1, 0.5) {
		t.Fatalf("insert refused")
	}
	for _, frame := range []float64{0, 20} {
		p, ok := pathAt(sh, frame, false)
		if !ok || len(p.v) != 5 {
			t.Fatalf("frame %v: %d vertices after insert", frame, len(p.v))
		}
		if !p.closed {
			t.Fatalf("closure lost")
		}
	}
	after, _ := pathAt(sh, 0, false)
	got := after.v[2]
	if math.Hypot(got[0]-want[0], got[1]-want[1]) > 1e-6 {
		t.Fatalf("new vertex at %v, want the curve point %v", got, want)
	}
}

func TestInsertPathVertexOnClosingSegment(t *testing.T) {
	d := vectorDoc(t)
	sh, _ := d.shapeItem(0, []int{0, 0})
	if !insertPathVertex(sh, 3, 0.5) {
		t.Fatalf("insert on the wrap segment refused")
	}
	p, _ := pathAt(sh, 0, false)
	if len(p.v) != 5 {
		t.Fatalf("vertices = %d", len(p.v))
	}
	// The new vertex sits after the old last one; vertex 0 is untouched.
	if p.v[0] != [2]float64{-40, 0} {
		t.Fatalf("vertex 0 moved: %v", p.v[0])
	}
}

func TestDeletePathVertexAcrossKeys(t *testing.T) {
	d := vectorDoc(t)
	sh, _ := d.shapeItem(0, []int{0, 0})
	if !deletePathVertex(sh, 1) {
		t.Fatalf("delete refused")
	}
	for _, frame := range []float64{0, 20} {
		p, _ := pathAt(sh, frame, false)
		if len(p.v) != 3 {
			t.Fatalf("frame %v: %d vertices", frame, len(p.v))
		}
	}
	// A static path is guarded at two vertices.
	zig, _ := d.shapeItem(0, []int{2})
	if !deletePathVertex(zig, 0) {
		t.Fatalf("static delete refused")
	}
	if deletePathVertex(zig, 0) {
		t.Fatalf("delete below two vertices must refuse")
	}
}

func TestPoseColumnOpsCarryShapeKeys(t *testing.T) {
	d := vectorDoc(t)
	sh, _ := d.shapeItem(0, []int{0, 0})

	if !d.insertPose(10) {
		t.Fatalf("insertPose refused")
	}
	if _, ok := pathAt(sh, 10, false); !ok {
		t.Fatalf("no path key inserted at 10")
	}
	// The inserted key copies the one before it.
	p10, _ := pathAt(sh, 10, false)
	p0, _ := pathAt(sh, 0, false)
	if p10.v[0] != p0.v[0] {
		t.Fatalf("inserted key differs from its source: %v vs %v", p10.v[0], p0.v[0])
	}

	if _, moved := d.retime(10, 12, -1); !moved {
		t.Fatalf("retime refused")
	}
	if _, ok := pathAt(sh, 12, false); !ok {
		t.Fatalf("path key did not move with the column")
	}

	if !d.deletePose(12) {
		t.Fatalf("deletePose refused")
	}
	if _, ok := pathAt(sh, 12, false); ok {
		t.Fatalf("path key survived the column delete")
	}
}

func TestShapeStructureEdits(t *testing.T) {
	d := vectorDoc(t)
	// New layer goes in front and indexes shift with it.
	i, ok := d.addShapeLayer("fresh")
	if !ok || i != 0 {
		t.Fatalf("addShapeLayer = %d %v", i, ok)
	}
	if len(d.shapeLayerIndices()) != 2 {
		t.Fatalf("layers = %v", d.shapeLayerIndices())
	}
	if !d.insertShapeItem(0, nil, newGroupItem("g", newRectItem(0, 0, 10, 10))) {
		t.Fatalf("insert into fresh layer refused")
	}
	nodes := d.shapeTree(0)
	if len(nodes) != 3 || nodes[0].ty != "gr" || nodes[1].ty != "rc" {
		t.Fatalf("fresh tree = %+v", nodes)
	}
	// Move within a group and delete.
	if !d.moveShapeItem(0, []int{0, 0}, 1) {
		t.Fatalf("move refused")
	}
	nodes = d.shapeTree(0)
	if nodes[1].ty != "tr" || nodes[2].ty != "rc" {
		t.Fatalf("after move: %v %v", nodes[1].ty, nodes[2].ty)
	}
	if !d.deleteShapeItem(0, []int{0, 1}) {
		t.Fatalf("delete refused")
	}
	if nodes = d.shapeTree(0); len(nodes) != 2 {
		t.Fatalf("after delete: %+v", nodes)
	}
	// The old vector layer shifted to index 1 and still reads.
	if got := d.shapeTree(1); len(got) != 9 {
		t.Fatalf("shifted layer tree = %d items", len(got))
	}
	if err := d.deleteLayer(0); err != nil {
		t.Fatalf("deleteLayer: %v", err)
	}
	if len(d.shapeLayerIndices()) != 1 {
		t.Fatalf("layer not deleted")
	}
}

func TestClipStillDecodesAfterShapeEdits(t *testing.T) {
	d := vectorDoc(t)
	sh, _ := d.shapeItem(0, []int{0, 0})
	insertPathVertex(sh, 0, 0.5)
	fill, _ := d.shapeItem(0, []int{0, 1})
	d.setPropObj(fill, "c", 0, []float64{0, 1, 0})
	d.addShapeLayer("extra")
	d.insertShapeItem(0, nil, newGroupItem("g", newEllipseItem(0, 0, 40, 40)))

	data, err := d.encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	m := NewModel()
	if err := m.Bundle().SetAnimation("vec", data); err != nil {
		t.Fatalf("edited clip no longer decodes: %v", err)
	}
	anim, err := m.Bundle().Animation("vec")
	if err != nil {
		t.Fatalf("animation: %v", err)
	}
	if u := anim.UnsupportedFeatures(); len(u) > 0 {
		t.Fatalf("edit introduced unsupported features: %v", u)
	}
}
