---
id: requirement:frame-events
type: requirement
title: Frame Events with Payload
---

Status: implemented as api:frame-events (plugin/events + Player.OnFrameSpan); editor authoring UI not built yet. One-shot triggers at exact frames carrying free-form payload, for sounds, effects, and game logic synced to animation.

```yaml
existing_coverage:
  markers: Lottie markers already fire via Player.OnMarker / StateMachinePlayer.OnMarker (api:state-machine-runtime); editor counts hits
  gap:
    - payload (sound id, volume, shake magnitude) — markers carry only name + range
    - repeated same-name events at different frames without polluting the Lottie document
event: {frame, name, payload: free JSON}
emission: reuse the marker crossing logic (speed, reverse, loop safe)
storage: per-animation doc under extensions/ (decision:collision-static-plugins pattern)
priority: defer until markers prove insufficient
```

Sits beside requirement:socket-track (an event names where via a socket: spawn at muzzle).
