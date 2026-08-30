package main

// Minimal Lottie document builders, mirroring editor/gensamples. The clips
// here are raster cutout animations: image layers moved by transforms, no
// vector shapes and no expressions, so any Lottie player renders them.

type obj = map[string]any

// doc assembles a Lottie document whose image assets live in the bundle's
// i/ directory.
func doc(name string, frames float64, layers []obj) obj {
	assets := make([]any, 0, len(allParts))
	for _, p := range allParts {
		assets = append(assets, obj{
			"id": p.name, "w": p.w(), "h": p.h(),
			"u": "i/", "p": p.file(),
		})
	}
	return obj{
		"v": "5.9.0", "nm": name, "fr": 60,
		"ip": 0, "op": frames, "w": canvas, "h": canvas,
		"assets": assets, "layers": layers,
	}
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

// holdKey is a hold keyframe: the value switches instantly and stays
// until the next key. Turn clips use it to swap limb sides and mirror
// the head with no morphing in between.
func holdKey(t float64, v []float64) obj {
	return obj{"t": t, "s": v, "h": 1}
}

// anim is an animated property built from keyframes.
func anim(keys ...obj) obj { return obj{"a": 1, "k": keys} }

// transform is a layer transform.
func transform(anchor, pos, scale, rot, opacity any) obj {
	return obj{"a": anchor, "p": pos, "s": scale, "r": rot, "o": opacity}
}

// imgLayer is an image layer referencing a part asset by id.
func imgLayer(name string, ind, parent int, frames float64, refID string, tr obj) obj {
	l := obj{
		"ty": 2, "nm": name, "ind": ind,
		"ip": 0, "op": frames, "st": 0,
		"refId": refID, "ks": tr,
	}
	if parent > 0 {
		l["parent"] = parent
	}
	return l
}
