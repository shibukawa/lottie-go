// Command gensamples writes the editor's sample bundles under
// testdata/editor. The samples are generated rather than downloaded so their
// licensing is unambiguous: every clip here is authored in this repository.
//
//	go run ./gensamples
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	lottie "github.com/shibukawa/lottie-go"
)

// editorExtraKey must match the editor's own constant: it is where node
// positions ride so a sample opens with a laid-out graph.
const editorExtraKey = "x-lottie-go-editor"

func main() {
	out := flag.String("out", filepath.Join("..", "testdata", "editor"), "output directory")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(out string) error {
	if err := writeSample(filepath.Join(out, "character"), "character",
		map[string]obj{
			"idle": idleClip(), "walk": walkClip(), "run": runClip(),
			"jump": jumpClip(), "hurt": hurtClip(),
		}, characterMachine()); err != nil {
		return err
	}
	if err := writeSample(filepath.Join(out, "spritesheet"), "spritesheet",
		map[string]obj{"actions": sheetClip()}, sheetMachine()); err != nil {
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
	b := lottie.NewBundle()
	for id, clip := range clips {
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
	b.Manifest().Generator = "lottie-go/editor gensamples"
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
		Initial: "idle",
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
			at(loopState("idle", "idle", []lottie.Transition{
				to("jump", event("jump"), grounded),
				to("run", event("run")),
				to("walk", event("walk")),
			}), 30, 140),
			at(loopState("walk", "walk", []lottie.Transition{
				to("jump", event("jump"), grounded),
				to("run", event("run")),
				to("idle", event("stop")),
			}), 250, 30),
			at(loopState("run", "run", []lottie.Transition{
				to("jump", event("jump"), grounded),
				to("walk", event("walk")),
				to("idle", event("stop")),
			}), 470, 30),
			at(onceState("jump", "jump", []lottie.Transition{
				to("idle", event("clipDone")),
			}), 250, 250),
			at(onceState("hurt", "hurt", []lottie.Transition{
				to("idle", event("clipDone")),
			}), 470, 250),
			// A global state's transitions apply from every state, which is
			// how "take damage at any time" is expressed.
			at(lottie.State{
				Name: "anywhere", Type: lottie.StateGlobal,
				Transitions: []lottie.Transition{to("hurt", event("hurt"))},
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
		s := lottie.State{
			Name: name, Type: lottie.StatePlayback, Animation: "actions",
			Segment: marker, Loop: loop, Autoplay: true, Transitions: trs,
		}
		return s
	}
	return &lottie.StateMachine{
		Initial: "idle",
		Inputs: []lottie.Input{
			{Type: lottie.InputEvent, Name: "walk"},
			{Type: lottie.InputEvent, Name: "stop"},
			{Type: lottie.InputEvent, Name: "jump"},
			{Type: lottie.InputEvent, Name: "clipDone"},
		},
		States: []lottie.State{
			at(seg("idle", "idle", true, []lottie.Transition{
				to("jump", event("jump")),
				to("walk", event("walk")),
			}), 30, 40),
			at(seg("walk", "walk", true, []lottie.Transition{
				to("jump", event("jump")),
				to("idle", event("stop")),
			}), 250, 40),
			at(seg("jump", "jump", false, []lottie.Transition{
				to("idle", event("clipDone")),
			}), 470, 40),
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

    cd editor && go run . ../testdata/editor/character/character.lottie

## character/

Five clips and the machine a platformer actually needs. Each clip has its own
colour, so a state change is obvious the moment it happens.

- Event inputs are the verbs a game fires: ` + "`walk`, `run`, `stop`, `jump`, `hurt`" + `.
- ` + "`jump`" + ` is guarded on the boolean ` + "`grounded`" + `, which the game owns.
- ` + "`anywhere`" + ` is a global state: its transition applies from every state,
  which is how damage is reachable at any time.
- ` + "`jump`" + ` and ` + "`hurt`" + ` are one-shot. An OnComplete interaction fires
  ` + "`clipDone`" + `, so they return to idle without a game-side timer.
- Every state lists its jump transition first: order decides which transition
  wins when two apply on the same tick.

The loose ` + "`.json`" + ` clips are kept next to the bundle so Import can be tried
against them.

## spritesheet/

One document holding three actions, named by Lottie markers, with each state
playing a range through ` + "`PlaybackState.segment`" + `. Use this one to see
segment playback rather than one-clip-per-state.
`
