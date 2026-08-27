# The rig contract

Every preset character is a cutout puppet: flat part images moved by
transforms. This file is the contract a design swap must honor. Honor it
and every clip keeps working with zero keyframe edits; break a size or an
anchor and limbs detach or orbit the wrong point.

## chibi-male slots

Three-quarter view, leading right (+x). Limbs are two-segment chains
parented through the body: `upper-arm -> forearm` (elbow), `thigh ->
shin` (knee). `near`/`far` mean depth (viewer side / other side), not
left/right — the runtime mirror swaps which body side is near.

| slot           | size  | anchor  | parent         | attach    |
|----------------|-------|---------|----------------|-----------|
| head           | 72x60 | (36,56) | body           | (24,3)    |
| body           | 48x48 | (24,45) | -              | (128,179) |
| upper-arm-near | 15x27 | (7,4)   | body           | (45,8)    |
| forearm-near   | 15x30 | (7,4)   | upper-arm-near | (7,23)    |
| upper-arm-far  | 15x27 | (7,4)   | body           | (3,8)     |
| forearm-far    | 15x30 | (7,4)   | upper-arm-far  | (7,23)    |
| thigh-near     | 18x33 | (9,4)   | body           | (32,45)   |
| shin-near      | 21x36 | (9,4)   | thigh-near     | (9,29)    |
| thigh-far      | 18x33 | (9,4)   | body           | (16,45)   |
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
- **Outline everything.** A 1px dark outline is what keeps flat-color
  parts readable against any game background; keep an equivalent edge
  treatment in restyled art.

## Authoring part images without an image tool

The preset's own art is authored as string art — one character per
pixel, scaled up — which any agent can write and any diff can review.
The pattern (from the generator, `editor/genpresets/parts.go`):

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

## Adding or removing slots

Slots can grow (a weapon, a cape) but that is a motion-side change too:
the new layer needs an asset entry, a layer with parent + anchor + attach
in *every* clip, and keyframes wherever it should move. Follow how an
existing limb segment is declared across the dump, add yours everywhere
consistently, and give it a fixed spec here-style (size/anchor/attach) so
later swaps can honor it.
