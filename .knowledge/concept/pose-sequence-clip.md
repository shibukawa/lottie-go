---
id: concept:pose-sequence-clip
type: concept
title: Pose-Sequence Clip Structure
---

The shape requirement:animation-presets clips actually have in JSON, measured
over all 38 clips of testdata/presets. requirement:keyframe-timeline and
requirement:pose-editing rest on it.

```yaml
measured:
  layers: flat image layers (ty 2), no precomps; rig chains built with ind/parent
  animated_props: 334 r, 60 p, 56 s, 50 o — rotation carries the motion
  fixed_props: ks.a and most ks.p are the rig spec (anchor and attach point), not motion
  key_times: every animated property of a clip shares one identical time set — 0 of 38 clips hold two
  counts: 2..7 key times per clip (idle 0/48/96, punch 0/4/7/12/20, death 0/8/28/40)
consequence:
  - a keyframe tick is a whole-body pose, not a per-property key; the timeline shows poses, not a dense property grid
  - selecting a tick selects a pose; an edit writes one time across many layers
  - no key insertion on the common path — the time already exists on every animated track
generator_origin: cmd/lottie-state-editor/genpresets clips.go authors []kf{t, pose, ease}; track() emits static when every pose agrees and keyframed otherwise, so synchronized times are structural, not coincidence
rig_semantics: the generator's pose fields (armN, elbowF, blade) are Go-side only; in JSON the same joints are ks.r on named layers (upper-arm-near, forearm-far, weapon), so layer names are the rig contract an editor must read
holds: flipTrack writes hold keyframes for discrete swaps (limb sides trading, view drawings); they toggle and must not read as draggable
serialization: generated clips are key-sorted, because cmd/lottie-state-editor/genpresets builds them from map[string]any; a map round-trip reproduces that order, so rewriting a preset produces no ordering diff
```

Imported clips need not share this. testdata/editor/* — the editor's own
samples — hold two distinct time sets in 5 of 10 clips, so the fallback path
in requirement:keyframe-timeline is exercised, not theoretical.
