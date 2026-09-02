# octopus — a textured soft-body sample

A purple octopus idles among the kelp: its mantle breathes, its eight arms
undulate, bubbles rise. Every moving part is a keyframed vector path, and
the art on it is an image painted through the path with lottie-go's
texture extension ([plugin/texture](../../../plugin/texture)):

- the **mantle** is a blob path whose spotted skin maps by **bounding box**,
  so the squash-and-stretch keyframes stretch the spots with it;
- the **arms** are tapered paths with a whole-arm texture mapped **per
  vertex** — `u` runs from the mantle to the tip, `v` across the arm — so
  every arm shades from dark at the head to pale at the tip and the suckers
  ride every bend the keyframes put into it;
- the **kelp** is stroked paths with a frond texture mapped **along the
  stroke**, repeating as the fronds sway.

The clip is plain Lottie. Press **T** to see it the way a player without
the extension does: the same shapes in their fallback solid colors. The
document that binds the images lives beside the clip in the bundle, at
`extensions/texture/swim.json`; loose copies of the clip, the document and
the three PNGs sit under `assets/` for inspection.

All art is drawn procedurally by the generator — no downloaded images, no
fonts — so there is nothing third-party to license. 480x360, 30 fps, a
three-second seamless loop.

The bundle is embedded in the command, so it needs no file argument and
plays from anywhere with a Go toolchain — the quickest way to hand the
sample to someone:

    go run github.com/shibukawa/lottie-go/examples/lottie/octopus@latest

Inside the repository, `go run ./examples/lottie/octopus`.

Controls: space pauses, R restarts, T toggles the textures. Setting
`LOTTIE_OCTOPUS_SCREENSHOT=out.png` writes one frame (`LOTTIE_OCTOPUS_FRAME`
picks which) and exits.

Regenerate the textures, the clip and the bundle with:

    go run ./examples/lottie/octopus/gen

The bundle also opens in the general player, which applies a bundle's
texture document when it has one:

    go run ./examples/lottie/player examples/lottie/octopus/octopus.lottie

and in the editor (`go run ./cmd/lottie-state-editor
examples/lottie/octopus/octopus.lottie`), where the Shapes tab shows the
paints on the fills and strokes and the UV pane on each arm.
