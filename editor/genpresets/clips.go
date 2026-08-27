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
	bx, by         float64 // body offset from rest (px, +y down)
	rot            float64 // body lean
	sx, sy         float64 // body scale % (squash and stretch)
	head           float64 // head rotation on the neck
	armN, armF     float64 // shoulder rotation
	elbowN, elbowF float64 // forearm rotation relative to the upper arm
	legN, legF     float64 // hip rotation
	kneeN, kneeF   float64 // shin rotation relative to the thigh
	alpha          float64 // whole-character opacity % (hurt flash)
	shadow         float64 // ground shadow scale % (reads as height)
	flip           bool    // turned: limb attach sides swap, head/shins x-mirror
}

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

// flipTrack is a two-state property driven by pose.flip: off / on values
// with hold keyframes, so the switch is instant — a turn shows no
// morphing in between, the sides just trade places at the midpoint.
func flipTrack(keys []kf, off, on []float64) obj {
	animated := false
	for _, kf := range keys[1:] {
		if kf.p.flip != keys[0].p.flip {
			animated = true
			break
		}
	}
	if !animated {
		v := off
		if keys[0].p.flip {
			v = on
		}
		return static(v)
	}
	ks := make([]obj, len(keys))
	for i, kf := range keys {
		v := off
		if kf.p.flip {
			v = on
		}
		ks[i] = holdKey(kf.t, v)
	}
	return anim(ks...)
}

// bodyMirrorX reflects an attach point across the body's vertical center.
func bodyMirrorX(pos [2]float64) [2]float64 {
	return [2]float64{48 - pos[0], pos[1]}
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
	seg := func(p part, swapSides, mirror bool, get func(pose) float64) obj {
		pos := obj(static([]float64{p.pos[0], p.pos[1]}))
		if swapSides {
			m := bodyMirrorX(p.pos)
			pos = flipTrack(keys, []float64{p.pos[0], p.pos[1]}, []float64{m[0], m[1]})
		}
		scale := obj(static([]float64{100, 100}))
		if mirror {
			scale = flipTrack(keys, []float64{100, 100}, []float64{-100, 100})
		}
		return transform(
			static([]float64{p.anchor[0], p.anchor[1]}),
			pos,
			scale,
			track(keys, scalar(get)),
			track(keys, scalar(func(p pose) float64 { return p.alpha })),
		)
	}
	bodyTr := transform(
		static([]float64{bodyPart.anchor[0], bodyPart.anchor[1]}),
		track(keys, func(p pose) []float64 { return []float64{restX + p.bx, restY + p.by} }),
		track(keys, func(p pose) []float64 { return []float64{p.sx, p.sy} }),
		track(keys, scalar(func(p pose) float64 { return p.rot })),
		track(keys, scalar(func(p pose) float64 { return p.alpha })),
	)
	shadowTr := transform(
		static([]float64{shadowPart.anchor[0], shadowPart.anchor[1]}),
		static([]float64{shadowPart.pos[0], shadowPart.pos[1]}),
		track(keys, func(p pose) []float64 { return []float64{p.shadow, p.shadow} }),
		static(0.0),
		static(28.0),
	)
	return doc(name, frames, []obj{
		imgLayer("forearm-near", 1, upperArmNInd, frames, "forearm-near", seg(forearmN, false, false, func(p pose) float64 { return p.elbowN })),
		imgLayer("upper-arm-near", upperArmNInd, bodyInd, frames, "upper-arm-near", seg(upperArmN, true, false, func(p pose) float64 { return p.armN })),
		imgLayer("head", 3, bodyInd, frames, "head", seg(headPart, false, true, func(p pose) float64 { return p.head })),
		imgLayer("shin-near", 4, thighNInd, frames, "shin-near", seg(shinNearPart, false, true, func(p pose) float64 { return p.kneeN })),
		imgLayer("thigh-near", thighNInd, bodyInd, frames, "thigh-near", seg(thighN, true, false, func(p pose) float64 { return p.legN })),
		imgLayer("body", bodyInd, 0, frames, "body", bodyTr),
		imgLayer("thigh-far", thighFInd, bodyInd, frames, "thigh-far", seg(thighF, true, false, func(p pose) float64 { return p.legF })),
		imgLayer("shin-far", 8, thighFInd, frames, "shin-far", seg(shinFarPart, false, true, func(p pose) float64 { return p.kneeF })),
		imgLayer("upper-arm-far", upperArmFInd, bodyInd, frames, "upper-arm-far", seg(upperArmF, true, false, func(p pose) float64 { return p.armF })),
		imgLayer("forearm-far", 10, upperArmFInd, frames, "forearm-far", seg(forearmF, false, false, func(p pose) float64 { return p.elbowF })),
		imgLayer("shadow", 11, 0, frames, "shadow", shadowTr),
	})
}

// --- Locomotion ---

func idleClip() clipDef {
	rest := base().elbows(-8, -8).knees(3, 3)
	up := rest.at(0, -5).nod(-3).arms(4, -4).elbows(-12, -12)
	return def("idle-anim", 96,
		k(0, rest, true), k(48, up, true), k(96, rest, true))
}

// Turn clips actually turn: the shoulders swing around, and at the
// midpoint the limb chains trade sides while the head and shoes mirror —
// an instant swap on hold keyframes, no morphing in between. The clip
// therefore ENDS facing the other way; the game flips its Mirrored flag
// when the turn completes, and the mirrored idle matches the end pose.
func idleTurnClip() clipDef {
	rest := base().elbows(-8, -8).knees(3, 3)
	windup := base().lean(-6).arms(25, -25).elbows(-18, -18).legs(-8, 8).knees(8, 8).nod(-4).at(0, -2)
	swapped := base().lean(6).arms(-25, 25).elbows(-18, -18).legs(8, -8).knees(8, 8).at(0, -2).turned()
	return def("idle-turn-anim", 20,
		k(0, rest, true), k(8, windup, true), k(10, swapped, true), k(20, rest.turned(), true))
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
	gather := base().legs(-8, 8).knees(16, 16).arms(24, -24).elbows(-16, -16).lean(-5).at(0, -3)
	swapped := base().legs(8, -8).knees(16, 16).arms(-24, 24).elbows(-16, -16).lean(5).at(0, -3).turned()
	out := base().legs(22, -22).knees(20, 8).arms(-18, 18).elbows(-14, -14).lean(4).turned()
	return def("walk-turn-anim", 20,
		k(0, from, true), k(8, gather, true), k(10, swapped, true), k(20, out, true))
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
	brake := base().legs(-15, 15).knees(30, 30).arms(20, -20).elbows(-30, -30).lean(-8).at(2, -2)
	swapped := base().legs(15, -15).knees(30, 30).arms(-20, 20).elbows(-30, -30).lean(8).at(-2, -2).turned()
	out := base().legs(45, -45).knees(55, 15).arms(-35, 35).elbows(-40, -40).lean(12).turned()
	return def("run-turn-anim", 16,
		k(0, from, true), k(6, brake, true), k(8, swapped, true), k(16, out, true))
}

func runToIdleClip() clipDef {
	from := base().legs(-50, 50).knees(15, 70).arms(40, -40).elbows(-40, -40).lean(12)
	brake := base().at(3, 2).lean(-14).legs(-38, 22).knees(10, 25).arms(18, -12).elbows(-15, -15).nod(-5)
	settle := base().lean(-4).legs(-12, 12).knees(6, 6).elbows(-10, -10)
	rest := base().elbows(-8, -8).knees(3, 3)
	return def("run-to-idle-anim", 24,
		k(0, from, true), k(8, brake, true), k(16, settle, true), k(24, rest, true))
}

// slideClip is a baseball slide: drop almost horizontal, lead leg
// extended, trailing knee tucked, near arm planted behind for balance.
// The character stays in place — horizontal travel is the game's job —
// so what animates is the drop, the held low pose, and the recovery.
func slideClip() clipDef {
	from := base().legs(-50, 50).knees(15, 70).arms(40, -40).elbows(-40, -40).lean(12)
	drop := base().at(-2, 16).lean(-38).legs(-52, -30).knees(45, 12).arms(50, 25).elbows(-25, -15).nod(14).squash(102, 98)
	low := base().at(-4, 26).lean(-62).legs(-45, -22).knees(55, 8).arms(62, 30).elbows(-30, -12).nod(24).squash(104, 96)
	up := base().lean(-10).legs(-28, 28).knees(12, 20).arms(10, -10).elbows(-15, -15)
	return def("slide-anim", 28,
		k(0, from, true), k(4, drop, false), k(7, low, true), k(20, low, true), k(28, up, true))
}

// --- Air ---

func jumpClip() clipDef {
	crouch := base().at(0, 10).squash(110, 88).lean(4).legs(-14, 14).knees(25, 25).arms(18, -18).elbows(-20, -20)
	launch := base().at(0, -60).squash(94, 106).legs(-30, 25).knees(15, 45).arms(-70, 70).elbows(-25, -25).shade(58)
	apex := base().at(0, -72).legs(-15, 15).knees(30, 45).arms(-80, 80).elbows(-20, -20).shade(48)
	out := base().at(0, -62).legs(-18, 24).knees(25, 40).arms(-80, 80).elbows(-20, -20).shade(52)
	return def("jump-anim", 32,
		k(0, base().elbows(-8, -8).knees(3, 3), true), k(5, crouch, true), k(12, launch, true),
		k(22, apex, true), k(32, out, true))
}

func fallClip() clipDef {
	from := base().at(0, -62).arms(-80, 80).elbows(-20, -20).legs(-20, 32).knees(20, 35).shade(52)
	settle := base().at(0, -38).arms(-70, 70).elbows(-25, -25).legs(-15, 26).knees(25, 30).shade(58).nod(5)
	return def("fall-anim", 16,
		k(0, from, true), k(16, settle, true))
}

func fallLoopClip() clipDef {
	a := base().at(0, -38).arms(-70, 70).elbows(-25, -25).legs(-15, 26).knees(25, 30).shade(58).nod(5)
	b := base().at(0, -44).arms(-80, 80).elbows(-18, -18).legs(-25, 16).knees(32, 24).shade(54).nod(8)
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

func punch2Clip() clipDef {
	from := base().at(4, 0).lean(8).arms(20, -40).elbows(-40, -15).legs(-12, 12).knees(8, 12)
	windup := base().lean(-2).arms(35, -20).elbows(-75, -20).legs(-12, 12).knees(8, 12)
	strike := base().at(8, 0).lean(16).squash(102, 98).arms(-100, 15).elbows(-3, -25).legs(-16, 14).knees(6, 14)
	hold := base().at(6, 0).lean(14).arms(-92, 12).elbows(-6, -25).legs(-16, 14).knees(6, 14)
	rest := base().elbows(-8, -8).knees(3, 3)
	return def("punch-2-anim", 24,
		k(0, from, true), k(5, windup, true), k(9, strike, false),
		k(14, hold, true), k(24, rest, true))
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

func jumpKickClip() clipDef {
	air := base().at(0, -48).arms(-60, 60).elbows(-25, -25).legs(-15, 22).knees(25, 28).shade(55)
	tuck := base().at(0, -45).lean(8).legs(25, -45).knees(40, 80).arms(-40, 40).elbows(-30, -30).shade(55)
	strike := base().at(3, -42).lean(12).legs(30, -72).knees(45, 5).arms(45, -50).elbows(-25, -25).shade(55)
	out := base().at(0, -40).arms(-60, 60).elbows(-25, -25).legs(-18, 26).knees(28, 30).shade(57)
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
		kickClip(), jumpKickClip(), guardClip(), guardHitClip(),
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
