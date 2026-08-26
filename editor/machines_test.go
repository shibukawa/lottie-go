package main

import (
	"path/filepath"
	"slices"
	"testing"

	lottie "github.com/shibukawa/lottie-go"
)

// A bundle can hold several state machines: the manifest lists them and
// each is its own file under s/. The toolbar's combobox picks between them.
func TestBundleHoldsSeveralStateMachines(t *testing.T) {
	dir := t.TempDir()
	m := NewModel()
	m.ImportClip(writeClip(t, dir, "idle-anim", 30, ""))

	m.NewMachine() // "machine"
	first := m.MachineID()
	m.AddState()
	m.RenameState(m.SelectedStateName(), "idle-state")

	m.NewMachine() // "machine2": ids do not collide
	second := m.MachineID()
	m.AddState()
	m.RenameState(m.SelectedStateName(), "other-state")

	if first == second {
		t.Fatalf("both machines got the id %q", first)
	}
	if got := m.MachineIDs(); !slices.Equal(got, []string{first, second}) {
		t.Fatalf("MachineIDs() = %v; want %v", got, []string{first, second})
	}

	// Switching is what the combobox does, and each keeps its own states.
	m.SelectMachine(first)
	if got := m.StateNames(); !slices.Equal(got, []string{"idle-state"}) {
		t.Errorf("machine %q states = %v", first, got)
	}
	m.SelectMachine(second)
	if got := m.StateNames(); !slices.Equal(got, []string{"other-state"}) {
		t.Errorf("machine %q states = %v", second, got)
	}

	// Both survive a save and reopen, as separate files under s/.
	out := filepath.Join(dir, "two.lottie")
	m.Save(out)
	reopened := NewModel()
	reopened.Open(out)
	if got := reopened.MachineIDs(); !slices.Equal(got, []string{first, second}) {
		t.Fatalf("after reopen MachineIDs() = %v; want %v", got, []string{first, second})
	}
	for _, id := range []string{first, second} {
		if _, err := reopened.Bundle().StateMachine(id); err != nil {
			t.Errorf("machine %q did not survive: %v", id, err)
		}
	}
	// Opening lands on the first one, and it runs.
	if reopened.MachineID() != first {
		t.Errorf("opened onto %q; want the first listed, %q", reopened.MachineID(), first)
	}
	if reopened.Preview() == nil {
		t.Errorf("preview did not start: %v", reopened.PreviewErr())
	}
}

// Machines are independent: the clips are shared, the graphs are not.
func TestMachinesShareClipsButNotStates(t *testing.T) {
	dir := t.TempDir()
	m := NewModel()
	m.ImportClip(writeClip(t, dir, "a-anim", 30, ""))
	m.NewMachine()
	m.AddState()
	m.NewMachine()

	// The second machine starts empty but sees the same clips.
	if got := m.StateNames(); len(got) != 0 {
		t.Errorf("a new machine starts with states %v", got)
	}
	if got := m.AnimationIDs(); !slices.Equal(got, []string{"a-anim"}) {
		t.Errorf("AnimationIDs() = %v; clips are shared across machines", got)
	}
	m.AddState()
	st := m.SelectedState()
	if st == nil || st.Animation != "a-anim" {
		t.Errorf("a state in the second machine did not pick up the shared clip: %+v", st)
	}
	if st.Type != lottie.StatePlayback {
		t.Errorf("state type = %q", st.Type)
	}
}
