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
    - punch-2           # spinning haymaker: face-on upright windup (no lean-back), then the swing spins past front - back view (darker body-back + one-eyed head-side: eyes stay on the opponent, the head turns only halfway) from strike to end, shoulders traded, hips planted; fires directly via punch2 event too
    - kick
    - kick-2            # spinning high kick, punch-2 pattern on the legs: face-on knee-fold chamber, spin to back view (body-back + one-eyed head-side), near leg whips up from leading edge; kick combo follow-up or direct kick2 event
    - jump-kick
  one-hand-sword:       # built as chibi-sword: chibi-male rig + sword slot, kick-2 dropped
    - slash             # diagonal downward cut; windup lifts the HAND (upper arm ~140deg), not just the blade, or the fist parks behind the hip and the sword reads as sticking out of the back; ~185deg of travel in 2 linear frames
    - slash-2           # combo follow-up: overhead chop, blade hauled vertical, step in, squash on impact; also direct via slash2
    - thrust            # lunging stab: blade stays LEVEL while the arm extends - arm and blade angles authored to cancel so the point tracks straight
  sword_rig:
    slot: {name: sword, size: 21x54, anchor: [10,10], parent: forearm-far, attach: [7,26]}
    hand: far (leading) - same limb every strike leads with, so the weapon swings with it for free
    depth: layer listed FIRST (frontmost) despite hanging off the far chain; depth-correct order buries the blade in the torso mid-cut
    art: hangs straight down from the grip, left/right symmetric (a turn mirrors head+shoes; symmetry needs no second drawing), NOT dimmed like the rest of the far chain
    carry: inherited clips are not re-posed - blade angle is derived per key to hold a fixed angle to the TORSO (tips with the body, stops flailing with the arm swing); only weapon attacks author blade per pose
    lean: blade world angle = body lean + arm + elbow + blade; a deep lean must be compensated or the tip points at the floor
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
