---
id: requirement:collision-authoring
type: requirement
title: Collision Authoring
---

Editor authors collision data on top of clips so games get hitboxes that move, appear, and disappear with the animation. Tool-specific extension; not part of dotLottie (stored per decision:collision-static-plugins).

```yaml
payloads:
  body_silhouette:
    scope: bundle
    shapes: [circle, box, convex-polygon]
    animated: false
    consumer: jakecoffman/cp rigid body; register into Space and go
  hitbox_track:
    scope: per animation
    shapes: [rect, circle]
    animated: frame spans, step (fighting-game style), no tweening
    consumer: SolarLune/resolv detection
tags:
  purpose: game-facing meaning of a box; several kinds coexist (hit, hurt, push/walk collision)
  free_form: true; a box may carry several
  query: game pulls boxes by tag without knowing names
editing:
  - display boxes over the stage, colored by tag
  - drag to move and resize on the stage
  - spans control when a box exists; editable against the playhead
acceptance:
  - data round-trips through save/reopen and survives foreign editors
  - runtime query by frame and tag (api:physics-resolv)
  - one-call registration into a cp Space (api:physics-cp)
```

UI: ui:collision-editing. Formats: data:physics-cp-body, data:physics-resolv-track.
