# Attachments: hair, cloth, ornaments

The base rigs have fifteen slots and a weapon. A real design has more:
a ponytail, a skirt, a cape, wide sleeves, a ribbon, a tail, ears, a
hat. Each of those is an **attachment** — its own cell on the sheet, its
own textured layer in every clip, hosted by a rig part, and moved by a
generator its **kind** selects. You name the thing and its kind; the
kind fills in the parent, the draw order, the cell shape, the prompt
wording and the motion.

## First decide: paint it in, or attach it?

An ornament that never moves apart from the part it sits on is painted
into that part. An emblem on the chest, a belt, armor plates, a collar,
a fringe glued to the forehead, boots, gloves: all part of the torso,
head, shin or forearm drawing. Attaching them buys nothing and costs a
textured style and a cell each.

Attach only what:

| it does                                   | kind    | examples                                              |
|-------------------------------------------|---------|-------------------------------------------------------|
| hangs and would swing when the body moves | `swing` | earrings, ribbon tails, tassels, a scarf end, a tail, a chain, a ponytail |
| covers a limb that bends underneath       | `drape` | skirt, hakama, cape, coat tails, wide sleeves, a mane |
| is hair that should move                  | `lock`  | side locks, a lifting fringe, back hair, a braid       |
| sticks out past the silhouette but is stiff | `rigid` | hat, glasses, shoulder pads, ears, horns, folded wings |

Six attachments is plenty. Two to four hair locks; the rest of the hair
belongs to the head drawing.

## The inventory step

After `model.png` is right and before the part prompts, look at it and
write the list. For each item: name, kind, host (the rig slot it
attaches to), where on the host it attaches (in the host's slot
coordinates — the rig table in lottie-character-preset's rig.md gives
the sizes), and a nominal size in the same units, by eye. Paired
things (two sleeves, two earrings) are drawn once: `paired: true`
derives the far copy darker, like the limbs.

```jsonc
"attachments": [
  { "name": "ponytail",   "kind": "lock",  "host": "head", "attach": [44, 12], "size": [26, 60], "order": "behind-head" },
  { "name": "side-lock",  "kind": "lock",  "host": "head", "attach": [12, 30], "size": [12, 40], "paired": true },
  { "name": "hakama",     "kind": "drape", "host": "body", "pivot": "waist", "size": [60, 40],
    "drivers": ["thigh-near", "thigh-far"], "panels": 2, "weight": 0.6 },
  { "name": "sleeve",     "kind": "drape", "host": "upper-arm", "size": [24, 34], "drivers": ["forearm"], "paired": true, "weight": 0.5 },
  { "name": "cape",       "kind": "drape", "host": "body", "size": [56, 70], "order": "behind-body", "views": "separate" },
  { "name": "ribbon-tail","kind": "swing", "host": "head", "attach": [40, 4], "size": [10, 36], "segments": 2, "damping": 0.85 },
  { "name": "fox-ears",   "kind": "rigid", "host": "head", "attach": [36, 2], "anchor": [20, 30], "size": [40, 30], "sway": { "amount": 4, "period": 96 } },
  { "name": "tail",       "kind": "swing", "host": "body", "attach": [10, 44], "size": [30, 50], "order": "behind-body", "segments": 2 }
]
```

## What each kind does

**rigid** — parented to the host, rides its transform, no keys of its
own. `anchor` says where its pivot is inside its own drawing (an ear's
base, a hat's brim center). Optional `sway` puts a slow sine on its
rotation: antennae, feathers, ears that twitch.

**swing** — the layer's rotation is a damped pendulum, baked per clip
from the pivot's real world motion (the tool reads the host's placement
per frame with the library's own `LayerPlacement`, parents included).
A run makes a ponytail stream back; a landing makes it whip; idle
leaves it still. `segments: 2` makes two layers, the second hanging
from the first like a forearm, so a scarf or a tail curls. Loops are
closed by cross-fading the last frames into the first. `damping` (0.8
default; higher swings longer) and `stiffness` are the tuning.

**drape** — a vertex morph. Each vertex rotates about the pivot by a
weighted share of a driver's angle: a skirt's near half follows the
near thigh, its far half the far thigh, the hem more than the waist. It
bakes straight from the clip's rotation keys, so the walk's thigh swing
folds the skirt without a physics pass. `panels: 2` splits one skirt
drawing down its vertical midline into front and back layers with the
legs between them — for a long skirt where legs should show. A cape
drapes from body lean and velocity; a sleeve from its forearm.

**lock** — a swing on the layer plus a follow on the vertices, so the
root moves with the head and the tip lags. Front locks draw in front of
the head; back hair behind head and body.

## Draw order and turns

Order is declared relative to a slot: `in-front-of-head`,
`behind-body`, `in-front-of-thigh-near`. Defaults by kind: rigid and
front locks in front of the host; swing in front of the host, tails
and ponytails behind; skirts in front of both thighs (the legs emerge
under the hem) or two panels; capes behind the body. New layers get a
fresh `ind` above the clip's highest and are inserted into the layer
array at the right depth; nothing existing is renumbered.

Turn clips swap the head and torso to their rear-quarter and back
drawings. By default (`views: "baked"`) those drawings are made *with*
the attachments as worn — the prompt for the head-side cell says "with
its ponytail and side locks as seen from behind" — and the attachment
layers copy the host front layer's opacity holds, so they vanish
exactly when the front drawing does. That is right for hair, ribbons
and skirts. A large cape or coat gets `views: "separate"`: its own side
and back cells and layers that follow the matching views instead.

## Prompts

Two wordings change when attachments exist, and `grid` inserts both:

- the host cell says **without** them: "torso — from the neck stump to
  the hips, WITHOUT the hakama and the cape, which have their own cells;
  draw the hips and belt line complete as if the skirt were not there";
- the attachment cell says **complete, with a root overlap**: "hakama —
  the whole hakama from the waistband to the hem, hanging straight,
  waistband at the top center, drawn slightly wider at the top than the
  torso's hips so it overlaps them";

and the side and back cells of a baked host say "as worn, with its
ponytail and side locks, seen from that view".

Hanging kinds are drawn vertical with the root at the top center, like
limbs. Rigid kinds are drawn upright as worn, and the spec's `anchor`
names the pivot.

## Shapes with lobes

A hair mass with lobes or a pleated hem is not star-shaped, and the
renderer fans each contour from its centroid. The tool does not ask you
to split such a drawing into slots: it decomposes the contour into a
few star-shaped sub-paths sharing the same texture and fill, and welds
the vertices they share so a morph moves both sides of a seam. Split
into separate attachments only for *motion* — a lock that swings on its
own is a slot; a lobe that rides with the head is a sub-path.

## What to look at

- **walk** at the contact pose: the skirt's hem leads the forward thigh
  and trails the back one; legs read as under the cloth, not beside it.
- **run** mid-cycle: ponytail and cape stream back; nothing swings
  forward of the head.
- **a turn** at the midpoint: attachments disappear with the front head
  and the side drawing shows them as worn — no doubled hair.
- **jump** at launch and landing: swings whip once and settle; no
  oscillation still going at idle.
- **death**: hanging things hang toward the ground, which is now
  sideways to the body — the pendulum works in world space, so check
  it did.
