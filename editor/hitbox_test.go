package main

import (
	"bytes"
	"testing"

	lottie "github.com/shibukawa/lottie-go"
	lottiecp "github.com/shibukawa/lottie-go/plugin/physics/cp"
	lottieresolv "github.com/shibukawa/lottie-go/plugin/physics/resolv"
	lottiesockets "github.com/shibukawa/lottie-go/plugin/sockets"
)

// stageModel is a model with one clip on stage, which hitbox edits key on.
func stageModel(t *testing.T) *Model {
	t.Helper()
	dir := t.TempDir()
	p := writeClip(t, dir, "attack", 60, "")
	m := NewModel()
	m.ImportClip(p)
	m.ShowClip(clipRef{Anim: "attack"})
	if m.StageAnimID() != "attack" {
		t.Fatalf("StageAnimID() = %q; want attack (%s)", m.StageAnimID(), m.Status())
	}
	return m
}

func TestHitboxAuthoring(t *testing.T) {
	m := stageModel(t)

	if m.StageTrack() != nil {
		t.Fatal("fresh clip should have no track")
	}
	m.AddHitbox(lottieresolv.KindRect)
	m.AddHitbox(lottieresolv.KindCircle)
	track := m.StageTrack()
	if track == nil || len(track.Boxes) != 2 {
		t.Fatalf("track after adds: %+v", track)
	}
	if m.SelectedHitboxIndex() != 1 {
		t.Fatalf("selection should follow the add: %d", m.SelectedHitboxIndex())
	}

	m.RenameHitbox("head")
	m.SetHitboxTagsCSV(" hurt , push ")
	b := m.SelectedHitbox()
	if b.Name != "head" || len(b.Tags) != 2 || b.Tags[0] != "hurt" || b.Tags[1] != "push" {
		t.Fatalf("rename/tags: %+v", b)
	}

	// Geometry drags land on the span under the playhead (frame 0 here,
	// where AddHitbox started the span).
	x0 := b.Spans[0].X
	m.DragHitbox(5, -3)
	if b.Spans[0].X != x0+5 {
		t.Fatalf("drag: got x=%v, want %v", b.Spans[0].X, x0+5)
	}
	r0 := b.Spans[0].R
	m.DragHitboxHandle(4, 0)
	if b.Spans[0].R != r0+4 {
		t.Fatalf("resize: got r=%v, want %v", b.Spans[0].R, r0+4)
	}

	m.DeleteHitbox()
	if len(m.StageTrack().Boxes) != 1 || m.SelectedHitboxIndex() != -1 {
		t.Fatalf("delete: %+v sel=%d", m.StageTrack().Boxes, m.SelectedHitboxIndex())
	}
}

func TestHitboxSpans(t *testing.T) {
	m := stageModel(t)
	m.AddHitbox(lottieresolv.KindRect)
	b := m.SelectedHitbox()

	// A second span cannot start under the first one.
	m.AddHitboxSpan()
	if len(b.Spans) != 1 {
		t.Fatalf("overlapping span was added: %+v", b.Spans)
	}

	m.PreviewSeek(20)
	m.AddHitboxSpan()
	if len(b.Spans) != 2 || b.Spans[1].From != 20 {
		t.Fatalf("span at playhead: %+v", b.Spans)
	}
	// Geometry steps from the previous span, not from zero.
	if b.Spans[1].W != b.Spans[0].W || b.Spans[1].X != b.Spans[0].X {
		t.Fatalf("span should copy the earlier pose: %+v", b.Spans)
	}

	m.SetSpanRange(22, 30)
	if b.Spans[1].From != 22 || b.Spans[1].To != 30 {
		t.Fatalf("SetSpanRange: %+v", b.Spans[1])
	}
	// The playhead at 20 is now outside every span, so span edits pick
	// nothing.
	if m.SelectedSpan() != nil {
		t.Fatal("no span should cover frame 20 now")
	}

	m.PreviewSeek(25)
	m.DeleteHitboxSpan()
	if len(b.Spans) != 1 {
		t.Fatalf("delete span: %+v", b.Spans)
	}
}

func TestCPBodyAuthoring(t *testing.T) {
	m := stageModel(t)

	m.AddCPShape(lottiecp.ShapeCircle)
	m.AddCPShape(lottiecp.ShapeBox)
	if got := len(m.CPBodyShapes()); got != 2 {
		t.Fatalf("shapes: got %d, want 2", got)
	}
	// Selecting a body shape and a hitbox are mutually exclusive: one
	// thing is dragged at a time.
	m.AddHitbox(lottieresolv.KindRect)
	if m.SelectedCPShapeIndex() != -1 {
		t.Fatal("adding a hitbox should drop the body selection")
	}
	m.SelectCPShape(0)
	if m.SelectedHitboxIndex() != -1 {
		t.Fatal("selecting a body shape should drop the hitbox selection")
	}

	s := m.SelectedCPShape()
	cx := s.Center.X
	m.DragCPShape(7, 0)
	if s.Center.X != cx+7 {
		t.Fatalf("drag: got %v, want %v", s.Center.X, cx+7)
	}
	r := s.Radius
	m.DragCPShapeHandle(3, 0)
	if s.Radius != r+3 {
		t.Fatalf("resize: got %v, want %v", s.Radius, r+3)
	}

	m.DeleteCPShape()
	if got := len(m.CPBodyShapes()); got != 1 {
		t.Fatalf("delete: got %d shapes, want 1", got)
	}
}

// The full authoring loop: hitboxes and body survive a save and reopen.
func TestCollisionRoundTripThroughSave(t *testing.T) {
	m := stageModel(t)
	m.AddHitbox(lottieresolv.KindRect)
	m.RenameHitbox("punch")
	m.SetHitboxTagsCSV("hit")
	m.AddCPShape(lottiecp.ShapeCircle)

	var buf bytes.Buffer
	if err := m.Bundle().Encode(&buf); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	rb, err := lottie.DecodeBundle(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("DecodeBundle: %v", err)
	}
	track, err := lottieresolv.Load(rb, "attack")
	if err != nil {
		t.Fatalf("Load track: %v", err)
	}
	if len(track.Boxes) != 1 || track.Boxes[0].Name != "punch" || !track.Boxes[0].HasTag("hit") {
		t.Fatalf("track did not survive: %+v", track.Boxes)
	}
	body, err := lottiecp.Load(rb, editorCPBodyID)
	if err != nil {
		t.Fatalf("Load body: %v", err)
	}
	if len(body.Shapes) != 1 || body.Shapes[0].Type != lottiecp.ShapeCircle {
		t.Fatalf("body did not survive: %+v", body.Shapes)
	}
}

func TestWindowAuthoring(t *testing.T) {
	m := stageModel(t)
	m.AddHitbox(lottieresolv.KindWindow)
	m.RenameHitbox("cancel")
	m.SetHitboxTagsCSV("cancelable")

	track := m.StageTrack()
	// A window is a pure flag: no geometry, absent from the stage query,
	// present in the window query.
	if got := track.At(0); len(got) != 0 {
		t.Fatalf("window leaked into At: %+v", got)
	}
	if !track.Open(0, "cancelable") {
		t.Fatal("window not open at its span")
	}
	// Span editing works exactly like a box.
	m.SetSpanRange(5, 9)
	if track.Open(4, "cancelable") || !track.Open(8, "cancelable") {
		t.Fatal("span edit did not move the window")
	}
	if got := HitboxLabel(0, track.Boxes[0]); got != "1: cancel (win)" {
		t.Fatalf("label: %q", got)
	}
}

func TestSocketAuthoring(t *testing.T) {
	m := stageModel(t)

	layers := m.StageLayerNames()
	if len(layers) == 0 {
		t.Fatal("stage clip should expose layer names")
	}
	m.AddSocket(layers[0])
	if got := m.Sockets(); len(got) != 1 || got[0].Name != layers[0] {
		t.Fatalf("Sockets: %+v", got)
	}
	// Duplicate names are refused.
	m.AddSocket(layers[0])
	if got := m.Sockets(); len(got) != 1 {
		t.Fatalf("duplicate slipped in: %+v", got)
	}

	m.ToggleSocketZ()
	if m.Sockets()[0].Z != lottiesockets.ZBehind {
		t.Fatalf("z toggle: %+v", m.Sockets()[0])
	}

	// The table rides the bundle through a save.
	var buf bytes.Buffer
	if err := m.Bundle().Encode(&buf); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	rb, err := lottie.DecodeBundle(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("DecodeBundle: %v", err)
	}
	set, err := lottiesockets.Load(rb)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(set.Sockets) != 1 || set.Sockets[0].Z != lottiesockets.ZBehind {
		t.Fatalf("sockets did not survive: %+v", set.Sockets)
	}

	m.DeleteSocket()
	if got := m.Sockets(); len(got) != 0 {
		t.Fatalf("delete: %+v", got)
	}
}

// Dropping a clip drops its hitbox track too: that cleanup moved from the
// core to this editor when the data became a plugin payload.
func TestRemoveClipDropsTrack(t *testing.T) {
	m := stageModel(t)
	m.AddHitbox(lottieresolv.KindRect)
	if got := lottieresolv.IDs(m.Bundle()); len(got) != 1 {
		t.Fatalf("track ids before removal: %v", got)
	}
	m.RemoveClip("attack")
	if got := lottieresolv.IDs(m.Bundle()); len(got) != 0 {
		t.Fatalf("track survived its clip: %v", got)
	}
	if m.StageTrack() != nil {
		t.Fatal("cache still serves the removed clip's track")
	}
}
