package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	lottie "github.com/shibukawa/lottie-go"
)

// runMorph bakes the spec's generators and the attachments' kind motion
// into the bundle's clips as path and rotation keys
// (concept:uv-morph-rig morph_tracks, attachments).
func runMorph(args []string) error {
	fs := flag.NewFlagSet("morph", flag.ContinueOnError)
	presets := fs.String("presets", "", "directory holding the base presets")
	in := fs.String("in", "", "bundle to morph (default work/<name>.lottie)")
	out := fs.String("o", "", "bundle to write (default: in place)")
	step := fs.Float64("step", 4, "frames between baked path keys")
	if err := fs.Parse(args); err != nil {
		return err
	}
	work := "."
	if fs.NArg() > 0 {
		work = fs.Arg(0)
	}
	spec, err := loadSpec(work)
	if err != nil {
		return err
	}
	basePath, err := resolveBase(spec.Base, *presets)
	if err != nil {
		return err
	}
	r, err := loadRig(basePath)
	if err != nil {
		return err
	}
	atts, err := expandAttachments(spec, r)
	if err != nil {
		return err
	}
	if *in == "" {
		*in = filepath.Join(work, spec.Name+".lottie")
	}
	if *out == "" {
		*out = *in
	}
	data, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	b, err := lottie.DecodeBundle(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	m := &morpher{spec: spec, rig: r, bundle: b, step: *step, geoms: map[string]*partGeom{}, atts: map[string]*attLayer{}}
	for _, al := range atts {
		m.atts[al.name] = al
	}
	for _, name := range b.ExtensionFiles(forgeDir) {
		raw, _ := b.ExtensionFile(name)
		g, err := parseGeom(raw)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		m.geoms[strings.TrimSuffix(strings.TrimPrefix(name, forgeDir), ".json")] = g
	}
	if len(m.geoms) == 0 {
		return fmt.Errorf("%s carries no extensions/forge/ contours; morph needs a bundle written by lottieforge rig", *in)
	}
	baked := 0
	for _, id := range b.AnimationIDs() {
		n, err := m.morphClip(id)
		if err != nil {
			return fmt.Errorf("%s: %w", id, err)
		}
		baked += n
	}
	var buf bytes.Buffer
	if err := b.Encode(&buf); err != nil {
		return err
	}
	if err := os.WriteFile(*out, buf.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s: %d tracks baked over %d clips\n", *out, baked, len(b.AnimationIDs()))
	return nil
}

type morpher struct {
	spec   *Spec
	rig    *rig
	bundle *lottie.Bundle
	step   float64
	geoms  map[string]*partGeom
	atts   map[string]*attLayer
}

// clipCtx is one clip being baked: its decoded form answers placement
// queries, its tree receives the keys.
type clipCtx struct {
	id      string
	anim    *lottie.Animation
	clip    obj
	fps, op float64
	layers  map[string]obj
	parents map[string]string
	cache   map[string]lottie.LayerPlacement
}

func (c *clipCtx) placement(name string, f float64) lottie.LayerPlacement {
	key := fmt.Sprintf("%s@%.2f", name, f)
	if p, ok := c.cache[key]; ok {
		return p
	}
	p, _ := c.anim.LayerPlacement(name, f)
	c.cache[key] = p
	return p
}

// vertexGen returns per-piece, per-vertex offsets for a layer at frame f.
type vertexGen func(c *clipCtx, layer string, g *partGeom, f float64) [][]pt

// rotGen returns a rotation offset in degrees for a layer at frame f.
type rotGen func(c *clipCtx, layer string, f float64) float64

func (m *morpher) morphClip(id string) (int, error) {
	raw, _ := m.bundle.AnimationJSON(id)
	anim, err := lottie.Decode(bytes.NewReader(raw))
	if err != nil {
		return 0, err
	}
	var clip obj
	if err := json.Unmarshal(raw, &clip); err != nil {
		return 0, err
	}
	c := &clipCtx{id: id, anim: anim, clip: clip, fps: num(clip["fr"]), op: num(clip["op"]),
		layers: map[string]obj{}, parents: map[string]string{}, cache: map[string]lottie.LayerPlacement{}}
	byInd := map[int]string{}
	for _, l := range layersOf(clip) {
		name, _ := l["nm"].(string)
		c.layers[name] = l
		byInd[layerInd(l)] = name
	}
	for name, l := range c.layers {
		if l["parent"] != nil {
			c.parents[name] = byInd[int(num(l["parent"]))]
		}
	}
	vgens := map[string][]vertexGen{}
	rgens := map[string][]rotGen{}
	add := func(layer string, vg vertexGen, rg rotGen) {
		if _, ok := c.layers[layer]; !ok {
			return
		}
		if vg != nil {
			vgens[layer] = append(vgens[layer], vg)
		}
		if rg != nil {
			rgens[layer] = append(rgens[layer], rg)
		}
	}
	// Attachment kinds imply motion in every clip.
	for name, al := range m.atts {
		switch al.att.Kind {
		case "swing":
			add(name, nil, m.swingGen(al))
		case "lock":
			add(name, followGen(3, al.att.Weight), m.swingGen(al))
		case "drape":
			add(name, drapeGen(al.att.Drivers, al.att.Weight), nil)
		case "rigid":
			if s := al.att.Sway; s != nil && s.Amount != 0 {
				add(name, nil, swayGen(s.Amount, s.Period))
			}
		}
	}
	for _, ms := range m.spec.Morph {
		if !(ms.Clips.All || ms.Clips.has(id) || len(ms.Clips.List) == 0 && ms.Generator == "bend") {
			continue
		}
		var targets []string
		if ms.Parts.All {
			for name := range m.geoms {
				targets = append(targets, name)
			}
		} else {
			targets = ms.Parts.List
		}
		for _, t := range targets {
			for _, name := range m.resolveTargets(t) {
				vg, rg := m.generator(ms)
				add(name, vg, rg)
			}
		}
	}
	baked := 0
	for layer, gens := range vgens {
		g, ok := m.geoms[layer]
		if !ok {
			continue
		}
		if m.bakePaths(c, layer, g, gens) {
			baked++
		}
	}
	for layer, gens := range rgens {
		if m.bakeRotation(c, layer, gens) {
			baked++
		}
	}
	if baked == 0 {
		return 0, nil
	}
	out, err := json.Marshal(clip)
	if err != nil {
		return 0, err
	}
	return baked, m.bundle.SetAnimation(id, out)
}

// resolveTargets expands a spec part name to layer names: a bare limb
// name means both sides, an attachment name means all its layers.
func (m *morpher) resolveTargets(name string) []string {
	var out []string
	for layer := range m.geoms {
		if layer == name || layer == name+"-near" || layer == name+"-far" {
			out = append(out, layer)
		}
	}
	for layer, al := range m.atts {
		if al.att.Name == name && layer != name {
			out = append(out, layer)
		}
	}
	return out
}

func (m *morpher) generator(ms MorphSpec) (vertexGen, rotGen) {
	switch ms.Generator {
	case "breathe":
		return breatheGen(orDefault(ms.Amount, 0.03), orDefault(ms.Period, 48)), nil
	case "squash":
		return squashGen(ms.At, orDefault(ms.Amount, 0.12), orDefault(ms.Recover, 6)), nil
	case "stretch":
		return stretchGen(ms.From, ms.To, orDefault(ms.Amount, 0.1)), nil
	case "bend":
		return bendGen(orDefault(ms.Threshold, 60), orDefault(ms.Reach, 4)), nil
	case "follow":
		return followGen(orDefault(ms.Lag, 3), orDefault(ms.Weight, 0.4)), nil
	case "drape":
		return drapeGen(ms.Drivers, orDefault(ms.Weight, 0.6)), nil
	case "sway":
		return nil, swayGen(orDefault(ms.Amount, 4), orDefault(ms.Period, 96))
	}
	return nil, nil
}

func orDefault(v, d float64) float64 {
	if v == 0 {
		return d
	}
	return v
}

// bakePaths sums the generators at every baked frame and rewrites the
// layer's contours as path keys; welds keep seams closed.
func (m *morpher) bakePaths(c *clipCtx, layer string, g *partGeom, gens []vertexGen) bool {
	l := c.layers[layer]
	shapes, _ := l["shapes"].([]any)
	if len(shapes) == 0 {
		return false
	}
	group, _ := shapes[0].(obj)
	items, _ := group["it"].([]any)
	if len(items) < len(g.Pieces) {
		return false
	}
	frames := bakeFrames(c.op, m.step)
	keys := make([][]obj, len(g.Pieces))
	moved := false
	for _, f := range frames {
		offs := make([][]pt, len(g.Pieces))
		for i, p := range g.Pieces {
			offs[i] = make([]pt, len(p.V))
		}
		for _, gen := range gens {
			o := gen(c, layer, g, f)
			if o == nil {
				continue
			}
			for i := range offs {
				for j := range offs[i] {
					if i < len(o) && j < len(o[i]) {
						offs[i][j][0] += o[i][j][0]
						offs[i][j][1] += o[i][j][1]
					}
				}
			}
		}
		for _, w := range g.Welds {
			a, b := &offs[w[0]][w[1]], &offs[w[2]][w[3]]
			avg := pt{(a[0] + b[0]) / 2, (a[1] + b[1]) / 2}
			*a, *b = avg, avg
		}
		for i, p := range g.Pieces {
			v := make([]pt, len(p.V))
			for j := range v {
				v[j] = pt{p.V[j][0] + offs[i][j][0], p.V[j][1] + offs[i][j][1]}
				if math.Abs(offs[i][j][0]) > 0.01 || math.Abs(offs[i][j][1]) > 0.01 {
					moved = true
				}
			}
			in, out := tangents(v, p.Corner)
			val := []obj{pathObj(v, in, out, true)}
			if f == frames[len(frames)-1] {
				keys[i] = append(keys[i], keyEnd(f, val))
			} else {
				keys[i] = append(keys[i], keyLin(f, val))
			}
		}
	}
	if !moved {
		return false
	}
	for i := range g.Pieces {
		item, _ := items[i].(obj)
		if item == nil || item["ty"] != "sh" {
			continue
		}
		item["ks"] = keyed(keys[i])
	}
	return true
}

func bakeFrames(op, step float64) []float64 {
	if step < 1 {
		step = 1
	}
	var frames []float64
	for f := 0.0; f < op; f += step {
		frames = append(frames, f)
	}
	return append(frames, op)
}

func (m *morpher) bakeRotation(c *clipCtx, layer string, gens []rotGen) bool {
	l := c.layers[layer]
	ks, _ := l["ks"].(obj)
	rp, _ := ks["r"].(obj)
	if rp == nil || num(rp["a"]) != 0 {
		return false
	}
	base := num(rp["k"])
	frames := bakeFrames(c.op, 2)
	var keys []obj
	moved := false
	for _, f := range frames {
		v := base
		for _, gen := range gens {
			v += gen(c, layer, f)
		}
		if math.Abs(v-base) > 0.05 {
			moved = true
		}
		if f == frames[len(frames)-1] {
			keys = append(keys, keyEnd(f, []float64{round2(v)}))
		} else {
			keys = append(keys, keyLin(f, []float64{round2(v)}))
		}
	}
	if !moved {
		return false
	}
	ks["r"] = keyed(keys)
	return true
}

// ---- vertex generators ----

func zeroOffsets(g *partGeom) [][]pt {
	out := make([][]pt, len(g.Pieces))
	for i, p := range g.Pieces {
		out[i] = make([]pt, len(p.V))
	}
	return out
}

func breatheGen(amount, period float64) vertexGen {
	return func(c *clipCtx, layer string, g *partGeom, f float64) [][]pt {
		k := amount * 0.5 * (1 - math.Cos(2*math.Pi*f/period))
		out := zeroOffsets(g)
		above := false
		for _, p := range g.Pieces {
			for _, v := range p.V {
				above = above || v[1] < g.Anchor[1]-1
			}
		}
		for i, p := range g.Pieces {
			for j, v := range p.V {
				if above && v[1] >= g.Anchor[1]-1 {
					continue
				}
				out[i][j] = pt{(v[0] - g.Anchor[0]) * k, (v[1] - g.Anchor[1]) * k}
			}
		}
		return out
	}
}

func squashGen(at, amount, recover float64) vertexGen {
	return func(c *clipCtx, layer string, g *partGeom, f float64) [][]pt {
		var k float64
		switch {
		case f >= at-3 && f <= at:
			k = (f - (at - 3)) / 3
		case f > at && f <= at+recover:
			k = 1 - (f-at)/recover
		}
		if k <= 0 {
			return nil
		}
		out := zeroOffsets(g)
		for i, p := range g.Pieces {
			for j, v := range p.V {
				out[i][j] = pt{(v[0] - g.Anchor[0]) * amount * k, -(v[1] - g.Anchor[1]) * amount * k}
			}
		}
		return out
	}
}

func stretchGen(from, to, amount float64) vertexGen {
	return func(c *clipCtx, layer string, g *partGeom, f float64) [][]pt {
		if f < from-2 || f > to+2 {
			return nil
		}
		k := 1.0
		if f < from {
			k = (f - (from - 2)) / 2
		} else if f > to {
			k = (to + 2 - f) / 2
		}
		out := zeroOffsets(g)
		for i, p := range g.Pieces {
			for j, v := range p.V {
				out[i][j] = pt{0, (v[1] - g.Anchor[1]) * amount * k}
			}
		}
		return out
	}
}

// bendGen closes the crease of a chain child: when the joint folds past
// threshold, the inner-side vertices near the joint slide toward the
// parent.
func bendGen(threshold, reach float64) vertexGen {
	return func(c *clipCtx, layer string, g *partGeom, f float64) [][]pt {
		parent := c.parents[layer]
		if parent == "" {
			return nil
		}
		theta := normDeg(deg(c.placement(layer, f).Angle) - deg(c.placement(parent, f).Angle))
		if math.Abs(theta) <= threshold {
			return nil
		}
		amt := reach * math.Min((math.Abs(theta)-threshold)/60, 1)
		inner := -1.0
		if theta < 0 {
			inner = 1
		}
		out := zeroOffsets(g)
		for i, p := range g.Pieces {
			for j, v := range p.V {
				if (v[0]-g.Anchor[0])*inner > 0 && v[1]-g.Anchor[1] < g.SlotH*0.35 {
					out[i][j] = pt{0, -amt}
				}
			}
		}
		return out
	}
}

// followGen lags the vertices behind the parent's rotation, tip more
// than root.
func followGen(lag, weight float64) vertexGen {
	return func(c *clipCtx, layer string, g *partGeom, f float64) [][]pt {
		parent := c.parents[layer]
		if parent == "" {
			return nil
		}
		prev := f - lag
		if prev < 0 {
			if c.op > 0 {
				prev += c.op
			} else {
				prev = 0
			}
		}
		d := normRad(c.placement(parent, prev).Angle-c.placement(parent, f).Angle) * weight
		return rotateOffsets(g, d, 0)
	}
}

// drapeGen rotates the cloth about its pivot by a blend of the drivers'
// angles relative to the host: near vertices follow the first driver,
// far vertices the second.
func drapeGen(drivers []string, weight float64) vertexGen {
	return func(c *clipCtx, layer string, g *partGeom, f float64) [][]pt {
		host := c.parents[layer]
		if host == "" || len(drivers) == 0 {
			return nil
		}
		hostAngle := c.placement(host, f).Angle
		var angles []float64
		for _, d := range drivers {
			name := d
			if _, ok := c.layers[name]; !ok {
				name = d + "-near"
				if _, ok := c.layers[name]; !ok {
					continue
				}
			}
			angles = append(angles, normRad(c.placement(name, f).Angle-hostAngle)*weight)
		}
		if len(angles) == 0 {
			return nil
		}
		minX, maxX := math.Inf(1), math.Inf(-1)
		for _, p := range g.Pieces {
			for _, v := range p.V {
				minX, maxX = math.Min(minX, v[0]), math.Max(maxX, v[0])
			}
		}
		out := zeroOffsets(g)
		for i, p := range g.Pieces {
			for j, v := range p.V {
				t := 0.0
				if maxX > minX {
					t = (v[0] - minX) / (maxX - minX)
				}
				a := angles[0]
				if len(angles) > 1 {
					a = angles[0]*(1-t) + angles[1]*t
				}
				depth := math.Max(0, math.Min(1, (v[1]-g.Anchor[1])/math.Max(g.SlotH, 1)))
				out[i][j] = rotateAbout(v, g.Anchor, a*depth)
			}
		}
		return out
	}
}

// rotateOffsets rotates every vertex about the anchor by angle scaled by
// its distance from the anchor (0 at the root, full at the farthest).
func rotateOffsets(g *partGeom, angle, minWeight float64) [][]pt {
	far := 0.0
	for _, p := range g.Pieces {
		for _, v := range p.V {
			far = math.Max(far, dist(v, g.Anchor))
		}
	}
	out := zeroOffsets(g)
	if far == 0 {
		return out
	}
	for i, p := range g.Pieces {
		for j, v := range p.V {
			w := minWeight + (1-minWeight)*dist(v, g.Anchor)/far
			out[i][j] = rotateAbout(v, g.Anchor, angle*w)
		}
	}
	return out
}

func rotateAbout(v, o pt, a float64) pt {
	dx, dy := v[0]-o[0], v[1]-o[1]
	cs, sn := math.Cos(a), math.Sin(a)
	return pt{dx*cs - dy*sn - dx, dx*sn + dy*cs - dy}
}

// ---- rotation generators ----

func swayGen(amount, period float64) rotGen {
	return func(c *clipCtx, layer string, f float64) float64 {
		return amount * math.Sin(2*math.Pi*f/period)
	}
}

// swingGen simulates a damped pendulum hung from the layer's pivot as it
// travels through the clip, and returns the local rotation that keeps it
// hanging (concept:attachment-kinds swing).
func (m *morpher) swingGen(al *attLayer) rotGen {
	type sim struct{ angles []float64 }
	cache := map[string]*sim{}
	return func(c *clipCtx, layer string, f float64) float64 {
		s, ok := cache[c.id]
		if !ok {
			s = &sim{angles: m.simulateSwing(c, layer, al)}
			cache[c.id] = s
		}
		i := int(math.Round(f))
		if i < 0 {
			i = 0
		}
		if i >= len(s.angles) {
			i = len(s.angles) - 1
		}
		if i < 0 {
			return 0
		}
		return s.angles[i]
	}
}

func (m *morpher) simulateSwing(c *clipCtx, layer string, al *attLayer) []float64 {
	n := int(c.op) + 1
	if n < 2 || c.fps <= 0 {
		return nil
	}
	pos := make([]pt, n)
	phi := make([]float64, n)
	for i := 0; i < n; i++ {
		p := c.placement(layer, float64(i))
		pos[i] = pt{p.X, p.Y}
		phi[i] = p.Angle
	}
	L := math.Max(al.h, 8)
	const g = 1500.0
	damp := (1 - al.att.Damping) * 8
	spring := al.att.Stiffness * 0.5 * g / L
	dt := 1 / c.fps
	const sub = 4
	theta, omega := phi[0], 0.0
	local := make([]float64, n)
	passes := 2
	for pass := 0; pass < passes; pass++ {
		for i := 0; i < n; i++ {
			prev, next := max(i-1, 0), min(i+1, n-1)
			ax := (pos[next][0] - 2*pos[i][0] + pos[prev][0]) / (dt * dt)
			ay := (pos[next][1] - 2*pos[i][1] + pos[prev][1]) / (dt * dt)
			ax, ay = clampAbs(ax, 6*g), clampAbs(ay, 6*g)
			h := dt / sub
			for k := 0; k < sub; k++ {
				acc := (ax*math.Cos(theta)-(g-ay)*math.Sin(theta))/L - damp*omega - spring*normRad(theta-phi[i])
				omega += acc * h
				theta += omega * h
			}
			local[i] = deg(normRad(theta - phi[i]))
		}
	}
	// Close the loop: ease the tail into the head.
	const fade = 6
	if n > fade*2 {
		for i := n - fade; i < n; i++ {
			t := float64(i-(n-fade)) / fade
			local[i] = local[i]*(1-t) + local[0]*t
		}
	}
	if al.segment == 2 {
		// The second segment lags the first by two frames.
		lagged := make([]float64, n)
		for i := range local {
			lagged[i] = local[max(i-2, 0)]
		}
		local = lagged
	}
	return local
}

func clampAbs(v, lim float64) float64 {
	if v > lim {
		return lim
	}
	if v < -lim {
		return -lim
	}
	return v
}

func deg(r float64) float64 { return r * 180 / math.Pi }

func normRad(a float64) float64 {
	for a > math.Pi {
		a -= 2 * math.Pi
	}
	for a < -math.Pi {
		a += 2 * math.Pi
	}
	return a
}

func normDeg(a float64) float64 {
	for a > 180 {
		a -= 360
	}
	for a < -180 {
		a += 360
	}
	return a
}
