package lottieevents

import (
	"strings"
	"testing"

	lottie "github.com/shibukawa/lottie-go"
)

const fixture = `{
  "v": "5.9.0", "fr": 60, "ip": 0, "op": 60, "w": 100, "h": 100,
  "layers": [
    {"ty": 3, "nm": "n", "ind": 1, "ip": 0, "op": 60, "st": 0, "ks": {}}
  ]
}`

func track() *Track {
	return &Track{Events: []Event{
		{Frame: 40, Name: "swing"},
		{Frame: 10, Name: "step", Payload: []byte(`{"vol":0.4}`)},
		{Frame: 30, Name: "step", Payload: []byte(`{"vol":0.7}`)},
	}}
}

func TestIn(t *testing.T) {
	tr := track()
	got := tr.In(0, 35)
	if len(got) != 2 || got[0].Name != "step" || got[0].Frame != 10 || got[1].Frame != 30 {
		t.Fatalf("In(0,35): %+v", got)
	}
	// Half-open: an event exactly at `to` waits for the next span.
	if got := tr.In(0, 10); len(got) != 0 {
		t.Fatalf("In(0,10): %+v", got)
	}
	if got := tr.In(10, 11); len(got) != 1 {
		t.Fatalf("In(10,11): %+v", got)
	}
}

func TestCueThroughPlayback(t *testing.T) {
	a, err := lottie.Decode(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	p := a.NewPlayer()
	p.SetLoop(true)
	p.Play()

	var fired []string
	Cue(p, track(), func(e Event) { fired = append(fired, e.Name) })

	// 60fps clip at 60 TPS advances one frame per Update; run through one
	// full loop and past frame 10 of the next.
	for range 75 {
		p.Update()
	}
	// step(10), step(30), swing(40), then the wrap replays step(10).
	want := []string{"step", "step", "swing", "step"}
	if len(fired) != len(want) {
		t.Fatalf("fired: %v", fired)
	}
	for i := range want {
		if fired[i] != want[i] {
			t.Fatalf("fired: %v, want %v", fired, want)
		}
	}
}

func TestBundleRoundTripAndPayload(t *testing.T) {
	b := lottie.NewBundle()
	if err := Store(b, "attack", track()); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if ids := IDs(b); len(ids) != 1 || ids[0] != "attack" {
		t.Fatalf("IDs: %v", ids)
	}
	got, err := Load(b, "attack")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Events) != 3 {
		t.Fatalf("events: %+v", got.Events)
	}
	ev := got.In(10, 11)[0]
	if string(ev.Payload) != `{"vol":0.4}` {
		t.Fatalf("payload: %s", ev.Payload)
	}
	Remove(b, "attack")
	if ids := IDs(b); len(ids) != 0 {
		t.Fatalf("Remove left %v", ids)
	}
}

func TestExtraFieldsSurvive(t *testing.T) {
	raw := []byte(`{"events":[{"frame":5,"name":"x","team":"red"}],"note":"keep"}`)
	tr, err := ParseTrack(raw)
	if err != nil {
		t.Fatalf("ParseTrack: %v", err)
	}
	out, err := tr.MarshalJSON()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, want := range []string{`"team":"red"`, `"note":"keep"`} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("extra member %s lost: %s", want, out)
		}
	}
}
