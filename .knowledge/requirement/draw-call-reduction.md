---
id: requirement:draw-call-reduction
type: requirement
title: Draw Call Reduction
---

Reduce GPU draw calls for masked/matted/precomp content, currently linear
in animation count (metric:gpu-draw-cost) because per-layer offscreen
flushes break every merge chain (concept:ebitengine-draw-batching).
Follows decision:shared-offscreen-pool, which fixed textures/VRAM but not
draw count.

```yaml
approaches:  # orthogonal; implement in this order
  1: concept:idle-snapshot-cache      # idle players -> ~3 draws, cpu ~0
  2: concept:phase-compositing tier1  # playing -> ~10 draws, constant
  3: concept:phase-compositing tier2  # nested precomps, on demand
deferred: per-layer content cache for playing-but-static subtrees; needs
  geometry-equality keys and matte-source recursion, revisit after 1+2
acceptance:
  - pixel output identical to golden frames (examples/gpuprobe -compare)
  - draws constant in animation count for cached/batched paths
  - every gated path falls back to the recursive renderer; correctness
    never depends on the optimization applying
  - no regression in examples/stress p99 (metric:performance-targets)
motivation: draw calls cost most on WebGL/mobile; desktop draw time is
  already within budget
```
