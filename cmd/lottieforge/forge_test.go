package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	lottie "github.com/shibukawa/lottie-go"
	lottietexture "github.com/shibukawa/lottie-go/plugin/texture"
)

const presetsDir = "../../examples/state-editor/presets"

const testSpec = `{
  "name": "tester",
  "base": "chibi-sword",
  "description": "a fox-eared shrine maiden with a red hakama and a naginata",
  "attachments": [
    {"name": "ponytail", "kind": "lock", "host": "head", "attach": [44, 12], "size": [26, 60], "order": "behind-head"},
    {"name": "hakama", "kind": "drape", "host": "body", "size": [60, 40], "panels": 2},
    {"name": "sleeve", "kind": "drape", "host": "upper-arm", "size": [24, 34], "paired": true},
    {"name": "fox-ears", "kind": "rigid", "host": "head", "size": [40, 30], "sway": {"amount": 4, "period": 48}},
    {"name": "tail", "kind": "swing", "host": "body", "attach": [10, 44], "size": [30, 50], "order": "behind-body", "segments": 2},
    {"name": "cape", "kind": "drape", "host": "body", "size": [56, 70], "order": "behind-body", "views": "separate"}
  ],
  "props": [{"slot": "sword", "name": "naginata", "length": 90}],
  "morph": [
    {"generator": "breathe", "parts": ["body", "head"], "clips": ["idle-anim"], "amount": 0.03, "period": 48},
    {"generator": "squash", "parts": "all", "clips": ["jump-anim"], "at": 22, "amount": 0.12, "recover": 6},
    {"generator": "bend", "parts": ["forearm", "shin"], "clips": "all", "threshold": 40}
  ],
  "clips": {
    "add": [{"name": "cast-anim", "from": "thrust-anim"}],
    "machine": [{"state": "cast-state", "animation": "cast-anim", "event": "cast", "from": ["idle-state"], "returns": "idle-state"}]
  }
}`

// paintSheets fills every template cell with a capsule so the pipeline
// has art to cut without any model output in the repository.
func paintSheets(t *testing.T, work string, m *manifest) {
	t.Helper()
	hues := []color.NRGBA{{220, 80, 80, 255}, {80, 160, 220, 255}, {90, 190, 90, 255}, {230, 200, 60, 255}, {180, 100, 200, 255}}
	for _, sh := range m.Sheets {
		if len(sh.Cells) == 0 {
			continue
		}
		img, err := loadImage(filepath.Join(work, "sheets", sh.ID+".template.png"))
		if err != nil {
			t.Fatal(err)
		}
		for i, c := range sh.Cells {
			r := image.Rect(c.Rect[0], c.Rect[1], c.Rect[2], c.Rect[3]).Inset(m.Margin + 6)
			drawCapsule(img, r, hues[i%len(hues)])
		}
		if err := writePNG(filepath.Join(work, "sheets", sh.ID+".png"), img); err != nil {
			t.Fatal(err)
		}
	}
}

// drawCapsule fills an ellipse inscribed in r with a dark outline.
func drawCapsule(img *image.NRGBA, r image.Rectangle, fill color.NRGBA) {
	cx, cy := float64(r.Min.X+r.Max.X)/2, float64(r.Min.Y+r.Max.Y)/2
	rx, ry := float64(r.Dx())/2, float64(r.Dy())/2
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			dx, dy := (float64(x)+0.5-cx)/rx, (float64(y)+0.5-cy)/ry
			d := dx*dx + dy*dy
			if d > 1 {
				continue
			}
			c := fill
			if d > 0.82 {
				c = color.NRGBA{30, 25, 35, 255}
			}
			i := img.PixOffset(x, y)
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, 255
		}
	}
}

func decodeBundleFile(t *testing.T, path string) *lottie.Bundle {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := lottie.DecodeBundle(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// checkBundle decodes every clip, binds its texture document, and
// verifies every path keeps one vertex count across its keys.
func checkBundle(t *testing.T, b *lottie.Bundle) map[string]*lottie.Animation {
	t.Helper()
	anims := map[string]*lottie.Animation{}
	for _, id := range b.AnimationIDs() {
		a, err := b.Animation(id)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if u := a.UnsupportedFeatures(); len(u) != 0 {
			t.Fatalf("%s: unsupported %v", id, u)
		}
		doc, err := lottietexture.Load(b, id)
		if err != nil {
			t.Fatalf("%s: texture doc: %v", id, err)
		}
		if doc == nil {
			t.Fatalf("%s: no texture document", id)
		}
		if err := doc.Apply(a.NewPlayer()); err != nil {
			t.Fatalf("%s: apply: %v", id, err)
		}
		raw, _ := b.AnimationJSON(id)
		var clip obj
		json.Unmarshal(raw, &clip)
		for _, l := range layersOf(clip) {
			shapes, _ := l["shapes"].([]any)
			for _, s := range shapes {
				g, _ := s.(obj)
				items, _ := g["it"].([]any)
				for _, it := range items {
					item, _ := it.(obj)
					if item["ty"] != "sh" {
						continue
					}
					ks, _ := item["ks"].(obj)
					if num(ks["a"]) == 0 {
						continue
					}
					keys, _ := ks["k"].([]any)
					count := -1
					for _, k := range keys {
						km, _ := k.(obj)
						sv, _ := km["s"].([]any)
						path, _ := sv[0].(obj)
						n := len(path["v"].([]any))
						if count >= 0 && n != count {
							t.Fatalf("%s %s: vertex count drifts across keys (%d vs %d)", id, l["nm"], count, n)
						}
						count = n
					}
				}
			}
		}
		anims[id] = a
	}
	return anims
}

func TestPipeline(t *testing.T) {
	if _, err := os.Stat(filepath.Join(presetsDir, "chibi-sword", "chibi-sword.lottie")); err != nil {
		t.Skip("presets not available:", err)
	}
	work := t.TempDir()
	if dir := os.Getenv("LOTTIEFORGE_WORK"); dir != "" {
		// Keep the pipeline's output somewhere a human can open it.
		work = dir
		os.MkdirAll(work, 0o755)
	}
	if err := os.WriteFile(filepath.Join(work, "character.json"), []byte(testSpec), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runGrid([]string{"-presets", presetsDir, work}); err != nil {
		t.Fatal("grid:", err)
	}
	m, err := loadManifest(work)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, sh := range m.Sheets {
		ids = append(ids, sh.ID)
	}
	if len(m.Sheets) != 5 {
		t.Fatalf("sheets = %v", ids)
	}
	for _, name := range []string{"model.md", "heads.md", "limbs.md", "attachments.md", "fix-cell.md"} {
		data, err := os.ReadFile(filepath.Join(work, "prompts", name))
		if err != nil {
			t.Fatal(err)
		}
		if name == "heads.md" && !bytes.Contains(data, []byte("WITHOUT the ponytail")) {
			t.Fatalf("heads prompt lacks the host wording:\n%s", data)
		}
		if name == "limbs.md" && !bytes.Contains(data, []byte("naginata")) {
			t.Fatalf("limbs prompt lacks the prop name")
		}
	}
	paintSheets(t, work, m)

	if err := runCut([]string{"-presets", presetsDir, work}); err != nil {
		t.Fatal("cut:", err)
	}
	var rep cutReport
	data, _ := os.ReadFile(filepath.Join(work, "report.json"))
	json.Unmarshal(data, &rep)
	if len(rep.Cells) == 0 {
		t.Fatal("no cells cut")
	}
	for _, c := range rep.Cells {
		if c.Status != "ok" {
			t.Errorf("cell %s %s: %s %s", c.ID, c.Slot, c.Status, c.Note)
		}
	}
	for _, f := range []string{"head.png", "upper-arm-far.png", "shadow.png", "sleeve-far.png", "cape-side.png"} {
		if _, err := os.Stat(filepath.Join(work, "parts", f)); err != nil {
			t.Errorf("parts/%s missing", f)
		}
	}

	if err := runRig([]string{"-presets", presetsDir, work}); err != nil {
		t.Fatal("rig:", err)
	}
	out := filepath.Join(work, "tester.lottie")
	b := decodeBundleFile(t, out)
	anims := checkBundle(t, b)
	idle := anims["idle-anim"]
	for _, name := range []string{"ponytail", "hakama-front", "hakama-back", "sleeve", "sleeve-far", "fox-ears", "tail", "tail-2", "cape", "cape-side", "cape-back", "head", "sword"} {
		if _, ok := idle.LayerPlacement(name, 0); !ok {
			t.Errorf("idle-anim lacks layer %s", name)
		}
	}
	if _, ok := b.AnimationJSON("cast-anim"); !ok {
		t.Error("cast-anim not added")
	}
	sm, err := b.StateMachine(b.StateMachineIDs()[0])
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, st := range sm.States {
		found = found || st.Name == "cast-state"
	}
	if !found {
		t.Error("cast-state not wired")
	}
	if len(b.ExtensionFiles(forgeDir)) < 15 {
		t.Errorf("forge contours: %d", len(b.ExtensionFiles(forgeDir)))
	}

	if err := runMorph([]string{"-presets", presetsDir, work}); err != nil {
		t.Fatal("morph:", err)
	}
	b = decodeBundleFile(t, out)
	checkBundle(t, b)
	raw, _ := b.AnimationJSON("run-anim")
	var run obj
	json.Unmarshal(raw, &run)
	keyedRot, keyedPath := false, false
	for _, l := range layersOf(run) {
		ks, _ := l["ks"].(obj)
		if l["nm"] == "tail" {
			r, _ := ks["r"].(obj)
			keyedRot = num(r["a"]) == 1
		}
		if l["nm"] == "hakama-front" {
			shapes, _ := l["shapes"].([]any)
			g, _ := shapes[0].(obj)
			items, _ := g["it"].([]any)
			sh, _ := items[0].(obj)
			pks, _ := sh["ks"].(obj)
			keyedPath = num(pks["a"]) == 1
		}
	}
	if !keyedRot {
		t.Error("run-anim: the tail did not swing")
	}
	if !keyedPath {
		t.Error("run-anim: the hakama did not drape")
	}

	// The raster tier from the same parts.
	if err := runRig([]string{"-presets", presetsDir, "-raster", "-o", filepath.Join(work, "raster.lottie"), work}); err != nil {
		t.Fatal("rig -raster:", err)
	}
	rb := decodeBundleFile(t, filepath.Join(work, "raster.lottie"))
	for _, id := range rb.AnimationIDs() {
		a, err := rb.Animation(id)
		if err != nil {
			t.Fatal(err)
		}
		if u := a.UnsupportedFeatures(); len(u) != 0 {
			t.Fatalf("raster %s: %v", id, u)
		}
	}
	if _, ok := rb.Image("tester-hakama-front.png"); !ok {
		t.Error("raster bundle lacks the attachment image")
	}
}

func TestDecompose(t *testing.T) {
	rect := []pt{{0, 0}, {10, 0}, {10, 6}, {0, 6}}
	if starIndex(rect) >= 0 {
		t.Fatal("a rectangle is star-shaped")
	}
	l := []pt{{0, 0}, {10, 0}, {10, 4}, {4, 4}, {4, 12}, {0, 12}}
	if starIndex(l) < 0 {
		t.Fatal("an L is not star-shaped about its centroid")
	}
	pieces := decompose(l)
	if len(pieces) < 2 {
		t.Fatalf("L not split: %v", pieces)
	}
	for _, p := range pieces {
		if starIndex(p) >= 0 {
			t.Errorf("piece not star-shaped: %v", p)
		}
	}
}

func TestTraceOutline(t *testing.T) {
	w, h := 20, 12
	mask := make([]bool, w*h)
	for y := 2; y < 10; y++ {
		for x := 3; x < 17; x++ {
			mask[y*w+x] = true
		}
	}
	comps := components(mask, w, h)
	if len(comps) != 1 || comps[0].area != 14*8 {
		t.Fatalf("components: %+v", comps)
	}
	outline := traceOutline(mask, w, h, comps[0])
	if len(outline) < 40 {
		t.Fatalf("outline too short: %d", len(outline))
	}
	for _, p := range outline {
		x, y := int(p[0]), int(p[1])
		if !(x == 3 || x == 16 || y == 2 || y == 9) {
			t.Fatalf("outline point %v not on the border", p)
		}
	}
	poly := simplifyTo(outline, 8)
	if len(poly) < 4 || len(poly) > 8 {
		t.Fatalf("simplified to %d vertices", len(poly))
	}
	grown := dilate(poly, 1)
	if signedArea(grown) <= signedArea(poly) && signedArea(grown) >= -signedArea(poly) {
		t.Fatalf("dilate shrank: %v -> %v", signedArea(poly), signedArea(grown))
	}
}
