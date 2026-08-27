package lottie

import "math"

// signedContourArea approximates the shoelace area of a contour from points
// sampled along its curves. With Y down, a clockwise contour (Lottie's
// default direction) yields a positive value.
func signedContourArea(b *bezierShape) float64 {
	n := len(b.V)
	if n < 3 {
		return 0
	}
	segs := n - 1
	if b.Closed {
		segs = n
	}
	const samples = 4
	area := 0.0
	px, py := b.V[0][0], b.V[0][1]
	for s := 0; s < segs; s++ {
		p0, p1, p2, p3 := segmentPoints(b, s)
		for i := 1; i <= samples; i++ {
			x, y := cubicPoint(p0, p1, p2, p3, float64(i)/samples)
			area += px*y - x*py
			px, py = x, y
		}
	}
	// Close back to the start.
	x, y := b.V[0][0], b.V[0][1]
	area += px*y - x*py
	return area / 2
}

// reverseContour flips a contour's direction in place: vertices reverse and
// the in/out tangents swap roles.
func reverseContour(b *bezierShape) {
	n := len(b.V)
	for len(b.I) < n {
		b.I = append(b.I, [2]float64{})
	}
	for len(b.O) < n {
		b.O = append(b.O, [2]float64{})
	}
	for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
		b.V[i], b.V[j] = b.V[j], b.V[i]
		b.I[i], b.I[j] = b.I[j], b.I[i]
		b.O[i], b.O[j] = b.O[j], b.O[i]
	}
	for i := 0; i < n; i++ {
		b.I[i], b.O[i] = b.O[i], b.I[i]
	}
}

// applyMerge combines the contours collected so far in the group by winding
// rules: add makes every winding uniform so the non-zero rule fills the
// union, subtract reverses everything after the first contour, and
// exclude-intersections marks the contours so fills switch to the even-odd
// rule. True boolean intersection needs path clipping and stays unsupported.
func (r *renderer) applyMerge(n *shapeNode, groupStart int) {
	geoms := r.geoms[groupStart:r.nGeoms]
	switch n.mergeMode {
	case 2: // add
		for i := range geoms {
			orientContour(&geoms[i].bez, true)
		}
	case 3: // subtract: the first contour minus the rest
		for i := range geoms {
			orientContour(&geoms[i].bez, i == 0)
		}
	case 5: // exclude intersections
		for i := range geoms {
			geoms[i].xor = true
		}
	}
}

// orientContour winds a closed contour clockwise (positive area with Y down)
// or counter-clockwise.
func orientContour(b *bezierShape, clockwise bool) {
	if !b.Closed {
		return
	}
	if (signedContourArea(b) > 0) != clockwise {
		reverseContour(b)
	}
}

// applyOffsetPath grows or shrinks the contours collected so far in the
// group. Positive amounts expand the filled region regardless of the
// contour's winding direction.
func (r *renderer) applyOffsetPath(n *shapeNode, f float64, groupStart int) {
	amount := n.amount.scalarAt(f, 0)
	if amount == 0 {
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
		offsetContour(&out.bez, &g.bez, amount)
		copyGeomInto(g, out)
	}
}

// offsetMiterLimit caps how far a sharp corner's miter point may travel, in
// multiples of the offset amount, approximating a bevel past it.
const offsetMiterLimit = 4.0

// offsetContour writes an offset copy of src into dst by sampling the curve
// densely and displacing each sample along its corner-bisector normal with
// miter scaling — sharp corners meet at their miter point on both the
// growing and the shrinking side, After Effects' default join. The result
// uses smooth Catmull-Rom style tangents.
func offsetContour(dst, src *bezierShape, amount float64) {
	n := len(src.V)
	dst.Closed = src.Closed
	if n < 2 {
		dst.V = append(dst.V, src.V...)
		dst.I = append(dst.I, src.I...)
		dst.O = append(dst.O, src.O...)
		return
	}
	// Positive amounts must expand the fill whatever the winding, and the
	// sampling normal points outward only for clockwise contours.
	if src.Closed && signedContourArea(src) < 0 {
		amount = -amount
	}
	segs := n - 1
	if src.Closed {
		segs = n
	}

	var pts [][2]float64
	for s := 0; s < segs; s++ {
		length := segmentLength(src, s)
		steps := int(length/6) + 1
		if steps > 48 {
			steps = 48
		}
		p0, p1, p2, p3 := segmentPoints(src, s)
		for i := 0; i < steps; i++ {
			x, y := cubicPoint(p0, p1, p2, p3, float64(i)/float64(steps))
			pts = append(pts, [2]float64{x, y})
		}
	}
	if !src.Closed {
		p0, p1, p2, p3 := segmentPoints(src, segs-1)
		x, y := cubicPoint(p0, p1, p2, p3, 1)
		pts = append(pts, [2]float64{x, y})
	}

	dir := func(a, b [2]float64) (float64, float64, bool) {
		dx, dy := b[0]-a[0], b[1]-a[1]
		l := math.Hypot(dx, dy)
		if l < 1e-9 {
			return 0, 0, false
		}
		return dx / l, dy / l, true
	}
	m := len(pts)
	for i := 0; i < m; i++ {
		prev, next := i-1, i+1
		if src.Closed {
			prev, next = (i-1+m)%m, (i+1)%m
		} else {
			prev, next = max(prev, 0), min(next, m-1)
		}
		d1x, d1y, ok1 := dir(pts[prev], pts[i])
		d2x, d2y, ok2 := dir(pts[i], pts[next])
		if !ok1 {
			d1x, d1y, ok1 = d2x, d2y, ok2
		}
		if !ok2 {
			d2x, d2y = d1x, d1y
		}
		if !ok1 {
			dst.V = append(dst.V, pts[i])
			continue
		}
		// Normals of the incoming and outgoing directions; their bisector
		// scaled by 1/cos(half-angle) lands on the two offset lines'
		// intersection.
		n1x, n1y := d1y, -d1x
		n2x, n2y := d2y, -d2x
		bx, by := n1x+n2x, n1y+n2y
		bl := math.Hypot(bx, by)
		if bl < 1e-9 {
			// A hairpin reversal has no bisector; fall back to one side.
			bx, by, bl = n1x, n1y, 1
		}
		bx /= bl
		by /= bl
		den := bx*n1x + by*n1y
		if den < 1/offsetMiterLimit {
			den = 1 / offsetMiterLimit
		}
		scale := amount / den
		dst.V = append(dst.V, [2]float64{pts[i][0] + bx*scale, pts[i][1] + by*scale})
	}

	// Smooth tangents keep the offset curve from looking faceted.
	m = len(dst.V)
	for i := 0; i < m; i++ {
		prev, next := i-1, i+1
		if src.Closed {
			prev, next = (i-1+m)%m, (i+1)%m
		} else {
			prev, next = max(prev, 0), min(next, m-1)
		}
		tx := (dst.V[next][0] - dst.V[prev][0]) / 6
		ty := (dst.V[next][1] - dst.V[prev][1]) / 6
		dst.I = append(dst.I, [2]float64{-tx, -ty})
		dst.O = append(dst.O, [2]float64{tx, ty})
	}
}
