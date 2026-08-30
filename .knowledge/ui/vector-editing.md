---
id: ui:vector-editing
type: ui
title: Vector Shape Editing UI
---

Where requirement:vector-editing sits inside ui:editor-shell's preview
pane. Implemented (cmd/lottie-state-editor/shapeui.go, shapeinspector.go). Follows the pattern
ui:keyframe-editing set: a tab owns the stage overlay, a list pane
mirrors the stage selection, and a selection-driven form edits values.

Implementation deviations: the layer picker, the item tree and the
structure buttons head the inspector pane — like ui:keyframe-editing
list.parts — instead of living in the strip, which has no height to
spare (user 2026-08-30); the strip keeps only the key chart and the tool
row. The gradient stop's color edits in a hex-plus-swatch row (no popup
picker); alpha stops are not on the ramp yet.

```yaml
ui:
  tab:
    kind: tab
    id: tab.shapes
    parent: tabs.collision
    state: joins Segment / Hitbox / Body / Sockets / Poses; the active tab owns the stage drag, so vertex handles never compete with pose or hitbox drags
    state: enabled only while the stage clip has, or is gaining, a shape layer; a purely raster rig arrives here through "add shape layer" in the tree footer
  tools:
    kind: buttons
    id: row.shapetools
    columns: [Select, Pen, Rect, Ellipse, Star]
    state: one active tool; Select is the default. Pen closes and commits on clicking the first vertex or on a right click anywhere (the last point joins the first — user 2026-08-31); Finish commits open, tool switch cancels
    why: a mode row is new to the editor — every other tab is single-gesture. Drawing needs it; keep it to this tab
  tree:
    kind: list
    id: list.shapes
    columns: [item, kind-or-hidden]
    state: the selected shape layer's nested groups and items, paint order, expandable; unknown item kinds listed inert (round-trip members, not editable)
    state: selection shared with the stage both ways, like list.parts
    action: rows drag to reorder within their group; footer adds a group, item or modifier and deletes the selection
    action: a layer picker above the tree switches between the clip's shape layers, or adds one
  overlay:
    kind: canvas-overlay
    id: stage.shapes
    state: selected path drawn with vertices square, bezier handles as pins; primitives show their parameter grips (corner radius, radii)
    state: solid once a key is selected and the playhead parked, faint otherwise — the requirement:pose-editing park hint applies verbatim
    action: vertex drag moves a point, handle drag bends the curve, alt-click toggles corner/smooth, click on a segment with Pen adds a vertex
  gradient:
    kind: canvas-overlay + form-row
    id: stage.gradient / form.ramp
    why: user 2026-08-30 — gradient editing follows the Flash idiom, the transform gizmo on stage and the stop ramp in the panel
    state: selecting a gf/gs item shows the Flash-style gizmo on stage — linear: center point, rotation handle, length handle along the axis; radial: center point, radius handle, rotation handle, and a focal marker for the highlight
    state: Lottie mapping — linear s/e endpoints derive from center+angle+length; radial keeps s as center, |e-s| as radius, h/a as the focal (highlight) offset the Flash focal triangle edits
    action: dragging any gizmo handle rewrites s/e (and h/a) at the selected key
    state: the ramp is a horizontal bar in form.shape with stop pointers under it, the Flash color-panel bar
    action: click in empty space under the bar adds a stop with the color interpolated at that position; a stop drags horizontally to move, drags away from the bar to delete; the last two stops refuse deletion
    action: clicking a stop selects it and its color edits in the same hex-plus-swatch row every other color uses; gs alpha stops ride the same bar as hollow pointers
  inspector:
    kind: form
    id: form.shape
    columns: [item, name, frame, ease, per-kind fields]
    state: per-kind fields — fl/st: color, opacity, width, cap, join, dashes; gf/gs: type picker plus the stop ramp (stage.gradient); sh: selected vertex xy and handle xy; rc/el/sr: parameters; tr: p r s o a
    state: values are the ones stored at the selected key, not interpolated; static properties show a Keyframe button that promotes them
    state: color fields are hex plus a swatch; the editor has no OS color picker dependency, so the swatch opens the same in-app popup everywhere
    state: a hint line names what blocks editing — no key selected, vertex counts out of step, or an item the renderer skips
    action: Undo shares the clip-edit button and stack
guigui_mapping:
  stage.shapes: previewStage overlay; new stageDragKinds (vertex, handle, gradient end, primitive grip) beside part swing and joint move
  list.shapes: basicwidget.List with indent, EnsureItemVisibleByIndex only on stage-driven selection
  form.shape: basicwidget.Form, numeric rows via inputrow.go, tab order per ui:keyframe-editing forms
  row.shapetools: plain buttons with a held state; guigui has no toolbar widget worth waiting for
screenshots: extend cmd/lottie-state-editor/screenshot.go setup strings with (layer, shape path, vertex) so the overlay states can be photographed
```

Selection stays exclusive across tabs, so a vertex, a part and a hitbox
are never draggable at once — the rule tabs.collision already enforces.
