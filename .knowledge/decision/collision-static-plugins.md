---
id: decision:collision-static-plugins
type: decision
title: Collision Data as Static Plugins
---

Collision support is import-gated: the core stays physics-blind and dependency-free (decision:no-cgo keeps only Ebitengine), and meaning comes from plugin modules a program imports only when it wants the data.

```yaml
core:
  knows: extensions/ members as opaque bytes only
  api: api:bundle-extension-files
  guarantee: unknown extension payloads survive any rewrite (policy:robustness)
plugins:
  - module: github.com/shibukawa/lottie-go/plugin/physics/cp
    package: lottiecp
    engine_dep: jakecoffman/cp/v2
  - module: github.com/shibukawa/lottie-go/plugin/physics/resolv
    package: lottieresolv
    engine_dep: solarlune/resolv
  each_bundles: [data schema, bundle read/write, engine wiring]
  isolation: separate go.mod per plugin; plain lottie-go import never fetches or links engines
naming: directories and payloads named after the engine they feed, not a neutral schema
consequences:
  - editor imports both plugins to author their data; caches parsed docs (overlay reads per frame)
  - clip deletion cleanup moved core -> editor (core cannot map anim to extension file)
  - editor go.mod: replace directives until plugins get their first tag
  - core exports MarshalWithExtra / UnmarshalExtra so plugin types keep unknown JSON members
rejected:
  - init()-registration hooks: unnecessary; plugins are plain helpers over the generic byte API
  - single-module packages: would put engine requires in the core go.mod
```

Serves requirement:collision-authoring.
