---
id: api:physics-resolv
type: api
title: resolv Plugin API (lottieresolv)
---

Package lottieresolv: typed IO and queries for data:physics-resolv-track plus a SolarLune/resolv mirror. Import-gated (decision:collision-static-plugins). Implemented.

```yaml
bundle_io:
  - "lottieresolv.IDs(b) []string"
  - "lottieresolv.Load(b, animID) (*Track, error)"
  - "lottieresolv.Store(b, animID, *Track) error"
  - "lottieresolv.Remove(b, animID)"
query:
  - "track.At(frame, tags...) []ActiveBox  // live rects+circles; tags filter any-of; none = all"
  - "track.WindowsAt(frame, tags...) / track.Open(frame, tag)  // requirement:state-windows flags"
  - "ActiveBox.Mirrored(axis)  // rule:facing-mirror; windows untouched"
  - "box.SpanAt(frame) (int, bool)"
  - "ActiveBox.Index points back into track.Boxes"
space_mirror:
  - "lottieresolv.NewTracker(space, track) *Tracker"
  - "tracker.SetOffset(x, y)     // character world position"
  - "tracker.Sync(frame)         // per Update after player.Update; inserts/moves/removes shapes"
  - "tracker.Shapes(tags...) []resolv.IShape"
  - "tracker.Remove()            // leave play; next Sync re-inserts"
  - "lottieresolv.Tag(name) resolv.Tags  // dedup: same name, same bit (resolv has 64 total)"
sync_semantics:
  windows: kind "window" (and unknown kinds) never enter the space
  rebuild: only when a box steps to another span; offset moves slide shapes
  shape_data: BoxData{Tracker, Index, Name} for collision callbacks
```

Serves requirement:collision-authoring acceptance "query by frame and tag".
