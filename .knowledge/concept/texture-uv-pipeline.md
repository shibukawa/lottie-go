---
id: concept:texture-uv-pipeline
type: concept
title: Coverage Mask times UV Mesh
---

How a textured fill or stroke draws, given a data:texture-document paint.
The problem: `vector.FillPath` takes a color scale and nothing else
(decision:use-vector-package), and the only vertex-level entry points,
`Path.AppendVerticesAndIndicesForFilling` and `ForStroke`, are deprecated,
emit `SrcX/SrcY` of 0, and build overlapping fans that only resolve through
the fill rule — overlapping triangles carry conflicting UVs, so they cannot
texture a concave shape.

The answer is to stop asking one mesh to do two jobs. Coverage and UV are
separated: `vector` keeps deciding which pixels are inside, and a mesh only
carries UV across them. The mesh may spill outside the shape, because the
mask decides what survives. This is concept:kage-shader-usage's mask trick,
already shipped for gradients, with the quad replaced by a mesh.
Implemented in texture.go (executeTexture and the mesh builders).

```yaml
steps:
  1_region: clip the command's geometry bounds to dst, as gradients do
  2_mesh: build UV-carrying triangles in dst space, per mapping mode — first, so a command that yields none bails before it takes an offscreen (executeGradient's stranded-fill rule)
  3_mask: pooled offscreen (decision:shared-offscreen-pool); rasterize the command's path into it with FillPath or StrokePath, shifted into mask space. Curves, fill rule, trim, dashes and antialiasing all come free and exact
  4_draw: one DrawTrianglesShader32; image0 the texture, image1 the mask; uint32 indices so a dense flattening never hits the uint16 ceiling
shader:
  unit: pixels — required, because pixel mode is what allows image0 and image1 to differ in size
  uv: SrcX/SrcY are converted to image0's texture coordinates by Ebitengine, so the interpolated src is a texture pixel position; it is made region-local with imageSrc0Origin for wrapping
  coverage: "imageSrc1At(imageSrc0Origin() + dst.xy - imageDstOrigin() - MaskOff).a — imageSrc1At rebases through image0's origin, hence the origin term; MaskOff is the region's offset inside the destination's bounds"
  wrap: on texel indices in the shader (clamp, repeat, mirror), not ebiten.Address, so an atlas sub-image repeats correctly
  filter: linear is a manual four-tap bilinear over wrapped texels, since imageSrcNAt is a point sample; nearest floors
  color: vertex ColorR..A carry tint (or 1,1,1 untinted), style alpha, repeater alphaMul and the player ColorScale premultiplied — the construction executeGradient uses
mesh_bbox:
  geometry: one quad over the region
  uv: the bounding box is taken in shape space (inverse style matrix), so a rotated group maps its own axes; each corner's UV follows from the inverse matrices and the placement inverse, and since every step is affine the quad interpolates exactly. No triangulation, so this mode is the cheap one and the fallback everything degrades to
mesh_vertex:
  flatten: the contour is flattened in dst space, steps from the control polygon's length (1 for a line, up to 32); a flattened point between two control vertices takes its UV by the segment's bezier t, so UV is authored per control vertex only
  triangulate: fan from the contour centroid — non-overlapping and correct whenever the contour is star-shaped about its centroid, which authored shapes almost always are. Ear clipping remains the quality upgrade when it is not
  inflate: every contour point is pushed 1.5 px away from the centroid (plus half the width and a pixel for a vertex-mapped stroke), so the antialiased edge pixels the mask still covers sit under a triangle
  holes: sub-paths are triangulated independently and holes are ignored — the mask has already made a hole transparent, so no triangle there is ever seen
  limits: two sub-paths genuinely overlapping in a nonzero fill leave the UV of the overlap to whichever triangle draws last; a contour not star-shaped about its centroid leaves untextured wedges until ear clipping lands
mesh_stroke:
  ribbon: two vertices per flattened centerline point, offset along the bisector normal of the incoming and outgoing directions, widened by the miter factor 1/cos(half angle) capped at 3 so the outside of a join stays covered
  width: built at 1.5× the stroke width plus a pixel; v runs 0..1 across the true width and past it on the over-wide part, which the mask clips back
  u: the authored per-vertex u when the path has a UV set, else arc length divided by (width × texture aspect) — one texture width per that many pixels, so the texture keeps its aspect with its height at the stroke's width
  ends: an open ribbon extends half a ribbon width past each end along the tangent, covering square and round caps
  dashes: each dash is its own contour, so arc-length u restarts per dash — a stamp per dash
invalidation:
  vertex_uv_needs_the_authored_vertices: any geometry of the command whose UV count no longer matches its vertex count — tm, rd, zz, op, mm rewrote the contour — makes the whole command fall back to bbox (uvUsable)
  rp: fine — copies share the source geometry's UV pointer
  stroke_on_fill: MappingStroke on an fl degrades to bbox
cost:
  per_textured_style: one pooled offscreen plus one shader draw, the same shape as a gradient
  batching: the dst switch and the shader change break the vector merge chain exactly as gradients do (concept:ebitengine-draw-batching); budget textured styles like gradient styles, not like plain fills (metric:gpu-draw-cost)
  fast_path: a vertex-mapped fill with antialias off and one convex sub-path could skip the mask and DrawTriangles straight to dst, which batches. Not built
```
