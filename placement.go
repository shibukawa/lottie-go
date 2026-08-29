package lottie

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// Layer placement queries. A game attaches things to an animation — a
// weapon to a hand, a particle emitter to a muzzle — by asking where a
// named layer sits at a frame. Animators drive that layer (typically a
// null, ty 3) in their own tool, so the attachment interpolates exactly
// like the artwork; the extension packages under plugin/ build sockets and
// root motion on top of this query.

// LayerPlacement is a layer's world transform at one frame, decomposed for
// gameplay use. X, Y are where the layer's local origin lands in animation
// coordinates; Angle is the rotation of its x-axis in radians (y-down, so
// clockwise positive); ScaleX and ScaleY are the axis lengths. Visible
// reports whether the layer (and every enclosing precomp) is inside its
// active frame range and not hidden — a query outside it still returns the
// transform, so a game can decide for itself whether that matters.
type LayerPlacement struct {
	X, Y           float64
	Angle          float64
	ScaleX, ScaleY float64
	Visible        bool
}

// GeoM returns the placement as an Ebitengine matrix: rotate by Angle,
// scale, then translate — the transform that maps a small local frame at
// the layer's origin into animation coordinates. Skew is not represented;
// use it for attaching sprites, not for reproducing the layer's exact
// pixel transform.
func (p LayerPlacement) GeoM() ebiten.GeoM {
	var g ebiten.GeoM
	g.Scale(p.ScaleX, p.ScaleY)
	g.Rotate(p.Angle)
	g.Translate(p.X, p.Y)
	return g
}

// Mirrored reflects the placement across the vertical line x = axis, the
// facing-flip games apply for a left-facing character: position mirrors,
// angle negates, geometry stays right-handed for the caller to flip its
// sprite. Hitboxes mirror the same way in their own packages.
func (p LayerPlacement) Mirrored(axis float64) LayerPlacement {
	p.X = 2*axis - p.X
	p.Angle = -p.Angle
	return p
}

// LayerPlacement returns the world transform of the first layer with the
// given name at the given composition frame. Root layers are searched in
// file order, then precomp contents depth-first; the first match wins, so
// give attachment layers unique names. It reports false when no layer has
// the name.
func (a *Animation) LayerPlacement(name string, frame float64) (LayerPlacement, bool) {
	m, visible, ok := placeLayer(a.layers, name, frame, identityMatrix, true, 0)
	if !ok {
		return LayerPlacement{}, false
	}
	return decompose(m, visible), true
}

// placeLayer finds name under layers and composes its world matrix. f is
// the frame in the current composition's time; entering a precomp remaps
// it the same way rendering does.
func placeLayer(layers []*layerNode, name string, f float64, root matrix, visible bool, depth int) (matrix, bool, bool) {
	if depth > 16 {
		return matrix{}, false, false
	}
	for _, l := range layers {
		if l.name == name {
			return root.mul(layerMatrix(l, f, 0)), visible && layerActive(l, f), true
		}
	}
	for _, l := range layers {
		if l.typ != 0 || len(l.comp) == 0 {
			continue
		}
		m, vis, ok := placeLayer(l.comp, name, l.localTime(f),
			root.mul(layerMatrix(l, f, 0)), visible && layerActive(l, f), depth+1)
		if ok {
			return m, vis, true
		}
	}
	return matrix{}, false, false
}

func layerActive(l *layerNode, f float64) bool {
	return !l.hidden && f >= l.ip && f < l.op
}

// decompose splits an affine matrix into the placement members. Skew folds
// into the scale axes; games attaching items do not need it.
func decompose(m matrix, visible bool) LayerPlacement {
	return LayerPlacement{
		X:       m.TX,
		Y:       m.TY,
		Angle:   math.Atan2(m.B, m.A),
		ScaleX:  math.Hypot(m.A, m.B),
		ScaleY:  math.Hypot(m.C, m.D),
		Visible: visible,
	}
}

// LayerTransform returns the exact matrix mapping the named layer's own
// coordinates into animation coordinates, resolved the same way
// LayerPlacement resolves a name.
//
// LayerPlacement is the friendlier form and the right one for attaching a
// sprite, but its decomposition folds a mirror into a half turn: a layer
// scaled -100% reads back as positive scale and a rotation 180° away.
// Anything that must reproduce the layer's own frame — an editor drawing a
// part's outline, or converting a drag back into the layer's parent space —
// needs the matrix that was actually composed.
func (a *Animation) LayerTransform(name string, frame float64) (ebiten.GeoM, bool) {
	m, _, ok := placeLayer(a.layers, name, frame, identityMatrix, true, 0)
	if !ok {
		return ebiten.GeoM{}, false
	}
	return m.toGeoM(), true
}

// LayerNames returns every layer name reachable from the animation's root
// composition — its own layers, then precomp contents depth-first —
// deduplicated in first-seen order. Empty names are skipped. This is what
// an editor offers when binding a socket to a layer.
func (a *Animation) LayerNames() []string {
	var out []string
	seen := map[string]bool{}
	var walk func(layers []*layerNode, depth int)
	walk = func(layers []*layerNode, depth int) {
		if depth > 16 {
			return
		}
		for _, l := range layers {
			if l.name != "" && !seen[l.name] {
				seen[l.name] = true
				out = append(out, l.name)
			}
		}
		for _, l := range layers {
			if l.typ == 0 {
				walk(l.comp, depth+1)
			}
		}
	}
	walk(a.layers, 0)
	return out
}

// LayerPlacement queries the named layer at the player's current frame.
func (p *Player) LayerPlacement(name string) (LayerPlacement, bool) {
	return p.anim.LayerPlacement(name, p.frame)
}
