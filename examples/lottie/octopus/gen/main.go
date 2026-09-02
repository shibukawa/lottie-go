// Command gen writes the octopus sample: three procedural textures, the
// swim clip, its texture document, and the dotLottie bundle that carries
// them all (examples/lottie/octopus/octopus.lottie). Loose copies of the
// clip, the document and the PNGs land under assets/ for inspection.
//
// The character is a soft body drawn with the texture extension
// (plugin/texture): the head is a keyframed blob whose spotted skin maps
// by bounding box, the eight arms are keyframed tapered paths with a
// whole-arm texture mapped per vertex — dark at the mantle, pale at the
// tip, suckers riding every bend — and
// the kelp in the background is stroked paths with a frond texture mapped
// along the stroke. Everything is generated here, so there is no
// third-party art to license.
//
// Regenerate with:
//
//	go run ./examples/lottie/octopus/gen
package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"

	lottie "github.com/shibukawa/lottie-go"
	lottietexture "github.com/shibukawa/lottie-go/plugin/texture"
)

type obj = map[string]any

const (
	compW, compH = 480.0, 360.0
	fps          = 30.0
	loopFrames   = 90.0 // three seconds; every track returns to its frame-0 value

	headCX, headCY = 240.0, 136.0
	headRX, headRY = 98.0, 88.0
)

// ---- Lottie builders ----

func static(v any) obj { return obj{"a": 0, "k": v} }

func keys(frames ...obj) obj { return obj{"a": 1, "k": frames} }

// keyL is a linear keyframe: the looping waves must not slow at their keys.
func keyL(t float64, v any) obj {
	return obj{"t": t, "s": v, "o": obj{"x": []float64{0.333}, "y": []float64{0.333}},
		"i": obj{"x": []float64{0.667}, "y": []float64{0.667}}}
}

// keyE eases in and out, for the bobbing that should breathe.
func keyE(t float64, v any) obj {
	return obj{"t": t, "s": v, "o": obj{"x": []float64{0.4}, "y": []float64{0}},
		"i": obj{"x": []float64{0.6}, "y": []float64{1}}}
}

// keyH holds its value until the next keyframe.
func keyH(t float64, v any) obj { return obj{"t": t, "s": v, "h": 1} }

func end(t float64, v any) obj { return obj{"t": t, "s": v} }

func vec(x, y float64) []float64 { return []float64{x, y} }

// pathData is one bezier path as Lottie stores it: absolute vertices with
// relative in/out tangents.
type pathData struct {
	closed  bool
	v, i, o [][2]float64
}

func (p pathData) obj() obj {
	enc := func(pts [][2]float64) [][]float64 {
		out := make([][]float64, len(pts))
		for k, pt := range pts {
			out[k] = []float64{round1(pt[0]), round1(pt[1])}
		}
		return out
	}
	return obj{"c": p.closed, "v": enc(p.v), "i": enc(p.i), "o": enc(p.o)}
}

func round1(x float64) float64 { return math.Round(x*10) / 10 }

// pathKeys keyframes a path linearly through the given shapes, the last one
// at the loop's end.
func pathKeys(shapes []pathData) obj {
	frames := make([]obj, len(shapes))
	for k, p := range shapes {
		t := loopFrames * float64(k) / float64(len(shapes)-1)
		if k == len(shapes)-1 {
			frames[k] = end(t, []obj{p.obj()})
		} else {
			frames[k] = keyL(t, []obj{p.obj()})
		}
	}
	return keys(frames...)
}

func shapeItem(name string, ks obj) obj { return obj{"ty": "sh", "nm": name, "ks": ks} }

func fill(name string, rgb []float64) obj {
	return obj{"ty": "fl", "nm": name, "c": static(rgb), "o": static(100.0), "r": 1}
}

func fillOpacity(name string, rgb []float64, opacity float64) obj {
	f := fill(name, rgb)
	f["o"] = static(opacity)
	return f
}

func stroke(name string, rgb []float64, w float64) obj {
	return obj{"ty": "st", "nm": name, "c": static(rgb), "o": static(100.0),
		"w": static(w), "lc": 2, "lj": 2}
}

func ellipse(name string, p obj, w, h float64) obj {
	return obj{"ty": "el", "nm": name, "p": p, "s": static(vec(w, h))}
}

func transformItem() obj {
	return obj{"ty": "tr", "p": static(vec(0, 0)), "a": static(vec(0, 0)),
		"s": static(vec(100, 100)), "r": static(0.0), "o": static(100.0)}
}

func group(name string, items ...obj) obj {
	return obj{"ty": "gr", "nm": name, "it": append(items, transformItem())}
}

func shapeLayer(name string, ind int, ks obj, shapes []obj) obj {
	if ks == nil {
		ks = obj{"a": static([]float64{0, 0, 0}), "p": static([]float64{0, 0, 0}),
			"s": static([]float64{100, 100, 100}), "r": static(0.0), "o": static(100.0)}
	}
	return obj{"ty": 4, "nm": name, "ind": ind, "ip": 0.0, "op": loopFrames, "st": 0.0, "sr": 1,
		"ks": ks, "shapes": shapes}
}

func solidLayer(name string, ind int, hex string) obj {
	return obj{"ty": 1, "nm": name, "ind": ind, "ip": 0.0, "op": loopFrames, "st": 0.0, "sr": 1,
		"ks": obj{"a": static([]float64{0, 0, 0}), "p": static([]float64{0, 0, 0}),
			"s": static([]float64{100, 100, 100}), "r": static(0.0), "o": static(100.0)},
		"sw": compW, "sh": compH, "sc": hex}
}

// smoothTangents gives a polyline the in/out tangents of a Catmull-Rom
// spline scaled by k; corners lists vertices left sharp.
func smoothTangents(v [][2]float64, closed bool, k float64, corners ...int) (in, out [][2]float64) {
	n := len(v)
	in, out = make([][2]float64, n), make([][2]float64, n)
	sharp := map[int]bool{}
	for _, c := range corners {
		sharp[c] = true
	}
	for j := range v {
		if sharp[j] || (!closed && (j == 0 || j == n-1)) {
			continue
		}
		prev, next := v[(j-1+n)%n], v[(j+1)%n]
		tx, ty := (next[0]-prev[0])*k, (next[1]-prev[1])*k
		out[j] = [2]float64{tx, ty}
		in[j] = [2]float64{-tx, -ty}
	}
	return in, out
}

// ---- the character ----

// headShape is the mantle: an ellipse flattened underneath, squashed by f
// (1 at rest) so the bounding-box mapping visibly stretches the skin.
func headShape(f float64) pathData {
	const n = 10
	v := make([][2]float64, n)
	for k := range v {
		th := 2*math.Pi*float64(k)/n - math.Pi/2
		ry := headRY * (2 - f)
		if math.Sin(th) > 0 { // the lower half, in screen coordinates
			ry *= 0.78
		}
		v[k] = [2]float64{headCX + headRX*f*math.Cos(th), headCY + ry*math.Sin(th)}
	}
	in, out := smoothTangents(v, true, 1.0/6)
	return pathData{closed: true, v: v, i: in, o: out}
}

// arm describes one tentacle's rest pose.
type arm struct {
	baseX, baseY float64 // where it leaves the mantle
	angle        float64 // radians; +π/2 points down the screen
	curl         float64 // radians of bend accumulated toward the tip
	length       float64
	width        float64 // at the base
	phase        float64 // wave offset so the arms do not move in unison
}

// armOutline traces the arm at wave phase ph: a centerline integrated
// from the base with the bend and an angular wave, then offset both ways
// by a width that tapers to the tip. The outline runs down the left side,
// around the tip and back up the right, so UV runs along the arm in u (0 at
// the mantle, 1 at the tip) and across it in v — v 0 the left edge, 1 the
// right.
func armOutline(a arm, ph float64) (pathData, [][2]float64) {
	const n = 8
	c := make([][2]float64, n)
	c[0] = [2]float64{a.baseX, a.baseY}
	step := a.length / float64(n-1)
	for k := 1; k < n; k++ {
		s := float64(k-1) / float64(n-2)
		ang := a.angle + a.curl*s + 0.42*s*math.Sin(2*math.Pi*s*0.9-ph)
		c[k] = [2]float64{c[k-1][0] + step*math.Cos(ang), c[k-1][1] + step*math.Sin(ang)}
	}
	var left, right, uvL, uvR [][2]float64
	for k := 0; k < n; k++ {
		s := float64(k) / float64(n-1)
		prev, next := c[max(k-1, 0)], c[min(k+1, n-1)]
		tx, ty := next[0]-prev[0], next[1]-prev[1]
		l := math.Hypot(tx, ty)
		nx, ny := -ty/l, tx/l
		hw := a.width / 2 * (1 - 0.78*s)
		left = append(left, [2]float64{c[k][0] + nx*hw, c[k][1] + ny*hw})
		right = append(right, [2]float64{c[k][0] - nx*hw, c[k][1] - ny*hw})
		uvL = append(uvL, [2]float64{s, 0.06})
		uvR = append(uvR, [2]float64{s, 0.94})
	}
	// Tip: a little past the last centerline point along its tangent.
	tx, ty := c[n-1][0]-c[n-2][0], c[n-1][1]-c[n-2][1]
	l := math.Hypot(tx, ty)
	hw := a.width / 2 * 0.22
	tip := [2]float64{c[n-1][0] + tx/l*hw*1.4, c[n-1][1] + ty/l*hw*1.4}

	v := append([][2]float64{}, left...)
	v = append(v, tip)
	for k := n - 1; k >= 0; k-- {
		v = append(v, right[k])
	}
	uv := append([][2]float64{}, uvL...)
	uv = append(uv, [2]float64{1, 0.5})
	for k := n - 1; k >= 0; k-- {
		uv = append(uv, uvR[k])
	}
	// The base corners hide under the mantle and the tip is pointed; the
	// rest of the outline is smooth.
	in, out := smoothTangents(v, true, 1.0/6, 0, n, 2*n)
	return pathData{closed: true, v: v, i: in, o: out}, uv
}

// arms places the eight arms under the mantle, the outer ones curling out.
func arms() []arm {
	var out []arm
	for k := 0; k < 8; k++ {
		t := (float64(k) + 0.5) / 8 // 0..1 across the underside
		spread := (t - 0.5) * 2     // -1..1
		out = append(out, arm{
			baseX:  headCX + spread*78,
			baseY:  headCY + 40 - math.Abs(spread)*22,
			angle:  math.Pi/2 + spread*1.02,
			curl:   spread * 1.25,
			length: 150 + math.Abs(spread)*28,
			width:  34 - math.Abs(spread)*6,
			phase:  float64(k) * 0.8,
		})
	}
	return out
}

// wavePhases are the four keyframes of one loop: a full turn of the wave,
// so frame 90 equals frame 0 and the loop is seamless.
var wavePhases = []float64{0, 2 * math.Pi / 3, 4 * math.Pi / 3, 2 * math.Pi}

// kelpFrond is one background frond: an open path from the sea floor
// swaying about its root.
func kelpFrond(x, height, ph, amp float64) pathData {
	const n = 6
	v := make([][2]float64, n)
	for k := range v {
		s := float64(k) / float64(n-1)
		v[k] = [2]float64{x + amp*s*math.Sin(math.Pi*s*0.9-ph), compH + 8 - s*height}
	}
	in, out := smoothTangents(v, false, 1.0/6)
	return pathData{closed: false, v: v, i: in, o: out}
}

// Fallback colors: what a player without the texture plugin draws.
var (
	skinColor  = []float64{0.48, 0.28, 0.67}
	armColor   = []float64{0.45, 0.25, 0.62}
	kelpColor  = []float64{0.16, 0.43, 0.24}
	white      = []float64{1, 1, 1}
	ink        = []float64{0.08, 0.05, 0.12}
	bubbleBlue = []float64{0.72, 0.9, 1}
)

func swimClip() (obj, *lottietexture.Doc) {
	doc := &lottietexture.Doc{}
	off := false

	// ---- octopus layer (ind 3) ----
	var octo []obj
	// Item order is paint order, first on top: shine, pupils, eyes, mouth,
	// head, then the arms behind everything.
	octo = append(octo, group("shine",
		ellipse("shine L", static(vec(212, headCY-19)), 6, 6),
		ellipse("shine R", static(vec(276, headCY-19)), 6, 6),
		fill("shine", white)))
	pupil := func(name string, x, y float64) obj {
		return ellipse(name, keys(
			keyE(0, vec(x, y)), keyE(30, vec(x+5, y+3)), keyE(60, vec(x-5, y+3)), end(loopFrames, vec(x, y))),
			15, 17)
	}
	octo = append(octo, group("pupils", pupil("pupil L", 208, headCY-10), pupil("pupil R", 272, headCY-10), fill("pupil", ink)))
	octo = append(octo, group("eyes",
		ellipse("eye L", static(vec(208, headCY-12)), 36, 40),
		ellipse("eye R", static(vec(272, headCY-12)), 36, 40),
		fill("eye white", white)))
	mouth := pathData{closed: false,
		v: [][2]float64{{226, headCY + 28}, {240, headCY + 35}, {254, headCY + 28}},
		i: [][2]float64{{0, 0}, {-7, 0}, {0, 0}},
		o: [][2]float64{{0, 0}, {7, 0}, {0, 0}}}
	octo = append(octo, group("mouth", shapeItem("smile", static(mouth.obj())), stroke("mouth", ink, 3)))
	head := pathKeys([]pathData{headShape(1), headShape(1.06), headShape(1)})
	// Rest, squashed wide at mid-loop, rest: the easing lives on the path
	// keys themselves, so swap the linear tangents for eased ones.
	for _, k := range head["k"].([]obj)[:2] {
		k["o"], k["i"] = obj{"x": []float64{0.4}, "y": []float64{0}}, obj{"x": []float64{0.6}, "y": []float64{1}}
	}
	octo = append(octo, group("head", shapeItem("mantle", head), fill("skin", skinColor)))
	headIndex := len(octo) - 1
	doc.Paints = append(doc.Paints, lottietexture.Paint{
		Layer: 3, Item: []int{headIndex, 1}, Texture: "skin", Tint: &off,
	})
	for k, a := range arms() {
		var shapes []pathData
		var uv [][2]float64
		for _, ph := range wavePhases {
			p, u := armOutline(a, ph+a.phase)
			shapes = append(shapes, p)
			uv = u
		}
		octo = append(octo, group("arm "+string(rune('1'+k)),
			shapeItem("arm", pathKeys(shapes)), fill("arm", armColor)))
		idx := len(octo) - 1
		doc.Paints = append(doc.Paints, lottietexture.Paint{
			Layer: 3, Item: []int{idx, 1}, Texture: "arm",
			Mapping: lottietexture.MappingVertex, Tint: &off,
		})
		doc.UVs = append(doc.UVs, lottietexture.UV{Layer: 3, Item: []int{idx, 0}, V: uv})
	}
	octoKS := obj{
		"a": static([]float64{headCX, headCY + 30, 0}),
		"p": keys(keyE(0, []float64{headCX, headCY + 32, 0}), keyE(45, []float64{headCX, headCY + 22, 0}),
			end(loopFrames, []float64{headCX, headCY + 32, 0})),
		"s": static([]float64{100, 100, 100}),
		"r": keys(keyE(0, -2.5), keyE(45, 2.5), end(loopFrames, -2.5)),
		"o": static(100.0),
	}

	// ---- kelp layer (ind 2) ----
	var kelp []obj
	for k, f := range []struct{ x, h, amp float64 }{{58, 250, 26}, {96, 185, 20}, {424, 265, 30}, {456, 170, 22}} {
		var shapes []pathData
		for _, ph := range wavePhases {
			shapes = append(shapes, kelpFrond(f.x, f.h, ph+float64(k)*1.3, f.amp))
		}
		kelp = append(kelp, group("kelp "+string(rune('1'+k)),
			shapeItem("frond", pathKeys(shapes)), stroke("frond", kelpColor, 15)))
		doc.Paints = append(doc.Paints, lottietexture.Paint{
			Layer: 2, Item: []int{k, 1}, Texture: "kelp",
			Mapping: lottietexture.MappingStroke, Wrap: lottietexture.WrapRepeat, Tint: &off,
		})
	}

	// ---- bubbles layer (ind 4): three bubbles at different heights of the
	// same rise, the wrap hidden off screen by a hold key ----
	bubble := func(name string, x, y0 float64, wrapAt float64, r float64) obj {
		var p obj
		if wrapAt <= 0 {
			p = keys(keyL(0, vec(x, 350)), keyL(30, vec(x+8, 230)), keyL(60, vec(x-6, 110)), end(loopFrames, vec(x+4, -20)))
		} else {
			p = keys(keyL(0, vec(x, y0)), keyH(wrapAt, vec(x+6, -20)), keyL(wrapAt+1, vec(x, 350)), end(loopFrames, vec(x, y0)))
		}
		return ellipse(name, p, r*2, r*2)
	}
	bubbles := []obj{group("bubbles",
		bubble("bubble 1", 330, 0, 0, 9),
		bubble("bubble 2", 150, 150, 40, 6),
		bubble("bubble 3", 384, 250, 62, 4.5),
		fillOpacity("bubble", bubbleBlue, 55))}

	clip := obj{
		"v": "5.9.0", "nm": "swim", "fr": fps, "ip": 0.0, "op": loopFrames,
		"w": compW, "h": compH,
		"assets": []obj{
			{"id": "skin", "w": 256, "h": 256, "u": "", "p": "skin.png", "e": 0},
			{"id": "arm", "w": 512, "h": 64, "u": "", "p": "arm.png", "e": 0},
			{"id": "kelp", "w": 128, "h": 32, "u": "", "p": "kelp.png", "e": 0},
		},
		"layers": []obj{
			shapeLayer("bubbles", 4, nil, bubbles),
			shapeLayer("octopus", 3, octoKS, octo),
			shapeLayer("kelp", 2, nil, kelp),
			solidLayer("sea", 1, "#0b2a4a"),
		},
	}
	return clip, doc
}

// ---- procedural textures ----

type canvas struct{ *image.NRGBA }

func newCanvas(w, h int, bg color.NRGBA) *canvas {
	c := &canvas{image.NewNRGBA(image.Rect(0, 0, w, h))}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c.SetNRGBA(x, y, bg)
		}
	}
	return c
}

// blend paints col over the pixel with coverage a.
func (c *canvas) blend(x, y int, col color.NRGBA, a float64) {
	if !(image.Point{x, y}).In(c.Rect) || a <= 0 {
		return
	}
	a = math.Min(a, 1) * float64(col.A) / 255
	d := c.NRGBAAt(x, y)
	mix := func(dc, sc uint8) uint8 { return uint8(float64(dc)*(1-a) + float64(sc)*a + 0.5) }
	c.SetNRGBA(x, y, color.NRGBA{mix(d.R, col.R), mix(d.G, col.G), mix(d.B, col.B), 255})
}

// disc is a soft-edged filled circle; soft is the width of the falloff.
func (c *canvas) disc(cx, cy, r float64, col color.NRGBA, soft float64) {
	for y := int(cy - r - soft); y <= int(cy+r+soft)+1; y++ {
		for x := int(cx - r - soft); x <= int(cx+r+soft)+1; x++ {
			d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
			c.blend(x, y, col, (r+soft-d)/math.Max(soft, 0.5))
		}
	}
}

// noise jitters every pixel's brightness by up to amount.
func (c *canvas) noise(rng *rand.Rand, amount float64) {
	b := c.Rect
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			d := c.NRGBAAt(x, y)
			k := 1 + (rng.Float64()*2-1)*amount
			cl := func(v uint8) uint8 { return uint8(math.Min(255, math.Max(0, float64(v)*k))) }
			c.SetNRGBA(x, y, color.NRGBA{cl(d.R), cl(d.G), cl(d.B), 255})
		}
	}
}

// skinTexture is the mantle: purple skin with pale spots and dark freckles.
func skinTexture(rng *rand.Rand) *canvas {
	c := newCanvas(256, 256, color.NRGBA{118, 68, 168, 255})
	c.noise(rng, 0.06)
	for k := 0; k < 46; k++ {
		r := 5 + rng.Float64()*17
		c.disc(rng.Float64()*256, rng.Float64()*256, r, color.NRGBA{186, 128, 222, uint8(140 + rng.Intn(80))}, 3)
	}
	for k := 0; k < 90; k++ {
		c.disc(rng.Float64()*256, rng.Float64()*256, 1.2+rng.Float64()*1.8, color.NRGBA{70, 30, 110, 200}, 1)
	}
	// A broad highlight toward the top left, the light from the surface.
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			d := math.Hypot(float64(x)-80, float64(y)-70) / 190
			c.blend(x, y, color.NRGBA{235, 200, 245, 255}, 0.22*math.Max(0, 1-d))
		}
	}
	return c
}

// armTexture is one whole arm, u from the mantle (left) to the tip
// (right): dark near the head and paler toward the tip on every arm alike,
// a row of suckers down the middle shrinking with the arm. Shading runs
// along the arm, not across it, so no arm reads as twisted whichever way
// it bends; the edges only darken a little to round the tube.
func armTexture(rng *rand.Rand) *canvas {
	const w, h = 512, 64
	c := newCanvas(w, h, color.NRGBA{96, 50, 150, 255})
	for x := 0; x < w; x++ {
		t := float64(x) / float64(w-1)
		for y := 0; y < h; y++ {
			c.blend(x, y, color.NRGBA{226, 176, 218, 255}, t*t*0.9+t*0.1)
			edge := math.Abs(float64(y)+0.5-h/2) / (h / 2)
			c.blend(x, y, color.NRGBA{40, 16, 70, 255}, 0.28*edge*edge)
		}
	}
	c.noise(rng, 0.04)
	for k := 0; k < 9; k++ {
		t := (float64(k) + 0.5) / 9
		cx := t * w
		r := 10 - 6.5*t
		c.disc(cx, h/2, r, color.NRGBA{246, 222, 238, 255}, 1.2)
		c.disc(cx, h/2, r*0.68, color.NRGBA{166, 104, 158, 255}, 1)
		c.disc(cx, h/2, r*0.3, color.NRGBA{118, 58, 116, 255}, 0.8)
	}
	return c
}

// kelpTexture is one repeat of a frond: green with a paler vein along u.
func kelpTexture(rng *rand.Rand) *canvas {
	c := newCanvas(128, 32, color.NRGBA{40, 112, 62, 255})
	for y := 0; y < 32; y++ {
		edge := math.Abs(float64(y)-15.5) / 15.5
		for x := 0; x < 128; x++ {
			c.blend(x, y, color.NRGBA{18, 62, 34, 255}, 0.55*edge*edge)
			vein := math.Max(0, 1-math.Abs(float64(y)-15.5)/4)
			c.blend(x, y, color.NRGBA{120, 190, 110, 255}, 0.6*vein)
		}
	}
	for k := 0; k < 18; k++ {
		c.disc(rng.Float64()*128, rng.Float64()*32, 2+rng.Float64()*3, color.NRGBA{80, 150, 80, 120}, 2)
	}
	c.noise(rng, 0.05)
	return c
}

func encodePNG(c *canvas) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, c.NRGBA); err != nil {
		log.Fatal(err)
	}
	return buf.Bytes()
}

func main() {
	dir := "examples/lottie/octopus"
	assets := filepath.Join(dir, "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		log.Fatal(err)
	}
	rng := rand.New(rand.NewSource(7))
	images := map[string][]byte{
		"skin.png": encodePNG(skinTexture(rng)),
		"arm.png":  encodePNG(armTexture(rng)),
		"kelp.png": encodePNG(kelpTexture(rng)),
	}
	clip, doc := swimClip()
	clipJSON, err := json.MarshalIndent(clip, "", " ")
	if err != nil {
		log.Fatal(err)
	}
	docJSON, err := json.MarshalIndent(doc, "", " ")
	if err != nil {
		log.Fatal(err)
	}

	b := lottie.NewBundle()
	if err := b.SetAnimation("swim", clipJSON); err != nil {
		log.Fatal(err)
	}
	for name, data := range images {
		b.SetImage(name, data)
	}
	if err := lottietexture.Store(b, "swim", doc); err != nil {
		log.Fatal(err)
	}
	var bundle bytes.Buffer
	if err := b.Encode(&bundle); err != nil {
		log.Fatal(err)
	}

	write := func(path string, data []byte) {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			log.Fatal(err)
		}
		log.Printf("wrote %s (%d bytes)", path, len(data))
	}
	write(filepath.Join(dir, "octopus.lottie"), bundle.Bytes())
	write(filepath.Join(assets, "swim.json"), clipJSON)
	write(filepath.Join(assets, "swim.texture.json"), docJSON)
	for name, data := range images {
		write(filepath.Join(assets, name), data)
	}
}
