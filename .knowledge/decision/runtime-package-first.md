---
id: decision:runtime-package-first
type: decision
title: Build Runtime Package Before Editor UI
---

Implement the state machine interpreter as a library package first. The editor preview and the game runtime both consume that one package; the editor never gets a preview-only interpreter.

```yaml
rationale:
  - what the editor previews must equal what the game plays
  - a second interpreter would drift and hide spec bugs
  - unblocks requirement:player-state-machine without any UI work
order:
  1: v2 bundle read/write plus markers (requirement:player-state-machine)
  2: api:state-machine-runtime interpreter
  3: requirement:editor-mvp UI on top
```

Inverts the stated build order (editor first) at the code level only; vision:state-machine-editor is still the first thing shipped.
