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
  two-hand-sword:       # built as chibi-sword: chibi-male rig + sword slot; punch/punch2/kick-2 dropped (both hands stay on the hilt)
    - slash             # diagonal downward cut; windup lifts the HANDS over the head, not just the blade, or the hilt parks at the hip and the sword reads as sticking out of the back; ~200deg of travel in 2 linear frames
    - slash-2           # combo follow-up: overhead chop, blade hauled vertical, step in, squash on impact; also direct via slash2
    - thrust            # lunging stab: blade stays LEVEL while the hands push from the hip; reach comes from the lunge + blade length, NOT from extending the arms
    - guard/guard-hit   # blocks with the weapon: hands at the belly, blade stood up in front (covers torso+head at this length)
  sword_rig:
    slot: {name: sword, size: 21x78, anchor: [10,10], parent: forearm-far, attach: [7,26]}
    hand: far (leading) - same limb every strike leads with, so the weapon swings with it for free
    depth: per CLIP, not global. Weapon clips lift the layer out of the far chain to just under the near hand (over the body) - depth-correct order buries the blade in the torso mid-cut. Carried clips leave it in the far chain with its own arm: in a gait the far arm swings behind the body half the time and a front-pinned blade floats over a torso its own hand is behind (user feedback 2026-08). Per-clip, not per-pose: a hold-keyed depth flip mid-swing pops
    art: hangs straight down from the grip, left/right symmetric (a turn mirrors head+shoes; symmetry needs no second drawing), NOT dimmed like the rest of the far chain
    carry: inherited clips are not re-posed - blade angle is derived per key to hold a fixed angle to the TORSO (tips with the body, stops flailing with the arm swing); weapon clips author blade per pose
    carry_per_clip: a long blade needs its own carry angle where the torso is already near horizontal (death, slide) or the point drags through the floor
    lean: blade world angle = body lean + arm + elbow + blade; a deep lean must be compensated or the tip points at the floor
    two_handed: shoulders are 42px apart, an arm reaches 41px - the hands can NEVER meet at arm's length. Two-handed poses keep both hands near the body centerline and let blade length do the reaching (which is how a two-handed sword is really held). Solve the second arm as a two-link chain onto the handle (gripped ~6px toward the pommel from the leading hand) and make out-of-reach poses a generator error, not a clamp
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
