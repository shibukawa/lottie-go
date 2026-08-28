package main

// readme is written next to the preset. It is the seed of the future
// skill document: everything an agent needs to customize the preset
// without breaking it.
const maleReadme = `# chibi-male preset

A 2.5-heads-tall character as a raster cutout rig: fifteen PNG part images
moved by transforms and nothing else — no vector shapes, no expressions,
no effects — so every clip decodes with zero unsupported features in
lottie-go and plays in any Lottie player. The drawing is a deliberate
placeholder — one flat color per part, three-quarter view — because the
preset's value is the motion; "male" describes the gait, not the art.

Open it:

` + "```bash" + `
cd editor && go run . ../testdata/presets/chibi-male/chibi-male.lottie
` + "```" + `

Regenerate after editing the generator:

` + "```bash" + `
cd editor && go run ./genpresets
` + "```" + `

## Rig

The character leads right; a left-facing character is the runtime mirror
(` + "`Mirrored` / `MirrorX`" + `), so there are no *-left clips. Limbs are
named by depth (near/far), not left/right, because the mirror swaps which
body-side is near. Facing right, the near (camera-side) limbs are the
character's left side and attach on the trailing (-x) edge; the far limbs
attach on the leading (+x) edge and show from behind the torso — swap
those and the pose reads as a back view. Arms and legs are two-segment
chains: the forearm is parented to the upper arm (elbow), the shin to the
thigh (knee).

Parts live in the bundle under ` + "`i/`" + ` and as loose files under
` + "`parts/`" + `. Each is referenced by every clip with a fixed anchor
(the joint it rotates around) and position in its parent's space:

| slot           | size  | anchor  | parent         | attach    |
|----------------|-------|---------|----------------|-----------|
| head           | 72x60 | (36,56) | body           | (24,3)    |
| head-side      | 72x60 | (36,56) | body           | (24,3)    |
| head-back      | 72x60 | (36,56) | body           | (24,3)    |
| body           | 48x48 | (24,45) | -              | (128,179) |
| body-side      | 48x48 | (24,45) | body           | (24,45)   |
| body-back      | 48x48 | (24,45) | body           | (24,45)   |
| upper-arm-near | 15x27 | (7,4)   | body           | (3,8)     |
| forearm-near   | 15x30 | (7,4)   | upper-arm-near | (7,23)    |
| upper-arm-far  | 15x27 | (7,4)   | body           | (45,8)    |
| forearm-far    | 15x30 | (7,4)   | upper-arm-far  | (7,23)    |
| thigh-near     | 18x33 | (9,4)   | body           | (16,45)   |
| shin-near      | 21x36 | (9,4)   | thigh-near     | (9,29)    |
| thigh-far      | 18x33 | (9,4)   | body           | (32,45)   |
| shin-far       | 21x36 | (9,4)   | thigh-far      | (9,29)    |
| shadow         | 84x15 | (42,7)  | -              | (128,236) |

Files are chibi-male-<slot>.png. Canvas is 256x256, ground at y=236,
60fps.

**Design swaps replace the images and nothing else.** A new outfit or a
new character (samurai, knight, robot) is fifteen new images at the same
sizes with the same anchor meanings: head drawn with the neck at (36,56),
limb segments hanging straight down from their joint, the shoe toe
pointing +x. Keep the far-side copies darker than the near side so depth
still reads. The keyframes never change.

## Clips

Loops: idle, walk, run, fall-loop, guard. Everything else is one-shot and
returns by itself through the machine.

- idle / walk / run / slide — ground movement. There is no dash clip: a
  dash is run played faster (per-state speed or ` + "`SetSpeed`" + `).
- idle-turn / walk-turn — rotate through the CAMERA side: the front
  stays visible the whole way, growing a touch as it passes the
  viewer, and the drawing mirrors at the midpoint on a self-mirror
  pose — limbs near straight, joints at zero, since a bent joint
  carried across the flip would kink the wrong way — so the geometry
  holds still while the near/far identities trade (a game-friendly
  lie). Each runs from its gait's rest/contact pose to that pose's
  in-place negation, so the mirrored gait picks up seamlessly. Turn
  clips end facing the other way; the game flips its Mirrored flag
  when the turn completes.
- run-to-idle — the foot-brake skid, and the reason run has NO turn
  clip: the leading leg thrusts out and slides on its sole, the
  trailing toe drags, the torso leans back — hips stay near standing
  height, nothing like slide's squat. A runner brakes with this and
  the game then decides: stand, or start the mirrored run.
- jump -> fall -> fall-loop — the air chain; each hands over on complete.
- punch -> punch-2 — firing punch during punch chains the follow-up,
  and punch2 also starts the haymaker directly from idle. Strikes lead
  with the far-side limb (the one on the leading edge, so it reaches
  the enemy in front). punch-2 is the spinning haymaker: the windup
  stays face-on and upright with the arm cocked behind, and when the
  swing whips through the body has spun past front — from the strike
  to the end the darker body-back shows with the shoulders traded and
  the hips planted. The head turns only halfway: the rear-quarter
  head-side stays up with one eye on the opponent, since the eyes
  never leave the target.
- kick, jump-kick — ground and air kicks: chambered (knee folded) until
  the strike frame, then the knee snaps straight on impact.
- kick -> kick-2 — the spinning high kick, punch-2's leg-side sibling:
  face-on knee-fold chamber, then the spin shows the darker back — the
  head at the one-eyed rear-quarter view, still watching the target —
  while the bright near leg whips up from the leading edge. Fires as
  the kick follow-up or directly via kick2.
- guard / guard-hit — the guard event toggles the stance on and off; a
  hurt event while guarding plays guard-hit instead of hurt.
- hurt, death — reachable from anywhere via the global state. death holds
  its last frame and has no way out: the game stops firing events and
  restarts the player (or the editor's Restart) to revive.

## Machine verbs

Events: walk, run, stop, turn, jump, slide, punch, punch2, kick, kick2,
guard, hurt, die. Booleans: grounded (jump needs it true; fall-loop exits to
idle when it turns true). guard toggles: it enters guard-state from idle
and leaves it again.
clipDone is internal: the OnComplete interaction fires it so one-shot
states hand over without a game-side timer.

## Rules for editing

- No expressions, effects, 3D, or time remap: keyframes only.
- Blend modes beyond normal/multiply/screen and mask modes beyond
  add/subtract are skipped by lottie-go — do not rely on them.
- After any edit, re-open the bundle and check
  ` + "`Animation.UnsupportedFeatures()`" + ` is empty (the generator's
  verify step shows the pattern).

preview.png is a contact sheet of every clip sampled six times, rendered
straight from the pose data by the generator.
`

// swordReadme documents the armed variant: same rig plus one slot, same
// motion vocabulary plus three weapon attacks.
const swordReadme = `# chibi-sword preset

The chibi-male rig carrying a long two-handed sword: the same sixteen-slot
raster cutout puppet, the same locomotion, air and reaction clips, and
three weapon attacks where the punches and the spinning kick used to be.
Like chibi-male it decodes with zero unsupported features — image layers
moved by transforms, no shapes, no expressions.

Open it:

` + "```bash" + `
cd editor && go run . ../testdata/presets/chibi-sword/chibi-sword.lottie
` + "```" + `

Regenerate after editing the generator:

` + "```bash" + `
cd editor && go run ./genpresets
` + "```" + `

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
  ` + "`wielded()`" + `; a per-clip choice means no depth pops mid-swing.
- **The second hand is solved, not eyeballed.** Weapon poses are
  two-handed: the near arm is fitted to the handle by a two-link solve
  (the generator's ` + "`held`" + `), so the grip lands exactly and stays
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
far arm swings. That is what the generator's ` + "`carry`" + ` helper
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
`
