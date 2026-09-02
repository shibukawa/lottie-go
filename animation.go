package lottie

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// Animation is an immutable, decoded Lottie animation. It is safe to share
// one Animation across multiple Players.
type Animation struct {
	name        string
	width       int
	height      int
	frameRate   float64
	inPoint     float64
	outPoint    float64
	layers      []*layerNode // in file order (first = topmost)
	markers     []Marker
	unsupported map[string]struct{}

	fontResolver FontResolver

	// Retained for texture paints (texture.go): precomp layer trees by
	// asset id, image assets by refId with the images decoded so far, and
	// the resolver external files load through.
	comps       map[string][]*layerNode
	imageAssets map[string]rawAsset
	images      map[string]*ebiten.Image
	resolver    AssetResolver

	// Compiled at decode; see analyzeCompositing.
	snapshotOK      bool
	rootShaderBlend bool // a root layer blends by sampling the backdrop
	phaseNodes      int
	generation      int // bumped when mutable state (font resolver) changes
}

// Decode parses a Lottie JSON document. Image assets must be embedded as
// data URIs; use DecodeWithAssets to load external image files.
func Decode(r io.Reader) (*Animation, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("lottie: read: %w", err)
	}
	return decodeJSON(data, nil)
}

// DecodeWithAssets parses a Lottie JSON document, using resolver to load
// externally referenced image assets.
func DecodeWithAssets(r io.Reader, resolver AssetResolver) (*Animation, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("lottie: read: %w", err)
	}
	return decodeJSON(data, resolver)
}

// AssetResolver loads external asset files referenced by an animation, such
// as images stored next to the JSON or inside a dotLottie archive. dir and
// name come from the asset's "u" and "p" fields.
type AssetResolver func(dir, name string) ([]byte, error)

func decodeJSON(data []byte, resolver AssetResolver) (*Animation, error) {
	var raw rawAnimation
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("lottie: parse JSON: %w", err)
	}
	if raw.FR <= 0 {
		return nil, fmt.Errorf("lottie: invalid frame rate %v", raw.FR)
	}
	if len(raw.Layers) == 0 {
		return nil, fmt.Errorf("lottie: no layers")
	}
	a := &Animation{
		name:        raw.Name,
		width:       raw.W,
		height:      raw.H,
		frameRate:   raw.FR,
		inPoint:     raw.IP,
		outPoint:    raw.OP,
		markers:     buildMarkers(raw.Markers),
		unsupported: map[string]struct{}{},
	}
	b := &builder{
		anim:     a,
		assets:   map[string]*rawAsset{},
		comps:    map[string][]*layerNode{},
		images:   map[string]*ebiten.Image{},
		fonts:    map[string]rawFont{},
		resolver: resolver,
	}
	for i := range raw.Assets {
		as := &raw.Assets[i]
		if as.ID != "" {
			b.assets[as.ID] = as
		}
	}
	if raw.Fonts != nil {
		for _, f := range raw.Fonts.List {
			b.fonts[f.Name] = f
		}
	}
	a.layers = b.buildLayers(raw.Layers)
	a.comps = b.comps
	a.images = b.images
	a.resolver = resolver
	a.imageAssets = map[string]rawAsset{}
	for id, as := range b.assets {
		if as.Layers == nil {
			a.imageAssets[id] = *as
		}
	}
	a.analyzeCompositing()
	return a, nil
}

// analyzeCompositing derives the render-plan flags that depend only on file
// structure: which layers may batch their offscreen work through the scratch
// atlases, and whether the whole animation may be snapshotted while idle.
// Both are static properties of the layer tree; only geometry animates.
func (a *Animation) analyzeCompositing() {
	a.snapshotOK = true
	for _, l := range a.layers {
		// Root layers with a non-normal blend mode composite against
		// whatever the game drew beneath the animation, so flattening
		// them into a snapshot would change the result. Precomp children
		// already flatten into the precomp's offscreen today.
		if l.blend != 0 {
			a.snapshotOK = false
		}
		if blendNeedsShader(l.blend) {
			a.rootShaderBlend = true
		}
	}
	// Precomp layer lists are cached per asset and shared between the
	// layers referencing them, so guard against revisiting.
	seen := map[*layerNode]bool{}
	var walk func(layers []*layerNode)
	walk = func(layers []*layerNode) {
		for _, l := range layers {
			if seen[l] {
				continue
			}
			seen[l] = true
			// Text extent is only known after shaping, so a masked text
			// layer cannot be given an atlas region up front. Backdrop-
			// sampling blends and effects run extra passes on the flattened
			// content, which the shared atlas regions cannot host.
			l.phaseOK = (len(l.masks) > 0 || l.matteMode != 0 && l.matteSrc != nil) &&
				l.typ != 5 && !blendNeedsShader(l.blend) && len(l.effects) == 0
			if l.phaseOK {
				a.phaseNodes++
			}
			walk(l.comp)
		}
	}
	walk(a.layers)
}

// Size returns the composition size in pixels.
func (a *Animation) Size() (w, h int) { return a.width, a.height }

// Duration returns the animation length.
func (a *Animation) Duration() time.Duration {
	frames := a.outPoint - a.inPoint
	return time.Duration(frames / a.frameRate * float64(time.Second))
}

// Name returns the composition name, if any.
func (a *Animation) Name() string { return a.name }

// FrameRate returns frames per second.
func (a *Animation) FrameRate() float64 { return a.frameRate }

// UnsupportedFeatures lists features present in the file that this renderer
// skips. Rendering continues without them.
func (a *Animation) UnsupportedFeatures() []string {
	out := make([]string, 0, len(a.unsupported))
	for f := range a.unsupported {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

func (a *Animation) note(feature string) {
	a.unsupported[feature] = struct{}{}
}

// layerNode is the compiled form of one layer.
type layerNode struct {
	name      string
	typ       int
	ind       int
	parent    *layerNode
	ip, op    float64 // composition frames
	st        float64 // start time offset (composition frames)
	stretch   float64 // time stretch (sr)
	transform *transformTracks
	shapes    []*shapeNode // layer type 4
	hidden    bool

	// Precomposition (ty: 0).
	comp         []*layerNode
	compW, compH float64      // clip size
	timeRemap    *vectorTrack // tm: layer time -> comp time, in seconds

	// Image (ty: 2).
	img *ebiten.Image

	// Masks, track matte, blend, effects.
	effects   []effectNode
	masks     []maskNode
	matteMode int // 0 none; tt: 1 alpha, 2 alpha-inv, 3 luma, 4 luma-inv
	matteSrc  *layerNode
	matteOnly bool // td: matte source, not drawn directly
	blend     int  // bm
	phaseOK   bool // may use the scratch atlases (analyzeCompositing)

	// Text (ty: 5).
	text *textNode

	autoOrient bool // ao: orient along the position path
}

// maskNode is one compiled mask.
type maskNode struct {
	mode      byte // 'a' add, 's' subtract, 'i' intersect
	inverted  bool
	shape     *shapeTrack
	opacity   *vectorTrack
	expansion *vectorTrack // nil when absent; pixels, negative contracts
}

// localTime converts composition frame time to layer-local frames.
func (l *layerNode) localTime(f float64) float64 {
	return (f - l.st) / l.stretch
}

// compTime maps layer-local frames to the referenced composition's frames,
// applying time remap when present. The tm property yields seconds into the
// precomposition.
func (l *layerNode) compTime(lt, frameRate float64) float64 {
	if l.timeRemap == nil {
		return lt
	}
	return l.timeRemap.scalarAt(lt, 0) * frameRate
}

// transformTracks bundles the animatable transform properties.
type transformTracks struct {
	anchor   *vectorTrack
	position *vectorTrack // nil when split
	posX     *vectorTrack // split-position components
	posY     *vectorTrack
	scale    *vectorTrack // percent
	rotation *vectorTrack // degrees
	opacity  *vectorTrack // 0..100
	skew     *vectorTrack // degrees
	skewAxis *vectorTrack // degrees
}

// matrixAt computes the transform matrix at layer-local frame f.
func (t *transformTracks) matrixAt(f float64) matrix {
	return t.matrixAtOriented(f, false)
}

// positionAt evaluates the (possibly split) position property.
func (t *transformTracks) positionAt(f float64) (px, py float64) {
	if t.posX != nil {
		return t.posX.scalarAt(f, 0), t.posY.scalarAt(f, 0)
	}
	if t.position != nil {
		p := t.position.at(f, nil)
		if len(p) > 0 {
			px = p[0]
		}
		if len(p) > 1 {
			py = p[1]
		}
	}
	return px, py
}

// orientationAt returns the direction of position motion, for auto-orient.
func (t *transformTracks) orientationAt(f float64) float64 {
	const eps = 0.5 // frames
	x0, y0 := t.positionAt(f - eps)
	x1, y1 := t.positionAt(f + eps)
	dx, dy := x1-x0, y1-y0
	if dx == 0 && dy == 0 {
		return 0
	}
	return math.Atan2(dy, dx)
}

// matrixAtOriented computes the transform matrix, optionally rotating along
// the position path (auto-orient).
func (t *transformTracks) matrixAtOriented(f float64, autoOrient bool) matrix {
	m := identityMatrix
	if t == nil {
		return m
	}
	px, py := t.positionAt(f)
	m = m.translate(px, py)
	if autoOrient {
		if ang := t.orientationAt(f); ang != 0 {
			m = m.rotate(ang)
		}
	}
	if t.rotation != nil {
		if r := t.rotation.scalarAt(f, 0); r != 0 {
			m = m.rotate(r * math.Pi / 180)
		}
	}
	if t.skew != nil {
		if sk := t.skew.scalarAt(f, 0); sk != 0 {
			axis := 0.0
			if t.skewAxis != nil {
				axis = t.skewAxis.scalarAt(f, 0)
			}
			// Lottie skews along the axis by -sk degrees (AE convention).
			m = m.skew(-sk*math.Pi/180, axis*math.Pi/180)
		}
	}
	if t.scale != nil {
		s := t.scale.at(f, nil)
		sx, sy := 1.0, 1.0
		if len(s) > 0 {
			sx = s[0] / 100
			sy = sx
		}
		if len(s) > 1 {
			sy = s[1] / 100
		}
		if sx != 1 || sy != 1 {
			m = m.scale(sx, sy)
		}
	}
	if t.anchor != nil {
		a := t.anchor.at(f, nil)
		var ax, ay float64
		if len(a) > 0 {
			ax = a[0]
		}
		if len(a) > 1 {
			ay = a[1]
		}
		if ax != 0 || ay != 0 {
			m = m.translate(-ax, -ay)
		}
	}
	return m
}

// opacityAt returns the transform opacity in [0,1].
func (t *transformTracks) opacityAt(f float64) float64 {
	if t == nil || t.opacity == nil {
		return 1
	}
	return clamp01(t.opacity.scalarAt(f, 100) / 100)
}

// shapeNode is the compiled form of one shape item.
type shapeNode struct {
	kind string // gr, sh, rc, el, fl, st, tr
	name string
	// jsonIndex is the item's position in its authored it (or shapes)
	// array, hidden and skipped siblings included, so a ShapeRef computed
	// from the document resolves against this tree (texture.go).
	jsonIndex int

	children  []*shapeNode     // gr
	transform *transformTracks // gr's tr item

	shape *shapeTrack // sh

	pos       *vectorTrack // rc, el
	size      *vectorTrack // rc, el
	roundness *vectorTrack // rc

	color    *vectorTrack // fl, st
	colorDiv float64      // fl, st: 1, or 255 for 0..255-scaled colors
	alphaDiv float64      // fl, st: likewise for the 4th channel
	opacity  *vectorTrack // fl, st, gf, gs (0..100)
	width    *vectorTrack // st, gs
	fillRule int          // fl, gf: 1 nonzero, 2 evenodd
	lineCap  int          // st, gs
	lineJoin int          // st, gs
	miter    float64      // st, gs

	// Gradient (gf, gs).
	gradKind      int // 1 linear, 2 radial
	gradStart     *vectorTrack
	gradEnd       *vectorTrack
	gradStops     *vectorTrack
	gradStopCount int

	// Trim path (tm).
	trimStart  *vectorTrack
	trimEnd    *vectorTrack
	trimOffset *vectorTrack
	trimMode   int

	// Primitive direction (rc, el, sr): 3 reverses the contour, which
	// nonzero-rule fills rely on for holes.
	dir int

	// Polystar (sr).
	starType   int // 1 star, 2 polygon
	rotation   *vectorTrack
	points     *vectorTrack
	outerR     *vectorTrack
	innerR     *vectorTrack
	outerRound *vectorTrack
	innerRound *vectorTrack

	// Repeater (rp).
	copies    *vectorTrack
	offset    *vectorTrack
	repAnchor *vectorTrack
	repPos    *vectorTrack
	repScale  *vectorTrack
	repRot    *vectorTrack
	repSO     *vectorTrack
	repEO     *vectorTrack
	repMode   int // 1 copies above the original, 2 below (reverse stack)

	// Stroke dashes (st, gs).
	dashPattern []*vectorTrack // alternating dash/gap lengths
	dashOffset  *vectorTrack

	// Pucker/bloat (pb), zig-zag (zz), offset path (op).
	amount   *vectorTrack // pb amount (percent), zz amplitude, op distance
	zzFreq   *vectorTrack // ridges per segment
	zzPoints *vectorTrack // 1 corner, 2 smooth

	// Merge paths (mm): 2 add, 3 subtract, 5 exclude intersections.
	mergeMode int
}

// builder compiles the raw model into node trees, recording unsupported
// features on the Animation as it goes.
type builder struct {
	anim     *Animation
	assets   map[string]*rawAsset
	comps    map[string][]*layerNode // built precomp layer trees by asset id
	building map[string]bool         // cycle guard
	images   map[string]*ebiten.Image
	fonts    map[string]rawFont // font name -> family/style
	resolver AssetResolver
	depth    int
}

var layerTypeNames = map[int]string{
	6:  "audio layer",
	13: "camera layer",
}

func (b *builder) buildLayers(raws []rawLayer) []*layerNode {
	byIndex := map[int]*layerNode{}
	var nodes []*layerNode
	parents := map[*layerNode]int{}
	var matteRefs []matteRef
	// prevNode tracks the previously built node in file order, for implicit
	// track-matte source resolution (the layer above).
	var prevNode *layerNode
	for i := range raws {
		rl := &raws[i]
		switch rl.Type {
		case 0, 1, 2, 3, 4, 5: // precomp, solid, image, null, shape, text
		default:
			name := layerTypeNames[rl.Type]
			if name == "" {
				name = fmt.Sprintf("layer type %d", rl.Type)
			}
			b.anim.note(name)
			// The skipped layer was still "the layer above": an implicit
			// track matte below it must not silently matte against the
			// next supported layer further up.
			prevNode = nil
			continue
		}
		n := &layerNode{
			name:    rl.Name,
			typ:     rl.Type,
			ip:      rl.IP,
			op:      rl.OP,
			st:      rl.ST,
			stretch: 1,
			hidden:  rl.Hidden,
			blend:   rl.Blend,
		}
		if rl.SR != nil && *rl.SR != 0 {
			n.stretch = *rl.SR
		}
		if rl.Index != nil {
			n.ind = *rl.Index
			byIndex[*rl.Index] = n
		}
		if rl.Parent != nil {
			parents[n] = *rl.Parent
		}
		n.transform = b.buildTransform(&rl.KS)
		switch rl.Type {
		case 4:
			n.shapes = b.buildShapeItems(rl.Shapes)
		case 1:
			// A solid layer renders as a filled rectangle spanning
			// (0,0)-(sw,sh) in layer space.
			cr, cg, cb := parseHexColor(rl.SolidColor)
			n.typ = 4
			n.shapes = []*shapeNode{
				{
					kind:      "rc",
					pos:       staticTrack(rl.SolidW/2, rl.SolidH/2),
					size:      staticTrack(rl.SolidW, rl.SolidH),
					roundness: staticTrack(0),
				},
				{
					kind:     "fl",
					color:    staticTrack(cr, cg, cb, 1),
					opacity:  staticTrack(100),
					fillRule: 1,
				},
			}
		case 0:
			n.comp = b.buildComp(rl.RefID)
			n.compW, n.compH = rl.W, rl.H
			if rl.TM != nil {
				n.timeRemap = b.vectorProp(rl.TM, "time remap", []float64{0})
			}
		case 2:
			n.img = b.buildImage(rl.RefID)
		case 5:
			n.text = b.buildText(rl.Text)
			if n.text != nil {
				b.anim.note(textResolverNote)
			}
		}
		n.autoOrient = rl.AO != 0
		n.masks = b.buildMasks(rl.Masks)
		n.effects = b.buildEffects(rl.Effects)
		if rl.Matte != nil && *rl.Matte != 0 {
			mode := *rl.Matte
			if mode >= 1 && mode <= 4 {
				n.matteMode = mode
			} else {
				b.anim.note(fmt.Sprintf("track matte mode %d", mode))
			}
		}
		n.matteOnly = rl.MatteSource == 1
		if n.matteMode != 0 {
			if rl.MatteParent != nil {
				// Resolved after all layers are built (forward refs).
				matteRefs = append(matteRefs, matteRef{n, *rl.MatteParent})
			} else if prevNode != nil {
				n.matteSrc = prevNode
			} else {
				b.anim.note("track matte without source")
				n.matteMode = 0
			}
		}
		b.noteLayerExtras(rl)
		nodes = append(nodes, n)
		prevNode = n
	}
	for n, pi := range parents {
		if p, ok := byIndex[pi]; ok && p != n {
			n.parent = p
		}
	}
	for _, mr := range matteRefs {
		if src, ok := byIndex[mr.tp]; ok && src != mr.layer {
			mr.layer.matteSrc = src
		} else {
			mr.layer.matteMode = 0
			b.anim.note("track matte without source")
		}
	}
	return nodes
}

type matteRef struct {
	layer *layerNode
	tp    int
}

// buildComp compiles the layer tree of a precomposition asset, caching per
// asset id and guarding against reference cycles.
func (b *builder) buildComp(refID string) []*layerNode {
	if refID == "" {
		b.anim.note("precomposition without refId")
		return nil
	}
	if comp, ok := b.comps[refID]; ok {
		return comp
	}
	as, ok := b.assets[refID]
	if !ok || as.Layers == nil {
		b.anim.note(fmt.Sprintf("missing precomposition asset %q", refID))
		return nil
	}
	if b.building == nil {
		b.building = map[string]bool{}
	}
	if b.building[refID] || b.depth > 32 {
		b.anim.note("precomposition reference cycle")
		return nil
	}
	b.building[refID] = true
	b.depth++
	comp := b.buildLayers(as.Layers)
	b.depth--
	delete(b.building, refID)
	b.comps[refID] = comp
	return comp
}

// buildImage loads an image asset. Embedded data URIs decode directly;
// external files go through the resolver when one is configured.
func (b *builder) buildImage(refID string) *ebiten.Image {
	if img, ok := b.images[refID]; ok {
		return img
	}
	as, ok := b.assets[refID]
	if !ok {
		b.anim.note(fmt.Sprintf("missing image asset %q", refID))
		return nil
	}
	img, note := loadImageAsset(as, b.resolver)
	if note != "" {
		b.anim.note(note)
	}
	b.images[refID] = img
	return img
}

// loadImageAsset decodes one image asset. Embedded data URIs decode
// directly; external files go through the resolver when one is configured.
// A failure comes back as the note to record, with a nil image.
func loadImageAsset(as *rawAsset, resolver AssetResolver) (*ebiten.Image, string) {
	var data []byte
	switch {
	case strings.HasPrefix(as.FileName, "data:"):
		idx := strings.Index(as.FileName, "base64,")
		if idx < 0 {
			return nil, "image asset with non-base64 data URI"
		}
		raw, err := base64.StdEncoding.DecodeString(as.FileName[idx+len("base64,"):])
		if err != nil {
			return nil, "undecodable embedded image"
		}
		data = raw
	case resolver != nil:
		raw, err := resolver(as.Path, as.FileName)
		if err != nil {
			return nil, fmt.Sprintf("unresolvable image asset %q", as.FileName)
		}
		data = raw
	default:
		return nil, fmt.Sprintf("external image asset %q", as.FileName)
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "undecodable embedded image"
	}
	return ebiten.NewImageFromImage(src), ""
}

func (b *builder) buildMasks(raws []rawMask) []maskNode {
	var out []maskNode
	for i := range raws {
		rm := &raws[i]
		var mode byte
		switch rm.Mode {
		case "a":
			mode = 'a'
		case "s":
			mode = 's'
		case "i":
			mode = 'i'
		case "n", "":
			continue
		default:
			b.anim.note(fmt.Sprintf("mask mode %q", rm.Mode))
			continue
		}
		var expansion *vectorTrack
		if len(rm.Expansion) > 0 {
			var p rawProp
			if json.Unmarshal(rm.Expansion, &p) == nil && len(p.K) > 0 {
				expansion = b.vectorProp(&p, "mask expansion", []float64{0})
			}
		}
		tr, err := parseShapeProp(rm.Points)
		if err != nil {
			b.anim.note("unparsable mask shape")
			continue
		}
		out = append(out, maskNode{
			mode:      mode,
			inverted:  rm.Inverted,
			shape:     tr,
			opacity:   b.vectorProp(rm.Opacity, "mask opacity", []float64{100}),
			expansion: expansion,
		})
	}
	return out
}

func (b *builder) noteLayerExtras(rl *rawLayer) {
	switch {
	case rl.Blend == 0 || rl.Blend == 2 || rl.Blend == 16:
		// normal, screen, add: fixed-function blends
	case blendNeedsShader(rl.Blend):
		// multiply, overlay .. exclusion: composited through blendShader
	default:
		// 12-15 (hue, saturation, color, luminosity) and 17 (hard mix)
		b.anim.note(fmt.Sprintf("blend mode %d", rl.Blend))
	}
	if rl.DDD != 0 {
		b.anim.note("3d layer")
	}
}

func (b *builder) buildTransform(rt *rawTransform) *transformTracks {
	t := &transformTracks{}
	t.anchor = b.vectorProp(rt.A, "transform anchor", []float64{0, 0})
	if rt.P != nil {
		if px, py, ok := rt.P.splitPosition(); ok {
			t.posX = b.vectorProp(px, "transform position x", []float64{0})
			t.posY = b.vectorProp(py, "transform position y", []float64{0})
		} else {
			t.position = b.vectorProp(rt.P, "transform position", []float64{0, 0})
		}
	} else {
		t.position = staticTrack(0, 0)
	}
	t.scale = b.vectorProp(rt.S, "transform scale", []float64{100, 100})
	t.rotation = b.vectorProp(rt.R, "transform rotation", []float64{0})
	t.opacity = b.vectorProp(rt.O, "transform opacity", []float64{100})
	t.skew = b.vectorProp(rt.SK, "transform skew", []float64{0})
	t.skewAxis = b.vectorProp(rt.SA, "transform skew axis", []float64{0})
	return t
}

// vectorProp parses a property, downgrading parse failures to a recorded
// unsupported feature plus the default value so rendering can continue.
func (b *builder) vectorProp(p *rawProp, what string, def []float64) *vectorTrack {
	if p == nil {
		return &vectorTrack{static: def}
	}
	if p.hasExpression() {
		b.anim.note("expressions")
	}
	tr, err := parseVectorProp(p, def)
	if err != nil {
		b.anim.note(fmt.Sprintf("unparsable %s", what))
		return &vectorTrack{static: def}
	}
	return tr
}

var shapeTypeNames = map[string]string{
	"tw": "twist",
}

func (b *builder) buildShapeItems(items []rawShapeItem) []*shapeNode {
	var nodes []*shapeNode
	// Whatever a branch below appends while handling items[i] is stamped
	// with i at the top of the next round (and once past the end), so no
	// branch has to remember to (see shapeNode.jsonIndex).
	stamped := 0
	stamp := func(i int) {
		for _, n := range nodes[stamped:] {
			n.jsonIndex = i
		}
		stamped = len(nodes)
	}
	for i := 0; i <= len(items); i++ {
		stamp(i - 1)
		if i == len(items) {
			break
		}
		it := &items[i]
		if it.Hidden {
			continue
		}
		switch it.Type {
		case "gr":
			n := &shapeNode{kind: "gr", name: it.Name}
			n.children = b.buildShapeItems(it.Items)
			// The group's transform is carried by a "tr" item.
			for j := range it.Items {
				if it.Items[j].Type == "tr" {
					n.transform = b.buildShapeTransform(&it.Items[j])
					break
				}
			}
			nodes = append(nodes, n)
		case "sh":
			if it.KS == nil {
				continue
			}
			if it.KS.hasExpression() {
				b.anim.note("expressions")
			}
			tr, err := parseShapeProp(it.KS)
			if err != nil {
				b.anim.note("unparsable path shape")
				continue
			}
			nodes = append(nodes, &shapeNode{kind: "sh", name: it.Name, shape: tr})
		case "rc":
			nodes = append(nodes, &shapeNode{
				kind:      "rc",
				name:      it.Name,
				dir:       it.direction(),
				pos:       b.vectorProp(it.P, "rect position", []float64{0, 0}),
				size:      b.vectorProp(it.S, "rect size", []float64{0, 0}),
				roundness: b.vectorProp(it.R, "rect roundness", []float64{0}),
			})
		case "el":
			nodes = append(nodes, &shapeNode{
				kind: "el",
				name: it.Name,
				dir:  it.direction(),
				pos:  b.vectorProp(it.P, "ellipse position", []float64{0, 0}),
				size: b.vectorProp(it.S, "ellipse size", []float64{0, 0}),
			})
		case "fl":
			rule := 1
			if it.R != nil {
				if vals, err := numbers(it.R.K); err == nil && len(vals) > 0 {
					rule = int(vals[0])
				}
			}
			n := &shapeNode{
				kind:     "fl",
				name:     it.Name,
				color:    b.vectorProp(it.C, "fill color", []float64{0, 0, 0, 1}),
				opacity:  b.vectorProp(it.O, "fill opacity", []float64{100}),
				fillRule: rule,
			}
			n.colorDiv, n.alphaDiv = colorDivisors(n.color)
			nodes = append(nodes, n)
		case "st":
			n := &shapeNode{
				kind:     "st",
				name:     it.Name,
				color:    b.vectorProp(it.C, "stroke color", []float64{0, 0, 0, 1}),
				opacity:  b.vectorProp(it.O, "stroke opacity", []float64{100}),
				width:    b.vectorProp(it.W, "stroke width", []float64{1}),
				lineCap:  it.LineCap,
				lineJoin: it.LineJoin,
				miter:    it.MiterLimit,
			}
			n.colorDiv, n.alphaDiv = colorDivisors(n.color)
			b.buildDashes(n, it)
			nodes = append(nodes, n)
		case "gf", "gs":
			n := &shapeNode{
				kind:     it.Type,
				name:     it.Name,
				opacity:  b.vectorProp(it.O, "gradient opacity", []float64{100}),
				gradKind: it.T,
			}
			if n.gradKind != 2 {
				n.gradKind = 1
			}
			n.gradStart = b.vectorProp(it.S, "gradient start", []float64{0, 0})
			n.gradEnd = b.vectorProp(it.E, "gradient end", []float64{0, 0})
			if it.G != nil && it.G.K != nil {
				n.gradStopCount = it.G.Count
				n.gradStops = b.vectorProp(it.G.K, "gradient stops", nil)
			} else {
				b.anim.note("gradient without stops")
				continue
			}
			if it.H != nil {
				if v, err := numbers(it.H.K); err == nil && len(v) > 0 && v[0] != 0 {
					b.anim.note("radial gradient highlight")
				}
			}
			if it.Type == "gf" {
				rule := 1
				if it.R != nil {
					if vals, err := numbers(it.R.K); err == nil && len(vals) > 0 {
						rule = int(vals[0])
					}
				}
				n.fillRule = rule
			} else {
				n.width = b.vectorProp(it.W, "stroke width", []float64{1})
				n.lineCap = it.LineCap
				n.lineJoin = it.LineJoin
				n.miter = it.MiterLimit
				b.buildDashes(n, it)
			}
			nodes = append(nodes, n)
		case "tm":
			nodes = append(nodes, &shapeNode{
				kind:       "tm",
				name:       it.Name,
				trimStart:  b.vectorProp(it.S, "trim start", []float64{0}),
				trimEnd:    b.vectorProp(it.E, "trim end", []float64{100}),
				trimOffset: b.vectorProp(it.O, "trim offset", []float64{0}),
				trimMode:   it.M,
			})
		case "sr":
			st := it.StarType
			if st != 2 {
				st = 1
			}
			nodes = append(nodes, &shapeNode{
				kind:       "sr",
				name:       it.Name,
				dir:        it.direction(),
				starType:   st,
				pos:        b.vectorProp(it.P, "polystar position", []float64{0, 0}),
				rotation:   b.vectorProp(it.R, "polystar rotation", []float64{0}),
				points:     b.vectorProp(it.Points, "polystar points", []float64{5}),
				outerR:     b.vectorProp(it.OuterR, "polystar outer radius", []float64{0}),
				innerR:     b.vectorProp(it.InnerR, "polystar inner radius", []float64{0}),
				outerRound: b.vectorProp(it.OuterRound, "polystar outer roundness", []float64{0}),
				innerRound: b.vectorProp(it.InnerRound, "polystar inner roundness", []float64{0}),
			})
		case "rd":
			nodes = append(nodes, &shapeNode{
				kind:      "rd",
				name:      it.Name,
				roundness: b.vectorProp(it.R, "corner radius", []float64{0}),
			})
		case "pb":
			nodes = append(nodes, &shapeNode{
				kind:   "pb",
				name:   it.Name,
				amount: b.vectorProp(it.A, "pucker/bloat amount", []float64{0}),
			})
		case "zz":
			nodes = append(nodes, &shapeNode{
				kind:     "zz",
				name:     it.Name,
				amount:   b.vectorProp(it.S, "zig zag size", []float64{0}),
				zzFreq:   b.vectorProp(it.R, "zig zag ridges", []float64{1}),
				zzPoints: b.vectorProp(it.Points, "zig zag point type", []float64{1}),
			})
		case "op":
			nodes = append(nodes, &shapeNode{
				kind:   "op",
				name:   it.Name,
				amount: b.vectorProp(it.A, "offset path amount", []float64{0}),
			})
		case "rp":
			n := &shapeNode{
				kind:    "rp",
				name:    it.Name,
				copies:  b.vectorProp(it.C, "repeater copies", []float64{1}),
				offset:  b.vectorProp(it.O, "repeater offset", []float64{0}),
				repMode: it.M, // 1 stacks copies above the original, 2 below
			}
			tr := it.RepeaterT
			if tr == nil {
				tr = &rawRepeaterTransform{}
			}
			n.repAnchor = b.vectorProp(tr.A, "repeater anchor", []float64{0, 0})
			n.repPos = b.vectorProp(tr.P, "repeater position", []float64{0, 0})
			n.repScale = b.vectorProp(tr.S, "repeater scale", []float64{100, 100})
			n.repRot = b.vectorProp(tr.R, "repeater rotation", []float64{0})
			n.repSO = b.vectorProp(tr.SO, "repeater start opacity", []float64{100})
			n.repEO = b.vectorProp(tr.EO, "repeater end opacity", []float64{100})
			nodes = append(nodes, n)
		case "mm":
			// Mode 1 simply merges contours into one path, which matches
			// how collected geometry already feeds styles. Add, subtract
			// and exclude-intersections combine by winding rules; true
			// intersection would need path booleans.
			switch it.MM {
			case 0, 1:
			case 2, 3, 5:
				nodes = append(nodes, &shapeNode{kind: "mm", name: it.Name, mergeMode: it.MM})
			default:
				b.anim.note(fmt.Sprintf("merge paths mode %d", it.MM))
			}
		case "tr":
			// Handled by the enclosing group.
		case "":
			// Ignore malformed items.
		default:
			name := shapeTypeNames[it.Type]
			if name == "" {
				name = fmt.Sprintf("shape type %q", it.Type)
			}
			b.anim.note(name)
		}
	}
	return nodes
}

// parseHexColor parses "#rrggbb" (or "#rgb") into 0..1 components.
// Malformed input yields black.
func parseHexColor(s string) (r, g, b float64) {
	s = strings.TrimPrefix(s, "#")
	hex := func(c byte) int {
		switch {
		case c >= '0' && c <= '9':
			return int(c - '0')
		case c >= 'a' && c <= 'f':
			return int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			return int(c-'A') + 10
		}
		return 0
	}
	switch len(s) {
	case 6:
		r = float64(hex(s[0])*16+hex(s[1])) / 255
		g = float64(hex(s[2])*16+hex(s[3])) / 255
		b = float64(hex(s[4])*16+hex(s[5])) / 255
	case 3:
		r = float64(hex(s[0])*17) / 255
		g = float64(hex(s[1])*17) / 255
		b = float64(hex(s[2])*17) / 255
	}
	return r, g, b
}

func (b *builder) buildShapeTransform(it *rawShapeItem) *transformTracks {
	rt := rawTransform{A: it.A, P: it.P, S: it.S, R: it.R, O: it.O, SK: it.SK, SA: it.SA}
	return b.buildTransform(&rt)
}

// buildDashes parses a stroke's dash array into pattern tracks.
func (b *builder) buildDashes(n *shapeNode, it *rawShapeItem) {
	for _, d := range it.dashes() {
		dv := d.Value
		switch d.Name {
		case "o":
			n.dashOffset = b.vectorProp(dv, "dash offset", []float64{0})
		case "d", "g", "":
			n.dashPattern = append(n.dashPattern, b.vectorProp(dv, "dash length", []float64{0}))
		}
	}
	if len(n.dashPattern) > 0 && n.dashOffset == nil {
		n.dashOffset = staticTrack(0)
	}
}
