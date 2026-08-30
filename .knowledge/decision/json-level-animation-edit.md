---
id: decision:json-level-animation-edit
type: decision
title: Edit Clips as JSON, Not Through the Runtime
---

requirement:pose-editing writes into a clip. The editor edits the raw Lottie
document; lottie.Animation stays immutable. Implemented.

```yaml
why:
  - Animation is a compiled render form. layerNode, vectorTrack and vectorKey are unexported and lossy — member order, unmodeled members and expression strings do not survive it. Round-tripping through it would strip exactly what jsonextra.go exists to preserve
  - the write path is already public: Bundle.AnimationJSON returns the document, Bundle.SetAnimation replaces it
  - decision:runtime-package-first — the runtime is decode-once and read-optimized; a mutable authoring API there widens the core surface for a tool-only need
  - matches decision:collision-static-plugins: the editor caches a parsed document per clip and stores back on every edit, so save needs no extra sync
how:
  - editor holds data:clip-edit-document per stage clip
  - each edit rewrites the JSON, calls SetAnimation, then re-Decodes for the preview, so the stage is drawn by the real renderer
rejected:
  mutable_core: Animation.SetKeyframe pushes authoring into the runtime and still needs a raw fallback for unmodeled members
  generator_source: editing cmd/lottie-state-editor/genpresets reaches only in-repo presets, never a bundle copied into a game (requirement:pose-editing open)
stage: an edit has to reach the picture or the drag reads as broken, which is exactly how it read when it did not. A clip on stage is rebuilt in place, holding its frame. A running state machine cannot be rebuilt that way — it owns a player per state and keeps the Animation it decoded when it started, and restarting it on every mouse move is worse than useless — so selecting a pose key switches the stage to the clip itself (requirement:pose-editing park), which a parked playhead needs anyway
cost: one re-encode and re-Decode per edit. Preset clips are small — 15 layers, 7 key times at most (concept:pose-sequence-clip) — so measure before caching harder; metric:performance-targets budgets the runtime, not the editor
```

Guards the property policy:robustness asks of every rewrite: a clip this
editor touches keeps the members it did not understand.
