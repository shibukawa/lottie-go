---
id: rule:x-member-namespace
type: rule
title: x- Members Live Only in the Working Tree
---

Members lottie-go adds to a clip while editing it use JSON keys prefixed
`x-`, and never reach a stored clip. They are the working form of data
stored elsewhere (decision:texture-weave); Unweave strips every one of them
before the clip is written or decoded.

```yaml
why_a_prefix:
  - the editor's tree (data:clip-edit-document) mixes real Lottie members with working members; a collision would overwrite a real one
  - no Lottie key contains a hyphen, so `x-` cannot collide; bare "x" is taken (rawProp.x is an expression or a split-position sub-property)
  - one prefix lets Unweave strip generically — any `x-` member, not a list it must keep current
never_stored: a saved clip containing an `x-` member is a bug; the untouched round-trip assertion (data:clip-edit-document) and a strip check after every store-back catch it
never_decoded: the core's Decode ignores unknown members, so a woven clip that leaks through renders as plain fills — harmless, still a bug, since paint must arrive through api:texture-binding
scope: clip JSON in memory only; the stored form is a document under extensions/ (api:bundle-extension-files)
current: x-tex on fl/st and x-uv on sh, the woven form of data:texture-document
```
