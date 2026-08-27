---
id: data:preset-clip-set
type: data
title: Preset Standard Clip Set
---

Standard clip vocabulary every preset in requirement:animation-presets ships. Names follow the editor's `-anim` suffix convention (data:state-machine namespaces). Kinds are proposed defaults: loop = repeats until input; oneshot = plays once then transitions; transition = short bridge between two loops.

```yaml
clips:
  - {name: idle,       kind: loop}
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
turns: real turns, not morphs - limb chains trade sides + head/shoes x-mirror on hold keys at midpoint; clip ENDS facing opposite, game flips Mirrored on completion (user feedback 2026-08)
no-dash: dropped - dash is run played faster (state speed / SetSpeed); a separate clip duplicated run (user feedback 2026-08)
attacks: per-kind sets in data:preset-combat-clips
open:
  - land clip (fall-loop -> idle) absent; decide whether run-to-idle covers it
```
