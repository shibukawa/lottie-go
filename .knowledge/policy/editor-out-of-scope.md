---
id: policy:editor-out-of-scope
type: policy
title: Editor Out of Scope
---

vision:state-machine-editor will not do these.

```yaml
excluded:
  - authoring or editing Lottie artwork; use system:lottie-editors
  - trimming or retiming clips; import them already cut
  - theme and font editing
  - web interaction authoring (pointer types deferred in decision:game-oriented-sm-subset)
  - runtime scripting or expressions (policy:out-of-scope)
```

The editor is a bundler and a graph authoring surface, nothing more.
