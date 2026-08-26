---
id: api:sockets
type: api
title: Sockets Plugin API (lottiesockets)
---

Package plugin/sockets (core subpackage, zero deps): the bundle-stored
socket table over api:layer-placement. Serves requirement:socket-track
and requirement:root-motion. Implemented.

```yaml
storage: extensions/sockets.json, bundle-level (socket names are a cross-clip convention)
socket: {name: game-facing id, layer: bound layer (empty = name), z: front|behind, dx/dy: layer-local trim applied through the layer transform — rides rotation and scale; the layer stays the position source of truth}
bundle_io:
  - "lottiesockets.Load(b) (*Set, error) / Store(b, *Set) / Remove(b)"
query:
  - "set.At(anim, frame, name) (Placed, bool)   // Placed = Socket + LayerPlacement"
  - "set.All(anim, frame) []Placed              // skips layers this clip lacks"
  - "Placed.Mirrored(axis)                      // Z unchanged: front stays front"
root_motion:
  - "lottiesockets.Displacement(anim, layer, from, to) (dx, dy, ok)"
  - games diff between Updates; on loop wrap apply remainder to clip end and re-base
editor: ui:collision-editing socket row binds stage layers, toggles z, draws cyan crosses with an angle tick
```
