package lottiecp

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	cp "github.com/jakecoffman/cp/v2"
	lottie "github.com/shibukawa/lottie-go"
)

func sampleBody() *Body {
	return &Body{
		Type: BodyDynamic, Mass: 2,
		Shapes: []Shape{
			{Type: ShapeCircle, Center: Point{X: 50, Y: 30}, Radius: 20, Friction: 0.5},
			{Type: ShapeBox, Center: Point{X: 50, Y: 80}, Width: 40, Height: 60, Sensor: true},
			{Type: ShapePolygon, Vertices: []Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 5, Y: 8}}},
		},
	}
}

func TestBuild(t *testing.T) {
	body, shapes := Build(sampleBody())
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
	def := &Body{
		Type: BodyStatic,
		Shapes: []Shape{
			{Type: "future-shape"},
			{Type: ShapeCircle, Radius: 5},
		},
	}
	body, shapes := Build(def)
	if body.GetType() != cp.BODY_STATIC {
		t.Fatalf("body type: got %v", body.GetType())
	}
	// The unknown shape is skipped, not fatal.
	if len(shapes) != 1 {
		t.Fatalf("got %d shapes, want 1", len(shapes))
	}
}

func TestBundleRoundTrip(t *testing.T) {
	b := lottie.NewBundle()
	if err := Store(b, "player", sampleBody()); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if got := IDs(b); !reflect.DeepEqual(got, []string{"player"}) {
		t.Fatalf("IDs: got %v", got)
	}
	got, err := Load(b, "player")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, sampleBody()) {
		t.Fatalf("did not round-trip:\n got %+v", got)
	}
	Remove(b, "player")
	if got := IDs(b); len(got) != 0 {
		t.Fatalf("Remove left %v", got)
	}
}

func TestExtraFieldsSurvive(t *testing.T) {
	raw := []byte(`{"type":"dynamic","hp":40,` +
		`"shapes":[{"type":"circle","radius":5,"material":"steel"}]}`)
	body, err := ParseBody(raw)
	if err != nil {
		t.Fatalf("ParseBody: %v", err)
	}
	out, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, want := range []string{`"hp":40`, `"material":"steel"`} {
		if !bytes.Contains(out, []byte(want)) {
			t.Fatalf("extra member %s lost: %s", want, out)
		}
	}
}
