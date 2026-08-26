package lottiecp

import (
	"testing"

	cp "github.com/jakecoffman/cp/v2"
	lottie "github.com/shibukawa/lottie-go"
)

func sampleBody() *lottie.CPBody {
	return &lottie.CPBody{
		Type: lottie.CPBodyDynamic, Mass: 2,
		Shapes: []lottie.CPShape{
			{Type: lottie.CPShapeCircle, Center: lottie.PhysPoint{X: 50, Y: 30}, Radius: 20, Friction: 0.5},
			{Type: lottie.CPShapeBox, Center: lottie.PhysPoint{X: 50, Y: 80}, Width: 40, Height: 60, Sensor: true},
			{Type: lottie.CPShapePolygon, Vertices: []lottie.PhysPoint{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 5, Y: 8}}},
		},
	}
}

func TestNewBody(t *testing.T) {
	body, shapes := NewBody(sampleBody())
	if len(shapes) != 3 {
		t.Fatalf("got %d shapes, want 3", len(shapes))
	}
	if got := body.Mass(); got != 2 {
		t.Fatalf("mass: got %v, want 2", got)
	}
	if m := body.Moment(); m <= 0 || m == cp.INFINITY {
		t.Fatalf("derived moment: got %v", m)
	}
	if got := shapes[0].Friction(); got != 0.5 {
		t.Fatalf("friction: got %v", got)
	}
	if !shapes[1].Sensor() {
		t.Fatal("box should be a sensor")
	}
}

func TestAddToSpace(t *testing.T) {
	space := cp.NewSpace()
	body, shapes := AddToSpace(space, sampleBody())
	if !space.ContainsBody(body) {
		t.Fatal("body not registered with the space")
	}
	for i, s := range shapes {
		if s.Space() != space {
			t.Fatalf("shape %d not registered with the space", i)
		}
	}
	// The wired-up body must actually simulate.
	body.SetPosition(cp.Vector{X: 100, Y: 100})
	space.SetGravity(cp.Vector{X: 0, Y: 100})
	for range 10 {
		space.Step(1.0 / 60)
	}
	if body.Position().Y <= 100 {
		t.Fatalf("body did not fall: %v", body.Position())
	}
}

func TestStaticAndUnknownShapes(t *testing.T) {
	def := &lottie.CPBody{
		Type: lottie.CPBodyStatic,
		Shapes: []lottie.CPShape{
			{Type: "future-shape"},
			{Type: lottie.CPShapeCircle, Radius: 5},
		},
	}
	body, shapes := NewBody(def)
	if body.GetType() != cp.BODY_STATIC {
		t.Fatalf("body type: got %v", body.GetType())
	}
	// The unknown shape is skipped, not fatal.
	if len(shapes) != 1 {
		t.Fatalf("got %d shapes, want 1", len(shapes))
	}
}
