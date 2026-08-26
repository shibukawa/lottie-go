package lottie

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func testBundleWithClip(t *testing.T) *Bundle {
	t.Helper()
	data, err := os.ReadFile("testdata/basic.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	b := NewBundle()
	if err := b.SetAnimation("clip", data); err != nil {
		t.Fatalf("SetAnimation: %v", err)
	}
	return b
}

func sampleTrack() *ResolvTrack {
	return &ResolvTrack{Boxes: []ResolvBox{
		{
			Name: "punch", Kind: ResolvRect, Tags: []string{"hit"},
			Spans: []ResolvSpan{
				{From: 10, To: 14, X: 100, Y: 40, W: 60, H: 30},
				{From: 14, To: 18, X: 130, Y: 40, W: 60, H: 30},
			},
		},
		{
			Name: "body", Kind: ResolvRect, Tags: []string{"hurt", "push"},
			Spans: []ResolvSpan{{From: 0, To: 60, X: 20, Y: 10, W: 40, H: 80}},
		},
		{
			Name: "head", Kind: ResolvCircle, Tags: []string{"hurt"},
			Spans: []ResolvSpan{{From: 0, To: 60, X: 40, Y: 8, R: 12}},
		},
	}}
}

func TestResolvTrackAt(t *testing.T) {
	tr := sampleTrack()

	if got := tr.At(5); len(got) != 2 {
		t.Fatalf("frame 5: got %d boxes, want 2", len(got))
	}
	got := tr.At(12)
	if len(got) != 3 {
		t.Fatalf("frame 12: got %d boxes, want 3", len(got))
	}
	if got[0].Name != "punch" || got[0].X != 100 {
		t.Fatalf("frame 12 punch: got %+v", got[0])
	}
	// The second span takes over at its From; the first ends exclusively.
	if g := tr.At(14); g[0].X != 130 {
		t.Fatalf("frame 14 punch span: got x=%v, want 130", g[0].X)
	}
	// To is exclusive: the box vanishes exactly at 18.
	if g := tr.At(18); len(g) != 2 {
		t.Fatalf("frame 18: got %d boxes, want 2", len(g))
	}

	hurt := tr.At(12, "hurt")
	if len(hurt) != 2 || hurt[0].Name != "body" || hurt[1].Name != "head" {
		t.Fatalf("hurt filter: got %+v", hurt)
	}
	// Multiple tags act as any-of.
	if g := tr.At(12, "hit", "push"); len(g) != 2 {
		t.Fatalf("hit|push filter: got %d boxes, want 2", len(g))
	}
	if g := tr.At(12, "nosuch"); len(g) != 0 {
		t.Fatalf("unknown tag: got %d boxes, want 0", len(g))
	}
	if idx := tr.At(12)[2].Index; idx != 2 {
		t.Fatalf("Index: got %d, want 2", idx)
	}
}

func TestPhysicsBundleRoundTrip(t *testing.T) {
	b := testBundleWithClip(t)

	body := &CPBody{
		Type: CPBodyDynamic, Mass: 2,
		Shapes: []CPShape{
			{Type: CPShapeCircle, Center: PhysPoint{X: 50, Y: 30}, Radius: 20, Friction: 0.5},
			{Type: CPShapeBox, Center: PhysPoint{X: 50, Y: 80}, Width: 40, Height: 60, Elasticity: 0.1},
			{Type: CPShapePolygon, Vertices: []PhysPoint{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 5, Y: 8}}, Sensor: true},
		},
	}
	if err := b.SetCPBody("player", body); err != nil {
		t.Fatalf("SetCPBody: %v", err)
	}
	if err := b.SetResolvTrack("clip", sampleTrack()); err != nil {
		t.Fatalf("SetResolvTrack: %v", err)
	}

	var buf bytes.Buffer
	if err := b.Encode(&buf); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	rb, err := DecodeBundle(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("DecodeBundle: %v", err)
	}

	if got := rb.CPBodyIDs(); !reflect.DeepEqual(got, []string{"player"}) {
		t.Fatalf("CPBodyIDs: got %v", got)
	}
	rBody, err := rb.CPBody("player")
	if err != nil {
		t.Fatalf("CPBody: %v", err)
	}
	if !reflect.DeepEqual(rBody, body) {
		t.Fatalf("cp body did not round-trip:\n got %+v\nwant %+v", rBody, body)
	}

	if got := rb.ResolvTrackIDs(); !reflect.DeepEqual(got, []string{"clip"}) {
		t.Fatalf("ResolvTrackIDs: got %v", got)
	}
	rTrack, err := rb.ResolvTrack("clip")
	if err != nil {
		t.Fatalf("ResolvTrack: %v", err)
	}
	if !reflect.DeepEqual(rTrack, sampleTrack()) {
		t.Fatalf("resolv track did not round-trip:\n got %+v", rTrack)
	}
}

func TestPhysicsExtraFieldsSurvive(t *testing.T) {
	raw := []byte(`{"boxes":[{"name":"b","kind":"rect","damage":12,` +
		`"spans":[{"from":0,"to":5,"x":1,"y":2,"w":3,"h":4,"knockback":7}]}],"custom":true}`)
	tr, err := ParseResolvTrack(raw)
	if err != nil {
		t.Fatalf("ParseResolvTrack: %v", err)
	}
	out, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, want := range []string{`"damage":12`, `"knockback":7`, `"custom":true`} {
		if !bytes.Contains(out, []byte(want)) {
			t.Fatalf("extra member %s lost: %s", want, out)
		}
	}
}

func TestForeignExtensionFilesSurvive(t *testing.T) {
	b := testBundleWithClip(t)
	var buf bytes.Buffer
	if err := b.Encode(&buf); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// Rebuild the archive with a foreign extension member injected, the way
	// another tool would have written it.
	rb, err := DecodeBundle(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("DecodeBundle: %v", err)
	}
	rb.extFiles["extensions/othertool/notes.json"] = []byte(`{"keep":"me"}`)
	buf.Reset()
	if err := rb.Encode(&buf); err != nil {
		t.Fatalf("re-Encode: %v", err)
	}
	rb2, err := DecodeBundle(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("re-DecodeBundle: %v", err)
	}
	if got := rb2.extFiles["extensions/othertool/notes.json"]; string(got) != `{"keep":"me"}` {
		t.Fatalf("foreign extension file lost: %q", got)
	}
}

func TestRemoveAnimationDropsResolvTrack(t *testing.T) {
	b := testBundleWithClip(t)
	if err := b.SetResolvTrack("clip", sampleTrack()); err != nil {
		t.Fatalf("SetResolvTrack: %v", err)
	}
	b.RemoveAnimation("clip")
	if got := b.ResolvTrackIDs(); len(got) != 0 {
		t.Fatalf("track survived its clip: %v", got)
	}
}
