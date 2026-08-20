package lottie

import (
	"archive/zip"
	"bytes"
	"math"
	"slices"
	"strings"
	"testing"
)

func TestPrecompBuild(t *testing.T) {
	json := `{
	  "fr": 30, "ip": 0, "op": 30, "w": 100, "h": 100,
	  "assets": [
	    { "id": "comp1", "layers": [
	      { "ty": 4, "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {},
	        "shapes": [
	          { "ty": "el", "p": { "a": 0, "k": [10, 10] }, "s": { "a": 0, "k": [8, 8] } },
	          { "ty": "fl", "c": { "a": 0, "k": [1, 0, 0] }, "o": { "a": 0, "k": 100 } }
	        ] }
	    ] }
	  ],
	  "layers": [
	    { "ty": 0, "ind": 1, "refId": "comp1", "w": 50, "h": 50,
	      "ip": 0, "op": 30, "st": 0, "ks": {} }
	  ]
	}`
	anim, err := Decode(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	if len(anim.UnsupportedFeatures()) != 0 {
		t.Errorf("unexpected unsupported: %v", anim.UnsupportedFeatures())
	}
	l := anim.layers[0]
	if l.typ != 0 || len(l.comp) != 1 || l.compW != 50 || l.compH != 50 {
		t.Fatalf("precomp not built: typ=%d comp=%d w=%v h=%v", l.typ, len(l.comp), l.compW, l.compH)
	}
	if l.comp[0].shapes == nil {
		t.Fatal("precomp child has no shapes")
	}
}

func TestPrecompCycleDetection(t *testing.T) {
	json := `{
	  "fr": 30, "ip": 0, "op": 30, "w": 100, "h": 100,
	  "assets": [
	    { "id": "a", "layers": [
	      { "ty": 0, "ind": 1, "refId": "b", "w": 10, "h": 10, "ip": 0, "op": 30, "st": 0, "ks": {} }
	    ] },
	    { "id": "b", "layers": [
	      { "ty": 0, "ind": 1, "refId": "a", "w": 10, "h": 10, "ip": 0, "op": 30, "st": 0, "ks": {} }
	    ] }
	  ],
	  "layers": [
	    { "ty": 0, "ind": 1, "refId": "a", "w": 10, "h": 10, "ip": 0, "op": 30, "st": 0, "ks": {} }
	  ]
	}`
	anim, err := Decode(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(anim.UnsupportedFeatures(), "precomposition reference cycle") {
		t.Errorf("cycle not reported: %v", anim.UnsupportedFeatures())
	}
}

func TestGradientStopMerging(t *testing.T) {
	var g gradientCmd
	// Two color stops red -> blue, alpha ramp 1 -> 0.
	data := []float64{
		0, 1, 0, 0,
		1, 0, 0, 1,
		0, 1,
		1, 0,
	}
	buildGradientStops(&g, data, 2, 1)
	if g.count != 2 {
		t.Fatalf("count = %d", g.count)
	}
	if g.colors[0] != [4]float32{1, 0, 0, 1} {
		t.Errorf("stop 0 = %v; want opaque red", g.colors[0])
	}
	if g.colors[1] != [4]float32{0, 0, 0, 0} {
		t.Errorf("stop 1 = %v; want transparent (premultiplied)", g.colors[1])
	}
	// Style opacity scales everything.
	buildGradientStops(&g, data[:8], 2, 0.5)
	if math.Abs(float64(g.colors[0][0])-0.5) > 1e-6 || math.Abs(float64(g.colors[0][3])-0.5) > 1e-6 {
		t.Errorf("stop 0 with opacity = %v; want premultiplied half red", g.colors[0])
	}
}

func TestTrimStraightLine(t *testing.T) {
	// A straight 100-unit horizontal line, trimmed to 25%..75%.
	var r renderer
	g := r.nextGeom()
	g.mat = identityMatrix
	g.bez = bezierShape{
		V: [][2]float64{{0, 0}, {100, 0}},
		I: [][2]float64{{0, 0}, {0, 0}},
		O: [][2]float64{{0, 0}, {0, 0}},
	}
	n := &shapeNode{
		kind:       "tm",
		trimStart:  staticTrack(25),
		trimEnd:    staticTrack(75),
		trimOffset: staticTrack(0),
		trimMode:   1,
	}
	r.applyTrim(n, 0, 0)
	if r.nGeoms != 1 {
		t.Fatalf("nGeoms = %d", r.nGeoms)
	}
	out := r.geoms[0].bez
	first := out.V[0]
	last := out.V[len(out.V)-1]
	if math.Abs(first[0]-25) > 0.5 || math.Abs(last[0]-75) > 0.5 {
		t.Errorf("trimmed span = %v..%v; want 25..75", first[0], last[0])
	}
	if out.Closed {
		t.Error("trimmed path should be open")
	}
}

func TestTrimWrapAround(t *testing.T) {
	// Offset pushes the 0..50% range across the wrap point.
	var r renderer
	g := r.nextGeom()
	g.mat = identityMatrix
	g.bez = bezierShape{
		V: [][2]float64{{0, 0}, {100, 0}},
		I: [][2]float64{{0, 0}, {0, 0}},
		O: [][2]float64{{0, 0}, {0, 0}},
	}
	n := &shapeNode{
		kind:       "tm",
		trimStart:  staticTrack(0),
		trimEnd:    staticTrack(50),
		trimOffset: staticTrack(270), // range becomes 0.75..1.25
		trimMode:   1,
	}
	r.applyTrim(n, 0, 0)
	if r.nGeoms != 2 {
		t.Fatalf("nGeoms = %d; want 2 (wrapped range splits)", r.nGeoms)
	}
}

func TestMaskBuild(t *testing.T) {
	json := `{
	  "fr": 30, "ip": 0, "op": 30, "w": 100, "h": 100,
	  "layers": [
	    { "ty": 4, "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {},
	      "masksProperties": [
	        { "mode": "a", "pt": { "a": 0, "k": { "c": true,
	          "v": [[0,0],[50,0],[50,50]], "i": [[0,0],[0,0],[0,0]], "o": [[0,0],[0,0],[0,0]] } },
	          "o": { "a": 0, "k": 100 } },
	        { "mode": "i", "pt": { "a": 0, "k": { "c": true,
	          "v": [[0,0],[50,0],[50,50]], "i": [[0,0],[0,0],[0,0]], "o": [[0,0],[0,0],[0,0]] } } }
	      ],
	      "shapes": [] }
	  ]
	}`
	anim, err := Decode(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	if len(anim.layers[0].masks) != 1 {
		t.Errorf("masks = %d; want 1 (intersect skipped)", len(anim.layers[0].masks))
	}
	if !slices.Contains(anim.UnsupportedFeatures(), `mask mode "i"`) {
		t.Errorf("intersect mode not reported: %v", anim.UnsupportedFeatures())
	}
}

func TestMatteResolution(t *testing.T) {
	json := `{
	  "fr": 30, "ip": 0, "op": 30, "w": 100, "h": 100,
	  "layers": [
	    { "ty": 4, "ind": 1, "td": 1, "ip": 0, "op": 30, "st": 0, "ks": {}, "shapes": [] },
	    { "ty": 4, "ind": 2, "tt": 1, "ip": 0, "op": 30, "st": 0, "ks": {}, "shapes": [] }
	  ]
	}`
	anim, err := Decode(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	src, tgt := anim.layers[0], anim.layers[1]
	if !src.matteOnly {
		t.Error("matte source not flagged")
	}
	if tgt.matteMode != 1 || tgt.matteSrc != src {
		t.Errorf("matte not resolved: mode=%d src=%v", tgt.matteMode, tgt.matteSrc)
	}
}

func TestEmbeddedImageAsset(t *testing.T) {
	// 1x1 red PNG.
	png1x1 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
	json := `{
	  "fr": 30, "ip": 0, "op": 30, "w": 100, "h": 100,
	  "assets": [
	    { "id": "img0", "w": 1, "h": 1, "u": "", "e": 1,
	      "p": "data:image/png;base64,` + png1x1 + `" }
	  ],
	  "layers": [
	    { "ty": 2, "ind": 1, "refId": "img0", "ip": 0, "op": 30, "st": 0, "ks": {} }
	  ]
	}`
	anim, err := Decode(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	if len(anim.UnsupportedFeatures()) != 0 {
		t.Errorf("unexpected unsupported: %v", anim.UnsupportedFeatures())
	}
	if anim.layers[0].img == nil {
		t.Fatal("embedded image not decoded")
	}
	w, h := anim.layers[0].img.Bounds().Dx(), anim.layers[0].img.Bounds().Dy()
	if w != 1 || h != 1 {
		t.Errorf("image size = %dx%d; want 1x1", w, h)
	}
}

func TestDecodeDotLottie(t *testing.T) {
	animJSON := `{
	  "fr": 30, "ip": 0, "op": 30, "w": 64, "h": 64,
	  "layers": [
	    { "ty": 4, "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {},
	      "shapes": [
	        { "ty": "el", "p": { "a": 0, "k": [32, 32] }, "s": { "a": 0, "k": [10, 10] } },
	        { "ty": "fl", "c": { "a": 0, "k": [0, 1, 0] }, "o": { "a": 0, "k": 100 } }
	      ] }
	  ]
	}`
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	mf, _ := zw.Create("manifest.json")
	mf.Write([]byte(`{"animations":[{"id":"anim1"}]}`))
	af, _ := zw.Create("animations/anim1.json")
	af.Write([]byte(animJSON))
	zw.Close()

	anim, err := DecodeDotLottie(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if w, h := anim.Size(); w != 64 || h != 64 {
		t.Errorf("Size() = %d, %d", w, h)
	}
	if len(anim.layers) != 1 {
		t.Errorf("layers = %d", len(anim.layers))
	}
	// Unknown id errors cleanly.
	if _, err := DecodeDotLottieAnimation(bytes.NewReader(buf.Bytes()), int64(buf.Len()), "nope"); err == nil {
		t.Error("unknown animation id: got nil error")
	}
	// Garbage input errors cleanly.
	if _, err := DecodeDotLottie(bytes.NewReader([]byte("not a zip")), 9); err == nil {
		t.Error("non-zip input: got nil error")
	}
}
