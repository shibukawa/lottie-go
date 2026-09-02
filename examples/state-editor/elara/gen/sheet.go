package main

// Reading the character sheets. The figures are drawn on printed graph
// paper, so getting a part out is two steps: lift the figure off the
// paper, then cut the part out of the figure.
//
// Lifting the figure cannot be a distance to one background colour — the
// grid lines run from bright beige down to a muted grey, and a tolerance
// wide enough to cross them also swallows the character's whites. What
// separates them is not colour but position: everything unsaturated and
// not dark is *candidate* paper, and only the part of it reachable from
// the crop's border is actually paper. The white coif, the ribbons and
// the underskirt are enclosed by the drawing's own linework, so
// connectivity protects them.

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"

	_ "image/jpeg"
	"image/png"
)

// figure is one view lifted off its sheet: pixels with alpha, in the
// sheet's own coordinates so cut polygons can be read straight off it.
type figure struct {
	img  *image.NRGBA
	x, y int // where the crop sits on the sheet
}

func loadSheet(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// paperish is the candidate-background test: unsaturated, not dark, and
// not bluer than it is red, which the navy habit is.
func paperish(c color.NRGBA) bool {
	hi := max(c.R, max(c.G, c.B))
	lo := min(c.R, min(c.G, c.B))
	return hi >= 140 && hi-lo <= 46 && c.R >= c.B
}

// lift crops a region of a sheet and clears the paper from it.
func lift(sheet image.Image, x0, y0, w, h int) *figure {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			r, g, b, _ := sheet.At(x0+x, y0+y).RGBA()
			img.SetNRGBA(x, y, color.NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255})
		}
	}
	seen := make([]bool, w*h)
	var stack [][2]int
	push := func(x, y int) {
		if x < 0 || y < 0 || x >= w || y >= h || seen[y*w+x] || !paperish(img.NRGBAAt(x, y)) {
			return
		}
		seen[y*w+x] = true
		stack = append(stack, [2]int{x, y})
	}
	for x := range w {
		push(x, 0)
		push(x, h-1)
	}
	for y := range h {
		push(0, y)
		push(w-1, y)
	}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		push(p[0]+1, p[1])
		push(p[0]-1, p[1])
		push(p[0], p[1]+1)
		push(p[0], p[1]-1)
	}
	for i, s := range seen {
		if s {
			img.SetNRGBA(i%w, i/w, color.NRGBA{})
		}
	}
	return &figure{img: img, x: x0, y: y0}
}

// inside reports whether a point is within a closed polygon, by the
// even-odd rule. Cut polygons are read off the sheet by eye, so they are
// allowed to be sloppy outside the figure: intersecting with the
// figure's own alpha is what gives the part its silhouette.
func inside(poly [][2]int, x, y int) bool {
	in := false
	for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
		xi, yi := float64(poly[i][0]), float64(poly[i][1])
		xj, yj := float64(poly[j][0]), float64(poly[j][1])
		if (yi > float64(y)) != (yj > float64(y)) &&
			float64(x) < (xj-xi)*(float64(y)-yi)/(yj-yi)+xi {
			in = !in
		}
	}
	return in
}

// cutAll takes every part out of the figure at once, working front to
// back. Each part claims the pixels inside its polygon that no part in
// front of it already took, so every pixel of the figure belongs to
// exactly one part.
//
// Claiming by pixel rather than by polygon is what makes the sloppy
// polygons workable: the sword's cut box lies across the veil, but the
// sword only claims where the sword is drawn, so the veil keeps the rest
// instead of gaining a rectangular hole.
func (f *figure) cutAll(order []region) (map[string]*part, error) {
	b := f.img.Bounds()
	w, h := b.Dx(), b.Dy()
	claimed := make([]bool, w*h)
	out := map[string]*part{}
	for i := len(order) - 1; i >= 0; i-- {
		r := order[i]
		minX, minY, maxX, maxY := w, h, -1, -1
		take := make([]bool, w*h)
		for y := range h {
			for x := range w {
				if claimed[y*w+x] || f.img.NRGBAAt(x, y).A < 128 {
					continue
				}
				if !inside(r.poly, x+f.x, y+f.y) {
					continue
				}
				take[y*w+x] = true
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x), max(maxY, y)
			}
		}
		if maxX < 0 {
			return nil, fmt.Errorf("%s: the cut polygon caught none of the figure", r.name)
		}
		pw, ph := maxX-minX+1, maxY-minY+1
		img := image.NewNRGBA(image.Rect(0, 0, pw, ph))
		for y := range ph {
			for x := range pw {
				sx, sy := minX+x, minY+y
				if !take[sy*w+sx] {
					continue
				}
				img.SetNRGBA(x, y, f.img.NRGBAAt(sx, sy))
				claimed[sy*w+sx] = true
			}
		}
		out[r.name] = &part{name: r.name, img: img, x: minX + f.x, y: minY + f.y}
	}
	return out, nil
}

// part is one cut piece and where it sat on the sheet, which is what
// keeps the pieces in register when they are reassembled.
type part struct {
	name string
	img  *image.NRGBA
	x, y int
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// grid writes the figure with a labelled 20px lattice over it, which is
// how the cut polygons below were read off the sheet in the first place.
func (f *figure) grid(path string) error {
	b := f.img.Bounds()
	out := image.NewNRGBA(b)
	for y := range b.Dy() {
		for x := range b.Dx() {
			c := f.img.NRGBAAt(x, y)
			if c.A == 0 {
				c = color.NRGBA{250, 250, 250, 255}
			}
			sx, sy := x+f.x, y+f.y
			switch {
			case sx%100 == 0 || sy%100 == 0:
				c = color.NRGBA{220, 40, 40, 255}
			case sx%20 == 0 || sy%20 == 0:
				c = color.NRGBA{
					uint8(math.Min(255, float64(c.R)*0.6+90)),
					uint8(math.Min(255, float64(c.G)*0.6+90)),
					uint8(math.Min(255, float64(c.B)*0.6+120)), 255}
			}
			out.SetNRGBA(x, y, c)
		}
	}
	return writePNG(path, out)
}
