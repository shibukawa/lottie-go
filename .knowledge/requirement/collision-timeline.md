---
id: requirement:collision-timeline
type: requirement
title: Collision Timeline Chart
---

Status: proposed. Replace ui:collision-editing's button rows with an After
Effects-style time chart: one row per annotation, spans as bars on a frame
axis, placement by direct manipulation, parameters edited in the selection
pane (requirement:selection-driven-ui).

```yaml
chart:
  rows: one per hitbox / window / socket of the stage clip (cp body shapes are unkeyed; a single row or stage-only)
  bars: a box's spans drawn on the frame axis, tag-colored; drag to move/extend an active range
  place: create an item by adding a row and placing a bar at the playhead (replaces +Rect/+Circle/+Win buttons)
  scrub: shares the playhead with the existing timeline
right_pane: selecting a bar or row shows its parameters (name, tags, geometry, from/to) in the inspector
transport:
  problem: the preview loops continuously, so a moving playhead makes placement on the chart nearly impossible
  rule: touching the chart (click, drag, bar edit) pauses playback at once; an explicit play button resumes
  controls: play/pause, frame step -1/+1, jump to span edges; scrubbing already pauses by definition
visibility: rows appear per requirement:editor-config physics_backend (none hides the chart)
animation_within:
  ask: size and position animate across frames inside a placed range
  today: data:physics-resolv-track spans are constant-geometry steps
  candidates:
    - adjacent spans as keyframes with optional interpolation flag (format grows; runtimes opt in)
    - keep steps; editor eases authoring many short spans
  open: pick after chart interaction design settles; format change touches api:physics-resolv consumers
stage: remains the geometry-editing surface (drag/resize); chart owns time, stage owns space
```

Replaces the rows.collision / rows.sockets strip in ui:collision-editing;
requirement:frame-events authoring is a natural later row type on the same
chart.
