# The UV-morph rig: what `rig` builds

A part becomes a shape layer whose closed path is filled with its
texture through per-vertex UV, parented and keyed exactly as the
preset's image layer was. Everything below is mechanical; the point of
writing it down is that an agent can script it (Go on the lottie-go
module: `lottie.DecodeBundle`, `plugin/texture`, `Bundle.Encode`) until
`lottieforge rig` ships, and can read the result in a dump.

## Two spaces, one fit

- **Slot space** is the template part's pixel space — the layer's own
  coordinates, where `ks.a` is the joint and each child's `ks.p` is an
  attach point. The contour is written here, so `ks` is copied from the
  template with every keyframe.
- **Texture space** is the generated part image, any resolution. UV is
  normalized over it (0..1).

The fit maps texture space into slot space. Its invariant is **joint
length**, not canvas size:

| slot kind                          | scale rule                                                    | anchor lands on            |
|------------------------------------|---------------------------------------------------------------|----------------------------|
| segment with a child (upper-arm, thigh) | top-cap center → bottom-cap center maps onto anchor → child attach (23 px for the chibi upper arm, 29 for the thigh) | top cap center → slot anchor |
| terminal segment (forearm, shin)   | alpha bbox height → slot height × `fit`                       | top cap center → slot anchor |
| head                               | bbox height → slot height × `fit`                             | bottom center (neck) → anchor |
| body                               | bbox height → slot height × `fit`                             | bottom center (hip) → anchor |
| prop                               | bbox height → `length` (default the template's)               | top center (grip) → anchor  |

Width follows the height scale. Horizontal attach offsets on the body
(shoulders at x 3 and 45 of the 48-wide template torso) stay as the
template has them unless `attach: "scale-x"` multiplies them by the
width ratio. Cap centers are read from the alpha: the extreme rows and
the mean x of the alpha a few rows inside — which is what the prompts'
"vertical, rounded caps" rules guarantee.

The rest pose is then an identity: at every vertex `uv = fit⁻¹(v) /
textureSize`, so frame 0 of idle paints the picture where the raster
rig would have drawn the image. `faithful.png` is that comparison,
rendered by lottie-go for both rigs; a difference over budget (default
2% of the character's pixels) fails the rig step and names the slot.

## Contour

1. Trace the alpha > 0.5 outer border of the (largest) component.
2. Simplify (Douglas-Peucker) toward the slot's vertex budget — 12 for
   limbs, 16 for head and body, `vertices` in the spec overrides —
   raising epsilon until the budget is met.
3. Dilate about 2 px outward. The texture's own alpha draws the
   silhouette (the shader multiplies the sampled texel), so the path
   only has to *contain* the art; a loose hull is right and a tight one
   shows key-colored halos at the edge.
4. Tangents: Catmull-Rom on smooth vertices; the two cap ends stay
   corner vertices (zero tangents) so a limb end stays round when the
   vertices near it move.
5. **Star check and decomposition**: every vertex must be visible from
   the centroid, or the renderer's centroid fan overlaps itself and
   leaves untextured wedges. A contour that fails is cut at its
   deepest reflex vertex into two, and again until every piece passes
   — each piece its own `sh` in the same group, under the same fill
   and paint, with its own UV entry over the same texture. The union is
   the same silhouette, so the mask is unchanged and the seam is
   invisible. Vertices two pieces share are recorded as welds and move
   together under every generator. Only a contour that still fails
   after the vertex budget is exhausted is refused, with the vertices
   named.

## The rewrite, per clip and per image layer

```jsonc
// template (chibi-sword forearm-far, abbreviated)
{ "ty": 2, "nm": "forearm-far", "ind": 4, "parent": 3, "refId": "forearm-far",
  "ip": 0, "op": 48, "st": 0, "ks": { "a": {...}, "p": {...}, "s": {...}, "r": {...}, "o": {...} } }

// forged
{ "ty": 4, "nm": "forearm-far", "ind": 4, "parent": 3,
  "ip": 0, "op": 48, "st": 0, "ks": { /* identical, every key */ },
  "shapes": [ { "ty": "gr", "nm": "forearm-far", "it": [
      { "ty": "sh", "nm": "contour", "ks": { "a": 0, "k": { "c": true, "v": [...], "i": [...], "o": [...] } } },
      { "ty": "fl", "nm": "fallback", "c": { "a": 0, "k": [0.93, 0.80, 0.71] }, "o": { "a": 0, "k": 100 }, "r": 1 },
      { "ty": "tr", "p": {"a":0,"k":[0,0]}, "a": {"a":0,"k":[0,0]}, "s": {"a":0,"k":[100,100]}, "r": {"a":0,"k":0}, "o": {"a":0,"k":100} }
  ] } ] }
```

and in `extensions/texture/<clip>.json`:

```jsonc
{ "paints": [ { "layer": 4, "item": [0, 1], "texture": "forearm-far", "mapping": "vertex", "tint": false } ],
  "uvs":    [ { "layer": 4, "item": [0, 0], "v": [[0.51, 0.02], [0.93, 0.11], ...] } ] }
```

- `assets` keeps the template entries: `id` = slot, `p` = the file in
  the bundle's `i/`, `w`/`h` = the new texture's size. That is how the
  paint's texture name resolves (an image asset with that refId).
- The `fl` color is the part's alpha-weighted mean, so a plain Lottie
  player shows a silhouette of the right hue.
- View swaps (head-side, body-back …) are rewritten the same way; the
  hold keys on their opacity live in `ks` and carry over untouched.
- Layer order, `ind` and `parent` never change: the sword's front-of-
  body placement, the far chain's draw order, everything the preset
  decided about depth is kept.
- The rest contours (one per sub-path), their UV and the welds are
  also stored in `extensions/forge/<slot>.json` so `morph` re-bakes
  from the untouched shape rather than from the previous bake.
- **Attachment layers** are added to every clip the same way, with
  `parent` = the host's `ind`, `a` = the item's pivot, `p` = the attach
  point in the host's slot space, a fresh `ind` above the clip's
  highest, inserted into the layer array at the kind's depth (array
  order is depth; `ind` is identity — nothing is renumbered), and `o`
  copied from the host's front-view layer so turn clips hide them with
  the front drawing. See references/attachments.md.

## Reading a forged bundle in the editor

Open it in `cmd/lottie-state-editor`: the Shapes tab lists each part's
group with its contour, the fill's texture section shows the paint
(image, mapping vertex, tint off), and the UV pane draws the contour
over the part image. Dragging a UV point there is the manual fix for a
texture that sits slightly off; dragging a vertex at a key is a manual
morph. Both survive a `morph` re-bake only if the spec records them —
say so to the user, and prefer changing the spec.
