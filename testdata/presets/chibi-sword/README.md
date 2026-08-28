# chibi-sword preset

The chibi-male rig carrying a long two-handed sword: the same sixteen-slot
raster cutout puppet, the same locomotion, air and reaction clips, and
three weapon attacks where the punches and the spinning kick used to be.
Like chibi-male it decodes with zero unsupported features — image layers
moved by transforms, no shapes, no expressions.

Open it:

```bash
cd editor && go run . ../testdata/presets/chibi-sword/chibi-sword.lottie
```

Regenerate after editing the generator:

```bash
cd editor && go run ./genpresets
```

## Rig

Identical to chibi-male (see that preset's README for the fifteen shared
slots, the near/far depth convention and the two-segment limb chains),
plus one slot:

| slot  | size  | anchor  | parent      | attach |
|-------|-------|---------|-------------|--------|
| sword | 21x78 | (10,10) | forearm-far | (7,26) |

Files are chibi-sword-<slot>.png; the fifteen shared slots are the same
drawings as chibi-male so the two presets stay comparable.

Three things about the weapon are deliberate:

- **It is held in the far (leading) hand.** Every strike in the rig leads
  with the far-side limb, because that limb sits on the leading (+x)
  edge and reaches the enemy in front instead of crossing the body. The
  weapon follows the same rule, and swings with the arm it is parented
  to for free.
- **Its depth follows what the character is doing, per clip.** While the
  weapon is being used the blade is lifted out of the far chain and
  drawn over the body (under the near hand, so the grip still reads),
  because depth-correct layering would bury it inside the torso in the
  middle of a cut. While the sword is merely carried it stays in the far
  chain with the arm holding it — in a gait that arm swings BEHIND the
  body half the time, and a blade pinned to the front there floats over
  a torso its own hand is behind. Clips declare this with
  `wielded()`; a per-clip choice means no depth pops mid-swing.
- **The second hand is solved, not eyeballed.** Weapon poses are
  two-handed: the near arm is fitted to the handle by a two-link solve
  (the generator's `held`), so the grip lands exactly and stays
  exact when a pose is retuned. This is a real constraint, not a
  formality — the shoulders sit 42px apart and an arm reaches 41px, so
  the hands can never meet out at arm's length. Two-handed poses have
  to keep the hands near the body's centerline and let the long blade do
  the reaching, which is how a two-handed sword is really held. A pose
  that puts the handle out of reach stops the generator instead of
  drawing a hand grasping at air beside the hilt.

The sword is drawn hanging straight down from the grip, left/right
symmetric, and is NOT dimmed like the rest of the far chain. Symmetry
means a turn (which mirrors the head and shoes) needs no second drawing;
full brightness keeps it from reading as background.

## Clips

Everything chibi-male has except kick-2 — a swordsman answers a second
attack with the weapon, not a spin kick — plus slash, slash-2 and thrust.

Away from the weapon the character carries the sword one-handed and the
inherited clips were not re-posed: the wrist holds the blade at a fixed
angle to the TORSO, so the sword tips with the body (it lies with the
character in death, leans with the run) but stops flailing whenever the
far arm swings. That is what the generator's `carry` helper
does. Clips whose torso is already near horizontal — death, slide — get
their own carry angle, because this blade is long enough to drag its
point through the floor at the default one.

The five weapon clips take both hands and set the blade per pose.

- slash -> slash-2 — firing slash during slash chains the follow-up, and
  slash2 also starts the overhead directly. slash is the diagonal
  downward cut: the hands go up over the head first, then the blade
  travels roughly 200 degrees in two frames, because a swing with no
  windup and no linear travel reads as a wave, not a cut. slash-2 is the
  heavy overhead chop — blade hauled fully vertical, a step into the
  swing, the body squashing on impact.
- thrust — the lunging stab. The blade stays LEVEL while the hands push
  forward from the hip, and the distance comes from the lunge and the
  blade's own length: the arms cannot extend without tearing the second
  hand off the hilt.
- guard / guard-hit — block with the weapon: hands at the belly, blade
  stood up in front, which covers torso and head at this blade length.
- kick and jump-kick are inherited unchanged; there are no punches,
  since a two-handed swordsman never lets go of the hilt to throw one.

## Machine verbs

Everything chibi-male answers to except punch, punch2 and kick2, plus
slash, slash2 and thrust. All three are reachable from idle; slash
additionally chains into slash2 the way punch chained into punch2.

## Rules for editing

Same as chibi-male, plus: the sword layer is parented to forearm-far,
and where it sits in the layer array is the depth decision above —
second (just under the near hand) in the weapon clips, second-to-last
(with the far arm, just over the shadow) everywhere else. Moving it
forward in a gait clip is what makes a blade float over the body while
the hand holding it is behind.

preview.png is a contact sheet of every clip sampled six times, rendered
straight from the pose data by the generator.
