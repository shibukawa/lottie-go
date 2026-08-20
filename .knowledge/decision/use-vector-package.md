---
id: decision:use-vector-package
type: decision
title: Build on Ebitengine vector Package
---

Use system:ebitengine `vector` package as the rasterizer; do not write a custom one. Requires Ebitengine v2.9+ (mandatory) because v2.9 reworked the vector API:

```yaml
v2.9_apis:
  - vector.FillPath / vector.StrokePath
  - vector.DrawPathOptions / vector.FillOptions
  - vector.FillRuleNonZero / vector.FillRuleEvenOdd
  - Path.AddPath / AddStroke / Bounds / Reset
deprecated:
  - AppendVerticesAndIndicesForFilling
  - AppendVerticesAndIndicesForStroke
```

v2.9 internals move from stencil-buffer rendering to polygon triangulation, which fits Lottie rendering. Mapping details: concept:lottie-to-vector-mapping.
