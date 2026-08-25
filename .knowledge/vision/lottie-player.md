---
id: vision:lottie-player
type: vision
title: Lottie Subset Renderer for Ebitengine
---

Pure-Go Lottie subset player rendered through system:ebitengine `vector` package. Plays UI motion assets authored in AE-independent editors (system:lottie-editors) directly inside an Ebitengine game.

```yaml
scope:
  in:
    - UI motion, icons, loaders, HUD elements
    - screen transitions, logo animations
    - clip sequencing driven by a state machine (vision:state-machine-editor)
  out:
    - skeletal character rigs (use Spine / nijilive)
    - full-screen complex vector animation (performance)
principles:
  - decision:no-cgo
  - decision:practical-subset
  - decision:use-vector-package
```

Phases: requirement:phase-p0 (minimum viable), requirement:phase-p1 (production), requirement:phase-p2 (optional), requirement:player-state-machine (state machine playback). Excluded features: policy:out-of-scope. Quality gates: metric:performance-targets, requirement:verification.

Game verbs such as walk / run / jump are served by sequencing short pre-authored clips, not by rigging; the exclusion above is about skeletal deformation only.
