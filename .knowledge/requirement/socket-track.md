---
id: requirement:socket-track
type: requirement
title: Attachment Sockets
---

Status: proposed. Named per-frame transforms (position, rotation, scale) queryable from a running player, so gameplay attaches things to the animation.

```yaml
consumers:
  - particle emitters (muzzle flash, dust at feet)
  - held-item swap: weapon follows hand_r with correct orientation
  - projectile spawn origin
  - requirement:root-motion reads a root socket's displacement
socket:
  name: stable game-facing id (hand_r, muzzle, feet)
  transform: {x, y, angle, scale}; animation coordinates (rule:coordinates-and-alpha)
  z_hint: draw attached item in front of / behind the character
query: "player-side, per frame: SocketTransform(name); mirrored variant per rule:facing-mirror"
authoring_options:
  layer_binding:
    idea: bind socket name to a null layer (ty 3) the animator moves in their Lottie tool
    pro: interpolation identical to the animation; renderer already computes layer matrices; no editor keying UI
    needs: core query exposing a layer's world matrix; dep-free, so core-eligible unlike physics
    open: layer-name collisions; layers inside precomps
  editor_track:
    idea: frame-span stepped points like data:physics-resolv-track
    role: fallback when the source animation has no null layers
  recommendation: layer_binding primary, editor_track fallback
storage: thin mapping doc under extensions/ (decision:collision-static-plugins pattern) — socket name -> layer name + z_hint
```

Highest-value item of the gameplay annotations; unlocks requirement:root-motion nearly for free.
