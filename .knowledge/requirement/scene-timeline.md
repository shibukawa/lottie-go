---
id: requirement:scene-timeline
type: requirement
title: Scene Timeline
---

Choreograph entrances on a scene-wide clock: the whole screen starts, and
a fixed time later another node's animation starts over it. Stored in
data:scene-document, run by api:scene-runtime, edited in the layout tool
(ui:layout-shell). Status: implemented.

```yaml
model:
  - node.start: seconds from scene start; until then the node neither draws, plays, nor takes input or focus
  - the ScenePlayer clock advances 1/TPS per Update; Time() reads it, Restart() replays entrances from zero
  - an action targeting an unentered node starts it early — an event-driven entrance (requirement:scene-interactions)
  - a late entrance that is the first focusable thing on screen receives initial focus
editor:
  - the timeline IS the node list — one row per participating node (requirement:scene-phases view), front layer on top; selection, draw-order reorder (drag a row's name vertically), and Delete all live here, so no separate nodes pane exists
  - bar from start over one pass of the clip; machine, image, text, and looping bars run open-ended and faded
  - drag a bar to move its entrance (two-decimal rounding); light while dragging, rebuild on release
  - a ruler lane above the rows scrubs the playhead; seeking replays from zero deterministically (SeekScene) and parks the transport
  - transport: starts stopped — a scene must not play itself on open — Play/Pause button, Replay restarts and plays; Play at the end starts over
  - playback pauses itself at the content end: the last entrance plus one pass of its clip — a LOOPING clip counts one pass too (excluding loops made a lone looped clip stop at 0s), machines/images/text get a one-second grace — marked on the strip
  - playhead rides the running clock; preview mode always runs regardless
wont:
  - keyframing properties over the timeline; motion lives in the Lottie clips
  - timeline-scrubbing to an arbitrary time (would need deterministic fast-forward)
```

The timeline choreographs *entrances*, not motion: what moves is still
authored in the clips and the state machines.
