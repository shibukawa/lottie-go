package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	lottie "github.com/shibukawa/lottie-go"
)

// The New… dialog builds on these: the embedded templates must decode as
// full bundles, and writing one out must produce a file the editor opens.

func TestTemplatesEmbedAndDecode(t *testing.T) {
	names := templateNames()
	if len(names) == 0 {
		t.Fatal("no templates embedded; run `go run ./genpresets`")
	}
	for _, name := range names {
		data, err := templateBytes(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		b, err := lottie.DecodeBundle(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatalf("%s: decode: %v", name, err)
		}
		if len(b.AnimationIDs()) == 0 || len(b.StateMachineIDs()) == 0 {
			t.Errorf("%s: template is not a full preset (%d clips, %d machines)",
				name, len(b.AnimationIDs()), len(b.StateMachineIDs()))
		}
	}
}

func TestTemplateMatchesPreset(t *testing.T) {
	for _, name := range templateNames() {
		data, err := templateBytes(name)
		if err != nil {
			t.Fatal(err)
		}
		disk, err := os.ReadFile(presetPath(name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, disk) {
			t.Errorf("%s: embedded template drifted from testdata/presets; run `go run ./genpresets`", name)
		}
	}
}

func TestStartNewAt(t *testing.T) {
	m := NewModel()
	path := filepath.Join(t.TempDir(), "fresh.lottie")
	m.StartNewAt(path)
	if m.Path() != path {
		t.Fatalf("Path() = %q; want %q", m.Path(), path)
	}
	if got := len(m.AnimationIDs()); got != 0 {
		t.Fatalf("new document holds %d clips; want none", got)
	}
}
