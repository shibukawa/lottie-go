---
id: decision:scene-references-bundles
type: decision
title: Scene File References Bundles
---

data:scene-document is a standalone JSON file that references bundles, not a member stored inside one.

```yaml
decision: standalone scene file; bundles listed by alias + relative path; nodes name {bundle alias, id}
because:
  - a scene composes actors from several bundles; no single bundle owns it
  - a bundle stays a reusable actor asset (vision:state-machine-editor output) untouched by layout work
  - scene edits never rewrite archives; cheap diffs, cheap saves
consequences:
  - loading needs a bundle resolver (api:scene-runtime); games embed both scene and bundles (go:embed friendly)
  - broken references are a validation case, not a parse error (policy:robustness)
```
