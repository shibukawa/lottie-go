package lottie

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// minimalAnimation is the smallest document decodeJSON accepts: a positive
// frame rate and one layer. markers is optional and given per test.
func minimalAnimation(markers string) []byte {
	m := ""
	if markers != "" {
		m = `,"markers":` + markers
	}
	return []byte(`{"v":"5.9.0","nm":"clip","fr":60,"ip":0,"op":120,"w":100,"h":100,
		"layers":[{"ty":3,"nm":"null","ind":1,"ip":0,"op":120,"st":0,
		"ks":{"a":{"a":0,"k":[0,0]},"p":{"a":0,"k":[50,50]},
		"s":{"a":0,"k":[100,100]},"r":{"a":0,"k":0},"o":{"a":0,"k":100}}}]` + m + `}`)
}

// buildArchive writes a .lottie archive from a literal set of members.
func buildArchive(t *testing.T, files map[string]string) *bytes.Reader {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range slices.Sorted(mapKeys(files)) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(files[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(buf.Bytes())
}

func mapKeys(m map[string]string) func(func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

func archiveNames(t *testing.T, data []byte) []string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range zr.File {
		out = append(out, f.Name)
	}
	slices.Sort(out)
	return out
}

const walkJumpMachine = `{
  "initial": "idle",
  "states": [
    {"name":"idle","type":"PlaybackState","animation":"idle","loop":true,"autoplay":true,
     "transitions":[{"type":"Transition","toState":"jump","guards":[{"type":"Event","inputName":"jump"}]}]},
    {"name":"jump","type":"PlaybackState","animation":"jump","autoplay":true,
     "transitions":[{"type":"Transition","toState":"idle","guards":[{"type":"Event","inputName":"jumpDone"}]}]}
  ],
  "inputs": [{"type":"Event","name":"jump"},{"type":"Event","name":"jumpDone"}],
  "interactions": [{"type":"OnComplete","actions":[{"type":"Fire","inputName":"jumpDone"}]}]
}`

func TestDecodeBundleV2(t *testing.T) {
	r := buildArchive(t, map[string]string{
		"manifest.json": `{"version":"2","generator":"test",
			"initial":{"animation":"jump","stateMachine":"character"},
			"animations":[{"id":"idle"},{"id":"jump","background":4126537471}],
			"stateMachines":[{"id":"character","name":"Character"}]}`,
		"a/idle.json":      string(minimalAnimation("")),
		"a/jump.json":      string(minimalAnimation("")),
		"s/character.json": walkJumpMachine,
	})
	b, err := DecodeBundle(r, r.Size())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := b.AnimationIDs(), []string{"idle", "jump"}; !slices.Equal(got, want) {
		t.Errorf("AnimationIDs() = %v; want %v", got, want)
	}
	if got, want := b.StateMachineIDs(), []string{"character"}; !slices.Equal(got, want) {
		t.Errorf("StateMachineIDs() = %v; want %v", got, want)
	}
	if _, err := b.Animation("idle"); err != nil {
		t.Errorf("Animation(idle): %v", err)
	}
	// The manifest names jump as initial, not the first listed animation.
	first, err := b.InitialAnimation()
	if err != nil {
		t.Fatal(err)
	}
	if jump, _ := b.Animation("jump"); first != jump {
		t.Error("InitialAnimation() did not honor the manifest")
	}
	sm, ok, err := b.InitialStateMachine()
	if err != nil || !ok {
		t.Fatalf("InitialStateMachine() = %v, %v, %v", sm, ok, err)
	}
	if sm.Initial != "idle" || len(sm.States) != 2 {
		t.Errorf("state machine parsed as %+v", sm)
	}
	// Decoding twice must hand back the same shared value.
	a1, _ := b.Animation("idle")
	a2, _ := b.Animation("idle")
	if a1 != a2 {
		t.Error("Animation() decoded the same id twice")
	}
}

func TestDecodeBundleV1UpgradesToV2(t *testing.T) {
	r := buildArchive(t, map[string]string{
		"manifest.json": `{"version":"1","author":"someone","revision":3,
			"animations":[{"id":"spin"}]}`,
		"animations/spin.json": string(minimalAnimation("")),
	})
	b, err := DecodeBundle(r, r.Size())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := b.AnimationIDs(), []string{"spin"}; !slices.Equal(got, want) {
		t.Fatalf("AnimationIDs() = %v; want %v", got, want)
	}
	var buf bytes.Buffer
	if err := b.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	if got, want := archiveNames(t, buf.Bytes()), []string{"a/spin.json", "manifest.json"}; !slices.Equal(got, want) {
		t.Errorf("encoded members = %v; want %v", got, want)
	}
	// v1-only manifest metadata this package does not model must survive.
	round, err := DecodeBundle(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if round.Manifest().Version != "2" {
		t.Errorf("Version = %q; want 2", round.Manifest().Version)
	}
	if got := string(round.Manifest().Extra["author"]); got != `"someone"` {
		t.Errorf("author = %s; want \"someone\"", got)
	}
	if got := string(round.Manifest().Extra["revision"]); got != "3" {
		t.Errorf("revision = %s; want 3", got)
	}
}

func TestBundleEncodeRoundTrip(t *testing.T) {
	r := buildArchive(t, map[string]string{
		"manifest.json":    `{"version":"2","animations":[{"id":"idle"},{"id":"jump"}],"stateMachines":[{"id":"character"}]}`,
		"a/idle.json":      string(minimalAnimation("")),
		"a/jump.json":      string(minimalAnimation("")),
		"s/character.json": walkJumpMachine,
		"i/sprite.png":     "not-a-real-png",
	})
	b, err := DecodeBundle(r, r.Size())
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := b.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	want := []string{"a/idle.json", "a/jump.json", "i/sprite.png", "manifest.json", "s/character.json"}
	if got := archiveNames(t, buf.Bytes()); !slices.Equal(got, want) {
		t.Errorf("encoded members = %v; want %v", got, want)
	}
	round, err := DecodeBundle(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := round.AnimationIDs(), []string{"idle", "jump"}; !slices.Equal(got, want) {
		t.Errorf("AnimationIDs() = %v; want %v", got, want)
	}
	sm, err := round.StateMachine("character")
	if err != nil {
		t.Fatal(err)
	}
	if len(sm.States) != 2 || sm.States[0].Transitions[0].ToState != "jump" {
		t.Errorf("state machine did not survive the round trip: %+v", sm)
	}
}

func TestBundleMutationSyncsManifest(t *testing.T) {
	b := NewBundle()
	if err := b.SetAnimation("walk", minimalAnimation("")); err != nil {
		t.Fatal(err)
	}
	if err := b.SetAnimation("run", minimalAnimation("")); err != nil {
		t.Fatal(err)
	}
	// Manifest order is author-controlled: a newly added clip appends
	// rather than reshuffling the list an editor is showing.
	if got, want := b.AnimationIDs(), []string{"walk", "run"}; !slices.Equal(got, want) {
		t.Errorf("AnimationIDs() = %v; want %v", got, want)
	}
	sm := &StateMachine{
		Initial: "walk",
		States:  []State{{Name: "walk", Type: StatePlayback, Animation: "walk", Loop: true}},
	}
	if err := b.SetStateMachine("character", sm); err != nil {
		t.Fatal(err)
	}
	if got, want := b.StateMachineIDs(), []string{"character"}; !slices.Equal(got, want) {
		t.Errorf("StateMachineIDs() = %v; want %v", got, want)
	}
	b.RemoveAnimation("run")
	if got, want := b.AnimationIDs(), []string{"walk"}; !slices.Equal(got, want) {
		t.Errorf("after remove, AnimationIDs() = %v; want %v", got, want)
	}
	var buf bytes.Buffer
	if err := b.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	if got, want := archiveNames(t, buf.Bytes()), []string{"a/walk.json", "manifest.json", "s/character.json"}; !slices.Equal(got, want) {
		t.Errorf("encoded members = %v; want %v", got, want)
	}
}

func TestSetAnimationRejectsNonLottie(t *testing.T) {
	b := NewBundle()
	if err := b.SetAnimation("bad", []byte(`{"not":"lottie"}`)); err == nil {
		t.Error("SetAnimation accepted a document with no layers")
	}
	if len(b.AnimationIDs()) != 0 {
		t.Error("a rejected animation entered the bundle")
	}
}

func TestDecodeBundleWithoutManifest(t *testing.T) {
	// Manifest is required by the spec, but files on disk are the truth.
	r := buildArchive(t, map[string]string{
		"a/solo.json": string(minimalAnimation("")),
	})
	b, err := DecodeBundle(r, r.Size())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := b.AnimationIDs(), []string{"solo"}; !slices.Equal(got, want) {
		t.Errorf("AnimationIDs() = %v; want %v", got, want)
	}
}

func TestDecodeBundleDropsStaleManifestEntries(t *testing.T) {
	r := buildArchive(t, map[string]string{
		"manifest.json": `{"version":"2","initial":{"animation":"ghost"},
			"animations":[{"id":"ghost"},{"id":"real"}]}`,
		"a/real.json": string(minimalAnimation("")),
	})
	b, err := DecodeBundle(r, r.Size())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := b.AnimationIDs(), []string{"real"}; !slices.Equal(got, want) {
		t.Errorf("AnimationIDs() = %v; want %v", got, want)
	}
	if b.Manifest().Initial != nil {
		t.Errorf("Initial = %+v; want nil once its target vanished", b.Manifest().Initial)
	}
	// Falling back to the first listed animation must still work.
	if _, err := b.InitialAnimation(); err != nil {
		t.Errorf("InitialAnimation(): %v", err)
	}
}

func TestDecodeBundleNoAnimations(t *testing.T) {
	r := buildArchive(t, map[string]string{"manifest.json": `{"version":"2","animations":[]}`})
	if _, err := DecodeBundle(r, r.Size()); err == nil {
		t.Error("DecodeBundle accepted an archive with no animations")
	}
}

func TestBundleValidate(t *testing.T) {
	b := NewBundle()
	if err := b.SetAnimation("idle", minimalAnimation(`[{"tm":0,"cm":"loop","dr":30}]`)); err != nil {
		t.Fatal(err)
	}
	sm := &StateMachine{
		Initial: "idle",
		States: []State{
			{Name: "idle", Type: StatePlayback, Animation: "idle", Segment: "loop"},
			{Name: "gone", Type: StatePlayback, Animation: "missing"},
			{Name: "badseg", Type: StatePlayback, Animation: "idle", Segment: "nope"},
		},
	}
	if err := b.SetStateMachine("m", sm); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range b.Validate() {
		got = append(got, p.Error())
	}
	joined := strings.Join(got, "\n")
	for _, want := range []string{
		`names unknown animation "missing"`,
		`names unknown marker "nope"`,
		`state "gone" is unreachable`,
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("Validate() missing %q; got:\n%s", want, joined)
		}
	}
}

func TestManifestEncodesAsV2(t *testing.T) {
	b := NewBundle()
	if err := b.SetAnimation("a", minimalAnimation("")); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := b.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	f, err := zr.Open("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.NewDecoder(f).Decode(&m); err != nil {
		t.Fatal(err)
	}
	if m["version"] != "2" {
		t.Errorf("version = %v; want \"2\"", m["version"])
	}
	if _, ok := m["animations"]; !ok {
		t.Error("manifest has no animations member")
	}
}

// pixelPNG is a 1x1 opaque PNG, small enough to embed in a test archive.
const pixelPNG = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00\x90wS\xde\x00\x00\x00\x0cIDATx\x9cc\xf8\xcf\xc0\x00\x00\x03\x01\x01\x00\xc9\xfe\x92\xef\x00\x00\x00\x00IEND\xaeB`\x82"

// imageAnimation references an external image the way an editor exports it:
// a directory in "u" and a file name in "p".
func imageAnimation(dir string) string {
	return `{"v":"5.9.0","nm":"img","fr":60,"ip":0,"op":60,"w":100,"h":100,
		"assets":[{"id":"sprite","w":1,"h":1,"u":"` + dir + `","p":"px.png"}],
		"layers":[{"ty":2,"refId":"sprite","nm":"image","ind":1,"ip":0,"op":60,"st":0,
		"ks":{"a":{"a":0,"k":[0,0]},"p":{"a":0,"k":[50,50]},
		"s":{"a":0,"k":[100,100]},"r":{"a":0,"k":0},"o":{"a":0,"k":100}}}]}`
}

// An animation's asset path points at whichever layout its archive used, so
// images must resolve from both i/ and images/ regardless of what "u" says.
func TestBundleResolvesImagesAcrossLayouts(t *testing.T) {
	for _, tt := range []struct {
		name    string
		imgPath string
		assetU  string
	}{
		{"v2 layout", "i/px.png", "i/"},
		{"v1 layout", "images/px.png", "images/"},
		{"v2 files with v1 asset path", "i/px.png", "images/"},
		{"asset path absent", "i/px.png", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := buildArchive(t, map[string]string{
				"manifest.json": `{"version":"2","animations":[{"id":"img"}]}`,
				"a/img.json":    imageAnimation(tt.assetU),
				tt.imgPath:      pixelPNG,
			})
			b, err := DecodeBundle(r, r.Size())
			if err != nil {
				t.Fatal(err)
			}
			anim, err := b.Animation("img")
			if err != nil {
				t.Fatal(err)
			}
			if got := anim.UnsupportedFeatures(); len(got) != 0 {
				t.Errorf("image did not load; UnsupportedFeatures() = %v", got)
			}
		})
	}
}

func TestBundleMissingImageDoesNotFailDecode(t *testing.T) {
	// A missing asset is skipped, not fatal (policy:robustness).
	r := buildArchive(t, map[string]string{
		"manifest.json": `{"version":"2","animations":[{"id":"img"}]}`,
		"a/img.json":    imageAnimation("i/"),
	})
	b, err := DecodeBundle(r, r.Size())
	if err != nil {
		t.Fatal(err)
	}
	anim, err := b.Animation("img")
	if err != nil {
		t.Errorf("Animation() failed on a missing image: %v", err)
	}
	// The skip is reported rather than silent, which is also what makes the
	// empty UnsupportedFeatures() in the resolving tests above meaningful.
	if got := anim.UnsupportedFeatures(); len(got) != 1 || !strings.Contains(got[0], "unresolvable image asset") {
		t.Errorf("UnsupportedFeatures() = %v; want an unresolvable image note", got)
	}
}

func TestBundleSetImageIsWrittenUnderI(t *testing.T) {
	b := NewBundle()
	if err := b.SetAnimation("img", []byte(imageAnimation("i/"))); err != nil {
		t.Fatal(err)
	}
	b.SetImage("art/px.png", []byte(pixelPNG))
	var buf bytes.Buffer
	if err := b.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	if got, want := archiveNames(t, buf.Bytes()), []string{"a/img.json", "i/px.png", "manifest.json"}; !slices.Equal(got, want) {
		t.Errorf("encoded members = %v; want %v", got, want)
	}
	round, err := DecodeBundle(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	anim, err := round.Animation("img")
	if err != nil {
		t.Fatal(err)
	}
	if got := anim.UnsupportedFeatures(); len(got) != 0 {
		t.Errorf("image did not survive the round trip: %v", got)
	}
}

func TestBundleAnimationJSONAndRemoveStateMachine(t *testing.T) {
	b := NewBundle()
	src := minimalAnimation("")
	if err := b.SetAnimation("clip", src); err != nil {
		t.Fatal(err)
	}
	got, ok := b.AnimationJSON("clip")
	if !ok || !bytes.Equal(got, src) {
		t.Errorf("AnimationJSON() = %s, %v", got, ok)
	}
	if _, ok := b.AnimationJSON("nope"); ok {
		t.Error("AnimationJSON reported a missing id as present")
	}
	if err := b.SetStateMachine("m", &StateMachine{Initial: "x"}); err != nil {
		t.Fatal(err)
	}
	b.RemoveStateMachine("m")
	if len(b.StateMachineIDs()) != 0 {
		t.Errorf("StateMachineIDs() = %v; want none", b.StateMachineIDs())
	}
	if _, err := b.StateMachine("m"); err == nil {
		t.Error("StateMachine() returned a removed machine")
	}
}

func TestEncodeEmptyBundleFails(t *testing.T) {
	var buf bytes.Buffer
	if err := NewBundle().Encode(&buf); err == nil {
		t.Error("Encode wrote an archive with no animations")
	}
	if buf.Len() != 0 {
		t.Errorf("Encode wrote %d bytes before failing", buf.Len())
	}
}
