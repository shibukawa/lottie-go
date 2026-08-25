package lottie

import (
	"encoding/json"
	"fmt"
	"sort"
)

// StateMachine is one dotLottie state machine document, stored as
// s/<id>.json inside a bundle. It describes which animation plays in each
// state and what moves playback between states, so a game can ask for
// "jump" instead of tracking frame ranges itself.
//
// Members this package does not model are preserved and written back
// unchanged, keeping a bundle edited here valid for other dotLottie
// runtimes.
type StateMachine struct {
	Initial      string        `json:"initial"`
	States       []State       `json:"states"`
	Inputs       []Input       `json:"inputs,omitempty"`
	Interactions []Interaction `json:"interactions,omitempty"`

	Extra ExtraFields `json:"-"`
}

// StateType discriminates the entries of StateMachine.States.
type StateType string

const (
	// StatePlayback plays one animation, optionally limited to a marker.
	StatePlayback StateType = "PlaybackState"
	// StateGlobal holds no animation; its transitions apply from any state.
	StateGlobal StateType = "GlobalState"
)

// PlayMode is the direction a PlaybackState plays in.
type PlayMode string

const (
	PlayForward PlayMode = "Forward"
	PlayReverse PlayMode = "Reverse"
)

// State is one entry of StateMachine.States. The playback fields apply only
// when Type is StatePlayback.
type State struct {
	Name string    `json:"name"`
	Type StateType `json:"type"`

	Animation    string       `json:"animation,omitempty"`
	Segment      string       `json:"segment,omitempty"`
	Loop         bool         `json:"loop,omitempty"`
	LoopCount    int          `json:"loopCount,omitempty"`
	Autoplay     bool         `json:"autoplay,omitempty"`
	Mode         PlayMode     `json:"mode,omitempty"`
	Speed        float64      `json:"speed,omitempty"`
	Transitions  []Transition `json:"transitions,omitempty"`
	EntryActions []Action     `json:"entryActions,omitempty"`
	ExitActions  []Action     `json:"exitActions,omitempty"`

	Extra ExtraFields `json:"-"`
}

// PlaybackSpeed returns Speed, resolving an absent value to 1. Speed is left
// as written in the document so a round trip does not invent a member.
func (s State) PlaybackSpeed() float64 {
	if s.Speed == 0 {
		return 1
	}
	return s.Speed
}

// PlaybackMode returns Mode, resolving an absent value to PlayForward.
func (s State) PlaybackMode() PlayMode {
	if s.Mode == "" {
		return PlayForward
	}
	return s.Mode
}

// TransitionType discriminates immediate from tweened transitions.
type TransitionType string

const (
	TransitionImmediate TransitionType = "Transition"
	TransitionTweened   TransitionType = "Tweened"
)

// Transition moves playback to another state once every guard passes.
//
// Order is significant: the runtime evaluates a state's transitions in
// declaration order and takes the first whose guards all pass.
type Transition struct {
	Type    TransitionType `json:"type"`
	ToState string         `json:"toState"`
	Guards  []Guard        `json:"guards,omitempty"`

	// Tweened only.
	Duration float64   `json:"duration,omitempty"`
	Easing   []float64 `json:"easing,omitempty"`

	Extra ExtraFields `json:"-"`
}

// GuardType is the kind of input a Guard inspects.
type GuardType string

const (
	GuardEvent   GuardType = "Event"
	GuardBoolean GuardType = "Boolean"
	GuardNumeric GuardType = "Numeric"
	GuardString  GuardType = "String"
)

// ConditionType is the comparison a non-event Guard applies.
type ConditionType string

const (
	ConditionEqual              ConditionType = "Equal"
	ConditionNotEqual           ConditionType = "NotEqual"
	ConditionGreaterThan        ConditionType = "GreaterThan"
	ConditionGreaterThanOrEqual ConditionType = "GreaterThanOrEqual"
	ConditionLessThan           ConditionType = "LessThan"
	ConditionLessThanOrEqual    ConditionType = "LessThanOrEqual"
)

// Guard is one condition on a Transition. A GuardEvent guard passes for the
// tick on which its input fires; the others compare an input's value.
type Guard struct {
	Type          GuardType     `json:"type"`
	InputName     string        `json:"inputName"`
	ConditionType ConditionType `json:"conditionType,omitempty"`
	// CompareTo is a raw JSON value so that an absent comparison stays
	// absent and an explicit false stays false. Build one with JSONValue
	// and read one with BoolValue, NumberValue, or StringValue.
	CompareTo json.RawMessage `json:"compareTo,omitempty"`

	Extra ExtraFields `json:"-"`
}

// InputType is the kind of a state machine variable.
type InputType string

const (
	InputEvent   InputType = "Event"
	InputBoolean InputType = "Boolean"
	InputNumeric InputType = "Numeric"
	InputString  InputType = "String"
)

// Input is a state machine variable. Event inputs are the game-facing
// trigger names: an input named "jump" is what game code fires.
type Input struct {
	Type InputType `json:"type"`
	Name string    `json:"name"`
	// Value is the initial value, raw so that an explicit false or 0
	// survives a rewrite. Event inputs have none.
	Value json.RawMessage `json:"value,omitempty"`

	Extra ExtraFields `json:"-"`
}

// InteractionType is the occurrence a listener reacts to.
type InteractionType string

const (
	InteractionOnComplete     InteractionType = "OnComplete"
	InteractionOnLoopComplete InteractionType = "OnLoopComplete"
	InteractionClick          InteractionType = "Click"
	InteractionPointerUp      InteractionType = "PointerUp"
	InteractionPointerDown    InteractionType = "PointerDown"
	InteractionPointerEnter   InteractionType = "PointerEnter"
	InteractionPointerMove    InteractionType = "PointerMove"
	InteractionPointerExit    InteractionType = "PointerExit"
)

// Interaction runs actions when its occurrence happens.
type Interaction struct {
	Type      InteractionType `json:"type"`
	LayerName string          `json:"layerName,omitempty"`
	Actions   []Action        `json:"actions,omitempty"`

	Extra ExtraFields `json:"-"`
}

// ActionType is the effect an Action applies.
type ActionType string

const (
	ActionFire        ActionType = "Fire"
	ActionToggle      ActionType = "Toggle"
	ActionIncrement   ActionType = "Increment"
	ActionDecrement   ActionType = "Decrement"
	ActionSetFrame    ActionType = "SetFrame"
	ActionSetProgress ActionType = "SetProgress"
	ActionSetTheme    ActionType = "SetTheme"
	ActionOpenURL     ActionType = "OpenUrl"
)

// Action is one effect run on state entry, on state exit, or by an
// Interaction.
type Action struct {
	Type      ActionType      `json:"type"`
	InputName string          `json:"inputName,omitempty"`
	Value     json.RawMessage `json:"value,omitempty"`
	URL       string          `json:"url,omitempty"`
	Target    string          `json:"target,omitempty"`

	Extra ExtraFields `json:"-"`
}

// JSONValue encodes v for use as Guard.CompareTo, Input.Value, or
// Action.Value. It returns nil if v cannot be encoded, which omits the
// member rather than writing a broken document.
func JSONValue(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return data
}

// BoolValue reads a raw JSON value as a bool. It reports false when the
// value is absent or is not a bool.
func BoolValue(raw json.RawMessage) (bool, bool) {
	var v bool
	if len(raw) == 0 || json.Unmarshal(raw, &v) != nil {
		return false, false
	}
	return v, true
}

// NumberValue reads a raw JSON value as a number.
func NumberValue(raw json.RawMessage) (float64, bool) {
	var v float64
	if len(raw) == 0 || json.Unmarshal(raw, &v) != nil {
		return 0, false
	}
	return v, true
}

// StringValue reads a raw JSON value as a string.
func StringValue(raw json.RawMessage) (string, bool) {
	var v string
	if len(raw) == 0 || json.Unmarshal(raw, &v) != nil {
		return "", false
	}
	return v, true
}

// ParseStateMachine decodes one state machine document.
func ParseStateMachine(data []byte) (*StateMachine, error) {
	var sm StateMachine
	if err := json.Unmarshal(data, &sm); err != nil {
		return nil, fmt.Errorf("lottie: parse state machine: %w", err)
	}
	return &sm, nil
}

// State returns the named state.
func (s *StateMachine) State(name string) (*State, bool) {
	for i := range s.States {
		if s.States[i].Name == name {
			return &s.States[i], true
		}
	}
	return nil, false
}

// Input returns the named input.
func (s *StateMachine) Input(name string) (*Input, bool) {
	for i := range s.Inputs {
		if s.Inputs[i].Name == name {
			return &s.Inputs[i], true
		}
	}
	return nil, false
}

// Validate reports structural problems that would leave the machine stuck or
// pointing at nothing. It checks only what the document itself can answer;
// animation ids are resolved against a bundle by Bundle.Validate.
func (s *StateMachine) Validate() []error {
	var problems []error
	seen := map[string]bool{}
	for _, st := range s.States {
		switch {
		case st.Name == "":
			problems = append(problems, fmt.Errorf("state with empty name"))
		case seen[st.Name]:
			problems = append(problems, fmt.Errorf("duplicate state %q", st.Name))
		}
		seen[st.Name] = true
	}
	if s.Initial == "" {
		problems = append(problems, fmt.Errorf("no initial state"))
	} else if !seen[s.Initial] {
		problems = append(problems, fmt.Errorf("initial state %q does not exist", s.Initial))
	}

	inputs := map[string]bool{}
	for _, in := range s.Inputs {
		inputs[in.Name] = true
	}
	for _, st := range s.States {
		if st.Type == StatePlayback && st.Animation == "" {
			problems = append(problems, fmt.Errorf("state %q plays no animation", st.Name))
		}
		for i, tr := range st.Transitions {
			if !seen[tr.ToState] {
				problems = append(problems, fmt.Errorf("state %q transition %d targets unknown state %q", st.Name, i, tr.ToState))
			}
			for _, g := range tr.Guards {
				if g.InputName != "" && !inputs[g.InputName] {
					problems = append(problems, fmt.Errorf("state %q transition %d guards on undeclared input %q", st.Name, i, g.InputName))
				}
			}
		}
	}
	for _, name := range s.unreachableStates() {
		problems = append(problems, fmt.Errorf("state %q is unreachable", name))
	}
	return problems
}

// unreachableStates returns named states no transition can arrive at, in
// document order. A GlobalState is reachable by definition: its transitions
// apply from everywhere.
func (s *StateMachine) unreachableStates() []string {
	reachable := map[string]bool{s.Initial: true}
	// A transition out of a global state can be taken from any state, so
	// seed those targets before walking.
	for _, st := range s.States {
		if st.Type != StateGlobal {
			continue
		}
		reachable[st.Name] = true
		for _, tr := range st.Transitions {
			reachable[tr.ToState] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, st := range s.States {
			if !reachable[st.Name] {
				continue
			}
			for _, tr := range st.Transitions {
				if !reachable[tr.ToState] {
					reachable[tr.ToState] = true
					changed = true
				}
			}
		}
	}
	var out []string
	for _, st := range s.States {
		if st.Name != "" && !reachable[st.Name] {
			out = append(out, st.Name)
		}
	}
	return out
}

// UnsupportedFeatures lists members of the document that this package models
// but deliberately does not act on, such as pointer interactions a game
// supplies itself. Playback continues without them.
func (s *StateMachine) UnsupportedFeatures() []string {
	found := map[string]struct{}{}
	for _, in := range s.Interactions {
		switch in.Type {
		case InteractionOnComplete, InteractionOnLoopComplete:
		default:
			found[fmt.Sprintf("interaction %s", in.Type)] = struct{}{}
		}
	}
	noteActions := func(actions []Action) {
		for _, a := range actions {
			switch a.Type {
			case ActionSetTheme, ActionOpenURL:
				found[fmt.Sprintf("action %s", a.Type)] = struct{}{}
			}
		}
	}
	for _, st := range s.States {
		noteActions(st.EntryActions)
		noteActions(st.ExitActions)
	}
	for _, in := range s.Interactions {
		noteActions(in.Actions)
	}
	out := make([]string, 0, len(found))
	for f := range found {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// Round-trip marshalling. Each type decodes its own fields, then keeps every
// remaining member so it survives a rewrite.

func (s StateMachine) MarshalJSON() ([]byte, error) {
	type alias StateMachine
	return encodeExtra(alias(s), s.Extra)
}

func (s *StateMachine) UnmarshalJSON(data []byte) error {
	type alias StateMachine
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := decodeExtra(data, a)
	if err != nil {
		return err
	}
	*s = StateMachine(a)
	s.Extra = extra
	return nil
}

func (s State) MarshalJSON() ([]byte, error) {
	type alias State
	return encodeExtra(alias(s), s.Extra)
}

func (s *State) UnmarshalJSON(data []byte) error {
	type alias State
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := decodeExtra(data, a)
	if err != nil {
		return err
	}
	*s = State(a)
	s.Extra = extra
	return nil
}

func (t Transition) MarshalJSON() ([]byte, error) {
	type alias Transition
	return encodeExtra(alias(t), t.Extra)
}

func (t *Transition) UnmarshalJSON(data []byte) error {
	type alias Transition
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := decodeExtra(data, a)
	if err != nil {
		return err
	}
	*t = Transition(a)
	t.Extra = extra
	return nil
}

func (g Guard) MarshalJSON() ([]byte, error) {
	type alias Guard
	return encodeExtra(alias(g), g.Extra)
}

func (g *Guard) UnmarshalJSON(data []byte) error {
	type alias Guard
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := decodeExtra(data, a)
	if err != nil {
		return err
	}
	*g = Guard(a)
	g.Extra = extra
	return nil
}

func (i Input) MarshalJSON() ([]byte, error) {
	type alias Input
	return encodeExtra(alias(i), i.Extra)
}

func (i *Input) UnmarshalJSON(data []byte) error {
	type alias Input
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := decodeExtra(data, a)
	if err != nil {
		return err
	}
	*i = Input(a)
	i.Extra = extra
	return nil
}

func (i Interaction) MarshalJSON() ([]byte, error) {
	type alias Interaction
	return encodeExtra(alias(i), i.Extra)
}

func (i *Interaction) UnmarshalJSON(data []byte) error {
	type alias Interaction
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := decodeExtra(data, a)
	if err != nil {
		return err
	}
	*i = Interaction(a)
	i.Extra = extra
	return nil
}

func (a Action) MarshalJSON() ([]byte, error) {
	type alias Action
	return encodeExtra(alias(a), a.Extra)
}

func (a *Action) UnmarshalJSON(data []byte) error {
	type alias Action
	var v alias
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	extra, err := decodeExtra(data, v)
	if err != nil {
		return err
	}
	*a = Action(v)
	a.Extra = extra
	return nil
}
