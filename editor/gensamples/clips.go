package main

// The sample character: a rounded body over a ground shadow, animated
// differently per clip. Each clip has its own colour so a state change is
// obvious the moment it happens in the preview.

const (
	size    = 200.0
	groundY = 162.0
	restY   = 118.0
)

type rgb struct{ r, g, b float64 }

var (
	colIdle = rgb{0.29, 0.62, 1.00}
	colWalk = rgb{0.16, 0.74, 0.67}
	colRun  = rgb{0.35, 0.78, 0.35}
	colJump = rgb{0.98, 0.62, 0.20}
	colHurt = rgb{0.93, 0.33, 0.33}
)

// shadow is a flat ellipse on the ground, so vertical motion reads clearly.
func shadow(frames float64, scaleKeys obj) obj {
	return shapeLayer("shadow", 2, frames,
		transform(static([]float64{0, 0}), static([]float64{size / 2, groundY}),
			scaleKeys, static(0.0), static(28.0)),
		[]any{group("g", ellipse(96, 22), fill(0, 0, 0, 1))})
}

// body is the character itself: a rounded rect with a lighter inner block so
// rotation is visible.
func body(frames float64, tr obj, c rgb) obj {
	return shapeLayer("body", 1, frames, tr, []any{
		group("outer", rect(96, 96, 22), fill(c.r, c.g, c.b, 1)),
		group("inner", rect(44, 20, 10), fill(1, 1, 1, 0.85)),
	})
}

func bodyTransform(pos, scale, rot any) obj {
	return transform(static([]float64{0, 0}), pos, scale, rot, static(100.0))
}

// idleClip breathes in place.
func idleClip() obj {
	const f = 90
	pos := anim(
		key(0, []float64{size / 2, restY}, true),
		key(45, []float64{size / 2, restY - 5}, true),
		key(f, []float64{size / 2, restY}, true))
	scale := anim(
		key(0, []float64{100, 100}, true),
		key(45, []float64{103, 96}, true),
		key(f, []float64{100, 100}, true))
	return doc("idle", size, size, f, []obj{
		body(f, bodyTransform(pos, scale, static(0.0)), colIdle),
		shadow(f, static([]float64{100, 100})),
	}, nil)
}

// walkClip rocks side to side with a small hop.
func walkClip() obj {
	const f = 48
	pos := anim(
		key(0, []float64{size/2 - 10, restY}, false),
		key(12, []float64{size / 2, restY - 8}, false),
		key(24, []float64{size/2 + 10, restY}, false),
		key(36, []float64{size / 2, restY - 8}, false),
		key(f, []float64{size/2 - 10, restY}, false))
	rot := anim(
		key(0, []float64{-7}, false),
		key(24, []float64{7}, false),
		key(f, []float64{-7}, false))
	return doc("walk", size, size, f, []obj{
		body(f, bodyTransform(pos, static([]float64{100, 100}), rot), colWalk),
		shadow(f, static([]float64{100, 100})),
	}, nil)
}

// runClip is the same idea, faster and leaning further.
func runClip() obj {
	const f = 24
	pos := anim(
		key(0, []float64{size/2 - 14, restY}, false),
		key(6, []float64{size / 2, restY - 16}, false),
		key(12, []float64{size/2 + 14, restY}, false),
		key(18, []float64{size / 2, restY - 16}, false),
		key(f, []float64{size/2 - 14, restY}, false))
	rot := anim(
		key(0, []float64{-16}, false),
		key(12, []float64{16}, false),
		key(f, []float64{-16}, false))
	sc := anim(
		key(0, []float64{104, 96}, false),
		key(6, []float64{96, 106}, false),
		key(12, []float64{104, 96}, false),
		key(18, []float64{96, 106}, false),
		key(f, []float64{104, 96}, false))
	return doc("run", size, size, f, []obj{
		body(f, bodyTransform(pos, sc, rot), colRun),
		shadow(f, static([]float64{100, 100})),
	}, nil)
}

// jumpClip is a one-shot arc with squash on take-off and landing.
func jumpClip() obj {
	const f = 36
	pos := anim(
		key(0, []float64{size / 2, restY}, true),
		key(6, []float64{size / 2, restY + 8}, true),
		key(18, []float64{size / 2, 48}, true),
		key(30, []float64{size / 2, restY + 6}, true),
		key(f, []float64{size / 2, restY}, true))
	sc := anim(
		key(0, []float64{100, 100}, true),
		key(6, []float64{118, 82}, true),
		key(18, []float64{88, 114}, true),
		key(30, []float64{116, 84}, true),
		key(f, []float64{100, 100}, true))
	// The shadow shrinks as the character rises, which sells the height.
	sh := anim(
		key(0, []float64{100, 100}, true),
		key(18, []float64{48, 48}, true),
		key(f, []float64{100, 100}, true))
	return doc("jump", size, size, f, []obj{
		body(f, bodyTransform(pos, sc, static(0.0)), colJump),
		shadow(f, sh),
	}, nil)
}

// hurtClip is a one-shot shake.
func hurtClip() obj {
	const f = 24
	pos := anim(
		key(0, []float64{size / 2, restY}, false),
		key(3, []float64{size/2 - 12, restY}, false),
		key(7, []float64{size/2 + 10, restY}, false),
		key(11, []float64{size/2 - 7, restY}, false),
		key(15, []float64{size/2 + 4, restY}, false),
		key(f, []float64{size / 2, restY}, false))
	rot := anim(
		key(0, []float64{0}, false),
		key(7, []float64{-12}, false),
		key(15, []float64{9}, false),
		key(f, []float64{0}, false))
	return doc("hurt", size, size, f, []obj{
		body(f, bodyTransform(pos, static([]float64{100, 100}), rot), colHurt),
		shadow(f, static([]float64{100, 100})),
	}, nil)
}

// sheetClip packs three actions into one document, named by markers. It is
// the sample for PlaybackState.segment.
func sheetClip() obj {
	const f = 180
	pos := anim(
		// idle: 0-60
		key(0, []float64{size / 2, restY}, true),
		key(30, []float64{size / 2, restY - 6}, true),
		key(60, []float64{size / 2, restY}, true),
		// walk: 60-120
		key(75, []float64{size / 2, restY - 10}, false),
		key(90, []float64{size/2 + 12, restY}, false),
		key(105, []float64{size / 2, restY - 10}, false),
		key(120, []float64{size / 2, restY}, false),
		// jump: 120-180
		key(138, []float64{size / 2, 52}, true),
		key(168, []float64{size / 2, restY}, true),
		key(f, []float64{size / 2, restY}, true))
	return doc("actions", size, size, f, []obj{
		body(f, bodyTransform(pos, static([]float64{100, 100}), static(0.0)), colIdle),
		shadow(f, static([]float64{100, 100})),
	}, []obj{
		marker("idle", 0, 60),
		marker("walk", 60, 60),
		marker("jump", 120, 60),
	})
}
