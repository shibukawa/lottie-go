---
id: data:texture-document
type: data
title: Texture Document
---

Per-clip document binding images to fl/st items and UV to sh vertices, for
requirement:texture-mapping. Defined by plugin lottietexture
(api:texture-binding); stored aside, edited woven (decision:texture-weave).
Implemented 2026-09-02 (plugin/texture/texture.go).

```yaml
storage: extensions/texture/<animID>.json (lottietexture.File); one per clip, entries address items inside it; written by json.Marshal, so the encoding is canonical and a stored document re-encodes byte for byte
doc: {paints: [Paint], uvs: [UV]}
address:   # shared by Paint and UV; regenerated on every store-back, never hand-edited
  asset: precomp asset id owning the layer; empty for root layers
  layer: the layer's ind — Lottie's own stable layer identity, what parent points at; the editor writes ind on every layer it creates
  item: index path from the layer's shapes through gr.it arrays, e.g. [0, 2] = shapes[0].it[2]; counts every authored item, hidden ones included, and the core's decoded tree records the same indices (shapeNode.jsonIndex)
paint:      # targets an fl or st
  texture: image asset refId, or a name bound at runtime
  mapping: bbox (default) | vertex | stroke
  wrap: clamp (default) | repeat | mirror
  filter: linear (default) | nearest
  tint: true (default) multiplies the texture by the item's c and o; false keeps c only as the fallback color
  transform: "{p, s, r, a} — Lottie property objects, static or keyframed. p offset in UV units, s scale percent, r degrees, a anchor in UV units. Lottie-shaped so the woven form keys like any other property"
uv:         # targets an sh
  v: "[[u, v], ...] parallel to the path's v[], normalized 0..1"
  static: not keyframed. The vertex count is already fixed across keys (requirement:vector-editing path_topology), so one array is well-defined; motion is transform's job and cheaper there
  length: must equal the vertex count; a mismatch disables vertex mapping for that path and notes it
  stroke: mapping stroke reads u only; the across-width coordinate is generated (concept:texture-uv-pipeline)
mapping_modes:
  bbox: UV is the normalized position in the path's shape-space bounding box. No uv entry, no mesh; one matrix, drawn like a gradient
  vertex: UV from the sibling sh items' uv entries, interpolated across a generated mesh — the mode requirement:texture-mapping is for
  stroke: u along the path (arc length or authored u), v across the width. st only; on an fl it degrades to bbox
units:
  uv: normalized over the referenced image, so the doc is resolution-independent and an atlas sub-image works unchanged
  pixels: "renderer converts at draw time — SrcX = Bounds().Min.X + u*Bounds().Dx(), likewise Y"
woven_form: x-tex on the fl/st holding the paint members minus the address; x-uv on the sh holding {v}. Same names, same values (rule:x-member-namespace)
unknown_members: preserved via ExtraFields at every level, as in data:physics-resolv-track
degradation:
  foreign_player: never sees the doc; draws the solid fill from c — a picture, not a hole
  missing_image: note and draw the solid fill (policy:robustness)
  unusable_uv: vertex mapping with no usable uv falls back to bbox and notes it
  dangling_address: a paint or uv whose address no longer resolves is kept verbatim and reported, never dropped (policy:robustness)
lifecycle: rename, duplicate and delete of a clip carry its doc — the editor's job, as for the resolv track (decision:collision-static-plugins consequences)
```

Not a bundle layout change beyond one more extensions/ subtree
(data:bundle-layout); any dotLottie runtime reading the clip is unaffected.
