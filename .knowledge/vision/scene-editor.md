---
id: vision:scene-editor
type: vision
title: Scene Layout Editor
---

Desktop tool that composes scenes — game screens and GUI menus — by placing instances of Lottie clips and state machines on a canvas. Third product layer: vision:lottie-player plays one clip, vision:state-machine-editor sequences clips into one actor, the scene editor arranges many actors into one screen. Games run the result through api:scene-runtime.

```yaml
scope:
  in:
    - place animation and machine instances; move, overlap (z-order), configure playback per node
    - author focus order and input bindings so a scene works as a pad/keyboard/mouse menu (requirement:scene-focus-navigation, requirement:scene-interactions)
    - live preview driving the real runtime with real input
  out:
    - artwork editing (policy:editor-out-of-scope)
    - game logic; scenes emit named callbacks, the game implements them
    - layout containers, text flow, 9-slice; placement is fixed coordinates
built_with: system:guigui
tool_name: layout — ships as the cmd/lottie-layout/ Go module, a sibling of cmd/lottie-state-editor/; scene files save as *.scene.json
samples:
  - examples/layout/opening-animation — a full game opening (phases, entrance timeline, chained playback, text, callbacks) runnable and editor-openable
  - examples/layout/searchlight — requirement:scene-camera as a story; a depth-0 mask is the searchlight, per-phase cameras are the stops, the game eases between them; a complete-event binding startles the found mouse
document: data:scene-document
decided:
  - storage: standalone file referencing bundles (decision:scene-references-bundles)
  - resolution: scene holds a design size; mapping to the real screen (centering, letterbox, side crop) is a runtime concern (api:scene-runtime screen mapping)
  - hit region: animation viewport box first, finer shapes later (requirement:scene-interactions)
  - nesting: none — scenes are flat; sub-menus and dialogs are separate scenes the game overlays as extra ScenePlayers (api:scene-runtime)
  - lifecycle: intro/main/outro are phases inside one scene (requirement:scene-phases), not separate files
  - static content: image and text nodes live beside animations in data:scene-document; text is named and overwritable at runtime (SetText)
```

Scenes are not part of dotLottie 2.0, so this is a lottie-go extension by design — unlike decision:align-dotlottie-state-machine there is no upstream schema to track. Editor scope: requirement:scene-editor-mvp.
