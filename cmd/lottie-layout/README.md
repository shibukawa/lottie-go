# Lottie Scene Layout (layout)

A desktop tool that arranges Lottie clips and dotLottie state machines
into scenes — game screens and GUI menus. It is the third layer over
lottie-go: the player plays one clip, the state machine editor sequences
clips into one actor, and this tool arranges many actors into one screen.

```
go run . [path/to/menu.scene.json]
```

## What it edits

A scene is a standalone JSON file (`*.scene.json`) that references
`.lottie` bundles, image files, and fonts by relative path. Each placed
node names what it shows (an animation, a state machine, a static image,
or a text block with font/size/alignment/anchor whose content the game
overwrites by name), where it sits (position, scale, rotation, opacity —
draw order is the overlap), when it enters (a start time on the scene
clock), which phase it belongs to (intro / main screen / outro, switched
by time, bindings, or the game), how it plays (segment, loop, speed, or
an entry-state override), whether focus can land on it (tab index,
directional neighbor links), and how it reacts (bindings from
`focus`/`hover`/`press`/`activate`/`cancel` to machine events, marker
segments, focus moves, phase switches, or named callbacks the game
receives).

## Working in the tool

- **Sources** (left): reference bundles, images, and fonts (+Text places
  a text node directly), preview the selected source live, and Place
  instances into the viewed phase.
- **Canvas** (center): drag nodes into place; green outlines mark
  focusable nodes, blue the selection, grey a node whose entrance has
  not come yet. Coordinates round to two decimals.
- **Timeline** (under the canvas): the node list and the choreography in
  one. Layer rows for the viewed phase, front on top — drag a name
  vertically to reorder the overlap, drag a bar horizontally to move its
  entrance, drag the ruler to scrub the playhead, Delete removes the
  selected node. The Play/Pause transport starts stopped and pauses
  itself when the last element's animation finishes (a looping clip
  counts one pass); Replay runs the choreography again.
- **Inspector** (right): parameters for the selection — the node
  (transform, playback, text style, focus, bindings, phase), or the
  scene itself (design size, focus options, and the phase list) via the
  Scene toolbar button.
- **Preview**: runs the real `ScenePlayer` with real input — Tab and
  Shift+Tab walk the tab order, cursor keys move directionally, Enter or
  Space activates, Esc cancels, and the mouse hovers and clicks. What the
  preview does is exactly what a game gets.

Games load the result with `lottie.DecodeScene` and drive it through
`ScenePlayer`; see the Scenes section of the repository README. For a
complete worked example — phases, entrance timeline, chained playback,
text nodes, callbacks — open the game-opening sample:

```
go run . ../../examples/layout/opening-animation/assets/opening.scene.json
```

## Screenshot mode

Setting `LAYOUT_SCREENSHOT=/path/out.png` renders the app for a few ticks
(`LAYOUT_SCREENSHOT_TICKS`, default 30), writes that frame, and exits —
how the UI is checked without a human looking at the window.
