---
id: requirement:editor-mvp
type: requirement
title: Editor MVP Scope
---

First shippable version of vision:state-machine-editor.

Implemented in the `editor/` module, a separate Go module so the library never depends on system:guigui.

```yaml
status: must-list complete; should-list partial
must:
  - open .lottie v1/v2 and plain .json clips
  - import clips into one bundle; rename and remove ids
  - create, rename, delete states; set animation, loop, speed, mode
  - draw transitions between states; edit their guards and their order
  - declare inputs; Event input names are the game-facing verbs
  - preview: run the machine, fire triggers by button, show current state
  - save as v2 preserving unknown fields
should:
  - done: validate missing initial, dangling toState, unknown animation id, unreachable state
  - open: undo / redo
deviations:
  - transitions are created in the inspector; the graph draws but does not author them
  - no file dialog; paths are typed into the toolbar
  - node positions live in State.Extra under "x-lottie-go-editor" (data:state-machine round-trip)
wont: policy:editor-out-of-scope
```

Preview runs the real interpreter (decision:runtime-package-first), so the editor cannot show behavior the game will not reproduce.
