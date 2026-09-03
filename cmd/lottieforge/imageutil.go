package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"math"
	"os"
	"strconv"
	"strings"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	_ "golang.org/x/image/webp"
)

func loadImage(path string) (*image.NRGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return toNRGBA(img), nil
}

func decodeImage(data []byte) (*image.NRGBA, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return toNRGBA(img), nil
}

func toNRGBA(img image.Image) *image.NRGBA {
	if n, ok := img.(*image.NRGBA); ok && n.Rect.Min == (image.Point{}) {
		return n
	}
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Rect, img, b.Min, draw.Src)
	return out
}

func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func writePNG(path string, img image.Image) error {
	return os.WriteFile(path, encodePNG(img), 0o644)
}

func parseHex(s string) (color.NRGBA, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return color.NRGBA{}, fmt.Errorf("color %q: want #RRGGBB", s)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return color.NRGBA{}, fmt.Errorf("color %q: %w", s, err)
	}
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255}, nil
}

func colorName(c color.NRGBA) string {
	switch {
	case c.R > 200 && c.B > 200 && c.G < 80:
		return "magenta"
	case c.G > 200 && c.R < 80 && c.B < 80:
		return "green"
	case c.B > 200 && c.R < 80 && c.G < 80:
		return "blue"
	case c.R > 200 && c.G > 200 && c.B > 200:
		return "white"
	case c.R > 200 && c.G > 200 && c.B < 80:
		return "yellow"
	case c.R > 200 && c.G < 80 && c.B < 80:
		return "red"
	case c.G > 200 && c.B > 200 && c.R < 80:
		return "cyan"
	}
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

// keyOut turns pixels near the key color transparent, ramping over a
// band so antialiased edges keep a soft alpha, and pulls the key color
// back out of those edge pixels (decontamination) so no fringe is left.
func keyOut(src *image.NRGBA, key color.NRGBA) *image.NRGBA {
	const near, far = 40.0, 100.0
	w, h := src.Rect.Dx(), src.Rect.Dy()
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	kr, kg, kb := float64(key.R), float64(key.G), float64(key.B)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := src.PixOffset(x, y)
			r, g, b, a := float64(src.Pix[i]), float64(src.Pix[i+1]), float64(src.Pix[i+2]), float64(src.Pix[i+3])
			d := math.Sqrt((r-kr)*(r-kr) + (g-kg)*(g-kg) + (b-kb)*(b-kb))
			t := (d - near) / (far - near)
			if t < 0 {
				t = 0
			}
			if t > 1 {
				t = 1
			}
			alpha := t * a / 255
			if alpha > 0 && alpha < 1 {
				// The pixel is a blend of the true color and the key;
				// invert the blend.
				r = clamp255((r - kr*(1-alpha)) / alpha)
				g = clamp255((g - kg*(1-alpha)) / alpha)
				b = clamp255((b - kb*(1-alpha)) / alpha)
			}
			o := out.PixOffset(x, y)
			out.Pix[o], out.Pix[o+1], out.Pix[o+2], out.Pix[o+3] = uint8(r), uint8(g), uint8(b), uint8(math.Round(alpha*255))
		}
	}
	return out
}

func clamp255(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

func alphaMask(img *image.NRGBA, threshold uint8) []bool {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	m := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			m[y*w+x] = img.Pix[img.PixOffset(x, y)+3] >= threshold
		}
	}
	return m
}

// component is one 4-connected blob of a mask.
type component struct {
	area   int
	box    image.Rectangle
	pixels []int
}

// components labels a mask's blobs, largest first.
func components(mask []bool, w, h int) []component {
	seen := make([]bool, len(mask))
	var out []component
	stack := make([]int, 0, 1024)
	for start := range mask {
		if !mask[start] || seen[start] {
			continue
		}
		c := component{box: image.Rect(start%w, start/w, start%w+1, start/w+1)}
		stack = append(stack[:0], start)
		seen[start] = true
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			c.pixels = append(c.pixels, i)
			x, y := i%w, i/w
			c.box = c.box.Union(image.Rect(x, y, x+1, y+1))
			for _, n := range [4]int{i - 1, i + 1, i - w, i + w} {
				if n < 0 || n >= len(mask) || !mask[n] || seen[n] {
					continue
				}
				if (n == i-1 && x == 0) || (n == i+1 && x == w-1) {
					continue
				}
				seen[n] = true
				stack = append(stack, n)
			}
		}
		c.area = len(c.pixels)
		out = append(out, c)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].area > out[j-1].area; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// keepComponent blanks every pixel outside the component.
func keepComponent(img *image.NRGBA, c component) *image.NRGBA {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	keep := make([]bool, w*h)
	for _, i := range c.pixels {
		keep[i] = true
	}
	// Keep the soft edge around the blob too: any pixel touching it.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			ok := keep[i]
			if !ok {
				for dy := -1; dy <= 1 && !ok; dy++ {
					for dx := -1; dx <= 1 && !ok; dx++ {
						nx, ny := x+dx, y+dy
						if nx >= 0 && ny >= 0 && nx < w && ny < h && keep[ny*w+nx] {
							ok = true
						}
					}
				}
			}
			if ok {
				copy(out.Pix[out.PixOffset(x, y):out.PixOffset(x, y)+4], img.Pix[img.PixOffset(x, y):img.PixOffset(x, y)+4])
			}
		}
	}
	return out
}

// alphaBounds is the bounding box of the pixels with alpha > 0.
func alphaBounds(img *image.NRGBA) image.Rectangle {
	var r image.Rectangle
	first := true
	w, h := img.Rect.Dx(), img.Rect.Dy()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if img.Pix[img.PixOffset(x, y)+3] > 8 {
				px := image.Rect(x, y, x+1, y+1)
				if first {
					r, first = px, false
				} else {
					r = r.Union(px)
				}
			}
		}
	}
	return r
}

func cropImage(img *image.NRGBA, r image.Rectangle) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(out, out.Rect, img, r.Min, draw.Src)
	return out
}

// cropFrac crops by fractions of the image size.
func cropFrac(img *image.NRGBA, f [4]float64) *image.NRGBA {
	w, h := float64(img.Rect.Dx()), float64(img.Rect.Dy())
	r := image.Rect(int(f[0]*w), int(f[1]*h), int(math.Ceil(f[2]*w)), int(math.Ceil(f[3]*h)))
	if r.Dx() < 1 || r.Dy() < 1 {
		return img
	}
	return cropImage(img, r)
}

func darken(img *image.NRGBA, f float64) *image.NRGBA {
	out := image.NewNRGBA(img.Rect)
	copy(out.Pix, img.Pix)
	for i := 0; i+3 < len(out.Pix); i += 4 {
		out.Pix[i] = uint8(float64(out.Pix[i]) * f)
		out.Pix[i+1] = uint8(float64(out.Pix[i+1]) * f)
		out.Pix[i+2] = uint8(float64(out.Pix[i+2]) * f)
	}
	return out
}

func meanColor(img *image.NRGBA) [3]float64 {
	var r, g, b, a float64
	for i := 0; i+3 < len(img.Pix); i += 4 {
		w := float64(img.Pix[i+3]) / 255
		r += float64(img.Pix[i]) * w
		g += float64(img.Pix[i+1]) * w
		b += float64(img.Pix[i+2]) * w
		a += w
	}
	if a == 0 {
		return [3]float64{0.5, 0.5, 0.5}
	}
	return [3]float64{round2(r / a / 255), round2(g / a / 255), round2(b / a / 255)}
}

func resizeTo(img *image.NRGBA, w, h int, nearest bool) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	var sc xdraw.Scaler = xdraw.CatmullRom
	if nearest {
		sc = xdraw.NearestNeighbor
	}
	sc.Scale(out, out.Rect, img, img.Rect, xdraw.Over, nil)
	return out
}

func fillRect(dst draw.Image, r image.Rectangle, c color.Color) {
	draw.Draw(dst, r, image.NewUniform(c), image.Point{}, draw.Src)
}

func strokeRect(dst draw.Image, r image.Rectangle, width int, c color.Color) {
	fillRect(dst, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+width), c)
	fillRect(dst, image.Rect(r.Min.X, r.Max.Y-width, r.Max.X, r.Max.Y), c)
	fillRect(dst, image.Rect(r.Min.X, r.Min.Y, r.Min.X+width, r.Max.Y), c)
	fillRect(dst, image.Rect(r.Max.X-width, r.Min.Y, r.Max.X, r.Max.Y), c)
}

// drawText writes s with the built-in 7x13 face scaled up by scale, its
// top-left at (x, y).
func drawText(dst draw.Image, x, y int, s string, scale int, c color.Color) {
	face := basicfont.Face7x13
	w := 7*len(s) + 2
	tmp := image.NewRGBA(image.Rect(0, 0, w, 14))
	d := font.Drawer{Dst: tmp, Src: image.NewUniform(c), Face: face,
		Dot: fixed.Point26_6{X: fixed.I(1), Y: fixed.I(11)}}
	d.DrawString(s)
	if scale < 1 {
		scale = 1
	}
	big := image.NewRGBA(image.Rect(0, 0, w*scale, 14*scale))
	xdraw.NearestNeighbor.Scale(big, big.Rect, tmp, tmp.Rect, xdraw.Src, nil)
	draw.Draw(dst, image.Rect(x, y, x+big.Rect.Dx(), y+big.Rect.Dy()), big, image.Point{}, draw.Over)
}

func textWidth(s string, scale int) int { return (7*len(s) + 2) * scale }
