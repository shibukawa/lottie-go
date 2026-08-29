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
    columns: ["Segment", "Poses", "Hitbox (resolv)", "Body (cp)", "Sockets"]
    state: annotation tab labels carry their engine so the destination of each group's data is legible at a glance
    state: the row above the tab bar holds what belongs to the stage rather than to a tab — the autoplay and onion-skin checkboxes, a zoom readout, -/+ and Fit. Autoplay used to live inside the Segment tab, where the other tabs could not reach it
    state: stage view is zoom over fit-to-pane plus a pan; wheel zooms about the cursor, the buttons about the pane centre, dragging empty stage pans, Fit returns. It applies to every tab because it is how the stage is looked at, not what it edits — collision shapes get the magnification too
    state: onion skin draws the keys either side of the playhead under the current frame, previous cool and next warm; it renders through its own paused player, because moving the stage player mid-draw would stop playback as a side effect of a display option. Off while the clip plays, where the pair would strobe
    state: the boundary between the state graph and the preview drags (splitterView), so posing can take the window back from the graph. The split is seeded from the height the preview used to have and then belongs to whoever dragged it; it lives for the session, not the document
    state: Poses is ui:keyframe-editing; the tab set is otherwise the annotation groups
    state: Segment (default) holds the whole-clip overview timeline — where the played range sits in the full extent, markers included; scrub-only, no buttons
    state: tabs the physics config leaves standing; Hitboxes holds the chart + add buttons (the redundant box dropdown is gone — chart rows select), Body and Sockets hold lists whose rows select for the inspector
    state: socket rows read "name / layer … · front|behind"; socket crosses drag on the stage, writing the layer-local dx/dy trim (also numeric in the socket pane)
  panel_tabs:
    hitboxes: {kind: buttons, labels: "+Rect / +Circle / +Win / Del", state: chart above; Del gated on selection}
    body: {kind: list, id: list.body, columns: [index, shape summary], action: row selects for inspector; +Circle / +Box / Del below}
    sockets: {kind: list, id: list.sockets, columns: [name, "layer … · front|behind"], action: row selects; one "+<layer>" button per stage layer below (disabled once bound), like the shape buttons on the other tabs, plus Delete}
  stage_visibility:
    note: no toggles — the active tab decides. Each overlay group (and its stage hit-testing) shows exactly while its tab is the working context; the Segment tab is the clean, undecorated preview
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
