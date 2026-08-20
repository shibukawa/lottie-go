---
id: flow:render-frame
type: flow
title: Render Pipeline
---

```yaml
flow:
  trigger: game Draw calls api:player-api Draw
  steps:
    - id: parse
      action: parse Lottie JSON once into IR (layer tree + property tracks)
      note: load-time only
    - id: evaluate
      action: evaluate property tracks at time t
    - id: build-path
      action: build vector.Path; reuse cache when geometry unchanged
      ref: policy:performance-caching
    - id: draw
      action: FillPath / StrokePath / DrawTrianglesShader
      ref: concept:lottie-to-vector-mapping
    - id: composite
      action: offscreen compositing where masks / mattes / blends require
      ref: concept:kage-shader-usage
  failure:
    unsupported_feature: skip element, record, continue (policy:robustness)
```
