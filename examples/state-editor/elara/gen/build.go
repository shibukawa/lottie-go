package main

// Assembling the parts into a clip a player can open.
//
// Each part becomes a shape layer: its traced outline as the path, a fill
// carrying the part's own image as a texture, and a per-vertex UV binding
// the two. The texture binding is not Lottie — nothing in the format
// paints an image through a path — so it travels beside the clip as the
// bundle's texture document, and a player without that extension shows
// the fills' flat colours instead of nothing.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"math"
	"os"

	lottie "github.com/shibukawa/lottie-go"
	lottietexture "github.com/shibukawa/lottie-go/plugin/texture"
)

type obj = map[string]any

func static(v any) obj { return obj{"a": 0, "k": v} }

// shapeLayer builds one part's layer. The outline is placed in canvas
// space here rather than through the layer transform, so a later stage
// can rewrite the path per keyframe — which is how the skinning will
// drive it — without the transform fighting it.
func shapeLayer(name string, ind int, m mesh, p placement, w, h float64, frames float64) obj {
	sx := p.scale
	if p.flipX {
		sx = -p.scale
	}
	sin, cos := math.Sincos(p.rot * math.Pi / 180)
	v := make([]any, len(m.outline))
	zero := make([]any, len(m.outline))
	for i, q := range m.outline {
		dx, dy := (q.x-w/2)*sx, (q.y-h/2)*p.scale
		v[i] = []any{p.x + dx*cos - dy*sin, p.y + dx*sin + dy*cos}
		zero[i] = []any{0.0, 0.0}
	}
	return obj{
		"ty": 4, "nm": name, "ind": ind, "ip": 0, "op": frames, "st": 0,
		"ks": obj{
			"a": static([]any{0, 0}), "p": static([]any{0, 0}),
			"s": static([]any{100, 100}), "r": static(0), "o": static(100),
		},
		"shapes": []any{obj{"ty": "gr", "nm": name, "it": []any{
			obj{"ty": "sh", "nm": "outline", "ks": static(obj{
				"c": true, "v": v, "i": zero, "o": zero,
			})},
			obj{"ty": "fl", "nm": "paint", "c": static([]any{0.85, 0.2, 0.85}),
				"o": static(100), "r": 1},
			obj{"ty": "tr", "p": static([]any{0, 0}), "a": static([]any{0, 0}),
				"s": static([]any{100, 100}), "r": static(0), "o": static(100)},
		}}},
	}
}

// buildBundle writes the assembled character as a one-clip bundle.
func buildBundle(parts map[string]*part, out string) error {
	const frames = 96.0
	b := lottie.NewBundle()
	var layers []any
	var assets []any
	doc := &lottietexture.Doc{}
	seen := map[string]bool{}

	// Layers are listed front to back, so the layout's back-to-front order
	// is walked in reverse.
	ind := 0
	for i := len(layout) - 1; i >= 0; i-- {
		pl := layout[i]
		pt, ok := parts[pl.part]
		if !ok {
			return fmt.Errorf("layout names %q, which was never cut", pl.part)
		}
		ind++
		name := fmt.Sprintf("%s-%d", pl.part, ind)
		bounds := pt.img.Bounds()
		w, h := float64(bounds.Dx()), float64(bounds.Dy())
		m := meshOf(pt.img, pl.points)
		if len(m.outline) < 3 {
			return fmt.Errorf("%s: traced no outline", pl.part)
		}
		if !seen[pl.part] {
			seen[pl.part] = true
			var buf bytes.Buffer
			if err := encodePNG(&buf, pt.img); err != nil {
				return err
			}
			b.SetImage(pl.part+".png", buf.Bytes())
			assets = append(assets, obj{
				"id": pl.part, "w": w, "h": h, "u": "i/", "p": pl.part + ".png",
			})
		}
		layers = append(layers, shapeLayer(name, ind, m, pl, w, h, frames))

		tint := false
		doc.Paints = append(doc.Paints, lottietexture.Paint{
			Layer: ind, Item: []int{0, 1}, Texture: pl.part,
			Mapping: lottietexture.MappingVertex, Tint: &tint,
		})
		uv := make([][2]float64, len(m.uv))
		for j, q := range m.uv {
			// A mirrored part reads the same art from the other side.
			u := q.x
			if pl.flipX {
				u = 1 - u
			}
			uv[j] = [2]float64{u, q.y}
		}
		doc.UVs = append(doc.UVs, lottietexture.UV{Layer: ind, Item: []int{0, 0}, V: uv})
	}

	clip := obj{
		"v": "5.9.0", "nm": "idle", "fr": 60, "ip": 0, "op": frames,
		"w": canvasW, "h": canvasH, "assets": assets, "layers": layers,
	}
	raw, err := json.Marshal(clip)
	if err != nil {
		return err
	}
	if err := b.SetAnimation("idle-anim", raw); err != nil {
		return err
	}
	if err := lottietexture.Store(b, "idle-anim", doc); err != nil {
		return err
	}
	// One state, so the editor has a machine to show and the clip plays
	// on open. The full verb set arrives with the clips it needs.
	if err := b.SetStateMachine("elara", &lottie.StateMachine{
		Initial: "idle-state",
		States: []lottie.State{{
			Name: "idle-state", Type: lottie.StatePlayback,
			Animation: "idle-anim", Loop: true, Autoplay: true,
		}},
	}); err != nil {
		return err
	}
	b.Manifest().Generator = "lottie-go/examples/state-editor/elara gen"
	var buf bytes.Buffer
	if err := b.Encode(&buf); err != nil {
		return err
	}
	if err := os.WriteFile(out, buf.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d layers, %d bytes)\n", out, len(layers), buf.Len())
	return nil
}

func encodePNG(buf *bytes.Buffer, img image.Image) error {
	return pngEncode(buf, img)
}
