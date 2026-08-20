package main

import (
	"strings"
	"testing"

	lottie "github.com/shibukawa/lottie-go"
)

// TestAllSamplesDecode ensures every bundled sample decodes and reports
// which features it needs beyond the supported subset.
func TestAllSamplesDecode(t *testing.T) {
	entries, err := assets.ReadDir("assets")
	if err != nil {
		t.Fatal(err)
	}
	// Bundled files that still need unsupported features (none since P1
	// added gradients and trim paths).
	wantUnsup := map[string]bool{}
	n := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		n++
		f, err := assets.Open("assets/" + e.Name())
		if err != nil {
			t.Fatal(err)
		}
		anim, err := lottie.Decode(f)
		f.Close()
		if err != nil {
			t.Errorf("%s: %v", e.Name(), err)
			continue
		}
		unsup := anim.UnsupportedFeatures()
		if wantUnsup[e.Name()] {
			if len(unsup) == 0 {
				t.Errorf("%s: expected unsupported features, got none", e.Name())
			}
			continue
		}
		if len(unsup) > 0 {
			t.Errorf("%s: unexpected unsupported features: %v", e.Name(), unsup)
		}
	}
	if n < 30 {
		t.Errorf("only %d samples found", n)
	}
}
