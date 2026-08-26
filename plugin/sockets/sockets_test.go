package lottiesockets

import (
	"math"
	"strings"
	"testing"

	lottie "github.com/shibukawa/lottie-go"
)

const fixture = `{
  "v": "5.9.0", "fr": 60, "ip": 0, "op": 60, "w": 200, "h": 200,
  "layers": [
    {"ty": 3, "nm": "hand_r", "ind": 1, "ip": 0, "op": 60, "st": 0, "ks": {
      "p": {"a": 1, "k": [
        {"t": 0, "s": [10, 50], "o": {"x": 0.5, "y": 0}, "i": {"x": 0.5, "y": 1}},
        {"t": 60, "s": [70, 50]}
      ]},
      "r": {"a": 0, "k": 90}
    }},
    {"ty": 3, "nm": "root", "ind": 2, "ip": 0, "op": 60, "st": 0, "ks": {
      "p": {"a": 1, "k": [
        {"t": 0, "s": [0, 100], "o": {"x": 0.5, "y": 0}, "i": {"x": 0.5, "y": 1}},
        {"t": 60, "s": [120, 100]}
      ]}
    }}
  ]
}`

func anim(t *testing.T) *lottie.Animation {
	t.Helper()
	a, err := lottie.Decode(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return a
}

func set() *Set {
	return &Set{Sockets: []Socket{
		{Name: "weapon", Layer: "hand_r", Z: ZBehind},
		{Name: "root"}, // layer defaults to the socket name
		{Name: "ghost", Layer: "no-such-layer"},
	}}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestAtResolvesBinding(t *testing.T) {
	a := anim(t)
	s := set()

	p, ok := s.At(a, 30, "weapon")
	if !ok {
		t.Fatal("weapon socket not resolved")
	}
	if !near(p.X, 40) || !near(p.Y, 50) {
		t.Fatalf("position: got (%v, %v), want (40, 50)", p.X, p.Y)
	}
	if !near(p.Angle, math.Pi/2) {
		t.Fatalf("angle: got %v", p.Angle)
	}
	if p.Z != ZBehind {
		t.Fatalf("z: got %q", p.Z)
	}

	if _, ok := s.At(a, 0, "no-such-socket"); ok {
		t.Fatal("unknown socket must report false")
	}
	if _, ok := s.At(a, 0, "ghost"); ok {
		t.Fatal("socket bound to a missing layer must report false")
	}
}

func TestAllSkipsMissingLayers(t *testing.T) {
	a := anim(t)
	got := set().All(a, 0)
	if len(got) != 2 || got[0].Name != "weapon" || got[1].Name != "root" {
		t.Fatalf("All: %+v", got)
	}
}

func TestMirrored(t *testing.T) {
	a := anim(t)
	p, _ := set().At(a, 30, "weapon")
	m := p.Mirrored(100)
	if !near(m.X, 160) || !near(m.Angle, -math.Pi/2) || m.Z != ZBehind {
		t.Fatalf("mirrored: %+v", m)
	}
}

// The local offset rides the layer's rotation and scale.
func TestLocalOffset(t *testing.T) {
	a := anim(t)
	s := &Set{Sockets: []Socket{
		// hand_r sits at 90 degrees, unit scale: local +x points down
		// the screen (y-down coordinates), so dx=5 lands at (x, y+5).
		{Name: "grip", Layer: "hand_r", DX: 5},
	}}
	p, ok := s.At(a, 30, "grip")
	if !ok {
		t.Fatal("grip not resolved")
	}
	if !near(p.X, 40) || !near(p.Y, 55) {
		t.Fatalf("offset placement: got (%v, %v), want (40, 55)", p.X, p.Y)
	}
}

// The angle trims: DR adds onto the layer's rotation, and RotateNone
// pins the angle to DR alone.
func TestAngleTrims(t *testing.T) {
	a := anim(t)
	s := &Set{Sockets: []Socket{
		{Name: "tilted", Layer: "hand_r", DR: 45}, // hand_r sits at 90°
		{Name: "level", Layer: "hand_r", Rotate: RotateNone},
		{Name: "pinned", Layer: "hand_r", Rotate: RotateNone, DR: 10},
	}}
	p, _ := s.At(a, 30, "tilted")
	if !near(p.Angle, 135*math.Pi/180) {
		t.Fatalf("follow+dr: got %v", p.Angle)
	}
	p, _ = s.At(a, 30, "level")
	if !near(p.Angle, 0) {
		t.Fatalf("none: got %v", p.Angle)
	}
	p, _ = s.At(a, 30, "pinned")
	if !near(p.Angle, 10*math.Pi/180) {
		t.Fatalf("none+dr: got %v", p.Angle)
	}
}

func TestDisplacement(t *testing.T) {
	a := anim(t)
	dx, dy, ok := Displacement(a, "root", 0, 30)
	if !ok || !near(dx, 60) || !near(dy, 0) {
		t.Fatalf("displacement: (%v, %v) ok=%v", dx, dy, ok)
	}
	if _, _, ok := Displacement(a, "nope", 0, 30); ok {
		t.Fatal("unknown layer must report false")
	}
}

func TestBundleRoundTrip(t *testing.T) {
	b := lottie.NewBundle()
	if err := Store(b, set()); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := Load(b)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Sockets) != 3 || got.Sockets[0].Layer != "hand_r" {
		t.Fatalf("round-trip: %+v", got.Sockets)
	}
	Remove(b)
	if _, err := Load(b); err == nil {
		t.Fatal("Remove left the table readable")
	}
}
