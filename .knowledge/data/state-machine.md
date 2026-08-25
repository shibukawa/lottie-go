---
id: data:state-machine
type: data
title: State Machine Document
---

One JSON file in `s/` of data:bundle-layout. Schema follows dotLottie 2.0; the implemented subset is decision:game-oriented-sm-subset.

```yaml
root:
  initial: string; name of entry state
  states: list of PlaybackState or GlobalState
  inputs: list of variable definitions
  interactions: list of listeners
playback_state:
  name: string; unique
  type: PlaybackState
  animation: string; animation id in a/
  segment: string; optional marker range
  loop: bool
  loopCount: int
  autoplay: bool
  mode: Forward | Reverse
  speed: float
  transitions: list
  entryActions: list
  exitActions: list
global_state:
  name: string
  type: GlobalState
  note: no animation; its transitions apply from any state
transition:
  type: Transition | Tweened
  toState: string
  guards: list; all must pass
  duration: float; Tweened only
  easing: 4 floats; Tweened cubic bezier
guard:
  type: Event | Boolean | Numeric | String
  inputName: string
  conditionType: Equal | NotEqual | GreaterThan | GreaterThanOrEqual | LessThan | LessThanOrEqual
  compareTo: value; omitted for Event
input:
  type: Event | Boolean | Numeric | String
  name: string
  value: initial value; omitted for Event
```

Transitions are evaluated in declaration order and the first whose guards all pass fires. Order is therefore semantic, and ui:editor-shell must expose it as editable.
