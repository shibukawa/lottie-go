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

- Go 1.27+ (`StateMachinePlayer.Set` / `Get` are generic methods)
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

## dotLottie bundles

A `.lottie` archive can hold many clips plus the state machines that
sequence them. `DecodeBundle` opens one; animations decode on first use, so
reading the manifest does not pay for every clip.

```go
f, _ := os.Open("character.lottie")
st, _ := f.Stat()
b, err := lottie.DecodeBundle(f, st.Size())
if err != nil {
    log.Fatal(err)
}
for _, id := range b.AnimationIDs() {
    anim, _ := b.Animation(id)
    log.Println(id, anim.Duration())
}
```

Bundles are also writable, which is what an authoring tool needs. `Encode`
always writes version 2, and reconciles the manifest against the files the
bundle actually holds:

```go
b := lottie.NewBundle()
b.SetAnimation("walk", walkJSON)
b.SetStateMachine("character", &lottie.StateMachine{
    Initial: "walk",
    States: []lottie.State{{
        Name: "walk", Type: lottie.StatePlayback, Animation: "walk", Loop: true,
    }},
})
out, _ := os.Create("character.lottie")
b.Encode(out)
```

`ParseStateMachine` reads the documents under `s/` into `StateMachine`.
Members this package does not model are preserved and written back
unchanged, so editing a bundle here keeps it valid for other dotLottie
runtimes. Those members are reachable as `ExtraFields`, so a tool can also
stash its own data in one — an editor's node positions, say — and have it
survive a rewrite. `Bundle.Validate` reports dangling transition targets, unknown
animation ids, unreachable states, and markers that do not exist.

## State machines

`NewStateMachinePlayer` runs a machine, so a game asks for `"jump"` instead
of tracking frame ranges:

```go
sm, err := b.NewStateMachinePlayer("character")
if err != nil {
    log.Fatal(err)
}

func (g *Game) Update() error {
    if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
        sm.Fire("jump")   // an Event input declared in the machine
    }
    sm.Set("speed", g.player.Speed)
    sm.Update()
    return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
    sm.Draw(screen, nil)
}
```

`Fire` queues an event for the next `Update`, so calling it from anywhere in
a frame is safe. An event is consumed by the transition that takes it, so one
`Fire` moves one step.

Value inputs persist until changed. `Set` and `Get` are generic methods, so
one pair covers all three input kinds and takes whatever numeric type the
game already holds:

```go
sm.Set("speed", 12.5)      // Numeric
sm.Set("hp", hitPoints)    // any int or float type, or an untyped constant
sm.Set("grounded", true)   // Boolean
sm.Set("weapon", "bow")    // String

speed, ok := sm.Get[float64]("speed")  // the type must be explicit
```

Passing a type the inputs cannot hold is a compile error, and `Get` reports
false when the input is unset or holds a different kind of value.

`Update` advances the current clip, runs the `OnComplete` and
`OnLoopComplete` interactions it triggers, then takes whichever transitions
apply — in declaration order, the current state's own before any
`GlobalState`'s. A one-shot clip therefore returns to idle on the tick it
ends, with no game-side timer.

Supported: `PlaybackState`, `GlobalState`, both transition kinds, all four
guard and input types, `OnComplete` / `OnLoopComplete`, and the `Fire`,
`Toggle`, `Increment`, `Decrement`, `SetFrame`, and `SetProgress` actions.
Pointer interactions are not run — a game supplies its own input — and
`OpenUrl` and `SetTheme` are out of scope.
`StateMachinePlayer.UnsupportedFeatures` reports whatever it skipped, and
`Err` surfaces a state naming an animation the bundle lacks.

Named frame ranges — what a state's `segment` refers to — come from the
Lottie `markers` array, and `Player` can use them directly without any state
machine:

```go
p := anim.NewPlayer()
p.SetMarkerRange("walk")   // or p.SetRange(start, end)
p.SetReverse(true)
p.SetLoopCount(3)
p.OnComplete(func() { ... })

w, h := p.Animation().Size()   // to place or scale the drawing
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

# Lottie-driven stopwatch UI: cartoon digits that launch and bounce on
# change, a squishy colon, and buttons that burst particles on click
go run ./examples/stopwatch

# performance target verification (5+ concurrent animations)
go run ./examples/stress
```

The stopwatch shows the intended game-UI pattern: ten `digit-N.json`
animations each display digit N at frame 0 and morph to N+1 when played,
so the game just calls `Seek(0); Play()` on the right player when a value
changes. Buttons are Lottie files whose press "pop" plays on click. All
assets are generated by `go run ./examples/stopwatch/gen`.

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

**Containers**: Lottie JSON (`.json`) and dotLottie (`.lottie`), both
archive layouts — version 2 (`a/ i/ s/ t/ f/`) and version 1
(`animations/ images/`). See [dotLottie bundles](#dotlottie-bundles).

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
