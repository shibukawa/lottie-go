package lottie

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
)

// maxTransitionsPerUpdate bounds a chain of transitions taken in one tick so
// that a cycle of unconditional transitions cannot hang the game loop.
const maxTransitionsPerUpdate = 16

// StateMachinePlayer runs a StateMachine over the animations of a Bundle. It
// is what lets a game ask for "jump" instead of tracking frame ranges:
//
//	sm, err := bundle.NewStateMachinePlayer("character")
//	...
//	func (g *Game) Update() error {
//		if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
//			sm.Fire("jump")
//		}
//		sm.Update()
//		return nil
//	}
//	func (g *Game) Draw(screen *ebiten.Image) { sm.Draw(screen, nil) }
//
// A StateMachinePlayer is not safe for concurrent use; drive it from the
// game's Update/Draw loop.
type StateMachinePlayer struct {
	bundle *Bundle
	sm     *StateMachine

	current *State
	player  *Player

	inputs  map[string]json.RawMessage
	pending map[string]bool // fired by the game, visible to the next Update
	active  map[string]bool // events this Update evaluates against

	completed     bool
	loopCompleted bool

	onStateChanged func(from, to string)

	unsupported map[string]struct{}
	err         error
}

// NewStateMachinePlayer starts the state machine with the given id. It
// enters the machine's initial state, so the returned player is already
// showing something.
func (b *Bundle) NewStateMachinePlayer(id string) (*StateMachinePlayer, error) {
	sm, err := b.StateMachine(id)
	if err != nil {
		return nil, err
	}
	m := &StateMachinePlayer{
		bundle:      b,
		sm:          sm,
		inputs:      make(map[string]json.RawMessage, len(sm.Inputs)),
		pending:     map[string]bool{},
		active:      map[string]bool{},
		unsupported: map[string]struct{}{},
	}
	for _, f := range sm.UnsupportedFeatures() {
		m.unsupported[f] = struct{}{}
	}
	for _, in := range sm.Inputs {
		if in.Type != InputEvent && len(in.Value) > 0 {
			m.inputs[in.Name] = in.Value
		}
	}
	if sm.Initial == "" {
		return nil, fmt.Errorf("lottie: state machine %q has no initial state", id)
	}
	if !m.enter(sm.Initial) {
		return nil, m.err
	}
	// The initial state may be a GlobalState, or may have an unconditional
	// transition out of it; settle before handing the player back.
	m.step()
	return m, nil
}

// Definition returns the document this player runs. Treat it as read-only
// while the player is live.
func (m *StateMachinePlayer) Definition() *StateMachine { return m.sm }

// State returns the name of the current state.
func (m *StateMachinePlayer) State() string {
	if m.current == nil {
		return ""
	}
	return m.current.Name
}

// Player returns the Player of the clip the current state plays, or nil in a
// state that plays none. Use it to read playback position for a scrub bar.
func (m *StateMachinePlayer) Player() *Player { return m.player }

// OnStateChanged registers a function to run on every state change,
// including the moves a single Update chains through. It runs during Update.
func (m *StateMachinePlayer) OnStateChanged(f func(from, to string)) {
	m.onStateChanged = f
}

// Err returns the first error hit while running, such as a state naming an
// animation the bundle does not hold. Playback continues past it with
// whatever was already on screen.
func (m *StateMachinePlayer) Err() error { return m.err }

// UnsupportedFeatures lists parts of the document this player does not act
// on, such as the pointer interactions a game supplies itself, plus problems
// found while running.
func (m *StateMachinePlayer) UnsupportedFeatures() []string {
	out := make([]string, 0, len(m.unsupported))
	for f := range m.unsupported {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

func (m *StateMachinePlayer) note(feature string) {
	m.unsupported[feature] = struct{}{}
}

// Fire raises an event input, the trigger a game uses to drive the machine.
// The event becomes visible to the next Update, so firing it from anywhere
// in a frame is safe. An unknown name is simply never matched.
func (m *StateMachinePlayer) Fire(input string) { m.pending[input] = true }

// InputValue constrains what a state machine input can hold: the Go types
// that map onto dotLottie's Boolean, Numeric, and String inputs. Every
// numeric width is admitted so that a game can pass whatever it already has
// — including an untyped constant, which defaults to int.
type InputValue interface {
	~bool | ~string |
		~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// Set sets a value input, which holds until it is set again:
//
//	sm.Set("speed", 12.5)
//	sm.Set("grounded", true)
//	sm.Set("weapon", "bow")
//
// Event inputs are not set; raise them with Fire.
func (m *StateMachinePlayer) Set[T InputValue](name string, v T) {
	m.inputs[name] = JSONValue(v)
}

// Get reads a value input. The type must be given explicitly, since there is
// nothing to infer it from:
//
//	speed, ok := sm.Get[float64]("speed")
//
// It reports false when the input is unset or holds a different kind of
// value.
func (m *StateMachinePlayer) Get[T InputValue](name string) (T, bool) {
	var v T
	raw, ok := m.inputs[name]
	if !ok || json.Unmarshal(raw, &v) != nil {
		var zero T
		return zero, false
	}
	return v, true
}

// Update advances the current clip, runs the playback interactions it
// triggers, then takes whatever transitions now apply. Call it once per tick
// from ebiten.Game's Update.
func (m *StateMachinePlayer) Update() {
	// Events the game fired since the last call become visible now.
	m.active, m.pending = m.pending, m.active
	clear(m.pending)

	m.completed, m.loopCompleted = false, false
	if m.player != nil {
		m.player.Update()
	}
	// Playback interactions run before transitions so that the events they
	// fire are evaluated in this same tick: a one-shot clip returns to idle
	// on the tick it ends, not the one after.
	if m.loopCompleted {
		m.runInteractions(InteractionOnLoopComplete)
	}
	if m.completed {
		m.runInteractions(InteractionOnComplete)
	}
	m.step()
	clear(m.active)
}

// Draw renders the current clip. opts may be nil.
func (m *StateMachinePlayer) Draw(dst *ebiten.Image, opts *DrawOptions) {
	if m.player != nil {
		m.player.Draw(dst, opts)
	}
}

// step takes transitions until none applies.
func (m *StateMachinePlayer) step() {
	for range maxTransitionsPerUpdate {
		tr, ok := m.nextTransition()
		if !ok {
			return
		}
		// An event is consumed by the transition that takes it, so one Fire
		// cannot cascade through several states in the same tick.
		for _, g := range tr.Guards {
			if g.Type == GuardEvent {
				delete(m.active, g.InputName)
			}
		}
		if !m.enter(tr.ToState) {
			return
		}
	}
	m.note(fmt.Sprintf("more than %d transitions in one update", maxTransitionsPerUpdate))
}

// nextTransition returns the transition that applies now. The current
// state's own transitions are considered first, in declaration order, then
// those of every global state: specific before general.
func (m *StateMachinePlayer) nextTransition() (*Transition, bool) {
	if m.current != nil {
		if tr, ok := m.pick(m.current.Transitions); ok {
			return tr, true
		}
	}
	for i := range m.sm.States {
		st := &m.sm.States[i]
		if st.Type != StateGlobal || st == m.current {
			continue
		}
		if tr, ok := m.pick(st.Transitions); ok {
			return tr, true
		}
	}
	return nil, false
}

func (m *StateMachinePlayer) pick(trs []Transition) (*Transition, bool) {
	for i := range trs {
		if m.guardsPass(trs[i].Guards) {
			return &trs[i], true
		}
	}
	return nil, false
}

// guardsPass reports whether every guard passes. A transition with no guards
// is unconditional.
func (m *StateMachinePlayer) guardsPass(gs []Guard) bool {
	for _, g := range gs {
		if !m.guardPasses(g) {
			return false
		}
	}
	return true
}

func (m *StateMachinePlayer) guardPasses(g Guard) bool {
	switch g.Type {
	case GuardEvent:
		return m.active[g.InputName]
	case GuardBoolean:
		want, okWant := BoolValue(g.CompareTo)
		got, okGot := BoolValue(m.inputs[g.InputName])
		if !okWant || !okGot {
			return false
		}
		return compareEquality(g.ConditionType, got == want)
	case GuardString:
		want, okWant := StringValue(g.CompareTo)
		got, okGot := StringValue(m.inputs[g.InputName])
		if !okWant || !okGot {
			return false
		}
		return compareEquality(g.ConditionType, got == want)
	case GuardNumeric:
		want, okWant := NumberValue(g.CompareTo)
		got, okGot := NumberValue(m.inputs[g.InputName])
		if !okWant || !okGot {
			return false
		}
		return compareNumbers(g.ConditionType, got, want)
	}
	m.note(fmt.Sprintf("guard %s", g.Type))
	return false
}

// compareEquality applies the two conditions that a bool or string guard can
// use. An absent condition means Equal.
func compareEquality(c ConditionType, equal bool) bool {
	switch c {
	case ConditionNotEqual:
		return !equal
	case "", ConditionEqual:
		return equal
	}
	return false
}

func compareNumbers(c ConditionType, got, want float64) bool {
	switch c {
	case "", ConditionEqual:
		return got == want
	case ConditionNotEqual:
		return got != want
	case ConditionGreaterThan:
		return got > want
	case ConditionGreaterThanOrEqual:
		return got >= want
	case ConditionLessThan:
		return got < want
	case ConditionLessThanOrEqual:
		return got <= want
	}
	return false
}

// enter moves to the named state, running the exit and entry actions around
// the change. It reports false when the state does not exist.
func (m *StateMachinePlayer) enter(name string) bool {
	next, ok := m.sm.State(name)
	if !ok {
		m.fail(fmt.Errorf("lottie: state machine has no state %q", name))
		return false
	}
	from := ""
	if m.current != nil {
		from = m.current.Name
		m.runActions(m.current.ExitActions)
	}
	m.current = next
	m.startPlayback(next)
	m.runActions(next.EntryActions)
	if m.onStateChanged != nil {
		m.onStateChanged(from, name)
	}
	return true
}

// startPlayback prepares the clip a state plays. A global state holds no
// animation and leaves whatever is on screen untouched: its purpose is to
// carry transitions that apply from anywhere.
func (m *StateMachinePlayer) startPlayback(st *State) {
	if st.Type != StatePlayback {
		return
	}
	anim, err := m.bundle.Animation(st.Animation)
	if err != nil {
		m.fail(err)
		return
	}
	p := anim.NewPlayer()
	if st.Segment != "" && !p.SetMarkerRange(st.Segment) {
		m.note(fmt.Sprintf("unknown marker %q in animation %q", st.Segment, st.Animation))
	}
	p.SetLoop(st.Loop)
	p.SetLoopCount(st.LoopCount)
	p.SetSpeed(st.PlaybackSpeed())
	p.SetReverse(st.PlaybackMode() == PlayReverse)
	p.Rewind()
	p.OnComplete(func() { m.completed = true })
	p.OnLoopComplete(func() { m.loopCompleted = true })
	if st.Autoplay {
		p.Play()
	} else {
		p.Pause()
	}
	m.player = p
}

func (m *StateMachinePlayer) runInteractions(t InteractionType) {
	for i := range m.sm.Interactions {
		if m.sm.Interactions[i].Type == t {
			m.runActions(m.sm.Interactions[i].Actions)
		}
	}
}

// runActions applies a list of actions. An event an action fires is visible
// immediately, unlike one the game fires with Fire.
func (m *StateMachinePlayer) runActions(actions []Action) {
	for _, a := range actions {
		switch a.Type {
		case ActionFire:
			m.active[a.InputName] = true
		case ActionToggle:
			v, _ := BoolValue(m.inputs[a.InputName])
			m.inputs[a.InputName] = JSONValue(!v)
		case ActionIncrement:
			m.addToInput(a, 1)
		case ActionDecrement:
			m.addToInput(a, -1)
		case ActionSetFrame:
			if f, ok := NumberValue(a.Value); ok && m.player != nil {
				m.player.SetFrame(f)
			}
		case ActionSetProgress:
			if v, ok := NumberValue(a.Value); ok && m.player != nil {
				m.player.SetProgress(v)
			}
		default:
			m.note(fmt.Sprintf("action %s", a.Type))
		}
	}
}

// addToInput moves a numeric input by the action's value, defaulting to one
// step in the given direction.
func (m *StateMachinePlayer) addToInput(a Action, sign float64) {
	step, ok := NumberValue(a.Value)
	if !ok {
		step = 1
	}
	v, _ := NumberValue(m.inputs[a.InputName])
	m.inputs[a.InputName] = JSONValue(v + sign*step)
}

func (m *StateMachinePlayer) fail(err error) {
	if m.err == nil {
		m.err = err
	}
	m.note(err.Error())
}
