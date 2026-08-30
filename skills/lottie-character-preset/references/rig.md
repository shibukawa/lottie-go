# The rig contract

Every preset character is a cutout puppet: flat part images moved by
transforms. This file is the contract a design swap must honor. Honor it
and every clip keeps working with zero keyframe edits; break a size or an
anchor and limbs detach or orbit the wrong point.

## chibi-male slots

Three-quarter view, leading right (+x). Limbs are two-segment chains
parented through the body: `upper-arm -> forearm` (elbow), `thigh ->
shin` (knee). `near`/`far` mean depth (viewer side / other side), not
left/right — the runtime mirror swaps which body side is near. Facing
right, the near limbs are the character's left side and attach on the
trailing (-x) edge; the far limbs attach on the leading (+x) edge and
peek out from behind the torso. Swapping that placement makes the pose
read as a back view under a forward-looking face.

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

- **size** — the image's pixel dimensions. The clip JSON declares these
  in its `assets` array (`w`/`h`); a replacement image must match or the
  layer scales it.
- **anchor** — the pixel the part rotates around: the neck for the head,
  the shoulder/elbow/hip/knee for limb segments, the hip for the body.
  Draw the new art with the joint at this exact pixel.
- **attach** — where the anchor sits in the parent's image space (canvas
  space for body and shadow). Keyframes move parts relative to these, so
  they are part of the motion data, not the art.

An attach point is a *rest* position, not a rivet: a layer's position is
an ordinary animatable property, so a joint can travel within a clip.
The shoulders are the ones worth moving. A shoulder girdle is not rigid,
and once an arm is straight there is nothing left for rotation to give —
driving the leading shoulder forward is the only thing that still adds
reach to a strike. Send the grip forward by the same distance when you
do, or the arm simply folds back up and you have gained nothing. Keep
this off the *chain children* (forearms, shins): they follow their
parent, and moving their attach detaches the limb at the joint.

Canvas 256x256, ground line y=236, 60fps. File names are
`chibi-male-<slot>.png` in `parts/` (any format Go's image decoders and
a blank-imported WebP decoder handle: PNG and WebP are both fine).

## Drawing rules

- **Pose the art neutrally.** Limb segments hang straight down from
  their joint; the shoe toe points +x. All motion comes from keyframes —
  art drawn "mid-pose" double-applies once animated.
- **Far side reads darker.** The preset multiplies far-part colors by
  ~0.72. Keep some such cue or the two arm chains become unreadable in
  motion.
- **Three-quarter, not profile.** Both eyes visible, torso front showing.
  The head leads +x like everything else.
- **Rotation is shown by view swaps, not mirroring.** head-side is the
  rear-quarter head (back of the head, one eye at the leading edge) and
  body-side the thinned torso — turn clips cut through them; body-back
  (the darker rear torso) is what the spin attacks show, under the
  one-eyed head-side — the eyes keep watching the opponent — while
  head-back is the fully faceless head for motions that need it. A restyled character needs all of them drawn, same
  sizes and anchors as their front versions.
- **Outline everything.** A 1px dark outline is what keeps flat-color
  parts readable against any game background; keep an equivalent edge
  treatment in restyled art.

## Authoring part images without an image tool

The preset's own art is authored as string art — one character per
pixel, scaled up — which any agent can write and any diff can review.
The pattern (from the generator, `cmd/lottie-state-editor/genpresets/parts.go`):

```go
art := []string{ // '.' = transparent, letters = palette entries
	".OOO.",
	"OAAAO",
	"OAAAO",
	".OOO.",
}
img := image.NewNRGBA(image.Rect(0, 0, w*3, len(art)*3)) // 3 = pixel scale
// for each art cell, fill a 3x3 block with palette[ch]
```

Recoloring existing parts is simpler still: decode the PNG, map old
palette colors to new ones pixel-by-pixel, re-encode. This preserves
sizes, anchors, and outlines by construction, which makes it the lowest
risk design swap of all — prefer it when the request is "same character,
different colors/team/rarity".

## chibi-sword: the armed variant

`chibi-sword` is the same fifteen slots plus one, and is the worked
example to copy when a character carries anything:

| slot  | size  | anchor  | parent      | attach |
|-------|-------|---------|-------------|--------|
| sword | 21x78 | (10,10) | forearm-far | (7,26) |

Its decisions generalize to any held prop:

- **Parent it to the far (leading) forearm.** Strikes lead with the
  far-side limb, so a weapon there reaches the enemy in front, and it
  swings with the arm for free — no separate weapon keyframes.
- **Draw it over the body, and then make that true.** Depth-correct
  order puts the far chain behind the torso, which swallows a blade in
  the middle of a cut, so the layer goes near the front (just under the
  near hand, so the grip still reads). That only holds up if the prop is
  in front of the body in *every* clip: chibi-sword's first version let
  the far arm swing freely while carrying, and the blade floated over a
  torso its own hand was behind. Either keep both hands on it
  throughout, or move the layer back into the far chain for the clips
  where the arm swings — and decide that per clip, never per pose, since
  a hold-keyed depth flip mid-swing pops on screen.
- **Bring the whole gripping limb's depth with it.** A far-side arm that
  reaches across the front of the body for a two-handed grip has to be
  drawn ahead of the torso, or its forearm disappears behind the body
  and the character reads as one-handed with a stump for the other arm.
  Move the upper arm too — moving only the forearm relocates the seam to
  the shoulder rather than closing it. Keep the segments' order among
  themselves: on the far side the upper arm draws over its own forearm,
  the reverse of the near chain.
- **Draw it hanging straight down from the grip and symmetric**, at full
  brightness. Symmetric means a turn (which mirrors head and shoes)
  needs no mirrored copy; full brightness keeps it from reading as
  background even though its arm chain is dimmed.

The clips it inherits from chibi-male were not re-posed: the blade angle
is derived per keyframe to hold a fixed angle to the *torso*, so the
sword tips with the body but does not flail with every arm swing. Note
that a blade's on-screen angle is `body lean + shoulder + elbow + blade`
— a clip with a deep lean needs that compensated or the tip points at
the floor, and a long weapon needs its own carry angle in clips whose
torso is already near horizontal (death, slide).

**Two-handed poses are geometrically constrained.** The shoulders sit
42px apart and an arm reaches 41px, so the two hands can never meet out
at arm's length: a two-handed pose must keep them near the body's
centerline and let the weapon's length do the reaching — which is how a
two-handed sword is really held, and why the thrust travels with the
body instead of extending the arms. Solve the second arm rather than
eyeballing it (the generator fits it as a two-link chain onto the
handle) and make an out-of-reach pose an error, or you get a hand
grasping at air beside the hilt.

## Adding or removing slots

Slots can grow (a weapon, a cape) but that is a motion-side change too:
the new layer needs an asset entry, a layer with parent + anchor + attach
in *every* clip, and keyframes wherever it should move. Follow how an
existing limb segment is declared across the dump, add yours everywhere
consistently, and give it a fixed spec here-style (size/anchor/attach) so
later swaps can honor it.
