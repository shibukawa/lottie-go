package main

// Minimal Lottie document builders, in the style of cmd/lottie-state-editor/gensamples: the
// sample is generated rather than downloaded so its licensing is
// unambiguous. These extend the editor's set with what a cutscene needs —
// precomps, solids, parenting, bezier paths, stars and strokes.

type obj = map[string]any

type rgb struct{ r, g, b float64 }

// doc assembles a Lottie document.
func doc(name string, w, h int, frames float64, assets, layers, markers []obj) obj {
	d := obj{
		"v": "5.9.0", "nm": name, "fr": fps,
		"ip": 0, "op": frames, "w": w, "h": h,
		"layers": layers,
	}
	if len(assets) > 0 {
		d["assets"] = assets
	}
	if len(markers) > 0 {
		d["markers"] = markers
	}
	return d
}

func precompAsset(id string, layers []obj) obj {
	return obj{"id": id, "fr": fps, "layers": layers}
}

func marker(name string, start, dur float64) obj {
	return obj{"tm": start, "cm": name, "dr": dur}
}

// static is a constant property.
func static(v any) obj { return obj{"a": 0, "k": v} }

// anim is an animated property built from keyframes.
func anim(keys ...obj) obj { return obj{"a": 1, "k": keys} }

func keyTangents(t float64, v []float64, ox, oy, ix, iy float64) obj {
	return obj{
		"t": t, "s": v,
		"o": obj{"x": []float64{ox}, "y": []float64{oy}},
		"i": obj{"x": []float64{ix}, "y": []float64{iy}},
	}
}

// keyE eases in and out — the default for body motion.
func keyE(t float64, v ...float64) obj { return keyTangents(t, v, 0.4, 0, 0.6, 1) }

// keyL is linear — scrolls and spins.
func keyL(t float64, v ...float64) obj { return keyTangents(t, v, 0.333, 0.333, 0.667, 0.667) }

// keyA accelerates — falling things.
func keyA(t float64, v ...float64) obj { return keyTangents(t, v, 0.55, 0, 0.9, 0.85) }

// keyH holds until the next keyframe.
func keyH(t float64, v ...float64) obj { return obj{"t": t, "s": v, "h": 1} }

// osc alternates v0/v1 every half frames from `from` to past `to`, easing
// between extremes. A positive phase delays the first extreme; the value
// then starts mid-swing, which is what a moving limb looks like.
func osc(from, to, half, phase float64, v0, v1 []float64) obj {
	var keys []obj
	if phase > 0 {
		mid := make([]float64, len(v0))
		for i := range v0 {
			mid[i] = (v0[i] + v1[i]) / 2
		}
		keys = append(keys, keyE(from, mid...))
	}
	lo := true
	for t := from + phase; t <= to+half; t += half {
		v := v0
		if !lo {
			v = v1
		}
		keys = append(keys, keyE(t, v...))
		lo = !lo
	}
	return anim(keys...)
}

// transform is a layer or group transform.
func transform(anchor, pos, scale, rot, opacity any) obj {
	return obj{"a": anchor, "p": pos, "s": scale, "r": rot, "o": opacity}
}

func tr(ax, ay, px, py, s, rot float64) obj {
	return transform(static([]float64{ax, ay}), static([]float64{px, py}),
		static([]float64{s, s}), static(rot), static(100.0))
}

func baseLayer(ty int, name string, ind, parent int, ip, op float64, ks obj) obj {
	l := obj{
		"ty": ty, "nm": name, "ind": ind,
		"ip": ip, "op": op, "st": 0,
		"ks": ks,
	}
	if parent != 0 {
		l["parent"] = parent
	}
	return l
}

func shapeLayer(name string, ind, parent int, ip, op float64, ks obj, shapes []any) obj {
	l := baseLayer(4, name, ind, parent, ip, op, ks)
	l["shapes"] = shapes
	return l
}

func precompLayer(name string, ind int, ref string, w, h int, ip, op float64, ks obj) obj {
	l := baseLayer(0, name, ind, 0, ip, op, ks)
	l["refId"] = ref
	l["w"] = w
	l["h"] = h
	return l
}

func solidLayer(name string, ind int, hex string, w, h int, ip, op float64, ks obj) obj {
	l := baseLayer(1, name, ind, 0, ip, op, ks)
	l["sc"] = hex
	l["sw"] = w
	l["sh"] = h
	return l
}

// nullLayer is an invisible transform for children to parent to.
func nullLayer(name string, ind int, ip, op float64, ks obj) obj {
	return baseLayer(3, name, ind, 0, ip, op, ks)
}

// group wraps shapes and their styles with an identity transform.
func group(name string, items ...any) obj {
	items = append(items, obj{
		"ty": "tr", "a": static([]float64{0, 0}), "p": static([]float64{0, 0}),
		"s": static([]float64{100, 100}), "r": static(0.0), "o": static(100.0),
	})
	return obj{"ty": "gr", "nm": name, "it": items}
}

// groupAt places its contents by a group transform, so a part drawn around
// the origin can be positioned and tilted in one place.
func groupAt(name string, x, y, rot float64, items ...any) obj {
	items = append(items, obj{
		"ty": "tr", "a": static([]float64{0, 0}), "p": static([]float64{x, y}),
		"s": static([]float64{100, 100}), "r": static(rot), "o": static(100.0),
	})
	return obj{"ty": "gr", "nm": name, "it": items}
}

func rect(x, y, w, h, radius float64) obj {
	return obj{"ty": "rc", "p": static([]float64{x, y}),
		"s": static([]float64{w, h}), "r": static(radius)}
}

func ellipse(x, y, w, h float64) obj {
	return obj{"ty": "el", "p": static([]float64{x, y}), "s": static([]float64{w, h})}
}

// ellipseAnim animates the ellipse size — the iris wipe closes with it.
func ellipseAnim(x, y float64, size obj) obj {
	return obj{"ty": "el", "p": static([]float64{x, y}), "s": size}
}

func polystar(x, y float64, points float64, outer, inner, rot float64) obj {
	return obj{"ty": "sr", "sy": 1, "d": 1,
		"p":  static([]float64{x, y}),
		"pt": static(points), "r": static(rot),
		"or": static(outer), "os": static(0.0),
		"ir": static(inner), "is": static(0.0)}
}

// path is a bezier shape. v are vertices; in/out tangents are relative to
// their vertex, nil meaning all-zero (a polygon).
func path(closed bool, v, in, out [][]float64) obj {
	n := len(v)
	if in == nil {
		in = make([][]float64, n)
	}
	if out == nil {
		out = make([][]float64, n)
	}
	for i := range n {
		if in[i] == nil {
			in[i] = []float64{0, 0}
		}
		if out[i] == nil {
			out[i] = []float64{0, 0}
		}
	}
	return obj{"ty": "sh", "ks": static(obj{"v": v, "i": in, "o": out, "c": closed})}
}

func fill(c rgb, a float64) obj {
	return obj{"ty": "fl", "c": static([]float64{c.r, c.g, c.b, 1}), "o": static(a * 100)}
}

// fillEO fills even-odd, so a shape inside another cuts a hole.
func fillEO(c rgb) obj {
	f := fill(c, 1)
	f["r"] = 2
	return f
}

func stroke(c rgb, width float64) obj {
	return obj{"ty": "st", "c": static([]float64{c.r, c.g, c.b, 1}),
		"o": static(100.0), "w": static(width), "lc": 2, "lj": 2}
}
