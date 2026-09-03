# Motion: inherited, morphed, and new

A forged character has three layers of motion. The first costs nothing;
the second is what the morph rig exists for; the third is the preset
skill's craft applied to a new look.

## 1. Inherited: the preset's clips

Every clip of the base preset — idle, walk, run, the turns, jump/fall,
hurt, death, guard, and the attack set of the base — plays on the forged
rig unchanged, because the rewrite keeps every layer's transform keys.
The state machine carries over as is: `sm.Fire("jump")` works before
any motion work starts. Retiming, amplifying or softening these is the
lottie-character-preset skill's business (clips.md there): edit
keyframes in the dumped clip JSON, repack, render, look.

What does *not* carry over automatically is design-dependent: a long
prop needs its carry angle checked in the near-horizontal clips (death,
slide), exactly as chibi-sword's rig.md warns, and a head with towering
hair may need the idle bob reduced.

## 2. Morph tracks: baked path keys

Morphs are offsets added to a part's rest contour and keyed on its path.
They never add or remove vertices. The spec lists them; `lottieforge
morph` bakes the sum per clip at the union of the generators' frames and
the clip's pose keys, with linear easing between (the pose keys already
carry the clip's timing).

| generator | what it moves                                                       | parameters                          | use it for |
|-----------|---------------------------------------------------------------------|-------------------------------------|------------|
| breathe   | vertices above the anchor scale toward/away from it                 | amount (0.03 = 3%), period (frames) | body and head in idle, guard, fall-loop |
| bend      | at keys where the joint angle passes `threshold`, the inner-side vertices nearest the joint slide toward the parent | threshold (deg), reach (px, default 4) | forearms and shins in every clip: closes the cutout crease |
| squash    | at frame `at`, compress along the parent axis by `amount`, widen to keep area, recover over `recover` frames | at, amount, recover, parts | landings (jump, fall-loop → idle), hits (hurt, guard-hit), the kick's contact |
| follow    | each vertex lags the parent's rotation track by `lag` frames, weighted by distance from the anchor × `weight` | lag, weight                          | hair, cape, tail, sleeves — secondary motion, since no expressions run |
| stretch   | on a frame span, elongate along the parent axis                     | from, to, amount                    | the jump's launch, the thrust's reach |

```jsonc
"morph": [
  { "generator": "breathe", "parts": ["body", "head"], "clips": ["idle-anim", "guard-anim"], "amount": 0.03, "period": 48 },
  { "generator": "bend",    "parts": ["forearm-near", "forearm-far", "shin-near", "shin-far"], "clips": "all", "threshold": 60 },
  { "generator": "squash",  "parts": "all", "clips": ["jump-anim"], "at": 22, "amount": 0.12, "recover": 6 },
  { "generator": "follow",  "parts": ["hair-back"], "clips": "all", "lag": 3, "weight": 0.4 }
]
```

Rules of thumb:

- Morph amounts are small. 3% breathing reads; 8% reads as a balloon.
  Squash at 12% on a landing is already cartoon.
- A morph on a loop must return to its start: generators with a period
  align to the clip's length (`morph` warns when `period` does not
  divide `op`).
- `bend` needs the joint's angle, which the tool reads from the
  parent-child rotation keys — it works on chain children only.
- `follow` is baked from the parent's *world* rotation over the clip,
  so a follow on hair-back reacts to body lean and head nod together.
- The renderer's fan interpolates UV linearly inside a part: a morph
  that folds a contour back on itself (a vertex crossing the centroid's
  line of sight) leaves a wedge. Keep offsets under a third of the
  part's width; `morph` checks the star property at every baked key
  and refuses the offending one.

Manual morphs — a specific vertex at a specific key — are made in the
editor's Shapes tab or through MCP `path verb=move_vertex`. Record the
intent in the spec as a note; a re-bake regenerates from the rest
contour and would lose them otherwise.

## 3. New clips

A character-specific move (a mage's cast, a kick with a tail whip, a
bow draw) is a new clip. The recipe is the preset skill's: copy the
nearest template clip to `work/<new>-anim.json`, rename `nm`, reshape
its keys, wire a state. The forge adds two things:

- **Morphs apply to new clips too.** Name the clip in a generator's
  `clips` list (or use `"all"`) and re-run `morph` after the keys are
  reshaped, since the bake reads the final pose keys.
- **A morph-only move exists.** A clip whose transforms barely change
  but whose contours do — a jelly wobble, a tail wag, an inhale before
  a shout — is authored as path keys directly. Start from the copied
  clip's rest contours (`extensions/forge/<slot>.json` holds them) and
  key vertex positions at three or four poses; the stretch and follow
  generators give a first pass to edit from.

```jsonc
"clips": {
  "add":     [ { "name": "cast-anim", "from": "punch-anim" } ],
  "machine": [ { "state": "cast-state", "animation": "cast-anim", "event": "cast",
                 "from": ["idle-state", "walk-state"], "returns": "idle-state" } ]
}
```

`rig` creates the copies and the states with the preset conventions
(one-shots return via `clipDone`, the new event declared in `inputs`,
`jump` stays first in every grounded state); reshaping the copy's keys
is the agent's work afterwards.

## What to look at

`lottiecheck -render` samples four frames per clip by default; for
motion work ask for more (`-samples 8`) and read these:

- **idle** frame 0 and mid-loop: breathing visible, nothing drifting.
- **walk** at the contact pose (frame 0) and the passing pose (quarter):
  a detached knee or elbow here is a fit problem, not a motion one.
- **the base's main attack** on its strike frame: bend closes the elbow
  crease; the prop still points where the template pointed it.
- **a turn** at its midpoint: view swaps happen on the right frames
  (the rewrite kept the hold keys); the far parts read darker.
- **jump** at the landing: squash lands on the frame the spec named.
