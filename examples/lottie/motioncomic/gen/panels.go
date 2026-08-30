package main

// Panel furniture: frames whose outside is opaque paper (they crop the
// clipped precomp under them), toned backgrounds, and the hand-lettered
// sound effect. All at 2x, like the sprites.

import (
	"image"
	"math"

	"github.com/fogleman/gg"
)

// paperRing fills paper between an outer rect and an inner path traced by
// trace, using even-odd so the panel interior stays transparent.
func (c *canvas) paperRing(w, h float64, trace func()) {
	c.SetFillRule(gg.FillRuleEvenOdd)
	c.DrawRectangle(0, 0, w, h)
	trace()
	c.col(paperC, 1)
	c.Fill()
	c.SetFillRule(gg.FillRuleWinding)
}

// rectFrame is a straight panel border: bold outer line, thin inner line,
// paper outside, transparent inside.
func rectFrame(w, h int) image.Image {
	c := newCanvas(w, h)
	fw, fh := float64(w), float64(h)
	const g = 40.0 // gutter
	c.paperRing(fw, fh, func() { c.DrawRectangle(g, g, fw-2*g, fh-2*g) })
	pts := [][2]float64{{g, g}, {fw - g, g}, {fw - g, fh - g}, {g, fh - g}}
	c.inkPoly(pts, 9, 2)
	in := [][2]float64{{g + 14, g + 14}, {fw - g - 14, g + 14}, {fw - g - 14, fh - g - 14}, {g + 14, fh - g - 14}}
	c.inkPoly(in, 3.5, 1.5)
	return c.Image()
}

// slashFrame is the impact panel: a parallelogram with slanted sides.
func slashFrame(w, h int, slant float64) image.Image {
	c := newCanvas(w, h)
	fw, fh := float64(w), float64(h)
	const g = 40.0
	pts := [][2]float64{{g + slant, g}, {fw - g, g}, {fw - g - slant, fh - g}, {g, fh - g}}
	c.paperRing(fw, fh, func() {
		c.MoveTo(pts[0][0], pts[0][1])
		for _, p := range pts[1:] {
			c.LineTo(p[0], p[1])
		}
		c.ClosePath()
	})
	c.inkPoly(pts, 10, 2)
	return c.Image()
}

// p1bg: the town at a sprint — toned sky, hatched building silhouettes.
func p1bg(w, h int) image.Image {
	c := newCanvas(w, h)
	fw, fh := float64(w), float64(h)
	c.col(paperC, 1)
	c.DrawRectangle(0, 0, fw, fh)
	c.Fill()
	c.toneRect(0, 0, fw, 170, 13, 3, 0.35)
	// Building silhouettes with vertical hatch and white windows.
	bl := []struct{ x, bw, bh float64 }{{40, 200, 300}, {300, 170, 380}, {530, 210, 260}, {760, 160, 340}}
	for _, b := range bl {
		top := fh - 100 - b.bh
		c.col(greyC, 0.55)
		c.DrawRectangle(b.x, top, b.bw, b.bh)
		c.Fill()
		c.col(inkC, 1)
		c.SetLineWidth(5)
		c.DrawRectangle(b.x, top, b.bw, b.bh)
		c.Stroke()
		c.col(inkC, 0.35)
		c.SetLineWidth(2)
		for hx := b.x + 14; hx < b.x+b.bw; hx += 16 {
			c.wobbly(hx, top+8, hx, top+b.bh-8, 1.2)
			c.Stroke()
		}
		c.SetRGBA(1, 1, 1, 0.95)
		for wy := top + 30; wy < fh-160; wy += 70 {
			c.DrawRectangle(b.x+24, wy, 34, 40)
			c.DrawRectangle(b.x+b.bw-58, wy, 34, 40)
		}
		c.Fill()
	}
	c.inkLine(0, fh-96, fw, fh-96, 6, 2.5)
	c.inkLine(0, fh-82, fw, fh-82, 3, 2)
	return c.Image()
}

// speedLines: a transparent sheet of horizontal tapers to drift over a
// panel.
func speedLines(w, h int) image.Image {
	c := newCanvas(w, h)
	ys := []float64{40, 90, 150, 210, 270, 330, 390, 430}
	for i, y := range ys {
		x := float64((i*137)%260) - 40
		ln := 300 + float64((i*211)%260)
		c.taper(x+ln, y+c.jitter(6), x, y, 7, inkC, 0.5)
		if i%2 == 0 {
			c.taper(x+ln+180, y+34, x+140, y+34, 4, inkC, 0.35)
		}
	}
	return c.Image()
}

// p2bg: the park side — scalloped bushes, petals already on the wind.
func p2bg(w, h int) image.Image {
	c := newCanvas(w, h)
	fw, fh := float64(w), float64(h)
	c.col(paperC, 1)
	c.DrawRectangle(0, 0, fw, fh)
	c.Fill()
	c.toneRect(0, 0, fw, 130, 15, 2.6, 0.3)
	mounds := []struct{ x, r float64 }{{120, 150}, {350, 190}, {600, 160}, {820, 180}}
	for _, m := range mounds {
		c.ellipseBlob(m.x, fh-120, m.r, m.r*0.62, greyC, 5)
		c.DrawEllipse(m.x, fh-120, m.r, m.r*0.62)
		c.Clip()
		c.toneRect(m.x-m.r, fh-120, m.r*2, m.r, 12, 2.8, 0.45)
		c.ResetClip()
		// Scallop tufts along the top edge.
		c.col(inkC, 0.8)
		c.SetLineWidth(3)
		for i := range 4 {
			t := -0.75 + float64(i)*0.5
			bx := m.x + t*m.r*0.8
			by := fh - 120 - m.r*0.55*math.Sqrt(math.Max(0, 1-t*t))
			c.DrawEllipticalArc(bx, by+8, 18, 12, math.Pi, 2*math.Pi)
			c.Stroke()
		}
	}
	c.inkLine(0, fh-70, fw, fh-70, 6, 2.5)
	c.petal(180, 100, 22, 0.5)
	c.petal(700, 160, 18, 2.2)
	return c.Image()
}

// p3bg: the corner in shock — radial concentration lines around a pink
// bloom.
func p3bg(w, h int) image.Image {
	c := newCanvas(w, h)
	fw, fh := float64(w), float64(h)
	cx, cy := fw/2, fh/2
	c.col(paperC, 1)
	c.DrawRectangle(0, 0, fw, fh)
	c.Fill()
	// The bloom behind the impact.
	for i, r := range []float64{300, 220, 150} {
		c.col(pinkC, 0.10+float64(i)*0.08)
		c.DrawEllipse(cx, cy, r*1.6, r)
		c.Fill()
	}
	// Concentration lines rushing inward from every edge.
	outer := math.Hypot(fw/2, fh/2) + 40
	for i := range 60 {
		ang := float64(i)*2*math.Pi/60 + 0.02*float64(i%5)
		r1 := outer
		r2 := 190 + float64((i*97)%70)
		x1, y1 := cx+math.Cos(ang)*r1, cy+math.Sin(ang)*r1*0.62
		x2, y2 := cx+math.Cos(ang)*r2, cy+math.Sin(ang)*r2*0.55
		c.taper(x1, y1, x2, y2, 16, inkC, 0.85)
	}
	return c.Image()
}

// p4bg: the aftermath — flowers, twinkles, a soft pink wash. The shoujo
// page finally gets to be itself.
func p4bg(w, h int) image.Image {
	c := newCanvas(w, h)
	fw, fh := float64(w), float64(h)
	c.col(paperC, 1)
	c.DrawRectangle(0, 0, fw, fh)
	c.Fill()
	c.col(pinkC, 0.16)
	c.DrawEllipse(fw/2, fh*0.4, fw*0.62, fh*0.55)
	c.Fill()
	c.toneRect(0, 0, fw, 60, 12, 3, 0.3)
	c.toneRect(0, fh-60, fw, 60, 12, 3, 0.3)
	flowers := []struct{ x, y, r float64 }{
		{70, 80, 44}, {170, 44, 30}, {930, 90, 46}, {830, 50, 28},
		{60, 400, 34}, {940, 396, 36}, {500, 40, 26},
	}
	for _, f := range flowers {
		c.flower(f.x, f.y, f.r, blushC, 0.9)
	}
	for i := range 9 {
		x := 90 + float64((i*257)%860)
		y := 70 + float64((i*173)%330)
		c.sparkle(x, y, 8+float64(i%3)*4, pinkC, 0.5)
	}
	return c.Image()
}

// sfxStroke draws one lettering stroke twice: a fat paper halo, then ink.
func (c *canvas) sfxStroke(trace func()) {
	for _, pass := range []struct {
		k inkRGB
		w float64
	}{{whiteC, 32}, {inkC, 18}} {
		trace()
		c.col(pass.k, 1)
		c.SetLineWidth(pass.w)
		c.SetLineCap(gg.LineCapRound)
		c.SetLineJoin(gg.LineJoinRound)
		c.Stroke()
	}
}

// sfxDon: 700x400 — a spiky burst carrying hand-lettered「ドンッ!!」.
// The strokes are authored as polylines so no font ships with the repo.
func sfxDon() image.Image {
	c := newCanvas(700, 400)
	// Burst.
	cx, cy := 350.0, 200.0
	c.MoveTo(cx+330, cy)
	for i := 1; i <= 16; i++ {
		ang := float64(i) * 2 * math.Pi / 16
		r := 330.0
		if i%2 == 1 {
			r = 205
		}
		c.LineTo(cx+math.Cos(ang)*r, cy+math.Sin(ang)*r*0.55)
	}
	c.ClosePath()
	c.SetRGBA(1, 1, 1, 1)
	c.FillPreserve()
	c.col(inkC, 1)
	c.SetLineWidth(7)
	c.SetLineJoin(gg.LineJoinRound)
	c.Stroke()
	// ド
	c.sfxStroke(func() { c.MoveTo(165, 100); c.LineTo(165, 300) })
	c.sfxStroke(func() { c.MoveTo(168, 170); c.QuadraticTo(220, 210, 255, 275) })
	c.sfxStroke(func() { c.MoveTo(250, 95); c.LineTo(278, 120) })
	c.sfxStroke(func() { c.MoveTo(290, 78); c.LineTo(318, 103) })
	// ン
	c.sfxStroke(func() { c.MoveTo(335, 140); c.QuadraticTo(365, 152, 378, 172) })
	c.sfxStroke(func() { c.MoveTo(330, 295); c.QuadraticTo(420, 290, 455, 150) })
	// ッ (small)
	c.sfxStroke(func() { c.MoveTo(487, 138); c.LineTo(503, 155) })
	c.sfxStroke(func() { c.MoveTo(524, 128); c.LineTo(540, 145) })
	c.sfxStroke(func() { c.MoveTo(568, 130); c.QuadraticTo(580, 200, 520, 242) })
	// !!
	c.sfxStroke(func() { c.MoveTo(612, 110); c.LineTo(604, 230) })
	c.sfxStroke(func() { c.MoveTo(650, 110); c.LineTo(642, 230) })
	for _, x := range []float64{600, 638} {
		c.DrawCircle(x, 272, 11)
		c.col(inkC, 1)
		c.Fill()
	}
	return c.Image()
}
