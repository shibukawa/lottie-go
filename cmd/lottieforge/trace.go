package main

import (
	"image"
	"math"
)

type pt = [2]float64

// traceOutline returns the outer boundary of a mask component as a closed
// polygon of pixel centers, clockwise on screen (y down), by Moore
// neighbour tracing from its top-left pixel.
func traceOutline(mask []bool, w, h int, c component) []pt {
	at := func(x, y int) bool {
		return x >= 0 && y >= 0 && x < w && y < h && mask[y*w+x]
	}
	// Restrict to this component so a nearby blob cannot hijack the walk.
	if len(c.pixels) > 0 {
		own := make([]bool, w*h)
		for _, i := range c.pixels {
			own[i] = true
		}
		at = func(x, y int) bool {
			return x >= 0 && y >= 0 && x < w && y < h && own[y*w+x]
		}
	}
	sx, sy := c.box.Min.X, c.box.Min.Y
	for ; sx < c.box.Max.X; sx++ {
		if at(sx, sy) {
			break
		}
	}
	if !at(sx, sy) {
		return nil
	}
	// Clockwise neighbour order starting from the west.
	dirs := [8][2]int{{-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}}
	var out []pt
	x, y := sx, sy
	back := 0 // direction we came from: west, since nothing is above-left
	for {
		out = append(out, pt{float64(x) + 0.5, float64(y) + 0.5})
		found := false
		// Start scanning one step clockwise past the backtrack direction.
		for k := 1; k <= 8; k++ {
			d := (back + k) % 8
			nx, ny := x+dirs[d][0], y+dirs[d][1]
			if at(nx, ny) {
				x, y = nx, ny
				// The new backtrack is the direction pointing to the cell we
				// scanned just before the hit.
				back = (d + 4 + 1) % 8
				found = true
				break
			}
		}
		if !found {
			return out // a single pixel
		}
		if x == sx && y == sy {
			break
		}
		if len(out) > 4*(w+h)*4 {
			break
		}
	}
	return out
}

func signedArea(p []pt) float64 {
	var a float64
	for i := range p {
		j := (i + 1) % len(p)
		a += p[i][0]*p[j][1] - p[j][0]*p[i][1]
	}
	return a / 2
}

func centroid(p []pt) pt {
	var c pt
	for _, v := range p {
		c[0] += v[0]
		c[1] += v[1]
	}
	n := float64(len(p))
	return pt{c[0] / n, c[1] / n}
}

func dist(a, b pt) float64 { return math.Hypot(a[0]-b[0], a[1]-b[1]) }

func cross(o, a, b pt) float64 {
	return (a[0]-o[0])*(b[1]-o[1]) - (a[1]-o[1])*(b[0]-o[0])
}

// simplify runs Douglas-Peucker over a closed polygon.
func simplify(p []pt, eps float64) []pt {
	if len(p) < 4 {
		return p
	}
	// Split at the vertex farthest from the first so both chains are open.
	far := 0
	for i := range p {
		if dist(p[0], p[i]) > dist(p[0], p[far]) {
			far = i
		}
	}
	a := dp(p[:far+1], eps)
	b := dp(append(append([]pt{}, p[far:]...), p[0]), eps)
	out := append([]pt{}, a...)
	out = append(out, b[1:len(b)-1]...)
	return out
}

func dp(p []pt, eps float64) []pt {
	if len(p) <= 2 {
		return p
	}
	a, b := p[0], p[len(p)-1]
	best, bi := 0.0, -1
	for i := 1; i < len(p)-1; i++ {
		d := pointLineDist(p[i], a, b)
		if d > best {
			best, bi = d, i
		}
	}
	if best <= eps || bi < 0 {
		return []pt{a, b}
	}
	l := dp(p[:bi+1], eps)
	r := dp(p[bi:], eps)
	return append(l[:len(l)-1], r...)
}

func pointLineDist(p, a, b pt) float64 {
	l := dist(a, b)
	if l == 0 {
		return dist(p, a)
	}
	return math.Abs(cross(a, b, p)) / l
}

// simplifyTo simplifies until the polygon has at most budget vertices.
func simplifyTo(p []pt, budget int) []pt {
	if budget < 4 {
		budget = 4
	}
	eps := 0.6
	out := simplify(p, eps)
	for len(out) > budget && eps < 1e4 {
		eps *= 1.3
		out = simplify(p, eps)
	}
	return out
}

// dilate pushes every vertex outward by d.
func dilate(p []pt, d float64) []pt {
	n := len(p)
	if n < 3 {
		return p
	}
	sign := 1.0
	if signedArea(p) < 0 {
		sign = -1
	}
	out := make([]pt, n)
	for i := range p {
		prev, next := p[(i+n-1)%n], p[(i+1)%n]
		e1 := pt{p[i][0] - prev[0], p[i][1] - prev[1]}
		e2 := pt{next[0] - p[i][0], next[1] - p[i][1]}
		n1 := normal(e1, sign)
		n2 := normal(e2, sign)
		nx, ny := n1[0]+n2[0], n1[1]+n2[1]
		l := math.Hypot(nx, ny)
		if l < 1e-9 {
			nx, ny, l = n1[0], n1[1], 1
		}
		out[i] = pt{p[i][0] + nx/l*d, p[i][1] + ny/l*d}
	}
	return out
}

// normal is the outward unit normal of edge e for a polygon of the given
// orientation sign (positive area = clockwise on a y-down screen).
func normal(e pt, sign float64) pt {
	l := math.Hypot(e[0], e[1])
	if l < 1e-9 {
		return pt{}
	}
	// For positive signed area (clockwise on screen) the outward normal is
	// to the left of travel in screen coordinates: (ey, -ex).
	return pt{sign * e[1] / l, -sign * e[0] / l}
}

// starIndex reports the first edge that the centroid fan would fold
// over, or -1 when every triangle keeps the polygon's orientation.
func starIndex(p []pt) int {
	if len(p) < 3 {
		return -1
	}
	c := centroid(p)
	sign := signedArea(p)
	for i := range p {
		j := (i + 1) % len(p)
		if cross(c, p[i], p[j])*sign < 0 {
			return i
		}
	}
	return -1
}

func pointInPolygon(q pt, p []pt) bool {
	in := false
	n := len(p)
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		if (p[i][1] > q[1]) != (p[j][1] > q[1]) {
			x := (p[j][0]-p[i][0])*(q[1]-p[i][1])/(p[j][1]-p[i][1]) + p[i][0]
			if q[0] < x {
				in = !in
			}
		}
	}
	return in
}

func segmentsCross(a, b, c, d pt) bool {
	d1, d2 := cross(c, d, a), cross(c, d, b)
	d3, d4 := cross(a, b, c), cross(a, b, d)
	return ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) && ((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0))
}

// validDiagonal reports whether vertices i and j can be joined inside p.
func validDiagonal(p []pt, i, j int) bool {
	n := len(p)
	if i == j || (i+1)%n == j || (j+1)%n == i {
		return false
	}
	a, b := p[i], p[j]
	for k := range p {
		l := (k + 1) % n
		if k == i || k == j || l == i || l == j {
			continue
		}
		if segmentsCross(a, b, p[k], p[l]) {
			return false
		}
	}
	mid := pt{(a[0] + b[0]) / 2, (a[1] + b[1]) / 2}
	return pointInPolygon(mid, p)
}

// decompose splits a polygon into pieces the centroid fan can texture
// (concept:uv-morph-rig decomposition). Each piece keeps the original
// vertex coordinates, so shared vertices are found by position.
func decompose(p []pt) [][]pt {
	var out [][]pt
	var rec func(q []pt, depth int)
	rec = func(q []pt, depth int) {
		bad := starIndex(q)
		if bad < 0 || depth > 6 || len(q) < 5 {
			out = append(out, q)
			return
		}
		n := len(q)
		sign := signedArea(q)
		// Prefer the reflex endpoint of the offending edge.
		cands := []int{bad, (bad + 1) % n}
		for _, ci := range cands {
			prev, next := q[(ci+n-1)%n], q[(ci+1)%n]
			if cross(prev, q[ci], next)*sign < 0 {
				cands = []int{ci, cands[0] ^ cands[1] ^ ci}
				break
			}
		}
		bestI, bestJ, bestScore := -1, -1, math.Inf(1)
		for _, i := range cands {
			for j := range q {
				if !validDiagonal(q, i, j) {
					continue
				}
				// Short diagonals that split the count evenly win.
				gap := (j - i + n) % n
				if gap < 2 || n-gap < 2 {
					continue
				}
				score := dist(q[i], q[j]) * (1 + math.Abs(float64(gap)-float64(n)/2)/float64(n))
				if score < bestScore {
					bestI, bestJ, bestScore = i, j, score
				}
			}
			if bestI >= 0 {
				break
			}
		}
		if bestI < 0 {
			out = append(out, q)
			return
		}
		i, j := bestI, bestJ
		if i > j {
			i, j = j, i
		}
		a := append([]pt{}, q[i:j+1]...)
		b := append(append([]pt{}, q[j:]...), q[:i+1]...)
		rec(a, depth+1)
		rec(b, depth+1)
	}
	rec(p, 0)
	return out
}

// tangents builds Catmull-Rom handles (relative in/out) for a closed
// polygon; corner vertices get none.
func tangents(p []pt, corner []bool) (in, out []pt) {
	n := len(p)
	in, out = make([]pt, n), make([]pt, n)
	for i := range p {
		if corner != nil && corner[i] {
			continue
		}
		prev, next := p[(i+n-1)%n], p[(i+1)%n]
		tx, ty := (next[0]-prev[0])/6, (next[1]-prev[1])/6
		in[i] = pt{-tx, -ty}
		out[i] = pt{tx, ty}
	}
	return in, out
}

// outlineOf traces, simplifies and dilates the largest blob of an image.
func outlineOf(mask []bool, w, h int, budget int, grow float64) []pt {
	comps := components(mask, w, h)
	if len(comps) == 0 {
		return nil
	}
	raw := traceOutline(mask, w, h, comps[0])
	if len(raw) < 3 {
		// A sliver: its bounding box.
		b := comps[0].box
		return []pt{{float64(b.Min.X), float64(b.Min.Y)}, {float64(b.Max.X), float64(b.Min.Y)},
			{float64(b.Max.X), float64(b.Max.Y)}, {float64(b.Min.X), float64(b.Max.Y)}}
	}
	poly := simplifyTo(raw, budget)
	if len(poly) < 3 {
		return nil
	}
	return dilate(poly, grow)
}

// polygonBounds is the bounding box of a polygon.
func polygonBounds(p []pt) image.Rectangle {
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, v := range p {
		minX, minY = math.Min(minX, v[0]), math.Min(minY, v[1])
		maxX, maxY = math.Max(maxX, v[0]), math.Max(maxY, v[1])
	}
	return image.Rect(int(math.Floor(minX)), int(math.Floor(minY)), int(math.Ceil(maxX)), int(math.Ceil(maxY)))
}
