---
id: api:bundle-extension-files
type: api
title: Bundle Extension File API
---

Core-side raw access to the bundle's extensions/ subtree. The core gives these files no meaning (decision:collision-static-plugins); plugins build typed IO on top. Implemented.

```yaml
bundle:
  - "b.ExtensionFile(name string) ([]byte, bool)"
  - "b.SetExtensionFile(name string, data []byte) error  // must live under extensions/; bytes cloned"
  - "b.RemoveExtensionFile(name string)"
  - "b.ExtensionFiles(prefix string) []string  // sorted; empty prefix lists all"
encode: every stored member written verbatim; foreign tools' payloads survive (policy:robustness)
decode: everything under extensions/ captured, no parsing
json_helpers:
  - "lottie.MarshalWithExtra(v any, extra ExtraFields) ([]byte, error)"
  - "lottie.UnmarshalExtra(data []byte, v any) (ExtraFields, error)"
  purpose: plugin document types round-trip unknown members like core types do
```

Not in data:bundle-layout's required set; RemoveAnimation leaves extension files alone (cleanup is the writer's job).
