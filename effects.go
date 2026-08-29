package lottie

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

// Effect type ids from the Lottie schema.
const (
	effectCustom     = 5
	effectTint       = 20
	effectFill       = 21
	effectTritone    = 23
	effectDropShadow = 25
	effectBlur       = 29
)

// blurrinessToSigma converts AE blurriness to a Gaussian sigma, matching
// lottie-web, and shadowSoftnessToSigma does the same for drop-shadow
// softness.
const (
	blurrinessToSigma     = 0.3
	shadowSoftnessToSigma = 0.25
)

// effectNode is one compiled layer effect. params holds the effect's
// parameters by schema index; entries the file omits stay nil. divs are
// the per-parameter color divisors decided from the authored keyframes.
type effectNode struct {
	kind   int
	params []*vectorTrack
	divs   [][2]float64
}

// scalar evaluates the i-th parameter at layer-local frame f.
func (e *effectNode) scalar(i int, f, def float64) float64 {
	if i >= len(e.params) || e.params[i] == nil {
		return def
	}
	return e.params[i].scalarAt(f, def)
}

// colorAt returns the i-th parameter as r, g, b, a components in 0..1.
func (e *effectNode) colorAt(i int, f float64) (r, g, b, a float64) {
	if i >= len(e.params) || e.params[i] == nil {
		return 0, 0, 0, 1
	}
	c := e.params[i].at(f, nil)
	r, g, b, a = at(c, 0), at(c, 1), at(c, 2), 1
	if len(c) > 3 {
		a = c[3]
	}
	// Some exporters write colors as 0..255; like styleCmd, the divisor
	// was decided at build time from the authored keyframes so an eased
	// overshoot past 1.0 mid-segment is left alone.
	if i < len(e.divs) {
		if d := e.divs[i][0]; d > 1 {
			r, g, b = r/d, g/d, b/d
		}
		if d := e.divs[i][1]; d > 1 {
			a /= d
		}
	}
	return r, g, b, a
}

// buildEffects compiles a layer's effect list, keeping the kinds this
// renderer implements and recording the rest as unsupported.
func (b *builder) buildEffects(raw json.RawMessage) []effectNode {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "[]" {
		return nil
	}
	var raws []rawEffect
	if err := json.Unmarshal(raw, &raws); err != nil {
		b.anim.note("unparsable effects")
		return nil
	}
	var out []effectNode
	for i := range raws {
		re := &raws[i]
		if re.Enabled != nil && *re.Enabled == 0 {
			continue
		}
		switch re.Type {
		case effectTint, effectFill, effectTritone, effectDropShadow, effectBlur:
			n := effectNode{kind: re.Type}
			for j := range re.Values {
				it := &re.Values[j]
				if it.V == nil {
					n.params = append(n.params, nil)
					n.divs = append(n.divs, [2]float64{1, 1})
					continue
				}
				tr := b.vectorProp(it.V, "effect parameter", nil)
				n.params = append(n.params, tr)
				rgb, a := colorDivisors(tr)
				n.divs = append(n.divs, [2]float64{rgb, a})
			}
			out = append(out, n)
		case effectCustom:
			// Expression controls; they carry values for expressions and do
			// not render on their own.
		default:
			if re.Name != "" {
				b.anim.note(fmt.Sprintf("effect %q", re.Name))
			} else {
				b.anim.note(fmt.Sprintf("effect type %d", re.Type))
			}
		}
	}
	return out
}

// effectPad returns how far, in device pixels, the layer's effects may push
// pixels beyond its base bounds.
func effectPad(l *layerNode, lt, scale float64) float64 {
	pad := 0.0
	for i := range l.effects {
		e := &l.effects[i]
		switch e.kind {
		case effectBlur:
			pad += 3 * e.scalar(0, lt, 0) * blurrinessToSigma * scale
		case effectDropShadow:
			pad += (e.scalar(3, lt, 5) + 3*e.scalar(4, lt, 0)*shadowSoftnessToSigma) * scale
		}
	}
	return pad
}

// applyEffects runs the layer's effects in file order on its flattened
// content. mat supplies the device scale for pixel-sized parameters.
func (r *renderer) applyEffects(content *ebiten.Image, l *layerNode, lt float64, mat matrix) {
	scale := mat.meanScale()
	for i := range l.effects {
		e := &l.effects[i]
		switch e.kind {
		case effectBlur:
			sigma := e.scalar(0, lt, 0) * blurrinessToSigma * scale
			sx, sy := sigma, sigma
			switch int(e.scalar(1, lt, 1)) {
			case 2: // horizontal only
				sy = 0
			case 3: // vertical only
				sx = 0
			}
			gaussianBlur(content, sx, sy)
		case effectDropShadow:
			applyDropShadow(content, e, lt, mat)
		case effectFill:
			cr, cg, cb, ca := e.colorAt(2, lt)
			opacity := clamp01(e.scalar(6, lt, 1)) * ca
			shaderPass(content, fillEffectShader(), map[string]any{
				"Color": []float32{float32(cr), float32(cg), float32(cb), float32(opacity)},
			})
		case effectTint:
			br, bg, bb, _ := e.colorAt(0, lt)
			wr, wg, wb, _ := e.colorAt(1, lt)
			amount := clamp01(e.scalar(2, lt, 100) / 100)
			shaderPass(content, tintEffectShader(), map[string]any{
				"Black":  []float32{float32(br), float32(bg), float32(bb)},
				"White":  []float32{float32(wr), float32(wg), float32(wb)},
				"Amount": float32(amount),
			})
		case effectTritone:
			hr, hg, hb, _ := e.colorAt(0, lt)
			mr, mg, mb, _ := e.colorAt(1, lt)
			dr, dg, db, _ := e.colorAt(2, lt)
			blend := clamp01(e.scalar(3, lt, 0) / 100)
			shaderPass(content, tritoneEffectShader(), map[string]any{
				"Bright": []float32{float32(hr), float32(hg), float32(hb)},
				"Mid":    []float32{float32(mr), float32(mg), float32(mb)},
				"Dark":   []float32{float32(dr), float32(dg), float32(db)},
				"Blend":  float32(blend),
			})
		}
	}
}

// applyDropShadow softens a colorized copy of the content's alpha and slides
// it underneath, offset by the effect's direction and distance in layer
// space.
func applyDropShadow(content *ebiten.Image, e *effectNode, lt float64, mat matrix) {
	cr, cg, cb, ca := e.colorAt(0, lt)
	opacity := clamp01(e.scalar(1, lt, 128)/255) * ca
	if opacity <= 0 {
		return
	}
	dirDeg := e.scalar(2, lt, 135)
	dist := e.scalar(3, lt, 5)
	softness := e.scalar(4, lt, 0)
	// The offset points down the AE angle (0 degrees is up, clockwise) and
	// rotates and scales with the layer through the linear part of mat.
	rad := dirDeg * math.Pi / 180
	lx, ly := math.Sin(rad)*dist, -math.Cos(rad)*dist
	dx := mat.A*lx + mat.C*ly
	dy := mat.B*lx + mat.D*ly

	b := content.Bounds()
	w, h := b.Dx(), b.Dy()
	shadow, shadowBase := sharedPool.get(w, h)
	shaderPassInto(shadow, content, fillEffectShader(), map[string]any{
		"Color": []float32{float32(cr), float32(cg), float32(cb), float32(opacity)},
	})
	sigma := softness * shadowSoftnessToSigma * mat.meanScale()
	gaussianBlur(shadow, sigma, sigma)

	tmp, tmpBase := sharedPool.get(w, h)
	var sop ebiten.DrawImageOptions
	sop.GeoM.Translate(dx, dy)
	tmp.DrawImage(shadow, &sop)
	// content draws from its region origin, so no compensating translation.
	var cop ebiten.DrawImageOptions
	tmp.DrawImage(content, &cop)
	var back ebiten.DrawImageOptions
	back.GeoM.Translate(float64(b.Min.X), float64(b.Min.Y))
	back.Blend = ebiten.BlendCopy
	content.DrawImage(tmp, &back)
	sharedPool.put(tmpBase)
	sharedPool.put(shadowBase)
}

// blurMaxSigma is the largest sigma the fixed blur kernel covers per pass
// (blurTaps of 24 spans three sigmas); larger blurs run on a downscaled
// image.
const blurMaxSigma = 8.0

// gaussianBlur blurs img in place with a separable Gaussian. Sigmas below
// the visible threshold skip their pass.
func gaussianBlur(img *ebiten.Image, sx, sy float64) {
	if sx < 0.1 && sy < 0.1 {
		return
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	cur := img
	cw, ch := w, h
	var bases []*ebiten.Image
	// Halve the image until the largest sigma fits the kernel; blurring the
	// small image then costs a fixed kernel regardless of radius.
	for math.Max(sx, sy) > blurMaxSigma && cw > 1 && ch > 1 {
		nw, nh := (cw+1)/2, (ch+1)/2
		half, halfBase := sharedPool.get(nw, nh)
		var op ebiten.DrawImageOptions
		op.GeoM.Scale(0.5, 0.5)
		op.Filter = ebiten.FilterLinear
		half.DrawImage(cur, &op)
		bases = append(bases, halfBase)
		cur, cw, ch = half, nw, nh
		sx /= 2
		sy /= 2
	}
	tmp, tmpBase := sharedPool.get(cw, ch)
	blurPass(tmp, cur, sx, 1, 0)
	blurPass(cur, tmp, sy, 0, 1)
	sharedPool.put(tmpBase)
	if cur != img {
		var op ebiten.DrawImageOptions
		op.GeoM.Scale(float64(w)/float64(cw), float64(h)/float64(ch))
		op.GeoM.Translate(float64(b.Min.X), float64(b.Min.Y))
		op.Filter = ebiten.FilterLinear
		op.Blend = ebiten.BlendCopy
		img.DrawImage(cur, &op)
	}
	for _, base := range bases {
		sharedPool.put(base)
	}
}

// blurPass runs one 1D Gaussian pass src -> dst, or a plain copy when the
// sigma is negligible.
func blurPass(dst, src *ebiten.Image, sigma, dirX, dirY float64) {
	if sigma < 0.1 {
		var op ebiten.DrawImageOptions
		op.GeoM.Translate(float64(dst.Bounds().Min.X), float64(dst.Bounds().Min.Y))
		op.Blend = ebiten.BlendCopy
		dst.DrawImage(src, &op)
		return
	}
	shaderPassInto(dst, src, blurEffectShader(), map[string]any{
		"Sigma": float32(math.Min(sigma, blurMaxSigma)),
		"Dir":   []float32{float32(dirX), float32(dirY)},
	})
}

// shaderPass applies a single-source shader to content in place through a
// pooled temporary.
func shaderPass(content *ebiten.Image, shader *ebiten.Shader, uniforms map[string]any) {
	b := content.Bounds()
	tmp, tmpBase := sharedPool.get(b.Dx(), b.Dy())
	shaderPassInto(tmp, content, shader, uniforms)
	var op ebiten.DrawImageOptions
	op.GeoM.Translate(float64(b.Min.X), float64(b.Min.Y))
	op.Blend = ebiten.BlendCopy
	content.DrawImage(tmp, &op)
	sharedPool.put(tmpBase)
}

// shaderPassInto draws src through the shader into dst. Both must be the
// same size.
func shaderPassInto(dst, src *ebiten.Image, shader *ebiten.Shader, uniforms map[string]any) {
	sb := src.Bounds()
	var op ebiten.DrawRectShaderOptions
	op.GeoM.Translate(float64(dst.Bounds().Min.X), float64(dst.Bounds().Min.Y))
	op.Images[0] = src
	op.Blend = ebiten.BlendCopy
	op.Uniforms = uniforms
	dst.DrawRectShader(sb.Dx(), sb.Dy(), shader, &op)
}

// Effect shaders, compiled on first use.
var (
	effectShaderOnce sync.Once
	fillShaderVal    *ebiten.Shader
	tintShaderVal    *ebiten.Shader
	tritoneShaderVal *ebiten.Shader
	blurShaderVal    *ebiten.Shader
)

func compileEffectShaders() {
	effectShaderOnce.Do(func() {
		compile := func(src string) *ebiten.Shader {
			s, err := ebiten.NewShader([]byte(src))
			if err != nil {
				panic("lottie: internal effect shader failed to compile: " + err.Error())
			}
			return s
		}
		fillShaderVal = compile(fillShaderSrc)
		tintShaderVal = compile(tintShaderSrc)
		tritoneShaderVal = compile(tritoneShaderSrc)
		blurShaderVal = compile(blurShaderSrc)
	})
}

func fillEffectShader() *ebiten.Shader    { compileEffectShaders(); return fillShaderVal }
func tintEffectShader() *ebiten.Shader    { compileEffectShaders(); return tintShaderVal }
func tritoneEffectShader() *ebiten.Shader { compileEffectShaders(); return tritoneShaderVal }
func blurEffectShader() *ebiten.Shader    { compileEffectShaders(); return blurShaderVal }

// fillShaderSrc flattens the source's alpha to one color; Color holds
// straight rgb and an overall opacity in a.
const fillShaderSrc = `//kage:unit pixels

package main

var Color vec4

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	a := imageSrc0At(src).a * Color.a
	return vec4(Color.rgb*a, a)
}
`

// tintShaderSrc maps luminance onto the Black..White ramp, mixed with the
// original by Amount.
const tintShaderSrc = `//kage:unit pixels

package main

var Black vec3
var White vec3
var Amount float

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	c := imageSrc0At(src)
	sc := c.rgb / max(c.a, 0.0001)
	l := dot(sc, vec3(0.2126, 0.7152, 0.0722))
	rgb := mix(sc, mix(Black, White, l), Amount)
	return vec4(rgb*c.a, c.a)
}
`

// tritoneShaderSrc maps luminance onto the Dark..Mid..Bright ramp, mixed
// back toward the original by Blend.
const tritoneShaderSrc = `//kage:unit pixels

package main

var Bright vec3
var Mid vec3
var Dark vec3
var Blend float

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	c := imageSrc0At(src)
	sc := c.rgb / max(c.a, 0.0001)
	l := dot(sc, vec3(0.2126, 0.7152, 0.0722))
	var t vec3
	if l < 0.5 {
		t = mix(Dark, Mid, 2*l)
	} else {
		t = mix(Mid, Bright, 2*l-1)
	}
	rgb := mix(t, sc, Blend)
	return vec4(rgb*c.a, c.a)
}
`

// blurShaderSrc is one 1D pass of a separable Gaussian with a fixed 24-tap
// radius; weights follow the Sigma uniform and normalize on the fly.
// Sampling outside the source is transparent, which decays edges naturally.
const blurShaderSrc = `//kage:unit pixels

package main

var Sigma float
var Dir vec2

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	sum := imageSrc0At(src)
	total := 1.0
	for i := 1; i <= 24; i++ {
		w := exp(-float(i*i) / (2 * Sigma * Sigma))
		sum += (imageSrc0At(src+Dir*float(i)) + imageSrc0At(src-Dir*float(i))) * w
		total += 2 * w
	}
	return sum / total
}
`
