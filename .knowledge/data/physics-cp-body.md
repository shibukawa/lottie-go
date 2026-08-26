---
id: data:physics-cp-body
type: data
title: cp Body Document
---

Rigid-body silhouette for jakecoffman/cp, defined by plugin lottiecp (decision:collision-static-plugins). Bundle-level: one silhouette serves every clip, so it is not keyframed.

```yaml
storage: extensions/physics/cp/<id>.json; editor manages id "body", leaves others untouched
body:
  type: dynamic (default) | kinematic | static
  mass: dynamic only; <=0 reads as 1
  moment: 0 means derive from shapes
  shapes: [Shape]
shape:
  type: circle | box | polygon
  circle: {center: {x, y}, radius}
  box: {center, width, height, radius: corner rounding}
  polygon: {vertices: [{x, y}], radius}; convex; hull recomputed on build
  material: {friction, elasticity, sensor}
coordinates: animation space (y down), passed to cp untouched
unknown_members: preserved via ExtraFields at every level
unknown_shape_type: skipped on build, kept in data (future format growth)
```

Consumed through api:physics-cp.
