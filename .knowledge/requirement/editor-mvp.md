---
id: requirement:editor-mvp
type: requirement
title: Editor MVP Scope
---

First shippable version of vision:state-machine-editor.

```yaml
must:
  - open .lottie v1/v2 and plain .json clips
  - import clips into one bundle; rename and remove ids
  - create, rename, delete states; set animation, loop, speed, mode
  - draw transitions between states; edit their guards and their order
  - declare inputs; Event input names are the game-facing verbs
  - preview: run the machine, fire triggers by button, show current state
  - save as v2 preserving unknown fields
should:
  - validate: missing initial, dangling toState, unknown animation id, unreachable state
  - undo / redo
wont: policy:editor-out-of-scope
```

Preview runs the real interpreter (decision:runtime-package-first), so the editor cannot show behavior the game will not reproduce.
