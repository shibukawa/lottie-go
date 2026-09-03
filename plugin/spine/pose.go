package lottiespine

import (
	"encoding/json"
	"math"
	"sort"
)

// The skeleton runtime the importer bakes from: bones with Spine's inherit
// modes, IK and transform constraints, and the timelines of one animation
// evaluated at a time. It is a converter's pose evaluator, not a player —
// no mixing, no events queue, no physics — written from the documented
// semantics of the format.

const degRad = math.Pi / 180

type bone struct {
	data     *BoneData
	index    int
	parent   *bone
	children []*bone
	inherit  string

	// The local pose the timelines write.
	x, y, rotation, scaleX, scaleY, shearX, shearY float64
	// The applied pose: the local pose as constrained so far this update.
	ax, ay, arotation, ascaleX, ascaleY, ashearX, ashearY float64
	// The world transform.
	a, b, c, d, worldX, worldY float64
}

type slot struct {
	data       *SlotData
	index      int
	bone       *bone
	setupColor [4]float64
	color      [4]float64
	attachment string
}

type ikConstraint struct {
	data     *IKData
	bones    []*bone
	target   *bone
	mix      float64
	softness float64
	bendDir  int
	compress bool
	stretch  bool
}

type transformConstraint struct {
	data   *TransformData
	bones  []*bone
	target *bone
	mix    transformMix
}

type updateStep struct {
	bone *bone
	ik   *ikConstraint
	tr   *transformConstraint
}

type deformKey struct{ slot, attachment string }

// pose is a skeleton posed by one animation at one time.
type pose struct {
	sk        *Skeleton
	bones     []*bone
	boneByNm  map[string]*bone
	slots     []*slot
	slotByNm  map[string]*slot
	iks       []*ikConstraint
	trs       []*transformConstraint
	ikByNm    map[string]*ikConstraint
	trByNm    map[string]*transformConstraint
	steps     []updateStep
	deforms   map[deformKey][]float64
	legacy    bool // 3.x normalized curves
	skinNames map[string]bool
	notes     *noteSet
}

func newPose(sk *Skeleton, skins map[string]bool, notes *noteSet) *pose {
	p := &pose{
		sk:        sk,
		boneByNm:  map[string]*bone{},
		slotByNm:  map[string]*slot{},
		ikByNm:    map[string]*ikConstraint{},
		trByNm:    map[string]*transformConstraint{},
		deforms:   map[deformKey][]float64{},
		legacy:    sk.Info.Major() < 4,
		skinNames: skins,
		notes:     notes,
	}
	for i := range sk.Bones {
		bd := &sk.Bones[i]
		b := &bone{data: bd, index: i, inherit: bd.inherit()}
		if bd.Parent != "" {
			if parent, ok := p.boneByNm[bd.Parent]; ok {
				b.parent = parent
				parent.children = append(parent.children, b)
			} else {
				notes.addf("bone %q names unknown parent %q; treated as a root", bd.Name, bd.Parent)
			}
		}
		p.bones = append(p.bones, b)
		p.boneByNm[bd.Name] = b
	}
	for i := range sk.Slots {
		sd := &sk.Slots[i]
		s := &slot{data: sd, index: i, bone: p.boneByNm[sd.Bone]}
		if s.bone == nil {
			notes.addf("slot %q names unknown bone %q; attached to the root", sd.Name, sd.Bone)
			s.bone = p.bones[0]
		}
		s.setupColor = parseColor(sd.Color)
		p.slots = append(p.slots, s)
		p.slotByNm[sd.Name] = s
	}
	for i := range sk.IK {
		d := &sk.IK[i]
		c := &ikConstraint{data: d, target: p.boneByNm[d.Target]}
		for _, name := range d.Bones {
			if b := p.boneByNm[name]; b != nil {
				c.bones = append(c.bones, b)
			}
		}
		if c.target == nil || len(c.bones) == 0 || len(c.bones) > 2 {
			notes.addf("ik constraint %q skipped: needs a target and one or two bones", d.Name)
			continue
		}
		p.iks = append(p.iks, c)
		p.ikByNm[d.Name] = c
	}
	for i := range sk.Transform {
		d := &sk.Transform[i]
		c := &transformConstraint{data: d, target: p.boneByNm[d.Target]}
		for _, name := range d.Bones {
			if b := p.boneByNm[name]; b != nil {
				c.bones = append(c.bones, b)
			}
		}
		if c.target == nil || len(c.bones) == 0 {
			notes.addf("transform constraint %q skipped: needs a target and bones", d.Name)
			continue
		}
		p.trs = append(p.trs, c)
		p.trByNm[d.Name] = c
	}
	if len(sk.Path) > 0 {
		notes.addf("%d path constraint(s) ignored: the constrained bones keep their keyed pose", len(sk.Path))
	}
	if len(sk.Physics) > 0 {
		notes.addf("%d physics constraint(s) ignored: the constrained bones keep their keyed pose", len(sk.Physics))
	}
	p.buildUpdateOrder()
	p.setToSetupPose()
	return p
}

// buildUpdateOrder lays out one update: every bone after its parent, every
// constraint after its target and the parents of its bones, and the
// children of a constrained bone after the constraint.
func (p *pose) buildUpdateOrder() {
	sorted := make([]bool, len(p.bones))
	var steps []updateStep
	var sortBone func(b *bone)
	sortBone = func(b *bone) {
		if sorted[b.index] {
			return
		}
		if b.parent != nil {
			sortBone(b.parent)
		}
		sorted[b.index] = true
		steps = append(steps, updateStep{bone: b})
	}
	var sortReset func(bs []*bone)
	sortReset = func(bs []*bone) {
		for _, b := range bs {
			if sorted[b.index] {
				sortReset(b.children)
			}
			sorted[b.index] = false
		}
	}
	type ordered struct {
		order int
		seq   int
		ik    *ikConstraint
		tr    *transformConstraint
	}
	var cs []ordered
	for i, c := range p.iks {
		cs = append(cs, ordered{order: c.data.Order, seq: i, ik: c})
	}
	for i, c := range p.trs {
		cs = append(cs, ordered{order: c.data.Order, seq: len(p.iks) + i, tr: c})
	}
	sort.SliceStable(cs, func(i, j int) bool {
		if cs[i].order != cs[j].order {
			return cs[i].order < cs[j].order
		}
		return cs[i].seq < cs[j].seq
	})
	for _, c := range cs {
		if ik := c.ik; ik != nil {
			sortBone(ik.target)
			parent := ik.bones[0]
			sortBone(parent)
			if len(ik.bones) == 1 {
				steps = append(steps, updateStep{ik: ik})
				sortReset(parent.children)
			} else {
				child := ik.bones[1]
				sortBone(child)
				steps = append(steps, updateStep{ik: ik})
				sortReset(parent.children)
				sorted[child.index] = true
			}
			continue
		}
		tr := c.tr
		sortBone(tr.target)
		for _, b := range tr.bones {
			sortBone(b)
		}
		steps = append(steps, updateStep{tr: tr})
		for _, b := range tr.bones {
			sortReset(b.children)
		}
		for _, b := range tr.bones {
			sorted[b.index] = true
		}
	}
	for _, b := range p.bones {
		sortBone(b)
	}
	p.steps = steps
}

// setToSetupPose resets every bone, slot and constraint to the skeleton's
// setup values and clears the deforms.
func (p *pose) setToSetupPose() {
	for _, b := range p.bones {
		d := b.data
		b.x, b.y, b.rotation = d.X, d.Y, d.Rotation
		b.scaleX, b.scaleY = d.scale()
		b.shearX, b.shearY = d.ShearX, d.ShearY
		b.inherit = d.inherit()
	}
	for _, s := range p.slots {
		s.color = s.setupColor
		s.attachment = s.data.Attachment
	}
	for _, c := range p.iks {
		c.mix = ptrOr(c.data.Mix, 1)
		c.softness = c.data.Softness
		c.bendDir = 1
		if !boolOr(c.data.BendPositive, true) {
			c.bendDir = -1
		}
		c.compress, c.stretch = c.data.Compress, c.data.Stretch
	}
	for _, c := range p.trs {
		c.mix = c.data.mixes()
	}
	for k := range p.deforms {
		delete(p.deforms, k)
	}
}

// ---- world transforms ----

func cosDeg(deg float64) float64    { return math.Cos(deg * degRad) }
func sinDeg(deg float64) float64    { return math.Sin(deg * degRad) }
func atan2Deg(y, x float64) float64 { return math.Atan2(y, x) / degRad }

// wrapDeg normalizes an angle to (-180, 180].
func wrapDeg(r float64) float64 { return r - 360*math.Round(r/360) }

// wrapRad normalizes an angle to (-π, π].
func wrapRad(r float64) float64 { return r - 2*math.Pi*math.Round(r/(2*math.Pi)) }

func (b *bone) localToWorld(x, y float64) (float64, float64) {
	return b.a*x + b.b*y + b.worldX, b.c*x + b.d*y + b.worldY
}

// updateWorldTransform recomputes the world transform from the applied
// pose and the parent's world transform.
func (b *bone) updateWorldTransform() {
	b.updateWorldTransformWith(b.ax, b.ay, b.arotation, b.ascaleX, b.ascaleY, b.ashearX, b.ashearY)
}

// updateWorldTransformWith computes the world transform from the given
// local pose, honoring the bone's inherit mode, and records that pose as the
// applied one.
func (b *bone) updateWorldTransformWith(x, y, rotation, scaleX, scaleY, shearX, shearY float64) {
	b.ax, b.ay, b.arotation, b.ascaleX, b.ascaleY, b.ashearX, b.ashearY = x, y, rotation, scaleX, scaleY, shearX, shearY
	p := b.parent
	if p == nil {
		rotY := rotation + 90 + shearY
		b.a = cosDeg(rotation+shearX) * scaleX
		b.b = cosDeg(rotY) * scaleY
		b.c = sinDeg(rotation+shearX) * scaleX
		b.d = sinDeg(rotY) * scaleY
		b.worldX, b.worldY = x, y
		return
	}
	pa, pb, pc, pd := p.a, p.b, p.c, p.d
	b.worldX = pa*x + pb*y + p.worldX
	b.worldY = pc*x + pd*y + p.worldY
	switch b.inherit {
	case "onlyTranslation":
		rotY := rotation + 90 + shearY
		b.a = cosDeg(rotation+shearX) * scaleX
		b.b = cosDeg(rotY) * scaleY
		b.c = sinDeg(rotation+shearX) * scaleX
		b.d = sinDeg(rotY) * scaleY
	case "noRotationOrReflection":
		s := pa*pa + pc*pc
		var prx float64
		if s > 0.0001 {
			s = math.Abs(pa*pd-pb*pc) / s
			pb = pc * s
			pd = pa * s
			prx = atan2Deg(pc, pa)
		} else {
			pa, pc = 0, 0
			prx = 90 - atan2Deg(pd, pb)
		}
		rx := rotation + shearX - prx
		ry := rotation + shearY - prx + 90
		la, lb := cosDeg(rx)*scaleX, cosDeg(ry)*scaleY
		lc, ld := sinDeg(rx)*scaleX, sinDeg(ry)*scaleY
		b.a = pa*la - pb*lc
		b.b = pa*lb - pb*ld
		b.c = pc*la + pd*lc
		b.d = pc*lb + pd*ld
	case "noScale", "noScaleOrReflection":
		cos, sin := cosDeg(rotation), sinDeg(rotation)
		za := pa*cos + pb*sin
		zc := pc*cos + pd*sin
		s := math.Sqrt(za*za + zc*zc)
		if s > 0.00001 {
			s = 1 / s
		}
		za *= s
		zc *= s
		s = math.Sqrt(za*za + zc*zc)
		if b.inherit == "noScale" && pa*pd-pb*pc < 0 {
			s = -s
		}
		r := math.Pi/2 + math.Atan2(zc, za)
		zb, zd := math.Cos(r)*s, math.Sin(r)*s
		la, lb := cosDeg(shearX)*scaleX, cosDeg(90+shearY)*scaleY
		lc, ld := sinDeg(shearX)*scaleX, sinDeg(90+shearY)*scaleY
		b.a = za*la + zb*lc
		b.b = za*lb + zb*ld
		b.c = zc*la + zd*lc
		b.d = zc*lb + zd*ld
	default: // normal
		rotY := rotation + 90 + shearY
		la, lb := cosDeg(rotation+shearX)*scaleX, cosDeg(rotY)*scaleY
		lc, ld := sinDeg(rotation+shearX)*scaleX, sinDeg(rotY)*scaleY
		b.a = pa*la + pb*lc
		b.b = pa*lb + pb*ld
		b.c = pc*la + pd*lc
		b.d = pc*lb + pd*ld
	}
}

// updateAppliedTransform derives the applied pose back from the world
// transform, after a constraint has edited the latter directly.
func (b *bone) updateAppliedTransform() {
	p := b.parent
	if p == nil {
		b.ax, b.ay = b.worldX, b.worldY
		b.arotation = atan2Deg(b.c, b.a)
		b.ascaleX = math.Hypot(b.a, b.c)
		b.ascaleY = math.Hypot(b.b, b.d)
		b.ashearX = 0
		b.ashearY = atan2Deg(b.a*b.b+b.c*b.d, b.a*b.d-b.b*b.c)
		return
	}
	pa, pb, pc, pd := p.a, p.b, p.c, p.d
	pid := 1 / (pa*pd - pb*pc)
	ia, ib, ic, id := pd*pid, pb*pid, pc*pid, pa*pid
	dx, dy := b.worldX-p.worldX, b.worldY-p.worldY
	b.ax = dx*ia - dy*ib
	b.ay = dy*id - dx*ic
	var ra, rb, rc, rd float64
	switch b.inherit {
	case "onlyTranslation":
		ra, rb, rc, rd = b.a, b.b, b.c, b.d
	case "noRotationOrReflection":
		s := math.Abs(pa*pd-pb*pc) / (pa*pa + pc*pc)
		sa, sc := pa, pc
		pb, pd = -sc*s, sa*s
		pid = 1 / (pa*pd - pb*pc)
		ia, ib = pd*pid, pb*pid
		ra = ia*b.a - ib*b.c
		rb = ia*b.b - ib*b.d
		rc = id*b.c - ic*b.a
		rd = id*b.d - ic*b.b
	case "noScale", "noScaleOrReflection":
		cos, sin := cosDeg(b.rotation), sinDeg(b.rotation)
		za, zc := pa*cos+pb*sin, pc*cos+pd*sin
		s := math.Hypot(za, zc)
		if s > 0.00001 {
			s = 1 / s
		}
		za *= s
		zc *= s
		s = math.Hypot(za, zc)
		r := math.Pi/2 + math.Atan2(zc, za)
		zb, zd := math.Cos(r)*s, math.Sin(r)*s
		pid = 1 / (za*zd - zb*zc)
		ia, ib, ic, id = zd*pid, zb*pid, zc*pid, za*pid
		ra = ia*b.a - ib*b.c
		rb = ia*b.b - ib*b.d
		rc = id*b.c - ic*b.a
		rd = id*b.d - ic*b.b
	default:
		ra = ia*b.a - ib*b.c
		rb = ia*b.b - ib*b.d
		rc = id*b.c - ic*b.a
		rd = id*b.d - ic*b.b
	}
	b.ashearX = 0
	b.ascaleX = math.Hypot(ra, rc)
	if b.ascaleX > 0.0001 {
		det := ra*rd - rb*rc
		b.ascaleY = det / b.ascaleX
		b.ashearY = -atan2Deg(ra*rb+rc*rd, det)
		b.arotation = atan2Deg(rc, ra)
	} else {
		b.ascaleX = 0
		b.ascaleY = math.Hypot(rb, rd)
		b.ashearY = 0
		b.arotation = 90 - atan2Deg(rd, rb)
	}
}

// update runs one update: the local pose becomes the applied pose, then
// bones and constraints run in their sorted order.
func (p *pose) update() {
	for _, b := range p.bones {
		b.ax, b.ay, b.arotation = b.x, b.y, b.rotation
		b.ascaleX, b.ascaleY = b.scaleX, b.scaleY
		b.ashearX, b.ashearY = b.shearX, b.shearY
	}
	for _, st := range p.steps {
		switch {
		case st.bone != nil:
			st.bone.updateWorldTransform()
		case st.ik != nil:
			st.ik.apply()
		case st.tr != nil:
			st.tr.apply()
		}
	}
}

// ---- IK ----

func (c *ikConstraint) apply() {
	if c.mix == 0 {
		return
	}
	t := c.target
	switch len(c.bones) {
	case 1:
		ikApply1(c.bones[0], t.worldX, t.worldY, c.compress, c.stretch, c.data.Uniform, c.mix)
	case 2:
		ikApply2(c.bones[0], c.bones[1], t.worldX, t.worldY, c.bendDir, c.stretch, c.data.Uniform, c.softness, c.mix)
	}
}

// ikApply1 rotates one bone toward the target, optionally scaling it along
// its length to reach.
func ikApply1(b *bone, targetX, targetY float64, compress, stretch, uniform bool, alpha float64) {
	p := b.parent
	var tx, ty float64
	rotationIK := -b.ashearX - b.arotation
	if p == nil {
		tx, ty = targetX-b.worldX, targetY-b.worldY
	} else {
		pa, pb, pc, pd := p.a, p.b, p.c, p.d
		switch b.inherit {
		case "onlyTranslation":
			tx, ty = targetX-b.worldX, targetY-b.worldY
		default:
			if b.inherit == "noRotationOrReflection" {
				s := math.Abs(pa*pd-pb*pc) / math.Max(0.0001, pa*pa+pc*pc)
				sa, sc := pa, pc
				pb, pd = -sc*s, sa*s
				rotationIK += atan2Deg(sc, sa)
			}
			x, y := targetX-p.worldX, targetY-p.worldY
			d := pa*pd - pb*pc
			if math.Abs(d) <= 0.0001 {
				tx, ty = 0, 0
			} else {
				tx = (x*pd-y*pb)/d - b.ax
				ty = (y*pa-x*pc)/d - b.ay
			}
		}
	}
	rotationIK += atan2Deg(ty, tx)
	if b.ascaleX < 0 {
		rotationIK += 180
	}
	rotationIK = wrapDeg(rotationIK)
	sx, sy := b.ascaleX, b.ascaleY
	if compress || stretch {
		if b.inherit == "noScale" || b.inherit == "noScaleOrReflection" {
			tx, ty = targetX-b.worldX, targetY-b.worldY
		}
		if l := b.data.Length * sx; l > 0.0001 {
			dd := tx*tx + ty*ty
			if (compress && dd < l*l) || (stretch && dd > l*l) {
				s := (math.Sqrt(dd)/l-1)*alpha + 1
				sx *= s
				if uniform {
					sy *= s
				}
			}
		}
	}
	b.updateWorldTransformWith(b.ax, b.ay, b.arotation+rotationIK*alpha, sx, sy, b.ashearX, b.ashearY)
}

// ikApply2 poses a parent and child bone so the child's tip reaches the
// target: the two-bone law-of-cosines solution, bent to bendDir, softened
// near full extension, and stretched past it when allowed.
func ikApply2(parent, child *bone, targetX, targetY float64, bendDir int, stretch, uniform bool, softness, alpha float64) {
	if parent.inherit != "normal" || child.inherit != "normal" {
		return
	}
	px, py := parent.ax, parent.ay
	psx, psy := parent.ascaleX, parent.ascaleY
	sx, sy := psx, psy
	csx := child.ascaleX
	var os1, os2 float64
	s2 := 1.0
	if psx < 0 {
		psx = -psx
		os1 = 180
		s2 = -1
	}
	if psy < 0 {
		psy = -psy
		s2 = -s2
	}
	if csx < 0 {
		csx = -csx
		os2 = 180
	}
	cx := child.ax
	var cy, cwx, cwy float64
	pp := parent.parent
	var a, b, c, d, ppx, ppy float64
	if pp != nil {
		a, b, c, d, ppx, ppy = pp.a, pp.b, pp.c, pp.d, pp.worldX, pp.worldY
	} else {
		a, d = 1, 1
	}
	u := math.Abs(psx-psy) <= 0.0001
	if !u || stretch {
		cy = 0
		cwx = a*cx + ppx
		cwy = c*cx + ppy
	} else {
		cy = child.ay
		cwx = a*cx + b*cy + ppx
		cwy = c*cx + d*cy + ppy
	}
	id := a*d - b*c
	if math.Abs(id) <= 0.0001 {
		id = 0
	} else {
		id = 1 / id
	}
	x, y := cwx-ppx, cwy-ppy
	dx := (x*d-y*b)*id - px
	dy := (y*a-x*c)*id - py
	l1 := math.Hypot(dx, dy)
	l2 := child.data.Length * csx
	if l1 < 0.0001 {
		ikApply1(parent, targetX, targetY, false, stretch, false, alpha)
		child.updateWorldTransformWith(cx, cy, 0, child.ascaleX, child.ascaleY, child.ashearX, child.ashearY)
		return
	}
	x, y = targetX-ppx, targetY-ppy
	tx := (x*d-y*b)*id - px
	ty := (y*a-x*c)*id - py
	dd := tx*tx + ty*ty
	if softness != 0 {
		softness *= psx * (csx + 1) * 0.5
		td := math.Sqrt(dd)
		sd := td - l1 - l2*psx + softness
		if sd > 0 {
			p := math.Min(1, sd/(softness*2)) - 1
			p = (sd - softness*(1-p*p)) / td
			tx -= p * tx
			ty -= p * ty
			dd = tx*tx + ty*ty
		}
	}
	var a1, a2 float64
	solved := false
	if u {
		l2 *= psx
		cos := (dd - l1*l1 - l2*l2) / (2 * l1 * l2)
		switch {
		case cos < -1:
			cos = -1
			a2 = math.Pi * float64(bendDir)
		case cos > 1:
			cos = 1
			a2 = 0
			if stretch {
				s := (math.Sqrt(dd)/(l1+l2)-1)*alpha + 1
				sx *= s
				if uniform {
					sy *= s
				}
			}
		default:
			a2 = math.Acos(cos) * float64(bendDir)
		}
		a, b = l1+l2*cos, l2*math.Sin(a2)
		a1 = math.Atan2(ty*a-tx*b, tx*a+ty*b)
		solved = true
	} else {
		a, b = psx*l2, psy*l2
		aa, bb := a*a, b*b
		ta := math.Atan2(ty, tx)
		c = bb*l1*l1 + aa*dd - aa*bb
		c1 := -2 * bb * l1
		c2 := bb - aa
		d = c1*c1 - 4*c2*c
		if d >= 0 {
			q := math.Sqrt(d)
			if c1 < 0 {
				q = -q
			}
			q = -(c1 + q) / 2
			r0, r1 := q/c2, c/q
			r := r1
			if math.Abs(r0) < math.Abs(r1) {
				r = r0
			}
			if r0 = dd - r*r; r0 >= 0 {
				y = math.Sqrt(r0) * float64(bendDir)
				a1 = ta - math.Atan2(y, r)
				a2 = math.Atan2(y/psy, (r-l1)/psx)
				solved = true
			}
		}
		if !solved {
			minAngle, minX, minY := math.Pi, l1-a, 0.0
			minDist := minX * minX
			maxAngle, maxX, maxY := 0.0, l1+a, 0.0
			maxDist := maxX * maxX
			c = -a * l1 / (aa - bb)
			if c >= -1 && c <= 1 {
				c = math.Acos(c)
				x = a*math.Cos(c) + l1
				y = b * math.Sin(c)
				d = x*x + y*y
				if d < minDist {
					minAngle, minDist, minX, minY = c, d, x, y
				}
				if d > maxDist {
					maxAngle, maxDist, maxX, maxY = c, d, x, y
				}
			}
			if dd <= (minDist+maxDist)/2 {
				a1 = ta - math.Atan2(minY*float64(bendDir), minX)
				a2 = minAngle * float64(bendDir)
			} else {
				a1 = ta - math.Atan2(maxY*float64(bendDir), maxX)
				a2 = maxAngle * float64(bendDir)
			}
		}
	}
	os := math.Atan2(cy, cx) * s2
	rotation := parent.arotation
	a1 = wrapDeg((a1-os)/degRad + os1 - rotation)
	parent.updateWorldTransformWith(px, py, rotation+a1*alpha, sx, sy, 0, 0)
	rotation = child.arotation
	a2 = wrapDeg(((a2+os)/degRad-child.ashearX)*s2 + os2 - rotation)
	child.updateWorldTransformWith(cx, cy, rotation+a2*alpha, child.ascaleX, child.ascaleY, child.ashearX, child.ashearY)
}

// ---- transform constraints ----

func (c *transformConstraint) apply() {
	m := c.mix
	if m.rotate == 0 && m.x == 0 && m.y == 0 && m.scaleX == 0 && m.scaleY == 0 && m.shearY == 0 {
		return
	}
	switch {
	case c.data.Local && c.data.Relative:
		c.applyRelativeLocal()
	case c.data.Local:
		c.applyAbsoluteLocal()
	case c.data.Relative:
		c.applyRelativeWorld()
	default:
		c.applyAbsoluteWorld()
	}
}

func (c *transformConstraint) worldOffsets() (offsetRotation, offsetShearY float64) {
	t := c.target
	sign := 1.0
	if t.a*t.d-t.b*t.c < 0 {
		sign = -1
	}
	return c.data.Rotation * degRad * sign, c.data.ShearY * degRad * sign
}

func rotateWorld(b *bone, r float64) {
	cos, sin := math.Cos(r), math.Sin(r)
	a, bb, cc, d := b.a, b.b, b.c, b.d
	b.a = cos*a - sin*cc
	b.b = cos*bb - sin*d
	b.c = sin*a + cos*cc
	b.d = sin*bb + cos*d
}

func (c *transformConstraint) applyAbsoluteWorld() {
	m, t, d := c.mix, c.target, c.data
	offsetRotation, offsetShearY := c.worldOffsets()
	ta, tb, tc, td := t.a, t.b, t.c, t.d
	for _, b := range c.bones {
		if m.rotate != 0 {
			r := math.Atan2(tc, ta) - math.Atan2(b.c, b.a) + offsetRotation
			rotateWorld(b, wrapRad(r)*m.rotate)
		}
		if m.x != 0 || m.y != 0 {
			tx, ty := t.localToWorld(d.X, d.Y)
			b.worldX += (tx - b.worldX) * m.x
			b.worldY += (ty - b.worldY) * m.y
		}
		if m.scaleX != 0 {
			if s := math.Hypot(b.a, b.c); s != 0 {
				s = (s + (math.Hypot(ta, tc)-s+d.ScaleX)*m.scaleX) / s
				b.a *= s
				b.c *= s
			}
		}
		if m.scaleY != 0 {
			if s := math.Hypot(b.b, b.d); s != 0 {
				s = (s + (math.Hypot(tb, td)-s+d.ScaleY)*m.scaleY) / s
				b.b *= s
				b.d *= s
			}
		}
		if m.shearY > 0 {
			by := math.Atan2(b.d, b.b)
			r := math.Atan2(td, tb) - math.Atan2(tc, ta) - (by - math.Atan2(b.c, b.a))
			r = by + (wrapRad(r)+offsetShearY)*m.shearY
			s := math.Hypot(b.b, b.d)
			b.b = math.Cos(r) * s
			b.d = math.Sin(r) * s
		}
		b.updateAppliedTransform()
	}
}

func (c *transformConstraint) applyRelativeWorld() {
	m, t, d := c.mix, c.target, c.data
	offsetRotation, offsetShearY := c.worldOffsets()
	ta, tb, tc, td := t.a, t.b, t.c, t.d
	for _, b := range c.bones {
		if m.rotate != 0 {
			r := math.Atan2(tc, ta) + offsetRotation
			rotateWorld(b, wrapRad(r)*m.rotate)
		}
		if m.x != 0 || m.y != 0 {
			tx, ty := t.localToWorld(d.X, d.Y)
			b.worldX += tx * m.x
			b.worldY += ty * m.y
		}
		if m.scaleX != 0 {
			s := (math.Hypot(ta, tc)-1+d.ScaleX)*m.scaleX + 1
			b.a *= s
			b.c *= s
		}
		if m.scaleY != 0 {
			s := (math.Hypot(tb, td)-1+d.ScaleY)*m.scaleY + 1
			b.b *= s
			b.d *= s
		}
		if m.shearY > 0 {
			r := wrapRad(math.Atan2(td, tb) - math.Atan2(tc, ta))
			r = math.Atan2(b.d, b.b) + (r-math.Pi/2+offsetShearY)*m.shearY
			s := math.Hypot(b.b, b.d)
			b.b = math.Cos(r) * s
			b.d = math.Sin(r) * s
		}
		b.updateAppliedTransform()
	}
}

func (c *transformConstraint) applyAbsoluteLocal() {
	m, t, d := c.mix, c.target, c.data
	t.updateAppliedTransform()
	for _, b := range c.bones {
		rotation := b.arotation
		if m.rotate != 0 {
			rotation += wrapDeg(t.arotation-rotation+d.Rotation) * m.rotate
		}
		x, y := b.ax, b.ay
		x += (t.ax - x + d.X) * m.x
		y += (t.ay - y + d.Y) * m.y
		scaleX, scaleY := b.ascaleX, b.ascaleY
		if m.scaleX != 0 {
			scaleX += (t.ascaleX - scaleX + d.ScaleX) * m.scaleX
		}
		if m.scaleY != 0 {
			scaleY += (t.ascaleY - scaleY + d.ScaleY) * m.scaleY
		}
		shearY := b.ashearY
		if m.shearY != 0 {
			shearY += wrapDeg(t.ashearY-shearY+d.ShearY) * m.shearY
		}
		b.updateWorldTransformWith(x, y, rotation, scaleX, scaleY, b.ashearX, shearY)
	}
}

func (c *transformConstraint) applyRelativeLocal() {
	m, t, d := c.mix, c.target, c.data
	t.updateAppliedTransform()
	for _, b := range c.bones {
		rotation := b.arotation + (t.arotation+d.Rotation)*m.rotate
		x := b.ax + (t.ax+d.X)*m.x
		y := b.ay + (t.ay+d.Y)*m.y
		scaleX := b.ascaleX * ((t.ascaleX-1+d.ScaleX)*m.scaleX + 1)
		scaleY := b.ascaleY * ((t.ascaleY-1+d.ScaleY)*m.scaleY + 1)
		shearY := b.ashearY + (t.ashearY+d.ShearY)*m.shearY
		b.updateWorldTransformWith(x, y, rotation, scaleX, scaleY, b.ashearX, shearY)
	}
}

// ---- timelines ----

// frameAt returns the index of the key at or before t, or -1 before the
// first key.
func frameAt(keys []Key, t float64) int {
	i := sort.Search(len(keys), func(i int) bool { return keys[i].Time > t })
	return i - 1
}

// curveSpec is one key's interpolation to the next.
type curveSpec struct {
	stepped bool
	pts     []float64 // absolute cx1 cy1 cx2 cy2 per component (4.x), or one normalized quad (3.x)
	legacy  bool
}

func (p *pose) curveOf(k *Key) curveSpec {
	raw := k.Curve
	if len(raw) == 0 {
		return curveSpec{}
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return curveSpec{stepped: s == "stepped"}
	}
	var pts []float64
	if json.Unmarshal(raw, &pts) == nil {
		return curveSpec{pts: pts, legacy: p.legacy}
	}
	var c1 float64
	if json.Unmarshal(raw, &c1) == nil {
		return curveSpec{pts: []float64{c1, ptrOr(k.C2, 0), ptrOr(k.C3, 1), ptrOr(k.C4, 1)}, legacy: true}
	}
	return curveSpec{}
}

// interp evaluates component comp between keys i and i+1 at time t, from
// value a to value b.
func (p *pose) interp(keys []Key, i int, t float64, comp int, a, b float64) float64 {
	if i+1 >= len(keys) {
		return a
	}
	k := &keys[i]
	t0, t1 := k.Time, keys[i+1].Time
	if t1 <= t0 {
		return a
	}
	cs := p.curveOf(k)
	if cs.stepped {
		return a
	}
	var cx1, cy1, cx2, cy2 float64
	switch {
	case cs.pts == nil:
		return a + (b-a)*(t-t0)/(t1-t0)
	case !cs.legacy && len(cs.pts) >= 4*(comp+1):
		cx1, cy1, cx2, cy2 = cs.pts[4*comp], cs.pts[4*comp+1], cs.pts[4*comp+2], cs.pts[4*comp+3]
	case len(cs.pts) >= 4:
		cx1 = t0 + cs.pts[0]*(t1-t0)
		cy1 = a + cs.pts[1]*(b-a)
		cx2 = t0 + cs.pts[2]*(t1-t0)
		cy2 = a + cs.pts[3]*(b-a)
	default:
		return a + (b-a)*(t-t0)/(t1-t0)
	}
	return bezierY(t0, a, cx1, cy1, cx2, cy2, t1, b, t)
}

// bezierY solves the cubic (x0,y0) (x1,y1) (x2,y2) (x3,y3) for the y at x,
// x being monotonic along the curve as Spine guarantees.
func bezierY(x0, y0, x1, y1, x2, y2, x3, y3, x float64) float64 {
	if x <= x0 {
		return y0
	}
	if x >= x3 {
		return y3
	}
	at := func(s float64, p0, p1, p2, p3 float64) float64 {
		u := 1 - s
		return u*u*u*p0 + 3*u*u*s*p1 + 3*u*s*s*p2 + s*s*s*p3
	}
	lo, hi := 0.0, 1.0
	s := (x - x0) / (x3 - x0)
	for range 24 {
		xs := at(s, x0, x1, x2, x3)
		if math.Abs(xs-x) < 1e-6 {
			break
		}
		if xs < x {
			lo = s
		} else {
			hi = s
		}
		s = (lo + hi) / 2
	}
	return at(s, y0, y1, y2, y3)
}

// value1 evaluates a one-component timeline at t; get reads a key's value
// with the timeline's default, and def is the result before the first key.
func (p *pose) value1(keys []Key, t float64, get func(*Key) float64, def float64) float64 {
	i := frameAt(keys, t)
	if i < 0 {
		return def
	}
	if i+1 >= len(keys) {
		return get(&keys[i])
	}
	return p.interp(keys, i, t, 0, get(&keys[i]), get(&keys[i+1]))
}

// valueN evaluates an n-component timeline at t.
func (p *pose) valueN(keys []Key, t float64, n int, get func(*Key, int) float64, def []float64) []float64 {
	i := frameAt(keys, t)
	out := make([]float64, n)
	if i < 0 {
		copy(out, def)
		return out
	}
	for c := 0; c < n; c++ {
		a := get(&keys[i], c)
		if i+1 >= len(keys) {
			out[c] = a
		} else {
			out[c] = p.interp(keys, i, t, c, a, get(&keys[i+1], c))
		}
	}
	return out
}

func keyXY(k *Key, c int, def float64) float64 {
	if c == 0 {
		return ptrOr(k.X, def)
	}
	return ptrOr(k.Y, def)
}

// colorComp reads component c of a key's color (or light) member.
func colorComp(k *Key, c int) float64 {
	s := k.Color
	if s == nil {
		s = k.Light
	}
	if s == nil {
		return 1
	}
	return parseColor(*s)[c]
}

// apply poses the skeleton by anim at time t (seconds), from the setup pose.
func (p *pose) apply(anim *Animation, t float64) {
	p.setToSetupPose()
	for name, tls := range anim.Bones {
		b := p.boneByNm[name]
		if b == nil {
			continue
		}
		d := b.data
		sx, sy := d.scale()
		for kind, keys := range tls {
			if len(keys) == 0 {
				continue
			}
			switch kind {
			case "rotate":
				b.rotation = d.Rotation + p.value1(keys, t, func(k *Key) float64 { return ptrOr(k.Value, ptrOr(k.Angle, 0)) }, 0)
			case "translate":
				v := p.valueN(keys, t, 2, func(k *Key, c int) float64 { return keyXY(k, c, 0) }, []float64{0, 0})
				b.x, b.y = d.X+v[0], d.Y+v[1]
			case "translatex":
				b.x = d.X + p.value1(keys, t, func(k *Key) float64 { return ptrOr(k.Value, 0) }, 0)
			case "translatey":
				b.y = d.Y + p.value1(keys, t, func(k *Key) float64 { return ptrOr(k.Value, 0) }, 0)
			case "scale":
				v := p.valueN(keys, t, 2, func(k *Key, c int) float64 { return keyXY(k, c, 1) }, []float64{1, 1})
				b.scaleX, b.scaleY = sx*v[0], sy*v[1]
			case "scalex":
				b.scaleX = sx * p.value1(keys, t, func(k *Key) float64 { return ptrOr(k.Value, 1) }, 1)
			case "scaley":
				b.scaleY = sy * p.value1(keys, t, func(k *Key) float64 { return ptrOr(k.Value, 1) }, 1)
			case "shear":
				v := p.valueN(keys, t, 2, func(k *Key, c int) float64 { return keyXY(k, c, 0) }, []float64{0, 0})
				b.shearX, b.shearY = d.ShearX+v[0], d.ShearY+v[1]
			case "shearx":
				b.shearX = d.ShearX + p.value1(keys, t, func(k *Key) float64 { return ptrOr(k.Value, 0) }, 0)
			case "sheary":
				b.shearY = d.ShearY + p.value1(keys, t, func(k *Key) float64 { return ptrOr(k.Value, 0) }, 0)
			case "inherit":
				if i := frameAt(keys, t); i >= 0 && keys[i].Inherit != nil {
					b.inherit = *keys[i].Inherit
				}
			default:
				p.notes.addf("bone timeline %q not supported", kind)
			}
		}
	}
	for name, tls := range anim.Slots {
		s := p.slotByNm[name]
		if s == nil {
			continue
		}
		for kind, keys := range tls {
			if len(keys) == 0 {
				continue
			}
			switch kind {
			case "attachment":
				if i := frameAt(keys, t); i >= 0 {
					if keys[i].Name == nil {
						s.attachment = ""
					} else {
						s.attachment = *keys[i].Name
					}
				}
			case "rgba", "color", "rgba2", "twoColor":
				v := p.valueN(keys, t, 4, colorComp, s.setupColor[:])
				copy(s.color[:], v)
			case "rgb", "rgb2":
				v := p.valueN(keys, t, 3, colorComp, s.setupColor[:3])
				copy(s.color[:3], v)
			case "alpha":
				s.color[3] = p.value1(keys, t, func(k *Key) float64 { return ptrOr(k.Value, 1) }, s.setupColor[3])
			default:
				p.notes.addf("slot timeline %q not supported", kind)
			}
		}
	}
	for name, keys := range anim.IK {
		c := p.ikByNm[name]
		if c == nil || len(keys) == 0 {
			continue
		}
		v := p.valueN(keys, t, 2, func(k *Key, comp int) float64 {
			if comp == 0 {
				return ptrOr(k.Mix, 1)
			}
			return ptrOr(k.Softness, 0)
		}, []float64{c.mix, c.softness})
		c.mix, c.softness = v[0], v[1]
		if i := frameAt(keys, t); i >= 0 {
			k := &keys[i]
			c.bendDir = 1
			if !boolOr(k.BendPositive, true) {
				c.bendDir = -1
			}
			c.compress = boolOr(k.Compress, false)
			c.stretch = boolOr(k.Stretch, false)
		}
	}
	for name, keys := range anim.Transform {
		c := p.trByNm[name]
		if c == nil || len(keys) == 0 {
			continue
		}
		get := func(k *Key, comp int) float64 {
			mx := ptrOr(k.MixX, ptrOr(k.TranslateMix, 1))
			msx := ptrOr(k.MixScaleX, ptrOr(k.ScaleMix, 1))
			switch comp {
			case 0:
				return ptrOr(k.MixRotate, ptrOr(k.RotateMix, 1))
			case 1:
				return mx
			case 2:
				return ptrOr(k.MixY, ptrOr(k.TranslateMix, mx))
			case 3:
				return msx
			case 4:
				return ptrOr(k.MixScaleY, ptrOr(k.ScaleMix, msx))
			}
			return ptrOr(k.MixShearY, ptrOr(k.ShearMix, 1))
		}
		m := c.mix
		v := p.valueN(keys, t, 6, get, []float64{m.rotate, m.x, m.y, m.scaleX, m.scaleY, m.shearY})
		c.mix = transformMix{v[0], v[1], v[2], v[3], v[4], v[5]}
	}
	p.applyDeforms(anim, t)
	p.update()
}

// deformTimelines yields every deform timeline of the animation in either
// spelling, restricted to the active skins.
func (p *pose) deformTimelines(anim *Animation, fn func(slot, attachment string, keys []Key)) {
	for skin, slots := range anim.Attachments {
		if !p.skinNames[skin] {
			continue
		}
		for slotName, atts := range slots {
			for attName, tls := range atts {
				for kind, keys := range tls {
					switch kind {
					case "deform":
						fn(slotName, attName, keys)
					default:
						p.notes.addf("attachment timeline %q not supported (slot %q)", kind, slotName)
					}
				}
			}
		}
	}
	for skin, slots := range anim.Deform {
		if !p.skinNames[skin] {
			continue
		}
		for slotName, atts := range slots {
			for attName, keys := range atts {
				fn(slotName, attName, keys)
			}
		}
	}
}

func (p *pose) applyDeforms(anim *Animation, t float64) {
	p.deformTimelines(anim, func(slotName, attName string, keys []Key) {
		i := frameAt(keys, t)
		if i < 0 {
			return
		}
		n := 0
		for _, k := range keys {
			n = max(n, k.Offset+len(k.Vertices))
		}
		expand := func(k *Key) []float64 {
			out := make([]float64, n)
			copy(out[k.Offset:], k.Vertices)
			return out
		}
		from := expand(&keys[i])
		if i+1 < len(keys) {
			// A deform key's curve eases the blend toward the next key,
			// so it is evaluated once as a 0..1 fraction.
			to := expand(&keys[i+1])
			f := p.interp(keys, i, t, 0, 0, 1)
			for j := range from {
				from[j] += (to[j] - from[j]) * f
			}
		}
		p.deforms[deformKey{slotName, attName}] = from
	})
}

// duration is the time of the animation's last key.
func (p *pose) duration(anim *Animation) float64 {
	var d float64
	last := func(keys []Key) {
		for _, k := range keys {
			d = math.Max(d, k.Time)
		}
	}
	for _, tls := range anim.Bones {
		for _, keys := range tls {
			last(keys)
		}
	}
	for _, tls := range anim.Slots {
		for _, keys := range tls {
			last(keys)
		}
	}
	for _, keys := range anim.IK {
		last(keys)
	}
	for _, keys := range anim.Transform {
		last(keys)
	}
	p.deformTimelines(anim, func(_, _ string, keys []Key) { last(keys) })
	last(anim.Events)
	for _, raw := range anim.DrawOrder {
		var k Key
		if json.Unmarshal(raw, &k) == nil {
			d = math.Max(d, k.Time)
		}
	}
	return d
}
