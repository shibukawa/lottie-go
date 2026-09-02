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
// The floor sits low enough to take the printed guide dashes with the
// paper — they are a mid grey, well above the drawing's linework but
// well below its steel. Colours this test lets through are still only
// candidates: the character's own greys survive because the border flood
// cannot reach them through its outlines.
func paperish(c color.NRGBA) bool {
	hi := max(c.R, max(c.G, c.B))
	lo := min(c.R, min(c.G, c.B))
	return hi >= 115 && hi-lo <= 46 && c.R >= c.B
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

// liftBox lifts one boxed panel as its own figure. The face panels are
// drawn inside a frame, and the pale ground inside it is walled off from
// the sheet's border — the flood that lifts everything else cannot reach
// it. Flooding from the panel's own edge can.
func liftBox(sheet image.Image, b [][2]int) *figure {
	x0, y0 := b[0][0], b[0][1]
	x1, y1 := b[2][0], b[2][1]
	return lift(sheet, x0, y0, x1-x0, y1-y0)
}

// eraseMargin clears a band around the crop. The face panels are drawn
// inside a frame that the paper test will not take — it is as dark as
// the linework — and the face reaches the panel's bottom edge, so the
// frame cannot be told from the face by connectivity either. Cutting a
// band off the edge takes the frame and costs a few pixels of neck.
func (f *figure) eraseMargin(n int) {
	b := f.img.Bounds()
	w, h := b.Dx(), b.Dy()
	for y := range h {
		for x := range w {
			if x < n || y < n || x >= w-n || y >= h-n {
				f.img.SetNRGBA(x, y, color.NRGBA{})
			}
		}
	}
}

// keepLargest drops everything but the biggest connected run of opaque
// pixels. What is left of a face panel after the margin comes off is the
// face and a few fragments of its frame; the face outweighs them by two
// orders of magnitude, so size tells them apart where neither colour nor
// connectivity to the edge could.
func (f *figure) keepLargest() {
	b := f.img.Bounds()
	w, h := b.Dx(), b.Dy()
	label := make([]int, w*h)
	best, bestSize := 0, 0
	next := 0
	for i := range label {
		if label[i] != 0 || f.img.NRGBAAt(i%w, i/w).A < 128 {
			continue
		}
		next++
		size := 0
		stack := [][2]int{{i % w, i / w}}
		label[i] = next
		for len(stack) > 0 {
			p := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			size++
			for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				x, y := p[0]+d[0], p[1]+d[1]
				if x < 0 || y < 0 || x >= w || y >= h || label[y*w+x] != 0 ||
					f.img.NRGBAAt(x, y).A < 128 {
					continue
				}
				label[y*w+x] = next
				stack = append(stack, [2]int{x, y})
			}
		}
		if size > bestSize {
			best, bestSize = next, size
		}
	}
	for i, l := range label {
		if l != best {
			f.img.SetNRGBA(i%w, i/w, color.NRGBA{})
		}
	}
}

// trim crops a figure down to what is actually drawn in it.
func (f *figure) trim(name string) (*part, error) {
	b := f.img.Bounds()
	minX, minY, maxX, maxY := b.Dx(), b.Dy(), -1, -1
	for y := range b.Dy() {
		for x := range b.Dx() {
			if f.img.NRGBAAt(x, y).A < 128 {
				continue
			}
			minX, minY = min(minX, x), min(minY, y)
			maxX, maxY = max(maxX, x), max(maxY, y)
		}
	}
	if maxX < 0 {
		return nil, fmt.Errorf("%s: the panel lifted to nothing", name)
	}
	w, h := maxX-minX+1, maxY-minY+1
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, f.img.NRGBAAt(minX+x, minY+y))
		}
	}
	return &part{name: name, img: img, x: minX + f.x, y: minY + f.y}, nil
}
