package lottie

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Scene is one composed screen — a game scene or a GUI menu — placing
// instances of animations and state machines from referenced bundles. It is
// a standalone JSON document (conventionally *.scene.json), not a bundle
// member: a scene composes actors from several bundles and no single bundle
// owns it.
//
// Members this package does not model are preserved and written back
// unchanged, the same way state machine documents round-trip.
type Scene struct {
	Name    string        `json:"name,omitempty"`
	Size    SceneSize     `json:"size"`
	Camera  SceneCamera   `json:"camera,omitzero"`
	Bundles []SceneBundle `json:"bundles,omitempty"`
	Images  []SceneAsset  `json:"images,omitempty"`
	Fonts   []SceneAsset  `json:"fonts,omitempty"`
	Phases  []ScenePhase  `json:"phases,omitempty"`
	Nodes   []SceneNode   `json:"nodes,omitempty"`
	Options SceneOptions  `json:"options,omitzero"`

	Extra ExtraFields `json:"-"`
}

// SceneAsset references one image or font file by a scene-local alias,
// the way SceneBundle references a bundle. Paths are relative to the
// scene file and load through the SceneLoader's File function.
type SceneAsset struct {
	Alias string `json:"alias"`
	Path  string `json:"path"`
}

// ScenePhase is one named stretch of the scene's life — an intro, the
// interactive screen, an outro. The first listed phase is where the scene
// starts; switching phases restarts the clock, so node Start times count
// from their phase's entry. A scene with no phases runs as one unnamed
// phase, which is where every scene was before phases existed.
type ScenePhase struct {
	Name string `json:"name"`
	// Duration ends the phase this many seconds in; 0 runs until switched
	// by an action or the host.
	Duration float64 `json:"duration,omitempty"`
	// Next is the phase entered when Duration elapses — an intro rolling
	// into the main screen. Empty stays put and reports OnPhaseEnd, so
	// the game can act on an outro finishing.
	Next string `json:"next,omitempty"`
	// Camera overrides the scene's camera while this phase runs; nil keeps
	// Scene.Camera.
	Camera *SceneCamera `json:"camera,omitempty"`
}

// SceneCamera is a 2D camera over the whole scene: X, Y move the camera in
// scene units (the content shifts the opposite way), Zoom magnifies around
// the design box's center, and Rotation tilts the camera in degrees
// clockwise (the content appears rotated the other way). Zoom resolves 0
// to 1, so the zero value is the identity camera.
//
// How strongly the camera moves each node is the node's Depth — the
// parallax factor. Entering a phase applies its camera; a game animates
// the camera by calling ScenePlayer.SetCamera each frame.
type SceneCamera struct {
	X        float64 `json:"x,omitempty"`
	Y        float64 `json:"y,omitempty"`
	Zoom     float64 `json:"zoom,omitempty"`
	Rotation float64 `json:"rotation,omitempty"`
}

// ZoomFactor returns Zoom, resolving an absent value to 1.
func (c SceneCamera) ZoomFactor() float64 {
	if c.Zoom == 0 {
		return 1
	}
	return c.Zoom
}

// isIdentity reports whether the camera leaves the scene untouched.
func (c SceneCamera) isIdentity() bool {
	return c.X == 0 && c.Y == 0 && c.ZoomFactor() == 1 && c.Rotation == 0
}

// CameraFor resolves the camera in effect for the named phase: the phase's
// own override, else the scene's. An unknown or empty name means the
// scene's.
func (s *Scene) CameraFor(phase string) SceneCamera {
	if p, ok := s.Phase(phase); ok && p.Camera != nil {
		return *p.Camera
	}
	return s.Camera
}

// Phase returns the named phase, or false.
func (s *Scene) Phase(name string) (*ScenePhase, bool) {
	for i := range s.Phases {
		if s.Phases[i].Name == name {
			return &s.Phases[i], true
		}
	}
	return nil, false
}

// SceneSize is the design resolution. Scene coordinates span this box;
// mapping it onto the real screen is the ScenePlayer's screen mapping.
type SceneSize struct {
	W int `json:"w"`
	H int `json:"h"`
}

// SceneBundle references one bundle by a scene-local alias. Path is
// relative to the scene file; the ScenePlayer's resolver turns it into a
// loaded Bundle.
type SceneBundle struct {
	Alias string `json:"alias"`
	Path  string `json:"path"`
}

// SceneOptions are scene-wide settings.
type SceneOptions struct {
	// HoverMovesFocus moves keyboard/pad focus to a focusable node the
	// pointer hovers, unifying mouse and pad into one focus state — the
	// usual game-menu behavior. Absent means true.
	HoverMovesFocus *bool `json:"hoverMovesFocus,omitempty"`
	// InitialFocus names the node focused when the scene starts. Empty
	// picks the focusable node with the lowest tab index.
	InitialFocus string `json:"initialFocus,omitempty"`
}

// hoverMovesFocus resolves the absent value to true.
func (o SceneOptions) hoverMovesFocus() bool {
	return o.HoverMovesFocus == nil || *o.HoverMovesFocus
}

// SceneNodeKind discriminates what a node plays.
type SceneNodeKind string

const (
	// SceneNodeAnimation plays one animation, optionally a marker segment.
	SceneNodeAnimation SceneNodeKind = "animation"
	// SceneNodeMachine runs a state machine of the source bundle.
	SceneNodeMachine SceneNodeKind = "machine"
	// SceneNodeImage shows one static image (Scene.Images).
	SceneNodeImage SceneNodeKind = "image"
	// SceneNodeText draws a text block whose content a game can overwrite
	// by node name at runtime.
	SceneNodeText SceneNodeKind = "text"
)

// SceneSource names what a node shows: an animation or machine id inside
// the bundle a scene-local alias refers to, or an image alias. Text nodes
// have no source; their content lives in SceneNode.Text.
type SceneSource struct {
	Bundle string `json:"bundle,omitempty"`
	ID     string `json:"id,omitempty"`
	Image  string `json:"image,omitempty"`
}

// SceneAlign is a horizontal alignment or anchor position.
type SceneAlign string

const (
	AlignLeft   SceneAlign = "left"
	AlignCenter SceneAlign = "center"
	AlignRight  SceneAlign = "right"
)

// SceneVAlign is a vertical anchor position.
type SceneVAlign string

const (
	AlignTop    SceneVAlign = "top"
	AlignMiddle SceneVAlign = "middle"
	AlignBottom SceneVAlign = "bottom"
)

// SceneText styles a text node. The anchor decides which point of the
// measured text block the node's transform positions, so a right-anchored
// score grows leftward; align sets how lines sit within the block.
type SceneText struct {
	// Value is the initial content; games overwrite it by node name with
	// SceneNodePlayer.SetText.
	Value string `json:"value,omitempty"`
	// Font is a Scene.Fonts alias.
	Font string `json:"font"`
	// Size is the font size in scene units; 0 resolves to 16.
	Size float64 `json:"size,omitempty"`
	// Align lays lines out inside the block; absent means left.
	Align SceneAlign `json:"align,omitempty"`
	// AnchorX / AnchorY pick the block point placed at the transform's
	// X, Y; absent means left / top.
	AnchorX SceneAlign  `json:"anchorX,omitempty"`
	AnchorY SceneVAlign `json:"anchorY,omitempty"`
	// Color is #rgb, #rrggbb, or #rrggbbaa; absent means white.
	Color string `json:"color,omitempty"`
	// LineHeight is a multiplier of Size; 0 resolves to 1.2.
	LineHeight float64 `json:"lineHeight,omitempty"`
}

func (t SceneText) size() float64 {
	if t.Size <= 0 {
		return 16
	}
	return t.Size
}

// lineSpacing is the pixel distance between line baselines.
func (t SceneText) lineSpacing() float64 {
	lh := t.LineHeight
	if lh <= 0 {
		lh = 1.2
	}
	return t.size() * lh
}

// anchorFractions turns the anchor into block-size fractions.
func (t SceneText) anchorFractions() (ax, ay float64) {
	switch t.AnchorX {
	case AlignCenter:
		ax = 0.5
	case AlignRight:
		ax = 1
	}
	switch t.AnchorY {
	case AlignMiddle:
		ay = 0.5
	case AlignBottom:
		ay = 1
	}
	return ax, ay
}

// SceneTransform places a node in scene coordinates. X, Y position the
// animation's top-left corner; Rotation is degrees clockwise (y-down).
//
// ScaleX, ScaleY, and Opacity resolve a zero value to 1, the way
// State.Speed does, so a hand-built zero-value transform still shows the
// node. Hide a node with SceneNodePlayer.SetVisible, not opacity 0.
type SceneTransform struct {
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	ScaleX   float64 `json:"scaleX,omitempty"`
	ScaleY   float64 `json:"scaleY,omitempty"`
	Rotation float64 `json:"rotation,omitempty"`
	Opacity  float64 `json:"opacity,omitempty"`
}

func (t SceneTransform) scaleX() float64 {
	if t.ScaleX == 0 {
		return 1
	}
	return t.ScaleX
}

func (t SceneTransform) scaleY() float64 {
	if t.ScaleY == 0 {
		return 1
	}
	return t.ScaleY
}

func (t SceneTransform) opacity() float64 {
	if t.Opacity == 0 {
		return 1
	}
	return t.Opacity
}

// ScenePlayback configures an animation node's playback, mirroring the
// playback fields of a state machine State. Speed resolves 0 to 1.
//
// Then chains what plays after this clip completes: the everyday case is
// an entrance clip played once rolling into an idle loop, without wiring
// a state machine for it. A step whose Loop is set parks the chain there.
type ScenePlayback struct {
	Segment   string   `json:"segment,omitempty"`
	Loop      bool     `json:"loop,omitempty"`
	LoopCount int      `json:"loopCount,omitempty"`
	Autoplay  bool     `json:"autoplay,omitempty"`
	Mode      PlayMode `json:"mode,omitempty"`
	Speed     float64  `json:"speed,omitempty"`

	Then []ScenePlayStep `json:"then,omitempty"`
}

// ScenePlayStep is one link of an animation node's chain: a clip played
// when the previous one completes. An empty Animation keeps playing the
// same clip's other segment.
type ScenePlayStep struct {
	Animation string   `json:"animation,omitempty"`
	Segment   string   `json:"segment,omitempty"`
	Loop      bool     `json:"loop,omitempty"`
	LoopCount int      `json:"loopCount,omitempty"`
	Mode      PlayMode `json:"mode,omitempty"`
	Speed     float64  `json:"speed,omitempty"`
}

// PlaybackSpeed returns Speed, resolving an absent value to 1.
func (s ScenePlayStep) PlaybackSpeed() float64 {
	if s.Speed == 0 {
		return 1
	}
	return s.Speed
}

// PlaybackSpeed returns Speed, resolving an absent value to 1.
func (p ScenePlayback) PlaybackSpeed() float64 {
	if p.Speed == 0 {
		return 1
	}
	return p.Speed
}

// PlaybackMode returns Mode, resolving an absent value to PlayForward.
func (p ScenePlayback) PlaybackMode() PlayMode {
	if p.Mode == "" {
		return PlayForward
	}
	return p.Mode
}

// SceneFocus makes a node a focus stop of the scene's menu navigation.
type SceneFocus struct {
	Focusable bool `json:"focusable,omitempty"`
	// TabIndex orders Next/Prev traversal; ties break by node order.
	TabIndex int `json:"tabIndex,omitempty"`
	// Neighbors override directional moves. An empty direction falls back
	// to the geometrically nearest focusable node in that direction.
	Neighbors SceneNeighbors `json:"neighbors,omitzero"`
}

// SceneNeighbors names the node each directional move goes to.
type SceneNeighbors struct {
	Up    string `json:"up,omitempty"`
	Down  string `json:"down,omitempty"`
	Left  string `json:"left,omitempty"`
	Right string `json:"right,omitempty"`
}

// SceneEvent is a semantic UI event a binding reacts to. Click and confirm
// button both mean SceneActivate; the source device never appears here.
type SceneEvent string

const (
	SceneFocusEvent SceneEvent = "focus"    // focus arrived
	SceneBlurEvent  SceneEvent = "blur"     // focus left
	SceneHover      SceneEvent = "hover"    // pointer moved onto the node
	SceneUnhover    SceneEvent = "unhover"  // pointer left the node
	ScenePress      SceneEvent = "press"    // pointer or confirm button went down
	SceneActivate   SceneEvent = "activate" // confirm completed: click released on the node, or confirm while focused
	SceneCancel     SceneEvent = "cancel"   // cancel button while focused
	SceneComplete   SceneEvent = "complete" // an animation node's clip finished a pass (each chain step reports)
)

// SceneEvents lists every binding event, in a stable order for editors.
func SceneEvents() []SceneEvent {
	return []SceneEvent{SceneFocusEvent, SceneBlurEvent, SceneHover, SceneUnhover, ScenePress, SceneActivate, SceneCancel, SceneComplete}
}

// SceneActionType is what a binding does when its event fires.
type SceneActionType string

const (
	// SceneFireEvent fires an input event on the node's machine.
	SceneFireEvent SceneActionType = "fireEvent"
	// ScenePlaySegment plays a marker segment of an animation node.
	ScenePlaySegment SceneActionType = "playSegment"
	// SceneCallback reports {node, name} to the game via OnCallback.
	SceneCallback SceneActionType = "callback"
	// SceneFocusAction moves focus to the node Arg names.
	SceneFocusAction SceneActionType = "focus"
	// ScenePhaseAction enters the phase Arg names — a start button ending
	// the intro, a quit button rolling the outro.
	ScenePhaseAction SceneActionType = "phase"
)

// SceneBinding maps one event to one action. A node may bind several
// actions to the same event — a visual reaction plus a callback, say.
//
// Target aims fireEvent and playSegment at another node by name; empty
// means the bound node itself. Acting on a node whose start time has not
// come yet starts it — an event-driven entrance.
type SceneBinding struct {
	On     SceneEvent      `json:"on"`
	Do     SceneActionType `json:"do"`
	Arg    string          `json:"arg,omitempty"`
	Target string          `json:"target,omitempty"`
}

// SceneNode is one placed instance. Several nodes may share a source and
// play independently. Node order is draw order: first is the back, so
// overlap is edited by reordering.
type SceneNode struct {
	Name      string         `json:"name"`
	Kind      SceneNodeKind  `json:"kind"`
	Source    SceneSource    `json:"source"`
	Transform SceneTransform `json:"transform,omitzero"`

	// Depth is the parallax factor: how strongly the scene camera moves
	// this node. nil resolves to 1 (tracks the camera fully); 0 pins the
	// node to the screen (a HUD); between 0 and 1 a background drifts
	// slower; above 1 a foreground leads. It is a pointer because 0 is a
	// meaningful value, unlike the transform's zero-resolves-to-1 fields.
	Depth *float64 `json:"depth,omitempty"`

	// Playback applies to animation nodes only.
	Playback ScenePlayback `json:"playback,omitzero"`
	// Entry overrides a machine node's initial state; empty keeps the
	// machine's own.
	Entry string `json:"entry,omitempty"`
	// Start delays the node's entrance: it neither draws, plays, nor takes
	// input until this many seconds into its phase, which is how an intro
	// choreographs one animation starting over another. Zero means from
	// the beginning.
	Start float64 `json:"start,omitempty"`
	// Phase names the phase this node belongs to; it participates only
	// while that phase runs, entering afresh each time the phase is
	// entered. Empty joins every phase and keeps playing across switches.
	Phase string `json:"phase,omitempty"`
	// Text styles a text node (kind text only).
	Text SceneText `json:"text,omitzero"`

	Focus    SceneFocus     `json:"focus,omitzero"`
	Bindings []SceneBinding `json:"bindings,omitempty"`

	Extra ExtraFields `json:"-"`
}

// ParallaxDepth returns Depth, resolving an absent value to 1.
func (n *SceneNode) ParallaxDepth() float64 {
	if n.Depth == nil {
		return 1
	}
	return *n.Depth
}

func (s Scene) MarshalJSON() ([]byte, error) {
	type alias Scene
	return encodeExtra(alias(s), s.Extra)
}

func (s *Scene) UnmarshalJSON(data []byte) error {
	type alias Scene
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := decodeExtra(data, a)
	if err != nil {
		return err
	}
	*s = Scene(a)
	s.Extra = extra
	return nil
}

func (n SceneNode) MarshalJSON() ([]byte, error) {
	type alias SceneNode
	return encodeExtra(alias(n), n.Extra)
}

func (n *SceneNode) UnmarshalJSON(data []byte) error {
	type alias SceneNode
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := decodeExtra(data, a)
	if err != nil {
		return err
	}
	*n = SceneNode(a)
	n.Extra = extra
	return nil
}

// ParseScene decodes a scene document.
func ParseScene(data []byte) (*Scene, error) {
	var s Scene
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("lottie: parsing scene: %w", err)
	}
	return &s, nil
}

// DecodeScene reads a scene document from r.
func DecodeScene(r io.Reader) (*Scene, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("lottie: reading scene: %w", err)
	}
	return ParseScene(data)
}

// Encode writes the scene as indented JSON, preserving unmodeled members.
func (s *Scene) Encode(w io.Writer) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

// Node returns the named node, or false. Names are unique per Validate.
func (s *Scene) Node(name string) (*SceneNode, bool) {
	for i := range s.Nodes {
		if s.Nodes[i].Name == name {
			return &s.Nodes[i], true
		}
	}
	return nil, false
}

// ParseSceneColor parses #rgb, #rrggbb, or #rrggbbaa.
func ParseSceneColor(s string) ([4]float64, bool) {
	hex := strings.TrimPrefix(s, "#")
	digit := func(c byte) (int, bool) {
		switch {
		case c >= '0' && c <= '9':
			return int(c - '0'), true
		case c >= 'a' && c <= 'f':
			return int(c-'a') + 10, true
		case c >= 'A' && c <= 'F':
			return int(c-'A') + 10, true
		}
		return 0, false
	}
	byteAt := func(i int) (float64, bool) {
		hi, ok1 := digit(hex[i])
		lo, ok2 := digit(hex[i+1])
		if !ok1 || !ok2 {
			return 0, false
		}
		return float64(hi*16+lo) / 255, true
	}
	switch len(hex) {
	case 3:
		var out [4]float64
		out[3] = 1
		for i := range 3 {
			v, ok := digit(hex[i])
			if !ok {
				return out, false
			}
			out[i] = float64(v*17) / 255
		}
		return out, true
	case 6, 8:
		out := [4]float64{0, 0, 0, 1}
		for i := 0; i < len(hex)/2; i++ {
			v, ok := byteAt(i * 2)
			if !ok {
				return out, false
			}
			out[i] = v
		}
		return out, true
	}
	return [4]float64{}, false
}

// Bundle returns the bundle reference with the given alias, or false.
func (s *Scene) Bundle(alias string) (*SceneBundle, bool) {
	for i := range s.Bundles {
		if s.Bundles[i].Alias == alias {
			return &s.Bundles[i], true
		}
	}
	return nil, false
}

// Validate reports structural problems. A broken bundle reference is a
// finding here, not a parse error, so an editor can open and repair a scene
// whose assets moved.
func (s *Scene) Validate() []error {
	var errs []error
	if s.Size.W <= 0 || s.Size.H <= 0 {
		errs = append(errs, fmt.Errorf("scene has no design size"))
	}
	// A negative zoom flips the scene, and combined with fractional depth
	// the parallax math has no real answer — reject it outright.
	if s.Camera.Zoom < 0 {
		errs = append(errs, fmt.Errorf("scene camera has negative zoom"))
	}
	for _, p := range s.Phases {
		if p.Camera != nil && p.Camera.Zoom < 0 {
			errs = append(errs, fmt.Errorf("phase %q camera has negative zoom", p.Name))
		}
	}
	aliases := map[string]bool{}
	for _, b := range s.Bundles {
		if b.Alias == "" {
			errs = append(errs, fmt.Errorf("bundle reference %q has no alias", b.Path))
			continue
		}
		if aliases[b.Alias] {
			errs = append(errs, fmt.Errorf("duplicate bundle alias %q", b.Alias))
		}
		aliases[b.Alias] = true
	}
	imageAliases := map[string]bool{}
	for _, a := range s.Images {
		if a.Alias == "" {
			errs = append(errs, fmt.Errorf("image reference %q has no alias", a.Path))
			continue
		}
		if imageAliases[a.Alias] {
			errs = append(errs, fmt.Errorf("duplicate image alias %q", a.Alias))
		}
		imageAliases[a.Alias] = true
	}
	fontAliases := map[string]bool{}
	for _, a := range s.Fonts {
		if a.Alias == "" {
			errs = append(errs, fmt.Errorf("font reference %q has no alias", a.Path))
			continue
		}
		if fontAliases[a.Alias] {
			errs = append(errs, fmt.Errorf("duplicate font alias %q", a.Alias))
		}
		fontAliases[a.Alias] = true
	}
	phases := map[string]bool{}
	for _, p := range s.Phases {
		if p.Name == "" {
			errs = append(errs, fmt.Errorf("a phase has no name"))
			continue
		}
		if phases[p.Name] {
			errs = append(errs, fmt.Errorf("duplicate phase name %q", p.Name))
		}
		phases[p.Name] = true
	}
	for _, p := range s.Phases {
		if p.Next != "" && !phases[p.Next] {
			errs = append(errs, fmt.Errorf("phase %q advances to unknown phase %q", p.Name, p.Next))
		}
		if p.Next != "" && p.Duration <= 0 {
			errs = append(errs, fmt.Errorf("phase %q names a next phase but has no duration", p.Name))
		}
	}
	names := map[string]bool{}
	kinds := map[string]SceneNodeKind{}
	for i := range s.Nodes {
		n := &s.Nodes[i]
		if n.Name == "" {
			errs = append(errs, fmt.Errorf("node %d has no name", i))
		} else if names[n.Name] {
			errs = append(errs, fmt.Errorf("duplicate node name %q", n.Name))
		}
		names[n.Name] = true
		kinds[n.Name] = n.Kind
		switch n.Kind {
		case SceneNodeAnimation, SceneNodeMachine:
			if !aliases[n.Source.Bundle] {
				errs = append(errs, fmt.Errorf("node %q references unknown bundle alias %q", n.Name, n.Source.Bundle))
			}
		case SceneNodeImage:
			if !imageAliases[n.Source.Image] {
				errs = append(errs, fmt.Errorf("node %q references unknown image alias %q", n.Name, n.Source.Image))
			}
		case SceneNodeText:
			if !fontAliases[n.Text.Font] {
				errs = append(errs, fmt.Errorf("node %q references unknown font alias %q", n.Name, n.Text.Font))
			}
			if n.Text.Color != "" {
				if _, ok := ParseSceneColor(n.Text.Color); !ok {
					errs = append(errs, fmt.Errorf("node %q has bad color %q", n.Name, n.Text.Color))
				}
			}
		default:
			errs = append(errs, fmt.Errorf("node %q has unknown kind %q", n.Name, n.Kind))
		}
		if n.Phase != "" && !phases[n.Phase] {
			errs = append(errs, fmt.Errorf("node %q belongs to unknown phase %q", n.Name, n.Phase))
		}
		// An endless loop never completes, so anything chained after it
		// can never play. A counted loop (LoopCount > 0) does complete.
		if n.Playback.Loop && n.Playback.LoopCount <= 0 && len(n.Playback.Then) > 0 {
			errs = append(errs, fmt.Errorf("node %q loops its first clip, so its chain never runs", n.Name))
		}
		for i, st := range n.Playback.Then {
			if st.Loop && st.LoopCount <= 0 && i != len(n.Playback.Then)-1 {
				errs = append(errs, fmt.Errorf("node %q chain step %d loops, so later steps never run", n.Name, i+1))
			}
		}
	}
	// Bindings check after every node is known, since a target may name a
	// node declared later.
	for i := range s.Nodes {
		n := &s.Nodes[i]
		for _, b := range n.Bindings {
			// The action applies to the target when one is named, else to
			// the bound node itself.
			tgtName, tgtKind := n.Name, n.Kind
			if b.Target != "" {
				if !names[b.Target] {
					errs = append(errs, fmt.Errorf("node %q binding target %q does not exist", n.Name, b.Target))
					continue
				}
				tgtName, tgtKind = b.Target, kinds[b.Target]
			}
			switch b.Do {
			case SceneFireEvent:
				if tgtKind != SceneNodeMachine {
					if tgtName == n.Name {
						errs = append(errs, fmt.Errorf("node %q binds fireEvent but is not a machine node", n.Name))
					} else {
						errs = append(errs, fmt.Errorf("node %q fireEvent target %q is not a machine node", n.Name, tgtName))
					}
				}
			case ScenePlaySegment:
				if tgtKind != SceneNodeAnimation {
					if tgtName == n.Name {
						errs = append(errs, fmt.Errorf("node %q binds playSegment but is not an animation node", n.Name))
					} else {
						errs = append(errs, fmt.Errorf("node %q playSegment target %q is not an animation node", n.Name, tgtName))
					}
				}
			case SceneCallback:
			case SceneFocusAction:
				if !names[b.Arg] {
					errs = append(errs, fmt.Errorf("node %q focus action names unknown node %q", n.Name, b.Arg))
				}
			case ScenePhaseAction:
				if !phases[b.Arg] {
					errs = append(errs, fmt.Errorf("node %q phase action names unknown phase %q", n.Name, b.Arg))
				}
			default:
				errs = append(errs, fmt.Errorf("node %q has unknown binding action %q", n.Name, b.Do))
			}
		}
	}
	check := func(node, kind, target string) {
		if target != "" && !names[target] {
			errs = append(errs, fmt.Errorf("node %q %s neighbor %q does not exist", node, kind, target))
		}
	}
	for i := range s.Nodes {
		n := &s.Nodes[i]
		check(n.Name, "up", n.Focus.Neighbors.Up)
		check(n.Name, "down", n.Focus.Neighbors.Down)
		check(n.Name, "left", n.Focus.Neighbors.Left)
		check(n.Name, "right", n.Focus.Neighbors.Right)
	}
	if f := s.Options.InitialFocus; f != "" && !names[f] {
		errs = append(errs, fmt.Errorf("initial focus %q does not exist", f))
	}
	return errs
}
