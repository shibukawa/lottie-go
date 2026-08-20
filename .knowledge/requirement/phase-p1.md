---
id: requirement:phase-p1
type: requirement
title: Phase P1 Production Ready
---

```yaml
features:
  - gradient fill / stroke: "ty: gf / gs, via Kage shader (concept:kage-shader-usage)"
  - trim path: "ty: tm, custom path subdivision"
  - image layer: "ty: 2, embedded assets only"
  - precomposition layer: "ty: 0, offscreen rendering"
  - solid layer: "ty: 1"
  - masks: "masksProperties, Add / Subtract"
  - track matte: "tt, alpha / alpha-inverted / luma / luma-inverted"
  - blend modes: "bm, normal / multiply / screen / add"
  - dotlottie container: requirement:input-formats
```

Luma matte and non-normal blend modes need Kage compositing (concept:kage-shader-usage). Image assets premultiplied at load (rule:coordinates-and-alpha).
