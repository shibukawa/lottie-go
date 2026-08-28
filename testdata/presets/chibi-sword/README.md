# chibi-sword preset

The chibi-male rig with a sword in its hand: the same sixteen-slot raster
cutout puppet, the same locomotion, air and reaction clips, and three
weapon attacks in place of the spinning kick. Like chibi-male it decodes
with zero unsupported features — image layers moved by transforms, no
shapes, no expressions.

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
| sword | 21x54 | (10,10) | forearm-far | (7,26) |

Files are chibi-sword-<slot>.png; the fifteen shared slots are the same
drawings as chibi-male so the two presets stay comparable.

Two things about the weapon are deliberate:

- **It is held in the far (leading) hand.** Every strike in the rig leads
  with the far-side limb, because that limb sits on the leading (+x)
  edge and reaches the enemy in front instead of crossing the body. The
  weapon follows the same rule, and swings with the arm it is parented
  to for free.
- **It draws in front of everything, though it hangs off the far arm.**
  Depth-correct layering would bury the blade inside the torso in the
  middle of a horizontal cut. A weapon that disappears mid-swing is a
  worse lie than one that is always visible.

The sword is drawn hanging straight down from the grip, left/right
symmetric, and is NOT dimmed like the rest of the far chain. Symmetry
means a turn (which mirrors the head and shoes) needs no second drawing;
full brightness keeps it from reading as background.

## Clips

Everything chibi-male has except kick-2 — a swordsman answers a second
attack with the weapon, not a spin kick — plus slash, slash-2 and thrust.

The inherited clips were not re-posed. The wrist holds the blade at a
fixed angle to the TORSO, so the sword tips with the body (it lies with
the character in death, leans with the run) but stops flailing whenever
the far arm swings. That is what the generator's `carry` helper
does; only the three weapon attacks set the blade angle per pose.

- slash -> slash-2 — firing slash during slash chains the follow-up, and
  slash2 also starts the overhead directly. slash is the diagonal
  downward cut: the blade goes up behind the head first, then travels
  roughly 200 degrees in two frames, because a swing with no windup and
  no linear travel reads as a wave, not a cut. slash-2 is the heavy
  overhead chop — blade hauled fully vertical, a step into the swing,
  the body squashing on impact.
- thrust — the lunging stab. The blade stays LEVEL while the whole arm
  extends: the arm angles and the blade angle are authored to cancel, so
  the point tracks straight forward instead of arcing. The near leg
  drives into a deep lunge and the body travels ~15px.
- punch / punch-2 are kept as-is: a pommel strike and a wrapped-around
  haymaker with the weapon hand. kick and jump-kick are unchanged.

## Machine verbs

Everything chibi-male answers to except kick2, plus slash, slash2 and
thrust. All three are reachable from idle; slash additionally chains into
slash2 the way punch chains into punch2.

## Rules for editing

Same as chibi-male, plus: the sword layer is parented to forearm-far and
listed FIRST in the layer array. Keep it there — moving it later in the
array is what makes a blade vanish behind the torso.

preview.png is a contact sheet of every clip sampled six times, rendered
straight from the pose data by the generator.
