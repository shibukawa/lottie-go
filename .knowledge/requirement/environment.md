---
id: requirement:environment
type: requirement
title: Build Environment
---

```yaml
environment:
  go: ">= 1.26"
  ebitengine: ">= 2.9 (mandatory, see decision:use-vector-package)"
  dependencies: stdlib + Ebitengine only; cgo prohibited (decision:no-cgo)
  platforms: [windows, macos, linux, wasm, ios, android]
```
