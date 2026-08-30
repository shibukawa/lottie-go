---
id: ui:layout-shell
type: ui
title: Layout Editor Main Window
---

Single window over system:guigui, the `cmd/lottie-layout/` module. Serves requirement:scene-editor-mvp; follows ui:editor-shell conventions (env-published model, generation state keys, zenity dialogs on a goroutine, screenshot via LAYOUT_SCREENSHOT).

```yaml
ui:
  root:
    kind: window
    id: screen.layout
    title: Lottie Scene Layout
    children:
      - kind: toolbar
        id: bar.file
        children:
          - {kind: text, id: label.path}
          - {kind: button, label: "Open…"}
          - {kind: button, label: Save}
          - {kind: button, label: "Save As…"}
          - {kind: button, label: Scene, action: scene settings in pane.inspector}
          - {kind: button, label: Preview, state: toggles to Edit; flips canvas.scene mode}
      - kind: row
        id: body
        children:
          - kind: panel
            id: pane.palette
            children:
              - {kind: toolbar, columns: ["+Bundle…", "+Image…", "+Font…", "+Text"], state: no reference lists — after Add there is nothing to do to a file; broken references surface in Problems; +Text places directly and needs a font first}
              - {kind: list, id: list.sources, columns: [alias, contents-summary], state: ONE row per bundle (a bundle is one actor, not a bag of parts) plus images; first auto-selected, action: Place appends one centered node joining the viewed phase — its initial machine, else its first clip}
              - {kind: image, id: view.source-preview, state: plays the selected source looped (images draw static) — names alone do not say what a clip looks like. No nodes list — the timeline is the node list}
          - kind: column
            children:
              - kind: canvas
                id: canvas.scene
                state: draws the scene through api:scene-runtime; edit mode drags nodes (two-decimal rounding) and outlines unentered (grey), focusable (green), selected (blue); preview mode feeds Tab/arrows/Enter/Esc and the pointer into the player
              - kind: timeline
                id: view.timeline
                state: requirement:scene-timeline — doubles as the node list; participating node rows front-on-top (drag names vertically to reorder overlap, Delete in the header), entrance bars dragged horizontally, a ruler lane scrubs the playhead, open-ended bars for machines/images/text/loops, content-end mark, Play/Pause transport (starts stopped, auto-pauses at content end), Replay, phase selector (requirement:scene-phases)
          - kind: panel
            id: pane.inspector
            state: switches on selection (requirement:selection-driven-ui pattern)
            children:
              - {kind: form, id: form.scene, columns: [name, width, height, hoverMovesFocus, initialFocus]}
              - {kind: list, id: list.phases, columns: [name, duration→next], action: "Add phase/Delete; phase form (name, duration, then) while selected"}
              - {kind: form, id: form.node, columns: [name, source, plays, start, phase, entry-state], state: plays is one dropdown over the bundle's machines and clips — switching swaps the node's content and resets segment/entry/chain}
              - {kind: list, id: list.then, columns: [clip/segment, once|loop], state: animation nodes; the chain after the clip completes, action: "Add step (clears the base loop, tail loops by default)/Delete; step form (clip, segment, loop) while selected"}
              - {kind: form, id: form.text, columns: [value, font, size, align, anchorX, anchorY, color], state: text nodes only}
              - {kind: form, id: form.transform, columns: [x, y, scaleX, scaleY, rotation, opacity]}
              - {kind: form, id: form.playback, columns: [segment, loop, loopCount, speed, mode, autoplay]}
              - {kind: form, id: form.focus, columns: [focusable, tabIndex, up, down, left, right]}
              - {kind: list, id: list.bindings, columns: [on→do, target+arg]}
              - {kind: form, id: form.binding, columns: [on, do, target, arg], state: all four actions offered (validator flags kind mismatches); target picks any node for fireEvent/playSegment; arg is a picker fed from the resolved target (events / markers / focusable nodes), free text for callback; shown only while a binding is selected}
      - {kind: text, id: label.status}
verified:
  - transform drags patch the live SceneNodePlayer and skip the doc generation, so playback never restarts mid-drag
  - preview keyboard needs SetButtonInputReceptive on the canvas, or keys only arrive after a click focuses it
  - preview pointer is fed from Tick, not HandlePointingInput, so hover works without button activity
```
