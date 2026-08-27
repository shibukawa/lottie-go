package lottie

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2/vector"
)

func closedSquare(x0, y0, x1, y1 float64) bezierShape {
	return bezierShape{
		Closed: true,
		V:      [][2]float64{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}},
		I:      [][2]float64{{0, 0}, {0, 0}, {0, 0}, {0, 0}},
		O:      [][2]float64{{0, 0}, {0, 0}, {0, 0}, {0, 0}},
	}
}

func bezBounds(b *bezierShape) (minX, minY, maxX, maxY float64) {
	minX, minY = math.Inf(1), math.Inf(1)
	maxX, maxY = math.Inf(-1), math.Inf(-1)
	for _, v := range b.V {
		minX, minY = math.Min(minX, v[0]), math.Min(minY, v[1])
		maxX, maxY = math.Max(maxX, v[0]), math.Max(maxY, v[1])
	}
	return
}

func TestOffsetPathExpandsEitherWinding(t *testing.T) {
	for _, reversed := range []bool{false, true} {
		var r renderer
		g := r.nextGeom()
		g.mat = identityMatrix
		g.bez = closedSquare(20, 20, 80, 80)
		if reversed {
			reverseContour(&g.bez)
		}
		n := &shapeNode{kind: "op", amount: staticTrack(10)}
		r.applyOffsetPath(n, 0, 0)
		minX, minY, maxX, maxY := bezBounds(&r.geoms[0].bez)
		// A positive amount must grow the square by ~10 on every side,
		// regardless of the contour's direction.
		if math.Abs(minX-10) > 1.5 || math.Abs(minY-10) > 1.5 ||
			math.Abs(maxX-90) > 1.5 || math.Abs(maxY-90) > 1.5 {
			t.Errorf("reversed=%v: offset bounds = (%v,%v)-(%v,%v); want ~(10,10)-(90,90)",
				reversed, minX, minY, maxX, maxY)
		}
	}
}

func TestOffsetPathContracts(t *testing.T) {
	var r renderer
	g := r.nextGeom()
	g.mat = identityMatrix
	g.bez = closedSquare(20, 20, 80, 80)
	n := &shapeNode{kind: "op", amount: staticTrack(-10)}
	r.applyOffsetPath(n, 0, 0)
	// Sample-based inward offsetting leaves zero-area folds near corners,
	// so measure the fill-relevant result: the left and right edges across
	// the middle band must sit at ~30 and ~70, and the corner miter must
	// land at (30, 30).
	out := &r.geoms[0].bez
	minX, maxX := math.Inf(1), math.Inf(-1)
	for _, v := range out.V {
		if v[1] > 45 && v[1] < 55 {
			minX = math.Min(minX, v[0])
			maxX = math.Max(maxX, v[0])
		}
	}
	if math.Abs(minX-30) > 1.5 || math.Abs(maxX-70) > 1.5 {
		t.Errorf("mid-band edges = %v..%v; want ~30..70", minX, maxX)
	}
	corner := out.V[0]
	if math.Abs(corner[0]-30) > 1.5 || math.Abs(corner[1]-30) > 1.5 {
		t.Errorf("corner miter = %v; want ~(30, 30)", corner)
	}
}

func TestSignedAreaAndReverse(t *testing.T) {
	sq := closedSquare(0, 0, 10, 10)
	// The square above winds clockwise with Y down: positive area.
	if a := signedContourArea(&sq); a <= 0 {
		t.Errorf("clockwise area = %v; want positive", a)
	}
	reverseContour(&sq)
	if a := signedContourArea(&sq); a >= 0 {
		t.Errorf("reversed area = %v; want negative", a)
	}
	// An ellipse (curved segments) keeps the same convention.
	var el bezierShape
	ellipseShape(&el, 0, 0, 10, 10)
	if a := signedContourArea(&el); a <= 0 {
		t.Errorf("ellipse area = %v; want positive", a)
	}
}

func TestMergeSubtractReversesLaterContours(t *testing.T) {
	var r renderer
	a := r.nextGeom()
	a.mat = identityMatrix
	a.bez = closedSquare(0, 0, 100, 100)
	b := r.nextGeom()
	b.mat = identityMatrix
	b.bez = closedSquare(25, 25, 75, 75)
	n := &shapeNode{kind: "mm", mergeMode: 3}
	r.applyMerge(n, 0)
	if signedContourArea(&r.geoms[0].bez) <= 0 {
		t.Error("first contour should stay clockwise")
	}
	if signedContourArea(&r.geoms[1].bez) >= 0 {
		t.Error("second contour should be reversed for subtraction")
	}
}

func TestMergeExcludeUsesEvenOdd(t *testing.T) {
	json := `{
	  "fr": 30, "ip": 0, "op": 30, "w": 100, "h": 100,
	  "layers": [
	    { "ty": 4, "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {},
	      "shapes": [
	        { "ty": "rc", "p": { "a": 0, "k": [40, 40] }, "s": { "a": 0, "k": [40, 40] }, "r": { "a": 0, "k": 0 } },
	        { "ty": "rc", "p": { "a": 0, "k": [60, 60] }, "s": { "a": 0, "k": [40, 40] }, "r": { "a": 0, "k": 0 } },
	        { "ty": "mm", "mm": 5 },
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
	var r renderer
	r.walkShapes(anim.layers[0].shapes, 0, identityMatrix, 1)
	if len(r.cmds) != 1 {
		t.Fatalf("cmds = %d; want 1", len(r.cmds))
	}
	if r.cmds[0].fillRule != vector.FillRuleEvenOdd {
		t.Errorf("fill rule = %v; want even-odd for exclude-intersections", r.cmds[0].fillRule)
	}
}

func TestMergeIntersectStillReported(t *testing.T) {
	json := `{
	  "fr": 30, "ip": 0, "op": 30, "w": 100, "h": 100,
	  "layers": [
	    { "ty": 4, "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {},
	      "shapes": [ { "ty": "mm", "mm": 4 } ] }
	  ]
	}`
	anim, err := Decode(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(anim.UnsupportedFeatures(), "merge paths mode 4") {
		t.Errorf("intersect not reported: %v", anim.UnsupportedFeatures())
	}
}

func TestMaskExpansionBuild(t *testing.T) {
	json := `{
	  "fr": 30, "ip": 0, "op": 30, "w": 100, "h": 100,
	  "layers": [
	    { "ty": 4, "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {},
	      "masksProperties": [
	        { "mode": "a", "o": { "a": 0, "k": 100 }, "x": { "a": 0, "k": 12 },
	          "pt": { "a": 0, "k": { "c": true,
	            "v": [[0,0],[50,0],[50,50]], "i": [[0,0],[0,0],[0,0]], "o": [[0,0],[0,0],[0,0]] } } }
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
	if len(m) != 1 || m[0].expansion == nil || m[0].expansion.scalarAt(0, 0) != 12 {
		t.Fatalf("mask expansion not built: %+v", m)
	}
}

func TestTextAnimatorFactor(t *testing.T) {
	// Square selector over the first half of ten characters.
	ta := textAnimator{basedOn: 1, shape: 1, units: 1, start: staticTrack(0), end: staticTrack(50)}
	for i, want := range []float64{1, 1, 1, 1, 1, 0, 0, 0, 0, 0} {
		if got := ta.factorAt(0, i, 10); got != want {
			t.Errorf("square factor[%d] = %v; want %v", i, got, want)
		}
	}
	// Index units: characters 0..2 of 10.
	ta = textAnimator{basedOn: 1, shape: 1, units: 2, start: staticTrack(0), end: staticTrack(3)}
	if got := ta.factorAt(0, 2, 10); got != 1 {
		t.Errorf("index factor[2] = %v; want 1", got)
	}
	if got := ta.factorAt(0, 3, 10); got != 0 {
		t.Errorf("index factor[3] = %v; want 0", got)
	}
	// Ramp up rises across the range; triangle peaks at the middle.
	ta = textAnimator{shape: 2, units: 1, start: staticTrack(0), end: staticTrack(100)}
	if a, b := ta.factorAt(0, 1, 10), ta.factorAt(0, 8, 10); a >= b {
		t.Errorf("ramp up not increasing: %v >= %v", a, b)
	}
	ta = textAnimator{shape: 4, units: 1, start: staticTrack(0), end: staticTrack(100)}
	mid := ta.factorAt(0, 5, 11)
	edge := ta.factorAt(0, 0, 11)
	if mid <= edge {
		t.Errorf("triangle: mid %v <= edge %v", mid, edge)
	}
	// The amount scales the factor.
	ta = textAnimator{shape: 1, units: 1, amount: staticTrack(50)}
	if got := ta.factorAt(0, 0, 4); got != 0.5 {
		t.Errorf("amount-scaled factor = %v; want 0.5", got)
	}
}

func TestTextAnimatorBuild(t *testing.T) {
	json := `{
	  "fr": 30, "ip": 0, "op": 30, "w": 200, "h": 100,
	  "layers": [
	    { "ty": 5, "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {},
	      "t": {
	        "d": { "k": [ { "t": 0, "s": { "t": "Hi there", "f": "F", "s": 24,
	          "fc": [1, 1, 1], "j": 0, "lh": 28, "tr": 50 } } ] },
	        "a": [
	          { "nm": "wave",
	            "s": { "b": 1, "sh": 4, "r": 1,
	              "s": { "a": 0, "k": 0 }, "e": { "a": 0, "k": 50 },
	              "o": { "a": 0, "k": 0 }, "a": { "a": 0, "k": 100 } },
	            "a": { "p": { "a": 0, "k": [0, -12] }, "o": { "a": 0, "k": 50 },
	              "r": { "a": 0, "k": 15 } } }
	        ]
	      } }
	  ]
	}`
	anim, err := Decode(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	// Only the font-resolver note remains; animators themselves parse.
	for _, f := range anim.UnsupportedFeatures() {
		if strings.Contains(f, "animator") {
			t.Errorf("animator reported unsupported: %v", f)
		}
	}
	tn := anim.layers[0].text
	if tn == nil || len(tn.animators) != 1 {
		t.Fatalf("animators = %+v; want 1", tn)
	}
	ta := &tn.animators[0]
	if ta.shape != 4 || ta.pos == nil || ta.rotation == nil || ta.opacity == nil {
		t.Errorf("animator not built: %+v", ta)
	}
	if tn.keys[0].doc.tracking != 50 {
		t.Errorf("doc tracking = %v; want 50", tn.keys[0].doc.tracking)
	}
}

func TestHitTestNamedLayer(t *testing.T) {
	json := `{
	  "fr": 30, "ip": 0, "op": 30, "w": 100, "h": 100,
	  "layers": [
	    { "ty": 4, "nm": "btn", "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {},
	      "shapes": [
	        { "ty": "rc", "p": { "a": 0, "k": [50, 50] }, "s": { "a": 0, "k": [40, 40] }, "r": { "a": 0, "k": 0 } },
	        { "ty": "fl", "c": { "a": 0, "k": [1, 0, 0] }, "o": { "a": 0, "k": 100 } }
	      ] }
	  ]
	}`
	anim, err := Decode(strings.NewReader(json))
	if err != nil {
		t.Fatal(err)
	}
	p := anim.NewPlayer()
	if !p.HitTest("btn", 50, 50) {
		t.Error("center of the button should hit")
	}
	if p.HitTest("btn", 5, 5) {
		t.Error("outside the button should not hit")
	}
	if p.HitTest("nope", 50, 50) {
		t.Error("an unknown layer name should not hit")
	}
	if p.HitTest("", 50, 50) {
		t.Error("an empty name never hits at the player level")
	}
}
