package main

import "math"

// The opening's clips. Everything is authored in this repository, so
// there is no third-party licensing to track. Marker names end in -seg,
// clip ids in -anim, matching the editor samples' convention.

// corpLogoClip is the vanity card: a gem pops in over a ring, a spark
// orbits it, and the whole card fades to black. One run, no markers —
// its completion (or the phase clock) ends the phase.
func corpLogoClip() obj {
	const f = 120
	// Every layer fades together at the end.
	fade := anim(
		key(0, []float64{100}, false),
		key(88, []float64{100}, true),
		key(112, []float64{0}, true),
		key(f, []float64{0}, false))
	pop := anim(
		key(0, []float64{0, 0}, true),
		key(20, []float64{114, 114}, true),
		key(32, []float64{100, 100}, true))
	ringPop := anim(
		key(4, []float64{0, 0}, true),
		key(28, []float64{100, 100}, true))
	// The spark swings around the gem while it holds.
	sparkPos := anim(
		key(30, []float64{200, 90}, true),
		key(55, []float64{310, 200}, true),
		key(80, []float64{200, 310}, true))
	sparkFade := anim(
		key(28, []float64{0}, true),
		key(36, []float64{100}, true),
		key(78, []float64{100}, true),
		key(90, []float64{0}, false))
	return doc("corp-logo-anim", 400, 400, f, []obj{
		layerAt("spark", 1, f, sparkPos, static([]float64{100, 100}), static(0.0), sparkFade,
			group("dot", ellipse(26, 26), fill(1, 1, 1, 1))),
		layerAt("gem", 2, f, static([]float64{200, 200}), pop, static(45.0), fade,
			group("outer", rect(150, 150, 18), fill(0.16, 0.68, 0.62, 1)),
			group("inner", rect(64, 64, 10), fill(1, 1, 1, 0.9))),
		layerAt("ring", 3, f, static([]float64{200, 200}), ringPop, static(0.0), fade,
			group("ring", ellipse(250, 250), stroke(1, 1, 1, 0.85, 7))),
	}, nil)
}

// Hokusai-ish palette: Prussian blue steps, unbleached-cloth cream.
var (
	deepBlue = rgb{0.10, 0.22, 0.38}
	midBlue  = rgb{0.20, 0.38, 0.58}
	bandBlue = rgb{0.47, 0.62, 0.78}
	cream    = rgb{0.96, 0.93, 0.85}
)

type pt struct{ x, y float64 }

// waveForm is one stage of the breaking wave's outline: the vertices the
// morph interpolates between as the wave rises, curls, and collapses.
// B summit, C left shoulder, D curl front, E lip tip, F lip underside,
// G hollow face; Claw scales the foam fingers hanging from the lip.
type waveForm struct {
	B, C, D, E, F, G pt
	Claw             float64
}

var (
	// A rounded mound, the claw not yet formed.
	formMound = waveForm{
		B: pt{-60, -430}, C: pt{-390, -390}, D: pt{-550, -250},
		E: pt{-470, -140}, F: pt{-370, -180}, G: pt{-420, -30}, Claw: 0.25,
	}
	// The full overhanging claw of the print.
	formClaw = waveForm{
		B: pt{-100, -560}, C: pt{-460, -530}, D: pt{-655, -390},
		E: pt{-590, -180}, F: pt{-430, -230}, G: pt{-440, -40}, Claw: 1,
	}
	// Collapsing: the summit drops and the lip is thrown forward.
	formSpill = waveForm{
		B: pt{-170, -480}, C: pt{-550, -440}, D: pt{-740, -280},
		E: pt{-650, -80}, F: pt{-470, -160}, G: pt{-450, -10}, Claw: 0.55,
	}
)

// bodyVal is the silhouette for one form: up the back, over the summit,
// the claw hooking over the leading (left) edge, the hollow face carved
// beneath it, down into the trough.
func bodyVal(w waveForm) obj {
	return curveVal(true,
		vx{w.B.x, w.B.y, 160, -20, -160, 20},
		vx{w.C.x, w.C.y, 120, -50, -110, 45},
		vx{w.D.x, w.D.y, 30, -95, -25, 80},
		vx{w.E.x, w.E.y, -50, -60, 45, 55},
		vx{w.F.x, w.F.y, -50, -60, 45, 55},
		vx{w.G.x, w.G.y, 55, -85, -40, 75},
		vx{-690, 300, 55, -140, 0, 0},
		vx{-940, 420, 0, 0, 0, 0},
		vx{980, 420, 0, 0, 0, 0},
		vx{960, 180, 0, 60, 0, -90},
		vx{560, -260, 170, 120, -170, -120})
}

// foamInward pushes the midpoint of an edge toward the crest's interior,
// which is where the foam band's scalloped inner edge lives.
func foamInward(a, b pt, inward float64) pt {
	mx, my := (a.x+b.x)/2, (a.y+b.y)/2
	dx, dy := -100-mx, -250-my
	l := math.Hypot(dx, dy)
	if l == 0 {
		l = 1
	}
	return pt{mx + dx/l*inward, my + dy/l*inward}
}

// foamVal is the cream band edging the whole crest for one form: its
// outer edge shares the silhouette's vertices and tangents, its inner
// edge scallops like the print's froth.
func foamVal(w waveForm) obj {
	a2 := pt{500, -300}
	i1 := pt{w.E.x + 60, w.E.y - 25}
	i2 := foamInward(w.D, w.E, 70)
	i3 := foamInward(w.C, w.D, 80)
	i4 := foamInward(w.B, w.C, 85)
	i5 := foamInward(a2, w.B, 95)
	return curveVal(true,
		vx{a2.x, a2.y, -60, 20, -170, -120},
		vx{w.B.x, w.B.y, 160, -20, -160, 20},
		vx{w.C.x, w.C.y, 120, -50, -110, 45},
		vx{w.D.x, w.D.y, 30, -95, -25, 80},
		vx{w.E.x, w.E.y, -50, -60, 40, 30},
		vx{i1.x, i1.y, -20, 30, 25, -35},
		vx{i2.x, i2.y, -30, 30, 35, -30},
		vx{i3.x, i3.y, -45, 20, 50, -18},
		vx{i4.x, i4.y, -55, 0, 60, 5},
		vx{i5.x, i5.y, -60, -25, 55, 35})
}

// fingerVal is one foam claw hanging off the lip, scaled with the curl.
func fingerVal(w waveForm, dx, dy, fw, fh float64) obj {
	s := w.Claw
	x, y := w.E.x+dx*s, w.E.y+dy*s
	fw, fh = fw*s, fh*s
	return curveVal(true,
		vx{x, y, fw * 0.5, -fh * 0.2, -fw * 0.5, fh * 0.2},
		vx{x - fw*0.6, y + fh, -fw * 0.2, -fh * 0.4, fw * 0.2, fh * 0.4},
		vx{x + fw*0.5, y + fh*0.45, -fw * 0.1, fh * 0.5, fw * 0.1, -fh * 0.5})
}

// waveKeys morphs a per-form path through the break: mound while rising,
// the claw fully formed overhead, spilling as it exits.
func waveKeys(val func(waveForm) obj) obj {
	return morphPath(
		shapeKey(0, val(formMound)),
		shapeKey(50, val(formMound)),
		shapeKey(110, val(formClaw)),
		shapeKey(145, val(formClaw)),
		shapeKey(180, val(formSpill)))
}

// greatWave is the breaking wave in its own coordinates, drawn as free
// bezier paths after the Great Wave off Kanagawa — and morphing through
// the break. The first group paints on top.
func greatWave() []any {
	finger := func(dx, dy, fw, fh float64) obj {
		return waveKeys(func(w waveForm) obj { return fingerVal(w, dx, dy, fw, fh) })
	}
	return []any{
		group("claw-foam",
			finger(30, 25, 40, 85),
			finger(95, 65, 34, 70),
			finger(160, 100, 28, 56),
			fill(cream.r, cream.g, cream.b, 1)),
		group("crest-foam", waveKeys(foamVal),
			fill(cream.r, cream.g, cream.b, 1)),
		// The water's banding: thick curved strokes following the swell.
		group("band-light", curve(false,
			vx{-560, 140, 0, 0, 120, -160},
			vx{-220, -180, -160, 80, 160, -80},
			vx{200, -320, -160, 20, 160, -20},
			vx{540, -160, -120, -70, 60, 40}),
			stroke(bandBlue.r, bandBlue.g, bandBlue.b, 1, 46)),
		group("band-mid", curve(false,
			vx{-640, 260, 0, 0, 140, -140},
			vx{-240, 20, -160, 60, 160, -60},
			vx{280, -110, -180, 10, 180, -10},
			vx{640, 60, -120, -80, 60, 40}),
			stroke(midBlue.r, midBlue.g, midBlue.b, 1, 64)),
		group("body", waveKeys(bodyVal),
			fill(deepBlue.r, deepBlue.g, deepBlue.b, 1)),
	}
}

// swell is a standing sea hump drawn as one bezier path, foam scalloped
// along its crest as a curved cream band.
func swell(w, h, foamY float64) []any {
	half := w / 2
	return []any{
		group("foam", curve(true,
			vx{-half * 0.9, foamY + 60, 0, 0, half * 0.3, -60},
			vx{-half * 0.25, foamY - 8, -half * 0.28, 0, half * 0.28, 0},
			vx{half * 0.45, foamY + 26, -half * 0.25, -40, half * 0.2, 30},
			vx{half * 0.8, foamY + 80, -half * 0.1, -30, 0, 0},
			vx{half * 0.5, foamY + 60, half * 0.1, 30, -half * 0.1, -30},
			vx{half * 0.05, foamY + 46, half * 0.2, 20, -half * 0.2, -20},
			vx{-half * 0.45, foamY + 60, half * 0.2, 6, -half * 0.2, -6}),
			fill(cream.r, cream.g, cream.b, 1)),
		group("hump", curve(true,
			vx{-half, 40, 0, 0, half * 0.3, -h * 0.5},
			vx{0, -h * 0.45, -half * 0.35, 0, half * 0.35, 0},
			vx{half, 60, -half * 0.3, -h * 0.45, 0, 0},
			vx{half, h, 0, 0, 0, 0},
			vx{-half, h, 0, 0, 0, 0}),
			fill(midBlue.r, midBlue.g, midBlue.b, 1)),
	}
}

// oceanClip is the big picture, after the Great Wave off Kanagawa: the
// claw-crested wave sweeps over the frame, morphing as it breaks and
// shedding spray (wave-seg), Fuji small on the horizon, then the sea
// settles into a gentle loop (sea-seg). The sky stays transparent so the
// backdrop shows through.
func oceanClip() obj {
	const f = 300 // wave: [0,180), sea loop: [180,300)

	// The wave rides in from the right, its curl overhanging the leading
	// left edge, and exits left.
	wavePos := anim(
		key(0, []float64{2250, 1450}, true),
		key(60, []float64{1520, 860}, true),
		key(110, []float64{900, 790}, true),
		key(155, []float64{80, 1000}, true),
		key(180, []float64{-800, 1500}, false))
	waveRot := anim(
		key(0, []float64{8}, true),
		key(110, []float64{-5}, true),
		key(180, []float64{-12}, false))

	// Whitecap spray: droplets burst off the crest while it breaks, fly
	// up-left, and fall away. Deterministic table, no two alike.
	type sprayDef struct{ dx, dy, size, birth float64 }
	sprays := []sprayDef{
		{-40, -80, 26, 62}, {60, -130, 20, 68}, {-140, -60, 16, 74},
		{30, -40, 30, 80}, {-220, -110, 14, 88}, {120, -80, 18, 94},
		{-80, -170, 12, 102}, {-10, -95, 22, 110}, {-170, -35, 15, 118},
		{80, -60, 13, 124},
	}
	layers := make([]obj, 0, len(sprays)+5)
	for i, s := range sprays {
		// Birth rides the crest as the wave advances left.
		bx := 1300 - (s.birth-60)*8.5
		by := 320 - (s.birth-60)*0.9
		end := math.Min(s.birth+52, 178)
		pos := anim(
			key(s.birth, []float64{bx, by}, true),
			key(s.birth+20, []float64{bx - 170 + s.dx, by - 160 + s.dy}, true),
			key(end, []float64{bx - 340 + s.dx*1.5, by + 320}, false))
		fade := anim(
			key(s.birth, []float64{0}, false),
			key(s.birth+8, []float64{95}, true),
			key(end-10, []float64{80}, true),
			key(end, []float64{0}, false))
		layers = append(layers, layerAt("spray", i+1, f, pos,
			static([]float64{100, 100}), static(0.0), fade,
			group("d", ellipse(s.size, s.size*1.15), fill(cream.r, cream.g, cream.b, 1))))
	}
	// The standing sea: the two swells bob gently in counter-phase —
	// values at 180 and 300 match, so the sea-seg loops seamlessly.
	frontBob := anim(
		key(0, []float64{640, 760}, false),
		key(90, []float64{640, 748}, false),
		key(180, []float64{640, 760}, false),
		key(240, []float64{640, 748}, false),
		key(f, []float64{640, 760}, false))
	backBob := anim(
		key(0, []float64{640, 700}, false),
		key(90, []float64{640, 706}, false),
		key(180, []float64{640, 692}, false),
		key(240, []float64{640, 700}, false),
		key(f, []float64{640, 692}, false))
	n := len(sprays)
	layers = append(layers,
		layerAt("great-wave", n+1, f, wavePos, static([]float64{100, 100}), waveRot, static(100.0),
			greatWave()...),
		layerAt("swell-front", n+2, f, frontBob, static([]float64{100, 100}), static(0.0), static(100.0),
			swell(980, 330, -150)...),
		layerAt("swell-back", n+3, f, backBob, static([]float64{100, 100}), static(0.0), static(100.0),
			swell(860, 260, -118)...),
		// A still horizon band, so Fuji stands in the water and the
		// bobbing swells never open a gap of sky beneath it.
		layerAt("horizon", n+4, f, static([]float64{640, 620}), static([]float64{100, 100}), static(0.0), static(100.0),
			group("s", rect(1500, 130, 0), fill(deepBlue.r, deepBlue.g, deepBlue.b, 1))),
		layerAt("sea-deep", n+5, f, static([]float64{640, 740}), static([]float64{100, 100}), static(0.0), static(100.0),
			group("s", rect(1500, 260, 0), fill(deepBlue.r, deepBlue.g, deepBlue.b, 1))),
		layerAt("fuji", n+6, f, static([]float64{760, 0}), static([]float64{100, 100}), static(0.0), static(100.0),
			group("snow", polygon([2]float64{0, 405}, [2]float64{52, 454}, [2]float64{30, 446},
				[2]float64{10, 456}, [2]float64{-16, 446}, [2]float64{-40, 454}),
				fill(cream.r, cream.g, cream.b, 1)),
			group("mountain", polygon([2]float64{0, 405}, [2]float64{155, 550}, [2]float64{-155, 550}),
				fill(deepBlue.r, deepBlue.g, deepBlue.b, 1))))
	return doc("ocean-anim", 1280, 720, f, layers, []obj{
		marker("wave-seg", 0, 180),
		marker("sea-seg", 180, 120),
	})
}

// titleClip is the badge behind the game title text: rays fan out, a sun
// disc pops, a navy band slides in for the lettering; the idle segment
// keeps the rays turning (45 degrees per loop, so the eight-ray fan wraps
// seamlessly) and the disc breathing.
func titleClip() obj {
	const f = 150 // in: [0,45), idle loop: [45,150)
	rayScale := anim(
		key(0, []float64{0, 0}, true),
		key(26, []float64{112, 112}, true),
		key(40, []float64{100, 100}, true))
	raySpin := anim(
		key(0, []float64{-30}, true),
		key(45, []float64{0}, false),
		key(f, []float64{45}, false))
	discPop := anim(
		key(6, []float64{0, 0}, true),
		key(30, []float64{116, 116}, true),
		key(45, []float64{100, 100}, false),
		key(97, []float64{105, 105}, false),
		key(f, []float64{100, 100}, false))
	bandPop := anim(
		key(18, []float64{0, 100}, true),
		key(45, []float64{100, 100}, false))
	ray := func(deg float64) obj {
		return obj{"ty": "gr", "nm": "ray", "it": []any{
			rect(470, 18, 9), fill(1, 0.84, 0.35, 0.9),
			obj{"ty": "tr", "a": static([]float64{0, 0}), "p": static([]float64{0, 0}),
				"s": static([]float64{100, 100}), "r": static(deg), "o": static(100.0)},
		}}
	}
	return doc("title-anim", 700, 360, f, []obj{
		layerAt("band", 1, f, static([]float64{350, 180}), bandPop, static(-2.0), static(100.0),
			group("band", rect(600, 130, 26), fill(0.10, 0.14, 0.30, 1)),
			group("edge", rect(600, 130, 26), stroke(1, 0.84, 0.35, 0.9, 6))),
		layerAt("disc", 2, f, static([]float64{350, 180}), discPop, static(0.0), static(100.0),
			group("disc", ellipse(260, 260), fill(0.98, 0.55, 0.18, 1)),
			group("core", ellipse(190, 190), fill(1, 0.76, 0.32, 1))),
		layerAt("rays", 3, f, static([]float64{350, 180}), rayScale, raySpin, static(85.0),
			ray(0), ray(45), ray(90), ray(135)),
	}, []obj{
		marker("in-seg", 0, 45),
		marker("idle-seg", 45, 105),
	})
}

type rgb struct{ r, g, b float64 }

// darkBackdropClip is the vanity card's stage: near-black with a few
// slowly pulsing stars, looping.
func darkBackdropClip() obj {
	const f = 180
	star := func(ind int, x, y, size, phase float64) obj {
		lo, hi := 25.0, 90.0
		mid := key(phase, []float64{hi}, false)
		fade := anim(
			key(0, []float64{lo}, false), mid,
			key(f, []float64{lo}, false))
		return layerAt("star", ind, f, static([]float64{x, y}), static([]float64{100, 100}),
			static(0.0), fade,
			group("s", ellipse(size, size), fill(1, 1, 1, 1)))
	}
	return doc("bg-dark-anim", 1280, 720, f, []obj{
		star(1, 210, 120, 8, 50),
		star(2, 1080, 90, 6, 110),
		star(3, 890, 250, 7, 80),
		star(4, 380, 540, 6, 140),
		star(5, 1180, 470, 8, 30),
		layerAt("bg", 6, f, static([]float64{640, 360}), static([]float64{100, 100}),
			static(0.0), static(100.0),
			group("bg", rect(1300, 740, 0), fill(0.05, 0.06, 0.12, 1))),
	}, nil)
}

// skyClip is the opening's backdrop, the woodblock print's unbleached
// paper: flat beige with two pale cloud bands swaying gently, looping.
func skyClip() obj {
	const f = 240
	sway := func(x, y, amp float64) obj {
		return anim(
			key(0, []float64{x, y}, true),
			key(f/2, []float64{x + amp, y}, true),
			key(f, []float64{x, y}, false))
	}
	cloudBand := func(ind int, x, y, w, s, amp float64) obj {
		return layerAt("cloud", ind, f, sway(x, y, amp),
			static([]float64{s, s}), static(0.0), static(60.0),
			group("band",
				ellipseAt(0, 0, w, 46), ellipseAt(w*0.32, 20, w*0.55, 36),
				ellipseAt(-w*0.3, 24, w*0.45, 30),
				fill(cream.r, cream.g, cream.b, 1)))
	}
	return doc("bg-sky-anim", 1280, 720, f, []obj{
		cloudBand(1, 300, 140, 380, 100, 30),
		cloudBand(2, 950, 250, 300, 80, -26),
		layerAt("horizon-haze", 3, f, static([]float64{640, 500}), static([]float64{100, 100}),
			static(0.0), static(100.0),
			group("g", rect(1300, 220, 0), fill(0.90, 0.86, 0.76, 1))),
		layerAt("sky", 4, f, static([]float64{640, 360}), static([]float64{100, 100}),
			static(0.0), static(100.0),
			group("bg", rect(1300, 740, 0), fill(0.85, 0.80, 0.68, 1))),
	}, nil)
}
