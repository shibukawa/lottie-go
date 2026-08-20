---
id: concept:lottie-to-vector-mapping
type: concept
title: Lottie to vector Package Mapping
---

A Lottie shape reduces to "cubic bezier path + fill + stroke + transform", mapping nearly 1:1 onto `vector.Path` (decision:use-vector-package).

```yaml
mapping:
  bezier_path: Path.MoveTo / LineTo / CubicTo / Close
  rect_ellipse: Path.Arc / ArcTo or bezier approximation
  fill_rule: FillRuleNonZero / FillRuleEvenOdd
  solid_fill: vector.FillPath + FillOptions
  stroke: vector.StrokePath + StrokeOptions (LineCap / LineJoin)
  gradient: DrawTrianglesShader + Kage (concept:kage-shader-usage)
  mask_matte: offscreen ebiten.Image + Kage compositing
```

Coordinate and alpha conventions: rule:coordinates-and-alpha.
