---
id: data:clip-edit-document
type: data
title: Editable Clip Document
---

The editor-side mutable form of one clip, per decision:json-level-animation-edit.
Backs requirement:keyframe-timeline and requirement:pose-editing.
Implemented in cmd/lottie-state-editor/clipdoc.go.

```yaml
holds:
  tree: the decoded map form of Bundle.AnimationJSON, unmodeled members intact
  layers: per layer, {index, nm, ind, parent, ty, refId, ks.a} resolved once at load
  chains: parent links flattened to a root path per layer, so a stage drag converts to the layer's own space
  keys: per animated property, its time list; their union is the pose set while concept:pose-sequence-clip holds
addressing: a key is (layer index, property name, key index); indices stay valid because the editor owns the tree and never reorders layers
ordering: rewriting emits members key-sorted, which is the order cmd/lottie-state-editor/genpresets already produces, so an edited preset diffs only where a value changed
numbers: decoded as json.Number, so untouched values re-emit byte for byte; indentation is read off the source and reproduced. An untouched preset clip round-trips identically, which is asserted
promotion: keying a static property writes the old value at every pose time first, so the clip stays a pose sequence — the inverse of cmd/lottie-state-editor/genpresets track() collapsing an unchanging track
dirty: an edit counter separate from the machine-preview generation — ui:editor-shell records that selecting something must not report the preview as edited
lifecycle: built when a clip becomes the stage clip, dropped on reload, stored back through Bundle.SetAnimation on every edit
shapes: implemented in cmd/lottie-state-editor/shapedoc.go — a shape layer's nested gr/item tree is addressed as (layer index, item path, member, key), unknown item kinds carried inert, and the layer's shape key times join the pose columns (insert, delete, retime, ease). Path keys carry the topology constraint the flat transform tracks never had: every key of one path property keeps the same vertex count and closure, so vertex insert/delete rewrites every key at once
```

Not a new file format: the document is the clip, so data:bundle-layout is
unchanged and a bundle saved after pose editing stays readable by any
dotLottie runtime.
