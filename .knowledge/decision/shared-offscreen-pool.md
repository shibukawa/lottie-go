---
id: decision:shared-offscreen-pool
type: decision
title: Shared Offscreen Pool Sized to Layer Bounds
---

Implemented 2026-08-26 (merged to main). Offscreens for masks, mattes, and
precomps come from one process-wide pool instead of per-Player pools, and
cover only the layer's own bounds instead of the destination. Rationale:
offscreens are permanently isolated textures (concept:ebitengine-draw-batching),
so their count is what matters, and it must track composition depth, not
animation count.

```yaml
design:
  pool: process-wide, mutex-guarded, per-bucket cap 4, Deallocate on overflow
  bucketing: pow2 to 256 then 128-step; absorbs per-frame bounds drift
  exact_path: resampled precomp offscreens skip bucketing (backing size
    shifts bilinear sampling by 1/255)
  bounds: convex hull of bezier control points + stroke reach pad; masks and
    mattes only remove coverage, so content bounds are safe for every matte
    mode; text falls back to dst bounds (extent known only after shaping)
results: {textures_x20: 44 -> 6, vram_x20_mib: 246.1 -> 6.4, pixels: identical
  over 360 golden frames}
fixed_in_passing:
  - square line caps reach width/2*sqrt2; bounds pad was width/2
  - executeGradient returned an offscreen to the pool with a deferred
    vector fill still pending when the gradient matrix was singular
```

Draw calls unchanged (242 at x20); reduction is requirement:draw-call-reduction.
