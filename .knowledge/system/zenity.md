---
id: system:zenity
type: system
title: Zenity
---

Cross-platform native file dialogs for Go, used by requirement:editor-mvp for open, save, and clip import.

```yaml
reference: https://github.com/ncruces/zenity
backends: macOS osascript, Windows Win32, Linux zenity binary
properties:
  - pure Go, no cgo (consistent with decision:no-cgo)
  - only the editor module depends on it; the library does not
constraints:
  - dialogs block until dismissed; run them off the render goroutine
  - cancel returns zenity.ErrCanceled and is a normal outcome, not a failure
```

The editor runs a dialog on a goroutine and applies the result from the guigui Tick, which keeps the window drawing and leaves every model field owned by the main goroutine (system:guigui).
