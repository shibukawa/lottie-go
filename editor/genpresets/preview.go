package main

// The preview sheet renders the pose data directly — a tiny affine
// compositor over the part sprites — so the character can be judged from
// a PNG without running a player. It intentionally shares the pose
// sequences with the clip builder rather than re-reading the JSON: the
// sheet documents authoring intent, the editor previews the truth.

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

// mat is a 2x3 affine matrix mapping (x,y) to (a*x+c*y+tx, b*x+d*y+ty).
type mat struct{ a, b, c, d, tx, ty float64 }

func identity() mat { return mat{1, 0, 0, 1, 0, 0} }

func (m mat) mul(n mat) mat {
	return mat{
		a: m.a*n.a + m.c*n.b, b: m.b*n.a + m.d*n.b,
		c: m.a*n.c + m.c*n.d, d: m.b*n.c + m.d*n.d,
		tx: m.a*n.tx + m.c*n.ty + m.tx, ty: m.b*n.tx + m.d*n.ty + m.ty,
	}
}

func translate(x, y float64) mat { return mat{1, 0, 0, 1, x, y} }
func scale(x, y float64) mat     { return mat{x, 0, 0, y, 0, 0} }

func rotate(deg float64) mat {
	s, c := math.Sincos(deg * math.Pi / 180)
	return mat{c, s, -s, c, 0, 0}
}

func (m mat) invert() mat {
	det := m.a*m.d - m.b*m.c
	ia, ib, ic, id := m.d/det, -m.b/det, -m.c/det, m.a/det
	return mat{ia, ib, ic, id, -(ia*m.tx + ic*m.ty), -(ib*m.tx + id*m.ty)}
}

func (m mat) apply(x, y float64) (float64, float64) {
	return m.a*x + m.c*y + m.tx, m.b*x + m.d*y + m.ty
}

// nodeTransform is the Lottie layer transform: move anchor to pos, then
// rotate and scale about it.
func nodeTransform(pos, anchor [2]float64, deg, sx, sy float64) mat {
	return translate(pos[0], pos[1]).mul(rotate(deg)).mul(scale(sx/100, sy/100)).mul(translate(-anchor[0], -anchor[1]))
}

// poseAt linearly interpolates the pose sequence at frame t. Easing is
// ignored: the sheet samples poses, not curves.
func poseAt(keys []kf, t float64) pose {
	if t <= keys[0].t {
		return keys[0].p
	}
	for i := 1; i < len(keys); i++ {
		if t <= keys[i].t {
			a, b := keys[i-1], keys[i]
			f := (t - a.t) / (b.t - a.t)
			p := lerpPose(a.p, b.p, f)
			// flip is a hold: it switches only when the next key is reached.
			p.flip = a.p.flip
			if f >= 1 {
				p.flip = b.p.flip
			}
			return p
		}
	}
	return keys[len(keys)-1].p
}

func lerpPose(a, b pose, f float64) pose {
	l := func(x, y float64) float64 { return x + (y-x)*f }
	return pose{
		bx: l(a.bx, b.bx), by: l(a.by, b.by), rot: l(a.rot, b.rot),
		sx: l(a.sx, b.sx), sy: l(a.sy, b.sy), head: l(a.head, b.head),
		armN: l(a.armN, b.armN), armF: l(a.armF, b.armF),
		elbowN: l(a.elbowN, b.elbowN), elbowF: l(a.elbowF, b.elbowF),
		legN: l(a.legN, b.legN), legF: l(a.legF, b.legF),
		kneeN: l(a.kneeN, b.kneeN), kneeF: l(a.kneeF, b.kneeF),
		alpha: l(a.alpha, b.alpha), shadow: l(a.shadow, b.shadow),
	}
}

// drawPart composites one sprite under a world transform, inverse-mapped
// with nearest sampling to keep the pixel look.
func drawPart(dst *image.NRGBA, src *image.NRGBA, world mat, opacity float64) {
	sw, sh := float64(src.Bounds().Dx()), float64(src.Bounds().Dy())
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, c := range [][2]float64{{0, 0}, {sw, 0}, {0, sh}, {sw, sh}} {
		x, y := world.apply(c[0], c[1])
		minX, minY = math.Min(minX, x), math.Min(minY, y)
		maxX, maxY = math.Max(maxX, x), math.Max(maxY, y)
	}
	inv := world.invert()
	b := dst.Bounds()
	x0, y0 := max(int(minX), b.Min.X), max(int(minY), b.Min.Y)
	x1, y1 := min(int(maxX)+1, b.Max.X), min(int(maxY)+1, b.Max.Y)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			sx, sy := inv.apply(float64(x)+0.5, float64(y)+0.5)
			if sx < 0 || sy < 0 || sx >= sw || sy >= sh {
				continue
			}
			c := src.NRGBAAt(int(sx), int(sy))
			if c.A == 0 {
				continue
			}
			a := float64(c.A) / 255 * opacity / 100
			if a <= 0 {
				continue
			}
			d := dst.NRGBAAt(x, y)
			blend := func(s, o uint8) uint8 {
				return uint8(float64(s)*a + float64(o)*(1-a))
			}
			dst.SetNRGBA(x, y, color.NRGBA{
				blend(c.R, d.R), blend(c.G, d.G), blend(c.B, d.B), 255})
		}
	}
}

// renderPose draws the whole character into a canvas-sized frame,
// back to front.
func renderPose(p pose) *image.NRGBA {
	frame := image.NewNRGBA(image.Rect(0, 0, int(canvas), int(canvas)))
	bg := color.NRGBA{58, 63, 84, 255}
	for i := range frame.Pix {
		switch i % 4 {
		case 0:
			frame.Pix[i] = bg.R
		case 1:
			frame.Pix[i] = bg.G
		case 2:
			frame.Pix[i] = bg.B
		case 3:
			frame.Pix[i] = bg.A
		}
	}
	body := nodeTransform([2]float64{restX + p.bx, restY + p.by}, bodyPart.anchor, p.rot, p.sx, p.sy)
	seg := func(parent mat, pt part, deg float64) mat {
		return parent.mul(nodeTransform(pt.pos, pt.anchor, deg, 100, 100))
	}
	// On flip, limb roots trade sides and the head and shins x-mirror,
	// matching the clip builder's flipTrack.
	root := func(pt part, deg float64) mat {
		pos := pt.pos
		if p.flip {
			pos = bodyMirrorX(pos)
		}
		return body.mul(nodeTransform(pos, pt.anchor, deg, 100, 100))
	}
	msx := 100.0
	if p.flip {
		msx = -100
	}
	mirrored := func(parent mat, pt part, deg float64) mat {
		return parent.mul(nodeTransform(pt.pos, pt.anchor, deg, msx, 100))
	}
	uArmN := root(upperArmN, p.armN)
	uArmF := root(upperArmF, p.armF)
	thN := root(thighN, p.legN)
	thF := root(thighF, p.legF)
	shadow := nodeTransform(shadowPart.pos, shadowPart.anchor, 0, p.shadow, p.shadow)
	type draw struct {
		pt      part
		world   mat
		opacity float64
	}
	// Reverse of the clip's front-to-back layer order.
	for _, d := range []draw{
		{shadowPart, shadow, 28},
		{forearmF, seg(uArmF, forearmF, p.elbowF), p.alpha},
		{upperArmF, uArmF, p.alpha},
		{shinFarPart, mirrored(thF, shinFarPart, p.kneeF), p.alpha},
		{thighF, thF, p.alpha},
		{bodyPart, body, p.alpha},
		{thighN, thN, p.alpha},
		{shinNearPart, mirrored(thN, shinNearPart, p.kneeN), p.alpha},
		{headPart, mirrored(body, headPart, p.head), p.alpha},
		{upperArmN, uArmN, p.alpha},
		{forearmN, seg(uArmN, forearmN, p.elbowN), p.alpha},
	} {
		drawPart(frame, d.pt.render(), d.world, d.opacity)
	}
	return frame
}

// previewSheet renders every clip as a row of sampled frames: one glance
// says whether a pose is broken before anything reaches the editor.
func previewSheet(defs []clipDef, cols int) ([]byte, error) {
	cell := int(canvas)
	sheet := image.NewNRGBA(image.Rect(0, 0, cell*cols, cell*len(defs)))
	for row, d := range defs {
		for col := range cols {
			t := d.frames * float64(col) / float64(cols-1)
			frame := renderPose(poseAt(d.keys, t))
			for y := range cell {
				for x := range cell {
					sheet.SetNRGBA(col*cell+x, row*cell+y, frame.NRGBAAt(x, y))
				}
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, sheet); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
