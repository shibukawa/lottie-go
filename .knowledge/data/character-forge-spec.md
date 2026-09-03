---
id: data:character-forge-spec
type: data
title: Character Forge Spec and Sheet Manifest
---

Two JSON documents drive cmd/lottieforge (requirement:ai-character-forge).
The spec is what the agent writes; the manifest is generated from the spec
and the base rig and describes the sheets the image model is asked to
fill. Both live in the work directory beside the lottierepack layout.

```yaml
character.json:
  name: bundle and file prefix, e.g. kitsune-miko
  base: chibi-male | chibi-sword — the preset whose rig, clips and machine are inherited
  description: free text the prompts embed — who the character is, outfit, colors, props
  style: rendering words the prompts embed ("clean anime cel shading, thick dark outline, flat colors")
  proportions: stated in prompts, defaulted from the base ("chibi, about 2.5 heads tall, big head")
  key: chroma background, default "#FF00FF"; choose another when the design uses magenta
  sheet_size: "[1024, 1024] default; per model"
  parts:  # per-slot overrides; absent slots take the base rig's contract
    head: {fit: 1.3}
    body: {attach: scale-x}
  attachments:  # concept:attachment-kinds; name + kind, the rest defaults by kind and host
    - {name: ponytail, kind: lock, host: head, attach: [44, 12], size: [26, 60], order: behind-head}
    - {name: side-lock, kind: lock, host: head, attach: [12, 30], size: [12, 40], paired: true}   # near drawn, far derived
    - {name: hakama, kind: drape, host: body, pivot: waist, size: [60, 40], drivers: [thigh-near, thigh-far], panels: 2, weight: 0.6}
    - {name: sleeve, kind: drape, host: upper-arm, size: [24, 34], drivers: [forearm], paired: true, weight: 0.5}
    - {name: cape, kind: drape, host: body, size: [56, 70], order: behind-body, views: separate}
    - {name: ribbon-tail, kind: swing, host: head, attach: [40, 4], size: [10, 36], segments: 2, damping: 0.85}
    - {name: fox-ears, kind: rigid, host: head, attach: [36, 2], anchor: [20, 30], size: [40, 30], sway: {amount: 4, period: 96}}
    - {name: tail, kind: swing, host: body, attach: [10, 44], size: [30, 50], order: behind-body, segments: 2}
  attachment_fields:
    kind: rigid | swing | drape | lock
    host: the rig slot it parents to; the tool copies that layer's opacity track
    attach: where the pivot sits in the host's slot space; default the host's top center for head items, the waist for body drapes
    anchor: the pivot inside the item's own image (rigid items only; hanging kinds pivot at the top-cap center the fit finds)
    size: nominal size in slot units, sets the sheet cell's aspect and the fit target; from the model sheet by eye
    order: in-front-of-<slot> | behind-<slot>; defaults by kind
    paired: draw once, derive the far copy darker, two layers
    views: baked (default — the host's side and back cells include it) | separate (own side and back cells and layers)
    kind_fields: "drape — drivers, weight, panels, pivot; swing and lock — segments, damping, stiffness; rigid — sway"
  props:
    - {slot: sword, name: naginata, parent: forearm-far, attach: [7, 26], anchor: [10, 10], length: 78}
  morph:
    - {generator: breathe, parts: [body, head], clips: [idle-anim, guard-anim], amount: 0.03, period: 48}
    - {generator: squash, parts: all, clips: [jump-anim], at: 22, amount: 0.12, recover: 6}
    - {generator: bend, parts: [forearm-near, forearm-far, shin-near, shin-far], clips: all, threshold: 60}
    - {generator: follow, parts: [ponytail], clips: all, lag: 3, weight: 0.4}   # optional on top of the kind's default motion
  clips:
    add: [{name: cast-anim, from: punch-anim}]   # copies; reshaped afterwards by the preset skill's recipes
    machine: [{state: cast-state, animation: cast-anim, event: cast, from: [idle-state, walk-state], returns: idle-state}]
  raster: false   # true writes the t0 rig
sheets.json:  # written by lottieforge grid, read by cut
  key: "#FF00FF"
  sheets:
    - id: model
      size: [1024, 1024]
      cells: []     # full body reference, no grid; also the attachment every part prompt carries
    - id: heads
      size: [1024, 1024]
      cells:
        - {id: A1, slot: head,      view: front,        rect: [24, 40, 312, 280], joint: bottom, blobs: 1}
        - {id: A2, slot: head-side, view: rear-quarter, rect: [352, 40, 640, 280], joint: bottom, blobs: 1}
        - {id: A3, slot: head-back, view: back,         rect: [680, 40, 968, 280], joint: bottom, blobs: 1}
    - id: torsos
      cells: [body front, body-side, body-back — joint bottom (hip), neck stump at the top edge]
    - id: limbs
      cells: [upper-arm, forearm, thigh, shin, then props — joint top; drawn once each, near and far derive]
    - id: attachments
      cells: [one per attachment, plus side and back cells for views separate — joint top for hanging kinds, the declared anchor for rigid; only when the spec has attachments]
  cell_rule: rect is the slot's size times one scale for the whole sheet, so cells keep slot aspect ratios and the part fills its cell top to bottom; a margin (8 px) inside the border is ignored by cut; a label band under each cell carries the slot name for the model and means nothing to the tool
  layout: rows packed by height; the tool picks the largest scale at which every cell of the sheet fits, so limb sheets get big limbs (a 15x27 upper arm at x8 is 120x216)
  host_cells: a host with attachments gets the "without its <names>" wording in its cell line, and its side and back cells the "as worn, with its <names>" wording (concept:attachment-kinds views baked)
derived_parts:  # cut writes these without asking the model
  far_side: upper-arm-far, forearm-far, thigh-far, shin-far = the near drawing with colors multiplied by 0.72 (skills/lottie-character-preset rig.md — far side reads darker)
  shadow: the base preset's shadow image, unchanged
report.json:  # cut's verdict per cell; what the agent reads before rigging
  cell: {id, slot, status: ok|empty|border|multi|halo, bbox, blobs, note}
  empty: no alpha inside the margin
  border: alpha within the margin — the drawing crossed into a neighbour or hugged the edge; redo the cell
  multi: more blobs than the slot allows — a hand drawn apart from its forearm, or a label inside the cell
  halo: key-colored fringe left after decontamination beyond a threshold — usually a soft shadow the prompt forbade
rest_contours:  # written by rig, read by morph
  file: "extensions/forge/<slot>.json — {slot, size, anchor, fit, paths: [{v: [[x, y]], uv: [[u, v]]}], welds: [[path, vertex, path, vertex]]} in slot space; the untouched contours every morph bake starts from, one entry per sub-path of the decomposition"
```
