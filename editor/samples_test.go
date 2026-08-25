package main

import (
	"path/filepath"
	"testing"
)

// The committed samples under testdata/editor are the first thing anyone
// opens, so they have to load clean and be drivable. Regenerate them with
// `go run ./gensamples` if these fail after a change.

func sampleDir(name string) string {
	return filepath.Join("..", "testdata", "editor", name)
}

func openSample(t *testing.T, dir, file string) *Model {
	t.Helper()
	m := NewModel()
	m.Open(filepath.Join(sampleDir(dir), file))
	if m.Machine() == nil {
		t.Fatalf("%s: no machine loaded: %s", file, m.Status())
	}
	if probs := m.Problems(); len(probs) != 0 {
		t.Fatalf("%s: validation problems: %v", file, probs)
	}
	if m.Preview() == nil {
		t.Fatalf("%s: preview did not start: %v", file, m.PreviewErr())
	}
	if m.PreviewStale() {
		t.Errorf("%s: reports as edited straight after loading", file)
	}
	return m
}

// fire raises a trigger and ticks until the machine settles somewhere.
func fire(t *testing.T, m *Model, trigger, want string, limit int) {
	t.Helper()
	sm := m.Preview()
	sm.Fire(trigger)
	for range limit {
		sm.Update()
		if sm.State() == want {
			return
		}
	}
	t.Fatalf("after firing %q the machine is in %q; want %q", trigger, sm.State(), want)
}

func TestCharacterSample(t *testing.T) {
	m := openSample(t, "character", "character.lottie")

	if got := len(m.AnimationIDs()); got != 5 {
		t.Errorf("AnimationIDs() = %d clips; want 5", got)
	}
	if got := m.EventInputs(); len(got) != 6 {
		t.Errorf("EventInputs() = %v; want six verbs", got)
	}
	sm := m.Preview()
	if sm.State() != "idle" {
		t.Fatalf("initial state = %q; want idle", sm.State())
	}

	fire(t, m, "walk", "walk", 2)
	fire(t, m, "run", "run", 2)
	fire(t, m, "stop", "idle", 2)

	// jump is a one-shot: it must come home on its own, no timer.
	fire(t, m, "jump", "jump", 2)
	for range 60 {
		sm.Update()
		if sm.State() == "idle" {
			break
		}
	}
	if sm.State() != "idle" {
		t.Errorf("jump did not return to idle; stuck in %q", sm.State())
	}

	// The global state makes damage reachable from anywhere.
	fire(t, m, "walk", "walk", 2)
	fire(t, m, "hurt", "hurt", 2)
}

// jump is guarded on a boolean the game owns, so clearing it blocks the jump
// while leaving every other verb working.
func TestCharacterJumpRespectsGroundedGuard(t *testing.T) {
	m := openSample(t, "character", "character.lottie")
	sm := m.Preview()

	sm.Set("grounded", false)
	sm.Fire("jump")
	for range 5 {
		sm.Update()
	}
	if sm.State() != "idle" {
		t.Errorf("jumped while not grounded; state = %q", sm.State())
	}
	sm.Set("grounded", true)
	fire(t, m, "jump", "jump", 2)
}

func TestSpritesheetSampleUsesSegments(t *testing.T) {
	m := openSample(t, "spritesheet", "spritesheet.lottie")
	sm := m.Preview()

	anim, err := m.Bundle().Animation("actions")
	if err != nil {
		t.Fatal(err)
	}
	if got := len(anim.Markers()); got != 3 {
		t.Fatalf("Markers() = %d; want 3", got)
	}
	// Each state plays only its own marker range.
	idleMarker, ok := anim.Marker("idle")
	if !ok {
		t.Fatal("no idle marker")
	}
	start, end := sm.Player().Range()
	if start != idleMarker.Start || end != idleMarker.End {
		t.Errorf("idle range = [%v,%v); want the marker's [%v,%v)",
			start, end, idleMarker.Start, idleMarker.End)
	}

	fire(t, m, "walk", "walk", 2)
	walkMarker, _ := anim.Marker("walk")
	start, end = sm.Player().Range()
	if start != walkMarker.Start || end != walkMarker.End {
		t.Errorf("walk range = [%v,%v); want [%v,%v)", start, end, walkMarker.Start, walkMarker.End)
	}
}

// The samples ship with a hand-arranged graph, carried in each state's extra
// fields rather than the fallback grid.
func TestSamplesCarryNodePositions(t *testing.T) {
	for _, tt := range []struct{ dir, file string }{
		{"character", "character.lottie"},
		{"spritesheet", "spritesheet.lottie"},
	} {
		t.Run(tt.dir, func(t *testing.T) {
			m := openSample(t, tt.dir, tt.file)
			for i := range m.Machine().States {
				if _, ok := nodePos(&m.Machine().States[i]); !ok {
					t.Errorf("state %q has no stored position", m.Machine().States[i].Name)
				}
			}
		})
	}
}

// Loose clips are kept beside each bundle so Import can be tried on them.
func TestSampleClipsImportIndividually(t *testing.T) {
	m := NewModel()
	for _, name := range []string{"idle", "walk", "run", "jump", "hurt"} {
		m.ImportClip(filepath.Join(sampleDir("character"), name+".json"))
	}
	if got := m.AnimationIDs(); len(got) != 5 {
		t.Fatalf("AnimationIDs() = %v; want 5 clips (%s)", got, m.Status())
	}
	for _, id := range m.AnimationIDs() {
		anim, err := m.Bundle().Animation(id)
		if err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		if u := anim.UnsupportedFeatures(); len(u) != 0 {
			t.Errorf("%s uses features the player skips: %v", id, u)
		}
	}
}
