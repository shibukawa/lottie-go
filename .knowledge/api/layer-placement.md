---
id: api:layer-placement
type: api
title: Layer Placement Query
---

Core-side query for a named layer's world transform, the foundation of
requirement:socket-track and requirement:root-motion. Dependency-free, so
it lives in the core, not a plugin. Implemented.

```yaml
api:
  - "anim.LayerPlacement(name, frame) (LayerPlacement, bool)  // false: no such layer"
  - "player.LayerPlacement(name)  // at the current frame"
  - "anim.LayerNames() []string   // reachable names, first-seen order, deduped"
placement: {X, Y: local origin in animation coords, Angle: radians y-down, ScaleX, ScaleY, Visible}
  - "GeoM() — scale·rotate·translate for attaching sprites (skew dropped)"
  - "Mirrored(axis) — facing flip per rule:facing-mirror"
resolution:
  search: root layers in file order, then precomp contents depth-first; first match wins (name collisions resolved by order — give attachment layers unique names)
  precomp: time remapped via the precomp layer's start/stretch, matrix composed, exactly as rendering does
  visible: layer and every enclosing precomp inside ip/op and not hidden; out-of-window queries still answer
```

Consumed by api:sockets; the null-layer convention is the animator's side
of the contract.
