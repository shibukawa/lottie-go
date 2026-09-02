---
id: vision:state-machine-editor
type: vision
title: Lottie State Machine Editor
---

Desktop authoring tool that bundles short Lottie clips into one dotLottie v2 archive (data:bundle-layout) and defines a state machine (data:state-machine) over them. Games drive playback by verb ("idle", "walk", "run", "jump") through api:state-machine-runtime instead of tracking frame ranges.

```yaml
scope:
  in:
    - import many .json / .lottie clips into one bundle
    - author states, transitions, guards, inputs
    - live preview with manual trigger firing
    - read v1 and v2 archives; always write v2
    - agent access: the same Model operations over MCP (requirement:editor-mcp)
  out: policy:editor-out-of-scope
built_with: system:guigui
principles:
  - decision:align-dotlottie-state-machine
  - decision:game-oriented-sm-subset
  - decision:runtime-package-first
```

Extends vision:lottie-player from UI motion into game-driven clip sequencing. The editor ships as the first product, but requirement:player-state-machine is the first code written, because the preview runs the real interpreter.
