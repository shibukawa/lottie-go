# Image model prompts

The prompts an agent fills from the spec and hands to the user (or to
its own image tool). Placeholders are `{{...}}`. Keep the prompts in
English whatever language the user writes in — image models follow it
best — and keep the rules verbatim; every line exists because a model
broke it. `lottieforge grid` writes these filled into `work/prompts/`.

Common fields:

| placeholder     | from the spec                                   | example                                        |
|-----------------|-------------------------------------------------|------------------------------------------------|
| `{{description}}` | description                                   | a fox-eared shrine maiden with a red hakama, white kimono top and a naginata |
| `{{style}}`     | style                                           | clean anime cel shading, thick dark outline, flat colors, no gradients |
| `{{proportions}}` | proportions (defaults from the base)          | chibi proportions, about 2.5 heads tall, big head, short limbs |
| `{{key}}` / `{{key_name}}` | key                                  | `#FF00FF` / magenta                            |
| `{{cell_list}}` | sheets.json cells of one sheet                  | see P2                                         |
| `{{without}}` / `{{worn}}` | attachments of a host cell           | "WITHOUT the hakama and the cape" / "with its ponytail and side locks as worn" |

Pick the key color away from the palette. Magenta suits most designs; a
pink or purple character takes `#00FF00` (green) instead.

## P1 — model sheet (text to image)

The reference every later prompt attaches. One character, one pose.

```
Character design reference for a 2D game sprite, to be cut into animation parts.

Subject: {{description}}.
Style: {{style}}. {{proportions}}.

Pose: standing still, relaxed A-pose. Arms hang straight down, slightly away from the body so they do not touch the torso. Legs straight and slightly apart, both feet flat on the same line, toes pointing to the RIGHT. Head level, looking straight ahead.
View: three-quarter view facing RIGHT — both eyes visible, the front of the torso visible, the character's own right side is the far side.
Background: one flat, uniform {{key_name}} ({{key}}) color filling the whole image. No gradient, no floor line, no cast shadow, no drop shadow, no vignette, no frame, no border.
Rendering: crisp edges with no glow, no outer blur, no soft shadow around the character, no motion lines, no sparkles, no text, no labels, no watermark, no extra objects or props other than the ones described.
Layout: the character centered, about 85% of the image height, nothing cropped.
```

Check before going on: view, facing, feet, background, cropping, and
that the design has all the parts the spec names (a prop in the hand
counts — it is drawn separately later, but seeing it here locks its
look).

### P1b — turnaround (only if the design has a distinctive back)

The rig's turn clips show the rear-quarter and back of head and torso.
The part prompts ask for those views; this optional sheet pins them
down first when the back of the design matters (a cape, a tail, a
pattern).

```
Same character as the attached reference, same style and colors, same A-pose.
Three views side by side, same scale, on one flat {{key_name}} ({{key}}) background: (1) front three-quarter view facing right, (2) rear three-quarter view facing right — mostly the back of the head and torso, one eye just visible at the leading edge, (3) full back view.
No shadow, no text, no labels, nothing else.
```

## P2 — part sheet (reference image + template image + text)

One prompt per sheet: `heads`, `torsos`, `limbs` (props ride on the
limbs sheet). Attach `model.png` and the sheet's template PNG. The cell
list comes from `sheets.json`, one line per cell, in reading order.

```
Attached: (1) the character reference, (2) a grid template.

Task: redraw the character from (1) as SEPARATE BODY PARTS, one part per cell of the grid in (2), like an exploded cutout-puppet sheet for 2D animation.
Keep the template exactly as it is — its flat {{key_name}} ({{key}}) background, the black cell borders and the label text under each cell. Draw nothing on the borders or in the label bands. Draw each part entirely inside its own cell, with clear space between the part and the cell border.

Cells, row by row:
{{cell_list}}

Rules for every part:
- Same character, same colors, same outline weight, same rendering as the reference. Do not restyle.
- Draw each part COMPLETE, including what other parts normally cover: the whole upper arm even where the torso would overlap it, the whole neck stump under the head, the whole thigh up to the hip.
- Limb segments hang STRAIGHT DOWN, vertical, with the joint at the TOP center of the cell. Upper arm = shoulder to elbow. Forearm = elbow to fingertips, hand included. Thigh = hip to knee. Shin = knee to toes, foot included, foot pointing RIGHT.
- Every limb segment ends in a ROUNDED CAP that extends a little past the joint at BOTH ends, so neighbouring segments overlap when a joint bends. No flat cut-off ends.
- The torso is drawn from the neck stump at the top to the hips at the bottom, hip at the bottom center of the cell, without arms and without legs.
- The head is drawn with all of its hair and a short neck stump, neck at the bottom center of the cell.
- A cell that says WITHOUT something: draw that part complete as if the item were not worn (the hips and belt line under a skirt, the scalp under a ponytail); the item has its own cell. A cell that says AS WORN: include those items in that view.
- Hanging items (hair locks, ribbons, tails, skirts, sleeves, capes) are drawn complete, hanging STRAIGHT DOWN, their root or waistband at the TOP center of the cell, slightly wider at the root than where they attach so they overlap the part they hang from.
- Each part fills its cell from top to bottom; the cells are already sized so the parts keep the reference's proportions — do not enlarge a small part to fill a wide cell.
- Flat {{key_name}} background inside every cell. No shadow, no glow, no soft edges, no text, no labels inside the cells, no arrows, no numbers, no extra parts.
```

Example cell list for the chibi `heads` sheet:

```
A1 head — the whole head, front three-quarter view facing right, both eyes visible, neck stump at the bottom center
A2 head-side — the same head from the rear three-quarter view: mostly the back of the head, one eye just visible at the leading (right) edge, neck at the bottom center
A3 head-back — the same head from directly behind, no face visible, neck at the bottom center
```

With attachments in the spec the head lines change to, e.g.:

```
A1 head — the whole head WITHOUT the ponytail and the side locks (they have their own cells); front three-quarter view facing right, both eyes visible, the scalp complete where the locks attach, neck stump at the bottom center
A2 head-side — the same head from the rear three-quarter view, AS WORN with its ponytail and side locks seen from that angle, one eye just visible at the leading (right) edge, neck at the bottom center
```

and for `limbs`:

```
A1 upper-arm — one upper arm, shoulder to elbow, hanging straight down, shoulder cap at the top center, elbow cap at the bottom
A2 forearm — one forearm with the hand, elbow to fingertips, hanging straight down, elbow cap at the top center, hand relaxed and open at the bottom
A3 thigh — one thigh, hip to knee, hanging straight down, hip cap at the top center, knee cap at the bottom
A4 shin — one lower leg with the foot, knee to toes, knee cap at the top center, the foot flat at the bottom pointing RIGHT
B1 sword — the naginata alone, drawn vertical, grip at the top center of the cell, blade pointing down, full brightness, symmetric so it reads the same mirrored
```

Near and far sides are never asked for: the far copy is derived darker
by the tool, exactly as the presets do.

An `attachments` sheet lists one cell per attachment, worded by kind:

```
A1 ponytail — the ponytail alone, from the hair tie at the top center of the cell to the tip, hanging straight down, the root drawn slightly wider so it overlaps the scalp
A2 side-lock — one side lock of hair, root at the top center, hanging straight down
A3 hakama — the whole hakama from the waistband to the hem, hanging straight and relaxed, waistband at the top center and a little wider than the torso's hips, no legs visible
A4 sleeve — one wide kimono sleeve alone, shoulder opening at the top center, hanging straight down, no arm inside
B1 fox-ears — both fox ears as one piece as worn on top of the head, upright, the base of the ears at the bottom center
B2 cape — the whole cape from the collar at the top center to the hem, hanging straight, seen from the front three-quarter view
```

Rigid items are drawn upright as worn; everything else hangs from the
top center. Paired items (sleeves, side locks, earrings) are drawn once.

### P2-text — the same without a template image

For a model that ignores attached grids (Grok often does): describe the
layout instead, and let `cut` fall back to connected components.

```
Attached: the character reference.
Redraw the character as SEPARATE BODY PARTS on one flat {{key_name}} ({{key}}) background, arranged in a grid of {{rows}} rows, left to right, top to bottom, with wide empty space between parts so that no two parts touch:
{{cell_list_as_rows}}
(the rules block of P2, unchanged)
```

## P3 — fix one cell

Regenerating a whole sheet drifts the other cells. Fix the one that
failed, with the failed sheet attached as the image to edit.

```
Edit the attached sheet. Change ONLY cell {{cell}} ({{slot}}): {{reason}}. Redraw that part {{instruction}}, entirely inside its cell with space to the border, on the flat {{key_name}} background. Leave every other cell, the borders and the labels exactly as they are.
```

Reasons the report produces map to instructions:

| report status | `{{reason}}`                                | `{{instruction}}`                                            |
|---------------|---------------------------------------------|--------------------------------------------------------------|
| empty         | the cell is empty                            | as listed for that cell                                      |
| border        | the drawing touches or crosses the cell border | smaller, centered, with clear space all round               |
| multi         | it is drawn as several pieces                | as ONE connected piece (hand attached to the forearm, no separate details) |
| halo          | it has a soft shadow or glow around it       | with crisp edges and no shadow or glow                       |
| (fit)         | the joint end is cut flat / the part is diagonal | hanging vertically with rounded caps past both joints     |

## P4 — a prop or an extra slot on its own

When a prop was added to the spec after the sheets exist, or a slot
needs a second try at higher resolution.

```
Same character and style as the attached reference. Draw only {{part}}: {{cell_instruction}}. One flat {{key_name}} ({{key}}) background, the object centered and filling about 80% of the image height, crisp edges, no shadow, no text.
```

## Model notes

- **Gemini** (image editing with references): keeps attached grids and
  labels well; P2 with both attachments is the default path. When it
  restyles, lead the prompt with "Use the attached reference for the
  exact design" and repeat the style line.
- **Grok**: excellent P1 results; for P2 prefer P2-text and expect to
  confirm slot order from `contact.png`. It tends to add a ground shadow
  — the "no cast shadow" line matters, and `halo` in the report is the
  usual consequence.
- **Any model**: transparency requests produce fake checkerboards; ask
  for the flat key color, never "transparent background". Text inside
  cells gets baked into the art; that is why labels sit in a band under
  each cell. A model that draws all parts the same size ignores the cell
  aspect — the fit still works, but a torso drawn as wide as it is tall
  will look squat; redo the cell rather than accept it.
