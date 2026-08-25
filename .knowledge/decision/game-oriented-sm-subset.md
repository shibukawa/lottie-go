---
id: decision:game-oriented-sm-subset
type: decision
title: Game-Oriented State Machine Subset
---

Support the part of data:state-machine a game needs; skip web-page interactivity. Consistent with decision:practical-subset.

```yaml
supported:
  states: [PlaybackState, GlobalState]
  transitions: [Transition, Tweened]
  guards: [Event, Boolean, Numeric, String]
  inputs: [Event, Boolean, Numeric, String]
  interactions: [OnComplete, OnLoopComplete]
  actions: [Fire, Toggle, Increment, SetFrame, SetProgress]
deferred:
  interactions: [Click, PointerUp, PointerDown, PointerEnter, PointerMove, PointerExit]
  reason: game owns its own input; no DOM hit target
excluded:
  actions: [OpenUrl, SetTheme]
  reason: OpenUrl is web-only; SetTheme needs dotLottie themes, not implemented
```

Game code fires Event inputs directly (api:state-machine-runtime), so pointer interactions add nothing. OnComplete is mandatory, not optional: a one-shot clip such as "jump" must return to idle without a game-side timer.
