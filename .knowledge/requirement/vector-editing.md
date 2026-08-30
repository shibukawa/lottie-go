---
id: requirement:vector-editing
type: requirement
title: Vector Shape Editing
---

Status: implemented 2026-08-30 — cmd/lottie-state-editor shapedoc.go (document ops),
shape.go (model), shapeui.go (stage overlay, input, panel), shapeinspector.go
(inspector pane and the gradient ramp). Scope fixed with the user the same day
(decision:vector-authoring-in-editor).
Author and edit vector shape layers in the editor: full drawing, both
imported clips and vector character rigs, keyframe editing from the start.
The vector analogue of what raster part replacement did for images —
change the art next to the real renderer instead of round-tripping
through raw JSON.

```yaml
depends:
  - decision:vector-authoring-in-editor — the policy move this implements
  - decision:json-level-animation-edit — every edit rewrites the JSON, SetAnimation, re-Decode; lottie.Animation stays immutable
  - data:clip-edit-document — gains a shape tree per shape layer (its open item)
  - requirement:keyframe-timeline — a key is selected before it is edited, same park rule as requirement:pose-editing
targets:
  layers: shape layers (ty 4) of the stage clip, in any bundle clip — imported UI assets and hand-built rigs alike
  never: text layers stay read-only; raster part images are replaced whole, never painted (policy:editor-out-of-scope)
stages:  # implementation order; key editing ships inside each stage, not after them
  s0_select:
    - shape tree pane: the nested gr/item structure of the selected layer, front to back; selection shared with the stage both ways, like ui:keyframe-editing list.parts
    - on-stage pick: click selects the topmost visible shape whose filled or stroked area covers the point; occluded shapes picked from the tree
  s1_style:
    - fill (fl) and stroke (st): color, opacity; stroke width, cap, join, miter, dashes
    - gradients (gf, gs): type linear/radial, start/end points, stop colors and positions — edited the Flash way: transform gizmo on stage, stop ramp in the inspector (ui:vector-editing gradient; user 2026-08-30)
    - keyed variants of all of these edit at the selected key; static ones promote on first keying, per the data:clip-edit-document promotion rule
  s2_geometry:
    - path (sh) vertex move, in/out bezier handle drag, corner/smooth toggle, vertex add on segment, vertex delete, open/close
    - primitive parameters: rect (rc) size/roundness, ellipse (el) size, polystar (sr) points/radii — edited as parameters, not converted to paths
    - shape transform (tr per group) and layer ks edit; parenting a shape layer into a rig works as image layers do today
  s3_author:
    - add and delete shape layers, groups and items; reorder within the tree (paint order)
    - pen tool: draw a new path click-by-click, close on the first vertex
    - primitive tools: drop a rect / ellipse / polystar
    - add modifiers the renderer supports: tm, rd, pb, zz, op, mm (not intersect), rp
keys:
  edit_at: the selected keyframe only, playhead parked on it — requirement:pose-editing park applies unchanged, onion skin and zoom included
  promotion: keying a static shape property materializes the old value at every existing key time first
  path_topology: Lottie interpolates paths vertex-wise, so all keys of one path property must agree on vertex count and closure. A vertex add or delete at one key rewrites every key of that property (neighbors get the vertex interpolated in place); the editor never lets key shapes drift out of step
  ease: same picker as pose columns
subset_guard: the editor authors only what the renderer draws. After each edit the clip re-encodes and re-Decodes for the preview (stage shows renderer truth); UnsupportedFeatures must stay empty, checked on edit and refused loudly rather than saved silently (decision:practical-subset)
round_trip: unmodeled members survive — shape items the editor does not understand are preserved and shown inert in the tree, not dropped (policy:robustness)
undo: the existing clip-edit stack; one step per drag or tree operation, Begin/End collapsing a pen stroke or a handle swing
rig_direction: a vector rig is shape layers parented like image parts. Pose editing on such a rig means the pose tab learns to drag shape layers by their ks — today it picks image layers only (requirement:pose-editing scope). Preset library stays raster (requirement:animation-presets); vector rigs are new bundles, not a preset migration
deviations:
  - the pen draws corner vertices only; curves are made afterwards by dragging the vertex's handles or Smooth/Corner. Drag-while-placing bezier authoring is not built
  - primitives drop at a fixed default size where clicked; sizing is done afterwards on the stage — the selected geometry shows a box whose corners resize about the opposite corner and whose inside drags the whole shape (user 2026-08-30: numbers alone were not an editor)
  - geometry also inserts from the tree (+Path/+Rect/+Ellipse/+Star into the selected group at its origin), because choosing the place in the structure first is often the intent (user 2026-08-30); the stage tools stay for placing by click
  - items copy, paste and duplicate through an editor-session clipboard (deep copies, subtree and keys included; pastes across layers and clips) — buttons, since the editor has no keyboard input path (user 2026-08-30)
  - the box corner markers hide while an animated shape's playhead is between keys (user 2026-08-30): a grip must not offer a drag the park rule would refuse
  - the radial gradient's focal point (h/a) edits numerically, not by a stage marker yet
  - gradient alpha stops are preserved and honored but not editable on the ramp; color stops only
  - editing an animated member between its keys is refused with a park hint, mirroring pose editing, rather than auto-keying
open:
  - masks and track mattes: renderer supports them; editing them is deferred until the shape surface is stable
  - whether repeater (rp) and merge (mm) get on-stage feedback beyond re-render
  - vector rig conventions: slot naming, anchor rules, whether skills/lottie-character-preset grows a vector rig reference
  - pose-tab pick rule for shape layers: filled-area hit test vs bounding quad
```

Serves the same loop as requirement:pose-editing: a human corrects vector
art live on the real renderer, and decision:ai-skills-workflow agents get
a viewer where their shape edits can be watched and fixed by hand.
