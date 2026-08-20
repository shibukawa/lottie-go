---
id: rule:coordinates-and-alpha
type: rule
title: Coordinates and Alpha Rules
---

```yaml
rules:
  - lottie top-left origin, y-down == ebitengine; no conversion
  - ebitengine images use premultiplied alpha; premultiply embedded
    image assets at load time
  - apply transforms to vertices during path construction, not via
    ebiten.GeoM, so stroke width scales per Lottie spec
```
