package main

// Clips are authored as pose sequences. A pose gives every rig track one
// value; the builder turns the sequence into per-track keyframes and
// collapses tracks that never move into static properties. Rotations are
// degrees, clockwise, zero hanging straight down (limbs) or upright
// (body); negative swings a limb forward (+x), matching the right-facing
// character. Elbows bend forward (negative), knees bend backward
// (positive), like the joints they stand in for.

import (
	"fmt"
	"math"
	"slices"
)

type pose struct {
	bx, by         float64  // body offset from rest (px, +y down)
	rot            float64  // body lean
	sx, sy         float64  // body scale % (squash and stretch)
	head           float64  // head rotation on the neck
	armN, armF     float64  // shoulder rotation
	elbowN, elbowF float64  // forearm rotation relative to the upper arm
	legN, legF     float64  // hip rotation
	kneeN, kneeF   float64  // shin rotation relative to the thigh
	shNX, shNY     float64  // near shoulder shifted off its rest attach
	shFX, shFY     float64  // far shoulder likewise
	blade          float64  // sword rotation relative to the far forearm
	alpha          float64  // whole-character opacity % (hurt flash)
	shadow         float64  // ground shadow scale % (reads as height)
	flip           bool     // turned: limb attach sides swap, head/shins x-mirror
	swap           bool     // shoulder attach sides swap only (a wrapped-around shoulder)
	swapLegs       bool     // hip attach sides swap only (a spinning kick)
	view           viewKind // which head/body drawing shows: front, side, back
	headView       viewKind // head override: the eyes track the opponent, so
	headViewSet    bool     // the head often rotates less than the body
}

// viewKind picks the drawing set for the head and torso, so a rotation
// is shown by stepping through views instead of mirroring in place: a
// turn passes through the rear-quarter side view (one eye, thin torso),
// punch-2's windup goes all the way to the back view.
type viewKind int

const (
	viewFront viewKind = iota
	viewSide
	viewBack
)

func base() pose { return pose{sx: 100, sy: 100, alpha: 100, shadow: 100} }

func (p pose) at(x, y float64) pose          { p.bx, p.by = x, y; return p }
func (p pose) lean(deg float64) pose         { p.rot = deg; return p }
func (p pose) squash(sx, sy float64) pose    { p.sx, p.sy = sx, sy; return p }
func (p pose) nod(deg float64) pose          { p.head = deg; return p }
func (p pose) arms(near, far float64) pose   { p.armN, p.armF = near, far; return p }
func (p pose) elbows(near, far float64) pose { p.elbowN, p.elbowF = near, far; return p }
func (p pose) legs(near, far float64) pose   { p.legN, p.legF = near, far; return p }
func (p pose) knees(near, far float64) pose  { p.kneeN, p.kneeF = near, far; return p }
func (p pose) blades(deg float64) pose       { p.blade = deg; return p }

// shoulders slides the arm roots off their rest attach points. The
// shoulder girdle is not rigid — driving the leading shoulder through a
// swing is most of where a strike's extra reach comes from, and no
// amount of arm rotation substitutes for it, since a straight arm has
// already spent its length.
func (p pose) shoulders(nx, ny, fx, fy float64) pose {
	p.shNX, p.shNY, p.shFX, p.shFY = nx, ny, fx, fy
	return p
}
func (p pose) fade(a float64) pose  { p.alpha = a; return p }
func (p pose) shade(s float64) pose { p.shadow = s; return p }
func (p pose) turned() pose         { p.flip = true; return p }
func (p pose) away() pose           { p.view = viewBack; return p }
func (p pose) aside() pose          { p.view = viewSide; return p }
func (p pose) swapped() pose        { p.swap = true; return p }
func (p pose) swappedLegs() pose    { p.swapLegs = true; return p }

// faceAside keeps the head at the rear-quarter view (one eye on the
// opponent) while the body shows whatever view is set — the head never
// turns as far as the body in a spin attack.
func (p pose) faceAside() pose { p.headView, p.headViewSet = viewSide, true; return p }

// headViewOf is the view the head layers actually show.
func headViewOf(p pose) viewKind {
	if p.headViewSet {
		return p.headView
	}
	return p.view
}

type kf struct {
	t    float64
	p    pose
	ease bool
}

func k(t float64, p pose, ease bool) kf { return kf{t, p, ease} }

// clipDef is one clip as authored: the document and the preview sheet are
// both built from it, so the sheet cannot drift from the data.
type clipDef struct {
	name   string
	frames float64
	keys   []kf
}

func def(name string, frames float64, keys ...kf) clipDef {
	return clipDef{name: name, frames: frames, keys: keys}
}

// track builds one property from the pose sequence: static when every key
// agrees, keyframed otherwise.
func track(keys []kf, get func(pose) []float64) obj {
	first := get(keys[0].p)
	animated := false
	for _, kf := range keys[1:] {
		if !slices.Equal(get(kf.p), first) {
			animated = true
			break
		}
	}
	if !animated {
		if len(first) == 1 {
			return static(first[0])
		}
		return static(first)
	}
	ks := make([]obj, len(keys))
	for i, kf := range keys {
		ks[i] = key(kf.t, get(kf.p), kf.ease)
	}
	return anim(ks...)
}

func scalar(get func(pose) float64) func(pose) []float64 {
	return func(p pose) []float64 { return []float64{get(p)} }
}

// flipTrack is a two-state property driven by a pose predicate: off / on
// values with hold keyframes, so the switch is instant — a turn shows no
// morphing in between, the sides just trade places at the midpoint.
func flipTrack(keys []kf, off, on []float64, pred func(pose) bool) obj {
	animated := false
	for _, kf := range keys[1:] {
		if pred(kf.p) != pred(keys[0].p) {
			animated = true
			break
		}
	}
	if !animated {
		v := off
		if pred(keys[0].p) {
			v = on
		}
		return static(v)
	}
	ks := make([]obj, len(keys))
	for i, kf := range keys {
		v := off
		if pred(kf.p) {
			v = on
		}
		ks[i] = holdKey(kf.t, v)
	}
	return anim(ks...)
}

// armsSwapped says whether the shoulders sit on their traded sides: a
// full turn does it, and so does the waist-twist of the haymaker — the
// hips stay planted there, so only the arms trade.
func armsSwapped(p pose) bool { return p.flip || p.swap }

// legsSwapped says whether the hips trade sides: a full turn, or the
// spin of a turning kick.
func legsSwapped(p pose) bool { return p.flip || p.swapLegs }

// imageMirrored says whether the head and shoes x-mirror: only a real
// turn does — a shoulder swap keeps the character facing forward.
func imageMirrored(p pose) bool { return p.flip }

// bodyMirrorX reflects an attach point across the body's vertical center.
func bodyMirrorX(pos [2]float64) [2]float64 {
	return [2]float64{48 - pos[0], pos[1]}
}

// holdTrack is like track but switches instantly at every key — for the
// head / back-head opacity swap, which must cut, not crossfade.
func holdTrack(keys []kf, get func(pose) []float64) obj {
	first := get(keys[0].p)
	animated := false
	for _, kf := range keys[1:] {
		if !slices.Equal(get(kf.p), first) {
			animated = true
			break
		}
	}
	if !animated {
		if len(first) == 1 {
			return static(first[0])
		}
		return static(first)
	}
	ks := make([]obj, len(keys))
	for i, kf := range keys {
		ks[i] = holdKey(kf.t, get(kf.p))
	}
	return anim(ks...)
}

// clip assembles the rig into a document. Limbs are two-segment chains
// (upper arm carrying a forearm, thigh carrying a shin) parented through
// the body. Layer order is front to back: the near arm chain crosses in
// front of the face on a punch, the near leg chain in front of the body,
// far chains behind everything but the shadow.
func clip(name string, frames float64, keys []kf) obj {
	const (
		bodyInd      = 6
		upperArmNInd = 2
		thighNInd    = 5
		thighFInd    = 7
		upperArmFInd = 9
		forearmFInd  = 10
	)
	// swapSides: attach point trades sides on flip (limb roots).
	// mirror: the image x-mirrors on flip (head keeps the face reading
	// left; shins keep the shoe toe pointing the way the character now
	// faces). Chain children (forearms, shins) follow their parent, so
	// their attach never moves.
	seg := func(p part, swapSides func(pose) bool, mirror bool, get func(pose) float64) obj {
		pos := obj(static([]float64{p.pos[0], p.pos[1]}))
		if swapSides != nil {
			m := bodyMirrorX(p.pos)
			pos = flipTrack(keys, []float64{p.pos[0], p.pos[1]}, []float64{m[0], m[1]}, swapSides)
		}
		scale := obj(static([]float64{100, 100}))
		if mirror {
			scale = flipTrack(keys, []float64{100, 100}, []float64{-100, 100}, imageMirrored)
		}
		return transform(
			static([]float64{p.anchor[0], p.anchor[1]}),
			pos,
			scale,
			track(keys, scalar(get)),
			track(keys, scalar(func(p pose) float64 { return p.alpha })),
		)
	}
	// armSeg is seg for the two shoulders, whose attach can also slide
	// within a pose. A clip that trades the arms' sides has to switch on
	// hold keys, as before, or the swap would morph; one that only slides
	// them eases, so a shoulder driving through a swing travels smoothly.
	armSeg := func(pt part, far bool, get func(pose) float64) obj {
		root := func(p pose) []float64 {
			r := armRoot(far, p)
			return []float64{r[0], r[1]}
		}
		pos := obj(nil)
		if predVaries(keys, armsSwapped) {
			pos = holdTrack(keys, root)
		} else {
			pos = track(keys, root)
		}
		return transform(
			static([]float64{pt.anchor[0], pt.anchor[1]}),
			pos,
			static([]float64{100, 100}),
			track(keys, scalar(get)),
			track(keys, scalar(func(p pose) float64 { return p.alpha })),
		)
	}
	// The head is three layers (front, side, back) and the torso two
	// (front, side); exactly one of each set is visible per pose. The
	// switches are cuts on hold keys when the clip rotates through views,
	// and plain alpha tracks otherwise so the hurt flash still eases.
	viewOpFor := func(viewOf func(pose) viewKind, visible func(viewKind) bool) obj {
		varies := false
		for _, kf := range keys[1:] {
			if viewOf(kf.p) != viewOf(keys[0].p) {
				varies = true
				break
			}
		}
		get := func(p pose) []float64 {
			if visible(viewOf(p)) {
				return []float64{p.alpha}
			}
			return []float64{0}
		}
		if varies {
			return holdTrack(keys, get)
		}
		return track(keys, get)
	}
	bodyView := func(p pose) viewKind { return p.view }
	viewOp := func(visible func(viewKind) bool) obj { return viewOpFor(bodyView, visible) }
	headViewOp := func(visible func(viewKind) bool) obj { return viewOpFor(headViewOf, visible) }
	headLayer := func(name string, ind int, visible func(viewKind) bool) obj {
		tr := transform(
			static([]float64{headPart.anchor[0], headPart.anchor[1]}),
			static([]float64{headPart.pos[0], headPart.pos[1]}),
			flipTrack(keys, []float64{100, 100}, []float64{-100, 100}, imageMirrored),
			track(keys, scalar(func(p pose) float64 { return p.head })),
			headViewOp(visible),
		)
		return imgLayer(name, ind, bodyInd, frames, name, tr)
	}
	bodyViewLayer := func(name string, ind int, pt part, visible func(viewKind) bool) obj {
		return imgLayer(name, ind, bodyInd, frames, name, transform(
			static([]float64{pt.anchor[0], pt.anchor[1]}),
			static([]float64{pt.pos[0], pt.pos[1]}),
			flipTrack(keys, []float64{100, 100}, []float64{-100, 100}, imageMirrored),
			static(0.0),
			viewOp(visible),
		))
	}
	bodySideLayer := bodyViewLayer("body-side", 13, bodySidePart, func(v viewKind) bool { return v == viewSide })
	bodyBackLayer := bodyViewLayer("body-back", 15, bodyBackPart, func(v viewKind) bool { return v == viewBack })
	bodyTr := transform(
		static([]float64{bodyPart.anchor[0], bodyPart.anchor[1]}),
		track(keys, func(p pose) []float64 { return []float64{restX + p.bx, restY + p.by} }),
		track(keys, func(p pose) []float64 { return []float64{p.sx, p.sy} }),
		track(keys, scalar(func(p pose) float64 { return p.rot })),
		// A parent's opacity does not cascade in Lottie, so hiding the
		// front torso during the other views leaves the children alone.
		viewOp(func(v viewKind) bool { return v == viewFront }),
	)
	shadowTr := transform(
		static([]float64{shadowPart.anchor[0], shadowPart.anchor[1]}),
		static([]float64{shadowPart.pos[0], shadowPart.pos[1]}),
		track(keys, func(p pose) []float64 { return []float64{p.shadow, p.shadow} }),
		static(0.0),
		static(28.0),
	)
	farForearm := imgLayer("forearm-far", 10, upperArmFInd, frames, "forearm-far", seg(forearmF, nil, false, func(p pose) float64 { return p.elbowF }))
	farUpperArm := imgLayer("upper-arm-far", upperArmFInd, bodyInd, frames, "upper-arm-far", armSeg(upperArmF, true, func(p pose) float64 { return p.armF }))
	layers := []obj{
		imgLayer("forearm-near", 1, upperArmNInd, frames, "forearm-near", seg(forearmN, nil, false, func(p pose) float64 { return p.elbowN })),
		imgLayer("upper-arm-near", upperArmNInd, bodyInd, frames, "upper-arm-near", armSeg(upperArmN, false, func(p pose) float64 { return p.armN })),
		headLayer("head", 3, func(v viewKind) bool { return v == viewFront }),
		headLayer("head-side", 12, func(v viewKind) bool { return v == viewSide }),
		headLayer("head-back", 14, func(v viewKind) bool { return v == viewBack }),
		imgLayer("shin-near", 4, thighNInd, frames, "shin-near", seg(shinNearPart, nil, true, func(p pose) float64 { return p.kneeN })),
		imgLayer("thigh-near", thighNInd, bodyInd, frames, "thigh-near", seg(thighN, legsSwapped, false, func(p pose) float64 { return p.legN })),
		bodySideLayer,
		bodyBackLayer,
		imgLayer("body", bodyInd, 0, frames, "body", bodyTr),
		imgLayer("thigh-far", thighFInd, bodyInd, frames, "thigh-far", seg(thighF, legsSwapped, false, func(p pose) float64 { return p.legF })),
		imgLayer("shin-far", 8, thighFInd, frames, "shin-far", seg(shinFarPart, nil, true, func(p pose) float64 { return p.kneeF })),
		imgLayer("shadow", 11, 0, frames, "shadow", shadowTr),
	}
	if swordRig {
		// The whole far arm comes in front of the torso, because that is
		// where it is: it reaches across the FRONT of the body to put its
		// hand on the hilt. Left in the far chain the forearm vanishes
		// behind the torso and the character reads as one-handed with a
		// stump for the other arm; leaving the upper arm behind only moves
		// the seam up to the shoulder.
		layers = slices.Insert(layers, 7, farUpperArm, farForearm)
	} else {
		layers = slices.Insert(layers, len(layers)-1, farUpperArm, farForearm)
	}
	if swordRig {
		sword := imgLayer("sword", 16, forearmFInd, frames, "sword", transform(
			static([]float64{swordPart.anchor[0], swordPart.anchor[1]}),
			static([]float64{swordPart.pos[0], swordPart.pos[1]}),
			static([]float64{100, 100}),
			track(keys, scalar(func(p pose) float64 { return p.blade })),
			track(keys, scalar(func(p pose) float64 { return p.alpha })),
		))
		// The blade sits just behind the near hand: over the body, under
		// the fingers gripping it. It can stay there in every clip
		// because both hands are always on the hilt in front of the
		// character — depth-correct placement in the far chain would
		// swallow it in the torso mid-cut, and a weapon that vanishes
		// mid-swing is the worse lie.
		layers = slices.Insert(layers, 1, sword)
	}
	return doc(name, frames, layers)
}

// --- Locomotion ---

// idleClip breathes downward: the feet stay planted and the knees flex,
// dropping the hip a touch — never upward, which reads as floating.
// The 2px hip drop matches what the bent knees shorten the leg column
// by, so the shoes stay on the ground line.
func idleClip() clipDef {
	rest := base().elbows(-8, -8).knees(3, 3)
	sink := base().at(0, 2).legs(-9, -9).knees(24, 24).nod(3).arms(5, -5).elbows(-14, -14)
	return def("idle-anim", 96,
		k(0, rest, true), k(48, sink, true), k(96, rest, true))
}

// Turn clips actually turn: the shoulders swing around, and at the
// midpoint the limb chains trade sides while the head and shoes mirror —
// an instant swap on hold keyframes, no morphing in between. The clip
// therefore ENDS facing the other way; the game flips its Mirrored flag
// when the turn completes, and the mirrored idle matches the end pose.
func idleTurnClip() clipDef {
	// Like walk-turn, the standing turn goes through the camera side:
	// front views only, a slight scale-up passing the viewer, and the
	// mirror lands on a self-mirror pose — limbs straight, joints at
	// zero — so the geometry never jumps and nothing bends the wrong
	// way. The end pose is the rest pose negated in place.
	rest := base().elbows(-8, -8).knees(3, 3)
	gatherA := base().at(0, -1).arms(12, -12).elbows(-2, -2).legs(-4, 4).knees(1, 1).squash(103, 103)
	gatherB := base().at(0, -1).arms(-12, 12).elbows(2, 2).legs(4, -4).knees(-1, -1).squash(103, 103).turned()
	mirroredRest := base().elbows(8, 8).knees(-3, -3).turned()
	return def("idle-turn-anim", 20,
		k(0, rest, true), k(9, gatherA, true), k(11, gatherB, true),
		k(20, mirroredRest, true))
}

func walkClip() clipDef {
	contactA := base().legs(-30, 30).knees(8, 30).arms(25, -25).elbows(-12, -20).lean(4)
	passA := base().at(0, -7).legs(0, 0).knees(25, 22).arms(0, 0).elbows(-16, -16).lean(4).nod(2)
	contactB := base().legs(30, -30).knees(30, 8).arms(-25, 25).elbows(-20, -12).lean(4)
	passB := passA.knees(22, 25)
	return def("walk-anim", 48,
		k(0, contactA, false), k(12, passA, false), k(24, contactB, false),
		k(36, passB, false), k(48, contactA, false))
}

// walkTurnClip turns through the camera side: the front stays visible
// the whole way — no side views — growing a touch as it passes the
// viewer, and the drawing simply mirrors at the midpoint. The mirror
// lands on a left/right symmetric pose, so the geometry holds still
// while the near/far identities trade (a game-friendly lie: the legs
// swap in the data, not on screen), and the clip starts on the walk's
// contact pose and ends on its mirror, so the reversed walk picks up
// seamlessly.
func walkTurnClip() clipDef {
	// The mirror must land on the MIRRORED geometry, so the swap-moment
	// pose has to be its own mirror image: limbs near straight, joints
	// at zero. A bent knee carried across the flip would kink the wrong
	// way for the new facing; instead the bends ease out on the way in
	// and regrow, correctly mirrored, on the way out.
	from := base().legs(-30, 30).knees(8, 30).arms(25, -25).elbows(-12, -20).lean(4)
	gatherA := base().at(0, -2).legs(-12, 12).knees(2, 2).arms(14, -14).elbows(-4, -4).squash(103, 103)
	// Mirrored poses negate every value in place — lean, nod, knees and
	// elbows included. Near/far values are NOT exchanged: the flip
	// already moves each limb to the mirrored attach, so its own value
	// just flips sign. (Arms and legs being antisymmetric pairs hid
	// this for a while; the knees, elbows and lean gave it away.)
	gatherB := base().at(0, -2).legs(12, -12).knees(-2, -2).arms(-14, 14).elbows(4, 4).squash(103, 103).turned()
	out := base().legs(30, -30).knees(-8, -30).arms(-25, 25).elbows(12, 20).lean(-4).turned()
	return def("walk-turn-anim", 20,
		k(0, from, true), k(9, gatherA, true), k(11, gatherB, true), k(20, out, true))
}

func runClip() clipDef {
	contactA := base().legs(-50, 50).knees(15, 70).arms(40, -40).elbows(-40, -40).lean(12)
	passA := base().at(0, -10).squash(97, 103).legs(0, 0).knees(55, 55).arms(0, 0).elbows(-40, -40).lean(12)
	contactB := base().legs(50, -50).knees(70, 15).arms(-40, 40).elbows(-40, -40).lean(12)
	return def("run-anim", 24,
		k(0, contactA, false), k(6, passA, false), k(12, contactB, false),
		k(18, passA, false), k(24, contactA, false))
}

// runToIdleClip is a foot-brake skid: the leading leg thrusts out and
// slides on its sole, the trailing toe drags behind, the torso leans
// back against the momentum — the hips dip only as far as the braced
// leg forces them, nothing like the slide clip's squat. There is no
// run-turn: a runner brakes with this, and the game then decides
// whether to stand or to start the mirrored run.
func runToIdleClip() clipDef {
	from := base().legs(-50, 50).knees(15, 70).arms(40, -40).elbows(-40, -40).lean(12)
	// Arms spread for balance: the dark far arm reaches forward from the
	// leading shoulder, the bright near arm sweeps back-up past the
	// trailing edge.
	plant := base().at(2, 7).lean(13).legs(2, -48).knees(30, 8).arms(70, -85).elbows(-8, -8).nod(-3).shade(112)
	skid := base().at(3, 8).lean(17).legs(-10, -60).knees(28, 6).arms(85, -100).elbows(-6, -6).nod(-4).shade(118)
	ease := base().at(1, 2).lean(-4).legs(-10, 8).knees(8, 8).arms(-8, 6).elbows(-12, -12).shade(104)
	rest := base().elbows(-8, -8).knees(3, 3)
	return def("run-to-idle-anim", 28,
		k(0, from, true), k(5, plant, false), k(14, skid, true),
		k(21, ease, true), k(28, rest, true))
}

// slideClip goes down before it goes back: a full squat first — hips
// all the way down, knees folded, feet planted, torso still upright —
// and only then the lean-back extension into the slide. Tipping
// backwards from standing height reads as falling over, not sliding.
// Horizontal travel is the game's job; the few pixels here just sell
// the glide.
func slideClip() clipDef {
	from := base().legs(-30, 30).knees(10, 40).arms(25, -25).elbows(-25, -25).lean(8)
	crouch := base().at(2, 26).legs(-65, -60).knees(115, 108).arms(20, -30).elbows(-30, -20).lean(12).nod(5).squash(103, 95).shade(114)
	settle := crouch.at(3, 27)
	// Both legs extend forward along the ground, the near one resting
	// just above the far one.
	glide := base().at(8, 34).lean(-34).legs(-46, -40).knees(16, 15).arms(55, 25).elbows(-25, -12).nod(16).squash(104, 96).shade(124)
	slid := glide.at(14, 34)
	up := base().at(6, 6).lean(-6).legs(-25, 15).knees(28, 12).arms(10, -10).elbows(-15, -15)
	return def("slide-anim", 30,
		k(0, from, true), k(4, crouch, false), k(9, settle, true),
		k(13, glide, false), k(22, slid, true), k(30, up, true))
}

// --- Air ---

// Air poses lead with the far-side limbs, like the attacks: the leading
// arm and leg reach forward-up, the near side trails behind.
func jumpClip() clipDef {
	crouch := base().at(0, 10).squash(110, 88).lean(4).legs(-14, 14).knees(25, 25).arms(18, -18).elbows(-20, -20)
	launch := base().at(0, -60).squash(94, 106).legs(25, -30).knees(45, 15).arms(70, -70).elbows(-25, -25).shade(58)
	apex := base().at(0, -72).legs(15, -15).knees(45, 30).arms(80, -80).elbows(-20, -20).shade(48)
	out := base().at(0, -62).legs(24, -18).knees(40, 25).arms(80, -80).elbows(-20, -20).shade(52)
	return def("jump-anim", 32,
		k(0, base().elbows(-8, -8).knees(3, 3), true), k(5, crouch, true), k(12, launch, true),
		k(22, apex, true), k(32, out, true))
}

func fallClip() clipDef {
	from := base().at(0, -62).arms(80, -80).elbows(-20, -20).legs(32, -20).knees(35, 20).shade(52)
	settle := base().at(0, -38).arms(70, -70).elbows(-25, -25).legs(26, -15).knees(30, 25).shade(58).nod(5)
	return def("fall-anim", 16,
		k(0, from, true), k(16, settle, true))
}

func fallLoopClip() clipDef {
	a := base().at(0, -38).arms(70, -70).elbows(-25, -25).legs(26, -15).knees(30, 25).shade(58).nod(5)
	b := base().at(0, -44).arms(80, -80).elbows(-18, -18).legs(16, -25).knees(24, 32).shade(54).nod(8)
	return def("fall-loop-anim", 32,
		k(0, a, true), k(16, b, true), k(32, a, true))
}

// --- Reactions ---

func hurtClip() clipDef {
	rest := base().elbows(-8, -8).knees(3, 3)
	recoil := base().at(-7, 0).lean(-18).nod(-15).fade(45).arms(20, 25).elbows(-30, -30).knees(10, 10)
	back := base().at(-3, 0).lean(-12).nod(-10).arms(10, 15).elbows(-20, -20).knees(8, 8)
	flash := back.fade(45)
	return def("hurt-anim", 20,
		k(0, rest, true), k(3, recoil, true), k(8, back, true),
		k(12, flash, true), k(20, rest, true))
}

func deathClip() clipDef {
	rest := base().elbows(-8, -8).knees(3, 3)
	stagger := base().lean(-20).nod(-10).arms(15, 20).elbows(-25, -25).at(-3, 3).knees(12, 12)
	down := base().at(0, 30).lean(-85).nod(-15).legs(25, 32).knees(18, 24).arms(35, 45).elbows(-15, -15).squash(100, 96)
	lie := base().at(0, 32).lean(-88).nod(-18).legs(26, 34).knees(20, 26).arms(38, 48).elbows(-15, -15).squash(100, 96)
	return def("death-anim", 48,
		k(0, rest, true), k(8, stagger, true), k(28, down, true), k(40, lie, true))
}

// --- Attacks (unarmed) ---
//
// Strikes lead with the far-side limb: after the near/far correction it
// is the limb on the leading (+x) edge, so it reaches the enemy in front
// instead of crossing the whole body. The combo follow-up then swings
// the trailing (near) limb through for the bigger second hit.

func punchClip() clipDef {
	ready := base().lean(4).arms(-20, -15).elbows(-35, -40).legs(-10, 10).knees(8, 12)
	windup := base().lean(-4).arms(-25, 30).elbows(-35, -75).legs(-10, 10).knees(8, 12)
	strike := base().at(6, 0).lean(12).arms(25, -95).elbows(-45, -5).legs(-14, 12).knees(6, 14)
	hold := base().at(4, 0).lean(10).arms(20, -88).elbows(-40, -8).legs(-14, 12).knees(6, 14)
	rest := base().elbows(-8, -8).knees(3, 3)
	return def("punch-anim", 20,
		k(0, ready, true), k(4, windup, true), k(7, strike, false),
		k(12, hold, true), k(20, rest, true))
}

// punch2Clip is the spinning haymaker. The windup stays face-on and
// upright — no lean-back — with the arm cocked behind; at the moment
// the swing whips through, the body has spun past front, so from there
// to the end the BACK shows (faceless head, darker torso) with the
// shoulders traded, hips planted: a waist twist carried all the way
// around. Reachable as the punch follow-up and via the punch2 event.
func punch2Clip() clipDef {
	// The non-punching far arm stays folded from the very first frame —
	// elbow bent hard, fist pulled in — so nothing about it reads as a
	// second punch.
	from := base().at(4, 0).lean(6).arms(20, -12).elbows(-40, -130).legs(-12, 12).knees(8, 12)
	// The windup cocks the fist by folding the elbow — the upper arm
	// barely leaves the side, nothing pulls back — and extension only
	// begins at the spin frame.
	wind := base().lean(2).arms(10, -15).elbows(-105, -130).legs(-10, 10).knees(8, 10)
	windDeep := base().at(-1, 1).lean(4).arms(18, -16).elbows(-120, -130).legs(-10, 10).knees(8, 10)
	// The rear arm folds through the back view: elbow bent hard so the
	// forearm tucks across the turned body — hidden behind the torso,
	// only the elbow showing at the trailing edge.
	spin := base().at(4, 0).lean(10).arms(-40, 35).elbows(-55, -110).legs(-8, 8).knees(8, 10).away().faceAside().swapped()
	strike := base().at(9, 0).lean(16).squash(103, 97).arms(-100, 30).elbows(-3, -115).legs(-16, 14).knees(6, 14).away().faceAside().swapped()
	hold := base().at(7, 0).lean(13).arms(-92, 30).elbows(-6, -115).legs(-16, 14).knees(6, 14).away().faceAside().swapped()
	settle := base().at(4, 0).lean(6).arms(-20, 25).elbows(-15, -110).legs(-10, 10).knees(6, 8).away().faceAside().swapped()
	return def("punch-2-anim", 28,
		k(0, from, true), k(5, wind, true), k(9, windDeep, true),
		k(10, spin, false), k(12, strike, false),
		k(17, hold, true), k(28, settle, true))
}

// kickClip chambers first — hip raised, knee fully folded — and only
// snaps the knee straight at the moment of impact.
func kickClip() clipDef {
	rest := base().elbows(-8, -8).knees(3, 3)
	chamber := base().at(0, -2).lean(-10).legs(8, -55).knees(10, 85).arms(25, -20).elbows(-25, -25)
	raised := base().at(0, -3).lean(-13).legs(8, -68).knees(10, 95).arms(28, -24).elbows(-25, -25)
	strike := base().at(2, -3).lean(-16).legs(8, -85).knees(10, 5).arms(32, -30).elbows(-20, -20)
	hold := base().at(1, -2).lean(-14).legs(8, -75).knees(10, 15).arms(28, -26).elbows(-20, -20)
	return def("kick-anim", 24,
		k(0, rest, true), k(6, chamber, true), k(10, raised, true), k(12, strike, false),
		k(16, hold, true), k(24, rest, true))
}

// kick2Clip is the spinning high kick, punch-2's leg-side sibling: the
// windup stays face-on and chambers by folding the knee, then the spin
// carries the body past front — back view from there to the end — and
// the bright near leg whips up from the leading edge, knee snapping
// straight at head height on the strike frame.
func kick2Clip() clipDef {
	// The torso leans slightly BACK through the kick (counterweight to
	// the raised leg), the bright arm's elbow stays bent, and the body
	// offsets are solved per key so the support foot's ground contact
	// never slides: the support leg counters the body lean to stay
	// vertical, and (bx, by) puts its toe on the same spot every key.
	from := base().lean(4).arms(-15, -20).elbows(-30, -35).legs(-10, 10).knees(8, 12)
	chamber := base().at(-3.8, -0.7).lean(0).arms(20, -20).elbows(-45, -25).legs(-35, 10).knees(110, 12)
	chamberDeep := base().at(-0.9, 0.1).lean(2).arms(24, -22).elbows(-50, -25).legs(-45, 10).knees(125, 14)
	spin := base().at(-4.1, -3.7).lean(-6).arms(28, 30).elbows(-60, -110).legs(-60, 6).knees(60, 10).away().faceAside().swapped().swappedLegs()
	strike := base().at(-4.3, -4.5).lean(-12).squash(102, 98).arms(30, 28).elbows(-80, -110).legs(-104, 12).knees(5, 8).away().faceAside().swapped().swappedLegs()
	hold := base().at(-4.2, -4.4).lean(-10).arms(28, 26).elbows(-75, -110).legs(-96, 10).knees(12, 8).away().faceAside().swapped().swappedLegs()
	settle := base().at(-1, 0).lean(4).arms(-15, 20).elbows(-30, -105).legs(-15, 10).knees(20, 12).away().faceAside().swapped().swappedLegs()
	return def("kick-2-anim", 28,
		k(0, from, true), k(5, chamber, true), k(9, chamberDeep, true),
		k(10, spin, false), k(12, strike, false),
		k(17, hold, true), k(28, settle, true))
}

func jumpKickClip() clipDef {
	air := base().at(0, -48).arms(60, -60).elbows(-25, -25).legs(22, -15).knees(28, 25).shade(55)
	tuck := base().at(0, -45).lean(8).legs(25, -45).knees(40, 80).arms(40, -40).elbows(-30, -30).shade(55)
	strike := base().at(3, -42).lean(12).legs(30, -72).knees(45, 5).arms(45, -50).elbows(-25, -25).shade(55)
	out := base().at(0, -40).arms(60, -60).elbows(-25, -25).legs(26, -18).knees(30, 28).shade(57)
	return def("jump-kick-anim", 28,
		k(0, air, true), k(6, tuck, true), k(10, strike, false),
		k(20, strike, true), k(28, out, true))
}

// --- Defense ---

func guardStance() pose {
	return base().at(0, 3).lean(6).squash(103, 98).
		arms(-45, -55).elbows(-95, -85).legs(-12, 12).knees(10, 14).nod(3)
}

func guardClip() clipDef {
	stance := guardStance()
	breathe := stance.at(0, 2)
	return def("guard-anim", 32,
		k(0, stance, true), k(16, breathe, true), k(32, stance, true))
}

func guardHitClip() clipDef {
	stance := guardStance()
	pushed := stance.at(-8, 3).lean(-2).arms(-38, -48).elbows(-100, -90).fade(50)
	back := stance.at(-4, 3)
	return def("guard-hit-anim", 16,
		k(0, stance, true), k(4, pushed, false), k(8, back, true), k(16, stance, true))
}

// --- Sword ---
//
// The blade is gripped by the far (leading) hand, the same limb every
// unarmed strike leads with, so a weapon swing reaches the enemy in
// front instead of crossing the body.

// carried rewrites an inherited clip so the character holds the sword
// the way anyone carries a blade this size: both hands on the hilt at
// the waist, upper arms hanging, forearms angled in across the belly,
// the blade sweeping down and BEHIND. It costs the gaits their arm
// swing, which is the trade a greatsword makes anyway, and it buys two
// things. The hands are in front of the body in every clip, so the
// weapon can simply be drawn in front and never has to change depth
// mid-clip. And the carried silhouette — a long diagonal trailing back
// — is nothing like any attack, which all drive the blade forward, so a
// glance says whether this character is swinging or not.
func carried(defs ...clipDef) []clipDef {
	out := make([]clipDef, len(defs))
	for i, d := range defs {
		out[i] = carry(d)
	}
	return out
}

// holdArc is the blade's angle in WORLD space at the start and end of a
// carried clip — world, not body-relative, because that is what decides
// whether the point clears the ground however the torso is pitched. The
// default holds one angle throughout; the clips that end up horizontal
// (slide) or flat on the floor (death) sweep the blade back on the way
// down, and a turn sweeps it through vertical so the mirror has
// something symmetric to land on.
func holdArc(name string) (float64, float64) {
	switch name {
	case "idle-turn-anim", "walk-turn-anim":
		return 45, -45
	case "slide-anim":
		return 45, 72
	case "death-anim":
		return 45, 82
	}
	return 45, 45
}

// lowGrip is the far arm folded onto the hilt at the body's centerline:
// upper arm hanging at the side, forearm angled in across the belly.
// This is the shape every low two-handed pose uses. The alternative the
// solver offers — swinging the upper arm back to horizontal and folding
// the elbow hard — reaches the same point, but parks the elbow up at
// shoulder height where the head hides it, and the visible bend then
// reads backwards.
func lowGrip(p pose, armF, elbowF, world float64) pose {
	if p.flip {
		armF, elbowF = -armF, -elbowF
	}
	p.armF, p.elbowF = armF, elbowF
	p.blade = world - p.rot - armF - elbowF
	return held(eyesUp(p))
}

// eyesUp keeps the character from ever looking at the floor. Head angle
// is relative to the torso, so a deep forward lean drags the face down
// with it unless the neck gives that rotation back — a swordsman at a
// lean of 30 with a neutral neck is staring at their own feet. Poses
// that deliberately look further up are left alone.
func eyesUp(p pose) pose {
	const brow = -8 // chin up, looking a little above the horizon
	if p.rot+p.head > brow {
		p.head = brow - p.rot
	}
	return p
}

// highGrip is the same idea overhead: the arm goes up nearly straight,
// because at full stretch the hilt is at the limit of what the near arm
// can still reach.
func highGrip(p pose, armF, elbowF, world float64) pose {
	return lowGrip(p, armF, elbowF, world)
}

// swordHold is what posing a clip in the editor settled on: the blade's
// angle on the far forearm, how the near hand grips, and where the head
// looks. One value covers the whole clip, a list gives one per keyframe,
// and an empty field keeps what the generator derives — the near arm
// solved onto the hilt, the blade on holdArc's world angle.
//
// These are transcribed from that posing session rather than derived,
// which is why they are bare numbers. An authored head angle is taken as
// given, without the eyes-up clamp: the clamp exists to stop a derived
// pose from staring at the floor, not to overrule a pose someone sat and
// aimed by hand.
type swordHold struct {
	blade  []float64
	armN   []float64
	elbowN []float64
	head   []float64
}

// Locomotion and air, posed in the editor. The blade rides much further
// back than the derived carry had it — trailing near horizontal in the
// gaits, swung up on the jump — and the near arm is aimed by hand rather
// than solved, so the two hands sit on the hilt the way they looked
// right rather than the way the solver placed them.
var swordHolds = map[string]swordHold{
	"walk-anim": {blade: []float64{17}, armN: []float64{27}, elbowN: []float64{-100}, head: []float64{0}},
	"run-anim":  {blade: []float64{20}, armN: []float64{38}, elbowN: []float64{-102}, head: []float64{-10}},
	// The brake's first key is the run's own contact pose, so it takes
	// the run's new grip: left on the values it was posed from, the
	// handoff out of run-state would jump.
	"run-to-idle-anim": {
		blade:  []float64{20, 50, 50, 40, 0},
		armN:   []float64{38, 20, 20, 20, 20},
		elbowN: []float64{-102, -83.15, -84.89, -75.32, -77.22},
		head:   []float64{-10, -10, -10, 10, 10},
	},
	"jump-anim":      {blade: []float64{91}, head: []float64{0}},
	"fall-anim":      {blade: []float64{26}, head: []float64{0}},
	"fall-loop-anim": {blade: []float64{65}, head: []float64{-8}},
}

// perKey reads a swordHold field: one value covers every key, a list is
// indexed, and an empty list means the field was not authored.
func perKey(vs []float64, i int) (float64, bool) {
	switch {
	case len(vs) == 0:
		return 0, false
	case len(vs) == 1:
		return vs[0], true
	}
	return vs[min(i, len(vs)-1)], true
}

func carry(d clipDef) clipDef {
	h := swordHolds[d.name]
	from, to := holdArc(d.name)
	keys := make([]kf, len(d.keys))
	for i, key := range d.keys {
		p := key.p
		armF, elbowF := 2.0, 72.0
		if p.flip {
			armF, elbowF = -armF, -elbowF
		}
		p.armF, p.elbowF = armF, elbowF
		if blade, ok := perKey(h.blade, i); ok {
			p.blade = blade
		} else {
			world := from + (to-from)*key.t/d.frames
			p.blade = world - p.rot - armF - elbowF
		}
		if head, ok := perKey(h.head, i); ok {
			p.head = head
		} else {
			p = eyesUp(p)
		}
		p = held(p)
		if armN, ok := perKey(h.armN, i); ok {
			p.armN = armN
		}
		if elbowN, ok := perKey(h.elbowN, i); ok {
			p.elbowN = elbowN
		}
		key.p = p
		keys[i] = key
	}
	return clipDef{name: d.name, frames: d.frames, keys: keys}
}

// carriedRest is the pose an attack starts and ends on: exactly the
// carry, so a swing picks up from where idle left the character.
func carriedRest() pose {
	return lowGrip(base().lean(4).knees(3, 3), 2, 72, 45)
}

// --- The two-handed grip ---
//
// Segment lengths in body space: the elbow is 19px down the upper arm
// (its forearm attach less its anchor), the hand 22px down the forearm
// (the sword's attach less the forearm anchor).
const (
	upperArmLen = 19.0
	forearmLen  = 22.0
	// The second hand takes the handle above the first, toward the pommel.
	gripOffset = 6.0
)

// limbDir is the unit vector a limb at rotation deg points along, in the
// renderer's convention: zero hangs straight down, negative swings
// forward (+x).
func limbDir(deg float64) (float64, float64) {
	r := deg * math.Pi / 180
	return -math.Sin(r), math.Cos(r)
}

// armRoot is where a shoulder actually sits for a pose: its rest attach
// plus whatever the pose slides it by, and mirrored when a turn trades
// the two arms' sides. Everything that needs a shoulder position — the
// clip builder, the preview compositor, the two-handed grip solve — goes
// through here, so a moved shoulder cannot come apart from the hand
// that is supposed to be on the hilt.
func armRoot(far bool, p pose) [2]float64 {
	pt, ox, oy := upperArmN, p.shNX, p.shNY
	if far {
		pt, ox, oy = upperArmF, p.shFX, p.shFY
	}
	pos := [2]float64{pt.pos[0] + ox, pt.pos[1] + oy}
	if armsSwapped(p) {
		pos = bodyMirrorX(pos)
	}
	return pos
}

// predVaries reports whether a pose predicate changes over the clip.
func predVaries(keys []kf, pred func(pose) bool) bool {
	for _, kf := range keys[1:] {
		if pred(kf.p) != pred(keys[0].p) {
			return true
		}
	}
	return false
}

func handAt(root [2]float64, shoulder, elbow float64) (float64, float64) {
	ux, uy := limbDir(shoulder)
	fx, fy := limbDir(shoulder + elbow)
	return root[0] + upperArmLen*ux + forearmLen*fx,
		root[1] + upperArmLen*uy + forearmLen*fy
}

// held puts the near hand on the sword handle by solving the near arm as
// a two-link chain, so a two-handed pose lands exactly instead of being
// eyeballed.
//
// The rig makes this a real constraint: the shoulders sit 42px apart and
// an arm only reaches 41px, so the hands can never meet out at arm's
// length. A two-handed pose has to keep them near the body's centerline
// and let the long blade do the reaching — which is how a two-handed
// sword is actually held. Out-of-reach poses stop the generator rather
// than draw a hand grasping at air beside the hilt.
func held(p pose) pose {
	near := armRoot(false, p)
	hx, hy := handAt(armRoot(true, p), p.armF, p.elbowF)
	bx, by := limbDir(p.armF + p.elbowF + p.blade)
	tx, ty := hx-gripOffset*bx, hy-gripOffset*by
	dx, dy := tx-near[0], ty-near[1]
	d := math.Hypot(dx, dy)
	reach := upperArmLen + forearmLen
	if d > reach || d < 1 {
		panic(fmt.Sprintf("two-handed grip is %.1fpx from the near shoulder; the arm reaches %.0f — bring the far hand toward the body centerline (arm %.0f, elbow %.0f)",
			d, reach, p.armF, p.elbowF))
	}
	deg := 180 / math.Pi
	toward := math.Atan2(-dx, dy) * deg
	shoulder := math.Acos(clamp1((upperArmLen*upperArmLen+d*d-forearmLen*forearmLen)/(2*upperArmLen*d))) * deg
	bend := 180 - math.Acos(clamp1((upperArmLen*upperArmLen+forearmLen*forearmLen-d*d)/(2*upperArmLen*forearmLen)))*deg
	// Both signs solve the triangle and mirror the elbow; take the one
	// that bends forward like every other elbow in the rig, but check the
	// solve rather than trust the derivation.
	p.armN, p.elbowN = toward+shoulder, -bend
	if !reaches(p, tx, ty) {
		p.armN, p.elbowN = toward-shoulder, bend
		if !reaches(p, tx, ty) {
			panic(fmt.Sprintf("two-link solve missed the handle at (%.1f, %.1f)", tx, ty))
		}
	}
	return p
}

func reaches(p pose, tx, ty float64) bool {
	x, y := handAt(armRoot(false, p), p.armN, p.elbowN)
	return math.Hypot(x-tx, y-ty) < 0.5
}

func clamp1(v float64) float64 { return math.Max(-1, math.Min(1, v)) }

// The five weapon clips and the sword's own kick and jump were posed by
// hand in the editor; these are transcriptions of that session, which is
// why they are literal numbers rather than derived from the two-handed
// grip solver. The solver stays in use for everything still carried —
// it just no longer has the last word on the poses someone sat and
// aimed.

// slashClip is the diagonal downward cut. The blade goes up and behind
// the head first — the windup is what makes a swing read as heavy — and
// the cut itself is two frames of linear travel through roughly 200
// degrees, so the arc never turns to mush. The hips and the stance open
// a beat ahead of the blade, and the near shoulder drives forward under
// the head's base through the cut.
func slashClip() clipDef {
	ready := base().lean(4).nod(-12).arms(-6.61, 2).elbows(-79.09, 72).
		legs(0, 0).knees(3, 3).blades(-33)
	raise := base().lean(-8).nod(-4).arms(-148.19, 140).elbows(-35.97, -8).
		legs(-4, 6).knees(4, 6).blades(-10.17)
	raiseDeep := base().at(-2, 0).lean(-12).nod(2.24).arms(-132.97, 182.12).
		elbows(-19.26, -10).legs(-6, 10).knees(4, 8).blades(-30.87).shoulders(7, 0, 0, 0)
	drive := base().at(4, 5).lean(16).nod(-24).arms(-142.69, 167.63).
		elbows(-14.83, -8).legs(10, -50).knees(0, 34).blades(-37.56).shoulders(9, 1, -1, -1)
	cut := base().at(24, 8).lean(30).squash(103, 97).nod(-25).arms(-59.92, -45.24).
		elbows(-0.67, -17.32).legs(2, -70).knees(0, 50).blades(-42.38).shoulders(21, 0, -4, -2)
	follow := base().at(22, 9).lean(28).nod(-24.83).arms(-58.87, -43.52).
		elbows(-8.57, -14.41).legs(2, -64).knees(2, 46).blades(-32.88).shoulders(23, 0, -3, -2)
	return def("slash-anim", 24,
		k(0, ready, true), k(6, raise, true), k(9, raiseDeep, true),
		k(11, drive, false), k(13, cut, false), k(17, follow, true),
		k(24, ready, true))
}

// slash2Clip answers the first cut from the other direction: a rising
// cut. Two downward cuts back to back read as the same attack played
// twice however different the numbers are; up after down is a combo.
func slash2Clip() clipDef {
	from := base().lean(4).nod(-4.08).arms(-1.29, 4.66).elbows(-37.98, 42.62).
		legs(0, 0).knees(3, 3).blades(13.73)
	sink := base().at(-4, 8).lean(-6).nod(-2).arms(-13.74, 18.33).
		elbows(-29.98, 16.86).legs(-14, 10).knees(24, 18).blades(28.52)
	// The posed value here was 366.08 — a full turn of wind-up the editor
	// had accumulated, which spun the blade right round and back inside
	// four frames. Unwrapped to the same angle.
	sinkDeep := base().at(-6, 10).lean(-10).nod(4.07).arms(5.46, 17.95).
		elbows(-62.08, 28.89).legs(-18, 12).knees(30, 22).blades(6.08)
	drive := base().at(4, 5).lean(6).nod(-6.05).arms(-29.1, -31.9).
		elbows(-24.08, 43.55).legs(-30, 22).knees(16, 6).blades(46.62).shoulders(10, 4, 0, 0)
	rise := base().at(14, 2).lean(-4).squash(97, 104).nod(-4.63).arms(-93.57, -97.25).
		elbows(-15.64, -21.5).legs(-62.61, 33.92).knees(39.75, 22.77).blades(-18.88).
		shoulders(19, -2, -5, 4)
	top := base().at(12, 16).lean(-6).nod(-3.65).arms(-121.2, -138.3).
		elbows(-5.48, -20.4).legs(-64.02, 49.69).knees(43.96, 16.13).blades(3.24).
		shoulders(25, -2, 5, 4)
	settle := base().lean(4).nod(-12).arms(-6.61, 2).elbows(-60.28, 72).
		legs(7.09, -28.01).knees(25.89, 21.83).blades(2.74)
	return def("slash-2-anim", 30,
		k(0, from, true), k(6, sink, true), k(10, sinkDeep, true),
		k(13, drive, false), k(16, rise, false), k(21, top, true),
		k(30, settle, true))
}

// thrustClip is the lunging stab. The blade stays level throughout while
// the near shoulder slides across to sit behind the far one, putting
// both arm roots on the line of the thrust so both arms can run out
// along it — from the shoulders where they rest, two arms of equal
// length can only meet straight up or straight down, never out in front.
func thrustClip() clipDef {
	ready := base().lean(4).nod(-12).arms(-6.61, 2).elbows(-79.09, 72).
		legs(0, 0).knees(3, 3).blades(-149.24)
	chamber := base().at(-4, 1).lean(-8).nod(3.23).arms(29.12, 5).
		elbows(-110.2, 80).legs(-14, 12).knees(12, 14).blades(-167)
	coil := base().at(-6, 2).lean(-11).nod(7.49).arms(33.53, 8).
		elbows(-120.76, 84).legs(-18, 14).knees(16, 16).blades(-202.02)
	lunge := base().at(0, 7).lean(10).nod(-10.72).arms(-83.06, -72.09).
		elbows(-31.22, -63.76).legs(-57, 16).knees(50, 0).blades(35.11).shoulders(22, -1, 0, -2)
	reach := base().at(2, 13).lean(11).nod(-10.5).arms(-100.64, -99.83).
		elbows(-3.35, -5.18).legs(-71.7, 33.56).knees(55.63, 5.73).blades(2.56).shoulders(36, -1, 0, -2)
	settle := base().lean(4).nod(-6.82).arms(-6.61, 2).elbows(-79.09, 72).
		legs(0, 0).knees(3, 3).blades(-176.95)
	return def("thrust-anim", 26,
		k(0, ready, true), k(6, chamber, true), k(9, coil, true),
		k(12, lunge, false), k(17, reach, false), k(26, settle, true))
}

// swordKickClip is the ground kick with the weapon still in both hands:
// the near arm swings up as a counterweight to the leg instead of
// staying on the hilt, and the far leg chambers deeper before it snaps.
func swordKickClip() clipDef {
	rest := base().nod(-8).arms(50.2, 2).elbows(-77.22, 72).
		legs(0, 0).knees(3, 3).blades(58.93)
	chamber := base().at(0, -2).lean(-10).arms(52.24, 2).elbows(-72.44, 72).
		legs(8, -55).knees(10, 85).blades(66.54)
	raised := base().at(0, -3).lean(-13).arms(75.71, 2).elbows(-70.99, 72).
		legs(8, -68).knees(10, 95).blades(65.59)
	strike := base().at(2, -3).lean(-16).arms(88.56, 2).elbows(-69.53, 72).
		legs(8, -100.67).knees(10, 5).blades(76.77)
	hold := base().at(1, -2).lean(-14).arms(42.51, 2).elbows(-70.5, 72).
		legs(8, -75).knees(10, 74.33).blades(51.27)
	return def("kick-anim", 24,
		k(0, rest, true), k(6, chamber, true), k(10, raised, true),
		k(12, strike, false), k(16, hold, true), k(24, rest, true))
}

// swordJumpClip carries the blade up rather than trailing it: swung to
// near vertical for the whole arc, with the near arm working through the
// jump instead of hanging.
func swordJumpClip() clipDef {
	rest := base().arms(23.06, 2).elbows(-96.01, 72).legs(0, 0).knees(3, 3).blades(67.97)
	crouch := base().at(0, 10).lean(4).squash(110, 100).arms(17.85, 2).
		elbows(-81.31, 72).legs(-14, 14).knees(25, 25).blades(76.3)
	launch := base().at(0, -60).arms(27.89, 2).elbows(-86.61, 72).
		legs(25, -30).knees(40.81, 15).blades(67.87).shade(58)
	apex := base().at(0, -72).arms(28.93, 2).elbows(-97.2, 72).
		legs(15, -42.63).knees(45, 58.53).blades(58.23).shade(48)
	out := base().at(0, -62).arms(44.4, 2).elbows(-103.54, 72).
		legs(24, -18).knees(57.37, 25).blades(65.16).shade(52)
	return def("jump-anim", 32,
		k(0, rest, true), k(5, crouch, true), k(12, launch, true),
		k(22, apex, true), k(32, out, true))
}

// swordGuardStance blocks with the weapon held out in front: a guard
// whose hilt sits behind the body's own centerline covers nothing and
// reads as flinching away rather than meeting the hit.
func swordGuardStance() pose {
	return base().at(6, 3).lean(10).squash(103, 98).nod(-9.92).
		arms(-56.36, -50.38).elbows(-52.01, -134.16).
		legs(-2.39, -35.35).knees(10, 28.88).blades(-32.31).shoulders(8, 2, 0, 0)
}

func swordGuardClip() clipDef {
	stance := swordGuardStance()
	// The posed breathe key drifts the far arm and the leading leg; the
	// clip's last key is the stance again rather than the third pose it
	// was left on, which sat some seventy degrees away at the far elbow
	// and snapped every time the loop came round.
	breathe := base().at(0, 2).lean(10).squash(103, 98).nod(-8.29).
		arms(-61.86, -25.9).elbows(-53.83, -134.72).
		legs(10.69, -46.3).knees(10, 46.7).blades(-33.77).shoulders(8, 2, 0, 0)
	return def("guard-anim", 32,
		k(0, stance, true), k(16, breathe, true), k(32, stance, true))
}

func swordGuardHitClip() clipDef {
	stance := swordGuardStance()
	// The hit drives the whole guard back and rocks the blade off
	// vertical; the hands stay locked on the hilt.
	pushed := lowGrip(base().at(-4, 3).lean(0).squash(103, 98).
		legs(-12, 12).knees(10, 14).shoulders(4, 2, 0, 0).fade(50), -42, 118, 212)
	back := stance.at(2, 3)
	return def("guard-hit-anim", 16,
		k(0, stance, true), k(4, pushed, false), k(8, back, true), k(16, stance, true))
}

// chibiSwordDefs is the sword preset: the same locomotion, air and
// reaction vocabulary carried with a blade in hand, the ground kick
// kept, and the punches dropped — a two-handed swordsman never lets go
// of the hilt to throw one. In their place go three weapon attacks and
// a guard that blocks with the blade.
// swordIdleClip replaces the unarmed idle outright, because posing it in
// the editor turned it into a different stance: feet staggered rather
// than together, weight rocking back as the knees flex, and the far arm
// working with the near one instead of hanging still. Only the breathing
// beat survives from chibi-male's version, so there is nothing left to
// inherit.
func swordIdleClip() clipDef {
	rest := base().arms(-7, 2).elbows(-44, 46.34).
		legs(23, -29).knees(-2, 21).blades(14)
	sink := base().at(0, 2).arms(17.27, 22.23).elbows(-70.97, 12.1).
		legs(19.52, -46.49).knees(24.38, 46.26).blades(37.7)
	return def("idle-anim", 96,
		k(0, rest, true), k(48, sink, true), k(96, rest, true))
}

func chibiSwordDefs() []clipDef {
	// jump and the ground kick are posed for the sword rather than
	// carried: both work the near arm off the hilt, so there is nothing
	// left for the carry to derive.
	defs := carried(
		idleTurnClip(), walkClip(), walkTurnClip(),
		runClip(), runToIdleClip(),
		slideClip(), fallClip(), fallLoopClip(),
		hurtClip(), deathClip(),
		jumpKickClip(),
	)
	defs = append(defs, swordJumpClip(), swordKickClip())
	// The idle is authored for the sword, not carried, so it goes in
	// ahead of the rest — the preview sheet reads in this order.
	defs = append([]clipDef{swordIdleClip()}, defs...)
	return append(defs,
		slashClip(), slash2Clip(), thrustClip(),
		swordGuardClip(), swordGuardHitClip())
}

// chibiMaleDefs is every clip in the preset, in preview-sheet row order.
func chibiMaleDefs() []clipDef {
	return []clipDef{
		idleClip(), idleTurnClip(), walkClip(), walkTurnClip(),
		runClip(), runToIdleClip(),
		slideClip(), jumpClip(), fallClip(), fallLoopClip(),
		hurtClip(), deathClip(), punchClip(), punch2Clip(),
		kickClip(), kick2Clip(), jumpKickClip(), guardClip(), guardHitClip(),
	}
}

// clipsOf renders the defs to documents, keyed by animation id.
func clipsOf(defs []clipDef) map[string]obj {
	m := map[string]obj{}
	for _, d := range defs {
		m[d.name] = clip(d.name, d.frames, d.keys)
	}
	return m
}
