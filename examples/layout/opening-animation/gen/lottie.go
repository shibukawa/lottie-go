package main

// Minimal Lottie document builders. The samples are generated rather than
// downloaded so their licensing is unambiguous: everything here is authored
// in this repository.

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

// shapeLayer is a shape layer holding one group.
func shapeLayer(name string, ind int, frames float64, tr obj, shapes []any) obj {
	return obj{
		"ty": 4, "nm": name, "ind": ind,
		"ip": 0, "op": frames, "st": 0,
		"ks": tr, "shapes": shapes,
	}
}

// group wraps drawing primitives with their own transform.
func group(name string, items ...any) obj {
	items = append(items, obj{
		"ty": "tr", "a": static([]float64{0, 0}), "p": static([]float64{0, 0}),
		"s": static([]float64{100, 100}), "r": static(0.0), "o": static(100.0),
	})
	return obj{"ty": "gr", "nm": name, "it": items}
}

func rect(w, h, radius float64) obj {
	return obj{"ty": "rc", "p": static([]float64{0, 0}),
		"s": static([]float64{w, h}), "r": static(radius)}
}

func ellipse(w, h float64) obj {
	return obj{"ty": "el", "p": static([]float64{0, 0}), "s": static([]float64{w, h})}
}

// ellipseAt places an ellipse away from the group origin, which is how
// the wave's foam scallops chain along an edge inside one layer.
func ellipseAt(x, y, w, h float64) obj {
	return obj{"ty": "el", "p": static([]float64{x, y}), "s": static([]float64{w, h})}
}

// polygon is a straight-edged closed path — Mount Fuji needs no curves.
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

// curveVal is one bezier contour value, Lottie's {c, v, i, o} encoding.
func curveVal(closed bool, pts ...vx) obj {
	v := make([][]float64, len(pts))
	in := make([][]float64, len(pts))
	out := make([][]float64, len(pts))
	for i, p := range pts {
		v[i] = []float64{p.x, p.y}
		in[i] = []float64{p.ix, p.iy}
		out[i] = []float64{p.ox, p.oy}
	}
	return obj{"c": closed, "v": v, "i": in, "o": out}
}

// curve is a free bezier path — the wave's silhouette is drawn, not
// assembled from primitives.
func curve(closed bool, pts ...vx) obj {
	return obj{"ty": "sh", "ks": static(curveVal(closed, pts...))}
}

// shapeKey is one keyframe of a morphing path.
func shapeKey(t float64, val obj) obj {
	return obj{"t": t, "s": []any{val},
		"i": obj{"x": []float64{0.4}, "y": []float64{1}},
		"o": obj{"x": []float64{0.6}, "y": []float64{0}}}
}

// morphPath is a path whose contour itself animates — how the wave's
// silhouette changes as it rises and collapses. Every keyframe value must
// hold the same number of vertices.
func morphPath(keys ...obj) obj {
	return obj{"ty": "sh", "ks": obj{"a": 1, "k": keys}}
}

func fill(r, g, b, a float64) obj {
	return obj{"ty": "fl", "c": static([]float64{r, g, b, 1}), "o": static(a * 100)}
}

func stroke(r, g, b, a, width float64) obj {
	return obj{"ty": "st", "c": static([]float64{r, g, b, 1}), "o": static(a * 100),
		"w": static(width)}
}

// layerAt is shapeLayer with an animated or static layer transform built
// from parts, a shorthand the opening clips lean on.
func layerAt(name string, ind int, frames float64, pos, scale, rot, opacity any, shapes ...any) obj {
	return shapeLayer(name, ind, frames,
		transform(static([]float64{0, 0}), pos, scale, rot, opacity), shapes)
}
