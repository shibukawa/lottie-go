---
id: api:physics-cp
type: api
title: cp Plugin API (lottiecp)
---

Package lottiecp: typed IO for data:physics-cp-body plus jakecoffman/cp wiring. Import-gated (decision:collision-static-plugins). Implemented.

```yaml
bundle_io:
  - "lottiecp.IDs(b *lottie.Bundle) []string"
  - "lottiecp.Load(b, id) (*Body, error)   // parses afresh; edit in place, Store back"
  - "lottiecp.Store(b, id, *Body) error"
  - "lottiecp.Remove(b, id)"
engine:
  - "lottiecp.Build(*Body) (*cp.Body, []*cp.Shape)  // not yet in a space"
  - "lottiecp.AddToSpace(space, *Body) (*cp.Body, []*cp.Shape)  // ready for SetPosition"
build_semantics:
  moment_zero: derived per shape (MomentForCircle/Box2/Poly), mass split evenly
  polygon: NewPolyShape computes convex hull
  material: friction/elasticity/sensor applied per shape
  unknown_shape_type: skipped, others still build
```

Serves requirement:collision-authoring acceptance "one-call registration".
