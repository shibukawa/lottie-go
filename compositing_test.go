package lottie

import (
	"strings"
	"testing"
)

// analyzeCompositing gates both draw-call optimizations, so a wrong flag
// either loses the speedup silently or, worse, flattens blends that must
// composite against the backdrop.
func TestAnalyzeCompositing(t *testing.T) {
	const maskProps = `"hasMask": true,
	  "masksProperties": [{ "mode": "a", "o": { "a": 0, "k": 100 },
	    "pt": { "a": 0, "k": { "c": true, "v": [[0,0],[50,0],[50,50],[0,50]],
	      "i": [[0,0],[0,0],[0,0],[0,0]], "o": [[0,0],[0,0],[0,0],[0,0]] } } }]`

	t.Run("blend mode disables snapshots", func(t *testing.T) {
		anim := decodeLayers(t, `
		  { "ty": 4, "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {}, "bm": 1, "shapes": [] }`)
		if anim.snapshotOK {
			t.Error("snapshotOK = true for an animation with a non-normal root blend")
		}
	})

	t.Run("normal blends allow snapshots", func(t *testing.T) {
		anim := decodeLayers(t, `
		  { "ty": 4, "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {}, "shapes": [] }`)
		if !anim.snapshotOK {
			t.Error("snapshotOK = false without any blend modes")
		}
	})

	t.Run("masked shape layer joins phases", func(t *testing.T) {
		anim := decodeLayers(t, `
		  { "ty": 4, "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {}, `+maskProps+`, "shapes": [] }`)
		if !anim.layers[0].phaseOK || anim.phaseNodes != 1 {
			t.Errorf("phaseOK = %v, phaseNodes = %d; want true, 1", anim.layers[0].phaseOK, anim.phaseNodes)
		}
	})

	t.Run("masked text layer stays on the fallback path", func(t *testing.T) {
		anim := decodeLayers(t, `
		  { "ty": 5, "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {}, `+maskProps+`,
		    "t": { "d": { "k": [] } } }`)
		if anim.layers[0].phaseOK {
			t.Error("phaseOK = true for a masked text layer, whose bounds are unknowable")
		}
	})

	t.Run("shared precomp layers count once", func(t *testing.T) {
		src := `{
		  "fr": 30, "ip": 0, "op": 30, "w": 200, "h": 200,
		  "assets": [{ "id": "c", "layers": [
		    { "ty": 4, "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {}, ` + maskProps + `, "shapes": [] }
		  ]}],
		  "layers": [
		    { "ty": 0, "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {}, "refId": "c", "w": 200, "h": 200 },
		    { "ty": 0, "ind": 2, "ip": 0, "op": 30, "st": 0, "ks": {}, "refId": "c", "w": 200, "h": 200 }
		  ]
		}`
		anim, err := Decode(strings.NewReader(src))
		if err != nil {
			t.Fatal(err)
		}
		if anim.phaseNodes != 1 {
			t.Errorf("phaseNodes = %d; want 1 (the shared asset's masked layer, once)", anim.phaseNodes)
		}
	})
}

func TestScratchAtlasAlloc(t *testing.T) {
	var a scratchAtlas
	a.reset()

	r1, ok := a.alloc(100, 40)
	if !ok || r1.Dx() != 100 || r1.Dy() != 40 {
		t.Fatalf("alloc(100, 40) = %v, %v", r1, ok)
	}
	r2, ok := a.alloc(100, 40)
	if !ok || r1.Overlaps(r2) {
		t.Fatalf("second region %v overlaps first %v", r2, r1)
	}
	if r2.Min.X-r1.Max.X < 2 {
		t.Errorf("regions %v and %v are not separated by a gutter", r1, r2)
	}

	// A fresh shelf must not share rows with earlier regions, or its
	// full-width clear would erase them.
	a.newShelf()
	r3, ok := a.alloc(200, 30)
	if !ok {
		t.Fatal("alloc after newShelf failed")
	}
	if r3.Min.Y < r1.Max.Y {
		t.Errorf("region %v after newShelf shares rows with %v", r3, r1)
	}

	if _, ok := a.alloc(scratchAtlasSize+1, 10); ok {
		t.Error("alloc wider than the atlas succeeded")
	}
	for {
		if _, ok := a.alloc(500, 500); !ok {
			break // must eventually report full rather than overlap
		}
	}
}

func decodeLayers(t *testing.T, layers string) *Animation {
	t.Helper()
	src := `{ "fr": 30, "ip": 0, "op": 30, "w": 200, "h": 200, "layers": [` + layers + `] }`
	anim, err := Decode(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	return anim
}
