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

A bundle may hold several machines. `NewStateMachinePlayer("")` takes the one
the manifest names as initial, or the first listed. `SetMachine` swaps
another in, which is how a game changes a character's whole animation set at
once — surfacing from underwater onto land:

```go
sm, _ := b.NewStateMachinePlayer("")   // or a specific id
sm.SetMachine("underwater")
```

The new machine starts at its own initial state, but **value inputs carry
over by name**: they describe the game's world, not the machine, so whatever
`speed` or `grounded` were, they still are — and the machine settles against
them on entry rather than showing a frame it was never really in. Events do
not carry; one fired at the machine being left is dropped. This is a
lottie-go convenience, not part of the dotLottie schema, and it changes
nothing about the document.

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

Markers are also cues, not just ranges. `OnMarker` fires as playback passes
one, so a footstep or a hit frame hangs off the animation instead of a
frame count in game code:

```go
p.OnMarker(func(m lottie.Marker) { play(m.Name) })

// or, with the state machine, told which state was playing:
sm.OnMarker(func(state string, m lottie.Marker) { play(state, m.Name) })
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

# GPU cost inspection and pixel-regression guard for the compositing path
go run -tags ebitenginedebug ./examples/gpuprobe -copies 20 |
	go run ./examples/gpuprobe -summarize
go run ./examples/gpuprobe -golden /tmp/base    # record reference renders
go run ./examples/gpuprobe -compare /tmp/base   # verify against them
```

`gpuprobe` exists because draw-call merging and texture allocation are not
observable from outside Ebitengine. Built with the `ebitenginedebug` tag,
Ebitengine dumps every graphics command and every internal texture once per
frame, and `-summarize` reduces that to the numbers worth tracking. Its
`-golden`/`-compare` modes render fixed frames of every bundled asset to PNG,
which is how changes to offscreen allocation are shown to be pixel-neutral.

The stopwatch shows the intended game-UI pattern: ten `digit-N.json`
animations each display digit N at frame 0 and morph to N+1 when played,
so the game just calls `Seek(0); Play()` on the right player when a value
changes. Buttons are Lottie files whose press "pop" plays on click. All
assets are generated by `go run ./examples/stopwatch/gen`.

The gallery samples come from the public-domain (CC0) `data/` set of
[LottieFiles/test-files](https://github.com/LottieFiles/test-files); see
[examples/gallery/assets/README.md](examples/gallery/assets/README.md).

## Collision plugins

dotLottie says nothing about physics, so collision data rides in the
bundle under `extensions/` — a directory the core treats as opaque bytes
and carries through any rewrite verbatim (`ExtensionFile` /
`SetExtensionFile` are the raw accessors, and files from tools this build
does not know survive untouched).

Meaning comes from static plugins: importing a plugin module is what
enables reading and writing its payload, and a program that never imports
one never links its code or its engine. There are two, named after the
engines they feed:

**`github.com/shibukawa/lottie-go/plugin/physics/cp`** — rigid body
silhouettes (`extensions/physics/cp/`): fixed circles, boxes, and convex
polygons describing a character's physical outline, with friction,
elasticity, and sensor flags. One definition per bundle serves every clip,
and it drops straight into a [jakecoffman/cp](https://github.com/jakecoffman/cp)
space:

```go
def, _ := lottiecp.Load(b, "body")
body, _ := lottiecp.AddToSpace(space, def)   // *cp.Body, ready to position
```

**`github.com/shibukawa/lottie-go/plugin/physics/resolv`** — frame-stepped
hitboxes (`extensions/physics/resolv/`), the fighting-game kind: named
boxes carrying free-form tags — `hit`, `hurt`, `push` — each active over
frame spans `[from, to)` with constant geometry per span. They are
authored per animation and queried by frame and tag:

```go
track, _ := lottieresolv.Load(b, "attack")
for _, box := range track.At(player.Frame(), "hit") { ... }
```

or mirrored into a [SolarLune/resolv](https://github.com/SolarLune/resolv)
space, with shapes inserted and removed as spans start and end:

```go
tracker := lottieresolv.NewTracker(space, track)

func (g *Game) Update() error {
    player.Update()
    tracker.SetOffset(g.character.X, g.character.Y)
    tracker.Sync(player.Frame())
    // collide via resolv; shapes answer for lottieresolv.Tag("hurt")
}
```

The bundled editor imports both plugins and edits their data over the
stage: dragging moves and resizes, the span row times when a box appears
and disappears, and the tag line colors boxes by meaning (hit red, hurt
green, push amber).

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
- Masks, track mattes and precomps need an offscreen. Ebitengine drops an
  image from its shared texture atlas as soon as anything is drawn into one,
  and doubles the wait before it may rejoin on every further such use, so
  offscreens are permanently textures of their own and their count is what
  matters. One process-wide pool serves every `Player`, and each offscreen
  covers only the layer's own bounds rather than the whole destination, which
  keeps that count flat as animations are added: 20 concurrent matte
  animations hold 6 textures totalling 6.4MiB.
- While a player's draw inputs repeat — same frame, transform, color scale,
  and destination — the frame is baked once and reused, so an idle player
  costs one texture draw and no evaluation. Ebitengine migrates the
  read-only bake onto its source atlas after ~10 frames, at which point the
  composites of every idle player merge: 20 paused matte animations settle
  at 3 draw calls per frame, down from 242. Baking is skipped for
  animations using blend modes (they must composite against the backdrop),
  and `Player.SetSnapshotCache(false)` turns it off.
- When one composition holds two or more masked or matted layers, their
  offscreen work runs as shared phases over two scratch atlases — contents
  on one, mask coverage and matte sources on the other — so the fills batch
  across layers and the combines and composites each merge into single
  draws. A synthetic composition with 16 masked layers renders in 14 draw
  calls instead of 197. Whether a layer can join is decided at decode time;
  layers that cannot (masked text, an overflowing frame) fall back to the
  pooled path, which remains the reference behavior.

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
