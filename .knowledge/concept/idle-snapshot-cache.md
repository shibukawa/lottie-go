---
id: concept:idle-snapshot-cache
type: concept
title: Idle Player Snapshot Cache
---

Implemented 2026-08-26 (Player.drawSnapshot; opt out via
Player.SetSnapshotCache). When a Player's draw inputs repeat, bake the whole animation into one offscreen and composite
that; Ebitengine then migrates the read-only bake onto its source atlas
after 10 frames and the composites of every idle player merge
(concept:ebitengine-draw-batching).

```yaml
cache_key: [frame, root_matrix, color_scale, antialias, dst_bounds, generation]
generation: Animation counter bumped by SetFontResolver, so resolver swaps
  invalidate snapshots
bake_rule: key must repeat 2 consecutive frames; first frame renders direct
fresh_image_per_bake: reuse would grow usedAsDestinationCount and defer
  atlas rejoin exponentially
invalidate: any key change -> Deallocate, return to direct rendering
guard: disable per animation when any layer blend mode != normal
  (flattening changes blending against the backdrop under the animation)
color_scale: baked in, so key includes it (alpha scaling is nonlinear
  across overlapping layers)
measured: {idle_matte_x20_draws: 242 -> 3 at frame 13, textures: 26 -> 6 at frame 12, idle_cpu: ~0}
vram: one bounds-sized texture per idle player, freed on resume
```

Orthogonal to concept:phase-compositing (idle vs playing). Covers all layer
types including plain shapes, so idle CPU eval also drops to zero.
