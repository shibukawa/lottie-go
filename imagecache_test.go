package lottie

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"sync"
	"testing"
)

// embeddedImageAnimation carries its picture as a data URI.
func embeddedImageAnimation() string {
	uri := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte(pixelPNG))
	return strings.Replace(imageAnimation(""), `"u":"","p":"px.png"`, `"u":"","p":"`+uri+`","e":1`, 1)
}

// otherPNG encodes a picture that is not pixelPNG.
func otherPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// SetAnimation decodes to validate; Animation must hand back that value
// rather than decode the same bytes again.
func TestSetAnimationKeepsDecodedAnimation(t *testing.T) {
	b := NewBundle()
	if err := b.SetAnimation("clip", clipAnimation(4, "")); err != nil {
		t.Fatal(err)
	}
	stored := b.anims["clip"]
	if stored == nil {
		t.Fatal("SetAnimation discarded the decoded animation")
	}
	got, err := b.Animation("clip")
	if err != nil {
		t.Fatal(err)
	}
	if got != stored {
		t.Error("Animation decoded again instead of returning the stored value")
	}
}

// Two clips of one bundle naming the same picture share one image, and so
// does re-storing a clip; replacing the picture's bytes decodes afresh.
func TestBundleSharesDecodedImageAssets(t *testing.T) {
	r := buildArchive(t, map[string]string{
		"manifest.json": `{"version":"2","animations":[{"id":"one"},{"id":"two"}]}`,
		"a/one.json":    imageAnimation("i/"),
		"a/two.json":    imageAnimation("i/"),
		"i/px.png":      pixelPNG,
	})
	b, err := DecodeBundle(r, r.Size())
	if err != nil {
		t.Fatal(err)
	}
	one, err := b.Animation("one")
	if err != nil {
		t.Fatal(err)
	}
	two, err := b.Animation("two")
	if err != nil {
		t.Fatal(err)
	}
	if one.layers[0].img == nil || one.layers[0].img != two.layers[0].img {
		t.Fatal("the two clips decoded the picture separately")
	}
	if err := b.SetAnimation("one", []byte(imageAnimation("i/"))); err != nil {
		t.Fatal(err)
	}
	again, _ := b.Animation("one")
	if again == one {
		t.Fatal("SetAnimation did not replace the animation")
	}
	if again.layers[0].img != one.layers[0].img {
		t.Error("re-storing the clip decoded the picture again")
	}
	b.SetImage("px.png", otherPNG(t))
	if err := b.SetAnimation("one", []byte(imageAnimation("i/"))); err != nil {
		t.Fatal(err)
	}
	replaced, _ := b.Animation("one")
	if replaced.layers[0].img == one.layers[0].img {
		t.Error("a replaced picture was served from the cache")
	}
	if got := replaced.layers[0].img.Bounds().Dx(); got != 2 {
		t.Errorf("replaced picture is %d wide; want the new 2x2", got)
	}
}

// Embedded pictures share by their data URI within a bundle; a plain
// Decode has no cache and stands alone.
func TestEmbeddedImageAssetsShareWithinBundle(t *testing.T) {
	doc := embeddedImageAnimation()
	b := NewBundle()
	for _, id := range []string{"one", "two"} {
		if err := b.SetAnimation(id, []byte(doc)); err != nil {
			t.Fatal(err)
		}
	}
	one, _ := b.Animation("one")
	two, _ := b.Animation("two")
	if one.layers[0].img == nil {
		t.Fatalf("embedded image did not load: %v", one.UnsupportedFeatures())
	}
	if one.layers[0].img != two.layers[0].img {
		t.Error("embedded picture decoded twice within one bundle")
	}
	a1, err := Decode(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	a2, _ := Decode(strings.NewReader(doc))
	if a1.layers[0].img == a2.layers[0].img {
		t.Error("plain Decode shared an image across animations")
	}
}

// The lazy caches take readers and writers on several goroutines; run
// under -race.
func TestBundleCachesAreConcurrencySafe(t *testing.T) {
	b := NewBundle()
	for _, id := range []string{"one", "two"} {
		if err := b.SetAnimation(id, []byte(imageAnimation("i/"))); err != nil {
			t.Fatal(err)
		}
	}
	b.SetImage("px.png", []byte(pixelPNG))
	sm, err := ParseStateMachine([]byte(`{"initial":"s","states":[
	  {"name":"s","type":"PlaybackState","animation":"one","autoplay":true}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := b.SetStateMachine("m", sm); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				id := []string{"one", "two"}[(g+i)%2]
				if _, err := b.Animation(id); err != nil {
					t.Error(err)
				}
				if _, err := b.StateMachine("m"); err != nil {
					t.Error(err)
				}
				b.AnimationJSON(id)
				b.ImageNames()
				if g == 0 {
					b.SetImage("px.png", []byte(pixelPNG))
					if err := b.SetAnimation("two", []byte(imageAnimation("i/"))); err != nil {
						t.Error(err)
					}
				}
			}
		}(g)
	}
	wg.Wait()
	var buf bytes.Buffer
	if err := b.Encode(&buf); err != nil {
		t.Fatal(err)
	}
}

// Two players on different goroutines may resolve the same texture at
// once; the first-use decode must happen once and race-free.
func TestImageAssetConcurrentLookups(t *testing.T) {
	anim, err := Decode(strings.NewReader(embeddedImageAnimation()))
	if err != nil {
		t.Fatal(err)
	}
	// Forget the image the layer build decoded so the lazy path runs.
	anim.images = nil
	var wg sync.WaitGroup
	imgs := make([]any, 8)
	for g := range imgs {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			imgs[g] = anim.imageAsset("sprite")
		}(g)
	}
	wg.Wait()
	for g, img := range imgs {
		if img == nil || img != imgs[0] {
			t.Fatalf("goroutine %d saw %v; want the one image %v", g, img, imgs[0])
		}
	}
}
