---
id: policy:robustness
type: policy
title: Robustness Policy
---

```yaml
rules:
  - unsupported feature: never panic; skip element and continue rendering
  - debug mode enumerates unsupported features encountered
    (api:player-api UnsupportedFeatures)
  - malformed JSON returns error; must not crash host game
```

Verified by fallback tests in requirement:verification.
