package main

// Drawing kit for the shoujo-manga look: ink lines with a hand wobble,
// screentone dots, tapered speed lines, sparkles, flowers. Everything is
// drawn at 2x resolution and placed at 50% so the camera can lean in
// without the raster pixelating.

import (
	"math"
	"math/rand"

	"github.com/fogleman/gg"
)

type inkRGB struct{ r, g, b float64 }

var (
	inkC   = inkRGB{0.10, 0.08, 0.11}
	paperC = inkRGB{0.973, 0.961, 0.941}
	pinkC  = inkRGB{0.91, 0.63, 0.71}
	blushC = inkRGB{0.95, 0.80, 0.85}
	toneC  = inkRGB{0.45, 0.42, 0.46}
)

type canvas struct {
	*gg.Context
	rng *rand.Rand
}

// newCanvas is transparent; sprites and frames rely on the alpha channel.
func newCanvas(w, h int) *canvas {
	return &canvas{gg.NewContext(w, h), rand.New(rand.NewSource(int64(w)*31 + int64(h)))}
}

func (c *canvas) col(k inkRGB, a float64) { c.SetRGBA(k.r, k.g, k.b, a) }

func (c *canvas) jitter(amp float64) float64 { return (c.rng.Float64()*2 - 1) * amp }

// wobbly draws the current path idea as a hand-drawn polyline between two
// points: subdivided with a little perpendicular noise.
func (c *canvas) wobbly(x1, y1, x2, y2, amp float64) {
	dx, dy := x2-x1, y2-y1
	dist := math.Hypot(dx, dy)
	n := int(dist/28) + 1
	nx, ny := -dy/dist, dx/dist
	c.MoveTo(x1, y1)
	for i := 1; i <= n; i++ {
		t := float64(i) / float64(n)
		j := 0.0
		if i < n {
			j = c.jitter(amp)
		}
		c.LineTo(x1+dx*t+nx*j, y1+dy*t+ny*j)
	}
}

// inkLine strokes a wobbly line.
func (c *canvas) inkLine(x1, y1, x2, y2, width, amp float64) {
	c.wobbly(x1, y1, x2, y2, amp)
	c.col(inkC, 1)
	c.SetLineWidth(width)
	c.SetLineCap(gg.LineCapRound)
	c.Stroke()
}

// inkPoly strokes a closed wobbly polygon through pts.
func (c *canvas) inkPoly(pts [][2]float64, width, amp float64) {
	for i := range pts {
		a, b := pts[i], pts[(i+1)%len(pts)]
		c.inkLine(a[0], a[1], b[0], b[1], width, amp)
	}
}

// taper fills a speed line: a long thin triangle from the thick end to a
// point.
func (c *canvas) taper(x1, y1, x2, y2, w float64, k inkRGB, a float64) {
	dx, dy := x2-x1, y2-y1
	d := math.Hypot(dx, dy)
	nx, ny := -dy/d*w/2, dx/d*w/2
	c.MoveTo(x1+nx, y1+ny)
	c.LineTo(x1-nx, y1-ny)
	c.LineTo(x2, y2)
	c.ClosePath()
	c.col(k, a)
	c.Fill()
}

// toneRect lays halftone dots over a rectangle — the screentone.
func (c *canvas) toneRect(x, y, w, h, spacing, r, alpha float64) {
	c.col(toneC, alpha)
	row := 0
	for yy := y + spacing/2; yy < y+h; yy += spacing {
		off := 0.0
		if row%2 == 1 {
			off = spacing / 2
		}
		for xx := x + spacing/2 + off; xx < x+w; xx += spacing {
			c.DrawCircle(xx, yy, r)
		}
		row++
	}
	c.Fill()
}

// toneCircle dots a disc, for shading rounded bodies.
func (c *canvas) toneCircle(cx, cy, rad, spacing, r, alpha float64) {
	c.DrawCircle(cx, cy, rad)
	c.Clip()
	c.toneRect(cx-rad, cy-rad, rad*2, rad*2, spacing, r, alpha)
	c.ResetClip()
}

// sparkle is the four-point twinkle cross shoujo panels are dusted with.
func (c *canvas) sparkle(x, y, r float64, k inkRGB, a float64) {
	w := r * 0.28
	c.MoveTo(x, y-r)
	c.QuadraticTo(x+w, y-w, x+r, y)
	c.QuadraticTo(x+w, y+w, x, y+r)
	c.QuadraticTo(x-w, y+w, x-r, y)
	c.QuadraticTo(x-w, y-w, x, y-r)
	c.ClosePath()
	c.col(k, a)
	c.Fill()
}

// flower is five petal ellipses around a small centre.
func (c *canvas) flower(x, y, r float64, petal inkRGB, alpha float64) {
	for i := range 5 {
		ang := float64(i)*2*math.Pi/5 - math.Pi/2
		px, py := x+math.Cos(ang)*r*0.62, y+math.Sin(ang)*r*0.62
		c.Push()
		c.RotateAbout(ang+math.Pi/2, px, py)
		c.DrawEllipse(px, py, r*0.42, r*0.55)
		c.Pop()
		c.col(petal, alpha)
		c.FillPreserve()
		c.col(inkC, alpha)
		c.SetLineWidth(2.5)
		c.Stroke()
	}
	c.DrawCircle(x, y, r*0.22)
	c.col(inkC, alpha)
	c.Fill()
}

// petal is a single falling cherry petal.
func (c *canvas) petal(x, y, r, rot float64) {
	c.Push()
	c.RotateAbout(rot, x, y)
	c.MoveTo(x, y-r)
	c.QuadraticTo(x+r*0.9, y-r*0.4, x+r*0.35, y+r)
	c.QuadraticTo(x, y+r*0.55, x-r*0.35, y+r)
	c.QuadraticTo(x-r*0.9, y-r*0.4, x, y-r)
	c.ClosePath()
	c.col(pinkC, 0.9)
	c.FillPreserve()
	c.col(inkC, 1)
	c.SetLineWidth(2.5)
	c.Stroke()
	c.Pop()
}

// shoujoEye is the genre's centrepiece: a tall iris under a heavy lash,
// stacked highlights, three flick lashes.
func (c *canvas) shoujoEye(x, y, w, h float64, flip float64) {
	// Iris.
	c.DrawEllipse(x, y, w/2, h/2)
	c.col(pinkC, 0.55)
	c.FillPreserve()
	c.col(inkC, 1)
	c.SetLineWidth(3)
	c.Stroke()
	// Pupil.
	c.DrawEllipse(x, y+h*0.08, w*0.26, h*0.30)
	c.col(inkC, 1)
	c.Fill()
	// Highlights: one big top-left, one small bottom-right.
	c.DrawEllipse(x-flip*w*0.18, y-h*0.22, w*0.16, h*0.14)
	c.SetRGBA(1, 1, 1, 0.95)
	c.Fill()
	c.DrawEllipse(x+flip*w*0.14, y+h*0.18, w*0.08, h*0.07)
	c.SetRGBA(1, 1, 1, 0.9)
	c.Fill()
	// Heavy upper lash with three flicks.
	c.col(inkC, 1)
	c.SetLineWidth(h * 0.16)
	c.SetLineCap(gg.LineCapRound)
	c.DrawEllipticalArc(x, y+h*0.05, w*0.62, h*0.62, math.Pi+0.35, 2*math.Pi-0.35)
	c.Stroke()
	for i := range 3 {
		ang := -math.Pi/2 + flip*(0.45+float64(i)*0.35)
		sx, sy := x+math.Cos(ang)*w*0.58, y+math.Sin(ang)*h*0.58
		ex, ey := x+math.Cos(ang)*w*0.85, y+math.Sin(ang)*h*0.85-6
		c.SetLineWidth(3.5)
		c.wobbly(sx, sy, ex, ey, 0)
		c.Stroke()
	}
}

// closedEye is the ">.<" squeeze for the collision panel.
func (c *canvas) closedEye(x, y, w float64, flip float64) {
	c.col(inkC, 1)
	c.SetLineWidth(5)
	c.SetLineCap(gg.LineCapRound)
	c.MoveTo(x+flip*w/2, y-w*0.35)
	c.QuadraticTo(x-flip*w*0.4, y, x+flip*w/2, y+w*0.35)
	c.Stroke()
}

// swirlEye is the dazed spiral.
func (c *canvas) swirlEye(x, y, r float64) {
	c.col(inkC, 1)
	c.SetLineWidth(3.5)
	c.SetLineCap(gg.LineCapRound)
	turns := 2.2
	steps := 40
	c.MoveTo(x+r, y)
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		ang := t * turns * 2 * math.Pi
		rad := r * (1 - t*0.9)
		c.LineTo(x+math.Cos(ang)*rad, y+math.Sin(ang)*rad)
	}
	c.Stroke()
}
