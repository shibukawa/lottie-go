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
    - slash             # diagonal downward cut. Order: hands over the head -> HIPS thrown forward + stance opened a beat early (torso and blade starting together has no weight) -> blade whips ~200deg in 2 linear frames. Finishes with the hilt at FACE height, not back at the waist - that is where the reach comes from (user feedback 2026-08)
    - slash-2           # combo follow-up: overhead chop, blade hauled vertical, step in, squash on impact; also direct via slash2
    - thrust            # lunging stab: blade stays LEVEL while the hands push from the hip; reach comes from the lunge + blade length, NOT from extending the arms
    - guard/guard-hit   # blocks with the weapon: hands at the belly, blade stood up in front (covers torso+head at this length)
  sword_rig:
    slot: {name: sword, size: 21x78, anchor: [10,10], parent: forearm-far, attach: [7,26]}
    hand: far (leading) - same limb every strike leads with, so the weapon swings with it for free
    depth: layer sits second, just under the near hand (over the body) - depth-correct order in the far chain buries the blade in the torso mid-cut. Valid ONLY because every clip keeps both hands on the hilt in front of the body; the first version let the far arm swing while carrying and the blade floated over a torso its own hand was behind (user feedback 2026-08). If a clip must swing the arm free, move the layer back to the far chain for that clip - per clip, never per pose (a hold-keyed depth flip mid-swing pops)
    grip_limb_depth: the WHOLE far arm (upper arm + forearm) moves ahead of the torso - it reaches across the front of the body onto the hilt. Left in the far chain the forearm hides behind the body and the character reads as one-handed with a stump; moving only the forearm just relocates the seam to the shoulder (user feedback 2026-08). Keep the segments' own order: on the far side the upper arm draws over its forearm, the reverse of the near chain
    art: hangs straight down from the grip, left/right symmetric (a turn mirrors head+shoes; symmetry needs no second drawing), NOT dimmed like the rest of the far chain
    carry: inherited clips are rewritten to the two-handed waist hold - hands on the hilt at the body centerline, upper arms hanging, forearms angled in, blade sweeping down and BEHIND. Costs the gaits their arm swing (the trade a greatsword makes) and buys: always-front depth, plus a carried silhouette that looks nothing like any attack, so a glance reads swinging vs not (user request 2026-08)
    carry_angle: aimed in WORLD space, not torso-relative - that is what decides ground clearance however the torso is pitched. Ramped start->end per clip: default flat, slide/death sweep further back on the way down, turns sweep through vertical so the mirror lands on something symmetric
    lean: blade world angle = body lean + arm + elbow + blade; a deep lean must be compensated or the tip points at the floor
    lean_eats_angles: limb and blade angles are body-relative - on-screen angle = body lean + limb rotation. At lean 24 a hip at -26 stands the leg vertical and a forward blade points at the floor; open stance and blade by the lean, drop the hips to keep feet grounded (user feedback 2026-08)
    elbow_branch: a folded arm has two IK solutions. Upper arm swung back to horizontal + hard elbow fold parks the elbow at shoulder height where the head hides it and the visible bend reads BACKWARDS; upper arm hanging + forearm folded in reaches the same point and reads right. Low hands -> hanging upper arm; hands above the head -> near-straight arm (user feedback 2026-08)
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
