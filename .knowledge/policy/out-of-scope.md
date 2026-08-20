---
id: policy:out-of-scope
type: policy
title: Permanently Out of Scope
---

Never implement:

```yaml
excluded:
  - expressions (most Lottie players also lack these)
  - AE effects: ef
  - camera layer: "ty: 13"
  - 3d layers
  - complex time remapping cases
```

Encountering these must not break rendering (policy:robustness).
