# Spec, sheets, and the cut

What `lottieforge grid` and `lottieforge cut` do, precisely enough to
script by hand until they ship. The spec is the only file an agent
writes; everything else is derived from it and the base preset's rig
contract (lottie-character-preset's rig.md).

## character.json

```jsonc
{
  "name": "kitsune-miko",
  "base": "chibi-sword",                  // chibi-male | chibi-sword
  "description": "a fox-eared shrine maiden with a red hakama, white kimono top and a naginata",
  "style": "clean anime cel shading, thick dark outline, flat colors, no gradients",
  "proportions": "chibi, about 2.5 heads tall, big head, short limbs",   // default from base
  "key": "#FF00FF",                       // default; pick another if the design is magenta
  "sheet_size": [1024, 1024],             // default; match the model's native output
  "parts": {                              // per-slot overrides, all optional
    "head": { "fit": 1.3 },               // hair towers over the template head
    "body": { "attach": "scale-x" }       // broad torso: scale shoulder x offsets by width ratio
  },
  "attachments": [                        // hair, cloth, ornaments — references/attachments.md
    { "name": "ponytail", "kind": "lock",  "host": "head", "attach": [44, 12], "size": [26, 60], "order": "behind-head" },
    { "name": "hakama",   "kind": "drape", "host": "body", "size": [60, 40], "drivers": ["thigh-near", "thigh-far"], "panels": 2 }
  ],
  "props": [
    { "slot": "sword", "name": "naginata", "parent": "forearm-far",
      "attach": [7, 26], "anchor": [10, 10], "length": 78 }
  ],
  "morph": [ /* see motion.md */ ],
  "clips": { "add": [], "machine": [] },  // see motion.md
  "raster": false                         // true: build the raster (t0) rig instead
}
```

Slot names, parents, anchors and attach points default to the base
rig's table. A prop on `chibi-sword` reuses the `sword` slot (parent,
attach, layer order and the per-clip blade angles all carry), whatever
it is called; `length` scales the fit so a naginata reads longer than
the template's blade without touching a keyframe.

## sheets.json and the templates

`grid` lays every slot that needs a drawing onto sheets:

| sheet    | cells (chibi)                                        | joint edge |
|----------|------------------------------------------------------|------------|
| model    | none — the full-body reference                       | —          |
| heads    | head, head-side, head-back (+ hair slots)            | bottom     |
| torsos   | body, body-side, body-back (+ cape slots)            | bottom     |
| limbs    | upper-arm, forearm, thigh, shin, then props          | top        |
| attachments | one cell per attachment; side and back cells for `views: separate` (only when the spec has attachments) | top for hanging kinds, the declared anchor for rigid |

Cell geometry: each cell is its slot's size × one scale per sheet, so
cells keep slot aspect ratios and the part fills the cell top to bottom;
the tool picks the largest scale at which every cell fits the sheet
with a 24 px label band under each and a 16 px gutter. Attachment
cells take their aspect from the spec's `size`. For chibi limbs
on 1024² that is about ×8: a 15×27 upper arm becomes a 120×216 cell,
big enough for real detail. A host with attachments gets "WITHOUT its …" in its front cell line
and "AS WORN with its …" in its side and back cell lines.

Template PNG: flat key color, 2 px black border per cell, the slot name
in the label band under it, nothing else. The border and band exist for
the model; `cut` reads only the manifest rectangles.

```jsonc
{
  "key": "#FF00FF",
  "sheets": [
    { "id": "model", "size": [1024, 1024], "cells": [] },
    { "id": "heads", "size": [1024, 1024], "cells": [
      { "id": "A1", "slot": "head",      "view": "front",        "rect": [24, 40, 312, 280], "joint": "bottom", "blobs": 1 },
      { "id": "A2", "slot": "head-side", "view": "rear-quarter", "rect": [352, 40, 640, 280], "joint": "bottom", "blobs": 1 },
      { "id": "A3", "slot": "head-back", "view": "back",         "rect": [680, 40, 968, 280], "joint": "bottom", "blobs": 1 }
    ]},
    ...
  ]
}
```

## The cut

Per cell, inside `rect` shrunk by an 8 px margin:

1. **Key.** Alpha = 0 where the pixel is within a color distance of the
   key (default 60 in RGB), 1 where far, ramped between; then
   decontaminate edge pixels by pulling the key color out of the RGB in
   proportion to the removed alpha. Anti-aliased edges against magenta
   otherwise keep a pink fringe.
2. **Blobs.** Connected components of alpha > 0.5; drop specks under
   0.2% of the cell area. `blobs: 1` keeps the largest and flags `multi`
   when a second one holds more than 3% of the alpha.
3. **Flags.** `empty` — no alpha; `border` — alpha inside the margin;
   `halo` — after decontamination, more than 2% of the part's edge
   pixels are still within the key distance (a soft shadow); `multi` as
   above. Each flag lands in `report.json` with the cell id and the slot.
4. **Crop** to the alpha bbox, write `parts/<slot>.png` at sheet
   resolution. No resizing here: the morph rig samples this image as
   is, and the raster tier resizes to slot size later.
5. **Derive.** `upper-arm-far` etc. = the near part with RGB × 0.72,
   alpha untouched; `paired` attachments the same. A `panels: 2` drape
   is split at its vertical midline into `<name>-front` and
   `<name>-back` textures. `shadow` = the base preset's shadow image.
6. **Contact sheet.** `contact.png`: every part on its slot's nominal
   rectangle (scaled), joint marked, name under it — what the agent
   looks at to catch a diagonal limb or a head drawn from the front on
   the back cell, which no flag can see.

### Fallback: no grid

When the model ignored the template, `cut -free` takes the whole sheet
as one cell, finds all blobs, orders them top-to-bottom then
left-to-right, and assigns them to the sheet's slots in that order,
printing the assignment. The agent confirms against `contact.png` and
can pass `-order head,head-back,head-side` to fix a swap.

### Raster tier

`rig -raster` resizes each part into its slot canvas (fit by height,
anchor at the slot's anchor pixel, nearest-neighbour for pixel-art
styles, linear otherwise) and writes the preset's raster bundle with
the images swapped — the lottie-character-preset design-swap path, now
fed by the cut. Use it for a pixel-art look, or for a player without
the texture plugin.
