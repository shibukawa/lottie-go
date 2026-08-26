package main

import (
	"path/filepath"
	"slices"
	"strings"
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

func TestRenameMachine(t *testing.T) {
	dir := t.TempDir()
	m := NewModel()
	m.ImportClip(writeClip(t, dir, "a-anim", 30, ""))
	m.NewMachine()
	m.AddState()
	old := m.MachineID()

	m.RenameMachine(old, "hero")
	if m.MachineID() != "hero" {
		t.Fatalf("MachineID() = %q; want hero", m.MachineID())
	}
	if got := m.MachineIDs(); !slices.Equal(got, []string{"hero"}) {
		t.Fatalf("MachineIDs() = %v; the old id should be gone", got)
	}
	// The states came with it: renaming moves the file, not its contents.
	if got := m.StateNames(); len(got) != 1 {
		t.Errorf("StateNames() = %v; want the state to survive", got)
	}
	// And it is written under the new name.
	out := filepath.Join(dir, "r.lottie")
	m.Save(out)
	reopened := NewModel()
	reopened.Open(out)
	if got := reopened.MachineIDs(); !slices.Equal(got, []string{"hero"}) {
		t.Errorf("after reopen MachineIDs() = %v", got)
	}
}

func TestRenameMachineRefusesACollision(t *testing.T) {
	dir := t.TempDir()
	m := NewModel()
	m.ImportClip(writeClip(t, dir, "a-anim", 30, ""))
	m.NewMachine()
	first := m.MachineID()
	m.NewMachine()
	second := m.MachineID()

	m.RenameMachine(second, first)
	if got := m.MachineIDs(); !slices.Equal(got, []string{first, second}) {
		t.Errorf("MachineIDs() = %v; a colliding rename should change nothing", got)
	}
	if !strings.Contains(m.Status(), "already exists") {
		t.Errorf("Status() = %q; want it to say why", m.Status())
	}
}

func TestDeleteMachineMovesToWhatIsLeft(t *testing.T) {
	dir := t.TempDir()
	m := NewModel()
	m.ImportClip(writeClip(t, dir, "a-anim", 30, ""))
	m.NewMachine()
	first := m.MachineID()
	m.AddState()
	m.NewMachine()
	second := m.MachineID()
	m.AddState()

	m.DeleteMachine(second)
	if got := m.MachineIDs(); !slices.Equal(got, []string{first}) {
		t.Fatalf("MachineIDs() = %v; want just %q", got, first)
	}
	// Deleting the open one lands on whatever remains, still running.
	if m.MachineID() != first {
		t.Errorf("MachineID() = %q; want %q", m.MachineID(), first)
	}
	if m.Preview() == nil {
		t.Errorf("preview did not follow: %v", m.PreviewErr())
	}
}

// Deleting the last machine leaves the editor with nothing to run rather
// than a stale player.
func TestDeleteLastMachineClearsThePreview(t *testing.T) {
	dir := t.TempDir()
	m := NewModel()
	m.ImportClip(writeClip(t, dir, "a-anim", 30, ""))
	m.NewMachine()
	m.AddState()

	m.DeleteMachine(m.MachineID())
	if got := m.MachineIDs(); len(got) != 0 {
		t.Fatalf("MachineIDs() = %v; want none", got)
	}
	if m.Machine() != nil {
		t.Error("a machine is still selected")
	}
	if m.Preview() != nil {
		t.Error("the preview kept running a deleted machine")
	}
	// The clips are untouched: they belong to the bundle, not the machine.
	if got := m.AnimationIDs(); !slices.Equal(got, []string{"a-anim"}) {
		t.Errorf("AnimationIDs() = %v; deleting a machine took the clips", got)
	}
}
