package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	lottie "github.com/shibukawa/lottie-go"
)

func clipJSON(frames int, markers string) []byte {
	m := ""
	if markers != "" {
		m = `,"markers":` + markers
	}
	return fmt.Appendf(nil, `{"v":"5.9.0","fr":60,"ip":0,"op":%d,"w":100,"h":100,
		"layers":[{"ty":3,"nm":"anchor","ind":1,"ip":0,"op":%d,"st":0,
		"ks":{"a":{"a":0,"k":[0,0]},"p":{"a":0,"k":[50,50]},
		"s":{"a":0,"k":[100,100]},"r":{"a":0,"k":0},"o":{"a":0,"k":100}}}]`+m+`}`, frames, frames)
}

func writeClip(t *testing.T, dir, name string, frames int, markers string) string {
	t.Helper()
	p := filepath.Join(dir, name+".json")
	if err := os.WriteFile(p, clipJSON(frames, markers), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// The whole authoring path: import clips, wire a machine, save, reopen.
func TestAuthorAndRoundTrip(t *testing.T) {
	dir := t.TempDir()
	idle := writeClip(t, dir, "idle", 60, "")
	jump := writeClip(t, dir, "jump", 3, "")

	m := NewModel()
	m.ImportClip(idle)
	m.ImportClip(jump)
	if got := m.AnimationIDs(); len(got) != 2 {
		t.Fatalf("AnimationIDs() = %v; want 2 clips (%s)", got, m.Status())
	}

	m.NewMachine()
	m.AddState() // becomes "state", initial, wired to the first clip
	first := m.SelectedStateName()
	m.RenameState(first, "idle")
	m.AddState()
	m.RenameState(m.SelectedStateName(), "jump")

	if got := m.StateNames(); len(got) != 2 || got[0] != "idle" || got[1] != "jump" {
		t.Fatalf("StateNames() = %v; want [idle jump]", got)
	}
	if m.Machine().Initial != "idle" {
		t.Errorf("Initial = %q; want idle", m.Machine().Initial)
	}

	m.AddInput(lottie.InputEvent)
	m.RenameInput(0, "jump")
	if got := m.EventInputs(); len(got) != 1 || got[0] != "jump" {
		t.Fatalf("EventInputs() = %v; want [jump]", got)
	}

	m.SelectState("idle")
	m.AddTransition("jump")
	m.AddGuard()
	if g := m.SelectedTransition().Guards; len(g) != 1 || g[0].InputName != "jump" {
		t.Fatalf("guard = %+v; want one on the jump input", g)
	}

	// Move a node so the position has something to preserve.
	m.SetNodePos(1, image.Pt(70, 80))
	out := filepath.Join(dir, "character.lottie")
	m.Save(out)
	if !strings.Contains(m.Status(), "saved") {
		t.Fatalf("save failed: %s", m.Status())
	}

	reopened := NewModel()
	reopened.Open(out)
	if reopened.Machine() == nil {
		t.Fatalf("no machine after reopen: %s", reopened.Status())
	}
	if got := reopened.StateNames(); len(got) != 2 || got[0] != "idle" {
		t.Errorf("StateNames() = %v; want [idle jump]", got)
	}
	if got := reopened.EventInputs(); len(got) != 1 || got[0] != "jump" {
		t.Errorf("EventInputs() = %v; want [jump]", got)
	}
	// Node positions ride in the state's extra fields, which is the whole
	// reason ExtraFields is exported.
	if got := reopened.NodePos(1); got != image.Pt(70, 80) {
		t.Errorf("NodePos(1) = %v; want the saved position %v", got, image.Pt(70, 80))
	}
	if probs := reopened.Problems(); len(probs) != 0 {
		t.Errorf("Problems() = %v; want none", probs)
	}
}

func TestPreviewRunsTheRealInterpreter(t *testing.T) {
	dir := t.TempDir()
	m := NewModel()
	m.ImportClip(writeClip(t, dir, "idle", 60, ""))
	m.ImportClip(writeClip(t, dir, "jump", 3, ""))

	m.NewMachine()
	m.AddState()
	m.RenameState(m.SelectedStateName(), "idle")
	m.AddState()
	m.RenameState(m.SelectedStateName(), "jump")
	// jump is a one-shot that ends and comes back.
	st, _ := m.Machine().State("jump")
	st.Animation = "jump"
	st.Loop = false
	idle, _ := m.Machine().State("idle")
	idle.Animation = "idle"

	m.AddInput(lottie.InputEvent)
	m.RenameInput(0, "jump")
	m.SelectState("idle")
	m.AddTransition("jump")
	m.AddGuard()

	// OnComplete brings it home without a game-side timer.
	m.Machine().Interactions = append(m.Machine().Interactions, lottie.Interaction{
		Type:    lottie.InteractionOnComplete,
		Actions: []lottie.Action{{Type: lottie.ActionFire, InputName: "done"}},
	})
	m.AddInput(lottie.InputEvent)
	m.RenameInput(1, "done")
	m.SelectState("jump")
	m.AddTransition("idle")
	m.AddGuard()
	m.SelectedTransition().Guards[0].InputName = "done"
	m.Touch()

	m.RestartPreview()
	sm := m.Preview()
	if sm == nil {
		t.Fatalf("no preview: %v", m.PreviewErr())
	}
	if sm.State() != "idle" {
		t.Fatalf("State() = %q; want idle", sm.State())
	}
	sm.Fire("jump")
	sm.Update()
	if sm.State() != "jump" {
		t.Fatalf("State() = %q; want jump", sm.State())
	}
	for range 5 {
		sm.Update()
		if sm.State() == "idle" {
			return
		}
	}
	t.Errorf("did not return to idle; stuck in %q", sm.State())
}

func TestDeleteStateDropsDanglingTransitions(t *testing.T) {
	dir := t.TempDir()
	m := NewModel()
	m.ImportClip(writeClip(t, dir, "a", 10, ""))
	m.NewMachine()
	m.AddState()
	m.RenameState(m.SelectedStateName(), "a")
	m.AddState()
	m.RenameState(m.SelectedStateName(), "b")
	m.SelectState("a")
	m.AddTransition("b")

	m.DeleteState("b")
	if got := m.StateNames(); len(got) != 1 || got[0] != "a" {
		t.Fatalf("StateNames() = %v; want [a]", got)
	}
	st, _ := m.Machine().State("a")
	if len(st.Transitions) != 0 {
		t.Errorf("transition to a deleted state survived: %+v", st.Transitions)
	}
	// Which means the document must still validate.
	if probs := m.Problems(); len(probs) != 0 {
		t.Errorf("Problems() = %v; want none", probs)
	}
}

func TestRenameStateRepointsTransitions(t *testing.T) {
	dir := t.TempDir()
	m := NewModel()
	m.ImportClip(writeClip(t, dir, "a", 10, ""))
	m.NewMachine()
	m.AddState()
	m.RenameState(m.SelectedStateName(), "a")
	m.AddState()
	m.RenameState(m.SelectedStateName(), "b")
	m.SelectState("a")
	m.AddTransition("b")

	m.RenameState("b", "bee")
	st, _ := m.Machine().State("a")
	if st.Transitions[0].ToState != "bee" {
		t.Errorf("ToState = %q; want bee", st.Transitions[0].ToState)
	}
	if probs := m.Problems(); len(probs) != 0 {
		t.Errorf("Problems() = %v; want none", probs)
	}
}

func TestRenameInputRepointsGuards(t *testing.T) {
	dir := t.TempDir()
	m := NewModel()
	m.ImportClip(writeClip(t, dir, "a", 10, ""))
	m.NewMachine()
	m.AddState()
	m.AddInput(lottie.InputEvent)
	m.SelectState(m.StateNames()[0])
	m.AddTransition(m.StateNames()[0])
	m.AddGuard()
	m.RenameInput(0, "go")
	if got := m.SelectedTransition().Guards[0].InputName; got != "go" {
		t.Errorf("guard input = %q; want go", got)
	}
}

func TestMarkersFeedTheSegmentPicker(t *testing.T) {
	dir := t.TempDir()
	m := NewModel()
	m.ImportClip(writeClip(t, dir, "sheet", 90,
		`[{"tm":0,"cm":"idle","dr":30},{"tm":30,"cm":"walk","dr":30}]`))
	got := m.Markers("sheet")
	if len(got) != 2 || got[0] != "idle" || got[1] != "walk" {
		t.Errorf("Markers() = %v; want [idle walk]", got)
	}
	// The summary shares a row with the clip name, so it stays terse.
	if s := m.ClipSummary("sheet"); !strings.Contains(s, "▾2") {
		t.Errorf("ClipSummary() = %q; want it to note two markers", s)
	}
}

func TestOpenRejectsGarbage(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.lottie")
	if err := os.WriteFile(bad, []byte("not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewModel()
	m.Open(bad)
	if !strings.Contains(m.Status(), "load failed") {
		t.Errorf("Status() = %q; want a load failure", m.Status())
	}
	if m.Machine() != nil {
		t.Error("a failed open left a machine behind")
	}
}

// Problems runs from Build, and widgets hash the generation in their state
// key. If validating counted as an edit, the key would never settle and the
// tree would rebuild every tick — which also made the preview read as
// permanently stale.
func TestProblemsDoesNotCountAsAnEdit(t *testing.T) {
	dir := t.TempDir()
	m := NewModel()
	m.ImportClip(writeClip(t, dir, "a", 10, ""))
	m.NewMachine()
	m.AddState()
	m.RestartPreview()
	if m.PreviewStale() {
		t.Fatal("preview is stale immediately after a restart")
	}
	gen := m.Generation()
	for range 5 {
		m.Problems()
	}
	if got := m.Generation(); got != gen {
		t.Errorf("Generation() = %d after validating; want %d unchanged", got, gen)
	}
	if m.PreviewStale() {
		t.Error("validating marked the preview stale")
	}
}

// Two fresh states must not be drawn on top of each other.
func TestNewStatesDoNotOverlap(t *testing.T) {
	dir := t.TempDir()
	m := NewModel()
	m.ImportClip(writeClip(t, dir, "a", 10, ""))
	m.NewMachine()
	m.AddState()
	m.AddState()
	a, b := m.NodePos(0), m.NodePos(1)
	// Nodes are 6 unit-sizes wide; at any sane unit size 200px clears them.
	if dx := b.X - a.X; dx < 200 && a.Y == b.Y {
		t.Errorf("states at %v and %v overlap on one row", a, b)
	}
}

// Opening a file must leave the preview matching what was loaded. Open
// bumped the generation after starting the preview, so a freshly loaded
// bundle claimed to be edited.
func TestFreshlyOpenedBundleIsNotStale(t *testing.T) {
	dir := t.TempDir()
	m := NewModel()
	m.ImportClip(writeClip(t, dir, "a", 10, ""))
	m.NewMachine()
	m.AddState()
	out := filepath.Join(dir, "b.lottie")
	m.Save(out)

	reopened := NewModel()
	reopened.Open(out)
	if reopened.Preview() == nil {
		t.Fatalf("no preview after open: %v / %s", reopened.PreviewErr(), reopened.Status())
	}
	if reopened.PreviewStale() {
		t.Error("a freshly opened bundle reports itself as edited")
	}
	// And it stays settled while the UI rebuilds against it.
	for range 3 {
		reopened.Problems()
	}
	if reopened.PreviewStale() {
		t.Error("rebuilding the inspector marked the fresh bundle as edited")
	}
}
