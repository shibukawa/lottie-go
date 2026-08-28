package main

// The preset's state machine wires all nineteen clips behind game-facing
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

// chibiMachine is the full platformer wiring, shared by both presets.
// Guard order matters: jump is listed first in every grounded state so it
// wins a simultaneous request, and guard-state's own hurt transition
// outranks the global one so a guarded hit plays guard-hit-anim instead
// of hurt-anim. death-state has no transitions: after die only the
// editor's Restart pseudo-event (or a fresh player) leaves it, and the
// game is expected to stop firing events.
//
// The sword variant keeps every verb but kick2 — a swordsman answers a
// second attack with the weapon — and adds slash, slash2 and thrust,
// wired like the punches: slash chains into slash2, and both slash2 and
// thrust are also reachable straight from idle.
func chibiMachine(sword bool) *lottie.StateMachine {
	grounded := boolIs("grounded", true)
	done := event("clipDone")
	attacks := []lottie.Transition{
		to("punch-state", event("punch")),
		to("punch-2-state", event("punch2")),
		to("kick-state", event("kick")),
	}
	inputs := []lottie.Input{
		{Type: lottie.InputEvent, Name: "walk"},
		{Type: lottie.InputEvent, Name: "run"},
		{Type: lottie.InputEvent, Name: "stop"},
		{Type: lottie.InputEvent, Name: "turn"},
		{Type: lottie.InputEvent, Name: "jump"},
		{Type: lottie.InputEvent, Name: "slide"},
		{Type: lottie.InputEvent, Name: "punch"},
		{Type: lottie.InputEvent, Name: "punch2"},
		{Type: lottie.InputEvent, Name: "kick"},
	}
	if sword {
		attacks = append(attacks,
			to("slash-state", event("slash")),
			to("slash-2-state", event("slash2")),
			to("thrust-state", event("thrust")))
		inputs = append(inputs,
			lottie.Input{Type: lottie.InputEvent, Name: "slash"},
			lottie.Input{Type: lottie.InputEvent, Name: "slash2"},
			lottie.Input{Type: lottie.InputEvent, Name: "thrust"})
	} else {
		attacks = append(attacks, to("kick-2-state", event("kick2")))
		inputs = append(inputs, lottie.Input{Type: lottie.InputEvent, Name: "kick2"})
	}
	inputs = append(inputs,
		lottie.Input{Type: lottie.InputEvent, Name: "guard"},
		lottie.Input{Type: lottie.InputEvent, Name: "hurt"},
		lottie.Input{Type: lottie.InputEvent, Name: "die"},
		lottie.Input{Type: lottie.InputEvent, Name: "clipDone"},
		lottie.Input{Type: lottie.InputBoolean, Name: "grounded", Value: lottie.JSONValue(true)})
	// The ground kick chains into its spin follow-up only unarmed.
	kickTransitions := []lottie.Transition{to("idle-state", done)}
	if !sword {
		kickTransitions = append([]lottie.Transition{to("kick-2-state", event("kick"))}, kickTransitions...)
	}
	weapon := []lottie.State{}
	if sword {
		weapon = []lottie.State{
			at(onceState("slash-state", "slash-anim", []lottie.Transition{
				to("slash-2-state", event("slash")),
				to("idle-state", done),
			}), 920, 430),
			at(onceState("slash-2-state", "slash-2-anim", []lottie.Transition{
				to("idle-state", done),
			}), 920, 560),
			at(onceState("thrust-state", "thrust-anim", []lottie.Transition{
				to("idle-state", done),
			}), 1140, 430),
		}
	} else {
		weapon = []lottie.State{
			at(onceState("kick-2-state", "kick-2-anim", []lottie.Transition{
				to("idle-state", done),
			}), 920, 430),
		}
	}
	idleTransitions := []lottie.Transition{
		to("jump-state", event("jump"), grounded),
		to("run-state", event("run")),
		to("walk-state", event("walk")),
		to("slide-state", event("slide")),
	}
	idleTransitions = append(idleTransitions, attacks...)
	idleTransitions = append(idleTransitions,
		to("idle-turn-state", event("turn")),
		to("guard-state", event("guard")))
	states := []lottie.State{
		// Column 1: ground loops and their turns.
		at(loopState("idle-state", "idle-anim", idleTransitions), 40, 40),
		at(onceState("idle-turn-state", "idle-turn-anim", []lottie.Transition{
			to("idle-state", done),
		}), 40, 170),
		at(loopState("walk-state", "walk-anim", []lottie.Transition{
			to("jump-state", event("jump"), grounded),
			to("run-state", event("run")),
			to("slide-state", event("slide")),
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
		}), 480, 40),
		at(onceState("run-to-idle-state", "run-to-idle-anim", []lottie.Transition{
			to("idle-state", done),
		}), 700, 170),
		at(onceState("slide-state", "slide-anim", []lottie.Transition{
			to("idle-state", done),
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
		at(onceState("kick-state", "kick-anim", kickTransitions), 260, 430),
	}
	// The weapon (or the spin kick it replaces) sits with the rest of the
	// attacks, so the graph reads the same in both presets.
	states = append(states, weapon...)
	states = append(states, []lottie.State{
		at(loopState("guard-state", "guard-anim", []lottie.Transition{
			to("guard-hit-state", event("hurt")),
			to("idle-state", event("guard")),
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
	}...)
	return &lottie.StateMachine{
		Initial: "idle-state",
		Inputs:  inputs,
		States:  states,
		Interactions: []lottie.Interaction{{
			Type:    lottie.InteractionOnComplete,
			Actions: []lottie.Action{{Type: lottie.ActionFire, InputName: "clipDone"}},
		}},
	}
}
