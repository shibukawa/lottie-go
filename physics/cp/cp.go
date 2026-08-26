// Package lottiecp turns a lottie-go CPBody definition — the rigid body
// silhouette an editor placed over a character — into a jakecoffman/cp body
// ready to drop into a Space. It lives in its own module so the core
// library keeps its no-dependency promise.
//
// Coordinates pass through untouched: the shapes land in the animation's
// coordinate space (y down). A game that runs cp with y-up gravity flips
// the gravity sign or transforms when drawing, exactly as it would for any
// screen-space physics.
package lottiecp

import (
	cp "github.com/jakecoffman/cp/v2"
	lottie "github.com/shibukawa/lottie-go"
)

// NewBody builds the body and its shapes without adding them to a space,
// for callers that pool or post-process. Most games want AddToSpace.
//
// A dynamic body with Moment zero gets one derived from its shapes, with
// the mass split evenly among them.
func NewBody(def *lottie.CPBody) (*cp.Body, []*cp.Shape) {
	var body *cp.Body
	switch def.Type {
	case lottie.CPBodyStatic:
		body = cp.NewStaticBody()
	case lottie.CPBodyKinematic:
		body = cp.NewKinematicBody()
	default:
		mass := def.Mass
		if mass <= 0 {
			mass = 1
		}
		moment := def.Moment
		if moment == 0 {
			moment = derivedMoment(mass, def.Shapes)
		}
		body = cp.NewBody(mass, moment)
	}

	shapes := make([]*cp.Shape, 0, len(def.Shapes))
	for _, s := range def.Shapes {
		shape := newShape(body, s)
		if shape == nil {
			continue
		}
		shape.SetFriction(s.Friction)
		shape.SetElasticity(s.Elasticity)
		shape.SetSensor(s.Sensor)
		shapes = append(shapes, shape)
	}
	return body, shapes
}

// AddToSpace builds the body and registers it and every shape with the
// space, which is all a game needs before calling body.SetPosition.
func AddToSpace(space *cp.Space, def *lottie.CPBody) (*cp.Body, []*cp.Shape) {
	body, shapes := NewBody(def)
	space.AddBody(body)
	for _, s := range shapes {
		space.AddShape(s)
	}
	return body, shapes
}

func newShape(body *cp.Body, s lottie.CPShape) *cp.Shape {
	switch s.Type {
	case lottie.CPShapeCircle:
		return cp.NewCircle(body, s.Radius, cp.Vector{X: s.Center.X, Y: s.Center.Y})
	case lottie.CPShapeBox:
		return cp.NewBox2(body, boxBB(s), s.Radius)
	case lottie.CPShapePolygon:
		verts := vectors(s.Vertices)
		if len(verts) < 3 {
			return nil
		}
		// NewPolyShape computes the convex hull, so a hand-authored vertex
		// order cannot produce an inside-out shape.
		return cp.NewPolyShape(body, len(verts), verts, cp.NewTransformIdentity(), s.Radius)
	}
	// An unknown type is future data, not an error; the other shapes still
	// stand.
	return nil
}

func boxBB(s lottie.CPShape) cp.BB {
	return cp.NewBB(
		s.Center.X-s.Width/2, s.Center.Y-s.Height/2,
		s.Center.X+s.Width/2, s.Center.Y+s.Height/2,
	)
}

func vectors(ps []lottie.PhysPoint) []cp.Vector {
	out := make([]cp.Vector, len(ps))
	for i, p := range ps {
		out[i] = cp.Vector{X: p.X, Y: p.Y}
	}
	return out
}

func derivedMoment(mass float64, shapes []lottie.CPShape) float64 {
	n := 0
	for _, s := range shapes {
		if s.Type == lottie.CPShapeCircle || s.Type == lottie.CPShapeBox ||
			(s.Type == lottie.CPShapePolygon && len(s.Vertices) >= 3) {
			n++
		}
	}
	if n == 0 {
		return cp.INFINITY
	}
	part := mass / float64(n)
	total := 0.0
	for _, s := range shapes {
		switch s.Type {
		case lottie.CPShapeCircle:
			total += cp.MomentForCircle(part, 0, s.Radius, cp.Vector{X: s.Center.X, Y: s.Center.Y})
		case lottie.CPShapeBox:
			total += cp.MomentForBox2(part, boxBB(s))
		case lottie.CPShapePolygon:
			if verts := vectors(s.Vertices); len(verts) >= 3 {
				total += cp.MomentForPoly(part, len(verts), verts, cp.Vector{}, s.Radius)
			}
		}
	}
	return total
}
