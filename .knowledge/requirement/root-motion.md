---
id: requirement:root-motion
type: requirement
title: Root Motion
---

Status: proposed. World displacement of the character driven by the animation itself — lunging attacks, dodge rolls — instead of game-side velocity.

```yaml
api_shape: cumulative displacement since clip start; game diffs between Updates
  reason: survives speed changes, seeks, loops (consume-and-reset breaks on rewind)
loop_semantics: on wrap, game applies (end displacement - last read) then re-bases
source: a root socket's translation (requirement:socket-track layer binding) — no separate authoring
mirroring: x negates per rule:facing-mirror
```

Follows requirement:socket-track; nearly free once sockets exist.
