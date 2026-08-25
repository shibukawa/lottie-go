package lottie

import (
	"bytes"
	"testing"
	"time"
)

func TestAnimationMarkers(t *testing.T) {
	anim, err := Decode(bytes.NewReader(minimalAnimation(
		`[{"tm":0,"cm":"idle","dr":30},{"tm":30,"cm":"walk","dr":60},{"tm":90,"cm":"hit"}]`)))
	if err != nil {
		t.Fatal(err)
	}
	got := anim.Markers()
	want := []Marker{
		{Name: "idle", Start: 0, End: 30},
		{Name: "walk", Start: 30, End: 90},
		{Name: "hit", Start: 90, End: 90}, // no duration: a point marker
	}
	if len(got) != len(want) {
		t.Fatalf("Markers() = %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Markers()[%d] = %+v; want %+v", i, got[i], want[i])
		}
	}
	walk, ok := anim.Marker("walk")
	if !ok {
		t.Fatal("Marker(walk) not found")
	}
	if d := walk.Duration(anim); d != time.Second {
		t.Errorf("walk duration = %v; want 1s (60 frames at 60fps)", d)
	}
	if _, ok := anim.Marker("nope"); ok {
		t.Error("Marker(nope) reported found")
	}
}

func TestAnimationWithoutMarkers(t *testing.T) {
	anim, err := Decode(bytes.NewReader(minimalAnimation("")))
	if err != nil {
		t.Fatal(err)
	}
	if got := anim.Markers(); len(got) != 0 {
		t.Errorf("Markers() = %v; want none", got)
	}
}

// Markers() must not hand out the animation's own slice: an Animation is
// shared across players and documented as immutable.
func TestMarkersReturnsCopy(t *testing.T) {
	anim, err := Decode(bytes.NewReader(minimalAnimation(`[{"tm":0,"cm":"idle","dr":30}]`)))
	if err != nil {
		t.Fatal(err)
	}
	anim.Markers()[0].Name = "clobbered"
	if got, _ := anim.Marker("idle"); got.Name != "idle" {
		t.Errorf("Markers() exposed the internal slice; name is now %q", got.Name)
	}
}

// Markers are cues the animation carries: a game hangs a footstep or a hit
// frame off them rather than counting frames itself.
func TestPlayerEmitsMarkers(t *testing.T) {
	anim, err := Decode(bytes.NewReader(clipAnimation(6,
		`[{"tm":0,"cm":"start","dr":3},{"tm":3,"cm":"hit","dr":3}]`)))
	if err != nil {
		t.Fatal(err)
	}
	p := anim.NewPlayer()
	var got []string
	p.OnMarker(func(m Marker) { got = append(got, m.Name) })

	for range 6 {
		p.Update()
	}
	want := []string{"start", "hit"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("markers = %v; want %v", got, want)
	}
}

// Looping crosses the markers again on every pass.
func TestMarkersRepeatWhenLooping(t *testing.T) {
	anim, err := Decode(bytes.NewReader(clipAnimation(4,
		`[{"tm":2,"cm":"hit","dr":1}]`)))
	if err != nil {
		t.Fatal(err)
	}
	p := anim.NewPlayer()
	p.SetLoop(true)
	hits := 0
	p.OnMarker(func(m Marker) { hits++ })
	for range 12 {
		p.Update()
	}
	if hits < 2 {
		t.Errorf("marker fired %d times over three passes; want one per pass", hits)
	}
}

// A marker outside the played range is not the clip's business.
func TestMarkersOutsideTheRangeStayQuiet(t *testing.T) {
	anim, err := Decode(bytes.NewReader(clipAnimation(20,
		`[{"tm":2,"cm":"early","dr":1},{"tm":15,"cm":"late","dr":1}]`)))
	if err != nil {
		t.Fatal(err)
	}
	p := anim.NewPlayer()
	p.SetRange(10, 20)
	p.Rewind()
	var got []string
	p.OnMarker(func(m Marker) { got = append(got, m.Name) })
	for range 10 {
		p.Update()
	}
	for _, n := range got {
		if n == "early" {
			t.Errorf("a marker before the range fired: %v", got)
		}
	}
	if len(got) == 0 {
		t.Error("the marker inside the range never fired")
	}
}

func TestReversePlaybackStillCrossesMarkers(t *testing.T) {
	anim, err := Decode(bytes.NewReader(clipAnimation(6,
		`[{"tm":2,"cm":"hit","dr":1}]`)))
	if err != nil {
		t.Fatal(err)
	}
	p := anim.NewPlayer()
	p.SetReverse(true)
	p.Rewind()
	hits := 0
	p.OnMarker(func(m Marker) { hits++ })
	for range 6 {
		p.Update()
	}
	if hits == 0 {
		t.Error("playing backwards never crossed the marker")
	}
}

// Unnamed markers cannot be referred to, so they are not worth reporting.
func TestUnnamedMarkersAreNotEmitted(t *testing.T) {
	anim, err := Decode(bytes.NewReader(clipAnimation(6, `[{"tm":2,"dr":1}]`)))
	if err != nil {
		t.Fatal(err)
	}
	p := anim.NewPlayer()
	fired := 0
	p.OnMarker(func(m Marker) { fired++ })
	for range 6 {
		p.Update()
	}
	if fired != 0 {
		t.Errorf("an unnamed marker fired %d times", fired)
	}
}
