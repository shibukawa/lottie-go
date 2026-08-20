---
id: metric:performance-targets
type: metric
title: Performance Targets
---

```yaml
targets:
  concurrent_playback: 5 UI-scale animations at sustained 60fps
  draw_time_per_frame: "< 2ms"
  parse_time_typical_ui_animation: "< 10ms"
```

Constraint: triangulation runs on CPU; per-frame path deformation is expensive. Mitigations: policy:performance-caching. Regression measurement: requirement:verification.
