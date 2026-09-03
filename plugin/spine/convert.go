package lottiespine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"path"
	"sort"
	"strings"

	lottie "github.com/shibukawa/lottie-go"
	lottietexture "github.com/shibukawa/lottie-go/plugin/texture"

	_ "image/jpeg" // atlas pages and loose images may be JPEG
)

// MeshMode says how a Spine mesh becomes Lottie paths.
type MeshMode string

const (
	// MeshHull writes one path from the mesh's hull vertices; inner vertices
	// are dropped and the texture interpolates across a fan from the hull's
	// centroid. Right whenever the hull is star-shaped, which character
	// meshes almost always are, at a fifth of the size of MeshTriangles.
	// The default.
	MeshHull MeshMode = "hull"
	// MeshTriangles writes one three-vertex path per Spine triangle, so the
	// whole mesh — inner vertices and all — deforms exactly as Spine draws
	// it, for meshes whose inner vertices carry the deformation.
	MeshTriangles MeshMode = "triangles"
)

// Options tunes a conversion. The zero value bakes at 30 fps, scale 1, the
// default skin, the hull of each mesh, keys within a pixel of the motion,
// and no images.
type Options struct {
	// FPS is the sampling rate; every frame is a key, so it is also the
	// clip's frame rate.
	FPS float64
	// Scale multiplies every coordinate; Spine rigs are often authored at a
	// size the game then scales down.
	Scale float64
	// Skins are shown over the default skin, in order; a later skin's
	// attachments win where names clash.
	Skins []string
	Mesh  MeshMode
	// Bones adds a null layer per bone carrying its baked world transform,
	// for tools that want to attach things to the rig.
	Bones bool
	// Tolerance is how far, in composition pixels, a baked frame may stray
	// from the straight line between the keys that replace it before it is
	// kept as a key of its own; the same number serves as degrees and
	// percent for the bone layers. Zero means 1; negative keeps every frame
	// that is not exactly on that line.
	Tolerance float64
	// SkeletonBounds sizes the composition by the skeleton's declared
	// bounds alone. By default they are widened to whatever any animation
	// reaches, so nothing is ever clipped.
	SkeletonBounds bool
	// Atlas is the texture atlas the attachments were packed into; nil means
	// one image file per attachment, found through ReadImage.
	Atlas *Atlas
	// ReadPage returns the bytes of an atlas page image by the name the
	// atlas gives it.
	ReadPage func(name string) ([]byte, error)
	// ReadImage returns the bytes of a loose attachment image by the
	// attachment's path, without extension ("head" for images/head.png).
	ReadImage func(path string) ([]byte, error)
	// MachineID names a state machine to generate with one looping state
	// per animation and an event per animation to switch; empty skips it.
	MachineID string
}

// Result is what a conversion produced, ready to write into a bundle or an
// exploded directory.
type Result struct {
	// Clips maps animation id to Lottie JSON; ClipOrder lists the ids in
	// the skeleton's order.
	Clips     map[string][]byte
	ClipOrder []string
	// Docs maps animation id to its texture document.
	Docs map[string]*lottietexture.Doc
	// Images maps bundle image file name to encoded bytes.
	Images map[string][]byte
	// Machine is the generated state machine, stored under MachineID, or nil.
	Machine   *lottie.StateMachine
	MachineID string
	// Width and Height are the clips' composition size, shared by every
	// clip so a character keeps its place when the state machine switches;
	// OriginX and OriginY are where Spine's origin (usually the root bone,
	// on the floor) landed in that composition.
	Width, Height    float64
	OriginX, OriginY float64
	// Notes lists everything the importer skipped or approximated.
	Notes []string
}

// Bundle assembles the result into a dotLottie bundle.
func (r *Result) Bundle() (*lottie.Bundle, error) {
	b := lottie.NewBundle()
	for _, id := range r.ClipOrder {
		if err := b.SetAnimation(id, r.Clips[id]); err != nil {
			return nil, err
		}
		if doc := r.Docs[id]; !doc.Empty() {
			if err := lottietexture.Store(b, id, doc); err != nil {
				return nil, err
			}
		}
	}
	for name, data := range r.Images {
		b.SetImage(name, data)
	}
	if r.Machine != nil {
		if err := b.SetStateMachine(r.MachineID, r.Machine); err != nil {
			return nil, err
		}
	}
	return b, nil
}

type noteSet struct {
	seen  map[string]bool
	notes []string
}

func (n *noteSet) addf(format string, args ...any) {
	s := fmt.Sprintf(format, args...)
	if n.seen == nil {
		n.seen = map[string]bool{}
	}
	if n.seen[s] {
		return
	}
	n.seen[s] = true
	n.notes = append(n.notes, s)
}

// ---- resolved attachments ----

type weight struct {
	bone *bone
	x, y float64
	w    float64
}

// drawable is one region or mesh attachment resolved against the atlas and
// the skeleton: what a slot draws when it shows the attachment.
type drawable struct {
	slot    *slot
	name    string // the name the slot shows it under
	att     *Attachment
	kind    string // region | mesh
	texture string // image asset id / texture name
	color   [4]float64

	// Texture-space UV per vertex.
	uvs [][2]float64
	// Mesh: triangles, hull count, the geometry source (the parent for a
	// linked mesh) and, when weighted, the weights per vertex; deformName
	// is the attachment the deform timelines address.
	triangles  []int
	hull       int
	vertices   []float64
	weights    [][]weight
	deformName string
	// Region: the four corners in bone space.
	corners [4][2]float64
}

// converter carries one conversion's state.
type converter struct {
	sk    *Skeleton
	opts  Options
	notes *noteSet
	pose  *pose
	// drawables per slot index, keyed by attachment name.
	draw   []map[string]*drawable
	images map[string][]byte
	// assets lists the image assets every clip declares: id, file, size.
	assets    []imageAsset
	assetByID map[string]*imageAsset
	// missing texture names: paints keep the name for a runtime binding.
	missing map[string]bool
}

type imageAsset struct {
	id, file string
	w, h     int
	page     *AtlasPage
}

// Convert bakes a skeleton into Lottie clips: one per animation, each a
// shape layer per slot whose attachments are keyframed paths textured
// through lottie-go's texture extension.
func Convert(sk *Skeleton, opts Options) (*Result, error) {
	if opts.FPS <= 0 {
		opts.FPS = 30
	}
	if opts.Scale <= 0 {
		opts.Scale = 1
	}
	if opts.Mesh == "" {
		opts.Mesh = MeshHull
	}
	switch {
	case opts.Tolerance == 0:
		opts.Tolerance = 1
	case opts.Tolerance < 0:
		opts.Tolerance = exactTol
	}
	if opts.Mesh != MeshTriangles && opts.Mesh != MeshHull {
		return nil, fmt.Errorf("lottiespine: unknown mesh mode %q", opts.Mesh)
	}
	c := &converter{
		sk:        sk,
		opts:      opts,
		notes:     &noteSet{},
		images:    map[string][]byte{},
		assetByID: map[string]*imageAsset{},
		missing:   map[string]bool{},
	}
	skins := c.activeSkins()
	c.pose = newPose(sk, skins, c.notes)
	c.resolveAttachments(skins)

	names := make([]string, 0, len(sk.Animations))
	for name := range sk.Animations {
		names = append(names, name)
	}
	sort.Strings(names)
	var baked []*bakedClip
	if len(names) == 0 {
		c.notes.addf("no animations; the setup pose is written as clip \"setup\"")
		baked = append(baked, c.bake("setup", &Animation{}))
	}
	for _, name := range names {
		baked = append(baked, c.bake(name, sk.Animations[name]))
	}

	// One composition size for every clip: the skeleton's declared bounds
	// widened to whatever any animation reaches.
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	declared := sk.Info.Width > 0 && sk.Info.Height > 0
	if declared {
		minX, minY = sk.Info.X, sk.Info.Y
		maxX, maxY = sk.Info.X+sk.Info.Width, sk.Info.Y+sk.Info.Height
	}
	if opts.SkeletonBounds && !declared {
		c.notes.addf("skeleton declares no bounds; the animations' reach sizes the composition")
	}
	for _, bc := range baked {
		if opts.SkeletonBounds && declared {
			break
		}
		for _, fr := range bc.frames {
			for _, ds := range fr.draws {
				if !ds.visible {
					continue
				}
				for i := 0; i+1 < len(ds.verts); i += 2 {
					minX, maxX = math.Min(minX, ds.verts[i]), math.Max(maxX, ds.verts[i])
					minY, maxY = math.Min(minY, ds.verts[i+1]), math.Max(maxY, ds.verts[i+1])
				}
			}
		}
	}
	if minX > maxX {
		minX, minY, maxX, maxY = -50, -50, 50, 50
		c.notes.addf("nothing is drawn; composition size is a placeholder")
	}
	res := &Result{
		Clips:  map[string][]byte{},
		Docs:   map[string]*lottietexture.Doc{},
		Images: c.images,
		Width:  math.Ceil((maxX - minX) * opts.Scale),
		Height: math.Ceil((maxY - minY) * opts.Scale),
	}
	res.OriginX = round(-minX*opts.Scale, 2)
	res.OriginY = round(maxY*opts.Scale, 2)
	used := map[string]bool{}
	for _, bc := range baked {
		id := clipID(bc.name, used)
		clip, doc := c.emit(bc, id, minX, maxY, res.Width, res.Height)
		data, err := json.Marshal(clip)
		if err != nil {
			return nil, err
		}
		res.Clips[id] = data
		res.Docs[id] = doc
		res.ClipOrder = append(res.ClipOrder, id)
	}
	if opts.MachineID != "" {
		res.Machine = machine(res.ClipOrder)
		res.MachineID = opts.MachineID
	}
	res.Notes = c.notes.notes
	return res, nil
}

// clipID makes an animation name safe as a bundle id and file name.
func clipID(name string, used map[string]bool) string {
	id := strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '-'
		}
		return r
	}, strings.TrimSpace(name))
	if id == "" || id == "." || id == ".." {
		id = "clip"
	}
	base := id
	for n := 2; used[id]; n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	used[id] = true
	return id
}

// activeSkins is the default skin plus the requested ones.
func (c *converter) activeSkins() map[string]bool {
	have := map[string]bool{}
	for _, s := range c.sk.Skins {
		have[s.Name] = true
	}
	active := map[string]bool{"default": true}
	for _, name := range c.opts.Skins {
		if !have[name] {
			c.notes.addf("skin %q not in the skeleton; ignored", name)
			continue
		}
		active[name] = true
	}
	var others []string
	for _, s := range c.sk.Skins {
		if s.Name != "default" && !active[s.Name] {
			others = append(others, s.Name)
		}
	}
	if len(others) > 0 && len(c.opts.Skins) == 0 {
		c.notes.addf("skins not imported (pass one to show it over the default skin): %s", strings.Join(others, ", "))
	}
	return active
}

// resolveAttachments builds the drawable of every region and mesh
// attachment the active skins hold, later skins overriding earlier ones.
func (c *converter) resolveAttachments(active map[string]bool) {
	c.draw = make([]map[string]*drawable, len(c.pose.slots))
	for i := range c.draw {
		c.draw[i] = map[string]*drawable{}
	}
	skins := []Skin{}
	for _, s := range c.sk.Skins {
		if s.Name == "default" {
			skins = append(skins, s)
		}
	}
	for _, requested := range c.opts.Skins {
		for _, s := range c.sk.Skins {
			if s.Name == requested {
				skins = append(skins, s)
			}
		}
	}
	for _, skin := range skins {
		for slotName, atts := range skin.Attachments {
			s := c.pose.slotByNm[slotName]
			if s == nil {
				c.notes.addf("skin %q names unknown slot %q", skin.Name, slotName)
				continue
			}
			names := make([]string, 0, len(atts))
			for name := range atts {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				att := atts[name]
				if att == nil {
					continue
				}
				if d := c.resolve(s, name, att, &skin); d != nil {
					c.draw[s.index][name] = d
				}
			}
		}
	}
}

func (c *converter) resolve(s *slot, name string, att *Attachment, skin *Skin) *drawable {
	d := &drawable{slot: s, name: name, att: att, color: parseColor(att.Color), deformName: name}
	geom := att
	switch att.kind() {
	case "region":
		d.kind = "region"
	case "mesh":
		d.kind = "mesh"
	case "linkedmesh":
		d.kind = "mesh"
		parent := c.findMesh(s.data.Name, att.Parent, att.Skin, skin)
		if parent == nil {
			c.notes.addf("linked mesh %q in slot %q has no parent mesh %q; skipped", name, s.data.Name, att.Parent)
			return nil
		}
		geom = parent
		if boolOr(att.Timelines, boolOr(att.Deform, true)) {
			d.deformName = att.Parent
		}
	case "boundingbox", "point", "path":
		return nil // not drawn
	case "clipping":
		c.notes.addf("clipping attachment %q in slot %q ignored: nothing is clipped", name, s.data.Name)
		return nil
	default:
		c.notes.addf("attachment type %q (%q in slot %q) not supported", att.Type, name, s.data.Name)
		return nil
	}
	if len(att.Sequence) > 0 && string(att.Sequence) != "null" {
		c.notes.addf("attachment %q in slot %q has a sequence; only its first frame is used", name, s.data.Name)
	}
	imagePath := att.imagePath(name)
	region, page := c.regionFor(imagePath)
	d.texture = c.assetFor(imagePath, region, page, att)
	toTex := func(u, v float64) [2]float64 {
		if region != nil {
			u, v = region.pageUV(u, v)
		}
		return [2]float64{u, v}
	}
	if d.kind == "region" {
		u0, v0, u1, v1 := 0.0, 0.0, 1.0, 1.0
		if region != nil {
			u0, v0, u1, v1 = region.packedRect()
		}
		sx, sy := ptrOr(att.ScaleX, 1), ptrOr(att.ScaleY, 1)
		cos, sin := cosDeg(att.Rotation), sinDeg(att.Rotation)
		corner := func(u, v float64) [2]float64 {
			lx := (u - 0.5) * att.Width * sx
			ly := (0.5 - v) * att.Height * sy
			return [2]float64{lx*cos - ly*sin + att.X, lx*sin + ly*cos + att.Y}
		}
		for i, uv := range [4][2]float64{{u0, v0}, {u1, v0}, {u1, v1}, {u0, v1}} {
			d.corners[i] = corner(uv[0], uv[1])
			d.uvs = append(d.uvs, toTex(uv[0], uv[1]))
		}
		return d
	}
	n := len(geom.UVs) / 2
	if n < 3 || len(geom.Triangles) < 3 {
		c.notes.addf("mesh %q in slot %q has no triangles; skipped", name, s.data.Name)
		return nil
	}
	for i := 0; i < n; i++ {
		d.uvs = append(d.uvs, toTex(geom.UVs[2*i], geom.UVs[2*i+1]))
	}
	d.triangles = geom.Triangles
	d.hull = geom.Hull
	if d.hull <= 0 || d.hull > n {
		d.hull = n
	}
	if len(geom.Vertices) == 2*n {
		d.vertices = geom.Vertices
		return d
	}
	// Weighted: per vertex a bone count, then bone index, x, y, weight.
	verts := geom.Vertices
	d.weights = make([][]weight, n)
	pos := 0
	for i := 0; i < n; i++ {
		if pos >= len(verts) {
			c.notes.addf("mesh %q in slot %q has a short vertex array; skipped", name, s.data.Name)
			return nil
		}
		count := int(verts[pos])
		pos++
		for k := 0; k < count; k++ {
			if pos+4 > len(verts) {
				c.notes.addf("mesh %q in slot %q has a short vertex array; skipped", name, s.data.Name)
				return nil
			}
			bi := int(verts[pos])
			if bi < 0 || bi >= len(c.pose.bones) {
				c.notes.addf("mesh %q in slot %q weights unknown bone %d; skipped", name, s.data.Name, bi)
				return nil
			}
			d.weights[i] = append(d.weights[i], weight{bone: c.pose.bones[bi], x: verts[pos+1], y: verts[pos+2], w: verts[pos+3]})
			pos += 4
		}
	}
	return d
}

// findMesh locates a linked mesh's parent: the mesh of that name in the
// given skin (or the linked mesh's own skin) in the same slot.
func (c *converter) findMesh(slotName, parent, skinName string, own *Skin) *Attachment {
	try := func(s *Skin) *Attachment {
		if s == nil {
			return nil
		}
		if att := s.Attachments[slotName][parent]; att != nil && att.kind() == "mesh" {
			return att
		}
		return nil
	}
	if skinName != "" {
		for i := range c.sk.Skins {
			if c.sk.Skins[i].Name == skinName {
				if att := try(&c.sk.Skins[i]); att != nil {
					return att
				}
			}
		}
	}
	if att := try(own); att != nil {
		return att
	}
	for i := range c.sk.Skins {
		if att := try(&c.sk.Skins[i]); att != nil {
			return att
		}
	}
	return nil
}

// regionFor finds the atlas region of an attachment path, when there is
// an atlas.
func (c *converter) regionFor(imagePath string) (*AtlasRegion, *AtlasPage) {
	if c.opts.Atlas == nil {
		return nil, nil
	}
	r := c.opts.Atlas.Find(imagePath)
	if r == nil {
		c.notes.addf("atlas has no region %q; the paint keeps the name for a runtime texture", imagePath)
		return nil, nil
	}
	return r, r.Page
}

// assetFor registers the image an attachment paints with and returns the
// texture name the paint uses: the atlas page's id, or the attachment
// path. Images that cannot be read leave no asset, so the clip still
// validates and a game may bind the name at runtime.
func (c *converter) assetFor(imagePath string, region *AtlasRegion, page *AtlasPage, att *Attachment) string {
	if region != nil {
		id := strings.TrimSuffix(page.Name, path.Ext(page.Name))
		if _, ok := c.assetByID[id]; ok || c.missing[id] {
			return id
		}
		if c.opts.ReadPage == nil {
			c.missing[id] = true
			c.notes.addf("atlas page %q not loaded: no page reader", page.Name)
			return id
		}
		data, err := c.opts.ReadPage(page.Name)
		if err != nil {
			c.missing[id] = true
			c.notes.addf("atlas page %q not read (%v); paints keep the name for a runtime texture", page.Name, err)
			return id
		}
		w, h, data, err := prepareImage(data, page.PMA)
		if err != nil {
			c.missing[id] = true
			c.notes.addf("atlas page %q not decoded (%v)", page.Name, err)
			return id
		}
		if page.Width == 0 || page.Height == 0 {
			page.Width, page.Height = w, h
		}
		file := safeFile(path.Base(page.Name))
		c.images[file] = data
		c.addAsset(imageAsset{id: id, file: file, w: w, h: h, page: page})
		return id
	}
	if c.opts.Atlas != nil {
		// Named but not packed: a runtime texture.
		return imagePath
	}
	id := imagePath
	if _, ok := c.assetByID[id]; ok || c.missing[id] {
		return id
	}
	if c.opts.ReadImage == nil {
		c.missing[id] = true
		return id
	}
	data, err := c.opts.ReadImage(imagePath)
	if err != nil {
		c.missing[id] = true
		c.notes.addf("image %q not read (%v); the paint keeps the name for a runtime texture", imagePath, err)
		return id
	}
	w, h, data, err := prepareImage(data, false)
	if err != nil {
		c.missing[id] = true
		c.notes.addf("image %q not decoded (%v)", imagePath, err)
		return id
	}
	file := safeFile(strings.ReplaceAll(imagePath, "/", "_") + ".png")
	c.images[file] = data
	c.addAsset(imageAsset{id: id, file: file, w: w, h: h})
	return id
}

func (c *converter) addAsset(a imageAsset) {
	c.assets = append(c.assets, a)
	c.assetByID[a.id] = &c.assets[len(c.assets)-1]
}

func safeFile(name string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		return r
	}, name)
}

// prepareImage decodes an image for its size and, for a premultiplied
// atlas page, un-premultiplies it: Lottie image assets are straight alpha,
// and a PMA page loaded as one would darken every soft edge twice.
func prepareImage(data []byte, pma bool) (w, h int, out []byte, err error) {
	if !pma {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return 0, 0, nil, err
		}
		return cfg.Width, cfg.Height, data, nil
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, 0, nil, err
	}
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			// The PNG holds premultiplied values; read them as stored.
			var r, g, bl, a uint8
			switch s := src.(type) {
			case *image.NRGBA:
				p := s.NRGBAAt(x, y)
				r, g, bl, a = p.R, p.G, p.B, p.A
			default:
				pr, pg, pb, pa := src.At(x, y).RGBA()
				r, g, bl, a = uint8(pr>>8), uint8(pg>>8), uint8(pb>>8), uint8(pa>>8)
			}
			if a > 0 && a < 255 {
				un := func(v uint8) uint8 { return uint8(math.Min(255, math.Round(float64(v)*255/float64(a)))) }
				r, g, bl = un(r), un(g), un(bl)
			}
			dst.SetNRGBA(x-b.Min.X, y-b.Min.Y, color.NRGBA{r, g, bl, a})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return 0, 0, nil, err
	}
	return b.Dx(), b.Dy(), buf.Bytes(), nil
}

// ---- baking ----

type drawSample struct {
	verts   []float64 // world x, y per vertex, Spine coordinates
	visible bool
}

type frameSample struct {
	time   float64
	colors [][4]float64 // per slot: slot color
	draws  map[*drawable]*drawSample
	bones  [][6]float64 // per bone: a b c d worldX worldY
}

type bakedClip struct {
	name     string
	anim     *Animation
	frames   []*frameSample
	frameCnt int // op: the last sampled frame index
	events   []Key
	shown    map[*drawable]bool // drawables visible on some frame
}

// bake samples one animation at every frame.
func (c *converter) bake(name string, anim *Animation) *bakedClip {
	dur := c.pose.duration(anim)
	n := int(math.Round(dur * c.opts.FPS))
	if n < 1 {
		n = 1
	}
	bc := &bakedClip{name: name, anim: anim, frameCnt: n, events: anim.Events, shown: map[*drawable]bool{}}
	if len(anim.DrawOrder) > 0 {
		c.notes.addf("animation %q keys the draw order; the setup order is used throughout", name)
	}
	if len(anim.Path) > 0 || len(anim.Physics) > 0 {
		c.notes.addf("animation %q keys path or physics constraints; ignored", name)
	}
	for f := 0; f <= n; f++ {
		t := math.Min(float64(f)/c.opts.FPS, dur)
		c.pose.apply(anim, t)
		fs := &frameSample{time: t, draws: map[*drawable]*drawSample{}}
		for _, s := range c.pose.slots {
			fs.colors = append(fs.colors, s.color)
			for attName, d := range c.draw[s.index] {
				ds := &drawSample{verts: c.worldVertices(d), visible: s.attachment == attName}
				if ds.visible {
					bc.shown[d] = true
				}
				fs.draws[d] = ds
			}
		}
		if c.opts.Bones {
			for _, b := range c.pose.bones {
				fs.bones = append(fs.bones, [6]float64{b.a, b.b, b.c, b.d, b.worldX, b.worldY})
			}
		}
		bc.frames = append(bc.frames, fs)
	}
	return bc
}

// worldVertices computes a drawable's vertices in world space at the
// current pose, deform included.
func (c *converter) worldVertices(d *drawable) []float64 {
	if d.kind == "region" {
		out := make([]float64, 0, 8)
		for _, p := range d.corners {
			x, y := d.slot.bone.localToWorld(p[0], p[1])
			out = append(out, x, y)
		}
		return out
	}
	deform := c.pose.deforms[deformKey{d.slot.data.Name, d.deformName}]
	n := len(d.uvs)
	out := make([]float64, 0, 2*n)
	if d.weights == nil {
		for i := 0; i < n; i++ {
			x, y := d.vertices[2*i], d.vertices[2*i+1]
			if 2*i+1 < len(deform) {
				x += deform[2*i]
				y += deform[2*i+1]
			}
			wx, wy := d.slot.bone.localToWorld(x, y)
			out = append(out, wx, wy)
		}
		return out
	}
	di := 0
	for i := 0; i < n; i++ {
		var wx, wy float64
		for _, w := range d.weights[i] {
			x, y := w.x, w.y
			if di+1 < len(deform) {
				x += deform[di]
				y += deform[di+1]
			}
			di += 2
			bx, by := w.bone.localToWorld(x, y)
			wx += bx * w.w
			wy += by * w.w
		}
		out = append(out, wx, wy)
	}
	return out
}

// ---- emitting Lottie ----

type obj = map[string]any

func round(x float64, decimals int) float64 {
	p := math.Pow(10, float64(decimals))
	r := math.Round(x*p) / p
	if r == 0 {
		return 0 // no -0
	}
	return r
}

// track is one animated value sampled per frame, compressed to the keys a
// linear interpolation needs: a sample on the line between its neighbours
// is dropped, a hold frame keeps its value until the next key.
type track struct {
	values [][]float64
	hold   []bool
	// tol is how far a dropped sample may sit from the line between the
	// keys around it, in the values' own units.
	tol float64
}

// exactTol keeps every sample that is not exactly on the line.
const exactTol = 1e-4

func (t *track) static() bool {
	for _, v := range t.values[1:] {
		if !sameVec(v, t.values[0]) {
			return false
		}
	}
	return true
}

func sameVec(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Abs(a[i]-b[i]) > 1e-6 {
			return false
		}
	}
	return true
}

// keyFrames returns the indices of the samples that become keys: from
// each key the next is pushed as far as every sample in between stays
// within tol of the line joining them. A hold frame is always a key and
// so is the frame after it, since the value jumps there.
func (t *track) keyFrames() []int {
	n := len(t.values)
	keep := []int{0}
	for last := 0; last < n-1; {
		next := last + 1
		if t.hold == nil || !t.hold[last] {
			for next+1 < n && t.spanFits(last, next+1) {
				next++
			}
		}
		keep = append(keep, next)
		last = next
	}
	return keep
}

// spanFits reports whether keys at a and b alone reproduce every sample
// between them within tol, with no hold frame inside the span.
func (t *track) spanFits(a, b int) bool {
	va, vb := t.values[a], t.values[b]
	for j := a + 1; j < b; j++ {
		if t.hold != nil && t.hold[j] {
			return false
		}
		f := float64(j-a) / float64(b-a)
		for k, vj := range t.values[j] {
			if math.Abs(va[k]+(vb[k]-va[k])*f-vj) > t.tol {
				return false
			}
		}
	}
	return true
}

// property writes the track as a Lottie animated property; enc turns one
// sample into the property's value form. A path's keyframes hold their
// value in a one-element array while a static path holds it bare, which
// is what keyed is for.
func (t *track) property(enc func([]float64) any, keyed bool) obj {
	if t.static() {
		return obj{"a": 0, "k": enc(t.values[0])}
	}
	frames := t.keyFrames()
	keys := make([]obj, 0, len(frames))
	for j, f := range frames {
		v := enc(t.values[f])
		if keyed {
			v = []any{v}
		}
		k := obj{"t": float64(f), "s": v}
		switch {
		case j == len(frames)-1:
		case t.hold != nil && t.hold[f]:
			k["h"] = 1
		default:
			k["o"] = obj{"x": []float64{0.333}, "y": []float64{0.333}}
			k["i"] = obj{"x": []float64{0.667}, "y": []float64{0.667}}
		}
		keys = append(keys, k)
	}
	return obj{"a": 1, "k": keys}
}

func staticProp(v any) obj { return obj{"a": 0, "k": v} }

// emit writes one baked clip as Lottie JSON plus its texture document.
func (c *converter) emit(bc *bakedClip, id string, minX, maxY, w, h float64) (obj, *lottietexture.Doc) {
	scale := c.opts.Scale
	// Whole composition pixels: a baked vertex is a raster position, and
	// integers keep the JSON short and compress well.
	toLottie := func(x, y float64) (float64, float64) {
		return round((x-minX)*scale, 0), round((maxY-y)*scale, 0)
	}
	doc := &lottietexture.Doc{}
	var layers []obj
	nFrames := len(bc.frames)
	op := float64(bc.frameCnt)

	// Slots draw first to last in Spine; Lottie lists the top layer first.
	for si := len(c.pose.slots) - 1; si >= 0; si-- {
		s := c.pose.slots[si]
		var draws []*drawable
		for _, d := range c.draw[si] {
			if bc.shown[d] {
				draws = append(draws, d)
			}
		}
		if len(draws) == 0 {
			continue
		}
		sort.Slice(draws, func(i, j int) bool {
			// The setup attachment first, then by name.
			if a, b := draws[i].name == s.data.Attachment, draws[j].name == s.data.Attachment; a != b {
				return a
			}
			return draws[i].name < draws[j].name
		})
		layerInd := si + 1
		var groups []obj
		for gi, d := range draws {
			paths := c.pathsOf(d)
			var items []obj
			for pi, idx := range paths {
				tr := &track{tol: c.opts.Tolerance}
				for _, fr := range bc.frames {
					ds := fr.draws[d]
					v := make([]float64, 0, 2*len(idx))
					for _, vi := range idx {
						x, y := toLottie(ds.verts[2*vi], ds.verts[2*vi+1])
						v = append(v, x, y)
					}
					tr.values = append(tr.values, v)
				}
				items = append(items, obj{"ty": "sh", "nm": fmt.Sprintf("%s %d", d.name, pi), "ks": tr.property(pathValue, true)})
				uv := make([][2]float64, 0, len(idx))
				for _, vi := range idx {
					uv = append(uv, [2]float64{round(d.uvs[vi][0], 5), round(d.uvs[vi][1], 5)})
				}
				doc.UVs = append(doc.UVs, lottietexture.UV{Layer: layerInd, Item: []int{gi, pi}, V: uv})
			}
			// Fill: the slot color times the attachment color, hidden while
			// another attachment (or none) shows.
			col := &track{tol: exactTol}
			opa := &track{hold: make([]bool, nFrames), tol: exactTol}
			for fi, fr := range bc.frames {
				sc := fr.colors[si]
				col.values = append(col.values, []float64{
					round(sc[0]*d.color[0], 4), round(sc[1]*d.color[1], 4), round(sc[2]*d.color[2], 4), 1,
				})
				o := 0.0
				if fr.draws[d].visible {
					o = round(sc[3]*d.color[3]*100, 2)
				}
				opa.values = append(opa.values, []float64{o})
				if fi+1 < nFrames && fr.draws[d].visible != bc.frames[fi+1].draws[d].visible {
					opa.hold[fi] = true
				}
			}
			items = append(items,
				obj{"ty": "fl", "nm": d.name, "c": col.property(vecValue, false), "o": opa.property(scalarValue, false), "r": 1},
				transformItem())
			groups = append(groups, obj{"ty": "gr", "nm": d.name, "it": items})
			doc.Paints = append(doc.Paints, lottietexture.Paint{
				Layer: layerInd, Item: []int{gi, len(paths)},
				Texture: d.texture, Mapping: lottietexture.MappingVertex,
			})
		}
		layer := obj{
			"ty": 4, "nm": s.data.Name, "ind": layerInd, "ip": 0.0, "op": op, "st": 0.0, "sr": 1,
			"ks": identityTransform(), "shapes": groups,
		}
		if bm := blendMode(s.data.Blend); bm != 0 {
			layer["bm"] = bm
		}
		layers = append(layers, layer)
	}
	if c.opts.Bones {
		for bi, b := range c.pose.bones {
			tol := c.opts.Tolerance
			pos, rot, scl := &track{tol: tol}, &track{tol: tol}, &track{tol: tol}
			for _, fr := range bc.frames {
				m := fr.bones[bi]
				x, y := toLottie(m[4], m[5])
				pos.values = append(pos.values, []float64{x, y, 0})
				// Y flips, so a Spine angle turns the other way.
				rot.values = append(rot.values, []float64{round(-atan2Deg(m[2], m[0]), 2)})
				scl.values = append(scl.values, []float64{round(math.Hypot(m[0], m[2])*100, 2), round(math.Hypot(m[1], m[3])*100, 2), 100})
			}
			layers = append(layers, obj{
				"ty": 3, "nm": "bone:" + b.data.Name, "ind": len(c.pose.slots) + 1 + bi,
				"ip": 0.0, "op": op, "st": 0.0, "sr": 1,
				"ks": obj{
					"a": staticProp([]float64{0, 0, 0}), "p": pos.property(vecValue, false),
					"r": rot.property(scalarValue, false), "s": scl.property(vecValue, false), "o": staticProp(100.0),
				},
			})
		}
	}
	var assets []obj
	for _, a := range c.assets {
		assets = append(assets, obj{"id": a.id, "w": a.w, "h": a.h, "u": "", "p": a.file, "e": 0})
	}
	if assets == nil {
		assets = []obj{}
	}
	var markers []obj
	for _, ev := range bc.events {
		if ev.Name == nil {
			continue
		}
		markers = append(markers, obj{"cm": *ev.Name, "tm": round(ev.Time*c.opts.FPS, 2), "dr": 0})
	}
	clip := obj{
		"v": "5.9.0", "nm": bc.name, "fr": c.opts.FPS, "ip": 0.0, "op": op,
		"w": w, "h": h, "assets": assets, "layers": layers,
	}
	if markers != nil {
		clip["markers"] = markers
	}
	return clip, doc
}

// pathsOf lists the vertex index loops a drawable becomes: one quad for a
// region, one loop per triangle or the hull for a mesh.
func (c *converter) pathsOf(d *drawable) [][]int {
	if d.kind == "region" {
		return [][]int{{0, 1, 2, 3}}
	}
	if c.opts.Mesh == MeshHull {
		idx := make([]int, d.hull)
		for i := range idx {
			idx[i] = i
		}
		return [][]int{idx}
	}
	var out [][]int
	n := len(d.uvs)
	for i := 0; i+2 < len(d.triangles); i += 3 {
		a, b, cc := d.triangles[i], d.triangles[i+1], d.triangles[i+2]
		if a < 0 || b < 0 || cc < 0 || a >= n || b >= n || cc >= n {
			continue
		}
		out = append(out, []int{a, b, cc})
	}
	return out
}

func pathValue(v []float64) any {
	pts := make([][]float64, 0, len(v)/2)
	zeros := make([][]float64, 0, len(v)/2)
	for i := 0; i+1 < len(v); i += 2 {
		pts = append(pts, []float64{v[i], v[i+1]})
		zeros = append(zeros, []float64{0, 0})
	}
	return obj{"c": true, "v": pts, "i": zeros, "o": zeros}
}

func vecValue(v []float64) any    { return v }
func scalarValue(v []float64) any { return v[0] }

func identityTransform() obj {
	return obj{
		"a": staticProp([]float64{0, 0, 0}), "p": staticProp([]float64{0, 0, 0}),
		"s": staticProp([]float64{100, 100, 100}), "r": staticProp(0.0), "o": staticProp(100.0),
	}
}

func transformItem() obj {
	return obj{"ty": "tr", "p": staticProp([]float64{0, 0}), "a": staticProp([]float64{0, 0}),
		"s": staticProp([]float64{100, 100}), "r": staticProp(0.0), "o": staticProp(100.0)}
}

// blendMode maps a slot blend to a Lottie layer blend mode.
func blendMode(blend string) int {
	switch blend {
	case "additive":
		return 16
	case "multiply":
		return 1
	case "screen":
		return 2
	}
	return 0
}

// machine builds a state machine with a looping state per clip and an
// event per clip to switch to it from anywhere; the initial state is
// "idle" when there is one, else the first clip.
func machine(clips []string) *lottie.StateMachine {
	sm := &lottie.StateMachine{}
	initial := clips[0]
	var global lottie.State
	global.Name = "any"
	global.Type = lottie.StateGlobal
	for _, clip := range clips {
		if clip == "idle" {
			initial = clip
		}
		sm.Inputs = append(sm.Inputs, lottie.Input{Type: lottie.InputEvent, Name: clip})
		sm.States = append(sm.States, lottie.State{
			Name: clip, Type: lottie.StatePlayback, Animation: clip, Loop: true, Autoplay: true,
		})
		global.Transitions = append(global.Transitions, lottie.Transition{
			Type: lottie.TransitionImmediate, ToState: clip,
			Guards: []lottie.Guard{{Type: lottie.GuardEvent, InputName: clip}},
		})
	}
	sm.States = append(sm.States, global)
	sm.Initial = initial
	return sm
}
