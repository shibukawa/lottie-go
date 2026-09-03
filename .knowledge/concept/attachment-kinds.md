---
id: concept:attachment-kinds
type: concept
title: Attachment Kinds (Hair, Cloth, Ornaments)
---

Everything a character wears or grows that the base rig
(requirement:animation-presets) has no slot for — hair locks, skirts,
capes, sleeves, ribbons, tails, hats, badges — is an *attachment*: a
layer added to every clip, hosted by a rig part, drawn from its own
sheet cell, and moved by a generator chosen by its kind. Four kinds
cover what an image model draws on a character; the kind decides the
default parent, draw order, cell shape, prompt wording and motion, so
the spec names the thing and the kind and the rest defaults
(data:character-forge-spec attachments). Built by cmd/lottieforge for
requirement:ai-character-forge on a concept:uv-morph-rig.

```yaml
principle:
  bake_or_attach: an ornament that never moves apart from the part it sits on (an emblem, a belt, armor plates, a collar, bangs glued to the forehead) is painted into that part — not an attachment. Only what hangs, drapes, sticks out past the silhouette, or must move on its own gets a slot
  host_complete: the host is drawn complete without the attachment (a torso without its skirt, a head without its ponytail); the attachment is drawn complete with a root that overlaps the host — the cutout overlap rule applied to worn things
  every_clip: an attachment layer exists in every clip of the bundle, keys or not, as the preset skill requires of any added slot
kinds:
  rigid:
    what: hat, glasses, shoulder pad, wings at rest, a badge that hangs past the edge
    motion: none of its own — it rides the host's transform; optional sway (a sine on rotation) for antennae and feathers
    joint: authored anchor on the item, attach on the host; both from the spec, defaults at the item's top center onto the host's top
    order: in front of the host
  swing:
    what: earrings, ribbon tails, tassels, a ponytail, scarf ends, a tail, a chain
    motion: a damped pendulum on the layer's rotation, driven by the pivot's world motion — position and angle per frame from api:layer-placement — baked to rotation keys; loops closed by blending the last frames into the first
    joint: pivot at the top center (the root); attach at the host point it hangs from
    order: in front of the host by default; behind for a tail or a ponytail (declared)
    chain: a long item may be two segments (swing-2), the second parented to the first like a forearm, so a scarf or a tail curls
  drape:
    what: skirt, hakama, cape, coat tails, wide sleeves, a long mane
    motion: vertex morph — each vertex rotates about the pivot by a weighted share of a driver's angle (a skirt follows the thighs, near vertices the near thigh, far vertices the far one; a sleeve follows its forearm; a cape follows body lean and velocity), so the cloth folds around the limb that moves under it
    joint: pivot at the waist, shoulder or wrist; drivers are rig layers named in the spec, defaults by host (body -> thighs, upper-arm -> its forearm)
    order: a skirt in front of both thighs — the legs emerge below the hem — or two panels (front and back) with the legs between when the skirt is long; a cape behind the body
  lock:
    what: hair that moves — side locks, a fringe that lifts, back hair, a braid
    motion: swing on the layer plus follow on the vertices (the tip lags the root); a lock is a swing with a soft body
    joint: root at the scalp; attach on the head
    order: front locks in front of the head, back hair behind the head and body; one lock per moving mass, two to four per character, the rest painted into the head
views:  # what a head-side, head-back, body-side or body-back drawing shows
  baked: default — the view cells are drawn with the attachments as worn, and the attachment layers take the host front layer's opacity track (its hold keys), so a turn hides them exactly when the front drawing hides. Turn clips are short; the whole-view picture carries the cloth through them
  separate: an attachment worth its own side and back cells (a large cape, a long coat) gets them, and its own layers follow the matching view layers instead
  near_far: a paired item (two shoulder pads, two earrings, two sleeves) is drawn once and derived darker for the far side, like the limbs
shape:
  decomposition: a contour the fan cannot texture (a hair mass with lobes, a pleated hem) is split by the tool into star-shaped sub-paths that share the texture and the fill — same slot, same drawing — with the shared vertices welded so a morph moves both; slots split only for motion, never for shape
inventory:  # how the agent decides what is an attachment, from the model sheet
  hangs: anything below its root that would swing when the character moves -> swing or lock
  covers: cloth over a limb that bends under it -> drape
  protrudes: past the body silhouette and rigid -> rigid
  flat: on a part, moves with it -> bake into the part
  limit: six attachments is plenty; every one is a textured style and a cell
```

The same taxonomy serves a hand-drawn character: a user who has the
parts already fills the spec and skips the prompts.
