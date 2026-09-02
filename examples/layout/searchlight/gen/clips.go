package main

// The searchlight sample's clips: a dark kitchen at night, one actor per
// bundle, drawn at different parallax depths. Everything is authored in
// this repository. Marker names end in -seg, clip ids in -anim, matching
// the other samples' convention.

var (
	night     = rgb{0.06, 0.07, 0.13}
	wallCol   = rgb{0.19, 0.20, 0.29}
	floorCol  = rgb{0.14, 0.11, 0.10}
	floorSeam = rgb{0.10, 0.08, 0.07}
	wood      = rgb{0.36, 0.22, 0.13}
	woodLite  = rgb{0.50, 0.32, 0.18}
	shelfBack = rgb{0.22, 0.13, 0.08}
	cream     = rgb{0.96, 0.93, 0.85}
	gold      = rgb{0.80, 0.64, 0.28}
	inkDark   = rgb{0.12, 0.09, 0.07}
	cheeseY   = rgb{0.98, 0.80, 0.25}
	cheeseD   = rgb{0.82, 0.60, 0.14}
	mouseG    = rgb{0.62, 0.62, 0.68}
	mouseLite = rgb{0.74, 0.74, 0.80}
	pink      = rgb{0.95, 0.60, 0.68}
	black     = rgb{0, 0, 0}
	beam      = rgb{1, 0.95, 0.72}
	silhouet  = rgb{0.04, 0.06, 0.05}
)

// wallClip is the back of the room at depth 0.5: wall, floor, a framed
// picture, a wall clock with a swinging pendulum, and a window with the
// moon breathing behind it. Loops.
func wallClip() obj {
	const f = 240
	swing := anim(
		key(0, []float64{-9}, true),
		key(f/2, []float64{9}, true),
		key(f, []float64{-9}, false))
	moonGlow := anim(
		key(0, []float64{16}, true),
		key(f/2, []float64{42}, true),
		key(f, []float64{16}, false))
	seam := func(ind int, x float64) obj {
		return still("seam", ind, f, x, 620, group("s", rect(4, 200, 0), fill(floorSeam, 1)))
	}
	return doc("wall-anim", 1280, 720, f, []obj{
		// The pendulum hangs from the clock case and swings about its pivot.
		layerAt("pendulum", 1, f, static([]float64{640, 214}), s100, swing, o100,
			group("bob", ellipseAt(0, 96, 24, 24), fill(gold, 1)),
			group("rod", rectAt(0, 48, 5, 96, 0), fill(gold, 1))),
		still("clock", 2, f, 640, 160,
			group("pin", ellipse(8, 8), fill(inkDark, 1)),
			groupTr("hour", p0, s100, static(35.0), o100, rectAt(0, -14, 6, 32, 2), fill(inkDark, 1)),
			groupTr("minute", p0, s100, static(120.0), o100, rectAt(0, -20, 4, 44, 2), fill(inkDark, 1)),
			group("face", ellipse(104, 104), fill(cream, 1)),
			group("rim", ellipse(122, 122), fill(wood, 1)),
			group("case", rectAt(0, 44, 36, 90, 6), fill(wood, 1))),
		still("picture", 3, f, 420, 200,
			group("sun", ellipseAt(50, -32, 26, 26), fill(rgb{0.95, 0.80, 0.40}, 1)),
			group("hill", polygon([2]float64{-90, 55}, [2]float64{-30, -8}, [2]float64{18, 34},
				[2]float64{58, 8}, [2]float64{90, 55}), fill(rgb{0.35, 0.46, 0.42}, 1)),
			group("canvas", rect(180, 130, 0), fill(rgb{0.32, 0.46, 0.62}, 1)),
			group("frame", rect(200, 150, 4), fill(gold, 1))),
		still("window", 4, f, 980, 230,
			group("bar-v", rectAt(0, 0, 10, 260, 0), fill(woodLite, 1)),
			group("bar-h", rectAt(0, 0, 240, 10, 0), fill(woodLite, 1)),
			group("moon", ellipseAt(40, -52, 64, 64), fill(rgb{0.97, 0.94, 0.75}, 1)),
			groupTr("glow", p0, s100, r0, moonGlow, ellipseAt(40, -52, 124, 124), fill(rgb{0.97, 0.94, 0.75}, 1)),
			group("stars", ellipseAt(-72, -92, 5, 5), ellipseAt(-34, -18, 4, 4),
				ellipseAt(-90, 30, 3, 3), ellipseAt(70, 60, 4, 4), fill(cream, 0.9)),
			group("sky", rect(240, 260, 0), fill(night, 1)),
			group("frame", rect(266, 286, 6), fill(wood, 1))),
		still("baseboard", 5, f, 640, 524, group("b", rect(1280, 16, 0), fill(woodLite, 1))),
		seam(6, 160), seam(7, 420), seam(8, 700), seam(9, 960), seam(10, 1180),
		still("floor", 11, f, 640, 620, group("f", rect(1280, 200, 0), fill(floorCol, 1))),
		still("wall", 12, f, 640, 260, group("w", rect(1280, 520, 0), fill(wallCol, 1))),
	}, nil)
}

// shelfClip is a bookshelf at depth 0.7 with a spider bobbing on its
// thread from the top. 260x420, loops.
func shelfClip() obj {
	const f = 180
	bob := anim(
		key(0, []float64{206, 0}, true),
		key(f/2, []float64{206, 30}, true),
		key(f, []float64{206, 0}, false))
	type book struct {
		x, w, h float64
		c       rgb
	}
	rows := [][]book{
		{{22, 22, 74, rgb{0.55, 0.20, 0.18}}, {48, 26, 82, rgb{0.20, 0.35, 0.50}}, {76, 20, 68, rgb{0.70, 0.60, 0.25}},
			{104, 30, 80, rgb{0.25, 0.45, 0.30}}, {136, 24, 72, rgb{0.60, 0.30, 0.45}}},
		{{24, 28, 80, rgb{0.20, 0.35, 0.50}}, {54, 22, 66, rgb{0.70, 0.60, 0.25}}, {110, 34, 86, rgb{0.55, 0.20, 0.18}},
			{146, 26, 74, rgb{0.45, 0.45, 0.50}}, {176, 22, 62, rgb{0.25, 0.45, 0.30}}},
		{{30, 30, 84, rgb{0.60, 0.30, 0.45}}, {62, 24, 70, rgb{0.25, 0.45, 0.30}}, {90, 26, 78, rgb{0.20, 0.35, 0.50}},
			{160, 40, 60, rgb{0.70, 0.60, 0.25}}, {204, 30, 88, rgb{0.55, 0.20, 0.18}}},
		{{20, 24, 70, rgb{0.45, 0.45, 0.50}}, {46, 26, 84, rgb{0.55, 0.20, 0.18}}, {74, 22, 64, rgb{0.60, 0.30, 0.45}},
			{102, 30, 78, rgb{0.20, 0.35, 0.50}}, {134, 24, 70, rgb{0.70, 0.60, 0.25}}, {160, 24, 82, rgb{0.25, 0.45, 0.30}}},
	}
	var books []any
	for i, row := range rows {
		top := 96.0 + float64(i)*100 - 5
		for _, b := range row {
			books = append(books, group("book", rectAt(b.x+b.w/2, top-b.h/2, b.w, b.h, 2), fill(b.c, 1)))
		}
	}
	// One book leans against its neighbors, the way real shelves go.
	books = append(books, groupTr("leaning", static([]float64{212, 296 - 5}), s100, static(-16.0), o100,
		rectAt(-12, -40, 24, 80, 2), fill(rgb{0.45, 0.45, 0.50}, 1)))
	var boards []any
	for i := range 4 {
		boards = append(boards, rectAt(130, 96+float64(i)*100, 236, 10, 0))
	}
	boards = append(boards, fill(woodLite, 1))
	return doc("shelf-anim", 260, 420, f, []obj{
		layerAt("spider", 1, f, bob, s100, r0, o100,
			group("body", ellipseAt(0, 84, 16, 13), ellipseAt(0, 74, 9, 8), fill(inkDark, 1)),
			group("legs", curve(false, vx{-14, 78, 0, 0, 6, 4}, vx{0, 84, -6, -2, 6, 2}, vx{14, 78, -6, 4, 0, 0}),
				stroke(inkDark, 1, 2)),
			group("thread", rectAt(0, 36, 2, 76, 0), fill(cream, 0.5))),
		still("books", 2, f, 0, 0, books...),
		still("boards", 3, f, 0, 0, group("boards", boards...)),
		still("body", 4, f, 0, 0,
			group("back", rectAt(130, 210, 236, 396, 0), fill(shelfBack, 1)),
			group("frame", rectAt(130, 210, 260, 420, 4), fill(wood, 1))),
	}, nil)
}

// tableClip is the kitchen table at depth 1 with the plate of cheese,
// crumbs on the plate, and a crumb trail on the floor leading up to it.
// 600x420, static save for a candle-less stillness; loops trivially.
func tableClip() obj {
	const f = 60
	return doc("table-anim", 600, 420, f, []obj{
		still("crumbs", 1, f, 0, 0,
			group("plate-crumbs", ellipseAt(268, 142, 6, 4), ellipseAt(292, 158, 5, 4),
				ellipseAt(400, 156, 7, 4), ellipseAt(372, 130, 4, 3), fill(cheeseY, 1)),
			group("floor-trail", ellipseAt(60, 404, 7, 5), ellipseAt(104, 396, 6, 4),
				ellipseAt(150, 408, 8, 5), ellipseAt(196, 400, 5, 4), ellipseAt(236, 410, 6, 4),
				fill(cheeseY, 1))),
		still("cheese", 2, f, 330, 126,
			group("holes", ellipseAt(-20, -4, 12, 9), ellipseAt(18, -10, 8, 6), ellipseAt(4, 8, 7, 5),
				ellipseAt(38, 4, 6, 4), fill(cheeseD, 1)),
			group("top", polygon([2]float64{-72, 2}, [2]float64{68, -18}, [2]float64{58, 22}), fill(cheeseY, 1)),
			group("side", polygon([2]float64{58, 22}, [2]float64{68, -18}, [2]float64{68, -2},
				[2]float64{58, 38}, [2]float64{-72, 18}, [2]float64{-72, 2}), fill(cheeseD, 1))),
		still("plate", 3, f, 330, 150,
			group("rim", ellipse(230, 66), stroke(rgb{0.70, 0.72, 0.78}, 1, 4)),
			group("dish", ellipse(230, 66), fill(cream, 1))),
		still("top", 4, f, 300, 150,
			group("surface", ellipse(520, 124), fill(woodLite, 1)),
			group("edge", ellipseAt(0, 16, 520, 124), fill(wood, 1))),
		still("legs", 5, f, 0, 0,
			group("legs", rectAt(130, 280, 26, 220, 0), rectAt(470, 280, 26, 220, 0), fill(wood, 1))),
	}, nil)
}

// mouseClip is the culprit, at depth 1: a grey mouse facing the plate,
// nibbling (eat-seg, loops) — then startled when found: it jumps, its
// eye goes wide, and it keeps flinching (startled-seg, loops). 220x180.
func mouseClip() obj {
	const f = 100 // eat: [0,60), startled loop: [60,100)
	headPos := anim(
		key(0, []float64{150, 100}, false),
		key(60, []float64{150, 100}, false),
		key(64, []float64{150, 74}, true),
		key(72, []float64{150, 100}, true),
		key(f, []float64{150, 100}, false))
	headRot := anim(
		key(0, []float64{0}, true),
		key(15, []float64{7}, true),
		key(30, []float64{0}, true),
		key(45, []float64{7}, true),
		key(60, []float64{0}, false),
		key(64, []float64{-14}, true),
		key(70, []float64{-6}, true),
		key(78, []float64{-10}, true),
		key(86, []float64{-6}, true),
		key(94, []float64{-10}, true),
		key(f, []float64{0}, false))
	eyeScale := anim(
		key(0, []float64{100, 100}, false),
		key(60, []float64{100, 100}, false),
		key(64, []float64{170, 170}, true),
		key(f, []float64{170, 170}, false))
	bodyScale := anim(
		key(0, []float64{100, 100}, true),
		key(30, []float64{103, 103}, true),
		key(60, []float64{100, 100}, false),
		key(64, []float64{110, 88}, true),
		key(72, []float64{100, 100}, true),
		key(f, []float64{100, 100}, false))
	tailRot := anim(
		key(0, []float64{-12}, true),
		key(30, []float64{12}, true),
		key(60, []float64{-12}, false),
		key(64, []float64{38}, true),
		key(74, []float64{26}, true),
		key(88, []float64{34}, true),
		key(f, []float64{26}, false))
	return doc("mouse-anim", 220, 180, f, []obj{
		layerAt("head", 1, f, headPos, s100, headRot, o100,
			group("whiskers",
				curve(false, vx{30, 4, 0, 0, 0, 0}, vx{78, -8, 0, 0, 0, 0}),
				curve(false, vx{30, 10, 0, 0, 0, 0}, vx{80, 18, 0, 0, 0, 0}),
				stroke(cream, 0.85, 2)),
			group("nose", ellipseAt(36, 6, 13, 10), fill(pink, 1)),
			groupTr("eye", static([]float64{12, -8}), eyeScale, r0, o100,
				ellipseAt(2, -2, 4, 4), fill(cream, 1), ellipse(11, 11), fill(inkDark, 1)),
			group("ear-front-in", ellipseAt(6, -38, 18, 18), fill(pink, 1)),
			group("ear-front", ellipseAt(6, -38, 32, 32), fill(mouseG, 1)),
			group("face", ellipse(72, 60), fill(mouseG, 1)),
			group("ear-back-in", ellipseAt(-24, -32, 18, 18), fill(pink, 1)),
			group("ear-back", ellipseAt(-24, -32, 32, 32), fill(mouseG, 1))),
		layerAt("body", 2, f, static([]float64{110, 122}), bodyScale, r0, o100,
			group("feet", ellipseAt(-22, 40, 26, 12), ellipseAt(26, 40, 26, 12), fill(pink, 1)),
			group("belly", ellipseAt(4, 12, 84, 50), fill(mouseLite, 1)),
			group("body", ellipse(124, 82), fill(mouseG, 1))),
		layerAt("tail", 3, f, static([]float64{52, 128}), s100, tailRot, o100,
			group("tail", curve(false, vx{0, 0, 0, 0, -30, 6}, vx{-56, -16, 20, 14, -22, -14},
				vx{-84, 14, 6, -20, 0, 0}), stroke(pink, 1, 7))),
	}, []obj{
		marker("eat-seg", 0, 60),
		marker("startled-seg", 60, 40),
	})
}

// alertClip is the "!" that pops over the mouse when the light finds it.
// 120x140, one pass (pop-seg) — its completion is what startles the
// mouse, through a binding on the scene node.
func alertClip() obj {
	const f = 30
	pop := anim(
		key(0, []float64{0, 0}, true),
		key(12, []float64{128, 128}, true),
		key(20, []float64{100, 100}, false))
	wobble := anim(
		key(0, []float64{-14}, true),
		key(20, []float64{0}, false))
	return doc("alert-anim", 120, 140, f, []obj{
		layerAt("mark", 1, f, static([]float64{60, 90}), pop, wobble, o100,
			group("dot", ellipseAt(0, 40, 26, 26), fill(cheeseY, 1)),
			group("bar", rectAt(0, -18, 24, 74, 10), fill(cheeseY, 1)),
			group("dot-edge", ellipseAt(0, 40, 26, 26), stroke(inkDark, 1, 6)),
			group("bar-edge", rectAt(0, -18, 24, 74, 10), stroke(inkDark, 1, 6))),
	}, []obj{marker("pop-seg", 0, f)})
}

// plantClip is a potted plant in the foreground at depth 1.4, a near-
// black silhouette whose leaves sway. 320x520, loops.
func plantClip() obj {
	const f = 240
	sway := func(base float64) obj {
		return anim(
			key(0, []float64{base - 4}, true),
			key(f/2, []float64{base + 4}, true),
			key(f, []float64{base - 4}, false))
	}
	leaf := func(x, y, w, h, rot float64) obj {
		return groupTr("leaf", static([]float64{x, y}), s100, sway(rot), o100,
			ellipse(w, h), fill(silhouet, 1))
	}
	return doc("plant-anim", 320, 520, f, []obj{
		still("leaves", 1, f, 0, 0,
			leaf(96, 300, 150, 52, -36), leaf(226, 282, 160, 56, 32),
			leaf(160, 190, 64, 170, 0), leaf(66, 236, 124, 46, -62),
			leaf(254, 216, 124, 46, 58), leaf(130, 250, 110, 44, -18), leaf(196, 240, 110, 44, 16)),
		still("stem-pot", 2, f, 0, 0,
			group("stem", rectAt(160, 340, 12, 170, 0), fill(silhouet, 1)),
			group("rim", rectAt(160, 404, 200, 26, 6), fill(silhouet, 1)),
			group("pot", polygon([2]float64{72, 416}, [2]float64{248, 416}, [2]float64{226, 520},
				[2]float64{94, 520}), fill(silhouet, 1))),
	}, nil)
}

// maskClip is the searchlight itself, at depth 0 so it stays put on the
// screen while the camera roams: darkness with a soft-edged hole in the
// middle, breathing slightly, and dust motes drifting through the beam.
// The hole is an ellipse stroked thickly enough to reach past the
// screen corners (a ring, not a filled path with a hole). 1280x720,
// loops.
func maskClip() obj {
	const f = 240
	const cx, cy = 640.0, 360.0
	// A ring whose inner radius is r, extending out well past the design
	// box's corners (735 from the center).
	ring := func(r, opacity float64) obj {
		const outer = 1300.0
		w := outer - r
		return group("ring", ellipse(2*(r+w/2), 2*(r+w/2)), stroke(black, opacity, w))
	}
	breathe := anim(
		key(0, []float64{100, 100}, true),
		key(f/2, []float64{104, 104}, true),
		key(f, []float64{100, 100}, false))
	type mote struct{ x, y, dx, dy, size, birth float64 }
	motes := []mote{
		{-120, -60, 40, 70, 5, 0}, {80, -140, -30, 90, 4, 30}, {160, 40, -60, 50, 6, 60},
		{-40, 120, 50, -60, 4, 90}, {-170, 60, 70, 30, 5, 120}, {30, -30, 20, 80, 7, 150},
		{110, 150, -40, -70, 4, 180}, {-90, -150, 30, 90, 5, 210},
	}
	layers := make([]obj, 0, len(motes)+4)
	for i, m := range motes {
		end := m.birth + 120
		pos := anim(
			key(m.birth, []float64{cx + m.x, cy + m.y}, false),
			key(end, []float64{cx + m.x + m.dx, cy + m.y + m.dy}, false))
		fade := anim(
			key(m.birth, []float64{0}, false),
			key(m.birth+30, []float64{55}, true),
			key(end-30, []float64{45}, true),
			key(end, []float64{0}, false))
		layers = append(layers, layerAt("mote", i+1, f, pos, s100, r0, fade,
			group("m", ellipse(m.size, m.size), fill(beam, 1))))
	}
	n := len(motes)
	layers = append(layers,
		// Soft edge: the outermost ring is opaque, the two inside it
		// feather the beam's rim.
		layerAt("dark", n+1, f, static([]float64{cx, cy}), breathe, r0, o100,
			ring(250, 1), ring(236, 0.45), ring(222, 0.2)),
		layerAt("glow", n+2, f, static([]float64{cx, cy}), breathe, r0, static(7.0),
			group("g", ellipse(520, 520), fill(beam, 1))),
	)
	return doc("mask-anim", 1280, 720, f, layers, nil)
}
