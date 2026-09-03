package main

import (
	"encoding/json"
	"fmt"
	"image"
	"math"
)

// fitMap places texture pixels into slot space: slot = tex*s + t
// (concept:uv-morph-rig fit).
type fitMap struct {
	s      float64
	tx, ty float64
}

func (f fitMap) apply(p pt) pt { return pt{p[0]*f.s + f.tx, p[1]*f.s + f.ty} }

// computeFit scales the alpha box's height onto the slot's height times
// fit, centers it on the anchor's x, and pins the joint edge.
func computeFit(slotW, slotH float64, anchor pt, joint string, box image.Rectangle, fit float64) fitMap {
	bh := float64(box.Dy())
	if bh < 1 {
		bh = 1
	}
	s := slotH * fit / bh
	cx := float64(box.Min.X+box.Max.X) / 2
	f := fitMap{s: s, tx: anchor[0] - cx*s}
	if joint == "bottom" {
		f.ty = slotH - float64(box.Max.Y)*s
	} else {
		f.ty = -float64(box.Min.Y) * s
	}
	return f
}

// piece is one star-shaped sub-path of a part, in slot space, with its
// UV over the part's texture.
type piece struct {
	V      []pt   `json:"v"`
	UV     []pt   `json:"uv"`
	Corner []bool `json:"corner"`
}

// partGeom is a traced part: what rig writes into the clip and into
// extensions/forge/<layer>.json for morph to start from.
type partGeom struct {
	Slot   string     `json:"slot"`
	Size   [2]int     `json:"size"`
	Anchor pt         `json:"anchor"`
	SlotW  float64    `json:"slot_w"`
	SlotH  float64    `json:"slot_h"`
	Pieces []piece    `json:"pieces"`
	Welds  [][4]int   `json:"welds"`
	Color  [3]float64 `json:"color"`
}

// buildGeom traces img into slot space. budget caps the vertices of the
// whole outline before decomposition.
func buildGeom(name string, img *image.NRGBA, slotW, slotH float64, anchor pt, joint string, fit float64, budget int) (*partGeom, error) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	if w == 0 || h == 0 {
		return nil, fmt.Errorf("%s: empty image", name)
	}
	mask := alphaMask(img, 128)
	box := alphaBounds(img)
	if box.Empty() {
		return nil, fmt.Errorf("%s: no opaque pixels", name)
	}
	if budget <= 0 {
		budget = 12
	}
	grow := math.Max(2, math.Hypot(float64(box.Dx()), float64(box.Dy()))*0.015)
	outline := outlineOf(mask, w, h, budget, grow)
	if len(outline) < 3 {
		return nil, fmt.Errorf("%s: could not trace an outline", name)
	}
	f := computeFit(slotW, slotH, anchor, joint, box, fit)
	g := &partGeom{Slot: name, Size: [2]int{w, h}, Anchor: anchor, SlotW: slotW, SlotH: slotH, Color: meanColor(img)}
	pieces := decompose(outline)
	// Seam vertices are corners on both sides so the seam is one straight
	// line; the outline's own vertices stay smooth.
	seamCount := map[pt]int{}
	for _, pc := range pieces {
		for _, v := range pc {
			seamCount[v]++
		}
	}
	index := map[pt][][2]int{}
	for pi, pc := range pieces {
		p := piece{}
		for vi, v := range pc {
			p.V = append(p.V, f.apply(v))
			p.UV = append(p.UV, pt{round4(v[0] / float64(w)), round4(v[1] / float64(h))})
			p.Corner = append(p.Corner, seamCount[v] > 1)
			index[v] = append(index[v], [2]int{pi, vi})
		}
		g.Pieces = append(g.Pieces, p)
	}
	for _, refs := range index {
		for i := 1; i < len(refs); i++ {
			g.Welds = append(g.Welds, [4]int{refs[0][0], refs[0][1], refs[i][0], refs[i][1]})
		}
	}
	return g, nil
}

func round4(x float64) float64 { return math.Round(x*10000) / 10000 }

// shapeItems encodes the pieces as sh items followed by the fallback fill
// and the group transform, and reports the fill's item index.
func (g *partGeom) shapeItems(vertsOverride [][]pt) ([]obj, int) {
	var items []obj
	for i, p := range g.Pieces {
		v := p.V
		if vertsOverride != nil {
			v = vertsOverride[i]
		}
		in, out := tangents(v, p.Corner)
		items = append(items, obj{"ty": "sh", "nm": fmt.Sprintf("contour %d", i+1), "ks": static(pathObj(v, in, out, true))})
	}
	fillIdx := len(items)
	items = append(items, obj{"ty": "fl", "nm": "fallback", "c": static([]float64{g.Color[0], g.Color[1], g.Color[2]}), "o": static(100.0), "r": 1})
	items = append(items, obj{"ty": "tr", "p": static([]float64{0, 0}), "a": static([]float64{0, 0}),
		"s": static([]float64{100, 100}), "r": static(0.0), "o": static(100.0)})
	return items, fillIdx
}

func (g *partGeom) marshal() []byte {
	raw, _ := json.MarshalIndent(g, "", " ")
	return raw
}

func parseGeom(data []byte) (*partGeom, error) {
	var g partGeom
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, err
	}
	return &g, nil
}
