package lottie

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

func TestPolystarGeometry(t *testing.T) {
	var b bezierShape
	polystarShape(&b, true, 0, 0, 5, 0, 100, 50, 0, 0)
	if len(b.V) != 10 || !b.Closed {
		t.Fatalf("star: %d verts closed=%v; want 10, true", len(b.V), b.Closed)
	}
	// First vertex is the top outer point.
	if math.Abs(b.V[0][0]) > 1e-9 || math.Abs(b.V[0][1]+100) > 1e-9 {
		t.Errorf("star top vertex = %v; want (0,-100)", b.V[0])
	}
	// Alternating radii.
	r1 := math.Hypot(b.V[1][0], b.V[1][1])
	if math.Abs(r1-50) > 1e-9 {
		t.Errorf("inner vertex radius = %v; want 50", r1)
	}
	polystarShape(&b, false, 0, 0, 6, 0, 80, 0, 0, 0)
	if len(b.V) != 6 {
		t.Fatalf("polygon: %d verts; want 6", len(b.V))
	}
	// Roundness produces non-zero tangents.
	polystarShape(&b, false, 0, 0, 6, 0, 80, 0, 50, 0)
	if isZeroPt(b.O[0]) {
		t.Error("rounded polygon should have tangents")
	}
}

func TestRepeater(t *testing.T) {
	json := `{
	  "fr": 30, "ip": 0, "op": 30, "w": 200, "h": 100,
	  "layers": [{
	    "ty": 4, "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {},
	    "shapes": [{ "ty": "gr", "it": [
	      { "ty": "rc", "p": { "a": 0, "k": [10, 10] }, "s": { "a": 0, "k": [10, 10] }, "r": { "a": 0, "k": 0 } },
	      { "ty": "fl", "c": { "a": 0, "k": [1, 0, 0] }, "o": { "a": 0, "k": 100 } },
	      { "ty": "rp",
	        "c": { "a": 0, "k": 3 }, "o": { "a": 0, "k": 0 },
	        "tr": { "p": { "a": 0, "k": [30, 0] },
	          "so": { "a": 0, "k": 100 }, "eo": { "a": 0, "k": 20 } } }
	    ] }]
	  }]
	}`
	anim, err := Decode(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	if unsup := anim.UnsupportedFeatures(); len(unsup) != 0 {
		t.Fatalf("unsupported: %v", unsup)
	}
	var r renderer
	r.walkShapes(anim.layers[0].shapes, 0, identityMatrix, 1)
	if r.nGeoms != 3 || len(r.cmds) != 3 {
		t.Fatalf("nGeoms=%d cmds=%d; want 3, 3", r.nGeoms, len(r.cmds))
	}
	// Copies shift by 30px in x each; centers at 10, 40, 70.
	for k := 0; k < 3; k++ {
		g := &r.geoms[k]
		cx, _ := g.mat.apply(10, 10)
		want := 10 + 30*float64(k)
		if math.Abs(cx-want) > 1e-9 {
			t.Errorf("copy %d center x = %v; want %v", k, cx, want)
		}
	}
	// Opacity ramp 100 -> 20.
	wantAlpha := []float64{1, 0.6, 0.2}
	for k, c := range r.cmds {
		if math.Abs(c.alphaMul-wantAlpha[k]) > 1e-9 {
			t.Errorf("copy %d alphaMul = %v; want %v", k, c.alphaMul, wantAlpha[k])
		}
	}
	// Draw order: reversed execution draws the last copy first, so the
	// original (copy 0, cmds[0]) ends up on top.
}

func TestRoundCornersModifier(t *testing.T) {
	json := `{
	  "fr": 30, "ip": 0, "op": 30, "w": 100, "h": 100,
	  "layers": [{
	    "ty": 4, "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {},
	    "shapes": [
	      { "ty": "sh", "ks": { "a": 0, "k": { "c": true,
	        "v": [[0,0],[40,0],[40,40],[0,40]],
	        "i": [[0,0],[0,0],[0,0],[0,0]],
	        "o": [[0,0],[0,0],[0,0],[0,0]] } } },
	      { "ty": "rd", "r": { "a": 0, "k": 5 } },
	      { "ty": "fl", "c": { "a": 0, "k": [0,0,0] }, "o": { "a": 0, "k": 100 } }
	    ]
	  }]
	}`
	anim, err := Decode(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	var r renderer
	r.walkShapes(anim.layers[0].shapes, 0, identityMatrix, 1)
	if r.nGeoms != 1 {
		t.Fatalf("nGeoms = %d", r.nGeoms)
	}
	b := &r.geoms[0].bez
	if len(b.V) != 8 {
		t.Fatalf("rounded square verts = %d; want 8", len(b.V))
	}
	// Every emitted vertex stays on the square's edges, pulled back 5 from
	// each corner.
	for _, v := range b.V {
		onEdge := v[0] == 0 || v[0] == 40 || v[1] == 0 || v[1] == 40
		if !onEdge {
			t.Errorf("vertex %v not on square edge", v)
		}
	}
}

func TestDashedStroke(t *testing.T) {
	json := `{
	  "fr": 30, "ip": 0, "op": 30, "w": 200, "h": 100,
	  "layers": [{
	    "ty": 4, "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {},
	    "shapes": [
	      { "ty": "sh", "ks": { "a": 0, "k": { "c": false,
	        "v": [[0,0],[100,0]], "i": [[0,0],[0,0]], "o": [[0,0],[0,0]] } } },
	      { "ty": "st", "c": { "a": 0, "k": [0,0,0] }, "o": { "a": 0, "k": 100 },
	        "w": { "a": 0, "k": 2 },
	        "d": [
	          { "n": "d", "v": { "a": 0, "k": 10 } },
	          { "n": "g", "v": { "a": 0, "k": 10 } },
	          { "n": "o", "v": { "a": 0, "k": 0 } }
	        ] }
	    ]
	  }]
	}`
	anim, err := Decode(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	if unsup := anim.UnsupportedFeatures(); len(unsup) != 0 {
		t.Fatalf("unsupported: %v", unsup)
	}
	var r renderer
	r.walkShapes(anim.layers[0].shapes, 0, identityMatrix, 1)
	if len(r.cmds) != 1 || !r.cmds[0].dashed {
		t.Fatalf("expected one dashed command, got %+v", r.cmds)
	}
	c := r.cmds[0]
	// 100-unit line with 10/10 dashing: 5 dashes.
	if c.geomEnd-c.geomStart != 5 {
		t.Fatalf("dash count = %d; want 5", c.geomEnd-c.geomStart)
	}
	first := r.dashGeoms[c.geomStart].bez
	if math.Abs(first.V[0][0]-0) > 0.5 || math.Abs(first.V[len(first.V)-1][0]-10) > 0.5 {
		t.Errorf("first dash spans %v..%v; want 0..10", first.V[0][0], first.V[len(first.V)-1][0])
	}
}

func TestAutoOrient(t *testing.T) {
	json := `{
	  "fr": 30, "ip": 0, "op": 30, "w": 100, "h": 100,
	  "layers": [{
	    "ty": 4, "ind": 1, "ao": 1, "ip": 0, "op": 30, "st": 0,
	    "ks": { "p": { "a": 1, "k": [
	      { "t": 0, "s": [0, 0] }, { "t": 30, "s": [30, 30] }
	    ] } },
	    "shapes": []
	  }]
	}`
	anim, err := Decode(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	l := anim.layers[0]
	if !l.autoOrient {
		t.Fatal("autoOrient not set")
	}
	// Motion at 45 degrees: local +x axis should map to (cos45, sin45).
	m := layerMatrix(l, 15, 0)
	ox, oy := m.apply(0, 0)
	px, py := m.apply(1, 0)
	dx, dy := px-ox, py-oy
	want := math.Sqrt(2) / 2
	if math.Abs(dx-want) > 1e-6 || math.Abs(dy-want) > 1e-6 {
		t.Errorf("oriented x-axis = (%v, %v); want (%v, %v)", dx, dy, want, want)
	}
}

func TestTextLayerBuild(t *testing.T) {
	json := `{
	  "fr": 30, "ip": 0, "op": 60, "w": 200, "h": 100,
	  "fonts": { "list": [
	    { "fName": "Roboto-Bold", "fFamily": "Roboto", "fStyle": "Bold" }
	  ] },
	  "layers": [{
	    "ty": 5, "ind": 1, "ip": 0, "op": 60, "st": 0, "ks": {},
	    "t": { "d": { "k": [
	      { "t": 0, "s": { "t": "Hi", "f": "Roboto-Bold", "s": 24,
	        "fc": [1, 0, 0], "j": 2, "lh": 29 } },
	      { "t": 30, "s": { "t": "Bye", "f": "Roboto-Bold", "s": 24,
	        "fc": [1, 0, 0], "j": 2, "lh": 29 } }
	    ] } }
	  }]
	}`
	anim, err := Decode(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(anim.UnsupportedFeatures(), textResolverNote) {
		t.Errorf("expected resolver note, got %v", anim.UnsupportedFeatures())
	}
	anim.SetFontResolver(func(family, style string) *text.GoTextFaceSource { return nil })
	if slices.Contains(anim.UnsupportedFeatures(), textResolverNote) {
		t.Error("resolver note not cleared by SetFontResolver")
	}
	tn := anim.layers[0].text
	if tn == nil {
		t.Fatal("text node not built")
	}
	doc := tn.docAt(15)
	if doc.text != "Hi" || doc.family != "Roboto" || doc.style != "Bold" || doc.justify != 2 {
		t.Errorf("doc at 15 = %+v", doc)
	}
	if got := tn.docAt(45); got.text != "Bye" {
		t.Errorf("doc at 45 = %+v; want Bye (hold keyframes)", got)
	}
}
