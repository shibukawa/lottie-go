---
id: ui:editor-shell
type: ui
title: Editor Main Window
---

Single window over system:guigui. Serves requirement:editor-mvp.

```yaml
ui:
  root:
    kind: window
    id: screen.editor
    title: Lottie State Machine Editor
    children:
      - kind: toolbar
        id: bar.file
        children:
          - {kind: button, label: Open, action: flow:author-state-machine}
          - {kind: button, label: Import Clip}
          - {kind: button, label: Save}
          - {kind: select, id: sel.machine, label: State Machine}
      - kind: row
        id: body
        children:
          - kind: panel
            id: pane.clips
            title: Clips
            children:
              - {kind: list, id: list.animations, columns: [id, duration, size]}
          - kind: canvas
            id: canvas.graph
            title: State Graph
            state: nodes are states, edges are transitions; selection drives pane.inspector
            children:
              - {kind: contextmenu, id: menu.node, columns: [Add State, Set Initial, Delete]}
          - kind: panel
            id: pane.inspector
            title: Inspector
            children:
              - {kind: form, id: form.state, columns: [name, animation, segment, loop, loopCount, speed, mode]}
              - {kind: list, id: list.transitions, columns: [order, toState, guards]}
              - {kind: form, id: form.guard, columns: [type, inputName, conditionType, compareTo]}
              - {kind: list, id: list.inputs, columns: [type, name, value]}
      - kind: panel
        id: pane.preview
        title: Preview
        children:
          - {kind: image, id: view.stage, target: api:state-machine-runtime}
          - {kind: text, id: label.state, state: current state name}
          - {kind: list, id: list.triggers, state: one button per Event input; press fires it}
guigui_mapping:
  window: guigui.Run root widget
  toolbar/row/panel: guigui.LinearLayout plus basicwidget.Panel
  list: basicwidget.List or basicwidget.Table
  form: basicwidget.Form
  select: basicwidget.Select
  contextmenu: basicwidget.ContextMenuArea
  canvas.graph: custom widget; no basicwidget equivalent (system:guigui)
  view.stage: custom widget whose Draw calls the runtime
verified:
  - a custom widget must override Measure; the default 144x144 box clips its own Draw
  - validation run from Build must not bump the state key, or the tree rebuilds every tick
  - screenshots via guigui.RunWithCustomFunc, forwarding LayoutF as well as Layout
```

list.transitions must allow reordering because order decides which transition wins (data:state-machine).
