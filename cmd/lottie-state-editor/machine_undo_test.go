package main

import (
	"image"
	"testing"
)

func TestMachineUndoRestoresEditsButNotNodeDrags(t *testing.T) {
	m := openSample(t, "character", "character.lottie")
	if m.CanUndoMachineEdit() {
		t.Fatalf("history not empty after open")
	}
	walk, _ := m.Machine().State("walk-state")
	wasSpeed := walk.Speed
	walk.Speed = 4
	m.Touch()
	m.SelectState("walk-state")
	m.AddTransition("run-state")
	n := len(m.Machine().States)
	m.AddState()
	if len(m.Machine().States) != n+1 {
		t.Fatalf("AddState did not add")
	}
	// A node drag in between must not become a step of its own.
	m.SetNodePos(0, image.Pt(300, 300))

	m.UndoMachineEdit()
	if len(m.Machine().States) != n {
		t.Fatalf("undo did not remove the added state")
	}
	if p := m.NodePos(0); p != image.Pt(300, 300) {
		t.Fatalf("undo moved the node back: %v", p)
	}
	transitions := len(walk.Transitions)
	m.UndoMachineEdit()
	walk, _ = m.Machine().State("walk-state")
	if len(walk.Transitions) != transitions-1 {
		t.Fatalf("undo did not remove the added transition: %d, had %d", len(walk.Transitions), transitions)
	}
	m.UndoMachineEdit()
	walk, _ = m.Machine().State("walk-state")
	if walk.Speed != wasSpeed {
		t.Fatalf("speed after three undos = %v, want %v", walk.Speed, wasSpeed)
	}
	if m.CanUndoMachineEdit() {
		t.Fatalf("history should be exhausted")
	}
	// The added state was selected when it vanished, so the selection fell
	// back to the initial state rather than dangling.
	if m.SelectedStateName() != m.Machine().Initial {
		t.Fatalf("selection = %q, want the initial state", m.SelectedStateName())
	}
	if m.SelectedState() == nil {
		t.Fatalf("selection dangles after undo")
	}
	// Undo is a document edit: the bundle has the restored machine.
	sm, err := m.Bundle().StateMachine(m.MachineID())
	if err != nil {
		t.Fatal(err)
	}
	if st, _ := sm.State("walk-state"); st.Speed != wasSpeed {
		t.Fatalf("bundle not synced after undo")
	}
}

func TestMachineUndoFollowsRenameAndSwitch(t *testing.T) {
	m := openSample(t, "character", "character.lottie")
	m.SelectState("walk-state")
	m.AddTransition("run-state")
	m.RenameMachine(m.MachineID(), "hero")
	if !m.CanUndoMachineEdit() {
		t.Fatalf("history lost across a rename")
	}
	m.NewMachine()
	if m.CanUndoMachineEdit() {
		t.Fatalf("a fresh machine inherited another machine's history")
	}
	m.SelectMachine("hero")
	if m.CanUndoMachineEdit() {
		t.Fatalf("switching machines should start a fresh history")
	}
}
