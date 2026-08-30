# chibi-sword preset

The chibi-male rig carrying a long two-handed sword: the same sixteen-slot
raster cutout puppet, the same locomotion, air and reaction clips, and
three weapon attacks where the punches and the spinning kick used to be.
Like chibi-male it decodes with zero unsupported features — image layers
moved by transforms, no shapes, no expressions.

Open it:

```bash
cd cmd/lottie-state-editor && go run . ../../examples/state-editor/presets/chibi-sword/chibi-sword.lottie
```

Regenerate after editing the generator:

```bash
cd cmd/lottie-state-editor && go run ./genpresets
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
- **It always draws over the body**, just under the near hand so the
  grip still reads, even though it hangs off the far arm. Depth-correct
  layering would bury the blade inside the torso in the middle of a cut.
  This is only safe because both hands are on the hilt in front of the
  character in every single clip: the first attempt let the far arm
  swing freely while carrying, and then the blade floated over a torso
  its own hand was behind.
- **The whole far arm comes forward with it**, ahead of the torso,
  because that is where the arm is: it reaches across the FRONT of the
  body to put its hand on the hilt. Left in the far chain the forearm
  vanishes behind the torso and the character reads as one-handed with a
  stump for the other arm; moving only the forearm just relocates the
  seam to the shoulder. Within the arm the original order stands — the
  far upper arm draws over its own forearm, the reverse of the near
  chain.
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

Every clip is two-handed. The inherited ones are rewritten by the
generator's `carry` helper into the hold anyone uses for a
blade this size: hands together on the hilt at the waist, upper arms
hanging, forearms angled in across the belly, the blade sweeping down
and BEHIND. That costs the gaits their arm swing — the trade a
greatsword makes anyway — and buys two things. The hands are in front of
the body in every clip, so the weapon never has to change depth. And the
carried silhouette, a long diagonal trailing back, is nothing like any
attack, which all drive the blade forward: a glance says whether this
character is swinging or not.

`carry` aims the blade in WORLD space, not relative to the
torso, because that is what decides whether the point clears the ground
however the body is pitched. Most clips hold one angle throughout; the
ones that end up horizontal (slide) or flat on the floor (death) sweep
it further back on the way down, and a turn sweeps it through vertical
so the mirror has something symmetric to land on.

Every sword pose also passes an eyes-up rule: head angle is relative to
the torso, so a deep forward lean would drag the face down with it, and
a hero staring at their own feet reads as beaten no matter what the rest
of the pose is doing. The neck gives the lean back, keeping the head a
little above the horizon; poses that deliberately look further up are
left alone.

The five weapon clips set the blade per pose instead.

- slash -> slash-2 — firing slash during slash chains the follow-up, and
  slash2 also starts the overhead directly. slash is the diagonal
  downward cut, and its order matters: hands up over the head, then the
  HIPS — the body is already thrown forward and the stance already open
  a beat before the blade moves, because a swing whose torso and blade
  start together has no weight behind it — and only then the blade,
  travelling roughly 200 degrees in two linear frames, since a swing
  with no windup and no linear travel reads as a wave, not a cut. The
  finish is where the reach lives, and every part of it is spent on
  that: the body thrown a long way forward, the near arm running
  straight out from the shoulder beside the head down to the hilt
  instead of folding, the hilt itself forward of the chest, and the
  blade carried nearer the horizontal — an arm folded back with the
  blade hanging steeply off it wastes most of its length — and the near
  shoulder itself driven forward under the head's base, with the far one
  giving ground behind it. That last one is not decoration: once the arm
  is straight, rotation has nothing left to give, and the root is the
  only thing that can still travel. The leading knee folds deep under
  all of it, which is where the weight reads from. slash-2 answers it
  from the other direction: a rising cut. The blade drops behind to
  horizontal while the knees fold under it, then comes up through the
  front and finishes high and forward. Two downward cuts back to back
  read as the same attack played twice, however different the numbers
  are; up after down is a combo.
- thrust — the lunging stab. The blade stays LEVEL throughout, and the
  point goes out on the ARMS as much as the body: the near shoulder
  drives forward, the far elbow tucks at the waist and its forearm
  drives the hilt out past the chest, so the whole weapon ends up ahead
  of the character rather than the blade alone. The body travels less
  than it might, because the tip would otherwise run off the canvas.
- guard / guard-hit — block with the weapon, held out in FRONT: a guard
  whose hilt sits behind the body's own centerline covers nothing and
  reads as flinching away rather than meeting the hit. The blade rises
  forward rather than straight up, since a head this size is wide
  enough that a vertical blade in front of it just crosses the face.
- kick and jump-kick are inherited unchanged; there are no punches,
  since a two-handed swordsman never lets go of the hilt to throw one.

## Machine verbs

Everything chibi-male answers to except punch, punch2 and kick2, plus
slash, slash2 and thrust. All three are reachable from idle; slash
additionally chains into slash2 the way punch chained into punch2.

## Rules for editing

Same as chibi-male, plus: the sword layer is parented to forearm-far and
listed second, just under the near hand. Keep it there — but only as
long as every clip keeps both hands on the hilt in front of the body. A
clip that lets the far arm swing free while carrying has to move the
blade back into the far chain, or it will float over the torso while
the hand holding it is behind.

preview.png is a contact sheet of every clip sampled six times, rendered
straight from the pose data by the generator.
