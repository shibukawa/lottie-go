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

func fill(r, g, b, a float64) obj {
	return obj{"ty": "fl", "c": static([]float64{r, g, b, 1}), "o": static(a * 100)}
}
