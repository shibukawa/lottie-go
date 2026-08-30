# cutscene — a vector cutscene sample

A PSP-era "moving manga" cutscene, authored entirely in code: a pig late
for school sprints with toast in its mouth, a squirrel hurries the other
way, and they meet at a corner. 480x272 (the PSP's screen), 30 fps,
about 8 seconds.

Like the editor samples, the JSON is generated rather than downloaded so
its licensing is unambiguous: everything is authored in this repository.
The same storyboard retold with raster art lives in
[examples/lottie/motioncomic](../motioncomic).

The JSON is embedded in the command here, so it plays from anywhere:

    go run github.com/shibukawa/lottie-go/examples/lottie/cutscene@latest

Inside the repository, `go run ./examples/lottie/cutscene`, or open the file in
the general player for seeking and speed controls:

    go run ./examples/lottie/player examples/lottie/cutscene/cutscene.json

Regenerate the JSON with:

    go run ./examples/lottie/cutscene/gen

## Cut list

The timeline is a single composition; cuts are layer in/out points, and
each cut is also a Lottie marker so segment players can address them.

| marker | frames | shot |
| --- | --- | --- |
| `cut1-seg` | 0–66 | Pig sprints right, toast in mouth. Town scrolls left, speed lines, flying sweat. |
| `cut2-seg` | 66–120 | Squirrel sprints left, acorn in paws. Park scrolls right. |
| `cut3-seg` | 120–150 | The corner. Both enter at full speed, white flash, star burst, knockback. |
| `cut4-seg` | 150–248 | Aftermath: both sit dazed under orbiting stars. The toast lands on the pig, the acorn bonks the squirrel. Iris out. |

## Rigs

The two characters are precomps so cuts 1–3 can reuse them; the run
cycles are unrolled across the whole timeline so any in/out window works.

- **pig-comp** (200x160, feet on y=150): body is the root layer; the head
  (ears, snout, eye) and the four legs parent to it, and the toast
  parents to the head. The run cycle is 12 frames — legs swing ±42° from
  hip anchors, far legs a quarter cycle out of phase, the body bounces
  every half cycle and everything parented to it bounces along.
- **squirrel-comp** (160x140, feet on y=130): same structure, 10-frame
  cycle, with a big wagging tail behind the body and an acorn at the paws.

Cut 4 needs sitting poses, which are drawn fresh as single shape layers
(one per character) — a layer-rotation wobble is all the animation a
dazed sit needs.

## Effects vocabulary

The manga feel comes from stock symbols, each its own layer: speed lines
(hold-keyframe streaks), a white impact flash, an 8-point star burst
with scatter stars, orbiting dizzy stars (a rotating layer whose star
groups sit off-anchor), a radial shock burst, popping exclamation marks,
a sweat drop, and an even-odd iris wipe to close.
