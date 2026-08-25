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

The module carries a `replace` pointing at the parent directory, because the
editor needs library changes newer than the latest tag: `ExtraFields` had to
be exported so a tool can write vendor data into a state, and `Player` gained
an `Animation` accessor so the preview can size its stage. Drop the replace
and depend on a released version once those ship.

## Samples

`testdata/editor/` holds two generated bundles — a platformer character and
a marker-segmented spritesheet. See its README for what each one
demonstrates. Regenerate them with:

```bash
cd editor && go run ./gensamples
```

They are generated rather than downloaded so there is no third-party
licensing to track: every clip is authored in this repository.

## Layout

- **Clips** — the animations in the bundle, and the machine's inputs. Event
  input names are the triggers a game fires, so they are edited next to the
  clips they drive.
- **State graph** — states as nodes, transitions as arrows. Click to select,
  drag to move. `▶` marks the initial state.
- **Inspector** — the selected state's playback fields, its transitions in
  the order that decides which one wins, and the guards on the selected
  transition. Validation problems are listed at the bottom.
- **Preview** — runs the machine through the same interpreter a game uses,
  with one button per Event input and a control for every value input, so a
  transition guarded on a boolean can be exercised here.

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
