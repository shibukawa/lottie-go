---
id: requirement:spine-import
type: requirement
title: Spine Skeleton Import
---

Status: implemented 2026-09-03 — plugin/spine (lottiespine), the
`-import-spine` mode of cmd/lottierepack, texture documents applied by
cmd/lottiecheck. Read a Spine 4.x export (JSON skeleton plus .atlas or loose
images) and produce a bundle whose meshes keep deforming with their art,
through requirement:texture-mapping's per-vertex UV. Asked for as "take
Spine data in as the extension's UV morphing".

```yaml
why:
  - Spine is where skinned 2D characters are authored; a mesh there is vertices with UV moved by bones and deform keys, which is exactly a textured path with per-vertex UV here
  - the runtime already renders it (concept:texture-uv-pipeline); only the data was missing
  - a game keeps one player, one bundle format and one state machine API for hand-authored Lottie and Spine-born characters alike
approach: bake, not rig
  what: evaluate the skeleton at every frame of every animation and write what it draws as keyframes; no bones, constraints or weights survive into the clip
  why: Lottie has no skinning, no IK, no inherit modes and no draw-order keys; a rigged translation would cover only "normal" bones and lose every weighted mesh, while baking covers the whole update pipeline uniformly. Editability inside the editor is given up knowingly — the source of truth stays the Spine project
  cost: one key per frame per path; a clip is large in JSON (a 5 s clip of a 70-bone character runs to tens of MB pretty-printed) but zips to a few MB, and -mesh hull cuts it by ~5x
mapping:
  animation: one clip per Spine animation, id = name made file-safe; the setup pose alone when there are none
  slot: one shape layer, ind = slot index + 1 (stable addresses), listed in reverse slot order (Lottie draws the first layer on top); slot blend → bm (additive 16, multiply 1, screen 2)
  attachment: one group per region or mesh a slot ever shows in the clip — paths, then a fill whose c is slot color × attachment color and whose o is alpha × 100, held at 0 while another attachment shows; the setup attachment's group first
  mesh: one closed 3-vertex path per triangle (MeshTriangles, default) so inner vertices deform exactly, or one path of the hull vertices (MeshHull); weighted vertices are blended in world space, deform offsets added per weight entry
  region: one 4-vertex path over the packed (whitespace-stripped) rect
  uv: normalized over the atlas page, through the region's offsets and 90° rotation; loose images map 1:1
  paint: MappingVertex, tint default (true) so the fill color carries the slot tint; texture = page name without extension, or the attachment path
  images: atlas pages into i/, un-premultiplied when the atlas says pma (Lottie assets are straight alpha); an unreadable image leaves no asset, the paint keeps the name for Player.SetTexture
  coordinates: Y flipped; origin = top-left of the composition; composition = the skeleton's declared bounds widened to every animation's reach (SkeletonBounds keeps them as declared); Scale multiplies everything
  events: markers (cm, tm, dr 0)
  machine: optional; a looping PlaybackState per clip, an Event input per clip, one GlobalState with a transition per event; initial "idle" when present
  bones: optional null layers with the baked world transform, for sockets and tools
runtime_evaluated:
  - bone inherit: normal, onlyTranslation, noRotationOrReflection, noScale, noScaleOrReflection; the 4.2 inherit timeline
  - timelines: rotate, translate(x/y), scale(x/y), shear(x/y); attachment, rgba/rgb/alpha (rgba2/rgb2 read the light color); ik mix/softness/bend/compress/stretch; transform mixes; deform (4.2 attachments form and the older deform form); curves absolute (4.x) and normalized (3.x), stepped
  - constraints: IK one- and two-bone (softness, stretch, uniform, bend direction), transform (world/local × absolute/relative), in Spine's update order (constraints sorted by order, children of constrained bones after)
  - skins: default plus any requested, later ones overriding; linked meshes resolve their parent
not_converted:   # each is a Result.Note, never an error
  - path constraints, physics constraints: bones keep their keyed pose
  - clipping attachments, bounding boxes, points, paths: not drawn
  - draw-order timelines: setup order throughout
  - sequences: first frame only
  - two-color tint (dark color): ignored
verification:
  unit: plugin/spine/spine_test.go — world transforms and inherit modes, weighted mesh and region placement, deform interpolation, curves, two-bone IK reach and bend, one-bone IK plus transform constraint, atlas parsing with rotation and offsets, full conversion decoding into a playable bundle in both mesh modes with markers, machine, blend mode, null layers and the x-member purity guard
  sample: spineboy (Spine's example, 4.2.22) imported and rendered through lottiecheck -render on 2026-09-03 for every animation — poses, IK legs, weighted head mesh and slot tints checked by eye; not committed (Spine example assets are not ours to redistribute)
  not_covered: pixels in CI (no GPU), path constraints
tool: "lottierepack -import-spine hero.json -dir work [-out hero.lottie] [-atlas f] [-images dir] [-skin a,b] [-fps 30] [-scale 1] [-mesh triangles|hull] [-bounds union|skeleton] [-bones] [-machine=false]"
open:
  - path constraints — the one Spine feature a common rig (a tail, a vine) needs that is missing
  - draw-order keys could become a layer per (slot, order run); not asked for yet
  - clipping could become a mask on the clipped layers
```

The importer is a converter, not a player: policy:out-of-scope is untouched,
and the bundle it writes is what any lottie-go player already reads.
