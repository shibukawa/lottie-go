---
id: decision:texture-weave
type: decision
title: Texture Data Stored Aside, Edited Inline
---

User decision 2026-09-02: data:texture-document lives under extensions/
like every other lottie-go addition, so the clip JSON stays pure Lottie.
The editor nonetheless edits it as if it sat inside the clip — woven into
the tree on load, stripped back out on every store-back.

```yaml
storage: extensions/texture/<animID>.json, per clip, the data:physics-resolv-track precedent; the clip JSON carries nothing
working: each entry attached to its item in the editor's tree as an `x-` member (rule:x-member-namespace) — x-tex on the fl/st, x-uv on the sh — the shape the data has while a human moves vertices
weave: on load, resolve every entry's address and attach it; an entry whose address no longer resolves stays on the doc, is reported, and is written back unchanged
unweave: on store-back, walk the tree, strip every `x-` member into a fresh doc with addresses computed from the tree as it is now, then SetAnimation the pure clip and Store the doc. Addresses are never edited, only regenerated
why_not_pure_inline: the clip would stop being Lottie; a tool re-saving it may strip the members, and the file would advertise a member no spec defines (proposed first, rejected 2026-09-02)
why_not_pure_aside: UV must stay length-locked to v[] through vertex insert and delete, and follow items through reorder, copy, paste and clip duplication. Edited in a second document, every one of those operations learns a second bookkeeping; woven, the existing tree operations carry it for free
what_it_buys:
  - addressing drift cannot happen — an address is only ever computed at store-back from the current structure
  - the same Unweave feeds the preview — encode the tree, Unweave, Decode the pure clip, Apply the doc (api:texture-binding). One code path, exercised on every edit
  - a foreign player sees a plain clip with solid fills; a foreign tool re-saving the clip loses nothing the clip owned
costs:
  - textures are bundle-borne; a standalone JSON gets them only through the runtime API
  - the core cannot read the doc (decision:collision-static-plugins), so a plugin owns the file and the core exposes typed paint hooks
  - clip rename, duplicate and delete must carry the doc, exactly as they carry the resolv track today
```

Serves requirement:texture-mapping; the editing surface is ui:texture-mapping.
