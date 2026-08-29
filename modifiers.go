package lottie

import "math"

// maxPolystarPoints bounds runaway point counts: the value comes straight
// from the file, and an absurd one would otherwise allocate gigabytes of
// vertices per frame.
const maxPolystarPoints = 1024

// polystarShape builds a star or polygon centered at (cx, cy), following
// lottie-web's construction: vertices on alternating radii with roundness
// tangents perpendicular to the radius.
func polystarShape(dst *bezierShape, star bool, cx, cy, points, rotDeg, outerR, innerR, outerRound, innerRound float64) {
	numPts := int(math.Floor(points))
	if numPts < 3 {
		numPts = 3
	}
	if numPts > maxPolystarPoints {
		numPts = maxPolystarPoints
	}
	dst.Closed = true
	dst.V = dst.V[:0]
	dst.I = dst.I[:0]
	dst.O = dst.O[:0]

	ang := rotDeg*math.Pi/180 - math.Pi/2
	if star {
		steps := numPts * 2
		step := math.Pi / float64(numPts)
		outerLen := 2 * math.Pi * outerR / float64(steps*2) * (outerRound / 100)
		innerLen := 2 * math.Pi * innerR / float64(steps*2) * (innerRound / 100)
		for i := 0; i < steps; i++ {
			rad := outerR
			tl := outerLen
			if i%2 == 1 {
				rad = innerR
				tl = innerLen
			}
			appendStarVertex(dst, cx, cy, rad, ang, tl)
			ang += step
		}
	} else {
		step := 2 * math.Pi / float64(numPts)
		tl := 2 * math.Pi * outerR / float64(numPts*4) * (outerRound / 100)
		for i := 0; i < numPts; i++ {
			appendStarVertex(dst, cx, cy, outerR, ang, tl)
			ang += step
		}
	}
}

func appendStarVertex(dst *bezierShape, cx, cy, rad, ang, tangentLen float64) {
	x := rad * math.Cos(ang)
	y := rad * math.Sin(ang)
	// Unit direction of travel (angle increases).
	var tx, ty float64
	if rad != 0 {
		tx, ty = -y/rad, x/rad
	}
	dst.V = append(dst.V, [2]float64{cx + x, cy + y})
	dst.O = append(dst.O, [2]float64{tx * tangentLen, ty * tangentLen})
	dst.I = append(dst.I, [2]float64{-tx * tangentLen, -ty * tangentLen})
}

// applyRoundCorners rounds sharp corners (vertices with zero tangents) of
// the geometries collected so far in the group.
func (r *renderer) applyRoundCorners(n *shapeNode, f float64, groupStart int) {
	radius := n.roundness.scalarAt(f, 0)
	if radius <= 0 {
		return
	}
	t := &r.trim
	for gi := groupStart; gi < r.nGeoms; gi++ {
		g := &r.geoms[gi]
		t.out = t.out[:0]
		out := t.nextOut(g.mat)
		out.alpha = g.alpha
		out.xor = g.xor
		out.bez.Closed = g.bez.Closed
		roundContour(&out.bez, &g.bez, radius)
		copyGeomInto(g, out)
	}
}

// roundedTangent matches AE's rounded-corner handle length.
const roundedTangent = 0.5519

func roundContour(dst, src *bezierShape, radius float64) {
	n := len(src.V)
	for i := 0; i < n; i++ {
		v := src.V[i]
		sharp := isZeroPt(tangentAt(src.I, i)) && isZeroPt(tangentAt(src.O, i))
		isFirst := i == 0
		isLast := i == n-1
		if !sharp || n < 3 || (!src.Closed && (isFirst || isLast)) {
			dst.V = append(dst.V, v)
			dst.I = append(dst.I, tangentAt(src.I, i))
			dst.O = append(dst.O, tangentAt(src.O, i))
			continue
		}
		prev := src.V[(i-1+n)%n]
		next := src.V[(i+1)%n]
		d1 := math.Hypot(v[0]-prev[0], v[1]-prev[1])
		d2 := math.Hypot(v[0]-next[0], v[1]-next[1])
		a1 := math.Min(radius, d1/2)
		a2 := math.Min(radius, d2/2)
		if d1 <= 0 || d2 <= 0 {
			dst.V = append(dst.V, v)
			dst.I = append(dst.I, [2]float64{})
			dst.O = append(dst.O, [2]float64{})
			continue
		}
		// Point pulled back toward the previous vertex.
		p1 := [2]float64{v[0] + (prev[0]-v[0])*a1/d1, v[1] + (prev[1]-v[1])*a1/d1}
		// Point pushed forward toward the next vertex.
		p2 := [2]float64{v[0] + (next[0]-v[0])*a2/d2, v[1] + (next[1]-v[1])*a2/d2}
		dst.V = append(dst.V, p1)
		dst.I = append(dst.I, [2]float64{})
		dst.O = append(dst.O, [2]float64{(v[0] - p1[0]) * roundedTangent, (v[1] - p1[1]) * roundedTangent})
		dst.V = append(dst.V, p2)
		dst.I = append(dst.I, [2]float64{(v[0] - p2[0]) * roundedTangent, (v[1] - p2[1]) * roundedTangent})
		dst.O = append(dst.O, [2]float64{})
	}
}

func tangentAt(pts [][2]float64, i int) [2]float64 {
	if i < len(pts) {
		return pts[i]
	}
	return [2]float64{}
}

func isZeroPt(p [2]float64) bool {
	return math.Abs(p[0]) < 1e-9 && math.Abs(p[1]) < 1e-9
}

// applyPuckerBloat pulls vertices toward the contour's centroid and pushes
// tangent endpoints away from it — or the reverse for negative amounts —
// following lottie-web's construction.
func (r *renderer) applyPuckerBloat(n *shapeNode, f float64, groupStart int) {
	amount := n.amount.scalarAt(f, 0) / 100
	if amount == 0 {
		return
	}
	for gi := groupStart; gi < r.nGeoms; gi++ {
		bez := &r.geoms[gi].bez
		nv := len(bez.V)
		if nv == 0 {
			continue
		}
		for len(bez.I) < nv {
			bez.I = append(bez.I, [2]float64{})
		}
		for len(bez.O) < nv {
			bez.O = append(bez.O, [2]float64{})
		}
		var cx, cy float64
		for _, v := range bez.V {
			cx += v[0]
			cy += v[1]
		}
		cx /= float64(nv)
		cy /= float64(nv)
		for i := 0; i < nv; i++ {
			v := bez.V[i]
			// Tangent endpoints in absolute coordinates.
			ix, iy := v[0]+bez.I[i][0], v[1]+bez.I[i][1]
			ox, oy := v[0]+bez.O[i][0], v[1]+bez.O[i][1]
			vx := v[0] + (cx-v[0])*amount
			vy := v[1] + (cy-v[1])*amount
			nix := ix - (cx-ix)*amount
			niy := iy - (cy-iy)*amount
			nox := ox - (cx-ox)*amount
			noy := oy - (cy-oy)*amount
			bez.V[i] = [2]float64{vx, vy}
			bez.I[i] = [2]float64{nix - vx, niy - vy}
			bez.O[i] = [2]float64{nox - vx, noy - vy}
		}
	}
}

// applyZigZag replaces each contour with ridge points sampled along its
// segments, offset alternately along the curve normal. Point type 2 links
// the ridges with smooth tangents (a wave); anything else leaves corners.
func (r *renderer) applyZigZag(n *shapeNode, f float64, groupStart int) {
	amp := n.amount.scalarAt(f, 0)
	freq := int(math.Max(0, math.Round(n.zzFreq.scalarAt(f, 1))))
	// The ridge count comes straight from the file; cap it like the
	// repeater and dash counts, or one bad value hangs every frame.
	if freq > maxZigZagRidges {
		freq = maxZigZagRidges
	}
	smooth := n.zzPoints.scalarAt(f, 1) == 2
	if amp == 0 {
		return
	}
	t := &r.trim
	for gi := groupStart; gi < r.nGeoms; gi++ {
		g := &r.geoms[gi]
		t.out = t.out[:0]
		out := t.nextOut(g.mat)
		out.alpha = g.alpha
		out.xor = g.xor
		zigZagContour(&out.bez, &g.bez, amp, freq, smooth)
		copyGeomInto(g, out)
	}
}

// maxZigZagRidges bounds ridges per segment, mirroring maxDashSegments.
const maxZigZagRidges = 1024

func zigZagContour(dst, src *bezierShape, amp float64, freq int, smooth bool) {
	n := len(src.V)
	dst.Closed = src.Closed
	if n < 2 {
		dst.V = append(dst.V, src.V...)
		dst.I = append(dst.I, src.I...)
		dst.O = append(dst.O, src.O...)
		return
	}
	segs := n - 1
	if src.Closed {
		segs = n
	}
	// lottie-web starts the first point inward (direction -1 for a
	// clockwise contour), not outward.
	dir := -1.0
	place := func(s int, u float64) {
		p0, p1, p2, p3 := segmentPoints(src, s)
		x, y := cubicPoint(p0, p1, p2, p3, u)
		var nx, ny float64
		if u == 0 && (src.Closed || s > 0) {
			// Corner vertices displace along the perpendicular of the
			// prev->next vertex chord (the corner bisector), the way
			// lottie-web's getProjectingAngle does — the outgoing
			// segment's normal would kink the corner along one edge.
			prev := s - 1
			if prev < 0 {
				prev = n - 1
			}
			pv, nv := src.V[prev], src.V[(s+1)%n]
			dx, dy := nv[0]-pv[0], nv[1]-pv[1]
			if l := math.Hypot(dx, dy); l > 0 {
				nx, ny = dy/l, -dx/l
			} else {
				nx, ny = cubicNormal(p0, p1, p2, p3, u)
			}
		} else {
			nx, ny = cubicNormal(p0, p1, p2, p3, u)
		}
		dst.V = append(dst.V, [2]float64{x + nx*amp*dir, y + ny*amp*dir})
		dir = -dir
	}
	// Each segment contributes its start vertex plus freq interior ridges;
	// an open contour then needs its final vertex appended.
	for s := 0; s < segs; s++ {
		for j := 0; j <= freq; j++ {
			place(s, float64(j)/float64(freq+1))
		}
	}
	if !src.Closed {
		place(segs-1, 1)
	}
	m := len(dst.V)
	dst.I = dst.I[:0]
	dst.O = dst.O[:0]
	for i := 0; i < m; i++ {
		if !smooth {
			dst.I = append(dst.I, [2]float64{})
			dst.O = append(dst.O, [2]float64{})
			continue
		}
		// Catmull-Rom style tangents from the neighboring ridges.
		prev, next := i-1, i+1
		if src.Closed {
			prev, next = (i-1+m)%m, (i+1)%m
		} else {
			prev, next = max(prev, 0), min(next, m-1)
		}
		tx := (dst.V[next][0] - dst.V[prev][0]) / 4
		ty := (dst.V[next][1] - dst.V[prev][1]) / 4
		dst.I = append(dst.I, [2]float64{-tx, -ty})
		dst.O = append(dst.O, [2]float64{tx, ty})
	}
}

// cubicNormal returns the unit normal of the segment at parameter u, falling
// back to the chord for degenerate tangents.
func cubicNormal(p0, p1, p2, p3 [2]float64, u float64) (nx, ny float64) {
	mu := 1 - u
	dx := 3*mu*mu*(p1[0]-p0[0]) + 6*mu*u*(p2[0]-p1[0]) + 3*u*u*(p3[0]-p2[0])
	dy := 3*mu*mu*(p1[1]-p0[1]) + 6*mu*u*(p2[1]-p1[1]) + 3*u*u*(p3[1]-p2[1])
	if dx == 0 && dy == 0 {
		dx, dy = p3[0]-p0[0], p3[1]-p0[1]
	}
	l := math.Hypot(dx, dy)
	if l == 0 {
		return 0, 0
	}
	return dy / l, -dx / l
}

// maxRepeaterCopies bounds runaway repeater values.
const maxRepeaterCopies = 512

// applyRepeater duplicates the geometry and draw commands collected so far
// in the group, applying the per-copy transform cumulatively and the
// start/end opacity ramp. Copies stack below the original (AE's default).
func (r *renderer) applyRepeater(n *shapeNode, f float64, mat matrix, groupStart, cmdStart int) {
	// lottie-web ceils fractional copies, so 2.2 animating copies show 3.
	copies := int(math.Ceil(n.copies.scalarAt(f, 1)))
	if copies > maxRepeaterCopies {
		copies = maxRepeaterCopies
	}
	if copies <= 0 {
		r.nGeoms = groupStart
		r.cmds = r.cmds[:cmdStart]
		return
	}
	offset := n.offset.scalarAt(f, 0)
	so := clamp01(n.repSO.scalarAt(f, 100) / 100)
	// A single unmoved copy still takes the start-opacity ramp, so only
	// skip the rework when that too is a no-op.
	if copies == 1 && offset == 0 && so >= 1 {
		return
	}
	a := n.repAnchor.atInto(f, &r.repABuf)
	p := n.repPos.atInto(f, &r.repPBuf)
	s := n.repScale.atInto(f, &r.repSBuf)
	rot := n.repRot.scalarAt(f, 0)
	eo := clamp01(n.repEO.scalarAt(f, 100) / 100)
	minv, ok := mat.invert()
	if !ok {
		return
	}

	// Snapshot the affected geometry and commands into reusable arenas.
	nRep := 0
	for i := groupStart; i < r.nGeoms; i++ {
		if nRep == len(r.repGeoms) {
			r.repGeoms = append(r.repGeoms, geometry{})
		}
		copyGeomInto(&r.repGeoms[nRep], &r.geoms[i])
		nRep++
	}
	snapGeoms := r.repGeoms[:nRep]
	r.repCmds = append(r.repCmds[:0], r.cmds[cmdStart:]...)
	r.nGeoms = groupStart
	r.cmds = r.cmds[:cmdStart]

	sx, sy := 1.0, 1.0
	if len(s) > 0 {
		sx = s[0] / 100
		sy = sx
	}
	if len(s) > 1 {
		sy = s[1] / 100
	}
	for ki := 0; ki < copies; ki++ {
		// Composite mode 2 stacks copies in reverse: the original draws
		// last, on top. The opacity ramp stays keyed by the copy index.
		k := ki
		if n.repMode == 2 {
			k = copies - 1 - ki
		}
		e := float64(k) + offset
		T := repeaterMatrix(at(a, 0), at(a, 1), at(p, 0), at(p, 1), rot, sx, sy, e)
		alpha := so
		if copies > 1 {
			alpha = so + (eo-so)*float64(k)/float64(copies-1)
		}
		geomBase := r.nGeoms
		for i := range snapGeoms {
			src := &snapGeoms[i]
			slot := r.nextGeom()
			copyGeomInto(slot, src)
			// The repeater transform acts in the group's own space.
			slot.mat = mat.mul(T).mul(minv.mul(src.mat))
			slot.alpha = src.alpha * alpha
		}
		for _, c := range r.repCmds {
			if c.dashed {
				// Dashed commands own arena geometry; duplicate it.
				dashBase := r.nDash
				for i := c.geomStart; i < c.geomEnd; i++ {
					src := r.dashGeoms[i]
					slot := r.nextDash()
					copyGeomInto(slot, &src)
					slot.mat = mat.mul(T).mul(minv.mul(src.mat))
				}
				c.geomStart, c.geomEnd = dashBase, r.nDash
			} else {
				c.geomStart = geomBase + (c.geomStart - groupStart)
				c.geomEnd = geomBase + (c.geomEnd - groupStart)
			}
			c.alphaMul *= alpha
			r.cmds = append(r.cmds, c)
		}
	}
}

// repeaterMatrix computes the repeater transform raised to exponent e using
// component-wise exponentiation (exact for the transforms editors produce).
func repeaterMatrix(ax, ay, px, py, rotDeg, sx, sy, e float64) matrix {
	m := identityMatrix.translate(ax+px*e, ay+py*e)
	if rotDeg != 0 {
		m = m.rotate(rotDeg * e * math.Pi / 180)
	}
	if sx != 1 || sy != 1 {
		m = m.scale(powScale(sx, e), powScale(sy, e))
	}
	return m.translate(-ax, -ay)
}

// powScale is math.Pow that stays finite for a mirror (negative) scale
// raised to a fractional exponent: the magnitude interpolates and the sign
// alternates per whole copy, instead of the NaN Pow would return.
func powScale(s, e float64) float64 {
	if s >= 0 {
		return math.Pow(s, e)
	}
	p := math.Pow(-s, e)
	if int(math.Floor(e))%2 != 0 {
		return -p
	}
	return p
}

// maxDashSegments bounds the number of dash segments emitted per contour.
const maxDashSegments = 1024

// buildDashedRange converts the geometries in [start,end) into dashed open
// contours in the dash arena, returning the arena range.
func (r *renderer) buildDashedRange(n *shapeNode, f float64, start, end int) (int, int) {
	pat := r.dashVals[:0]
	for _, tr := range n.dashPattern {
		v := tr.scalarAt(f, 0)
		if v < 0 {
			v = 0
		}
		pat = append(pat, v)
	}
	r.dashVals = pat
	if len(pat)%2 == 1 {
		// A lone dash value implies an equal gap.
		pat = append(pat, pat[len(pat)-1])
	}
	patLen := 0.0
	for _, v := range pat {
		patLen += v
	}
	dashStart := r.nDash
	offset := 0.0
	if n.dashOffset != nil {
		offset = n.dashOffset.scalarAt(f, 0)
	}
	for gi := start; gi < end; gi++ {
		g := &r.geoms[gi]
		t := &r.trim
		total := t.contourLength(&g.bez)
		if total <= 0 {
			continue
		}
		if patLen <= 0 {
			slot := r.nextDash()
			copyGeomInto(slot, g)
			continue
		}
		phase := math.Mod(offset, patLen)
		if phase < 0 {
			phase += patLen
		}
		cur := -phase
		idx := 0
		contourStart := r.nDash
		firstClipped, lastClipped := false, false
		for cur < total && idx < maxDashSegments {
			seg := pat[idx%len(pat)]
			on := idx%2 == 0
			if on && cur+seg > 0 && cur < total {
				f0 := math.Max(cur, 0) / total
				f1 := math.Min(cur+seg, total) / total
				if f1 > f0 {
					t.out = t.out[:0]
					t.extract(g, f0, f1)
					for i := range t.out {
						slot := r.nextDash()
						copyGeomInto(slot, &t.out[i])
					}
					if cur < 0 {
						firstClipped = true
					}
					lastClipped = cur+seg > total
				}
			}
			cur += seg
			idx++
		}
		// When the seam of a closed contour falls inside an "on" dash —
		// the first piece was clipped at fraction 0 and the last at 1 —
		// the two pieces are one dash running through the seam; join them
		// so caps and joins render as one stroke, the way trims do.
		if g.bez.Closed && firstClipped && lastClipped && r.nDash-contourStart >= 2 {
			r.joinDashSeam(contourStart)
		}
	}
	return dashStart, r.nDash
}

// joinDashSeam merges the last dash piece of a contour (ending at the
// seam) with the first (starting there): the seam vertex keeps the last
// piece's in-tangent and the first piece's out-tangent.
func (r *renderer) joinDashSeam(contourStart int) {
	first := &r.dashGeoms[contourStart]
	last := &r.dashGeoms[r.nDash-1]
	if len(first.bez.V) == 0 || len(last.bez.V) == 0 {
		return
	}
	t := &r.trim
	t.out = t.out[:0]
	mg := t.nextOut(first.mat)
	mg.alpha = first.alpha
	mg.xor = first.xor
	mg.bez.V = append(mg.bez.V, last.bez.V...)
	mg.bez.I = append(mg.bez.I, last.bez.I...)
	mg.bez.O = append(mg.bez.O, last.bez.O...)
	mg.bez.O[len(mg.bez.O)-1] = first.bez.O[0]
	mg.bez.V = append(mg.bez.V, first.bez.V[1:]...)
	mg.bez.I = append(mg.bez.I, first.bez.I[1:]...)
	mg.bez.O = append(mg.bez.O, first.bez.O[1:]...)
	copyGeomInto(first, mg)
	r.nDash--
}
