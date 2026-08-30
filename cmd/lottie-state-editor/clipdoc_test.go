package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// The clip document is where a pose edit actually lands, so these tests
// check the numbers it writes rather than what the stage looks like: a wrong
// value decodes fine and simply animates wrong.

func readClip(t *testing.T, parts ...string) *clipDoc {
	t.Helper()
	path := filepath.Join(append([]string{"..", ".."}, parts...)...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	d, err := newClipDoc("clip", data)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return d
}

func presetClip(t *testing.T, name string) *clipDoc {
	t.Helper()
	return readClip(t, "testdata", "presets", "chibi-male", name)
}

// A preset clip is a pose sequence: every animated property shares one set
// of key times. The whole timeline design rests on it, so it is asserted
// rather than assumed.
func TestClipDocPoseSequence(t *testing.T) {
	d := presetClip(t, "punch-anim.json")
	if !d.posed {
		t.Fatalf("punch-anim is not a pose sequence; times=%v", d.times)
	}
	want := []float64{0, 4, 7, 12, 20}
	if !slices.Equal(d.times, want) {
		t.Errorf("times = %v; want %v", d.times, want)
	}
	if len(d.animatedLayers()) == 0 {
		t.Errorf("no animated layers found")
	}
}

// Clips the editor ships as samples are not pose sequences, so the fallback
// path is real. If this ever starts passing as posed, the per-layer rows
// have lost their only test subject.
func TestClipDocFallsBackWhenTimesDisagree(t *testing.T) {
	d := readClip(t, "testdata", "editor", "character", "walk-anim.json")
	if d.posed {
		t.Errorf("walk-anim reads as a pose sequence; expected disagreeing key times")
	}
	if len(d.animatedLayers()) == 0 {
		t.Errorf("no animated layers to draw as rows")
	}
}

// Re-encoding an untouched clip must reproduce it byte for byte. Presets are
// committed JSON, so any drift here shows up as diff noise on every save.
func TestClipDocRoundTripsUnchanged(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "presets", "chibi-male", "punch-anim.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	d, err := newClipDoc("clip", data)
	if err != nil {
		t.Fatal(err)
	}
	got, err := d.encode()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Errorf("re-encode differs from source (%d bytes vs %d)", len(got), len(data))
	}
}

func layerNamed(t *testing.T, d *clipDoc, name string) int {
	t.Helper()
	for i := range d.layers {
		if d.layers[i].name == name {
			return i
		}
	}
	t.Fatalf("no layer %q", name)
	return -1
}

// Editing a keyed property writes at that key and leaves its neighbours
// alone: a pose edit is one number at one time.
func TestClipDocSetValueAtKey(t *testing.T) {
	d := presetClip(t, "punch-anim.json")
	i := layerNamed(t, d, "forearm-near")

	before, ok := d.value(i, "r", 7)
	if !ok {
		t.Fatalf("no rotation key at frame 7")
	}
	at0, _ := d.value(i, "r", 0)

	if !d.setValue(i, "r", 7, []float64{before[0] + 15}) {
		t.Fatalf("setValue reported no change")
	}
	got, _ := d.value(i, "r", 7)
	if got[0] != before[0]+15 {
		t.Errorf("rotation at 7 = %v; want %v", got[0], before[0]+15)
	}
	if now, _ := d.value(i, "r", 0); now[0] != at0[0] {
		t.Errorf("rotation at 0 changed to %v; want %v", now[0], at0[0])
	}
	// Writing the same value again is not an edit, so a drag that ends
	// where it started never marks the bundle dirty.
	if d.setValue(i, "r", 7, got) {
		t.Errorf("setValue reported a change for an identical value")
	}
}

// Editing a static property keys it at every pose time, holding what it used
// to be everywhere the edit did not touch — the inverse of the generator
// collapsing an unchanging track to a static value.
func TestClipDocPromotesStaticProperty(t *testing.T) {
	d := presetClip(t, "punch-anim.json")
	i := layerNamed(t, d, "forearm-near")

	was, ok := d.staticValue(i, "p")
	if !ok {
		t.Fatalf("position is not static to begin with")
	}
	moved := []float64{was[0] + 3, was[1] - 2}
	if !d.setValue(i, "p", 7, moved) {
		t.Fatalf("setValue reported no change")
	}
	if _, still := d.staticValue(i, "p"); still {
		t.Errorf("position is still static after an edit")
	}
	for _, tt := range d.times {
		got, ok := d.valueAt(i, "p", tt)
		if !ok {
			t.Fatalf("no position key at pose time %v", tt)
		}
		want := was
		if tt == 7 {
			want = moved
		}
		if !slices.Equal(got, want) {
			t.Errorf("position at %v = %v; want %v", tt, got, want)
		}
	}
	// The clip must stay a pose sequence, or the timeline drops to
	// per-layer rows the moment anyone edits anything.
	if !d.posed {
		t.Errorf("clip stopped being a pose sequence after promotion")
	}
}

// A pose tick is the whole column, so retiming it moves every layer keyed
// there and the clip stays a pose sequence.
func TestClipDocRetimeMovesWholeColumn(t *testing.T) {
	d := presetClip(t, "punch-anim.json")
	i := layerNamed(t, d, "forearm-near")
	j := layerNamed(t, d, "upper-arm-near")
	wasI, _ := d.value(i, "r", 7)
	wasJ, _ := d.value(j, "r", 7)

	landed, moved := d.retime(7, 9, -1)
	if !moved || landed != 9 {
		t.Fatalf("retime(7 -> 9) = %v, %v", landed, moved)
	}
	if !d.posed || !slices.Equal(d.times, []float64{0, 4, 9, 12, 20}) {
		t.Fatalf("times = %v posed=%v; want [0 4 9 12 20] posed", d.times, d.posed)
	}
	if got, _ := d.value(i, "r", 9); !slices.Equal(got, wasI) {
		t.Errorf("forearm value did not travel with the key: %v want %v", got, wasI)
	}
	if got, _ := d.value(j, "r", 9); !slices.Equal(got, wasJ) {
		t.Errorf("upper-arm value did not travel with the key: %v want %v", got, wasJ)
	}
}

// A key stops one frame short of its neighbours, so a drag can never reorder
// the list or land two keys on one frame.
func TestClipDocRetimeClampsToNeighbours(t *testing.T) {
	d := presetClip(t, "punch-anim.json")
	if landed, _ := d.retime(7, 99, -1); landed != 11 {
		t.Errorf("retime(7 -> 99) landed at %v; want 11 (one short of 12)", landed)
	}
	d = presetClip(t, "punch-anim.json")
	if landed, _ := d.retime(7, -99, -1); landed != 5 {
		t.Errorf("retime(7 -> -99) landed at %v; want 5 (one past 4)", landed)
	}
}

// Hold keys carry the discrete swaps. The timeline marks them differently,
// so reading the flag has to work.
func TestClipDocDetectsHoldKeys(t *testing.T) {
	d := presetClip(t, "idle-turn-anim.json")
	found := false
	for i := range d.layers {
		for p := range d.layers[i].keyed {
			for _, tt := range d.layers[i].keyed[p] {
				if d.isHold(i, p, tt) {
					found = true
				}
			}
		}
	}
	if !found {
		t.Errorf("idle-turn has no hold keys; the turn should swap limb sides with them")
	}
}

// An edited document must still be a clip: encode, re-parse, and confirm the
// value survived the trip a save would take it on.
func TestClipDocEditSurvivesEncode(t *testing.T) {
	d := presetClip(t, "punch-anim.json")
	i := layerNamed(t, d, "forearm-near")
	if !d.setValue(i, "r", 7, []float64{-12.5}) {
		t.Fatal("setValue reported no change")
	}
	data, err := d.encode()
	if err != nil {
		t.Fatal(err)
	}
	var probe map[string]any
	if err := json.Unmarshal(data, &probe); err != nil {
		t.Fatalf("edited clip is not valid JSON: %v", err)
	}
	back, err := newClipDoc("clip", data)
	if err != nil {
		t.Fatalf("edited clip does not re-parse: %v", err)
	}
	got, ok := back.value(layerNamed(t, back, "forearm-near"), "r", 7)
	if !ok || got[0] != -12.5 {
		t.Errorf("rotation after round trip = %v (%v); want -12.5", got, ok)
	}
}
