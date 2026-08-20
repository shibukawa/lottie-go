package lottie

import (
	"math"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// geometry is one evaluated contour together with the transform that maps it
// to destination space. The bezier buffers are owned by the renderer and
// reused across frames. alpha carries per-copy opacity from repeaters.
type geometry struct {
	bez   bezierShape
	mat   matrix
	alpha float64
}

// drawCmd is one fill or stroke over a range of geometries. Style properties
// are evaluated at command-creation time. dashed commands index into the
// renderer's dash arena instead of the main geometry list.
type drawCmd struct {
	stroke     bool
	dashed     bool
	geomStart  int
	geomEnd    int
	r, g, b, a float64
	alphaMul   float64 // extra multiplier (repeater opacity ramp)
	fillRule   vector.FillRule
	strokeOpts vector.StrokeOptions
	grad       *gradientCmd // non-nil for gradient fills/strokes
}

// maxOffscreen bounds pooled offscreen dimensions.
const maxOffscreen = 4096

// imagePool reuses offscreen images by exact size.
type imagePool struct {
	free map[[2]int][]*ebiten.Image
}

func (p *imagePool) get(w, h int) *ebiten.Image {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if w > maxOffscreen {
		w = maxOffscreen
	}
	if h > maxOffscreen {
		h = maxOffscreen
	}
	key := [2]int{w, h}
	if p.free == nil {
		p.free = map[[2]int][]*ebiten.Image{}
	}
	if s := p.free[key]; len(s) > 0 {
		img := s[len(s)-1]
		p.free[key] = s[:len(s)-1]
		img.Clear()
		return img
	}
	return ebiten.NewImage(w, h)
}

func (p *imagePool) put(img *ebiten.Image) {
	b := img.Bounds()
	key := [2]int{b.Dx(), b.Dy()}
	p.free[key] = append(p.free[key], img)
}

// renderer holds reusable buffers for one Player.
type renderer struct {
	anim         *Animation
	geoms        []geometry
	nGeoms       int
	dashGeoms    []geometry
	nDash        int
	cmds         []drawCmd
	path         vector.Path
	maskPath     vector.Path
	shapeScratch bezierShape
	maskScratch  bezierShape
	vecScratch   []float64
	dashVals     []float64
	repGeoms     []geometry // repeater source snapshot
	repCmds      []drawCmd
	pool         imagePool
	trim         trimmer
}

func nextSlot(arr []geometry, n *int) ([]geometry, *geometry) {
	if *n == len(arr) {
		arr = append(arr, geometry{})
	}
	g := &arr[*n]
	*n++
	g.alpha = 1
	return arr, g
}

// nextGeom returns a reusable geometry slot.
func (r *renderer) nextGeom() *geometry {
	var g *geometry
	r.geoms, g = nextSlot(r.geoms, &r.nGeoms)
	return g
}

// nextDash returns a reusable slot in the dash arena.
func (r *renderer) nextDash() *geometry {
	var g *geometry
	r.dashGeoms, g = nextSlot(r.dashGeoms, &r.nDash)
	return g
}

func copyGeomInto(dst *geometry, src *geometry) {
	dst.mat = src.mat
	dst.alpha = src.alpha
	dst.bez.Closed = src.bez.Closed
	dst.bez.V = copyPoints(dst.bez.V, src.bez.V)
	dst.bez.I = copyPoints(dst.bez.I, src.bez.I)
	dst.bez.O = copyPoints(dst.bez.O, src.bez.O)
}

func copyPoints(dst, src [][2]float64) [][2]float64 {
	return append(dst[:0], src...)
}

// render draws the whole animation at composition frame f.
func (r *renderer) render(dst *ebiten.Image, anim *Animation, f float64, root matrix, cs ebiten.ColorScale, antialias bool) {
	r.anim = anim
	r.renderLayers(dst, anim.layers, f, root, cs, antialias)
}

// renderLayers draws a layer list bottom-up (layers are listed
// topmost-first).
func (r *renderer) renderLayers(dst *ebiten.Image, layers []*layerNode, f float64, root matrix, cs ebiten.ColorScale, antialias bool) {
	for i := len(layers) - 1; i >= 0; i-- {
		l := layers[i]
		if l.hidden || l.matteOnly {
			continue
		}
		if f < l.ip || f >= l.op {
			continue
		}
		r.renderLayer(dst, l, f, root, cs, antialias, true)
	}
}

// renderLayer draws one layer, including its masks and track matte.
func (r *renderer) renderLayer(dst *ebiten.Image, l *layerNode, f float64, root matrix, cs ebiten.ColorScale, antialias bool, applyMatte bool) {
	lt := l.localTime(f)
	opacity := l.transform.opacityAt(lt)
	if opacity <= 0 {
		return
	}
	mat := root.mul(layerMatrix(l, f, 0))
	blend := blendFor(l.blend)

	hasMatte := applyMatte && l.matteMode != 0 && l.matteSrc != nil
	if len(l.masks) == 0 && !hasMatte {
		r.renderBody(dst, l, f, lt, mat, opacity, cs, blend, antialias)
		return
	}

	// Flatten the layer into a dst-sized offscreen, apply masks and matte
	// there, then composite.
	bounds := dst.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	shift := identityMatrix.translate(-float64(bounds.Min.X), -float64(bounds.Min.Y))
	bodyMat := shift.mul(mat)
	var neutral ebiten.ColorScale

	content := r.pool.get(w, h)
	r.renderBody(content, l, f, lt, bodyMat, 1, neutral, ebiten.BlendSourceOver, antialias)

	if len(l.masks) > 0 {
		r.applyMasks(content, l.masks, lt, bodyMat, antialias)
	}
	if hasMatte {
		matteImg := r.pool.get(w, h)
		src := l.matteSrc
		if !src.hidden && f >= src.ip && f < src.op {
			r.renderLayer(matteImg, src, f, shift.mul(root), neutral, antialias, false)
		}
		combineMatte(content, matteImg, l.matteMode)
		r.pool.put(matteImg)
	}

	var op ebiten.DrawImageOptions
	op.GeoM.Translate(float64(bounds.Min.X), float64(bounds.Min.Y))
	op.ColorScale = cs
	op.ColorScale.ScaleAlpha(float32(opacity))
	op.Blend = blend
	dst.DrawImage(content, &op)
	r.pool.put(content)
}

// renderBody draws the layer's content with the given transform.
func (r *renderer) renderBody(dst *ebiten.Image, l *layerNode, f, lt float64, mat matrix, opacity float64, cs ebiten.ColorScale, blend ebiten.Blend, antialias bool) {
	switch l.typ {
	case 4:
		r.nGeoms = 0
		r.nDash = 0
		r.cmds = r.cmds[:0]
		r.walkShapes(l.shapes, lt, mat, opacity)
		for c := len(r.cmds) - 1; c >= 0; c-- {
			r.execute(dst, &r.cmds[c], cs, blend, antialias)
		}
	case 5:
		r.renderText(dst, l, lt, mat, opacity, cs, blend)
	case 0:
		if len(l.comp) == 0 || l.compW <= 0 || l.compH <= 0 {
			return
		}
		// Render the composition into an offscreen scaled to the on-screen
		// resolution so vectors stay crisp, then composite with clipping.
		s := mat.meanScale()
		if s <= 0 {
			return
		}
		if s > 4 {
			s = 4
		}
		ow := int(math.Ceil(l.compW * s))
		oh := int(math.Ceil(l.compH * s))
		if ow < 1 || oh < 1 {
			return
		}
		if ow > maxOffscreen {
			ow, s = maxOffscreen, float64(maxOffscreen)/l.compW
			oh = int(math.Ceil(l.compH * s))
		}
		if oh > maxOffscreen {
			oh, s = maxOffscreen, float64(maxOffscreen)/l.compH
			ow = int(math.Ceil(l.compW * s))
		}
		off := r.pool.get(ow, oh)
		var neutral ebiten.ColorScale
		r.renderLayers(off, l.comp, lt, identityMatrix.scale(s, s), neutral, antialias)
		var op ebiten.DrawImageOptions
		op.GeoM = mat.scale(1/s, 1/s).toGeoM()
		op.ColorScale = cs
		op.ColorScale.ScaleAlpha(float32(opacity))
		op.Blend = blend
		op.Filter = ebiten.FilterLinear
		dst.DrawImage(off, &op)
		r.pool.put(off)
	case 2:
		if l.img == nil {
			return
		}
		var op ebiten.DrawImageOptions
		op.GeoM = mat.toGeoM()
		op.ColorScale = cs
		op.ColorScale.ScaleAlpha(float32(opacity))
		op.Blend = blend
		op.Filter = ebiten.FilterLinear
		dst.DrawImage(l.img, &op)
	}
}

// applyMasks combines mask shapes into a coverage image and intersects the
// content with it.
func (r *renderer) applyMasks(content *ebiten.Image, masks []maskNode, lt float64, mat matrix, antialias bool) {
	b := content.Bounds()
	coverage := r.pool.get(b.Dx(), b.Dy())
	for i := range masks {
		m := &masks[i]
		alpha := clamp01(m.opacity.scalarAt(lt, 100) / 100)
		if alpha <= 0 {
			continue
		}
		bez := m.shape.at(lt, &r.maskScratch)
		r.maskPath.Reset()
		bez.appendToPath(&r.maskPath, mat)
		var op vector.DrawPathOptions
		op.AntiAlias = antialias
		switch m.mode {
		case 'a':
			op.ColorScale.Scale(float32(alpha), float32(alpha), float32(alpha), float32(alpha))
		case 's':
			op.ColorScale.ScaleAlpha(float32(alpha))
			op.Blend = ebiten.BlendDestinationOut
		}
		vector.FillPath(coverage, &r.maskPath, &vector.FillOptions{FillRule: vector.FillRuleNonZero}, &op)
	}
	var op ebiten.DrawImageOptions
	op.Blend = ebiten.BlendDestinationIn
	content.DrawImage(coverage, &op)
	r.pool.put(coverage)
}

var (
	lumaShaderOnce sync.Once
	lumaShader     *ebiten.Shader
)

const lumaShaderSrc = `//kage:unit pixels

package main

var Invert float

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	c := imageSrc0At(src)
	l := dot(c.rgb, vec3(0.2126, 0.7152, 0.0722))
	if Invert != 0 {
		l = c.a - l
	}
	return vec4(0, 0, 0, l)
}
`

// combineMatte intersects content with the matte according to the tt mode.
func combineMatte(content, matte *ebiten.Image, mode int) {
	b := content.Bounds()
	switch mode {
	case 1: // alpha
		var op ebiten.DrawImageOptions
		op.Blend = ebiten.BlendDestinationIn
		content.DrawImage(matte, &op)
	case 2: // alpha inverted
		var op ebiten.DrawImageOptions
		op.Blend = ebiten.BlendDestinationOut
		content.DrawImage(matte, &op)
	case 3, 4: // luma, luma inverted
		lumaShaderOnce.Do(func() {
			s, err := ebiten.NewShader([]byte(lumaShaderSrc))
			if err != nil {
				panic("lottie: internal luma shader failed to compile: " + err.Error())
			}
			lumaShader = s
		})
		var op ebiten.DrawRectShaderOptions
		op.Images[0] = matte
		op.Blend = ebiten.BlendDestinationIn
		invert := 0.0
		if mode == 4 {
			invert = 1
		}
		op.Uniforms = map[string]any{"Invert": invert}
		content.DrawRectShader(b.Dx(), b.Dy(), lumaShader, &op)
	}
}

// blendFor maps a Lottie bm value to an Ebitengine blend. Unsupported modes
// fall back to normal (they are reported at decode time).
func blendFor(bm int) ebiten.Blend {
	switch bm {
	case 1: // multiply: src*dst + dst*(1-srcAlpha)
		return ebiten.Blend{
			BlendFactorSourceRGB:        ebiten.BlendFactorDestinationColor,
			BlendFactorSourceAlpha:      ebiten.BlendFactorOne,
			BlendFactorDestinationRGB:   ebiten.BlendFactorOneMinusSourceAlpha,
			BlendFactorDestinationAlpha: ebiten.BlendFactorOneMinusSourceAlpha,
			BlendOperationRGB:           ebiten.BlendOperationAdd,
			BlendOperationAlpha:         ebiten.BlendOperationAdd,
		}
	case 2: // screen: src + dst*(1-srcColor)
		return ebiten.Blend{
			BlendFactorSourceRGB:        ebiten.BlendFactorOne,
			BlendFactorSourceAlpha:      ebiten.BlendFactorOne,
			BlendFactorDestinationRGB:   ebiten.BlendFactorOneMinusSourceColor,
			BlendFactorDestinationAlpha: ebiten.BlendFactorOneMinusSourceAlpha,
			BlendOperationRGB:           ebiten.BlendOperationAdd,
			BlendOperationAlpha:         ebiten.BlendOperationAdd,
		}
	default:
		return ebiten.BlendSourceOver
	}
}

// layerMatrix composes the layer transform with its parent chain. Parents
// contribute transform only, not opacity.
func layerMatrix(l *layerNode, f float64, depth int) matrix {
	m := l.transform.matrixAtOriented(l.localTime(f), l.autoOrient)
	if l.parent != nil && depth < 64 {
		return layerMatrix(l.parent, f, depth+1).mul(m)
	}
	return m
}

// walkShapes evaluates shape items at layer-local frame f, appending
// geometries and draw commands in array order. A style command covers every
// geometry appended within the same group before the style item.
func (r *renderer) walkShapes(nodes []*shapeNode, f float64, mat matrix, opacity float64) {
	groupStart := r.nGeoms
	cmdStart := len(r.cmds)
	for _, n := range nodes {
		switch n.kind {
		case "gr":
			childMat := mat
			childOp := opacity
			if n.transform != nil {
				childMat = mat.mul(n.transform.matrixAt(f))
				childOp *= n.transform.opacityAt(f)
			}
			r.walkShapes(n.children, f, childMat, childOp)
		case "sh":
			src := n.shape.at(f, &r.shapeScratch)
			g := r.nextGeom()
			g.mat = mat
			g.bez.Closed = src.Closed
			g.bez.V = copyPoints(g.bez.V, src.V)
			g.bez.I = copyPoints(g.bez.I, src.I)
			g.bez.O = copyPoints(g.bez.O, src.O)
		case "rc":
			p := n.pos.at(f, r.vecScratch)
			s := n.size.at(f, nil)
			round := n.roundness.scalarAt(f, 0)
			g := r.nextGeom()
			g.mat = mat
			rectShape(&g.bez, at(p, 0), at(p, 1), at(s, 0), at(s, 1), round)
		case "el":
			p := n.pos.at(f, r.vecScratch)
			s := n.size.at(f, nil)
			g := r.nextGeom()
			g.mat = mat
			ellipseShape(&g.bez, at(p, 0), at(p, 1), at(s, 0)/2, at(s, 1)/2)
		case "sr":
			p := n.pos.at(f, r.vecScratch)
			g := r.nextGeom()
			g.mat = mat
			polystarShape(&g.bez, n.starType == 1,
				at(p, 0), at(p, 1),
				n.points.scalarAt(f, 5),
				n.rotation.scalarAt(f, 0),
				n.outerR.scalarAt(f, 0), n.innerR.scalarAt(f, 0),
				n.outerRound.scalarAt(f, 0), n.innerRound.scalarAt(f, 0))
		case "fl", "st", "gf", "gs":
			r.emitStyle(n, f, opacity, groupStart, mat)
		case "tm":
			r.applyTrim(n, f, groupStart)
		case "rd":
			r.applyRoundCorners(n, f, groupStart)
		case "rp":
			r.applyRepeater(n, f, mat, groupStart, cmdStart)
		}
	}
}

// emitStyle appends draw commands for a style node covering the geometry
// collected so far in the group, splitting into runs of equal per-geometry
// alpha so repeater opacity ramps survive styles placed after the repeater.
func (r *renderer) emitStyle(n *shapeNode, f float64, opacity float64, groupStart int, mat matrix) {
	start := groupStart
	for start < r.nGeoms {
		alpha := r.geoms[start].alpha
		end := start + 1
		for end < r.nGeoms && r.geoms[end].alpha == alpha {
			end++
		}
		r.emitStyleRange(n, f, opacity*alpha, start, end, mat)
		start = end
	}
}

func (r *renderer) emitStyleRange(n *shapeNode, f float64, opacity float64, start, end int, mat matrix) {
	var cmd drawCmd
	switch n.kind {
	case "gf", "gs":
		cmd = r.gradientCmd(n, f, opacity, start, end, mat)
	default:
		cmd = r.styleCmd(n, f, opacity, start, end)
	}
	switch n.kind {
	case "fl", "gf":
		cmd.fillRule = vector.FillRuleNonZero
		if n.fillRule == 2 {
			cmd.fillRule = vector.FillRuleEvenOdd
		}
	case "st", "gs":
		cmd.stroke = true
		cmd.strokeOpts = r.strokeOpts(n, f, mat)
		if len(n.dashPattern) > 0 {
			cmd.dashed = true
			cmd.geomStart, cmd.geomEnd = r.buildDashedRange(n, f, start, end)
		}
	}
	r.cmds = append(r.cmds, cmd)
}

func (r *renderer) strokeOpts(n *shapeNode, f float64, mat matrix) vector.StrokeOptions {
	w := n.width.scalarAt(f, 1) * mat.meanScale()
	so := vector.StrokeOptions{
		Width:      float32(w),
		LineCap:    lineCap(n.lineCap),
		LineJoin:   lineJoin(n.lineJoin),
		MiterLimit: float32(n.miter),
	}
	if so.MiterLimit == 0 {
		so.MiterLimit = 4
	}
	return so
}

func at(v []float64, i int) float64 {
	if i < len(v) {
		return v[i]
	}
	return 0
}

func (r *renderer) styleCmd(n *shapeNode, f float64, opacity float64, start, end int) drawCmd {
	c := n.color.at(f, nil)
	cr, cg, cb := at(c, 0), at(c, 1), at(c, 2)
	ca := 1.0
	if len(c) > 3 {
		ca = c[3]
	}
	// Some exporters write colors as 0..255.
	if cr > 1 || cg > 1 || cb > 1 {
		cr /= 255
		cg /= 255
		cb /= 255
		if ca > 1 {
			ca /= 255
		}
	}
	alpha := clamp01(ca * opacity * clamp01(n.opacity.scalarAt(f, 100)/100))
	return drawCmd{
		geomStart: start,
		geomEnd:   end,
		r:         cr, g: cg, b: cb, a: alpha,
		alphaMul: 1,
	}
}

func (r *renderer) execute(dst *ebiten.Image, cmd *drawCmd, cs ebiten.ColorScale, blend ebiten.Blend, antialias bool) {
	if cmd.a*cmd.alphaMul <= 0 || cmd.geomEnd <= cmd.geomStart {
		return
	}
	arr := r.geoms
	if cmd.dashed {
		arr = r.dashGeoms
	}
	r.path.Reset()
	for i := cmd.geomStart; i < cmd.geomEnd; i++ {
		g := &arr[i]
		g.bez.appendToPath(&r.path, g.mat)
	}
	if cmd.grad != nil {
		r.executeGradient(dst, cmd, arr, cs, blend, antialias)
		return
	}
	a := cmd.a * cmd.alphaMul
	var op vector.DrawPathOptions
	op.AntiAlias = antialias
	op.Blend = blend
	// Premultiplied color scale.
	op.ColorScale.Scale(
		float32(cmd.r*a), float32(cmd.g*a), float32(cmd.b*a), float32(a))
	op.ColorScale.ScaleWithColorScale(cs)
	if cmd.stroke {
		so := cmd.strokeOpts
		vector.StrokePath(dst, &r.path, &so, &op)
	} else {
		fo := vector.FillOptions{FillRule: cmd.fillRule}
		vector.FillPath(dst, &r.path, &fo, &op)
	}
}

func lineCap(lc int) vector.LineCap {
	switch lc {
	case 2:
		return vector.LineCapRound
	case 3:
		return vector.LineCapSquare
	default:
		return vector.LineCapButt
	}
}

func lineJoin(lj int) vector.LineJoin {
	switch lj {
	case 2:
		return vector.LineJoinRound
	case 3:
		return vector.LineJoinBevel
	default:
		return vector.LineJoinMiter
	}
}
