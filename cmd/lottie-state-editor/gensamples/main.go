// Command gensamples writes the editor's sample bundles under
// examples/state-editor. The samples are generated rather than downloaded so their
// licensing is unambiguous: every clip here is authored in this repository.
//
//	go run ./gensamples
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	lottie "github.com/shibukawa/lottie-go"
)

// editorExtraKey must match the editor's own constant: it is where node
// positions ride so a sample opens with a laid-out graph.
const editorExtraKey = "x-lottie-go-editor"

func main() {
	out := flag.String("out", filepath.Join("..", "..", "examples", "state-editor"), "output directory")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(out string) error {
	if err := writeSample(filepath.Join(out, "character"), "character",
		map[string]obj{
			"idle-anim": idleClip(), "walk-anim": walkClip(), "run-anim": runClip(),
			"jump-anim": jumpClip(), "hurt-anim": hurtClip(),
		}, characterMachine()); err != nil {
		return err
	}
	if err := writeSample(filepath.Join(out, "spritesheet"), "spritesheet",
		map[string]obj{"actions-anim": sheetClip()}, sheetMachine()); err != nil {
		return err
	}
	if err := writeSample(filepath.Join(out, "combo"), "combo",
		map[string]obj{
			"ready-anim": readyClip(), "windup-anim": windupClip(),
			"strike-anim": strikeClip(), "recover-anim": recoverClip(),
		}, comboMachine()); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "README.md"), []byte(readme), 0o644)
}

// writeSample writes the loose clips and the bundle that ties them together.
// The clips stay on disk so the editor's Import can be tried against them.
func writeSample(dir, name string, clips map[string]obj, sm *lottie.StateMachine) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// Clips this run does not produce are left over from an earlier one.
	// Without this, renaming a clip leaves both names on disk and the stale
	// one looks like part of the sample.
	if err := removeStaleClips(dir, clips); err != nil {
		return err
	}
	b := lottie.NewBundle()
	// Sorted, not map order: the manifest records the order animations were
	// added, so iterating the map would rewrite the committed bundle on
	// every run and churn the repository for no reason.
	for _, id := range slices.Sorted(maps.Keys(clips)) {
		clip := clips[id]
		data, err := json.MarshalIndent(clip, "", " ")
		if err != nil {
			return fmt.Errorf("%s/%s: %w", name, id, err)
		}
		if err := os.WriteFile(filepath.Join(dir, id+".json"), data, 0o644); err != nil {
			return err
		}
		if err := b.SetAnimation(id, data); err != nil {
			return fmt.Errorf("%s/%s: %w", name, id, err)
		}
	}
	if err := b.SetStateMachine(name, sm); err != nil {
		return err
	}
	b.Manifest().Generator = "lottie-go/cmd/lottie-state-editor gensamples"
	if problems := b.Validate(); len(problems) > 0 {
		return fmt.Errorf("%s: %v", name, problems)
	}
	var buf bytes.Buffer
	if err := b.Encode(&buf); err != nil {
		return err
	}
	path := filepath.Join(dir, name+".lottie")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d clips, %d bytes)\n", path, len(clips), buf.Len())
	return nil
}

// removeStaleClips deletes the .json clips in dir that are not being
// written. The bundle and the README are left alone.
func removeStaleClips(dir string, clips map[string]obj) error {
	entries, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}
	for _, path := range entries {
		id := strings.TrimSuffix(filepath.Base(path), ".json")
		if _, ok := clips[id]; ok {
			continue
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		fmt.Printf("removed stale %s\n", path)
	}
	return nil
}

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

// characterMachine is the shape a platformer actually needs: verbs the game
// fires, a jump gated on a boolean the game owns, damage reachable from
// anywhere, and one-shot clips that return by themselves.
func characterMachine() *lottie.StateMachine {
	grounded := lottie.Guard{
		Type: lottie.GuardBoolean, InputName: "grounded",
		ConditionType: lottie.ConditionEqual, CompareTo: lottie.JSONValue(true),
	}
	// Order matters: jump is listed first everywhere so it wins over a
	// simultaneous move request.
	return &lottie.StateMachine{
		Initial: "idle-state",
		Inputs: []lottie.Input{
			{Type: lottie.InputEvent, Name: "walk"},
			{Type: lottie.InputEvent, Name: "run"},
			{Type: lottie.InputEvent, Name: "stop"},
			{Type: lottie.InputEvent, Name: "jump"},
			{Type: lottie.InputEvent, Name: "hurt"},
			{Type: lottie.InputEvent, Name: "clipDone"},
			{Type: lottie.InputBoolean, Name: "grounded", Value: lottie.JSONValue(true)},
		},
		States: []lottie.State{
			at(loopState("idle-state", "idle-anim", []lottie.Transition{
				to("jump-state", event("jump"), grounded),
				to("run-state", event("run")),
				to("walk-state", event("walk")),
			}), 30, 140),
			at(loopState("walk-state", "walk-anim", []lottie.Transition{
				to("jump-state", event("jump"), grounded),
				to("run-state", event("run")),
				to("idle-state", event("stop")),
			}), 250, 30),
			at(loopState("run-state", "run-anim", []lottie.Transition{
				to("jump-state", event("jump"), grounded),
				to("walk-state", event("walk")),
				to("idle-state", event("stop")),
			}), 470, 30),
			at(onceState("jump-state", "jump-anim", []lottie.Transition{
				to("idle-state", event("clipDone")),
			}), 250, 250),
			at(onceState("hurt-state", "hurt-anim", []lottie.Transition{
				to("idle-state", event("clipDone")),
			}), 470, 250),
			// A global state's transitions apply from every state, which is
			// how "take damage at any time" is expressed.
			at(lottie.State{
				Name: "anywhere-state", Type: lottie.StateGlobal,
				Transitions: []lottie.Transition{to("hurt-state", event("hurt"))},
			}, 30, 360),
		},
		// One-shot clips announce their own end, so the game never needs a
		// timer to bring the character home.
		Interactions: []lottie.Interaction{{
			Type:    lottie.InteractionOnComplete,
			Actions: []lottie.Action{{Type: lottie.ActionFire, InputName: "clipDone"}},
		}},
	}
}

// sheetMachine plays three ranges of one document, addressed by marker. It
// is the sample for PlaybackState.segment.
func sheetMachine() *lottie.StateMachine {
	seg := func(name, marker string, loop bool, trs []lottie.Transition) lottie.State {
		return lottie.State{
			Name: name, Type: lottie.StatePlayback, Animation: "actions-anim",
			Segment: marker, Loop: loop, Autoplay: true, Transitions: trs,
		}
	}
	return &lottie.StateMachine{
		Initial: "idle-state",
		Inputs: []lottie.Input{
			{Type: lottie.InputEvent, Name: "walk"},
			{Type: lottie.InputEvent, Name: "stop"},
			{Type: lottie.InputEvent, Name: "jump"},
			{Type: lottie.InputEvent, Name: "clipDone"},
		},
		States: []lottie.State{
			at(seg("idle-state", "idle-seg", true, []lottie.Transition{
				to("jump-state", event("jump")),
				to("walk-state", event("walk")),
			}), 30, 40),
			at(seg("walk-state", "walk-seg", true, []lottie.Transition{
				to("jump-state", event("jump")),
				to("idle-state", event("stop")),
			}), 250, 40),
			at(seg("jump-state", "jump-seg", false, []lottie.Transition{
				to("idle-state", event("clipDone")),
			}), 470, 40),
		},
		Interactions: []lottie.Interaction{{
			Type:    lottie.InteractionOnComplete,
			Actions: []lottie.Action{{Type: lottie.ActionFire, InputName: "clipDone"}},
		}},
	}
}

// comboMachine chains three one-shot clips: each hands over when it ends,
// so the three play as a single motion. It is the sample for sequential
// playback, which nothing else here shows.
func comboMachine() *lottie.StateMachine {
	return &lottie.StateMachine{
		Initial: "ready-state",
		Inputs: []lottie.Input{
			{Type: lottie.InputEvent, Name: "attack"},
			{Type: lottie.InputEvent, Name: "clipDone"},
		},
		States: []lottie.State{
			at(loopState("ready-state", "ready-anim", []lottie.Transition{
				to("windup-state", event("attack")),
			}), 30, 40),
			at(onceState("windup-state", "windup-anim", []lottie.Transition{
				to("strike-state", event("clipDone")),
			}), 250, 40),
			at(onceState("strike-state", "strike-anim", []lottie.Transition{
				to("recover-state", event("clipDone")),
			}), 470, 40),
			at(onceState("recover-state", "recover-anim", []lottie.Transition{
				to("ready-state", event("clipDone")),
			}), 250, 190),
		},
		Interactions: []lottie.Interaction{{
			Type:    lottie.InteractionOnComplete,
			Actions: []lottie.Action{{Type: lottie.ActionFire, InputName: "clipDone"}},
		}},
	}
}

const readme = `# Editor sample data

Generated by ` + "`go run ./gensamples`" + ` from the editor module. Every clip is
authored in this repository, so there is no third-party licensing to track.

Open a bundle with:

    cd cmd/lottie-state-editor && go run . ../../examples/state-editor/character/character.lottie

## Naming

Clips, states, markers and inputs are four separate namespaces, and dotLottie
does nothing to keep them apart. A sample where the clip, the state, the
marker and the event are all called ` + "`jump`" + ` reads fine on screen and is
unreadable as data, so the samples suffix them:

| kind | suffix | example |
| --- | --- | --- |
| animation | ` + "`-anim`" + ` | ` + "`jump-anim`" + ` |
| state | ` + "`-state`" + ` | ` + "`jump-state`" + ` |
| marker | ` + "`-seg`" + ` | ` + "`jump-seg`" + ` |
| input | none | ` + "`jump`" + ` |

Inputs stay bare on purpose: they are the names a game passes to
` + "`Fire`" + `, so they are the sample's public surface and should read like
one. This is a convention of these samples, not something the format
requires.

## character/

Five clips and the machine a platformer actually needs. Each clip has its own
colour, so a state change is obvious the moment it happens.

- Event inputs are the verbs a game fires: ` + "`walk`, `run`, `stop`, `jump`, `hurt`" + `.
- ` + "`jump-state`" + ` is guarded on the boolean ` + "`grounded`" + `, which the game owns.
- ` + "`anywhere-state`" + ` is a global state: its transition applies from every
  state, which is how damage is reachable at any time.
- ` + "`jump-state`" + ` and ` + "`hurt-state`" + ` are one-shot. An OnComplete
  interaction fires ` + "`clipDone`" + `, so they return to idle without a
  game-side timer.
- Every state lists its jump transition first: order decides which transition
  wins when two apply on the same tick.

The loose ` + "`.json`" + ` clips are kept next to the bundle so Import can be tried
against them.

## combo/

Three one-shot clips chained end to end. Firing ` + "`attack`" + ` once plays
windup, strike and recover as a single motion: each clip ends, the
OnComplete interaction fires ` + "`clipDone`" + `, and the next state picks it up.
Watch the timeline under the stage to see each clip hand over.

## spritesheet/

One document holding three actions, named by Lottie markers, with each state
playing a range through ` + "`PlaybackState.segment`" + `. Use this one to see
segment playback rather than one-clip-per-state.
`
