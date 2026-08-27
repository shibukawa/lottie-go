---
id: requirement:scene-focus-navigation
type: requirement
title: Scene Focus Navigation
---

Focus model that lets a scene act as a game menu for pad, keyboard, and mouse. Authored in vision:scene-editor, stored per node in data:scene-document, executed by api:scene-runtime. Status: proposed.

```yaml
model:
  - at most one focused node per scene
  - focusable flag per node; non-focusable nodes are decoration
  - Next/Prev (Tab / Shift+Tab / shoulder buttons) walks tabIndex order, wrapping
  - directional move (cursor keys / d-pad): explicit neighbor link wins; empty link falls back to geometric nearest in a direction cone; no candidate = stay
  - mouse hover moves focus when options.hoverMovesFocus, unifying pointer and pad focus into one state
  - focus and blur are bindable events (requirement:scene-interactions), so a machine node can play its own focus animation
editor:
  - toggle focusable and set tabIndex in the inspector
  - canvas overlay draws tab order and neighbor links; drag between nodes to set an explicit link
  - preview: Tab and cursor keys move focus visibly
```

Explicit links exist because geometric fallback misreads ring menus and staggered grids; authors override only where geometry lies.
