---
id: api:state-machine-runtime
type: api
title: State Machine Runtime API
---

Go API consumed by games and by the editor preview (decision:runtime-package-first). Extends api:player-api. Implemented.

```yaml
loading:
  - "lottie.DecodeBundle(r io.ReaderAt, size int64) (*Bundle, error)"
  - "b.Animation(id string) (*Animation, error)"
  - "b.StateMachineIDs() []string"
  - "b.NewStateMachinePlayer(id string) (*StateMachinePlayer, error)  // empty id: the manifest's initial, else the first"
driving:
  - "sm.Fire(input string)              // Event input: walk, run, jump"
  - "sm.Set(name string, v T)          // generic method, Go 1.27"
  - "sm.Get[T](name string) (T, bool)  // type must be explicit"
  - "sm.State() string"
  - "sm.OnStateChanged(func(from, to string))"
  - "sm.Player() *Player                // active clip, for a scrub bar"
  - "sm.SetMachine(id string) error   // swap the whole animation set"
  - "sm.MachineID() string"
  - "sm.Definition() *StateMachine"
  - "sm.Err() / sm.UnsupportedFeatures()"
game_loop:
  - "sm.Update() in ebiten.Game.Update"
  - "sm.Draw(screen, *lottie.DrawOptions) in Draw"
```

Naming: data:state-machine is the document type `StateMachine`; `StateMachinePlayer` is the running instance, mirroring Animation and Player.

Set and Get are generic methods, so one pair covers Boolean, Numeric, and String inputs. The constraint admits every numeric width plus named types, because a narrow `bool | float64 | string` would reject an untyped constant: `Set("speed", 5)` infers int. This raises the module's minimum to Go 1.27 (decision:generic-input-accessors).

Update advances the active clip, runs the OnComplete / OnLoopComplete interactions it triggers, then takes transitions until none applies. Fire queues an event for the next Update, so firing anywhere in a frame is safe; an event is consumed by the transition that takes it, so one Fire moves one step. Actions fire events visible in the same Update, which is what returns a one-shot clip to idle on the tick it ends. Unknown inputs never match and never panic (policy:robustness).

A bundle may hold several machines. The schema has no action that switches between them, so they are alternative entry points a host picks: SetMachine is a lottie-go convenience that changes nothing about the document. The new machine enters its own initial state, but value inputs carry over by name — they describe the game's world, not the machine — and it settles against them on entry. Events do not carry.

Transition precedence: the current state's own transitions in declaration order, then each GlobalState's. Chains are capped per Update so a cycle cannot hang the game loop.
