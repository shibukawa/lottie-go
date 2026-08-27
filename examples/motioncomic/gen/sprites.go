package main

// The cast, redrawn for shoujo print: white-filled bodies over toned
// backgrounds, enormous lashed eyes, blush marks, twinkles. Poses are
// frozen — the motion-comic style moves sprites, not limbs.

import (
	"image"
	"math"

	"github.com/fogleman/gg"
)

var (
	whiteC = inkRGB{1, 1, 1}
	greyC  = inkRGB{0.80, 0.78, 0.80}
	sweatC = inkRGB{0.72, 0.85, 0.93}
	crustC = inkRGB{0.87, 0.76, 0.62}
	crumbC = inkRGB{0.97, 0.93, 0.83}
)

// blob fills a traced path and outlines it in ink.
func (c *canvas) blob(fill inkRGB, alpha, lw float64, trace func()) {
	trace()
	c.col(fill, alpha)
	c.FillPreserve()
	c.col(inkC, 1)
	c.SetLineWidth(lw)
	c.SetLineJoin(gg.LineJoinRound)
	c.Stroke()
}

func (c *canvas) ellipseBlob(x, y, rx, ry float64, fill inkRGB, lw float64) {
	c.blob(fill, 1, lw, func() { c.DrawEllipse(x, y, rx, ry) })
}

// pigEar traces one pointed ear; flip mirrors it.
func (c *canvas) pigEar(x, y, s, flip float64) {
	c.blob(whiteC, 1, 4.5, func() {
		c.MoveTo(x, y)
		c.QuadraticTo(x+flip*10*s, y-46*s, x+flip*34*s, y-40*s)
		c.QuadraticTo(x+flip*44*s, y-8*s, x+flip*30*s, y+10*s)
		c.ClosePath()
	})
}

// pigHead draws the head around (x, y) with radius r. mood: "sparkle",
// "closed" or "swirl".
func (c *canvas) pigHead(x, y, r float64, mood string) {
	c.pigEar(x-r*0.75, y-r*0.55, r/80, -1)
	c.pigEar(x+r*0.55, y-r*0.65, r/80, 1)
	c.ellipseBlob(x, y, r, r*0.94, whiteC, 5)
	// Snout sits low-right; the pig faces right.
	c.ellipseBlob(x+r*0.62, y+r*0.28, r*0.34, r*0.24, blushC, 4)
	c.col(inkC, 1)
	c.DrawEllipse(x+r*0.52, y+r*0.28, r*0.05, r*0.09)
	c.DrawEllipse(x+r*0.74, y+r*0.28, r*0.05, r*0.09)
	c.Fill()
	// Blush slashes under the eye.
	c.col(pinkC, 0.8)
	c.SetLineWidth(3.5)
	for i := range 3 {
		bx := x - r*0.52 + float64(i)*10
		c.wobbly(bx, y+r*0.34, bx+12, y+r*0.5, 0)
		c.Stroke()
	}
	switch mood {
	case "sparkle":
		c.shoujoEye(x+r*0.1, y-r*0.12, r*0.52, r*0.68, 1)
	case "closed":
		c.closedEye(x+r*0.1, y-r*0.1, r*0.5, 1)
	case "swirl":
		c.swirlEye(x-r*0.3, y-r*0.15, r*0.24)
		c.swirlEye(x+r*0.35, y-r*0.2, r*0.26)
		// A dazed wavy mouth.
		c.col(inkC, 1)
		c.SetLineWidth(3.5)
		c.MoveTo(x-r*0.1, y+r*0.55)
		c.QuadraticTo(x, y+r*0.42, x+r*0.1, y+r*0.55)
		c.QuadraticTo(x+r*0.2, y+r*0.66, x+r*0.3, y+r*0.55)
		c.Stroke()
	}
}

// squirrelEar traces one tufted ear with a pink inner.
func (c *canvas) squirrelEar(x, y, s, flip float64) {
	c.blob(whiteC, 1, 4.5, func() {
		c.MoveTo(x, y)
		c.QuadraticTo(x+flip*4*s, y-52*s, x+flip*26*s, y-46*s)
		c.QuadraticTo(x+flip*38*s, y-10*s, x+flip*28*s, y+8*s)
		c.ClosePath()
	})
	c.col(pinkC, 0.6)
	c.DrawEllipse(x+flip*16*s, y-18*s, 8*s, 14*s)
	c.Fill()
}

// squirrelHead faces left. mood: "sparkle", "closed" or "teary".
func (c *canvas) squirrelHead(x, y, r float64, mood string) {
	c.squirrelEar(x-r*0.6, y-r*0.6, r/64, -1)
	c.squirrelEar(x+r*0.45, y-r*0.68, r/64, 1)
	c.ellipseBlob(x, y, r, r*0.92, whiteC, 5)
	// Muzzle low-left, nose on its tip.
	c.ellipseBlob(x-r*0.55, y+r*0.3, r*0.3, r*0.2, whiteC, 3.5)
	c.col(inkC, 1)
	c.DrawEllipse(x-r*0.78, y+r*0.22, r*0.09, r*0.08)
	c.Fill()
	c.col(pinkC, 0.8)
	c.SetLineWidth(3.5)
	for i := range 3 {
		bx := x + r*0.3 + float64(i)*10
		c.wobbly(bx, y+r*0.3, bx+12, y+r*0.46, 0)
		c.Stroke()
	}
	switch mood {
	case "sparkle":
		c.shoujoEye(x-r*0.12, y-r*0.12, r*0.5, r*0.66, -1)
	case "closed":
		c.closedEye(x-r*0.12, y-r*0.1, r*0.5, -1)
	case "teary":
		c.shoujoEye(x-r*0.3, y-r*0.1, r*0.42, r*0.56, -1)
		c.shoujoEye(x+r*0.32, y-r*0.14, r*0.42, r*0.56, 1)
		// Brimming tears: glossy pools under both eyes.
		c.col(sweatC, 0.9)
		c.DrawEllipse(x-r*0.3, y+r*0.28, r*0.2, r*0.09)
		c.DrawEllipse(x+r*0.32, y+r*0.24, r*0.2, r*0.09)
		c.Fill()
		c.col(inkC, 1)
		c.SetLineWidth(3)
		c.MoveTo(x-r*0.08, y+r*0.52)
		c.QuadraticTo(x+r*0.02, y+r*0.62, x+r*0.12, y+r*0.52)
		c.Stroke()
	}
}

func (c *canvas) leg(x, y, w, h, rot float64, fill inkRGB) {
	c.Push()
	c.RotateAbout(gg.Radians(rot), x, y)
	c.blob(fill, 1, 4.5, func() { c.DrawRoundedRectangle(x-w/2, y, w, h, w/2) })
	c.Pop()
}

func (c *canvas) toastAt(x, y, w, h, rot float64) {
	c.Push()
	c.RotateAbout(gg.Radians(rot), x, y)
	c.blob(crustC, 1, 5, func() { c.DrawRoundedRectangle(x-w/2, y-h/2, w, h, w*0.22) })
	c.col(crumbC, 1)
	c.DrawRoundedRectangle(x-w*0.36, y-h*0.36, w*0.72, h*0.72, w*0.14)
	c.Fill()
	// Crust hatching.
	c.col(inkC, 0.55)
	c.SetLineWidth(2)
	for i := range 3 {
		hx := x - w*0.25 + float64(i)*w*0.25
		c.wobbly(hx, y-h*0.24, hx+w*0.12, y+h*0.24, 0)
		c.Stroke()
	}
	c.Pop()
}

func (c *canvas) acornAt(x, y, s float64) {
	c.ellipseBlob(x, y+6*s, 10*s, 11*s, crustC, 3.5)
	c.blob(greyC, 1, 3.5, func() { c.DrawRoundedRectangle(x-12*s, y-9*s, 24*s, 10*s, 4*s) })
	// Cap hatching and stem.
	c.col(inkC, 0.6)
	c.SetLineWidth(1.8)
	for i := range 3 {
		c.wobbly(x-7*s+float64(i)*7*s, y-8*s, x-5*s+float64(i)*7*s, y, 0)
		c.Stroke()
	}
	c.inkLine(x, y-9*s, x+2*s, y-15*s, 3*s, 0)
}

// tailBlob is the squirrel's giant tail, swept up behind. flip=-1 mirrors.
func (c *canvas) tailBlob(x, y, s, flip float64) {
	c.blob(greyC, 1, 5, func() {
		c.MoveTo(x, y)
		c.CubicTo(x+flip*95*s, y+10*s, x+flip*120*s, y-60*s, x+flip*75*s, y-125*s)
		c.CubicTo(x+flip*55*s, y-155*s, x+flip*5*s, y-135*s, x+flip*18*s, y-95*s)
		c.CubicTo(x+flip*28*s, y-55*s, x+flip*8*s, y-20*s, x, y)
		c.ClosePath()
	})
	// Fur strokes.
	c.col(inkC, 0.6)
	c.SetLineWidth(2.5)
	for i := range 4 {
		t := 20 + float64(i)*26
		c.wobbly(x+flip*(40+t*0.3)*s, y-t*s, x+flip*(70+t*0.3)*s, y-(t+18)*s, 2)
		c.Stroke()
	}
}

// pigRunSprite: 340x300, facing right, toast in mouth, mid-stride.
func pigRunSprite() image.Image {
	c := newCanvas(340, 300)
	// Motion arcs trailing behind.
	c.col(inkC, 0.35)
	c.SetLineWidth(4)
	c.DrawEllipticalArc(30, 200, 26, 40, -1.1, 1.1)
	c.Stroke()
	c.DrawEllipticalArc(52, 210, 20, 30, -1.1, 1.1)
	c.Stroke()
	// Tail curl.
	c.col(inkC, 1)
	c.SetLineWidth(5)
	c.MoveTo(66, 180)
	c.CubicTo(40, 160, 60, 138, 78, 152)
	c.Stroke()
	// Far legs, body, near legs.
	c.leg(120, 230, 26, 58, 38, greyC)
	c.leg(200, 228, 26, 58, -34, greyC)
	c.ellipseBlob(150, 195, 92, 62, whiteC, 5)
	c.leg(105, 240, 28, 56, 18, whiteC)
	c.leg(215, 238, 28, 56, -22, whiteC)
	// Screentone shading along the belly.
	c.DrawEllipse(150, 195, 92, 62)
	c.Clip()
	c.toneRect(58, 205, 190, 60, 11, 2.6, 0.5)
	c.ResetClip()
	c.pigHead(215, 118, 82, "sparkle")
	c.toastAt(300, 178, 66, 56, -14)
	c.sparkle(300, 52, 18, whiteC, 0.95)
	c.sparkle(168, 30, 12, whiteC, 0.9)
	return c.Image()
}

// squirrelRunSprite: 360x320, facing left, acorn hugged to the chest.
func squirrelRunSprite() image.Image {
	c := newCanvas(360, 320)
	c.tailBlob(230, 210, 1.1, 1)
	c.leg(210, 250, 22, 50, 30, greyC)
	c.leg(135, 252, 22, 50, -26, greyC)
	c.ellipseBlob(170, 215, 68, 52, whiteC, 5)
	c.leg(120, 258, 24, 48, -12, whiteC)
	c.leg(225, 256, 24, 48, 24, whiteC)
	c.DrawEllipse(170, 215, 68, 52)
	c.Clip()
	c.toneRect(100, 228, 140, 45, 11, 2.6, 0.5)
	c.ResetClip()
	c.ellipseBlob(148, 228, 34, 26, blushC, 3.5)
	c.squirrelHead(105, 148, 64, "sparkle")
	c.acornAt(108, 226, 2.2)
	c.sparkle(50, 60, 16, whiteC, 0.95)
	c.sparkle(300, 100, 12, whiteC, 0.9)
	return c.Image()
}

// crashSprite: 1000x460, the two of them nose to nose at the moment of
// impact, snacks airborne.
func crashSprite() image.Image {
	c := newCanvas(1000, 460)
	// Impact spark between the heads.
	c.col(inkC, 1)
	c.SetLineWidth(5)
	for i := range 8 {
		ang := float64(i) * math.Pi / 4
		c.wobbly(500+math.Cos(ang)*30, 185+math.Sin(ang)*30,
			500+math.Cos(ang)*72, 185+math.Sin(ang)*72, 2)
		c.Stroke()
	}
	// Pig from the left, tipped back.
	c.Push()
	c.RotateAbout(gg.Radians(-14), 350, 300)
	c.leg(300, 330, 26, 56, 40, greyC)
	c.ellipseBlob(330, 300, 90, 60, whiteC, 5)
	c.leg(260, 340, 28, 54, 24, whiteC)
	c.leg(390, 338, 28, 54, -30, whiteC)
	c.pigHead(420, 210, 78, "closed")
	c.Pop()
	// Squirrel from the right, mirrored.
	c.Push()
	c.RotateAbout(gg.Radians(14), 650, 300)
	c.tailBlob(710, 320, 1.05, 1)
	c.ellipseBlob(670, 305, 66, 50, whiteC, 5)
	c.leg(620, 340, 24, 48, -22, whiteC)
	c.leg(720, 342, 24, 48, 26, whiteC)
	c.squirrelHead(580, 215, 62, "closed")
	c.Pop()
	// Lunch, ballistic.
	c.toastAt(300, 84, 66, 56, 34)
	c.acornAt(716, 70, 2.4)
	c.sparkle(390, 110, 20, whiteC, 0.95)
	c.sparkle(628, 120, 15, whiteC, 0.9)
	return c.Image()
}

// pigSitSprite: 320x320, flat on its behind, eyes spinning.
func pigSitSprite() image.Image {
	c := newCanvas(320, 320)
	c.ellipseBlob(160, 235, 92, 68, whiteC, 5)
	c.leg(96, 268, 30, 46, 64, whiteC)
	c.leg(224, 268, 30, 46, -64, whiteC)
	c.DrawEllipse(160, 235, 92, 68)
	c.Clip()
	c.toneRect(66, 250, 190, 60, 11, 2.6, 0.5)
	c.ResetClip()
	c.pigHead(163, 122, 80, "swirl")
	return c.Image()
}

// squirrelSitSprite: 320x320, tail limp on the ground, eyes brimming.
func squirrelSitSprite() image.Image {
	c := newCanvas(320, 320)
	// The tail has given up: flopped flat to the right.
	c.blob(greyC, 1, 5, func() {
		c.MoveTo(190, 250)
		c.CubicTo(260, 220, 300, 250, 290, 285)
		c.CubicTo(280, 305, 220, 300, 190, 285)
		c.ClosePath()
	})
	c.ellipseBlob(150, 240, 64, 56, whiteC, 5)
	c.leg(100, 272, 26, 42, 55, whiteC)
	c.leg(200, 272, 26, 42, -55, whiteC)
	c.ellipseBlob(140, 252, 32, 26, blushC, 3.5)
	c.squirrelHead(145, 132, 62, "teary")
	c.acornAt(52, 280, 2.2)
	return c.Image()
}

// toastSprite: 100x90, for its solo flight in panel 4.
func toastSprite() image.Image {
	c := newCanvas(100, 90)
	c.toastAt(50, 45, 72, 62, 0)
	return c.Image()
}

// sparkleSprite: 80x80, a big twinkle with a small companion. The big one
// gets a thin ink tracing of its own outline so it reads on white.
func sparkleSprite() image.Image {
	c := newCanvas(80, 80)
	c.sparkle(36, 40, 26, whiteC, 1)
	c.col(inkC, 0.85)
	c.SetLineWidth(2.5)
	w := 26 * 0.28
	c.MoveTo(36.0, 40-26.0)
	c.QuadraticTo(36+w, 40-w, 36+26.0, 40.0)
	c.QuadraticTo(36+w, 40+w, 36.0, 40+26.0)
	c.QuadraticTo(36-w, 40+w, 36-26.0, 40.0)
	c.QuadraticTo(36-w, 40-w, 36.0, 40-26.0)
	c.ClosePath()
	c.Stroke()
	c.sparkle(62, 20, 11, pinkC, 0.9)
	return c.Image()
}

// petalSprite: 60x60, one cherry petal for the park breeze.
func petalSprite() image.Image {
	c := newCanvas(60, 60)
	c.petal(30, 30, 20, 0.4)
	return c.Image()
}

// dropSprite: 60x80, one fat sweat drop.
func dropSprite() image.Image {
	c := newCanvas(60, 80)
	c.blob(sweatC, 0.95, 3.5, func() {
		c.MoveTo(30, 8)
		c.QuadraticTo(52, 40, 44, 58)
		c.QuadraticTo(38, 72, 30, 72)
		c.QuadraticTo(12, 70, 14, 50)
		c.QuadraticTo(16, 36, 30, 8)
		c.ClosePath()
	})
	c.DrawEllipse(24, 52, 5, 8)
	c.SetRGBA(1, 1, 1, 0.9)
	c.Fill()
	return c.Image()
}
