package main

// Minimal Lottie document builders, the same shape as the opening
// sample's. The clips are generated rather than downloaded so their
// licensing is unambiguous: everything here is authored in this
// repository.

type obj = map[string]any

// doc assembles a Lottie document.
func doc(name string, w, h int, frames float64, layers []obj, markers []obj) obj {
	d := obj{
		"v": "5.9.0", "nm": name, "fr": 60,
		"ip": 0, "op": frames, "w": w, "h": h,
		"layers": layers,
	}
	if len(markers) > 0 {
		d["markers"] = markers
	}
	return d
}

func marker(name string, start, dur float64) obj {
	return obj{"tm": start, "cm": name, "dr": dur}
}

// static is a constant property.
func static(v any) obj { return obj{"a": 0, "k": v} }

// key is one keyframe. Omitting the end value is the modern form: the next
// keyframe's start is the end.
func key(t float64, v []float64, ease bool) obj {
	k := obj{"t": t, "s": v}
	if ease {
		k["i"] = obj{"x": []float64{0.4}, "y": []float64{1}}
		k["o"] = obj{"x": []float64{0.6}, "y": []float64{0}}
	} else {
		k["i"] = obj{"x": []float64{0.5}, "y": []float64{1}}
		k["o"] = obj{"x": []float64{0.5}, "y": []float64{0}}
	}
	return k
}

// anim is an animated property built from keyframes.
func anim(keys ...obj) obj { return obj{"a": 1, "k": keys} }

// transform is a layer or group transform.
func transform(anchor, pos, scale, rot, opacity any) obj {
	return obj{"a": anchor, "p": pos, "s": scale, "r": rot, "o": opacity}
}

// shapeLayer is a shape layer holding shapes.
func shapeLayer(name string, ind int, frames float64, tr obj, shapes []any) obj {
	return obj{
		"ty": 4, "nm": name, "ind": ind,
		"ip": 0, "op": frames, "st": 0,
		"ks": tr, "shapes": shapes,
	}
}

// group wraps drawing primitives with an identity transform.
func group(name string, items ...any) obj {
	return groupTr(name, static([]float64{0, 0}), static([]float64{100, 100}),
		static(0.0), static(100.0), items...)
}

// groupTr is a group with its own (possibly animated) transform — how a
// pendulum swings or an eye widens without a layer of its own.
func groupTr(name string, pos, scale, rot, opacity any, items ...any) obj {
	items = append(items, obj{
		"ty": "tr", "a": static([]float64{0, 0}), "p": pos,
		"s": scale, "r": rot, "o": opacity,
	})
	return obj{"ty": "gr", "nm": name, "it": items}
}

func rect(w, h, radius float64) obj { return rectAt(0, 0, w, h, radius) }

// rectAt is a rectangle centered away from the group origin.
func rectAt(x, y, w, h, radius float64) obj {
	return obj{"ty": "rc", "p": static([]float64{x, y}),
		"s": static([]float64{w, h}), "r": static(radius)}
}

func ellipse(w, h float64) obj { return ellipseAt(0, 0, w, h) }

// ellipseAt is an ellipse centered away from the group origin.
func ellipseAt(x, y, w, h float64) obj {
	return obj{"ty": "el", "p": static([]float64{x, y}), "s": static([]float64{w, h})}
}

// polygon is a straight-edged closed path.
func polygon(pts ...[2]float64) obj {
	v := make([][]float64, len(pts))
	zero := make([][]float64, len(pts))
	for i, p := range pts {
		v[i] = []float64{p[0], p[1]}
		zero[i] = []float64{0, 0}
	}
	return obj{"ty": "sh", "ks": static(obj{"c": true, "v": v, "i": zero, "o": zero})}
}

// vx is one bezier vertex: a position with its in and out tangents,
// tangents relative to the vertex the way Lottie stores them.
type vx struct{ x, y, ix, iy, ox, oy float64 }

// curve is a free bezier path.
func curve(closed bool, pts ...vx) obj {
	v := make([][]float64, len(pts))
	in := make([][]float64, len(pts))
	out := make([][]float64, len(pts))
	for i, p := range pts {
		v[i] = []float64{p.x, p.y}
		in[i] = []float64{p.ix, p.iy}
		out[i] = []float64{p.ox, p.oy}
	}
	return obj{"ty": "sh", "ks": static(obj{"c": closed, "v": v, "i": in, "o": out})}
}

type rgb struct{ r, g, b float64 }

func fill(c rgb, a float64) obj {
	return obj{"ty": "fl", "c": static([]float64{c.r, c.g, c.b, 1}), "o": static(a * 100)}
}

func stroke(c rgb, a, width float64) obj {
	return obj{"ty": "st", "c": static([]float64{c.r, c.g, c.b, 1}), "o": static(a * 100),
		"w": static(width), "lc": 2, "lj": 2}
}

var (
	s100 = static([]float64{100, 100})
	p0   = static([]float64{0, 0})
	r0   = static(0.0)
	o100 = static(100.0)
)

// layerAt is shapeLayer with a layer transform built from parts.
func layerAt(name string, ind int, frames float64, pos, scale, rot, opacity any, shapes ...any) obj {
	return shapeLayer(name, ind, frames,
		transform(static([]float64{0, 0}), pos, scale, rot, opacity), shapes)
}

// still is layerAt for a layer that neither moves nor fades.
func still(name string, ind int, frames float64, x, y float64, shapes ...any) obj {
	return layerAt(name, ind, frames, static([]float64{x, y}), s100, r0, o100, shapes...)
}
