package lottie_test

import (
	"bytes"
	"fmt"
	"log"

	lottie "github.com/shibukawa/lottie-go"
)

// clip is a placeholder Lottie document; a real one comes from an editor.
func clip(frames int, markers string) []byte {
	m := ""
	if markers != "" {
		m = `,"markers":` + markers
	}
	return fmt.Appendf(nil, `{"v":"5.9.0","fr":60,"ip":0,"op":%d,"w":100,"h":100,
		"layers":[{"ty":3,"ind":1,"ip":0,"op":%d,"st":0,
		"ks":{"a":{"a":0,"k":[0,0]},"p":{"a":0,"k":[50,50]},
		"s":{"a":0,"k":[100,100]},"r":{"a":0,"k":0},"o":{"a":0,"k":100}}}]`+m+`}`,
		frames, frames)
}

// Building a bundle of clips, wiring a state machine over them, and driving
// it the way a game would: fire a named trigger, then tick.
func Example_stateMachine() {
	b := lottie.NewBundle()
	if err := b.SetAnimation("idle", clip(60, "")); err != nil {
		log.Fatal(err)
	}
	if err := b.SetAnimation("jump", clip(3, "")); err != nil {
		log.Fatal(err)
	}
	if err := b.SetStateMachine("character", &lottie.StateMachine{
		Initial: "idle",
		Inputs: []lottie.Input{
			{Type: lottie.InputEvent, Name: "jump"},
			{Type: lottie.InputEvent, Name: "landed"},
		},
		States: []lottie.State{{
			Name: "idle", Type: lottie.StatePlayback, Animation: "idle",
			Loop: true, Autoplay: true,
			Transitions: []lottie.Transition{{
				Type: lottie.TransitionImmediate, ToState: "jump",
				Guards: []lottie.Guard{{Type: lottie.GuardEvent, InputName: "jump"}},
			}},
		}, {
			Name: "jump", Type: lottie.StatePlayback, Animation: "jump",
			Autoplay: true,
			Transitions: []lottie.Transition{{
				Type: lottie.TransitionImmediate, ToState: "idle",
				Guards: []lottie.Guard{{Type: lottie.GuardEvent, InputName: "landed"}},
			}},
		}},
		// When the jump clip ends, raise "landed" so the machine returns to
		// idle on its own.
		Interactions: []lottie.Interaction{{
			Type:    lottie.InteractionOnComplete,
			Actions: []lottie.Action{{Type: lottie.ActionFire, InputName: "landed"}},
		}},
	}); err != nil {
		log.Fatal(err)
	}

	// Save it as a .lottie file with b.Encode(w); here we keep it in memory.
	var archive bytes.Buffer
	if err := b.Encode(&archive); err != nil {
		log.Fatal(err)
	}
	loaded, err := lottie.DecodeBundle(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	if err != nil {
		log.Fatal(err)
	}

	sm, err := loaded.NewStateMachinePlayer("character")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("start:", sm.State())

	sm.Fire("jump") // what a key press would do
	for range 4 {
		sm.Update() // in ebiten.Game.Update; then sm.Draw(screen, nil)
		fmt.Println("tick: ", sm.State())
	}

	// Output:
	// start: idle
	// tick:  jump
	// tick:  jump
	// tick:  jump
	// tick:  idle
}
