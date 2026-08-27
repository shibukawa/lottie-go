---
id: requirement:animation-presets
type: requirement
title: Character Animation Preset Library
---

Ship template .lottie bundles that agents customize per decision:ai-skills-workflow: swap outfit/colors/shapes, add motion, retime. Each preset is a complete bundle (data:bundle-layout) carrying the full data:preset-clip-set, its kind's data:preset-combat-clips, plus a wired state machine (data:state-machine), so a customized copy is game-ready without editor work.

```yaml
matrix:
  proportions: [chibi ~2.5 heads, standard 6-7 heads]
  kinds:
    - male-gait      # masculine humanoid movement; unarmed attacks
    - female-gait    # feminine humanoid movement; unarmed attacks
    - one-hand-sword # humanoid + sword
    - magic-staff    # humanoid + staff
    - quadruped      # four-legged animal; gait set reinterpreted (walk/trot/gallop)
  count: 2 x 5 = 10 presets
rig:
  - raster cutout: parts are WebP/PNG image layers moved by transforms + timed swaps; no vector shapes
  - three-quarter view, not profile: both eyes show, both limb chains flank the torso (game-like read)
  - two-segment limb chains: upper-arm->forearm (elbow), thigh->shin (knee), parented through body
  - slots: head (+ head-side rear-quarter, head-back), body (+ body-side), upper-arm/forearm-near/far, thigh/shin-near/far, weapon, shadow (near/far not l/r: mirror swaps sides); rotation shows by cutting between view drawings, never by mirroring in place (user feedback 2026-08)
  - facing right, near (camera-side) limbs = character's left = trailing (-x) attach; far limbs lead (+x) behind torso. Swapped, the pose reads as a back view (user-caught bug 2026-08)
  - part spec per slot: canvas size + anchor point, fixed; the contract art swaps must honor
  - template art is a deliberate placeholder: one flat color per slot, NO decorations (collar/placket/buckle etc.) - decoration belongs to customized variants; gender/character lives in motion + swapped art
status:
  - chibi-male shipped: testdata/presets/chibi-male, generator editor/genpresets, pose-sequence authoring, preview.png contact sheet, regression tests in editor/presets_test.go
  - Lottie has no mesh warp; "morphing" = transform animation + opacity/time swaps between part variants
authoring:
  - part images embedded in bundle (data:bundle-layout); WebP decodes via blank import in binaries
  - generated/authored in-repo, never downloaded (policy:risks)
  - subset-clean: decode with zero UnsupportedFeatures (decision:practical-subset)
positioning:
  - starting point, not a complete game set; agents author further clips using presets as reference
design_swap:
  - e.g. male/female samurai from one-hand-sword: regenerate part images to spec; keyframes untouched
  - depends on an image-gen capability in the user's agent stack; skill treats it as pluggable
open:
  - quadruped clip mapping (jump/slide semantics differ)
  - whether chibi and standard share one part-slot list
  - variant-swap granularity (hand poses, mouth shapes) per slot
```

Presets double as the skill's worked examples: the skill doc points at their JSON as the canonical structure to imitate.
