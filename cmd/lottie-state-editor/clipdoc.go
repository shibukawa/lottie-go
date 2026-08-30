package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
)

// A clip is edited as its own JSON document rather than through
// lottie.Animation. The compiled form is deliberately read-only and lossy —
// its tracks are unexported and members the package does not model are
// dropped — so the round trip is Bundle.AnimationJSON in, Bundle.SetAnimation
// out, and nothing in between needs the runtime to become mutable.
//
// Decoding keeps numbers as json.Number, so every value the editor does not
// touch is re-emitted byte for byte and only edited numbers are rewritten.
// Members come back key-sorted, which is the order editor/genpresets already
// produces, so tuning a preset diffs exactly where a value changed.

// poseProps are the transform members this editor understands. Anchor is in
// the list because it is worth reading and occasionally worth typing, but it
// is rig spec: nothing drags it, because moving an anchor detaches the part
// from its joint.
var poseProps = []string{"p", "r", "s", "o", "a"}

// clipDoc is one clip's editable document plus an index of the keyframes in
// it.
type clipDoc struct {
	id     string
	root   map[string]any
	indent string // reproduced on encode so a saved bundle diffs cleanly

	layers []clipLayer

	// times is the clip's key times. posed says they form a real pose
	// sequence — every animated property agrees on one set — which is true
	// of every preset clip and lets the timeline show poses instead of a
	// row per layer. When it is false, times is the union and the UI falls
	// back to per-layer rows.
	times []float64
	posed bool
}

// clipLayer indexes one layer. The transform members live in ks; keyed
// records which of them animate, and at what times.
type clipLayer struct {
	index int // position in root["layers"], which is also the draw order
	name  string
	ty    float64
	refID string // image layers only: the asset supplying width and height

	// ind names this layer, parent names the one it rides on. The rig
	// chains — forearm on upper arm on body — are these links.
	ind       float64
	parent    float64
	hasParent bool

	ks    map[string]any
	keyed map[string][]float64

	// shapeTimes is the union of the layer's animated shape-item members
	// (shapedoc.go). Shape keys join the pose columns: they show on the
	// timeline, and the column operations move them with everything else.
	shapeTimes []float64
}

// newClipDoc parses one clip. A document whose layers are not an array is
// rejected rather than half-indexed: every edit addresses a layer by index.
func newClipDoc(id string, data []byte) (*clipDoc, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var root map[string]any
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("clip %q: %w", id, err)
	}
	if _, ok := root["layers"].([]any); !ok {
		return nil, fmt.Errorf("clip %q: no layer array", id)
	}
	d := &clipDoc{id: id, root: root, indent: detectIndent(data)}
	d.index()
	return d, nil
}

// detectIndent reads the document's own formatting off its second line, so
// re-encoding neither inflates a compact bundle nor flattens an indented
// one. editor/genpresets writes a single space; a hand-authored file often
// uses two or a tab.
func detectIndent(data []byte) string {
	i := bytes.IndexByte(data, '\n')
	if i < 0 {
		return ""
	}
	rest := data[i+1:]
	n := 0
	for n < len(rest) && (rest[n] == ' ' || rest[n] == '\t') {
		n++
	}
	return string(rest[:n])
}

// index walks the layers once, recording where the keyframes are. Every
// later question — which ticks to draw, which key a click selects, what a
// drag writes — is answered from here rather than by re-walking the tree.
func (d *clipDoc) index() {
	d.layers = d.layers[:0]
	raw, _ := d.root["layers"].([]any)
	var sets [][]float64
	for i, lv := range raw {
		lm, ok := lv.(map[string]any)
		if !ok {
			continue
		}
		l := clipLayer{index: i}
		l.name, _ = lm["nm"].(string)
		l.ty, _ = jsonNum(lm["ty"])
		l.refID, _ = lm["refId"].(string)
		l.ind, _ = jsonNum(lm["ind"])
		l.parent, l.hasParent = jsonNum(lm["parent"])
		l.ks, _ = lm["ks"].(map[string]any)
		if l.ks == nil {
			d.layers = append(d.layers, l)
			continue
		}
		for _, p := range poseProps {
			keys, ok := propKeys(l.ks[p])
			if !ok {
				continue
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
			if len(ts) == 0 {
				continue
			}
			if l.keyed == nil {
				l.keyed = map[string][]float64{}
			}
			l.keyed[p] = ts
			sets = append(sets, ts)
		}
		d.layers = append(d.layers, l)
	}
	// Shape layers can animate members far below ks; their key times join
	// the same pool, or the timeline would show a vector clip as empty.
	for i := range d.layers {
		if d.layers[i].ty != 4 {
			continue
		}
		shapeSets := d.shapeKeyTimes(i)
		d.layers[i].shapeTimes, _ = poseSet(shapeSets)
		sets = append(sets, shapeSets...)
	}
	d.times, d.posed = poseSet(sets)
}

// poseSet reduces the clip's key times to one list. posed reports whether
// every animated property already agreed, which is what makes a tick a whole
// body pose rather than one property's key.
func poseSet(sets [][]float64) ([]float64, bool) {
	if len(sets) == 0 {
		return nil, false
	}
	union := map[float64]struct{}{}
	for _, s := range sets {
		for _, t := range s {
			union[t] = struct{}{}
		}
	}
	times := make([]float64, 0, len(union))
	for t := range union {
		times = append(times, t)
	}
	slices.Sort(times)
	posed := true
	for _, s := range sets {
		if !slices.Equal(s, times) {
			posed = false
			break
		}
	}
	return times, posed
}

// layer returns the indexed layer, or nil. Callers hold indices across
// frames, so every lookup is bounds-checked here instead of at each use.
func (d *clipDoc) layer(i int) *clipLayer {
	if i < 0 || i >= len(d.layers) {
		return nil
	}
	return &d.layers[i]
}

// encode serializes the document back to Lottie JSON.
func (d *clipDoc) encode() ([]byte, error) {
	if d.indent == "" {
		return json.Marshal(d.root)
	}
	return json.MarshalIndent(d.root, "", d.indent)
}

// ---- reading values ----

// jsonNum accepts the number forms a decoded document can hold. Everything
// arrives as json.Number because of UseNumber; the other cases cover values
// this editor has already written.
func jsonNum(v any) (float64, bool) {
	switch n := v.(type) {
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case float64:
		return n, true
	case int:
		return float64(n), true
	}
	return 0, false
}

// jsonNums reads a property value, which is a bare number for a scalar like
// rotation and an array for a vector like position.
func jsonNums(v any) ([]float64, bool) {
	if f, ok := jsonNum(v); ok {
		return []float64{f}, true
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]float64, 0, len(arr))
	for _, e := range arr {
		f, ok := jsonNum(e)
		if !ok {
			return nil, false
		}
		out = append(out, f)
	}
	return out, true
}

// propObj returns a transform member's {a, k} object. A document may also
// write the bare value with no wrapper; that form is readable but not
// something this editor rewrites, so it reports false.
func propObj(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, false
	}
	if _, ok := m["k"]; !ok {
		return nil, false
	}
	return m, true
}

// propKeys returns a member's keyframe list, or false when it is static. A
// keyframe list is an array of objects; a static vector is an array of
// numbers, which is why the first element decides.
func propKeys(v any) ([]any, bool) {
	m, ok := propObj(v)
	if !ok {
		return nil, false
	}
	arr, ok := m["k"].([]any)
	if !ok || len(arr) == 0 {
		return nil, false
	}
	if _, ok := arr[0].(map[string]any); !ok {
		return nil, false
	}
	return arr, true
}

// keyIndexAt finds the key of a property that sits exactly on a frame. Edits
// land on a key, never between two, so a miss means there is nothing to
// write.
func keyIndexAt(keys []any, frame float64) (int, bool) {
	for i, k := range keys {
		km, ok := k.(map[string]any)
		if !ok {
			continue
		}
		if t, ok := jsonNum(km["t"]); ok && t == frame {
			return i, true
		}
	}
	return 0, false
}

// valueAt reads what a layer's property holds at a key time: the stored
// value, not the interpolated one, so what the inspector shows is what is in
// the file. ok is false when the property is not animated or has no key
// there.
func (d *clipDoc) valueAt(layer int, prop string, frame float64) ([]float64, bool) {
	l := d.layer(layer)
	if l == nil || l.ks == nil {
		return nil, false
	}
	keys, ok := propKeys(l.ks[prop])
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

// staticValue reads a property that does not animate. Together with valueAt
// it covers every property the inspector can show.
func (d *clipDoc) staticValue(layer int, prop string) ([]float64, bool) {
	l := d.layer(layer)
	if l == nil || l.ks == nil {
		return nil, false
	}
	if _, animated := propKeys(l.ks[prop]); animated {
		return nil, false
	}
	m, ok := propObj(l.ks[prop])
	if !ok {
		return nil, false
	}
	return jsonNums(m["k"])
}

// value reads a property at a frame whether or not it animates, which is
// what a parameter pane wants: a static value is just as editable, it simply
// applies to the whole clip.
func (d *clipDoc) value(layer int, prop string, frame float64) ([]float64, bool) {
	if v, ok := d.valueAt(layer, prop, frame); ok {
		return v, true
	}
	return d.staticValue(layer, prop)
}

// valueNear reads a property at or before a frame: the last key that has
// happened, or the static value. Unlike value it answers between keys as
// well, which is what a readout of "what is this part doing right now"
// needs. It is exact for hold keys — the discrete swaps that decide which
// drawings are showing — and one step stale for interpolated ones.
func (d *clipDoc) valueNear(layer int, prop string, frame float64) ([]float64, bool) {
	l := d.layer(layer)
	if l == nil || l.ks == nil {
		return nil, false
	}
	keys, animated := propKeys(l.ks[prop])
	if !animated {
		return d.staticValue(layer, prop)
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

// isHold reports whether a key switches instantly instead of interpolating.
// Hold keys carry the discrete swaps — a limb trading sides, a view drawing
// changing — so the timeline marks them differently and nothing invites a
// drag on a value that has only two states.
func (d *clipDoc) isHold(layer int, prop string, frame float64) bool {
	l := d.layer(layer)
	if l == nil || l.ks == nil {
		return false
	}
	keys, ok := propKeys(l.ks[prop])
	if !ok {
		return false
	}
	i, ok := keyIndexAt(keys, frame)
	if !ok {
		return false
	}
	km, _ := keys[i].(map[string]any)
	h, ok := jsonNum(km["h"])
	return ok && h != 0
}

// poseIsHold reports whether every animated property at a pose time holds.
// A pose built entirely from hold keys is a swap, not something to nudge.
func (d *clipDoc) poseIsHold(frame float64) bool {
	any := false
	for i := range d.layers {
		for p := range d.layers[i].keyed {
			if !slices.Contains(d.layers[i].keyed[p], frame) {
				continue
			}
			any = true
			if !d.isHold(i, p, frame) {
				return false
			}
		}
	}
	return any
}

// ---- writing values ----

// setValue writes a property at a frame, promoting a static property to a
// keyed one first when the clip has a pose set to key it against. It reports
// whether the document changed, so a no-op drag does not mark the bundle
// edited.
func (d *clipDoc) setValue(layer int, prop string, frame float64, v []float64) bool {
	l := d.layer(layer)
	if l == nil || l.ks == nil || len(v) == 0 {
		return false
	}
	if _, animated := propKeys(l.ks[prop]); !animated {
		// A static property is the same value at every frame. Editing it at
		// one pose means the clip now needs a key at every pose, holding
		// what it used to be everywhere else — the inverse of the generator
		// collapsing an unchanging track back to a static value.
		if !d.promote(layer, prop) {
			return d.setStatic(layer, prop, v)
		}
	}
	keys, ok := propKeys(l.ks[prop])
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

// setStatic rewrites a property that has no pose set to key against, which
// is the only thing a clip with no keyframes at all can offer.
func (d *clipDoc) setStatic(layer int, prop string, v []float64) bool {
	l := d.layer(layer)
	if l == nil || l.ks == nil {
		return false
	}
	m, ok := propObj(l.ks[prop])
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

// numsLike writes v in the shape the document already used for that slot: a
// bare number stays bare, an array stays an array. Rotation is stored both
// ways depending on the authoring tool, and rewriting the shape would be a
// gratuitous diff.
func numsLike(old any, v []float64) any {
	if _, wasArray := old.([]any); !wasArray {
		if _, ok := jsonNum(old); ok && len(v) == 1 {
			return v[0]
		}
	}
	out := make([]any, len(v))
	for i, f := range v {
		out[i] = f
	}
	return out
}

// promote turns a static property into a keyframed one holding its current
// value at every pose time. It fails when the clip has no pose set, because
// there is then no agreed set of times to key against and inventing one
// would break the invariant the timeline relies on.
func (d *clipDoc) promote(layer int, prop string) bool {
	l := d.layer(layer)
	if l == nil || l.ks == nil || !d.posed || len(d.times) == 0 {
		return false
	}
	m, ok := propObj(l.ks[prop])
	if !ok {
		return false
	}
	if _, animated := propKeys(l.ks[prop]); animated {
		return true
	}
	v, ok := jsonNums(m["k"])
	if !ok {
		return false
	}
	keys := make([]any, 0, len(d.times))
	for _, t := range d.times {
		keys = append(keys, newKey(t, v))
	}
	m["a"] = 1
	m["k"] = keys
	d.index()
	return true
}

// newKey builds one keyframe in the shape editor/genpresets writes, with the
// linear easing that reads as "no easing chosen".
func newKey(t float64, v []float64) map[string]any {
	s := make([]any, len(v))
	for i, f := range v {
		s[i] = f
	}
	return map[string]any{
		"t": t,
		"s": s,
		"i": map[string]any{"x": []any{0.5}, "y": []any{1.0}},
		"o": map[string]any{"x": []any{0.5}, "y": []any{0.0}},
	}
}

// retime moves a key time. layer is negative for the whole pose column,
// which is what a tick means on a clip whose properties share one time set;
// otherwise every property of that one layer keyed at from moves together,
// so a row's tick also stays one tick. It reports the frame actually
// reached, which the caller uses to follow the drag.
func (d *clipDoc) retime(from, to float64, layer int) (float64, bool) {
	lo, hi, ok := d.neighbours(from, layer)
	if !ok {
		return from, false
	}
	to = math.Round(min(max(to, lo), hi))
	if to == from {
		return from, false
	}
	if layer < 0 {
		for i := range d.layers {
			for p := range d.layers[i].keyed {
				d.moveKey(i, p, from, to)
			}
			d.eachShapeProp(i, func(owner map[string]any, member string) {
				moveKeyProp(owner[member], from, to)
			})
		}
	} else {
		l := d.layer(layer)
		if l == nil {
			return from, false
		}
		for p := range l.keyed {
			d.moveKey(layer, p, from, to)
		}
		d.eachShapeProp(layer, func(owner map[string]any, member string) {
			moveKeyProp(owner[member], from, to)
		})
	}
	d.index()
	return to, true
}

// neighbours bounds a retime: a key may travel up to one frame short of each
// side, so it never crosses a sibling and the key list stays sorted without
// a resort mid-drag. The clip's own extent bounds the ends.
func (d *clipDoc) neighbours(frame float64, layer int) (lo, hi float64, ok bool) {
	times := d.times
	if layer >= 0 {
		l := d.layer(layer)
		if l == nil {
			return 0, 0, false
		}
		times = l.layerTimes()
	}
	i := slices.Index(times, frame)
	if i < 0 {
		return 0, 0, false
	}
	lo, hi = d.inPoint(), d.outPoint()
	if i > 0 {
		lo = times[i-1] + 1
	}
	if i < len(times)-1 {
		hi = times[i+1] - 1
	}
	if hi < lo {
		return 0, 0, false
	}
	return lo, hi, true
}

func (d *clipDoc) moveKey(layer int, prop string, from, to float64) {
	l := d.layer(layer)
	if l == nil || l.ks == nil {
		return
	}
	keys, ok := propKeys(l.ks[prop])
	if !ok {
		return
	}
	i, ok := keyIndexAt(keys, from)
	if !ok {
		return
	}
	km, _ := keys[i].(map[string]any)
	km["t"] = to
}

func (d *clipDoc) inPoint() float64 {
	f, _ := jsonNum(d.root["ip"])
	return f
}

func (d *clipDoc) outPoint() float64 {
	if f, ok := jsonNum(d.root["op"]); ok {
		return f
	}
	return 0
}

// layerTimes is the tick set of one row: the union of that layer's animated
// properties. A layer whose properties disagree is rare, but the row must
// still show every tick that can be selected.
func (l *clipLayer) layerTimes() []float64 {
	if len(l.keyed) == 0 && len(l.shapeTimes) == 0 {
		return nil
	}
	sets := make([][]float64, 0, len(l.keyed)+1)
	for _, ts := range l.keyed {
		sets = append(sets, ts)
	}
	if len(l.shapeTimes) > 0 {
		sets = append(sets, l.shapeTimes)
	}
	times, _ := poseSet(sets)
	return times
}

// layerSize is the part image's pixel size, which is also the layer's local
// box: an image layer draws from its own origin, and the anchor is a point
// inside that box. Only image layers have one, which is why the pose editor
// picks and outlines those and leaves shape layers alone.
func (d *clipDoc) layerSize(i int) (w, h float64, ok bool) {
	l := d.layer(i)
	if l == nil || l.refID == "" {
		return 0, 0, false
	}
	assets, _ := d.root["assets"].([]any)
	for _, av := range assets {
		am, ok := av.(map[string]any)
		if !ok {
			continue
		}
		if id, _ := am["id"].(string); id != l.refID {
			continue
		}
		w, wok := jsonNum(am["w"])
		h, hok := jsonNum(am["h"])
		if !wok || !hok || w <= 0 || h <= 0 {
			return 0, 0, false
		}
		return w, h, true
	}
	return 0, 0, false
}

// parentName is the name of the layer this one rides on, which is how a
// drag gets back into the space the transform is written in. Names are how
// the core resolves a layer, so an unnamed parent is no use here and the
// caller falls back to the animation's own space.
func (d *clipDoc) parentName(i int) (string, bool) {
	l := d.layer(i)
	if l == nil || !l.hasParent {
		return "", false
	}
	for j := range d.layers {
		if d.layers[j].ind == l.parent && d.layers[j].name != "" {
			return d.layers[j].name, true
		}
	}
	return "", false
}

// animatedLayers lists the layers with at least one keyframe, in draw order.
// They are the rows of the fallback timeline, and the only layers a retime
// can address.
func (d *clipDoc) animatedLayers() []int {
	var out []int
	for i := range d.layers {
		if len(d.layers[i].keyed) > 0 || len(d.layers[i].shapeTimes) > 0 {
			out = append(out, i)
		}
	}
	return out
}

// ---- draw order ----

// The layer array is the draw order: first is topmost. Reordering it is how
// a rig's overlap changes — the gripping forearm coming in front of the
// torso, a sword passing behind the body mid-swing — and the rig survives
// it, because parent links are by ind and resolve in a second pass rather
// than by position.
//
// Track mattes do not survive it. A matte with no explicit tp takes the
// layer immediately before it as its source, so moving either one silently
// retargets the matte. Clips using them refuse to reorder rather than
// quietly breaking.

// usesTrackMatte reports whether any layer is a track matte or the source of
// one.
func (d *clipDoc) usesTrackMatte() bool {
	raw, _ := d.root["layers"].([]any)
	for _, lv := range raw {
		lm, ok := lv.(map[string]any)
		if !ok {
			continue
		}
		if v, ok := jsonNum(lm["tt"]); ok && v != 0 {
			return true
		}
		if v, ok := jsonNum(lm["td"]); ok && v != 0 {
			return true
		}
	}
	return false
}

// moveLayer moves a layer so that it ends up at index to.
func (d *clipDoc) moveLayer(from, to int) bool {
	raw, _ := d.root["layers"].([]any)
	if from < 0 || from >= len(raw) || to < 0 || to >= len(raw) || from == to {
		return false
	}
	v := raw[from]
	raw = slices.Insert(slices.Delete(raw, from, from+1), to, v)
	d.root["layers"] = raw
	d.index()
	return true
}

// shiftIndex maps a layer index across a move, so a selection made before
// the move still names the same layer after it.
func shiftIndex(i, from, to int) int {
	switch {
	case i == from:
		return to
	case from < i && i <= to:
		return i - 1
	case to <= i && i < from:
		return i + 1
	}
	return i
}

// ---- poses as columns ----

// A pose is a column: the same time on every animated track. Inserting and
// deleting therefore work on all of them at once, which is what keeps the
// clip a pose sequence and a timeline tick a whole-body pose.

// keyAt returns a property's key object at a frame.
func keyAt(keys []any, frame float64) (map[string]any, bool) {
	i, ok := keyIndexAt(keys, frame)
	if !ok {
		return nil, false
	}
	km, ok := keys[i].(map[string]any)
	return km, ok
}

// insertPose adds a key at frame to every animated property, copying the key
// before it. A new pose starts as the one it follows and is then changed,
// which is how a pose sequence is built by hand; it also means the insert
// leaves the clip looking exactly as it did up to that point.
func (d *clipDoc) insertPose(frame float64) bool {
	if slices.Contains(d.times, frame) || frame < d.inPoint() || frame > d.outPoint() {
		return false
	}
	changed := false
	for li := range d.layers {
		l := &d.layers[li]
		for prop := range l.keyed {
			keys, ok := propKeys(l.ks[prop])
			if !ok {
				continue
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
				// The source key carries no value to copy (a legacy
				// trailing {t} key, or not a map at all); inserting nil
				// would corrupt the document.
				continue
			}
			at := len(keys)
			for i, k := range keys {
				km, _ := k.(map[string]any)
				if t, ok := jsonNum(km["t"]); ok && t > frame {
					at = i
					break
				}
			}
			m, _ := propObj(l.ks[prop])
			m["k"] = slices.Insert(keys, at, any(nk))
			changed = true
		}
		d.eachShapeProp(li, func(owner map[string]any, member string) {
			if insertKeyProp(owner[member], frame) {
				changed = true
			}
		})
	}
	if changed {
		d.index()
	}
	return changed
}

// copyKey duplicates a keyframe at a new time, carrying its value, its hold
// flag and its timing curve. The curve is copied rather than split, so a new
// key inside an eased segment retimes that segment slightly; the pose it
// shows at the moment it is inserted is exact.
func copyKey(src map[string]any, frame float64) map[string]any {
	nk := map[string]any{"t": frame}
	for _, member := range []string{"s", "i", "o", "h"} {
		if v, ok := src[member]; ok {
			nk[member] = deepCopyJSON(v)
		}
	}
	if _, ok := nk["s"]; !ok {
		return nil
	}
	return nk
}

// deepCopyJSON clones a decoded value, so an inserted key never shares a
// slice or map with the key it came from.
func deepCopyJSON(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, e := range t {
			out[k] = deepCopyJSON(e)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = deepCopyJSON(e)
		}
		return out
	}
	return v
}

// deletePose removes the key at frame from every animated property. A track
// is never left empty: the last key of a clip is what it settles on.
func (d *clipDoc) deletePose(frame float64) bool {
	if !slices.Contains(d.times, frame) || len(d.times) <= 1 {
		return false
	}
	changed := false
	for li := range d.layers {
		l := &d.layers[li]
		for prop := range l.keyed {
			keys, ok := propKeys(l.ks[prop])
			if !ok || len(keys) <= 1 {
				continue
			}
			i, ok := keyIndexAt(keys, frame)
			if !ok {
				continue
			}
			m, _ := propObj(l.ks[prop])
			m["k"] = slices.Delete(keys, i, i+1)
			changed = true
		}
		d.eachShapeProp(li, func(owner map[string]any, member string) {
			if deleteKeyProp(owner[member], frame) {
				changed = true
			}
		})
	}
	if changed {
		d.index()
	}
	return changed
}

// Easing values match editor/genpresets: the eased pair is what its clips
// use for a pose that arrives softly, the other is the "no curve chosen"
// shape a linear key carries.
func easedIn() any  { return map[string]any{"x": []any{0.4}, "y": []any{1.0}} }
func easedOut() any { return map[string]any{"x": []any{0.6}, "y": []any{0.0}} }
func linearIn() any { return map[string]any{"x": []any{0.5}, "y": []any{1.0}} }
func linearOut() any {
	return map[string]any{"x": []any{0.5}, "y": []any{0.0}}
}

// poseEased reports whether the column at frame carries the eased curve.
func (d *clipDoc) poseEased(frame float64) bool {
	for li := range d.layers {
		l := &d.layers[li]
		for prop := range l.keyed {
			keys, ok := propKeys(l.ks[prop])
			if !ok {
				continue
			}
			km, ok := keyAt(keys, frame)
			if !ok {
				continue
			}
			return easeAxisIs(km["o"], "x", 0.6)
		}
	}
	return false
}

// easeAxisIs tests one control point, which is enough to tell the two
// shapes this editor writes apart.
func easeAxisIs(obj any, axis string, want float64) bool {
	m, ok := obj.(map[string]any)
	if !ok {
		return false
	}
	arr, ok := m[axis].([]any)
	if !ok || len(arr) == 0 {
		f, ok := jsonNum(m[axis])
		return ok && f == want
	}
	f, ok := jsonNum(arr[0])
	return ok && f == want
}

// setPoseEase gives the whole column a curve. Easing is a property of the
// pose, not of one limb: a body that arrives softly while its arm arrives
// linearly reads as a mistake.
func (d *clipDoc) setPoseEase(frame float64, eased bool) bool {
	if !slices.Contains(d.times, frame) {
		return false
	}
	changed := false
	for li := range d.layers {
		l := &d.layers[li]
		for prop := range l.keyed {
			keys, ok := propKeys(l.ks[prop])
			if !ok {
				continue
			}
			km, ok := keyAt(keys, frame)
			if !ok {
				continue
			}
			// A hold key switches instantly; a curve on it means nothing.
			if h, ok := jsonNum(km["h"]); ok && h != 0 {
				continue
			}
			if eased {
				km["i"], km["o"] = easedIn(), easedOut()
			} else {
				km["i"], km["o"] = linearIn(), linearOut()
			}
			changed = true
		}
		d.eachShapeProp(li, func(owner map[string]any, member string) {
			if setEaseProp(owner[member], frame, eased) {
				changed = true
			}
		})
	}
	return changed
}

// setLength changes how long the clip runs. The document's out point and the
// layers' own are set together: a layer that stopped when the clip did must
// keep doing so, or lengthening a clip would make every part vanish at the
// old end.
func (d *clipDoc) setLength(op float64) bool {
	old := d.outPoint()
	if op <= d.inPoint() || op == old {
		return false
	}
	if len(d.times) > 0 && op < d.times[len(d.times)-1] {
		return false
	}
	d.root["op"] = op
	raw, _ := d.root["layers"].([]any)
	for _, lv := range raw {
		lm, ok := lv.(map[string]any)
		if !ok {
			continue
		}
		if v, ok := jsonNum(lm["op"]); ok && v == old {
			lm["op"] = op
		}
	}
	return true
}

// swapPose trades the pose between the rig's paired limbs at one frame: the
// near arm takes the far arm's, and back. Half a walk cycle is the other
// half with the legs traded, so building one is otherwise the same pose
// dialled in twice.
//
// Pairs come from the naming convention — a "-near" slot and the "-far" one
// with the same prefix — rather than a list kept here, so a rig that grows a
// slot is paired without this file being told about it.
//
// Only properties keyed on both sides trade. The static ones are rig spec:
// a limb's attach point puts it on its own side of the torso, and trading
// those would detach the pair rather than swap it. Draw order is left alone
// too — which limb is in front is its own edit.
func (d *clipDoc) swapPose(frame float64) bool {
	byName := make(map[string]int, len(d.layers))
	for i := range d.layers {
		if n := d.layers[i].name; n != "" {
			byName[n] = i
		}
	}
	changed := false
	for i := range d.layers {
		name := d.layers[i].name
		prefix, ok := strings.CutSuffix(name, "-near")
		if !ok {
			continue
		}
		j, ok := byName[prefix+"-far"]
		if !ok {
			continue
		}
		for _, prop := range poseProps {
			if !slices.Contains(d.layers[i].keyed[prop], frame) ||
				!slices.Contains(d.layers[j].keyed[prop], frame) {
				continue
			}
			a, oka := d.valueAt(i, prop, frame)
			b, okb := d.valueAt(j, prop, frame)
			if !oka || !okb {
				continue
			}
			a, b = slices.Clone(a), slices.Clone(b)
			if d.setValue(i, prop, frame, b) {
				changed = true
			}
			if d.setValue(j, prop, frame, a) {
				changed = true
			}
		}
	}
	return changed
}

// ---- parenting ----

// A layer rides on its parent: its position is a point in the parent's
// space, and its rotation adds to whatever the parent is doing. Changing
// that link is how a part moves between chains — a sword passing from one
// hand to the other — and the values have to be rewritten with it or the
// part jumps the moment the link changes.

// childrenOf lists the layers whose parent is this one.
func (d *clipDoc) childrenOf(layer int) []int {
	l := d.layer(layer)
	if l == nil {
		return nil
	}
	var out []int
	for i := range d.layers {
		if d.layers[i].hasParent && d.layers[i].parent == l.ind {
			out = append(out, i)
		}
	}
	return out
}

// descendantsOf is every layer riding on this one, however far down.
func (d *clipDoc) descendantsOf(layer int) []int {
	var out []int
	stack := []int{layer}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, c := range d.childrenOf(cur) {
			if slices.Contains(out, c) {
				continue // a cycle already in the file; do not spin on it
			}
			out = append(out, c)
			stack = append(stack, c)
		}
	}
	return out
}

// parentCandidates are the layers this one may be attached to. Its own
// descendants are left out rather than rejected afterwards: a cycle is not a
// mistake to report, it is a thing the picker should not be able to say.
func (d *clipDoc) parentCandidates(layer int) []int {
	l := d.layer(layer)
	if l == nil {
		return nil
	}
	bad := append(d.descendantsOf(layer), layer)
	var out []int
	for i := range d.layers {
		if slices.Contains(bad, i) {
			continue
		}
		// A parent is addressed by ind, so a layer without one cannot be
		// named as anybody's.
		if d.layers[i].ind == 0 {
			continue
		}
		out = append(out, i)
	}
	return out
}

// parentOf returns the layer index of this one's parent.
func (d *clipDoc) parentOf(layer int) (int, bool) {
	l := d.layer(layer)
	if l == nil || !l.hasParent {
		return 0, false
	}
	for i := range d.layers {
		if d.layers[i].ind == l.parent {
			return i, true
		}
	}
	return 0, false
}

// setParent rewrites the link. parent < 0 detaches the layer, which makes it
// ride on the composition itself.
func (d *clipDoc) setParent(layer, parent int) bool {
	l := d.layer(layer)
	if l == nil || layer == parent {
		return false
	}
	raw, _ := d.root["layers"].([]any)
	if l.index >= len(raw) {
		return false
	}
	lm, ok := raw[l.index].(map[string]any)
	if !ok {
		return false
	}
	if parent < 0 {
		if !l.hasParent {
			return false
		}
		delete(lm, "parent")
		d.index()
		return true
	}
	p := d.layer(parent)
	if p == nil || slices.Contains(d.descendantsOf(layer), parent) {
		return false
	}
	if l.hasParent && l.parent == p.ind {
		return false
	}
	lm["parent"] = p.ind
	d.index()
	return true
}

// layerHasParent reports whether a layer rides on another rather than on the
// composition.
func (d *clipDoc) layerHasParent(layer int) bool {
	l := d.layer(layer)
	return l != nil && l.hasParent
}

// keyTimesOf is one property's key times, empty when it does not animate.
func (d *clipDoc) keyTimesOf(layer int, prop string) []float64 {
	l := d.layer(layer)
	if l == nil || l.keyed == nil {
		return nil
	}
	return l.keyed[prop]
}

// nameProblem reports why a layer cannot be addressed by name, or "" when
// it can. api:layer-placement resolves a layer by name and takes the first
// match, so an unnamed layer cannot be found at all and a duplicated one
// finds whichever came first — and the pose editor asks for a part and its
// parent by name every time it converts a drag.
func (d *clipDoc) nameProblem(layer int) string {
	l := d.layer(layer)
	if l == nil {
		return "no such layer"
	}
	if l.name == "" {
		return "this layer has no name"
	}
	for i := range d.layers {
		if i != layer && d.layers[i].name == l.name {
			return "another layer is also called " + l.name
		}
	}
	return ""
}

// setName renames a layer, refusing a blank or a name already taken. Names
// are how everything outside this document refers to a layer, so a
// duplicate is not a cosmetic problem.
func (d *clipDoc) setName(layer int, name string) bool {
	l := d.layer(layer)
	if l == nil || name == "" || name == l.name {
		return false
	}
	for i := range d.layers {
		if i != layer && d.layers[i].name == name {
			return false
		}
	}
	raw, _ := d.root["layers"].([]any)
	if l.index >= len(raw) {
		return false
	}
	lm, ok := raw[l.index].(map[string]any)
	if !ok {
		return false
	}
	lm["nm"] = name
	d.index()
	return true
}
