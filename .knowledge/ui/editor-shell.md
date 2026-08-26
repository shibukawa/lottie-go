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
              - kind: list
                id: list.clips
                columns: [segment-or-file, source-file, duration]
                state: one row per file x marker; a file with no markers is one row
                action: preview that unit alone
              - kind: list
                id: list.machines
                columns: [id]
                state: a bundle may hold several; they share clips, not states
                state: a play marker names the manifest's default machine
                action: select opens it in canvas.graph; new, rename, delete, set or clear initial
              - kind: tabs
                id: tabs.interface
                columns: [Events, Values, Markers]
                state: split by direction; events and values come in, markers go out
              - kind: table
                id: table.io
                columns: [name, kind-or-source, control-or-hits]
                state: try button for an event, checkbox or field for a value, fired-count for a marker; a Restart pseudo-row leads the events tab and is not editable
                action: selecting an event or value traces its transitions on canvas.graph
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
          - {kind: text, id: label.state, state: current state name, or the clip being previewed}
          - kind: timeline
            id: view.timeline
            state: document as a track, played range as a band, markers as ticks, playhead
            action: drag to scrub
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
  - basicwidget.ListItem.Content hosts a custom row widget; mark its labels passthrough so the row still selects
  - separate a document-edit counter from the redraw counter, or selecting something reports the preview as edited
  - a per-frame readout must be written from Tick as well as Build, or it disagrees with what Draw paints
  - build only the visible tab: an unadded widget is not laid out, drawn or sent input
```

list.transitions must allow reordering because order decides which transition wins (data:state-machine).
