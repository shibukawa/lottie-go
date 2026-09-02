package lottie

import (
	"os"
	"testing"
)

// BenchmarkEvaluateFrame measures per-frame CPU work up to (but not
// including) GPU submission: property evaluation, geometry building, and
// command generation for every layer.
func BenchmarkEvaluateFrame(b *testing.B) {
	data, err := os.ReadFile("testdata/basic.json")
	if err != nil {
		b.Fatal(err)
	}
	anim, err := decodeJSON(data, nil, nil)
	if err != nil {
		b.Fatal(err)
	}
	var r renderer
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		f := float64(i % 120)
		for j := len(anim.layers) - 1; j >= 0; j-- {
			l := anim.layers[j]
			if l.typ != 4 || f < l.ip || f >= l.op {
				continue
			}
			r.nGeoms = 0
			r.cmds = r.cmds[:0]
			r.walkShapes(l.shapes, l.localTime(f), layerMatrix(l, f, 0), 1)
			r.path.Reset()
			for g := 0; g < r.nGeoms; g++ {
				r.geoms[g].bez.appendToPath(&r.path, r.geoms[g].mat)
			}
		}
	}
}

func BenchmarkDecode(b *testing.B) {
	data, err := os.ReadFile("testdata/basic.json")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := decodeJSON(data, nil, nil); err != nil {
			b.Fatal(err)
		}
	}
}
