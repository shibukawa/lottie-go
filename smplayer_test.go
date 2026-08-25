package lottie

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

// clipAnimation is a clip of the given length in frames at 60fps, so one
// Update advances exactly one frame.
func clipAnimation(frames int, markers string) []byte {
	m := ""
	if markers != "" {
		m = `,"markers":` + markers
	}
	return fmt.Appendf(nil, `{"v":"5.9.0","nm":"clip","fr":60,"ip":0,"op":%d,"w":100,"h":100,
		"layers":[{"ty":3,"nm":"null","ind":1,"ip":0,"op":%d,"st":0,
		"ks":{"a":{"a":0,"k":[0,0]},"p":{"a":0,"k":[50,50]},
		"s":{"a":0,"k":[100,100]},"r":{"a":0,"k":0},"o":{"a":0,"k":100}}}]`+m+`}`,
		frames, frames)
}

// newMachine builds a bundle from clip lengths and a state machine document,
// then starts it.
func newMachine(t *testing.T, clips map[string]int, doc string) *StateMachinePlayer {
	t.Helper()
	b := NewBundle()
	for id, frames := range clips {
		if err := b.SetAnimation(id, clipAnimation(frames, "")); err != nil {
			t.Fatal(err)
		}
	}
	sm, err := ParseStateMachine([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if err := b.SetStateMachine("m", sm); err != nil {
		t.Fatal(err)
	}
	m, err := b.NewStateMachinePlayer("m")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// updateUntil ticks until the machine reaches a state, returning how many
// ticks it took.
func updateUntil(t *testing.T, m *StateMachinePlayer, state string, limit int) int {
	t.Helper()
	for i := 1; i <= limit; i++ {
		m.Update()
		if m.State() == state {
			return i
		}
	}
	t.Fatalf("state %q not reached within %d ticks; stuck in %q", state, limit, m.State())
	return 0
}

const jumpDoc = `{
  "initial": "idle",
  "states": [
    {"name":"idle","type":"PlaybackState","animation":"idle","loop":true,"autoplay":true,
     "transitions":[{"type":"Transition","toState":"jump","guards":[{"type":"Event","inputName":"jump"}]}]},
    {"name":"jump","type":"PlaybackState","animation":"jump","autoplay":true,
     "transitions":[{"type":"Transition","toState":"idle","guards":[{"type":"Event","inputName":"jumpDone"}]}]}
  ],
  "inputs": [{"type":"Event","name":"jump"},{"type":"Event","name":"jumpDone"}],
  "interactions": [{"type":"OnComplete","actions":[{"type":"Fire","inputName":"jumpDone"}]}]
}`

// The headline case: a one-shot clip returns to idle by itself, with no
// game-side timer.
func TestOneShotReturnsToIdleOnComplete(t *testing.T) {
	m := newMachine(t, map[string]int{"idle": 10, "jump": 3}, jumpDoc)
	if m.State() != "idle" {
		t.Fatalf("initial state = %q; want idle", m.State())
	}
	m.Fire("jump")
	m.Update()
	if m.State() != "jump" {
		t.Fatalf("after Fire+Update state = %q; want jump", m.State())
	}
	// The jump clip is 3 frames, so it ends on the third tick after entry.
	if got := updateUntil(t, m, "idle", 5); got != 3 {
		t.Errorf("returned to idle after %d ticks; want 3", got)
	}
	if !m.player.IsPlaying() {
		t.Error("idle is a looping state and should be playing")
	}
}

// Fire is visible to the very next Update, wherever in the frame it is
// called from.
func TestFireIsVisibleToNextUpdate(t *testing.T) {
	m := newMachine(t, map[string]int{"idle": 10, "jump": 3}, jumpDoc)
	m.Fire("jump")
	if m.State() != "idle" {
		t.Error("Fire moved the machine before Update")
	}
	m.Update()
	if m.State() != "jump" {
		t.Errorf("state = %q; want jump", m.State())
	}
}

// An unmatched event lasts exactly one Update, so a stale trigger cannot
// fire later.
func TestEventLastsOneUpdate(t *testing.T) {
	m := newMachine(t, map[string]int{"idle": 10, "jump": 3}, jumpDoc)
	m.Fire("jumpDone") // nothing in idle reacts to this
	m.Update()
	m.Fire("jump")
	m.Update()
	if m.State() != "jump" {
		t.Fatalf("state = %q; want jump", m.State())
	}
	// If jumpDone had lingered, jump would have been left immediately.
	m.Update()
	if m.State() != "jump" {
		t.Errorf("a stale event fired a later transition; state = %q", m.State())
	}
}

// One event may take only one transition, so a single Fire cannot cascade
// through a chain of states that all guard on it.
func TestEventIsConsumedByItsTransition(t *testing.T) {
	m := newMachine(t, map[string]int{"a": 10, "b": 10, "c": 10}, `{
	  "initial":"a",
	  "states":[
	    {"name":"a","type":"PlaybackState","animation":"a","loop":true,"autoplay":true,
	     "transitions":[{"type":"Transition","toState":"b","guards":[{"type":"Event","inputName":"go"}]}]},
	    {"name":"b","type":"PlaybackState","animation":"b","loop":true,"autoplay":true,
	     "transitions":[{"type":"Transition","toState":"c","guards":[{"type":"Event","inputName":"go"}]}]},
	    {"name":"c","type":"PlaybackState","animation":"c","loop":true,"autoplay":true}],
	  "inputs":[{"type":"Event","name":"go"}]}`)
	m.Fire("go")
	m.Update()
	if m.State() != "b" {
		t.Errorf("state = %q; want b (one event, one transition)", m.State())
	}
	m.Fire("go")
	m.Update()
	if m.State() != "c" {
		t.Errorf("state = %q; want c", m.State())
	}
}

// Transitions are evaluated in declaration order and the first whose guards
// pass wins, which is why the editor must keep that order editable.
func TestFirstMatchingTransitionWins(t *testing.T) {
	doc := `{"initial":"a","states":[
	  {"name":"a","type":"PlaybackState","animation":"a","loop":true,"autoplay":true,
	   "transitions":[
	     {"type":"Transition","toState":"%s","guards":[{"type":"Event","inputName":"go"}]},
	     {"type":"Transition","toState":"%s","guards":[{"type":"Event","inputName":"go"}]}]},
	  {"name":"b","type":"PlaybackState","animation":"b","loop":true,"autoplay":true},
	  {"name":"c","type":"PlaybackState","animation":"c","loop":true,"autoplay":true}],
	  "inputs":[{"type":"Event","name":"go"}]}`
	clips := map[string]int{"a": 10, "b": 10, "c": 10}

	m := newMachine(t, clips, fmt.Sprintf(doc, "b", "c"))
	m.Fire("go")
	m.Update()
	if m.State() != "b" {
		t.Errorf("state = %q; want b (declared first)", m.State())
	}
	// Swapping the declaration order swaps the winner.
	m2 := newMachine(t, clips, fmt.Sprintf(doc, "c", "b"))
	m2.Fire("go")
	m2.Update()
	if m2.State() != "c" {
		t.Errorf("state = %q; want c (declared first)", m2.State())
	}
}

// A global state's transitions apply from anywhere, which is how "take
// damage from any state" is expressed.
func TestGlobalStateTransitionsApplyFromAnywhere(t *testing.T) {
	m := newMachine(t, map[string]int{"idle": 10, "walk": 10, "hurt": 3}, `{
	  "initial":"idle",
	  "states":[
	    {"name":"idle","type":"PlaybackState","animation":"idle","loop":true,"autoplay":true,
	     "transitions":[{"type":"Transition","toState":"walk","guards":[{"type":"Event","inputName":"walk"}]}]},
	    {"name":"walk","type":"PlaybackState","animation":"walk","loop":true,"autoplay":true},
	    {"name":"any","type":"GlobalState",
	     "transitions":[{"type":"Transition","toState":"hurt","guards":[{"type":"Event","inputName":"hurt"}]}]},
	    {"name":"hurt","type":"PlaybackState","animation":"hurt","autoplay":true}],
	  "inputs":[{"type":"Event","name":"walk"},{"type":"Event","name":"hurt"}]}`)
	m.Fire("walk")
	m.Update()
	if m.State() != "walk" {
		t.Fatalf("state = %q; want walk", m.State())
	}
	m.Fire("hurt")
	m.Update()
	if m.State() != "hurt" {
		t.Errorf("global transition did not apply; state = %q", m.State())
	}
}

// A state's own transitions are more specific than a global state's, so they
// are considered first.
func TestOwnTransitionBeatsGlobal(t *testing.T) {
	m := newMachine(t, map[string]int{"a": 10, "own": 10, "global": 10}, `{
	  "initial":"a",
	  "states":[
	    {"name":"a","type":"PlaybackState","animation":"a","loop":true,"autoplay":true,
	     "transitions":[{"type":"Transition","toState":"own","guards":[{"type":"Event","inputName":"go"}]}]},
	    {"name":"any","type":"GlobalState",
	     "transitions":[{"type":"Transition","toState":"global","guards":[{"type":"Event","inputName":"go"}]}]},
	    {"name":"own","type":"PlaybackState","animation":"own","loop":true,"autoplay":true},
	    {"name":"global","type":"PlaybackState","animation":"global","loop":true,"autoplay":true}],
	  "inputs":[{"type":"Event","name":"go"}]}`)
	m.Fire("go")
	m.Update()
	if m.State() != "own" {
		t.Errorf("state = %q; want own", m.State())
	}
}

// An initial global state is resolved before the player is handed back, so
// the caller never sees a state that draws nothing.
func TestInitialGlobalStateSettles(t *testing.T) {
	m := newMachine(t, map[string]int{"idle": 10}, `{
	  "initial":"boot",
	  "states":[
	    {"name":"boot","type":"GlobalState","transitions":[{"type":"Transition","toState":"idle"}]},
	    {"name":"idle","type":"PlaybackState","animation":"idle","loop":true,"autoplay":true}]}`)
	if m.State() != "idle" {
		t.Errorf("state = %q; want idle", m.State())
	}
	if m.Player() == nil {
		t.Error("no clip is playing after start")
	}
}

func TestNumericAndBooleanGuards(t *testing.T) {
	doc := `{"initial":"idle","states":[
	  {"name":"idle","type":"PlaybackState","animation":"idle","loop":true,"autoplay":true,
	   "transitions":[{"type":"Transition","toState":"run","guards":[
	     {"type":"Numeric","inputName":"speed","conditionType":"GreaterThan","compareTo":5},
	     {"type":"Boolean","inputName":"grounded","conditionType":"Equal","compareTo":true}]}]},
	  {"name":"run","type":"PlaybackState","animation":"run","loop":true,"autoplay":true}],
	  "inputs":[{"type":"Numeric","name":"speed","value":0},{"type":"Boolean","name":"grounded","value":false}]}`
	clips := map[string]int{"idle": 10, "run": 10}

	// Every guard must pass, so one alone does nothing.
	m := newMachine(t, clips, doc)
	m.Set("speed", 9)
	m.Update()
	if m.State() != "idle" {
		t.Errorf("transitioned with only one guard satisfied; state = %q", m.State())
	}
	m.Set("grounded", true)
	m.Update()
	if m.State() != "run" {
		t.Errorf("state = %q; want run", m.State())
	}
	// Value guards hold as long as the value does, unlike events.
	if got, _ := m.Get[float64]("speed"); got != 9 {
		t.Errorf("Get[float64](speed) = %v; want 9", got)
	}
	if got, _ := m.Get[bool]("grounded"); !got {
		t.Error("Get[bool](grounded) = false; want true")
	}
}

func TestNumericConditions(t *testing.T) {
	for _, tt := range []struct {
		cond ConditionType
		got  float64
		want bool
	}{
		{ConditionEqual, 5, true},
		{ConditionEqual, 6, false},
		{ConditionNotEqual, 6, true},
		{ConditionGreaterThan, 6, true},
		{ConditionGreaterThan, 5, false},
		{ConditionGreaterThanOrEqual, 5, true},
		{ConditionLessThan, 4, true},
		{ConditionLessThan, 5, false},
		{ConditionLessThanOrEqual, 5, true},
		{"", 5, true}, // an absent condition means Equal
	} {
		t.Run(fmt.Sprintf("%s_%v", tt.cond, tt.got), func(t *testing.T) {
			if got := compareNumbers(tt.cond, tt.got, 5); got != tt.want {
				t.Errorf("compareNumbers(%q, %v, 5) = %v; want %v", tt.cond, tt.got, got, tt.want)
			}
		})
	}
}

func TestStringGuard(t *testing.T) {
	m := newMachine(t, map[string]int{"idle": 10, "fire": 10}, `{
	  "initial":"idle","states":[
	  {"name":"idle","type":"PlaybackState","animation":"idle","loop":true,"autoplay":true,
	   "transitions":[{"type":"Transition","toState":"fire","guards":[
	     {"type":"String","inputName":"weapon","conditionType":"Equal","compareTo":"bow"}]}]},
	  {"name":"fire","type":"PlaybackState","animation":"fire","loop":true,"autoplay":true}],
	  "inputs":[{"type":"String","name":"weapon","value":"sword"}]}`)
	m.Update()
	if m.State() != "idle" {
		t.Fatalf("state = %q; want idle", m.State())
	}
	m.Set("weapon", "bow")
	m.Update()
	if m.State() != "fire" {
		t.Errorf("state = %q; want fire", m.State())
	}
	if got, _ := m.Get[string]("weapon"); got != "bow" {
		t.Errorf("Get[string](weapon) = %q; want bow", got)
	}
}

// segment lets several clips live in one animation, addressed by marker.
func TestSegmentPlayback(t *testing.T) {
	b := NewBundle()
	if err := b.SetAnimation("sheet", clipAnimation(90,
		`[{"tm":0,"cm":"idle","dr":30},{"tm":30,"cm":"walk","dr":30},{"tm":60,"cm":"jump","dr":30}]`)); err != nil {
		t.Fatal(err)
	}
	sm, err := ParseStateMachine([]byte(`{"initial":"idle","states":[
	  {"name":"idle","type":"PlaybackState","animation":"sheet","segment":"idle","loop":true,"autoplay":true,
	   "transitions":[{"type":"Transition","toState":"jump","guards":[{"type":"Event","inputName":"jump"}]}]},
	  {"name":"jump","type":"PlaybackState","animation":"sheet","segment":"jump","autoplay":true}],
	  "inputs":[{"type":"Event","name":"jump"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := b.SetStateMachine("m", sm); err != nil {
		t.Fatal(err)
	}
	m, err := b.NewStateMachinePlayer("m")
	if err != nil {
		t.Fatal(err)
	}
	if start, end := m.Player().Range(); start != 0 || end != 30 {
		t.Errorf("idle range = [%v,%v); want [0,30)", start, end)
	}
	m.Fire("jump")
	m.Update()
	if m.State() != "jump" {
		t.Fatalf("state = %q; want jump", m.State())
	}
	if start, end := m.Player().Range(); start != 60 || end != 90 {
		t.Errorf("jump range = [%v,%v); want [60,90)", start, end)
	}
	// Playback starts at the segment, not the document origin.
	if got := m.Player().Frame(); got != 60 {
		t.Errorf("jump start frame = %v; want 60", got)
	}
	m.Update()
	if got := m.Player().Frame(); got != 61 {
		t.Errorf("frame after one tick = %v; want 61", got)
	}
}

func TestUnknownSegmentIsReported(t *testing.T) {
	m := newMachine(t, map[string]int{"idle": 10}, `{"initial":"idle","states":[
	  {"name":"idle","type":"PlaybackState","animation":"idle","segment":"nope","loop":true,"autoplay":true}]}`)
	if got := m.UnsupportedFeatures(); !slices.Contains(got, `unknown marker "nope" in animation "idle"`) {
		t.Errorf("UnsupportedFeatures() = %v; want an unknown marker note", got)
	}
	// The clip still plays, over its whole length.
	if start, end := m.Player().Range(); start != 0 || end != 10 {
		t.Errorf("range = [%v,%v); want the whole clip", start, end)
	}
}

func TestReverseMode(t *testing.T) {
	m := newMachine(t, map[string]int{"idle": 10, "close": 3}, `{
	  "initial":"idle","states":[
	  {"name":"idle","type":"PlaybackState","animation":"idle","loop":true,"autoplay":true,
	   "transitions":[{"type":"Transition","toState":"close","guards":[{"type":"Event","inputName":"close"}]}]},
	  {"name":"close","type":"PlaybackState","animation":"close","mode":"Reverse","autoplay":true}],
	  "inputs":[{"type":"Event","name":"close"}]}`)
	m.Fire("close")
	m.Update()
	if !m.Player().IsReverse() {
		t.Error("mode Reverse did not reach the player")
	}
	// Reverse playback starts at the end of the range.
	if got := m.Player().Frame(); got != 3 {
		t.Errorf("start frame = %v; want 3", got)
	}
	m.Update()
	if got := m.Player().Frame(); got != 2 {
		t.Errorf("frame after one tick = %v; want 2", got)
	}
}

func TestLoopCountStopsAfterNPasses(t *testing.T) {
	anim, err := Decode(bytes.NewReader(clipAnimation(2, "")))
	if err != nil {
		t.Fatal(err)
	}
	p := anim.NewPlayer()
	p.SetLoop(true)
	p.SetLoopCount(2)
	loops, done := 0, 0
	p.OnLoopComplete(func() { loops++ })
	p.OnComplete(func() { done++ })
	for range 10 {
		p.Update()
	}
	if loops != 2 {
		t.Errorf("loop completions = %d; want 2", loops)
	}
	if done != 1 {
		t.Errorf("completions = %d; want 1", done)
	}
	if p.IsPlaying() {
		t.Error("player kept running past its loop count")
	}
}

func TestUnlimitedLoopNeverCompletes(t *testing.T) {
	anim, err := Decode(bytes.NewReader(clipAnimation(2, "")))
	if err != nil {
		t.Fatal(err)
	}
	p := anim.NewPlayer()
	p.SetLoop(true)
	done := 0
	p.OnComplete(func() { done++ })
	for range 20 {
		p.Update()
	}
	if done != 0 {
		t.Errorf("an unlimited loop completed %d times; want 0", done)
	}
	if !p.IsPlaying() {
		t.Error("an unlimited loop stopped")
	}
}

func TestEntryAndExitActions(t *testing.T) {
	m := newMachine(t, map[string]int{"a": 10, "b": 10}, `{
	  "initial":"a","states":[
	  {"name":"a","type":"PlaybackState","animation":"a","loop":true,"autoplay":true,
	   "exitActions":[{"type":"Increment","inputName":"exits","value":2}],
	   "transitions":[{"type":"Transition","toState":"b","guards":[{"type":"Event","inputName":"go"}]}]},
	  {"name":"b","type":"PlaybackState","animation":"b","loop":true,"autoplay":true,
	   "entryActions":[{"type":"Toggle","inputName":"entered"},
	                   {"type":"Decrement","inputName":"hp"}]}],
	  "inputs":[{"type":"Event","name":"go"},{"type":"Numeric","name":"exits","value":0},
	            {"type":"Boolean","name":"entered","value":false},{"type":"Numeric","name":"hp","value":3}]}`)
	m.Fire("go")
	m.Update()
	if got, _ := m.Get[float64]("exits"); got != 2 {
		t.Errorf("exits = %v; want 2", got)
	}
	if got, _ := m.Get[bool]("entered"); !got {
		t.Error("entry Toggle did not run")
	}
	if got, _ := m.Get[float64]("hp"); got != 2 {
		t.Errorf("hp = %v; want 2 (Decrement defaults to 1)", got)
	}
}

func TestOnStateChanged(t *testing.T) {
	m := newMachine(t, map[string]int{"idle": 10, "jump": 3}, jumpDoc)
	var moves []string
	m.OnStateChanged(func(from, to string) {
		moves = append(moves, from+"->"+to)
	})
	m.Fire("jump")
	m.Update()
	updateUntil(t, m, "idle", 5)
	want := []string{"idle->jump", "jump->idle"}
	if !slices.Equal(moves, want) {
		t.Errorf("moves = %v; want %v", moves, want)
	}
}

func TestMissingAnimationIsReported(t *testing.T) {
	b := NewBundle()
	if err := b.SetAnimation("idle", clipAnimation(10, "")); err != nil {
		t.Fatal(err)
	}
	sm, err := ParseStateMachine([]byte(`{"initial":"idle","states":[
	  {"name":"idle","type":"PlaybackState","animation":"idle","loop":true,"autoplay":true,
	   "transitions":[{"type":"Transition","toState":"ghost","guards":[{"type":"Event","inputName":"go"}]}]},
	  {"name":"ghost","type":"PlaybackState","animation":"missing","autoplay":true}],
	  "inputs":[{"type":"Event","name":"go"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := b.SetStateMachine("m", sm); err != nil {
		t.Fatal(err)
	}
	m, err := b.NewStateMachinePlayer("m")
	if err != nil {
		t.Fatal(err)
	}
	m.Fire("go")
	m.Update()
	if m.Err() == nil {
		t.Fatal("Err() = nil; want a missing animation error")
	}
	if !strings.Contains(m.Err().Error(), "missing") {
		t.Errorf("Err() = %v; want it to name the missing animation", m.Err())
	}
	// The previous clip stays on screen rather than the player going blank.
	if m.Player() == nil {
		t.Error("Player() = nil after a failed state entry")
	}
}

// A cycle of unconditional transitions must not hang the game loop.
func TestTransitionCycleIsBounded(t *testing.T) {
	m := newMachine(t, map[string]int{"a": 10, "b": 10}, `{
	  "initial":"a","states":[
	  {"name":"a","type":"PlaybackState","animation":"a","loop":true,"autoplay":true,
	   "transitions":[{"type":"Transition","toState":"b"}]},
	  {"name":"b","type":"PlaybackState","animation":"b","loop":true,"autoplay":true,
	   "transitions":[{"type":"Transition","toState":"a"}]}]}`)
	m.Update()
	found := false
	for _, f := range m.UnsupportedFeatures() {
		if strings.Contains(f, "transitions in one update") {
			found = true
		}
	}
	if !found {
		t.Errorf("UnsupportedFeatures() = %v; want a transition cap note", m.UnsupportedFeatures())
	}
}

func TestPointerInteractionsAreReportedNotRun(t *testing.T) {
	m := newMachine(t, map[string]int{"idle": 10, "hover": 10}, `{
	  "initial":"idle","states":[
	  {"name":"idle","type":"PlaybackState","animation":"idle","loop":true,"autoplay":true,
	   "transitions":[{"type":"Transition","toState":"hover","guards":[{"type":"Event","inputName":"enter"}]}]},
	  {"name":"hover","type":"PlaybackState","animation":"hover","loop":true,"autoplay":true}],
	  "inputs":[{"type":"Event","name":"enter"}],
	  "interactions":[{"type":"PointerEnter","actions":[{"type":"Fire","inputName":"enter"}]}]}`)
	for range 5 {
		m.Update()
	}
	if m.State() != "idle" {
		t.Errorf("a pointer interaction ran; state = %q", m.State())
	}
	if got := m.UnsupportedFeatures(); !slices.Contains(got, "interaction PointerEnter") {
		t.Errorf("UnsupportedFeatures() = %v; want interaction PointerEnter", got)
	}
}

func TestAutoplayFalseHoldsFirstFrame(t *testing.T) {
	m := newMachine(t, map[string]int{"idle": 10}, `{"initial":"idle","states":[
	  {"name":"idle","type":"PlaybackState","animation":"idle"}]}`)
	if m.Player().IsPlaying() {
		t.Error("autoplay is absent, so the clip should be paused")
	}
	m.Update()
	if got := m.Player().Frame(); got != 0 {
		t.Errorf("a paused clip advanced to %v", got)
	}
}

func TestDefinitionAndStateAccessors(t *testing.T) {
	m := newMachine(t, map[string]int{"idle": 10, "jump": 3}, jumpDoc)
	if m.Definition() == nil || m.Definition().Initial != "idle" {
		t.Error("Definition() did not return the document")
	}
	if m.State() != "idle" {
		t.Errorf("State() = %q; want idle", m.State())
	}
}

func TestNewStateMachinePlayerErrors(t *testing.T) {
	b := NewBundle()
	if err := b.SetAnimation("idle", clipAnimation(10, "")); err != nil {
		t.Fatal(err)
	}
	if _, err := b.NewStateMachinePlayer("nope"); err == nil {
		t.Error("started a state machine that does not exist")
	}
	if err := b.SetStateMachine("empty", &StateMachine{}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.NewStateMachinePlayer("empty"); err == nil {
		t.Error("started a state machine with no initial state")
	}
}

func TestSetFrameAndSetProgressActions(t *testing.T) {
	m := newMachine(t, map[string]int{"a": 100, "b": 100}, `{
	  "initial":"a","states":[
	  {"name":"a","type":"PlaybackState","animation":"a","loop":true,"autoplay":true,
	   "transitions":[{"type":"Transition","toState":"b","guards":[{"type":"Event","inputName":"go"}]}]},
	  {"name":"b","type":"PlaybackState","animation":"b","loop":true,"autoplay":true,
	   "entryActions":[{"type":"SetFrame","value":40}]}],
	  "inputs":[{"type":"Event","name":"go"}]}`)
	m.Fire("go")
	m.Update()
	if got := m.Player().Frame(); got != 40 {
		t.Errorf("SetFrame left the cursor at %v; want 40", got)
	}
	if got := m.Player().Progress(); got != 0.4 {
		t.Errorf("Progress() = %v; want 0.4", got)
	}
	m.Player().SetProgress(0.25)
	if got := m.Player().Frame(); got != 25 {
		t.Errorf("SetProgress(0.25) left the cursor at %v; want 25", got)
	}
}

func TestSetProgressAndFrameRespectSegment(t *testing.T) {
	anim, err := Decode(bytes.NewReader(clipAnimation(100, `[{"tm":40,"cm":"mid","dr":20}]`)))
	if err != nil {
		t.Fatal(err)
	}
	p := anim.NewPlayer()
	if !p.SetMarkerRange("mid") {
		t.Fatal("SetMarkerRange(mid) failed")
	}
	p.SetProgress(0.5)
	if got := p.Frame(); got != 50 {
		t.Errorf("SetProgress(0.5) within [40,60) = %v; want 50", got)
	}
	if got := p.Progress(); got != 0.5 {
		t.Errorf("Progress() = %v; want 0.5", got)
	}
	// A frame outside the segment is pulled back into it.
	p.SetFrame(10)
	if got := p.Frame(); got != 40 {
		t.Errorf("SetFrame(10) within [40,60) = %v; want 40", got)
	}
	// Duration follows the segment, not the whole document.
	if got := p.Duration(); got != 20*time.Second/60 {
		t.Errorf("segment Duration() = %v; want 20 frames at 60fps", got)
	}
	p.ClearRange()
	if start, end := p.Range(); start != 0 || end != 100 {
		t.Errorf("after ClearRange, range = [%v,%v); want [0,100)", start, end)
	}
	if got := p.Duration(); got != 100*time.Second/60 {
		t.Errorf("full Duration() = %v; want 100 frames at 60fps", got)
	}
}

func TestNotEqualConditions(t *testing.T) {
	if !compareEquality(ConditionNotEqual, false) {
		t.Error("NotEqual on unequal values should pass")
	}
	if compareEquality(ConditionNotEqual, true) {
		t.Error("NotEqual on equal values should fail")
	}
	if !compareEquality("", true) {
		t.Error("an absent condition should mean Equal")
	}
	if compareEquality("Bogus", true) {
		t.Error("an unknown condition should not pass")
	}
}

// A guard naming an input that was never set cannot pass, and must not
// panic or match by accident.
func TestGuardOnUnsetInput(t *testing.T) {
	m := newMachine(t, map[string]int{"idle": 10, "run": 10}, `{
	  "initial":"idle","states":[
	  {"name":"idle","type":"PlaybackState","animation":"idle","loop":true,"autoplay":true,
	   "transitions":[{"type":"Transition","toState":"run","guards":[
	     {"type":"Numeric","inputName":"never","conditionType":"GreaterThan","compareTo":0}]}]},
	  {"name":"run","type":"PlaybackState","animation":"run","loop":true,"autoplay":true}]}`)
	for range 5 {
		m.Update()
	}
	if m.State() != "idle" {
		t.Errorf("a guard on an unset input passed; state = %q", m.State())
	}
	// Setting it lets the guard through.
	m.Set("never", 1)
	m.Update()
	if m.State() != "run" {
		t.Errorf("state = %q; want run", m.State())
	}
}

func TestUnknownGuardTypeIsReported(t *testing.T) {
	m := newMachine(t, map[string]int{"idle": 10}, `{"initial":"idle","states":[
	  {"name":"idle","type":"PlaybackState","animation":"idle","loop":true,"autoplay":true,
	   "transitions":[{"type":"Transition","toState":"idle","guards":[{"type":"Telepathy","inputName":"x"}]}]}]}`)
	m.Update()
	if got := m.UnsupportedFeatures(); !slices.Contains(got, "guard Telepathy") {
		t.Errorf("UnsupportedFeatures() = %v; want guard Telepathy", got)
	}
}

func TestUnknownActionIsReported(t *testing.T) {
	m := newMachine(t, map[string]int{"idle": 10}, `{"initial":"idle","states":[
	  {"name":"idle","type":"PlaybackState","animation":"idle","loop":true,"autoplay":true,
	   "entryActions":[{"type":"LaunchRocket"}]}]}`)
	if got := m.UnsupportedFeatures(); !slices.Contains(got, "action LaunchRocket") {
		t.Errorf("UnsupportedFeatures() = %v; want action LaunchRocket", got)
	}
}

func TestDrawDoesNotPanicWithoutAClip(t *testing.T) {
	// A machine parked in a global state has nothing to draw; Draw must be
	// a no-op rather than a crash.
	m := newMachine(t, map[string]int{"idle": 10}, `{"initial":"wait","states":[
	  {"name":"wait","type":"GlobalState",
	   "transitions":[{"type":"Transition","toState":"idle","guards":[{"type":"Event","inputName":"go"}]}]},
	  {"name":"idle","type":"PlaybackState","animation":"idle","loop":true,"autoplay":true}],
	  "inputs":[{"type":"Event","name":"go"}]}`)
	if m.Player() != nil {
		t.Error("a global state should not start a clip")
	}
	m.Draw(nil, nil)
	m.Update()
}

// Set accepts whatever numeric type the game already has, including an
// untyped constant, which is why the constraint is wide rather than just
// bool|float64|string.
func TestSetAcceptsAnyNumericType(t *testing.T) {
	m := newMachine(t, map[string]int{"idle": 10, "run": 10}, `{
	  "initial":"idle","states":[
	  {"name":"idle","type":"PlaybackState","animation":"idle","loop":true,"autoplay":true,
	   "transitions":[{"type":"Transition","toState":"run","guards":[
	     {"type":"Numeric","inputName":"speed","conditionType":"GreaterThanOrEqual","compareTo":10}]}]},
	  {"name":"run","type":"PlaybackState","animation":"run","loop":true,"autoplay":true}],
	  "inputs":[{"type":"Numeric","name":"speed","value":0}]}`)

	m.Set("speed", 5) // untyped constant: defaults to int
	m.Update()
	if m.State() != "idle" {
		t.Fatalf("state = %q; want idle", m.State())
	}
	var speed float32 = 12.5
	m.Set("speed", speed)
	m.Update()
	if m.State() != "run" {
		t.Errorf("state = %q; want run", m.State())
	}
	// A float32 reads back through the numeric guard's own type.
	if got, ok := m.Get[float64]("speed"); !ok || got != 12.5 {
		t.Errorf("Get[float64](speed) = %v, %v; want 12.5, true", got, ok)
	}
	// Named types satisfy the constraint through their underlying type.
	type Level int
	m.Set("level", Level(3))
	if got, ok := m.Get[int]("level"); !ok || got != 3 {
		t.Errorf("Get[int](level) = %v, %v; want 3, true", got, ok)
	}
}

func TestGetRejectsMismatchedType(t *testing.T) {
	m := newMachine(t, map[string]int{"idle": 10}, `{"initial":"idle","states":[
	  {"name":"idle","type":"PlaybackState","animation":"idle","loop":true,"autoplay":true}]}`)
	m.Set("weapon", "bow")
	if _, ok := m.Get[float64]("weapon"); ok {
		t.Error("Get[float64] accepted a string input")
	}
	if got, ok := m.Get[string]("weapon"); !ok || got != "bow" {
		t.Errorf("Get[string](weapon) = %q, %v; want bow, true", got, ok)
	}
	// An input that was never set reports false and the zero value.
	if got, ok := m.Get[float64]("nothing"); ok || got != 0 {
		t.Errorf("Get[float64](nothing) = %v, %v; want 0, false", got, ok)
	}
	// A bool is not a number, even though JSON would happily hold either.
	m.Set("grounded", true)
	if _, ok := m.Get[float64]("grounded"); ok {
		t.Error("Get[float64] accepted a bool input")
	}
}
