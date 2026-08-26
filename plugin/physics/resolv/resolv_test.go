package lottieresolv

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/solarlune/resolv"

	lottie "github.com/shibukawa/lottie-go"
)

func sampleTrack() *Track {
	return &Track{Boxes: []Box{
		{
			Name: "punch", Kind: KindRect, Tags: []string{"hit"},
			Spans: []Span{
				{From: 10, To: 14, X: 100, Y: 40, W: 60, H: 30},
				{From: 14, To: 18, X: 130, Y: 40, W: 60, H: 30},
			},
		},
		{
			Name: "body", Kind: KindRect, Tags: []string{"hurt", "push"},
			Spans: []Span{{From: 0, To: 60, X: 20, Y: 10, W: 40, H: 80}},
		},
		{
			Name: "head", Kind: KindCircle, Tags: []string{"hurt"},
			Spans: []Span{{From: 0, To: 60, X: 40, Y: 8, R: 12}},
		},
	}}
}

func TestTrackAt(t *testing.T) {
	tr := sampleTrack()

	if got := tr.At(5); len(got) != 2 {
		t.Fatalf("frame 5: got %d boxes, want 2", len(got))
	}
	got := tr.At(12)
	if len(got) != 3 {
		t.Fatalf("frame 12: got %d boxes, want 3", len(got))
	}
	if got[0].Name != "punch" || got[0].X != 100 {
		t.Fatalf("frame 12 punch: got %+v", got[0])
	}
	// The second span takes over at its From; the first ends exclusively.
	if g := tr.At(14); g[0].X != 130 {
		t.Fatalf("frame 14 punch span: got x=%v, want 130", g[0].X)
	}
	// To is exclusive: the box vanishes exactly at 18.
	if g := tr.At(18); len(g) != 2 {
		t.Fatalf("frame 18: got %d boxes, want 2", len(g))
	}

	hurt := tr.At(12, "hurt")
	if len(hurt) != 2 || hurt[0].Name != "body" || hurt[1].Name != "head" {
		t.Fatalf("hurt filter: got %+v", hurt)
	}
	// Multiple tags act as any-of.
	if g := tr.At(12, "hit", "push"); len(g) != 2 {
		t.Fatalf("hit|push filter: got %d boxes, want 2", len(g))
	}
	if g := tr.At(12, "nosuch"); len(g) != 0 {
		t.Fatalf("unknown tag: got %d boxes, want 0", len(g))
	}
	if idx := tr.At(12)[2].Index; idx != 2 {
		t.Fatalf("Index: got %d, want 2", idx)
	}
}

func TestBundleRoundTrip(t *testing.T) {
	b := lottie.NewBundle()
	if err := Store(b, "attack", sampleTrack()); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if got := IDs(b); !reflect.DeepEqual(got, []string{"attack"}) {
		t.Fatalf("IDs: got %v", got)
	}
	got, err := Load(b, "attack")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, sampleTrack()) {
		t.Fatalf("did not round-trip:\n got %+v", got)
	}
	Remove(b, "attack")
	if got := IDs(b); len(got) != 0 {
		t.Fatalf("Remove left %v", got)
	}
}

func TestExtraFieldsSurvive(t *testing.T) {
	raw := []byte(`{"boxes":[{"name":"b","kind":"rect","damage":12,` +
		`"spans":[{"from":0,"to":5,"x":1,"y":2,"w":3,"h":4,"knockback":7}]}],"custom":true}`)
	tr, err := ParseTrack(raw)
	if err != nil {
		t.Fatalf("ParseTrack: %v", err)
	}
	out, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, want := range []string{`"damage":12`, `"knockback":7`, `"custom":true`} {
		if !bytes.Contains(out, []byte(want)) {
			t.Fatalf("extra member %s lost: %s", want, out)
		}
	}
}

func windowTrack() *Track {
	tr := sampleTrack()
	tr.Boxes = append(tr.Boxes, Box{
		Name: "cancel", Kind: KindWindow, Tags: []string{"cancelable"},
		Spans: []Span{{From: 20, To: 28}},
	})
	return tr
}

func TestWindows(t *testing.T) {
	tr := windowTrack()

	// Windows never leak into the geometric query.
	for _, ab := range tr.At(25) {
		if ab.Kind == KindWindow {
			t.Fatalf("window in At: %+v", ab)
		}
	}
	ws := tr.WindowsAt(25)
	if len(ws) != 1 || ws[0].Name != "cancel" {
		t.Fatalf("WindowsAt: %+v", ws)
	}
	if !tr.Open(25, "cancelable") {
		t.Fatal("Open should report the live window")
	}
	if tr.Open(30, "cancelable") {
		t.Fatal("window is closed at 30 (to is exclusive at 28)")
	}
	if tr.Open(25, "hit") {
		t.Fatal("tag filter must apply to windows")
	}

	// The tracker leaves windows out of the space: at frame 25 only body
	// and head are live geometry.
	space := resolv.NewSpace(640, 480, 16, 16)
	NewTracker(space, tr).Sync(25)
	if got := len(space.Shapes()); got != 2 {
		t.Fatalf("space shapes: got %d, want 2 (window excluded)", got)
	}
}

func TestMirrored(t *testing.T) {
	r := ActiveBox{Kind: KindRect, X: 10, Y: 5, W: 30, H: 20}.Mirrored(100)
	if r.X != 160 || r.Y != 5 {
		t.Fatalf("rect mirror: %+v", r)
	}
	c := ActiveBox{Kind: KindCircle, X: 40, Y: 8, R: 12}.Mirrored(100)
	if c.X != 160 || c.R != 12 {
		t.Fatalf("circle mirror: %+v", c)
	}
	w := ActiveBox{Kind: KindWindow, X: 0}.Mirrored(100)
	if w.X != 0 {
		t.Fatalf("window mirror must be a no-op: %+v", w)
	}
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

func TestRemoveFromSpace(t *testing.T) {
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
