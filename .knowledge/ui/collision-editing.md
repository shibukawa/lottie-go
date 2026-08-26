---
id: ui:collision-editing
type: ui
title: Collision Editing on the Stage
---

Collision authoring inside ui:editor-shell's preview pane. Serves requirement:collision-authoring.

```yaml
ui:
  overlay:
    kind: canvas-overlay
    id: stage.collision
    state: cp body shapes (violet) under live hitboxes at the playhead; hitboxes colored by tag — hit/attack red, hurt green, push/collision/body amber, untagged gray
    state: selected shape gets thick stroke + resize handle (rect bottom-right, circle rightmost)
    action: click selects topmost; drag moves; handle drag resizes; hidden when toggle off
    state: same transform as the stage drawing, so overlays land on rendered pixels
  panel:
    kind: tabs
    id: tabs.collision
    columns: [Hitboxes, Body, Sockets]
    state: tabs the physics config leaves standing; Hitboxes holds the chart + add buttons (the redundant box dropdown is gone — chart rows select), Body and Sockets hold lists whose rows select for the inspector
    state: socket rows read "name / layer … · front|behind"; socket crosses drag on the stage, writing the layer-local dx/dy trim (also numeric in the socket pane)
  panel_tabs:
    hitboxes: {kind: buttons, labels: "+Rect / +Circle / +Win / Del", state: chart above; Del gated on selection}
    body: {kind: list, id: list.body, columns: [index, shape summary], action: row selects for inspector; +Circle / +Box / Del below}
    sockets: {kind: list, id: list.sockets, columns: [name, "layer … · front|behind"], action: row selects; layer dropdown + +Socket + Del below}
  stage_toggles:
    note: overlay visibility sits in the stage's top-right corner — three independent toggles (hit / body / sock), gated by the physics config like their groups; each also mutes its group's stage hit-testing
  socket_overlay:
    state: cyan cross + angle tick per socket resolved on the stage clip (api:sockets), local dx/dy trim applied; dimmed when the layer is outside its active window; selected cross thicker
    action: drag a cross to write the socket's layer-local offset (inverse of the layer's rotate-and-scale, so the cross follows the cursor)
  chart:
    kind: canvas
    id: chart.spans
    state: one row per hitbox/window, spans as tag-colored bars (windows hollow), shared playhead, frame ruler; selected row highlighted; hidden rows when physics config disables resolv
    state: plot shares the scrub timeline's exact horizontal extent (same gutter and right pad), so a frame sits at one x on both; the played range (segment) shows as a band in the header
    state: span edits clamp to the clip's frame extent — nothing places beyond the last frame
    action: bar drag moves a span, edge drag retimes, header/empty-row drag scrubs, any touch pauses; transport buttons (play/pause, ±1 frame) in the gutter
    state: row label click selects the box; parameters edit in the inspector
selection: hitbox and body-shape selection mutually exclusive; one thing drags at a time
edit_context: hitbox edits target the stage clip (clip preview or the machine's current state's clip); span edits key on the playhead
staleness: hitbox edits do not mark the machine preview stale (docGen untouched)
```

Model side caches parsed plugin docs per clip; every edit Stores back so save needs no extra sync (decision:collision-static-plugins).
