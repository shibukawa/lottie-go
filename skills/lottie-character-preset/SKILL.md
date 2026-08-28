---
name: lottie-character-preset
description: Customize and extend lottie-go character animation presets (the raster cutout rigs under testdata/presets, e.g. chibi-male). Use this whenever the user wants to change a game character's appearance (new outfit, colors, a samurai / knight / robot / enemy variant), tweak or retime a motion (walk, run, attack, jump), add a brand-new animation clip, wire character animations into a game via the state machine, or produce a new character .lottie from a preset — even when they never say "preset" or "Lottie": any character-animation editing request in a project that uses lottie-go belongs to this skill. Also use it to diagnose why a Lottie file fails or renders wrong in lottie-go.
---

# lottie-go character presets

A preset is a complete, game-ready character: one `.lottie` bundle holding
~20 clips (idle/walk/run/jump/attacks/guard/hurt/death), the part images
they share, and a wired state machine a game drives by verbs
(`sm.Fire("jump")`). Presets live in `testdata/presets/` of the lottie-go
repository and are meant to be copied into a game and customized:

- `chibi-male` — 2.5 heads, unarmed, 19 clips. The base rig.
- `chibi-sword` — the same rig with a sword slot, 21 clips: the spinning
  kick swapped for `slash` / `slash-2` / `thrust`. Start here for any
  character that carries something; how the weapon is parented, layered
  and drawn is in [references/rig.md](references/rig.md).

The design that makes automated editing safe: characters are **raster
cutout rigs**. Part images (head, torso, limb segments) are moved by
transform keyframes only. The images and the motion are independent —
swap all images and every clip still plays; retune keyframes and the art
is untouched. Your job is always one side or the other.

## Tools

Two commands ship with lottie-go. Run them from the game module (they
resolve through its `go.mod`) or from a lottie-go checkout:

```bash
# Explode a bundle into editable files / rebuild it after editing
go run github.com/shibukawa/lottie-go/cmd/lottierepack -dump -dir work character.lottie
go run github.com/shibukawa/lottie-go/cmd/lottierepack -dir work -out character.lottie

# Validate + render: exits non-zero and names the problem if anything
# uses a feature lottie-go skips; -render writes sample frames as PNGs
go run github.com/shibukawa/lottie-go/cmd/lottiecheck -render preview/ character.lottie
```

The dump layout: `work/<clip-id>.json` (one animation per file),
`work/parts/*.png` (shared images), `work/machines/<id>.json` (state
machines). Deleting a clip file deletes the clip on repack.

If the lottie-go checkout is available, the user can watch your edits
land live: `cd editor && go run . -viewer path/to/character.lottie`
auto-reloads whenever the bundle changes on disk. Suggest it when the
user wants to follow along; it changes nothing about your workflow —
keep repacking and checking as usual.

**Always close the loop**: after any edit, repack, run lottiecheck with
`-render`, and actually look at the rendered PNGs (they are small; read a
few key frames). "It repacked" is not verification — a wrong anchor or a
broken keyframe decodes fine and looks wrong. The render is drawn by the
real lottie-go renderer, so what you see is what the game gets.

## Workflows

### Change the character's look (design swap)

Replace images in `work/parts/`, change nothing else. The contract per
slot (exact sizes, anchor = the joint, which way things point) is in
[references/rig.md](references/rig.md) — read it before drawing. In
short: same canvas size per part, joint at the same pixel, character
leads +x, far-side parts darker than near-side. If you have an image
generation tool, generate to spec; without one, author pixel art
programmatically (rig.md shows the string-art recipe used by the preset
itself). Recoloring is just image processing on the existing PNGs.

### Tweak a motion (retime / amplify / soften)

Edit the clip's JSON keyframes. Anatomy of a clip, what each track means,
and timing patterns (contact/passing, anticipation, follow-through) are
in [references/clips.md](references/clips.md). Speed changes usually
belong in the state machine (`speed` on a state) or the game
(`SetSpeed`), not in rewritten keyframes — rescale keyframe times only
when one clip alone must change tempo.

### Add a new clip

Copy the closest existing clip file to `work/<new-name>-anim.json`, set
its `nm` to the new name, and reshape its keyframes — every clip shares
the same 11 layers, so the closest clip is always a working skeleton.
Then wire it: add a state (and, if the game triggers it, an event input
and transitions) in `work/machines/<id>.json`. The machine format and the
wiring conventions (one-shots return via `clipDone`, guard-order rules)
are in [references/clips.md](references/clips.md).

### Make a new character from a preset

Copy the preset's `.lottie` (or its directory), dump, then do a design
swap — and optionally rename: clip ids stay as they are (games address
verbs, not files), only the bundle filename needs to change.

### Wire into a game

```go
b, _ := lottie.DecodeBundle(reader, size)
sm, _ := b.NewStateMachinePlayer("")     // default machine
sm.Fire("run"); sm.Set("grounded", true) // verbs and value inputs
sm.Update()                              // once per tick
sm.Draw(screen, nil)
```

Facing: clips are authored facing right; mirror at draw time for left
(`DrawOptions` / `MirrorX`). A `*-turn` clip ends facing the other way
(limb sides and the face swap at its midpoint), so flip the Mirrored
flag when the turn completes.

## Rules that keep files playable

- **Keyframes only — no expressions.** Lottie expressions are JavaScript
  that native players (lottie-go included) do not run. Anything dynamic
  — wiggle, bounce, springs — must be baked into keyframes; compute the
  values and emit keys. This matches how the whole non-web Lottie
  ecosystem works.
- **Stay inside the supported subset.** lottie-go covers most of Lottie
  (including blur/shadow-style effects, time remap, all common blend and
  mask modes) but not everything; the current lists and the safe
  fallbacks are in [references/subset.md](references/subset.md). You do
  not need to memorize them — lottiecheck names any violation — but
  consult subset.md before reaching for an exotic feature.
- **Keep the naming convention.** Clips end in `-anim`, states in
  `-state`, markers in `-seg`; inputs are bare verbs. dotLottie keeps
  these in four namespaces without distinguishing them, so the suffixes
  are what keep the machine JSON readable.
- **Preserve unknown fields.** Repack round-trips them; hand-written
  scripts that rewrite whole files can drop them. Edit surgically.

## When something is wrong

Run lottiecheck first — it names the animation and the feature or the
missing asset. A clip that checks `ok` but renders wrong is almost always
one of: an anchor moved (part rotates around the wrong point), a keyframe
`t` out of order, a layer `parent` index changed, or an image whose size
no longer matches its asset `w`/`h`. Compare against the untouched preset
dump — it is the known-good reference for every structure this skill
touches.
