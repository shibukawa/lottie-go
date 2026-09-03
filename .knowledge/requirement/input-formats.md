---
id: requirement:input-formats
type: requirement
title: Input Formats
---

```yaml
formats:
  - format: lottie-json (.json)
    priority: P0
    note: raw JSON; support first
  - format: dotlottie v1 (.lottie)
    priority: P1
    note: ZIP; animations/ and images/; implemented
  - format: dotlottie v2 (.lottie)
    priority: P1
    note: ZIP; a/ i/ s/ t/ f/; implemented, read and write (data:bundle-layout)
  - format: spine (.json + .atlas)
    priority: P2
    note: import only, baked into a bundle by cmd/lottierepack (requirement:spine-import); not a runtime format
unsupported:
  - externally referenced assets (URL-fetched images)
```

Parser must stay permissive across editor dialects (system:lottie-editors, policy:risks).

State machines in `s/` were previously excluded here on the grounds that the game owns state management. That exclusion is withdrawn: they are now in scope via decision:align-dotlottie-state-machine and requirement:player-state-machine.
