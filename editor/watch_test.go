package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	lottie "github.com/shibukawa/lottie-go"
)

// force runs one poll regardless of the interval gate.
func force(w *watcher) {
	w.next = time.Time{}
	w.tick()
}

func TestViewerAutoReload(t *testing.T) {
	data, err := os.ReadFile(presetPath("chibi-male"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "chibi.lottie")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewModel()
	m.SetViewer(true)
	m.Open(path)
	if m.Machine() == nil {
		t.Fatalf("open failed: %s", m.Status())
	}
	before := len(m.AnimationIDs())
	w := newWatcher(m)

	// An outside tool rewrites the bundle with one clip removed.
	b, err := lottie.DecodeBundle(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	b.RemoveAnimation("kick-anim")
	var buf bytes.Buffer
	if err := b.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	// First poll sees the change and waits for it to settle; the second
	// finds it stable and reloads.
	force(w)
	if got := len(m.AnimationIDs()); got != before {
		t.Fatalf("reloaded before the change settled (%d clips)", got)
	}
	force(w)
	if got := len(m.AnimationIDs()); got != before-1 {
		t.Fatalf("after reload got %d clips; want %d", got, before-1)
	}
	if m.Preview() == nil {
		t.Fatalf("preview did not restart: %v", m.PreviewErr())
	}

	// A quiet disk stays quiet: no further generation churn.
	gen := m.Generation()
	force(w)
	force(w)
	if m.Generation() != gen {
		t.Fatalf("reloaded without any file change")
	}
}
