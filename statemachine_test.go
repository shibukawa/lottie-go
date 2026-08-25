package lottie

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestParseStateMachine(t *testing.T) {
	sm, err := ParseStateMachine([]byte(walkJumpMachine))
	if err != nil {
		t.Fatal(err)
	}
	if sm.Initial != "idle" {
		t.Errorf("Initial = %q; want idle", sm.Initial)
	}
	idle, ok := sm.State("idle")
	if !ok {
		t.Fatal("State(idle) not found")
	}
	if idle.Type != StatePlayback || idle.Animation != "idle" || !idle.Loop {
		t.Errorf("idle = %+v", idle)
	}
	if got := idle.Transitions[0].Guards[0]; got.Type != GuardEvent || got.InputName != "jump" {
		t.Errorf("idle guard = %+v", got)
	}
	if _, ok := sm.Input("jumpDone"); !ok {
		t.Error("Input(jumpDone) not found")
	}
	if len(sm.Interactions) != 1 || sm.Interactions[0].Type != InteractionOnComplete {
		t.Errorf("interactions = %+v", sm.Interactions)
	}
	if got := sm.Interactions[0].Actions[0]; got.Type != ActionFire || got.InputName != "jumpDone" {
		t.Errorf("OnComplete action = %+v", got)
	}
}

// A bundle edited here must stay valid for other dotLottie runtimes, so
// members this package does not model have to survive a rewrite at every
// level of the document.
func TestStateMachinePreservesUnknownMembers(t *testing.T) {
	const src = `{
	  "initial": "idle",
	  "futureTopLevel": {"keep": true},
	  "states": [
	    {"name":"idle","type":"PlaybackState","animation":"idle","futureState":7,
	     "transitions":[{"type":"Transition","toState":"idle","futureTransition":"x",
	       "guards":[{"type":"Numeric","inputName":"speed","conditionType":"GreaterThan",
	                  "compareTo":5,"futureGuard":[1,2]}]}],
	     "entryActions":[{"type":"Increment","inputName":"n","futureAction":null}]}
	  ],
	  "inputs": [{"type":"Numeric","name":"speed","value":0,"futureInput":"y"}],
	  "interactions": [{"type":"OnComplete","futureInteraction":true,
	                    "actions":[{"type":"Fire","inputName":"done"}]}]
	}`
	sm, err := ParseStateMachine([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(sm)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"futureTopLevel":{"keep":true}`,
		`"futureState":7`,
		`"futureTransition":"x"`,
		`"futureGuard":[1,2]`,
		`"futureAction":null`,
		`"futureInput":"y"`,
		`"futureInteraction":true`,
	} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("rewrite dropped %s\ngot: %s", want, out)
		}
	}
	// Modeled members must still win over a stale unknown of the same name.
	sm.Initial = "other"
	out, err = json.Marshal(sm)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte(`"initial":"other"`)) {
		t.Errorf("edited field lost: %s", out)
	}
}

func TestStateMachineRoundTripsThroughBundle(t *testing.T) {
	b := NewBundle()
	if err := b.SetAnimation("idle", minimalAnimation("")); err != nil {
		t.Fatal(err)
	}
	sm, err := ParseStateMachine([]byte(`{"initial":"idle","vendorExt":"keep me",
		"states":[{"name":"idle","type":"PlaybackState","animation":"idle"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := b.SetStateMachine("m", sm); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := b.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	round, err := DecodeBundle(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	got, err := round.StateMachine("m")
	if err != nil {
		t.Fatal(err)
	}
	if s := string(got.Extra["vendorExt"]); s != `"keep me"` {
		t.Errorf("vendorExt = %s; want \"keep me\"", s)
	}
}

func TestStatePlaybackDefaults(t *testing.T) {
	sm, err := ParseStateMachine([]byte(`{"initial":"a","states":[
		{"name":"a","type":"PlaybackState","animation":"x"},
		{"name":"b","type":"PlaybackState","animation":"x","speed":2,"mode":"Reverse"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	a, _ := sm.State("a")
	if got := a.PlaybackSpeed(); got != 1 {
		t.Errorf("absent speed resolved to %v; want 1", got)
	}
	if got := a.PlaybackMode(); got != PlayForward {
		t.Errorf("absent mode resolved to %v; want Forward", got)
	}
	b, _ := sm.State("b")
	if got := b.PlaybackSpeed(); got != 2 {
		t.Errorf("PlaybackSpeed() = %v; want 2", got)
	}
	if got := b.PlaybackMode(); got != PlayReverse {
		t.Errorf("PlaybackMode() = %v; want Reverse", got)
	}
	// An absent speed must not be invented on rewrite.
	out, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(out, &members); err != nil {
		t.Fatal(err)
	}
	if _, ok := members["speed"]; ok {
		t.Errorf("rewrite invented a speed member: %s", out)
	}
}

// omitempty on a plain any would drop an explicit false and silently change
// what the guard means, so raw values carry these through verbatim.
func TestFalseAndZeroValuesSurvive(t *testing.T) {
	const src = `{"initial":"a","states":[{"name":"a","type":"PlaybackState","animation":"x",
	  "transitions":[{"type":"Transition","toState":"a","guards":[
	    {"type":"Boolean","inputName":"grounded","conditionType":"Equal","compareTo":false},
	    {"type":"Numeric","inputName":"hp","conditionType":"GreaterThan","compareTo":0}]}],
	  "entryActions":[{"type":"SetFrame","value":0}]}],
	  "inputs":[{"type":"Boolean","name":"grounded","value":false},
	            {"type":"Numeric","name":"hp","value":0}]}`
	sm, err := ParseStateMachine([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(sm)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"compareTo":false`, `"compareTo":0`, `"value":false`, `"value":0`} {
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("rewrite dropped %s\ngot: %s", want, out)
		}
	}
	guards := sm.States[0].Transitions[0].Guards
	if v, ok := BoolValue(guards[0].CompareTo); ok != true || v != false {
		t.Errorf("BoolValue = %v, %v; want false, true", v, ok)
	}
	if v, ok := NumberValue(guards[1].CompareTo); !ok || v != 0 {
		t.Errorf("NumberValue = %v, %v; want 0, true", v, ok)
	}
	// An Event guard has no comparison at all.
	if _, ok := BoolValue(nil); ok {
		t.Error("BoolValue reported an absent value as present")
	}
}

func TestJSONValueRoundTrip(t *testing.T) {
	if v, ok := BoolValue(JSONValue(true)); !ok || !v {
		t.Errorf("bool round trip = %v, %v", v, ok)
	}
	if v, ok := NumberValue(JSONValue(5)); !ok || v != 5 {
		t.Errorf("number round trip = %v, %v", v, ok)
	}
	if v, ok := StringValue(JSONValue("run")); !ok || v != "run" {
		t.Errorf("string round trip = %v, %v", v, ok)
	}
	if _, ok := StringValue(JSONValue(1)); ok {
		t.Error("StringValue accepted a number")
	}
}

func TestStateMachineValidate(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want string
	}{
		{"no initial", `{"states":[{"name":"a","type":"GlobalState"}]}`, "no initial state"},
		{"missing initial", `{"initial":"z","states":[{"name":"a","type":"GlobalState"}]}`, `initial state "z" does not exist`},
		{"dangling target", `{"initial":"a","states":[{"name":"a","type":"GlobalState",
			"transitions":[{"type":"Transition","toState":"nowhere"}]}]}`, `unknown state "nowhere"`},
		{"duplicate", `{"initial":"a","states":[{"name":"a","type":"GlobalState"},
			{"name":"a","type":"GlobalState"}]}`, `duplicate state "a"`},
		{"no animation", `{"initial":"a","states":[{"name":"a","type":"PlaybackState"}]}`, `state "a" plays no animation`},
		{"undeclared input", `{"initial":"a","states":[{"name":"a","type":"GlobalState",
			"transitions":[{"type":"Transition","toState":"a","guards":[{"type":"Event","inputName":"ghost"}]}]}]}`,
			`undeclared input "ghost"`},
		{"unreachable", `{"initial":"a","states":[{"name":"a","type":"GlobalState"},
			{"name":"orphan","type":"PlaybackState","animation":"x"}]}`, `state "orphan" is unreachable`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm, err := ParseStateMachine([]byte(tt.doc))
			if err != nil {
				t.Fatal(err)
			}
			var msgs []string
			for _, p := range sm.Validate() {
				msgs = append(msgs, p.Error())
			}
			if joined := strings.Join(msgs, "\n"); !strings.Contains(joined, tt.want) {
				t.Errorf("Validate() missing %q; got:\n%s", tt.want, joined)
			}
		})
	}
}

func TestStateMachineValidateAcceptsGoodMachine(t *testing.T) {
	sm, err := ParseStateMachine([]byte(walkJumpMachine))
	if err != nil {
		t.Fatal(err)
	}
	if got := sm.Validate(); len(got) != 0 {
		t.Errorf("Validate() = %v; want none", got)
	}
}

// A global state's transitions apply from anywhere, so its targets are
// reachable even when nothing points at them directly.
func TestGlobalStateTargetsAreReachable(t *testing.T) {
	sm, err := ParseStateMachine([]byte(`{"initial":"idle","states":[
		{"name":"idle","type":"PlaybackState","animation":"x"},
		{"name":"any","type":"GlobalState","transitions":[{"type":"Transition","toState":"hurt"}]},
		{"name":"hurt","type":"PlaybackState","animation":"x"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range sm.Validate() {
		if strings.Contains(p.Error(), "unreachable") {
			t.Errorf("unexpected: %v", p)
		}
	}
}

func TestStateMachineUnsupportedFeatures(t *testing.T) {
	sm, err := ParseStateMachine([]byte(`{"initial":"a","states":[
		{"name":"a","type":"PlaybackState","animation":"x",
		 "entryActions":[{"type":"SetTheme","value":"dark"},{"type":"Fire","inputName":"n"}]}],
		"interactions":[{"type":"PointerEnter","actions":[{"type":"OpenUrl","url":"https://example.com"}]},
		                {"type":"OnComplete","actions":[]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"action OpenUrl", "action SetTheme", "interaction PointerEnter"}
	if got := sm.UnsupportedFeatures(); !slices.Equal(got, want) {
		t.Errorf("UnsupportedFeatures() = %v; want %v", got, want)
	}
}
