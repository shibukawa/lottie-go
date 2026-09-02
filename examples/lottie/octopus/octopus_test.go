package main

import (
	"bytes"
	"strings"
	"testing"

	lottie "github.com/shibukawa/lottie-go"
	lottietexture "github.com/shibukawa/lottie-go/plugin/texture"
)

// The bundle must decode cleanly and its texture document must bind every
// entry: a stale address here would mean the generator and the clip
// disagree about where the arms are.
func TestBundleDecodesAndDresses(t *testing.T) {
	b, err := lottie.DecodeBundle(bytes.NewReader(bundleData), int64(len(bundleData)))
	if err != nil {
		t.Fatal(err)
	}
	anim, err := b.Animation("swim")
	if err != nil {
		t.Fatal(err)
	}
	if u := anim.UnsupportedFeatures(); len(u) != 0 {
		t.Fatalf("unsupported: %v", u)
	}
	doc, err := lottietexture.Load(b, "swim")
	if err != nil || doc == nil {
		t.Fatalf("texture document: %v %v", doc, err)
	}
	if len(doc.Paints) != 1+8+4 || len(doc.UVs) != 8 {
		t.Fatalf("paints %d uvs %d", len(doc.Paints), len(doc.UVs))
	}
	p := anim.NewPlayer()
	if err := doc.Apply(p); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := strings.Join(p.TextureNames(), ","); got != "arm,kelp,skin" {
		t.Fatalf("textures = %q", got)
	}
	for _, name := range []string{"skin.png", "arm.png", "kelp.png"} {
		if _, ok := b.Image(name); !ok {
			t.Fatalf("bundle lacks %s", name)
		}
	}
}
