---
id: concept:kage-shader-usage
type: concept
title: Kage Shader Usage
---

Kage shaders required where `vector` fills cannot express the effect:

```yaml
required_for:
  - linear / radial gradients (gf / gs)
  - luma matte (luminance -> alpha)
  - blend modes multiply / screen and similar
```

Gradient complexity is a known risk; excluded from P0, asset authors use solid colors until requirement:phase-p1 (policy:risks).
