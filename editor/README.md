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

## Samples

`testdata/editor/` holds three generated bundles — a platformer character,
a marker-segmented spritesheet, and a combo whose clips chain end to end.
See its README for what each one demonstrates. Regenerate them with:

```bash
cd editor && go run ./gensamples
```

They are generated rather than downloaded so there is no third-party
licensing to track: every clip is authored in this repository.

## Layout

- **Clips** — the animations in the bundle, and the machine's inputs.
  Selecting a clip plays it on its own, so it can be judged before being
  wired into any state.
- **Inputs** — every input carries the control that exercises it: a *Try*
  button for an event, a checkbox or field for a value. `Restart` sits at the
  top as a pseudo-input: it can be triggered like the rest but not renamed or
  removed, since the document does not declare it. Selecting an input traces
  the transitions that read it, in orange, on the graph.
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
  meant. Drag to scrub.

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
