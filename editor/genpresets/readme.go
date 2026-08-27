package main

// readme is written next to the preset. It is the seed of the future
// skill document: everything an agent needs to customize the preset
// without breaking it.
const readme = `# chibi-male preset

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
- idle-turn / walk-turn / run-turn — the character actually rotates:
  head and torso step through their side drawings (head-side shows the
  back of the head with one eye, body-side is the thinned torso), the
  limb chains trade sides at the midpoint, and the views cut on hold
  keyframes — no morphing. The clip ends facing the other way, so the
  game flips its Mirrored flag when the turn completes; the mirrored
  idle then matches the end pose.
- run-to-idle — braking stop.
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
