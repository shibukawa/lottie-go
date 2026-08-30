package main

import (
	"image"
	"math"
	"testing"
)

// The stage view is pure geometry over the model, so it is checked by
// asking where a point lands rather than by looking at a picture.

func stageBounds() image.Rectangle { return image.Rect(100, 50, 900, 650) }

func TestStageFitIsTheDefault(t *testing.T) {
	m := posedModel(t, "punch-anim")
	if m.StageZoom() != 1 {
		t.Errorf("StageZoom() = %v; want 1 before anything zooms", m.StageZoom())
	}
	if m.StageViewChanged() {
		t.Errorf("the view reports itself moved before anything moved it")
	}
	var s previewStage
	tr, ok := s.transform(m, stageBounds())
	if !ok {
		t.Fatal("no transform")
	}
	anim := m.PreviewAnimation()
	aw, ah := anim.Size()
	if want := stageFitScale(stageBounds(), aw, ah); tr.scale != want {
		t.Errorf("fit scale = %v; want %v", tr.scale, want)
	}
	// Fit means centred: the middle of the clip sits at the middle of the pane.
	x, y := tr.toScreen(float64(aw)/2, float64(ah)/2)
	b := stageBounds()
	if math.Abs(float64(x)-float64(b.Min.X+b.Max.X)/2) > 0.5 ||
		math.Abs(float64(y)-float64(b.Min.Y+b.Max.Y)/2) > 0.5 {
		t.Errorf("clip centre lands at (%v, %v); want the pane centre", x, y)
	}
}

// Zooming with the wheel holds whatever is under the cursor, which is the
// whole point: a joint magnified about the pane centre walks off the pane.
func TestStageWheelZoomHoldsThePointUnderTheCursor(t *testing.T) {
	m := posedModel(t, "punch-anim")
	var s previewStage
	b := stageBounds()
	cx, cy := 300, 200

	tr, _ := s.transform(m, b)
	ax, ay := tr.toAnim(cx, cy)

	for _, f := range []float64{1.1, 1.1, 1.1, 0.9} {
		s.zoomAt(m, b, cx, cy, f)
		tr, _ := s.transform(m, b)
		gx, gy := tr.toScreen(ax, ay)
		if math.Abs(float64(gx)-float64(cx)) > 0.5 || math.Abs(float64(gy)-float64(cy)) > 0.5 {
			t.Fatalf("zoom %v: the point under the cursor moved to (%v, %v); want (%v, %v)",
				f, gx, gy, cx, cy)
		}
	}
	if m.StageZoom() <= 1 {
		t.Errorf("StageZoom() = %v; want more than 1 after zooming in", m.StageZoom())
	}
	if !m.StageViewChanged() {
		t.Errorf("the view does not report itself moved after a zoom")
	}
}

// The buttons magnify about the middle of the pane, so the thing being
// looked at does not drift towards the centre on every press.
func TestStageButtonZoomHoldsThePaneCentre(t *testing.T) {
	m := posedModel(t, "punch-anim")
	var s previewStage
	b := stageBounds()
	// Start off-centre, or holding the centre would be trivially true.
	s.zoomAt(m, b, 250, 180, 2)

	tr, _ := s.transform(m, b)
	midX := float64(b.Min.X+b.Max.X) / 2
	midY := float64(b.Min.Y+b.Max.Y) / 2
	ax, ay := tr.toAnim(int(midX), int(midY))

	m.ZoomStage(stageZoomButtonStep)
	tr, _ = s.transform(m, b)
	gx, gy := tr.toScreen(ax, ay)
	if math.Abs(float64(gx)-midX) > 1 || math.Abs(float64(gy)-midY) > 1 {
		t.Errorf("pane centre showed (%v, %v) after a button zoom; want (%v, %v)",
			gx, gy, midX, midY)
	}
}

func TestStageZoomClampsAndResets(t *testing.T) {
	m := posedModel(t, "punch-anim")
	for range 60 {
		m.ZoomStage(stageZoomButtonStep)
	}
	if m.StageZoom() != stageZoomMax {
		t.Errorf("StageZoom() = %v after zooming in hard; want the %v ceiling",
			m.StageZoom(), stageZoomMax)
	}
	for range 60 {
		m.ZoomStage(1 / stageZoomButtonStep)
	}
	if m.StageZoom() != stageZoomMin {
		t.Errorf("StageZoom() = %v after zooming out hard; want the %v floor",
			m.StageZoom(), stageZoomMin)
	}
	m.SetStageView(3, 40, -20)
	m.ResetStageView()
	if m.StageZoom() != 1 || m.StageViewChanged() {
		t.Errorf("reset left zoom %v pan %v/%v; want the fit back",
			m.StageZoom(), m.stagePanX, m.stagePanY)
	}
}

// Panning moves the picture with the cursor, one screen pixel per pixel,
// whatever the zoom.
func TestStagePanMovesWithTheCursor(t *testing.T) {
	m := posedModel(t, "punch-anim")
	var s previewStage
	b := stageBounds()
	s.zoomAt(m, b, 400, 300, 4)

	tr, _ := s.transform(m, b)
	ax, ay := tr.toAnim(400, 300)
	px, py := m.StagePan()
	m.SetStageView(m.StageZoom(), px+37, py-11)

	tr, _ = s.transform(m, b)
	gx, gy := tr.toScreen(ax, ay)
	if math.Abs(float64(gx)-437) > 0.5 || math.Abs(float64(gy)-289) > 0.5 {
		t.Errorf("after panning by (37, -11) the point is at (%v, %v); want (437, 289)", gx, gy)
	}
}

// The split is seeded once from the real window height and then belongs to
// whoever dragged it.
func TestPreviewHeightSeededOnce(t *testing.T) {
	m := NewModel()
	m.InitPreviewHeight(300)
	if got := m.PreviewHeight(); got != 300 {
		t.Fatalf("PreviewHeight() = %v; want the seed 300", got)
	}
	m.SetPreviewHeight(420)
	m.InitPreviewHeight(300)
	if got := m.PreviewHeight(); got != 420 {
		t.Errorf("PreviewHeight() = %v; a later layout re-seeded a dragged split", got)
	}
}

// The onion skin picks the keys either side of the playhead. Which side a
// ghost is on decides its tint, so the flag matters as much as the frame.
func TestOnionGhostsBracketThePlayhead(t *testing.T) {
	m := posedModel(t, "punch-anim") // keys at 0 4 7 12 20
	if got := m.OnionGhosts(); got != nil {
		t.Errorf("OnionGhosts() = %v while the toggle is off; want none", got)
	}
	m.SetOnionSkin(true)

	m.SelectPoseKey(7, -1)
	got := m.OnionGhosts()
	want := []onionGhost{{frame: 4}, {frame: 12, next: true}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("at key 7: %v; want %v", got, want)
	}

	// The ends have one neighbour each, and asking for the missing one must
	// not invent a ghost at the same frame as the pose being edited.
	m.SelectPoseKey(0, -1)
	if got := m.OnionGhosts(); len(got) != 1 || got[0] != (onionGhost{frame: 4, next: true}) {
		t.Errorf("at the first key: %v; want only the next one", got)
	}
	m.SelectPoseKey(20, -1)
	if got := m.OnionGhosts(); len(got) != 1 || got[0] != (onionGhost{frame: 12}) {
		t.Errorf("at the last key: %v; want only the previous one", got)
	}

	// Between keys, the pair brackets the playhead — as useful scrubbing as
	// it is parked.
	m.PreviewSeek(9)
	got = m.OnionGhosts()
	if len(got) != 2 || got[0].frame != 7 || !got[1].next || got[1].frame != 12 {
		t.Errorf("between keys: %v; want 7 and 12", got)
	}
}

// Playing would change the pair several times a second and strobe.
func TestOnionGhostsStayOffWhilePlaying(t *testing.T) {
	m := posedModel(t, "punch-anim")
	m.SetOnionSkin(true)
	m.SelectPoseKey(7, -1)
	if len(m.OnionGhosts()) == 0 {
		t.Fatal("no ghosts while parked; the case is untested")
	}
	m.TogglePreviewPlaying()
	if !m.PreviewPlaying() {
		t.Fatal("play did not start")
	}
	if got := m.OnionGhosts(); got != nil {
		t.Errorf("OnionGhosts() = %v while playing; want none", got)
	}
}

// A clip whose properties disagree on their key times has no pose row, but
// it still has drawings either side of the playhead.
func TestOnionGhostsWorkWithoutAPoseRow(t *testing.T) {
	m := NewModel()
	m.Open("../../examples/state-editor/character/walk-anim.json")
	m.ShowClip(clipRef{Anim: "walk-anim"})
	m.SetCollisionTab(colPoses)
	m.SetOnionSkin(true)
	if len(m.PoseTimes()) != 0 {
		t.Fatal("the sample clip reads as a pose sequence; the fallback is untested")
	}
	d := m.StageClipDoc()
	if d == nil || len(d.times) < 3 {
		t.Skip("sample clip has too few keys to bracket")
	}
	m.PausePreview()
	m.PreviewSeek(d.times[1])
	if got := m.OnionGhosts(); len(got) != 2 {
		t.Errorf("OnionGhosts() = %v; want a pair from the union of every track", got)
	}
}
