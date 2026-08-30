package main

import "fmt"

// The four cuts. One composition, cuts as layer in/out windows: cut 1 the
// pig, cut 2 the squirrel, cut 3 the corner, cut 4 the aftermath. The
// manga vocabulary — speed lines, flash, star bursts, dizzy stars, the
// iris — each gets its own layer.

const (
	stageW = 480
	stageH = 272

	c1End = 66.0
	c2End = 120.0
	c3End = 150.0

	roadTop = 212.0
	feetY   = 240.0
)

var (
	white     = rgb{1, 1, 1}
	winBlue   = rgb{0.45, 0.58, 0.70}
	roadGray  = rgb{0.61, 0.63, 0.66}
	walkGray  = rgb{0.78, 0.80, 0.82}
	bushGreen = rgb{0.50, 0.77, 0.42}
	bushDark  = rgb{0.38, 0.65, 0.34}
	trunkCol  = rgb{0.54, 0.42, 0.30}
	starGold  = rgb{1, 0.83, 0.30}
	burstPale = rgb{1, 0.95, 0.78}
	markRed   = rgb{0.91, 0.35, 0.31}
	sweatBlue = rgb{0.62, 0.85, 0.96}
	irisInk   = rgb{0.07, 0.06, 0.08}
)

// scrollX is the transform of a layer that only pans: content is drawn in
// absolute coordinates and the layer slides linearly from x0 to x1.
func scrollX(ip, op, x0, x1 float64) obj {
	return transform(static([]float64{0, 0}),
		anim(keyL(ip, x0, 0), keyL(op, x1, 0)),
		static([]float64{100, 100}), static(0.0), static(100.0))
}

func skyLayer(ind int) obj {
	return solidLayer("sky", ind, "#AEE3F7", stageW, stageH, 0, totalF,
		tr(0, 0, 0, 0, 100, 0))
}

// roadLayer is the street every cut shares: surface and sidewalk strip.
func roadLayer(ind int) obj {
	return shapeLayer("road", ind, 0, 0, totalF, tr(0, 0, 0, 0, 100, 0), []any{
		group("walk", rect(240, 208, 480, 8, 0), fill(walkGray, 1)),
		group("surface", rect(240, 242, 480, 60, 0), fill(roadGray, 1)),
	})
}

// dashesLayer is the road's centre line, scrolling with the buildings so
// the ground moves too.
func dashesLayer(name string, ind int, ip, op, x0, x1 float64) obj {
	var ds []any
	for x := 20.0; x < 1180; x += 90 {
		ds = append(ds, rect(x, 246, 34, 6, 2))
	}
	return shapeLayer(name, ind, 0, ip, op, scrollX(ip, op, x0, x1),
		[]any{group("dashes", append(ds, fill(white, 0.9))...)})
}

// buildingsLayer is cut 1's town, wide enough for the whole scroll.
func buildingsLayer(ind int) obj {
	tints := []rgb{{0.85, 0.80, 0.72}, {0.72, 0.78, 0.84}, {0.83, 0.74, 0.80}}
	heights := []float64{118, 88, 142, 104, 126}
	var windows []any
	var bodies []any
	i := 0
	for x := 10.0; x < 1160; x += 102 {
		h := heights[i%len(heights)]
		cx := x + 44
		bodies = append(bodies, group(fmt.Sprintf("b%d", i),
			rect(cx, roadTop-h/2, 88, h, 2), fill(tints[i%len(tints)], 1), stroke(ink, outline)))
		for r := 0; r < int((h-40)/36); r++ {
			wy := roadTop - h + 26 + float64(r)*36
			windows = append(windows, rect(cx-20, wy, 20, 22, 2), rect(cx+20, wy, 20, 22, 2))
		}
		i++
	}
	shapes := append([]any{group("windows", append(windows, fill(winBlue, 1))...)}, bodies...)
	return shapeLayer("town", ind, 0, 0, c1End, scrollX(0, c1End, 0, -560), shapes)
}

// parkLayer is cut 2's side of town: bushes with the odd tree.
func parkLayer(ind int) obj {
	var bushes []any
	var trees []any
	i := 0
	for x := 0.0; x < 1000; x += 96 {
		c := bushGreen
		if i%2 == 1 {
			c = bushDark
		}
		bushes = append(bushes, group(fmt.Sprintf("bush%d", i),
			ellipse(x+48, 186, 96, 52), fill(c, 1), stroke(ink, outline)))
		if i%3 == 1 {
			trees = append(trees, group(fmt.Sprintf("tree%d", i),
				ellipse(x+30, 138, 74, 56), ellipse(x+4, 154, 46, 38), ellipse(x+56, 154, 46, 38),
				fill(bushDark, 1), stroke(ink, outline)))
			trees = append(trees, group(fmt.Sprintf("trunk%d", i),
				rect(x+30, 190, 14, 44, 2), fill(trunkCol, 1), stroke(ink, outline)))
		}
		i++
	}
	return shapeLayer("park", ind, 0, c1End, c2End, scrollX(c1End, c2End, -460, 0),
		append(bushes, trees...))
}

func cloudShapes() []any {
	spots := [][]float64{{70, 52}, {240, 84}, {410, 44}, {580, 70}}
	var cs []any
	for i, s := range spots {
		cs = append(cs, group(fmt.Sprintf("cloud%d", i),
			ellipse(s[0], s[1], 64, 26), ellipse(s[0]-24, s[1]+8, 44, 20),
			ellipse(s[0]+26, s[1]+8, 48, 20), fill(white, 0.92)))
	}
	return cs
}

func cloudLayer(name string, ind int, ip, op, x0, x1 float64) obj {
	return shapeLayer(name, ind, 0, ip, op, scrollX(ip, op, x0, x1), cloudShapes())
}

// speedLayer streams manga speed lines across the frame; a wide band of
// streaks pans fast rather than each streak animating on its own.
func speedLayer(name string, ind int, ip, op, x0, x1 float64) obj {
	rows := []float64{56, 96, 148, 188, 228}
	var ls []any
	for i, y := range rows {
		for x := float64(i%3) * 80; x < 1700; x += 260 {
			ls = append(ls, rect(x, y, 130, 4, 2))
		}
	}
	return shapeLayer(name, ind, 0, ip, op, scrollX(ip, op, x0, x1),
		[]any{group("lines", append(ls, fill(white, 0.65))...)})
}

// cornerLayer is the building both runners cannot see past: a front face,
// a darker side face, and the corner edge between them.
func cornerLayer(ind int) obj {
	return shapeLayer("corner", ind, 0, c2End, totalF, tr(0, 0, 0, 0, 100, 0), []any{
		group("knob", ellipse(238, 190, 5, 5), fill(ink, 1)),
		group("door", rect(228, 190, 26, 44, 4), fill(rgb{0.50, 0.36, 0.24}, 1), stroke(ink, outline)),
		group("windows",
			rect(204, 100, 26, 30, 2), rect(252, 100, 26, 30, 2),
			rect(204, 150, 26, 30, 2), rect(252, 150, 26, 30, 2),
			fill(winBlue, 1), stroke(ink, 2)),
		group("face", rect(228, 136, 120, 152, 2), fill(rgb{0.88, 0.84, 0.76}, 1), stroke(ink, outline)),
		group("side", rect(302, 140, 28, 144, 2), fill(rgb{0.72, 0.68, 0.60}, 1), stroke(ink, outline)),
	})
}

// drop is a teardrop path centred on (x, y), scaled by s.
func drop(x, y, s float64) obj {
	return path(true,
		[][]float64{{x, y - 14*s}, {x + 8*s, y + 2*s}, {x, y + 12*s}, {x - 8*s, y + 2*s}},
		[][]float64{nil, {0, -6 * s}, {6 * s, 0}, {0, 6 * s}},
		[][]float64{nil, {0, 6 * s}, {-6 * s, 0}, {0, -6 * s}})
}

// sweatRunLayer flicks sweat off the sprinting pig, two drops popping and
// flying back every cycle. The cycle is unrolled with hold keys.
func sweatRunLayer(ind int) obj {
	var pos, op []obj
	for t := 0.0; t < c1End; t += pigCycle {
		pos = append(pos, keyL(t, 0, 0), keyH(t+8, -18, -13))
		op = append(op, keyL(t, 95), keyH(t+8, 0))
	}
	return shapeLayer("sweat-run", ind, 0, 0, c1End,
		transform(static([]float64{0, 0}), obj{"a": 1, "k": pos},
			static([]float64{100, 100}), static(0.0), obj{"a": 1, "k": op}),
		[]any{
			group("drop-a", drop(226, 136, 1), fill(sweatBlue, 1), stroke(ink, 2)),
			group("drop-b", drop(244, 118, 0.75), fill(sweatBlue, 1), stroke(ink, 2)),
		})
}

// --- cut 3: the collision -------------------------------------------------

func flashLayer(ind int) obj {
	return solidLayer("flash", ind, "#FFFFFF", stageW, stageH, 138, 146,
		transform(static([]float64{0, 0}), static([]float64{0, 0}),
			static([]float64{100, 100}), static(0.0),
			anim(keyE(138, 0), keyE(140, 100), keyE(142, 100), keyE(146, 0))))
}

func impactLayer(ind int) obj {
	return shapeLayer("impact", ind, 0, 139, 154,
		transform(static([]float64{0, 0}), static([]float64{242, 168}),
			anim(keyE(139, 15, 15), keyE(145, 125, 125), keyE(153, 140, 140)),
			anim(keyL(139, 0), keyL(153, 25)),
			anim(keyE(147, 100), keyE(153, 0))),
		[]any{
			group("core", polystar(0, 0, 8, 40, 17, 22.5), fill(white, 1)),
			group("burst", polystar(0, 0, 8, 72, 30, 0), fill(starGold, 1), stroke(ink, outline)),
		})
}

func scatterLayer(ind int) obj {
	spots := [][]float64{{58, -36}, {-56, -30}, {40, 44}, {-42, 40}}
	var gs []any
	for i, s := range spots {
		c := white
		if i%2 == 1 {
			c = starGold
		}
		gs = append(gs, group(fmt.Sprintf("spark%d", i),
			polystar(s[0], s[1], 4, 11, 4.4, float64(i)*20), fill(c, 1), stroke(ink, 2)))
	}
	return shapeLayer("scatter", ind, 0, 140, 154,
		transform(static([]float64{0, 0}), static([]float64{242, 168}),
			anim(keyE(140, 30, 30), keyE(152, 170, 170)),
			static(0.0),
			anim(keyE(140, 100), keyE(145, 100), keyE(152, 0))),
		gs)
}

func pigRun3Layer(ind int) obj {
	pos := anim(keyL(120, -70, feetY), keyL(139, 202, feetY),
		keyE(142, 172, 222), keyE(149, 148, 238))
	rot := anim(keyE(139, 0), keyE(149, -18))
	return precompLayer("pig-run-3", ind, "pig-comp", 200, 160, c2End, c3End,
		transform(static([]float64{100, 150}), pos, static([]float64{90, 90}), rot, static(100.0)))
}

func squirrelRun3Layer(ind int) obj {
	pos := anim(keyL(120, 555, feetY), keyL(139, 288, feetY),
		keyE(142, 320, 222), keyE(149, 346, 238))
	rot := anim(keyE(139, 0), keyE(149, 22))
	return precompLayer("squirrel-run-3", ind, "squirrel-comp", 160, 140, c2End, c3End,
		transform(static([]float64{78, 130}), pos, static([]float64{90, 90}), rot, static(100.0)))
}

// --- cut 4: the aftermath -------------------------------------------------

// shockLayer is the radial burst behind the sitting pair, fading as the
// shock wears off.
func shockLayer(ind int) obj {
	return shapeLayer("shock", ind, 0, c3End, 178,
		transform(static([]float64{0, 0}), static([]float64{240, 150}),
			static([]float64{100, 100}),
			anim(keyL(c3End, 0), keyL(178, 16)),
			anim(keyE(c3End, 85), keyE(176, 0))),
		[]any{group("burst", polystar(0, 0, 24, 430, 96, 0), fill(burstPale, 0.9))})
}

// dazeEyes are the swirl stand-in: two hollow circles.
func dazeEyes(x1, y1, d1, x2, y2, d2 float64) any {
	return group("eyes", ellipse(x1, y1, d1, d1), ellipse(x2, y2, d2, d2), stroke(ink, 2.5))
}

// pigSitLayer draws the pig flat on its behind, wobbling. Feet on local
// (0, 0); the layer rotation is the whole performance.
func pigSitLayer(ind int) obj {
	scale := anim(keyE(c3End, 90, 90), keyE(176, 90, 90), keyE(179, 95, 85), keyE(183, 90, 90))
	return shapeLayer("pig-sit", ind, 0, c3End, totalF,
		transform(static([]float64{0, 0}), static([]float64{148, feetY}),
			scale, osc(c3End, totalF, 20, 0, []float64{-4}, []float64{4}),
			static(100.0)),
		[]any{
			dazeEyes(12, -94, 11, -6, -97, 10),
			group("nostrils", ellipse(25, -85, 3.5, 6), ellipse(32, -84, 3.5, 6), fill(pigDark, 1)),
			group("snout", append([]any{ellipse(28, -85, 26, 19)}, inked(pigSnout)...)...),
			group("skull", append([]any{ellipse(4, -88, 62, 58)}, inked(pigPink)...)...),
			group("ear-near", append([]any{path(true,
				[][]float64{{14, -108}, {20, -130}, {38, -112}},
				[][]float64{nil, {-6, 4}, {0, -8}}, nil)}, inked(pigPink)...)...),
			group("ear-far", append([]any{path(true,
				[][]float64{{-16, -110}, {-24, -130}, {-2, -116}},
				[][]float64{nil, {-2, 8}, {2, -6}}, nil)}, inked(pigSnout)...)...),
			groupAt("leg-l", -28, -8, 24, append([]any{rect(0, 0, 34, 15, 7.5)}, inked(pigPink)...)...),
			groupAt("leg-r", 28, -8, -24, append([]any{rect(0, 0, 34, 15, 7.5)}, inked(pigPink)...)...),
			group("body", append([]any{ellipse(0, -36, 92, 64)}, inked(pigPink)...)...),
		})
}

func squirrelSitLayer(ind int) obj {
	scale := anim(keyE(c3End, 90, 90), keyE(173, 90, 90), keyE(176, 95, 84), keyE(180, 90, 90))
	return shapeLayer("squirrel-sit", ind, 0, c3End, totalF,
		transform(static([]float64{0, 0}), static([]float64{348, feetY}),
			scale, osc(c3End, totalF, 20, 10, []float64{4}, []float64{-4}),
			static(100.0)),
		[]any{
			dazeEyes(-14, -78, 10, 0, -80, 9),
			group("nose", ellipse(-24, -71, 5, 5), fill(ink, 1)),
			group("muzzle", ellipse(-18, -69, 17, 12), fill(sqBelly, 1)),
			group("skull", append([]any{ellipse(-2, -72, 46, 42)}, inked(sqBrown)...)...),
			group("ear-near", append([]any{path(true,
				[][]float64{{-22, -90}, {-24, -110}, {-6, -92}},
				[][]float64{nil, {-4, 6}, {0, -8}}, nil)}, inked(sqBrown)...)...),
			group("ear-far", append([]any{path(true,
				[][]float64{{6, -92}, {12, -110}, {22, -88}},
				[][]float64{nil, {-4, 4}, {0, -6}}, nil)}, inked(sqTail)...)...),
			groupAt("leg-l", -20, -6, 20, append([]any{rect(0, 0, 24, 12, 6)}, inked(sqBrown)...)...),
			groupAt("leg-r", 16, -6, -20, append([]any{rect(0, 0, 24, 12, 6)}, inked(sqBrown)...)...),
			group("belly", ellipse(-4, -28, 28, 24), fill(sqBelly, 1)),
			group("body", append([]any{ellipse(0, -30, 58, 46)}, inked(sqBrown)...)...),
			// The tail has given up: it droops to the ground behind.
			group("tail", append([]any{path(true,
				[][]float64{{20, -48}, {54, -30}, {44, -2}, {24, -18}},
				[][]float64{{-10, -8}, {2, -14}, {12, 6}, {6, 12}},
				[][]float64{{12, 8}, {-2, 16}, {-10, -6}, {-4, -10}})}, inked(sqTail)...)...),
		})
}

// dizzyLayer orbits stars above a head: the layer spins and its star
// groups sit off-anchor, so rotation is the orbit; squashing the layer
// vertically turns the circle into a halo.
func dizzyLayer(name string, ind int, x, y, radius, turns float64) obj {
	return shapeLayer(name, ind, 0, 154, 238,
		transform(static([]float64{0, 0}), static([]float64{x, y}),
			static([]float64{100, 45}),
			anim(keyL(154, 0), keyL(238, turns)),
			anim(keyE(154, 0), keyE(160, 100), keyE(228, 100), keyE(238, 0))),
		[]any{
			// The squashed layer would squash the stars too, so they are
			// pre-stretched vertically to come out round.
			groupAt("star-a", radius, 0, 0, stretchStar(9, 4.5, 0)...),
			groupAt("star-b", -radius, 4, 0, stretchStar(7, 3.5, 30)...),
		})
}

// stretchStar is a dizzy star counter-scaled against the halo squash.
func stretchStar(outer, inner, rot float64) []any {
	return []any{obj{"ty": "gr", "nm": "star", "it": []any{
		polystar(0, 0, 5, outer, inner, rot),
		fill(starGold, 1), stroke(ink, 2),
		obj{"ty": "tr", "a": static([]float64{0, 0}), "p": static([]float64{0, 0}),
			"s": static([]float64{100, 100 / 0.45}), "r": static(0.0), "o": static(100.0)},
	}}}
}

// toastFallLayer: the toast went up at the collision and comes down on the
// pig's head, squashing on landing.
func toastFallLayer(ind int) obj {
	pos := anim(keyA(156, 84, -30), keyL(168, 128, 44), keyE(176, 152, 126),
		keyE(179, 152, 118), keyE(183, 152, 125))
	rot := anim(keyL(156, 0), keyL(176, 375), keyE(183, 390))
	scale := anim(keyE(176, 90, 90), keyE(179, 115, 56), keyE(183, 81, 101), keyE(187, 90, 90))
	return shapeLayer("toast-fall", ind, 0, 156, totalF,
		transform(static([]float64{0, 0}), pos, scale, rot, static(100.0)),
		[]any{
			group("crumb", rect(0, 0, 28, 22, 5), fill(toastPale, 1)),
			group("crust", append([]any{rect(0, 0, 38, 32, 8)}, inked(toastEdge)...)...),
		})
}

// acornFallLayer: the acorn bonks the squirrel's head, bounces off, and
// rolls to a stop.
func acornFallLayer(ind int) obj {
	pos := anim(keyA(162, 430, -30), keyL(174, 352, 148),
		keyE(180, 316, 196), keyE(186, 302, 232), keyE(194, 288, 234))
	rot := anim(keyL(162, 0), keyL(186, -420), keyE(194, -470))
	return shapeLayer("acorn-fall", ind, 0, 162, totalF,
		transform(static([]float64{0, 0}), pos, static([]float64{90, 90}), rot, static(100.0)),
		[]any{acornGroups(0, 0)})
}

func exclaimLayer(ind int) obj {
	scale := anim(keyE(176, 0, 0), keyE(179, 130, 130), keyE(182, 100, 100))
	return shapeLayer("exclaim", ind, 0, 176, totalF,
		transform(static([]float64{0, 0}), static([]float64{186, 92}),
			scale, static(8.0), static(100.0)),
		[]any{
			groupAt("bar", 0, -10, 0, append([]any{rect(0, 0, 10, 26, 5)}, inked(markRed)...)...),
			groupAt("dot", 3, 14, 0, append([]any{ellipse(0, 0, 9, 9)}, inked(markRed)...)...),
		})
}

func sweatDropLayer(ind int) obj {
	pos := anim(keyE(188, 382, 132), keyE(212, 385, 143))
	scale := anim(keyE(188, 0, 0), keyE(191, 115, 115), keyE(194, 100, 100))
	return shapeLayer("sweat-drop", ind, 0, 188, totalF,
		transform(static([]float64{0, 0}), pos, scale, static(0.0), static(100.0)),
		[]any{group("drop", drop(0, 0, 1.4), fill(sweatBlue, 1), stroke(ink, 2.5))})
}

// irisLayer closes the scene: a black cover with an even-odd circular hole
// shrinking to nothing.
func irisLayer(ind int) obj {
	// The hole closes on the pig's toast-topped face, as tradition demands.
	hole := ellipseAnim(160, 150, anim(keyE(230, 700, 700), keyE(246, 0, 0)))
	return shapeLayer("iris", ind, 0, 230, totalF, tr(0, 0, 0, 0, 100, 0),
		[]any{group("iris", rect(240, 136, 980, 560, 0), hole, fillEO(irisInk))})
}
