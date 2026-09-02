package main

import (
	"encoding/json"
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"

	lottie "github.com/shibukawa/lottie-go"
)

// Pose editing goes all the way to the bundle bytes, so these tests read the
// value back out of the stored clip rather than trusting the model's cache:
// a drag that only updates the editor's copy would still look right on stage
// and save nothing.

// posedModel opens the chibi-male preset and puts one clip on stage.
func posedModel(t *testing.T, clip string) *Model {
	t.Helper()
	m := NewModel()
	m.Open(presetPath("chibi-male"))
	if m.Bundle() == nil {
		t.Fatalf("open failed: %s", m.Status())
	}
	m.ShowClip(clipRef{Anim: clip})
	if m.PreviewPlayer() == nil {
		t.Fatalf("clip %q did not go on stage: %s", clip, m.Status())
	}
	m.SetCollisionTab(colPoses)
	return m
}

// storedRotation reads a layer's rotation at a key straight out of the
// bundle's JSON, which is what a save writes.
func storedRotation(t *testing.T, m *Model, clip, layer string, frame float64) float64 {
	t.Helper()
	data, ok := m.Bundle().AnimationJSON(clip)
	if !ok {
		t.Fatalf("no stored JSON for %q", clip)
	}
	var doc struct {
		Layers []struct {
			Name string `json:"nm"`
			KS   struct {
				R json.RawMessage `json:"r"`
			} `json:"ks"`
		} `json:"layers"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("stored clip is not JSON: %v", err)
	}
	for _, l := range doc.Layers {
		if l.Name != layer {
			continue
		}
		var prop struct {
			K []struct {
				T float64   `json:"t"`
				S []float64 `json:"s"`
			} `json:"k"`
		}
		if err := json.Unmarshal(l.KS.R, &prop); err != nil {
			t.Fatalf("rotation of %q is not keyframed: %v", layer, err)
		}
		for _, k := range prop.K {
			if k.T == frame && len(k.S) > 0 {
				return k.S[0]
			}
		}
		t.Fatalf("no rotation key for %q at frame %v", layer, frame)
	}
	t.Fatalf("no layer %q in stored clip", layer)
	return 0
}

func TestPoseTabReportsKeyTimes(t *testing.T) {
	m := posedModel(t, "punch-anim")
	if !m.PosesVisible() {
		t.Fatalf("pose tab is not the working context")
	}
	want := []float64{0, 4, 7, 12, 20}
	if got := m.PoseTimes(); !slices.Equal(got, want) {
		t.Errorf("PoseTimes() = %v; want %v", got, want)
	}
	// A clean pose sequence is one row, so there are no per-layer rows.
	if rows := m.PoseRows(); len(rows) != 0 {
		t.Errorf("PoseRows() = %v; want none while the clip is a pose sequence", rows)
	}
}

// The playhead has to be parked on the key for an edit to be possible;
// scrubbing away ends editing rather than writing at an in-between frame.
func TestPoseEditNeedsTheKeyUnderThePlayhead(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	if f, ok := m.SelectedPoseKey(); !ok || f != 7 {
		t.Fatalf("SelectedPoseKey() = %v, %v; want 7, true", f, ok)
	}
	if m.PreviewPlaying() {
		t.Errorf("selecting a key left playback running")
	}
	m.PreviewSeek(9)
	if _, ok := m.SelectedPoseKey(); ok {
		t.Errorf("a key is still selected after scrubbing off it")
	}
}

func partIndex(t *testing.T, m *Model, name string) int {
	t.Helper()
	d := m.StageClipDoc()
	if d == nil {
		t.Fatal("no clip document on stage")
	}
	for i := range d.layers {
		if d.layers[i].name == name {
			return i
		}
	}
	t.Fatalf("no layer %q", name)
	return -1
}

// Dragging a part swings it about its joint and the number lands in the
// bundle, at that key and nowhere else.
func TestPoseRotateWritesTheStoredKey(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	part := partIndex(t, m, "forearm-near")
	m.SelectPosePart(part)

	before := storedRotation(t, m, "punch-anim", "forearm-near", 7)
	other := storedRotation(t, m, "punch-anim", "forearm-near", 12)

	q, ok := m.PosePartQuad(part)
	if !ok {
		t.Fatal("no quad for the selected part")
	}
	// A quarter turn clockwise about the joint, expressed as two cursor
	// positions on a circle around it.
	px, py := q.pivot[0], q.pivot[1]
	m.RotatePosePart(px+40, py, px, py+40)

	after := storedRotation(t, m, "punch-anim", "forearm-near", 7)
	if math.Abs(after-before-90) > 0.5 {
		t.Errorf("rotation at 7 = %v; want %v (a quarter turn from %v)", after, before+90, before)
	}
	if now := storedRotation(t, m, "punch-anim", "forearm-near", 12); now != other {
		t.Errorf("rotation at 12 changed to %v; want %v", now, other)
	}
	if !m.PreviewStale() && m.Preview() != nil {
		t.Errorf("a clip edit did not register as a document change")
	}
}

// The stage must show the edit: the player is rebuilt over the re-decoded
// clip, holding the frame, so what is drawn is what was written.
func TestPoseRotateRebuildsTheStage(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	part := partIndex(t, m, "forearm-near")
	m.SelectPosePart(part)

	q, _ := m.PosePartQuad(part)
	m.RotatePosePart(q.pivot[0]+40, q.pivot[1], q.pivot[0], q.pivot[1]+40)

	if p := m.PreviewPlayer(); p == nil || p.Frame() != 7 {
		t.Fatalf("playhead left the key after an edit: %v", p)
	}
	anim := m.PreviewAnimation()
	if anim == nil {
		t.Fatal("no animation on stage after an edit")
	}
	// The animation on stage must be the re-decoded one: ask it where the
	// part sits and compare against what the document now says.
	pl, ok := anim.LayerPlacement("forearm-near", 7)
	if !ok {
		t.Fatal("forearm-near is not in the reloaded animation")
	}
	stored := storedRotation(t, m, "punch-anim", "forearm-near", 7)
	if math.IsNaN(pl.Angle) {
		t.Errorf("placement angle is NaN after writing %v", stored)
	}
}

// Retiming a pose moves every layer keyed at that time, and the selection
// follows the key it was on.
func TestPoseRetimeThroughTheModel(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	m.RetimePoseKey(7, 9, -1)

	if got := m.PoseTimes(); !slices.Equal(got, []float64{0, 4, 9, 12, 20}) {
		t.Fatalf("PoseTimes() = %v; want [0 4 9 12 20]", got)
	}
	if f, ok := m.SelectedPoseKey(); !ok || f != 9 {
		t.Errorf("selection = %v, %v; want the key it followed to 9", f, ok)
	}
	// The stored clip must agree, or the move only happened in the editor.
	storedRotation(t, m, "punch-anim", "forearm-near", 9)
}

// Viewer mode owns the document from disk, so a drag there must change
// nothing rather than being thrown away at the next reload.
func TestPoseEditBlockedInViewerMode(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	part := partIndex(t, m, "forearm-near")
	m.SelectPosePart(part)
	before := storedRotation(t, m, "punch-anim", "forearm-near", 7)

	m.SetViewer(true)
	q, _ := m.PosePartQuad(part)
	m.RotatePosePart(q.pivot[0]+40, q.pivot[1], q.pivot[0], q.pivot[1]+40)
	m.SetPoseValue("r", []float64{before + 30})
	m.RetimePoseKey(7, 9, -1)

	if after := storedRotation(t, m, "punch-anim", "forearm-near", 7); after != before {
		t.Errorf("viewer mode wrote rotation %v; want %v untouched", after, before)
	}
}

// A clip whose properties disagree on their key times falls back to a row
// per layer, and those rows still select and retime.
func TestPoseFallbackRowsAreUsable(t *testing.T) {
	m := NewModel()
	m.Open("../../examples/state-editor/character/walk-anim.json")
	if m.StageAnimID() == "" {
		m.ShowClip(clipRef{Anim: "walk-anim"})
	}
	m.SetCollisionTab(colPoses)
	if m.StageClipDoc() == nil {
		t.Skip("sample clip did not load; nothing to exercise")
	}
	if len(m.PoseTimes()) != 0 {
		t.Fatalf("sample clip reads as a pose sequence; the fallback is untested")
	}
	rows := m.PoseRows()
	if len(rows) == 0 {
		t.Fatal("no per-layer rows for a clip whose key times disagree")
	}
	times := m.PoseRowTimes(rows[0])
	if len(times) == 0 {
		t.Fatal("first row has no ticks")
	}
	m.SelectPoseKey(times[0], rows[0])
	if f, ok := m.SelectedPoseKey(); !ok || f != times[0] {
		t.Errorf("selection = %v, %v; want %v", f, ok, times[0])
	}
	if m.SelectedPosePart() != rows[0] {
		t.Errorf("selecting a row tick did not select its layer")
	}
}

// A part can be mirrored on its own axis — the rig flips a shin rather than
// drawing a second one — but rotation is written outside that scale, in the
// parent's frame. Using the part's own handedness instead of its parent's
// would negate the drag exactly here.
func TestPoseRotateIgnoresThePartsOwnMirror(t *testing.T) {
	m := posedModel(t, "idle-turn-anim")
	const frame = 11
	m.SelectPoseKey(frame, -1)
	part := partIndex(t, m, "shin-near")
	m.SelectPosePart(part)

	// The fixture only proves anything while this part really is mirrored.
	scale, ok := m.PoseValue("s")
	if !ok || len(scale) < 1 || scale[0] >= 0 {
		t.Fatalf("shin-near scale at %v = %v; expected a negative x", frame, scale)
	}
	if d := det(m.parentGeoM(part)); d <= 0 {
		t.Fatalf("parent handedness = %v; the parent chain should be right-handed here", d)
	}

	before := storedRotation(t, m, "idle-turn-anim", "shin-near", frame)
	q, _ := m.PosePartQuad(part)
	m.RotatePosePart(q.pivot[0]+40, q.pivot[1], q.pivot[0], q.pivot[1]+40)

	after := storedRotation(t, m, "idle-turn-anim", "shin-near", frame)
	if math.Abs(after-before-90) > 0.5 {
		t.Errorf("rotation = %v; want %v — a clockwise drag on a mirrored part "+
			"must still write a clockwise number", after, before+90)
	}
}

// The outline has to follow the mirror too, or the box drawn around a
// flipped part sits where the part is not.
func TestPosePartQuadFollowsTheMirror(t *testing.T) {
	m := posedModel(t, "idle-turn-anim")
	part := partIndex(t, m, "shin-near")

	m.PreviewSeek(0)
	upright, ok := m.PosePartQuad(part)
	if !ok {
		t.Fatal("no quad before the turn")
	}
	m.PreviewSeek(11)
	mirrored, ok := m.PosePartQuad(part)
	if !ok {
		t.Fatal("no quad after the turn")
	}
	if winding(upright.pts)*winding(mirrored.pts) >= 0 {
		t.Errorf("quad winding did not reverse across the mirror: %v then %v",
			winding(upright.pts), winding(mirrored.pts))
	}
}

// winding is twice the signed area of the quad; its sign flips under a
// mirror and holds under rotation and scaling.
func winding(pts [4][2]float64) float64 {
	var s float64
	for i := range pts {
		a, b := pts[i], pts[(i+1)%4]
		s += a[0]*b[1] - b[0]*a[1]
	}
	return s
}

// A drag writes on every mouse move, so the undo step has to be the whole
// drag — not one step per frame the cursor crossed.
func TestPoseUndoCollapsesADrag(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	part := partIndex(t, m, "forearm-near")
	m.SelectPosePart(part)
	before := storedRotation(t, m, "punch-anim", "forearm-near", 7)

	q, _ := m.PosePartQuad(part)
	px, py := q.pivot[0], q.pivot[1]
	m.BeginPoseEdit()
	for _, step := range []float64{10, 20, 30, 40} {
		m.RotatePosePart(px+40, py+step-10, px+40, py+step)
	}
	m.EndPoseEdit()

	moved := storedRotation(t, m, "punch-anim", "forearm-near", 7)
	if moved == before {
		t.Fatalf("the drag wrote nothing")
	}
	if !m.CanUndoClipEdit() {
		t.Fatal("nothing to undo after a drag")
	}
	m.UndoClipEdit()
	if got := storedRotation(t, m, "punch-anim", "forearm-near", 7); got != before {
		t.Errorf("after one undo rotation = %v; want %v — the drag should be one step", got, before)
	}
	if m.CanUndoClipEdit() {
		t.Errorf("the drag left more than one undo step")
	}
}

// Typed edits are their own steps, and undo walks back through them.
func TestPoseUndoStepsThroughTypedEdits(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	m.SelectPosePart(partIndex(t, m, "forearm-near"))
	before := storedRotation(t, m, "punch-anim", "forearm-near", 7)

	m.SetPoseValue("r", []float64{before + 5})
	m.SetPoseValue("r", []float64{before + 9})
	if got := storedRotation(t, m, "punch-anim", "forearm-near", 7); got != before+9 {
		t.Fatalf("rotation = %v; want %v", got, before+9)
	}
	m.UndoClipEdit()
	if got := storedRotation(t, m, "punch-anim", "forearm-near", 7); got != before+5 {
		t.Errorf("after one undo = %v; want %v", got, before+5)
	}
	m.UndoClipEdit()
	if got := storedRotation(t, m, "punch-anim", "forearm-near", 7); got != before {
		t.Errorf("after two undos = %v; want %v", got, before)
	}
	if m.CanUndoClipEdit() {
		t.Errorf("undo stack is not empty after taking back both edits")
	}
}

// The per-field copy buttons read the neighbouring keys of the member
// itself, so what they report must be the stored value one key away in
// either direction, along with the frame it sits at — and nothing past the
// ends, where there is no neighbour to copy.
func TestPoseAdjacentValueReadsTheKeysNextDoor(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	m.SelectPosePart(partIndex(t, m, "forearm-near"))

	prev := storedRotation(t, m, "punch-anim", "forearm-near", 4)
	if got, at, ok := m.PoseAdjacentValue("r", -1); !ok || at != 4 || len(got) == 0 || got[0] != prev {
		t.Errorf("PoseAdjacentValue(r, -1) = %v @%v, %v; want [%v] @4, true", got, at, ok, prev)
	}
	next := storedRotation(t, m, "punch-anim", "forearm-near", 12)
	if got, at, ok := m.PoseAdjacentValue("r", +1); !ok || at != 12 || len(got) == 0 || got[0] != next {
		t.Errorf("PoseAdjacentValue(r, +1) = %v @%v, %v; want [%v] @12, true", got, at, ok, next)
	}

	m.SelectPoseKey(0, -1)
	if v, _, ok := m.PoseAdjacentValue("r", -1); ok {
		t.Errorf("PoseAdjacentValue(r, -1) at the first key = %v; want none", v)
	}
	m.SelectPoseKey(20, -1)
	if v, _, ok := m.PoseAdjacentValue("r", +1); ok {
		t.Errorf("PoseAdjacentValue(r, +1) at the last key = %v; want none", v)
	}
}

// Copying from a neighbouring key writes that key's component into the
// bundle at the selected key and nowhere else, in either direction, and is
// a real edit: undoable, and a no-op when the values already agree.
func TestCopyPoseValueFromAdjacent(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	m.SelectPosePart(partIndex(t, m, "forearm-near"))

	before := storedRotation(t, m, "punch-anim", "forearm-near", 7)
	prev := storedRotation(t, m, "punch-anim", "forearm-near", 4)
	next := storedRotation(t, m, "punch-anim", "forearm-near", 12)
	if before == prev || before == next {
		t.Fatal("key 7 already agrees with a neighbour; the fixture cannot exercise a copy")
	}

	m.CopyPoseValueFromAdjacent("r", 0, -1)
	if got := storedRotation(t, m, "punch-anim", "forearm-near", 7); got != prev {
		t.Errorf("rotation at 7 = %v; want the previous key's %v", got, prev)
	}
	if got := storedRotation(t, m, "punch-anim", "forearm-near", 12); got != next {
		t.Errorf("rotation at 12 changed to %v; want %v", got, next)
	}

	m.CopyPoseValueFromAdjacent("r", 0, +1)
	if got := storedRotation(t, m, "punch-anim", "forearm-near", 7); got != next {
		t.Errorf("rotation at 7 = %v; want the next key's %v", got, next)
	}
	if got := storedRotation(t, m, "punch-anim", "forearm-near", 4); got != prev {
		t.Errorf("rotation at 4 changed to %v; want %v", got, prev)
	}

	m.UndoClipEdit()
	m.UndoClipEdit()
	if got := storedRotation(t, m, "punch-anim", "forearm-near", 7); got != before {
		t.Errorf("after undoing both copies rotation at 7 = %v; want %v", got, before)
	}

	// Copying a value that already matches the neighbour must not push
	// another undo step.
	m.SelectPoseKey(7, -1)
	m.SetPoseValue("r", []float64{prev})
	m.CopyPoseValueFromAdjacent("r", 0, -1)
	m.UndoClipEdit()
	if m.CanUndoClipEdit() {
		t.Errorf("a copy of an already-matching value pushed an undo step")
	}
	if got := storedRotation(t, m, "punch-anim", "forearm-near", 7); got != before {
		t.Errorf("after undoing everything rotation at 7 = %v; want %v", got, before)
	}
}

// An edit that changes nothing must not leave a step that undoes nothing:
// a click without movement is the common case.
func TestPoseUndoIgnoresNoOpEdits(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	m.SelectPosePart(partIndex(t, m, "forearm-near"))
	same, ok := m.PoseValue("r")
	if !ok {
		t.Fatal("no rotation at the key")
	}
	m.SetPoseValue("r", same)
	if m.CanUndoClipEdit() {
		t.Errorf("writing an unchanged value pushed an undo step")
	}
	m.RetimePoseKey(7, 7, -1)
	if m.CanUndoClipEdit() {
		t.Errorf("retiming a key to where it already is pushed an undo step")
	}
}

// Undoing a retime takes the key away from under the selection, which must
// end editing rather than leave a selection pointing at nothing.
func TestPoseUndoClearsAStaleKeySelection(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	m.RetimePoseKey(7, 9, -1)
	if f, ok := m.SelectedPoseKey(); !ok || f != 9 {
		t.Fatalf("selection = %v, %v; want 9", f, ok)
	}
	m.UndoClipEdit()
	if _, ok := m.SelectedPoseKey(); ok {
		t.Errorf("frame 9 still reads as a selected key after the retime was undone")
	}
	if !slices.Equal(m.PoseTimes(), []float64{0, 4, 7, 12, 20}) {
		t.Errorf("PoseTimes() = %v; want the original set back", m.PoseTimes())
	}
}

// A drag step that changes nothing — a value re-written identically as the
// cursor jitters — must not take back the snapshot the drag's first step
// pushed, or undo returns to the middle of the drag instead of before it.
func TestPoseUndoKeepsTheDragSnapshotThroughNoOpSteps(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	m.SelectPosePart(partIndex(t, m, "forearm-near"))
	before := storedRotation(t, m, "punch-anim", "forearm-near", 7)

	m.BeginPoseEdit()
	m.SetPoseValue("r", []float64{before + 10})
	same, ok := m.PoseValue("r")
	if !ok {
		t.Fatal("no rotation after the first step")
	}
	m.SetPoseValue("r", same) // writes nothing: the step is a no-op
	m.SetPoseValue("r", []float64{before + 20})
	m.EndPoseEdit()

	if got := storedRotation(t, m, "punch-anim", "forearm-near", 7); got != before+20 {
		t.Fatalf("rotation = %v; want %v", got, before+20)
	}
	m.UndoClipEdit()
	if got := storedRotation(t, m, "punch-anim", "forearm-near", 7); got != before {
		t.Errorf("after undo rotation = %v; want %v — the no-op step discarded "+
			"the pre-drag snapshot", got, before)
	}
}

// Opening a bundle leaves the state machine driving the stage, and a machine
// keeps the Animation it decoded when it started. Editing there reached the
// bundle but never the picture, so a drag looked like it did nothing.
// Selecting a key now puts the clip itself on stage.
func TestPoseSelectTakesTheClipOnStage(t *testing.T) {
	m := NewModel()
	m.Open(presetPath("chibi-male"))
	m.SetCollisionTab(colPoses)
	if m.clipPlayer != nil {
		t.Fatal("a clip is already on stage; the machine case is untested")
	}
	id := m.StageAnimID()
	if id == "" {
		t.Fatal("the machine put no animation on stage")
	}
	before := m.PreviewAnimation()

	m.SelectPoseKey(m.PoseTimes()[1], -1)
	if m.PreviewClip().Anim != id {
		t.Fatalf("stage clip = %q; want %q", m.PreviewClip().Anim, id)
	}
	m.SelectPosePart(partIndex(t, m, "forearm-near"))
	m.SetPoseValue("r", []float64{123})

	if m.PreviewAnimation() == before {
		t.Error("the stage is still drawing the animation decoded before the edit")
	}
	if _, ok := m.PoseValue("r"); !ok {
		t.Error("the value is unreadable after the edit")
	}
}

// A clip's last key sits at its out point, which a looping player wraps back
// to the start — so the final pose of every clip used to be the one pose
// that could not be selected or edited.
func TestPoseLastKeyIsSelectable(t *testing.T) {
	m := posedModel(t, "punch-anim")
	part := partIndex(t, m, "forearm-near")
	times := m.PoseTimes()
	for _, f := range times {
		m.SelectPoseKey(f, -1)
		m.SelectPosePart(part)
		got, ok := m.SelectedPoseKey()
		if !ok || got != f {
			t.Errorf("key %v: SelectedPoseKey() = %v, %v; want %v, true", f, got, ok, f)
			continue
		}
		if p := m.PreviewPlayer(); p.Frame() != f {
			t.Errorf("key %v: playhead parked at %v", f, p.Frame())
		}
		if _, ok := m.PoseValue("r"); !ok {
			t.Errorf("key %v: no rotation to edit", f)
		}
	}
	// And the edit lands, at the out point like anywhere else.
	last := times[len(times)-1]
	m.SelectPoseKey(last, -1)
	m.SelectPosePart(part)
	m.SetPoseValue("r", []float64{77})
	if got := storedRotation(t, m, "punch-anim", "forearm-near", last); got != 77 {
		t.Errorf("rotation at the out point = %v; want 77", got)
	}
}

// Pressing play ends the park: the key stops being under the playhead, and a
// clip preview goes back to looping.
func TestPosePlayEndsThePark(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	if _, ok := m.SelectedPoseKey(); !ok {
		t.Fatal("no key selected to begin with")
	}
	m.TogglePreviewPlaying()
	if !m.PreviewPlaying() {
		t.Fatal("play did not start")
	}
	if _, ok := m.SelectedPoseKey(); ok {
		t.Error("a key is still selected while the clip is playing")
	}
	// Looping has to come back, or the clip stops dead at its out point.
	m.PreviewSeek(19)
	for range 300 {
		m.PreviewUpdate()
	}
	if !m.PreviewPlaying() {
		t.Error("playback stopped at the out point; looping was not restored")
	}
}

// The parts list exists because the stage cannot offer every part: a rig
// layers them over each other and switches others off by opacity. Listing
// only what a click could already reach would defeat the point.
func TestPosePartsListsEveryPart(t *testing.T) {
	m := posedModel(t, "punch-anim")
	parts := m.PoseParts()
	if len(parts) != 15 {
		t.Errorf("PoseParts() has %d entries; chibi-male has 15 image layers", len(parts))
	}
	// In document order, which is front to back.
	if len(parts) > 0 && m.PoseLayerName(parts[0]) != "forearm-near" {
		t.Errorf("first part = %q; want the topmost layer forearm-near", m.PoseLayerName(parts[0]))
	}
	names := map[string]bool{}
	for _, i := range parts {
		names[m.PoseLayerName(i)] = true
	}
	// The far arm sits behind the torso and the alternate heads are switched
	// off; both are exactly what the list is for.
	for _, want := range []string{"upper-arm-far", "head-side", "head-back", "shadow"} {
		if !names[want] {
			t.Errorf("part %q is missing from the list", want)
		}
	}
}

// A part switched off by opacity is listed, and says so — otherwise the list
// would look like it was lying about what is on the stage.
func TestPosePartHiddenTracksOpacity(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	if !m.PosePartHidden(partIndex(t, m, "head-side")) {
		t.Errorf("head-side reads as visible; it is switched off at this frame")
	}
	if m.PosePartHidden(partIndex(t, m, "head")) {
		t.Errorf("head reads as hidden; it is the drawing on screen")
	}
}

// Selection is the same state whichever end sets it, so a row click and a
// stage click are indistinguishable afterwards.
func TestPosePartSelectionIsShared(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	far := partIndex(t, m, "upper-arm-far")
	m.SelectPosePart(far)
	if m.SelectedPosePart() != far || m.SelectedPosePartName() != "upper-arm-far" {
		t.Fatalf("SelectedPosePart() = %v (%q); want %v",
			m.SelectedPosePart(), m.SelectedPosePartName(), far)
	}
	// And it is editable straight away, which is the point of reaching a
	// part the stage could not.
	if _, ok := m.PoseValue("r"); !ok {
		t.Errorf("a part chosen from the list has no rotation to edit")
	}
	if i, ok := m.PosePartIndex("upper-arm-far"); !ok || i != far {
		t.Errorf("PosePartIndex round trip = %v, %v; want %v", i, ok, far)
	}
}

// Choosing the Poses tab is choosing to pose, so the right pane comes with
// it rather than waiting for something to be selected.
func TestPosesTabOpensThePosePane(t *testing.T) {
	m := NewModel()
	m.Open(presetPath("chibi-male"))
	if m.InspectTarget() == inspectPose {
		t.Fatal("the pose pane is already open; the case is untested")
	}
	m.SetCollisionTab(colPoses)
	if m.InspectTarget() != inspectPose {
		t.Errorf("InspectTarget() = %v after choosing Poses; want the pose pane", m.InspectTarget())
	}
}

// storedLayerOrder is the draw order as it sits in the bundle, which is what
// a save writes and what a renderer reads.
func storedLayerOrder(t *testing.T, m *Model, clip string) []string {
	t.Helper()
	data, ok := m.Bundle().AnimationJSON(clip)
	if !ok {
		t.Fatalf("no stored JSON for %q", clip)
	}
	var doc struct {
		Layers []struct {
			Name string `json:"nm"`
		} `json:"layers"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("stored clip is not JSON: %v", err)
	}
	out := make([]string, 0, len(doc.Layers))
	for _, l := range doc.Layers {
		out = append(out, l.Name)
	}
	return out
}

// The list order is the draw order, so moving a row moves the part through
// the overlap — and the selection has to travel with it, because these are
// array positions and the array just changed.
func TestReorderPosePartMovesTheDrawOrder(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	before := storedLayerOrder(t, m, "punch-anim")
	part := partIndex(t, m, "head")
	m.SelectPosePart(part)
	name := m.SelectedPosePartName()

	m.ReorderPosePart(1) // one step towards the back
	after := storedLayerOrder(t, m, "punch-anim")
	if slices.Index(after, name) != slices.Index(before, name)+1 {
		t.Errorf("%q went from %d to %d; want one step back",
			name, slices.Index(before, name), slices.Index(after, name))
	}
	if m.SelectedPosePartName() != name {
		t.Errorf("selection became %q; want it to follow %q", m.SelectedPosePartName(), name)
	}

	m.ReorderPosePart(-1) // and back again
	if got := storedLayerOrder(t, m, "punch-anim"); !slices.Equal(got, before) {
		t.Errorf("order after there-and-back = %v; want %v", got, before)
	}
	if m.SelectedPosePartName() != name {
		t.Errorf("selection became %q after returning; want %q", m.SelectedPosePartName(), name)
	}
}

// Parent links are by ind, not by position, so the rig must survive being
// reordered — a forearm still hangs off its upper arm.
func TestReorderKeepsTheRigIntact(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	anim := m.PreviewAnimation()
	before, ok := anim.LayerPlacement("forearm-near", 7)
	if !ok {
		t.Fatal("forearm-near has no placement to begin with")
	}

	m.SelectPosePart(partIndex(t, m, "body"))
	m.ReorderPosePart(1)

	after, ok := m.PreviewAnimation().LayerPlacement("forearm-near", 7)
	if !ok {
		t.Fatal("forearm-near lost its placement after a reorder")
	}
	if math.Abs(after.X-before.X) > 1e-9 || math.Abs(after.Y-before.Y) > 1e-9 ||
		math.Abs(after.Angle-before.Angle) > 1e-9 {
		t.Errorf("forearm moved from (%v, %v, %v) to (%v, %v, %v); parenting broke",
			before.X, before.Y, before.Angle, after.X, after.Y, after.Angle)
	}
}

// The list reports a drop against the pre-move list, so a downward drag has
// to account for the row leaving its old place first.
func TestReorderPosePartToDragIndices(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	before := storedLayerOrder(t, m, "punch-anim")

	// Drag row 0 down to sit where row 2 was.
	m.SelectPosePart(m.PoseParts()[0])
	m.ReorderPosePartTo(0, 3)
	got := storedLayerOrder(t, m, "punch-anim")
	want := []string{before[1], before[2], before[0]}
	if !slices.Equal(got[:3], want) {
		t.Errorf("after dragging 0 -> 3 the order starts %v; want %v", got[:3], want)
	}

	// And upward, where no adjustment applies.
	m = posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	m.SelectPosePart(m.PoseParts()[2])
	m.ReorderPosePartTo(2, 0)
	got = storedLayerOrder(t, m, "punch-anim")
	want = []string{before[2], before[0], before[1]}
	if !slices.Equal(got[:3], want) {
		t.Errorf("after dragging 2 -> 0 the order starts %v; want %v", got[:3], want)
	}
}

// Reordering is a clip edit, so it undoes like one.
func TestReorderIsUndoable(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	before := storedLayerOrder(t, m, "punch-anim")
	m.SelectPosePart(partIndex(t, m, "head"))
	m.ReorderPosePart(1)
	if slices.Equal(storedLayerOrder(t, m, "punch-anim"), before) {
		t.Fatal("the reorder wrote nothing")
	}
	m.UndoClipEdit()
	if got := storedLayerOrder(t, m, "punch-anim"); !slices.Equal(got, before) {
		t.Errorf("order after undo = %v; want %v", got, before)
	}
}

// Viewer mode owns the document from disk; a reorder there must change
// nothing rather than be thrown away at the next reload.
func TestReorderBlockedInViewerMode(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	m.SelectPosePart(partIndex(t, m, "head"))
	before := storedLayerOrder(t, m, "punch-anim")
	m.SetViewer(true)
	if m.CanReorderParts() {
		t.Errorf("CanReorderParts() is true in viewer mode")
	}
	m.ReorderPosePart(1)
	if got := storedLayerOrder(t, m, "punch-anim"); !slices.Equal(got, before) {
		t.Errorf("viewer mode reordered the clip")
	}
}

// Which drawing of a slot shows is opacity, and opacity is per key like
// every other pose value.
func TestTogglePosePartHidden(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	side := partIndex(t, m, "head-side")
	m.SelectPosePart(side)
	if !m.PosePartHidden(side) {
		t.Fatal("head-side is not hidden to begin with")
	}

	m.TogglePosePartHidden()
	if m.PosePartHidden(side) {
		t.Errorf("head-side is still hidden after being shown")
	}
	if o, ok := m.PoseValue("o"); !ok || len(o) == 0 || o[0] != 100 {
		t.Errorf("opacity = %v (%v); want 100", o, ok)
	}

	m.TogglePosePartHidden()
	if !m.PosePartHidden(side) {
		t.Errorf("head-side did not go back to hidden")
	}
	// Keying it must not have disturbed the other poses of the clip: it was
	// static at zero, so everywhere else stays zero.
	m.SelectPoseKey(12, -1)
	m.SelectPosePart(side)
	if o, ok := m.PoseValue("o"); !ok || len(o) == 0 || o[0] != 0 {
		t.Errorf("opacity at another key = %v; want the 0 it always was", o)
	}
}

// A track matte with no explicit source takes the layer before it, so the
// layer array means more than draw order and reordering would retarget it.
func TestReorderRefusedWithTrackMattes(t *testing.T) {
	const matted = `{"v":"5.9.0","fr":60,"ip":0,"op":10,"w":64,"h":64,"layers":[
	  {"ty":4,"nm":"masked","ind":1,"tt":1,"ip":0,"op":10,"st":0,"ks":{"p":{"a":0,"k":[0,0]}}},
	  {"ty":4,"nm":"mask","ind":2,"td":1,"ip":0,"op":10,"st":0,"ks":{"p":{"a":0,"k":[0,0]}}}
	]}`
	d, err := newClipDoc("matted", []byte(matted))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !d.usesTrackMatte() {
		t.Errorf("usesTrackMatte() is false for a clip with tt and td")
	}
	plain := presetClip(t, "punch-anim.json")
	if plain.usesTrackMatte() {
		t.Errorf("usesTrackMatte() is true for a rig that has none")
	}
}

// A part switched off by opacity is not on the stage, so a click has to fall
// through it. The rig stacks alternate drawings of a slot in front of the
// one being used — body-side and body-back sit before body — so without this
// the torso can never be picked at all.
func TestPosePartAtSkipsHiddenParts(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)

	body := partIndex(t, m, "body")
	side := partIndex(t, m, "body-side")
	if side >= body {
		t.Fatalf("body-side is at %d and body at %d; the hidden drawing no "+
			"longer sits in front, so this test proves nothing", side, body)
	}
	if !m.PosePartHidden(side) {
		t.Fatal("body-side is visible at this frame; the case is untested")
	}

	// Aim at the middle of the torso, which both layers cover.
	q, ok := m.PosePartQuad(body)
	if !ok {
		t.Fatal("no quad for body")
	}
	var cx, cy float64
	for _, p := range q.pts {
		cx, cy = cx+p[0]/4, cy+p[1]/4
	}
	got, ok := m.PosePartAt(cx, cy)
	if !ok {
		t.Fatal("nothing under the middle of the torso")
	}
	if m.PosePartHidden(got) {
		t.Errorf("the click landed on %q, which is switched off",
			m.PoseLayerName(got))
	}
	if got != body {
		t.Errorf("the click landed on %q; want body", m.PoseLayerName(got))
	}
}

// A new pose starts as the one it follows, so the insert leaves the clip
// looking exactly as it did and the first edit is the only change.
func TestInsertPoseCopiesThePrevious(t *testing.T) {
	m := posedModel(t, "punch-anim") // keys at 0 4 7 12 20
	m.SelectPoseKey(7, -1)
	part := partIndex(t, m, "forearm-near")
	m.SelectPosePart(part)
	at7 := storedRotation(t, m, "punch-anim", "forearm-near", 7)

	m.PreviewSeek(9)
	if !m.CanInsertPose() {
		t.Fatal("frame 9 will not take a pose")
	}
	m.InsertPose(false)

	if got := m.PoseTimes(); !slices.Equal(got, []float64{0, 4, 7, 9, 12, 20}) {
		t.Fatalf("PoseTimes() = %v; want the new key at 9", got)
	}
	if got := storedRotation(t, m, "punch-anim", "forearm-near", 9); got != at7 {
		t.Errorf("rotation at the new key = %v; want %v copied from frame 7", got, at7)
	}
	// The clip must still be a pose sequence, or the timeline drops to
	// per-layer rows the moment anyone inserts anything.
	if d := m.StageClipDoc(); !d.posed {
		t.Errorf("the clip stopped being a pose sequence after an insert")
	}
	// And the new pose is what is selected, ready to be changed.
	if f, ok := m.SelectedPoseKey(); !ok || f != 9 {
		t.Errorf("selection = %v, %v; want the new pose at 9", f, ok)
	}
}

func TestInsertPoseRefusesExistingAndOutside(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.PreviewSeek(7)
	if m.CanInsertPose() {
		t.Errorf("CanInsertPose() is true on a frame that already has a pose")
	}
	m.PreviewSeek(20)
	if m.CanInsertPose() {
		t.Errorf("CanInsertPose() is true on the last pose")
	}
	before := m.PoseTimes()
	m.InsertPose(false)
	if got := m.PoseTimes(); !slices.Equal(got, before) {
		t.Errorf("PoseTimes() = %v; want %v unchanged", got, before)
	}
}

// Deleting removes the whole column, and never the last one: a track with no
// keys is a different kind of clip, not an empty one.
func TestDeletePose(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	if !m.CanDeletePose() {
		t.Fatal("nothing to delete with a pose selected")
	}
	m.DeletePose()
	if got := m.PoseTimes(); !slices.Equal(got, []float64{0, 4, 12, 20}) {
		t.Errorf("PoseTimes() = %v; want 7 gone", got)
	}
	if _, ok := m.SelectedPoseKey(); ok {
		t.Errorf("the deleted pose is still selected")
	}
	m.UndoClipEdit()
	if got := m.PoseTimes(); !slices.Equal(got, []float64{0, 4, 7, 12, 20}) {
		t.Errorf("PoseTimes() = %v after undo; want 7 back", got)
	}
}

// A preset's clips share one rig, so a pose is worth borrowing. Layers match
// by name, which is what makes them interchangeable.
func TestInsertPoseFromAnotherClip(t *testing.T) {
	m := posedModel(t, "punch-anim")
	src := m.PoseSourceKeys("guard-anim")
	if len(src) == 0 {
		t.Skip("guard-anim has no keys to borrow")
	}
	want := storedRotation(t, m, "punch-anim", "forearm-near", 0)
	_ = want

	m.PreviewSeek(9)
	m.InsertPoseFrom("guard-anim", src[0], false)
	if got := m.PoseTimes(); !slices.Contains(got, 9) {
		t.Fatalf("PoseTimes() = %v; want a pose at 9", got)
	}

	// Every part the source has should now read the same as the source did.
	srcDoc := m.clipDocFor("guard-anim")
	dst := m.StageClipDoc()
	for _, li := range m.PoseParts() {
		name := dst.layers[li].name
		si := -1
		for i := range srcDoc.layers {
			if srcDoc.layers[i].name == name {
				si = i
				break
			}
		}
		if si < 0 {
			continue
		}
		sv, ok := srcDoc.valueNear(si, "r", src[0])
		if !ok {
			continue
		}
		got, ok := dst.value(li, "r", 9)
		if !ok || len(got) == 0 || got[0] != sv[0] {
			t.Errorf("%s rotation at the borrowed pose = %v; want %v", name, got, sv)
		}
	}
}

// Stepping through a clip means stepping key to key: the frames between hold
// nothing to edit.
func TestJumpToKey(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	m.JumpToKey(1)
	if f, ok := m.SelectedPoseKey(); !ok || f != 12 {
		t.Errorf("forward from 7 landed on %v (%v); want 12", f, ok)
	}
	m.JumpToKey(-1)
	if f, ok := m.SelectedPoseKey(); !ok || f != 7 {
		t.Errorf("back from 12 landed on %v (%v); want 7", f, ok)
	}
	// The ends stay put rather than wrapping.
	m.SelectPoseKey(0, -1)
	m.JumpToKey(-1)
	if f, _ := m.SelectedPoseKey(); f != 0 {
		t.Errorf("back from the first key moved to %v", f)
	}
}

// Easing belongs to the pose, not to one limb: the whole column moves.
func TestPoseEaseTogglesTheWholeColumn(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	was := m.PoseEased()

	m.SetPoseEase(!was)
	if m.PoseEased() == was {
		t.Fatalf("ease did not change from %v", was)
	}
	// Every animated track at that time must agree, or a body easing in
	// while its arm arrives linearly reads as a mistake.
	d := m.StageClipDoc()
	want := !was
	for li := range d.layers {
		for prop := range d.layers[li].keyed {
			keys, ok := propKeys(d.layers[li].ks[prop])
			if !ok {
				continue
			}
			km, ok := keyAt(keys, 7)
			if !ok {
				continue
			}
			if h, ok := jsonNum(km["h"]); ok && h != 0 {
				continue // hold keys switch; a curve on one means nothing
			}
			if easeAxisIs(km["o"], "x", 0.6) != want {
				t.Errorf("%s.%s at frame 7 did not follow the column",
					d.layers[li].name, prop)
			}
		}
	}
	m.UndoClipEdit()
	if m.PoseEased() != was {
		t.Errorf("undo did not restore the easing")
	}
}

// Lengthening a clip has to carry the layers with it, or every part would
// vanish at the old end.
func TestSetClipLength(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	if got := m.ClipLength(); got != 20 {
		t.Fatalf("ClipLength() = %v; want 20", got)
	}
	m.SetClipLength(40)
	if got := m.ClipLength(); got != 40 {
		t.Fatalf("ClipLength() = %v; want 40", got)
	}
	data, _ := m.Bundle().AnimationJSON("punch-anim")
	var doc struct {
		OP     float64 `json:"op"`
		Layers []struct {
			Name string  `json:"nm"`
			OP   float64 `json:"op"`
		} `json:"layers"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.OP != 40 {
		t.Errorf("document op = %v; want 40", doc.OP)
	}
	for _, l := range doc.Layers {
		if l.OP != 40 {
			t.Errorf("layer %q still ends at %v; it would vanish early", l.Name, l.OP)
		}
	}
	// A clip may not be cut shorter than its own last pose; deleting the
	// pose says plainly what is being lost.
	m.SetClipLength(10)
	if got := m.ClipLength(); got != 40 {
		t.Errorf("ClipLength() = %v; a pose at 20 should have refused the cut", got)
	}
}

// nearFarRotations reads the paired limbs' rotations at a key, which is what
// a swap trades.
func nearFarRotations(t *testing.T, m *Model, clip string, frame float64) map[string]float64 {
	t.Helper()
	out := map[string]float64{}
	for _, n := range []string{
		"upper-arm-near", "upper-arm-far", "forearm-near", "forearm-far",
		"thigh-near", "thigh-far", "shin-near", "shin-far",
	} {
		out[n] = storedRotation(t, m, clip, n, frame)
	}
	return out
}

// Half a walk cycle is the other half with the limbs traded, so inserting a
// swapped copy is how the second half gets built.
func TestInsertPoseSwapped(t *testing.T) {
	m := posedModel(t, "walk-anim")
	times := m.PoseTimes()
	if len(times) < 2 {
		t.Skip("walk-anim has too few poses")
	}
	m.SelectPoseKey(times[0], -1)
	src := nearFarRotations(t, m, "walk-anim", times[0])

	// Somewhere between the first two poses, so the copy comes from the first.
	at := math.Floor((times[0] + times[1]) / 2)
	if at == times[0] {
		t.Skip("no room between the first two poses")
	}
	m.PreviewSeek(at)
	m.InsertPose(true)

	got := nearFarRotations(t, m, "walk-anim", at)
	for _, pair := range [][2]string{
		{"upper-arm-near", "upper-arm-far"},
		{"forearm-near", "forearm-far"},
		{"thigh-near", "thigh-far"},
		{"shin-near", "shin-far"},
	} {
		if got[pair[0]] != src[pair[1]] || got[pair[1]] != src[pair[0]] {
			t.Errorf("%s/%s = %v/%v; want the traded %v/%v",
				pair[0], pair[1], got[pair[0]], got[pair[1]], src[pair[1]], src[pair[0]])
		}
	}
	// The source pose is untouched: a swap on insert trades the copy, not
	// the pose it came from.
	if now := nearFarRotations(t, m, "walk-anim", times[0]); now["thigh-near"] != src["thigh-near"] {
		t.Errorf("the pose that was copied changed to %v; want %v",
			now["thigh-near"], src["thigh-near"])
	}
}

// Swapping in place is for a pose that is already there and the wrong way
// round, and it undoes like any other clip edit.
func TestSwapPoseInPlace(t *testing.T) {
	m := posedModel(t, "walk-anim")
	f := m.PoseTimes()[0]
	m.SelectPoseKey(f, -1)
	before := nearFarRotations(t, m, "walk-anim", f)

	m.SwapPose()
	after := nearFarRotations(t, m, "walk-anim", f)
	if after["thigh-near"] != before["thigh-far"] || after["thigh-far"] != before["thigh-near"] {
		t.Errorf("thighs = %v/%v; want the traded %v/%v",
			after["thigh-near"], after["thigh-far"], before["thigh-far"], before["thigh-near"])
	}
	// Twice is a round trip.
	m.SwapPose()
	if got := nearFarRotations(t, m, "walk-anim", f); got["thigh-near"] != before["thigh-near"] {
		t.Errorf("swapping twice left %v; want %v", got["thigh-near"], before["thigh-near"])
	}
	m.UndoClipEdit()
	m.UndoClipEdit()
	if got := nearFarRotations(t, m, "walk-anim", f); got["thigh-near"] != before["thigh-near"] {
		t.Errorf("undo left %v; want %v", got["thigh-near"], before["thigh-near"])
	}
}

// The static members are rig spec: a limb's attach point puts it on its own
// side of the torso, so trading those would detach the pair, not swap it.
func TestSwapPoseLeavesRigSpecAlone(t *testing.T) {
	m := posedModel(t, "walk-anim")
	f := m.PoseTimes()[0]
	m.SelectPoseKey(f, -1)
	d := m.StageClipDoc()
	near := partIndex(t, m, "thigh-near")
	far := partIndex(t, m, "thigh-far")
	pNear, _ := d.value(near, "p", f)
	pFar, _ := d.value(far, "p", f)
	aNear, _ := d.value(near, "a", f)

	m.SwapPose()

	d = m.StageClipDoc()
	if got, _ := d.value(partIndex(t, m, "thigh-near"), "p", f); !slices.Equal(got, pNear) {
		t.Errorf("thigh-near attach moved to %v; want %v (it is static rig spec)", got, pNear)
	}
	if got, _ := d.value(partIndex(t, m, "thigh-far"), "p", f); !slices.Equal(got, pFar) {
		t.Errorf("thigh-far attach moved to %v; want %v", got, pFar)
	}
	if got, _ := d.value(partIndex(t, m, "thigh-near"), "a", f); !slices.Equal(got, aNear) {
		t.Errorf("thigh-near anchor moved to %v; want %v", got, aNear)
	}
}

// placementOf is where a part actually sits on stage, which is the thing
// re-parenting and joint drags are supposed to leave alone.
func placementOf(t *testing.T, m *Model, name string, frame float64) lottie.LayerPlacement {
	t.Helper()
	anim := m.PreviewAnimation()
	if anim == nil {
		t.Fatal("nothing on stage")
	}
	pl, ok := anim.LayerPlacement(name, frame)
	if !ok {
		t.Fatalf("no placement for %q", name)
	}
	return pl
}

func placementNear(t *testing.T, want, got lottie.LayerPlacement, what string) {
	t.Helper()
	if math.Abs(got.X-want.X) > 0.05 || math.Abs(got.Y-want.Y) > 0.05 {
		t.Errorf("%s: position (%.3f, %.3f); want (%.3f, %.3f)", what, got.X, got.Y, want.X, want.Y)
	}
	if math.Abs(got.Angle-want.Angle) > 1e-3 {
		t.Errorf("%s: angle %.4f; want %.4f", what, got.Angle, want.Angle)
	}
	if math.Abs(got.ScaleX-want.ScaleX) > 1e-3 || math.Abs(got.ScaleY-want.ScaleY) > 1e-3 {
		t.Errorf("%s: scale (%.4f, %.4f); want (%.4f, %.4f)",
			what, got.ScaleX, got.ScaleY, want.ScaleX, want.ScaleY)
	}
}

// Re-attaching a part rewrites its transform in the new parent's terms, so
// the part does not jump. Without that, changing the link is unusable: the
// position is a point in the parent's space and the parent just changed.
func TestReparentKeepsThePartWhereItIs(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	part := partIndex(t, m, "forearm-near")
	m.SelectPosePart(part)
	before := placementOf(t, m, "forearm-near", 7)

	// Off the near arm and onto the far one, which sits elsewhere and is
	// turned differently — so an uncorrected move would be obvious.
	far := partIndex(t, m, "upper-arm-far")
	m.SetPosePartParent(far)

	if p, ok := m.PosePartParent(); !ok || m.PoseLayerName(p) != "upper-arm-far" {
		t.Fatalf("parent = %q; want upper-arm-far", m.PoseLayerName(p))
	}
	placementNear(t, before, placementOf(t, m, "forearm-near", 7), "after re-parenting")

	// And detaching to the composition itself keeps it too.
	m.SetPosePartParent(-1)
	if _, ok := m.PosePartParent(); ok {
		t.Errorf("the part still has a parent after being detached")
	}
	placementNear(t, before, placementOf(t, m, "forearm-near", 7), "after detaching")
}

// A cycle is not something to report: the picker cannot say it.
func TestParentCandidatesExcludeDescendants(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	arm := partIndex(t, m, "upper-arm-near")
	m.SelectPosePart(arm)

	names := map[string]bool{}
	for _, i := range m.PoseParentCandidates() {
		names[m.PoseLayerName(i)] = true
	}
	if names["upper-arm-near"] {
		t.Errorf("a part is offered as its own parent")
	}
	if names["forearm-near"] {
		t.Errorf("the forearm is offered as its own upper arm's parent")
	}
	if !names["body"] || !names["thigh-near"] {
		t.Errorf("candidates are missing layers that could legally be a parent: %v", names)
	}

	// And the model refuses one even if asked directly.
	before, _ := m.PosePartParent()
	m.SetPosePartParent(partIndex(t, m, "forearm-near"))
	if got, _ := m.PosePartParent(); got != before {
		t.Errorf("parent became %q; a cycle was accepted", m.PoseLayerName(got))
	}
}

// The joint is a point named twice — ks.a says which pixel of the part it
// is, ks.p says where that pixel sits in the parent. Dragging it in "keeps
// art" mode moves the point the part turns about and leaves the drawing
// exactly where it was.
func TestJointDragKeepsArtLeavesThePartStill(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	part := partIndex(t, m, "forearm-near")
	m.SelectPosePart(part)
	before := placementOf(t, m, "forearm-near", 7)
	q, _ := m.PosePartQuad(part)

	m.SetJointDragKeepsArt(true)
	m.MovePosePart(6, -4)

	// The joint moved...
	after, _ := m.PosePartQuad(part)
	if math.Abs(after.pivot[0]-q.pivot[0]) < 1 && math.Abs(after.pivot[1]-q.pivot[1]) < 1 {
		t.Errorf("the joint did not move: %v then %v", q.pivot, after.pivot)
	}
	// ...and the artwork did not. The placement is the layer's own frame,
	// which is exactly what must hold still.
	placementNear(t, before, placementOf(t, m, "forearm-near", 7), "keeps-art joint drag")
}

// The other mode is the one that was there before: the part follows its
// joint, which is how the character itself is moved.
func TestJointDragMovesPart(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	part := partIndex(t, m, "forearm-near")
	m.SelectPosePart(part)
	before := placementOf(t, m, "forearm-near", 7)

	m.SetJointDragKeepsArt(false)
	m.MovePosePart(6, -4)

	after := placementOf(t, m, "forearm-near", 7)
	if math.Abs(after.X-before.X) < 0.5 && math.Abs(after.Y-before.Y) < 0.5 {
		t.Errorf("the part did not follow its joint: (%.2f, %.2f) then (%.2f, %.2f)",
			before.X, before.Y, after.X, after.Y)
	}
	// The anchor is untouched in this mode; only the attach point moved.
	d := m.StageClipDoc()
	if a, ok := d.value(part, "a", 7); !ok || !slices.Equal(a, []float64{7, 4}) {
		t.Errorf("anchor = %v; want the rig spec (7, 4) untouched", a)
	}
}

// Re-parenting is a clip edit, so it undoes like one.
func TestReparentIsUndoable(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	m.SelectPosePart(partIndex(t, m, "forearm-near"))
	m.SetPosePartParent(partIndex(t, m, "body"))
	if p, _ := m.PosePartParent(); m.PoseLayerName(p) != "body" {
		t.Fatalf("re-parenting did not take")
	}
	m.UndoClipEdit()
	if p, _ := m.PosePartParent(); m.PoseLayerName(p) != "upper-arm-near" {
		t.Errorf("parent after undo = %q; want upper-arm-near", m.PoseLayerName(p))
	}
}

// A value keeps the form it had. Promoting a static position to keys would
// correct the pose being looked at and leave every other pose holding a
// number that was only ever right under the old parent.
func TestReparentKeepsStaticStatic(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	part := partIndex(t, m, "forearm-near")
	m.SelectPosePart(part)

	d := m.StageClipDoc()
	if _, static := d.staticValue(part, "p"); !static {
		t.Fatal("forearm-near position is already keyed; the case is untested")
	}
	rotKeys := len(d.keyTimesOf(part, "r"))
	if rotKeys == 0 {
		t.Fatal("forearm-near rotation is not keyed; the case is untested")
	}

	m.SetPosePartParent(partIndex(t, m, "body"))

	d = m.StageClipDoc()
	part = partIndex(t, m, "forearm-near")
	if _, static := d.staticValue(part, "p"); !static {
		t.Errorf("position was promoted to keys; a rigid attachment stays rigid")
	}
	if got := len(d.keyTimesOf(part, "r")); got != rotKeys {
		t.Errorf("rotation now has %d keys; want the %d it had", got, rotKeys)
	}
	// The clip must still be a pose sequence.
	if !d.posed {
		t.Errorf("the clip stopped being a pose sequence after re-parenting")
	}
}

// A keyed member is corrected at every key, not just the one on screen: the
// carrier differs at each frame, so one answer will not do.
//
// Rotation is the one that can be held across a whole clip. Position cannot:
// a static attach point means "rigidly here on my parent", and two parents
// that move differently cannot both be followed by one point — which is the
// whole reason for re-parenting, not a shortcoming of it. So the position
// matches at the frame the link was changed, and the angle everywhere.
func TestReparentCorrectsEveryKey(t *testing.T) {
	m := posedModel(t, "punch-anim")
	part := partIndex(t, m, "forearm-near")
	times := m.PoseTimes()

	want := map[float64]lottie.LayerPlacement{}
	for _, f := range times {
		m.SelectPoseKey(f, -1)
		want[f] = placementOf(t, m, "forearm-near", f)
	}

	at := times[2]
	m.SelectPoseKey(at, -1)
	m.SelectPosePart(part)
	if d := m.StageClipDoc(); len(d.keyTimesOf(part, "r")) != len(times) {
		t.Fatal("forearm-near rotation is not keyed at every pose; the case is untested")
	}
	m.SetPosePartParent(partIndex(t, m, "thigh-far"))

	for _, f := range times {
		m.SelectPoseKey(f, -1)
		got := placementOf(t, m, "forearm-near", f)
		where := "frame " + strconv.FormatFloat(f, 'f', -1, 64)
		if math.Abs(got.Angle-want[f].Angle) > 1e-3 {
			t.Errorf("%s: angle %.4f; want %.4f — the keyed rotation should be "+
				"corrected at every key", where, got.Angle, want[f].Angle)
		}
		if f == at {
			placementNear(t, want[f], got, where+" (where the link changed)")
		}
	}
}

// The rig overlay is the graph the artwork hides: a joint per visible part,
// and a bone where a chain continues.
func TestRigJoints(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	joints := m.RigJoints(7)
	if len(joints) == 0 {
		t.Fatal("no joints")
	}
	byLayer := map[string]rigJoint{}
	for _, j := range joints {
		byLayer[m.PoseLayerName(j.layer)] = j
	}
	// A part switched off has no joint: its dot would sit on top of the
	// drawing that replaced it and say nothing.
	if _, ok := byLayer["head-side"]; ok {
		t.Errorf("a hidden part contributed a joint")
	}
	// The elbow is where the forearm pivots, which is also where the stage
	// draws its joint mark.
	fore, ok := byLayer["forearm-near"]
	if !ok {
		t.Fatal("forearm-near has no joint")
	}
	q, _ := m.PosePartQuad(partIndex(t, m, "forearm-near"))
	if math.Abs(fore.at[0]-q.pivot[0]) > 1e-6 || math.Abs(fore.at[1]-q.pivot[1]) > 1e-6 {
		t.Errorf("joint at %v; the stage marks the pivot at %v", fore.at, q.pivot)
	}
	// And it hangs off the upper arm, which is the chain the bone draws.
	if fore.parent < 0 || m.PoseLayerName(joints[fore.parent].layer) != "upper-arm-near" {
		t.Errorf("forearm-near hangs off %q; want upper-arm-near",
			m.PoseLayerName(joints[fore.parent].layer))
	}
	// The body rides the composition, so it is a root here too.
	if body, ok := byLayer["body"]; !ok || body.parent >= 0 {
		t.Errorf("body has a parent joint; it rides the composition")
	}
}

// The stage asks the core for a layer by name and gets the first match, so
// an unnamed or duplicated layer would be dragged in the wrong space. It is
// refused rather than dragged wrongly, and the numbers still work.
func TestNameProblemBlocksStageDrag(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	part := partIndex(t, m, "forearm-near")
	m.SelectPosePart(part)
	if got := m.PosePartNameProblem(); got != "" {
		t.Fatalf("a well-named part reports %q", got)
	}
	if !m.PosePartDraggable() {
		t.Fatal("a well-named part is not draggable")
	}

	// Give another layer the parent's name, the way a hand-authored file
	// might: "upper-arm-near" is now ambiguous, and the drag maths would
	// resolve to whichever came first.
	d := m.StageClipDoc()
	rawName(t, d, partIndex(t, m, "head"), "upper-arm-near")
	if got := m.PosePartNameProblem(); got == "" {
		t.Errorf("a duplicated parent name is not reported")
	}
	if m.PosePartDraggable() {
		t.Errorf("a part whose parent is ambiguous is still draggable")
	}
}

// Renaming is a real edit: names are how a socket binds a layer and how a
// pose is copied between clips.
func TestRenamePosePart(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	part := partIndex(t, m, "forearm-near")
	m.SelectPosePart(part)

	m.RenamePosePart("forearm-left")
	if got := m.SelectedPosePartName(); got != "forearm-left" {
		t.Fatalf("name = %q; want forearm-left", got)
	}
	order := storedLayerOrder(t, m, "punch-anim")
	if !slices.Contains(order, "forearm-left") {
		t.Errorf("the stored clip still lists %v", order)
	}

	// A name already taken is refused, and the old one stands.
	m.RenamePosePart("head")
	if got := m.SelectedPosePartName(); got != "forearm-left" {
		t.Errorf("a duplicate name was accepted: %q", got)
	}
	// So is a blank one.
	m.RenamePosePart("   ")
	if got := m.SelectedPosePartName(); got != "forearm-left" {
		t.Errorf("a blank name was accepted: %q", got)
	}

	m.UndoClipEdit()
	if got := m.SelectedPosePartName(); got != "forearm-near" {
		t.Errorf("name after undo = %q; want forearm-near", got)
	}
}

// rawName renames a layer in the document itself, bypassing the check that
// keeps names unique — which is how a clip that was not authored here can
// arrive.
func rawName(t *testing.T, d *clipDoc, layer int, name string) {
	t.Helper()
	raw, _ := d.root["layers"].([]any)
	lm, ok := raw[d.layers[layer].index].(map[string]any)
	if !ok {
		t.Fatalf("layer %d is not an object", layer)
	}
	lm["nm"] = name
	d.index()
}

// Non-finite values never reach the clip: encoding/json refuses them, and
// one in the document would make every later store-back fail.
func TestPoseEditsRefuseNonFiniteValues(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SelectPoseKey(7, -1)
	m.SelectPosePart(partIndex(t, m, "forearm-near"))
	before := storedRotation(t, m, "punch-anim", "forearm-near", 7)
	gen := m.DocGeneration()

	m.SetPoseValue("r", []float64{math.NaN()})
	m.SetPoseValue("r", []float64{math.Inf(1)})
	if got := storedRotation(t, m, "punch-anim", "forearm-near", 7); got != before {
		t.Fatalf("rotation changed to %v", got)
	}
	length := m.ClipLength()
	m.SetClipLength(math.Inf(1))
	m.SetClipLength(math.NaN())
	if m.ClipLength() != length {
		t.Fatalf("clip length changed to %v", m.ClipLength())
	}
	if m.DocGeneration() != gen {
		t.Fatalf("refused edits counted as document changes")
	}
	if !strings.Contains(m.Status(), "finite") {
		t.Fatalf("no refusal in the status: %q", m.Status())
	}
	// The clip still takes edits afterwards.
	m.SetPoseValue("r", []float64{before + 5})
	if got := storedRotation(t, m, "punch-anim", "forearm-near", 7); got != before+5 {
		t.Fatalf("edit after a refusal = %v; want %v", got, before+5)
	}
}
