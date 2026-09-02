---
id: requirement:texture-mapping
type: requirement
title: Textured Fill and Stroke
---

Status: implemented 2026-09-02 — core texture.go (paint hooks, mesh
builders, shader), plugin/texture (document, weave), editor texture.go and
textureui.go (inspector section, bbox gizmo, UV pane). Asked for once
requirement:vector-editing made paths editable: a fill or a stroke should be
able to paint an image, with UV given per path vertex, so a deforming vector
shape carries deforming art.

An extension, not Lottie. Nothing in the format expresses this, so the data
lives beside the clip under extensions/ and is edited as if it were inside
it (decision:texture-weave) — the first deliberate step past
decision:practical-subset's "subset of the spec" framing, additive and
shaped so the clip itself stays plain Lottie.

```yaml
why:
  - a vector rig gets texture without ceasing to be a vector rig — the outline animates and deforms, the art follows, and one image serves every frame
  - raster part swapping (requirement:pose-editing) moves rigid pieces; this bends them
  - gradients already proved the machinery (concept:kage-shader-usage); per-vertex UV is the same trick with a mesh instead of a quad
targets:
  items: fl and st. gf and gs keep their gradients — a textured gradient is not a thing anyone asked for
  paths: sh vertices, and the flattened points between them
  never: text layers; images stay images (policy:editor-out-of-scope)
format: data:texture-document
render: concept:texture-uv-pipeline
bind: api:texture-binding
author: ui:texture-mapping
shipped:
  s1_bbox: plugin, core hooks, bbox mapping with the keyframed placement transform, wrap and filter, weave/unweave in the editor, the inspector section and the stage gizmo
  s2_vertex: uv entries, vertex mapping over a centroid fan with UV carried by bezier t, the UV pane, seeding on switching to vertex mapping, UV kept in step with vertex insert and delete
  s3_stroke: stroke mapping — arc-length u in texture widths (or authored u), over-wide ribbon with miter widening and cap extension
  state_machines: StateMachinePlayer.OnPlayer plus lottietexture.Attach, so machine-driven characters and the editor's machine preview are dressed too
not_shipped:
  s4_quality: ear clipping for contours not star-shaped about their centroid; the no-mask fast path — the centroid fan and the mask route ship alone
  uv_pane: box-select and rotate of a UV selection — point drag, whole-set drag and wheel scale ship (ui:texture-mapping deviations)
guards:
  purity: no stored clip carries an x- member (rule:x-member-namespace) — checked with a key regexp after every store-back, refusing the store, and asserted in the editor tests
  round_trip: an untouched clip re-encodes byte for byte through Weave and Unweave; a document this plugin wrote re-encodes byte for byte (json.Marshal is canonical); a foreign-written document is canonicalized on its first save, its unknown members kept
  degradation: a textured clip loaded by a player that never calls the plugin draws its solid fills — the clip JSON is plain Lottie
  carry: clip rename, duplicate and delete move or remove the document, tested beside the resolv track's carry
  cost: a textured style costs like a gradient style, not like a plain fill; a clip that uses none pays nothing (no paint bound means no command carries a textureCmd)
  robustness: missing image, wrong UV length, dangling address, or a modifier that invalidated the UV each degrade and report, never panic (policy:robustness)
verification:
  core: texture_test.go — reference resolution over hidden items and precomps, UV count check, command wiring, transform parsing, fan / bbox / ribbon meshes, texture_shader_test.go compiles both Kage shaders
  plugin: weave/unweave round trip, unplaced entries, generic x- stripping, JSON forms, bundle IO canonical bytes, Apply with a stale entry
  editor: texture_test.go — weave on stage, edit stores pure clip plus doc, undo carries the doc, options and transform round trip, vertex insert/delete keep UV in step, UV edits, clip operations carry, unplaced kept and dropped, image binding adds the asset
  sample: examples/lottie/octopus — a generated soft-body character (mantle by bbox, eight arms per vertex, kelp along strokes); its test decodes the bundle and binds every entry, and LOTTIE_OCTOPUS_SCREENSHOT renders one frame for eyeballing
  not_covered: pixels in CI — no test renders through a GPU; the mesh and mask geometry are asserted, the shader compiled, and the sample's screenshots were checked by hand on 2026-09-02
open:
  - whether rp should offset UV per copy, which would make one path a tiled strip
  - whether a second UV set is ever wanted, and whether uv.v being the only key leaves room for it
  - mm and op could keep vertex UV by carrying UV through the boolean rather than falling back; deferred until either is used with a texture
  - normal or displacement maps, i.e. whether paint.texture ever becomes a list
```
