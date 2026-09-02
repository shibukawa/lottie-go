---
id: requirement:playback-speed
type: requirement
title: Variable Playback Speed
---

Mid-playback speed change. Status: implemented.

```yaml
implemented:
  - "runtime: Player.SetSpeed(float64) callable anytime mid-playback; 1 = normal, negative = reverse (api:player-api)"
  - "scene doc: playback.speed per node and per chain step (ScenePlayStep.Speed) — a chain already expresses per-clip speed changes (api:scene-runtime)"
  - "state machine: State.Speed per state (api:state-machine-runtime)"
  - "layout tool: inspector speed field on node playback AND on each Then step (step rows show a ×N tag when off 1); pass-duration math honors both"
  - "in-clip speed curves: Lottie time remap (tm) on precomps is supported — author speed ramps inside the clip itself"
wont:
  - no keyframed speed track at scene level, consistent with requirement:scene-timeline wont (motion lives in clips); games needing dynamic ramps call SetSpeed per frame
```
