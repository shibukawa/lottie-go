---
id: requirement:scene-phases
type: requirement
title: Scene Phases
---

Named stretches of a scene's life — a startup animation, the interactive
screen, an outro — switched by time, by input, or by the game. The
scene-level counterpart of a state machine's states. Stored in
data:scene-document, run by api:scene-runtime, edited in ui:layout-shell.
Status: implemented.

```yaml
model:
  - phase: {name, duration: seconds (0 = until switched), next: phase entered when duration elapses}
  - the first listed phase is where the scene starts
  - node.phase: participates only while that phase runs, entering afresh each entry; empty joins every phase and keeps playing across switches (a background loop must not pop)
  - switching restarts the scene clock, so node Start times (requirement:scene-timeline) count from phase entry
  - focus survives when its node still participates, else moves to the initial choice among what remains
switching:
  - automatic: duration elapses -> OnPhaseEnd fires, then Next is entered (intro rolling into main)
  - by binding: the phase action (requirement:scene-interactions) — a cancel button rolling the outro
  - by the game: sp.SetPhase; re-entering the running phase replays it
  - a timed phase with no next stays and reports OnPhaseEnd once — how the game learns its outro finished
editor:
  - phases list + form (name, duration, then) in the scene pane; delete releases member nodes
  - timeline and canvas show one phase at a time via the timeline's phase selector, following automatic advances
  - placing assigns the viewed phase; the node form reassigns
```

An intro/outro is a phase, not a separate scene file: the phases share
nodes, assets, and the focus model, and the file stays one screen.
