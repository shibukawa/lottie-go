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
  - method: golden frame comparison
    detail: examples/lottie/gpuprobe -golden/-compare renders 6 frames of every
      bundled asset to PNG; rendering changes must show pixel-neutrality
  - method: gpu command inspection
    detail: ebitenginedebug build dumps per-frame draw commands and texture
      census; gpuprobe -summarize tracks metric:gpu-draw-cost
```
