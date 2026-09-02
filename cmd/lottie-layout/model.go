package main

import (
	"bytes"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	lottie "github.com/shibukawa/lottie-go"
)

// inspectTarget names what the inspector pane edits.
type inspectTarget int

const (
	inspectNode inspectTarget = iota
	inspectScene
)

// sourceKind discriminates the palette's placeable things.
type sourceKind int

const (
	// sourceBundle places one node showing the whole bundle: one player,
	// one layer. What it plays — a machine or a clip — is chosen on the
	// node afterwards.
	sourceBundle sourceKind = iota
	sourceImage
)

// sourceRef is one placeable thing: a referenced bundle or image.
type sourceRef struct {
	Kind  sourceKind
	Alias string
}

// Model is the layout editor's document and selection. It knows nothing
// about widgets, so the same operations back the palette, the canvas, and
// the inspector.
type Model struct {
	scene *lottie.Scene
	path  string

	// Loaded bundles by alias; load failures keep their message so the
	// palette can show a broken reference instead of dropping it.
	bundles    map[string]*lottie.Bundle
	bundleErrs map[string]string

	selNode    int
	selBinding int
	selSource  int
	selStep    int
	inspect    inspectTarget

	// preview switches the canvas from arranging nodes to driving the
	// scene with real input.
	preview bool

	// playing runs the edit-mode transport. It starts stopped — a scene
	// should not play itself the moment it opens — and pauses on its own
	// when the choreography's last element finishes (contentEnd).
	// Preview mode always runs.
	playing bool

	// viewPhase is the phase the canvas and timeline show; empty means
	// the scene's initial phase.
	viewPhase string
	selPhase  int

	// Editor-side caches: decoded images for the palette preview and
	// placement sizing, raw file bytes for the player's loader, and load
	// failures per asset alias.
	imgCache  map[string]*ebiten.Image
	rawCache  map[string][]byte
	assetErrs map[string]string

	// player is what the canvas draws in both modes; it lazily rebuilds
	// when docGen moves. Transform drags patch the live node instead of
	// bumping docGen, so dragging never restarts playback. playerGen is
	// the docGen the last build attempt — successful or not — was for; a
	// failed attempt stays failed until the document changes, so a broken
	// scene does not re-decode every asset on every frame.
	player    *lottie.ScenePlayer
	playerErr error
	playerGen int

	// seekTarget and seekTime remember the last scrub: the time asked for
	// and the clock it landed on. A repeat of the same target on a player
	// nothing has advanced since is a no-op, so a held ruler drag does not
	// replay the scene every tick.
	seekTarget float64
	seekTime   float64
	seekValid  bool

	generation int
	docGen     int
	status     string

	problemsCache []string
	problemsGen   int

	dialog     chan dialogResult
	dialogOpen bool
}

func NewModel() *Model {
	m := &Model{
		scene:      &lottie.Scene{Size: lottie.SceneSize{W: 1280, H: 720}},
		bundles:    map[string]*lottie.Bundle{},
		bundleErrs: map[string]string{},
		imgCache:   map[string]*ebiten.Image{},
		rawCache:   map[string][]byte{},
		assetErrs:  map[string]string{},
		selNode:    -1,
		selBinding: -1,
		selSource:  -1,
		selStep:    -1,
		selPhase:   -1,
		inspect:    inspectScene,
		playerGen:  -1, // no build attempted yet
		dialog:     make(chan dialogResult, 1),
	}
	m.status = "New scene. Add a bundle to begin."
	return m
}

func (m *Model) Generation() int      { return m.generation }
func (m *Model) Status() string       { return m.status }
func (m *Model) Path() string         { return m.path }
func (m *Model) Scene() *lottie.Scene { return m.scene }

func (m *Model) setStatus(format string, args ...any) {
	m.status = fmt.Sprintf(format, args...)
}

// touch records a document edit that changes what plays: the player is
// stale and rebuilds on the next read.
func (m *Model) touch() {
	m.docGen++
	m.generation++
}

// touchLight records an edit the live player was already patched for —
// a transform drag — so playback continues uninterrupted.
func (m *Model) touchLight() { m.generation++ }

// round2 keeps dragged and default geometry to two decimals, matching the
// state machine editor's convention.
func round2(v float64) float64 { return math.Round(v*100) / 100 }

// ---- files ----

// sceneDir is what bundle paths are relative to: the scene file's
// directory, or the working directory while the scene is unsaved.
func (m *Model) sceneDir() string {
	if m.path == "" {
		return "."
	}
	return filepath.Dir(m.path)
}

// Open loads a scene file and every bundle it references.
func (m *Model) Open(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		m.setStatus("no path given")
		m.generation++
		return
	}
	f, err := os.Open(path)
	if err != nil {
		m.setStatus("open failed: %v", err)
		m.generation++
		return
	}
	defer f.Close()
	s, err := lottie.DecodeScene(f)
	if err != nil {
		m.setStatus("load failed: %v", err)
		m.generation++
		return
	}
	m.scene = s
	m.path = path
	m.selNode, m.selBinding, m.selSource, m.selPhase = -1, -1, -1, -1
	m.inspect = inspectScene
	m.preview = false
	m.playing = false
	m.viewPhase = ""
	m.reloadBundles()
	m.touch()
	m.setStatus("loaded %s (%d bundles, %d nodes)", path, len(s.Bundles), len(s.Nodes))
}

// Save writes the scene as JSON.
func (m *Model) Save(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = m.path
	}
	if path == "" {
		m.setStatus("no save path given")
		m.generation++
		return
	}
	// Encode first: a document that cannot be serialized (an Inf that
	// slipped into a field, say) must not truncate the file already on
	// disk. Then write beside the target and rename over it, so a crash
	// mid-write leaves the previous save intact too.
	var buf bytes.Buffer
	if err := m.scene.Encode(&buf); err != nil {
		m.setStatus("save failed: %v", err)
		m.generation++
		return
	}
	if err := writeFileAtomic(path, buf.Bytes()); err != nil {
		m.setStatus("write failed: %v", err)
		m.generation++
		return
	}
	oldDir := m.sceneDir()
	m.path = path
	// Saving into another directory moves what relative paths mean; keep
	// them pointing at the same files.
	if newDir := m.sceneDir(); newDir != oldDir {
		m.rebasePaths(oldDir)
		m.reloadBundles()
	}
	m.setStatus("saved %s", path)
	m.generation++
}

// writeFileAtomic writes data to a temporary file in path's directory and
// renames it into place, so the target is either the old or the new
// content, never a truncated mix.
func writeFileAtomic(path string, data []byte) error {
	dir, base := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	tmp, err := os.CreateTemp(dir, "."+base+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	_, werr := tmp.Write(data)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Rename(tmpPath, path)
	}
	if werr != nil {
		os.Remove(tmpPath)
		return werr
	}
	return nil
}

// rebasePaths rewrites relative bundle, image, and font paths after the
// scene file moved, so every reference keeps pointing at the same file.
func (m *Model) rebasePaths(oldDir string) {
	rebase := func(p *string) {
		if filepath.IsAbs(*p) {
			return
		}
		abs := filepath.Join(oldDir, *p)
		if rel, err := filepath.Rel(m.sceneDir(), abs); err == nil {
			*p = filepath.ToSlash(rel)
		} else {
			*p = abs
		}
	}
	for i := range m.scene.Bundles {
		rebase(&m.scene.Bundles[i].Path)
	}
	for i := range m.scene.Images {
		rebase(&m.scene.Images[i].Path)
	}
	for i := range m.scene.Fonts {
		rebase(&m.scene.Fonts[i].Path)
	}
}

// ---- bundles ----

// AddBundle references a bundle file, storing the path relative to the
// scene when possible so the pair travels together.
func (m *Model) AddBundle(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		m.setStatus("no bundle path given")
		m.generation++
		return
	}
	stored := path
	if rel, err := filepath.Rel(m.sceneDir(), path); err == nil && !strings.HasPrefix(rel, "..") {
		stored = filepath.ToSlash(rel)
	}
	var taken []string
	for _, b := range m.scene.Bundles {
		if b.Path == stored {
			m.setStatus("bundle already referenced as %q", b.Alias)
			m.generation++
			return
		}
		taken = append(taken, b.Alias)
	}
	alias := uniqueID(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), taken)
	m.scene.Bundles = append(m.scene.Bundles, lottie.SceneBundle{Alias: alias, Path: stored})
	m.loadBundle(alias)
	m.setStatus("added bundle %q", alias)
	m.touch()
}

// RemoveBundle drops a reference. Nodes using it stay and validate as
// broken, so removing the wrong bundle is repairable.
func (m *Model) RemoveBundle(alias string) {
	for i, b := range m.scene.Bundles {
		if b.Alias == alias {
			m.scene.Bundles = append(m.scene.Bundles[:i], m.scene.Bundles[i+1:]...)
			break
		}
	}
	delete(m.bundles, alias)
	delete(m.bundleErrs, alias)
	m.setStatus("removed bundle %q", alias)
	m.touch()
}

// reloadBundles loads every referenced bundle and asset from scratch.
func (m *Model) reloadBundles() {
	m.bundles = map[string]*lottie.Bundle{}
	m.bundleErrs = map[string]string{}
	m.imgCache = map[string]*ebiten.Image{}
	m.rawCache = map[string][]byte{}
	m.assetErrs = map[string]string{}
	for _, b := range m.scene.Bundles {
		m.loadBundle(b.Alias)
	}
	for _, a := range m.scene.Images {
		m.loadImage(a.Alias)
	}
	for _, a := range m.scene.Fonts {
		m.loadFontCheck(a.Alias)
	}
}

// readAssetFile reads a referenced file relative to the scene, caching
// the bytes so player rebuilds do not reread the disk.
func (m *Model) readAssetFile(path string) ([]byte, error) {
	if data, ok := m.rawCache[path]; ok {
		return data, nil
	}
	p := path
	if !filepath.IsAbs(p) {
		p = filepath.Join(m.sceneDir(), filepath.FromSlash(p))
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	m.rawCache[path] = data
	return data, nil
}

// loadImage decodes a referenced image for the palette preview and for
// sizing placements; the runtime decodes its own copy through the loader.
func (m *Model) loadImage(alias string) {
	delete(m.imgCache, alias)
	delete(m.assetErrs, alias)
	var ref *lottie.SceneAsset
	for i := range m.scene.Images {
		if m.scene.Images[i].Alias == alias {
			ref = &m.scene.Images[i]
		}
	}
	if ref == nil {
		return
	}
	data, err := m.readAssetFile(ref.Path)
	if err != nil {
		m.assetErrs[alias] = err.Error()
		return
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		m.assetErrs[alias] = err.Error()
		return
	}
	m.imgCache[alias] = ebiten.NewImageFromImage(img)
}

// loadFontCheck verifies a font file reads, so a broken reference shows
// in Problems before a text node needs it.
func (m *Model) loadFontCheck(alias string) {
	delete(m.assetErrs, alias)
	for _, a := range m.scene.Fonts {
		if a.Alias == alias {
			if _, err := m.readAssetFile(a.Path); err != nil {
				m.assetErrs[alias] = err.Error()
			}
			return
		}
	}
}

func (m *Model) loadBundle(alias string) {
	ref, ok := m.scene.Bundle(alias)
	if !ok {
		return
	}
	delete(m.bundles, alias)
	delete(m.bundleErrs, alias)
	b, err := m.readBundle(ref.Path)
	if err != nil {
		m.bundleErrs[alias] = err.Error()
		return
	}
	m.bundles[alias] = b
}

func (m *Model) readBundle(path string) (*lottie.Bundle, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.sceneDir(), filepath.FromSlash(path))
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	return lottie.DecodeBundle(f, st.Size())
}

// storedPath makes a picked file's path scene-relative when possible.
func (m *Model) storedPath(path string) string {
	if rel, err := filepath.Rel(m.sceneDir(), path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return path
}

// assetAlias derives a unique alias from a file name.
func assetAlias(path string, taken []string) string {
	return uniqueID(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), taken)
}

// AddImage references an image file for image nodes.
func (m *Model) AddImage(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	var taken []string
	for _, a := range m.scene.Images {
		taken = append(taken, a.Alias)
	}
	alias := assetAlias(path, taken)
	m.scene.Images = append(m.scene.Images, lottie.SceneAsset{Alias: alias, Path: m.storedPath(path)})
	m.loadImage(alias)
	m.setStatus("added image %q", alias)
	m.touch()
}

// AddFont references a font file for text nodes.
func (m *Model) AddFont(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	var taken []string
	for _, a := range m.scene.Fonts {
		taken = append(taken, a.Alias)
	}
	alias := assetAlias(path, taken)
	m.scene.Fonts = append(m.scene.Fonts, lottie.SceneAsset{Alias: alias, Path: m.storedPath(path)})
	m.loadFontCheck(alias)
	m.setStatus("added font %q", alias)
	m.touch()
}

// Image returns the editor-side decoded image for an alias.
func (m *Model) Image(alias string) (*ebiten.Image, bool) {
	img, ok := m.imgCache[alias]
	return img, ok
}

// FontAliases lists the referenced fonts, for the text node's picker.
func (m *Model) FontAliases() []string {
	out := make([]string, 0, len(m.scene.Fonts))
	for _, a := range m.scene.Fonts {
		out = append(out, a.Alias)
	}
	return out
}

// Bundle returns the loaded bundle for an alias.
func (m *Model) Bundle(alias string) (*lottie.Bundle, bool) {
	b, ok := m.bundles[alias]
	return b, ok
}

// Sources lists everything placeable: one row per referenced bundle —
// a bundle is one actor, not a bag of parts — then every image.
func (m *Model) Sources() []sourceRef {
	var out []sourceRef
	for _, ref := range m.scene.Bundles {
		if _, ok := m.bundles[ref.Alias]; ok {
			out = append(out, sourceRef{Kind: sourceBundle, Alias: ref.Alias})
		}
	}
	for _, a := range m.scene.Images {
		out = append(out, sourceRef{Kind: sourceImage, Alias: a.Alias})
	}
	return out
}

// initialContent picks what a freshly placed bundle node shows: the
// manifest's initial machine, else the first machine, else the first
// animation.
func initialContent(b *lottie.Bundle) (kind lottie.SceneNodeKind, id string, ok bool) {
	if in := b.Manifest().Initial; in != nil && in.StateMachine != "" {
		return lottie.SceneNodeMachine, in.StateMachine, true
	}
	if ids := b.StateMachineIDs(); len(ids) > 0 {
		return lottie.SceneNodeMachine, ids[0], true
	}
	if ids := b.AnimationIDs(); len(ids) > 0 {
		return lottie.SceneNodeAnimation, ids[0], true
	}
	return "", "", false
}

func (m *Model) SelectSource(i int)  { m.selSource = i; m.generation++ }
func (m *Model) SelectedSource() int { return m.selSource }

// ---- nodes ----

func (m *Model) SelectedNode() *lottie.SceneNode {
	if m.selNode < 0 || m.selNode >= len(m.scene.Nodes) {
		return nil
	}
	return &m.scene.Nodes[m.selNode]
}

func (m *Model) SelectedNodeIndex() int { return m.selNode }

func (m *Model) SelectNode(i int) {
	if i >= len(m.scene.Nodes) {
		i = -1
	}
	m.selNode = i
	m.selBinding = -1
	m.selStep = -1
	if i >= 0 {
		m.inspect = inspectNode
	}
	m.generation++
}

// SelectNodeByName selects the node the canvas hit.
func (m *Model) SelectNodeByName(name string) {
	for i := range m.scene.Nodes {
		if m.scene.Nodes[i].Name == name {
			m.SelectNode(i)
			return
		}
	}
	m.SelectNode(-1)
}

// InspectTarget reports what the inspector should edit.
func (m *Model) InspectTarget() inspectTarget { return m.inspect }

// ShowScenePane opens the scene settings in the inspector.
func (m *Model) ShowScenePane() {
	m.inspect = inspectScene
	m.generation++
}

// PlaceSource adds one instance of a source to the scene — one node per
// bundle: one player, one layer — centered on the design box, joining
// the phase currently viewed, and selects it. What a bundle node plays
// (machine, clip, segment, chain) is chosen on the node afterwards.
func (m *Model) PlaceSource(ref sourceRef) {
	w, h := 0, 0
	var kind lottie.SceneNodeKind
	var src lottie.SceneSource
	switch ref.Kind {
	case sourceBundle:
		b, ok := m.bundles[ref.Alias]
		if !ok {
			m.setStatus("bundle %q is not loaded", ref.Alias)
			m.generation++
			return
		}
		var id string
		kindK, id, okc := initialContent(b)
		if !okc {
			m.setStatus("bundle %q holds nothing playable", ref.Alias)
			m.generation++
			return
		}
		kind = kindK
		src = lottie.SceneSource{Bundle: ref.Alias, ID: id}
		if kind == lottie.SceneNodeAnimation {
			if anim, err := b.Animation(id); err == nil {
				w, h = anim.Size()
			}
		} else if p, err := b.NewStateMachinePlayer(id); err == nil && p.Player() != nil {
			w, h = p.Player().Animation().Size()
		}
	case sourceImage:
		kind = lottie.SceneNodeImage
		src = lottie.SceneSource{Image: ref.Alias}
		if img, ok := m.imgCache[ref.Alias]; ok {
			b := img.Bounds()
			w, h = b.Dx(), b.Dy()
		}
	}
	n := lottie.SceneNode{
		Name:   uniqueID(ref.Alias, m.NodeNames()),
		Kind:   kind,
		Source: src,
		Phase:  m.viewPhase,
		Transform: lottie.SceneTransform{
			X: round2(float64(m.scene.Size.W-w) / 2),
			Y: round2(float64(m.scene.Size.H-h) / 2),
		},
	}
	if kind == lottie.SceneNodeAnimation {
		n.Playback = lottie.ScenePlayback{Loop: true, Autoplay: true}
	}
	m.scene.Nodes = append(m.scene.Nodes, n)
	m.setStatus("placed %q", n.Name)
	m.touch()
	m.SelectNode(len(m.scene.Nodes) - 1)
}

// SetNodeContent switches what a bundle node plays: another machine or
// another clip of the same bundle. Content-specific settings reset.
func (m *Model) SetNodeContent(i int, kind lottie.SceneNodeKind, id string) {
	if i < 0 || i >= len(m.scene.Nodes) {
		return
	}
	n := &m.scene.Nodes[i]
	if n.Kind == kind && n.Source.ID == id {
		return
	}
	n.Kind = kind
	n.Source.ID = id
	n.Entry = ""
	n.Playback.Segment = ""
	n.Playback.Then = nil
	if kind == lottie.SceneNodeAnimation && !n.Playback.Autoplay && !n.Playback.Loop {
		n.Playback = lottie.ScenePlayback{Loop: true, Autoplay: true}
	}
	m.selStep = -1
	m.touch()
}

// AddTextNode places a text node. Text needs a referenced font first.
func (m *Model) AddTextNode() {
	fonts := m.FontAliases()
	if len(fonts) == 0 {
		m.setStatus("add a font first (Add Font…)")
		m.generation++
		return
	}
	n := lottie.SceneNode{
		Name:  uniqueID("text", m.NodeNames()),
		Kind:  lottie.SceneNodeText,
		Phase: m.viewPhase,
		Transform: lottie.SceneTransform{
			X: round2(float64(m.scene.Size.W) / 2),
			Y: round2(float64(m.scene.Size.H) / 2),
		},
		Text: lottie.SceneText{
			Value: "Text", Font: fonts[0], Size: 24,
			AnchorX: lottie.AlignCenter, AnchorY: lottie.AlignMiddle,
		},
	}
	m.scene.Nodes = append(m.scene.Nodes, n)
	m.setStatus("placed %q", n.Name)
	m.touch()
	m.SelectNode(len(m.scene.Nodes) - 1)
}

func (m *Model) DeleteNode(i int) {
	if i < 0 || i >= len(m.scene.Nodes) {
		return
	}
	name := m.scene.Nodes[i].Name
	m.scene.Nodes = append(m.scene.Nodes[:i], m.scene.Nodes[i+1:]...)
	m.clearNodeRefs(name)
	m.selNode = -1
	m.setStatus("deleted %q", name)
	m.touch()
}

// clearNodeRefs drops focus links pointing at a node that no longer
// exists, so deleting never leaves dangling navigation.
func (m *Model) clearNodeRefs(name string) {
	clearRef := func(s *string) {
		if *s == name {
			*s = ""
		}
	}
	for i := range m.scene.Nodes {
		nb := &m.scene.Nodes[i].Focus.Neighbors
		clearRef(&nb.Up)
		clearRef(&nb.Down)
		clearRef(&nb.Left)
		clearRef(&nb.Right)
	}
	clearRef(&m.scene.Options.InitialFocus)
}

// MoveNode reorders a node in draw order: later is nearer the front, so
// this is the overlap edit.
func (m *Model) MoveNode(i, delta int) {
	j := i + delta
	if i < 0 || i >= len(m.scene.Nodes) || j < 0 || j >= len(m.scene.Nodes) {
		return
	}
	m.scene.Nodes[i], m.scene.Nodes[j] = m.scene.Nodes[j], m.scene.Nodes[i]
	m.selNode = j
	m.touch()
}

// RenameNode renames a node and repoints every focus link at it. The name
// is the game-facing id, so uniqueness is enforced here.
func (m *Model) RenameNode(i int, name string) {
	name = strings.TrimSpace(name)
	if i < 0 || i >= len(m.scene.Nodes) || name == "" {
		return
	}
	old := m.scene.Nodes[i].Name
	if old == name {
		return
	}
	for _, n := range m.scene.Nodes {
		if n.Name == name {
			m.setStatus("a node named %q already exists", name)
			m.generation++
			return
		}
	}
	m.scene.Nodes[i].Name = name
	rename := func(s *string) {
		if *s == old {
			*s = name
		}
	}
	for j := range m.scene.Nodes {
		nb := &m.scene.Nodes[j].Focus.Neighbors
		rename(&nb.Up)
		rename(&nb.Down)
		rename(&nb.Left)
		rename(&nb.Right)
	}
	rename(&m.scene.Options.InitialFocus)
	m.touch()
}

// DragNode moves a node by a canvas delta, patching the live player node
// so playback never restarts mid-drag.
func (m *Model) DragNode(i int, dx, dy float64) {
	if i < 0 || i >= len(m.scene.Nodes) {
		return
	}
	n := &m.scene.Nodes[i]
	n.Transform.X = round2(n.Transform.X + dx)
	n.Transform.Y = round2(n.Transform.Y + dy)
	m.patchLiveTransform(n)
	m.touchLight()
}

// SetNodeTransform writes one node's transform from the inspector.
func (m *Model) SetNodeTransform(i int, tf lottie.SceneTransform) {
	if i < 0 || i >= len(m.scene.Nodes) {
		return
	}
	n := &m.scene.Nodes[i]
	n.Transform = tf
	m.patchLiveTransform(n)
	m.touchLight()
}

// SetNodeDepth writes a node's parallax depth. 1 is the default and stays
// out of the document; the live player reads the shared definition, so no
// rebuild is needed.
func (m *Model) SetNodeDepth(i int, d float64) {
	if i < 0 || i >= len(m.scene.Nodes) {
		return
	}
	if d == 1 {
		m.scene.Nodes[i].Depth = nil
	} else {
		m.scene.Nodes[i].Depth = &d
	}
	m.touchLight()
}

// SetSceneCamera writes the scene's camera. A zoom of exactly 1 is the
// default and stays out of the document.
func (m *Model) SetSceneCamera(c lottie.SceneCamera) {
	if c.Zoom == 1 {
		c.Zoom = 0
	}
	m.scene.Camera = c
	m.touch()
}

// SetPhaseCamera sets or clears a phase's camera override.
func (m *Model) SetPhaseCamera(i int, c *lottie.SceneCamera) {
	if i < 0 || i >= len(m.scene.Phases) {
		return
	}
	if c != nil && c.Zoom == 1 {
		c.Zoom = 0
	}
	m.scene.Phases[i].Camera = c
	m.touch()
}

// applyEditCamera neutralizes the camera on the running player while
// arranging: edit mode lays nodes out in plain scene coordinates, and the
// canvas shows the camera as a framing overlay instead. Preview mode keeps
// the document's camera, the way a game would see it.
func (m *Model) applyEditCamera(sp *lottie.ScenePlayer) {
	if sp != nil && !m.preview {
		sp.SetCamera(lottie.SceneCamera{})
	}
}

func (m *Model) patchLiveTransform(n *lottie.SceneNode) {
	if m.player == nil {
		return
	}
	if live, ok := m.player.Node(n.Name); ok {
		live.SetTransform(n.Transform)
	}
}

// SetNodeStart drags a node's entrance time on the timeline. Like a
// position drag it only patches lightly; CommitNodeStart on release
// rebuilds the player so the gate takes effect.
func (m *Model) SetNodeStart(i int, sec float64) {
	if i < 0 || i >= len(m.scene.Nodes) {
		return
	}
	// The upper clamp keeps a typo in the start field from stretching the
	// timeline (and its per-frame tick loop) to astronomical spans.
	m.scene.Nodes[i].Start = min(max(0, round2(sec)), maxSceneSeconds)
	m.touchLight()
}

// maxSceneSeconds bounds entrance times and scrub targets: an hour is far
// beyond any cutscene, and small enough that replay-based seeking and the
// timeline's second ticks stay affordable.
const maxSceneSeconds = 3600

// CommitNodeStart applies a finished timeline drag.
func (m *Model) CommitNodeStart() { m.touch() }

// Playback speed bounds. The inspector clamps what is typed and the
// duration estimates clamp what a file says, so a pass is always a finite
// length: a denormal speed would otherwise make the timeline's span
// infinite and its tick loop endless.
const (
	minSpeed = 0.01
	maxSpeed = 100
)

// clampSpeed bounds a playback speed to [minSpeed, maxSpeed]; a non-finite
// or non-positive value resolves to the default 1.
func clampSpeed(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return 1
	}
	return min(max(v, minSpeed), maxSpeed)
}

// passSeconds is one pass of a clip, optionally narrowed to a marker.
func (m *Model) passSeconds(alias, animID, segment string, speed float64) (float64, bool) {
	b, ok := m.bundles[alias]
	if !ok {
		return 0, false
	}
	speed = clampSpeed(speed)
	anim, err := b.Animation(animID)
	if err != nil {
		return 0, false
	}
	if segment != "" {
		if mk, ok := anim.Marker(segment); ok {
			fps := anim.FrameRate()
			if fps <= 0 {
				fps = 60
			}
			return (mk.End - mk.Start) / fps / speed, true
		}
	}
	return anim.Duration().Seconds() / speed, true
}

// NodeDuration is how long the node runs through its chain — one pass of
// each link, in seconds — for the timeline's bars and the content end.
// closed reports whether it actually finishes; a chain parked on a loop
// (or a plain looping clip) is open past that point. A machine's length
// is open-ended, so 0 and false.
func (m *Model) NodeDuration(n *lottie.SceneNode) (float64, bool) {
	if n == nil || n.Kind != lottie.SceneNodeAnimation {
		return 0, false
	}
	total, ok := m.passSeconds(n.Source.Bundle, n.Source.ID, n.Playback.Segment, n.Playback.PlaybackSpeed())
	if !ok {
		return 0, false
	}
	if n.Playback.Loop {
		return total, false
	}
	anim := n.Source.ID
	for _, st := range n.Playback.Then {
		if st.Animation != "" {
			anim = st.Animation
		}
		d, ok := m.passSeconds(n.Source.Bundle, anim, st.Segment, st.PlaybackSpeed())
		if !ok {
			break
		}
		total += d
		if st.Loop {
			return total, false
		}
	}
	return total, true
}

// SelectedSourceRef resolves the palette's source selection.
func (m *Model) SelectedSourceRef() (sourceRef, bool) {
	src := m.Sources()
	if m.selSource < 0 || m.selSource >= len(src) {
		return sourceRef{}, false
	}
	return src[m.selSource], true
}

func (m *Model) NodeNames() []string {
	out := make([]string, 0, len(m.scene.Nodes))
	for _, n := range m.scene.Nodes {
		out = append(out, n.Name)
	}
	return out
}

// FocusableNodeNames lists the nodes focus can land on, for the neighbor
// and initial-focus pickers.
func (m *Model) FocusableNodeNames() []string {
	var out []string
	for _, n := range m.scene.Nodes {
		if n.Focus.Focusable {
			out = append(out, n.Name)
		}
	}
	return out
}

// ---- per-node source introspection, for the inspector's pickers ----

// Markers lists the marker names of a node's animation.
func (m *Model) Markers(n *lottie.SceneNode) []string {
	if n == nil || n.Kind != lottie.SceneNodeAnimation {
		return nil
	}
	b, ok := m.bundles[n.Source.Bundle]
	if !ok {
		return nil
	}
	anim, err := b.Animation(n.Source.ID)
	if err != nil {
		return nil
	}
	var out []string
	for _, mk := range anim.Markers() {
		if mk.Name != "" {
			out = append(out, mk.Name)
		}
	}
	return out
}

// machineOf resolves a machine node's document.
func (m *Model) machineOf(n *lottie.SceneNode) *lottie.StateMachine {
	if n == nil || n.Kind != lottie.SceneNodeMachine {
		return nil
	}
	b, ok := m.bundles[n.Source.Bundle]
	if !ok {
		return nil
	}
	sm, err := b.StateMachine(n.Source.ID)
	if err != nil {
		return nil
	}
	return sm
}

// MachineStates lists a machine node's state names, for the entry picker.
func (m *Model) MachineStates(n *lottie.SceneNode) []string {
	sm := m.machineOf(n)
	if sm == nil {
		return nil
	}
	out := make([]string, 0, len(sm.States))
	for _, st := range sm.States {
		out = append(out, st.Name)
	}
	return out
}

// MachineEvents lists a machine node's Event inputs, for the fireEvent
// picker.
func (m *Model) MachineEvents(n *lottie.SceneNode) []string {
	sm := m.machineOf(n)
	if sm == nil {
		return nil
	}
	var out []string
	for _, in := range sm.Inputs {
		if in.Type == lottie.InputEvent {
			out = append(out, in.Name)
		}
	}
	return out
}

// ---- playback chain ----

func (m *Model) SelectedStepIndex() int { return m.selStep }

func (m *Model) SelectStep(i int) { m.selStep = i; m.generation++ }

func (m *Model) SelectedStep() *lottie.ScenePlayStep {
	n := m.SelectedNode()
	if n == nil || m.selStep < 0 || m.selStep >= len(n.Playback.Then) {
		return nil
	}
	return &n.Playback.Then[m.selStep]
}

// AddStep chains one more clip after the node's current playback: the
// idle loop after the entrance. The base clip's loop clears, since a
// looping clip never completes into its chain.
func (m *Model) AddStep() {
	n := m.SelectedNode()
	if n == nil || n.Kind != lottie.SceneNodeAnimation {
		return
	}
	n.Playback.Loop = false
	n.Playback.Then = append(n.Playback.Then, lottie.ScenePlayStep{Loop: true})
	// Only the last step may loop; the one before this new tail must run
	// through.
	if len(n.Playback.Then) > 1 {
		n.Playback.Then[len(n.Playback.Then)-2].Loop = false
	}
	m.selStep = len(n.Playback.Then) - 1
	m.touch()
}

func (m *Model) DeleteStep(i int) {
	n := m.SelectedNode()
	if n == nil || i < 0 || i >= len(n.Playback.Then) {
		return
	}
	n.Playback.Then = append(n.Playback.Then[:i], n.Playback.Then[i+1:]...)
	m.selStep = -1
	m.touch()
}

// markersFor lists an animation's marker names by bundle alias and id.
func (m *Model) markersFor(alias, animID string) []string {
	b, ok := m.bundles[alias]
	if !ok {
		return nil
	}
	anim, err := b.Animation(animID)
	if err != nil {
		return nil
	}
	var out []string
	for _, mk := range anim.Markers() {
		if mk.Name != "" {
			out = append(out, mk.Name)
		}
	}
	return out
}

// StepMarkers lists the markers of the clip a chain step plays: its own
// animation, or the one the chain was on before it.
func (m *Model) StepMarkers(n *lottie.SceneNode, step int) []string {
	if n == nil {
		return nil
	}
	anim := n.Source.ID
	for i := 0; i <= step && i < len(n.Playback.Then); i++ {
		if a := n.Playback.Then[i].Animation; a != "" {
			anim = a
		}
	}
	return m.markersFor(n.Source.Bundle, anim)
}

// BundleAnimations lists the clip ids of a node's bundle.
func (m *Model) BundleAnimations(n *lottie.SceneNode) []string {
	if n == nil {
		return nil
	}
	b, ok := m.bundles[n.Source.Bundle]
	if !ok {
		return nil
	}
	return b.AnimationIDs()
}

// BundleMachines lists the machine ids of a node's bundle.
func (m *Model) BundleMachines(n *lottie.SceneNode) []string {
	if n == nil {
		return nil
	}
	b, ok := m.bundles[n.Source.Bundle]
	if !ok {
		return nil
	}
	return b.StateMachineIDs()
}

// ---- bindings ----

func (m *Model) SelectedBinding() *lottie.SceneBinding {
	n := m.SelectedNode()
	if n == nil || m.selBinding < 0 || m.selBinding >= len(n.Bindings) {
		return nil
	}
	return &n.Bindings[m.selBinding]
}

func (m *Model) SelectBinding(i int) { m.selBinding = i; m.generation++ }

func (m *Model) SelectedBindingIndex() int { return m.selBinding }

func (m *Model) AddBinding() {
	n := m.SelectedNode()
	if n == nil {
		return
	}
	do := lottie.SceneCallback
	if n.Kind == lottie.SceneNodeMachine {
		do = lottie.SceneFireEvent
	}
	n.Bindings = append(n.Bindings, lottie.SceneBinding{On: lottie.SceneActivate, Do: do})
	m.selBinding = len(n.Bindings) - 1
	m.touch()
}

func (m *Model) DeleteBinding(i int) {
	n := m.SelectedNode()
	if n == nil || i < 0 || i >= len(n.Bindings) {
		return
	}
	n.Bindings = append(n.Bindings[:i], n.Bindings[i+1:]...)
	m.selBinding = -1
	m.touch()
}

// ---- preview ----

func (m *Model) PreviewMode() bool { return m.preview }

// TogglePreview flips between arranging and driving. Entering preview
// rebuilds the player, so it starts from the scene's initial focus.
func (m *Model) TogglePreview() {
	m.preview = !m.preview
	if m.preview {
		m.docGen++ // force a fresh player
		if m.Player() == nil {
			m.setStatus("preview: fix the scene errors first")
		} else {
			m.setStatus("preview: Tab/arrows move focus, Enter activates, Esc cancels")
		}
	} else {
		// Back to arranging: the camera returns to a framing overlay.
		m.applyEditCamera(m.player)
		m.setStatus("editing")
	}
	m.generation++
}

// ReplayScene restarts the running scene from 0s and runs the transport,
// so the timeline's entrances play again.
func (m *Model) ReplayScene() {
	if sp := m.Player(); sp != nil {
		sp.Restart()
		if m.viewPhase != "" {
			sp.SetPhase(m.viewPhase)
		}
		m.applyEditCamera(sp)
		m.playing = true
		m.setStatus("replaying from 0s")
	}
	m.generation++
}

// Player is the running scene the canvas draws, rebuilt lazily after any
// structural edit. It is nil while the scene has an error that prevents
// starting — a missing bundle, say.
func (m *Model) Player() *lottie.ScenePlayer {
	// Only the document generation decides: a nil player whose build
	// already failed for this generation is not retried, since every
	// attempt decodes every image and font the scene references.
	if m.playerGen != m.docGen {
		m.rebuildPlayer()
	}
	return m.player
}

func (m *Model) PlayerErr() error { return m.playerErr }

func (m *Model) rebuildPlayer() {
	m.playerGen = m.docGen
	m.player, m.playerErr = nil, nil
	m.seekValid = false
	byPath := map[string]*lottie.Bundle{}
	for _, ref := range m.scene.Bundles {
		if b, ok := m.bundles[ref.Alias]; ok {
			byPath[ref.Path] = b
		}
	}
	sp, err := m.scene.NewScenePlayerWithLoader(lottie.SceneLoader{
		Bundle: func(path string) (*lottie.Bundle, error) {
			if b, ok := byPath[path]; ok {
				return b, nil
			}
			return nil, fmt.Errorf("not loaded")
		},
		File: m.readAssetFile,
	})
	if err != nil {
		m.playerErr = err
		return
	}
	sp.OnCallback(func(node, name string) {
		m.setStatus("callback: node=%q name=%q", node, name)
		m.generation++
	})
	sp.OnPhaseChanged(func(from, to string) {
		// Follow the running scene, so the timeline and canvas show the
		// phase an auto-advance or a binding entered. Entering a phase
		// re-resolves the document camera, so edit mode neutralizes it
		// again.
		m.viewPhase = to
		m.applyEditCamera(sp)
		m.generation++
	})
	if m.viewPhase != "" && m.viewPhase != sp.Phase() {
		if !sp.SetPhase(m.viewPhase) {
			m.viewPhase = sp.Phase()
		}
	}
	m.applyEditCamera(sp)
	m.player = sp
}

// ---- transport ----

// ScenePlaying reports whether the edit-mode transport runs. Preview mode
// always runs regardless.
func (m *Model) ScenePlaying() bool { return m.playing }

// ContentEnd is when the viewed choreography finishes: the last entrance
// plus one pass of its clip — a looping clip counts one pass, so a scene
// of a single looped clip still plays through once before the transport
// parks itself. Machines, images, and text have no length; they get a
// one-second grace so an entrance is always watchable.
func (m *Model) ContentEnd() float64 {
	end := 0.0
	for i := range m.scene.Nodes {
		n := &m.scene.Nodes[i]
		if !m.nodeInView(n) {
			continue
		}
		// One pass through the chain when the length is known — a loop's
		// single pass included — else a one-second grace.
		e := n.Start
		if d, _ := m.NodeDuration(n); d > 0 {
			e += d
		} else {
			e += 1
		}
		end = max(end, e)
	}
	return end
}

// maxSeekSeconds bounds how far a scrub replays: the replay runs
// synchronously on the game loop, one Update per tick of scene time, so
// ten minutes (36,000 updates at 60 TPS) is the most a single scrub may
// cost. Entrances can sit further out (maxSceneSeconds); the playhead
// then parks at this limit.
const maxSeekSeconds = 600

// SeekScene scrubs the scene to an absolute time by replaying from zero
// — the only deterministic way to land mid-choreography. Scrubbing parks
// the transport; Play resumes from here. Asking again for the time the
// scene already sits at is free, so a held ruler drag costs nothing until
// the cursor moves.
func (m *Model) SeekScene(sec float64) {
	sp := m.Player()
	if sp == nil {
		return
	}
	if math.IsNaN(sec) {
		sec = 0
	}
	sec = min(max(0, sec), maxSeekSeconds)
	if m.seekValid && sec == m.seekTarget && sp.Time() == m.seekTime && !m.playing {
		return
	}
	sp.Restart()
	if m.viewPhase != "" {
		sp.SetPhase(m.viewPhase)
	}
	m.applyEditCamera(sp)
	tps := float64(ebiten.TPS())
	if tps <= 0 {
		tps = 60
	}
	for range int(sec*tps + 0.5) {
		sp.Update()
	}
	m.seekTarget, m.seekTime, m.seekValid = sec, sp.Time(), true
	m.playing = false
	m.generation++
}

// SwapNodes exchanges two nodes' draw-order slots, which is how the
// timeline reorders overlap by dragging rows.
func (m *Model) SwapNodes(i, j int) {
	if i == j || i < 0 || j < 0 || i >= len(m.scene.Nodes) || j >= len(m.scene.Nodes) {
		return
	}
	m.scene.Nodes[i], m.scene.Nodes[j] = m.scene.Nodes[j], m.scene.Nodes[i]
	switch m.selNode {
	case i:
		m.selNode = j
	case j:
		m.selNode = i
	}
	m.touch()
}

// TogglePlayback is the timeline's play/pause button. Playing from the
// end starts over, the way a video editor's play button does.
func (m *Model) TogglePlayback() {
	if m.playing {
		m.playing = false
		m.setStatus("paused")
		m.generation++
		return
	}
	if sp := m.Player(); sp != nil && sp.Time() >= m.ContentEnd() {
		sp.Restart()
		if m.viewPhase != "" {
			sp.SetPhase(m.viewPhase)
		}
		m.applyEditCamera(sp)
	}
	m.playing = true
	m.setStatus("playing")
	m.generation++
}

// EditTick advances the scene while the transport runs, pausing on its
// own when the choreography finishes. The canvas calls it each tick in
// edit mode; preview mode updates unconditionally.
func (m *Model) EditTick() {
	sp := m.Player()
	if sp == nil || !m.playing {
		return
	}
	if sp.Time() >= m.ContentEnd() {
		m.playing = false
		m.setStatus("finished at %.2fs", sp.Time())
		m.generation++
		return
	}
	sp.Update()
}

// ---- phases ----

// ViewPhase is the phase the canvas and timeline show; empty for a scene
// without phases.
func (m *Model) ViewPhase() string {
	if m.viewPhase == "" && len(m.scene.Phases) > 0 {
		return m.scene.Phases[0].Name
	}
	return m.viewPhase
}

// SetViewPhase switches which phase is being edited and previewed.
func (m *Model) SetViewPhase(name string) {
	m.viewPhase = name
	if sp := m.Player(); sp != nil && name != "" {
		sp.SetPhase(name)
		m.applyEditCamera(sp)
	}
	m.generation++
}

func (m *Model) SelectedPhaseIndex() int { return m.selPhase }

func (m *Model) SelectPhase(i int) {
	m.selPhase = i
	m.generation++
}

func (m *Model) SelectedPhase() *lottie.ScenePhase {
	if m.selPhase < 0 || m.selPhase >= len(m.scene.Phases) {
		return nil
	}
	return &m.scene.Phases[m.selPhase]
}

// AddPhase appends a phase. The first phase added becomes where the
// scene starts.
func (m *Model) AddPhase() {
	var taken []string
	for _, p := range m.scene.Phases {
		taken = append(taken, p.Name)
	}
	name := uniqueID("phase", taken)
	m.scene.Phases = append(m.scene.Phases, lottie.ScenePhase{Name: name})
	m.selPhase = len(m.scene.Phases) - 1
	m.setStatus("added phase %q", name)
	m.touch()
}

// DeletePhase drops a phase; nodes that belonged to it join every phase
// again, and references to it clear.
func (m *Model) DeletePhase(i int) {
	if i < 0 || i >= len(m.scene.Phases) {
		return
	}
	name := m.scene.Phases[i].Name
	m.scene.Phases = append(m.scene.Phases[:i], m.scene.Phases[i+1:]...)
	for j := range m.scene.Nodes {
		if m.scene.Nodes[j].Phase == name {
			m.scene.Nodes[j].Phase = ""
		}
		bs := m.scene.Nodes[j].Bindings
		for k := range bs {
			if bs[k].Do == lottie.ScenePhaseAction && bs[k].Arg == name {
				bs[k].Arg = ""
			}
		}
	}
	for j := range m.scene.Phases {
		if m.scene.Phases[j].Next == name {
			m.scene.Phases[j].Next = ""
		}
	}
	if m.viewPhase == name {
		m.viewPhase = ""
	}
	m.selPhase = -1
	m.setStatus("deleted phase %q", name)
	m.touch()
}

// RenamePhase renames a phase and repoints everything at it.
func (m *Model) RenamePhase(i int, name string) {
	name = strings.TrimSpace(name)
	if i < 0 || i >= len(m.scene.Phases) || name == "" {
		return
	}
	old := m.scene.Phases[i].Name
	if old == name {
		return
	}
	for _, p := range m.scene.Phases {
		if p.Name == name {
			m.setStatus("a phase named %q already exists", name)
			m.generation++
			return
		}
	}
	m.scene.Phases[i].Name = name
	for j := range m.scene.Phases {
		if m.scene.Phases[j].Next == old {
			m.scene.Phases[j].Next = name
		}
	}
	for j := range m.scene.Nodes {
		if m.scene.Nodes[j].Phase == old {
			m.scene.Nodes[j].Phase = name
		}
		bs := m.scene.Nodes[j].Bindings
		for k := range bs {
			if bs[k].Do == lottie.ScenePhaseAction && bs[k].Arg == old {
				bs[k].Arg = name
			}
		}
	}
	if m.viewPhase == old {
		m.viewPhase = name
	}
	m.touch()
}

// PhaseNames lists every phase, for pickers.
func (m *Model) PhaseNames() []string {
	out := make([]string, 0, len(m.scene.Phases))
	for _, p := range m.scene.Phases {
		out = append(out, p.Name)
	}
	return out
}

// nodeInView reports whether a node participates in the viewed phase.
func (m *Model) nodeInView(n *lottie.SceneNode) bool {
	return n.Phase == "" || n.Phase == m.ViewPhase()
}

// TimelineNodes lists the document indices of the nodes the timeline
// shows: the viewed phase's members plus the phaseless, in draw order.
func (m *Model) TimelineNodes() []int {
	var out []int
	for i := range m.scene.Nodes {
		if m.nodeInView(&m.scene.Nodes[i]) {
			out = append(out, i)
		}
	}
	return out
}

// ---- validation ----

// Problems lists everything wrong with the scene: structural findings,
// bundle load failures, and whatever stopped the player.
func (m *Model) Problems() []string {
	if m.problemsGen == m.generation+1 {
		return m.problemsCache
	}
	var out []string
	for _, err := range m.scene.Validate() {
		out = append(out, err.Error())
	}
	for alias, msg := range m.bundleErrs {
		out = append(out, fmt.Sprintf("bundle %q: %s", alias, msg))
	}
	for alias, msg := range m.assetErrs {
		out = append(out, fmt.Sprintf("asset %q: %s", alias, msg))
	}
	sort.Strings(out)
	if m.Player() == nil && m.playerErr != nil {
		out = append(out, m.playerErr.Error())
	}
	m.problemsCache, m.problemsGen = out, m.generation+1
	return out
}

func uniqueID(base string, taken []string) string {
	used := map[string]bool{}
	for _, t := range taken {
		used[t] = true
	}
	if base == "" {
		base = "node"
	}
	if !used[base] {
		return base
	}
	for i := 2; ; i++ {
		id := fmt.Sprintf("%s%d", base, i)
		if !used[id] {
			return id
		}
	}
}
