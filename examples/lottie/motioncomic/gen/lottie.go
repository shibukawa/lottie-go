package main

// Minimal Lottie builders, the raster cut of examples/lottie/cutscene/gen: image
// assets and layers, precomps, nulls and solids — no shape layers at all.
// The sample is generated in-repository so its licensing is unambiguous.

type obj = map[string]any

const (
	fps    = 30.0
	totalF = 300.0
)

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

func imageAsset(id string, w, h int, dataURI string) obj {
	return obj{"id": id, "w": w, "h": h, "u": "", "p": dataURI, "e": 1}
}

func precompAsset(id string, layers []obj) obj {
	return obj{"id": id, "fr": fps, "layers": layers}
}

func marker(name string, start, dur float64) obj {
	return obj{"tm": start, "cm": name, "dr": dur}
}

func static(v any) obj { return obj{"a": 0, "k": v} }

func anim(keys ...obj) obj { return obj{"a": 1, "k": keys} }

func keyTangents(t float64, v []float64, ox, oy, ix, iy float64) obj {
	return obj{
		"t": t, "s": v,
		"o": obj{"x": []float64{ox}, "y": []float64{oy}},
		"i": obj{"x": []float64{ix}, "y": []float64{iy}},
	}
}

// keyE eases in and out; keyL is linear; keyA accelerates like a fall;
// keyH holds until the next keyframe.
func keyE(t float64, v ...float64) obj { return keyTangents(t, v, 0.4, 0, 0.6, 1) }
func keyL(t float64, v ...float64) obj { return keyTangents(t, v, 0.333, 0.333, 0.667, 0.667) }
func keyA(t float64, v ...float64) obj { return keyTangents(t, v, 0.55, 0, 0.9, 0.85) }
func keyH(t float64, v ...float64) obj { return obj{"t": t, "s": v, "h": 1} }

// osc alternates v0/v1 every half frames on [from, to] — the idle bob that
// keeps a frozen sprite alive inside its panel.
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

func imageLayer(name string, ind, parent int, ref string, ip, op float64, ks obj) obj {
	l := baseLayer(2, name, ind, parent, ip, op, ks)
	l["refId"] = ref
	return l
}

func precompLayer(name string, ind, parent int, ref string, w, h int, ip, op float64, ks obj) obj {
	l := baseLayer(0, name, ind, parent, ip, op, ks)
	l["refId"] = ref
	l["w"] = w
	l["h"] = h
	return l
}

func nullLayer(name string, ind, parent int, ip, op float64, ks obj) obj {
	return baseLayer(3, name, ind, parent, ip, op, ks)
}

func solidLayer(name string, ind int, hex string, w, h int, ip, op float64, ks obj) obj {
	l := baseLayer(1, name, ind, 0, ip, op, ks)
	l["sc"] = hex
	l["sw"] = w
	l["sh"] = h
	return l
}
