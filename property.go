package lottie

import "math"

// easing is a per-keyframe cubic-bezier timing function with control points
// (outX, outY) and (inX, inY), like CSS cubic-bezier.
type easing struct {
	outX, outY, inX, inY float64
	linear               bool
}

var linearEasing = easing{linear: true}

// at maps linear progress u in [0,1] to eased progress.
func (e easing) at(u float64) float64 {
	if e.linear || u <= 0 || u >= 1 {
		return u
	}
	// Solve bezierX(t) = u for t with Newton iterations, then evaluate Y.
	t := u
	for i := 0; i < 8; i++ {
		x := cubicBezier1D(e.outX, e.inX, t) - u
		if math.Abs(x) < 1e-6 {
			break
		}
		dx := cubicBezier1DDeriv(e.outX, e.inX, t)
		if math.Abs(dx) < 1e-8 {
			break
		}
		t -= x / dx
		if t < 0 {
			t = 0
		} else if t > 1 {
			t = 1
		}
	}
	return cubicBezier1D(e.outY, e.inY, t)
}

// cubicBezier1D evaluates a bezier with P0=0, P3=1 and control values p1, p2.
func cubicBezier1D(p1, p2, t float64) float64 {
	mt := 1 - t
	return 3*mt*mt*t*p1 + 3*mt*t*t*p2 + t*t*t
}

func cubicBezier1DDeriv(p1, p2, t float64) float64 {
	mt := 1 - t
	return 3*mt*mt*p1 + 6*mt*t*(p2-p1) + 3*t*t*(1-p2)
}

// vectorKey is one keyframe of an n-dimensional value.
type vectorKey struct {
	t         float64
	value     []float64
	hold      bool
	ease      easing
	ti, to    []float64 // spatial bezier tangents (position properties)
	legacyEnd []float64 // explicit end value from the legacy keyframe format
}

// vectorTrack animates an n-dimensional value over layer-local frames.
type vectorTrack struct {
	static []float64
	keys   []vectorKey
}

func staticTrack(v ...float64) *vectorTrack { return &vectorTrack{static: v} }

func (tr *vectorTrack) isStatic() bool { return tr.keys == nil }

// at evaluates the track at frame f. dst is used as scratch when non-nil.
func (tr *vectorTrack) at(f float64, dst []float64) []float64 {
	if tr.keys == nil {
		return tr.static
	}
	keys := tr.keys
	if f <= keys[0].t {
		return keys[0].value
	}
	last := keys[len(keys)-1]
	if f >= last.t {
		return last.value
	}
	// Find segment: keys[i].t <= f < keys[i+1].t.
	i := 0
	for i < len(keys)-1 && keys[i+1].t <= f {
		i++
	}
	k0, k1 := keys[i], keys[i+1]
	if k0.hold {
		return k0.value
	}
	u := (f - k0.t) / (k1.t - k0.t)
	u = k0.ease.at(u)
	n := len(k0.value)
	if len(k1.value) < n {
		n = len(k1.value)
	}
	if cap(dst) < n {
		dst = make([]float64, n)
	}
	dst = dst[:n]
	if k0.to != nil && k0.ti != nil && n >= 2 {
		// Spatial bezier interpolation for position keyframes.
		for d := 0; d < n; d++ {
			p0 := k0.value[d]
			p3 := k1.value[d]
			var c1, c2 float64
			if d < len(k0.to) {
				c1 = p0 + k0.to[d]
			} else {
				c1 = p0
			}
			if d < len(k0.ti) {
				c2 = p3 + k0.ti[d]
			} else {
				c2 = p3
			}
			mt := 1 - u
			dst[d] = mt*mt*mt*p0 + 3*mt*mt*u*c1 + 3*mt*u*u*c2 + u*u*u*p3
		}
		return dst
	}
	for d := 0; d < n; d++ {
		dst[d] = k0.value[d] + (k1.value[d]-k0.value[d])*u
	}
	return dst
}

// scalarAt is a convenience for 1-dimensional tracks.
func (tr *vectorTrack) scalarAt(f float64, def float64) float64 {
	v := tr.at(f, nil)
	if len(v) == 0 {
		return def
	}
	return v[0]
}

// shapeKey is one keyframe of a bezier shape.
type shapeKey struct {
	t     float64
	value bezierShape
	hold  bool
	ease  easing
}

// shapeTrack animates a bezier shape.
type shapeTrack struct {
	static bezierShape
	keys   []shapeKey
}

func (tr *shapeTrack) isStatic() bool { return tr.keys == nil }

func (tr *shapeTrack) at(f float64, dst *bezierShape) bezierShape {
	if tr.keys == nil {
		return tr.static
	}
	keys := tr.keys
	if f <= keys[0].t {
		return keys[0].value
	}
	last := keys[len(keys)-1]
	if f >= last.t {
		return last.value
	}
	i := 0
	for i < len(keys)-1 && keys[i+1].t <= f {
		i++
	}
	k0, k1 := keys[i], keys[i+1]
	if k0.hold || len(k0.value.V) != len(k1.value.V) {
		return k0.value
	}
	u := (f - k0.t) / (k1.t - k0.t)
	u = k0.ease.at(u)
	return lerpShape(k0.value, k1.value, u, dst)
}

func lerpShape(a, b bezierShape, u float64, dst *bezierShape) bezierShape {
	n := len(a.V)
	if dst == nil {
		dst = &bezierShape{}
	}
	dst.Closed = a.Closed
	dst.V = lerpPoints(dst.V, a.V, b.V, u)
	dst.I = lerpPoints(dst.I, a.I, b.I, u)
	dst.O = lerpPoints(dst.O, a.O, b.O, u)
	_ = n
	return *dst
}

func lerpPoints(dst, a, b [][2]float64, u float64) [][2]float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if cap(dst) < n {
		dst = make([][2]float64, n)
	}
	dst = dst[:n]
	for i := 0; i < n; i++ {
		dst[i][0] = a[i][0] + (b[i][0]-a[i][0])*u
		dst[i][1] = a[i][1] + (b[i][1]-a[i][1])*u
	}
	return dst
}
