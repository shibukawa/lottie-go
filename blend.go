package lottie

import (
	"image"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

// blendNeedsShader reports whether the Lottie blend mode reads the backdrop
// and therefore composites through blendShader rather than a fixed-function
// ebiten.Blend: 1 multiply, 3 overlay, 4 darken, 5 lighten, 6 color dodge,
// 7 color burn, 8 hard light, 9 soft light, 10 difference, 11 exclusion.
// Multiply is here because its W3C form keeps the source visible over a
// transparent backdrop — the src*(1-dstAlpha) term — which a one-pass
// fixed-function blend cannot express.
func blendNeedsShader(bm int) bool { return bm == 1 || bm >= 3 && bm <= 11 }

var (
	blendShaderOnce sync.Once
	blendShader     *ebiten.Shader
)

// blendShaderSrc implements the separable W3C compositing blend modes on
// premultiplied sources: src0 is the layer content, src1 the backdrop. The
// result fully replaces the destination rect (draw it with BlendCopy).
const blendShaderSrc = `//kage:unit pixels

package main

var Mode float
var Scale vec4

func screenB(b, s vec3) vec3 {
	return b + s - b*s
}

// hardLightB is HardLight(Cb, Cs); Overlay is the same with operands swapped.
func hardLightB(b, s vec3) vec3 {
	return mix(2*b*s, screenB(b, 2*s-1), step(vec3(0.5), s))
}

func softLightB(b, s vec3) vec3 {
	d := mix(((16*b-12)*b+4)*b, sqrt(b), step(vec3(0.25), b))
	lo := b - (1-2*s)*b*(1-b)
	hi := b + (2*s-1)*(d-b)
	return mix(lo, hi, step(vec3(0.5), s))
}

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	s := imageSrc0At(src) * Scale
	b := imageSrc1At(src)
	sc := s.rgb / max(s.a, 0.0001)
	bc := b.rgb / max(b.a, 0.0001)
	m := int(Mode)
	var f vec3
	if m == 1 {
		f = bc * sc
	} else if m == 3 {
		f = hardLightB(sc, bc)
	} else if m == 4 {
		f = min(bc, sc)
	} else if m == 5 {
		f = max(bc, sc)
	} else if m == 6 {
		f = min(vec3(1), bc/max(1-sc, vec3(0.0001)))
	} else if m == 7 {
		f = 1 - min(vec3(1), (1-bc)/max(sc, vec3(0.0001)))
	} else if m == 8 {
		f = hardLightB(bc, sc)
	} else if m == 9 {
		f = softLightB(bc, sc)
	} else if m == 10 {
		f = abs(bc - sc)
	} else {
		f = bc + sc - 2*bc*sc
	}
	rgb := s.rgb*(1-b.a) + b.rgb*(1-s.a) + s.a*b.a*f
	return vec4(rgb, s.a+b.a*(1-s.a))
}
`

func ensureBlendShader() {
	blendShaderOnce.Do(func() {
		s, err := ebiten.NewShader([]byte(blendShaderSrc))
		if err != nil {
			panic("lottie: internal blend shader failed to compile: " + err.Error())
		}
		blendShader = s
	})
}

// compositeBlend composites content over the bounds region of dst with a
// backdrop-sampling blend mode. dst must not be the final screen image, which
// Ebitengine forbids reading; renderer.render keeps root-level shader blends
// off the screen by wrapping the animation in an offscreen.
func compositeBlend(dst, content *ebiten.Image, bounds image.Rectangle, bm int, opacity float64, cs ebiten.ColorScale) {
	ensureBlendShader()
	w, h := bounds.Dx(), bounds.Dy()
	backdrop, backdropBase := sharedPool.get(w, h)
	// A sub-image source draws with its region's upper-left at the origin,
	// so no translation is needed to land it at backdrop's (0, 0).
	var cp ebiten.DrawImageOptions
	cp.Blend = ebiten.BlendCopy
	backdrop.DrawImage(dst.SubImage(bounds).(*ebiten.Image), &cp)

	a := float32(opacity)
	var op ebiten.DrawRectShaderOptions
	op.GeoM.Translate(float64(bounds.Min.X), float64(bounds.Min.Y))
	op.Images[0] = content
	op.Images[1] = backdrop
	op.Blend = ebiten.BlendCopy
	op.Uniforms = map[string]any{
		"Mode":  float32(bm),
		"Scale": []float32{cs.R() * a, cs.G() * a, cs.B() * a, cs.A() * a},
	}
	dst.DrawRectShader(w, h, blendShader, &op)
	sharedPool.put(backdropBase)
}
