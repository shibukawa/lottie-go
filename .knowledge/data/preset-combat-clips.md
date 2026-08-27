---
id: data:preset-combat-clips
type: data
title: Preset Combat Clip Sets
---

Attack clips layered on data:preset-clip-set, varying per preset kind (requirement:animation-presets). Every kind carries 3-4 attacks so games can build combos and variety without authoring. All oneshot; combo chaining follows the windup/strike/recover pattern of the existing combo sample (editor/gensamples).

```yaml
per_kind:
  unarmed:              # male-gait / female-gait
    - punch
    - punch-2           # haymaker: winds up turned away (head-back view, shoulders traded), also fires directly via punch2 event
    - kick
    - jump-kick
  one-hand-sword:
    - slash
    - slash-2           # combo follow-up
    - thrust
    - air-slash         # from jump/fall
  magic-staff:
    - cast              # quick projectile
    - cast-charge       # hold then release
    - cast-area         # AoE, longer recover
    - staff-swing       # melee fallback
  quadruped:
    - bite
    - claw
    - pounce            # closes distance; ends in landing pose
defense:                # all kinds
  - guard               # loop: the guard event toggles it on and off (was a boolean; user wanted an event 2026-08)
  - guard-hit           # oneshot: flinch while guarding, back to guard
common:
  - attacks/guard-hit: oneshot; return to idle (fall-loop if airborne) on complete
  - strike with the far-side limb (leading edge, reaches the enemy in front); combo follow-up swings the trailing (near) limb (user feedback 2026-08)
  - kicks chamber (knee folded) until the strike frame, then snap the knee straight on impact
  - naming: `-anim` suffix like base set
open:
  - whether combo follow-ups are separate clips or markers within one clip
```
