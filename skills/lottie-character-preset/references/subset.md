# The lottie-go feature subset

lottie-go plays a practical subset of Lottie. Unsupported features are
skipped (the file still plays without them) and reported via
`Animation.UnsupportedFeatures()` — which is exactly what `lottiecheck`
prints. So: generate conservatively, then let the tool verify. This page
is for choosing an approach *before* you generate.

## Not supported — never emit these

- **Expressions** — JavaScript in the JSON. No native player runs them.
  Bake the motion into keyframes instead: compute wiggle/bounce/spring
  values yourself and emit a keyframe per beat (a wiggle at 4Hz on a
  60fps clip is keys every ~7-8 frames with alternating offsets).
- **3D layers** (`ddd: 1`, z-axis anything) — fake depth with scale and
  layer order.
- **Merge-paths intersect mode** — the other merge modes (merge, add,
  subtract, exclude-intersections) work.
- **Blend modes hue / saturation / color / luminosity** — all the
  common ones work (normal, multiply, screen, add, overlay, darken,
  lighten, color dodge/burn, hard/soft light, difference, exclusion).
- **Mask modes difference / lighten / darken** — add, subtract and
  intersect (plus inverted, plus expansion) work.
- **Effects beyond the five below** — supported effects are Gaussian
  blur, drop shadow, fill, tint, tritone (parameters animatable).
  Anything else (glow, wave warp, displacement...) is skipped.
- **The twist modifier** — pucker-bloat, zig-zag, offset-path and
  rounded-corners all work.
- **Per-character 3D text** — text layers and text animators work
  (position/scale/rotation/opacity/fill/tracking with range selectors).

## Fine in general, but not for preset characters

Presets are raster cutout rigs on purpose: images + transforms is the
representation an automated editor can reason about and a design swap
can't break. Vector shapes, gradients, trim paths, mattes and the
supported effects are all available in lottie-go — use them for UI and
cutscene work — but keep preset character clips to image layers and
transform keyframes unless the user explicitly wants otherwise. One
pragmatic exception: a vector shape layer for a simple prop or an impact
flash inside an attack clip is fine; it swaps out with the clip, not
with the character art.

## Runtime realities worth remembering

- **Fonts are never bundled**: text layers need the game to supply a
  font via `SetFontResolver`. Don't rely on text in character clips.
- **Facing is a runtime mirror**: author right-facing only; no `-left`
  clip variants, ever.
- **State machine pointer interactions** (Click/PointerDown...) exist
  for UI use; games feed inputs explicitly, so preset machines stick to
  events, booleans and `OnComplete`.
- **Markers** (`markers` array, `-seg` names) address frame ranges
  inside one document; a state can play a marker via `segment`. Useful
  when packing several micro-variations into one clip.

When unsure whether a construct is safe, emit it and run
`lottiecheck` — the answer is authoritative for the lottie-go version
actually in the game's go.mod, which beats any static list (this one
included).
