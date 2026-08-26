---
id: api:frame-events
type: api
title: Frame Events Plugin API (lottieevents)
---

Package plugin/events (core subpackage, zero deps): payload-carrying cues
serving requirement:frame-events. Implemented.

```yaml
storage: extensions/events/<animID>.json
event: {frame, name, payload: free JSON (sound id, volume, shake magnitude)}
bundle_io:
  - "lottieevents.IDs(b) / Load(b, animID) / Store(b, animID, *Track) / Remove(b, animID)"
query:
  - "track.In(from, to) []Event  // half-open [from, to), frame order"
emission:
  - "lottieevents.Cue(player, track, fn)  // fires through playback: loops, reverse, marker-identical crossing"
  - claims the player's single OnFrameSpan slot; compose manually if the game needs its own span handler
core_hook:
  - "player.OnFrameSpan(func(from, to))  // every span Update sweeps, wrap partials included; seeks do not sweep"
editor: no authoring UI yet; tracks are hand-authored or written by tools
```

Markers (api:player-api) stay the zero-setup path; this track exists for
payloads and repeated same-name cues.
