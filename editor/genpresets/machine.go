package main

// The preset's state machine wires all twenty clips behind game-facing
// verbs. Conventions follow editor/gensamples: -state / -anim suffixes,
// bare input names, clipDone fired by the OnComplete interaction so
// one-shot clips hand over by themselves.

import (
	"encoding/json"

	lottie "github.com/shibukawa/lottie-go"
)

const editorExtraKey = "x-lottie-go-editor"

func at(s lottie.State, x, y int) lottie.State {
	raw, err := json.Marshal(struct {
		X int `json:"x"`
		Y int `json:"y"`
	}{x, y})
	if err != nil {
		return s
	}
	if s.Extra == nil {
		s.Extra = lottie.ExtraFields{}
	}
	s.Extra[editorExtraKey] = raw
	return s
}

func event(name string) lottie.Guard {
	return lottie.Guard{Type: lottie.GuardEvent, InputName: name}
}

func boolIs(name string, v bool) lottie.Guard {
	return lottie.Guard{
		Type: lottie.GuardBoolean, InputName: name,
		ConditionType: lottie.ConditionEqual, CompareTo: lottie.JSONValue(v),
	}
}

func loopState(name, anim string, transitions []lottie.Transition) lottie.State {
	return lottie.State{
		Name: name, Type: lottie.StatePlayback, Animation: anim,
		Loop: true, Autoplay: true, Transitions: transitions,
	}
}

func onceState(name, anim string, transitions []lottie.Transition) lottie.State {
	return lottie.State{
		Name: name, Type: lottie.StatePlayback, Animation: anim,
		Autoplay: true, Transitions: transitions,
	}
}

func to(state string, guards ...lottie.Guard) lottie.Transition {
	return lottie.Transition{Type: lottie.TransitionImmediate, ToState: state, Guards: guards}
}

// chibiMachine is the full platformer wiring. Guard order matters: jump is
// listed first in every grounded state so it wins a simultaneous request,
// and guard-state's own hurt transition outranks the global one so a
// guarded hit plays guard-hit-anim instead of hurt-anim. death-state has
// no transitions: after die only the editor's Restart pseudo-event (or a
// fresh player) leaves it, and the game is expected to stop firing events.
func chibiMachine() *lottie.StateMachine {
	grounded := boolIs("grounded", true)
	done := event("clipDone")
	return &lottie.StateMachine{
		Initial: "idle-state",
		Inputs: []lottie.Input{
			{Type: lottie.InputEvent, Name: "walk"},
			{Type: lottie.InputEvent, Name: "run"},
			{Type: lottie.InputEvent, Name: "dash"},
			{Type: lottie.InputEvent, Name: "stop"},
			{Type: lottie.InputEvent, Name: "turn"},
			{Type: lottie.InputEvent, Name: "jump"},
			{Type: lottie.InputEvent, Name: "slide"},
			{Type: lottie.InputEvent, Name: "punch"},
			{Type: lottie.InputEvent, Name: "kick"},
			{Type: lottie.InputEvent, Name: "hurt"},
			{Type: lottie.InputEvent, Name: "die"},
			{Type: lottie.InputEvent, Name: "clipDone"},
			{Type: lottie.InputBoolean, Name: "grounded", Value: lottie.JSONValue(true)},
			{Type: lottie.InputBoolean, Name: "guarding", Value: lottie.JSONValue(false)},
		},
		States: []lottie.State{
			// Column 1: ground loops and their turns.
			at(loopState("idle-state", "idle-anim", []lottie.Transition{
				to("jump-state", event("jump"), grounded),
				to("dash-state", event("dash")),
				to("run-state", event("run")),
				to("walk-state", event("walk")),
				to("punch-state", event("punch")),
				to("kick-state", event("kick")),
				to("idle-turn-state", event("turn")),
				to("guard-state", boolIs("guarding", true)),
			}), 40, 40),
			at(onceState("idle-turn-state", "idle-turn-anim", []lottie.Transition{
				to("idle-state", done),
			}), 40, 170),
			at(loopState("walk-state", "walk-anim", []lottie.Transition{
				to("jump-state", event("jump"), grounded),
				to("run-state", event("run")),
				to("dash-state", event("dash")),
				to("idle-state", event("stop")),
				to("walk-turn-state", event("turn")),
			}), 260, 40),
			at(onceState("walk-turn-state", "walk-turn-anim", []lottie.Transition{
				to("walk-state", done),
			}), 260, 170),
			at(loopState("run-state", "run-anim", []lottie.Transition{
				to("jump-state", event("jump"), grounded),
				to("slide-state", event("slide")),
				to("walk-state", event("walk")),
				to("run-to-idle-state", event("stop")),
				to("run-turn-state", event("turn")),
			}), 480, 40),
			at(onceState("run-turn-state", "run-turn-anim", []lottie.Transition{
				to("run-state", done),
			}), 480, 170),
			at(onceState("dash-state", "dash-anim", []lottie.Transition{
				to("jump-state", event("jump"), grounded),
				to("run-state", done),
			}), 700, 40),
			at(onceState("run-to-idle-state", "run-to-idle-anim", []lottie.Transition{
				to("idle-state", done),
			}), 700, 170),
			at(onceState("slide-state", "slide-anim", []lottie.Transition{
				to("run-state", done),
			}), 700, 300),
			// Column 2: air.
			at(onceState("jump-state", "jump-anim", []lottie.Transition{
				to("jump-kick-state", event("kick")),
				to("fall-state", done),
			}), 40, 300),
			at(onceState("fall-state", "fall-anim", []lottie.Transition{
				to("jump-kick-state", event("kick")),
				to("fall-loop-state", done),
			}), 260, 300),
			at(loopState("fall-loop-state", "fall-loop-anim", []lottie.Transition{
				to("jump-kick-state", event("kick")),
				to("idle-state", grounded),
			}), 480, 300),
			at(onceState("jump-kick-state", "jump-kick-anim", []lottie.Transition{
				to("fall-loop-state", done),
			}), 480, 430),
			// Column 3: attacks and defense.
			at(onceState("punch-state", "punch-anim", []lottie.Transition{
				to("punch-2-state", event("punch")),
				to("idle-state", done),
			}), 40, 430),
			at(onceState("punch-2-state", "punch-2-anim", []lottie.Transition{
				to("idle-state", done),
			}), 40, 560),
			at(onceState("kick-state", "kick-anim", []lottie.Transition{
				to("idle-state", done),
			}), 260, 430),
			at(loopState("guard-state", "guard-anim", []lottie.Transition{
				to("guard-hit-state", event("hurt")),
				to("idle-state", boolIs("guarding", false)),
			}), 260, 560),
			at(onceState("guard-hit-state", "guard-hit-anim", []lottie.Transition{
				to("guard-state", done),
			}), 480, 560),
			// Reactions, reachable from anywhere.
			at(onceState("hurt-state", "hurt-anim", []lottie.Transition{
				to("idle-state", done),
			}), 700, 430),
			at(onceState("death-state", "death-anim", nil), 700, 560),
			at(lottie.State{
				Name: "anywhere-state", Type: lottie.StateGlobal,
				Transitions: []lottie.Transition{
					to("death-state", event("die")),
					to("hurt-state", event("hurt")),
				},
			}, 920, 40),
		},
		Interactions: []lottie.Interaction{{
			Type:    lottie.InteractionOnComplete,
			Actions: []lottie.Action{{Type: lottie.ActionFire, InputName: "clipDone"}},
		}},
	}
}
