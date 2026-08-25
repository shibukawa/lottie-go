---
id: system:guigui
type: system
title: Guigui
---

Go GUI framework on system:ebitengine, used for ui:editor-shell. Widgets are retained structs whose child tree is rebuilt by re-running Build.

```yaml
reference: https://github.com/guigui-gui/guigui
lifecycle: Build then Layout then Handle*Input then Tick per tick; Draw per frame
constraints:
  - widget must work from its zero value; children are plain fields
  - variable child count needs guigui.WidgetSlice, not a value slice
  - state mutated outside handlers needs WriteStateKey or RequestRebuild
  - no node-graph widget; canvas.graph needs custom Draw plus HandlePointingInput
fit:
  - same Ebitengine runtime as the player, so Player.Draw works inside a widget Draw
  - alpha API with no tagged releases; pin a pseudo-version
```

Detailed usage rules live in the repository skill using-guigui, not in this catalog.
