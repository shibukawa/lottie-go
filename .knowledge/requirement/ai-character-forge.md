---
id: requirement:ai-character-forge
type: requirement
title: AI Character Forge (Image Prompt to UV-Morph Rig)
---

Status: designed 2026-09-03, not implemented. Make a new game character
from a text description, with an image model (Gemini, Grok) drawing the
art and a coding agent doing everything else: the skill writes the prompts
that get part-split art back, a CLI cuts the sheets into rig parts, turns
the parts into a textured vector rig (requirement:texture-mapping vertex
mapping) that inherits every clip of a preset
(requirement:animation-presets), and the existing verify loop
(decision:ai-skills-workflow) closes it. Flow: flow:ai-character-forge.
Formats: data:character-forge-spec. Rig: concept:uv-morph-rig. Decisions:
decision:image-gen-by-prompt, decision:morph-rig-over-cutout. Skill:
skills/lottie-character-forge.

```yaml
goals:
  - one spec in, one .lottie out: the user describes a character, pastes generated images, and gets a bundle with the base preset's whole clip set and machine, in its own look
  - part split is the image model's job, guided: the prompt carries the rig's slot list, a grid template image and the drawing rules that make a part usable (joint caps, neutral hang, key-color background); the tool measures what came back and names the cells to redo
  - motion inherits: every template clip's transform keys drive the new parts unchanged (concept:uv-morph-rig rest_identity); new motion is added on top — morph tracks (breathing, follow-through, squash, bend) baked into path keys, and new clips by the preset skill's recipes
  - the agent never edits pixels or UV by hand: contours and UV are computed from the alpha, morphs are generated from parameters; hand fixes happen in the editor (requirement:editor-mcp)
  - what the character wears or grows beyond the rig's slots — hair locks, a skirt, a cape, sleeves, ribbons, a tail, a hat — is an attachment of a declared kind (concept:attachment-kinds); the kind defaults its parent, draw order, cell, prompt wording and motion, and the tool adds its layer to every clip
inputs:
  spec: data:character-forge-spec character.json — base preset, description, style, key color, props, extra parts, morph list, new clips
  images: one PNG per sheet from any image model; the tool never calls the model (decision:image-gen-by-prompt)
outputs:
  bundle: <name>.lottie — clips rewritten to shape layers, i/ holding the part textures, extensions/texture/<clip>.json per clip, the base machine plus the spec's new states
  work_dir: the lottierepack layout plus sheets/, prompts/, parts/, report.json, faithful.png and preview/
tiers:  # each usable alone; the skill picks by what the user has
  t0_raster: cut sheets into the preset's slot sizes and swap the raster rig's images — no new rig code, the path skills/lottie-character-preset documents; art lands at 15x27 forearms, so it suits pixel-art styles only
  t1_morph: parts become textured paths, template coordinates kept, textures at any resolution; the primary tier
  t2_motion: morph tracks and new clips on the t1 rig
tooling:  # cmd/lottieforge — one command, four subcommands; designed, not built
  grid: character.json -> sheets.json, work/sheets/*.template.png (key color, cell borders, label band) and work/prompts/*.md filled from the skill's templates with the base rig's slots
  cut: sheets + sheets.json -> work/parts/<slot>.png at sheet resolution, report.json (per cell — empty, border, multi, halo), contact.png, plus the derived far-side parts and the shadow
  rig: parts + base preset -> bundle; default writes t1 (concept:uv-morph-rig — trace, simplify, decompose to star-shaped sub-paths, fit into slot space, identity UV, rewrite every clip's image layers as shape layers, emit the texture docs); inserts each attachment's layer into every clip at its kind's draw order with the host's opacity track; -raster writes t0
  morph: bakes the spec's morph list into path keys of the named clips, and the attachments' kind motion — swing on rotation from the pivot's world motion (api:layer-placement), drape from the drivers' angles, lock as both; vertex counts preserved; re-runnable, since generators start from the stored rest contour
prerequisites:  # gaps in the shipped tools the loop depends on; small, first
  - lottierepack dumps and repacks extensions/ (today a/, i/, s/ only: a texture doc edited in work/ is lost on repack)
  - lottiecheck -render applies extensions/texture through lottietexture.Load before drawing (today a textured clip renders its fallback solids, so the visual check cannot see the art)
  - lottiecheck reports a texture doc entry whose address or UV count does not resolve (today only the runtime notes it)
verification:
  faithful: frame 0 of idle-anim rendered from the t1 rig matches the same frame from a t0 rig built from the same parts within a pixel-difference budget — both drawn by lottie-go, so the conversion is checked by the renderer, not by eye; rig writes faithful.png and exits non-zero over budget
  playable: lottiecheck exits 0 on the bundle; UnsupportedFeatures empty; every texture doc address resolves
  visual: contact sheets per clip (lottiecheck -render) read by the agent, the preset skill's rule — a wrong joint fit shows as a detached limb in the walk's contact pose, a bad contour as an untextured wedge
  editor: the bundle opens in cmd/lottie-state-editor; the Shapes tab shows each part's paint and UV; MCP render answers the agent while a human adjusts
  tests: lottieforge's own tests forge a character from generated parts (the string-art recipe of the presets, upscaled), so the repository commits no model output (policy:risks)
rules:
  - the clip JSON stays plain Lottie; a foreign player sees the parts' mean colors as solid fills (data:texture-document degradation)
  - a path's vertex count and order never change after rig: morphs move vertices, never add them (requirement:vector-editing path_topology; UV is per vertex)
  - a contour the fan cannot texture is decomposed into star-shaped sub-paths sharing the drawing, seams welded (concept:texture-uv-pipeline mesh_vertex limits); slots split only for motion — a lock that swings is its own slot, a lobe of hair that does not is a sub-path
  - a host is drawn complete without its attachments and an attachment complete with a root overlap; ornaments that move with their part are painted into it, never attached (concept:attachment-kinds bake_or_attach)
  - the template's joint chain lengths are the invariant a fit honors, not the canvas sizes (concept:uv-morph-rig fit)
  - near/far, the shadow and derived views come from one drawing each; the model draws every slot once
  - no model-generated or downloaded art is committed here (policy:risks)
out_of_scope:
  - calling Gemini or Grok from the tool; background removal beyond a key color; rigging arbitrary poses (the model is asked for the neutral hang and told to redo what is not)
  - skeletal mesh deformation with bone weights: lottie-go has per-vertex UV on a path and nothing more; a bend is a morph key
  - quadruped and standard-proportion presets until requirement:animation-presets ships them; the forge inherits whatever rig the base has
open:
  - swing tuning: one damping and one stiffness per kind, or per attachment; whether the pendulum reads gravity in world space during death and slide
  - heads whose hair towers over the slot: fitting by face height needs a landmark the alpha does not give; the per-slot fit factor is the manual answer
  - whether t1 should also carry a raster machine in the same bundle (one look, two rigs) so a game picks per platform
  - ear clipping in the renderer would make the tool's decomposition unnecessary; the welds would remain for morphs across the seam
```
