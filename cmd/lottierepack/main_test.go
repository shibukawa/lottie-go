package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	lottie "github.com/shibukawa/lottie-go"
)

// fixtureBundle writes a bundle with one clip and extension payloads in
// both shapes plugins use: a nested directory and a file at the root.
func fixtureBundle(t *testing.T, dir string) string {
	t.Helper()
	clip, err := os.ReadFile(filepath.Join("..", "..", "testdata", "basic.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	b := lottie.NewBundle()
	if err := b.SetAnimation("clip", clip); err != nil {
		t.Fatalf("SetAnimation: %v", err)
	}
	for name, data := range map[string]string{
		"extensions/physics/cp/body.json":     `{"shapes":[{"kind":"circle","r":4}]}`,
		"extensions/physics/resolv/clip.json": `{"boxes":[]}`,
		"extensions/sockets.json":             `{"sockets":{"hand":{"layer":"arm"}}}`,
		"extensions/texture/clip.bin":         "\x00\x01\x02not json",
	} {
		if err := b.SetExtensionFile(name, []byte(data)); err != nil {
			t.Fatalf("SetExtensionFile %s: %v", name, err)
		}
	}
	var buf bytes.Buffer
	if err := b.Encode(&buf); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	src := filepath.Join(dir, "src.lottie")
	if err := os.WriteFile(src, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return src
}

func decode(t *testing.T, path string) *lottie.Bundle {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := lottie.DecodeBundle(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("DecodeBundle %s: %v", path, err)
	}
	return b
}

func TestExtensionsRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	src := fixtureBundle(t, tmp)
	work := filepath.Join(tmp, "work")
	if err := dumpBundle(src, work); err != nil {
		t.Fatalf("dump: %v", err)
	}
	for _, rel := range []string{
		filepath.Join("physics", "cp", "body.json"),
		filepath.Join("physics", "resolv", "clip.json"),
		"sockets.json",
		filepath.Join("texture", "clip.bin"),
	} {
		if _, err := os.Stat(filepath.Join(work, "extensions", rel)); err != nil {
			t.Errorf("dump did not write extensions/%s: %v", rel, err)
		}
	}
	// Non-JSON bytes must come out untouched.
	if got, _ := os.ReadFile(filepath.Join(work, "extensions", "texture", "clip.bin")); string(got) != "\x00\x01\x02not json" {
		t.Errorf("binary payload altered on dump: %q", got)
	}

	out := filepath.Join(tmp, "out.lottie")
	if err := repack(work, "", out); err != nil {
		t.Fatalf("repack: %v", err)
	}
	want := decode(t, src)
	got := decode(t, out)
	if !reflect.DeepEqual(got.ExtensionFiles(""), want.ExtensionFiles("")) {
		t.Fatalf("extension members: got %v, want %v", got.ExtensionFiles(""), want.ExtensionFiles(""))
	}
	for _, name := range want.ExtensionFiles("") {
		w, _ := want.ExtensionFile(name)
		g, _ := got.ExtensionFile(name)
		// JSON is re-indented on dump; compare its meaning, not its bytes.
		if filepath.Ext(name) == ".json" {
			if !jsonEqual(w, g) {
				t.Errorf("%s: got %s, want %s", name, g, w)
			}
		} else if !bytes.Equal(w, g) {
			t.Errorf("%s: got %q, want %q", name, g, w)
		}
	}
}

func TestExtensionsDirectoryIsAuthoritative(t *testing.T) {
	tmp := t.TempDir()
	src := fixtureBundle(t, tmp)
	work := filepath.Join(tmp, "work")
	if err := dumpBundle(src, work); err != nil {
		t.Fatalf("dump: %v", err)
	}
	// Edit one member, delete another, add a new one.
	edited := filepath.Join(work, "extensions", "sockets.json")
	if err := os.WriteFile(edited, []byte(`{"sockets":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(work, "extensions", "physics", "resolv", "clip.json")); err != nil {
		t.Fatal(err)
	}
	added := filepath.Join(work, "extensions", "events", "clip.json")
	if err := os.MkdirAll(filepath.Dir(added), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(added, []byte(`{"events":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(tmp, "out.lottie")
	if err := repack(work, "", out); err != nil {
		t.Fatalf("repack: %v", err)
	}
	got := decode(t, out)
	wantNames := []string{
		"extensions/events/clip.json",
		"extensions/physics/cp/body.json",
		"extensions/sockets.json",
		"extensions/texture/clip.bin",
	}
	if names := got.ExtensionFiles(""); !reflect.DeepEqual(names, wantNames) {
		t.Fatalf("extension members: got %v, want %v", names, wantNames)
	}
	if data, _ := got.ExtensionFile("extensions/sockets.json"); !jsonEqual(data, []byte(`{"sockets":{}}`)) {
		t.Errorf("edited member not applied: %s", data)
	}
}

func TestMissingExtensionsDirKeepsBase(t *testing.T) {
	tmp := t.TempDir()
	src := fixtureBundle(t, tmp)
	work := filepath.Join(tmp, "work")
	if err := dumpBundle(src, work); err != nil {
		t.Fatalf("dump: %v", err)
	}
	// A dump from before the tool knew about extensions has no such
	// directory; repacking it must not strip the base's payloads.
	if err := os.RemoveAll(filepath.Join(work, "extensions")); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(tmp, "out.lottie")
	if err := repack(work, "", out); err != nil {
		t.Fatalf("repack: %v", err)
	}
	want := decode(t, src).ExtensionFiles("")
	if got := decode(t, out).ExtensionFiles(""); !reflect.DeepEqual(got, want) {
		t.Fatalf("extension members: got %v, want %v", got, want)
	}
}

func TestExtensionRelPathRejectsEscapes(t *testing.T) {
	for _, name := range []string{
		"extensions/../manifest.json",
		"extensions/physics/../../x.json",
		`extensions/a\b.json`,
		"extensions/",
		"other/file.json",
	} {
		if _, err := extensionRelPath(name); err == nil {
			t.Errorf("%q accepted", name)
		}
	}
	if rel, err := extensionRelPath("extensions/physics/cp/body.json"); err != nil || rel != filepath.Join("physics", "cp", "body.json") {
		t.Errorf("good name: rel=%q err=%v", rel, err)
	}
}

func jsonEqual(a, b []byte) bool {
	var va, vb any
	if err := json.Unmarshal(a, &va); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &vb); err != nil {
		return false
	}
	return reflect.DeepEqual(va, vb)
}
