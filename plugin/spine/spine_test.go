package lottiespine

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
	"testing"

	lottie "github.com/shibukawa/lottie-go"
	lottietexture "github.com/shibukawa/lottie-go/plugin/texture"
)

// A small rig: an arm bone pointing up from the root, a hand bone at its
// tip, a weighted mesh on the arm and a region on the hand. The wave
// animation swings the arm and fires an event halfway.
const testSkeleton = `{
"skeleton": {"spine": "4.2.22", "x": -50, "y": 0, "width": 100, "height": 100},
"bones": [
	{"name": "root"},
	{"name": "arm", "parent": "root", "length": 40, "rotation": 90},
	{"name": "hand", "parent": "arm", "length": 20, "x": 40}
],
"slots": [
	{"name": "arm", "bone": "arm", "attachment": "arm"},
	{"name": "hand", "bone": "hand", "attachment": "hand", "color": "ff000080", "blend": "additive"}
],
"skins": [{"name": "default", "attachments": {
	"arm": {"arm": {"type": "mesh", "width": 10, "height": 40,
		"uvs": [0, 0, 1, 0, 1, 1, 0, 1],
		"triangles": [0, 1, 2, 0, 2, 3],
		"vertices": [1, 1, 40, 5, 1,  1, 1, 40, -5, 1,  2, 1, 0, -5, 0.5, 2, -40, -5, 0.5,  1, 1, 0, 5, 1],
		"hull": 4}},
	"hand": {"hand": {"width": 20, "height": 10, "x": 10}}
}}],
"events": {"hit": {}},
"animations": {
	"wave": {
		"bones": {"arm": {"rotate": [{"value": 0}, {"time": 1, "value": 90}]}},
		"slots": {"hand": {"attachment": [{"name": "hand"}, {"time": 0.5}]}},
		"events": [{"time": 0.5, "name": "hit"}]
	}
}
}`

func mustParse(t *testing.T, src string) *Skeleton {
	t.Helper()
	sk, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return sk
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-3 }

func TestBoneWorldTransforms(t *testing.T) {
	sk := mustParse(t, testSkeleton)
	p := newPose(sk, map[string]bool{"default": true}, &noteSet{})
	anim := sk.Animations["wave"]
	p.apply(anim, 0)
	hand := p.boneByNm["hand"]
	if !near(hand.worldX, 0) || !near(hand.worldY, 40) {
		t.Fatalf("hand at rest = %.2f,%.2f, want 0,40", hand.worldX, hand.worldY)
	}
	// Along the hand's own x, which points up the screen.
	x, y := hand.localToWorld(10, 0)
	if !near(x, 0) || !near(y, 50) {
		t.Fatalf("hand tip = %.2f,%.2f, want 0,50", x, y)
	}
	p.apply(anim, 1)
	if !near(hand.worldX, -40) || !near(hand.worldY, 0) {
		t.Fatalf("hand at the end = %.2f,%.2f, want -40,0", hand.worldX, hand.worldY)
	}
	p.apply(anim, 0.5)
	if !near(hand.worldX, -40/math.Sqrt2) || !near(hand.worldY, 40/math.Sqrt2) {
		t.Fatalf("hand halfway = %.2f,%.2f (linear key)", hand.worldX, hand.worldY)
	}
}

func TestInheritModes(t *testing.T) {
	sk := mustParse(t, `{"skeleton": {"spine": "4.2"},
	"bones": [
		{"name": "root", "rotation": 45, "scaleX": 2, "scaleY": 2},
		{"name": "t", "parent": "root", "x": 10, "inherit": "onlyTranslation"},
		{"name": "s", "parent": "root", "x": 10, "inherit": "noScale"},
		{"name": "r", "parent": "root", "x": 10, "inherit": "noRotationOrReflection"}
	], "slots": [], "skins": []}`)
	p := newPose(sk, map[string]bool{"default": true}, &noteSet{})
	p.apply(&Animation{}, 0)
	for _, name := range []string{"t", "s", "r"} {
		b := p.boneByNm[name]
		// Position always inherits: 10 along a root scaled 2 and turned 45.
		if !near(b.worldX, 20*math.Cos(math.Pi/4)) || !near(b.worldY, 20*math.Sin(math.Pi/4)) {
			t.Errorf("%s at %.2f,%.2f", name, b.worldX, b.worldY)
		}
	}
	tr := p.boneByNm["t"]
	if !near(tr.a, 1) || !near(tr.c, 0) {
		t.Errorf("onlyTranslation keeps identity axes, got a=%.2f c=%.2f", tr.a, tr.c)
	}
	s := p.boneByNm["s"]
	if !near(math.Hypot(s.a, s.c), 1) || !near(atan2Deg(s.c, s.a), 45) {
		t.Errorf("noScale keeps rotation without scale, got len %.2f angle %.2f", math.Hypot(s.a, s.c), atan2Deg(s.c, s.a))
	}
	r := p.boneByNm["r"]
	if !near(atan2Deg(r.c, r.a), 0) || !near(math.Hypot(r.a, r.c), 2) {
		t.Errorf("noRotationOrReflection keeps scale without rotation, got len %.2f angle %.2f", math.Hypot(r.a, r.c), atan2Deg(r.c, r.a))
	}
}

func TestWeightedMeshAndRegion(t *testing.T) {
	sk := mustParse(t, testSkeleton)
	c := &converter{sk: sk, notes: &noteSet{}, images: map[string][]byte{}, assetByID: map[string]*imageAsset{}, missing: map[string]bool{}}
	c.opts.Mesh = MeshTriangles
	skins := map[string]bool{"default": true}
	c.pose = newPose(sk, skins, c.notes)
	c.resolveAttachments(skins)
	c.pose.apply(sk.Animations["wave"], 0)

	arm := c.draw[0]["arm"]
	v := c.worldVertices(arm)
	// Vertex 2 is half on the arm at (0,-5) and half on the hand at (-40,-5):
	// both land on the same world point, so the blend is exact.
	if !near(v[4], 5) || !near(v[5], 0) {
		t.Fatalf("weighted vertex = %.2f,%.2f, want 5,0", v[4], v[5])
	}
	if !near(v[0], -5) || !near(v[1], 40) {
		t.Fatalf("tip vertex = %.2f,%.2f, want -5,40", v[0], v[1])
	}
	hand := c.draw[1]["hand"]
	hv := c.worldVertices(hand)
	var cx, cy float64
	for i := 0; i < 8; i += 2 {
		cx += hv[i] / 4
		cy += hv[i+1] / 4
	}
	if !near(cx, 0) || !near(cy, 50) {
		t.Fatalf("region centre = %.2f,%.2f, want 0,50", cx, cy)
	}
	if hand.color != [4]float64{1, 1, 1, 1} || c.pose.slots[1].color[3] < 0.5 || c.pose.slots[1].color[3] > 0.51 {
		t.Fatalf("slot color %v", c.pose.slots[1].color)
	}
}

func TestDeformTimeline(t *testing.T) {
	src := strings.Replace(testSkeleton, `"events": [{"time": 0.5, "name": "hit"}]`,
		`"events": [{"time": 0.5, "name": "hit"}],
		"attachments": {"default": {"arm": {"arm": {"deform": [{"offset": 2, "vertices": [0, 0]}, {"time": 1, "offset": 2, "vertices": [10, 0]}]}}}}`, 1)
	sk := mustParse(t, src)
	c := &converter{sk: sk, notes: &noteSet{}, images: map[string][]byte{}, assetByID: map[string]*imageAsset{}, missing: map[string]bool{}}
	skins := map[string]bool{"default": true}
	c.pose = newPose(sk, skins, c.notes)
	c.resolveAttachments(skins)
	c.pose.apply(sk.Animations["wave"], 0.5)
	// The key at offset 2 moves the second weight entry (vertex 1) by 10
	// along its bone's x by the end; halfway it is 5.
	if got := c.pose.deforms[deformKey{"arm", "arm"}]; len(got) != 4 || !near(got[2], 5) {
		t.Fatalf("deform at half time = %v, want [0 0 5 0]", got)
	}
	v := c.worldVertices(c.draw[0]["arm"])
	c.pose.deforms = map[deformKey][]float64{}
	plain := c.worldVertices(c.draw[0]["arm"])
	// The arm points at 135 degrees at half time; vertex 1 moved 5 along it.
	if !near(v[2]-plain[2], 5*math.Cos(3*math.Pi/4)) || !near(v[3]-plain[3], 5*math.Sin(3*math.Pi/4)) {
		t.Fatalf("deformed vertex moved by %.2f,%.2f", v[2]-plain[2], v[3]-plain[3])
	}
}

func TestCurves(t *testing.T) {
	p := &pose{}
	keys := []Key{{Time: 0, Value: f(0), Curve: json.RawMessage(`[0.5, 0, 0.5, 10]`)}, {Time: 1, Value: f(10)}}
	get := func(k *Key) float64 { return *k.Value }
	if v := p.value1(keys, 0.5, get, 0); !near(v, 5) {
		t.Errorf("symmetric bezier at midpoint = %.3f, want 5", v)
	}
	if v := p.value1(keys, 0.25, get, 0); v >= 2.5 {
		t.Errorf("ease-in should lag the line: %.3f", v)
	}
	stepped := []Key{{Time: 0, Value: f(0), Curve: json.RawMessage(`"stepped"`)}, {Time: 1, Value: f(10)}}
	if v := p.value1(stepped, 0.9, get, 0); v != 0 {
		t.Errorf("stepped = %.3f", v)
	}
	if v := p.value1(keys, -1, get, 7); v != 7 {
		t.Errorf("before the first key = %.3f, want the default", v)
	}
	if v := p.value1(keys, 5, get, 0); v != 10 {
		t.Errorf("after the last key = %.3f", v)
	}
	legacy := &pose{legacy: true}
	lk := []Key{{Time: 0, Value: f(0), Curve: json.RawMessage(`[0.5, 0, 0.5, 1]`)}, {Time: 1, Value: f(10)}}
	if v := legacy.value1(lk, 0.5, get, 0); !near(v, 5) {
		t.Errorf("legacy normalized bezier at midpoint = %.3f", v)
	}
}

func f(v float64) *float64 { return &v }

func TestTwoBoneIK(t *testing.T) {
	sk := mustParse(t, `{"skeleton": {"spine": "4.2"},
	"bones": [
		{"name": "root"},
		{"name": "target", "parent": "root", "x": 30, "y": 40},
		{"name": "upper", "parent": "root", "length": 40, "rotation": 90},
		{"name": "lower", "parent": "upper", "length": 40, "x": 40}
	],
	"slots": [], "skins": [],
	"ik": [{"name": "leg", "bones": ["upper", "lower"], "target": "target", "bendPositive": false}]}`)
	p := newPose(sk, map[string]bool{"default": true}, &noteSet{})
	p.apply(&Animation{}, 0)
	lower := p.boneByNm["lower"]
	tipX, tipY := lower.localToWorld(40, 0)
	if !near(tipX, 30) || !near(tipY, 40) {
		t.Fatalf("IK tip = %.2f,%.2f, want the target 30,40", tipX, tipY)
	}
	// The bend goes the negative way: the knee sits on the far side.
	if lower.worldX > 0 {
		t.Fatalf("knee at %.2f,%.2f should bend negative", lower.worldX, lower.worldY)
	}
	// Out of reach: the chain straightens toward the target.
	sk.Bones[1].X, sk.Bones[1].Y = 300, 0
	p = newPose(sk, map[string]bool{"default": true}, &noteSet{})
	p.apply(&Animation{}, 0)
	lower = p.boneByNm["lower"]
	tipX, tipY = lower.localToWorld(40, 0)
	if !near(tipX, 80) || !near(tipY, 0) {
		t.Fatalf("straightened tip = %.2f,%.2f, want 80,0", tipX, tipY)
	}
}

func TestSingleBoneIKAndTransformConstraint(t *testing.T) {
	sk := mustParse(t, `{"skeleton": {"spine": "4.2"},
	"bones": [
		{"name": "root"},
		{"name": "target", "parent": "root", "x": 0, "y": 50},
		{"name": "arm", "parent": "root", "length": 40},
		{"name": "follow", "parent": "root", "x": 100}
	],
	"slots": [], "skins": [],
	"ik": [{"name": "aim", "order": 0, "bones": ["arm"], "target": "target"}],
	"transform": [{"name": "copy", "order": 1, "bones": ["follow"], "target": "arm", "mixX": 0, "mixY": 0, "mixScaleX": 0, "mixShearY": 0}]}`)
	p := newPose(sk, map[string]bool{"default": true}, &noteSet{})
	p.apply(&Animation{}, 0)
	arm := p.boneByNm["arm"]
	if !near(atan2Deg(arm.c, arm.a), 90) {
		t.Fatalf("arm aims at %.2f degrees, want 90", atan2Deg(arm.c, arm.a))
	}
	follow := p.boneByNm["follow"]
	if !near(atan2Deg(follow.c, follow.a), 90) || !near(follow.worldX, 100) {
		t.Fatalf("follow copies rotation only: angle %.2f at x %.2f", atan2Deg(follow.c, follow.a), follow.worldX)
	}
}

func TestAtlasParseAndMapping(t *testing.T) {
	atlas, err := ParseAtlas([]byte(`
page.png
	size: 200, 100
	filter: Linear, Linear
	pma: true
plain
	bounds: 10, 20, 30, 40
turned
	bounds: 50, 10, 30, 40
	offsets: 2, 3, 40, 50
	rotate: 90
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(atlas.Pages) != 1 || atlas.Pages[0].Width != 200 || !atlas.Pages[0].PMA || len(atlas.Pages[0].Regions) != 2 {
		t.Fatalf("pages %+v", atlas.Pages[0])
	}
	plain := atlas.Find("plain")
	if u, v := plain.pageUV(0, 0); !near(u, 10.0/200) || !near(v, 20.0/100) {
		t.Errorf("plain top left = %.3f,%.3f", u, v)
	}
	if u, v := plain.pageUV(1, 1); !near(u, 40.0/200) || !near(v, 60.0/100) {
		t.Errorf("plain bottom right = %.3f,%.3f", u, v)
	}
	turned := atlas.Find("turned.png")
	if turned == nil {
		t.Fatal("extension-stripped lookup failed")
	}
	// The packed 30x40 area sits at original pixels x 2..32, y 7..47
	// (offset y from the bottom: 50-3-40). Rotated 90 clockwise it
	// occupies 40x30 on the page at 50,10: the packed top-left corner
	// lands at the page's top-right of that area.
	if u, v := turned.pageUV(2.0/40, 7.0/50); !near(u, 50.0/200) || !near(v, 40.0/100) {
		t.Errorf("turned packed top left = %.3f,%.3f, want 0.25,0.40", u, v)
	}
	if u, v := turned.pageUV(32.0/40, 7.0/50); !near(u, 50.0/200) || !near(v, 10.0/100) {
		t.Errorf("turned packed top right = %.3f,%.3f, want 0.25,0.10", u, v)
	}
	if u, v := turned.pageUV(2.0/40, 47.0/50); !near(u, 90.0/200) || !near(v, 40.0/100) {
		t.Errorf("turned packed bottom left = %.3f,%.3f, want 0.45,0.40", u, v)
	}
	u0, v0, u1, v1 := turned.packedRect()
	if !near(u0, 2.0/40) || !near(v0, 7.0/50) || !near(u1, 32.0/40) || !near(v1, 47.0/50) {
		t.Errorf("packed rect %.3f %.3f %.3f %.3f", u0, v0, u1, v1)
	}
}

func TestParseLegacySkins(t *testing.T) {
	sk := mustParse(t, `{"skeleton": {"spine": "3.8.99"}, "bones": [{"name": "root"}],
	"slots": [{"name": "s", "bone": "root", "attachment": "a"}],
	"skins": {"default": {"s": {"a": {"width": 4, "height": 4}}}, "alt": {"s": {"a": {"width": 8, "height": 8}}}}}`)
	if len(sk.Skins) != 2 || sk.Skins[0].Name != "default" {
		t.Fatalf("skins %+v", sk.Skins)
	}
	if sk.Info.Major() != 3 {
		t.Fatalf("major %d", sk.Info.Major())
	}
}

func testPNG(w, h int) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x * 255 / w), uint8(y * 255 / h), 128, 255})
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func TestConvertProducesPlayableBundle(t *testing.T) {
	sk := mustParse(t, testSkeleton)
	for _, mode := range []MeshMode{MeshTriangles, MeshHull} {
		res, err := Convert(sk, Options{
			Mesh:      mode,
			MachineID: "rig",
			Bones:     true,
			ReadImage: func(path string) ([]byte, error) { return testPNG(20, 10), nil },
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(res.ClipOrder) != 1 || res.ClipOrder[0] != "wave" {
			t.Fatalf("clips %v", res.ClipOrder)
		}
		// The hand dips 5 below the floor at the end of the wave, so the
		// composition grows past the skeleton's declared 100x100.
		if res.Width != 100 || res.Height != 105 || res.OriginX != 50 || res.OriginY != 100 {
			t.Fatalf("composition %gx%g origin %g,%g", res.Width, res.Height, res.OriginX, res.OriginY)
		}
		if len(res.Images) != 2 {
			t.Fatalf("images %v", res.Images)
		}
		b, err := res.Bundle()
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := b.Encode(&buf); err != nil {
			t.Fatal(err)
		}
		b, err = lottie.DecodeBundle(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		if err != nil {
			t.Fatal(err)
		}
		if problems := b.Validate(); len(problems) > 0 {
			t.Fatal(problems)
		}
		anim, err := b.Animation("wave")
		if err != nil {
			t.Fatal(err)
		}
		if u := anim.UnsupportedFeatures(); len(u) > 0 {
			t.Fatalf("unsupported %v", u)
		}
		w, h := anim.Size()
		if w != 100 || h != 105 || anim.Duration() <= 0 {
			t.Fatalf("size %dx%d duration %v", w, h, anim.Duration())
		}
		m, ok := anim.Marker("hit")
		if !ok || m.Start != 15 {
			t.Fatalf("marker %+v %v", m, ok)
		}
		doc, err := lottietexture.Load(b, "wave")
		if err != nil || doc == nil {
			t.Fatalf("doc %v %v", doc, err)
		}
		paths := 2
		if mode == MeshHull {
			paths = 1
		}
		if len(doc.Paints) != 2 || len(doc.UVs) != paths+1 {
			t.Fatalf("%s: paints %d uvs %d", mode, len(doc.Paints), len(doc.UVs))
		}
		p := anim.NewPlayer()
		if err := doc.Apply(p); err != nil {
			t.Fatalf("%s: apply: %v", mode, err)
		}
		if got := strings.Join(p.TextureNames(), ","); got != "arm,hand" {
			t.Fatalf("textures %q", got)
		}
		sm, err := b.StateMachine("rig")
		if err != nil || sm.Initial != "wave" || len(sm.Inputs) != 1 {
			t.Fatalf("machine %+v %v", sm, err)
		}
		if _, err := b.NewStateMachinePlayer("rig"); err != nil {
			t.Fatal(err)
		}
		// The clip stays plain Lottie: no x- members, additive blend on the
		// hand, a null layer per bone.
		var clip struct {
			Layers []struct {
				Type int    `json:"ty"`
				Name string `json:"nm"`
				BM   int    `json:"bm"`
			} `json:"layers"`
		}
		json.Unmarshal(res.Clips["wave"], &clip)
		if len(clip.Layers) != 2+3 || clip.Layers[0].Name != "hand" || clip.Layers[0].BM != 16 || clip.Layers[2].Type != 3 {
			t.Fatalf("layers %+v", clip.Layers)
		}
		if bytes.Contains(res.Clips["wave"], []byte(`"x-`)) {
			t.Fatal("clip carries an x- member")
		}
	}
}

func TestConvertWithAtlas(t *testing.T) {
	sk := mustParse(t, testSkeleton)
	atlas, err := ParseAtlas([]byte("sheet.png\n\tsize: 64, 32\n\tpma: true\narm\n\tbounds: 0, 0, 10, 40\n\trotate: 90\nhand\n\tbounds: 40, 0, 20, 10\n"))
	if err != nil {
		t.Fatal(err)
	}
	pages := 0
	res, err := Convert(sk, Options{
		Atlas: atlas,
		ReadPage: func(name string) ([]byte, error) {
			pages++
			if name != "sheet.png" {
				t.Errorf("page %q", name)
			}
			return testPNG(64, 32), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if pages != 1 || len(res.Images) != 1 {
		t.Fatalf("pages read %d, images %d", pages, len(res.Images))
	}
	doc := res.Docs["wave"]
	for _, pt := range doc.Paints {
		if pt.Texture != "sheet" {
			t.Fatalf("paint texture %q", pt.Texture)
		}
	}
	// The hand's four corners cover the region at 40,0 size 20x10.
	var hand *lottietexture.UV
	for i := range doc.UVs {
		if doc.UVs[i].Layer == 2 {
			hand = &doc.UVs[i]
		}
	}
	if hand == nil || len(hand.V) != 4 {
		t.Fatalf("hand uv %+v", hand)
	}
	if !near(hand.V[0][0], 40.0/64) || !near(hand.V[0][1], 0) || !near(hand.V[2][0], 60.0/64) || !near(hand.V[2][1], 10.0/32) {
		t.Fatalf("hand corners %v", hand.V)
	}
	// A premultiplied page is un-premultiplied: still decodable, same size.
	cfg, _, err := image.DecodeConfig(bytes.NewReader(res.Images["sheet.png"]))
	if err != nil || cfg.Width != 64 {
		t.Fatalf("page image %v %v", cfg, err)
	}
}

func TestSkeletonBounds(t *testing.T) {
	sk := mustParse(t, testSkeleton)
	res, err := Convert(sk, Options{SkeletonBounds: true, Scale: 2})
	if err != nil {
		t.Fatal(err)
	}
	if res.Width != 200 || res.Height != 200 || res.OriginX != 100 || res.OriginY != 200 {
		t.Fatalf("composition %gx%g origin %g,%g", res.Width, res.Height, res.OriginX, res.OriginY)
	}
	if len(res.Images) != 0 {
		t.Fatal("no reader, no images")
	}
	if len(res.Docs["wave"].Paints) != 2 {
		t.Fatal("paints keep their texture names for a runtime binding")
	}
}

func TestTrackCompression(t *testing.T) {
	tr := &track{}
	for i := 0; i <= 10; i++ {
		v := float64(i)
		if i > 5 {
			v = 5
		}
		tr.values = append(tr.values, []float64{v})
	}
	if got := tr.keyFrames(); len(got) != 3 || got[1] != 5 {
		t.Fatalf("ramp then plateau keys = %v, want [0 5 10]", got)
	}
	tr.hold = make([]bool, 11)
	tr.hold[7] = true
	if got := tr.keyFrames(); len(got) != 5 {
		t.Fatalf("a hold keeps itself and its successor: %v", got)
	}
	static := &track{values: [][]float64{{1, 2}, {1, 2}, {1, 2}}}
	if !static.static() {
		t.Fatal("constant track should be static")
	}
	prop := static.property(vecValue, false)
	if prop["a"] != 0 {
		t.Fatalf("static property %v", prop)
	}
}

func TestClipIDs(t *testing.T) {
	used := map[string]bool{}
	if id := clipID("run/left", used); id != "run-left" {
		t.Fatal(id)
	}
	if id := clipID("run/left", used); id != "run-left-2" {
		t.Fatal(id)
	}
	if id := clipID("", used); id != "clip" {
		t.Fatal(id)
	}
}
