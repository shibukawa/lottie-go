package lottie

import (
	"encoding/json"
	"image"
	"math"
	"strings"
	"testing"
)

// texTestClip is a shape layer (ind 1) holding a group with a square path,
// a hidden fill and a live fill, then a stroke outside the group; plus a
// precomp layer whose asset holds one more shape layer, so references into
// assets resolve too.
const texTestClip = `{
  "v": "5.7.1", "fr": 30, "ip": 0, "op": 60, "w": 200, "h": 200,
  "assets": [
    {"id": "comp_0", "layers": [
      {"ty": 4, "ind": 1, "ip": 0, "op": 60, "st": 0, "ks": {},
       "shapes": [
         {"ty": "sh", "ks": {"a": 0, "k": {"c": true, "v": [[0,0],[10,0],[10,10]], "i": [[0,0],[0,0],[0,0]], "o": [[0,0],[0,0],[0,0]]}}},
         {"ty": "fl", "c": {"a": 0, "k": [1,0,0,1]}, "o": {"a": 0, "k": 100}}
       ]}
    ]}
  ],
  "layers": [
    {"ty": 4, "ind": 1, "nm": "shapes", "ip": 0, "op": 60, "st": 0, "ks": {},
     "shapes": [
       {"ty": "gr", "it": [
         {"ty": "sh", "ks": {"a": 0, "k": {"c": true, "v": [[0,0],[100,0],[100,100],[0,100]], "i": [[0,0],[0,0],[0,0],[0,0]], "o": [[0,0],[0,0],[0,0],[0,0]]}}},
         {"ty": "fl", "hd": true, "c": {"a": 0, "k": [0,0,1,1]}, "o": {"a": 0, "k": 100}},
         {"ty": "fl", "c": {"a": 0, "k": [1,1,1,1]}, "o": {"a": 0, "k": 100}},
         {"ty": "tr", "p": {"a": 0, "k": [0,0]}, "a": {"a": 0, "k": [0,0]}, "s": {"a": 0, "k": [100,100]}, "r": {"a": 0, "k": 0}, "o": {"a": 0, "k": 100}}
       ]},
       {"ty": "st", "c": {"a": 0, "k": [0,0,0,1]}, "o": {"a": 0, "k": 100}, "w": {"a": 0, "k": 4}}
     ]},
    {"ty": 0, "ind": 2, "refId": "comp_0", "w": 200, "h": 200, "ip": 0, "op": 60, "st": 0, "ks": {}}
  ]
}`

func texTestPlayer(t *testing.T) *Player {
	t.Helper()
	anim, err := Decode(strings.NewReader(texTestClip))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return anim.NewPlayer()
}

func TestShapeRefResolvesAuthoredIndices(t *testing.T) {
	p := texTestPlayer(t)
	paint := &TexturePaint{Texture: "skin"}
	// The live fill is items[0].it[2]: the hidden fill before it still
	// counts, because the address is computed from the document.
	if err := p.SetTexturePaint(ShapeRef{Layer: 1, Item: []int{0, 2}}, paint); err != nil {
		t.Fatalf("fill: %v", err)
	}
	if err := p.SetTexturePaint(ShapeRef{Layer: 1, Item: []int{0, 1}}, paint); err == nil {
		t.Fatal("hidden fill resolved; it is not in the decoded tree")
	}
	if err := p.SetTexturePaint(ShapeRef{Layer: 1, Item: []int{0, 0}}, paint); err == nil {
		t.Fatal("a path accepted a paint")
	}
	if err := p.SetTexturePaint(ShapeRef{Layer: 1, Item: []int{1}}, &TexturePaint{Texture: "brush"}); err != nil {
		t.Fatalf("stroke: %v", err)
	}
	if err := p.SetTexturePaint(ShapeRef{Layer: 7, Item: []int{0}}, paint); err == nil {
		t.Fatal("unknown layer resolved")
	}
	if err := p.SetTexturePaint(ShapeRef{Layer: 1, Item: []int{0}}, paint); err == nil {
		t.Fatal("a group accepted a paint")
	}
	if err := p.SetTexturePaint(ShapeRef{Asset: "comp_0", Layer: 1, Item: []int{1}}, paint); err != nil {
		t.Fatalf("precomp fill: %v", err)
	}
	if err := p.SetTexturePaint(ShapeRef{Asset: "nope", Layer: 1, Item: []int{1}}, paint); err == nil {
		t.Fatal("unknown asset resolved")
	}
	if got, want := p.TextureNames(), []string{"brush", "skin"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("TextureNames = %v, want %v", got, want)
	}
	if err := p.SetTexturePaint(ShapeRef{Layer: 1, Item: []int{1}}, nil); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if got := p.TextureNames(); len(got) != 1 || got[0] != "skin" {
		t.Fatalf("after clearing the stroke, names = %v", got)
	}
	if err := p.SetTexturePaint(ShapeRef{Layer: 1, Item: []int{0, 2}}, &TexturePaint{}); err == nil {
		t.Fatal("a paint without a texture name was accepted")
	}
}

func TestSetVertexUVChecksTheCount(t *testing.T) {
	p := texTestPlayer(t)
	ref := ShapeRef{Layer: 1, Item: []int{0, 0}}
	uv := [][2]float32{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
	if err := p.SetVertexUV(ref, uv); err != nil {
		t.Fatalf("uv: %v", err)
	}
	if err := p.SetVertexUV(ref, uv[:3]); err == nil {
		t.Fatal("three UVs for a four-vertex path were accepted")
	}
	if err := p.SetVertexUV(ShapeRef{Layer: 1, Item: []int{0, 2}}, uv); err == nil {
		t.Fatal("a fill accepted UVs")
	}
	// The bound set is copied, so the caller's slice is free again.
	uv[0][0] = 9
	n, _ := p.anim.shapeNodeAt(ref)
	if got := p.r.uvs[n][0][0]; got != 0 {
		t.Fatalf("bound UV aliases the caller's slice: %v", got)
	}
	if err := p.SetVertexUV(ref, nil); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, bound := p.r.uvs[n]; bound {
		t.Fatal("nil did not clear the UV set")
	}
}

func TestTexturedCommandsCarryPaintAndUV(t *testing.T) {
	p := texTestPlayer(t)
	fill := ShapeRef{Layer: 1, Item: []int{0, 2}}
	path := ShapeRef{Layer: 1, Item: []int{0, 0}}
	if err := p.SetTexturePaint(fill, &TexturePaint{Texture: "skin", Mapping: MappingVertex, Tint: true}); err != nil {
		t.Fatal(err)
	}
	if err := p.SetVertexUV(path, [][2]float32{{0, 0}, {1, 0}, {1, 1}, {0, 1}}); err != nil {
		t.Fatal(err)
	}
	r := &p.r
	r.nGeoms, r.nDash, r.nGrad, r.nTex = 0, 0, 0, 0
	r.cmds = r.cmds[:0]
	r.walkShapes(p.anim.layers[0].shapes, 0, identityMatrix, 1)
	if len(r.cmds) != 2 {
		t.Fatalf("commands = %d, want fill and stroke", len(r.cmds))
	}
	// Commands are appended in walk order: the group's fill, then the stroke.
	if r.cmds[0].tex == nil || r.cmds[0].tex.paint.name != "skin" {
		t.Fatalf("the fill command carries no paint: %+v", r.cmds[0].tex)
	}
	if r.cmds[1].tex != nil {
		t.Fatal("the unpainted stroke carries a paint")
	}
	if got := r.geoms[0].uv; len(got) != 4 {
		t.Fatalf("path geometry uv = %v", got)
	}
	if !uvUsable(r.geoms, 0, 1) {
		t.Fatal("uv not usable on the untouched path")
	}
	// A player without paints stays snapshot-eligible; with one it never is.
	if !p.anim.snapshotOK {
		t.Skip("clip not snapshot-eligible; cannot check the bypass")
	}
	if p.drawSnapshot(nil, 0, identityMatrix, p.snapKey.cs, true) {
		t.Fatal("a textured player took the snapshot path")
	}
}

func TestTextureTransformParses(t *testing.T) {
	tr := json.RawMessage(`{"p": {"a": 1, "k": [{"t": 0, "s": [0, 0]}, {"t": 30, "s": [1, 0]}]}, "s": {"a": 0, "k": [200, 200]}, "r": {"a": 0, "k": 90}}`)
	tp, err := newTexturePaint(&TexturePaint{Texture: "x", Transform: tr})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// At frame 30 the anchor (0,0) sits at (1,0), scaled 2x and turned 90°:
	// uv (0,0) is placed at (1,0), so sampling (1,0) reads uv (0,0).
	inv, ok := tp.tr.matrixAt(30).invert()
	if !ok {
		t.Fatal("singular placement")
	}
	u, v := inv.apply(1, 0)
	if math.Abs(u) > 1e-9 || math.Abs(v) > 1e-9 {
		t.Fatalf("inverse placement of (1,0) = (%v, %v), want (0, 0)", u, v)
	}
	if _, err := newTexturePaint(&TexturePaint{Texture: "x", Transform: json.RawMessage(`{"p": `)}); err == nil {
		t.Fatal("broken transform JSON accepted")
	}
	if _, err := newTexturePaint(&TexturePaint{Texture: "x", Transform: json.RawMessage(`{"p": {"a": 0, "k": [0, 0], "x": "var $bm_rt = 1;"}}`)}); err == nil {
		t.Fatal("an expression in the transform was accepted silently")
	}
}

// squareGeom is a 100x100 axis-aligned square at the origin with corner UVs.
func squareGeom(uv bool) geometry {
	g := geometry{mat: identityMatrix}
	g.bez.Closed = true
	g.bez.V = [][2]float64{{0, 0}, {100, 0}, {100, 100}, {0, 100}}
	g.bez.I = make([][2]float64, 4)
	g.bez.O = make([][2]float64, 4)
	if uv {
		g.uv = [][2]float32{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
	}
	return g
}

func TestUVMeshFansFromTheCentroid(t *testing.T) {
	var r renderer
	geoms := []geometry{squareGeom(true)}
	cmd := drawCmd{geomStart: 0, geomEnd: 1, tex: &textureCmd{mat: identityMatrix, uvInv: identityMatrix}}
	tex := texSpace{ox: 10, oy: 20, w: 64, h: 32, inv: identityMatrix}
	r.meshVertexUV(geoms, &cmd, tex, texInflate)
	if len(r.texVerts) != 5 || len(r.texIdx) != 12 {
		t.Fatalf("verts %d idx %d, want a centroid plus four corners in four triangles", len(r.texVerts), len(r.texIdx))
	}
	c := r.texVerts[0]
	if c.DstX != 50 || c.DstY != 50 || c.SrcX != 10+32 || c.SrcY != 20+16 {
		t.Fatalf("centroid vertex = %+v", c)
	}
	// Corners land on the texture's own bounds and are pushed out by the
	// inflation, so the antialiased edge stays covered.
	want := [][2]float32{{10, 20}, {74, 20}, {74, 52}, {10, 52}}
	for i, w := range want {
		v := r.texVerts[i+1]
		if v.SrcX != w[0] || v.SrcY != w[1] {
			t.Fatalf("corner %d src = (%v, %v), want %v", i, v.SrcX, v.SrcY, w)
		}
		d := math.Hypot(float64(v.DstX)-50, float64(v.DstY)-50)
		if math.Abs(d-(math.Hypot(50, 50)+texInflate)) > 1e-3 {
			t.Fatalf("corner %d sits %v from the centroid, want the corner distance plus inflation", i, d)
		}
	}
	for _, i := range r.texIdx {
		if int(i) >= len(r.texVerts) {
			t.Fatalf("index %d out of range", i)
		}
	}
}

func TestUVMeshCarriesUVAlongCurves(t *testing.T) {
	var r renderer
	g := squareGeom(true)
	// Bulge the top edge into a curve: the flattened points between the two
	// corners must interpolate their UVs by the bezier parameter.
	g.bez.O[0] = [2]float64{30, -40}
	g.bez.I[1] = [2]float64{-30, -40}
	geoms := []geometry{g}
	cmd := drawCmd{geomStart: 0, geomEnd: 1}
	tex := texSpace{w: 100, h: 100, inv: identityMatrix}
	r.meshVertexUV(geoms, &cmd, tex, 0)
	if len(r.texVerts) <= 5 {
		t.Fatalf("the curved edge was not subdivided: %d vertices", len(r.texVerts))
	}
	prev := float32(-1)
	for _, v := range r.texVerts[1:] {
		if v.DstY > 0.5 { // past the top edge
			break
		}
		if v.SrcX < prev {
			t.Fatalf("u along the curved edge is not monotonic: %v after %v", v.SrcX, prev)
		}
		if v.SrcY != 0 {
			t.Fatalf("v along the top edge = %v, want 0", v.SrcY)
		}
		prev = v.SrcX
	}
}

func TestBBoxMeshIsOneQuadInShapeSpace(t *testing.T) {
	var r renderer
	// The style sits in a group translated by (50, 0); the bbox is taken in
	// that space, so the shape still spans UV 0..1.
	mat := identityMatrix.translate(50, 0)
	g := squareGeom(false)
	g.mat = mat
	geoms := []geometry{g}
	cmd := drawCmd{geomStart: 0, geomEnd: 1, tex: &textureCmd{mat: mat, uvInv: identityMatrix}}
	tex := texSpace{w: 10, h: 10, inv: identityMatrix}
	region := image.Rect(50, 0, 150, 100)
	r.meshBBox(geoms, &cmd, region, tex)
	if len(r.texVerts) != 4 || len(r.texIdx) != 6 {
		t.Fatalf("verts %d idx %d, want one quad", len(r.texVerts), len(r.texIdx))
	}
	want := [][4]float32{{50, 0, 0, 0}, {150, 0, 10, 0}, {50, 100, 0, 10}, {150, 100, 10, 10}}
	for i, w := range want {
		v := r.texVerts[i]
		if v.DstX != w[0] || v.DstY != w[1] || v.SrcX != w[2] || v.SrcY != w[3] {
			t.Fatalf("vertex %d = %+v, want %v", i, v, w)
		}
	}
}

func TestStrokeMeshIsARibbon(t *testing.T) {
	var r renderer
	g := geometry{mat: identityMatrix}
	g.bez.V = [][2]float64{{0, 0}, {200, 0}}
	g.bez.I = make([][2]float64, 2)
	g.bez.O = make([][2]float64, 2)
	geoms := []geometry{g}
	cmd := drawCmd{geomStart: 0, geomEnd: 1, stroke: true}
	// A 50x25 texture on a 10-wide stroke: one texture width per 20 px of
	// arc keeps its aspect at the stroke's width, so 200 px is 10 repeats.
	tex := texSpace{w: 50, h: 25, inv: identityMatrix}
	r.meshStroke(geoms, &cmd, tex, 10)
	if len(r.texVerts) != 4 || len(r.texIdx) != 6 {
		t.Fatalf("verts %d idx %d, want one quad for one segment", len(r.texVerts), len(r.texIdx))
	}
	if u0, u1 := r.texVerts[0].SrcX, r.texVerts[2].SrcX; u0 != 0 || math.Abs(float64(u1)-500) > 1e-3 {
		t.Fatalf("u runs %v..%v in texture pixels, want 0..500", u0, u1)
	}
	// The ribbon is over-wide and its ends extend past the caps.
	if r.texVerts[0].DstX >= 0 || r.texVerts[2].DstX <= 200 {
		t.Fatalf("ends not extended: %v .. %v", r.texVerts[0].DstX, r.texVerts[2].DstX)
	}
	if hw := math.Abs(float64(r.texVerts[0].DstY)); hw <= 5 {
		t.Fatalf("ribbon half-width %v is not wider than the stroke", hw)
	}
	// Authored u overrides arc length.
	r.texVerts, r.texIdx = r.texVerts[:0], r.texIdx[:0]
	geoms[0].uv = [][2]float32{{2, 0}, {3, 0}}
	r.meshStroke(geoms, &cmd, tex, 10)
	if u0, u1 := r.texVerts[0].SrcX, r.texVerts[2].SrcX; u0 != 100 || u1 != 150 {
		t.Fatalf("authored u gave %v..%v, want 100..150", u0, u1)
	}
}

func TestUVUsableNeedsMatchingCounts(t *testing.T) {
	geoms := []geometry{squareGeom(true), squareGeom(false)}
	if !uvUsable(geoms, 0, 1) {
		t.Fatal("square with UV not usable")
	}
	if uvUsable(geoms, 0, 2) {
		t.Fatal("a geometry without UV passed")
	}
	geoms[0].bez.V = geoms[0].bez.V[:3] // a modifier changed the vertex count
	if uvUsable(geoms, 0, 1) {
		t.Fatal("mismatched count passed")
	}
	if uvUsable(geoms, 1, 1) {
		t.Fatal("an empty range passed")
	}
}

func TestFlattenStepsScaleWithLength(t *testing.T) {
	if n := flattenSteps([2]float64{0, 0}, [2]float64{0, 0}, [2]float64{9, 9}, [2]float64{9, 9}); n != 1 {
		t.Fatalf("straight segment steps = %d", n)
	}
	short := flattenSteps([2]float64{0, 0}, [2]float64{1, 1}, [2]float64{2, 1}, [2]float64{3, 0})
	long := flattenSteps([2]float64{0, 0}, [2]float64{100, 100}, [2]float64{200, 100}, [2]float64{300, 0})
	if short >= long || long > 32 || short < 1 {
		t.Fatalf("steps short %d long %d", short, long)
	}
}
