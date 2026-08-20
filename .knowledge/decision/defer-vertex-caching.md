---
id: decision:defer-vertex-caching
type: decision
title: Defer Static-Path Vertex Caching
---

Do not implement the vertex/index caching and static-flag mitigations from
policy:performance-caching until measurements demand it. Buffer pooling +
Path.Reset() alone meets metric:performance-targets with wide margin.

```yaml
measured: 2026-08-20, Apple M3, examples/stress (CC0 mixed workload:
  precomp, gradient, trim, matte)
results:
  - players: 5
    p99_draw_ms: 0.98
  - players: 20
    p99_draw_ms: 1.41
  - players: 40
    p99_draw_ms: 1.14 (60fps sustained)
target: p99 < 2ms at 5 players
revisit_when: real game assets exceed the 2ms budget
```

Consistent with decision:practical-subset (build on demonstrated need).
