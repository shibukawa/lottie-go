---
id: requirement:keyframe-timeline
type: requirement
title: Keyframe Marks on the Timeline
---

Status: implemented — the Poses tab of the strip under the stage
(editor/posechart.go). Show where a clip's keyframes are and make them
selectable.
Today ui:editor-shell's view.timeline draws markers, the played band and the
playhead, but nothing says a clip holds 5 poses at frames 0/4/7/12/20 — the
frames that are the only ones an edit can land on (requirement:pose-editing).

```yaml
marks:
  source: the stage clip's animated properties, read from data:clip-edit-document
  pose_row: when concept:pose-sequence-clip holds, one Poses row, one tick per pose time — 2..7 ticks, legible at clip width
  layer_rows: when key times disagree, one row per animated layer with that layer's own ticks; label is the layer name
  shape: hold keys draw square, interpolated keys round, so a flipTrack toggle is not mistaken for something to drag
  static: a layer with no animated property has no row; promoting it is an explicit edit (requirement:pose-editing)
  ruler: shares the timelineView track extent and the chart plot extent, so one frame sits at one x across stage timeline, collision chart and keyframe rows (ui:collision-editing chart rule)
select:
  tick: selects the pose (pose row) or the single key (layer row); drives the inspector per requirement:selection-driven-ui
  empty_track: scrubs, as chart.go already does
  pause_first: any touch pauses playback before acting (requirement:collision-timeline transport rule)
retime:
  drag: a tick moves along the frame axis; a pose tick moves the whole column, a layer tick moves one key
  constraints: snap to whole frames, clamp to ip..op, never cross a neighbouring key
  resort: deferred to release so drag indices stay valid, as the span chart does
operations:
  insert: a pose at the playhead, copying the one before it — a new pose starts as the one it follows and is then changed, so the insert leaves the clip looking as it did
  delete: the whole column; the last pose is kept, because a track with no keys is a different kind of clip rather than an empty one
  borrow: a pose filled from another clip's key, layers matched by name. A preset's clips share one rig, which is what makes them interchangeable
  swap: trade the pose between the rig's paired limbs, as a pose is inserted or on one already placed. Half a walk cycle is the other half with the legs traded, so the alternative is dialling the same pose in twice
  swap_bound: pairs come from the "-near"/"-far" naming rather than a list, so a rig that grows a slot is paired without being declared. Only properties keyed on both sides trade — a static attach point is rig spec, and trading it would detach the pair. Draw order is untouched; which limb is in front is its own edit
  navigate: jump to the previous or next key — the frames between poses hold nothing to edit
  ease: per pose, the whole column at once (editor/genpresets 0.4/0.6 against 0.5/0.5). A body arriving softly while its arm arrives linearly reads as a mistake
  length: the clip's out point, set together with the layers' own or every part would vanish at the old end; refuses to cut past the last pose
non_goals:
  - easing curve shapes beyond the generator's two; the pair covers what the presets use
  - inserting a key on one track alone, which would break the pose column
built:
  - one row, pose or layer, drawn with the span chart's gutter, ruler and transport
  - a tick is a diamond, or a square when every property keyed at that time holds; a pose mixing held and moving properties reads as moving, which is the honest reading
  - a row label click selects that row's first key, so a row becomes the edit target without hunting for a tick
  - a retime drag follows the key, not the cursor: clamping stops it one frame short of a neighbour, and the drag keeps hold of where it landed
```

Placed as a tab beside the collision groups in ui:collision-editing, so the
strip under the stage keeps one row set visible at a time. Retiming existing
poses is the narrow carve-out in policy:editor-out-of-scope; cutting new clips
out of a longer document stays excluded.
