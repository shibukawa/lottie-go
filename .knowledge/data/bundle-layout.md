---
id: data:bundle-layout
type: data
title: dotLottie v2 Bundle Layout
---

ZIP archive the editor reads and writes (decision:align-dotlottie-state-machine).

```yaml
layout:
  manifest.json: required; lists animations, state machines, themes
  a/: required; one Lottie JSON per clip, named by id
  i/: optional; shared images
  s/: optional; one JSON per data:state-machine
  t/: optional; themes
  f/: optional; fonts
  extensions/: optional; tool-specific payloads, opaque to the core (api:bundle-extension-files); physics subtrees defined by data:physics-cp-body and data:physics-resolv-track, texture/ by data:texture-document
v1_difference:
  animations/: v1 name for a/
  images/: v1 name for i/
  status: both layouts read; v2 always written
clip_granularity:
  one_clip_per_animation:
    preferred: true
    note: each PlaybackState names an animation id
  segments_in_one_animation:
    note: PlaybackState.segment names a marker range inside one animation
    status: markers parse and Player limits playback to one (requirement:player-state-machine)
```

Editor loads v1 and v2, always writes v2 (requirement:input-formats).
