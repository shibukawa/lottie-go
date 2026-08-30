package lottie

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"
	"testing"
)

// sceneTestBundle holds a plain clip and a button machine whose states
// mirror focus: normal -> focused on "focus", back on "blur", and
// "activate" moves to pressed.
func sceneTestBundle(t *testing.T) *Bundle {
	t.Helper()
	b := NewBundle()
	if err := b.SetAnimation("logo", clipAnimation(30, `[{"cm":"in","tm":0,"dr":10},{"cm":"out","tm":10,"dr":10}]`)); err != nil {
		t.Fatal(err)
	}
	if err := b.SetAnimation("btn-anim", clipAnimation(30, "")); err != nil {
		t.Fatal(err)
	}
	sm, err := ParseStateMachine([]byte(`{
		"initial": "normal",
		"inputs": [
			{"type": "Event", "name": "focus"},
			{"type": "Event", "name": "blur"},
			{"type": "Event", "name": "activate"},
			{"type": "Numeric", "name": "hp", "value": 100}
		],
		"states": [
			{"name": "normal", "type": "PlaybackState", "animation": "btn-anim", "loop": true, "autoplay": true,
			 "transitions": [{"type": "Transition", "toState": "focused", "guards": [{"type": "Event", "inputName": "focus"}]}]},
			{"name": "focused", "type": "PlaybackState", "animation": "btn-anim", "loop": true, "autoplay": true,
			 "transitions": [
				{"type": "Transition", "toState": "normal", "guards": [{"type": "Event", "inputName": "blur"}]},
				{"type": "Transition", "toState": "pressed", "guards": [{"type": "Event", "inputName": "activate"}]}]},
			{"name": "pressed", "type": "PlaybackState", "animation": "btn-anim", "loop": true, "autoplay": true}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := b.SetStateMachine("btn", sm); err != nil {
		t.Fatal(err)
	}
	return b
}

// menuScene is a 400x400 menu: a logo at the top and two focusable
// buttons stacked below it.
func menuScene(t *testing.T) (*Scene, *ScenePlayer) {
	t.Helper()
	s, err := ParseScene([]byte(`{
		"size": {"w": 400, "h": 400},
		"bundles": [{"alias": "ui", "path": "ui.lottie"}],
		"nodes": [
			{"name": "logo", "kind": "animation", "source": {"bundle": "ui", "id": "logo"},
			 "transform": {"x": 150, "y": 0},
			 "playback": {"loop": true, "autoplay": true}},
			{"name": "start", "kind": "machine", "source": {"bundle": "ui", "id": "btn"},
			 "transform": {"x": 50, "y": 150},
			 "focus": {"focusable": true, "tabIndex": 0},
			 "bindings": [{"on": "activate", "do": "callback", "arg": "start-game"}]},
			{"name": "quit", "kind": "machine", "source": {"bundle": "ui", "id": "btn"},
			 "transform": {"x": 50, "y": 280},
			 "focus": {"focusable": true, "tabIndex": 1}}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if errs := s.Validate(); len(errs) > 0 {
		t.Fatalf("valid scene reported %v", errs)
	}
	b := sceneTestBundle(t)
	sp, err := s.NewScenePlayer(func(path string) (*Bundle, error) {
		if path != "ui.lottie" {
			return nil, fmt.Errorf("unexpected path %q", path)
		}
		return b, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return s, sp
}

func nodeState(t *testing.T, sp *ScenePlayer, name string) string {
	t.Helper()
	n, ok := sp.Node(name)
	if !ok {
		t.Fatalf("no node %q", name)
	}
	return n.Machine().State()
}

func TestSceneRoundTripPreservesUnknownMembers(t *testing.T) {
	src := `{
		"size": {"w": 100, "h": 100},
		"x-tool": {"grid": 8},
		"nodes": [{"name": "a", "kind": "animation", "source": {"bundle": "ui", "id": "logo"},
			"x-note": "keep me"}]
	}`
	s, err := ParseScene([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := s.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{`"x-tool"`, `"grid": 8`, `"x-note"`, "keep me"} {
		if !strings.Contains(out, want) {
			t.Errorf("round-trip lost %s:\n%s", want, out)
		}
	}
	if !s.Options.hoverMovesFocus() {
		t.Error("absent hoverMovesFocus should resolve to true")
	}
}

func TestSceneValidate(t *testing.T) {
	s, err := ParseScene([]byte(`{
		"size": {"w": 0, "h": 100},
		"bundles": [{"alias": "ui", "path": "a.lottie"}, {"alias": "ui", "path": "b.lottie"}],
		"options": {"initialFocus": "ghost"},
		"nodes": [
			{"name": "a", "kind": "animation", "source": {"bundle": "ui", "id": "x"},
			 "focus": {"neighbors": {"down": "missing"}},
			 "bindings": [{"on": "activate", "do": "fireEvent", "arg": "go"}]},
			{"name": "a", "kind": "widget", "source": {"bundle": "nope", "id": "x"}},
			{"name": "c", "kind": "animation", "source": {"bundle": "nope", "id": "x"}}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	errs := s.Validate()
	for _, want := range []string{
		"no design size", "duplicate bundle alias", "duplicate node name",
		"unknown kind", "unknown bundle alias", "fireEvent but is not a machine",
		"neighbor \"missing\"", "initial focus \"ghost\"",
	} {
		found := false
		for _, e := range errs {
			if strings.Contains(e.Error(), want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing finding %q in %v", want, errs)
		}
	}
}

func TestScenePlayerInitialFocus(t *testing.T) {
	_, sp := menuScene(t)
	if got := sp.Focused(); got != "start" {
		t.Fatalf("initial focus = %q, want lowest tab index \"start\"", got)
	}
	// The focus event auto-fires because the machine declares one; it
	// becomes visible on the next Update.
	sp.Update()
	if got := nodeState(t, sp, "start"); got != "focused" {
		t.Errorf("focused node's machine in %q, want focused", got)
	}
	if got := nodeState(t, sp, "quit"); got != "normal" {
		t.Errorf("unfocused node's machine in %q, want normal", got)
	}
}

func TestScenePlayerTabAndDirectionalFocus(t *testing.T) {
	_, sp := menuScene(t)
	sp.Update()
	sp.MoveFocus(FocusNext)
	sp.Update()
	if got := sp.Focused(); got != "quit" {
		t.Fatalf("after Next focus = %q, want quit", got)
	}
	sp.MoveFocus(FocusNext) // wraps
	sp.Update()
	if got := sp.Focused(); got != "start" {
		t.Fatalf("after wrap focus = %q, want start", got)
	}
	sp.MoveFocus(FocusDown) // geometric: quit is straight below
	sp.Update()
	if got := sp.Focused(); got != "quit" {
		t.Fatalf("after Down focus = %q, want quit", got)
	}
	sp.MoveFocus(FocusDown) // nothing below: stays
	if got := sp.Focused(); got != "quit" {
		t.Fatalf("focus moved off the bottom to %q", got)
	}
	sp.Update()
	if got := nodeState(t, sp, "start"); got != "normal" {
		t.Errorf("blurred node's machine in %q, want normal", got)
	}
	if got := nodeState(t, sp, "quit"); got != "focused" {
		t.Errorf("focused node's machine in %q, want focused", got)
	}
}

func TestScenePlayerExplicitNeighborWins(t *testing.T) {
	s, _ := menuScene(t)
	// Point start's Down link back at itself: the geometric candidate
	// (quit) must not be used.
	n, _ := s.Node("start")
	n.Focus.Neighbors.Down = "start"
	b := sceneTestBundle(t)
	sp, err := s.NewScenePlayer(func(string) (*Bundle, error) { return b, nil })
	if err != nil {
		t.Fatal(err)
	}
	sp.MoveFocus(FocusDown)
	if got := sp.Focused(); got != "start" {
		t.Fatalf("explicit link ignored; focus = %q", got)
	}
}

func TestScenePlayerActivateCallbackReplacesDefault(t *testing.T) {
	_, sp := menuScene(t)
	var gotNode, gotName string
	sp.OnCallback(func(node, name string) { gotNode, gotName = node, name })
	sp.Update()
	sp.Activate()
	sp.Update()
	if gotNode != "start" || gotName != "start-game" {
		t.Errorf("callback = (%q, %q), want (start, start-game)", gotNode, gotName)
	}
	// The explicit binding replaces the default fireEvent, so the machine
	// stays in focused, not pressed.
	if got := nodeState(t, sp, "start"); got != "focused" {
		t.Errorf("machine in %q, want focused (default suppressed)", got)
	}
}

func TestScenePlayerActivateDefaultFiresMachine(t *testing.T) {
	_, sp := menuScene(t)
	sp.Focus("quit")
	sp.Update()
	sp.Activate() // quit has no bindings: default fires the machine's event
	sp.Update()
	if got := nodeState(t, sp, "quit"); got != "pressed" {
		t.Errorf("machine in %q, want pressed", got)
	}
}

func TestScenePlayerCancelFallsBackToSceneCallback(t *testing.T) {
	_, sp := menuScene(t)
	var gotNode, gotName string
	sp.OnCallback(func(node, name string) { gotNode, gotName = node, name })
	sp.Cancel() // focused node handles nothing named cancel
	if gotNode != "" || gotName != "cancel" {
		t.Errorf("callback = (%q, %q), want (\"\", cancel)", gotNode, gotName)
	}
}

func TestScenePlayerPointerHoverAndClick(t *testing.T) {
	_, sp := menuScene(t)
	sp.SetScreenMapping(800, 800, ScaleContain) // 400x400 design doubled
	var events []string
	sp.OnFocusChanged(func(from, to string) { events = append(events, from+">"+to) })
	sp.OnCallback(func(node, name string) { events = append(events, node+":"+name) })

	// quit spans (50,280)-(150,380) in scene space, doubled on screen.
	sp.Pointer(200, 660, false) // hover moves focus (default option)
	if got := sp.Focused(); got != "quit" {
		t.Fatalf("hover focus = %q, want quit", got)
	}
	sp.Pointer(200, 660, true)  // press
	sp.Pointer(200, 660, false) // release on the same node: activate
	sp.Update()
	if got := nodeState(t, sp, "quit"); got != "pressed" {
		t.Errorf("machine in %q, want pressed after click", got)
	}

	// Press then drag off before release: no activate.
	sp2 := func() *ScenePlayer { _, p := menuScene(t); return p }()
	sp2.SetScreenMapping(800, 800, ScaleContain)
	sp2.Pointer(200, 660, true)
	sp2.Pointer(10, 10, false)
	sp2.Update()
	if got := nodeState(t, sp2, "quit"); got == "pressed" {
		t.Error("drag off the node still activated it")
	}
	_ = events
}

func TestScenePlayerSetGet(t *testing.T) {
	_, sp := menuScene(t)
	n, _ := sp.Node("start")
	if v, ok := n.Get[float64]("hp"); !ok || v != 100 {
		t.Fatalf("declared value = %v %v, want 100 true", v, ok)
	}
	n.Set("hp", 25)
	if v, _ := n.Get[float64]("hp"); v != 25 {
		t.Errorf("after Set hp = %v, want 25", v)
	}
	logo, _ := sp.Node("logo")
	if _, ok := logo.Get[float64]("hp"); ok {
		t.Error("animation node reported a machine input")
	}
}

func TestScenePlayerPlaySegmentBinding(t *testing.T) {
	s, _ := menuScene(t)
	n, _ := s.Node("logo")
	n.Bindings = append(n.Bindings, SceneBinding{On: SceneHover, Do: ScenePlaySegment, Arg: "out"})
	b := sceneTestBundle(t)
	sp, err := s.NewScenePlayer(func(string) (*Bundle, error) { return b, nil })
	if err != nil {
		t.Fatal(err)
	}
	// logo spans (150,0)-(250,100); no mapping set, so scene coords pass through.
	sp.Pointer(200, 50, false)
	logo, _ := sp.Node("logo")
	p := logo.Player()
	if !p.IsPlaying() {
		t.Fatal("playSegment left the player paused")
	}
	if f := p.Frame(); f < 10 {
		t.Errorf("frame = %v, want inside the out segment (>=10)", f)
	}
}

func TestScenePlayerEntryOverride(t *testing.T) {
	s, _ := menuScene(t)
	n, _ := s.Node("quit")
	n.Entry = "pressed"
	b := sceneTestBundle(t)
	sp, err := s.NewScenePlayer(func(string) (*Bundle, error) { return b, nil })
	if err != nil {
		t.Fatal(err)
	}
	if got := nodeState(t, sp, "quit"); got != "pressed" {
		t.Errorf("entry override ignored; state = %q", got)
	}
}

func TestSceneNodeStartDelay(t *testing.T) {
	s, _ := menuScene(t)
	// quit enters half a second in; until then it is invisible, unhittable,
	// and outside the focus order.
	n, _ := s.Node("quit")
	n.Start = 0.5
	b := sceneTestBundle(t)
	sp, err := s.NewScenePlayer(func(string) (*Bundle, error) { return b, nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := sp.NodeAt(60, 290); ok {
		t.Error("unstarted node took a pointer hit")
	}
	sp.MoveFocus(FocusNext) // start -> wraps straight back to start
	if got := sp.Focused(); got != "start" {
		t.Fatalf("unstarted node entered tab order; focus = %q", got)
	}
	for range 31 { // 31 ticks at 60tps crosses 0.5s
		sp.Update()
	}
	if _, ok := sp.NodeAt(60, 290); !ok {
		t.Error("started node not hittable")
	}
	sp.MoveFocus(FocusNext)
	if got := sp.Focused(); got != "quit" {
		t.Errorf("after entrance focus = %q, want quit", got)
	}
}

func TestSceneRestartReplaysEntrances(t *testing.T) {
	s, _ := menuScene(t)
	n, _ := s.Node("logo")
	n.Start = 0.2
	b := sceneTestBundle(t)
	sp, err := s.NewScenePlayer(func(string) (*Bundle, error) { return b, nil })
	if err != nil {
		t.Fatal(err)
	}
	for range 20 {
		sp.Update()
	}
	if sp.Time() < 0.3 {
		t.Fatalf("clock = %v, want ~0.33", sp.Time())
	}
	logo, _ := sp.Node("logo")
	if !logo.started {
		t.Fatal("logo never entered")
	}
	sp.Restart()
	if sp.Time() != 0 || logo.started {
		t.Errorf("restart left time=%v started=%v", sp.Time(), logo.started)
	}
	if got := sp.Focused(); got != "start" {
		t.Errorf("restart focus = %q, want the initial choice", got)
	}
}

func TestSceneBindingTargetAndFocusAction(t *testing.T) {
	s, _ := menuScene(t)
	// Activating start plays the logo's "out" segment and moves focus to
	// quit — cross-node actions plus a focus move.
	n, _ := s.Node("start")
	n.Bindings = []SceneBinding{
		{On: SceneActivate, Do: ScenePlaySegment, Arg: "out", Target: "logo"},
		{On: SceneActivate, Do: SceneFocusAction, Arg: "quit"},
	}
	if errs := s.Validate(); len(errs) > 0 {
		t.Fatalf("valid bindings reported %v", errs)
	}
	b := sceneTestBundle(t)
	sp, err := s.NewScenePlayer(func(string) (*Bundle, error) { return b, nil })
	if err != nil {
		t.Fatal(err)
	}
	sp.Update()
	sp.Activate()
	logo, _ := sp.Node("logo")
	if f := logo.Player().Frame(); f < 10 {
		t.Errorf("logo frame = %v, want inside the out segment", f)
	}
	if got := sp.Focused(); got != "quit" {
		t.Errorf("focus = %q, want quit via the focus action", got)
	}
}

func TestSceneValidateBindingTargets(t *testing.T) {
	s, _ := menuScene(t)
	n, _ := s.Node("start")
	n.Bindings = []SceneBinding{
		{On: SceneActivate, Do: ScenePlaySegment, Arg: "x", Target: "ghost"},
		{On: SceneActivate, Do: SceneFireEvent, Arg: "go", Target: "logo"},
		{On: SceneActivate, Do: SceneFocusAction, Arg: "ghost"},
	}
	errs := s.Validate()
	for _, want := range []string{
		`target "ghost" does not exist`,
		`fireEvent target "logo" is not a machine node`,
		`focus action names unknown node "ghost"`,
	} {
		found := false
		for _, e := range errs {
			if strings.Contains(e.Error(), want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing finding %q in %v", want, errs)
		}
	}
}

// phasedScene is an intro that rolls into main after half a second: the
// logo belongs to the intro, the buttons to main, and a phaseless
// background plays through the switch.
func phasedScene(t *testing.T) *ScenePlayer {
	t.Helper()
	s, _ := menuScene(t)
	s.Phases = []ScenePhase{
		{Name: "intro", Duration: 0.5, Next: "main"},
		{Name: "main"},
	}
	logo, _ := s.Node("logo")
	logo.Phase = "intro"
	st, _ := s.Node("start")
	st.Phase = "main"
	q, _ := s.Node("quit")
	q.Phase = "main"
	// Not looped, so a frame that only moved forward across the switch
	// proves the node kept playing rather than restarting.
	s.Nodes = append(s.Nodes, SceneNode{
		Name: "bg", Kind: SceneNodeAnimation,
		Source:   SceneSource{Bundle: "ui", ID: "logo"},
		Playback: ScenePlayback{Autoplay: true},
	})
	if errs := s.Validate(); len(errs) > 0 {
		t.Fatalf("valid phased scene reported %v", errs)
	}
	b := sceneTestBundle(t)
	sp, err := s.NewScenePlayer(func(string) (*Bundle, error) { return b, nil })
	if err != nil {
		t.Fatal(err)
	}
	return sp
}

func TestScenePhasesAutoAdvance(t *testing.T) {
	sp := phasedScene(t)
	if got := sp.Phase(); got != "intro" {
		t.Fatalf("initial phase = %q, want intro", got)
	}
	// Main's buttons do not participate during the intro.
	if _, ok := sp.NodeAt(60, 160); ok {
		t.Error("main-phase node hittable during intro")
	}
	if got := sp.Focused(); got != "" {
		t.Fatalf("intro focused %q, want nothing focusable", got)
	}
	var ends, changes []string
	sp.OnPhaseEnd(func(p string) { ends = append(ends, p) })
	sp.OnPhaseChanged(func(from, to string) { changes = append(changes, from+">"+to) })

	bg, _ := sp.Node("bg")
	for range 20 {
		sp.Update()
	}
	bgFrame := bg.Player().Frame()
	for range 20 { // crosses 0.5s
		sp.Update()
	}
	if got := sp.Phase(); got != "main" {
		t.Fatalf("phase = %q, want auto-advanced main", got)
	}
	if len(ends) != 1 || ends[0] != "intro" {
		t.Errorf("phase ends = %v, want [intro]", ends)
	}
	if len(changes) != 1 || changes[0] != "intro>main" {
		t.Errorf("phase changes = %v, want [intro>main]", changes)
	}
	if got := sp.Focused(); got != "start" {
		t.Errorf("main focus = %q, want start", got)
	}
	// The phaseless background kept playing through the switch.
	if bg.Player().Frame() <= bgFrame {
		t.Error("phaseless node restarted on the phase switch")
	}
	// The intro's logo left with its phase.
	logo, _ := sp.Node("logo")
	if logo.Started() && sp.Phase() == "main" {
		if _, ok := sp.NodeAt(200, 50); ok {
			t.Error("intro node still hittable in main")
		}
	}
}

func TestScenePhaseActionAndReplay(t *testing.T) {
	sp := phasedScene(t)
	// Jump straight to main, then a binding sends us back to the intro.
	if !sp.SetPhase("main") {
		t.Fatal("SetPhase(main) refused")
	}
	st, _ := sp.Node("start")
	st.Definition().Bindings = []SceneBinding{{On: SceneActivate, Do: ScenePhaseAction, Arg: "intro"}}
	sp.Update()
	sp.Activate()
	if got := sp.Phase(); got != "intro" {
		t.Fatalf("phase = %q, want intro via the phase action", got)
	}
	if sp.Time() != 0 {
		t.Errorf("clock = %v, want 0 after the switch", sp.Time())
	}
}

func TestScenePlaybackChain(t *testing.T) {
	// The everyday pattern: the entrance segment plays once, then the
	// idle segment loops forever — no state machine needed.
	s, _ := menuScene(t)
	logo, _ := s.Node("logo")
	logo.Playback = ScenePlayback{
		Segment: "in", Autoplay: true,
		Then: []ScenePlayStep{{Segment: "out", Loop: true}},
	}
	logo.Bindings = []SceneBinding{{On: SceneComplete, Do: SceneCallback, Arg: "clip-done"}}
	if errs := s.Validate(); len(errs) > 0 {
		t.Fatalf("valid chain reported %v", errs)
	}
	b := sceneTestBundle(t)
	sp, err := s.NewScenePlayer(func(string) (*Bundle, error) { return b, nil })
	if err != nil {
		t.Fatal(err)
	}
	var completes int
	sp.OnCallback(func(node, name string) {
		if node == "logo" && name == "clip-done" {
			completes++
		}
	})
	n, _ := sp.Node("logo")
	for range 10 { // the "in" segment is frames [0,10) at 60fps
		sp.Update()
	}
	if completes != 1 {
		t.Fatalf("completes = %d after the entrance, want 1", completes)
	}
	if f := n.Player().Frame(); f < 10 {
		t.Fatalf("frame = %v, want inside the idle segment (>=10)", f)
	}
	if !n.Player().IsPlaying() {
		t.Fatal("chain left the player paused")
	}
	for range 25 { // the idle loop wraps at least once
		sp.Update()
	}
	if f := n.Player().Frame(); f < 10 || f >= 20 {
		t.Errorf("idle frame = %v, want looping inside [10,20)", f)
	}
	// A looping step never completes again.
	if completes != 1 {
		t.Errorf("completes = %d while idling, want still 1", completes)
	}
}

func TestSceneValidateChain(t *testing.T) {
	s, _ := menuScene(t)
	logo, _ := s.Node("logo")
	logo.Playback = ScenePlayback{
		Loop: true,
		Then: []ScenePlayStep{{Segment: "out", Loop: true}, {Segment: "in"}},
	}
	errs := s.Validate()
	for _, want := range []string{"loops its first clip", "step 1 loops"} {
		found := false
		for _, e := range errs {
			if strings.Contains(e.Error(), want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing finding %q in %v", want, errs)
		}
	}
}

const testFontPath = "examples/lottie/stopwatch/assets/LuckiestGuy-Regular.ttf"

// assetScene is one image node and one right-anchored text node.
func assetScene(t *testing.T) *ScenePlayer {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	font, err := os.ReadFile(testFontPath)
	if err != nil {
		t.Skipf("test font unavailable: %v", err)
	}
	files := map[string][]byte{"badge.png": buf.Bytes(), "ui.ttf": font}

	s, err := ParseScene([]byte(`{
		"size": {"w": 400, "h": 300},
		"images": [{"alias": "badge", "path": "badge.png"}],
		"fonts": [{"alias": "ui", "path": "ui.ttf"}],
		"nodes": [
			{"name": "badge", "kind": "image", "source": {"image": "badge"},
			 "transform": {"x": 10, "y": 10}},
			{"name": "score", "kind": "text",
			 "transform": {"x": 390, "y": 10},
			 "text": {"value": "0", "font": "ui", "size": 24, "align": "right",
			          "anchorX": "right", "color": "#ffcc00"}}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if errs := s.Validate(); len(errs) > 0 {
		t.Fatalf("valid asset scene reported %v", errs)
	}
	sp, err := s.NewScenePlayerWithLoader(SceneLoader{
		File: func(path string) ([]byte, error) {
			data, ok := files[path]
			if !ok {
				return nil, fmt.Errorf("no file %q", path)
			}
			return data, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return sp
}

func TestSceneImageAndTextNodes(t *testing.T) {
	sp := assetScene(t)
	badge, _ := sp.Node("badge")
	if _, _, w, h := badge.LocalRect(); w != 40 || h != 20 {
		t.Errorf("image rect = %vx%v, want 40x20", w, h)
	}
	if _, ok := sp.NodeAt(30, 20); !ok {
		t.Error("image node not hittable")
	}

	score, _ := sp.Node("score")
	x0, _, w, h := score.LocalRect()
	if w <= 0 || h <= 0 {
		t.Fatalf("text measured %vx%v", w, h)
	}
	// Right-anchored: the block extends left of the transform point.
	if x0 != -w {
		t.Errorf("anchor offset = %v, want -width %v", x0, -w)
	}
	// The game overwrites the content by name; the block grows.
	score.SetText("12345")
	if _, _, w2, _ := score.LocalRect(); w2 <= w {
		t.Errorf("width after SetText = %v, want > %v", w2, w)
	}
	if score.Text() != "12345" {
		t.Errorf("Text() = %q", score.Text())
	}
	// SetText survives a Restart: the value belongs to the game.
	sp.Restart()
	if score.Text() != "12345" {
		t.Error("Restart clobbered SetText")
	}
	badge.SetText("ignored")
	if badge.Text() != "" {
		t.Error("SetText leaked onto an image node")
	}
}

func TestSceneAssetValidation(t *testing.T) {
	s, err := ParseScene([]byte(`{
		"size": {"w": 100, "h": 100},
		"nodes": [
			{"name": "a", "kind": "image", "source": {"image": "ghost"}},
			{"name": "b", "kind": "text", "text": {"font": "ghost", "color": "#zzz"}}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	errs := s.Validate()
	for _, want := range []string{"unknown image alias", "unknown font alias", "bad color"} {
		found := false
		for _, e := range errs {
			if strings.Contains(e.Error(), want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing finding %q in %v", want, errs)
		}
	}
}

func TestSceneScreenMapping(t *testing.T) {
	s := &Scene{Size: SceneSize{W: 400, H: 200}}
	sp, err := s.NewScenePlayer(nil)
	if err != nil {
		t.Fatal(err)
	}
	check := func(mode ScaleMode, x, y, wantX, wantY float64) {
		t.Helper()
		g := sp.ScreenGeoM()
		gx, gy := g.Apply(x, y)
		if gx != wantX || gy != wantY {
			t.Errorf("mode %d: (%v,%v) -> (%v,%v), want (%v,%v)", mode, x, y, gx, gy, wantX, wantY)
		}
	}
	sp.SetScreenMapping(800, 800, ScaleContain) // scale 2, y letterboxed by 200
	check(ScaleContain, 0, 0, 0, 200)
	check(ScaleContain, 400, 200, 800, 600)
	sp.SetScreenMapping(800, 800, ScaleCover) // scale 4, x cropped by 400
	check(ScaleCover, 0, 0, -400, 0)
	sp.SetScreenMapping(800, 800, ScaleStretch)
	check(ScaleStretch, 400, 200, 800, 800)
	sp.SetScreenMapping(800, 800, ScaleCenter)
	check(ScaleCenter, 0, 0, 200, 300)
}
