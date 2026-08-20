package lottie

import (
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// bezierShape is a cubic bezier contour. V holds absolute vertices, I and O
// hold in/out tangents relative to their vertex, matching the Lottie "ks"
// shape encoding.
type bezierShape struct {
	Closed  bool
	V, I, O [][2]float64
}

// appendToPath adds the contour to p with every control point transformed by m.
func (b *bezierShape) appendToPath(p *vector.Path, m matrix) {
	n := len(b.V)
	if n == 0 {
		return
	}
	x, y := m.apply(b.V[0][0], b.V[0][1])
	p.MoveTo(float32(x), float32(y))
	segs := n - 1
	if b.Closed {
		segs = n
	}
	for s := 0; s < segs; s++ {
		j := s
		k := (s + 1) % n
		c1x := b.V[j][0]
		c1y := b.V[j][1]
		if j < len(b.O) {
			c1x += b.O[j][0]
			c1y += b.O[j][1]
		}
		c2x := b.V[k][0]
		c2y := b.V[k][1]
		if k < len(b.I) {
			c2x += b.I[k][0]
			c2y += b.I[k][1]
		}
		x1, y1 := m.apply(c1x, c1y)
		x2, y2 := m.apply(c2x, c2y)
		x3, y3 := m.apply(b.V[k][0], b.V[k][1])
		p.CubicTo(float32(x1), float32(y1), float32(x2), float32(y2), float32(x3), float32(y3))
	}
	if b.Closed {
		p.Close()
	}
}

// kappa approximates a quarter circle with one cubic segment.
const kappa = 0.5522847498307936

// ellipseShape builds a bezier ellipse centered at (cx, cy).
func ellipseShape(dst *bezierShape, cx, cy, rx, ry float64) {
	ox := rx * kappa
	oy := ry * kappa
	dst.Closed = true
	// Start at top, clockwise (matches Lottie's default direction).
	dst.V = append(dst.V[:0],
		[2]float64{cx, cy - ry},
		[2]float64{cx + rx, cy},
		[2]float64{cx, cy + ry},
		[2]float64{cx - rx, cy},
	)
	dst.O = append(dst.O[:0],
		[2]float64{ox, 0},
		[2]float64{0, oy},
		[2]float64{-ox, 0},
		[2]float64{0, -oy},
	)
	dst.I = append(dst.I[:0],
		[2]float64{-ox, 0},
		[2]float64{0, -oy},
		[2]float64{ox, 0},
		[2]float64{0, oy},
	)
}

// rectShape builds a (rounded) rectangle centered at (cx, cy).
func rectShape(dst *bezierShape, cx, cy, w, h, r float64) {
	hw, hh := w/2, h/2
	if r < 0 {
		r = 0
	}
	if r > hw {
		r = hw
	}
	if r > hh {
		r = hh
	}
	dst.Closed = true
	if r == 0 {
		dst.V = append(dst.V[:0],
			[2]float64{cx + hw, cy - hh},
			[2]float64{cx + hw, cy + hh},
			[2]float64{cx - hw, cy + hh},
			[2]float64{cx - hw, cy - hh},
		)
		dst.I = append(dst.I[:0], [2]float64{}, [2]float64{}, [2]float64{}, [2]float64{})
		dst.O = append(dst.O[:0], [2]float64{}, [2]float64{}, [2]float64{}, [2]float64{})
		return
	}
	o := r * kappa
	// Clockwise from the start of the top-right corner arc.
	dst.V = append(dst.V[:0],
		[2]float64{cx + hw - r, cy - hh}, // top edge end, before top-right arc
		[2]float64{cx + hw, cy - hh + r}, // right edge start
		[2]float64{cx + hw, cy + hh - r}, // right edge end
		[2]float64{cx + hw - r, cy + hh}, // bottom edge start
		[2]float64{cx - hw + r, cy + hh}, // bottom edge end
		[2]float64{cx - hw, cy + hh - r}, // left edge start
		[2]float64{cx - hw, cy - hh + r}, // left edge end
		[2]float64{cx - hw + r, cy - hh}, // top edge start
	)
	dst.O = append(dst.O[:0],
		[2]float64{o, 0}, [2]float64{},
		[2]float64{0, o}, [2]float64{},
		[2]float64{-o, 0}, [2]float64{},
		[2]float64{0, -o}, [2]float64{},
	)
	// In-tangents point backward along the direction of travel at each
	// corner's exit vertex.
	dst.I = append(dst.I[:0],
		[2]float64{}, [2]float64{0, -o},
		[2]float64{}, [2]float64{o, 0},
		[2]float64{}, [2]float64{0, o},
		[2]float64{}, [2]float64{-o, 0},
	)
}
