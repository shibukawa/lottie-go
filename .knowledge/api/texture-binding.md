---
id: api:texture-binding
type: api
title: Texture Binding API (lottietexture)
---

Two layers, split at the decision:collision-static-plugins boundary: the
core renders textured paint and exposes typed hooks; package plugin/texture
(lottietexture, zero deps, a core subpackage like api:sockets) owns
data:texture-document and its file. Extends api:player-api. Implemented
2026-09-02 (texture.go, plugin/texture).

```yaml
core:
  types:
    - "ShapeRef{Asset string; Layer int; Item []int}  // the data:texture-document address; String() for messages"
    - "TexturePaint{Texture string; Mapping TextureMapping; Wrap TextureWrap; Filter TextureFilter; Tint bool; Transform json.RawMessage}  // Transform holds a Lottie transform object (p s r a), parsed by the core's own builder; zero Tint means untinted"
    - "MappingBBox | MappingVertex | MappingStroke, WrapClamp | WrapRepeat | WrapMirror, FilterLinear | FilterNearest"
  player:
    - "p.SetTexturePaint(ref ShapeRef, tp *TexturePaint) error  // nil clears; errors name the reason: no such layer, hidden item, a path, an empty texture name, a bad transform"
    - "p.SetVertexUV(ref ShapeRef, uv [][2]float32) error       // nil clears; the count must equal the path's vertex count; the slice is copied"
    - "p.SetTexture(name string, img *ebiten.Image)              // runtime image by name, wins over an asset; nil unbinds"
    - "p.TextureNames() []string                                 // names the bound paints reference, sorted"
    - "sm.OnPlayer(func(animID string, p *Player))               // StateMachinePlayer: every clip player it creates, the current one at once"
  bundle:
    - "b.ImageNames() []string / b.Image(name) ([]byte, bool)    // the shared images an asset's p can point at"
  why_player_not_animation: lottie.Animation stays immutable and shared (decision:json-level-animation-edit); paint is per player, so two players can dress one clip differently
  errors_not_notes: an unresolved reference is a returned error, not an UnsupportedFeatures entry — the feature is supported, the target is absent; a missing image at draw time draws the solid fill silently
  snapshot: a player with any paint bound never takes the idle snapshot (concept:idle-snapshot-cache) — a bound texture may be a render target the game redraws
plugin:
  - "const Dir = \"extensions/texture/\"; File(animID) string"
  - "Load(b, animID) (*Doc, error)   // nil, nil when the clip has none"
  - "Store(b, animID, *Doc) error / Remove(b, animID)"
  - "doc.Apply(p *lottie.Player) error   // every entry; errors joined, the rest still bind"
  - "Attach(sm *lottie.StateMachinePlayer, b *lottie.Bundle)   // OnPlayer that loads and applies per clip"
  - "Weave(clip map[string]any, d *Doc) (unplaced *Doc)       // tree form: attach x-tex / x-uv; entries whose address fails come back"
  - "Unweave(clip map[string]any) *Doc                        // strip every x- member into a fresh doc, addresses from the tree; nil when nothing"
  - "WeaveJSON / UnweaveJSON over bytes, re-encoded compactly — for tools without a tree of their own"
  - "MemberTex = \"x-tex\", MemberUV = \"x-uv\"; doc.Append, doc.Empty"
image_resolution:
  1: a name bound through SetTexture
  2: an image asset with that refId — decoded on first use from the bundle's i/, a data URI, or the JSON's directory; a miss is cached
  3: unresolved — the solid fill (policy:robustness)
game_flow: "anim, _ := b.Animation(id); p := anim.NewPlayer(); if doc, _ := lottietexture.Load(b, id); doc != nil { doc.Apply(p) }; p.SetTexture(\"portrait\", img)  — or lottietexture.Attach(sm, b) for a state machine"
editor_flow: load — Weave into the clip tree; every store-back — Unweave, SetAnimation(pure), Store(doc) or Remove, then the stage player is rebuilt and the doc applied (decision:texture-weave); undo snapshots carry the doc bytes
standalone_json: no file to load, but the player hooks work — a game can texture a plain JSON clip from code
```
