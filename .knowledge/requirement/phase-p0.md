---
id: requirement:phase-p0
type: requirement
title: Phase P0 Minimum Viable
---

Covers the majority of assets from existing editors. All items required before P1.

```yaml
features:
  - lottie json parser: [ip, op, fr, w, h, layers]
  - transform: [a anchor, p position, s scale, r rotation, o opacity]
  - keyframe interpolation: [linear, hold, bezier easing]
  - shape layer: "ty: 4"
  - path: "ty: sh (cubic bezier)"
  - rect: "ty: rc"
  - ellipse: "ty: el"
  - nested group: "ty: gr, transform composition"
  - solid fill: "ty: fl (NonZero / EvenOdd)"
  - solid stroke: "ty: st (width, line cap, line join)"
  - layer parenting: parent
  - layer visibility: ip/op in-out points
```

Rendering approach: concept:lottie-to-vector-mapping, flow:render-frame.
