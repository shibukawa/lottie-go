package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"regexp"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	lottietexture "github.com/shibukawa/lottie-go/plugin/texture"
)

// The texture document is edited woven and stored aside: these tests read
// the bundle bytes back, the way the shape tests do, so what is asserted is
// what a save would write — a pure clip and a document beside it.

var xMember = regexp.MustCompile(`"x-[^"\\]*":`)

func storedIsPure(t *testing.T, m *Model, id string) {
	t.Helper()
	data, ok := m.Bundle().AnimationJSON(id)
	if !ok {
		t.Fatalf("no stored clip %q", id)
	}
	if xMember.Match(data) {
		t.Fatalf("a working member reached the stored clip: %s", xMember.Find(data))
	}
}

func texturedModel(t *testing.T) *Model {
	t.Helper()
	m := NewModel()
	if err := m.Bundle().SetAnimation("vec", []byte(vectorClipJSON)); err != nil {
		t.Fatalf("fixture clip rejected: %v", err)
	}
	doc := &lottietexture.Doc{
		Paints: []lottietexture.Paint{{Layer: 1, Item: []int{0, 1}, Texture: "skin", Mapping: lottietexture.MappingVertex}},
		UVs:    []lottietexture.UV{{Layer: 1, Item: []int{0, 0}, V: [][2]float64{{0, 0.5}, {0.5, 0}, {1, 0.5}, {0.5, 1}}}},
	}
	if err := lottietexture.Store(m.Bundle(), "vec", doc); err != nil {
		t.Fatal(err)
	}
	m.ShowClip(clipRef{Anim: "vec"})
	if m.PreviewPlayer() == nil {
		t.Fatalf("clip did not go on stage: %s", m.Status())
	}
	m.PausePreview()
	m.SetCollisionTab(colShapes)
	return m
}

func TestTextureDocumentWeavesIntoTheStagedClip(t *testing.T) {
	m := texturedModel(t)
	m.SelectShapeNode([]int{0, 1})
	if !m.ShapeCanTexture() {
		t.Fatal("a fill cannot take a texture")
	}
	if got := m.ShapeTextureName(); got != "skin" {
		t.Fatalf("woven texture = %q", got)
	}
	if got := m.ShapeTextureString("mapping"); got != "vertex" {
		t.Fatalf("woven mapping = %q", got)
	}
	m.SelectShapeNode([]int{0, 0})
	uv, ok := m.ShapeUVs()
	if !ok || len(uv) != 4 || uv[1] != [2]float64{0.5, 0} {
		t.Fatalf("woven uv = %v ok=%v", uv, ok)
	}
	if !m.ShapeUVEditable() {
		t.Fatal("the path with a UV set is not UV-editable")
	}
	// The bundle still holds a plain clip: weaving is in-memory only.
	storedIsPure(t, m, "vec")
	// The stage player was dressed from the document.
	if names := m.PreviewPlayer().TextureNames(); len(names) != 1 || names[0] != "skin" {
		t.Fatalf("stage player textures = %v", names)
	}
}

func TestTextureEditStoresPureClipAndDocument(t *testing.T) {
	m := shapeModel(t)
	m.SelectShapeNode([]int{0, 1})
	if _, has := m.ShapeTexture(); has {
		t.Fatal("fixture fill already textured")
	}
	m.SetShapeTexture("skin")
	storedIsPure(t, m, "vec")
	doc, err := lottietexture.Load(m.Bundle(), "vec")
	if err != nil || doc == nil || len(doc.Paints) != 1 {
		t.Fatalf("stored document = %+v, %v", doc, err)
	}
	p := doc.Paints[0]
	if p.Layer != 1 || len(p.Item) != 2 || p.Item[0] != 0 || p.Item[1] != 1 || p.Texture != "skin" {
		t.Fatalf("paint address = %+v", p)
	}
	// Switching to vertex mapping seeds the sibling path's UV set from its
	// bounding box, so the document gains a UV entry too.
	m.SetShapeTextureString("mapping", string(lottietexture.MappingVertex))
	doc, _ = lottietexture.Load(m.Bundle(), "vec")
	if doc == nil || len(doc.UVs) != 1 || len(doc.UVs[0].V) != 4 {
		t.Fatalf("no seeded UV in the document: %+v", doc)
	}
	if doc.UVs[0].Item[0] != 0 || doc.UVs[0].Item[1] != 0 {
		t.Fatalf("UV address = %v", doc.UVs[0].Item)
	}
	// The diamond's leftmost vertex is at u 0, the top one at v 0.
	if v := doc.UVs[0].V; v[0][0] != 0 || v[1][1] != 0 || v[2][0] != 1 || v[3][1] != 1 {
		t.Fatalf("seeded UV = %v", v)
	}
	storedIsPure(t, m, "vec")
	// Undo takes the document back with the clip, step by step.
	m.UndoClipEdit()
	doc, _ = lottietexture.Load(m.Bundle(), "vec")
	if doc == nil || len(doc.UVs) != 0 || len(doc.Paints) != 1 {
		t.Fatalf("after one undo: %+v", doc)
	}
	m.UndoClipEdit()
	if doc, _ := lottietexture.Load(m.Bundle(), "vec"); doc != nil {
		t.Fatalf("after two undos the document remains: %+v", doc)
	}
	m.SelectShapeNode([]int{0, 1})
	if _, has := m.ShapeTexture(); has {
		t.Fatal("the woven paint survived the undo")
	}
}

func TestPaintOptionsAndTransformRoundTrip(t *testing.T) {
	m := texturedModel(t)
	m.SelectShapeNode([]int{0, 1})
	m.SetShapeTextureString("wrap", string(lottietexture.WrapRepeat))
	m.SetShapeTextureTint(false)
	m.SetShapeTexTransformComponent("s", 0, 200)
	doc, _ := lottietexture.Load(m.Bundle(), "vec")
	p := doc.Paints[0]
	if p.Wrap != lottietexture.WrapRepeat || p.Tinted() {
		t.Fatalf("paint options = %+v", p)
	}
	if !bytes.Contains(p.Transform, []byte(`"s"`)) || !bytes.Contains(p.Transform, []byte(`200`)) {
		t.Fatalf("transform = %s", p.Transform)
	}
	// Reads come back through the woven form.
	if v, ok := m.ShapeTexTransformValue("s"); !ok || v[0] != 200 || v[1] != 100 {
		t.Fatalf("transform s = %v ok=%v", v, ok)
	}
	m.SetShapeTextureTint(true)
	doc, _ = lottietexture.Load(m.Bundle(), "vec")
	if doc.Paints[0].Tint != nil {
		t.Fatal("tint true should drop the member")
	}
	m.SetShapeTexture("")
	doc, _ = lottietexture.Load(m.Bundle(), "vec")
	if doc == nil || len(doc.Paints) != 0 || len(doc.UVs) != 1 {
		t.Fatalf("after removing the paint: %+v", doc)
	}
}

func TestVertexInsertAndDeleteKeepUVInStep(t *testing.T) {
	m := texturedModel(t)
	m.SelectShapeNode([]int{0, 0})
	m.SelectPoseKey(0, -1)
	m.InsertShapeVertex(1, 0.5)
	uv, ok := m.ShapeUVs()
	if !ok || len(uv) != 5 {
		t.Fatalf("after insert uv = %v ok=%v", uv, ok)
	}
	// The new point sits between its neighbours (0.5,0) and (1,0.5).
	if uv[2] != [2]float64{0.75, 0.25} {
		t.Fatalf("inserted uv = %v", uv[2])
	}
	doc, _ := lottietexture.Load(m.Bundle(), "vec")
	if len(doc.UVs[0].V) != 5 {
		t.Fatalf("stored uv count = %d", len(doc.UVs[0].V))
	}
	m.SelectShapeVert(2)
	m.DeleteShapeVertex()
	uv, ok = m.ShapeUVs()
	if !ok || len(uv) != 4 || uv[2] != [2]float64{1, 0.5} {
		t.Fatalf("after delete uv = %v ok=%v", uv, ok)
	}
	// The stage player is rebuilt with the document on every store-back,
	// so the renderer always sees a UV set matching the path.
	if names := m.PreviewPlayer().TextureNames(); len(names) != 1 {
		t.Fatalf("stage player lost its paint: %v", names)
	}
}

func TestUVEditsReachTheDocument(t *testing.T) {
	m := texturedModel(t)
	m.SelectShapeNode([]int{0, 0})
	m.SelectPoseKey(0, -1)
	m.SetShapeUV(0, 0.1, 0.2)
	m.MoveShapeUV(-1, 0.1, 0)
	doc, _ := lottietexture.Load(m.Bundle(), "vec")
	if v := doc.UVs[0].V; v[0] != [2]float64{0.2, 0.2} || v[1] != [2]float64{0.6, 0} {
		t.Fatalf("uv after edits = %v", v)
	}
	before := doc.UVs[0].V
	m.ScaleShapeUV(2)
	doc, _ = lottietexture.Load(m.Bundle(), "vec")
	// Scaling about the centroid keeps the centroid and doubles the spread.
	var cu0, cu1 float64
	for i, p := range doc.UVs[0].V {
		cu0 += before[i][0]
		cu1 += p[0]
	}
	if d := cu1 - cu0; d < -1e-6 || d > 1e-6 {
		t.Fatalf("centroid moved by %v", d/4)
	}
	if got, want := doc.UVs[0].V[2][0]-doc.UVs[0].V[0][0], 2*(before[2][0]-before[0][0]); got < want-1e-6 || got > want+1e-6 {
		t.Fatalf("spread %v, want %v", got, want)
	}
	m.ClearShapeUV()
	doc, _ = lottietexture.Load(m.Bundle(), "vec")
	if len(doc.UVs) != 0 {
		t.Fatalf("uv not cleared: %+v", doc.UVs)
	}
	m.SeedShapeUV()
	doc, _ = lottietexture.Load(m.Bundle(), "vec")
	if len(doc.UVs) != 1 || len(doc.UVs[0].V) != 4 {
		t.Fatalf("uv not re-seeded: %+v", doc.UVs)
	}
}

func TestClipOperationsCarryTheDocument(t *testing.T) {
	m := texturedModel(t)
	copyID := m.DuplicateClip("vec")
	if copyID == "" {
		t.Fatal("duplicate failed")
	}
	if doc, _ := lottietexture.Load(m.Bundle(), copyID); doc == nil || len(doc.Paints) != 1 {
		t.Fatalf("copy has no document: %+v", doc)
	}
	m.RenameClip(copyID, "variant")
	if doc, _ := lottietexture.Load(m.Bundle(), "variant"); doc == nil {
		t.Fatal("renamed clip lost its document")
	}
	if doc, _ := lottietexture.Load(m.Bundle(), copyID); doc != nil {
		t.Fatal("the old id kept a document")
	}
	m.RemoveClip("variant")
	if doc, _ := lottietexture.Load(m.Bundle(), "variant"); doc != nil {
		t.Fatal("removed clip kept a document")
	}
	if _, ok := m.Bundle().ExtensionFile(lottietexture.File("vec")); !ok {
		t.Fatal("the original's document went with the copy")
	}
}

func TestUnplacedEntriesAreKeptAndReported(t *testing.T) {
	m := NewModel()
	if err := m.Bundle().SetAnimation("vec", []byte(vectorClipJSON)); err != nil {
		t.Fatal(err)
	}
	doc := &lottietexture.Doc{Paints: []lottietexture.Paint{
		{Layer: 1, Item: []int{0, 1}, Texture: "skin"},
		{Layer: 9, Item: []int{0}, Texture: "gone"},
	}}
	if err := lottietexture.Store(m.Bundle(), "vec", doc); err != nil {
		t.Fatal(err)
	}
	m.ShowClip(clipRef{Anim: "vec"})
	m.PausePreview()
	m.SetCollisionTab(colShapes)
	if got := m.UnplacedTextures(); len(got) != 1 {
		t.Fatalf("unplaced = %v", got)
	}
	// An unrelated edit writes the stale entry back untouched.
	m.SelectShapeNode([]int{0, 1})
	m.SetShapeTextureString("wrap", string(lottietexture.WrapMirror))
	stored, _ := lottietexture.Load(m.Bundle(), "vec")
	if len(stored.Paints) != 2 || stored.Paints[1].Texture != "gone" || stored.Paints[1].Layer != 9 {
		t.Fatalf("stale entry not written back: %+v", stored.Paints)
	}
	m.DropUnplacedTextures()
	stored, _ = lottietexture.Load(m.Bundle(), "vec")
	if len(stored.Paints) != 1 {
		t.Fatalf("drop left %+v", stored.Paints)
	}
	if got := m.UnplacedTextures(); len(got) != 0 {
		t.Fatalf("still reported after drop: %v", got)
	}
}

func TestBindTextureFileAddsTheImageAsset(t *testing.T) {
	m := shapeModel(t)
	var buf bytes.Buffer
	img := image.NewNRGBA(image.Rect(0, 0, 8, 4))
	img.Set(0, 0, color.NRGBA{255, 0, 0, 255})
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	m.Bundle().SetImage("skin.png", buf.Bytes())
	choices := m.TextureChoices()
	if len(choices) != 1 || choices[0].File != "skin.png" || choices[0].ID != "" {
		t.Fatalf("choices before binding = %+v", choices)
	}
	m.SelectShapeNode([]int{0, 1})
	m.BindTextureFile("skin.png")
	d := storedDoc(t, m)
	assets, _ := d.root["assets"].([]any)
	if len(assets) != 1 {
		t.Fatalf("assets = %v", assets)
	}
	as := assets[0].(map[string]any)
	if as["p"] != "skin.png" || as["id"] != "skin" {
		t.Fatalf("asset = %v", as)
	}
	if w, _ := jsonNum(as["w"]); w != 8 {
		t.Fatalf("asset width = %v", as["w"])
	}
	doc, _ := lottietexture.Load(m.Bundle(), "vec")
	if doc.Paints[0].Texture != "skin" {
		t.Fatalf("paint names %q", doc.Paints[0].Texture)
	}
	if got := m.TextureFile("skin"); got != "skin.png" {
		t.Fatalf("TextureFile = %q", got)
	}
	// The stage player resolves the asset to a real image.
	if names := m.PreviewPlayer().TextureNames(); len(names) != 1 || names[0] != "skin" {
		t.Fatalf("stage textures = %v", names)
	}
	// Binding the same file again reuses the asset.
	m.BindTextureFile("skin.png")
	if assets, _ := storedDoc(t, m).root["assets"].([]any); len(assets) != 1 {
		t.Fatalf("asset duplicated: %v", assets)
	}
}

// Opening another bundle drops the decoded texture images: the cache is
// keyed by file name, and the new bundle may carry a different picture
// under the same name.
func TestOpeningABundleDropsTheTextureImageCache(t *testing.T) {
	m := texturedModel(t)
	m.texImages = map[string]*ebiten.Image{"skin.png": nil}
	m.texDocs = map[string]cachedTexDoc{"vec": {}}
	dir := t.TempDir()
	m.Open(writeClip(t, dir, "other", 10, ""))
	if m.texImages != nil || m.texDocs != nil {
		t.Fatalf("caches survived Open: %v %v", m.texImages, m.texDocs)
	}
	m = texturedModel(t)
	m.texImages = map[string]*ebiten.Image{"skin.png": nil}
	m.RemoveClip("vec")
	if m.texImages != nil {
		t.Fatalf("cache survived RemoveClip: %v", m.texImages)
	}
}

// The texture document is parsed once per stored bytes: a stage player
// rebuilt on every drag step dresses itself from the same parse.
func TestTextureDocIsParsedOncePerChange(t *testing.T) {
	m := texturedModel(t)
	d1, err := m.textureDoc("vec")
	if err != nil || d1 == nil {
		t.Fatalf("textureDoc: %v %v", d1, err)
	}
	d2, _ := m.textureDoc("vec")
	if d1 != d2 {
		t.Fatalf("unchanged document was parsed again")
	}
	m.SelectShapeNode([]int{0, 0})
	m.SelectPoseKey(0, -1)
	m.MoveShapeUV(0, 0.1, 0)
	d3, err := m.textureDoc("vec")
	if err != nil || d3 == nil || d3 == d1 {
		t.Fatalf("changed document was not parsed again: %v %v", d3, err)
	}
	if got := d3.UVs[0].V[0][0]; got != 0.1 {
		t.Fatalf("fresh parse reads %v; want 0.1", got)
	}
	m.texDocs = nil
	if _, err := m.textureDoc("no-such-clip"); err != nil {
		t.Fatalf("a clip without a document errs: %v", err)
	}
}
