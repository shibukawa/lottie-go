package lottie

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"math"
	"testing"
)

// Regression tests for the second review round.

// Two phases whose focus bindings switch to each other used to recurse
// deliver -> SetPhase -> changeFocus -> deliver without bound and crash
// with a stack overflow; the binding-depth cap must cut the cycle.
func TestSceneBindingCycleDoesNotOverflow(t *testing.T) {
	s, _ := menuScene(t)
	s.Phases = []ScenePhase{{Name: "a"}, {Name: "b"}}
	start, _ := s.Node("start")
	start.Phase = "a"
	start.Bindings = []SceneBinding{{On: SceneFocusEvent, Do: ScenePhaseAction, Arg: "b"}}
	quit, _ := s.Node("quit")
	quit.Phase = "b"
	quit.Bindings = []SceneBinding{{On: SceneFocusEvent, Do: ScenePhaseAction, Arg: "a"}}
	if errs := s.Validate(); len(errs) > 0 {
		t.Fatalf("scene should validate: %v", errs)
	}
	b := sceneTestBundle(t)
	sp, err := s.NewScenePlayer(func(string) (*Bundle, error) { return b, nil })
	if err != nil {
		t.Fatal(err)
	}
	for range 5 {
		sp.Update() // must not overflow either
	}
}

// A zig-zag ridge count straight from the file must be capped like the
// repeater and dash counts, or one bad value hangs every frame.
func TestZigZagRidgeCountCapped(t *testing.T) {
	var r renderer
	g := r.nextGeom()
	g.mat = identityMatrix
	g.bez = bezierShape{
		V: [][2]float64{{0, 0}, {100, 0}},
		I: make([][2]float64, 2),
		O: make([][2]float64, 2),
	}
	n := &shapeNode{
		kind:     "zz",
		amount:   staticTrack(1),
		zzFreq:   staticTrack(5e6),
		zzPoints: staticTrack(1),
	}
	r.applyZigZag(n, 0, 0)
	if got := len(r.geoms[0].bez.V); got > maxZigZagRidges+2 {
		t.Fatalf("zig-zag emitted %d vertices; cap is %d ridges", got, maxZigZagRidges)
	}
}

// A polystar point count straight from the file must be capped too.
func TestPolystarPointCountCapped(t *testing.T) {
	var b bezierShape
	polystarShape(&b, true, 0, 0, 2e6, 0, 100, 50, 0, 0)
	if got := len(b.V); got > maxPolystarPoints*2 {
		t.Fatalf("polystar emitted %d vertices; cap is %d points", got, maxPolystarPoints)
	}
}

// Shape direction 3 reverses a primitive's contour: the standard
// two-ellipse donut relies on the inner ring winding the other way.
func TestShapeDirectionReversesPrimitives(t *testing.T) {
	anim := decodeLayers(t, `{"ty":4,"ind":1,"ip":0,"op":30,"st":0,"ks":{},"shapes":[
	  {"ty":"el","d":1,"p":{"a":0,"k":[0,0]},"s":{"a":0,"k":[100,100]}},
	  {"ty":"el","d":3,"p":{"a":0,"k":[0,0]},"s":{"a":0,"k":[50,50]}},
	  {"ty":"fl","c":{"a":0,"k":[1,0,0,1]},"o":{"a":0,"k":100}}]}`)
	var r renderer
	r.walkShapes(anim.layers[0].shapes, 0, identityMatrix, 1)
	if r.nGeoms != 2 {
		t.Fatalf("geometries = %d; want 2", r.nGeoms)
	}
	outer := signedContourArea(&r.geoms[0].bez)
	inner := signedContourArea(&r.geoms[1].bez)
	if outer == 0 || inner == 0 {
		t.Fatal("degenerate contours")
	}
	if (outer > 0) == (inner > 0) {
		t.Fatalf("d:3 contour winds the same way as d:1 (outer %v, inner %v); the donut hole fills solid", outer, inner)
	}
}

// A repeater with a single unmoved copy still applies its start opacity,
// as lottie-web does.
func TestRepeaterSingleCopyStartOpacity(t *testing.T) {
	anim := decodeLayers(t, `{"ty":4,"ind":1,"ip":0,"op":30,"st":0,"ks":{},"shapes":[
	  {"ty":"el","p":{"a":0,"k":[0,0]},"s":{"a":0,"k":[10,10]}},
	  {"ty":"fl","c":{"a":0,"k":[1,0,0,1]},"o":{"a":0,"k":100}},
	  {"ty":"rp","c":{"a":0,"k":1},"o":{"a":0,"k":0},
	   "tr":{"so":{"a":0,"k":0},"eo":{"a":0,"k":100}}}]}`)
	var r renderer
	r.walkShapes(anim.layers[0].shapes, 0, identityMatrix, 1)
	if len(r.cmds) == 0 {
		t.Fatal("no draw command emitted")
	}
	if got := r.cmds[0].alphaMul; got != 0 {
		t.Fatalf("single-copy repeater alphaMul = %v; want 0 (start opacity 0)", got)
	}
}

// Fractional copies ceil, as lottie-web does: 2.2 animating copies show 3.
func TestRepeaterCopiesCeil(t *testing.T) {
	anim := decodeLayers(t, `{"ty":4,"ind":1,"ip":0,"op":30,"st":0,"ks":{},"shapes":[
	  {"ty":"el","p":{"a":0,"k":[0,0]},"s":{"a":0,"k":[10,10]}},
	  {"ty":"fl","c":{"a":0,"k":[1,0,0,1]},"o":{"a":0,"k":100}},
	  {"ty":"rp","c":{"a":0,"k":2.2},"o":{"a":0,"k":0},
	   "tr":{"p":{"a":0,"k":[20,0]}}}]}`)
	var r renderer
	r.walkShapes(anim.layers[0].shapes, 0, identityMatrix, 1)
	if got := len(r.cmds); got != 3 {
		t.Fatalf("draw commands = %d; want 3 copies for copies=2.2", got)
	}
}

// A mirror scale with a fractional offset used to make math.Pow return NaN
// and vanish every copy.
func TestRepeaterMirrorScaleStaysFinite(t *testing.T) {
	m := repeaterMatrix(0, 0, 10, 0, 0, -1, 1, 1.5)
	for _, v := range []float64{m.A, m.B, m.C, m.D, m.TX, m.TY} {
		if math.IsNaN(v) {
			t.Fatalf("repeater matrix has NaN: %+v", m)
		}
	}
}

// Effect colors use the same build-time 0..255 decision as fills now.
func TestEffectColorScaleFromAuthoredValues(t *testing.T) {
	anim := decodeLayers(t, `{"ty":4,"ind":1,"ip":0,"op":30,"st":0,"ks":{},
	  "ef":[{"ty":21,"en":1,"ef":[
	    {"v":{"a":0,"k":0}},{"v":{"a":0,"k":0}},
	    {"v":{"a":0,"k":[255,0,0]}},
	    {"v":{"a":0,"k":0}},{"v":{"a":0,"k":0}},{"v":{"a":0,"k":0}},
	    {"v":{"a":0,"k":100}}]}],
	  "shapes":[{"ty":"el","p":{"a":0,"k":[0,0]},"s":{"a":0,"k":[10,10]}},
	    {"ty":"fl","c":{"a":0,"k":[1,1,1,1]},"o":{"a":0,"k":100}}]}`)
	if len(anim.layers[0].effects) != 1 {
		t.Fatalf("effects = %d; want 1", len(anim.layers[0].effects))
	}
	e := &anim.layers[0].effects[0]
	cr, cg, cb, _ := e.colorAt(2, 0)
	if !near(cr, 1) || !near(cg, 0) || !near(cb, 0) {
		t.Fatalf("0..255 effect color read back as (%v, %v, %v); want (1, 0, 0)", cr, cg, cb)
	}
}

// Encode must carry over archive members it does not model — a v1 themes
// directory, a stray readme — instead of silently dropping them.
func TestBundleEncodePreservesUnknownMembers(t *testing.T) {
	var src bytes.Buffer
	zw := zip.NewWriter(&src)
	for name, data := range map[string]string{
		"manifest.json":    `{"version":"2","animations":[{"id":"anim"}]}`,
		"a/anim.json":      string(clipAnimation(10, "")),
		"themes/dark.json": `{"bg":"#000"}`,
		"states/m.json":    `{"initial":"x"}`,
		"readme.txt":       "hello",
	} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := DecodeBundle(bytes.NewReader(src.Bytes()), int64(src.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := b.Encode(&out); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, f := range zr.File {
		got[f.Name] = true
	}
	if !got["t/dark.json"] {
		t.Error("v1 themes/dark.json neither mapped to t/ nor preserved")
	}
	for _, name := range []string{"states/m.json", "readme.txt"} {
		if !got[name] {
			t.Errorf("member %s dropped on the Decode -> Encode round trip", name)
		}
	}
}

// A dash running through a closed contour's seam must come out as one
// piece, joined the way trims are.
func TestDashSeamJoined(t *testing.T) {
	var r renderer
	g := r.nextGeom()
	g.mat = identityMatrix
	g.bez = bezierShape{
		Closed: true,
		V:      [][2]float64{{0, 0}, {10, 0}, {10, 10}, {0, 10}},
		I:      make([][2]float64, 4),
		O:      make([][2]float64, 4),
	}
	n := &shapeNode{
		dashPattern: []*vectorTrack{staticTrack(25), staticTrack(5)},
		dashOffset:  staticTrack(10),
	}
	start, end := r.buildDashedRange(n, 0, 0, 1)
	// Perimeter 40, pattern on 25 / gap 5, phase 10: visible ranges are
	// [0,15] and [20,40], and the latter runs through the seam into the
	// former — one joined piece from arc 20 around to arc 15.
	if end-start != 1 {
		t.Fatalf("dash pieces = %d; want 1 joined through the seam", end-start)
	}
	b := &r.dashGeoms[start].bez
	first, last := b.V[0], b.V[len(b.V)-1]
	if !near(first[0], 10) || !near(first[1], 10) {
		t.Errorf("joined dash starts at %v; want (10, 10), arc position 20", first)
	}
	if !near(last[0], 10) || !near(last[1], 5) {
		t.Errorf("joined dash ends at %v; want (10, 5), arc position 15", last)
	}
}

// scalarAt is a dedicated scalar path now; it must agree with at().
func TestScalarAtMatchesAt(t *testing.T) {
	var p rawProp
	if err := json.Unmarshal([]byte(`{"a":1,"k":[
	  {"t":0,"s":[3],"o":{"x":0.4,"y":1.6},"i":{"x":0.6,"y":1}},
	  {"t":10,"s":[7],"h":1},
	  {"t":20,"s":[1]},
	  {"t":30}]}`), &p); err != nil {
		t.Fatal(err)
	}
	tr, err := parseVectorProp(&p, []float64{0})
	if err != nil {
		t.Fatal(err)
	}
	for f := -2.0; f <= 32; f += 0.25 {
		want := 0.0
		if v := tr.at(f, nil); len(v) > 0 {
			want = v[0]
		}
		if got := tr.scalarAt(f, 0); !near(got, want) {
			t.Fatalf("scalarAt(%v) = %v; at gives %v", f, got, want)
		}
	}
}
