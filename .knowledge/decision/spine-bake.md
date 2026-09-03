---
id: decision:spine-bake
type: decision
title: Spine Import Bakes to Keyframed Paths
---

Decided 2026-09-03 for requirement:spine-import: a Spine skeleton is
evaluated frame by frame and written as keyframed textured paths, not
translated into a parented layer rig.

```yaml
chosen: bake every frame of every animation into path keys on a shape layer per slot, textured through data:texture-document
rejected:
  layer_rig: a null layer per bone with parenting, shape layers under them — covers only bones with normal inheritance, and a weighted mesh (most of what makes a Spine character) has no parent to sit under; IK, transform and inherit modes would each need a Lottie equivalent that does not exist
  runtime_skinning: teach the player bones and weights — a second animation model inside the core, against decision:practical-subset and decision:collision-static-plugins (the core stays Lottie; extensions live in plugins)
consequences:
  - the clip is plain Lottie with one key per frame: heavy in bytes, trivial to play, editable only in the sense a rendered frame is
  - the Spine project remains the source; re-import replaces the clips (lottierepack's repack removes clips missing from the directory, so a re-import into the same -dir is a clean replacement)
  - the hull path is the default (user decision 2026-09-03: a fifth of the size, and spineboy's meshes all read right); it needs a star-shaped hull (concept:texture-uv-pipeline mesh_vertex limits). One path per triangle reproduces the mesh exactly through the centroid fan, since a triangle's fan is affine, for rigs whose inner vertices carry the deformation
  - constraints the evaluator lacks (path, physics) degrade to the keyed pose and are reported, never fatal (policy:robustness)
```

Placed under plugin/ beside plugin/texture because its output is that
plugin's document; it is a build-time tool, and a game that never imports
it links none of it.
