---
id: concept:phase-compositing
type: concept
title: Load-Time Gated Phase Compositing
---

Proposed (requirement:draw-call-reduction, priority 2). Batch offscreen
work into phases over shared scratch atlases so masked/matted layers stop
flushing per layer. Gating is decided at decode: which layers need
offscreens and who nests in whom is static file structure; only region
geometry changes per frame.

```yaml
load_time_plan:
  tier0: no offscreen nodes -> plain path (already optimal)
  tier1: all nodes depth 1 -> phase path
  tier2_or_text_offscreen: fallback to recursive path (extendable later)
  prevalence: {tier0: 52, tier1: 8, tier2: 1}  # metric:gpu-draw-cost
phases_tier1:
  1: all layer contents -> scratch atlas A regions (fills batch across layers)
  2: all mask coverage + matte sources -> atlas B
  3: combine B->A destination-in; merges to ~1 draw (FillAll skips overlap check)
  4: composite A->screen in layer order; merges to ~1 draw, order preserved
per_frame: shelf packer over each atlas, reset per frame; packer overflow
  drops only the overflowing nodes to the recursive path that frame
expected: matte_x20 242 -> ~10 draws, constant in count, while playing
splits_not_breaks: [mixed fill rules, blend modes != normal]
tier2_extension: A->A draw is forbidden (self-draw), so nesting needs
  depth-parity ping-pong atlases; depth is known at load
```

Orthogonal to concept:idle-snapshot-cache (playing vs idle).
