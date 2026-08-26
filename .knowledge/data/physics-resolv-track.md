---
id: data:physics-resolv-track
type: data
title: resolv Hitbox Track Document
---

Frame-stepped hitboxes for SolarLune/resolv, defined by plugin lottieresolv (decision:collision-static-plugins). One document per animation, named by the animation id — boxes only mean anything against that clip's frames.

```yaml
storage: extensions/physics/resolv/<animID>.json
track: {boxes: [Box]}
box:
  name: editor label; index disambiguates duplicates
  kind: rect | circle | window (geometry-less timed flag, requirement:state-windows)
  tags: free-form strings (hit, hurt, push, ...); game-facing meaning; several allowed
  spans: [Span]; editor keeps frame order; queries take first covering span
span:
  active: frames [from, to); from inclusive, to exclusive
  geometry: constant per span — steps, not tweens (fighting-game convention)
  rect: {x, y: top-left, w, h}
  circle: {x, y: center, r}
coordinates: animation space
unknown_members: preserved via ExtraFields at every level (e.g. damage, knockback ride on a box or span)
lifecycle: removing a clip removes its track — editor's job, not the core's
```

Consumed through api:physics-resolv.
