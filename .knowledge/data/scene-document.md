---
id: data:scene-document
type: data
title: Scene Document
---

One composed screen for vision:scene-editor. A standalone JSON file that references bundles (decision:scene-references-bundles). Node names are the game-facing ids, like socket names in api:sockets.

```yaml
root:
  name: string; scene id
  size: {w, h}; design resolution — scene coords span this box; screen mapping is runtime-side (api:scene-runtime)
  bundles: list of {alias, path}; relative to the scene file
  images: list of {alias, path}; static image files (png/jpeg/webp via consumer-registered decoders)
  fonts: list of {alias, path}; ttf/otf for text nodes
  phases: list of {name, duration, next}; requirement:scene-phases — first is where the scene starts
  nodes: ordered list; order is draw order, first = back — overlap is edited by reordering
  options:
    hoverMovesFocus: bool; menu default true
    initialFocus: node name; empty = lowest tabIndex
node:
  name: string; unique in scene, game-facing
  kind: animation | machine | image | text
  source: {bundle: alias, id} for animation/machine, {image: alias} for image; text has none. One bundle = one node = one player/layer; id picks WHICH machine or clip of it plays and is switched on the node, not by placing parts
  playback.then: list of {animation, segment, loop, ...} — chain played as each clip completes; a looping link parks there (entrance once, then idle loop forever — the everyday pattern without a state machine)
  text: # kind text only; content overwritable by name at runtime (SetText)
    {value, font: alias, size, align: left|center|right, anchorX: left|center|right, anchorY: top|middle|bottom, color: "#rrggbb(aa)", lineHeight}
  phase: phase this node belongs to; empty = every phase (requirement:scene-phases)
  transform: {x, y, scaleX, scaleY, rotation, opacity}
  playback: # kind animation only
    {segment: marker name, loop, loopCount, speed, mode, autoplay}
  entry: string # kind machine only; initial state override, empty = machine's own initial
  start: seconds from scene start before the node enters (requirement:scene-timeline); 0 = from the beginning
  focus:
    focusable: bool
    tabIndex: int; Next/Prev traversal order
    neighbors: {up, down, left, right: node name}; empty = geometric nearest
  bindings: list # requirement:scene-interactions
    - {on: focus|blur|hover|unhover|press|activate|cancel, do: fireEvent|playSegment|callback|focus, arg: event / marker / callback / node name, target: node the action acts on (empty = self)}
```

Numbers written by the editor round to two decimals, matching the state machine editor's geometry convention.
