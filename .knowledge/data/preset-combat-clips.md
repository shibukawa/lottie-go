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
    - punch-2           # combo follow-up
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
  - guard               # loop: hold stance while input held
  - guard-hit           # oneshot: flinch while guarding, back to guard
common:
  - attacks/guard-hit: oneshot; return to idle (fall-loop if airborne) on complete
  - naming: `-anim` suffix like base set
open:
  - whether combo follow-ups are separate clips or markers within one clip
```
