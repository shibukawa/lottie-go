---
id: decision:no-cgo
type: decision
title: No cgo
---

Forbid cgo and C library bindings. Preserves Ebitengine advantages: no C compiler on Windows, unchanged WASM and mobile builds. ThorVG binding explicitly rejected. Dependencies limited to Go stdlib + system:ebitengine.
