---
id: requirement:pose-editing
type: requirement
title: Keyframe Pose Editing
---

Status: implemented — cmd/lottie-state-editor/pose.go, cmd/lottie-state-editor/poseui.go, the Pose pane in
cmd/lottie-state-editor/inspector.go. Adjust a rig's pose at a selected keyframe by dragging the
part on stage, instead of editing cmd/lottie-state-editor/genpresets Go source or raw JSON and
re-rendering to see the result. The commits behind requirement:animation-presets
("straighten the leading arm", "spend the whole pose on the swing's reach") are
each one number in one keyframe found by trial and re-render.

```yaml
depends: requirement:keyframe-timeline — a pose is selected before it is edited
edit_at:
  rule: decided — the selected keyframe only; the playhead parks on it
  off_key: scrubbing away clears the edit selection rather than writing an interpolated frame
  why: auto-keying at an arbitrary frame would break the one-time-set invariant every preset clip holds (concept:pose-sequence-clip)
park:
  - selecting a key switches the stage to that clip when a state machine was driving it. A machine keeps the Animation it decoded at start, so edits reached the bundle and never the picture; a parked playhead and a running machine are not compatible either. "Back to machine" returns
  - parking turns looping off. A clip's last key sits at its out point, and a looping player wraps that straight back to the start, so the final pose of every clip was the one pose that could neither be selected nor edited
  - pressing play ends the park and restores looping
onion_skin:
  what: the keys either side of the playhead drawn faintly under the current one, previous tinted cool and next warm
  why: how far a limb travels between two poses is most of what posing decides, and stepping back and forth to compare means holding the previous pose in your head
  scope: a display toggle beside autoplay; view state, not saved. Off while playing, where the bracketing pair changes several times a second
magnification:
  problem: fit-to-pane puts a chibi rig at about a hundred pixels and a forearm at a few, which is not a drag target
  answers: a stage zoom (wheel about the cursor, buttons about the pane centre, drag-to-pan, Fit to return) and a draggable graph/preview splitter — the two independent ways of making the picture bigger
  scope: view state, not document state; it is not saved and it applies to every tab, collision editing included
surfaces:
  stage_drag: drag a part to rotate it about its own ks.a anchor. The cursor is mapped into the parent's space with api:layer-placement on the parent layer, which already composes the chain, so the part follows the cursor
  precedent: the api:sockets cross drag inverts a resolved placement the same way to write a layer-local offset
  pick: click selects the topmost *visible* image layer whose quad covers the point — a part switched off by opacity is not on the stage, and letting one intercept clicks made the torso unpickable, since the rig stacks body-side and body-back in front of body; parts the stage cannot reach — behind another, or switched off by opacity — are picked from the Parts list instead (ui:keyframe-editing list.parts), which selects both ways
  numeric: the selected part's p / r / s / o at that key, edited in the inspector pane (requirement:selection-driven-ui)
  readout: the value at the key, not the interpolated value, so what is typed is what is stored
scope:
  props: ks.r, ks.p, ks.s, ks.o
  anchor: ks.a is rig spec, so nothing drags it by itself — but the joint drag in keeps-art mode moves it deliberately, paired with ks.p so the part stays put. That is the one edit where changing an anchor is the intent rather than an accident (skills/lottie-character-preset references/rig.md)
  layers: image layers of the stage clip; shape and text layers stay read-only
  names: a layer's name is editable, because it is the handle everything outside the clip uses — a socket binding, a cross-clip pose copy, the near/far pairing. Blanks and duplicates are refused; a socket left bound to the old name is reported rather than silently repointed
  name_guard: the stage refuses to drag a part whose name, or whose parent's name, is blank or duplicated. api:layer-placement resolves by name and takes the first match, so the failure would be a wrong matrix, not a missing one — plausible numbers written into the wrong space
  rig_overlay: a dot per visible joint and a bone where a chain continues, over the onion neighbours as well. It is the parent graph made visible, which is what makes re-parenting judgeable
  parent: what a part hangs from is editable, and the transform is restated in the new parent's terms so the part does not jump. A static member is corrected once and stays static — static means rigidly attached, which is still true of the new parent — and a keyed one is corrected at every key, since the carrier differs per frame. Position therefore matches only at the frame the link changed: two parents that move differently cannot both be followed by one point, which is the point of re-parenting
  cycles: the candidate list excludes the part's own descendants. A cycle is not a mistake to report, it is a thing the picker should not be able to say
  joint_drag: two readings, both wanted — the part follows its attach point (also how the character is moved, the body's joint being its position in the composition), or the artwork holds still and only the point it turns about moves (a' = a + L-inverse . dp, L being the part's own rotate-and-scale)
  order: the layer array is the draw order, and reordering it is how a rig's overlap is authored per clip. Parent links are by ind and resolve in a second pass, so the rig survives; track-matte clips refuse, because an implicit matte source is the layer before it
  visibility: opacity toggles per key, which is how a slot's alternate drawings (a head from another angle) take turns
  never: shape paths, gradients, part images (policy:editor-out-of-scope)
promotion: editing a property that is static across the clip materializes a key at every existing pose time holding the old value, then writes the new value at the selected time — the inverse of cmd/lottie-state-editor/genpresets track()
mirror: not authored. Facing flips stay rule:facing-mirror at runtime
verify: after each edit the clip is re-encoded and re-Decoded for the preview, so the stage shows what the renderer produces, never an editor-side approximation (decision:json-level-animation-edit)
undo: built, scoped to clip edits — a stack of encoded clips, one step per drag (BeginPoseEdit/EndPoseEdit collapse a swing), no-op edits dropped so nothing undoes nothing. It is a button in the pose pane rather than a shortcut: the editor has no keyboard input path at all. Machine-graph undo stays open in requirement:editor-mvp
deviations:
  - dragging anywhere inside the part rotates it about its joint; there is no separate rotation grip. The joint mark doubles as the move grip, mirroring how a collision shape carries one resize handle
  - the "park on a key first" hint lives in the inspector pane, not on the stage: a custom Draw has no text, and the pane is already where the rule bites
  - the pane names the cmd/lottie-state-editor/genpresets joint beside the layer (upper-arm-near -> arms(near)), which is the transcription aid the probe role asks for
  - core gained api:layer-placement LayerTransform: the decomposed placement folds a mirror into a half turn, so an outline drawn from it lands wrong on a flipped part
role:
  decided: for testdata/presets the editor is a probe, not the source of truth. cmd/lottie-state-editor/genpresets keeps authoring those clips; a pose found by dragging is transcribed back into its clips.go pose values by hand
  consequence_readout: the numeric field is the deliverable, not a convenience. Show rotation in degrees, the unit the generator's pose fields already use, so a value reads straight across
  consequence_naming: name the joint beside the layer where a rig slot maps to a pose field (upper-arm-near -> armN), so transcription does not need the rig table open
  consequence_save: saving over an in-repo preset writes work the next generator run discards; warn on save when the bundle path is under testdata/presets
  not_doing: emitting Go literals the generator can paste — the mapping would have to track clips.go, and hand transcription is cheap at 2..7 poses per clip
  unaffected: a bundle copied into a game has no generator behind it, so edits there are simply the source of truth
open:
  - IK or FK only: dragging a hand today rotates the hand. Rotating the chain to reach a point is a second feature, and the two-segment chains of requirement:animation-presets make it tractable
  - whether a pose column can be copied, mirrored or interpolated between neighbours (walk cycles want all three)
```

Serves requirement:animation-presets authoring and decision:ai-skills-workflow:
an agent editing a preset can be watched live through the existing
`-viewer` reload, and a human can correct the result without a round trip
through Go source.
