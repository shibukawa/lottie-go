package lottie

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// matrix is a 2D affine transform:
//
//	| A C TX |
//	| B D TY |
type matrix struct {
	A, B, C, D, TX, TY float64
}

var identityMatrix = matrix{A: 1, D: 1}

func (m matrix) mul(n matrix) matrix {
	return matrix{
		A:  m.A*n.A + m.C*n.B,
		B:  m.B*n.A + m.D*n.B,
		C:  m.A*n.C + m.C*n.D,
		D:  m.B*n.C + m.D*n.D,
		TX: m.A*n.TX + m.C*n.TY + m.TX,
		TY: m.B*n.TX + m.D*n.TY + m.TY,
	}
}

func (m matrix) apply(x, y float64) (float64, float64) {
	return m.A*x + m.C*y + m.TX, m.B*x + m.D*y + m.TY
}

func (m matrix) translate(x, y float64) matrix {
	return m.mul(matrix{A: 1, D: 1, TX: x, TY: y})
}

func (m matrix) scale(sx, sy float64) matrix {
	return m.mul(matrix{A: sx, D: sy})
}

func (m matrix) rotate(rad float64) matrix {
	sin, cos := math.Sincos(rad)
	return m.mul(matrix{A: cos, B: sin, C: -sin, D: cos})
}

// skew applies a Lottie skew: skew amount (radians) along the given axis (radians).
func (m matrix) skew(sk, axis float64) matrix {
	if sk == 0 {
		return m
	}
	m = m.rotate(-axis)
	m = m.mul(matrix{A: 1, C: math.Tan(sk), D: 1})
	return m.rotate(axis)
}

// meanScale reports the average scale factor of the matrix, used to scale
// stroke widths the way Lottie specifies.
func (m matrix) meanScale() float64 {
	det := math.Abs(m.A*m.D - m.B*m.C)
	return math.Sqrt(det)
}

func (m matrix) toGeoM() ebiten.GeoM {
	var g ebiten.GeoM
	g.SetElement(0, 0, m.A)
	g.SetElement(0, 1, m.C)
	g.SetElement(0, 2, m.TX)
	g.SetElement(1, 0, m.B)
	g.SetElement(1, 1, m.D)
	g.SetElement(1, 2, m.TY)
	return g
}

// invert returns the inverse transform; ok is false for a singular matrix.
func (m matrix) invert() (matrix, bool) {
	det := m.A*m.D - m.B*m.C
	if math.Abs(det) < 1e-12 {
		return identityMatrix, false
	}
	inv := 1 / det
	return matrix{
		A:  m.D * inv,
		B:  -m.B * inv,
		C:  -m.C * inv,
		D:  m.A * inv,
		TX: (m.C*m.TY - m.D*m.TX) * inv,
		TY: (m.B*m.TX - m.A*m.TY) * inv,
	}, true
}

func matrixFromGeoM(g ebiten.GeoM) matrix {
	return matrix{
		A:  g.Element(0, 0),
		C:  g.Element(0, 1),
		TX: g.Element(0, 2),
		B:  g.Element(1, 0),
		D:  g.Element(1, 1),
		TY: g.Element(1, 2),
	}
}
