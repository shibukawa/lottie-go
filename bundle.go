package lottie

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
)

// Manifest is the manifest.json of a dotLottie archive. Members this package
// does not model are preserved so that reading a v1 archive and writing it
// back as v2 keeps its metadata.
type Manifest struct {
	Version       string                 `json:"version"`
	Generator     string                 `json:"generator,omitempty"`
	Initial       *ManifestInitial       `json:"initial,omitempty"`
	Animations    []ManifestAnimation    `json:"animations"`
	Themes        []ManifestTheme        `json:"themes,omitempty"`
	StateMachines []ManifestStateMachine `json:"stateMachines,omitempty"`

	Extra ExtraFields `json:"-"`
}

func (m Manifest) MarshalJSON() ([]byte, error) {
	type alias Manifest
	return encodeExtra(alias(m), m.Extra)
}

func (m *Manifest) UnmarshalJSON(data []byte) error {
	type alias Manifest
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := decodeExtra(data, a)
	if err != nil {
		return err
	}
	*m = Manifest(a)
	m.Extra = extra
	return nil
}

// ManifestInitial names what a player should load first. Both members are
// optional; with neither set, the first animation wins.
type ManifestInitial struct {
	Animation    string `json:"animation,omitempty"`
	StateMachine string `json:"stateMachine,omitempty"`
}

// ManifestAnimation is one entry of Manifest.Animations. ID matches a file
// in the archive's a/ directory.
type ManifestAnimation struct {
	ID           string   `json:"id"`
	InitialTheme string   `json:"initialTheme,omitempty"`
	Background   uint32   `json:"background,omitempty"`
	Themes       []string `json:"themes,omitempty"`
}

// ManifestTheme is one entry of Manifest.Themes.
type ManifestTheme struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// ManifestStateMachine is one entry of Manifest.StateMachines. ID matches a
// file in the archive's s/ directory.
type ManifestStateMachine struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// Bundle is a decoded dotLottie archive: the animations it holds plus the
// state machines, themes, images, and fonts that accompany them.
//
// Both archive layouts load: version 2 (a/ i/ s/ t/ f/) and the older
// version 1 (animations/ images/). Encode always writes version 2.
//
// Animations are decoded on first use, so opening a bundle to inspect its
// manifest does not pay for every clip in it.
type Bundle struct {
	manifest Manifest

	animJSON map[string][]byte // id -> Lottie JSON
	smJSON   map[string][]byte // id -> state machine JSON
	themes   map[string][]byte // id -> theme JSON
	images   map[string][]byte // base name -> bytes
	fonts    map[string][]byte // base name -> bytes
	files    map[string][]byte // every archive member, by cleaned path

	anims map[string]*Animation
	sms   map[string]*StateMachine
}

// NewBundle returns an empty version 2 bundle.
func NewBundle() *Bundle {
	b := newBundle()
	b.manifest.Version = "2"
	return b
}

func newBundle() *Bundle {
	return &Bundle{
		animJSON: map[string][]byte{},
		smJSON:   map[string][]byte{},
		themes:   map[string][]byte{},
		images:   map[string][]byte{},
		fonts:    map[string][]byte{},
		files:    map[string][]byte{},
		anims:    map[string]*Animation{},
		sms:      map[string]*StateMachine{},
	}
}

// DecodeBundle reads a dotLottie archive of either version.
func DecodeBundle(r io.ReaderAt, size int64) (*Bundle, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("lottie: open dotLottie: %w", err)
	}
	b := newBundle()
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := path.Clean(f.Name)
		data, err := readZipFile(f)
		if err != nil {
			return nil, fmt.Errorf("lottie: read %s: %w", f.Name, err)
		}
		b.files[name] = data

		dir, base := path.Split(name)
		switch dir {
		case "a/", "animations/":
			b.animJSON[stripJSONExt(base)] = data
		case "i/", "images/":
			b.images[base] = data
		case "s/":
			b.smJSON[stripJSONExt(base)] = data
		case "t/":
			b.themes[stripJSONExt(base)] = data
		case "f/":
			b.fonts[base] = data
		}
	}
	if len(b.animJSON) == 0 {
		return nil, fmt.Errorf("lottie: dotLottie archive holds no animations")
	}
	if raw, ok := b.files["manifest.json"]; ok {
		if err := json.Unmarshal(raw, &b.manifest); err != nil {
			return nil, fmt.Errorf("lottie: dotLottie manifest: %w", err)
		}
	}
	// A manifest may be absent, stale, or list ids the archive does not
	// hold. The files on disk are the truth; reconcile against them rather
	// than failing (see policy:robustness).
	b.syncManifest()
	return b, nil
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func stripJSONExt(name string) string {
	return strings.TrimSuffix(name, ".json")
}

// syncManifest makes the manifest describe exactly the files the bundle
// holds, keeping the metadata of entries that survive.
func (b *Bundle) syncManifest() {
	b.manifest.Animations = reconcile(b.manifest.Animations, sortedKeys(b.animJSON),
		func(e ManifestAnimation) string { return e.ID },
		func(id string) ManifestAnimation { return ManifestAnimation{ID: id} })
	b.manifest.StateMachines = reconcile(b.manifest.StateMachines, sortedKeys(b.smJSON),
		func(e ManifestStateMachine) string { return e.ID },
		func(id string) ManifestStateMachine { return ManifestStateMachine{ID: id} })
	b.manifest.Themes = reconcile(b.manifest.Themes, sortedKeys(b.themes),
		func(e ManifestTheme) string { return e.ID },
		func(id string) ManifestTheme { return ManifestTheme{ID: id} })

	if in := b.manifest.Initial; in != nil {
		if in.Animation != "" {
			if _, ok := b.animJSON[in.Animation]; !ok {
				in.Animation = ""
			}
		}
		if in.StateMachine != "" {
			if _, ok := b.smJSON[in.StateMachine]; !ok {
				in.StateMachine = ""
			}
		}
		if in.Animation == "" && in.StateMachine == "" {
			b.manifest.Initial = nil
		}
	}
}

// reconcile keeps the entries whose id still exists, in their original
// order, then appends a default entry for every id that had none.
func reconcile[T any](existing []T, ids []string, idOf func(T) string, newEntry func(string) T) []T {
	present := make(map[string]bool, len(ids))
	for _, id := range ids {
		present[id] = true
	}
	out := make([]T, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, e := range existing {
		id := idOf(e)
		if present[id] && !seen[id] {
			seen[id] = true
			out = append(out, e)
		}
	}
	for _, id := range ids {
		if !seen[id] {
			out = append(out, newEntry(id))
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Manifest returns the bundle's manifest. The returned pointer is live:
// editing it changes what Encode writes.
func (b *Bundle) Manifest() *Manifest { return &b.manifest }

// AnimationIDs returns the animation ids in manifest order.
func (b *Bundle) AnimationIDs() []string {
	out := make([]string, 0, len(b.manifest.Animations))
	for _, a := range b.manifest.Animations {
		out = append(out, a.ID)
	}
	return out
}

// StateMachineIDs returns the state machine ids in manifest order.
func (b *Bundle) StateMachineIDs() []string {
	out := make([]string, 0, len(b.manifest.StateMachines))
	for _, s := range b.manifest.StateMachines {
		out = append(out, s.ID)
	}
	return out
}

// AnimationJSON returns the raw Lottie document for an id.
func (b *Bundle) AnimationJSON(id string) ([]byte, bool) {
	data, ok := b.animJSON[id]
	return data, ok
}

// Animation decodes and returns the animation with the given id. Repeated
// calls return the same value, which is safe to share across players.
func (b *Bundle) Animation(id string) (*Animation, error) {
	if a, ok := b.anims[id]; ok {
		return a, nil
	}
	data, ok := b.animJSON[id]
	if !ok {
		return nil, fmt.Errorf("lottie: no animation %q in bundle", id)
	}
	a, err := decodeJSON(data, b.resolveAsset)
	if err != nil {
		return nil, fmt.Errorf("lottie: animation %q: %w", id, err)
	}
	b.anims[id] = a
	return a, nil
}

// StateMachine parses and returns the state machine with the given id.
func (b *Bundle) StateMachine(id string) (*StateMachine, error) {
	if s, ok := b.sms[id]; ok {
		return s, nil
	}
	data, ok := b.smJSON[id]
	if !ok {
		return nil, fmt.Errorf("lottie: no state machine %q in bundle", id)
	}
	s, err := ParseStateMachine(data)
	if err != nil {
		return nil, fmt.Errorf("lottie: state machine %q: %w", id, err)
	}
	b.sms[id] = s
	return s, nil
}

// InitialAnimation returns the animation the manifest names as initial,
// falling back to the first one listed.
func (b *Bundle) InitialAnimation() (*Animation, error) {
	if in := b.manifest.Initial; in != nil && in.Animation != "" {
		return b.Animation(in.Animation)
	}
	ids := b.AnimationIDs()
	if len(ids) == 0 {
		return nil, fmt.Errorf("lottie: bundle holds no animations")
	}
	return b.Animation(ids[0])
}

// InitialStateMachine returns the state machine the manifest names as
// initial, falling back to the first one listed. It reports false when the
// bundle has none.
func (b *Bundle) InitialStateMachine() (*StateMachine, bool, error) {
	id := ""
	if in := b.manifest.Initial; in != nil && in.StateMachine != "" {
		id = in.StateMachine
	} else if ids := b.StateMachineIDs(); len(ids) > 0 {
		id = ids[0]
	}
	if id == "" {
		return nil, false, nil
	}
	s, err := b.StateMachine(id)
	if err != nil {
		return nil, false, err
	}
	return s, true, nil
}

// resolveAsset loads an image referenced by an animation. dir and name come
// from the asset's "u" and "p" members, which point at either archive
// layout, so fall back to matching on file name alone.
func (b *Bundle) resolveAsset(dir, name string) ([]byte, error) {
	if dir != "" {
		if data, ok := b.files[path.Clean(path.Join(strings.TrimPrefix(dir, "/"), name))]; ok {
			return data, nil
		}
	}
	base := path.Base(name)
	if data, ok := b.images[base]; ok {
		return data, nil
	}
	if data, ok := b.fonts[base]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("lottie: asset %s not found in bundle", path.Join(dir, name))
}

// SetAnimation adds or replaces an animation. The document is parsed to
// reject a file that is not a usable Lottie animation before it enters the
// bundle.
func (b *Bundle) SetAnimation(id string, lottieJSON []byte) error {
	if id == "" {
		return fmt.Errorf("lottie: animation id must not be empty")
	}
	if _, err := decodeJSON(lottieJSON, b.resolveAsset); err != nil {
		return fmt.Errorf("lottie: animation %q: %w", id, err)
	}
	b.animJSON[id] = bytes.Clone(lottieJSON)
	delete(b.anims, id)
	b.syncManifest()
	return nil
}

// RemoveAnimation drops an animation and any manifest entry for it.
func (b *Bundle) RemoveAnimation(id string) {
	delete(b.animJSON, id)
	delete(b.anims, id)
	b.syncManifest()
}

// SetStateMachine adds or replaces a state machine.
func (b *Bundle) SetStateMachine(id string, sm *StateMachine) error {
	if id == "" {
		return fmt.Errorf("lottie: state machine id must not be empty")
	}
	data, err := json.Marshal(sm)
	if err != nil {
		return fmt.Errorf("lottie: state machine %q: %w", id, err)
	}
	b.smJSON[id] = data
	b.sms[id] = sm
	b.syncManifest()
	return nil
}

// RemoveStateMachine drops a state machine and any manifest entry for it.
func (b *Bundle) RemoveStateMachine(id string) {
	delete(b.smJSON, id)
	delete(b.sms, id)
	b.syncManifest()
}

// SetImage adds or replaces a shared image, stored under i/ on encode.
func (b *Bundle) SetImage(name string, data []byte) {
	b.images[path.Base(name)] = bytes.Clone(data)
}

// Validate reports problems across the whole bundle: each state machine's
// own structural problems plus references to animations or markers the
// bundle does not hold.
func (b *Bundle) Validate() []error {
	var problems []error
	for _, id := range b.StateMachineIDs() {
		sm, err := b.StateMachine(id)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		for _, p := range sm.Validate() {
			problems = append(problems, fmt.Errorf("state machine %q: %w", id, p))
		}
		for _, st := range sm.States {
			if st.Animation == "" {
				continue
			}
			if _, ok := b.animJSON[st.Animation]; !ok {
				problems = append(problems, fmt.Errorf("state machine %q: state %q names unknown animation %q", id, st.Name, st.Animation))
				continue
			}
			if st.Segment == "" {
				continue
			}
			anim, err := b.Animation(st.Animation)
			if err != nil {
				problems = append(problems, err)
				continue
			}
			if _, ok := anim.Marker(st.Segment); !ok {
				problems = append(problems, fmt.Errorf("state machine %q: state %q names unknown marker %q in animation %q", id, st.Name, st.Segment, st.Animation))
			}
		}
	}
	return problems
}

// Encode writes the bundle as a version 2 dotLottie archive.
func (b *Bundle) Encode(w io.Writer) error {
	// An archive with no animations is not loadable, so refuse to write one
	// rather than produce a file that cannot be reopened.
	if len(b.animJSON) == 0 {
		return fmt.Errorf("lottie: bundle holds no animations")
	}
	b.syncManifest()
	b.manifest.Version = "2"

	zw := zip.NewWriter(w)
	manifest, err := json.Marshal(&b.manifest)
	if err != nil {
		return fmt.Errorf("lottie: encode manifest: %w", err)
	}
	if err := writeZipFile(zw, "manifest.json", manifest); err != nil {
		return err
	}
	for _, group := range []struct {
		dir   string
		ext   string
		files map[string][]byte
	}{
		{"a/", ".json", b.animJSON},
		{"i/", "", b.images},
		{"s/", ".json", b.smJSON},
		{"t/", ".json", b.themes},
		{"f/", "", b.fonts},
	} {
		for _, name := range sortedKeys(group.files) {
			if err := writeZipFile(zw, group.dir+name+group.ext, group.files[name]); err != nil {
				return err
			}
		}
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("lottie: encode dotLottie: %w", err)
	}
	return nil
}

func writeZipFile(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("lottie: encode %s: %w", name, err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("lottie: encode %s: %w", name, err)
	}
	return nil
}
