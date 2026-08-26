package lottieresolv

import (
	"testing"

	"github.com/solarlune/resolv"

	lottie "github.com/shibukawa/lottie-go"
)

func sampleTrack() *lottie.ResolvTrack {
	return &lottie.ResolvTrack{Boxes: []lottie.ResolvBox{
		{
			Name: "punch", Kind: lottie.ResolvRect, Tags: []string{"hit"},
			Spans: []lottie.ResolvSpan{
				{From: 10, To: 14, X: 100, Y: 40, W: 60, H: 30},
				{From: 14, To: 18, X: 130, Y: 40, W: 60, H: 30},
			},
		},
		{
			Name: "body", Kind: lottie.ResolvRect, Tags: []string{"hurt", "push"},
			Spans: []lottie.ResolvSpan{{From: 0, To: 60, X: 20, Y: 10, W: 40, H: 80}},
		},
		{
			Name: "head", Kind: lottie.ResolvCircle, Tags: []string{"hurt"},
			Spans: []lottie.ResolvSpan{{From: 0, To: 60, X: 40, Y: 8, R: 12}},
		},
	}}
}

func TestTagDedup(t *testing.T) {
	if Tag("dedup-check") != Tag("dedup-check") {
		t.Fatal("the same name must map to the same bit")
	}
}

func TestSyncLifecycle(t *testing.T) {
	space := resolv.NewSpace(640, 480, 16, 16)
	tr := NewTracker(space, sampleTrack())

	tr.Sync(5)
	if got := len(space.Shapes()); got != 2 {
		t.Fatalf("frame 5: %d shapes in space, want 2", got)
	}
	tr.Sync(12)
	if got := len(space.Shapes()); got != 3 {
		t.Fatalf("frame 12: %d shapes in space, want 3", got)
	}
	punch := tr.Shapes("hit")
	if len(punch) != 1 {
		t.Fatalf("hit shapes: got %d, want 1", len(punch))
	}
	first := punch[0]
	if d, ok := first.Data().(BoxData); !ok || d.Name != "punch" || d.Index != 0 {
		t.Fatalf("shape data: got %+v", first.Data())
	}

	// Stepping to the second span rebuilds the shape at its new geometry.
	tr.Sync(15)
	moved := tr.Shapes("hit")[0]
	if x := moved.Position().X; x < 130 {
		t.Fatalf("punch did not step forward: x=%v", x)
	}

	// Past the last span the box leaves the space.
	tr.Sync(30)
	if got := len(tr.Shapes("hit")); got != 0 {
		t.Fatalf("frame 30: punch still live (%d shapes)", got)
	}
	if got := len(space.Shapes()); got != 2 {
		t.Fatalf("frame 30: %d shapes in space, want 2", got)
	}
}

func TestOffsetMovesShapes(t *testing.T) {
	space := resolv.NewSpace(640, 480, 16, 16)
	tr := NewTracker(space, sampleTrack())
	tr.Sync(0)
	head := tr.Shapes("hurt")[1]
	x0 := head.Position().X

	tr.SetOffset(200, 0)
	tr.Sync(0)
	if got := head.Position().X - x0; got != 200 {
		t.Fatalf("offset shift: got %v, want 200", got)
	}
}

func TestShapesTagFilter(t *testing.T) {
	space := resolv.NewSpace(640, 480, 16, 16)
	tr := NewTracker(space, sampleTrack())
	tr.Sync(12)
	if got := len(tr.Shapes("hurt")); got != 2 {
		t.Fatalf("hurt: got %d, want 2", got)
	}
	// Multiple names act as any-of.
	if got := len(tr.Shapes("hit", "push")); got != 2 {
		t.Fatalf("hit|push: got %d, want 2", got)
	}
	if got := len(tr.Shapes()); got != 3 {
		t.Fatalf("all: got %d, want 3", got)
	}
}

func TestRemove(t *testing.T) {
	space := resolv.NewSpace(640, 480, 16, 16)
	tr := NewTracker(space, sampleTrack())
	tr.Sync(12)
	tr.Remove()
	if got := len(space.Shapes()); got != 0 {
		t.Fatalf("after Remove: %d shapes in space", got)
	}
	tr.Sync(12)
	if got := len(space.Shapes()); got != 3 {
		t.Fatalf("re-Sync: %d shapes in space, want 3", got)
	}
}
