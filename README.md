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

Pointer interactions run when the game feeds input through `PointerDown` /
`PointerMove` / `PointerUp` (in composition coordinates; apply the inverse
of your draw transform to the cursor first). `Click` needs press and
release on the same target, and `PointerEnter` / `PointerExit` derive from
how each target's hit state changes across moves. An interaction naming a
layer hit-tests that layer's bounds on the current frame — also available
directly as `Player.HitTest` — and one naming no layer reacts anywhere.

What `OpenUrl` and `SetTheme` mean is the game's decision: register
`OnAction` to receive them.
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

## Scenes

A scene arranges many animations and state machines into one screen — a
game scene or a GUI menu — with positions, overlap, focus order, and input
bindings. It is a standalone JSON file (`*.scene.json`) referencing one or
more bundles, authored in the `cmd/lottie-layout/` tool and played back with the same
runtime:

```go
scene, _ := lottie.DecodeScene(f)
sp, _ := scene.NewScenePlayer(func(path string) (*lottie.Bundle, error) {
    return openBundle(path) // fs.FS / go:embed next to the scene file
})
sp.SetScreenMapping(screenW, screenH, lottie.ScaleContain)
sp.OnCallback(func(node, name string) { /* "start-game", ... */ })

func (g *Game) Update() error {
    if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
        sp.MoveFocus(lottie.FocusDown)   // d-pad / cursor keys; FocusNext for Tab
    }
    if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
        sp.Activate()                    // confirm; sp.Cancel() for the cancel button
    }
    x, y := ebiten.CursorPosition()
    sp.Pointer(float64(x), float64(y), ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft))
    sp.Update()
    return nil
}

func (g *Game) Draw(screen *ebiten.Image) { sp.Draw(screen, nil) }
```

Focus, hover, click, and the confirm button all reduce to a handful of
semantic events (`focus`, `blur`, `hover`, `press`, `activate`, `cancel`).
A node reacts through its bindings — fire an event into its machine, play
a marker segment, or report a named callback to the game — and a machine
node that declares an event input of the same name needs no wiring at all:
a button's normal/focused/pressed looks are just states. Individual nodes
are reachable by name for values a menu displays:

```go
if hp, ok := sp.Node("hp-bar"); ok {
    hp.Set("hp", g.player.HP)
}
```

`SetScreenMapping` maps the scene's design resolution onto the real
window: `ScaleContain` letterboxes, `ScaleCover` fills and crops the
sides, `ScaleStretch` distorts, `ScaleCenter` stays 1:1. `Pointer` inverts
the same mapping, so the game passes raw cursor coordinates.

A placed bundle is **one node — one player, one layer**; which of its
machines or clips it shows is a property of the node, switched in place.
An animation node can also chain clips: `playback.then` lists what plays
as each clip completes, so "play the entrance once, then loop idle
forever" needs no state machine — and every completion fires a `complete`
event that bindings can turn into a machine event, a phase switch, or a
callback.

Scenes also run a clock. A node's `start` time delays its entrance — it
neither draws nor takes input until then — which is how an intro
choreographs one animation starting over another. `Restart()` replays the
choreography from zero, `Time()` reads the clock, and a binding can aim
`fireEvent`/`playSegment` at another node by name (starting it early if
its time has not come) or move focus with the `focus` action.

A scene's life splits into named **phases** — a startup animation, the
interactive screen, an outro. A node names the phase it belongs to
(empty joins all of them and keeps playing across switches); a timed
phase rolls into its `next` automatically, a binding's `phase` action
switches on input, and `sp.SetPhase`/`sp.OnPhaseEnd` give the game the
same control — `OnPhaseEnd` is how it learns the outro finished:

```go
sp.OnPhaseEnd(func(phase string) {
    if phase == "outro" { g.leaveMenu() }
})
```

A scene also carries a 2D **camera** — position, zoom, rotation — that
each phase may override, with per-node parallax: a node's `depth` scales
how strongly the camera moves it (1 tracks fully, 0 pins to the screen
like a HUD, values between let a background drift slower, above 1 a
foreground leads). Hit tests and focus geometry follow the camera, and a
game animates it per frame:

```go
sp.SetCamera(lottie.SceneCamera{X: g.camX, Zoom: 1.2})
```

Entering a phase, `Restart`, and a clip chain moving to another clip all
rebuild the node's player, and whatever the game attached to the previous
one — a texture paint, an `OnMarker` cue, a collision tracker — goes with
it. `OnNodeStart` runs whenever a node's player or machine is (re)created,
and once right away for the nodes already running, so it is the one place
to dress them:

```go
sp.OnNodeStart(func(node string, p *lottie.Player, sm *lottie.StateMachinePlayer) {
    if node == "hero" && p != nil {
        p.SetTexture("skin", g.heroSkin)
        p.OnMarker(g.onHeroCue)
    }
})
```

Beyond animations, a scene places static **images** and **text**. Text
nodes carry font, size, alignment, and an anchor (a right-anchored score
grows leftward), and are named so the game overwrites them at runtime:

```go
if score, ok := sp.Node("score"); ok {
    score.SetText(fmt.Sprintf("SCORE %06d", g.score))
}
```

Image and font files are referenced beside the bundles and load through
`NewScenePlayerWithLoader(lottie.SceneLoader{Bundle: ..., File: ...})`;
image formats come from the binary's blank imports (`image/png`,
`image/jpeg`, `golang.org/x/image/webp`), as with every image asset.

The layout tool lives in `cmd/lottie-layout/` (a separate module, like
`cmd/lottie-state-editor/`):

```
go run ./cmd/lottie-layout my-menu.scene.json
```

It places clips, machines, images, and text from referenced files on a
canvas — with a live preview of the selected source before placing —
drags positions, reorders overlap, edits focus order, bindings, and
phases, and choreographs entrances on a timeline pane: layer rows,
draggable start bars, a playhead, a phase selector, and a Play/Pause
transport that starts stopped and pauses itself when the last element
finishes. The Preview button drives the very same `ScenePlayer` with
keyboard and mouse.

Run the demos:

```
# minimal code sample: play a single file
go run ./examples/lottie/basic path/to/animation.json

# gallery of 60 bundled CC0 sample animations (click a tile to zoom)
go run ./examples/lottie/gallery

# standalone viewer: open any Lottie JSON by argument or drag & drop,
# with pause/seek/speed/loop controls and a progress bar
go run ./examples/lottie/player [path/to/animation.json]

# Lottie-driven stopwatch UI: cartoon digits that launch and bounce on
# change, a squishy colon, and buttons that burst particles on click
go run ./examples/lottie/stopwatch

# a game opening built as one scene: skippable vanity card, a Hokusai-
# styled breaking wave settling into a calm sea, title, PRESS START
go run ./examples/layout/opening-animation

# the scene camera and parallax depth as a story: a searchlight sweeps a
# dark kitchen — the beam is a screen-pinned depth-0 mask, each stop a
# phase's camera — and finally catches a mouse eating the cheese
go run ./examples/layout/searchlight

# performance target verification (5+ concurrent animations)
go run ./examples/lottie/stress

# GPU cost inspection and pixel-regression guard for the compositing path
go run -tags ebitenginedebug ./examples/lottie/gpuprobe -copies 20 |
	go run ./examples/lottie/gpuprobe -summarize
go run ./examples/lottie/gpuprobe -golden /tmp/base    # record reference renders
go run ./examples/lottie/gpuprobe -compare /tmp/base   # verify against them
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
assets are generated by `go run ./examples/lottie/stopwatch/gen`.

The gallery samples come from the public-domain (CC0) `data/` set of
[LottieFiles/test-files](https://github.com/LottieFiles/test-files); see
[examples/lottie/gallery/assets/README.md](examples/lottie/gallery/assets/README.md).

## Character presets and AI customization

`examples/state-editor/presets/` holds game-ready character templates: raster cutout
rigs (part images moved by transform keyframes — no vector shapes, no
expressions) with a full clip set and a wired state machine. The first is
`chibi-male`, a 2.5-heads character with 19 clips from idle to death. The
art is a deliberate placeholder — one flat color per part — because the
value is the motion: replace the fifteen part images at the same sizes and
anchors and every clip plays with the new look.

Presets are built for automated editing. Two commands close the loop:

```bash
# explode a bundle into clips / parts / machines / extensions, and rebuild it
go run github.com/shibukawa/lottie-go/cmd/lottierepack -dump -dir work character.lottie
go run github.com/shibukawa/lottie-go/cmd/lottierepack -dir work -out character.lottie

# validate against the supported subset and render sample frames
go run github.com/shibukawa/lottie-go/cmd/lottiecheck -render preview/ character.lottie
```

[skills/lottie-character-preset](skills/lottie-character-preset/SKILL.md)
packages the whole workflow — rig contract, clip and state-machine
conventions, the supported subset — as an agent skill: copy it into your
project's `.claude/skills/` (or point your coding agent at it) and ask
for "a samurai version of this character" or "a livelier walk". Presets
regenerate from source with `cd cmd/lottie-state-editor && go run ./genpresets`.

A second skill, [skills/lottie-character-forge](skills/lottie-character-forge/SKILL.md),
makes a character that does not exist yet, with an image model (Gemini,
Grok) drawing the art. `cmd/lottieforge` does the mechanical half:

```bash
go run ./cmd/lottieforge grid  work   # spec -> grid templates + prompts to paste into the model
go run ./cmd/lottieforge cut   work   # returned sheets -> parts/, a report naming cells to redo
go run ./cmd/lottieforge rig   work   # parts + a preset -> work/<name>.lottie, every clip inherited
go run ./cmd/lottieforge morph work   # bake breathing, bends, squash, cloth and hair motion
```

The prompts ask for the character already split into rig parts on a grid
template; `rig` traces each part into a path with per-vertex UV (the
texture extension above), keeps the preset's transform keys, and adds
hair, skirts, capes, ribbons, tails and ornaments as typed attachments
that draw in the right order and move by kind — a pendulum for what
hangs, a vertex morph following the limbs for cloth. `lottiecheck -render`
then shows the result through the real renderer.

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

### Timed windows

A box with kind `window` is a hitbox with no shape: a pure timed flag —
a cancel window, invincibility, super armor — sharing tags and frame
spans with its siblings but never entering geometric queries or a resolv
space. The editor's `+Win` button makes one; games read it in a line:

```go
if track.Open(player.Frame(), "cancelable") { ... }
```

### Attachment sockets

Gameplay attaches things to the animation — a weapon to a hand, a
particle emitter to a muzzle — through named layer transforms. The core
query works on any animation, no plugin needed:

```go
pl, ok := anim.LayerPlacement("hand_r", player.Frame())
// pl.X, pl.Y, pl.Angle, pl.ScaleX/Y, pl.Visible — parents and precomps included
```

The animator drives a null layer in their own tool, so the attachment
interpolates exactly like the artwork. The
`github.com/shibukawa/lottie-go/plugin/sockets` package (a core
subpackage — no extra dependencies) adds the bundle-stored socket table
mapping stable game-facing names to layers, with a draw-order hint:

```go
set, _ := lottiesockets.Load(bundle)
if p, ok := set.At(anim, player.Frame(), "weapon"); ok {
    // p.X/p.Y/p.Angle place the sword; p.Z says front or behind
}
```

Root motion is the same query applied to a root null:
`lottiesockets.Displacement(anim, "root", lastFrame, frame)` reports how
far the character moved as drawn, so lunges and rolls come from the
animation instead of hand-tuned velocities.

### Frame events

For cues markers cannot carry — a payload, repeated same-name firings —
`github.com/shibukawa/lottie-go/plugin/events` stores per-animation event
tracks under `extensions/events/` and fires them through playback with
marker-identical crossing semantics (loops and reverse included):

```go
track, _ := lottieevents.Load(bundle, "attack")
lottieevents.Cue(player, track, func(e lottieevents.Event) {
    // e.Name, e.Frame, e.Payload (raw JSON: {"sound":"step","vol":0.4})
})
```

The underlying hook is `Player.OnFrameSpan`, which reports every frame
span Update sweeps; anything marker-like can be built on it.

### Facing

Data is authored facing right; every query offers the flip for a
left-facing character on one convention (position mirrors across a
vertical axis, angle negates): `ActiveBox.Mirrored`,
`LayerPlacement.Mirrored`, `Placed.Mirrored`, and `lottiecp.MirrorX`.

## Textured fills and strokes (extension)

Lottie has no way to paint an image through a vector path. lottie-go adds
one as an extension: a fill or stroke can name an image, mapped by the
shape's bounding box, by a UV per path vertex, or along a stroke. The
outline still animates and deforms; the picture follows it. The clip JSON
stays plain Lottie — a player without the extension draws the fill's
solid color — and the data lives beside the clip in the bundle, at
`extensions/texture/<clip>.json`, through the `plugin/texture` package:

```go
import lottietexture "github.com/shibukawa/lottie-go/plugin/texture"

anim, _ := b.Animation("hero")
p := anim.NewPlayer()
if doc, err := lottietexture.Load(b, "hero"); err == nil && doc != nil {
    doc.Apply(p) // binds every paint and UV set to this player
}
p.SetTexture("portrait", portraitImage) // a runtime image, by name

// State machines: every clip player the machine creates gets dressed.
lottietexture.Attach(sm, b)
```

The core hooks are `Player.SetTexturePaint`, `SetVertexUV` and
`SetTexture`, so a plain JSON clip can be textured from code too.
Rendering rasterizes the path into a coverage mask as usual and draws one
UV mesh through it with a Kage shader, so curves, fill rules, trim, dashes
and antialiasing are untouched; a textured style costs about what a
gradient does. The editor's Shapes tab binds images, edits the placement
on the stage, and lays out per-vertex UV in a pane.
[examples/lottie/octopus](examples/lottie/octopus) is a complete sample —
a soft-bodied octopus whose skin, suckers and kelp are all textured paths,
generated in-repository. Its bundle is embedded, so it runs from anywhere:

```bash
go run github.com/shibukawa/lottie-go/examples/lottie/octopus@latest
```

### Importing Spine skeletons

The same extension is what makes a [Spine](http://esotericsoftware.com/)
rig importable: a Spine mesh is a set of vertices with UVs that bones and
deform keys move, which is exactly what a textured path with per-vertex
UV is. `lottierepack -import-spine` reads the JSON export of Spine 4.x
(with its `.atlas`, or loose images) and bakes it into a bundle:

```bash
go run github.com/shibukawa/lottie-go/cmd/lottierepack -import-spine hero.json -dir work -out hero.lottie
```

Every animation becomes a clip; every slot a shape layer; every region or
mesh attachment a keyframed path — the mesh's hull by default, or one
path per triangle with `-mesh triangles` when the inner vertices carry
the deformation, at about five times the size — whose fill paints the
atlas page through the mesh's own UV. Bones with every inherit mode, IK and
transform constraints, weighted meshes, deform keys, slot colors,
attachment swaps and blend modes are evaluated at every frame and written
as keys; events become markers, and a state machine with one looping
state and one event per animation is generated so `sm.Fire("run")` works
on the first load. The clips are plain Lottie, so a player without the
extension still shows the shapes in their slot colors. Spine's own
spineboy (JSON, atlas and page: 467 KB) imports to a 504 KB bundle. Path and physics
constraints, clipping and draw-order keys are not converted and are
listed as notes. Keys are kept only where the motion leaves the straight
line between keys by more than `-tolerance` pixels (1 by default; 0
keeps every frame), and each key carries an easing fitted to the frames
it replaces, allowed to run them `-timing-tolerance` pixels early or
late (3 by default); `-skin`, `-fps`, `-scale`, `-bounds skeleton` and
`-bones` tune the rest, and the package behind the flag is
`plugin/spine`. Spine is a trademark of Esoteric Software; the importer
reads the exported files and needs no Spine runtime.

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
& individual, incl. offset wrap), rounded-corner, pucker-bloat, zig-zag and
offset-path modifiers, repeaters (cumulative transform + opacity ramp),
merge paths (merge, add, subtract and exclude-intersections via winding
rules; intersect needs true path booleans and stays unsupported).

**Styles**: solid fills (non-zero / even-odd), solid strokes (width, cap,
join, miter), dash patterns (incl. offset), linear & radial gradient fills
and strokes (Kage shader, alpha stops supported); textured fills and
strokes as a lottie-go extension (see above).

**Compositing**: masks (add / subtract / intersect, inverted variants,
expansion), track mattes (alpha, alpha inverted, luma, luma inverted —
luma via Kage),
blend modes (normal, multiply, screen and add fixed-function; overlay,
darken, lighten, color dodge, color burn, hard light, soft light,
difference and exclusion via a backdrop-sampling Kage shader), time remap
on precompositions.

**Effects**: Gaussian blur, drop shadow, fill, tint, tritone. Blur and
shadow softness run as separable Kage passes (large radii blur a
downscaled copy); parameters may be animated. Other effect types are
reported as unsupported and skipped.

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

Document text, size, fill color, justification, line height, tracking, and
line breaks are supported. Text animators run per glyph: position, scale,
rotation, opacity, fill color, and tracking, driven by range selectors
(percent or index units; characters, characters-excluding-spaces, words or
lines; square, ramp, triangle, round and smooth shapes).

Skipped-but-tolerated (reported via `anim.UnsupportedFeatures()`):
expressions, 3D layers, the merge-paths intersect mode,
hue / saturation / color / luminosity blend modes, difference / lighten /
darken mask modes, effects other than the five above, and the twist
modifier. See `.knowledge/` for the full requirement catalog.

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
- `go run ./examples/lottie/stress` verifies the performance target (5 concurrent
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

Bundled sample animations in `examples/lottie/gallery/assets/` are CC0 (public
domain) from [LottieFiles/test-files](https://github.com/LottieFiles/test-files).
