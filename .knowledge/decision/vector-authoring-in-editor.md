---
id: decision:vector-authoring-in-editor
type: decision
title: Vector Authoring Moves Into the Editor
---

Reverses part of policy:editor-out-of-scope: vision:state-machine-editor
gains full vector shape authoring — creating, deleting and drawing shape
layers and their contents, not just tuning them. Decided 2026-08-30 with
the user; scope answers recorded in requirement:vector-editing.

```yaml
was: authoring or editing Lottie artwork excluded; the editor is a bundler, graph surface and pose tuner, not a drawing tool
now: vector artwork — paths, fills, strokes, gradients, shape groups — is authorable in the editor. Raster stays swap-not-paint: part images are replaced whole, never painted pixel by pixel
why:
  - the raster part replacement workflow (design swap via decision:ai-skills-workflow, and in-editor image change) proved art edits belong next to the preview: seeing the change land on the real renderer is what made "many things much easier" (user 2026-08-30)
  - imported vector clips (UI assets, generated Lottie) had no edit surface at all — every color tweak was a round trip through raw JSON or an external tool
  - future vector character rigs (parts as shape layers, not images) need an authoring surface; requirement:animation-presets keeps presets raster, so this is additive, not a preset change
chosen: full authoring, both targets (imported clips and vector rigs), key editing from the start — the user declined the smaller style-only and geometry-only tiers
rejected:
  ai_workflow_only: agents can already edit shape JSON via cmd/lottierepack; rejected as the only path because a human correcting a shape by hand should not need an agent round trip — same reasoning that built requirement:pose-editing
  style_only_carve_out: smallest policy change, but leaves geometry and authoring where they were; user chose full scope
unchanged:
  - decision:json-level-animation-edit — vector edits ride the same rewrite-SetAnimation-reDecode path
  - decision:practical-subset — the editor must not author what the renderer skips; UnsupportedFeatures stays the gate
  - policy:editor-out-of-scope keeps theme/font editing, clip cutting, web interactions and scripting excluded
```

The editor's identity widens: bundler, graph surface, pose tuner, and now
vector author. Still not a raster paint tool.
