package lottie

import (
	"encoding/json"
	"fmt"
	"image"
	"math"
	"slices"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Textured fills and strokes are an extension, not Lottie: nothing in the
// format paints an image through a path. A game (or the lottietexture
// plugin, reading a bundle's extensions/texture document) binds a
// TexturePaint to a fill or stroke item and, for per-vertex mapping, a UV
// per vertex to the path items it covers. Rendering separates coverage from
// UV: the vector package still decides which pixels are inside — curves,
// fill rule, trim, dashes, antialiasing — by rasterizing a coverage mask,
// and a mesh built here only carries UV across those pixels, so it may spill
// past the outline without consequence. One pooled offscreen plus one
// shader draw per textured style, the same shape as a gradient.

// ShapeRef addresses one item of a shape layer: the precomposition asset
// that holds the layer (empty for the root composition), the layer's ind,
// and the index path from the layer's shapes array down through the it
// arrays of enclosing groups. Indices count every item as authored, hidden
// ones included, so a reference computed from the document resolves
// against the decoded tree.
type ShapeRef struct {
	Asset string
	Layer int
	Item  []int
}

func (r ShapeRef) String() string {
	if r.Asset != "" {
		return fmt.Sprintf("asset %q layer %d item %v", r.Asset, r.Layer, r.Item)
	}
	return fmt.Sprintf("layer %d item %v", r.Layer, r.Item)
}

// TextureMapping says how a textured paint finds its UV.
type TextureMapping int

const (
	// MappingBBox stretches the texture once over the path's shape-space
	// bounding box. It needs no UV data and is what the other mappings fall
	// back to when theirs is unusable.
	MappingBBox TextureMapping = iota
	// MappingVertex interpolates the UV bound per path vertex with
	// Player.SetVertexUV across the filled area.
	MappingVertex
	// MappingStroke runs u along the stroke and v across its width, one
	// texture width per (stroke width × texture aspect) pixels of arc so the
	// texture keeps its aspect at the stroke's width; a bound per-vertex UV
	// supplies u instead. Strokes only; on a fill it degrades to MappingBBox.
	MappingStroke
)

// TextureWrap says what a sample outside the texture reads.
type TextureWrap int

const (
	WrapClamp TextureWrap = iota
	WrapRepeat
	WrapMirror
)

// TextureFilter picks the sampling filter.
type TextureFilter int

const (
	FilterLinear TextureFilter = iota
	FilterNearest
)

// TexturePaint describes how a fill or stroke paints an image instead of
// its solid color. Texture names an image: one bound at runtime with
// Player.SetTexture wins, then an image asset of the animation with that
// refId; an unresolved name draws the plain solid fill, so the item's color
// stays the fallback other players show. UV is normalized 0..1 over the
// image's bounds, which makes an atlas sub-image work unchanged.
type TexturePaint struct {
	Texture string
	Mapping TextureMapping
	Wrap    TextureWrap
	Filter  TextureFilter
	// Tint multiplies the texture by the item's own color and opacity;
	// without it only the opacity applies and the color is the fallback.
	Tint bool
	// Transform places the texture in UV space as a Lottie transform object
	// with p, s, r and a members (static or keyframed, layer-local frames):
	// the anchor a (UV units) sits at position p (UV units), scaled by s
	// (percent) and rotated by r (degrees). Nil spans the texture once over
	// UV 0..1. Since the transform moves the picture, s 200 shows it twice
	// as large and p scrolls it.
	Transform json.RawMessage
}

// texturePaint is a TexturePaint parsed for rendering.
type texturePaint struct {
	name    string
	mapping TextureMapping
	wrap    TextureWrap
	filter  TextureFilter
	tint    bool
	tr      *transformTracks // nil: identity
}

// textureCmd carries a textured style for one draw command.
type textureCmd struct {
	paint *texturePaint
	mat   matrix // shape space -> destination space at style time
	uvInv matrix // UV -> sampled UV: the inverse of the placement transform
}

// texPoint is one flattened contour point in destination space with its UV
// (u doubles as arc length while a stroke ribbon is being built).
type texPoint struct{ x, y, u, v float64 }

// texSpace converts UV into the texture's own pixel coordinates, which is
// what a vertex's SrcX/SrcY are, placement transform included.
type texSpace struct {
	ox, oy, w, h float64
	inv          matrix
}

func (t texSpace) src(u, v float64) (float32, float32) {
	su, sv := t.inv.apply(u, v)
	return float32(t.ox + su*t.w), float32(t.oy + sv*t.h)
}

// texInflate is how far, in pixels, a UV mesh is pushed past the contour it
// was flattened from, so the antialiased edge pixels the mask still covers
// sit under a triangle. The mask decides coverage; spill costs nothing.
const texInflate = 1.5

func newTexturePaint(tp *TexturePaint) (*texturePaint, error) {
	if tp.Texture == "" {
		return nil, fmt.Errorf("lottie: texture paint without a texture name")
	}
	p := &texturePaint{
		name:    tp.Texture,
		mapping: tp.Mapping,
		wrap:    tp.Wrap,
		filter:  tp.Filter,
		tint:    tp.Tint,
	}
	if len(tp.Transform) > 0 {
		tr, err := parseTextureTransform(tp.Transform)
		if err != nil {
			return nil, err
		}
		p.tr = tr
	}
	return p, nil
}

// parseTextureTransform reads a Lottie transform object through the same
// builder the layers use, on a throwaway animation so its notes come back
// as an error instead of landing in the real animation's report.
func parseTextureTransform(raw json.RawMessage) (*transformTracks, error) {
	var rt rawTransform
	if err := json.Unmarshal(raw, &rt); err != nil {
		return nil, fmt.Errorf("lottie: texture transform: %w", err)
	}
	tmp := &builder{anim: &Animation{unsupported: map[string]struct{}{}}}
	tr := tmp.buildTransform(&rt)
	for note := range tmp.anim.unsupported {
		return nil, fmt.Errorf("lottie: texture transform: %s", note)
	}
	return tr, nil
}

// SetTexturePaint paints the fill or stroke item at ref with a texture, or
// clears it with nil. The reference must resolve to an fl or st item of
// this player's animation. Paint is per player, so two players can dress
// one shared animation differently.
func (p *Player) SetTexturePaint(ref ShapeRef, tp *TexturePaint) error {
	n, err := p.anim.shapeNodeAt(ref)
	if err != nil {
		return err
	}
	if n.kind != "fl" && n.kind != "st" {
		return fmt.Errorf("lottie: %s is a %s item, not a fill or stroke", ref, n.kind)
	}
	if tp == nil {
		delete(p.r.paints, n)
		return nil
	}
	rp, err := newTexturePaint(tp)
	if err != nil {
		return err
	}
	if p.r.paints == nil {
		p.r.paints = map[*shapeNode]*texturePaint{}
	}
	p.r.paints[n] = rp
	return nil
}

// SetVertexUV binds one normalized UV per vertex of the path item at ref,
// for paints that use MappingVertex (or MappingStroke, which reads u); nil
// clears it. The count must match the path's vertex count, which Lottie
// keeps constant across a path's keyframes.
func (p *Player) SetVertexUV(ref ShapeRef, uv [][2]float32) error {
	n, err := p.anim.shapeNodeAt(ref)
	if err != nil {
		return err
	}
	if n.kind != "sh" {
		return fmt.Errorf("lottie: %s is a %s item, not a path", ref, n.kind)
	}
	if uv == nil {
		delete(p.r.uvs, n)
		return nil
	}
	if want := n.shape.vertexCount(); want != len(uv) {
		return fmt.Errorf("lottie: %s: %d UVs for a path of %d vertices", ref, len(uv), want)
	}
	if p.r.uvs == nil {
		p.r.uvs = map[*shapeNode][][2]float32{}
	}
	p.r.uvs[n] = slices.Clone(uv)
	return nil
}

// SetTexture binds an image to a texture name, taking precedence over an
// image asset of the same name; nil unbinds. The player never owns the
// image: one drawn into between frames shows its new content on the next
// Draw with no rebind.
func (p *Player) SetTexture(name string, img *ebiten.Image) {
	if img == nil {
		delete(p.r.textures, name)
		return
	}
	if p.r.textures == nil {
		p.r.textures = map[string]*ebiten.Image{}
	}
	p.r.textures[name] = img
}

// TextureNames lists the texture names the bound paints reference, sorted:
// what a game has to supply through SetTexture or as image assets.
func (p *Player) TextureNames() []string {
	var names []string
	for _, tp := range p.r.paints {
		if !slices.Contains(names, tp.name) {
			names = append(names, tp.name)
		}
	}
	slices.Sort(names)
	return names
}

// shapeNodeAt resolves a ShapeRef against the decoded tree.
func (a *Animation) shapeNodeAt(ref ShapeRef) (*shapeNode, error) {
	layers := a.layers
	if ref.Asset != "" {
		var ok bool
		if layers, ok = a.comps[ref.Asset]; !ok {
			return nil, fmt.Errorf("lottie: %s: no precomposition asset %q", ref, ref.Asset)
		}
	}
	var layer *layerNode
	for _, l := range layers {
		if l.ind == ref.Layer {
			layer = l
			break
		}
	}
	if layer == nil {
		return nil, fmt.Errorf("lottie: %s: no layer with ind %d", ref, ref.Layer)
	}
	if len(ref.Item) == 0 {
		return nil, fmt.Errorf("lottie: %s: empty item path", ref)
	}
	nodes := layer.shapes
	var n *shapeNode
	for depth, idx := range ref.Item {
		n = nil
		for _, c := range nodes {
			if c.jsonIndex == idx {
				n = c
				break
			}
		}
		if n == nil {
			return nil, fmt.Errorf("lottie: %s: no item at index %d", ref, idx)
		}
		if depth < len(ref.Item)-1 {
			if n.kind != "gr" {
				return nil, fmt.Errorf("lottie: %s: item %d is a %s, not a group", ref, idx, n.kind)
			}
			nodes = n.children
		}
	}
	return n, nil
}

// imageAsset returns the animation's image asset by refId, decoding it on
// first use; a miss is cached too, so a paint naming a broken asset does not
// retry every frame.
func (a *Animation) imageAsset(refID string) *ebiten.Image {
	if img, ok := a.images[refID]; ok {
		return img
	}
	var img *ebiten.Image
	if as, ok := a.imageAssets[refID]; ok {
		img, _ = loadImageAsset(&as, a.resolver)
	}
	if a.images == nil {
		a.images = map[string]*ebiten.Image{}
	}
	a.images[refID] = img
	return img
}

// vertexCount is the path's vertex count, constant across its keys.
func (tr *shapeTrack) vertexCount() int {
	if tr == nil {
		return 0
	}
	if tr.keys == nil {
		return len(tr.static.V)
	}
	if len(tr.keys) == 0 {
		return 0
	}
	return len(tr.keys[0].value.V)
}

// textureCmd evaluates a paint at frame f into the command arena, which
// resets with the geometry arenas (see nextGradCmd for the aliasing rule).
func (r *renderer) textureCmd(tp *texturePaint, f float64, mat matrix) *textureCmd {
	if r.nTex == len(r.texCmds) {
		r.texCmds = append(r.texCmds, textureCmd{})
	}
	c := &r.texCmds[r.nTex]
	r.nTex++
	c.paint, c.mat, c.uvInv = tp, mat, identityMatrix
	if tp.tr != nil {
		if inv, ok := tp.tr.matrixAt(f).invert(); ok {
			c.uvInv = inv
		}
	}
	return c
}

// lookupTexture resolves a paint's texture name: runtime binding first,
// then the animation's image assets.
func (r *renderer) lookupTexture(name string) *ebiten.Image {
	if img := r.textures[name]; img != nil {
		return img
	}
	if r.anim == nil {
		return nil
	}
	return r.anim.imageAsset(name)
}

var (
	texShaderOnce sync.Once
	texShader     *ebiten.Shader
)

// texShaderSrc samples image0 (the texture) at the interpolated source
// position and multiplies by the coverage of image1 (the mask) under the
// fragment. Pixel mode is what lets the two images differ in size. Wrapping
// is done on texel indices here rather than through ebiten.Address, so an
// atlas sub-image repeats correctly; the linear filter is a manual
// bilinear, since imageSrc0At is a point sample.
const texShaderSrc = `//kage:unit pixels

package main

var Wrap float   // 0 clamp, 1 repeat, 2 mirror
var Filter float // 0 linear, 1 nearest
var MaskOff vec2 // mask origin relative to the destination's bounds

func wrapIndex(i float, n float) float {
	if Wrap == 1 {
		return mod(i, n)
	}
	if Wrap == 2 {
		m := mod(i, 2*n)
		if m >= n {
			m = 2*n - 1 - m
		}
		return m
	}
	return clamp(i, 0, n-1)
}

func texel(i vec2) vec4 {
	sz := imageSrc0Size()
	p := vec2(wrapIndex(i.x, sz.x), wrapIndex(i.y, sz.y)) + 0.5
	return imageSrc0UnsafeAt(imageSrc0Origin() + p)
}

func sample(t vec2) vec4 {
	if Filter == 1 {
		return texel(floor(t))
	}
	q := t - 0.5
	i := floor(q)
	f := q - i
	c00 := texel(i)
	c10 := texel(i + vec2(1, 0))
	c01 := texel(i + vec2(0, 1))
	c11 := texel(i + vec2(1, 1))
	return mix(mix(c00, c10, f.x), mix(c01, c11, f.x), f.y)
}

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	// dst.xy is a position on the destination's internal texture;
	// imageDstOrigin makes it relative to the destination's bounds, and
	// MaskOff then lands on the mask, which covers only the style's region.
	// imageSrc1At rebases through image0's origin, hence that term.
	m := dst.xy - imageDstOrigin() - MaskOff
	cov := imageSrc1At(imageSrc0Origin() + m).a
	if cov <= 0 {
		return vec4(0)
	}
	// src arrives in image0's texture pixels; wrap on region-local texels.
	return sample(src-imageSrc0Origin()) * cov * color
}
`

// executeTexture draws one textured style: the UV mesh for its mapping,
// then the coverage mask, then a single shader draw over both. A mapping
// that cannot be built degrades to the bounding-box one; a texture that
// cannot be found degrades to the solid fill.
func (r *renderer) executeTexture(dst *ebiten.Image, cmd *drawCmd, arr []geometry, cs ebiten.ColorScale, blend ebiten.Blend, antialias bool) {
	tc := cmd.tex
	img := r.lookupTexture(tc.paint.name)
	if img == nil {
		r.executeSolid(dst, cmd, arr, cs, blend, antialias)
		return
	}
	region, ok := geomControlBounds(arr, cmd.geomStart, cmd.geomEnd)
	if !ok {
		return
	}
	pad := 2
	if cmd.stroke {
		pad = int(cmd.strokeOpts.Width) + 2
	}
	region = region.Inset(-pad).Intersect(dst.Bounds())
	if region.Empty() {
		return
	}

	mapping := tc.paint.mapping
	if mapping == MappingStroke && !cmd.stroke {
		mapping = MappingBBox
	}
	if mapping == MappingVertex && !uvUsable(arr, cmd.geomStart, cmd.geomEnd) {
		mapping = MappingBBox
	}
	tb := img.Bounds()
	tex := texSpace{
		ox: float64(tb.Min.X), oy: float64(tb.Min.Y),
		w: float64(tb.Dx()), h: float64(tb.Dy()),
		inv: tc.uvInv,
	}
	// The mesh comes first: a command that yields none must bail before it
	// takes an offscreen (see executeGradient on stranded fills).
	r.texVerts, r.texIdx = r.texVerts[:0], r.texIdx[:0]
	switch mapping {
	case MappingVertex:
		inflate := texInflate
		if cmd.stroke {
			inflate += float64(cmd.strokeOpts.Width)/2 + 1
		}
		r.meshVertexUV(arr, cmd, tex, inflate)
	case MappingStroke:
		r.meshStroke(arr, cmd, tex, float64(cmd.strokeOpts.Width))
	default:
		r.meshBBox(arr, cmd, region, tex)
	}
	if len(r.texIdx) == 0 {
		return
	}
	// Tint, style alpha, the repeater ramp and the player's color scale ride
	// on the vertex color, premultiplied, as they do for gradients.
	a := cmd.a * cmd.alphaMul
	cr, cg, cb := cmd.r, cmd.g, cmd.b
	if !tc.paint.tint {
		cr, cg, cb = 1, 1, 1
	}
	col := [4]float32{
		float32(cr*a) * cs.R(), float32(cg*a) * cs.G(), float32(cb*a) * cs.B(), float32(a) * cs.A(),
	}
	for i := range r.texVerts {
		v := &r.texVerts[i]
		v.ColorR, v.ColorG, v.ColorB, v.ColorA = col[0], col[1], col[2], col[3]
	}

	w, h := region.Dx(), region.Dy()
	mask, maskBase := sharedPool.get(w, h)
	shift := identityMatrix.translate(-float64(region.Min.X), -float64(region.Min.Y))
	r.maskPath.Reset()
	for i := cmd.geomStart; i < cmd.geomEnd; i++ {
		g := &arr[i]
		g.bez.appendToPath(&r.maskPath, shift.mul(g.mat))
	}
	var pop vector.DrawPathOptions
	pop.AntiAlias = antialias
	if cmd.stroke {
		so := cmd.strokeOpts
		vector.StrokePath(mask, &r.maskPath, &so, &pop)
	} else {
		fo := vector.FillOptions{FillRule: cmd.fillRule}
		vector.FillPath(mask, &r.maskPath, &fo, &pop)
	}

	texShaderOnce.Do(func() {
		s, err := ebiten.NewShader([]byte(texShaderSrc))
		if err != nil {
			panic("lottie: internal texture shader failed to compile: " + err.Error())
		}
		texShader = s
	})
	if r.texUnis == nil {
		r.texUnis = map[string]any{"MaskOff": make([]float32, 2)}
	}
	u := r.texUnis
	u["Wrap"] = float32(tc.paint.wrap)
	u["Filter"] = float32(tc.paint.filter)
	mo := u["MaskOff"].([]float32)
	mo[0] = float32(region.Min.X - dst.Bounds().Min.X)
	mo[1] = float32(region.Min.Y - dst.Bounds().Min.Y)

	var top ebiten.DrawTrianglesShaderOptions
	top.Images[0] = img
	top.Images[1] = mask
	top.Blend = blend
	top.Uniforms = u
	dst.DrawTrianglesShader32(r.texVerts, r.texIdx, texShader, &top)
	sharedPool.put(maskBase)
}

// uvUsable reports whether every geometry of the command still carries a
// UV per vertex: a modifier that rewrote the contour (trim, round corners,
// zig zag, offset, merge) leaves the counts out of step, and the paint
// falls back to the bounding box.
func uvUsable(arr []geometry, start, end int) bool {
	for i := start; i < end; i++ {
		g := &arr[i]
		if len(g.uv) == 0 || len(g.uv) != len(g.bez.V) {
			return false
		}
	}
	return end > start
}

// meshBBox is one quad over the region. The bounding box is taken in shape
// space, so a rotated group still maps its own axes; the UV of each corner
// follows from the inverse matrices, and since every step is affine the
// interpolation across the quad is exact.
func (r *renderer) meshBBox(arr []geometry, cmd *drawCmd, region image.Rectangle, tex texSpace) {
	inv, ok := cmd.tex.mat.invert()
	if !ok {
		return
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for i := cmd.geomStart; i < cmd.geomEnd; i++ {
		g := &arr[i]
		m := inv.mul(g.mat)
		for j, v := range g.bez.V {
			for _, pt := range [3][2]float64{
				v,
				{v[0] + tangentAt(g.bez.I, j)[0], v[1] + tangentAt(g.bez.I, j)[1]},
				{v[0] + tangentAt(g.bez.O, j)[0], v[1] + tangentAt(g.bez.O, j)[1]},
			} {
				x, y := m.apply(pt[0], pt[1])
				minX, minY = math.Min(minX, x), math.Min(minY, y)
				maxX, maxY = math.Max(maxX, x), math.Max(maxY, y)
			}
		}
	}
	if minX > maxX {
		return
	}
	bw, bh := maxX-minX, maxY-minY
	if bw <= 0 {
		bw = 1
	}
	if bh <= 0 {
		bh = 1
	}
	base := uint32(len(r.texVerts))
	for _, c := range [4]image.Point{
		region.Min, {region.Max.X, region.Min.Y}, {region.Min.X, region.Max.Y}, region.Max,
	} {
		px, py := inv.apply(float64(c.X), float64(c.Y))
		sx, sy := tex.src((px-minX)/bw, (py-minY)/bh)
		r.texVerts = append(r.texVerts, ebiten.Vertex{
			DstX: float32(c.X), DstY: float32(c.Y), SrcX: sx, SrcY: sy,
		})
	}
	r.texIdx = append(r.texIdx, base, base+1, base+2, base+1, base+3, base+2)
}

// meshVertexUV fans each contour from its centroid: non-overlapping, hence
// UV-consistent, whenever the contour is star-shaped about that point,
// which authored shapes almost always are. Holes need nothing — the mask
// has already made them transparent. Two genuinely overlapping sub-paths
// leave the overlap's UV to whichever triangle draws last.
func (r *renderer) meshVertexUV(arr []geometry, cmd *drawCmd, tex texSpace, inflate float64) {
	for i := cmd.geomStart; i < cmd.geomEnd; i++ {
		g := &arr[i]
		r.flattenContour(g, true)
		pts := r.texPts
		if len(pts) < 3 {
			continue
		}
		var cx, cy, cu, cv float64
		for _, p := range pts {
			cx, cy, cu, cv = cx+p.x, cy+p.y, cu+p.u, cv+p.v
		}
		n := float64(len(pts))
		cx, cy, cu, cv = cx/n, cy/n, cu/n, cv/n
		base := uint32(len(r.texVerts))
		sx, sy := tex.src(cu, cv)
		r.texVerts = append(r.texVerts, ebiten.Vertex{
			DstX: float32(cx), DstY: float32(cy), SrcX: sx, SrcY: sy,
		})
		for _, p := range pts {
			dx, dy := p.x-cx, p.y-cy
			if d := math.Hypot(dx, dy); d > 1e-9 {
				k := (d + inflate) / d
				dx, dy = dx*k, dy*k
			}
			sx, sy := tex.src(p.u, p.v)
			r.texVerts = append(r.texVerts, ebiten.Vertex{
				DstX: float32(cx + dx), DstY: float32(cy + dy), SrcX: sx, SrcY: sy,
			})
		}
		m := uint32(len(pts))
		for k := uint32(0); k < m; k++ {
			r.texIdx = append(r.texIdx, base, base+1+k, base+1+(k+1)%m)
		}
	}
}

// meshStroke lays a ribbon along each flattened centerline, built over-wide
// so the wedges outside miter and round joins and the caps stay covered;
// the mask clips it back to the true stroke. v spans the true width 0..1,
// u is the authored per-vertex u or arc length in texture widths.
func (r *renderer) meshStroke(arr []geometry, cmd *drawCmd, tex texSpace, width float64) {
	if width <= 0 {
		return
	}
	hw := width/2*1.5 + 1
	vLo, vHi := 0.5-hw/width, 0.5+hw/width
	for i := cmd.geomStart; i < cmd.geomEnd; i++ {
		g := &arr[i]
		useUV := len(g.uv) > 0 && len(g.uv) == len(g.bez.V)
		r.flattenContour(g, useUV)
		pts := r.texPts
		m := len(pts)
		if m < 2 {
			continue
		}
		if !useUV {
			// One texture width per (width × aspect) pixels of arc keeps
			// the texture's aspect with its height at the stroke's width.
			per := width * tex.w / math.Max(tex.h, 1)
			if per <= 0 {
				per = 1
			}
			for k := range pts {
				pts[k].u /= per
			}
		}
		closed := g.bez.Closed && m > 2
		base := uint32(len(r.texVerts))
		for k := 0; k < m; k++ {
			prev, next := k-1, k+1
			if closed {
				prev, next = (k-1+m)%m, (k+1)%m
			} else {
				prev, next = max(prev, 0), min(next, m-1)
			}
			// Incoming and outgoing directions; their bisector is the
			// ribbon's normal at a corner, widened by the miter factor so
			// the outside of the join stays under the ribbon.
			ix, iy := unit(pts[k].x-pts[prev].x, pts[k].y-pts[prev].y)
			ox, oy := unit(pts[next].x-pts[k].x, pts[next].y-pts[k].y)
			if prev == k {
				ix, iy = ox, oy
			}
			if next == k {
				ox, oy = ix, iy
			}
			nx, ny := unit(-iy-oy, ix+ox)
			if nx == 0 && ny == 0 {
				nx, ny = -iy, ix
			}
			scale := 1.0
			if cosHalf := nx*-iy + ny*ix; cosHalf > 0.2 {
				scale = math.Min(1/cosHalf, 3)
			}
			px, py := pts[k].x, pts[k].y
			if !closed {
				// Extend the ends along the tangent so caps are covered.
				if k == 0 {
					px, py = px-ox*hw, py-oy*hw
				} else if k == m-1 {
					px, py = px+ix*hw, py+iy*hw
				}
			}
			ex, ey := nx*hw*scale, ny*hw*scale
			s0x, s0y := tex.src(pts[k].u, vLo)
			s1x, s1y := tex.src(pts[k].u, vHi)
			r.texVerts = append(r.texVerts,
				ebiten.Vertex{DstX: float32(px - ex), DstY: float32(py - ey), SrcX: s0x, SrcY: s0y},
				ebiten.Vertex{DstX: float32(px + ex), DstY: float32(py + ey), SrcX: s1x, SrcY: s1y},
			)
		}
		segs := m - 1
		if closed {
			segs = m
		}
		for k := 0; k < segs; k++ {
			a := base + uint32(k)*2
			b := base + uint32((k+1)%m)*2
			r.texIdx = append(r.texIdx, a, a+1, b, a+1, b+1, b)
		}
	}
}

func unit(x, y float64) (float64, float64) {
	l := math.Hypot(x, y)
	if l < 1e-9 {
		return 0, 0
	}
	return x / l, y / l
}

// flattenContour samples the geometry's contour into r.texPts in
// destination space. With useUV the authored UV is carried along each
// segment by its bezier parameter, so UV is authored per control vertex
// only; otherwise u accumulates arc length in pixels and v stays 0. Open
// contours end on their last vertex; closed ones stop short of repeating
// the first.
func (r *renderer) flattenContour(g *geometry, useUV bool) {
	r.texPts = r.texPts[:0]
	b := &g.bez
	n := len(b.V)
	if n == 0 {
		return
	}
	segs := n - 1
	if b.Closed {
		segs = n
	}
	arc := 0.0
	uvAt := func(j int) (float64, float64) {
		if useUV && j < len(g.uv) {
			return float64(g.uv[j][0]), float64(g.uv[j][1])
		}
		return 0, 0
	}
	var lx, ly float64
	for s := 0; s < segs; s++ {
		p0, p1, p2, p3 := segmentPoints(b, s)
		p0[0], p0[1] = g.mat.apply(p0[0], p0[1])
		p1[0], p1[1] = g.mat.apply(p1[0], p1[1])
		p2[0], p2[1] = g.mat.apply(p2[0], p2[1])
		p3[0], p3[1] = g.mat.apply(p3[0], p3[1])
		j, k := s, (s+1)%n
		u0, v0 := uvAt(j)
		u1, v1 := uvAt(k)
		steps := flattenSteps(p0, p1, p2, p3)
		for i := 0; i < steps; i++ {
			t := float64(i) / float64(steps)
			x, y := cubicPoint(p0, p1, p2, p3, t)
			pt := texPoint{x: x, y: y}
			if useUV {
				pt.u, pt.v = u0+(u1-u0)*t, v0+(v1-v0)*t
			} else {
				if len(r.texPts) > 0 {
					arc += math.Hypot(x-lx, y-ly)
				}
				pt.u = arc
			}
			lx, ly = x, y
			r.texPts = append(r.texPts, pt)
		}
	}
	if !b.Closed {
		last := n - 1
		x, y := g.mat.apply(b.V[last][0], b.V[last][1])
		pt := texPoint{x: x, y: y}
		if useUV {
			pt.u, pt.v = uvAt(last)
		} else {
			if len(r.texPts) > 0 {
				arc += math.Hypot(x-lx, y-ly)
			}
			pt.u = arc
		}
		r.texPts = append(r.texPts, pt)
	}
}

// flattenSteps picks a subdivision from the control polygon's length in
// pixels: a straight segment needs one step, a long curve a few dozen.
func flattenSteps(p0, p1, p2, p3 [2]float64) int {
	if p1 == p0 && p2 == p3 {
		return 1
	}
	l := math.Hypot(p1[0]-p0[0], p1[1]-p0[1]) +
		math.Hypot(p2[0]-p1[0], p2[1]-p1[1]) +
		math.Hypot(p3[0]-p2[0], p3[1]-p2[1])
	return min(max(int(math.Ceil(math.Sqrt(l*1.5))), 1), 32)
}
