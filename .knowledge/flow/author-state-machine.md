---
id: flow:author-state-machine
type: flow
title: Author a State Machine
---

```yaml
flow:
  trigger: author opens or creates a bundle in ui:editor-shell
  steps:
    - id: load
      action: read data:bundle-layout; retain unknown fields for round-trip
    - id: import
      action: add clips under a/; assign stable ids
    - id: declare-inputs
      action: define Event inputs named for game verbs (walk, run, jump)
    - id: build-graph
      action: add one PlaybackState per clip; set initial; add GlobalState for any-state transitions
    - id: wire
      action: add transitions with Event guards; order them; add OnComplete returns for one-shot clips
    - id: preview
      action: run api:state-machine-runtime and fire triggers manually
    - id: validate
      action: report missing initial, dangling toState, unknown animation, unreachable state
    - id: save
      output: dotLottie v2 archive
  failure:
    invalid_graph: warn but still save; the runtime must tolerate it (policy:robustness)
```

The game then loads the archive and calls sm.Fire with the same input names the author declared.
