package main

// Clips are authored as pose sequences. A pose gives every rig track one
// value; the builder turns the sequence into per-track keyframes and
// collapses tracks that never move into static properties. Rotations are
// degrees, clockwise, zero hanging straight down (limbs) or upright
// (body); negative swings a limb forward (+x), matching the right-facing
// character. Elbows bend forward (negative), knees bend backward
// (positive), like the joints they stand in for.

import "slices"

type pose struct {
	bx, by         float64  // body offset from rest (px, +y down)
	rot            float64  // body lean
	sx, sy         float64  // body scale % (squash and stretch)
	head           float64  // head rotation on the neck
	armN, armF     float64  // shoulder rotation
	elbowN, elbowF float64  // forearm rotation relative to the upper arm
	legN, legF     float64  // hip rotation
	kneeN, kneeF   float64  // shin rotation relative to the thigh
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
func (p pose) fade(a float64) pose           { p.alpha = a; return p }
func (p pose) shade(s float64) pose          { p.shadow = s; return p }
func (p pose) turned() pose                  { p.flip = true; return p }
func (p pose) away() pose                    { p.view = viewBack; return p }
func (p pose) aside() pose                   { p.view = viewSide; return p }
func (p pose) swapped() pose                 { p.swap = true; return p }
func (p pose) swappedLegs() pose             { p.swapLegs = true; return p }

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
	return doc(name, frames, []obj{
		imgLayer("forearm-near", 1, upperArmNInd, frames, "forearm-near", seg(forearmN, nil, false, func(p pose) float64 { return p.elbowN })),
		imgLayer("upper-arm-near", upperArmNInd, bodyInd, frames, "upper-arm-near", seg(upperArmN, armsSwapped, false, func(p pose) float64 { return p.armN })),
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
		imgLayer("upper-arm-far", upperArmFInd, bodyInd, frames, "upper-arm-far", seg(upperArmF, armsSwapped, false, func(p pose) float64 { return p.armF })),
		imgLayer("forearm-far", 10, upperArmFInd, frames, "forearm-far", seg(forearmF, nil, false, func(p pose) float64 { return p.elbowF })),
		imgLayer("shadow", 11, 0, frames, "shadow", shadowTr),
	})
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
	rest := base().elbows(-8, -8).knees(3, 3)
	windup := base().lean(-4).arms(20, -20).elbows(-16, -16).legs(-6, 6).knees(6, 6).at(0, -1).aside()
	swapped := base().lean(4).arms(-20, 20).elbows(-16, -16).legs(6, -6).knees(6, 6).at(0, -1).aside().turned()
	settle := base().arms(-10, 10).elbows(-12, -12).legs(3, -3).knees(4, 4).turned()
	return def("idle-turn-anim", 20,
		k(0, rest, true), k(6, windup, true), k(10, swapped, true),
		k(14, settle, true), k(20, rest.turned(), true))
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

func walkTurnClip() clipDef {
	from := base().legs(-22, 22).knees(8, 20).arms(18, -18).elbows(-14, -14).lean(4)
	gather := base().legs(-8, 8).knees(16, 16).arms(24, -24).elbows(-16, -16).lean(-4).at(0, -2).aside()
	swapped := base().legs(8, -8).knees(16, 16).arms(-24, 24).elbows(-16, -16).lean(4).at(0, -2).aside().turned()
	out := base().legs(22, -22).knees(20, 8).arms(-18, 18).elbows(-14, -14).lean(4).turned()
	return def("walk-turn-anim", 20,
		k(0, from, true), k(6, gather, true), k(10, swapped, true),
		k(14, base().legs(14, -14).knees(16, 10).arms(-20, 20).elbows(-14, -14).lean(4).turned(), true),
		k(20, out, true))
}

func runClip() clipDef {
	contactA := base().legs(-50, 50).knees(15, 70).arms(40, -40).elbows(-40, -40).lean(12)
	passA := base().at(0, -10).squash(97, 103).legs(0, 0).knees(55, 55).arms(0, 0).elbows(-40, -40).lean(12)
	contactB := base().legs(50, -50).knees(70, 15).arms(-40, 40).elbows(-40, -40).lean(12)
	return def("run-anim", 24,
		k(0, contactA, false), k(6, passA, false), k(12, contactB, false),
		k(18, passA, false), k(24, contactA, false))
}

func runTurnClip() clipDef {
	from := base().legs(-45, 45).knees(15, 55).arms(35, -35).elbows(-40, -40).lean(12)
	brake := base().legs(-15, 15).knees(30, 30).arms(20, -20).elbows(-30, -30).lean(-6).at(2, -2).aside()
	swapped := base().legs(15, -15).knees(30, 30).arms(-20, 20).elbows(-30, -30).lean(6).at(-2, -2).aside().turned()
	out := base().legs(45, -45).knees(55, 15).arms(-35, 35).elbows(-40, -40).lean(12).turned()
	return def("run-turn-anim", 16,
		k(0, from, true), k(5, brake, true), k(8, swapped, true),
		k(11, base().legs(30, -30).knees(40, 20).arms(-28, 28).elbows(-34, -34).lean(10).turned(), true),
		k(16, out, true))
}

func runToIdleClip() clipDef {
	from := base().legs(-50, 50).knees(15, 70).arms(40, -40).elbows(-40, -40).lean(12)
	brake := base().at(3, 2).lean(-14).legs(-38, 22).knees(10, 25).arms(18, -12).elbows(-15, -15).nod(-5)
	settle := base().lean(-4).legs(-12, 12).knees(6, 6).elbows(-10, -10)
	rest := base().elbows(-8, -8).knees(3, 3)
	return def("run-to-idle-anim", 24,
		k(0, from, true), k(8, brake, true), k(16, settle, true), k(24, rest, true))
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
	from := base().at(4, 0).lean(6).arms(20, -40).elbows(-40, -15).legs(-12, 12).knees(8, 12)
	// The windup cocks the fist by folding the elbow — the upper arm
	// barely leaves the side, nothing pulls back — and extension only
	// begins at the spin frame.
	wind := base().lean(2).arms(10, -18).elbows(-105, -20).legs(-10, 10).knees(8, 10)
	windDeep := base().at(-1, 1).lean(4).arms(18, -20).elbows(-120, -20).legs(-10, 10).knees(8, 10)
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

// chibiMaleDefs is every clip in the preset, in preview-sheet row order.
func chibiMaleDefs() []clipDef {
	return []clipDef{
		idleClip(), idleTurnClip(), walkClip(), walkTurnClip(),
		runClip(), runTurnClip(), runToIdleClip(),
		slideClip(), jumpClip(), fallClip(), fallLoopClip(),
		hurtClip(), deathClip(), punchClip(), punch2Clip(),
		kickClip(), kick2Clip(), jumpKickClip(), guardClip(), guardHitClip(),
	}
}

// chibiMaleClips renders the defs to documents, keyed by animation id.
func chibiMaleClips() map[string]obj {
	m := map[string]obj{}
	for _, d := range chibiMaleDefs() {
		m[d.name] = clip(d.name, d.frames, d.keys)
	}
	return m
}
