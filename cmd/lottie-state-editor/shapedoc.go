package main

import (
	"fmt"
	"math"
	"slices"
)

// Shape layers are edited through the same document as poses
// (decision:json-level-animation-edit): the tree is rewritten, stored back
// with SetAnimation, and the stage is redrawn by the real renderer. What is
// different is the addressing — a shape item lives in a nested tree of
// groups, so it is named by the index path that reaches it rather than by a
// layer index alone.

// shapeNode describes one item of a shape layer's tree, depth-first in
// document order, which is also paint order: the first item draws on top.
type shapeNode struct {
	layer int   // index into d.layers
	path  []int // indices into the nested it arrays, rooted at "shapes"
	depth int
	ty    string // "gr", "sh", "fl", ... ; "" when the item has no ty
	name  string
}

// shapeItemLabel names an item kind the way the panel lists it.
var shapeItemLabels = map[string]string{
	"gr": "group", "sh": "path", "rc": "rect", "el": "ellipse", "sr": "star",
	"fl": "fill", "st": "stroke", "gf": "gradient fill", "gs": "gradient stroke",
	"tr": "transform", "tm": "trim", "rd": "round corners", "rp": "repeater",
	"mm": "merge", "op": "offset path", "pb": "pucker/bloat", "zz": "zig zag",
}

func shapeItemLabel(ty string) string {
	if l, ok := shapeItemLabels[ty]; ok {
		return l
	}
	if ty == "" {
		return "item"
	}
	return ty
}

// shapeGeometryKinds are the items a stage click can land on.
func isShapeGeometry(ty string) bool {
	switch ty {
	case "sh", "rc", "el", "sr":
		return true
	}
	return false
}

// shapeLayers lists the layers a shape tree can be read from.
func (d *clipDoc) shapeLayerIndices() []int {
	var out []int
	for i := range d.layers {
		if d.layers[i].ty == 4 {
			out = append(out, i)
		}
	}
	return out
}

// layerShapes returns a shape layer's top-level item array.
func (d *clipDoc) layerShapes(layer int) ([]any, bool) {
	l := d.layer(layer)
	if l == nil || l.ty != 4 {
		return nil, false
	}
	raw, _ := d.root["layers"].([]any)
	if l.index >= len(raw) {
		return nil, false
	}
	lm, ok := raw[l.index].(map[string]any)
	if !ok {
		return nil, false
	}
	arr, ok := lm["shapes"].([]any)
	return arr, ok
}

// shapeTree flattens a layer's items depth-first. Unknown items are listed
// too — they are preserved on save, and a tree that hides them would lie
// about the paint order.
func (d *clipDoc) shapeTree(layer int) []shapeNode {
	arr, ok := d.layerShapes(layer)
	if !ok {
		return nil
	}
	var out []shapeNode
	var walk func(items []any, prefix []int, depth int)
	walk = func(items []any, prefix []int, depth int) {
		for i, iv := range items {
			im, ok := iv.(map[string]any)
			if !ok {
				continue
			}
			ty, _ := im["ty"].(string)
			name, _ := im["nm"].(string)
			path := append(slices.Clone(prefix), i)
			out = append(out, shapeNode{layer: layer, path: path, depth: depth, ty: ty, name: name})
			if ty == "gr" {
				if it, ok := im["it"].([]any); ok {
					walk(it, path, depth+1)
				}
			}
		}
	}
	walk(arr, nil, 0)
	return out
}

// shapeItem resolves an item by its path.
func (d *clipDoc) shapeItem(layer int, path []int) (map[string]any, bool) {
	arr, ok := d.layerShapes(layer)
	if !ok || len(path) == 0 {
		return nil, false
	}
	var item map[string]any
	for step, idx := range path {
		if idx < 0 || idx >= len(arr) {
			return nil, false
		}
		item, ok = arr[idx].(map[string]any)
		if !ok {
			return nil, false
		}
		if step == len(path)-1 {
			return item, true
		}
		if ty, _ := item["ty"].(string); ty != "gr" {
			return nil, false
		}
		arr, ok = item["it"].([]any)
		if !ok {
			return nil, false
		}
	}
	return nil, false
}

// shapeParentItems returns the array an item lives in, so it can be moved
// or deleted within it.
func (d *clipDoc) shapeParentItems(layer int, path []int) ([]any, map[string]any, bool) {
	if len(path) == 0 {
		return nil, nil, false
	}
	if len(path) == 1 {
		arr, ok := d.layerShapes(layer)
		return arr, nil, ok
	}
	parent, ok := d.shapeItem(layer, path[:len(path)-1])
	if !ok {
		return nil, nil, false
	}
	arr, ok := parent["it"].([]any)
	return arr, parent, ok
}

// ---- generic property access on any {a, k} member ----

// These mirror the layer ks helpers in clipdoc.go, working on any owner map
// (a shape item) and member name, which is what shape properties need: the
// same {a, k} shape appears on fills, strokes, gradients and primitives.

func propValueAtObj(owner map[string]any, member string, frame float64) ([]float64, bool) {
	if owner == nil {
		return nil, false
	}
	keys, ok := propKeys(owner[member])
	if !ok {
		return nil, false
	}
	i, ok := keyIndexAt(keys, frame)
	if !ok {
		return nil, false
	}
	km, _ := keys[i].(map[string]any)
	return jsonNums(km["s"])
}

func propStaticObj(owner map[string]any, member string) ([]float64, bool) {
	if owner == nil {
		return nil, false
	}
	if _, animated := propKeys(owner[member]); animated {
		return nil, false
	}
	m, ok := propObj(owner[member])
	if !ok {
		return nil, false
	}
	return jsonNums(m["k"])
}

// propValueObj reads a member at a frame whether or not it animates.
func propValueObj(owner map[string]any, member string, frame float64) ([]float64, bool) {
	if v, ok := propValueAtObj(owner, member, frame); ok {
		return v, true
	}
	return propStaticObj(owner, member)
}

// propValueNearObj reads the last key at or before frame, or the static
// value — exact on keys, one step stale between them, like valueNear.
func propValueNearObj(owner map[string]any, member string, frame float64) ([]float64, bool) {
	if owner == nil {
		return nil, false
	}
	keys, animated := propKeys(owner[member])
	if !animated {
		return propStaticObj(owner, member)
	}
	best := -1
	for i, k := range keys {
		km, ok := k.(map[string]any)
		if !ok {
			continue
		}
		if t, ok := jsonNum(km["t"]); ok && t <= frame {
			best = i
		}
	}
	if best < 0 {
		best = 0
	}
	km, _ := keys[best].(map[string]any)
	return jsonNums(km["s"])
}

// propIsKeyedObj reports whether the member holds a key exactly at frame.
func propIsKeyedObj(owner map[string]any, member string, frame float64) bool {
	_, ok := propValueAtObj(owner, member, frame)
	return ok
}

// propAnimatedObj reports whether the member animates at all.
func propAnimatedObj(owner map[string]any, member string) bool {
	if owner == nil {
		return false
	}
	_, animated := propKeys(owner[member])
	return animated
}

// setPropObj writes a member at a frame. A static member is promoted to
// promoteTimes first when the caller supplies them (the frame must be one
// of them), holding its old value at every other time; with none, the
// static value is rewritten, which applies to the whole clip. An animated
// member with no key at frame refuses — writing between keys would break
// the one-time-set invariant the same way it would for a pose.
func (d *clipDoc) setPropObj(owner map[string]any, member string, frame float64, v []float64, promoteTimes []float64) bool {
	if owner == nil || len(v) == 0 {
		return false
	}
	if !propAnimatedObj(owner, member) {
		if !d.promoteObj(owner, member, promoteTimes) {
			m, ok := propObj(owner[member])
			if !ok {
				return false
			}
			old, _ := jsonNums(m["k"])
			if slices.Equal(old, v) {
				return false
			}
			m["k"] = numsLike(m["k"], v)
			return true
		}
	}
	keys, ok := propKeys(owner[member])
	if !ok {
		return false
	}
	i, ok := keyIndexAt(keys, frame)
	if !ok {
		return false
	}
	km, _ := keys[i].(map[string]any)
	old, _ := jsonNums(km["s"])
	if slices.Equal(old, v) {
		return false
	}
	km["s"] = numsLike(km["s"], v)
	return true
}

// promoteObj keys a static member at every given time holding its current
// value, exactly as promote does for a layer transform member.
func (d *clipDoc) promoteObj(owner map[string]any, member string, times []float64) bool {
	if owner == nil || len(times) == 0 {
		return false
	}
	m, ok := propObj(owner[member])
	if !ok {
		return false
	}
	if propAnimatedObj(owner, member) {
		return true
	}
	v, ok := jsonNums(m["k"])
	if !ok {
		return false
	}
	keys := make([]any, 0, len(times))
	for _, t := range times {
		keys = append(keys, newKey(t, v))
	}
	m["a"] = 1
	m["k"] = keys
	d.index()
	return true
}

// promotePathObj keys a static path at every given time holding its
// current shape, so an edit at one key can land there alone.
func (d *clipDoc) promotePathObj(item map[string]any, times []float64) bool {
	m, ok := pathOwner(item)
	if !ok || len(times) == 0 {
		return false
	}
	if _, animated := pathKeys(m); animated {
		return true
	}
	p, ok := parsePathObj(m["k"])
	if !ok {
		return false
	}
	keys := make([]any, 0, len(times))
	for _, t := range times {
		keys = append(keys, map[string]any{
			"t": t, "s": []any{encodePathObj(p)},
			"i": map[string]any{"x": []any{0.5}, "y": []any{1.0}},
			"o": map[string]any{"x": []any{0.5}, "y": []any{0.0}},
		})
	}
	m["a"] = 1
	m["k"] = keys
	d.index()
	return true
}

// ---- shape-key indexing ----

// shapePropMembers are the {a, k} members each item kind can animate. The
// path member of sh is included: its keys hold path objects rather than
// numbers, but their times index the same way.
var shapePropMembers = map[string][]string{
	"sh": {"ks"},
	"rc": {"p", "s", "r"},
	"el": {"p", "s"},
	"sr": {"p", "pt", "r", "ir", "is", "or", "os"},
	"fl": {"c", "o"},
	"st": {"c", "o", "w"},
	"gf": {"o", "s", "e", "h", "a"},
	"gs": {"o", "s", "e", "h", "a", "w"},
	"tr": {"p", "a", "s", "r", "o", "sk", "sa"},
	"tm": {"s", "e", "o"},
	"rd": {"r"},
	"rp": {"c", "o"},
	"op": {"a"},
	"pb": {"a"},
	"zz": {"s", "r"},
}

// eachShapeProp visits every animatable member of every item of a layer's
// shape tree. The gradient color ramp rides inside g, so it is visited as a
// nested owner.
func (d *clipDoc) eachShapeProp(layer int, fn func(owner map[string]any, member string)) {
	for _, n := range d.shapeTree(layer) {
		item, ok := d.shapeItem(layer, n.path)
		if !ok {
			continue
		}
		for _, member := range shapePropMembers[n.ty] {
			fn(item, member)
		}
		if n.ty == "gf" || n.ty == "gs" {
			if g, ok := item["g"].(map[string]any); ok {
				fn(g, "k")
			}
		}
	}
}

// The pose-column operations in clipdoc.go work member by member on layer
// transforms; these are the same moves for one {a, k} property value, so a
// shape layer's animated members can join the columns.

// moveKeyProp retimes the key at from.
func moveKeyProp(v any, from, to float64) {
	keys, ok := propKeys(v)
	if !ok {
		return
	}
	if i, ok := keyIndexAt(keys, from); ok {
		km, _ := keys[i].(map[string]any)
		km["t"] = to
	}
}

// insertKeyProp copies the key before frame into a new key at frame.
func insertKeyProp(v any, frame float64) bool {
	keys, ok := propKeys(v)
	if !ok {
		return false
	}
	src := 0
	for i, k := range keys {
		km, ok := k.(map[string]any)
		if !ok {
			continue
		}
		if t, ok := jsonNum(km["t"]); ok && t < frame {
			src = i
		}
	}
	srcKey, _ := keys[src].(map[string]any)
	nk := copyKey(srcKey, frame)
	if nk == nil {
		return false
	}
	at := len(keys)
	for i, k := range keys {
		km, _ := k.(map[string]any)
		if t, ok := jsonNum(km["t"]); ok && t > frame {
			at = i
			break
		}
	}
	m, _ := propObj(v)
	m["k"] = slices.Insert(keys, at, any(nk))
	return true
}

// deleteKeyProp removes the key at frame, keeping at least one.
func deleteKeyProp(v any, frame float64) bool {
	keys, ok := propKeys(v)
	if !ok || len(keys) <= 1 {
		return false
	}
	i, ok := keyIndexAt(keys, frame)
	if !ok {
		return false
	}
	m, _ := propObj(v)
	m["k"] = slices.Delete(keys, i, i+1)
	return true
}

// setEaseProp gives the key at frame a curve, skipping hold keys the way
// setPoseEase does.
func setEaseProp(v any, frame float64, eased bool) bool {
	keys, ok := propKeys(v)
	if !ok {
		return false
	}
	km, ok := keyAt(keys, frame)
	if !ok {
		return false
	}
	if h, ok := jsonNum(km["h"]); ok && h != 0 {
		return false
	}
	if eased {
		km["i"], km["o"] = easedIn(), easedOut()
	} else {
		km["i"], km["o"] = linearIn(), linearOut()
	}
	return true
}

// shapeKeyTimes collects the key times of every animated shape property of
// a layer, one sorted set per property.
func (d *clipDoc) shapeKeyTimes(layer int) [][]float64 {
	var sets [][]float64
	d.eachShapeProp(layer, func(owner map[string]any, member string) {
		keys, ok := propKeys(owner[member])
		if !ok {
			return
		}
		ts := make([]float64, 0, len(keys))
		for _, k := range keys {
			km, ok := k.(map[string]any)
			if !ok {
				continue
			}
			if t, ok := jsonNum(km["t"]); ok {
				ts = append(ts, t)
			}
		}
		if len(ts) > 0 {
			sets = append(sets, ts)
		}
	})
	return sets
}

// ---- path geometry ----

// pathData is one bezier path: absolute vertices with relative in/out
// tangents, exactly as Lottie stores them.
type pathData struct {
	closed  bool
	v, i, o [][2]float64
}

func parsePathObj(v any) (pathData, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return pathData{}, false
	}
	var p pathData
	if c, ok := m["c"].(bool); ok {
		p.closed = c
	} else if cn, ok := jsonNum(m["c"]); ok {
		p.closed = cn != 0
	}
	read := func(key string) ([][2]float64, bool) {
		arr, ok := m[key].([]any)
		if !ok {
			return nil, false
		}
		out := make([][2]float64, 0, len(arr))
		for _, e := range arr {
			xy, ok := jsonNums(e)
			if !ok || len(xy) < 2 {
				return nil, false
			}
			out = append(out, [2]float64{xy[0], xy[1]})
		}
		return out, true
	}
	var okV, okI, okO bool
	p.v, okV = read("v")
	p.i, okI = read("i")
	p.o, okO = read("o")
	if !okV || !okI || !okO || len(p.v) == 0 ||
		len(p.i) != len(p.v) || len(p.o) != len(p.v) {
		return pathData{}, false
	}
	return p, true
}

func encodePathObj(p pathData) map[string]any {
	enc := func(pts [][2]float64) []any {
		out := make([]any, len(pts))
		for i, pt := range pts {
			out[i] = []any{pt[0], pt[1]}
		}
		return out
	}
	return map[string]any{"c": p.closed, "v": enc(p.v), "i": enc(p.i), "o": enc(p.o)}
}

// pathOwner returns a sh item's ks property object.
func pathOwner(item map[string]any) (map[string]any, bool) {
	if ty, _ := item["ty"].(string); ty != "sh" {
		return nil, false
	}
	return propObj(item["ks"])
}

// pathKeys is the keyframe list of a path property object, or false when
// the path is static. Unlike a numeric member, the static form here is a
// map (the path object itself), so the array-of-maps test is the whole
// distinction.
func pathKeys(m map[string]any) ([]any, bool) {
	arr, ok := m["k"].([]any)
	if !ok || len(arr) == 0 {
		return nil, false
	}
	if _, ok := arr[0].(map[string]any); !ok {
		return nil, false
	}
	return arr, true
}

// pathAt reads a path at a frame: the key stored exactly there, the static
// value, or — for display between keys — the last key before the frame.
func pathAt(item map[string]any, frame float64, near bool) (pathData, bool) {
	m, ok := pathOwner(item)
	if !ok {
		return pathData{}, false
	}
	keys, animated := pathKeys(m)
	if !animated {
		return parsePathObj(m["k"])
	}
	idx, onKey := keyIndexAt(keys, frame)
	if !onKey {
		if !near {
			return pathData{}, false
		}
		idx = 0
		for i, k := range keys {
			km, ok := k.(map[string]any)
			if !ok {
				continue
			}
			if t, ok := jsonNum(km["t"]); ok && t <= frame {
				idx = i
			}
		}
	}
	km, _ := keys[idx].(map[string]any)
	s, _ := km["s"].([]any)
	if len(s) == 0 {
		return pathData{}, false
	}
	return parsePathObj(s[0])
}

// pathKeyed reports whether the sh item's path animates.
func pathAnimated(item map[string]any) bool {
	m, ok := pathOwner(item)
	if !ok {
		return false
	}
	_, animated := pathKeys(m)
	return animated
}

// setPathAt writes a whole path at a frame: into the static slot, or into
// the key stored exactly there.
func (d *clipDoc) setPathAt(item map[string]any, frame float64, p pathData) bool {
	m, ok := pathOwner(item)
	if !ok {
		return false
	}
	keys, animated := pathKeys(m)
	if !animated {
		m["k"] = encodePathObj(p)
		return true
	}
	i, ok := keyIndexAt(keys, frame)
	if !ok {
		return false
	}
	km, _ := keys[i].(map[string]any)
	km["s"] = []any{encodePathObj(p)}
	return true
}

// eachPathKey rewrites every stored path of a sh item — the static one, or
// every keyframe's. Topology edits go through here: Lottie interpolates a
// path vertex-wise, so every key must keep the same vertex count and the
// same closure or the clip stops decoding as one path.
func eachPathKey(item map[string]any, fn func(p pathData) (pathData, bool)) bool {
	m, ok := pathOwner(item)
	if !ok {
		return false
	}
	keys, animated := pathKeys(m)
	if !animated {
		p, ok := parsePathObj(m["k"])
		if !ok {
			return false
		}
		np, changed := fn(p)
		if !changed {
			return false
		}
		m["k"] = encodePathObj(np)
		return true
	}
	changed := false
	for _, k := range keys {
		km, ok := k.(map[string]any)
		if !ok {
			continue
		}
		s, _ := km["s"].([]any)
		if len(s) == 0 {
			continue
		}
		p, ok := parsePathObj(s[0])
		if !ok {
			continue
		}
		np, ch := fn(p)
		if !ch {
			continue
		}
		km["s"] = []any{encodePathObj(np)}
		changed = true
	}
	return changed
}

// insertPathVertex splits segment seg at parameter t, on every stored key,
// so the shape keeps looking the same at the moment of the split and every
// key keeps the same topology.
func insertPathVertex(item map[string]any, seg int, t float64) bool {
	t = min(max(t, 0.01), 0.99)
	return eachPathKey(item, func(p pathData) (pathData, bool) {
		n := len(p.v)
		segs := n - 1
		if p.closed {
			segs = n
		}
		if seg < 0 || seg >= segs {
			return p, false
		}
		j, k := seg, (seg+1)%n
		// The cubic between two vertices, in absolute points.
		p0 := p.v[j]
		p1 := [2]float64{p.v[j][0] + p.o[j][0], p.v[j][1] + p.o[j][1]}
		p2 := [2]float64{p.v[k][0] + p.i[k][0], p.v[k][1] + p.i[k][1]}
		p3 := p.v[k]
		q0 := lerp2(p0, p1, t)
		q1 := lerp2(p1, p2, t)
		q2 := lerp2(p2, p3, t)
		r0 := lerp2(q0, q1, t)
		r1 := lerp2(q1, q2, t)
		s := lerp2(r0, r1, t)
		// The new vertex always goes right after j, which keeps every index
		// before it stable — including j itself, and k when the split segment
		// is the closing one that wraps to vertex 0.
		at := j + 1
		np := pathData{closed: p.closed}
		np.v = slices.Insert(slices.Clone(p.v), at, s)
		np.i = slices.Insert(slices.Clone(p.i), at, [2]float64{r0[0] - s[0], r0[1] - s[1]})
		np.o = slices.Insert(slices.Clone(p.o), at, [2]float64{r1[0] - s[0], r1[1] - s[1]})
		np.o[j] = [2]float64{q0[0] - p0[0], q0[1] - p0[1]}
		kAfter := k
		if k >= at {
			kAfter = k + 1
		}
		np.i[kAfter] = [2]float64{q2[0] - p3[0], q2[1] - p3[1]}
		return np, true
	})
}

// deletePathVertex removes one vertex from every stored key. A path keeps
// at least two vertices — one vertex is a dot, not a path.
func deletePathVertex(item map[string]any, idx int) bool {
	return eachPathKey(item, func(p pathData) (pathData, bool) {
		if idx < 0 || idx >= len(p.v) || len(p.v) <= 2 {
			return p, false
		}
		np := pathData{closed: p.closed}
		np.v = slices.Delete(slices.Clone(p.v), idx, idx+1)
		np.i = slices.Delete(slices.Clone(p.i), idx, idx+1)
		np.o = slices.Delete(slices.Clone(p.o), idx, idx+1)
		return np, true
	})
}

// setPathClosed opens or closes the path on every stored key.
func setPathClosed(item map[string]any, closed bool) bool {
	return eachPathKey(item, func(p pathData) (pathData, bool) {
		if p.closed == closed {
			return p, false
		}
		p.closed = closed
		return p, true
	})
}

func lerp2(a, b [2]float64, t float64) [2]float64 {
	return [2]float64{a[0] + (b[0]-a[0])*t, a[1] + (b[1]-a[1])*t}
}

// ---- gradients ----

// gradStop is one color stop of a gradient ramp. Alpha stops are carried
// separately, appended to the same flat array after the color stops.
type gradStop struct {
	pos     float64
	r, g, b float64
}

type gradAlphaStop struct {
	pos, a float64
}

// gradientRamp reads a gf/gs item's stops at a frame. The flat array is
// p*4 color entries, then any number of (pos, alpha) pairs.
func gradientRamp(item map[string]any, frame float64) ([]gradStop, []gradAlphaStop, bool) {
	g, ok := item["g"].(map[string]any)
	if !ok {
		return nil, nil, false
	}
	count, _ := jsonNum(g["p"])
	flat, ok := propValueNearObj(g, "k", frame)
	if !ok {
		return nil, nil, false
	}
	n := int(count)
	if n <= 0 || len(flat) < n*4 {
		return nil, nil, false
	}
	stops := make([]gradStop, 0, n)
	for i := range n {
		stops = append(stops, gradStop{flat[i*4], flat[i*4+1], flat[i*4+2], flat[i*4+3]})
	}
	var alphas []gradAlphaStop
	for i := n * 4; i+1 < len(flat); i += 2 {
		alphas = append(alphas, gradAlphaStop{flat[i], flat[i+1]})
	}
	return stops, alphas, true
}

// setGradientRamp writes the stops back at a frame, keeping the alpha
// stops as they are. p follows the color stop count.
func (d *clipDoc) setGradientRamp(item map[string]any, frame float64, stops []gradStop, alphas []gradAlphaStop, promoteTimes []float64) bool {
	g, ok := item["g"].(map[string]any)
	if !ok || len(stops) < 2 {
		return false
	}
	flat := make([]float64, 0, len(stops)*4+len(alphas)*2)
	for _, s := range stops {
		flat = append(flat, round4(s.pos), round4(s.r), round4(s.g), round4(s.b))
	}
	for _, a := range alphas {
		flat = append(flat, round4(a.pos), round4(a.a))
	}
	if !d.setPropObj(g, "k", frame, flat, promoteTimes) {
		return false
	}
	g["p"] = float64(len(stops))
	return true
}

func round4(v float64) float64 { return math.Round(v*10000) / 10000 }

// ---- item and layer construction ----

// staticProp builds an {a: 0, k: v} member.
func staticProp(v any) map[string]any {
	return map[string]any{"a": 0, "k": v}
}

func staticVec(vals ...float64) map[string]any {
	arr := make([]any, len(vals))
	for i, v := range vals {
		arr[i] = v
	}
	return staticProp(arr)
}

// newShapeTransform is the identity transform every new group carries.
func newShapeTransform() map[string]any {
	return map[string]any{
		"ty": "tr",
		"p":  staticVec(0, 0),
		"a":  staticVec(0, 0),
		"s":  staticVec(100, 100),
		"r":  staticProp(0.0),
		"o":  staticProp(100.0),
	}
}

func newGroupItem(name string, items ...map[string]any) map[string]any {
	it := make([]any, 0, len(items)+1)
	for _, m := range items {
		it = append(it, m)
	}
	it = append(it, newShapeTransform())
	return map[string]any{"ty": "gr", "nm": name, "it": it}
}

func newFillItem(r, g, b float64) map[string]any {
	return map[string]any{"ty": "fl", "c": staticVec(r, g, b), "o": staticProp(100.0), "r": 1}
}

func newStrokeItem(r, g, b, w float64) map[string]any {
	return map[string]any{
		"ty": "st", "c": staticVec(r, g, b), "o": staticProp(100.0),
		"w": staticProp(w), "lc": 2, "lj": 2,
	}
}

func newGradientFillItem(radial bool) map[string]any {
	t := 1
	if radial {
		t = 2
	}
	return map[string]any{
		"ty": "gf", "o": staticProp(100.0), "r": 1, "t": t,
		"s": staticVec(-50, 0), "e": staticVec(50, 0),
		"g": map[string]any{
			"p": 2.0,
			"k": staticVec(0, 1, 1, 1, 1, 0, 0, 0),
		},
	}
}

func newPathItem(p pathData) map[string]any {
	return map[string]any{"ty": "sh", "ks": staticProp(encodePathObj(p))}
}

func newRectItem(cx, cy, w, h float64) map[string]any {
	return map[string]any{
		"ty": "rc", "p": staticVec(cx, cy), "s": staticVec(w, h), "r": staticProp(0.0),
	}
}

func newEllipseItem(cx, cy, w, h float64) map[string]any {
	return map[string]any{"ty": "el", "p": staticVec(cx, cy), "s": staticVec(w, h)}
}

func newStarItem(cx, cy, r float64) map[string]any {
	return map[string]any{
		"ty": "sr", "sy": 1, "p": staticVec(cx, cy), "pt": staticProp(5.0),
		"r":  staticProp(0.0),
		"ir": staticProp(round2(r * 0.5)), "is": staticProp(0.0),
		"or": staticProp(r), "os": staticProp(0.0),
	}
}

// addShapeLayer prepends a fresh shape layer, which puts it in front — a
// drawing just made is the one being looked at. It returns the new layer's
// index (always 0).
func (d *clipDoc) addShapeLayer(name string) (int, bool) {
	raw, _ := d.root["layers"].([]any)
	maxInd := 0.0
	for i := range d.layers {
		maxInd = max(maxInd, d.layers[i].ind)
	}
	lm := map[string]any{
		"ty": 4, "nm": name, "ind": maxInd + 1,
		"ip": d.inPoint(), "op": d.outPoint(), "st": 0.0, "sr": 1,
		"ks": map[string]any{
			"a": staticVec(0, 0, 0), "p": staticVec(0, 0, 0),
			"s": staticVec(100, 100, 100), "r": staticProp(0.0), "o": staticProp(100.0),
		},
		"shapes": []any{},
	}
	d.root["layers"] = slices.Insert(raw, 0, any(lm))
	d.index()
	return 0, true
}

// insertShapeItem places an item into a container: the layer's own shapes
// array when groupPath is empty, or a group's it array. at is the index it
// lands on, clamped; 0 is the front, which is on top.
func (d *clipDoc) insertShapeItem(layer int, groupPath []int, item map[string]any, at int) bool {
	if len(groupPath) == 0 {
		arr, ok := d.layerShapes(layer)
		if !ok {
			return false
		}
		l := d.layer(layer)
		raw, _ := d.root["layers"].([]any)
		lm, _ := raw[l.index].(map[string]any)
		lm["shapes"] = slices.Insert(arr, min(max(at, 0), len(arr)), any(item))
		return true
	}
	group, ok := d.shapeItem(layer, groupPath)
	if !ok {
		return false
	}
	if ty, _ := group["ty"].(string); ty != "gr" {
		return false
	}
	it, ok := group["it"].([]any)
	if !ok {
		return false
	}
	group["it"] = slices.Insert(it, min(max(at, 0), len(it)), any(item))
	return true
}

// deleteShapeItem removes the item at path.
func (d *clipDoc) deleteShapeItem(layer int, path []int) bool {
	arr, parent, ok := d.shapeParentItems(layer, path)
	if !ok {
		return false
	}
	idx := path[len(path)-1]
	if idx < 0 || idx >= len(arr) {
		return false
	}
	arr = slices.Delete(arr, idx, idx+1)
	if parent != nil {
		parent["it"] = arr
	} else {
		l := d.layer(layer)
		raw, _ := d.root["layers"].([]any)
		lm, _ := raw[l.index].(map[string]any)
		lm["shapes"] = arr
	}
	return true
}

// moveShapeItem shifts an item within its own container: delta -1 is one
// place toward the front (earlier in the array), +1 toward the back. Items
// never move between groups — that would silently restyle them, since fills
// and strokes apply within their group.
func (d *clipDoc) moveShapeItem(layer int, path []int, delta int) bool {
	arr, _, ok := d.shapeParentItems(layer, path)
	if !ok {
		return false
	}
	i := path[len(path)-1]
	j := i + delta
	if i < 0 || i >= len(arr) || j < 0 || j >= len(arr) {
		return false
	}
	arr[i], arr[j] = arr[j], arr[i]
	return true
}

// deleteLayer removes a whole layer. Shape editing adds layers, so it must
// also be able to take one back; a layer other layers ride on refuses, the
// same way re-parenting protects the chains.
func (d *clipDoc) deleteLayer(layer int) error {
	l := d.layer(layer)
	if l == nil {
		return fmt.Errorf("no such layer")
	}
	for i := range d.layers {
		if i != layer && d.layers[i].hasParent && d.layers[i].parent == l.ind {
			return fmt.Errorf("layer %q rides on it", d.layers[i].name)
		}
	}
	raw, _ := d.root["layers"].([]any)
	if l.index >= len(raw) {
		return fmt.Errorf("no such layer")
	}
	d.root["layers"] = slices.Delete(raw, l.index, l.index+1)
	d.index()
	return nil
}
