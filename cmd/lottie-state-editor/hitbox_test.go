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
	if s.Radius != round2(r+3) {
		t.Fatalf("resize: got %v, want %v", s.Radius, round2(r+3))
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

// The inspector edits whatever was last selected; every selection path
// must record itself so the pane follows.
func TestInspectTargetFollowsSelection(t *testing.T) {
	m := stageModel(t)

	if m.InspectTarget() != inspectState {
		t.Fatalf("default target: %v", m.InspectTarget())
	}
	m.AddHitbox(lottieresolv.KindRect)
	if m.InspectTarget() != inspectHitbox {
		t.Fatal("adding a hitbox should focus it")
	}
	m.AddCPShape(lottiecp.ShapeCircle)
	if m.InspectTarget() != inspectCPShape {
		t.Fatal("adding a body shape should focus it")
	}
	m.AddSocket(m.StageLayerNames()[0])
	if m.InspectTarget() != inspectSocket {
		t.Fatal("adding a socket should focus it")
	}
	m.SelectHitbox(0)
	if m.InspectTarget() != inspectHitbox {
		t.Fatal("selecting a hitbox should focus it")
	}
	m.NewMachine()
	if m.InspectTarget() != inspectMachine {
		t.Fatal("creating a machine should focus it")
	}
	m.AddState()
	if m.InspectTarget() != inspectState {
		t.Fatal("adding a state should focus it")
	}
	m.SelectMachine(m.MachineIDs()[0])
	if m.InspectTarget() != inspectMachine {
		t.Fatal("selecting a machine should focus it")
	}
	// Deleting the focused thing falls back to the state pane rather than
	// showing an empty editor.
	m.SelectHitbox(0)
	m.DeleteHitbox()
	if m.InspectTarget() != inspectState {
		t.Fatalf("after delete: %v", m.InspectTarget())
	}
}

// Dragging a socket on the stage writes a layer-local offset; the bound
// layer stays the position's source of truth.
func TestDragSocketWritesLocalOffset(t *testing.T) {
	m := stageModel(t)
	m.AddSocket(m.StageLayerNames()[0]) // "anchor": position (50,50), no rotation
	m.DragSocket(5, 3)
	s := m.SelectedSocket()
	if s.DX != 5 || s.DY != 3 {
		t.Fatalf("offset: %+v", s)
	}
	p, ok := m.loadSockets().At(m.PreviewAnimation(), 0, s.Name)
	if !ok || p.X != 55 || p.Y != 53 {
		t.Fatalf("placement with offset: %+v ok=%v", p, ok)
	}
}

func TestRenameSocketKeepsBinding(t *testing.T) {
	m := stageModel(t)
	layer := m.StageLayerNames()[0]
	m.AddSocket(layer)
	m.RenameSocket("weapon")
	s := m.SelectedSocket()
	if s == nil || s.Name != "weapon" || s.LayerName() != layer {
		t.Fatalf("rename broke the binding: %+v", s)
	}
}

// The physics switch travels with the bundle in the manifest's extra
// member and gates which tooling shows.
func TestPhysicsBackendConfig(t *testing.T) {
	m := stageModel(t)

	if m.PhysicsBackend() != "both" || !m.ResolvEnabled() || !m.CPEnabled() {
		t.Fatalf("default: %q", m.PhysicsBackend())
	}
	m.SetPhysicsBackend("cp")
	if m.ResolvEnabled() || !m.CPEnabled() {
		t.Fatal("cp: resolv tooling should be off")
	}
	m.SetPhysicsBackend("none")
	if m.ResolvEnabled() || m.CPEnabled() {
		t.Fatal("none: everything should be off")
	}

	var buf bytes.Buffer
	if err := m.Bundle().Encode(&buf); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	rb, err := lottie.DecodeBundle(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("DecodeBundle: %v", err)
	}
	m2 := NewModel()
	m2.bundle = rb
	if m2.PhysicsBackend() != "none" {
		t.Fatalf("config did not survive the save: %q", m2.PhysicsBackend())
	}
	// Garbage in the member reads as the default, not a crash.
	m.SetPhysicsBackend("quantum")
	if m.PhysicsBackend() != "both" {
		t.Fatalf("unknown value: %q", m.PhysicsBackend())
	}
}

func TestShowConfigPane(t *testing.T) {
	m := stageModel(t)
	m.ShowConfigPane()
	if m.InspectTarget() != inspectConfig {
		t.Fatalf("target: %v", m.InspectTarget())
	}
}

func TestTransport(t *testing.T) {
	m := stageModel(t)
	if !m.PreviewPlaying() {
		t.Fatal("a shown clip should be playing")
	}
	m.PausePreview()
	if m.PreviewPlaying() {
		t.Fatal("PausePreview did not pause")
	}
	m.PreviewSeek(10)
	m.StepPreviewFrame(1)
	if got := m.PreviewPlayer().Frame(); got != 11 {
		t.Fatalf("step: frame %v, want 11", got)
	}
	m.StepPreviewFrame(-1)
	if got := m.PreviewPlayer().Frame(); got != 10 {
		t.Fatalf("step back: frame %v, want 10", got)
	}
	m.TogglePreviewPlaying()
	if !m.PreviewPlaying() {
		t.Fatal("toggle should resume")
	}
}

// Chart drags address spans by index and defer re-sorting to release.
func TestChartSpanOps(t *testing.T) {
	m := stageModel(t)
	m.AddHitbox(lottieresolv.KindRect) // span [0, 10)
	m.PreviewSeek(20)
	m.AddHitboxSpan() // span [20, 30)
	b := m.SelectedHitbox()

	// Shift the first span past the second; order is restored on release.
	m.ShiftSpan(0, 0, 25)
	if b.Spans[0].From != 25 || b.Spans[0].To != 35 {
		t.Fatalf("shift: %+v", b.Spans[0])
	}
	m.NormalizeSpans(0)
	if b.Spans[0].From != 20 || b.Spans[1].From != 25 {
		t.Fatalf("normalize: %+v", b.Spans)
	}

	// Clamps: a span cannot slide below zero nor shrink past one frame.
	m.ShiftSpan(0, 0, -100)
	if b.Spans[0].From != 0 || b.Spans[0].To != 10 {
		t.Fatalf("clamp at zero: %+v", b.Spans[0])
	}
	m.SetSpanEdge(0, 0, true, 0)
	if b.Spans[0].To != 1 {
		t.Fatalf("right edge min width: %+v", b.Spans[0])
	}
	m.SetSpanEdge(0, 0, false, 100)
	if b.Spans[0].From != 0 {
		t.Fatalf("left edge min width: %+v", b.Spans[0])
	}

	// Nothing lands beyond the clip's last frame (60 here): shifting far
	// right parks the span against the end, and the right edge cannot
	// cross it either.
	m.ShiftSpan(0, 1, 1000)
	if b.Spans[1].From != 50 || b.Spans[1].To != 60 {
		t.Fatalf("clamp at clip end: %+v", b.Spans[1])
	}
	m.SetSpanEdge(0, 1, true, 999)
	if b.Spans[1].To != 60 {
		t.Fatalf("right edge clamp: %+v", b.Spans[1])
	}
}

// New boxes and spans clamp to the clip too: a span started near the end
// must not spill past it.
func TestAddSpanClampsToClip(t *testing.T) {
	m := stageModel(t)
	m.PreviewSeek(58)
	m.AddHitbox(lottieresolv.KindRect)
	sp := m.SelectedHitbox().Spans[0]
	if sp.From != 58 || sp.To != 60 {
		t.Fatalf("span at clip end: %+v", sp)
	}
}

// The active tab decides which overlay group shows; the Segment tab is
// the clean preview.
func TestOverlayFollowsTab(t *testing.T) {
	m := stageModel(t)
	if m.CollisionTab() != colSegment {
		t.Fatalf("default tab: %v", m.CollisionTab())
	}
	if m.OverlayVisible() || m.HitboxesVisible() || m.BodyVisible() || m.SocketsVisible() {
		t.Fatal("the segment overview must show a clean stage")
	}
	m.SetCollisionTab(colHitboxes)
	if !m.HitboxesVisible() || m.BodyVisible() || m.SocketsVisible() {
		t.Fatal("hitbox tab shows only hitboxes")
	}
	m.SetCollisionTab(colBody)
	if !m.BodyVisible() || m.HitboxesVisible() {
		t.Fatal("body tab shows only the silhouette")
	}
	m.SetCollisionTab(colSockets)
	if !m.SocketsVisible() || m.BodyVisible() || !m.OverlayVisible() {
		t.Fatal("socket tab shows only sockets")
	}
	// With the config excluding cp, a stored Body tab clamps back to the
	// overview and the silhouette stays hidden.
	m.SetCollisionTab(colBody)
	m.SetPhysicsBackend("resolv")
	if m.BodyVisible() || m.CollisionTab() != colSegment {
		t.Fatalf("clamp: tab=%v visible=%v", m.CollisionTab(), m.BodyVisible())
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
