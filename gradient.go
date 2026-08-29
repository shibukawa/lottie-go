package lottie

import (
	"image"
	"math"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// maxGradStops is the stop capacity of the gradient shader.
const maxGradStops = 16

// gradientCmd carries an evaluated gradient style for one draw command.
type gradientCmd struct {
	kind   int // 1 linear, 2 radial
	sx, sy float64
	ex, ey float64
	mat    matrix // shape space -> destination space at style time
	count  int
	stops  [maxGradStops]float32
	colors [maxGradStops][4]float32 // premultiplied RGBA
}

var (
	gradShaderOnce sync.Once
	gradShader     *ebiten.Shader
)

const gradShaderSrc = `//kage:unit pixels

package main

var Kind float
var Start vec2
var End vec2
var Count float
var InvA vec4 // inverse matrix [a, c, b, d]
var InvT vec2 // inverse translation
var Stops [16]float
var Colors [16]vec4

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	cov := imageSrc0At(src).a
	// dst.xy is a position on the internal texture, where the destination
	// image can sit anywhere; imageDstOrigin turns it into a position
	// relative to the destination's bounds, which is what InvT is built for.
	q := dst.xy - imageDstOrigin()
	p := vec2(
		InvA.x*q.x+InvA.y*q.y+InvT.x,
		InvA.z*q.x+InvA.w*q.y+InvT.y,
	)
	var t float
	if Kind == 2 {
		d := End - Start
		r := length(d)
		t = length(p-Start) / max(r, 1e-6)
	} else {
		d := End - Start
		t = dot(p-Start, d) / max(dot(d, d), 1e-6)
	}
	t = clamp(t, 0, 1)
	col := Colors[0]
	for i := 0; i < 15; i++ {
		if float(i) < Count-1 {
			t0 := Stops[i]
			t1 := Stops[i+1]
			f := clamp((t-t0)/max(t1-t0, 1e-6), 0, 1)
			col = mix(col, Colors[i+1], f)
		}
	}
	return col * cov * color
}
`

// gradientCmd evaluates a gf/gs node into a draw command. The command
// struct comes from the renderer's arena, reset alongside the geometry
// arenas, so evaluating gradients every frame stays off the heap.
func (r *renderer) gradientCmd(n *shapeNode, f float64, opacity float64, start, end int, mat matrix) drawCmd {
	alpha := clamp01(opacity * clamp01(n.opacity.scalarAt(f, 100)/100))
	g := r.nextGradCmd()
	g.kind, g.mat = n.gradKind, mat
	s := n.gradStart.atInto(f, &r.gradStartB)
	e := n.gradEnd.atInto(f, &r.gradEndB)
	g.sx, g.sy = at(s, 0), at(s, 1)
	g.ex, g.ey = at(e, 0), at(e, 1)
	r.gradAlphas = buildGradientStops(g, n.gradStops.atInto(f, &r.gradStopsB), n.gradStopCount, alpha, r.gradAlphas)
	return drawCmd{
		geomStart: start,
		geomEnd:   end,
		a:         alpha,
		alphaMul:  1,
		grad:      g,
	}
}

// alphaStop is one (position, alpha) pair of a gradient's opacity tail.
type alphaStop struct{ pos, a float64 }

// buildGradientStops merges Lottie's flattened color stops (pos,r,g,b)*count
// plus optional trailing alpha stops (pos,a)* into premultiplied RGBA stops.
// scratch is reused across calls and the grown slice is returned.
func buildGradientStops(g *gradientCmd, data []float64, count int, alpha float64, scratch []alphaStop) []alphaStop {
	if count <= 0 || len(data) < count*4 {
		// Malformed: fall back to opaque black -> transparent.
		g.count = 2
		g.stops = [maxGradStops]float32{0, 1}
		g.colors[0] = [4]float32{0, 0, 0, float32(alpha)}
		g.colors[1] = [4]float32{0, 0, 0, float32(alpha)}
		return scratch
	}
	// The alpha tail starts after the file's full color-stop run; slice it
	// before clamping count, or extra color stops read back as alpha stops.
	alphas := scratch[:0]
	rest := data[count*4:]
	if count > maxGradStops {
		count = maxGradStops
	}
	for i := 0; i+1 < len(rest); i += 2 {
		alphas = append(alphas, alphaStop{rest[i], rest[i+1]})
	}
	alphaAt := func(pos float64) float64 {
		if len(alphas) == 0 {
			return 1
		}
		if pos <= alphas[0].pos {
			return alphas[0].a
		}
		for i := 0; i < len(alphas)-1; i++ {
			a0, a1 := alphas[i], alphas[i+1]
			if pos <= a1.pos {
				span := a1.pos - a0.pos
				if span <= 0 {
					return a1.a
				}
				u := (pos - a0.pos) / span
				return a0.a + (a1.a-a0.a)*u
			}
		}
		return alphas[len(alphas)-1].a
	}
	g.count = count
	for i := 0; i < count; i++ {
		pos := data[i*4]
		cr, cg, cb := data[i*4+1], data[i*4+2], data[i*4+3]
		a := alphaAt(pos) * alpha
		g.stops[i] = float32(pos)
		g.colors[i] = [4]float32{
			float32(cr * a), float32(cg * a), float32(cb * a), float32(a),
		}
	}
	return alphas
}

// nextGradCmd hands out a gradientCmd from the arena, growing it on
// demand. Slots are reclaimed when the geometry arenas reset; a command
// whose pointer predates a growth keeps reading its old backing, which
// still holds the finished value — commands are never mutated after build.
func (r *renderer) nextGradCmd() *gradientCmd {
	if r.nGrad == len(r.gradCmds) {
		r.gradCmds = append(r.gradCmds, gradientCmd{})
	}
	g := &r.gradCmds[r.nGrad]
	r.nGrad++
	return g
}

// geomControlBounds bounds the commands' geometry in device space from the
// bezier control points — a superset of the curves themselves, which only
// costs a little area on the coverage mask.
func geomControlBounds(arr []geometry, start, end int) (image.Rectangle, bool) {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for i := start; i < end; i++ {
		g := &arr[i]
		for j, v := range g.bez.V {
			px, py := v[0], v[1]
			for _, pt := range [3][2]float64{
				{px, py},
				{px + tangentAt(g.bez.I, j)[0], py + tangentAt(g.bez.I, j)[1]},
				{px + tangentAt(g.bez.O, j)[0], py + tangentAt(g.bez.O, j)[1]},
			} {
				x, y := g.mat.apply(pt[0], pt[1])
				minX, minY = math.Min(minX, x), math.Min(minY, y)
				maxX, maxY = math.Max(maxX, x), math.Max(maxY, y)
			}
		}
	}
	if minX > maxX {
		return image.Rectangle{}, false
	}
	return image.Rect(int(math.Floor(minX)), int(math.Floor(minY)),
		int(math.Ceil(maxX)), int(math.Ceil(maxY))), true
}

// executeGradient renders the commands' geometry as a coverage mask and
// shades it with the gradient shader. arr is the geometry array the
// command indexes; the path is built once, directly in mask space.
func (r *renderer) executeGradient(dst *ebiten.Image, cmd *drawCmd, arr []geometry, cs ebiten.ColorScale, blend ebiten.Blend, antialias bool) {
	region, ok := geomControlBounds(arr, cmd.geomStart, cmd.geomEnd)
	if !ok {
		return
	}
	if cmd.stroke {
		pad := int(cmd.strokeOpts.Width) + 2
		region = region.Inset(-pad)
	} else {
		region = region.Inset(-2)
	}
	region = region.Intersect(dst.Bounds())
	if region.Empty() {
		return
	}

	// Bail out before taking an offscreen. Returning an unread offscreen to
	// the pool would strand the fill that was rasterized into it: the vector
	// package defers a fill until its target is next used, so an offscreen
	// that goes back unused carries that pending work into whichever layer
	// borrows it next.
	grad := cmd.grad
	inv, ok := grad.mat.invert()
	if !ok {
		return
	}

	w, h := region.Dx(), region.Dy()
	mask, maskBase := sharedPool.get(w, h)

	// Rebuild the path shifted into mask space.
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

	gradShaderOnce.Do(func() {
		s, err := ebiten.NewShader([]byte(gradShaderSrc))
		if err != nil {
			panic("lottie: internal gradient shader failed to compile: " + err.Error())
		}
		gradShader = s
	})

	// The shader receives positions relative to the destination's bounds
	// (see gradShaderSrc), so fold the bounds origin into the inverse
	// translation: p = inv·(q + min) = inv·q + (inv·min).
	ox := float64(dst.Bounds().Min.X)
	oy := float64(dst.Bounds().Min.Y)
	invTX, invTY := inv.apply(ox, oy)

	if r.gradStopsU == nil {
		r.gradStopsU = make([]float32, maxGradStops)
		r.gradColors = make([]float32, maxGradStops*4)
	}
	stops, colors := r.gradStopsU, r.gradColors
	for i := 0; i < grad.count; i++ {
		stops[i] = grad.stops[i]
		copy(colors[i*4:], grad.colors[i][:])
	}
	// Pad the tail so the cumulative mix in the shader stays at the last color.
	for i := grad.count; i < maxGradStops; i++ {
		stops[i] = 1
		if grad.count > 0 {
			copy(colors[i*4:], grad.colors[grad.count-1][:])
		}
	}

	// The repeater opacity ramp rides on the vertex color (stop colors
	// already carry the style opacity).
	am := float32(cmd.alphaMul)
	vs := r.gradVerts[:]
	for i := range vs {
		vs[i].ColorR = cs.R() * am
		vs[i].ColorG = cs.G() * am
		vs[i].ColorB = cs.B() * am
		vs[i].ColorA = cs.A() * am
	}
	x0, y0 := float32(region.Min.X), float32(region.Min.Y)
	x1, y1 := float32(region.Max.X), float32(region.Max.Y)
	fw, fh := float32(w), float32(h)
	vs[0].DstX, vs[0].DstY, vs[0].SrcX, vs[0].SrcY = x0, y0, 0, 0
	vs[1].DstX, vs[1].DstY, vs[1].SrcX, vs[1].SrcY = x1, y0, fw, 0
	vs[2].DstX, vs[2].DstY, vs[2].SrcX, vs[2].SrcY = x0, y1, 0, fh
	vs[3].DstX, vs[3].DstY, vs[3].SrcX, vs[3].SrcY = x1, y1, fw, fh

	// The uniform map and its slices are renderer-owned scratch, refilled
	// per draw: Ebitengine copies uniform values during the call, so the
	// backing arrays are free again the moment it returns.
	if r.gradUnis == nil {
		r.gradUnis = map[string]any{
			"Start": make([]float32, 2),
			"End":   make([]float32, 2),
			"InvA":  make([]float32, 4),
			"InvT":  make([]float32, 2),
		}
	}
	u := r.gradUnis
	u["Kind"] = float32(grad.kind)
	u["Count"] = float32(grad.count)
	fill2 := func(key string, a, b float64) {
		s := u[key].([]float32)
		s[0], s[1] = float32(a), float32(b)
	}
	fill2("Start", grad.sx, grad.sy)
	fill2("End", grad.ex, grad.ey)
	ia := u["InvA"].([]float32)
	ia[0], ia[1], ia[2], ia[3] = float32(inv.A), float32(inv.C), float32(inv.B), float32(inv.D)
	fill2("InvT", invTX, invTY)
	u["Stops"] = stops
	u["Colors"] = colors

	var top ebiten.DrawTrianglesShaderOptions
	top.Images[0] = mask
	top.Blend = blend
	top.Uniforms = u
	dst.DrawTrianglesShader(vs, gradIndices[:], gradShader, &top)
	sharedPool.put(maskBase)
}

var gradIndices = [6]uint16{0, 1, 2, 1, 3, 2}
