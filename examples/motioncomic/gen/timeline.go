package main

// The motion-comic timeline: a camera null glides over an oversized comic
// page while panels slam or drift in and stay put, Gravity-Rush style.
// Panel interiors are precomps — a precomp layer clips to its own w/h, so
// sprites can drift inside a panel without leaking past the border.

import "image"

const (
	stageW = 480
	stageH = 272

	t2 = 62.0  // panel 2 slides in
	t3 = 124.0 // panel 3 slams in
	t4 = 188.0 // panel 4 fades in
)

// Panel centres on the 1000x920 virtual page.
var (
	p1c = [2]float64{250, 170}
	p2c = [2]float64{740, 190}
	p3c = [2]float64{500, 480}
	p4c = [2]float64{500, 770}
)

type dims map[string]image.Point

// imgAt places a raster asset by its centre at 50% (art is drawn at 2x).
func imgAt(d dims, name string, ind, parent int, ref string, ip, op, cx, cy float64) obj {
	return imageLayer(name, ind, parent, ref, ip, op,
		tr(float64(d[ref].X)/2, float64(d[ref].Y)/2, cx, cy, 50, 0))
}

// imgTr is imgAt with a custom transform; the anchor still needs to be the
// image centre, so callers get it computed here.
func imgTr(d dims, name string, ind, parent int, ref string, ip, op float64, pos, scale, rot, opacity any) obj {
	return imageLayer(name, ind, parent, ref, ip, op,
		transform(static([]float64{float64(d[ref].X) / 2, float64(d[ref].Y) / 2}),
			pos, scale, rot, opacity))
}

// --- panel interiors -----------------------------------------------------

func p1Comp(d dims) obj {
	return precompAsset("p1-comp", []obj{
		imgTr(d, "sweat", 1, 0, "img-drop", 0, totalF,
			static([]float64{308, 52}), static([]float64{40, 40}), static(14.0),
			pulses(0, totalF, 24, 0)),
		imgTr(d, "twinkle", 2, 0, "img-sparkle", 0, totalF,
			static([]float64{58, 44}), static([]float64{45, 45}), static(0.0),
			pulses(0, totalF, 24, 12)),
		imgTr(d, "pig", 3, 0, "img-pig-run", 0, totalF,
			osc(0, totalF, 5, 0, []float64{205, 155}, []float64{205, 151}),
			static([]float64{50, 50}),
			osc(0, totalF, 10, 0, []float64{-2}, []float64{2}),
			static(100.0)),
		imgTr(d, "lines", 4, 0, "img-lines", 0, totalF,
			anim(keyL(0, 235, 120), keyL(t2, 195, 120)),
			static([]float64{50, 50}), static(0.0), static(100.0)),
		imgTr(d, "town", 5, 0, "img-p1bg", 0, totalF,
			anim(keyL(0, 210, 120), keyL(t2, 190, 120)),
			static([]float64{50, 50}), static(0.0), static(100.0)),
	})
}

// pulses fades a layer in and out on a fixed cycle, unrolled.
func pulses(from, to, period, phase float64) obj {
	var keys []obj
	for t := from + phase; t < to; t += period {
		keys = append(keys,
			keyL(t, 0), keyL(t+3, 100), keyH(t+10, 100), keyL(t+15, 0))
	}
	return anim(keys...)
}

func p2Comp(d dims) obj {
	return precompAsset("p2-comp", []obj{
		imgTr(d, "petal-a", 1, 0, "img-petal", 0, totalF,
			anim(keyL(t2, 320, -20), keyL(120, 250, 270)),
			static([]float64{50, 50}),
			anim(keyL(t2, 0), keyL(120, 320)), static(100.0)),
		imgTr(d, "petal-b", 2, 0, "img-petal", 0, totalF,
			anim(keyL(t2+8, 120, -20), keyL(128, 60, 250)),
			static([]float64{40, 40}),
			anim(keyL(t2+8, 40), keyL(128, -280)), static(100.0)),
		imgTr(d, "squirrel", 3, 0, "img-squirrel-run", 0, totalF,
			osc(0, totalF, 5, 0, []float64{195, 158}, []float64{195, 154}),
			static([]float64{50, 50}),
			osc(0, totalF, 10, 0, []float64{2}, []float64{-2}),
			static(100.0)),
		imgTr(d, "lines", 4, 0, "img-lines", 0, totalF,
			anim(keyL(t2, 225, 120), keyL(120, 255, 120)),
			static([]float64{-50, 50}), static(0.0), static(100.0)),
		imgTr(d, "park", 5, 0, "img-p2bg", 0, totalF,
			anim(keyL(t2, 190, 120), keyL(120, 210, 120)),
			static([]float64{50, 50}), static(0.0), static(100.0)),
	})
}

func p3Comp(d dims) obj {
	// The pair trembles: held jitter keys, then stillness.
	var shake []obj
	shake = append(shake, keyE(t3, 410, 125))
	for t, i := t3+4.0, 0; t < 176; t, i = t+3, i+1 {
		dx := []float64{2.5, -2, 1.5, -2.5}[i%4]
		dy := []float64{-1.5, 1, -1, 1.5}[i%4]
		shake = append(shake, keyH(t, 410+dx, 125+dy))
	}
	shake = append(shake, keyE(180, 410, 125))
	return precompAsset("p3-comp", []obj{
		imgTr(d, "pair", 1, 0, "img-crash", 0, totalF,
			anim(shake...), static([]float64{50, 50}), static(0.0), static(100.0)),
		imgTr(d, "burst", 2, 0, "img-p3bg", 0, totalF,
			static([]float64{410, 120}),
			osc(t3, totalF, 6, 0, []float64{50, 50}, []float64{52, 52}),
			static(0.0), static(100.0)),
	})
}

func p4Comp(d dims) obj {
	layers := []obj{
		imgTr(d, "sweat", 1, 0, "img-drop", 0, totalF,
			anim(keyE(238, 365, 60), keyE(268, 367, 70)),
			anim(keyE(232, 0, 0), keyE(235, 57, 57), keyE(238, 50, 50)),
			static(10.0), static(100.0)),
		imgTr(d, "toast", 2, 0, "img-toast", 0, totalF,
			anim(keyA(214, 100, -30), keyL(220, 135, 55), keyE(226, 148, 92),
				keyE(229, 148, 84), keyE(233, 148, 91)),
			anim(keyE(226, 50, 50), keyE(229, 63, 38), keyE(233, 45, 56), keyE(237, 50, 50)),
			anim(keyL(214, 0), keyL(226, 370), keyE(233, 384)),
			anim(keyH(0, 0), keyH(214, 100))),
	}
	layers = append(layers, orbitLayers(d, "halo-pig", 3, 150, 85, 540)...)
	layers = append(layers, orbitLayers(d, "halo-squirrel", 6, 355, 95, -540)...)
	layers = append(layers,
		imgTr(d, "pig", 9, 0, "img-pig-sit", 0, totalF,
			static([]float64{148, 160}), static([]float64{50, 50}),
			osc(t4, totalF, 20, 0, []float64{-3}, []float64{3}), static(100.0)),
		imgTr(d, "squirrel", 10, 0, "img-squirrel-sit", 0, totalF,
			static([]float64{352, 162}), static([]float64{50, 50}),
			osc(t4, totalF, 20, 10, []float64{3}, []float64{-3}), static(100.0)),
		imgAt(d, "garden", 11, 0, "img-p4bg", 0, totalF, 250, 120),
	)
	return precompAsset("p4-comp", layers)
}

// orbitLayers is the dizzy halo: a spinning null with two sparkles riding
// off-anchor, so the layer rotation is the orbit.
func orbitLayers(d dims, name string, nullInd int, x, y, turns float64) []obj {
	n := nullLayer(name, nullInd, 0, 0, totalF,
		transform(static([]float64{0, 0}), static([]float64{x, y}),
			static([]float64{100, 100}),
			anim(keyL(204, 0), keyL(290, turns)), static(100.0)))
	fade := anim(keyE(204, 0), keyE(210, 100), keyE(282, 100), keyE(290, 0))
	a := imgTr(d, name+"-a", nullInd+1, nullInd, "img-sparkle", 0, totalF,
		static([]float64{36, 0}), static([]float64{45, 45}), static(0.0), fade)
	b := imgTr(d, name+"-b", nullInd+2, nullInd, "img-sparkle", 0, totalF,
		static([]float64{-36, 6}), static([]float64{35, 35}), static(0.0), fade)
	return []obj{a, b, n}
}

// --- the page ------------------------------------------------------------

// cameraKS is the rig everything hangs from: the anchor tracks the focus
// point on the page, the position stays at screen centre (plus the crash
// shake), and scale leans in and out.
func cameraKS() obj {
	anchor := anim(
		keyE(0, p1c[0], p1c[1]), keyE(58, p1c[0], p1c[1]),
		keyE(74, p2c[0], p2c[1]), keyE(116, p2c[0], p2c[1]),
		keyE(126, p3c[0], p3c[1]), keyE(186, p3c[0], p3c[1]),
		keyE(202, p4c[0], p4c[1]))
	pos := anim(
		keyE(0, 240, 136), keyE(t3+2, 240, 136),
		keyH(t3+3, 246, 131), keyH(t3+5, 234, 141), keyH(t3+7, 245, 133),
		keyH(t3+9, 236, 140), keyH(t3+11, 243, 134), keyE(t3+16, 240, 136))
	scale := anim(
		keyE(0, 100, 100), keyE(116, 100, 100),
		keyE(126, 64, 64), keyE(129, 60, 60), keyE(135, 66, 66),
		keyE(186, 66, 66), keyE(202, 92, 92))
	rot := anim(keyE(116, 0), keyE(128, -2.5), keyE(142, 0))
	return transform(anchor, pos, scale, rot, static(100.0))
}

// panelNull carries one panel's entrance; frame and interior parent to it.
func panelNull(name string, ind int, c [2]float64, ip float64, pos, scale, rot, opacity any) obj {
	return baseLayer(3, name, ind, 17, ip, totalF,
		transform(static([]float64{c[0], c[1]}), pos, scale, rot, opacity))
}

func buildDoc(d dims, assets []obj) obj {
	stat100 := static([]float64{100, 100})
	nulls := []obj{
		panelNull("p1-null", 13, p1c, 0,
			anim(keyE(0, -130, p1c[1]), keyE(9, 282, p1c[1]), keyE(14, p1c[0], p1c[1])),
			stat100, anim(keyE(0, -9), keyE(14, -2.5)), static(100.0)),
		panelNull("p2-null", 14, p2c, t2,
			anim(keyE(t2, 1300, p2c[1]), keyE(t2+8, 706, p2c[1]), keyE(t2+13, p2c[0], p2c[1])),
			stat100, anim(keyE(t2, 7), keyE(t2+13, 2)), static(100.0)),
		panelNull("p3-null", 15, p3c, t3,
			static([]float64{p3c[0], p3c[1]}),
			anim(keyE(t3, 170, 170), keyE(t3+4, 92, 92), keyE(t3+8, 100, 100)),
			static(0.0), anim(keyE(t3, 0), keyE(t3+2, 100))),
		panelNull("p4-null", 16, p4c, t4,
			anim(keyE(t4, p4c[0], p4c[1]+36), keyE(t4+10, p4c[0], p4c[1])),
			stat100, static(-1.5), anim(keyE(t4, 0), keyE(t4+8, 100))),
	}
	camera := nullLayer("camera", 17, 0, 0, totalF, cameraKS())

	frame := func(name string, ind, parent int, ref string, ip float64, c [2]float64) obj {
		return imgAt(d, name, ind, parent, ref, ip, totalF, c[0], c[1])
	}
	inner := func(name string, ind, parent int, ref string, w, h int, ip float64, c [2]float64) obj {
		return precompLayer(name, ind, parent, ref, w, h, ip, totalF,
			tr(float64(w)/2, float64(h)/2, c[0], c[1], 100, 0))
	}

	layers := []obj{
		imgTr(d, "final-twinkle", 1, 0, "img-sparkle", 290, totalF,
			static([]float64{240, 136}),
			anim(keyE(290, 0, 0), keyE(296, 140, 140), keyE(300, 120, 120)),
			static(15.0), static(100.0)),
		solidLayer("whiteout", 2, "#FFFFFF", stageW, stageH, 286, totalF,
			transform(static([]float64{0, 0}), static([]float64{0, 0}),
				stat100, static(0.0), anim(keyE(286, 0), keyE(298, 100)))),
		solidLayer("flash", 3, "#FFFFFF", stageW, stageH, t3, t3+10,
			transform(static([]float64{0, 0}), static([]float64{0, 0}),
				stat100, static(0.0),
				anim(keyE(t3, 0), keyE(t3+2, 100), keyE(t3+4, 100), keyE(t3+9, 0)))),
		imgTr(d, "sfx-don", 4, 15, "img-sfx", t3+4, totalF,
			static([]float64{310, 400}),
			anim(keyE(t3+4, 0, 0), keyE(t3+8, 62, 62), keyE(t3+12, 50, 50)),
			static(-8.0), static(100.0)),
		frame("p4-frame", 5, 16, "img-frame4", t4, p4c),
		inner("p4-inner", 6, 16, "p4-comp", 500, 240, t4, p4c),
		frame("p3-frame", 7, 15, "img-frame3", t3, p3c),
		inner("p3-inner", 8, 15, "p3-comp", 820, 240, t3, p3c),
		frame("p2-frame", 9, 14, "img-frame-r", t2, p2c),
		inner("p2-inner", 10, 14, "p2-comp", 400, 240, t2, p2c),
		frame("p1-frame", 11, 13, "img-frame-r", 0, p1c),
		inner("p1-inner", 12, 13, "p1-comp", 400, 240, 0, p1c),
	}
	layers = append(layers, nulls...)
	layers = append(layers, camera,
		solidLayer("paper", 18, "#F8F5F0", stageW, stageH, 0, totalF,
			tr(0, 0, 0, 0, 100, 0)))

	comps := []obj{p1Comp(d), p2Comp(d), p3Comp(d), p4Comp(d)}
	markers := []obj{
		marker("panel1-seg", 0, t2),
		marker("panel2-seg", t2, t3-t2),
		marker("panel3-seg", t3, t4-t3),
		marker("panel4-seg", t4, totalF-t4),
	}
	return doc("motioncomic", stageW, stageH, totalF,
		append(assets, comps...), layers, markers)
}
