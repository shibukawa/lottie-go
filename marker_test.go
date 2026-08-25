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
