---
id: requirement:scene-editor-mvp
type: requirement
title: Scene Editor MVP Scope
---

First shippable version of vision:scene-editor. Status: must-list implemented as the `cmd/lottie-layout/` Go module (tool name layout), a sibling of `cmd/lottie-state-editor/` in the workspace; should-list open.

```yaml
must:
  - open or create a scene file; add bundle references; one palette list carries every animation and machine as a placeable source (decision:scene-references-bundles) — no bundle list, since after Add there is nothing to do to a file
  - live preview of the selected source before placing; names alone do not say what a clip looks like
  - timeline pane per requirement:scene-timeline
  - scene settings pane: design size, hoverMovesFocus, initialFocus; canvas frames the design box and previews Contain/Cover mapping
  - place instances on the canvas; drag to move; reorder to change overlap (data:scene-document draw order)
  - inspector per selected node (requirement:selection-driven-ui pattern):
      animation node: segment, loop, loopCount, speed, mode, autoplay
      machine node: which machine, entry state override
      both: name, transform
  - several instances of one source with independent playback state
  - focus authoring per requirement:scene-focus-navigation
  - binding authoring per requirement:scene-interactions
  - preview runs api:scene-runtime with real keyboard/pad/mouse input (decision:runtime-package-first)
  - save and load data:scene-document preserving unknown fields
should:
  - snap and alignment guides
  - scale / rotation / opacity handles on canvas (inspector-only in must)
  - undo / redo
wont:
  - layout containers, anchoring, responsive reflow
  - text entry widgets; text lives in the clips (policy:out-of-scope)
deviations:
  - node source (bundle/id) is read-only in the inspector; re-place to change it
  - removing a bundle reference has no UI; edit the scene JSON (rare, and the reference is plainly visible there)
  - neighbor links are edited as dropdowns, not dragged on the canvas
  - the canvas focus overlay is outlines (green focusable, blue selected) without tab-order numbers
  - preview keys are fixed (Tab/arrows/Enter/Space/Esc), standing in for a game's own mapping
```

Node positions and z come from direct manipulation, so unlike requirement:editor-mvp the canvas is the primary authoring surface, not the inspector.
