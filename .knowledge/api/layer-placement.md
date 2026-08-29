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
  - "anim.LayerTransform(name, frame) (ebiten.GeoM, bool)  // the composed matrix, undecomposed"
  - "anim.LayerNames() []string   // reachable names, first-seen order, deduped"
placement: {X, Y: local origin in animation coords, Angle: radians y-down, ScaleX, ScaleY, Visible}
  - "GeoM() — scale·rotate·translate for attaching sprites (skew dropped)"
  - "Mirrored(axis) — facing flip per rule:facing-mirror"
resolution:
  search: root layers in file order, then precomp contents depth-first; first match wins (name collisions resolved by order — give attachment layers unique names)
  precomp: time remapped via the precomp layer's start/stretch, matrix composed, exactly as rendering does
  visible: layer and every enclosing precomp inside ip/op and not hidden; out-of-window queries still answer
```

LayerTransform is there because the decomposition cannot express a mirror:
ScaleX and ScaleY are axis lengths, so a layer scaled -100% reads back as
positive scale and an angle a half turn away. Attaching a sprite does not
care; reproducing the layer's own frame does, which is what
requirement:pose-editing needs to outline a flipped part and to turn a drag
back into the parent's space.

Resolution by name is the contract's sharp edge: an unnamed layer cannot be
found and a duplicated one finds whichever came first, silently. Callers
that convert a drag through a layer's matrix — requirement:pose-editing —
must check before trusting the answer, because the failure is a wrong
matrix rather than a missing one.

Consumed by api:sockets; the null-layer convention is the animator's side
of the contract.
