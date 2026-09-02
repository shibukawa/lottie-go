package main

// Turning a cut part into a mesh: trace its silhouette, thin the trace
// down to a workable number of points, and read a UV off each one.
//
// The UV is where the whole approach pays off. It is fixed to the art and
// never animates; only the outline moves. So a limb is one part that
// bends, not a thigh and a shin that pivot, and the drawing bends with
// it — armour plates, straps, piping and all.

import (
	"image"
	"math"
)

type pt struct{ x, y float64 }

// trace walks the alpha boundary from the topmost opaque pixel, keeping a
// wall on one side, and returns the closed contour.
func trace(img image.Image) []pt {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	solid := func(x, y int) bool {
		if x < 0 || y < 0 || x >= w || y >= h {
			return false
		}
		_, _, _, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
		return a>>8 >= 128
	}
	sx, sy := -1, -1
	for y := 0; y < h && sy < 0; y++ {
		for x := range w {
			if solid(x, y) {
				sx, sy = x, y
				break
			}
		}
	}
	if sy < 0 {
		return nil
	}
	dirs := [8][2]int{{-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}}
	out := []pt{{float64(sx), float64(sy)}}
	cx, cy, back := sx, sy, 0
	for range w * h * 8 {
		found := false
		for i := range 8 {
			d := dirs[(back+1+i)%8]
			nx, ny := cx+d[0], cy+d[1]
			if !solid(nx, ny) {
				continue
			}
			for j := range 8 {
				if dirs[j][0] == cx-nx && dirs[j][1] == cy-ny {
					back = j
					break
				}
			}
			cx, cy = nx, ny
			out = append(out, pt{float64(cx), float64(cy)})
			found = true
			break
		}
		if !found || (cx == sx && cy == sy && len(out) > 2) {
			break
		}
	}
	return out
}

// simplify thins a contour to n points by repeatedly dropping the one
// whose removal moves the outline least. Keeping the points where the
// silhouette actually turns is what stops a bend from looking faceted at
// the joint while wasting points down a straight shin.
func simplify(c []pt, n int) []pt {
	pts := append([]pt(nil), c...)
	area := func(a, b, d pt) float64 {
		return math.Abs((b.x-a.x)*(d.y-a.y)-(d.x-a.x)*(b.y-a.y)) / 2
	}
	for len(pts) > n {
		worst, at := math.Inf(1), 1
		for i := range pts {
			a, b, d := pts[(i-1+len(pts))%len(pts)], pts[i], pts[(i+1)%len(pts)]
			if v := area(a, b, d); v < worst {
				worst, at = v, i
			}
		}
		pts = append(pts[:at], pts[at+1:]...)
	}
	return pts
}

// mesh is one part ready to be drawn: an outline in the part image's own
// pixels, and the UV each of those points reads the art at.
type mesh struct {
	outline []pt
	uv      []pt
}

func meshOf(img image.Image, points int) mesh {
	c := trace(img)
	if len(c) == 0 {
		return mesh{}
	}
	if len(c) > points {
		c = simplify(c, points)
	}
	b := img.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	uv := make([]pt, len(c))
	for i, p := range c {
		uv[i] = pt{p.x / w, p.y / h}
	}
	return mesh{outline: c, uv: uv}
}
