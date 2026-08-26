---
id: requirement:state-windows
type: requirement
title: Timed State Windows
---

Status: implemented inside data:physics-resolv-track as kind "window" (first candidate won: reuses editor spans, tags, timeline). Geometry-less tagged frame spans: pure time flags a game reads by tag, same span semantics as data:physics-resolv-track ([from, to), step).

```yaml
uses: [cancelable, invincible, super-armor, throw-immune, counter-stance]
why_not_hitboxes:
  - invincibility can be "hurtbox absent", but counter/cancel windows have no geometry at all
shape: {name, tags, spans: [{from, to}]}; query by frame + tag like track.At
implementation_candidates:
  - boxes without geometry inside the resolv track (reuses editor spans + timeline UI)
  - separate window track document
cost: cheapest of the gameplay annotations; high fighting-game value
```

Serves the same authoring flow as requirement:collision-authoring (ui:collision-editing span row).
