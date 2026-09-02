---
id: ui:texture-mapping
type: ui
title: Texture and UV Editing UI
---

Where requirement:texture-mapping is authored: inside the shapes tab of
ui:vector-editing, since a paint belongs to an fl or st item and UV to an
sh item's vertices. No new tab, and no second document model — the
data:texture-document is woven into the clip tree while the clip is staged
(decision:texture-weave), so the inspector edits x-tex and x-uv members
like any other item member and the existing tree operations carry them.
Implemented 2026-09-02 (cmd/lottie-state-editor/texture.go, textureui.go).

Implementation deviations: the UV pane sits in the inspector under the
vertex list, not beside the stage — the inspector is where the gradient
ramp lives and the pane is the same kind of thing; it offers point drag,
whole-set drag on empty space and wheel scale about the centroid, not
box-select or rotate. The gizmo is a square at the texture's origin (UV
0,0 placed) and a circle half a texture width along its u axis, which sets
rotation and a uniform scale in one drag; non-uniform scale and the anchor
are typed. The unplaced entries list under the form with one Drop button
rather than in the tree footer.

```yaml
ui:
  inspector:
    kind: form
    id: form.shape / texture section
    state: appears on fl and st selection, below the existing color rows, collapsed until a texture is bound
    columns: [image, mapping, wrap, filter, tint, offset, scale, rotation]
    action: the image row picks from the bundle's i/ images, or imports a file into the bundle; clearing it removes the paint and the item is a plain fill again
    state: the color swatch stays and is relabeled tint, so the fallback color stays visible and editable — it is what a player without the plugin draws (data:texture-document degradation)
    state: mapping vertex is offered only while a sibling sh exists; mapping stroke only on st
    state: the hint line ui:vector-editing already uses names why a mapping is unavailable — a tm in the group, a UV length out of step, an unresolved image
    state: the transform rows are keyed properties, so their key times join the layer's pose columns and edit at the parked key like every other member (requirement:keyframe-timeline)
  gizmo:
    kind: canvas-overlay
    id: stage.texmap
    why: mapping bbox is a matrix, which is what the gradient gizmo already edits; reuse the idiom rather than invent one
    state: mapping bbox selected shows center, rotation and scale handles over the shape, the stage.gradient pattern
    action: dragging writes the transform at the selected key, park rule unchanged
  uvpane:
    kind: panel
    id: pane.uv
    why: per-vertex UV is a second space; it cannot be edited on the stage, which shows only the first
    state: opens beside the stage while mapping vertex is selected — the texture drawn to fit, the selected path's UV polygon over it, points square like stage vertices
    state: selection mirrors the stage and the vertex list both ways, the pattern list.shapes set; hovering a stage vertex lights its UV point
    action: drag a UV point to move it; the stage redraws textured live
    action: box-select and a uniform scale or rotate of the selection, since laying out a whole contour point by point is the common case
    state: flattened points are not shown — UV is authored per control vertex only, and the rest interpolates (concept:texture-uv-pipeline mesh_vertex)
  seed:
    action: switching to mapping vertex seeds x-uv by projecting the current bbox layout onto the vertices, so editing starts from the picture already on screen instead of every UV at zero
    action: a Reset button re-seeds the same way
  topology:
    state: a vertex added or deleted on the path inserts or removes its UV in step, inside the same rewrite requirement:vector-editing path_topology already performs — an inserted vertex takes the UV interpolated between its neighbours
  dangling:
    kind: list-footer
    id: list.shapes / unplaced
    state: doc entries whose address did not resolve on load list inert under the tree, named by their stored address, so nothing silently vanishes; Delete drops one, otherwise it is written back as it came
  clips:
    action: rename, duplicate and delete of a clip carry its doc, the model.go path the resolv track already takes
guigui_mapping:
  form.shape texture section: basicwidget.Form rows, numeric rows via inputrow.go, the image row a button opening the existing asset list
  pane.uv: a second previewStage instance with its own drag kind; the stage's exclusive-selection rule extends to it, so a UV drag and a vertex drag are never live at once
  stage.texmap: a stageDragKind beside the gradient handles
store_back: every edit encodes the tree, Unweaves, SetAnimation(pure) plus lottietexture.Store(doc), then Decodes the pure clip and Applies the doc for the preview — the data:clip-edit-document lifecycle with one more step, and the strip check that no x- member survived (rule:x-member-namespace)
screenshots: extend cmd/lottie-state-editor/screenshot.go setup strings with (texture bound, mapping, selected UV point)
```

Undo shares the clip-edit stack; one step per drag, Begin/End collapsing a
UV box-transform exactly as it collapses a handle swing.
