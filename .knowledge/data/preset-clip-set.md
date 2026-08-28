---
id: data:preset-clip-set
type: data
title: Preset Standard Clip Set
---

Standard clip vocabulary every preset in requirement:animation-presets ships. Names follow the editor's `-anim` suffix convention (data:state-machine namespaces). Kinds are proposed defaults: loop = repeats until input; oneshot = plays once then transitions; transition = short bridge between two loops.

```yaml
clips:
  - {name: idle,       kind: loop, note: breathe downward via knee flex with feet planted; an upward bob reads as floating}
  - {name: idle-turn,  kind: transition}
  - {name: walk,       kind: loop}
  - {name: walk-turn,  kind: transition}
  - {name: run,        kind: loop}
  - {name: run-turn,   kind: transition}
  - {name: run-to-idle,kind: transition, note: braking stop}
  - {name: slide,      kind: oneshot}
  - {name: jump,       kind: oneshot, note: launch; chains to fall}
  - {name: fall,       kind: transition, note: entry into fall-loop}
  - {name: fall-loop,  kind: loop}
  - {name: hurt,       kind: oneshot}
  - {name: death,      kind: oneshot, note: holds last frame}
facing: authored facing right; left via runtime mirror (Mirrored/MirrorX), no *-left clips
turns: two rotation styles, never morphs. idle/run-turn rotate through the REAR: side-view drawings (head-side one-eye, body-side thinned) cut on hold keys, limbs trade sides at midpoint. walk-turn rotates through the CAMERA side: front drawings only, slight scale-up passing the viewer, mirror at a SELF-MIRROR midpoint pose (limbs straight, joints ~0 - a bent joint carried across the flip kinks backwards for the new facing) so geometry holds while near/far identities trade; runs contact-pose -> mirrored contact for seamless reversed gait (user feedback 2026-08). Clips END facing opposite; game flips Mirrored on completion
no-dash: dropped - dash is run played faster (state speed / SetSpeed); a separate clip duplicated run (user feedback 2026-08)
air: airborne poses (jump/fall/jump-kick) lead with the far-side arms AND legs; near side trails (user feedback 2026-08)
attacks: per-kind sets in data:preset-combat-clips
open:
  - land clip (fall-loop -> idle) absent; decide whether run-to-idle covers it
```
