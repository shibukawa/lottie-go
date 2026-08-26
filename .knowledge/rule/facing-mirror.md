---
id: rule:facing-mirror
type: rule
title: Facing Mirror Convention
---

Status: proposed. One shared convention for a left-facing character, offered by every gameplay-annotation query so games do not each reinvent flipping.

```yaml
mirror:
  position: x -> width - x (or negate around a chosen axis; axis = animation center by default)
  angle: negate
  hitboxes: rect x mirrors from its far edge; circle center mirrors
  z_hint: unchanged (front stays front)
applies_to: [requirement:socket-track, requirement:root-motion, api:physics-resolv queries, api:physics-cp shapes]
form: query-time option (facing parameter), not stored data — data stays authored right-facing
```

Constraint on the runtime APIs, decided once so all annotation queries agree.
