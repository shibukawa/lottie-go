package main

import (
	"encoding/json"
	"math"
)

// obj is a Lottie document node as encoding/json decodes it.
type obj = map[string]any

func static(v any) obj { return obj{"a": 0, "k": v} }

// keyLin is a keyframe with linear handles; baked tracks are dense, so
// easing between keys would only fight the generator.
func keyLin(t float64, v any) obj {
	return obj{"t": t, "s": v,
		"o": obj{"x": []float64{0.333}, "y": []float64{0.333}},
		"i": obj{"x": []float64{0.667}, "y": []float64{0.667}}}
}

func keyEnd(t float64, v any) obj { return obj{"t": t, "s": v} }

func keyed(frames []obj) obj { return obj{"a": 1, "k": frames} }

func round2(x float64) float64 { return math.Round(x*100) / 100 }

func vec2(p [2]float64) []float64 { return []float64{round2(p[0]), round2(p[1])} }

func vecs(pts [][2]float64) [][]float64 {
	out := make([][]float64, len(pts))
	for i, p := range pts {
		out[i] = vec2(p)
	}
	return out
}

// pathObj encodes one closed bezier path as Lottie stores it.
func pathObj(v, in, out [][2]float64, closed bool) obj {
	return obj{"c": closed, "v": vecs(v), "i": vecs(in), "o": vecs(out)}
}

// num reads any JSON number.
func num(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	}
	return 0
}

// nums reads a JSON array of numbers.
func nums(v any) []float64 {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]float64, len(arr))
	for i, x := range arr {
		out[i] = num(x)
	}
	return out
}

// propValue reads a property's value: the static k, or the first key's s.
func propValue(p any) []float64 {
	m, ok := p.(obj)
	if !ok {
		return nil
	}
	if num(m["a"]) == 0 {
		switch k := m["k"].(type) {
		case []any:
			return nums(k)
		default:
			return []float64{num(k)}
		}
	}
	keys, _ := m["k"].([]any)
	if len(keys) == 0 {
		return nil
	}
	first, _ := keys[0].(obj)
	return nums(first["s"])
}

func deepCopy(v any) any {
	raw, _ := json.Marshal(v)
	var out any
	json.Unmarshal(raw, &out)
	return out
}

func layerInd(l obj) int { return int(num(l["ind"])) }

func layersOf(clip obj) []obj {
	raw, _ := clip["layers"].([]any)
	out := make([]obj, 0, len(raw))
	for _, l := range raw {
		if m, ok := l.(obj); ok {
			out = append(out, m)
		}
	}
	return out
}

func setLayers(clip obj, layers []obj) {
	raw := make([]any, len(layers))
	for i, l := range layers {
		raw[i] = l
	}
	clip["layers"] = raw
}
