---
name: lottie-character-forge
description: Make a brand-new lottie-go game character from a text description, with an image model (Gemini, Grok, or any image tool) drawing the art. Use this whenever the user wants a character that does not exist yet — "make me a fox-eared shrine maiden", "a robot enemy for my game", "turn this concept into a playable character" — or wants generated/AI art turned into an animated .lottie rig with UV vertex morphing. It writes the image prompts (with the part-split layout the rig needs), turns the returned sheets into parts, builds a textured vector rig that inherits every clip of a preset, adds morph motion, and verifies by rendering. For editing an EXISTING preset character (recolor, retime, a new clip on the same art) use lottie-character-preset instead.
---

# lottie-go character forge

The forge turns a description into a game-ready `.lottie`: the image
model draws, the tools measure and rig, the agent steers. The result is a
**UV-morph rig** — every body part is a closed vector path with the
generated picture painted through it, one UV per vertex (lottie-go's
texture extension), parented and keyframed exactly like the preset
character it was built from. So the new character walks, runs, attacks,
turns and dies on day one, at the image model's resolution, and its parts
can bend, breathe and squash on top of that.

Status: the workflow, formats and prompts here are the design; the CLI
`lottieforge` they name is specified in
[references/morph-rig.md](references/morph-rig.md) and
[references/sheets.md](references/sheets.md) but not yet in the
repository. Until it lands, each step says what it must produce, and an
agent can script it (Go against the lottie-go module, or Python with
Pillow for the image steps) from those specifications. Two shipped tools
also need small changes before the loop closes for textured bundles:
`lottierepack` must round-trip `extensions/`, and `lottiecheck -render`
must apply the texture documents. Say so to the user if they are not done.

## What you produce

```
work/
  character.json        the spec you write (references/sheets.md has the schema)
  sheets.json           generated: which cell of which sheet is which part
  prompts/*.md          generated: filled prompts the user pastes into the image model
  sheets/*.template.png generated: grid templates attached to the part prompts
  sheets/*.png          what came back from the image model (the user drops these in)
  parts/<slot>.png      cut parts at sheet resolution, plus derived far-side parts
  report.json           cut's verdict per cell
  <clip>-anim.json, parts/, machines/, extensions/   the lottierepack layout of the result
  <name>.lottie         the bundle
  preview/              rendered contact sheets
```

## Workflow

1. **Spec.** Write `work/character.json`. Pick the base preset:
   `chibi-sword` when the character holds anything, else `chibi-male`.
   Fill description, style, proportions and props. Defaults cover
   everything else. Do not ask the user what the spec can default.
2. **Prompts.** Generate `sheets.json`, the template images and the
   filled prompts (`lottieforge grid`, or by hand from
   [references/prompts.md](references/prompts.md) and
   [references/sheets.md](references/sheets.md)). Tell the user exactly
   which prompt to run with which attachments and where to save each
   result: `work/sheets/model.png`, then `heads.png`, `torsos.png`,
   `limbs.png`. If your own stack has an image tool, run them yourself.
3. **Model sheet first.** Look at `model.png` before asking for parts:
   three-quarter view, facing right, arms hanging, feet pointing right,
   flat key-color background, nothing cropped, no shadow. Wrong: issue
   the fix-up prompt, not a new design. This image is the reference
   every part prompt attaches, so its correctness is cheap here and
   expensive later.
4. **Inventory.** From `model.png`, list what the base rig cannot
   hold — what hangs (ribbons, a ponytail, a tail), what covers a limb
   (skirt, cape, wide sleeves), what sticks out (ears, a hat) — and add
   each to the spec as an attachment with a kind, per
   [references/attachments.md](references/attachments.md). Ornaments
   that ride a part (a belt, an emblem, a fringe) are painted into that
   part and get nothing. Re-run `grid`: the host cells now say
   "without its …", and an attachments sheet joins the set.
5. **Part sheets.** Run the part prompts (three, plus the attachments
   sheet when there is one), each with `model.png`
   and its template attached. Look at each result once: parts inside
   their cells, joint caps present, limbs vertical, nothing drawn on the
   borders.
6. **Cut.** `lottieforge cut work` → `parts/`, `report.json`,
   `contact.png`. Read the report. A cell that is `empty`, `border`,
   `multi` or `halo` goes back to the image model with the single-cell
   fix-up prompt. Do not accept a flagged cell "because it looks fine":
   the flags predict a detached limb or a halo in motion.
7. **Rig.** `lottieforge rig work -o work/<name>.lottie`. It traces each
   part, fits it into the template's joint geometry, rewrites every clip
   of the base preset to shape layers with vertex-mapped textures, adds
   each attachment's layer to every clip at its kind's depth, and
   writes `faithful.png` — the morph rig against a raster rig from the
   same parts at idle frame 0. It refuses a part whose contour the
   renderer cannot texture even after decomposing it, and names it.
8. **Check.** `lottiecheck -render work/preview work/<name>.lottie`, and
   read the PNGs: idle, the walk's contact pose, an attack's strike
   frame, a turn's midpoint. A limb floating off its joint is a fit
   problem (set `parts.<slot>.fit`, or redraw the cell with proper
   caps); an untextured wedge is a contour problem; a color silhouette
   instead of art means the texture document did not resolve (report
   the tool gap above).
9. **Motion.** Bake the spec's morph list (`lottieforge morph work`):
   breathing on idle, bend at the elbows and knees, squash on landing —
   and the attachments' own motion, which their kinds imply: swings
   from the pivot's real path, drapes from the driver limbs' angles,
   locks as both. Add character-specific clips by copying the
   nearest template clip and reshaping it — the recipes in
   [references/motion.md](references/motion.md) and in
   lottie-character-preset's clips.md. Wire states. Check again.
10. **Review.** If a human wants to look: the editor's viewer mode
   follows the bundle on disk; its `-mcp` mode lets you and the human
   share one selection while you fix a vertex or a UV point together.
11. **Deliver** the bundle. Keep `work/` — the next variant (recolor,
    outfit, another weapon) starts from it and skips most of the above.

## Rules that keep the result playable

- **The clip JSON stays plain Lottie.** Textures live in
  `extensions/texture/<clip>.json`; never invent members inside a clip.
  A player without the extension draws each part as a flat silhouette in
  its mean color, which is the intended fallback.
- **Vertex count and order are frozen at rig time.** Morphs and hand
  edits move vertices; they never insert or delete them, because UV is
  per vertex and every key of a path must agree.
- **Contours are star-shaped about their centroid, or decomposed.** The
  renderer fans each contour from its centroid; the rig step cuts a
  drawing with lobes into star-shaped sub-paths that share its texture,
  seams welded. Split into slots only for motion: a lock that swings is
  a slot, a lobe that rides with the head is a sub-path.
- **Hosts are drawn without their attachments; attachments complete,
  with a root overlap.** A torso without its skirt, a head without its
  ponytail; the skirt from waistband to hem, the ponytail from scalp to
  tip. Ornaments that move with a part are painted into it.
- **Joint lengths are the contract, not canvas sizes.** A part image can
  be any size; what must match the template is where its joints sit
  relative to each other. The fit does that from the caps, which is why
  the prompts insist on rounded caps hanging vertically.
- **Draw every slot once.** Far-side limbs are derived darker, the
  shadow is inherited, the turn views (rear-quarter and back) are the
  only extra drawings, and only for head and torso.
- **Never commit model output to the lottie-go repository.** A user's
  game project owns its art; this repository's samples are generated.
- **Verify by rendering, every time.** "It repacked" is not a result.

## When something is wrong

- lottiecheck fails → it names the clip and the feature or the missing
  asset; a forged bundle uses nothing exotic, so this is almost always a
  hand edit made after rig.
- Parts are drawn as one figure → the exploded-view wording plus the
  attached template usually fixes it on the second try; otherwise ask for
  the model sheet with visible gaps between every limb and cut it by
  connected components (references/sheets.md fallback).
- Style differs between sheets → attach `model.png` again and add "same
  character, same rendering, same outline weight"; regenerate only the
  drifting sheet.
- A skirt or sleeve reads as beside the leg or arm rather than over it
  → the drape has no driver, or its weight is too low; the default
  drivers are the host's chain children — name them, then re-bake.
- Hair or a ribbon keeps swinging in idle → damping too high for the
  clip; lower it, or set the swing to settle (`damping` 0.7) and let the
  loop cross-fade take the rest.
- A limb detaches only in deep poses (the kick's chamber, the slash's
  windup) → the segment's cap on the child end is short; raise `fit`
  slightly or redraw with a fuller cap. The bend morph hides small gaps
  but not missing art.
- The texture shows through the wrong part of the image (a hand on the
  shoulder) → the cap detection picked the wrong end; the part was drawn
  upside down or diagonal — redraw the cell vertical, joint at the top.
