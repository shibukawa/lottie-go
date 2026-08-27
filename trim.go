package lottie

import "math"

// trimSamples is the arc-length sampling resolution per cubic segment.
const trimSamples = 16

// trimmer extracts sub-ranges of bezier contours by arc length.
type trimmer struct {
	segLens []float64 // per-segment lengths of the current contour
	out     []geometry
	lens    []float64 // per-contour lengths (trimAcross)
}

// nextOut returns a reusable output slot, keeping its point buffers.
func (t *trimmer) nextOut(mat matrix) *geometry {
	if len(t.out) < cap(t.out) {
		t.out = t.out[:len(t.out)+1]
	} else {
		t.out = append(t.out, geometry{})
	}
	g := &t.out[len(t.out)-1]
	g.mat = mat
	g.alpha = 1
	g.xor = false
	g.bez.Closed = false
	g.bez.V = g.bez.V[:0]
	g.bez.I = g.bez.I[:0]
	g.bez.O = g.bez.O[:0]
	return g
}

// applyTrim trims the geometries collected so far in the current group.
func (r *renderer) applyTrim(n *shapeNode, f float64, groupStart int) {
	s := n.trimStart.scalarAt(f, 0) / 100
	e := n.trimEnd.scalarAt(f, 100) / 100
	o := n.trimOffset.scalarAt(f, 0) / 360
	if s > e {
		s, e = e, s
	}
	a := s + o
	b := e + o
	if b-a >= 1 {
		return // full path
	}
	geoms := r.geoms[groupStart:r.nGeoms]
	if len(geoms) == 0 {
		return
	}
	if b <= a {
		r.nGeoms = groupStart // empty range: drop the geometry
		return
	}
	// Normalize a into [0,1).
	base := math.Floor(a)
	a -= base
	b -= base

	t := &r.trim
	t.out = t.out[:0]
	if n.trimMode == 2 {
		t.trimAcross(geoms, a, b)
	} else {
		for gi := range geoms {
			t.trimContour(&geoms[gi], a, b)
		}
	}
	// Replace the collected range with the trimmed output.
	r.nGeoms = groupStart
	for i := range t.out {
		slot := r.nextGeom()
		copyGeomInto(slot, &t.out[i])
	}
}

// trimContour emits the [a,b) fraction range of one contour, where the
// range may wrap past 1.
func (t *trimmer) trimContour(g *geometry, a, b float64) {
	if b <= 1 {
		t.extract(g, a, b)
		return
	}
	t.extract(g, a, 1)
	t.extract(g, 0, b-1)
}

// trimAcross treats all contours as one concatenated path ("individually"
// trim mode in Lottie terms measures across shapes sequentially).
func (t *trimmer) trimAcross(geoms []geometry, a, b float64) {
	total := 0.0
	t.lens = t.lens[:0]
	for i := range geoms {
		l := t.contourLength(&geoms[i].bez)
		t.lens = append(t.lens, l)
		total += l
	}
	lens := t.lens
	if total <= 0 {
		return
	}
	emit := func(lo, hi float64) {
		start := 0.0
		for i := range geoms {
			frac := lens[i] / total
			if frac <= 0 {
				start += frac
				continue
			}
			end := start + frac
			s := math.Max(lo, start)
			e := math.Min(hi, end)
			if s < e {
				t.extract(&geoms[i], (s-start)/frac, (e-start)/frac)
			}
			start = end
		}
	}
	if b <= 1 {
		emit(a, b)
	} else {
		emit(a, 1)
		emit(0, b-1)
	}
}

// contourLength measures a contour and caches per-segment lengths.
func (t *trimmer) contourLength(b *bezierShape) float64 {
	n := len(b.V)
	if n < 2 {
		t.segLens = t.segLens[:0]
		return 0
	}
	segs := n - 1
	if b.Closed {
		segs = n
	}
	t.segLens = t.segLens[:0]
	total := 0.0
	for s := 0; s < segs; s++ {
		l := segmentLength(b, s)
		t.segLens = append(t.segLens, l)
		total += l
	}
	return total
}

func segmentPoints(b *bezierShape, s int) (p0, p1, p2, p3 [2]float64) {
	n := len(b.V)
	j := s
	k := (s + 1) % n
	p0 = b.V[j]
	p3 = b.V[k]
	p1 = p0
	if j < len(b.O) {
		p1[0] += b.O[j][0]
		p1[1] += b.O[j][1]
	}
	p2 = p3
	if k < len(b.I) {
		p2[0] += b.I[k][0]
		p2[1] += b.I[k][1]
	}
	return
}

func cubicPoint(p0, p1, p2, p3 [2]float64, u float64) (x, y float64) {
	mu := 1 - u
	a := mu * mu * mu
	b := 3 * mu * mu * u
	c := 3 * mu * u * u
	d := u * u * u
	return a*p0[0] + b*p1[0] + c*p2[0] + d*p3[0],
		a*p0[1] + b*p1[1] + c*p2[1] + d*p3[1]
}

func segmentLength(b *bezierShape, s int) float64 {
	p0, p1, p2, p3 := segmentPoints(b, s)
	px, py := p0[0], p0[1]
	total := 0.0
	for i := 1; i <= trimSamples; i++ {
		u := float64(i) / trimSamples
		x, y := cubicPoint(p0, p1, p2, p3, u)
		total += math.Hypot(x-px, y-py)
		px, py = x, y
	}
	return total
}

// segmentParamAtLength inverts arc length to curve parameter within one
// segment using the sampled chord table.
func segmentParamAtLength(b *bezierShape, s int, target float64) float64 {
	p0, p1, p2, p3 := segmentPoints(b, s)
	px, py := p0[0], p0[1]
	acc := 0.0
	for i := 1; i <= trimSamples; i++ {
		u := float64(i) / trimSamples
		x, y := cubicPoint(p0, p1, p2, p3, u)
		step := math.Hypot(x-px, y-py)
		if acc+step >= target {
			if step <= 0 {
				return u
			}
			frac := (target - acc) / step
			return (float64(i-1) + frac) / trimSamples
		}
		acc += step
		px, py = x, y
	}
	return 1
}

// splitCubic returns the control points of the [u0,u1] sub-segment.
func splitCubic(p0, p1, p2, p3 [2]float64, u0, u1 float64) (q0, q1, q2, q3 [2]float64) {
	// First cut at u0 keeping the right part, then cut that at the
	// remapped u1.
	r0, r1, r2, r3 := deCasteljauRight(p0, p1, p2, p3, u0)
	if u0 >= 1 {
		return r0, r1, r2, r3
	}
	w := (u1 - u0) / (1 - u0)
	return deCasteljauLeft(r0, r1, r2, r3, w)
}

func lerpPt(a, b [2]float64, u float64) [2]float64 {
	return [2]float64{a[0] + (b[0]-a[0])*u, a[1] + (b[1]-a[1])*u}
}

func deCasteljauLeft(p0, p1, p2, p3 [2]float64, u float64) (q0, q1, q2, q3 [2]float64) {
	a := lerpPt(p0, p1, u)
	bb := lerpPt(p1, p2, u)
	c := lerpPt(p2, p3, u)
	d := lerpPt(a, bb, u)
	e := lerpPt(bb, c, u)
	f := lerpPt(d, e, u)
	return p0, a, d, f
}

func deCasteljauRight(p0, p1, p2, p3 [2]float64, u float64) (q0, q1, q2, q3 [2]float64) {
	a := lerpPt(p0, p1, u)
	bb := lerpPt(p1, p2, u)
	c := lerpPt(p2, p3, u)
	d := lerpPt(a, bb, u)
	e := lerpPt(bb, c, u)
	f := lerpPt(d, e, u)
	return f, e, c, p3
}

// extract appends the [f0,f1] arc-length fraction range of the contour to
// t.out as a new open contour.
func (t *trimmer) extract(g *geometry, f0, f1 float64) {
	total := t.contourLength(&g.bez)
	if total <= 0 || f1 <= f0 {
		return
	}
	if f0 <= 0 && f1 >= 1 {
		out := t.nextOut(g.mat)
		out.alpha = g.alpha
		out.xor = g.xor
		out.bez.Closed = g.bez.Closed
		out.bez.V = append(out.bez.V, g.bez.V...)
		out.bez.I = append(out.bez.I, g.bez.I...)
		out.bez.O = append(out.bez.O, g.bez.O...)
		return
	}
	lenStart := f0 * total
	lenEnd := f1 * total

	// Locate start and end segments.
	segs := len(t.segLens)
	acc := 0.0
	startSeg, endSeg := -1, segs-1
	var startU, endU float64
	for s := 0; s < segs; s++ {
		l := t.segLens[s]
		if startSeg < 0 && acc+l >= lenStart {
			startSeg = s
			startU = segmentParamAtLength(&g.bez, s, lenStart-acc)
		}
		if acc+l >= lenEnd {
			endSeg = s
			endU = segmentParamAtLength(&g.bez, s, lenEnd-acc)
			break
		}
		acc += l
	}
	if startSeg < 0 {
		return
	}
	if endSeg < startSeg {
		endSeg, endU = startSeg, 1
	}

	out := t.nextOut(g.mat)
	out.alpha = g.alpha
	out.xor = g.xor

	push := func(v, in, o [2]float64) {
		out.bez.V = append(out.bez.V, v)
		out.bez.I = append(out.bez.I, in)
		out.bez.O = append(out.bez.O, o)
	}
	for s := startSeg; s <= endSeg; s++ {
		p0, p1, p2, p3 := segmentPoints(&g.bez, s)
		u0, u1 := 0.0, 1.0
		if s == startSeg {
			u0 = startU
		}
		if s == endSeg {
			u1 = endU
		}
		if u1 <= u0 {
			continue
		}
		q0, q1, q2, q3 := splitCubic(p0, p1, p2, p3, u0, u1)
		if len(out.bez.V) == 0 {
			push(q0, [2]float64{}, [2]float64{q1[0] - q0[0], q1[1] - q0[1]})
		} else {
			last := len(out.bez.V) - 1
			out.bez.O[last] = [2]float64{q1[0] - q0[0], q1[1] - q0[1]}
		}
		push(q3, [2]float64{q2[0] - q3[0], q2[1] - q3[1]}, [2]float64{})
	}
}
