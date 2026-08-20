# lottie-go

A pure-Go [Lottie](https://lottie.github.io/) player for
[Ebitengine](https://ebitengine.org/), rendering through the `vector`
package. No cgo, no dependencies beyond Ebitengine — WASM and mobile builds
work unchanged.

The goal is a practical subset for **UI motion** (icons, loaders, HUD
elements, transitions) authored in editors like Lottie Creator, Lottielab,
SVGator, and Glaxnimate. It is not a full spec implementation: unsupported
features are skipped explicitly and rendering continues.

## Requirements

- Go 1.26+
- Ebitengine v2.9+ (uses `vector.FillPath` / `vector.StrokePath`)

## Usage

```go
f, _ := os.Open("loading.json")
anim, err := lottie.Decode(f)
if err != nil {
    log.Fatal(err)
}
player := anim.NewPlayer()
player.SetLoop(true)

// ebiten.Game implementation:
func (g *Game) Update() error {
    player.Update()
    return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
    player.Draw(screen, nil) // or &lottie.DrawOptions{GeoM: ..., ColorScale: ...}
}
```

Run the demos:

```
# minimal code sample: play a single file
go run ./examples/basic path/to/animation.json

# gallery of 60 bundled CC0 sample animations (click a tile to zoom)
go run ./examples/gallery

# standalone viewer: open any Lottie JSON by argument or drag & drop,
# with pause/seek/speed/loop controls and a progress bar
go run ./examples/player [path/to/animation.json]
```

The gallery samples come from the public-domain (CC0) `data/` set of
[LottieFiles/test-files](https://github.com/LottieFiles/test-files); see
[examples/gallery/assets/README.md](examples/gallery/assets/README.md).

## Supported features

**Layers**: shape, null, solid, precomposition (offscreen with clipping and
resolution-aware scaling), image (embedded data-URI assets, plus external
assets via `DecodeWithAssets`), text (see below), parenting, in/out points,
time stretch, auto-orient.

**Transforms & keyframes**: anchor, position (incl. split x/y and spatial
bezier paths), scale, rotation, skew, opacity; linear / hold / cubic-bezier
easing; legacy end-value (`e`) keyframes.

**Shapes**: bezier paths, rectangles (incl. rounded), ellipses, polystars
(stars & polygons incl. roundness), nested groups, trim paths (simultaneous
& individual, incl. offset wrap), rounded-corner modifiers, repeaters
(cumulative transform + opacity ramp), merge paths (merge mode).

**Styles**: solid fills (non-zero / even-odd), solid strokes (width, cap,
join, miter), dash patterns (incl. offset), linear & radial gradient fills
and strokes (Kage shader, alpha stops supported).

**Compositing**: masks (add / subtract), track mattes (alpha, alpha
inverted, luma, luma inverted — luma via Kage), blend modes (normal,
multiply, screen).

**Containers**: Lottie JSON (`.json`) and dotLottie (`.lottie`) via
`DecodeDotLottie` / `DecodeDotLottieAnimation`.

**Text layers** render through Ebitengine's `text/v2`. Fonts are never
bundled or fetched; supply them from your game:

```go
src, _ := text.NewGoTextFaceSource(bytes.NewReader(fontTTF))
anim.SetFontResolver(func(family, style string) *text.GoTextFaceSource {
    return src // pick a face per family/style as needed
})
```

Document text, size, fill color, justification, line height, and line
breaks are supported; per-character animators are reported as unsupported.

Skipped-but-tolerated (reported via `anim.UnsupportedFeatures()`):
expressions, effects, 3D layers, time remap, text animators, boolean merge
modes, other blend and mask modes, zig-zag / offset-path / pucker-bloat
modifiers. See `.knowledge/` for the full requirement catalog.

## Design notes

- Lottie and Ebitengine share a top-left origin with Y down; no coordinate
  conversion happens anywhere.
- Transforms are baked into path vertices (not `GeoM`) so stroke widths scale
  the way Lottie specifies; stroke width is multiplied by the mean scale of
  the accumulated transform.
- Colors are drawn premultiplied via `DrawPathOptions.ColorScale`.
- Per-frame CPU work (property evaluation + path building) reuses buffers;
  the included benchmark measures it at single-digit microseconds for a
  typical UI asset.
- `go run ./examples/stress` verifies the performance target (5 concurrent
  UI-scale animations, p99 draw under 2ms). Measured on an M3: p99 0.98ms
  at 5 players; 40 players still hold 60fps.

## Verification

- Pixel-compared against lottie-web reference renders from
  [LottieFiles/test-files](https://github.com/LottieFiles/test-files)
  (RMSE ~1% across transforms, easing, winding, gradients, mattes, masks,
  precomps, polystars, dashes, auto-orient).
- Exports from Glaxnimate, Lottielab, SVGator, and Lottie Creator decode
  and render with no unsupported-feature fallbacks.

## License

Apache License 2.0 — see [LICENSE](LICENSE).

Bundled sample animations in `examples/gallery/assets/` are CC0 (public
domain) from [LottieFiles/test-files](https://github.com/LottieFiles/test-files).
