package lottie

import (
	"math"
	"strings"
	"testing"
)

// placementFixture builds an animation exercising every path a placement
// query takes: a transformed null, a parented child, and a null inside a
// precomp with a start-time offset.
const placementFixture = `{
  "v": "5.9.0", "fr": 60, "ip": 0, "op": 60, "w": 200, "h": 200,
  "assets": [
    {"id": "pc", "layers": [
      {"ty": 3, "nm": "muzzle", "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {
        "p": {"a": 1, "k": [
          {"t": 0, "s": [5, 5], "o": {"x": 0.5, "y": 0}, "i": {"x": 0.5, "y": 1}},
          {"t": 30, "s": [35, 5]}
        ]}
      }}
    ]}
  ],
  "layers": [
    {"ty": 3, "nm": "hand", "ind": 1, "ip": 0, "op": 40, "st": 0, "ks": {
      "p": {"a": 0, "k": [50, 60]},
      "s": {"a": 0, "k": [200, 100]},
      "r": {"a": 0, "k": 30}
    }},
    {"ty": 3, "nm": "tip", "ind": 2, "parent": 1, "ip": 0, "op": 60, "st": 0, "ks": {
      "p": {"a": 0, "k": [10, 0]}
    }},
    {"ty": 0, "nm": "gun", "ind": 3, "refId": "pc", "ip": 0, "op": 60, "st": 10,
     "w": 100, "h": 100, "ks": {
      "p": {"a": 0, "k": [100, 100]}
    }}
  ]
}`

func placementAnim(t *testing.T) *Animation {
	t.Helper()
	a, err := Decode(strings.NewReader(placementFixture))
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return a
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestLayerPlacementTransform(t *testing.T) {
	a := placementAnim(t)

	p, ok := a.LayerPlacement("hand", 0)
	if !ok {
		t.Fatal("hand not found")
	}
	if !near(p.X, 50) || !near(p.Y, 60) {
		t.Fatalf("position: got (%v, %v)", p.X, p.Y)
	}
	if !near(p.Angle, math.Pi/6) {
		t.Fatalf("angle: got %v, want 30 deg", p.Angle)
	}
	if !near(p.ScaleX, 2) || !near(p.ScaleY, 1) {
		t.Fatalf("scale: got (%v, %v)", p.ScaleX, p.ScaleY)
	}
	if !p.Visible {
		t.Fatal("hand should be visible at frame 0")
	}

	if _, ok := a.LayerPlacement("no-such-layer", 0); ok {
		t.Fatal("unknown layer must report false")
	}
}

func TestLayerPlacementParentChain(t *testing.T) {
	a := placementAnim(t)
	p, ok := a.LayerPlacement("tip", 0)
	if !ok {
		t.Fatal("tip not found")
	}
	// The child's local origin (10, 0) through the parent's scale(2,1) then
	// rotate(30) then translate(50, 60).
	wantX := 50 + 20*math.Cos(math.Pi/6)
	wantY := 60 + 20*math.Sin(math.Pi/6)
	if !near(p.X, wantX) || !near(p.Y, wantY) {
		t.Fatalf("position: got (%v, %v), want (%v, %v)", p.X, p.Y, wantX, wantY)
	}
}

func TestLayerPlacementPrecompTime(t *testing.T) {
	a := placementAnim(t)
	// The precomp layer starts at frame 10, so composition frame 25 is
	// local frame 15: the muzzle keyframe [5..35] is halfway, x = 20, plus
	// the precomp layer's own (100, 100).
	p, ok := a.LayerPlacement("muzzle", 25)
	if !ok {
		t.Fatal("muzzle not found")
	}
	if !near(p.X, 120) || !near(p.Y, 105) {
		t.Fatalf("position: got (%v, %v), want (120, 105)", p.X, p.Y)
	}
	if !p.Visible {
		t.Fatal("muzzle should be visible at frame 25")
	}
	// Local frame 45 is past the muzzle's op (30): placement still
	// answers, visibility reports the truth.
	p, ok = a.LayerPlacement("muzzle", 55)
	if !ok || p.Visible {
		t.Fatalf("muzzle at 55: ok=%v visible=%v, want found but not visible", ok, p.Visible)
	}
}

func TestLayerPlacementVisibilityWindow(t *testing.T) {
	a := placementAnim(t)
	if p, _ := a.LayerPlacement("hand", 45); p.Visible {
		t.Fatal("hand is past its op at frame 45")
	}
}

func TestLayerPlacementMirrored(t *testing.T) {
	p := LayerPlacement{X: 30, Y: 40, Angle: math.Pi / 4, ScaleX: 1, ScaleY: 1}
	m := p.Mirrored(100)
	if !near(m.X, 170) || !near(m.Y, 40) || !near(m.Angle, -math.Pi/4) {
		t.Fatalf("mirrored: %+v", m)
	}
}

func TestLayerNames(t *testing.T) {
	a := placementAnim(t)
	got := a.LayerNames()
	want := []string{"hand", "tip", "gun", "muzzle"}
	if len(got) != len(want) {
		t.Fatalf("LayerNames: got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("LayerNames[%d]: got %v", i, got)
		}
	}
}

func TestPlayerLayerPlacement(t *testing.T) {
	a := placementAnim(t)
	p := a.NewPlayer()
	p.SetFrame(25)
	pl, ok := p.LayerPlacement("muzzle")
	if !ok || !near(pl.X, 120) {
		t.Fatalf("player query: ok=%v x=%v", ok, pl.X)
	}
}

func TestOnFrameSpan(t *testing.T) {
	a := placementAnim(t)
	p := a.NewPlayer()
	p.SetLoop(true)
	p.Play()

	var spans [][2]float64
	p.OnFrameSpan(func(from, to float64) { spans = append(spans, [2]float64{from, to}) })

	// Drive advance directly: +40 then +40 wraps the 60-frame range once.
	p.advance(40)
	p.advance(40)
	if len(spans) != 3 {
		t.Fatalf("spans: %v", spans)
	}
	if spans[0] != [2]float64{0, 40} || spans[1] != [2]float64{40, 60} || spans[2] != [2]float64{0, 20} {
		t.Fatalf("spans: %v", spans)
	}
	// The wrap swept every frame exactly once.
	total := 0.0
	for _, s := range spans {
		total += s[1] - s[0]
	}
	if !near(total, 80) {
		t.Fatalf("swept %v frames, want 80", total)
	}
}

// mirrorFixture holds a layer scaled negatively on x, the way a rigged
// character flips a part to face the other way.
const mirrorFixture = `{
  "v": "5.9.0", "fr": 60, "ip": 0, "op": 60, "w": 200, "h": 200,
  "layers": [
    {"ty": 3, "nm": "flipped", "ind": 1, "ip": 0, "op": 60, "st": 0, "ks": {
      "p": {"a": 0, "k": [100, 100]},
      "s": {"a": 0, "k": [-100, 100]}
    }}
  ]
}`

// LayerTransform exists because the decomposed placement cannot express a
// mirror: it reports positive scale and folds the flip into a half turn.
// Anything reproducing the layer's own frame needs the composed matrix.
func TestLayerTransformKeepsMirror(t *testing.T) {
	a, err := Decode(strings.NewReader(mirrorFixture))
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	g, ok := a.LayerTransform("flipped", 0)
	if !ok {
		t.Fatal("flipped not found")
	}
	// A mirror flips x and leaves y alone. The probe point is off the
	// x-axis on purpose: on it, a mirror and a half turn agree.
	x, y := g.Apply(10, 20)
	if !near(x, 90) || !near(y, 120) {
		t.Errorf("mirrored point: got (%v, %v); want (90, 120)", x, y)
	}
	if d := g.Element(0, 0)*g.Element(1, 1) - g.Element(0, 1)*g.Element(1, 0); d >= 0 {
		t.Errorf("determinant = %v; want negative for a mirrored layer", d)
	}
	// The decomposition, by contrast, reports a right-handed frame and
	// turns the flip into a half turn, which also flips y. That is exactly
	// the gap the matrix accessor closes.
	p, _ := a.LayerPlacement("flipped", 0)
	if p.ScaleX < 0 {
		t.Errorf("LayerPlacement.ScaleX = %v; expected the decomposition to lose the sign", p.ScaleX)
	}
	pg := p.GeoM()
	if px, py := pg.Apply(10, 20); !near(px, 90) || !near(py, 80) {
		t.Errorf("LayerPlacement.GeoM point = (%v, %v); want the half-turn (90, 80) "+
			"that makes the matrix accessor necessary", px, py)
	}
}

// A transform must resolve through a parent chain and a precomp the same way
// a placement does, since it shares the resolver.
func TestLayerTransformMatchesPlacement(t *testing.T) {
	a := placementAnim(t)
	for _, name := range []string{"hand", "tip", "muzzle"} {
		g, ok := a.LayerTransform(name, 20)
		if !ok {
			t.Fatalf("%s not found", name)
		}
		p, _ := a.LayerPlacement(name, 20)
		x, y := g.Apply(0, 0)
		if !near(x, p.X) || !near(y, p.Y) {
			t.Errorf("%s origin: matrix (%v, %v), placement (%v, %v)", name, x, y, p.X, p.Y)
		}
	}
}
