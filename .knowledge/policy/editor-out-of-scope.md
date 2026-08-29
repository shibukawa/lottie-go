---
id: policy:editor-out-of-scope
type: policy
title: Editor Out of Scope
---

vision:state-machine-editor will not do these.

```yaml
excluded:
  - authoring or editing Lottie artwork — shapes, paths, gradients, part images; use system:lottie-editors
  - cutting new clips out of a longer document, or splitting one into several; import them already cut
  - theme and font editing
  - web interaction authoring (pointer types deferred in decision:game-oriented-sm-subset)
  - runtime scripting or expressions (policy:out-of-scope)
carved_out:
  transform_keyframes:
    allow: requirement:pose-editing — moving a limb at an existing keyframe
    why: requirement:animation-presets makes motion and art independent axes on purpose ("swap all images and every clip still plays; retune keyframes and the art is untouched"). Rotating a part is not drawing one, and the preset library is not usable without a way to tune poses
    bound: transform properties of image layers only; shape and text layers stay read-only
  compositing:
    allow: layer draw order and per-key opacity on an existing rig (requirement:pose-editing)
    why: which limb is in front is motion, not drawing — the same independence requirement:animation-presets rests on. No pixel changes
    bound: reordering only; adding, removing or retyping layers stays out
  retiming:
    allow: moving keyframe times, adding and removing poses, and setting a clip's own length (requirement:keyframe-timeline operations)
    why: a pose sequence with no way to add a pose is a set of numbers to tune, not something to author. Length follows, because a clip that gained a pose usually needs room for it
    bound: not splitting one clip into several, and not cutting a clip shorter than its own last pose — deleting that pose says plainly what is being lost
```

The editor is a bundler, a graph authoring surface and — for rigs it can
read — a pose tuner. It is not a drawing tool.
