package main

import (
	"path/filepath"
	"testing"
)

// A clip can be judged on its own, before it is wired into any state.
func TestPreviewOneClip(t *testing.T) {
	m := openSample(t, "character", "character.lottie")
	if m.PreviewClip() != "" {
		t.Fatal("a freshly opened bundle should be previewing the machine")
	}
	if m.PreviewAnimation() == nil {
		t.Fatal("nothing on the stage")
	}

	m.ShowClip("jump")
	if m.PreviewClip() != "jump" {
		t.Errorf("PreviewClip() = %q; want jump", m.PreviewClip())
	}
	jump, _ := m.Bundle().Animation("jump")
	if m.PreviewAnimation() != jump {
		t.Error("the stage is not showing the selected clip")
	}
	// A lone clip loops, so it can be watched without restarting it.
	before := m.clipPlayer.Frame()
	m.PreviewUpdate()
	if m.clipPlayer.Frame() == before {
		t.Error("the clip did not advance")
	}
	for range 200 {
		m.PreviewUpdate()
	}
	if !m.clipPlayer.IsPlaying() {
		t.Error("the clip stopped instead of looping")
	}
	// While a clip has the stage, the machine has no active state to show.
	if m.ActiveState() != "" {
		t.Errorf("ActiveState() = %q while previewing a clip", m.ActiveState())
	}

	m.ShowMachine()
	if m.PreviewClip() != "" {
		t.Error("ShowMachine did not release the stage")
	}
	if m.ActiveState() != "idle" {
		t.Errorf("ActiveState() = %q; want idle", m.ActiveState())
	}
}

// Restarting is about the machine, so it also takes the stage back.
func TestRestartReturnsToTheMachine(t *testing.T) {
	m := openSample(t, "character", "character.lottie")
	m.ShowClip("walk")
	m.RestartPreview()
	if m.PreviewClip() != "" {
		t.Errorf("PreviewClip() = %q; Restart should return to the machine", m.PreviewClip())
	}
	if m.ActiveState() != "idle" {
		t.Errorf("ActiveState() = %q; want idle", m.ActiveState())
	}
}

// The graph follows playback: whichever state the machine is in is the one
// drawn as active.
func TestActiveStateFollowsPlayback(t *testing.T) {
	m := openSample(t, "character", "character.lottie")
	if m.ActiveState() != "idle" {
		t.Fatalf("ActiveState() = %q; want idle", m.ActiveState())
	}
	m.Fire("walk")
	m.PreviewUpdate()
	if m.ActiveState() != "walk" {
		t.Errorf("ActiveState() = %q; want walk", m.ActiveState())
	}
	m.Fire("hurt")
	m.PreviewUpdate()
	if m.ActiveState() != "hurt" {
		t.Errorf("ActiveState() = %q; want hurt", m.ActiveState())
	}
}

// Selecting an input traces the transitions that read it.
func TestSelectedInputTracesTransitions(t *testing.T) {
	m := openSample(t, "character", "character.lottie")
	names := m.InputNames()
	jump := -1
	for i, n := range names {
		if n == "jump" {
			jump = i
		}
	}
	if jump < 0 {
		t.Fatalf("no jump input in %v", names)
	}
	m.SelectInput(jump)
	if got := m.SelectedInputName(); got != "jump" {
		t.Fatalf("SelectedInputName() = %q; want jump", got)
	}

	// idle, walk and run each guard their jump transition on it; nothing
	// else does.
	var traced, plain int
	for _, st := range m.Machine().States {
		for _, tr := range st.Transitions {
			if TransitionUsesInput(tr, "jump") {
				traced++
				if tr.ToState != "jump" {
					t.Errorf("state %q traces a transition to %q", st.Name, tr.ToState)
				}
			} else {
				plain++
			}
		}
	}
	if traced != 3 {
		t.Errorf("%d transitions traced; want 3", traced)
	}
	if plain == 0 {
		t.Error("every transition traced; the highlight would be meaningless")
	}

	m.SelectInput(-1)
	if m.SelectedInputName() != "" {
		t.Error("deselecting left an input traced")
	}
	// With nothing selected nothing is traced.
	for _, st := range m.Machine().States {
		for _, tr := range st.Transitions {
			if TransitionUsesInput(tr, m.SelectedInputName()) {
				t.Fatal("a transition is traced with no input selected")
			}
		}
	}
}

// The try button beside an event fires it into the running machine.
func TestFireFromTheInputTable(t *testing.T) {
	m := openSample(t, "character", "character.lottie")
	m.Fire("walk")
	m.PreviewUpdate()
	if m.ActiveState() != "walk" {
		t.Errorf("ActiveState() = %q; want walk", m.ActiveState())
	}
	// Firing with no machine running must not panic.
	empty := NewModel()
	empty.Fire("nothing")
}

// The boolean control beside an input writes straight into the machine.
func TestSetInputValueFromTheTable(t *testing.T) {
	m := openSample(t, "character", "character.lottie")
	SetInputValue(m, "grounded", false)
	if v, _ := m.Preview().Get[bool]("grounded"); v {
		t.Error("grounded was not cleared")
	}
	m.Fire("jump")
	for range 5 {
		m.PreviewUpdate()
	}
	if m.ActiveState() != "idle" {
		t.Errorf("jumped while not grounded; state = %q", m.ActiveState())
	}
}

func TestShowClipRejectsUnknownID(t *testing.T) {
	m := openSample(t, "character", "character.lottie")
	m.ShowClip("nope")
	if m.PreviewClip() != "" {
		t.Error("an unknown clip took the stage")
	}
}

func TestPreviewLabelDescribesTheStage(t *testing.T) {
	m := openSample(t, "character", "character.lottie")
	if got := m.PreviewLabel(); got != "state: idle" {
		t.Errorf("PreviewLabel() = %q; want state: idle", got)
	}
	m.ShowClip("run")
	if got := m.PreviewLabel(); got != "clip: run" {
		t.Errorf("PreviewLabel() = %q; want clip: run", got)
	}
	empty := NewModel()
	if got := empty.PreviewLabel(); got != "no preview" {
		t.Errorf("PreviewLabel() = %q; want no preview", got)
	}
}

// A clip plays even before any machine exists, which is the point of
// previewing one on its own.
func TestClipPreviewNeedsNoMachine(t *testing.T) {
	m := NewModel()
	m.ImportClip(filepath.Join(sampleDir("character"), "idle.json"))
	if m.Machine() != nil {
		t.Fatal("no machine expected yet")
	}
	m.ShowClip("idle")
	if m.PreviewAnimation() == nil {
		t.Error("a clip should play even with no machine defined")
	}
	m.PreviewUpdate() // must not panic with no machine running
}

// Selecting things is a view change. Only editing the document may report
// the running preview as out of date.
func TestSelectionDoesNotStaleThePreview(t *testing.T) {
	m := openSample(t, "character", "character.lottie")
	if m.PreviewStale() {
		t.Fatal("stale straight after loading")
	}
	m.SelectInput(1)
	m.SelectState("walk")
	m.SelectTransition(0)
	m.ShowClip("run")
	m.ShowMachine()
	m.Problems()
	if m.PreviewStale() {
		t.Error("selecting marked the preview as edited")
	}
	// An actual edit does.
	m.AddState()
	if !m.PreviewStale() {
		t.Error("editing the document did not mark the preview as edited")
	}
	m.RestartPreview()
	if m.PreviewStale() {
		t.Error("restarting did not clear the stale mark")
	}
}

// The timeline reads the player behind the stage, whichever is showing.
func TestTimelineFollowsTheStage(t *testing.T) {
	m := openSample(t, "spritesheet", "spritesheet.lottie")
	if m.PreviewPlayer() == nil {
		t.Fatal("no player behind the stage")
	}
	// Markers are what a segment names, so the timeline can show them.
	if got := len(m.PreviewMarkers()); got != 3 {
		t.Errorf("PreviewMarkers() = %d; want 3", got)
	}
	// The band is the segment, narrower than the document.
	start, end := m.PreviewPlayer().Range()
	if start != 0 || end != 60 {
		t.Errorf("idle range = [%v,%v); want the idle marker [0,60)", start, end)
	}

	// Scrubbing moves the playhead, clamped into the playing range.
	m.PreviewSeek(30)
	if got := m.PreviewPlayer().Frame(); got != 30 {
		t.Errorf("Frame() = %v; want 30", got)
	}
	m.PreviewSeek(500)
	if got := m.PreviewPlayer().Frame(); got > end {
		t.Errorf("Frame() = %v; scrubbing escaped the range end %v", got, end)
	}

	// A clip on its own shows its whole self, markers included.
	m.ShowClip("actions")
	if got := len(m.PreviewMarkers()); got != 3 {
		t.Errorf("clip PreviewMarkers() = %d; want 3", got)
	}
	start, end = m.PreviewPlayer().Range()
	if start != 0 || end != 180 {
		t.Errorf("clip range = [%v,%v); want the whole document", start, end)
	}
}

func TestTimelineIsQuietWithNothingLoaded(t *testing.T) {
	m := NewModel()
	if m.PreviewPlayer() != nil {
		t.Error("a player exists with nothing loaded")
	}
	if m.PreviewMarkers() != nil {
		t.Error("markers exist with nothing loaded")
	}
	m.PreviewSeek(10) // must not panic
}
