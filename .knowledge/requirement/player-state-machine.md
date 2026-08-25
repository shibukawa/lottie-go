---
id: requirement:player-state-machine
type: requirement
title: Player State Machine Support
---

Library work behind api:state-machine-runtime, sequenced by decision:runtime-package-first. Complete.

```yaml
done:
  - id: v2-bundle
    what: Bundle reads a/ i/ s/ t/ f/ and the v1 animations/ images/ names; Encode always writes v2
    note: a manifest that is absent or stale is reconciled against the files present
  - id: multi-animation
    what: one Bundle holds every animation, state machine, theme, image, and font
    note: animations decode on first use; DecodeDotLottie delegates to Bundle
  - id: markers
    what: Animation.Markers and Animation.Marker expose named frame ranges
  - id: sm-document
    what: data:state-machine parses and rewrites, preserving unmodeled members
  - id: segment-playback
    what: Player.SetRange and SetMarkerRange limit playback to a marker
    note: Seek, Position, Progress, and Duration all follow the active range
  - id: completion-event
    what: Player.OnComplete and OnLoopComplete, with SetLoopCount for n passes
  - id: reverse-playback
    what: Player.SetReverse; Rewind starts at the range end when reversed
  - id: interpreter
    what: StateMachinePlayer evaluates guards, queues fired events, runs actions
```

Frame advance stays tied to ebiten.TPS, consistent with api:player-api. Remaining work is requirement:editor-mvp, the guigui UI.
