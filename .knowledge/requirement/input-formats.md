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
  - format: dotlottie (.lottie)
    priority: P1
    note: ZIP archive; unpack via archive/zip; read manifest.json;
      may contain multiple animations, embedded themes, embedded images
unsupported:
  - dotlottie state machines (game owns state management)
  - externally referenced assets (URL-fetched images)
```

Parser must stay permissive across editor dialects (system:lottie-editors, policy:risks).
