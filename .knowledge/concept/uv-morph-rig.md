---
id: concept:uv-morph-rig
type: concept
title: UV-Morph Rig
---

A character rig where every part is a closed textured path instead of an
image layer: the part's picture is painted through the path with a UV per
vertex (requirement:texture-mapping vertex mapping), so moving a vertex
bends the art. Built by cmd/lottieforge rig from a raster cutout preset
(requirement:animation-presets) and a set of part images; built from the
preset so that the preset's motion comes along
(decision:morph-rig-over-cutout).

```yaml
layer_rewrite:  # per clip, per image layer of the template
  keep: ind, parent, nm, ip, op, st, and ks entire — a, p, s, r, o with every key
  replace: "ty 2 + refId  ->  ty 4 + shapes: [gr{nm: slot, it: [sh (the contour), fl (fallback color), tr (identity)]}]"
  texture_doc: "paint {layer: ind, item: [0, 1], texture: slot, mapping: vertex, tint: false}; uv {layer: ind, item: [0, 0], v: uv}"
  assets: the template's image asset entries stay — id = slot, p = the file in i/, w/h = the new texture's size; that is what the paint's texture name resolves through (api:texture-binding image_resolution)
  fallback_color: the part's alpha-weighted mean color on the fl, so a player without the plugin shows a silhouette of the right hue (data:texture-document degradation)
  view_swaps: head-side, body-back and the rest are rewritten the same way; the hold keys on their opacity that turn clips use are in ks and stay
spaces:
  slot_space: the template part's pixel space — the layer's own coordinates, where a is the joint and the children's p are attach points. The contour lives here, so ks needs no change
  texture_space: the new part image, any resolution; UV is normalized over it. Slot and texture space are related by the fit
  rest_identity: at every vertex, UV = fit^-1(vertex) / texture size — the rest pose paints the image exactly where the raster rig would, which is what the faithful check measures
fit:  # how a hi-res part lands in slot space
  invariant: joint chain lengths. A segment with a child (upper-arm, thigh) maps the distance from its top cap center to its bottom cap center onto anchor -> child attach (23 px for the chibi upper arm), so the child's attach lands on the new art's joint. Terminal segments (forearm+hand, shin+foot), head and body scale their alpha bbox height onto the slot's; the spec's fit factor per slot multiplies that (towering hair — head fit 1.3)
  anchor: the joint pixel — top cap center for limbs, bottom center (neck) for heads, bottom center (hip) for the body — lands on the slot's anchor; the art may extend past the nominal slot rectangle, since nothing clips a shape layer
  width: follows the height scale; horizontal attach offsets (shoulders at x 3 and 45 of the 48-wide body) stay template values unless attach scale-x is set, which scales them by the width ratio
  cap_detection: the top and bottom caps are the extreme alpha rows; their centers are the mean x of the alpha in the rows just inside — which is why the prompts insist on rounded caps hanging vertically
contour:
  trace: alpha > 0.5 -> outer border of each connected component (marching squares); the largest kept unless the slot allows several
  simplify: Douglas-Peucker toward a vertex budget (12 for limbs, 16 for head and body; epsilon grows until the budget is met), then dilated ~2 px outward. The texture's own alpha draws the silhouette — the shader multiplies the texel, so transparent margin draws nothing — hence the path only has to contain the art — a loose hull is right, a tight one shows halos
  tangents: Catmull-Rom for smooth vertices; the two cap ends kept as corner vertices so a limb's end stays round after a bend
  star_check: every vertex sees the centroid (the fan is the mesh — concept:texture-uv-pipeline mesh_vertex); a contour that fails is decomposed, not refused
  decomposition: a failing contour is cut at reflex vertices into the fewest star-shaped sub-paths (greedy — cut the deepest reflex vertex toward the vertex that makes both halves pass, repeat), each an sh of its own in the same group under the same fl and paint, each with its own uv entry over the same texture; the union is the same silhouette, so the coverage mask is unchanged and the seam draws nothing
  welds: the vertices two sub-paths share are recorded (extensions/forge/<slot>.json welds) and every generator moves a welded pair as one, so a morph never opens the seam
  count: more vertices, finer morph control, denser fan; the budget is a spec field per slot
attachments:  # concept:attachment-kinds
  layer: one shape layer per attachment, textured like a part, parent = host, a = the item's pivot, p = the attach point in the host's space — a new ind above the clip's highest, inserted into the layer array at the kind's draw position (array order is depth; ind never renumbers)
  visibility: o copies the host's front-view layer's opacity track (the hold keys of turn clips), unless the attachment has separate view cells and layers
  transform_motion: swing bakes rotation keys — a damped pendulum (stiffness, damping per kind) integrated per frame from the pivot's world path and angle (api:layer-placement), run over the clip twice for loops and cross-faded over the last six frames into the first; sway is a sine on rotation
  vertex_motion: drape and lock ride the morph_tracks below; drape's driver angles come from the driver layers' rotation keys in the same clip, so a skirt bakes from the walk's thigh swings without a physics pass
  panels: a drape may be two layers from one drawing (skirt-front, skirt-back — the tool splits the drawing at its vertical midline into two textures), placed before and behind the limbs it covers
morph_tracks:  # what makes it a morph rig and not just a vector cutout
  where: path keys on the sh — static by default; every key holds the same vertices in the same order and only positions move (requirement:vector-editing path_topology)
  rest_stored: the rest contour rides in the bundle (extensions/forge/<slot>.json) so morph re-bakes start from it rather than from the last bake
  generators:
    breathe: vertices above the anchor scale toward and away from it by amount over period; body and head in idle and guard
    follow: hair, cape, tail — each vertex offset by lag frames of the parent's rotation track, weighted by its distance from the anchor; secondary motion baked, since there are no expressions
    drape: each vertex rotates about the pivot by weight × a driver's angle, blended across the item's width between the drivers (near thigh on the near half of a skirt, far thigh on the far half) and by distance from the pivot, so the hem leads and the waist stays; the vertex morph of cloth over a limb
    stretch: over a frame span, elongate along the parent axis by amount; launches and reaches
    squash: at a frame (landing, hit) compress along the parent axis and widen to keep area, recover over n frames; whole rig or listed parts
    bend: on a chain child, at keys where the joint angle passes a threshold, the inner-side vertices nearest the joint slide toward the parent so the crease fills; the cutout's gap at a deep bend closes
  composition: generators add offsets to the rest contour; the sum is keyed at the union of their frames plus the clip's pose keys, linear between, so the bake makes no easing decisions
  hand_edit: the editor's Shapes tab moves any vertex at any key, and MCP path verb=move_vertex does the same from an agent; UV never changes with a morph
why_two_segments: the fan interpolates UV linearly inside one part; a bend across a whole arm would shear the picture, so the rig keeps the preset's two-segment chains and morphs only near joints
cost: one textured style per part, priced like a gradient (metric:gpu-draw-cost); fifteen parts per chibi, so a crowd should budget accordingly or use the raster tier
```
