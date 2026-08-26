package lottie

import (
	"bytes"
	"os"
	"reflect"
	"testing"
)

func testBundleWithClip(t *testing.T) *Bundle {
	t.Helper()
	data, err := os.ReadFile("testdata/basic.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	b := NewBundle()
	if err := b.SetAnimation("clip", data); err != nil {
		t.Fatalf("SetAnimation: %v", err)
	}
	return b
}

func TestExtensionFileAPI(t *testing.T) {
	b := NewBundle()
	if err := b.SetExtensionFile("extensions/physics/cp/body.json", []byte(`{"a":1}`)); err != nil {
		t.Fatalf("SetExtensionFile: %v", err)
	}
	if err := b.SetExtensionFile("extensions/physics/resolv/idle.json", []byte(`{"b":2}`)); err != nil {
		t.Fatalf("SetExtensionFile: %v", err)
	}
	// The bytes must not alias the caller's slice.
	src := []byte(`{"c":3}`)
	if err := b.SetExtensionFile("extensions/other/notes.json", src); err != nil {
		t.Fatalf("SetExtensionFile: %v", err)
	}
	src[2] = 'X'
	if got, _ := b.ExtensionFile("extensions/other/notes.json"); string(got) != `{"c":3}` {
		t.Fatalf("stored bytes alias the caller's: %q", got)
	}

	if err := b.SetExtensionFile("elsewhere/file.json", nil); err == nil {
		t.Fatal("a path outside extensions/ must be refused")
	}

	got := b.ExtensionFiles("extensions/physics/")
	want := []string{"extensions/physics/cp/body.json", "extensions/physics/resolv/idle.json"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExtensionFiles: got %v, want %v", got, want)
	}
	if got := b.ExtensionFiles(""); len(got) != 3 {
		t.Fatalf("all extension files: got %v", got)
	}

	b.RemoveExtensionFile("extensions/physics/cp/body.json")
	if _, ok := b.ExtensionFile("extensions/physics/cp/body.json"); ok {
		t.Fatal("removed file still present")
	}
}

func TestExtensionFilesRoundTrip(t *testing.T) {
	b := testBundleWithClip(t)
	if err := b.SetExtensionFile("extensions/physics/cp/body.json", []byte(`{"mass":2}`)); err != nil {
		t.Fatalf("SetExtensionFile: %v", err)
	}
	if err := b.SetExtensionFile("extensions/othertool/notes.json", []byte(`{"keep":"me"}`)); err != nil {
		t.Fatalf("SetExtensionFile: %v", err)
	}

	var buf bytes.Buffer
	if err := b.Encode(&buf); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	rb, err := DecodeBundle(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("DecodeBundle: %v", err)
	}
	if got, _ := rb.ExtensionFile("extensions/physics/cp/body.json"); string(got) != `{"mass":2}` {
		t.Fatalf("plugin payload lost: %q", got)
	}
	// A tool this build knows nothing about survives too.
	if got, _ := rb.ExtensionFile("extensions/othertool/notes.json"); string(got) != `{"keep":"me"}` {
		t.Fatalf("foreign payload lost: %q", got)
	}
}
