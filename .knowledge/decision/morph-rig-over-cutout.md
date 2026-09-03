---
id: decision:morph-rig-over-cutout
type: decision
title: Forge Builds a UV-Morph Rig on the Cutout Preset's Motion
---

The forged character is a concept:uv-morph-rig — textured paths — rather
than a raster cutout (what the presets are) or a mesh, and it is built by
rewriting the preset's clips, not by authoring a rig of its own. Decided
2026-09-03 for requirement:ai-character-forge (user: the target is this
project's UV vertex morphing data).

```yaml
why_morph:
  - resolution: an image model returns 1024 px sheets; the chibi rig's slots are 15..72 px. A textured path keeps the template's coordinates and samples the hi-res texture at draw scale, so the art stays sharp; the raster rig would downscale it, or every clip would need rescaling
  - deformation: cutout parts are rigid; vertex morphs give breathing, follow-through, squash and creases that close at a bend — what separates a puppet from a character, all bakeable as path keys
  - same motion: the morph rig's ks is the preset's ks, so the clips and the machine come for free and the preset skill's motion recipes still apply
why_not_a_new_rig:
  - the presets encode the motion decisions of data:preset-clip-set and data:preset-combat-clips; a rig with its own slot list would inherit none
  - one conversion, tested against the raster rig by pixel difference, is verifiable; a hand-authored vector rig is not
why_not_mesh: none in lottie-go and none in Lottie; per-vertex UV on a fan is the deformation the renderer has (requirement:animation-presets status)
keeps:
  - the raster tier (-raster): pixel-art styles, and players without plugin/texture
  - two-segment chains: the fan interpolates linearly inside a part, so a bend across a whole arm would shear; segments are where the joints are
costs:
  - fifteen textured styles per character, each priced like a gradient (metric:gpu-draw-cost)
  - the look is lottie-go specific: plain players draw silhouettes in the parts' mean colors
  - contours must be star-shaped until ear clipping ships (requirement:texture-mapping not_shipped); hair and capes split into slots
```
