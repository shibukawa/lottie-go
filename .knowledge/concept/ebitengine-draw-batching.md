---
id: concept:ebitengine-draw-batching
type: concept
title: Ebitengine Draw Batching and Atlas Migration
---

How system:ebitengine merges draws and manages its texture atlas. Verified
against v2.9.10 source and by experiment (2026-08-26); load-bearing for
requirement:draw-call-reduction.

```yaml
draw_merge_conditions:  # internal/graphicscommand/command.go
  all_equal: [shader, uniforms, dst, src_backends, blend, fill_rule]
  overlap_check: skipped for FillRuleFillAll (DrawImage); required otherwise
  note: src identity is the backend (atlas page), not the user image
vector_pkg:
  fill_batching: FillPath defers per-dst; flushes when dst is next used or
    antialias/blend/fill_rule changes between calls
  method: stencil buffer (Loop-Blinn); cost scales with bbox area
  antialias: 8 sample offsets; multiplies triangles, not draw calls
atlas:
  render_into_isolates: drawing into an image removes it from the shared
    source atlas (ensureIsolatedFromSource)
  rejoin_rule: usedAsSourceCount >= 10 * 2^min(usedAsDestinationCount, 31)
  dst_count_never_resets: repeated re-bakes into one image defer rejoin
    forever; allocate a fresh image per bake to keep the 10-frame rejoin
  writepixels_exempt: WritePixels does not isolate an image
```

Experiment: 20 images baked once then composited every frame merge from 22
draws to 3 draws at frame 12 (textures 24 to 4), stable thereafter.

Consequences: plain shape layers batch fully (6 draws at any animation
count); offscreens are isolated textures, so their count and their
composites are what cost (decision:shared-offscreen-pool).
