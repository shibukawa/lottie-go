package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	lottie "github.com/shibukawa/lottie-go"
)

// clipJSON is a minimal clip of the given length in frames at 60fps.
func clipJSON(frames int) []byte {
	return fmt.Appendf(nil, `{"v":"5.9.0","nm":"clip","fr":60,"ip":0,"op":%d,"w":100,"h":100,
		"layers":[{"ty":3,"nm":"null","ind":1,"ip":0,"op":%d,"st":0,
		"ks":{"a":{"a":0,"k":[0,0]},"p":{"a":0,"k":[50,50]},
		"s":{"a":0,"k":[100,100]},"r":{"a":0,"k":0},"o":{"a":0,"k":100}}}]}`,
		frames, frames)
}

// writeTestBundle writes a bundle holding one clip and one machine to dir.
func writeTestBundle(t *testing.T, dir, name string) string {
	t.Helper()
	b := lottie.NewBundle()
	if err := b.SetAnimation("logo", clipJSON(30)); err != nil {
		t.Fatal(err)
	}
	sm, err := lottie.ParseStateMachine([]byte(`{
		"initial": "idle",
		"inputs": [{"type": "Event", "name": "activate"}],
		"states": [{"name": "idle", "type": "PlaybackState", "animation": "logo",
			"loop": true, "autoplay": true}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := b.SetStateMachine("btn", sm); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := b.Encode(f); err != nil {
		t.Fatal(err)
	}
	return path
}

// testModel is a model with one bundle referenced and loaded.
func testModel(t *testing.T) (*Model, string) {
	t.Helper()
	dir := t.TempDir()
	path := writeTestBundle(t, dir, "ui.lottie")
	m := NewModel()
	m.path = filepath.Join(dir, "menu.scene.json") // bundle paths resolve against this
	m.AddBundle(path)
	if len(m.scene.Bundles) != 1 {
		t.Fatalf("bundle not referenced: %v", m.scene.Bundles)
	}
	if _, ok := m.Bundle("ui"); !ok {
		t.Fatalf("bundle not loaded; errs=%v", m.bundleErrs)
	}
	return m, dir
}

// placeClip places the bundle and switches the node's content to the
// plain "logo" clip — the pre-bundle-node shape most tests want.
func placeClip(t *testing.T, m *Model) {
	t.Helper()
	m.PlaceSource(m.Sources()[0])
	m.SetNodeContent(m.SelectedNodeIndex(), lottie.SceneNodeAnimation, "logo")
}

func TestSourcesListBundlesWhole(t *testing.T) {
	m, _ := testModel(t)
	src := m.Sources()
	// One row for the whole bundle — parts are not exploded.
	if len(src) != 1 || src[0].Kind != sourceBundle || src[0].Alias != "ui" {
		t.Fatalf("sources = %v, want one bundle row", src)
	}
}

func TestPlaceSource(t *testing.T) {
	m, _ := testModel(t)
	src := m.Sources()
	m.PlaceSource(src[0])
	m.PlaceSource(src[0]) // same bundle again: name must stay unique
	if got := m.NodeNames(); len(got) != 2 || got[0] != "ui" || got[1] != "ui2" {
		t.Fatalf("node names = %v", got)
	}
	n := &m.scene.Nodes[0]
	// A bundle with machines places as one machine node.
	if n.Kind != lottie.SceneNodeMachine || n.Source.ID != "btn" {
		t.Fatalf("placed node = %v %q, want the bundle's machine", n.Kind, n.Source.ID)
	}
	// Centered in the 1280x720 default design box; the clip is 100x100.
	if n.Transform.X != 590 || n.Transform.Y != 310 {
		t.Errorf("placed at (%v,%v), want centered (590,310)", n.Transform.X, n.Transform.Y)
	}
	if m.Player() == nil {
		t.Fatalf("player did not build: %v", m.PlayerErr())
	}
}

func TestSetNodeContent(t *testing.T) {
	m, _ := testModel(t)
	m.PlaceSource(m.Sources()[0])
	m.SetNodeContent(0, lottie.SceneNodeAnimation, "logo")
	n := &m.scene.Nodes[0]
	if n.Kind != lottie.SceneNodeAnimation || n.Source.ID != "logo" {
		t.Fatalf("content switch left %v %q", n.Kind, n.Source.ID)
	}
	if !n.Playback.Loop || !n.Playback.Autoplay {
		t.Error("clip content should default to loop + autoplay")
	}
	if m.Player() == nil {
		t.Fatalf("player did not rebuild: %v", m.PlayerErr())
	}
	if len(m.Problems()) != 0 {
		t.Errorf("problems: %v", m.Problems())
	}
}

func TestMoveAndDeleteNode(t *testing.T) {
	m, _ := testModel(t)
	src := m.Sources()
	m.PlaceSource(src[0])
	m.PlaceSource(src[0])
	m.MoveNode(1, -1)
	if got := m.NodeNames(); got[0] != "ui2" || got[1] != "ui" {
		t.Fatalf("after move: %v", got)
	}
	// Deleting a node clears focus links that pointed at it.
	m.scene.Nodes[1].Focus.Neighbors.Up = "ui2"
	m.scene.Options.InitialFocus = "ui2"
	m.DeleteNode(0)
	if got := m.scene.Nodes[0].Focus.Neighbors.Up; got != "" {
		t.Errorf("dangling neighbor %q survived delete", got)
	}
	if m.scene.Options.InitialFocus != "" {
		t.Error("dangling initial focus survived delete")
	}
}

func TestRenameNodeRepointsLinks(t *testing.T) {
	m, _ := testModel(t)
	src := m.Sources()
	m.PlaceSource(src[0])
	m.PlaceSource(src[0])
	m.scene.Nodes[0].Focus.Neighbors.Down = "ui2"
	m.scene.Options.InitialFocus = "ui2"
	m.RenameNode(1, "start")
	if got := m.scene.Nodes[0].Focus.Neighbors.Down; got != "start" {
		t.Errorf("neighbor link = %q, want start", got)
	}
	if m.scene.Options.InitialFocus != "start" {
		t.Errorf("initial focus = %q, want start", m.scene.Options.InitialFocus)
	}
	// A duplicate name is refused.
	m.RenameNode(0, "start")
	if m.scene.Nodes[0].Name == "start" {
		t.Error("duplicate rename went through")
	}
}

func TestDragRoundsAndPatchesLivePlayer(t *testing.T) {
	m, _ := testModel(t)
	placeClip(t, m)
	sp := m.Player()
	if sp == nil {
		t.Fatalf("no player: %v", m.PlayerErr())
	}
	gen := m.docGen
	m.DragNode(0, 10.567, -3.333)
	if m.docGen != gen {
		t.Error("drag bumped docGen; playback would restart mid-drag")
	}
	n := &m.scene.Nodes[0]
	if n.Transform.X != 600.57 || n.Transform.Y != 306.67 {
		t.Errorf("dragged to (%v,%v), want two-decimal rounding", n.Transform.X, n.Transform.Y)
	}
	live, _ := sp.Node("ui")
	if live.Transform().X != n.Transform.X {
		t.Error("live player node was not patched")
	}
}

func TestSaveOpenRoundTrip(t *testing.T) {
	m, dir := testModel(t)
	m.PlaceSource(m.Sources()[0])
	m.scene.Nodes[0].Focus.Focusable = true
	scenePath := filepath.Join(dir, "menu.scene.json")
	m.Save(scenePath)
	if _, err := os.Stat(scenePath); err != nil {
		t.Fatal(err)
	}

	m2 := NewModel()
	m2.Open(scenePath)
	if got := m2.NodeNames(); len(got) != 1 || got[0] != "ui" {
		t.Fatalf("reloaded nodes = %v", got)
	}
	if _, ok := m2.Bundle("ui"); !ok {
		t.Fatalf("relative bundle path did not resolve: %v", m2.bundleErrs)
	}
	if m2.Player() == nil {
		t.Fatalf("player did not build after reload: %v", m2.PlayerErr())
	}
	if len(m2.Problems()) != 0 {
		t.Errorf("problems after round-trip: %v", m2.Problems())
	}
}

func TestNodeStartAndDuration(t *testing.T) {
	m, _ := testModel(t)
	placeClip(t, m) // logo: 30 frames at 60fps = 0.5s
	n := &m.scene.Nodes[0]
	n.Playback.Loop = false
	if d, ok := m.NodeDuration(n); !ok || d != 0.5 {
		t.Errorf("duration = %v %v, want 0.5 true", d, ok)
	}
	// A chain adds one pass per link and opens at a looping tail.
	n.Playback.Then = []lottie.ScenePlayStep{{Loop: true}}
	if d, ok := m.NodeDuration(n); ok || d != 1.0 {
		t.Errorf("chained duration = %v %v, want 1.0 open", d, ok)
	}
	n.Playback.Then = nil
	gen := m.docGen
	m.SetNodeStart(0, 1.234567)
	if n.Start != 1.23 {
		t.Errorf("start = %v, want two-decimal rounding", n.Start)
	}
	if m.docGen != gen {
		t.Error("SetNodeStart bumped docGen mid-drag")
	}
	m.CommitNodeStart()
	if m.docGen == gen {
		t.Error("CommitNodeStart did not rebuild")
	}
	m.SetNodeStart(0, -5)
	if n.Start != 0 {
		t.Errorf("negative start not clamped: %v", n.Start)
	}
	// A machine's length is open-ended.
	m.PlaceSource(m.Sources()[0])
	if _, ok := m.NodeDuration(&m.scene.Nodes[1]); ok {
		t.Error("machine node reported a closed duration")
	}
}

func TestTransportStartsStoppedAndAutoPauses(t *testing.T) {
	m, _ := testModel(t)
	placeClip(t, m)
	m.scene.Nodes[0].Playback.Loop = false // 0.5s clip, closed
	m.touch()
	if m.ScenePlaying() {
		t.Fatal("transport running before Play was pressed")
	}
	if end := m.ContentEnd(); end != 0.5 {
		t.Fatalf("content end = %v, want 0.5", end)
	}
	// Not playing: ticks advance nothing.
	m.EditTick()
	if got := m.Player().Time(); got != 0 {
		t.Fatalf("paused transport advanced to %v", got)
	}
	m.TogglePlayback()
	for range 40 {
		m.EditTick()
	}
	if m.ScenePlaying() {
		t.Error("transport did not pause at the content end")
	}
	if got := m.Player().Time(); got < 0.5 {
		t.Errorf("paused at %v, want at/after the end 0.5", got)
	}
	// Play at the end starts over.
	m.TogglePlayback()
	if !m.ScenePlaying() || m.Player().Time() != 0 {
		t.Errorf("play-at-end: playing=%v time=%v, want restart", m.ScenePlaying(), m.Player().Time())
	}
}

func TestLoopedClipStillPlaysOnce(t *testing.T) {
	// The bug: a freshly placed clip loops, loops were excluded from the
	// content end, so Play parked itself at 0.00s immediately.
	m, _ := testModel(t)
	placeClip(t, m) // loop + autoplay defaults
	if end := m.ContentEnd(); end != 0.5 {
		t.Fatalf("content end = %v, want one pass 0.5", end)
	}
	m.TogglePlayback()
	for range 10 {
		m.EditTick()
	}
	if !m.ScenePlaying() {
		t.Fatal("transport stopped mid-choreography")
	}
	if got := m.Player().Time(); got <= 0 {
		t.Fatalf("clock did not advance: %v", got)
	}
}

func TestSeekScene(t *testing.T) {
	m, _ := testModel(t)
	placeClip(t, m)
	m.scene.Nodes[0].Start = 0.3
	m.touch()
	m.SeekScene(0.4)
	sp := m.Player()
	if got := sp.Time(); got < 0.39 || got > 0.41 {
		t.Fatalf("seeked to %v, want ~0.4", got)
	}
	n, _ := sp.Node("ui")
	if !n.Started() {
		t.Error("seek past the entrance left the node unentered")
	}
	if m.ScenePlaying() {
		t.Error("scrubbing should park the transport")
	}
	m.SeekScene(0.1)
	if n2, _ := m.Player().Node("ui"); n2.Started() {
		t.Error("seek before the entrance left the node entered")
	}
}

func TestSwapNodes(t *testing.T) {
	m, _ := testModel(t)
	m.PlaceSource(m.Sources()[0])
	m.PlaceSource(m.Sources()[0])
	m.SelectNode(0)
	m.SwapNodes(0, 1)
	if got := m.NodeNames(); got[0] != "ui2" || got[1] != "ui" {
		t.Fatalf("after swap: %v", got)
	}
	if m.SelectedNodeIndex() != 1 {
		t.Error("selection did not follow the swap")
	}
}

func TestPhaseOps(t *testing.T) {
	m, _ := testModel(t)
	m.PlaceSource(m.Sources()[0])
	m.AddPhase()
	m.AddPhase()
	if got := m.PhaseNames(); len(got) != 2 || got[0] != "phase" || got[1] != "phase2" {
		t.Fatalf("phases = %v", got)
	}
	m.scene.Phases[0].Duration = 1
	m.scene.Phases[0].Next = "phase2"
	m.scene.Nodes[0].Phase = "phase"
	m.scene.Nodes[0].Bindings = []lottie.SceneBinding{{On: lottie.SceneActivate, Do: lottie.ScenePhaseAction, Arg: "phase"}}

	m.RenamePhase(0, "intro")
	if m.scene.Phases[0].Name != "intro" || m.scene.Nodes[0].Phase != "intro" {
		t.Error("rename did not repoint the node")
	}
	if m.scene.Nodes[0].Bindings[0].Arg != "intro" {
		t.Error("rename did not repoint the binding")
	}
	m.RenamePhase(1, "main")
	if m.scene.Phases[0].Next != "main" {
		t.Error("rename did not repoint next")
	}

	if len(m.Problems()) != 0 {
		t.Errorf("valid phased scene reported %v", m.Problems())
	}
	if m.Player() == nil {
		t.Fatalf("player failed: %v", m.PlayerErr())
	}
	if got := m.Player().Phase(); got != "intro" {
		t.Errorf("player phase = %q, want intro", got)
	}

	m.DeletePhase(0)
	if m.scene.Nodes[0].Phase != "" {
		t.Error("delete did not release the node")
	}
	if m.scene.Nodes[0].Bindings[0].Arg != "" {
		t.Error("delete did not clear the binding arg")
	}
}

func TestImageAndTextPlacement(t *testing.T) {
	m, dir := testModel(t)
	// A tiny PNG next to the scene.
	img := image.NewRGBA(image.Rect(0, 0, 8, 4))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	imgPath := filepath.Join(dir, "badge.png")
	if err := os.WriteFile(imgPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	m.AddImage(imgPath)
	src := m.Sources()
	last := src[len(src)-1]
	if last.Kind != sourceImage || last.Alias != "badge" {
		t.Fatalf("image source = %v", last)
	}
	m.PlaceSource(last)
	n := m.SelectedNode()
	if n == nil || n.Kind != lottie.SceneNodeImage || n.Source.Image != "badge" {
		t.Fatalf("placed image node = %+v", n)
	}

	// Text needs a font first.
	m.AddTextNode()
	if m.SelectedNode().Kind == lottie.SceneNodeText {
		t.Fatal("text node placed without a font")
	}
	fontAbs, err := filepath.Abs(filepath.Join("..", "..", testFontPathRel))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fontAbs); err != nil {
		t.Skipf("test font unavailable: %v", err)
	}
	m.AddFont(fontAbs)
	m.AddTextNode()
	n = m.SelectedNode()
	if n == nil || n.Kind != lottie.SceneNodeText || n.Text.Font == "" {
		t.Fatalf("placed text node = %+v", n)
	}
	if m.Player() == nil {
		t.Fatalf("player failed with assets: %v", m.PlayerErr())
	}
	if len(m.Problems()) != 0 {
		t.Errorf("problems: %v", m.Problems())
	}
}

const testFontPathRel = "examples/lottie/stopwatch/assets/LuckiestGuy-Regular.ttf"

func TestRemoveBundleKeepsNodesAsProblems(t *testing.T) {
	m, _ := testModel(t)
	m.PlaceSource(m.Sources()[0])
	m.RemoveBundle("ui")
	if len(m.scene.Nodes) != 1 {
		t.Fatal("node vanished with its bundle")
	}
	if len(m.Problems()) == 0 {
		t.Error("broken reference reported no problem")
	}
	if m.Player() != nil {
		t.Error("player built against a missing bundle")
	}
}

func TestCameraEditAndEditModeNeutralizes(t *testing.T) {
	m, _ := testModel(t)
	placeClip(t, m)

	// The document takes the camera; a zoom of exactly 1 stays out of it.
	m.SetSceneCamera(lottie.SceneCamera{X: 40, Zoom: 1})
	if m.scene.Camera.X != 40 || m.scene.Camera.Zoom != 0 {
		t.Fatalf("scene camera = %+v, want x=40 with zoom normalized away", m.scene.Camera)
	}

	// Edit mode arranges without the camera: the rebuilt player runs the
	// identity even though the document pans.
	sp := m.Player()
	if sp == nil {
		t.Fatalf("player did not build: %v", m.PlayerErr())
	}
	if got := sp.Camera(); got != (lottie.SceneCamera{}) {
		t.Fatalf("edit-mode camera = %+v, want identity", got)
	}

	// Preview mode plays the document's camera, the way a game would.
	m.TogglePreview()
	sp = m.Player()
	if got := sp.Camera(); got.X != 40 {
		t.Fatalf("preview camera = %+v, want the document's x=40", got)
	}
	m.TogglePreview()
	if got := m.Player().Camera(); got != (lottie.SceneCamera{}) {
		t.Fatalf("camera after leaving preview = %+v, want identity", got)
	}
}

func TestSetNodeDepthNormalizesDefault(t *testing.T) {
	m, _ := testModel(t)
	placeClip(t, m)
	i := m.SelectedNodeIndex()
	m.SetNodeDepth(i, 0.5)
	if d := m.scene.Nodes[i].ParallaxDepth(); d != 0.5 {
		t.Fatalf("depth = %v, want 0.5", d)
	}
	// 1 is the default and must leave the document.
	m.SetNodeDepth(i, 1)
	if m.scene.Nodes[i].Depth != nil {
		t.Fatalf("depth 1 stored as %v, want absent", *m.scene.Nodes[i].Depth)
	}
	// 0 is meaningful (screen-pinned) and must stay.
	m.SetNodeDepth(i, 0)
	if d := m.scene.Nodes[i].Depth; d == nil || *d != 0 {
		t.Fatalf("depth 0 not stored")
	}
}

func TestPhaseCameraOverride(t *testing.T) {
	m, _ := testModel(t)
	m.AddPhase()
	m.SetPhaseCamera(0, &lottie.SceneCamera{X: 100, Zoom: 1})
	p := &m.scene.Phases[0]
	if p.Camera == nil || p.Camera.X != 100 || p.Camera.Zoom != 0 {
		t.Fatalf("phase camera = %+v, want x=100 with zoom normalized away", p.Camera)
	}
	m.SetPhaseCamera(0, nil)
	if p.Camera != nil {
		t.Fatal("phase camera override not cleared")
	}
}
