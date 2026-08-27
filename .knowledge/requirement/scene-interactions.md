---
id: requirement:scene-interactions
type: requirement
title: Scene Interaction Bindings
---

Per-node bindings from semantic UI events to playback and game callbacks. Authored in vision:scene-editor, stored in data:scene-document, executed by api:scene-runtime. Status: implemented.

```yaml
events: # semantic, source-agnostic: click and confirm button both mean activate
  - focus / blur: focus arrived / left (requirement:scene-focus-navigation)
  - hover / unhover: pointer over the node; fires even when hover does not move focus
  - press: confirm button or mouse button went down on the focused node
  - activate: confirm completed — click released on the node, or confirm button pressed while focused
  - cancel: cancel button; delivered to the focused node, else to the scene
  - complete: an animation node's clip finished a pass (each playback.then chain link reports) — the intro-ended hook
actions: # several per event allowed, e.g. visual + callback
  - fireEvent: fire an input event on a machine (api:state-machine-runtime Fire)
  - playSegment: play a marker segment; empty arg plays the whole clip
  - callback: report {node, name} to the game; the game implements behavior
  - focus: move focus to the node arg names
  - phase: enter the phase arg names (requirement:scene-phases)
target: fireEvent and playSegment take an optional target node (empty = the bound node); acting on a node whose start time (requirement:scene-timeline) has not come starts it — an event-driven entrance
defaults:
  - machine node with no binding for an event auto-fires an input event of the same name when its machine declares one (focus, blur, press, activate, cancel)
  - explicit bindings replace the default for that event
hit_test:
  decided: activate/hover hit region is the node's transformed animation viewport box; finer shapes may reuse collision plugin geometry (api:physics-cp) later
```

Visual reaction lives in the state machine (a button's normal/focused/pressed states are just states), so bindings mostly carry events in and callbacks out rather than describing animation.
