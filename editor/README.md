# Lottie State Machine Editor

A desktop tool for bundling short Lottie clips into one dotLottie v2 archive
and wiring the state machine a game drives by name — `sm.Fire("jump")`
rather than tracking frame ranges.

Built with [Guigui](https://github.com/guigui-gui/guigui). It is a separate
Go module, so the `lottie-go` library itself never pulls in a GUI toolkit.

```bash
cd editor && go run . ../testdata/editor/character/character.lottie
```

The path argument is optional; **Open…**, **Save As…**, and **Import…** open
native file dialogs through [zenity](https://github.com/ncruces/zenity),
which is pure Go and needs no cgo.

`-viewer` starts **viewer mode**: the editor watches every file the
document was loaded from — the `.lottie` bundle (whose images travel
inside it), or a loose clip `.json` plus anything imported after — and
reloads automatically when any of them changes on disk. A change only
triggers once the file stats hold still for a poll interval, so a tool
rewriting several files (a `lottierepack`, an agent editing clips) is
picked up when it finishes, not mid-write. The disk is the source of
truth in this mode: saving is greyed out and in-editor edits last only
until the next reload. It is the intended way to watch AI edits land
live:

```bash
cd editor && go run . -viewer ../testdata/presets/chibi-male/chibi-male.lottie
```

The editor is its own module and depends on a released `lottie-go`, so it
installs like any other command:

```bash
go run github.com/shibukawa/lottie-go/editor@latest
```

The repository root carries a `go.work` that points the editor at the
library in this checkout, so working on both at once needs no `replace`
directive. Set `GOWORK=off` to build the editor against the released
library instead — worth doing before a release, to check the library
actually exports everything the editor uses.

Right now that build fails: the Markers tab needs `Player.OnMarker` and
`StateMachinePlayer.OnMarker`, which are newer than v0.5.2. The editor
builds from this checkout until the next library release.

## Samples

`testdata/editor/` holds three generated bundles — a platformer character,
a marker-segmented spritesheet, and a combo whose clips chain end to end.
See its README for what each one demonstrates. Regenerate them with:

```bash
cd editor && go run ./gensamples
```

They are generated rather than downloaded so there is no third-party
licensing to track: every clip is authored in this repository.

Their names carry what kind of thing they are — `-anim` for clips, `-state`
for states, `-seg` for markers, nothing for inputs. dotLottie keeps those in
four separate namespaces but does nothing to tell them apart, so a machine
where the clip, the state, the marker and the event are all called `jump`
reads fine on screen and is unreadable as data. Inputs stay bare because
they are what a game passes to `Fire`.

## Layout

- **Clips** — one row per playable unit, which is a file narrowed to one of
  its markers. A document carrying three markers is three clips here, so the
  same file name repeats down the source column. Selecting one plays it on
  its own, so it can be judged before being wired into any state.
- **Machines** — beside the clips. A bundle may hold several state machines,
  each its own file under `s/`, sharing the bundle's clips but not each
  other's states. Create, rename and delete them here; selecting one opens
  it in the graph. `▶` marks the one the manifest names as default — what
  `NewStateMachinePlayer("")` loads — and *Set initial* names it; naming the
  one already marked clears the choice, putting "the first listed" back. Nothing in the format switches between machines at
  runtime, so they are alternative entry points a game picks between: it
  makes a `StateMachinePlayer` per machine and drives whichever it wants.
- **Interface** — the machine's external surface, split by direction:
  - *Events* come in from the game. Each carries a *Try* button. `Restart`
    leads the tab as a pseudo-event: triggerable like the rest, but not
    renamable or removable, since the document does not declare it.
  - *Values* come in too, each with a checkbox or a field.
  - *Markers* go the other way: the animation passes them and the game
    reacts. They are not declared by the machine and cannot be driven from
    here, so the tab reports where each one lives and how often it has
    fired this run.

  Selecting an event or value traces the transitions that read it, in
  orange, on the graph.
- **State graph** — states as nodes, transitions as arrows, outlined as its
  own working area. Click to select, drag to move. `▶` marks the initial
  state, and the state the preview is currently in is filled green, so the
  machine can be watched running.
- **Inspector** — the selected state's playback fields, its transitions in
  the order that decides which one wins, and the guards on the selected
  transition. Validation problems are listed at the bottom.
- **Preview** — the stage, outlined to match the graph. It runs the machine
  through the same interpreter a game uses, or plays a single clip.
  Everything that drives it lives in the Inputs table.
- **Timeline** — under the stage: the whole document as a track, the range
  actually playing as a lighter band, every marker as a labelled tick, and
  the playhead. A state that names a segment shows as a band covering only
  part of the document, which is how you tell the segment was cut where you
  meant. Drag to scrub. The selected hitbox's active spans draw on the same
  ruler, so when it comes out and disappears reads against the playhead.
- **Collision** — the strip under the timeline, editing the two physics
  extensions (see the library README). The hitbox row works on the clip on
  stage: `+Rect` / `+Circle` add a frame-stepped box, the name and tag
  fields describe it, and tags color it by meaning — `hit` red, `hurt`
  green, `push` amber, untagged gray. The span row times the current box:
  from/to fields edit the interval under the playhead, `+Span` starts a new
  pose at the playhead copying the previous one. The body row places the
  bundle-wide rigid silhouette (violet). All shapes are dragged on the
  stage itself: inside moves, the white grip resizes, empty stage
  deselects. The checkbox hides the whole overlay.

## Notes

A native dialog blocks for as long as it is on screen, so dialogs run on
their own goroutine and their result is applied from `Tick`. The window
keeps rendering while one is open, and every `Model` field stays owned by
the main goroutine. Only one dialog runs at a time; the buttons grey out
while one is up.

Transitions are created and edited in the inspector; the graph draws them.
Drag-to-connect is not implemented.

Node positions are stored in each state's extra fields under
`x-lottie-go-editor`. The dotLottie schema has nowhere to record graph
layout, and `lottie-go` writes members it does not model back unchanged, so
positions survive a save without making the file invalid for other runtimes.

Setting `LSM_EDITOR_SCREENSHOT` to a path renders a frame to a PNG and
exits, which is how the UI is checked without a display:

```bash
LSM_EDITOR_SCREENSHOT=/tmp/shot.png LSM_EDITOR_SCREENSHOT_TICKS=45 \
  go run . path/to/character.lottie
```
