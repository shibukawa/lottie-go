package main

// The two characters, each a precomp so every cut can reuse them. The run
// cycles are unrolled over the whole timeline: a precomp layer only windows
// its asset, so the asset must animate for as long as any window needs.

const (
	fps      = 30.0
	totalF   = 248.0
	pigCycle = 12.0
	sqCycle  = 10.0
)

var (
	ink       = rgb{0.16, 0.12, 0.14}
	pigPink   = rgb{0.96, 0.66, 0.72}
	pigSnout  = rgb{0.91, 0.53, 0.61}
	pigDark   = rgb{0.69, 0.30, 0.39}
	toastEdge = rgb{0.79, 0.54, 0.29}
	toastPale = rgb{0.95, 0.86, 0.66}
	sqBrown   = rgb{0.71, 0.47, 0.23}
	sqTail    = rgb{0.54, 0.33, 0.15}
	sqBelly   = rgb{0.91, 0.81, 0.66}
	acornCap  = rgb{0.45, 0.29, 0.15}
)

const outline = 3.0

// inked is the manga look: every filled part gets the same dark outline.
func inked(c rgb) []any { return []any{fill(c, 1), stroke(ink, outline)} }

// pigComp is the pig mid-sprint, facing right, toast in mouth. 200x160
// local, feet on y=150. The body is the parent of everything: legs hang
// from hip anchors on it, the head bobs against it, the toast rides the
// head. Layer inds are local to the precomp.
func pigComp() obj {
	const (
		bodyX, bodyY = 92.0, 108.0
		headX, headY = 138.0, 82.0
	)
	bounce := osc(0, totalF, pigCycle/4, 0,
		[]float64{bodyX, bodyY}, []float64{bodyX, bodyY - 5})
	body := shapeLayer("body", 4, 0, 0, totalF,
		transform(static([]float64{bodyX, bodyY}), bounce,
			static([]float64{100, 100}), static(6.0), static(100.0)),
		[]any{
			// First in the list draws in front, so the torso leads and the
			// tail sits behind it — the coloured squiggle over its own
			// slightly fatter ink pass.
			group("torso", append([]any{ellipse(bodyX, bodyY, 94, 64)}, inked(pigPink)...)...),
			group("tail", path(false,
				[][]float64{{46, 100}, {36, 92}, {44, 84}, {38, 78}},
				[][]float64{nil, {6, 4}, {-6, 0}, {4, 2}},
				[][]float64{{-8, -2}, {6, -6}, {-6, -2}, nil}),
				stroke(pigPink, outline+1)),
			group("tail-line", path(false,
				[][]float64{{46, 100}, {36, 92}, {44, 84}, {38, 78}},
				[][]float64{nil, {6, 4}, {-6, 0}, {4, 2}},
				[][]float64{{-8, -2}, {6, -6}, {-6, -2}, nil}),
				stroke(ink, outline+4)),
		})

	leg := func(name string, ind int, hipX, hipY float64, phase float64, near bool) obj {
		c := pigPink
		if !near {
			c = pigSnout
		}
		rot := osc(0, totalF, pigCycle/2, phase, []float64{-42}, []float64{42})
		return shapeLayer(name, ind, 4, 0, totalF,
			transform(static([]float64{0, 0}), static([]float64{hipX, hipY}),
				static([]float64{100, 100}), rot, static(100.0)),
			[]any{group(name, append([]any{rect(0, 15, 16, 36, 8)}, inked(c)...)...)})
	}

	headRot := osc(0, totalF, pigCycle/2, 0, []float64{-4}, []float64{4})
	head := shapeLayer("head", 3, 4, 0, totalF,
		transform(static([]float64{headX, headY}), static([]float64{headX, headY}),
			static([]float64{100, 100}), headRot, static(100.0)),
		[]any{
			group("nostrils", ellipse(160, 92, 4, 7), ellipse(170, 92, 4, 7), fill(pigDark, 1)),
			group("snout", append([]any{ellipse(165, 92, 30, 22)}, inked(pigSnout)...)...),
			group("eye", ellipse(146, 74, 7, 9), fill(ink, 1)),
			group("skull", append([]any{ellipse(headX, headY, 66, 62)}, inked(pigPink)...)...),
			group("ear-near", append([]any{path(true,
				[][]float64{{146, 52}, {148, 30}, {166, 46}},
				[][]float64{nil, {-6, 4}, {0, -8}}, nil)}, inked(pigPink)...)...),
			group("ear-far", append([]any{path(true,
				[][]float64{{116, 56}, {106, 36}, {130, 46}},
				[][]float64{nil, {-4, 6}, {2, -6}}, nil)}, inked(pigSnout)...)...),
		})

	toastRot := osc(0, totalF, pigCycle/2, 0, []float64{-20}, []float64{-9})
	toast := shapeLayer("toast", 2, 3, 0, totalF,
		transform(static([]float64{176, 104}), static([]float64{176, 104}),
			static([]float64{100, 100}), toastRot, static(100.0)),
		[]any{
			group("crumb", rect(180, 100, 28, 22, 5), fill(toastPale, 1)),
			group("crust", append([]any{rect(180, 100, 38, 32, 8)}, inked(toastEdge)...)...),
		})

	return precompAsset("pig-comp", []obj{
		toast,
		head,
		leg("leg-front-near", 1, 124, 124, 0, true),
		leg("leg-hind-near", 5, 64, 124, pigCycle/2, true),
		body,
		leg("leg-front-far", 6, 110, 122, pigCycle/4, false),
		leg("leg-hind-far", 7, 76, 122, 3*pigCycle/4, false),
	})
}

// squirrelComp is the squirrel mid-sprint, facing left, acorn in paws.
// 160x140 local, feet on y=130.
func squirrelComp() obj {
	const (
		bodyX, bodyY = 78.0, 102.0
		headX, headY = 48.0, 80.0
	)
	bounce := osc(0, totalF, sqCycle/4, 0,
		[]float64{bodyX, bodyY}, []float64{bodyX, bodyY - 4})
	body := shapeLayer("body", 4, 0, 0, totalF,
		transform(static([]float64{bodyX, bodyY}), bounce,
			static([]float64{100, 100}), static(-6.0), static(100.0)),
		[]any{
			group("belly", ellipse(66, 108, 30, 26), fill(sqBelly, 1)),
			group("torso", append([]any{ellipse(bodyX, bodyY, 60, 48)}, inked(sqBrown)...)...),
		})

	// The tail is most of the silhouette: a fat S-curve rising behind the
	// body, wagging from its base.
	tailRot := osc(0, totalF, sqCycle/2, 0, []float64{-10}, []float64{12})
	tail := shapeLayer("tail", 6, 4, 0, totalF,
		transform(static([]float64{104, 104}), static([]float64{104, 104}),
			static([]float64{100, 100}), tailRot, static(100.0)),
		[]any{group("tail", append([]any{path(true,
			[][]float64{{100, 112}, {140, 84}, {124, 34}, {96, 58}},
			[][]float64{{-10, 12}, {8, 18}, {16, -2}, {-6, -18}},
			[][]float64{{12, -14}, {6, -22}, {-16, 2}, {8, 22}})}, inked(sqTail)...)...)})

	leg := func(name string, ind int, hipX, hipY float64, phase float64, near bool) obj {
		c := sqBrown
		if !near {
			c = sqTail
		}
		rot := osc(0, totalF, sqCycle/2, phase, []float64{38}, []float64{-38})
		return shapeLayer(name, ind, 4, 0, totalF,
			transform(static([]float64{0, 0}), static([]float64{hipX, hipY}),
				static([]float64{100, 100}), rot, static(100.0)),
			[]any{group(name, append([]any{rect(0, 11, 11, 24, 5)}, inked(c)...)...)})
	}

	headRot := osc(0, totalF, sqCycle/2, 0, []float64{4}, []float64{-4})
	head := shapeLayer("head", 2, 4, 0, totalF,
		transform(static([]float64{headX, headY}), static([]float64{headX, headY}),
			static([]float64{100, 100}), headRot, static(100.0)),
		[]any{
			group("nose", ellipse(27, 87, 5, 5), fill(ink, 1)),
			group("muzzle", ellipse(34, 90, 18, 13), fill(sqBelly, 1)),
			group("eye", ellipse(38, 74, 6, 8), fill(ink, 1)),
			group("skull", append([]any{ellipse(headX, headY, 48, 44)}, inked(sqBrown)...)...),
			group("ear-near", append([]any{path(true,
				[][]float64{{30, 62}, {30, 40}, {46, 58}},
				[][]float64{nil, {-4, 6}, {0, -8}}, nil)}, inked(sqBrown)...)...),
			group("ear-far", append([]any{path(true,
				[][]float64{{52, 62}, {58, 42}, {68, 60}},
				[][]float64{nil, {-4, 4}, {0, -6}}, nil)}, inked(sqTail)...)...),
		})

	acorn := shapeLayer("acorn", 3, 4, 0, totalF,
		transform(static([]float64{36, 104}), static([]float64{36, 104}),
			static([]float64{100, 100}),
			osc(0, totalF, sqCycle/2, 0, []float64{-8}, []float64{8}), static(100.0)),
		[]any{acornGroups(36, 104)})

	return precompAsset("squirrel-comp", []obj{
		head,
		acorn,
		leg("leg-front-near", 1, 62, 118, 0, true),
		leg("leg-hind-near", 5, 94, 118, sqCycle/2, true),
		body,
		tail,
		leg("leg-front-far", 7, 70, 116, sqCycle/4, false),
		leg("leg-hind-far", 8, 88, 116, 3*sqCycle/4, false),
	})
}

// acornGroups draws an acorn centred at (x, y): nut, cap, stem. Shared by
// the squirrel rig and the falling acorn in cut 4.
func acornGroups(x, y float64) any {
	return group("acorn",
		groupAt("cap", x, y-6, 0, append([]any{rect(0, 0, 21, 9, 4)}, inked(acornCap)...)...),
		groupAt("nut", x, y+3, 0, append([]any{ellipse(0, 0, 18, 18)}, inked(toastEdge)...)...),
		groupAt("stem", x, y-12, 0, append([]any{rect(0, 0, 3, 6, 1.5)}, inked(acornCap)...)...),
	)
}
