package main

// The five decoration slots are added here rather than by hand, so the
// sample can be rebuilt from the preset with one command. Each is a
// layer parented to the part it hangs off; four of them are static and
// ride along on that parenting alone.
//
// The twin-tails are not static. Hair that holds one angle through a
// run reads as a helmet, so their rotation is simulated: the tail is a
// damped spring hanging from the head, pulled toward straight down in
// WORLD space and shoved by the head's own acceleration. Converting
// that world angle back into the head's frame is what produces the lag
// — the head turns, the hair keeps pointing where it was, then catches
// up and overshoots.

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
)

type slot struct {
	name      string
	file      string
	w, h      int
	anchor    [2]float64
	parent    string
	attach    [2]float64
	rot       float64
	scaleX    float64
	before    string  // drawn in front of this layer
	swaysWith float64 // non-zero: simulate, using this as the resting world angle
}

var slots = []slot{
	// Tails hang off the head, so they swing and mirror with it. The far
	// one stays behind the head; the near one is drawn in FRONT, so the
	// trailing lock falls over the cheek and shoulder the way long hair
	// does instead of peeking out at the silhouette's edge.
	{"tail-far", "knight-nun-tail.png", 24, 57, [2]float64{12, 4}, "head", [2]float64{70, 24}, 8, 100, "shin-near", 8},
	{"tail-near", "knight-nun-tail.png", 24, 57, [2]float64{12, 4}, "head", [2]float64{2, 24}, -8, -100, "head", -8},
	// The skirt hangs from the hip in front of both legs, so the thighs
	// swing under it instead of through it.
	{"skirt", "knight-nun-skirt.png", 54, 27, [2]float64{27, 3}, "body", [2]float64{24, 40}, 0, 100, "shin-near", 0},
	// The scabbard rides the trailing hip, behind the torso.
	{"scabbard", "knight-nun-scabbard.png", 12, 51, [2]float64{6, 4}, "body", [2]float64{8, 30}, 14, 100, "thigh-far", 0},
	// The veil's tail hangs behind everything but the shadow.
	{"cape", "knight-nun-cape.png", 30, 66, [2]float64{15, 4}, "body", [2]float64{24, 16}, 0, 100, "shadow", 0},
}

// Spring constants, in units of 1/s^2 and 1/s. Stiff enough to follow a
// run, loose enough to keep swinging for a beat after the head stops.
const (
	stiffness = 150.0
	damping   = 10.0
	// How hard the pivot's own acceleration shoves the hair sideways.
	shove = 0.35
	// And how hard its vertical acceleration splays the tails outward.
	// A rigid pendulum would ignore this — acceleration along its own
	// axis cannot rotate it — but hair is not rigid, and without it a
	// gait that only bobs up and down leaves the hair dead still.
	bounce = 0.95
	// Hair swings, it does not spin. A hard landing or a lunge can put
	// enough into the spring to carry the tail past anything a head of
	// hair would do, so the swing is capped either side of rest.
	swingLimit = 38.0
	// Frames between baked keys. Two is smooth at 60fps and keeps the
	// keyframe count off the bundle's size.
	stride = 2
)

type obj = map[string]any

func num(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	}
	return 0
}

func nums(v any) []float64 {
	switch k := v.(type) {
	case []any:
		out := make([]float64, len(k))
		for i, x := range k {
			out[i] = num(x)
		}
		return out
	case float64:
		return []float64{k}
	}
	return nil
}

// prop reads a Lottie property at a frame, interpolating linearly. The
// sway only needs a smooth driving signal, so the real easing curves do
// not have to be reproduced here.
func prop(p obj, t float64) []float64 {
	if num(p["a"]) == 0 {
		return nums(p["k"])
	}
	keys, _ := p["k"].([]any)
	if len(keys) == 0 {
		return nil
	}
	first, _ := keys[0].(obj)
	if t <= num(first["t"]) {
		return nums(first["s"])
	}
	for i := 1; i < len(keys); i++ {
		a, _ := keys[i-1].(obj)
		b, _ := keys[i].(obj)
		ta, tb := num(a["t"]), num(b["t"])
		if t > tb {
			continue
		}
		va, vb := nums(a["s"]), nums(b["s"])
		if num(a["h"]) == 1 || tb == ta {
			return va
		}
		f := (t - ta) / (tb - ta)
		out := make([]float64, len(va))
		for j := range va {
			out[j] = va[j] + (vb[j]-va[j])*f
		}
		return out
	}
	last, _ := keys[len(keys)-1].(obj)
	return nums(last["s"])
}

// mat is a 2x3 affine transform, matching the renderer's convention.
type mat struct{ a, b, c, d, tx, ty float64 }

func mul(m, n mat) mat {
	return mat{
		a: m.a*n.a + m.c*n.b, b: m.b*n.a + m.d*n.b,
		c: m.a*n.c + m.c*n.d, d: m.b*n.c + m.d*n.d,
		tx: m.a*n.tx + m.c*n.ty + m.tx, ty: m.b*n.tx + m.d*n.ty + m.ty,
	}
}

func node(pos, anchor []float64, deg, sx, sy float64) mat {
	s, c := math.Sincos(deg * math.Pi / 180)
	r := mat{c, s, -s, c, 0, 0}
	sc := mat{sx / 100, 0, 0, sy / 100, 0, 0}
	m := mul(mat{1, 0, 0, 1, pos[0], pos[1]}, r)
	m = mul(m, sc)
	return mul(m, mat{1, 0, 0, 1, -anchor[0], -anchor[1]})
}

func apply(m mat, x, y float64) (float64, float64) {
	return m.a*x + m.c*y + m.tx, m.b*x + m.d*y + m.ty
}

// worldOf composes a layer's transform chain at a frame.
func worldOf(byName map[string]obj, byInd map[float64]obj, layer obj, t float64) mat {
	ks, _ := layer["ks"].(obj)
	a, _ := ks["a"].(obj)
	p, _ := ks["p"].(obj)
	sPr, _ := ks["s"].(obj)
	r, _ := ks["r"].(obj)
	scale := prop(sPr, t)
	m := node(prop(p, t), prop(a, t), prop(r, t)[0], scale[0], scale[1])
	if parent, ok := layer["parent"]; ok {
		if pl, ok := byInd[num(parent)]; ok {
			return mul(worldOf(byName, byInd, pl, t), m)
		}
	}
	return m
}

// hang returns the world angle of a transform's local "down" axis, and
// whether the transform mirrors. A mirrored parent reverses the sense of
// a child's rotation, which the caller has to undo.
func hang(m mat) (float64, float64) {
	x0, y0 := apply(m, 0, 0)
	x1, y1 := apply(m, 0, 1)
	dx, dy := x1-x0, y1-y0
	deg := math.Atan2(-dx, dy) * 180 / math.Pi
	sign := 1.0
	if m.a*m.d-m.b*m.c < 0 {
		sign = -1
	}
	return deg, sign
}

// sway simulates one tail over the clip and returns its rotation, in the
// head's frame, at every frame. Loops are run three times over so the
// swing that comes back is the one already settled into the cycle.
func sway(byName map[string]obj, byInd map[float64]obj, s slot, frames float64, loop bool) []float64 {
	head := byName[s.parent]
	const dt = 1.0 / 60
	n := int(frames)
	pivot := func(f float64) (float64, float64) {
		return apply(worldOf(byName, byInd, head, math.Mod(f, frames)), s.attach[0], s.attach[1])
	}
	psi, vel := s.swaysWith, 0.0
	passes := 1
	if loop {
		passes = 3
	}
	out := make([]float64, n+1)
	for pass := range passes {
		for f := 0; f <= n; f++ {
			ft := float64(f)
			m := worldOf(byName, byInd, head, ft)
			wp, sign := hang(m)
			// The pivot's own acceleration shoves the hair the other way.
			x0, y0 := pivot(ft - 1)
			x1, y1 := pivot(ft)
			x2, y2 := pivot(ft + 1)
			ax := (x2 - 2*x1 + x0) / (dt * dt)
			ay := (y2 - 2*y1 + y0) / (dt * dt)
			outward := math.Copysign(1, s.swaysWith)
			acc := -stiffness*(psi-s.swaysWith) - damping*vel -
				shove*ax + outward*bounce*ay
			vel += acc * dt
			psi += vel * dt
			if lo, hi := s.swaysWith-swingLimit, s.swaysWith+swingLimit; psi < lo {
				psi, vel = lo, 0
			} else if psi > hi {
				psi, vel = hi, 0
			}
			if pass == passes-1 {
				out[f] = sign * (psi - wp)
			}
		}
	}
	return out
}

// bake turns the per-frame angles into keyframes, dropping any that a
// straight line between its neighbours already passes through.
func bake(angles []float64, frames float64) obj {
	type key struct{ t, v float64 }
	var picked []key
	for f := 0; f < len(angles); f += stride {
		picked = append(picked, key{float64(f), angles[f]})
	}
	if last := float64(len(angles) - 1); picked[len(picked)-1].t != last {
		picked = append(picked, key{last, angles[len(angles)-1]})
	}
	kept := []key{picked[0]}
	for i := 1; i < len(picked)-1; i++ {
		prev, cur, next := kept[len(kept)-1], picked[i], picked[i+1]
		f := (cur.t - prev.t) / (next.t - prev.t)
		if math.Abs(prev.v+(next.v-prev.v)*f-cur.v) > 0.4 {
			kept = append(kept, cur)
		}
	}
	kept = append(kept, picked[len(picked)-1])
	if len(kept) == 2 && math.Abs(kept[0].v-kept[1].v) < 0.4 {
		return obj{"a": 0.0, "k": math.Round(kept[0].v*100) / 100}
	}
	out := make([]any, len(kept))
	for i, k := range kept {
		out[i] = obj{
			"t": k.t, "s": []any{math.Round(k.v*100) / 100},
			"i": obj{"x": []any{0.5}, "y": []any{1.0}},
			"o": obj{"x": []any{0.5}, "y": []any{0.0}},
		}
	}
	return obj{"a": 1.0, "k": out}
}

// loops are the clips that repeat, and so need their sway to come back
// to where it started.
var loops = map[string]bool{
	"idle-anim": true, "walk-anim": true, "run-anim": true,
	"fall-loop-anim": true, "guard-anim": true,
}

func addSlots(dir string) error {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no clips in %s: dump the bundle there first", dir)
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var doc obj
		if err := json.Unmarshal(raw, &doc); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		layers, _ := doc["layers"].([]any)
		byName := map[string]obj{}
		byInd := map[float64]obj{}
		maxInd := 0.0
		for _, l := range layers {
			lo, _ := l.(obj)
			byName[fmt.Sprint(lo["nm"])] = lo
			byInd[num(lo["ind"])] = lo
			maxInd = math.Max(maxInd, num(lo["ind"]))
		}
		if _, done := byName["cape"]; done {
			continue
		}
		assets, _ := doc["assets"].([]any)
		frames := num(doc["op"])
		name := fmt.Sprint(doc["nm"])
		for _, s := range slots {
			maxInd++
			assets = append(assets, obj{
				"id": s.name, "w": float64(s.w), "h": float64(s.h),
				"u": "i/", "p": s.file,
			})
			rot := any(obj{"a": 0.0, "k": s.rot})
			if s.swaysWith != 0 {
				rot = bake(sway(byName, byInd, s, frames, loops[name]), frames)
			}
			layer := obj{
				"ty": 2.0, "nm": s.name, "ind": maxInd,
				"parent": num(byName[s.parent]["ind"]),
				"ip":     0.0, "op": frames, "st": 0.0, "refId": s.name,
				"ks": obj{
					"a": obj{"a": 0.0, "k": []any{s.anchor[0], s.anchor[1]}},
					"p": obj{"a": 0.0, "k": []any{s.attach[0], s.attach[1]}},
					"s": obj{"a": 0.0, "k": []any{s.scaleX, 100.0}},
					"r": rot,
					"o": obj{"a": 0.0, "k": 100.0},
				},
			}
			at := len(layers)
			for i, l := range layers {
				lo, _ := l.(obj)
				if fmt.Sprint(lo["nm"]) == s.before {
					at = i
					break
				}
			}
			layers = append(layers, nil)
			copy(layers[at+1:], layers[at:])
			layers[at] = layer
		}
		doc["assets"] = assets
		doc["layers"] = layers
		out, err := json.MarshalIndent(doc, "", " ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, out, 0o644); err != nil {
			return err
		}
	}
	sort.Strings(paths)
	fmt.Printf("added %d slots to %d clips in %s\n", len(slots), len(paths), dir)
	return nil
}
