---
id: ui:keyframe-editing
type: ui
title: Keyframe and Pose Editing UI
---

Where requirement:keyframe-timeline and requirement:pose-editing sit inside
ui:editor-shell's preview pane. Extends the strip that ui:collision-editing
already owns rather than adding a pane. Implemented.

```yaml
ui:
  tab:
    kind: tab
    id: tab.poses
    parent: tabs.collision
    state: joins Segment / Hitbox / Body / Sockets; the active tab decides which overlay the stage draws, so pose handles and hitboxes never compete for a drag
    state: the autoplay toggle above the tab bar reaches this tab too, which matters here — posing wants the stage to stay still, so arriving paused is the useful default while tuning
  chart:
    kind: canvas
    id: chart.keys
    state: pose row with one tick per pose time, or one row per animated layer when key times disagree
    state: hold keys square, interpolated keys round; selected tick filled
    state: shares chartGutter and the plot extent of chart.spans, so a frame is at one x on every ruler
    action: tick click selects, tick drag retimes, empty-row drag scrubs, any touch pauses
    action: transport (play/pause, +-1 frame) reuses the gutter buttons
  footer:
    kind: buttons
    id: row.poses
    columns: ["+Pose", Delete, "|<", ">|", Swap, length, "swap near/far", "copy from <clip> <key> Insert"]
    state: two rows — the pose operations and the clip's length on the first, borrowing a pose from another clip on the second, which needs two pickers and does not share
    action: +Pose and Insert are disabled on a frame that already holds a pose; Delete refuses the last one
    state: the swap near/far checkbox rides on both inserts rather than being a third button, because trading limbs is a property of the copy being made; Swap applies the same to a pose already placed
  rig:
    kind: canvas-overlay
    id: stage.rig
    state: a dot per visible joint and a bone wherever a chain continues — drawn only where a parent has exactly one child, so a hub like the torso reads as its joint rather than as a starburst of five
    state: toggled beside onion skin. The onion neighbours get joints only, no bones: three skeletons of lines over one drawing is a thicket, and the dots alone still answer where a joint was
  overlay:
    kind: canvas-overlay
    id: stage.pose
    state: the selected part outlined and its ks.a anchor marked; the outline comes from api:layer-placement LayerTransform, so a mirrored part is outlined mirrored
    state: shown while tab.poses is active; solid once a keyframe is selected and the playhead is parked on it, faint otherwise
    action: click picks the topmost image layer whose quad covers the point; dragging inside the outline swings the part about its joint; dragging the joint mark moves ks.p
    state: no rotation grip — the part is the grip. A rig is a skeleton of rotations, so the common gesture gets the whole target and the joint mark is left to the rarer move
  parts:
    kind: list
    id: list.parts
    columns: [part, joint-or-hidden]
    state: every image layer of the stage clip, front to back; the row note is the cmd/lottie-state-editor/genpresets joint, or "hidden" for a part switched off by opacity
    state: selection is shared with the stage both ways — a row click picks the part, a stage pick highlights and scrolls to the row
    why: a rig layers parts over each other and switches others off, so a stage click can only reach what is on top and visible, which are the parts needing posing least
    action: choosing the Poses tab opens this pane, rather than waiting for something to be selected
    action: rows drag to reorder, and a Front/Back footer pair does the same one place at a time; the list order is the draw order, so this is how the overlap is authored
    action: a Hide/Show footer button flips the selected part's opacity at the current key, which is how a slot's alternate drawings take turns
    state: reordering is refused entirely on a clip using track mattes — a matte without an explicit source takes the layer before it, so the array means more than draw order there
  inspector:
    kind: form
    id: form.pose
    columns: [part, joint, parent, joint drag, frame, ease, r, p(x,y), s(x,y), o, a(x,y)]
    state: parent is a picker whose candidates exclude the part's own descendants, so a cycle cannot be named; choosing one rewrites the transform so the part does not move
    state: joint drag picks whether the joint mark carries the part or slides under it
    state: part is the layer's name and is editable; blanks and duplicates are refused, and a socket still bound to the old name is reported
    state: joint is a picker, not a readout — it names the parts as the rig does (arms(near) rather than upper-arm-near), so a pose written in the generator's vocabulary can be followed without translating it. Parts with no joint list by layer name, so it is never a dead end
    state: ease is a picker over the two curves cmd/lottie-state-editor/genpresets writes, and applies to the whole pose column
    state: values are the ones stored at the key, not interpolated; a static property shows its value with a Keyframe button that promotes it
    state: rotation in degrees, and the joint row names the cmd/lottie-state-editor/genpresets pose field where a rig slot maps to one — this pane is how a probed pose is read back out (requirement:pose-editing role)
    state: a hint line carries the rule the stage cannot show — an edit needs a key under the playhead — and names what is missing when nothing is editable
    action: an Undo button takes back the last clip edit, one step per drag
    state: replaces the fixed panel per requirement:selection-driven-ui when a pose or key is selected
forms:
  tab: Tab walks a form's text fields and Shift-Tab walks back, stepping over disabled ones. Nothing in guigui claims Tab, so a field would otherwise swallow it and the mouse would be the only way out. A text input commits on losing focus, so tabbing out is also how the value lands
guigui_mapping:
  chart.keys: custom widget beside chart.go; same plotRect and frameAt helpers
  stage.pose: previewStage overlay; two more stageDragKinds (part swing, joint move) next to hitbox, body and socket
  form.pose: basicwidget.Form, numeric rows via inputrow.go
  tab order: TextInput.OnHandleButtonInput plus Context.SetFocused, the idiom guigui's own examples use
  list.parts: basicwidget.List; EnsureItemVisibleByIndex only when the stage moved the selection, or it would fight a hand-scrolled list
screenshots: the harness in cmd/lottie-state-editor/screenshot.go takes a setup string (clip, tab, key, part), because a pane that only appears under a selection cannot be photographed from a cold start
verified_carried_over:
  - a custom widget must override Measure or the default box clips its Draw
  - a per-frame readout must be written from Tick as well as Build
  - build only the visible tab; an unadded widget is not laid out, drawn or sent input
```

Selection stays exclusive: a pose part, a hitbox and a body shape cannot be
dragged at once, which the active-tab rule already enforces.
