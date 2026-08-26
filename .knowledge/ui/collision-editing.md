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
      - {kind: field, id: fld.name}
      - {kind: field, id: fld.tags, state: comma-separated, free-form}
      - {kind: field, id: fld.from-to, state: span under playhead; [from, to)}
      - {kind: button, label: +Span, action: new span at playhead, copies nearest earlier pose, clipped to next span}
      - {kind: button, label: Del span / Del}
      - {kind: button, label: body +Circle / +Box / Del, state: bundle-level cp body, count shown}
  timeline:
    state: selected box's spans drawn as tag-colored bars inside the track
selection: hitbox and body-shape selection mutually exclusive; one thing drags at a time
edit_context: hitbox edits target the stage clip (clip preview or the machine's current state's clip); span edits key on the playhead
staleness: hitbox edits do not mark the machine preview stale (docGen untouched)
```

Model side caches parsed plugin docs per clip; every edit Stores back so save needs no extra sync (decision:collision-static-plugins).
