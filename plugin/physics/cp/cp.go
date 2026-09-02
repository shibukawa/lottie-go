package lottiecp

import (
	"math"

	cp "github.com/jakecoffman/cp/v2"
)

// Build turns a body definition into a cp body with its shapes attached
// but not yet added to a space, for callers that pool or post-process.
// Most games want AddToSpace.
//
// A dynamic body with Moment zero gets one derived from its shapes, with
// the mass split evenly among them. Shapes without extent — a circle of
// radius zero, a box with no width — derive no moment; the body then gets
// cp.INFINITY, like a body without shapes, rather than a zero that would
// make its inverse infinite.
func Build(def *Body) (*cp.Body, []*cp.Shape) {
	var body *cp.Body
	switch def.Type {
	case BodyStatic:
		body = cp.NewStaticBody()
	case BodyKinematic:
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
func AddToSpace(space *cp.Space, def *Body) (*cp.Body, []*cp.Shape) {
	body, shapes := Build(def)
	space.AddBody(body)
	for _, s := range shapes {
		space.AddShape(s)
	}
	return body, shapes
}

func newShape(body *cp.Body, s Shape) *cp.Shape {
	switch s.Type {
	case ShapeCircle:
		return cp.NewCircle(body, s.Radius, cp.Vector{X: s.Center.X, Y: s.Center.Y})
	case ShapeBox:
		return cp.NewBox2(body, boxBB(s), s.Radius)
	case ShapePolygon:
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

func boxBB(s Shape) cp.BB {
	return cp.NewBB(
		s.Center.X-s.Width/2, s.Center.Y-s.Height/2,
		s.Center.X+s.Width/2, s.Center.Y+s.Height/2,
	)
}

func vectors(ps []Point) []cp.Vector {
	out := make([]cp.Vector, len(ps))
	for i, p := range ps {
		out[i] = cp.Vector{X: p.X, Y: p.Y}
	}
	return out
}

// MirrorX returns a deep copy of the definition reflected across the
// vertical line x = axis — the facing flip for a left-facing character.
// Centers and vertices mirror; convexity survives because Build recomputes
// the polygon hull anyway.
func MirrorX(def *Body, axis float64) *Body {
	out := *def
	out.Shapes = make([]Shape, len(def.Shapes))
	for i, s := range def.Shapes {
		s.Center.X = 2*axis - s.Center.X
		if len(s.Vertices) > 0 {
			vs := make([]Point, len(s.Vertices))
			for j, v := range s.Vertices {
				vs[j] = Point{X: 2*axis - v.X, Y: v.Y}
			}
			s.Vertices = vs
		}
		out.Shapes[i] = s
	}
	return &out
}

func derivedMoment(mass float64, shapes []Shape) float64 {
	n := 0
	for _, s := range shapes {
		if s.Type == ShapeCircle || s.Type == ShapeBox ||
			(s.Type == ShapePolygon && len(s.Vertices) >= 3) {
			n++
		}
	}
	if n == 0 {
		return cp.INFINITY
	}
	if mass <= 0 || math.IsNaN(mass) || math.IsInf(mass, 0) {
		return cp.INFINITY
	}
	part := mass / float64(n)
	total := 0.0
	for _, s := range shapes {
		switch s.Type {
		case ShapeCircle:
			total += cp.MomentForCircle(part, 0, s.Radius, cp.Vector{X: s.Center.X, Y: s.Center.Y})
		case ShapeBox:
			total += cp.MomentForBox2(part, boxBB(s))
		case ShapePolygon:
			if verts := vectors(s.Vertices); len(verts) >= 3 {
				total += cp.MomentForPoly(part, len(verts), verts, cp.Vector{}, s.Radius)
			}
		}
	}
	// A moment of zero (or a non-number) is what degenerate geometry sums
	// to; cp would turn it into an infinite inverse, so treat it as "no
	// valid shapes". The negated comparison also catches NaN.
	if !(total > 0) || math.IsInf(total, 0) {
		return cp.INFINITY
	}
	return total
}
