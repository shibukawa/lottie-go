package lottie

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestTimeRemapBuild(t *testing.T) {
	json := `{
	  "fr": 30, "ip": 0, "op": 60, "w": 100, "h": 100,
	  "assets": [
	    { "id": "comp1", "layers": [
	      { "ty": 4, "ind": 1, "ip": 0, "op": 60, "st": 0, "ks": {}, "shapes": [] }
	    ] }
	  ],
	  "layers": [
	    { "ty": 0, "ind": 1, "refId": "comp1", "w": 100, "h": 100,
	      "ip": 0, "op": 60, "st": 0, "ks": {},
	      "tm": { "a": 1, "k": [
	        { "t": 0, "s": [2], "o": { "x": 0, "y": 0 }, "i": { "x": 1, "y": 1 } },
	        { "t": 60, "s": [0] }
	      ] } }
	  ]
	}`
	anim, err := Decode(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	if unsup := anim.UnsupportedFeatures(); len(unsup) != 0 {
		t.Errorf("UnsupportedFeatures() = %v; want none", unsup)
	}
	l := anim.layers[0]
	if l.timeRemap == nil {
		t.Fatal("time remap not built")
	}
	// The track maps 2s -> 0s linearly, so halfway through it reads 1s,
	// which is frame 30 at 30fps: the precomp plays backwards.
	if got := l.compTime(30, anim.frameRate); math.Abs(got-30) > 1e-9 {
		t.Errorf("compTime(30) = %v; want 30", got)
	}
	if got := l.compTime(0, anim.frameRate); math.Abs(got-60) > 1e-9 {
		t.Errorf("compTime(0) = %v; want 60", got)
	}
	if got := l.compTime(60, anim.frameRate); math.Abs(got-0) > 1e-9 {
		t.Errorf("compTime(60) = %v; want 0", got)
	}
}

func TestTimeRemapAbsent(t *testing.T) {
	l := &layerNode{}
	if got := l.compTime(12.5, 30); got != 12.5 {
		t.Errorf("compTime without tm = %v; want 12.5", got)
	}
}

func TestBlendModeSupport(t *testing.T) {
	decode := func(bm int) *Animation {
		json := `{
		  "fr": 30, "ip": 0, "op": 30, "w": 100, "h": 100,
		  "layers": [
		    { "ty": 4, "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {}, "bm": ` +
			strconv.Itoa(bm) + `, "shapes": [] }
		  ]
		}`
		anim, err := Decode(strings.NewReader(json))
		if err != nil {
			t.Fatal(err)
		}
		return anim
	}
	// Darken samples the backdrop: no note, shader route, offscreen root.
	anim := decode(4)
	if unsup := anim.UnsupportedFeatures(); len(unsup) != 0 {
		t.Errorf("darken reported unsupported: %v", unsup)
	}
	if !anim.rootShaderBlend {
		t.Error("rootShaderBlend = false for a darken root layer")
	}
	if anim.snapshotOK {
		t.Error("snapshotOK = true for a non-normal blend")
	}
	// Add is fixed-function: no note, no shader route.
	anim = decode(16)
	if unsup := anim.UnsupportedFeatures(); len(unsup) != 0 {
		t.Errorf("add reported unsupported: %v", unsup)
	}
	if anim.rootShaderBlend {
		t.Error("rootShaderBlend = true for add, which is fixed-function")
	}
	// Hue is not implemented and stays reported.
	anim = decode(12)
	if !slices.Contains(anim.UnsupportedFeatures(), "blend mode 12") {
		t.Errorf("hue not reported: %v", anim.UnsupportedFeatures())
	}
}

func TestBlendShaderCompiles(t *testing.T) {
	ensureBlendShader()
	if blendShader == nil {
		t.Fatal("blend shader did not compile")
	}
}

func TestPuckerBloatCollapse(t *testing.T) {
	var r renderer
	g := r.nextGeom()
	g.mat = identityMatrix
	g.bez = bezierShape{
		Closed: true,
		V:      [][2]float64{{0, 0}, {100, 0}, {100, 100}, {0, 100}},
		I:      [][2]float64{{0, 0}, {0, 0}, {0, 0}, {0, 0}},
		O:      [][2]float64{{0, 0}, {0, 0}, {0, 0}, {0, 0}},
	}
	n := &shapeNode{kind: "pb", amount: staticTrack(100)}
	r.applyPuckerBloat(n, 0, 0)
	// At 100% every vertex sits on the centroid and the tangents reach back
	// out to where the vertex was.
	for i, v := range r.geoms[0].bez.V {
		if math.Abs(v[0]-50) > 1e-9 || math.Abs(v[1]-50) > 1e-9 {
			t.Errorf("vertex %d = %v; want (50, 50)", i, v)
		}
	}
	// The first vertex was (0,0); its tangent endpoints were also (0,0), and
	// they move away from the center by the full amount: to (-50, -50)
	// relative to the collapsed vertex.
	in := r.geoms[0].bez.I[0]
	if math.Abs(in[0]+100) > 1e-9 || math.Abs(in[1]+100) > 1e-9 {
		t.Errorf("in tangent 0 = %v; want (-100, -100)", in)
	}
}

func TestPuckerBloatZeroIsNoop(t *testing.T) {
	var r renderer
	g := r.nextGeom()
	g.mat = identityMatrix
	g.bez = bezierShape{V: [][2]float64{{1, 2}, {3, 4}}, I: [][2]float64{{0, 0}, {0, 0}}, O: [][2]float64{{0, 0}, {0, 0}}}
	n := &shapeNode{kind: "pb", amount: staticTrack(0)}
	r.applyPuckerBloat(n, 0, 0)
	if v := r.geoms[0].bez.V[0]; v != [2]float64{1, 2} {
		t.Errorf("vertex moved with zero amount: %v", v)
	}
}

func TestZigZagLine(t *testing.T) {
	var r renderer
	g := r.nextGeom()
	g.mat = identityMatrix
	g.bez = bezierShape{
		V: [][2]float64{{0, 0}, {100, 0}},
		I: [][2]float64{{0, 0}, {0, 0}},
		O: [][2]float64{{0, 0}, {0, 0}},
	}
	n := &shapeNode{
		kind:     "zz",
		amount:   staticTrack(10),
		zzFreq:   staticTrack(1),
		zzPoints: staticTrack(1),
	}
	r.applyZigZag(n, 0, 0)
	out := r.geoms[0].bez
	// One segment with one interior ridge: start, middle, end, offset
	// alternately along the downward-then-upward normal.
	want := [][2]float64{{0, -10}, {50, 10}, {100, -10}}
	if len(out.V) != len(want) {
		t.Fatalf("points = %d; want %d (%v)", len(out.V), len(want), out.V)
	}
	for i := range want {
		if math.Abs(out.V[i][0]-want[i][0]) > 1e-9 || math.Abs(out.V[i][1]-want[i][1]) > 1e-9 {
			t.Errorf("point %d = %v; want %v", i, out.V[i], want[i])
		}
	}
	// Corner points carry no tangents.
	for i, o := range out.O {
		if o != ([2]float64{}) {
			t.Errorf("out tangent %d = %v; want zero", i, o)
		}
	}
}

func TestZigZagSmoothTangents(t *testing.T) {
	var r renderer
	g := r.nextGeom()
	g.mat = identityMatrix
	g.bez = bezierShape{
		V: [][2]float64{{0, 0}, {100, 0}},
		I: [][2]float64{{0, 0}, {0, 0}},
		O: [][2]float64{{0, 0}, {0, 0}},
	}
	n := &shapeNode{
		kind:     "zz",
		amount:   staticTrack(10),
		zzFreq:   staticTrack(1),
		zzPoints: staticTrack(2),
	}
	r.applyZigZag(n, 0, 0)
	out := r.geoms[0].bez
	if len(out.V) != 3 {
		t.Fatalf("points = %d; want 3", len(out.V))
	}
	// The middle ridge's tangent follows its neighbors: (100,0)-(0,0) gives
	// a quarter delta of (25, 0).
	if o := out.O[1]; math.Abs(o[0]-25) > 1e-9 || math.Abs(o[1]) > 1e-9 {
		t.Errorf("smooth out tangent = %v; want (25, 0)", o)
	}
}

func TestShapeModifierBuild(t *testing.T) {
	json := `{
	  "fr": 30, "ip": 0, "op": 30, "w": 100, "h": 100,
	  "layers": [
	    { "ty": 4, "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {},
	      "shapes": [
	        { "ty": "el", "p": { "a": 0, "k": [50, 50] }, "s": { "a": 0, "k": [40, 40] } },
	        { "ty": "pb", "a": { "a": 0, "k": 30 } },
	        { "ty": "zz", "s": { "a": 0, "k": 5 }, "r": { "a": 0, "k": 3 }, "pt": { "a": 0, "k": 2 } },
	        { "ty": "fl", "c": { "a": 0, "k": [1, 0, 0] }, "o": { "a": 0, "k": 100 } }
	      ] }
	  ]
	}`
	anim, err := Decode(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	if unsup := anim.UnsupportedFeatures(); len(unsup) != 0 {
		t.Errorf("UnsupportedFeatures() = %v; want none", unsup)
	}
	shapes := anim.layers[0].shapes
	if shapes[1].kind != "pb" || shapes[1].amount.scalarAt(0, 0) != 30 {
		t.Errorf("pb not built: %+v", shapes[1])
	}
	if shapes[2].kind != "zz" || shapes[2].amount.scalarAt(0, 0) != 5 || shapes[2].zzFreq.scalarAt(0, 0) != 3 {
		t.Errorf("zz not built: %+v", shapes[2])
	}
	// The modified shape still renders: one geometry, one fill command.
	var r renderer
	r.walkShapes(anim.layers[0].shapes, 0, identityMatrix, 1)
	if r.nGeoms != 1 || len(r.cmds) != 1 {
		t.Errorf("nGeoms=%d cmds=%d; want 1, 1", r.nGeoms, len(r.cmds))
	}
}

func TestEffectsBuild(t *testing.T) {
	json := `{
	  "fr": 30, "ip": 0, "op": 30, "w": 100, "h": 100,
	  "layers": [
	    { "ty": 4, "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {},
	      "ef": [
	        { "ty": 29, "nm": "Gaussian Blur", "ef": [
	          { "ty": 0, "nm": "Blurriness", "v": { "a": 0, "k": 10 } },
	          { "ty": 7, "nm": "Blur Dimensions", "v": { "a": 0, "k": 1 } },
	          { "ty": 7, "nm": "Repeat Edge Pixels", "v": { "a": 0, "k": 0 } }
	        ] },
	        { "ty": 25, "nm": "Drop Shadow", "en": 0, "ef": [] },
	        { "ty": 5, "nm": "Slider Control", "ef": [
	          { "ty": 0, "nm": "Slider", "v": { "a": 0, "k": 3 } }
	        ] },
	        { "ty": 26, "nm": "Radial Wipe", "ef": [] }
	      ],
	      "shapes": [] }
	  ]
	}`
	anim, err := Decode(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	l := anim.layers[0]
	// The blur survives; the disabled shadow and the inert slider drop out.
	if len(l.effects) != 1 || l.effects[0].kind != effectBlur {
		t.Fatalf("effects = %+v; want one gaussian blur", l.effects)
	}
	if got := l.effects[0].scalar(0, 0, 0); got != 10 {
		t.Errorf("blurriness = %v; want 10", got)
	}
	if !slices.Contains(anim.UnsupportedFeatures(), `effect "Radial Wipe"`) {
		t.Errorf("radial wipe not reported: %v", anim.UnsupportedFeatures())
	}
	if slices.Contains(anim.UnsupportedFeatures(), "effects") {
		t.Errorf("blanket effects note still present: %v", anim.UnsupportedFeatures())
	}
	// Bounds pad: 3 sigma of blurriness 10 at scale 1.
	pad := effectPad(l, 0, 1)
	if math.Abs(pad-10*blurrinessToSigma*3) > 1e-9 {
		t.Errorf("effectPad = %v; want %v", pad, 10*blurrinessToSigma*3)
	}
	// A layer with effects leaves the atlas phases to the extra passes.
	if l.phaseOK {
		t.Error("phaseOK = true for a layer with effects")
	}
}

func TestEffectColorParam(t *testing.T) {
	e := &effectNode{kind: effectFill, params: []*vectorTrack{
		nil, nil, staticTrack(1, 0.5, 0.25, 1), nil, nil, nil, staticTrack(0.8),
	}}
	r, g, b, a := e.colorAt(2, 0)
	if r != 1 || g != 0.5 || b != 0.25 || a != 1 {
		t.Errorf("colorAt = %v,%v,%v,%v", r, g, b, a)
	}
	if got := e.scalar(6, 0, 1); got != 0.8 {
		t.Errorf("opacity = %v; want 0.8", got)
	}
	// Missing parameters fall back to the default.
	if got := e.scalar(9, 0, 5); got != 5 {
		t.Errorf("missing param = %v; want default 5", got)
	}
}

func TestEffectShadersCompile(t *testing.T) {
	compileEffectShaders()
	if fillShaderVal == nil || tintShaderVal == nil || tritoneShaderVal == nil || blurShaderVal == nil {
		t.Fatal("effect shaders did not compile")
	}
}

func TestMaskCoverageBaseModes(t *testing.T) {
	// Build-level check that intersect and inverted survive into maskNode
	// and that subtract-only masks still build (they now start from full
	// coverage at render time).
	json := `{
	  "fr": 30, "ip": 0, "op": 30, "w": 100, "h": 100,
	  "layers": [
	    { "ty": 4, "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {},
	      "masksProperties": [
	        { "mode": "s", "inv": true, "pt": { "a": 0, "k": { "c": true,
	          "v": [[0,0],[50,0],[50,50]], "i": [[0,0],[0,0],[0,0]], "o": [[0,0],[0,0],[0,0]] } },
	          "o": { "a": 0, "k": 50 } }
	      ],
	      "shapes": [] }
	  ]
	}`
	anim, err := Decode(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	if unsup := anim.UnsupportedFeatures(); len(unsup) != 0 {
		t.Errorf("UnsupportedFeatures() = %v; want none", unsup)
	}
	m := anim.layers[0].masks
	if len(m) != 1 || m[0].mode != 's' || !m[0].inverted {
		t.Fatalf("mask = %+v; want inverted subtract", m)
	}
}
