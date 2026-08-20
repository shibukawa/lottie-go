---
id: policy:risks
type: policy
title: Risk Register
---

```yaml
risks:
  - risk: CPU cost of path-deforming animation
    impact: high
    mitigation: restrict to UI scale; cache static paths (policy:performance-caching)
  - risk: JSON dialects across editors
    impact: medium
    mitigation: cross-test 4 editors (system:lottie-editors); permissive parser
  - risk: breadth of Lottie spec
    impact: medium
    mitigation: hold phase boundaries; P2 only on demand (decision:practical-subset)
  - risk: gradient implementation complexity
    impact: medium
    mitigation: exclude from P0; solid colors until requirement:phase-p1
```
