---
id: requirement:verification
type: requirement
title: Verification Plan
---

```yaml
verification:
  - method: reference comparison
    detail: render same file in lottie-web; compare specific frames pixel by pixel
  - method: cross-editor testing
    detail: verify exports from all system:lottie-editors; each has JSON quirks
  - method: benchmarks
    detail: testing.B per-frame draw time; catch regressions vs metric:performance-targets
  - method: fallback verification
    detail: load files with unsupported features; confirm no panic (policy:robustness)
```
