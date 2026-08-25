package lottie

import (
	"image"
	"strings"
	"testing"
)

func TestBucketSize(t *testing.T) {
	cases := []struct{ in, want int }{
		{1, 16}, {16, 16}, {17, 32}, {100, 128}, {256, 256},
		{257, 384}, {400, 512}, {512, 512}, {600, 640}, {1024, 1024},
	}
	for _, c := range cases {
		if got := bucketSize(c.in); got != c.want {
			t.Errorf("bucketSize(%d) = %d; want %d", c.in, got, c.want)
		}
	}
	// A bucket must never be smaller than what was asked for, or the pool
	// would hand back an image that cannot hold the offscreen.
	prev := 0
	for n := 1; n <= 2048; n++ {
		b := bucketSize(n)
		if b < n {
			t.Fatalf("bucketSize(%d) = %d; smaller than requested", n, b)
		}
		if b < prev {
			t.Fatalf("bucketSize(%d) = %d; smaller than bucketSize(%d) = %d", n, b, n-1, prev)
		}
		prev = b
	}
}

// shapeBounds decides how large an offscreen a masked or matted layer gets, so
// anything it fails to cover is silently clipped out of the composite.
func TestShapeBoundsCoversFilledPath(t *testing.T) {
	anim := mustDecodeShape(t, `
	  { "ty": "sh", "ks": { "a": 0, "k": { "c": true,
	    "v": [[10,20],[110,20],[110,70],[10,70]],
	    "i": [[0,0],[0,0],[0,0],[0,0]], "o": [[0,0],[0,0],[0,0],[0,0]] } } },
	  { "ty": "fl", "c": { "a": 0, "k": [0,0,0] }, "o": { "a": 0, "k": 100 } }`)

	var r renderer
	got, ok := r.shapeBounds(anim.layers[0], 0, identityMatrix)
	if !ok {
		t.Fatal("shapeBounds reported no bounds for a shape layer")
	}
	path := image.Rect(10, 20, 110, 70)
	if !got.Union(path).Eq(got) {
		t.Errorf("bounds %v do not cover the path %v", got, path)
	}
	// The whole point is to be smaller than the destination, so a fill with
	// no stroke should stay within a few pixels of its path.
	if got.Dx() > path.Dx()+8 || got.Dy() > path.Dy()+8 {
		t.Errorf("bounds %v are much larger than the path %v", got, path)
	}
}

// A square cap on a diagonal reaches half the stroke width times sqrt(2) past
// the end point, which is the furthest any stroke feature gets from its path
// short of a miter join.
func TestShapeBoundsCoversSquareCaps(t *testing.T) {
	anim := mustDecodeShape(t, `
	  { "ty": "sh", "ks": { "a": 0, "k": { "c": false,
	    "v": [[50,50],[150,150]], "i": [[0,0],[0,0]], "o": [[0,0],[0,0]] } } },
	  { "ty": "st", "c": { "a": 0, "k": [0,0,0] }, "o": { "a": 0, "k": 100 },
	    "w": { "a": 0, "k": 20 }, "lc": 3, "lj": 2 }`)

	var r renderer
	got, ok := r.shapeBounds(anim.layers[0], 0, identityMatrix)
	if !ok {
		t.Fatal("shapeBounds reported no bounds for a shape layer")
	}
	// Half of 20 out along the diagonal in both axes is 14.14, so 15 is the
	// first whole pixel that must still be inside.
	caps := image.Rect(50-15, 50-15, 150+15, 150+15)
	if !got.Union(caps).Eq(got) {
		t.Errorf("bounds %v do not cover the square caps %v", got, caps)
	}
}

func TestLayerBoundsUnknownForText(t *testing.T) {
	// Text extent is only known after shaping, so the renderer has to fall
	// back to the destination bounds rather than guess.
	anim := mustDecodeShape(t, `{ "ty": "fl", "c": { "a": 0, "k": [0,0,0] }, "o": { "a": 0, "k": 100 } }`)
	l := anim.layers[0]
	l.typ = 5
	var r renderer
	if _, ok := r.layerBounds(l, 0, 0, identityMatrix); ok {
		t.Error("layerBounds claimed to know the extent of a text layer")
	}
}

func mustDecodeShape(t *testing.T, shapes string) *Animation {
	t.Helper()
	src := `{
	  "fr": 30, "ip": 0, "op": 30, "w": 400, "h": 400,
	  "layers": [{
	    "ty": 4, "ind": 1, "ip": 0, "op": 30, "st": 0, "ks": {},
	    "shapes": [` + shapes + `]
	  }]
	}`
	anim, err := Decode(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	return anim
}
