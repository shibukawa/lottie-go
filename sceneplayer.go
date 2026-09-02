package lottie

import (
	"bytes"
	"fmt"
	"image"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// ScaleMode maps a scene's design size onto the real screen.
type ScaleMode int

const (
	// ScaleContain fits the whole design box on screen, centered, leaving
	// letterbox bars where the aspect ratios differ.
	ScaleContain ScaleMode = iota
	// ScaleCover fills the screen, centered, cropping the design box's
	// sides or top where the aspect ratios differ.
	ScaleCover
	// ScaleStretch fills the screen exactly, distorting aspect.
	ScaleStretch
	// ScaleCenter draws at 1:1 scale, centered.
	ScaleCenter
)

// FocusDirection is one focus move of MoveFocus.
type FocusDirection int

const (
	FocusUp FocusDirection = iota
	FocusDown
	FocusLeft
	FocusRight
	// FocusNext walks tab order forward, wrapping; FocusPrev backward.
	FocusNext
	FocusPrev
)

// SceneResolver loads the bundle a scene references, handed the reference's
// path verbatim. It typically reads from an fs.FS or go:embed next to the
// scene file.
type SceneResolver func(path string) (*Bundle, error)

// SceneLoader supplies what a scene references beyond bundles. File
// serves image and font files by their reference paths; image data goes
// through image.Decode, so the consuming binary registers formats by
// blank-importing them (image/png, image/jpeg, golang.org/x/image/webp).
type SceneLoader struct {
	Bundle SceneResolver
	File   func(path string) ([]byte, error)
}

// ScenePlayer runs a Scene: it plays every node, routes focus and input,
// and reports interaction callbacks. The game pushes input each frame —
// the player reads no device itself, so keys stay rebindable:
//
//	sp.MoveFocus(lottie.FocusDown)   // d-pad / cursor keys / Tab
//	sp.Activate()                    // confirm button
//	sp.Pointer(x, y, pressed)        // raw screen coords
//	sp.Update()                      // in ebiten.Game.Update
//	sp.Draw(screen, nil)             // in Draw
//
// A ScenePlayer is not safe for concurrent use; drive it from the game's
// Update/Draw loop.
type ScenePlayer struct {
	scene   *Scene
	bundles map[string]*Bundle
	images  map[string]*ebiten.Image
	fonts   map[string]*text.GoTextFaceSource
	nodes   []*SceneNodePlayer
	byName  map[string]*SceneNodePlayer

	focused     *SceneNodePlayer
	hovered     *SceneNodePlayer
	pointerDown bool
	pressed     *SceneNodePlayer // node the pointer went down on

	screenW, screenH int
	scaleMode        ScaleMode
	mapped           bool

	// clock is seconds since the current phase was entered; it gates each
	// node's Start time and the phase's Duration.
	clock float64

	// phase is the running phase's name, empty for a scene that declares
	// none. phaseEnded remembers that OnPhaseEnd fired, so a phase that
	// stays put reports its end once.
	phase      string
	phaseEnded bool

	// cam is the camera in effect, resolved from the document on start and
	// on every phase entry, and overridden at runtime with SetCamera.
	cam SceneCamera

	// bindingDepth counts nested deliver calls: focus and phase actions
	// re-enter deliver, and two bindings pointing at each other would
	// otherwise recurse without bound.
	bindingDepth int

	onPhaseChanged func(from, to string)
	onPhaseEnd     func(phase string)

	onFocusChanged func(from, to string)
	onCallback     func(node, name string)

	unsupported map[string]struct{}
	err         error
}

// SceneNodePlayer is one running node of a ScenePlayer: the document node
// plus its live playback instance and runtime overrides.
type SceneNodePlayer struct {
	sp  *ScenePlayer
	def *SceneNode

	player  *Player             // animation node
	machine *StateMachinePlayer // machine node
	img     *ebiten.Image       // image node
	face    *text.GoTextFace    // text node

	// Chain state (animation nodes): which clip is loaded, which Then
	// step is playing (-1 = the base playback), and whether the current
	// pass completed this tick.
	curAnim  string
	stepIdx  int
	clipDone bool

	// textVal is the text node's current content, initialized from the
	// document and overwritten by the game with SetText. It survives
	// Restart and phase switches: a score display belongs to the host.
	textVal   string
	textColor [4]float64
	measFor   string // textVal the cached measure below was taken for
	measW     float64
	measH     float64

	visible bool
	started bool // the node's Start time has come (or an action forced it)
	tf      SceneTransform
}

// NewScenePlayer starts every node of the scene. resolve is called once per
// bundle reference; a failing resolve is an error naming the alias, so a
// missing asset is diagnosed against the scene, not the filesystem. A scene
// that also references images or fonts needs NewScenePlayerWithLoader.
func (s *Scene) NewScenePlayer(resolve SceneResolver) (*ScenePlayer, error) {
	return s.NewScenePlayerWithLoader(SceneLoader{Bundle: resolve})
}

// NewScenePlayerWithLoader starts every node of the scene, loading
// bundles, images, and fonts through the loader. Every load failure is an
// error naming the alias.
func (s *Scene) NewScenePlayerWithLoader(l SceneLoader) (*ScenePlayer, error) {
	sp := &ScenePlayer{
		scene:       s,
		bundles:     map[string]*Bundle{},
		images:      map[string]*ebiten.Image{},
		fonts:       map[string]*text.GoTextFaceSource{},
		byName:      map[string]*SceneNodePlayer{},
		unsupported: map[string]struct{}{},
	}
	for i := range s.Bundles {
		ref := &s.Bundles[i]
		if l.Bundle == nil {
			return nil, fmt.Errorf("lottie: scene references bundle %q but the loader loads no bundles", ref.Alias)
		}
		b, err := l.Bundle(ref.Path)
		if err != nil {
			return nil, fmt.Errorf("lottie: scene bundle %q (%s): %w", ref.Alias, ref.Path, err)
		}
		sp.bundles[ref.Alias] = b
	}
	loadFile := func(kind string, a SceneAsset) ([]byte, error) {
		if l.File == nil {
			return nil, fmt.Errorf("lottie: scene references %s %q but the loader loads no files", kind, a.Alias)
		}
		data, err := l.File(a.Path)
		if err != nil {
			return nil, fmt.Errorf("lottie: scene %s %q (%s): %w", kind, a.Alias, a.Path, err)
		}
		return data, nil
	}
	for _, a := range s.Images {
		data, err := loadFile("image", a)
		if err != nil {
			return nil, err
		}
		// Formats come from the consuming binary's blank imports, the
		// way this package handles every image asset.
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("lottie: scene image %q (%s): %w", a.Alias, a.Path, err)
		}
		sp.images[a.Alias] = ebiten.NewImageFromImage(img)
	}
	for _, a := range s.Fonts {
		data, err := loadFile("font", a)
		if err != nil {
			return nil, err
		}
		src, err := text.NewGoTextFaceSource(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("lottie: scene font %q (%s): %w", a.Alias, a.Path, err)
		}
		sp.fonts[a.Alias] = src
	}
	if len(s.Phases) > 0 {
		sp.phase = s.Phases[0].Name
	}
	sp.cam = s.CameraFor(sp.phase)
	for i := range s.Nodes {
		def := &s.Nodes[i]
		n := &SceneNodePlayer{sp: sp, def: def, visible: true, started: def.Start <= 0, tf: def.Transform}
		n.textVal = def.Text.Value
		if err := n.start(); err != nil {
			return nil, fmt.Errorf("lottie: scene node %q: %w", def.Name, err)
		}
		sp.nodes = append(sp.nodes, n)
		if _, dup := sp.byName[def.Name]; !dup {
			sp.byName[def.Name] = n
		}
	}
	if n, ok := sp.initialFocusNode(); ok {
		sp.changeFocus(n)
	}
	return sp, nil
}

// start creates the node's playback instance from its source.
func (n *SceneNodePlayer) start() error {
	bundle := func() (*Bundle, error) {
		b, ok := n.sp.bundles[n.def.Source.Bundle]
		if !ok {
			return nil, fmt.Errorf("unknown bundle alias %q", n.def.Source.Bundle)
		}
		return b, nil
	}
	switch n.def.Kind {
	case SceneNodeAnimation:
		pb := n.def.Playback
		n.stepIdx, n.clipDone = -1, false
		p, err := n.loadClip(n.def.Source.ID)
		if err != nil {
			return err
		}
		if pb.Segment != "" && !p.SetMarkerRange(pb.Segment) {
			n.sp.note(fmt.Sprintf("node %q: unknown marker %q", n.def.Name, pb.Segment))
		}
		p.SetLoop(pb.Loop)
		p.SetLoopCount(pb.LoopCount)
		p.SetSpeed(pb.PlaybackSpeed())
		p.SetReverse(pb.PlaybackMode() == PlayReverse)
		p.Rewind()
		if pb.Autoplay {
			p.Play()
		} else {
			p.Pause()
		}
	case SceneNodeMachine:
		b, err := bundle()
		if err != nil {
			return err
		}
		m, err := b.NewStateMachinePlayer(n.def.Source.ID)
		if err != nil {
			return err
		}
		if n.def.Entry != "" && !m.EnterState(n.def.Entry) {
			n.sp.note(fmt.Sprintf("node %q: unknown entry state %q", n.def.Name, n.def.Entry))
		}
		n.machine = m
	case SceneNodeImage:
		img, ok := n.sp.images[n.def.Source.Image]
		if !ok {
			return fmt.Errorf("unknown image alias %q", n.def.Source.Image)
		}
		n.img = img
	case SceneNodeText:
		src, ok := n.sp.fonts[n.def.Text.Font]
		if !ok {
			return fmt.Errorf("unknown font alias %q", n.def.Text.Font)
		}
		n.face = &text.GoTextFace{Source: src, Size: n.def.Text.size()}
		n.textColor = [4]float64{1, 1, 1, 1}
		if c := n.def.Text.Color; c != "" {
			if parsed, ok := ParseSceneColor(c); ok {
				n.textColor = parsed
			}
		}
		n.measFor = "\x00unmeasured"
	default:
		return fmt.Errorf("unknown kind %q", n.def.Kind)
	}
	return nil
}

// loadClip swaps the node's player onto the named animation of its
// bundle, wiring completion so the chain can advance.
func (n *SceneNodePlayer) loadClip(id string) (*Player, error) {
	b, ok := n.sp.bundles[n.def.Source.Bundle]
	if !ok {
		return nil, fmt.Errorf("unknown bundle alias %q", n.def.Source.Bundle)
	}
	anim, err := b.Animation(id)
	if err != nil {
		return nil, err
	}
	p := anim.NewPlayer()
	p.OnComplete(func() { n.clipDone = true })
	n.player, n.curAnim = p, id
	return p, nil
}

// advanceChain plays the next Then step after a completion. Past the last
// step the player simply stays where the clip ended.
func (n *SceneNodePlayer) advanceChain() {
	steps := n.def.Playback.Then
	next := n.stepIdx + 1
	if next >= len(steps) {
		return
	}
	n.stepIdx = next
	st := steps[next]
	p := n.player
	if st.Animation != "" && st.Animation != n.curAnim {
		var err error
		if p, err = n.loadClip(st.Animation); err != nil {
			n.sp.note(fmt.Sprintf("node %q: %v", n.def.Name, err))
			return
		}
	}
	if st.Segment == "" {
		p.SetRange(0, 0)
	} else if !p.SetMarkerRange(st.Segment) {
		n.sp.note(fmt.Sprintf("node %q: unknown marker %q", n.def.Name, st.Segment))
	}
	p.SetLoop(st.Loop)
	p.SetLoopCount(st.LoopCount)
	p.SetSpeed(st.PlaybackSpeed())
	mode := st.Mode
	if mode == "" {
		mode = PlayForward
	}
	p.SetReverse(mode == PlayReverse)
	p.Rewind()
	p.Play()
}

// Scene returns the document this player runs. Treat it as read-only while
// the player is live.
func (sp *ScenePlayer) Scene() *Scene { return sp.scene }

// Node returns the named node's running instance. An unknown name reports
// false, never panics.
func (sp *ScenePlayer) Node(name string) (*SceneNodePlayer, bool) {
	n, ok := sp.byName[name]
	return n, ok
}

// Nodes returns every node in draw order (first is the back). The slice is
// the player's own; do not modify it.
func (sp *ScenePlayer) Nodes() []*SceneNodePlayer { return sp.nodes }

// OnFocusChanged registers a function to run on every focus move, handed
// node names; an empty string means no focus.
func (sp *ScenePlayer) OnFocusChanged(f func(from, to string)) { sp.onFocusChanged = f }

// OnCallback registers the function that receives callback bindings —
// the scene's outputs a game acts on. A cancel that no node handled
// arrives with an empty node name and name "cancel".
func (sp *ScenePlayer) OnCallback(f func(node, name string)) { sp.onCallback = f }

// Err returns the first error hit while running. Playback continues past
// it with whatever was already on screen.
func (sp *ScenePlayer) Err() error { return sp.err }

// UnsupportedFeatures lists problems found while running, such as bindings
// naming markers a clip does not hold.
func (sp *ScenePlayer) UnsupportedFeatures() []string {
	out := make([]string, 0, len(sp.unsupported))
	for f := range sp.unsupported {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

func (sp *ScenePlayer) note(feature string) { sp.unsupported[feature] = struct{}{} }

// SetScreenMapping maps the design box (Scene.Size) onto a w x h screen.
// Call it again on resize. Draw applies the mapping and Pointer inverts
// it, so the game passes raw screen coordinates. Without a mapping the
// scene draws 1:1 at the origin.
func (sp *ScenePlayer) SetScreenMapping(w, h int, mode ScaleMode) {
	sp.screenW, sp.screenH = w, h
	sp.scaleMode = mode
	sp.mapped = w > 0 && h > 0 && sp.scene.Size.W > 0 && sp.scene.Size.H > 0
}

// mapping returns the scene-to-screen scale and offset.
func (sp *ScenePlayer) mapping() (sx, sy, ox, oy float64) {
	if !sp.mapped {
		return 1, 1, 0, 0
	}
	dw, dh := float64(sp.scene.Size.W), float64(sp.scene.Size.H)
	w, h := float64(sp.screenW), float64(sp.screenH)
	switch sp.scaleMode {
	case ScaleContain:
		s := math.Min(w/dw, h/dh)
		sx, sy = s, s
	case ScaleCover:
		s := math.Max(w/dw, h/dh)
		sx, sy = s, s
	case ScaleStretch:
		return w / dw, h / dh, 0, 0
	case ScaleCenter:
		sx, sy = 1, 1
	}
	return sx, sy, (w - dw*sx) / 2, (h - dh*sy) / 2
}

// ScreenGeoM returns the scene-to-screen transform of the current mapping,
// for a game drawing its own overlays in scene coordinates.
func (sp *ScenePlayer) ScreenGeoM() ebiten.GeoM {
	sx, sy, ox, oy := sp.mapping()
	var g ebiten.GeoM
	g.Scale(sx, sy)
	g.Translate(ox, oy)
	return g
}

// GeoM is the camera's scene-to-view transform for a node at the given
// parallax depth, over a w x h design box. Depth scales every component of
// the camera's effect — the translation multiplies by it, the zoom raises
// to its power, the rotation multiplies by it — so depth 0 is exactly the
// identity (a screen-pinned HUD) and depth 1 is the full camera.
func (c SceneCamera) GeoM(w, h int, depth float64) ebiten.GeoM {
	var g ebiten.GeoM
	if depth == 0 || c.isIdentity() {
		return g
	}
	cx, cy := float64(w)/2, float64(h)/2
	// The camera moves; the content shifts the opposite way. Zoom and
	// rotation pivot on the design box's center.
	g.Translate(-c.X*depth-cx, -c.Y*depth-cy)
	if r := c.Rotation * depth; r != 0 {
		g.Rotate(-r * math.Pi / 180)
	}
	if z := c.ZoomFactor(); z != 1 {
		s := math.Pow(z, depth)
		g.Scale(s, s)
	}
	g.Translate(cx, cy)
	return g
}

// Camera returns the camera in effect right now.
func (sp *ScenePlayer) Camera() SceneCamera { return sp.cam }

// SetCamera overrides the camera at runtime — a game panning or zooming
// per frame. Like SetTransform it is not persisted: entering a phase (or
// Restart) resolves the camera from the document again.
//
// Camera then reports the override, so a game easing toward the phase's
// camera reads its target from the document — Scene().CameraFor(Phase())
// — not from Camera, or it would chase itself.
func (sp *ScenePlayer) SetCamera(c SceneCamera) { sp.cam = c }

// cameraGeoM is the running camera's transform for one node's depth.
func (sp *ScenePlayer) cameraGeoM(depth float64) ebiten.GeoM {
	return sp.cam.GeoM(sp.scene.Size.W, sp.scene.Size.H, depth)
}

// Update advances the scene clock and every started node. Call it once
// per tick from ebiten.Game's Update, after pushing this frame's input.
// A node whose Start time arrives this tick makes its entrance: it draws,
// plays, and takes input from here on.
func (sp *ScenePlayer) Update() {
	tps := float64(ebiten.TPS())
	if tps <= 0 {
		tps = 60
	}
	sp.clock += 1 / tps
	entered := false
	for _, n := range sp.nodes {
		if !n.inPhase() {
			continue
		}
		if !n.started && sp.clock >= n.def.Start {
			n.started = true
			entered = true
		}
		if !n.started {
			continue
		}
		switch {
		case n.machine != nil:
			n.machine.Update()
		case n.player != nil:
			n.player.Update()
			// A completed pass advances the chain — the entrance clip
			// rolling into the idle loop — and reports as an event so a
			// binding can react (fire, switch phase, call back).
			if n.clipDone {
				n.clipDone = false
				n.advanceChain()
				sp.deliver(n, SceneComplete)
			}
		}
	}
	// A late entrance may be the first focusable thing on screen.
	if entered && sp.focused == nil {
		if n, ok := sp.initialFocusNode(); ok {
			sp.changeFocus(n)
		}
	}
	// A timed phase ends here: report it, then roll into the next phase
	// when one is named — an intro giving way to the main screen.
	if ph, ok := sp.scene.Phase(sp.phase); ok && !sp.phaseEnded && ph.Duration > 0 && sp.clock >= ph.Duration {
		sp.phaseEnded = true
		if sp.onPhaseEnd != nil {
			sp.onPhaseEnd(ph.Name)
		}
		if ph.Next != "" {
			sp.SetPhase(ph.Next)
		}
	}
}

// inPhase reports whether the node participates right now: it names the
// running phase, or no phase at all.
func (n *SceneNodePlayer) inPhase() bool {
	return n.def.Phase == "" || n.def.Phase == n.sp.phase
}

// Phase is the running phase's name, empty for a scene without phases.
func (sp *ScenePlayer) Phase() string { return sp.phase }

// OnPhaseChanged registers a function to run on every phase switch,
// including automatic Next advances.
func (sp *ScenePlayer) OnPhaseChanged(f func(from, to string)) { sp.onPhaseChanged = f }

// OnPhaseEnd registers a function to run when a timed phase's duration
// elapses, before any automatic advance — how a game learns its outro
// finished.
func (sp *ScenePlayer) OnPhaseEnd(f func(phase string)) { sp.onPhaseEnd = f }

// SetPhase enters the named phase, reporting false for an unknown name.
// The clock restarts, members of the entered phase make their entrances
// afresh, members of other phases leave, and phaseless nodes keep playing
// through the switch — a background loop should not pop. Entering the
// running phase again replays it.
// maxBindingDepth bounds nested binding delivery, mirroring the state
// machine player's maxTransitionsPerUpdate.
const maxBindingDepth = 16

func (sp *ScenePlayer) SetPhase(name string) bool {
	if _, ok := sp.scene.Phase(name); !ok {
		return false
	}
	from := sp.phase
	sp.phase = name
	sp.phaseEnded = false
	sp.clock = 0
	sp.cam = sp.scene.CameraFor(name)
	sp.hovered, sp.pressed = nil, nil
	sp.pointerDown = false
	for _, n := range sp.nodes {
		if n.def.Phase != name || n.def.Phase == "" {
			continue
		}
		n.started = n.def.Start <= 0
		if err := n.start(); err != nil {
			sp.note(fmt.Sprintf("node %q: %v", n.def.Name, err))
		}
	}
	// Focus survives when its node still participates; otherwise it moves
	// to the scene's initial choice among what is on screen now.
	if sp.focused != nil && !sp.focused.focusable() {
		if n, ok := sp.initialFocusNode(); ok {
			sp.changeFocus(n)
		} else {
			sp.changeFocus(nil)
		}
	} else if sp.focused == nil {
		if n, ok := sp.initialFocusNode(); ok {
			sp.changeFocus(n)
		}
	}
	if sp.onPhaseChanged != nil {
		sp.onPhaseChanged(from, name)
	}
	return true
}

// Time is seconds since the scene (re)started — the timeline the nodes'
// Start times run on.
func (sp *ScenePlayer) Time() float64 { return sp.clock }

// Restart replays the scene from the top: the clock returns to zero,
// every node's playback is rebuilt, entrances wait for their Start times
// again, and focus returns to the scene's initial choice. Runtime
// visibility and transform overrides survive; they belong to the host. A
// SetCamera override does not: the camera resolves from the document
// again, the way entering a phase resolves it.
func (sp *ScenePlayer) Restart() {
	sp.clock = 0
	sp.focused, sp.hovered, sp.pressed = nil, nil, nil
	sp.pointerDown = false
	// From the top means the first phase too, with its timed end re-armed.
	if len(sp.scene.Phases) > 0 {
		sp.phase = sp.scene.Phases[0].Name
	}
	sp.cam = sp.scene.CameraFor(sp.phase)
	sp.phaseEnded = false
	for _, n := range sp.nodes {
		n.started = n.def.Start <= 0
		if err := n.start(); err != nil {
			sp.note(fmt.Sprintf("node %q: %v", n.def.Name, err))
		}
	}
	if n, ok := sp.initialFocusNode(); ok {
		sp.changeFocus(n)
	}
}

// Draw renders every visible node in document order, first at the back,
// through the camera and the screen mapping. opts may be nil; its GeoM
// applies after the mapping, its ColorScale to every node.
func (sp *ScenePlayer) Draw(dst *ebiten.Image, opts *DrawOptions) {
	base := sp.ScreenGeoM()
	if opts != nil {
		base.Concat(opts.GeoM)
	}
	camera := !sp.cam.isIdentity()
	for _, n := range sp.nodes {
		if !n.visible || !n.started || !n.inPhase() {
			continue
		}
		nb := base
		if camera {
			nb = sp.cameraGeoM(n.def.ParallaxDepth())
			nb.Concat(base)
		}
		o := DrawOptions{GeoM: n.geoM(nb)}
		if opts != nil {
			o.ColorScale = opts.ColorScale
			o.DisableAntiAlias = opts.DisableAntiAlias
		}
		o.ColorScale.ScaleAlpha(float32(n.tf.opacity()))
		switch {
		case n.machine != nil:
			n.machine.Draw(dst, &o)
		case n.player != nil:
			n.player.Draw(dst, &o)
		case n.img != nil:
			var op ebiten.DrawImageOptions
			op.GeoM = o.GeoM
			op.ColorScale = o.ColorScale
			op.Filter = ebiten.FilterLinear
			dst.DrawImage(n.img, &op)
		case n.face != nil:
			n.drawText(dst, &o)
		}
	}
}

// drawText paints a text node line by line, aligning each inside the
// measured block, anchored the way the document says.
func (n *SceneNodePlayer) drawText(dst *ebiten.Image, o *DrawOptions) {
	t := n.def.Text
	x0, y0, w, _ := n.localRect()
	ls := t.lineSpacing()
	c := n.textColor
	for i, line := range splitTextLines(n.textVal) {
		if line == "" {
			continue
		}
		lw, _ := text.Measure(line, n.face, 0)
		var xoff float64
		switch t.Align {
		case AlignCenter:
			xoff = (w - lw) / 2
		case AlignRight:
			xoff = w - lw
		}
		var g ebiten.GeoM
		g.Translate(x0+xoff, y0+float64(i)*ls)
		g.Concat(o.GeoM)
		var op text.DrawOptions
		op.GeoM = g
		op.ColorScale.Scale(
			float32(c[0]*c[3]), float32(c[1]*c[3]), float32(c[2]*c[3]), float32(c[3]))
		op.ColorScale.ScaleWithColorScale(o.ColorScale)
		op.Filter = ebiten.FilterLinear
		text.Draw(dst, line, n.face, &op)
	}
}

// geoM composes the node's own transform with base.
func (n *SceneNodePlayer) geoM(base ebiten.GeoM) ebiten.GeoM {
	var g ebiten.GeoM
	g.Scale(n.tf.scaleX(), n.tf.scaleY())
	g.Rotate(n.tf.Rotation * math.Pi / 180)
	g.Translate(n.tf.X, n.tf.Y)
	g.Concat(base)
	return g
}

// --- node accessors ---

// Name is the node's game-facing id.
func (n *SceneNodePlayer) Name() string { return n.def.Name }

// Kind reports what the node plays.
func (n *SceneNodePlayer) Kind() SceneNodeKind { return n.def.Kind }

// Definition returns the document node. Treat it as read-only while the
// player is live.
func (n *SceneNodePlayer) Definition() *SceneNode { return n.def }

// Machine returns the node's running state machine, or nil for an
// animation node.
func (n *SceneNodePlayer) Machine() *StateMachinePlayer { return n.machine }

// Player returns the node's clip player: the animation node's own, or the
// machine's current one — for a scrub bar either way. May be nil.
func (n *SceneNodePlayer) Player() *Player {
	if n.machine != nil {
		return n.machine.Player()
	}
	return n.player
}

// Fire raises an event input on a machine node, a no-op elsewhere.
func (n *SceneNodePlayer) Fire(input string) {
	if n.machine != nil {
		n.machine.Fire(input)
	}
}

// Set sets a value input of a machine node — a menu's HP bar is a machine
// node the game feeds with n.Set("hp", v). A no-op elsewhere.
func (n *SceneNodePlayer) Set[T InputValue](name string, v T) {
	if n.machine != nil {
		n.machine.Set(name, v)
	}
}

// Get reads a value input of a machine node; false elsewhere.
func (n *SceneNodePlayer) Get[T InputValue](name string) (T, bool) {
	if n.machine != nil {
		return n.machine.Get[T](name)
	}
	var zero T
	return zero, false
}

// Started reports whether the node's entrance has happened: its Start
// time came, or an action forced it. An editor uses this to keep
// unentered nodes arrangeable.
func (n *SceneNodePlayer) Started() bool { return n.started }

// SetText overwrites a text node's content — a score, a nickname — and
// is a no-op elsewhere. The value belongs to the game: it survives
// Restart and phase switches.
func (n *SceneNodePlayer) SetText(s string) {
	if n.def.Kind == SceneNodeText {
		n.textVal = s
	}
}

// Text is a text node's current content, "" elsewhere.
func (n *SceneNodePlayer) Text() string { return n.textVal }

// SetVisible shows or hides the node. A hidden node neither draws nor
// takes pointer hits or focus.
func (n *SceneNodePlayer) SetVisible(v bool) { n.visible = v }

// Visible reports whether the node draws.
func (n *SceneNodePlayer) Visible() bool { return n.visible }

// Transform returns the node's current transform, which starts as the
// document's and moves with SetTransform.
func (n *SceneNodePlayer) Transform() SceneTransform { return n.tf }

// SetTransform overrides the node's placement at runtime — sliding a menu
// in, say. The document is not modified.
func (n *SceneNodePlayer) SetTransform(t SceneTransform) { n.tf = t }

// textSize measures the text block, cached until the content changes.
func (n *SceneNodePlayer) textSize() (w, h float64) {
	if n.face == nil {
		return 0, 0
	}
	if n.measFor != n.textVal {
		n.measW, n.measH = text.Measure(n.textVal, n.face, n.def.Text.lineSpacing())
		n.measFor = n.textVal
	}
	return n.measW, n.measH
}

// localRect is the node's hit box in its own coordinates. Animations,
// machines, and images span (0,0)-(w,h); a text block shifts by its
// anchor so the transform's X, Y place the anchored point.
func (n *SceneNodePlayer) localRect() (x0, y0, w, h float64) {
	switch n.def.Kind {
	case SceneNodeImage:
		if n.img != nil {
			b := n.img.Bounds()
			return 0, 0, float64(b.Dx()), float64(b.Dy())
		}
	case SceneNodeText:
		w, h := n.textSize()
		ax, ay := n.def.Text.anchorFractions()
		return -ax * w, -ay * h, w, h
	default:
		if p := n.Player(); p != nil {
			iw, ih := p.Animation().Size()
			return 0, 0, float64(iw), float64(ih)
		}
	}
	return 0, 0, 0, 0
}

// LocalRect is the node's hit box in its own coordinates, before the
// transform — for an editor drawing selection outlines.
func (n *SceneNodePlayer) LocalRect() (x, y, w, h float64) { return n.localRect() }

// viewGeoM is the node's local-to-view transform: its own placement, then
// the camera at its depth — the same chain Draw composes before the screen
// mapping, so hit tests and focus geometry match what is on screen.
func (n *SceneNodePlayer) viewGeoM() ebiten.GeoM {
	var g ebiten.GeoM
	g.Scale(n.tf.scaleX(), n.tf.scaleY())
	g.Rotate(n.tf.Rotation * math.Pi / 180)
	g.Translate(n.tf.X, n.tf.Y)
	if !n.sp.cam.isIdentity() {
		g.Concat(n.sp.cameraGeoM(n.def.ParallaxDepth()))
	}
	return g
}

// center is the node's center in view coordinates (scene coordinates as
// the camera shows them).
func (n *SceneNodePlayer) center() (x, y float64) {
	x0, y0, w, h := n.localRect()
	g := n.viewGeoM()
	return g.Apply(x0+w/2, y0+h/2)
}

// contains reports whether the view-space point falls in the node's hit
// region: the transformed local box (requirement's box-first rule).
func (n *SceneNodePlayer) contains(x, y float64) bool {
	x0, y0, w, h := n.localRect()
	if w <= 0 || h <= 0 {
		return false
	}
	g := n.viewGeoM()
	if !g.IsInvertible() {
		return false
	}
	g.Invert()
	lx, ly := g.Apply(x, y)
	return lx >= x0 && lx < x0+w && ly >= y0 && ly < y0+h
}

// --- focus ---

func (n *SceneNodePlayer) focusable() bool {
	return n.visible && n.started && n.inPhase() && n.def.Focus.Focusable
}

// initialFocusNode picks the node focused at start: the named one, or the
// focusable node with the lowest tab index.
func (sp *ScenePlayer) initialFocusNode() (*SceneNodePlayer, bool) {
	if name := sp.scene.Options.InitialFocus; name != "" {
		if n, ok := sp.byName[name]; ok && n.focusable() {
			return n, true
		}
	}
	order := sp.tabOrder()
	if len(order) == 0 {
		return nil, false
	}
	return order[0], true
}

// tabOrder is every focusable node sorted by tab index, ties by document
// order.
func (sp *ScenePlayer) tabOrder() []*SceneNodePlayer {
	var out []*SceneNodePlayer
	for _, n := range sp.nodes {
		if n.focusable() {
			out = append(out, n)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].def.Focus.TabIndex < out[j].def.Focus.TabIndex
	})
	return out
}

// Focused returns the focused node's name, or "".
func (sp *ScenePlayer) Focused() string {
	if sp.focused == nil {
		return ""
	}
	return sp.focused.def.Name
}

// Focus moves focus to the named node, reporting false when it does not
// exist or is not focusable.
func (sp *ScenePlayer) Focus(name string) bool {
	n, ok := sp.byName[name]
	if !ok || !n.focusable() {
		return false
	}
	sp.changeFocus(n)
	return true
}

// changeFocus fires blur, moves, fires focus, then notifies.
func (sp *ScenePlayer) changeFocus(to *SceneNodePlayer) {
	if to == sp.focused {
		return
	}
	from := sp.focused
	fromName := ""
	if from != nil {
		fromName = from.def.Name
		sp.deliver(from, SceneBlurEvent)
	}
	sp.focused = to
	toName := ""
	if to != nil {
		toName = to.def.Name
		sp.deliver(to, SceneFocusEvent)
	}
	if sp.onFocusChanged != nil {
		sp.onFocusChanged(fromName, toName)
	}
}

// MoveFocus moves focus the way a d-pad, cursor keys, or Tab would.
// FocusNext and FocusPrev walk tab order, wrapping. A direction follows
// the node's explicit neighbor link when set, else falls back to the
// geometrically nearest focusable node whose center lies in that
// direction's cone; with no candidate, focus stays.
func (sp *ScenePlayer) MoveFocus(dir FocusDirection) {
	order := sp.tabOrder()
	if len(order) == 0 {
		return
	}
	if sp.focused == nil || !sp.focused.focusable() {
		sp.changeFocus(order[0])
		return
	}
	switch dir {
	case FocusNext, FocusPrev:
		i := 0
		for j, n := range order {
			if n == sp.focused {
				i = j
				break
			}
		}
		if dir == FocusNext {
			i = (i + 1) % len(order)
		} else {
			i = (i + len(order) - 1) % len(order)
		}
		sp.changeFocus(order[i])
	default:
		if name := sp.focused.def.Focus.Neighbors.dir(dir); name != "" {
			if n, ok := sp.byName[name]; ok && n.focusable() {
				sp.changeFocus(n)
			}
			return // an explicit link never falls back
		}
		if n, ok := sp.geometricNeighbor(dir); ok {
			sp.changeFocus(n)
		}
	}
}

func (nb SceneNeighbors) dir(d FocusDirection) string {
	switch d {
	case FocusUp:
		return nb.Up
	case FocusDown:
		return nb.Down
	case FocusLeft:
		return nb.Left
	case FocusRight:
		return nb.Right
	}
	return ""
}

// geometricNeighbor picks the nearest focusable node whose center lies in
// the direction's cone: the main-axis distance must exceed the cross-axis
// one, which keeps a grid from skipping diagonally.
func (sp *ScenePlayer) geometricNeighbor(dir FocusDirection) (*SceneNodePlayer, bool) {
	cx, cy := sp.focused.center()
	var best *SceneNodePlayer
	bestDist := math.Inf(1)
	for _, n := range sp.nodes {
		if n == sp.focused || !n.focusable() {
			continue
		}
		x, y := n.center()
		dx, dy := x-cx, y-cy
		var main, cross float64
		switch dir {
		case FocusUp:
			main, cross = -dy, dx
		case FocusDown:
			main, cross = dy, dx
		case FocusLeft:
			main, cross = -dx, dy
		case FocusRight:
			main, cross = dx, dy
		}
		if main <= 0 || math.Abs(cross) > main {
			continue
		}
		if d := dx*dx + dy*dy; d < bestDist {
			best, bestDist = n, d
		}
	}
	return best, best != nil
}

// --- input ---

// Activate delivers the confirm button to the focused node.
func (sp *ScenePlayer) Activate() {
	if sp.focused != nil {
		sp.deliver(sp.focused, SceneActivate)
	}
}

// Cancel delivers the cancel button to the focused node. When no node
// handles it — nothing focused, or the node has neither a cancel binding
// nor a machine event named cancel — it surfaces as OnCallback("",
// "cancel") so a menu can still close.
func (sp *ScenePlayer) Cancel() {
	if sp.focused != nil && sp.deliver(sp.focused, SceneCancel) {
		return
	}
	if sp.onCallback != nil {
		sp.onCallback("", string(SceneCancel))
	}
}

// Pointer feeds the mouse or touch state, in raw screen coordinates —
// the current mapping is inverted internally. Call it every frame the
// pointer is in play; hover, press, and activate derive from changes
// between calls. Press on a focusable node moves focus to it; hovering
// moves focus when the scene's hoverMovesFocus option allows.
func (sp *ScenePlayer) Pointer(x, y float64, pressed bool) {
	g := sp.ScreenGeoM()
	if g.IsInvertible() {
		g.Invert()
		x, y = g.Apply(x, y)
	}
	hit := sp.nodeAt(x, y)

	if hit != sp.hovered {
		if sp.hovered != nil {
			sp.deliver(sp.hovered, SceneUnhover)
		}
		sp.hovered = hit
		if hit != nil {
			sp.deliver(hit, SceneHover)
			if sp.scene.Options.hoverMovesFocus() && hit.focusable() {
				sp.changeFocus(hit)
			}
		}
	}

	switch {
	case pressed && !sp.pointerDown:
		sp.pressed = hit
		if hit != nil {
			if hit.focusable() {
				sp.changeFocus(hit)
			}
			sp.deliver(hit, ScenePress)
		}
	case !pressed && sp.pointerDown:
		if sp.pressed != nil && sp.pressed == hit {
			sp.deliver(hit, SceneActivate)
		}
		sp.pressed = nil
	}
	sp.pointerDown = pressed
}

// NodeAt returns the topmost visible node containing the scene-space
// point, for an editor's picking; Pointer does the same test itself.
func (sp *ScenePlayer) NodeAt(x, y float64) (*SceneNodePlayer, bool) {
	n := sp.nodeAt(x, y)
	return n, n != nil
}

func (sp *ScenePlayer) nodeAt(x, y float64) *SceneNodePlayer {
	for i := len(sp.nodes) - 1; i >= 0; i-- {
		n := sp.nodes[i]
		if n.visible && n.started && n.inPhase() && n.contains(x, y) {
			return n
		}
	}
	return nil
}

// deliver runs the node's bindings for the event. With none bound, a
// machine node whose machine declares an Event input of the same name
// auto-fires it — a button's focus/press/activate states need no explicit
// wiring. Reports whether anything ran.
func (sp *ScenePlayer) deliver(n *SceneNodePlayer, ev SceneEvent) bool {
	// Focus and phase actions re-enter deliver; a binding cycle (two nodes
	// whose bindings focus each other, two phases that switch to each
	// other) must be cut, not crash the process — the machine player
	// bounds its transition chains the same way.
	if sp.bindingDepth >= maxBindingDepth {
		sp.note(fmt.Sprintf("node %q: binding chain stopped at depth %d; bindings likely form a cycle", n.def.Name, maxBindingDepth))
		return false
	}
	sp.bindingDepth++
	defer func() { sp.bindingDepth-- }()
	ran := false
	for _, b := range n.def.Bindings {
		if b.On != ev {
			continue
		}
		ran = true
		switch b.Do {
		case SceneFireEvent:
			tgt := sp.actionTarget(n, b)
			if tgt == nil {
				continue
			}
			if tgt.machine != nil {
				tgt.machine.Fire(b.Arg)
			} else {
				sp.note(fmt.Sprintf("node %q: fireEvent on animation node %q", n.def.Name, tgt.def.Name))
			}
		case ScenePlaySegment:
			tgt := sp.actionTarget(n, b)
			if tgt == nil {
				continue
			}
			switch {
			case tgt.player == nil:
				sp.note(fmt.Sprintf("node %q: playSegment on machine node %q", n.def.Name, tgt.def.Name))
			case b.Arg == "":
				// No marker named: play the whole clip from the top.
				tgt.player.SetRange(0, 0)
				tgt.player.Rewind()
				tgt.player.Play()
			case tgt.player.SetMarkerRange(b.Arg):
				tgt.player.Rewind()
				tgt.player.Play()
			default:
				sp.note(fmt.Sprintf("node %q: unknown marker %q", n.def.Name, b.Arg))
			}
		case SceneCallback:
			if sp.onCallback != nil {
				sp.onCallback(n.def.Name, b.Arg)
			}
		case SceneFocusAction:
			if !sp.Focus(b.Arg) {
				sp.note(fmt.Sprintf("node %q: cannot focus %q", n.def.Name, b.Arg))
			}
		case ScenePhaseAction:
			if !sp.SetPhase(b.Arg) {
				sp.note(fmt.Sprintf("node %q: unknown phase %q", n.def.Name, b.Arg))
			}
		default:
			sp.note(fmt.Sprintf("node %q: binding action %q", n.def.Name, b.Do))
		}
	}
	if ran {
		return true
	}
	if n.machine != nil && machineDeclaresEvent(n.machine.Definition(), string(ev)) {
		n.machine.Fire(string(ev))
		return true
	}
	return false
}

// actionTarget resolves which node a fireEvent or playSegment acts on:
// the named target, or the bound node itself. Acting on a node whose
// Start time has not come yet starts it — an event-driven entrance.
func (sp *ScenePlayer) actionTarget(n *SceneNodePlayer, b SceneBinding) *SceneNodePlayer {
	tgt := n
	if b.Target != "" {
		var ok bool
		if tgt, ok = sp.byName[b.Target]; !ok {
			sp.note(fmt.Sprintf("node %q: unknown binding target %q", n.def.Name, b.Target))
			return nil
		}
	}
	tgt.started = true
	return tgt
}

// machineDeclaresEvent reports whether the machine has an Event input of
// the given name, the condition for a default binding.
func machineDeclaresEvent(sm *StateMachine, name string) bool {
	for _, in := range sm.Inputs {
		if in.Type == InputEvent && in.Name == name {
			return true
		}
	}
	return false
}
