---
id: metric:gpu-draw-cost
type: metric
title: GPU Draw Cost Baseline
---

Measured 2026-08-26, Apple M3, 512x512 screen, examples/lottie/gpuprobe with the
ebitenginedebug command log (requirement:verification). Explains where draw
calls go per concept:ebitengine-draw-batching.

```yaml
plain_shape_layers:
  draws: 6 at any animation count (vector deferred batching)
masked_or_matted_layers:  # layers-matte-above.json x20
  draws: 242 (~12 per animation, linear)
  anatomy: {stencil: 80, clear: 81, resolve: 40, matte_combine: 20, composite: 21}
  cause: per-layer offscreen flush; dst switch breaks every merge chain
antialias_off:
  draws: unchanged (242)
  triangles: 42804 -> 5704  # fill-rate lever only
after_decision_shared_offscreen_pool:  # x20
  textures: 44 -> 6 (flat in animation count)
  vram_mib: 246.1 -> 6.4
  draws: unchanged
asset_prevalence:  # 61 bundled files
  offscreen_free: 52
  flat_depth_1: 8
  nested_depth_2: 1
```

```yaml
after_draw_call_reduction:  # 2026-08-26
  idle_matte_x20: {draws: 242 -> 3, converges: frame 13}
  playing_16_masked_layers_one_comp: {draws: 197 -> 14}
  playing_nested_masked_precomp: {draws: 204 -> 24}
  playing_separate_single_node_animations: unchanged (by design)
```

CPU draw time is not the bottleneck (decision:defer-vertex-caching); draw
call count matters mainly for WebGL/mobile targets.
