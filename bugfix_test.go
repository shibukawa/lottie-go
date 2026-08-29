package lottie

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// Regression tests for a batch of bugs found by review: each test pins the
// corrected behavior and names the failure it guards against.

// timeRemapFixture puts a null inside a precomp whose layer carries a
// constant time remap of 0.25s: at 60fps every root frame shows the
// precomp's frame 15.
const timeRemapFixture = `{
  "v": "5.9.0", "fr": 60, "ip": 0, "op": 60, "w": 200, "h": 200,
  "assets": [
    {"id": "pc", "layers": [
      {"ty": 3, "nm": "muzzle", "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {
        "p": {"a": 1, "k": [
          {"t": 0, "s": [5, 5], "o": {"x": 0.5, "y": 0}, "i": {"x": 0.5, "y": 1}},
          {"t": 30, "s": [35, 5]}
        ]}
      }}
    ]}
  ],
  "layers": [
    {"ty": 0, "nm": "gun", "ind": 1, "refId": "pc", "ip": 0, "op": 60, "st": 0,
     "w": 100, "h": 100, "tm": {"a": 0, "k": 0.25}, "ks": {
      "p": {"a": 0, "k": [100, 100]}
    }}
  ]
}`

// LayerPlacement must apply a precomp's time remap the way rendering does;
// it used to pass only the stretch/start-time local frame down, so
// attachments drifted from the drawn artwork.
func TestLayerPlacementPrecompTimeRemap(t *testing.T) {
	a, err := Decode(strings.NewReader(timeRemapFixture))
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	// The remap pins the precomp at its frame 15: halfway through the
	// muzzle's [5..35] sweep (the symmetric ease resolves to 0.5), plus
	// the precomp layer's own (100, 100).
	for _, frame := range []float64{0, 40} {
		p, ok := a.LayerPlacement("muzzle", frame)
		if !ok {
			t.Fatalf("muzzle not found at frame %v", frame)
		}
		if !near(p.X, 120) || !near(p.Y, 105) {
			t.Fatalf("frame %v: got (%v, %v), want (120, 105)", frame, p.X, p.Y)
		}
	}
}

// A seek that lands on the range end must not silently pause the player:
// the next Update runs off the end and reports completion, so a scrub to
// 100% still fires OnComplete.
func TestSeekToEndCompletesOnNextUpdate(t *testing.T) {
	anim, err := Decode(bytes.NewReader(clipAnimation(10, "")))
	if err != nil {
		t.Fatal(err)
	}
	p := anim.NewPlayer()
	done := 0
	p.OnComplete(func() { done++ })
	p.Play()
	p.SetProgress(1)
	if !p.IsPlaying() {
		t.Fatal("seeking to the end paused the player without completing")
	}
	if done != 0 {
		t.Fatal("completion fired from the seek itself")
	}
	p.Update()
	if done != 1 {
		t.Fatalf("completions after the next Update = %d; want 1", done)
	}
	if p.IsPlaying() {
		t.Error("player still running after completing")
	}
}

// Restart must return a phased scene to its first phase and re-arm the
// timed phase end, or the intro never replays and OnPhaseEnd fires once
// per process instead of once per run.
func TestSceneRestartResetsPhase(t *testing.T) {
	sp := phasedScene(t)
	for range 40 { // crosses the intro's 0.5s
		sp.Update()
	}
	if got := sp.Phase(); got != "main" {
		t.Fatalf("phase before restart = %q, want main", got)
	}
	sp.Restart()
	if got := sp.Phase(); got != "intro" {
		t.Errorf("phase after restart = %q, want intro", got)
	}

	// A timed phase with no successor: its end event must fire again on
	// every run.
	s, _ := menuScene(t)
	s.Phases = []ScenePhase{{Name: "only", Duration: 0.3}}
	b := sceneTestBundle(t)
	sp2, err := s.NewScenePlayer(func(string) (*Bundle, error) { return b, nil })
	if err != nil {
		t.Fatal(err)
	}
	ends := 0
	sp2.OnPhaseEnd(func(string) { ends++ })
	for range 30 {
		sp2.Update()
	}
	if ends != 1 {
		t.Fatalf("phase ends before restart = %d; want 1", ends)
	}
	sp2.Restart()
	for range 30 {
		sp2.Update()
	}
	if ends != 2 {
		t.Errorf("phase ends after restart = %d; want 2", ends)
	}
}

// Validate must accept a counted loop before a chain: LoopCount makes the
// loop complete, so the chain does run.
func TestValidateAllowsCountedLoopChain(t *testing.T) {
	s, _ := menuScene(t)
	n, _ := s.Node("logo")
	n.Playback.Loop = true
	n.Playback.LoopCount = 2
	n.Playback.Then = []ScenePlayStep{{Segment: "out"}}
	if errs := s.Validate(); len(errs) != 0 {
		t.Errorf("counted loop with a chain reported %v", errs)
	}
	n.Playback.LoopCount = 0
	if errs := s.Validate(); len(errs) == 0 {
		t.Error("endless loop with a chain passed validation")
	}
}

// A gradient declaring more color stops than the renderer keeps must slice
// its alpha-stop tail before clamping, or trailing color stops are read
// back as phantom opacity stops.
func TestGradientStopsBeyondCapKeepOpacity(t *testing.T) {
	const declared = maxGradStops + 1
	data := make([]float64, 0, declared*4)
	for i := range declared {
		pos := float64(i) / float64(declared-1)
		data = append(data, pos, 1, 0.5, 0) // opaque orange, no alpha tail
	}
	var g gradientCmd
	buildGradientStops(&g, data, declared, 1, nil)
	if g.count != maxGradStops {
		t.Fatalf("stop count = %d; want clamped to %d", g.count, maxGradStops)
	}
	for i := range g.count {
		if g.colors[i][3] != 1 {
			t.Fatalf("stop %d alpha = %v; want fully opaque (phantom alpha stops parsed from color data)", i, g.colors[i][3])
		}
	}
}

// The 0..255-color heuristic must be decided from the authored keyframes,
// not the interpolated per-frame value: an overshooting ease briefly pushes
// a 0..1 color past 1.0 and used to flash it to near-black.
func TestColorScaleDecidedFromAuthoredKeys(t *testing.T) {
	anim := decodeLayers(t, `{"ty":4,"ind":1,"ip":0,"op":30,"st":0,"ks":{},"shapes":[
	  {"ty":"fl","c":{"a":1,"k":[
	     {"t":0,"s":[0,0,0,1],"o":{"x":0.5,"y":1.8},"i":{"x":0.5,"y":1}},
	     {"t":30,"s":[1,1,1,1]}]},"o":{"a":0,"k":100}}]}`)
	var fl *shapeNode
	for _, n := range anim.layers[0].shapes {
		if n.kind == "fl" {
			fl = n
		}
	}
	if fl == nil {
		t.Fatal("no fill node parsed")
	}
	if fl.colorDiv != 1 || fl.alphaDiv != 1 {
		t.Fatalf("divisors = (%v, %v); want (1, 1) for a 0..1-authored color", fl.colorDiv, fl.alphaDiv)
	}
	// Sanity: the ease really does overshoot mid-segment, the case the old
	// per-frame check misread as a 0..255 color.
	over := false
	for f := 0.0; f <= 30; f += 0.5 {
		if v := fl.color.at(f, nil); len(v) > 0 && v[0] > 1 {
			over = true
			break
		}
	}
	if !over {
		t.Fatal("fixture never overshoots; the scenario is not exercised")
	}

	tr := staticTrack(255, 128, 0, 255)
	if rgb, a := colorDivisors(tr); rgb != 255 || a != 255 {
		t.Fatalf("0..255 static color divisors = (%v, %v); want (255, 255)", rgb, a)
	}
}

// A legacy keyframe's explicit end value defines its segment's target: a
// file authored with e_i != s_{i+1} interpolates toward e_i and jump-cuts
// at the boundary, as lottie-web plays it.
func TestLegacyEndValueJumpCut(t *testing.T) {
	var p rawProp
	if err := json.Unmarshal([]byte(`{"a":1,"k":[
	  {"t":0,"s":[0],"e":[10]},
	  {"t":10,"s":[100],"e":[100]},
	  {"t":20}]}`), &p); err != nil {
		t.Fatal(err)
	}
	tr, err := parseVectorProp(&p, []float64{0})
	if err != nil {
		t.Fatal(err)
	}
	if got := tr.scalarAt(5, 0); !near(got, 5) {
		t.Errorf("mid-segment = %v; want 5 (interpolating toward e=[10])", got)
	}
	if got := tr.scalarAt(10, 0); !near(got, 100) {
		t.Errorf("at the boundary = %v; want the jump cut to 100", got)
	}
}

// A trim range wrapping a closed contour's seam must come out as one
// continuous subpath: two pieces meeting there render butt caps instead of
// a stroke join.
func TestTrimWrapJoinsClosedSeam(t *testing.T) {
	tr := &trimmer{}
	g := geometry{alpha: 1}
	g.bez = bezierShape{
		Closed: true,
		V:      [][2]float64{{0, 0}, {10, 0}, {10, 10}, {0, 10}},
		I:      make([][2]float64, 4),
		O:      make([][2]float64, 4),
	}
	tr.trimContour(&g, 0.75, 1.25)
	if len(tr.out) != 1 {
		t.Fatalf("wrapped trim produced %d subpaths; want 1 joined through the seam", len(tr.out))
	}
	b := &tr.out[0].bez
	if b.Closed {
		t.Error("trimmed range came out closed")
	}
	if len(b.V) != 3 {
		t.Fatalf("vertices = %d (%v); want 3", len(b.V), b.V)
	}
	if !near(b.V[0][0], 0) || !near(b.V[0][1], 10) {
		t.Errorf("start = %v; want (0, 10), fraction 0.75 of the perimeter", b.V[0])
	}
	if !near(b.V[2][0], 10) || !near(b.V[2][1], 0) {
		t.Errorf("end = %v; want (10, 0), fraction 0.25 of the perimeter", b.V[2])
	}
}

// SetMachine must drop the old machine's clip: with a GlobalState initial
// state, the abandoned player used to keep updating and completing the new
// machine's interactions.
func TestSetMachineDropsOldPlayer(t *testing.T) {
	b := NewBundle()
	if err := b.SetAnimation("swim", clipAnimation(4, "")); err != nil {
		t.Fatal(err)
	}
	for id, doc := range map[string]string{
		"a": `{"initial":"swim","states":[
		  {"name":"swim","type":"PlaybackState","animation":"swim","loop":true,"autoplay":true}]}`,
		"b": `{"initial":"any","states":[
		  {"name":"any","type":"GlobalState","transitions":[
		    {"type":"Transition","toState":"idle","guards":[{"type":"Event","inputName":"go"}]}]},
		  {"name":"idle","type":"PlaybackState","animation":"swim","autoplay":true}],
		  "inputs":[{"type":"Event","name":"go"}]}`,
	} {
		sm, err := ParseStateMachine([]byte(doc))
		if err != nil {
			t.Fatalf("machine %s: %v", id, err)
		}
		if err := b.SetStateMachine(id, sm); err != nil {
			t.Fatal(err)
		}
	}
	m, err := b.NewStateMachinePlayer("a")
	if err != nil {
		t.Fatal(err)
	}
	m.Update()
	if m.Player() == nil {
		t.Fatal("machine a is not playing its clip")
	}
	if err := m.SetMachine("b"); err != nil {
		t.Fatal(err)
	}
	if m.Player() != nil {
		t.Error("old machine's player survived SetMachine into a GlobalState initial")
	}
}
