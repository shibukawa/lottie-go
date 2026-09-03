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
    hair-back: {parent: head, attach: [36, 20], anchor: [40, 10], size: [80, 70], vertices: 10, order: behind-head}
    body: {attach: scale-x}
  props:
    - {slot: sword, name: naginata, parent: forearm-far, attach: [7, 26], anchor: [10, 10], length: 78}
  morph:
    - {generator: breathe, parts: [body, head], clips: [idle-anim, guard-anim], amount: 0.03, period: 48}
    - {generator: squash, parts: all, clips: [jump-anim], at: 22, amount: 0.12, recover: 6}
    - {generator: bend, parts: [forearm-near, forearm-far, shin-near, shin-far], clips: all, threshold: 60}
    - {generator: follow, parts: [hair-back], clips: all, lag: 3, weight: 0.4}
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
  cell_rule: rect is the slot's size times one scale for the whole sheet, so cells keep slot aspect ratios and the part fills its cell top to bottom; a margin (8 px) inside the border is ignored by cut; a label band under each cell carries the slot name for the model and means nothing to the tool
  layout: rows packed by height; the tool picks the largest scale at which every cell of the sheet fits, so limb sheets get big limbs (a 15x27 upper arm at x8 is 120x216)
  extra_slots: spec parts with a size join the sheet whose joint edge they share
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
  file: "extensions/forge/<slot>.json — {slot, size, anchor, fit, v: [[x, y]], uv: [[u, v]]} in slot space; the untouched contour every morph bake starts from"
```
