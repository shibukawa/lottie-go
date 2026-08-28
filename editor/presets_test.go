package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	lottie "github.com/shibukawa/lottie-go"
)

// The committed preset under testdata/presets is the template the AI
// customization workflow starts from, so beyond loading clean it must
// decode with zero unsupported features — that emptiness is the baseline
// the workflow's own validation compares against. Regenerate with
// `go run ./genpresets` if these fail after a change.

func presetPath(name string) string {
	return filepath.Join("..", "testdata", "presets", name, name+".lottie")
}

// settle ticks until the machine reaches want, for following one-shot
// chains without firing anything.
func settle(t *testing.T, sm *lottie.StateMachinePlayer, want string, limit int) {
	t.Helper()
	for range limit {
		sm.Update()
		if sm.State() == want {
			return
		}
	}
	t.Fatalf("machine settled in %q; want %q", sm.State(), want)
}

func TestChibiMalePreset(t *testing.T) {
	m := NewModel()
	m.Open(presetPath("chibi-male"))
	if m.Machine() == nil {
		t.Fatalf("no machine loaded: %s", m.Status())
	}
	if probs := m.Problems(); len(probs) != 0 {
		t.Fatalf("validation problems: %v", probs)
	}
	if m.Preview() == nil {
		t.Fatalf("preview did not start: %v", m.PreviewErr())
	}
	if got := len(m.AnimationIDs()); got != 19 {
		t.Errorf("AnimationIDs() = %d clips; want 19", got)
	}

	sm := m.Preview()
	if sm.State() != "idle-state" {
		t.Fatalf("initial state = %q; want idle-state", sm.State())
	}

	// Locomotion round trip, including a turn bridge and the braking stop.
	fire(t, m, "walk", "walk-state", 10)
	fire(t, m, "run", "run-state", 10)
	fire(t, m, "stop", "run-to-idle-state", 10)
	settle(t, sm, "idle-state", 60)
	fire(t, m, "turn", "idle-turn-state", 10)
	settle(t, sm, "idle-state", 40)

	// The jump chain hands over by itself and grounded=true brings it home.
	fire(t, m, "jump", "jump-state", 10)
	settle(t, sm, "idle-state", 200)

	// Punch fired during punch chains the follow-up, and the haymaker is
	// also reachable directly.
	fire(t, m, "punch", "punch-state", 10)
	fire(t, m, "punch", "punch-2-state", 10)
	settle(t, sm, "idle-state", 80)
	fire(t, m, "punch2", "punch-2-state", 10)
	settle(t, sm, "idle-state", 80)
	fire(t, m, "kick2", "kick-2-state", 10)
	settle(t, sm, "idle-state", 80)
	fire(t, m, "kick", "kick-state", 10)
	fire(t, m, "kick", "kick-2-state", 10)
	settle(t, sm, "idle-state", 80)

	// Guard toggles on the guard event; a hit while guarding plays
	// guard-hit, not hurt.
	fire(t, m, "guard", "guard-state", 10)
	fire(t, m, "hurt", "guard-hit-state", 10)
	settle(t, sm, "guard-state", 60)
	fire(t, m, "guard", "idle-state", 10)

	// Death is terminal.
	fire(t, m, "die", "death-state", 10)
	settle(t, sm, "death-state", 120)
}

func TestChibiMaleDecodesClean(t *testing.T) {
	data, err := os.ReadFile(presetPath("chibi-male"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := lottie.DecodeBundle(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range b.AnimationIDs() {
		a, err := b.Animation(id)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if notes := a.UnsupportedFeatures(); len(notes) > 0 {
			t.Errorf("%s: unsupported features: %v", id, notes)
		}
	}
}
