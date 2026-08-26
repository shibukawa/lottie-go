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
    kind: rows
    id: rows.collision
    children:
      - {kind: checkbox, id: chk.show, state: overlay visibility}
      - {kind: select, id: sel.box, state: "N: name per box in the stage clip's track"}
      - {kind: button, label: +Rect / +Circle, action: add hitbox with span at playhead, mid-stage}
      - {kind: button, label: +Win, action: add geometry-less window box; shares tag/span editing, never drawn on stage}
      - {kind: button, label: Del, state: enabled only with a hitbox selected}
      - note: name, tags, span from/to, +Span/Del span moved to the inspector panes (requirement:selection-driven-ui)
      - {kind: button, label: body +Circle / +Box / Del, state: bundle-level cp body, count shown}
      - kind: row
        id: rows.sockets
        children:
          - {kind: select, id: sel.layer, state: stage clip layer names (anim.LayerNames)}
          - {kind: button, label: +Socket, action: bind layer as socket, socket name = layer name}
          - {kind: select, id: sel.socket, state: "N: name (front|behind)"}
          - {kind: button, label: Del, state: enabled only with a socket selected}
          - note: socket rename and the z toggle live in the inspector socket pane
  socket_overlay:
    state: cyan cross + angle tick per socket resolved on the stage clip (api:sockets); dimmed when the layer is outside its active window; selected cross thicker
  timeline:
    state: selected box's spans drawn as tag-colored bars inside the track
selection: hitbox and body-shape selection mutually exclusive; one thing drags at a time
edit_context: hitbox edits target the stage clip (clip preview or the machine's current state's clip); span edits key on the playhead
staleness: hitbox edits do not mark the machine preview stale (docGen untouched)
```

Model side caches parsed plugin docs per clip; every edit Stores back so save needs no extra sync (decision:collision-static-plugins).
