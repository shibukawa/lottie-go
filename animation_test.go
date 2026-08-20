package lottie

import (
	"math"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

func loadTestAnimation(t *testing.T, name string) *Animation {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	anim, err := Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return anim
}

func TestDecodeBasic(t *testing.T) {
	anim := loadTestAnimation(t, "basic.json")
	if w, h := anim.Size(); w != 200 || h != 200 {
		t.Errorf("Size() = %d, %d; want 200, 200", w, h)
	}
	if got, want := anim.Duration(), 2*time.Second; got != want {
		t.Errorf("Duration() = %v; want %v", got, want)
	}
	if anim.FrameRate() != 60 {
		t.Errorf("FrameRate() = %v; want 60", anim.FrameRate())
	}
	// Null, shape, and text layers all survive since P2.
	if len(anim.layers) != 3 {
		t.Fatalf("len(layers) = %d; want 3", len(anim.layers))
	}
	if unsup := anim.UnsupportedFeatures(); len(unsup) != 0 {
		t.Errorf("UnsupportedFeatures() = %v; want none", unsup)
	}
}

func TestDecodeErrors(t *testing.T) {
	if _, err := Decode(strings.NewReader("{not json")); err == nil {
		t.Error("malformed JSON: got nil error")
	}
	if _, err := Decode(strings.NewReader(`{"fr": 0, "layers": [{}]}`)); err == nil {
		t.Error("zero frame rate: got nil error")
	}
	if _, err := Decode(strings.NewReader(`{"fr": 30}`)); err == nil {
		t.Error("no layers: got nil error")
	}
}

func TestLayerParenting(t *testing.T) {
	anim := loadTestAnimation(t, "basic.json")
	var shapeLayer *layerNode
	for _, l := range anim.layers {
		if l.typ == 4 {
			shapeLayer = l
		}
	}
	if shapeLayer == nil {
		t.Fatal("no shape layer")
	}
	if shapeLayer.parent == nil || shapeLayer.parent.name != "parent null" {
		t.Fatalf("parent not resolved: %+v", shapeLayer.parent)
	}
	// Parent scales 200% and translates (100,100); child at frame 0 sits at
	// local origin, so layer-space origin maps to (100,100).
	m := layerMatrix(shapeLayer, 0, 0)
	x, y := m.apply(0, 0)
	if math.Abs(x-100) > 1e-9 || math.Abs(y-100) > 1e-9 {
		t.Errorf("origin maps to (%v, %v); want (100, 100)", x, y)
	}
	// A point at local (1,0) is scaled by the parent's 200%.
	x, y = m.apply(1, 0)
	if math.Abs(x-102) > 1e-9 {
		t.Errorf("x scale: got %v; want 102", x)
	}
	_ = y
}

func TestKeyframeInterpolation(t *testing.T) {
	anim := loadTestAnimation(t, "basic.json")
	var l *layerNode
	for _, cand := range anim.layers {
		if cand.typ == 4 {
			l = cand
		}
	}
	pos := l.transform.position

	if got := pos.at(0, nil); got[0] != 0 {
		t.Errorf("pos at 0 = %v; want 0", got[0])
	}
	if got := pos.at(60, nil); got[0] != 50 {
		t.Errorf("pos at 60 = %v; want 50", got[0])
	}
	// Symmetric ease(0.5, 0, 0.5, 1) passes through the midpoint.
	if got := pos.at(30, nil); math.Abs(got[0]-25) > 1e-6 {
		t.Errorf("pos at 30 = %v; want 25", got[0])
	}
	// Eased curve must differ from linear away from the midpoint.
	if got := pos.at(15, nil); math.Abs(got[0]-12.5) < 0.5 {
		t.Errorf("pos at 15 = %v; want eased (not ~12.5)", got[0])
	}

	// Hold keyframe: value stays until the next key.
	rot := l.transform.rotation
	if got := rot.scalarAt(15, -1); got != 0 {
		t.Errorf("rot at 15 = %v; want 0 (hold)", got)
	}
	if got := rot.scalarAt(30, -1); got != 90 {
		t.Errorf("rot at 30 = %v; want 90", got)
	}
}

func TestLegacyEndValueKeyframes(t *testing.T) {
	json := `{
	  "fr": 30, "ip": 0, "op": 30, "w": 10, "h": 10,
	  "layers": [{
	    "ty": 4, "ind": 1, "ip": 0, "op": 30, "st": 0,
	    "ks": { "p": { "a": 1, "k": [
	      { "t": 0, "s": [0, 0], "e": [10, 0] },
	      { "t": 30 }
	    ] } },
	    "shapes": []
	  }]
	}`
	anim, err := Decode(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	pos := anim.layers[0].transform.position
	if got := pos.at(15, nil); math.Abs(got[0]-5) > 1e-9 {
		t.Errorf("legacy keyframe at 15 = %v; want 5", got[0])
	}
	if got := pos.at(30, nil); math.Abs(got[0]-10) > 1e-9 {
		t.Errorf("legacy keyframe at 30 = %v; want 10", got[0])
	}
}

func TestSplitPosition(t *testing.T) {
	json := `{
	  "fr": 30, "ip": 0, "op": 30, "w": 10, "h": 10,
	  "layers": [{
	    "ty": 4, "ind": 1, "ip": 0, "op": 30, "st": 0,
	    "ks": { "p": { "s": true,
	      "x": { "a": 0, "k": 7 },
	      "y": { "a": 1, "k": [
	        { "t": 0, "s": [0] }, { "t": 30, "s": [30] }
	      ] } } },
	    "shapes": []
	  }]
	}`
	anim, err := Decode(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	tr := anim.layers[0].transform
	m := tr.matrixAt(15)
	x, y := m.apply(0, 0)
	if math.Abs(x-7) > 1e-9 || math.Abs(y-15) > 1e-9 {
		t.Errorf("split position at 15 = (%v, %v); want (7, 15)", x, y)
	}
}

func TestEasingSolver(t *testing.T) {
	e := easing{outX: 0.25, outY: 0.1, inX: 0.25, inY: 1}
	if got := e.at(0); got != 0 {
		t.Errorf("at(0) = %v", got)
	}
	if got := e.at(1); got != 1 {
		t.Errorf("at(1) = %v", got)
	}
	// cubic-bezier(0.25, 0.1, 0.25, 1) at u=0.5 is ~0.8024.
	if got := e.at(0.5); math.Abs(got-0.8024) > 0.01 {
		t.Errorf("at(0.5) = %v; want ~0.8024", got)
	}
	// Monotonic in u.
	prev := -1.0
	for u := 0.0; u <= 1.0; u += 0.05 {
		v := e.at(u)
		if v < prev-1e-9 {
			t.Fatalf("not monotonic at u=%v: %v < %v", u, v, prev)
		}
		prev = v
	}
}

func TestShapeInterpolation(t *testing.T) {
	json := `{
	  "fr": 30, "ip": 0, "op": 30, "w": 10, "h": 10,
	  "layers": [{
	    "ty": 4, "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {},
	    "shapes": [{ "ty": "sh", "ks": { "a": 1, "k": [
	      { "t": 0, "s": [{ "c": true,
	        "v": [[0,0],[10,0],[10,10]],
	        "i": [[0,0],[0,0],[0,0]],
	        "o": [[0,0],[0,0],[0,0]] }] },
	      { "t": 30, "s": [{ "c": true,
	        "v": [[0,0],[20,0],[20,20]],
	        "i": [[0,0],[0,0],[0,0]],
	        "o": [[0,0],[0,0],[0,0]] }] }
	    ] } }]
	  }]
	}`
	anim, err := Decode(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	sh := anim.layers[0].shapes[0]
	if sh.kind != "sh" {
		t.Fatalf("kind = %q", sh.kind)
	}
	var scratch bezierShape
	got := sh.shape.at(15, &scratch)
	if math.Abs(got.V[1][0]-15) > 1e-9 || math.Abs(got.V[2][1]-15) > 1e-9 {
		t.Errorf("shape at 15: V = %v; want V[1][0]=15, V[2][1]=15", got.V)
	}
	if !got.Closed {
		t.Error("interpolated shape lost Closed flag")
	}
}

func TestPlayerLifecycle(t *testing.T) {
	anim := loadTestAnimation(t, "basic.json")
	p := anim.NewPlayer()
	if !p.IsPlaying() {
		t.Error("new player should be playing")
	}
	p.Seek(time.Second)
	if got := p.Position(); math.Abs(got.Seconds()-1) > 1e-9 {
		t.Errorf("Position after Seek(1s) = %v", got)
	}
	// Seek beyond the end clamps and non-loop playback stops there.
	p.SetLoop(false)
	p.Seek(10 * time.Second)
	if got := p.Position(); math.Abs(got.Seconds()-2) > 1e-9 {
		t.Errorf("Position after over-seek = %v; want 2s", got)
	}
	// Loop wrapping.
	p2 := anim.NewPlayer()
	p2.SetLoop(true)
	p2.frame = p2.anim.outPoint + 30 // simulate overshoot
	p2.frame = p2.clampFrame(p2.frame)
	if math.Abs(p2.frame-30) > 1e-9 {
		t.Errorf("loop wrap frame = %v; want 30", p2.frame)
	}
	if !p2.IsPlaying() {
		t.Error("looping player must keep playing")
	}
	p.Pause()
	if p.IsPlaying() {
		t.Error("Pause did not stop playback")
	}
}

func TestWalkShapesCommands(t *testing.T) {
	anim := loadTestAnimation(t, "basic.json")
	var l *layerNode
	for _, cand := range anim.layers {
		if cand.typ == 4 {
			l = cand
		}
	}
	var r renderer
	r.walkShapes(l.shapes, 0, identityMatrix, 1)
	if r.nGeoms != 3 {
		t.Fatalf("nGeoms = %d; want 3 (path, rect, ellipse)", r.nGeoms)
	}
	if len(r.cmds) != 2 {
		t.Fatalf("len(cmds) = %d; want 2 (fill, stroke)", len(r.cmds))
	}
	fill, stroke := r.cmds[0], r.cmds[1]
	if fill.stroke || !stroke.stroke {
		t.Fatalf("command kinds wrong: %+v, %+v", fill, stroke)
	}
	if fill.geomStart != 0 || fill.geomEnd != 3 {
		t.Errorf("fill covers geoms [%d,%d); want [0,3)", fill.geomStart, fill.geomEnd)
	}
	if fill.r != 1 || fill.g != 0 || fill.b != 0 || fill.a != 1 {
		t.Errorf("fill color = %v %v %v %v; want 1 0 0 1", fill.r, fill.g, fill.b, fill.a)
	}
	if math.Abs(stroke.a-0.8) > 1e-9 {
		t.Errorf("stroke alpha = %v; want 0.8", stroke.a)
	}
	if stroke.strokeOpts.Width != 2 {
		t.Errorf("stroke width = %v; want 2", stroke.strokeOpts.Width)
	}
	// Group transform translates geometry by (10,10): triangle top vertex
	// (0,-10) lands at (10,0).
	g := r.geoms[0]
	x, y := g.mat.apply(g.bez.V[0][0], g.bez.V[0][1])
	if math.Abs(x-10) > 1e-9 || math.Abs(y-0) > 1e-9 {
		t.Errorf("triangle top maps to (%v, %v); want (10, 0)", x, y)
	}
}

func TestGeometryShapes(t *testing.T) {
	var b bezierShape
	ellipseShape(&b, 0, 0, 10, 5)
	if len(b.V) != 4 || !b.Closed {
		t.Fatalf("ellipse: %d vertices, closed=%v", len(b.V), b.Closed)
	}
	if b.V[0] != [2]float64{0, -5} || b.V[1] != [2]float64{10, 0} {
		t.Errorf("ellipse vertices: %v", b.V)
	}
	rectShape(&b, 0, 0, 20, 10, 0)
	if len(b.V) != 4 {
		t.Fatalf("sharp rect: %d vertices", len(b.V))
	}
	rectShape(&b, 0, 0, 20, 10, 3)
	if len(b.V) != 8 {
		t.Fatalf("rounded rect: %d vertices", len(b.V))
	}
	// Roundness is clamped to half the short side.
	rectShape(&b, 0, 0, 20, 10, 100)
	for _, v := range b.V {
		if math.Abs(v[0]) > 10+1e-9 || math.Abs(v[1]) > 5+1e-9 {
			t.Fatalf("clamped rounded rect out of bounds: %v", b.V)
		}
	}
}

func TestUnsupportedFeatureRobustness(t *testing.T) {
	// Unknown layer/shape types, masks, effects, expressions: decode must
	// succeed and report them.
	json := `{
	  "fr": 30, "ip": 0, "op": 30, "w": 10, "h": 10,
	  "layers": [
	    { "ty": 13, "ip": 0, "op": 30, "st": 0, "ks": {} },
	    { "ty": 4, "ind": 1, "ip": 0, "op": 30, "st": 0,
	      "ks": { "p": { "a": 0, "k": [0,0], "x": "var $bm_rt = value;" } },
	      "masksProperties": [{}],
	      "tt": 1,
	      "bm": 2,
	      "shapes": [
	        { "ty": "rp", "nm": "repeat" },
	        { "ty": "xyz", "nm": "future" },
	        { "ty": "el", "p": { "a": 0, "k": [0,0] }, "s": { "a": 0, "k": [4,4] } },
	        { "ty": "fl", "c": { "a": 0, "k": [255, 128, 0] }, "o": { "a": 0, "k": 100 } }
	      ] }
	  ]
	}`
	anim, err := Decode(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	unsup := anim.UnsupportedFeatures()
	// Masks (empty entry) and blend mode 2 (screen) are supported now; the
	// matte has no usable source layer (the layer above is a skipped
	// camera), which is reported.
	for _, want := range []string{"camera layer", "track matte without source", `shape type "xyz"`, "expressions"} {
		if !slices.Contains(unsup, want) {
			t.Errorf("UnsupportedFeatures() = %v; missing %q", unsup, want)
		}
	}
	// The supported subset still renders: ellipse + fill produce one command.
	var r renderer
	r.walkShapes(anim.layers[0].shapes, 0, identityMatrix, 1)
	if r.nGeoms != 1 || len(r.cmds) != 1 {
		t.Fatalf("nGeoms=%d cmds=%d; want 1, 1", r.nGeoms, len(r.cmds))
	}
	// 0..255 color heuristic.
	if math.Abs(r.cmds[0].r-1) > 1e-9 || math.Abs(r.cmds[0].g-128.0/255) > 1e-9 {
		t.Errorf("color = %v %v; want 1 %v", r.cmds[0].r, r.cmds[0].g, 128.0/255)
	}
}
