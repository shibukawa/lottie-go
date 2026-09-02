package main

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/ncruces/zenity"
	lottie "github.com/shibukawa/lottie-go"
	lottietexture "github.com/shibukawa/lottie-go/plugin/texture"
)

// Textured fills and strokes (plugin/texture) are edited woven: while a
// clip is on stage its texture document is attached to the clip tree as
// x-tex members on fills and strokes and x-uv members on paths, so the
// shape tools — vertex insertion, reordering, copy and paste, clip
// duplication — carry the data without knowing it is there. Every
// store-back unweaves the tree into the bundle: the plain clip under a/,
// the document under extensions/texture/. Addresses are regenerated from
// the tree each time, never edited, so they cannot drift.

// texMemberKey matches an x- member used as a key, not a value named x-…,
// so a store-back that would leak a working member into a clip is refused.
var texMemberKey = regexp.MustCompile(`"x-[^"\\]*":`)

// weaveTextures attaches the clip's texture document to a freshly parsed
// clip. Entries whose item no longer exists, and the document's own
// unknown members, are kept aside so the store-back writes them back.
func (m *Model) weaveTextures(d *clipDoc) {
	if d == nil {
		return
	}
	if m.texUnplaced == nil {
		m.texUnplaced = map[string]*lottietexture.Doc{}
		m.texExtra = map[string]lottie.ExtraFields{}
	}
	delete(m.texUnplaced, d.id)
	delete(m.texExtra, d.id)
	doc, err := lottietexture.Load(m.bundle, d.id)
	if err != nil {
		m.setStatus("texture document of %q unreadable: %v", d.id, err)
		return
	}
	if doc == nil {
		return
	}
	if left := lottietexture.Weave(d.root, doc); left != nil {
		m.texUnplaced[d.id] = left
	}
	if len(doc.Extra) > 0 {
		m.texExtra[d.id] = doc.Extra
	}
}

// unweaveTextures strips the working members out of the tree into the
// document the bundle stores, the kept-aside entries and members included.
// The tree is plain Lottie afterwards; nil means there is nothing to store.
func (m *Model) unweaveTextures(d *clipDoc) *lottietexture.Doc {
	doc := lottietexture.Unweave(d.root)
	if left := m.texUnplaced[d.id]; left != nil {
		if doc == nil {
			doc = &lottietexture.Doc{}
		}
		doc.Append(left)
	}
	if extra := m.texExtra[d.id]; len(extra) > 0 {
		if doc == nil {
			doc = &lottietexture.Doc{}
		}
		doc.Extra = extra
	}
	return doc
}

// encodeClipForStore serializes the clip the way the bundle holds it: the
// pure Lottie bytes and the texture document beside them. The tree is
// woven again before returning, so editing continues where it was.
func (m *Model) encodeClipForStore(d *clipDoc) ([]byte, *lottietexture.Doc, error) {
	doc := m.unweaveTextures(d)
	data, err := d.encode()
	if doc != nil {
		// Only entries with a live item re-attach; the rest stay aside.
		lottietexture.Weave(d.root, doc)
	}
	if err != nil {
		return nil, nil, err
	}
	if texMemberKey.Match(data) {
		return nil, nil, fmt.Errorf("a working member survived the store-back")
	}
	return data, doc, nil
}

// storeClipDoc writes the clip and its texture document into the bundle.
func (m *Model) storeClipDoc(d *clipDoc) error {
	data, doc, err := m.encodeClipForStore(d)
	if err != nil {
		return err
	}
	if err := m.bundle.SetAnimation(d.id, data); err != nil {
		return err
	}
	if doc == nil || doc.Empty() && len(doc.Extra) == 0 {
		lottietexture.Remove(m.bundle, d.id)
		return nil
	}
	return lottietexture.Store(m.bundle, d.id, doc)
}

// applyTextures dresses a clip player with the bundle's texture document,
// the step every preview player takes after it is created.
func (m *Model) applyTextures(id string, p *lottie.Player) {
	doc, err := lottietexture.Load(m.bundle, id)
	if err != nil || doc == nil {
		return
	}
	if err := doc.Apply(p); err != nil {
		m.setStatus("texture: %v", err)
	}
}

// carryTextures moves or copies a clip's texture document along with the
// clip, the way its hitbox track travels.
func (m *Model) carryTextures(from, to string, move bool) {
	if t, err := lottietexture.Load(m.bundle, from); err == nil && t != nil {
		if err := lottietexture.Store(m.bundle, to, t); err != nil {
			m.setStatus("carried %q but not its textures: %v", from, err)
		}
	}
	if move {
		lottietexture.Remove(m.bundle, from)
		delete(m.texUnplaced, from)
		delete(m.texExtra, from)
	}
}

// UnplacedTextures lists the stage clip's document entries whose item no
// longer exists, so nothing vanishes silently: they are written back as
// they came until deleted here.
func (m *Model) UnplacedTextures() []string {
	d := m.StageClipDoc()
	if d == nil {
		return nil
	}
	left := m.texUnplaced[d.id]
	if left == nil {
		return nil
	}
	var out []string
	for _, p := range left.Paints {
		out = append(out, fmt.Sprintf("paint %s at %s", p.Texture, p.Ref()))
	}
	for _, u := range left.UVs {
		out = append(out, fmt.Sprintf("uv (%d) at %s", len(u.V), u.Ref()))
	}
	return out
}

// DropUnplacedTextures deletes the kept-aside entries of the stage clip.
func (m *Model) DropUnplacedTextures() {
	d := m.StageClipDoc()
	if d == nil || m.blockEdit() || m.texUnplaced[d.id] == nil {
		return
	}
	m.snapshotClip()
	delete(m.texUnplaced, d.id)
	m.touchClipDoc()
}

// ---- the selected fill or stroke's paint ----

// texTransformMembers are the placement transform's members, in the order
// the inspector shows them.
var texTransformMembers = []string{"p", "s", "r", "a"}

// ShapeCanTexture reports whether the selected item takes a texture.
func (m *Model) ShapeCanTexture() bool {
	n, ok := m.SelectedShapeNode()
	return ok && (n.ty == "fl" || n.ty == "st")
}

// ShapeTexture reads the selected fill or stroke's woven paint.
func (m *Model) ShapeTexture() (map[string]any, bool) {
	if !m.ShapeCanTexture() {
		return nil, false
	}
	item, ok := m.SelectedShapeItem()
	if !ok {
		return nil, false
	}
	tex, ok := item[lottietexture.MemberTex].(map[string]any)
	return tex, ok
}

// ShapeTextureName is the asset id the selected paint names ("" for none).
func (m *Model) ShapeTextureName() string {
	tex, ok := m.ShapeTexture()
	if !ok {
		return ""
	}
	s, _ := tex["texture"].(string)
	return s
}

// ShapeTextureString reads one of the paint's enumerated members —
// mapping, wrap, filter — with "" standing for the default.
func (m *Model) ShapeTextureString(key string) string {
	tex, ok := m.ShapeTexture()
	if !ok {
		return ""
	}
	s, _ := tex[key].(string)
	return s
}

// ShapeTextureTint reads the tint flag, true when absent.
func (m *Model) ShapeTextureTint() bool {
	tex, ok := m.ShapeTexture()
	if !ok {
		return true
	}
	if b, ok := tex["tint"].(bool); ok {
		return b
	}
	return true
}

// editTexture runs one write against the selected paint inside a clip
// edit; fn returns false when nothing changed.
func (m *Model) editTexture(fn func(item map[string]any) bool) {
	if m.blockEdit() || !m.ShapeCanTexture() {
		return
	}
	item, ok := m.SelectedShapeItem()
	if !ok {
		return
	}
	pushed := m.snapshotClip()
	if !fn(item) {
		if pushed {
			m.dropLastSnapshot()
		}
		return
	}
	m.touchClipDoc()
}

// SetShapeTexture binds an image asset to the selected fill or stroke;
// "" removes the paint and the item is a plain fill again.
func (m *Model) SetShapeTexture(assetID string) {
	m.editTexture(func(item map[string]any) bool {
		if assetID == "" {
			if _, has := item[lottietexture.MemberTex]; !has {
				return false
			}
			delete(item, lottietexture.MemberTex)
			return true
		}
		tex, ok := item[lottietexture.MemberTex].(map[string]any)
		if !ok {
			tex = map[string]any{}
			item[lottietexture.MemberTex] = tex
		}
		if cur, _ := tex["texture"].(string); cur == assetID {
			return false
		}
		tex["texture"] = assetID
		return true
	})
}

// SetShapeTextureString writes mapping, wrap or filter; "" restores the
// default by dropping the member. Switching to vertex mapping seeds a UV
// set on every sibling path that has none, so editing starts from the
// picture already on screen rather than from every UV at zero.
func (m *Model) SetShapeTextureString(key, val string) {
	m.editTexture(func(item map[string]any) bool {
		tex, ok := item[lottietexture.MemberTex].(map[string]any)
		if !ok {
			return false
		}
		cur, _ := tex[key].(string)
		if cur == val {
			return false
		}
		if val == "" {
			delete(tex, key)
		} else {
			tex[key] = val
		}
		if key == "mapping" && val == string(lottietexture.MappingVertex) {
			m.seedSiblingUVs()
		}
		return true
	})
}

// SetShapeTextureTint writes the tint flag; true is the default and drops
// the member.
func (m *Model) SetShapeTextureTint(on bool) {
	m.editTexture(func(item map[string]any) bool {
		tex, ok := item[lottietexture.MemberTex].(map[string]any)
		if !ok || m.ShapeTextureTint() == on {
			return false
		}
		if on {
			delete(tex, "tint")
		} else {
			tex["tint"] = false
		}
		return true
	})
}

// texTransform returns the paint's placement transform, creating it with
// static defaults when create is set.
func texTransform(tex map[string]any, create bool) (map[string]any, bool) {
	tr, ok := tex["transform"].(map[string]any)
	if ok || !create {
		return tr, ok
	}
	tr = map[string]any{
		"p": staticVec(0, 0), "s": staticVec(100, 100),
		"r": staticProp(0.0), "a": staticVec(0, 0),
	}
	tex["transform"] = tr
	return tr, true
}

// texTransformDefault is a member's value while the transform is absent.
func texTransformDefault(member string) []float64 {
	switch member {
	case "s":
		return []float64{100, 100}
	case "r":
		return []float64{0}
	}
	return []float64{0, 0}
}

// ShapeTexTransformValue reads one placement member at the edit frame.
func (m *Model) ShapeTexTransformValue(member string) ([]float64, bool) {
	tex, ok := m.ShapeTexture()
	if !ok {
		return nil, false
	}
	tr, ok := texTransform(tex, false)
	if !ok {
		return texTransformDefault(member), true
	}
	if v, ok := propValueNearObj(tr, member, m.shapeEditFrame()); ok {
		return v, true
	}
	return texTransformDefault(member), true
}

// ShapeTexTransformWritable mirrors ShapeMemberWritable for a placement
// member: static members always, keyed ones only parked on a key.
func (m *Model) ShapeTexTransformWritable(member string) bool {
	tex, ok := m.ShapeTexture()
	if !ok {
		return false
	}
	tr, ok := texTransform(tex, false)
	if !ok {
		return true
	}
	return !propAnimatedObj(tr, member) || propIsKeyedObj(tr, member, m.shapeEditFrame())
}

// ShapeTexTransformKeyTimes lists a placement member's key times.
func (m *Model) ShapeTexTransformKeyTimes(member string) []float64 {
	tex, ok := m.ShapeTexture()
	if !ok {
		return nil
	}
	tr, ok := texTransform(tex, false)
	if !ok {
		return nil
	}
	return shapeMemberKeyTimes(tr, member)
}

// ShapeTexTransformAdjacent reads a member at its own key one step from the
// edit frame, the neighbour-copy source.
func (m *Model) ShapeTexTransformAdjacent(member string, dir int) ([]float64, float64, bool) {
	tex, ok := m.ShapeTexture()
	if !ok {
		return nil, 0, false
	}
	tr, ok := texTransform(tex, false)
	if !ok {
		return nil, 0, false
	}
	t, ok := adjacentTime(shapeMemberKeyTimes(tr, member), m.shapeEditFrame(), dir)
	if !ok {
		return nil, 0, false
	}
	v, ok := propValueAtObj(tr, member, t)
	return v, t, ok
}

// SetShapeTexTransform writes one placement member at the edit frame,
// promoting a static member the way every other shape member promotes.
func (m *Model) SetShapeTexTransform(member string, v []float64) {
	d := m.StageClipDoc()
	if d == nil {
		return
	}
	frame := m.shapeEditFrame()
	m.editTexture(func(item map[string]any) bool {
		tex, ok := item[lottietexture.MemberTex].(map[string]any)
		if !ok {
			return false
		}
		tr, _ := texTransform(tex, true)
		if !d.setPropObj(tr, member, frame, v, m.shapePromoteTimes(frame)) {
			if propAnimatedObj(tr, member) {
				m.setStatus("park on a key to edit an animated value")
				m.generation++
			}
			return false
		}
		return true
	})
}

// SetShapeTexTransformComponent writes one component of a member.
func (m *Model) SetShapeTexTransformComponent(member string, comp int, v float64) {
	cur, ok := m.ShapeTexTransformValue(member)
	if !ok || comp >= len(cur) {
		return
	}
	next := slices.Clone(cur)
	next[comp] = v
	m.SetShapeTexTransform(member, next)
}

// ---- the bounding-box gizmo ----

// The stage gizmo for bbox mapping follows the gradient one: a square at
// the texture's origin (where UV 0,0 lands, i.e. the anchor placed at p)
// and a circle half a texture width along its u axis, so dragging the
// circle turns and scales the texture at once. Both live in the group's
// space, where the mapping's bounding box is.

// ShapeTexGizmoActive reports whether the selected paint maps by bbox.
func (m *Model) ShapeTexGizmoActive() bool {
	tex, ok := m.ShapeTexture()
	if !ok {
		return false
	}
	mapping, _ := tex["mapping"].(string)
	return mapping == "" || mapping == string(lottietexture.MappingBBox)
}

// shapeGroupBBox is the bounding box of the selected style's sibling
// geometry in group space — the box bbox mapping stretches the texture over.
func (m *Model) shapeGroupBBox() (lo, hi [2]float64, ok bool) {
	n, hasSel := m.SelectedShapeNode()
	d := m.StageClipDoc()
	if !hasSel || d == nil {
		return lo, hi, false
	}
	parent := n.path[:len(n.path)-1]
	var items []any
	if len(parent) == 0 {
		items, ok = d.layerShapes(n.layer)
	} else {
		group, found := d.shapeItem(n.layer, parent)
		if !found {
			return lo, hi, false
		}
		items, ok = group["it"].([]any)
	}
	if !ok {
		return lo, hi, false
	}
	lo = [2]float64{math.Inf(1), math.Inf(1)}
	hi = [2]float64{math.Inf(-1), math.Inf(-1)}
	found := false
	frame := m.stageFrame()
	for _, iv := range items {
		im, isMap := iv.(map[string]any)
		if !isMap {
			continue
		}
		if ty, _ := im["ty"].(string); !isShapeGeometry(ty) {
			continue
		}
		poly, polyOK := shapeItemPolygon(im, frame)
		if !polyOK {
			continue
		}
		for _, p := range poly {
			lo[0], lo[1] = math.Min(lo[0], p[0]), math.Min(lo[1], p[1])
			hi[0], hi[1] = math.Max(hi[0], p[0]), math.Max(hi[1], p[1])
			found = true
		}
	}
	if !found || hi[0]-lo[0] <= 0 || hi[1]-lo[1] <= 0 {
		return lo, hi, false
	}
	return lo, hi, true
}

// uvToGroup maps a UV point into group space through the bbox.
func uvToGroup(lo, hi [2]float64, u, v float64) [2]float64 {
	return [2]float64{lo[0] + u*(hi[0]-lo[0]), lo[1] + v*(hi[1]-lo[1])}
}

// ShapeTexGizmoPoints returns the gizmo's square and circle in group space.
func (m *Model) ShapeTexGizmoPoints() (origin, axis [2]float64, ok bool) {
	if !m.ShapeTexGizmoActive() {
		return origin, axis, false
	}
	lo, hi, ok := m.shapeGroupBBox()
	if !ok {
		return origin, axis, false
	}
	p, _ := m.ShapeTexTransformValue("p")
	s, _ := m.ShapeTexTransformValue("s")
	r, _ := m.ShapeTexTransformValue("r")
	if len(p) < 2 || len(s) < 2 || len(r) < 1 {
		return origin, axis, false
	}
	ang := r[0] * math.Pi / 180
	half := s[0] / 100 / 2
	origin = uvToGroup(lo, hi, p[0], p[1])
	axis = uvToGroup(lo, hi, p[0]+math.Cos(ang)*half, p[1]+math.Sin(ang)*half)
	return origin, axis, true
}

// MoveShapeTexGizmo drags one gizmo handle by a group-space delta: "origin"
// moves the placement, "axis" turns it and scales it (uniformly) so the
// circle follows the cursor.
func (m *Model) MoveShapeTexGizmo(which string, dx, dy float64) {
	lo, hi, ok := m.shapeGroupBBox()
	if !ok {
		return
	}
	bw, bh := hi[0]-lo[0], hi[1]-lo[1]
	p, _ := m.ShapeTexTransformValue("p")
	if len(p) < 2 {
		return
	}
	switch which {
	case "origin":
		m.SetShapeTexTransform("p", []float64{round4(p[0] + dx/bw), round4(p[1] + dy/bh)})
	case "axis":
		_, axis, ok := m.ShapeTexGizmoPoints()
		if !ok {
			return
		}
		nx, ny := axis[0]+dx, axis[1]+dy
		ox, oy := uvToGroup(lo, hi, p[0], p[1])[0], uvToGroup(lo, hi, p[0], p[1])[1]
		vu, vv := (nx-ox)/bw, (ny-oy)/bh
		if math.Hypot(vu, vv) < 1e-4 {
			return
		}
		m.SetShapeTexTransform("r", []float64{round2(math.Atan2(vv, vu) * 180 / math.Pi)})
		scale := round2(math.Hypot(vu, vv) * 2 * 100)
		m.SetShapeTexTransform("s", []float64{scale, scale})
	}
}

// ---- per-vertex UV of the selected path ----

// ShapeUVs reads the selected path's UV set, when it has one whose length
// matches the path.
func (m *Model) ShapeUVs() ([][2]float64, bool) {
	item, ok := m.SelectedShapeItem()
	if !ok {
		return nil, false
	}
	p, ok := m.ShapePath()
	if !ok {
		return nil, false
	}
	uv := readUV(item)
	if len(uv) != len(p.v) {
		return nil, false
	}
	return uv, true
}

// readUV decodes an item's x-uv member; nil when absent or malformed.
func readUV(item map[string]any) [][2]float64 {
	uvm, ok := item[lottietexture.MemberUV].(map[string]any)
	if !ok {
		return nil
	}
	arr, ok := uvm["v"].([]any)
	if !ok {
		return nil
	}
	out := make([][2]float64, 0, len(arr))
	for _, e := range arr {
		xy, ok := jsonNums(e)
		if !ok || len(xy) < 2 {
			return nil
		}
		out = append(out, [2]float64{xy[0], xy[1]})
	}
	return out
}

// writeUV stores a UV set as the item's x-uv member; nil removes it.
func writeUV(item map[string]any, uv [][2]float64) {
	if uv == nil {
		delete(item, lottietexture.MemberUV)
		return
	}
	arr := make([]any, len(uv))
	for i, p := range uv {
		arr[i] = []any{round4(p[0]), round4(p[1])}
	}
	uvm, ok := item[lottietexture.MemberUV].(map[string]any)
	if !ok {
		uvm = map[string]any{}
		item[lottietexture.MemberUV] = uvm
	}
	uvm["v"] = arr
}

// ShapeUVEditable reports whether the selected path takes a UV set: it has
// one, or a sibling paint maps by vertex.
func (m *Model) ShapeUVEditable() bool {
	n, ok := m.SelectedShapeNode()
	if !ok || n.ty != "sh" {
		return false
	}
	if _, has := m.ShapeUVs(); has {
		return true
	}
	return m.siblingVertexPaint() != nil
}

// siblingVertexPaint finds a paint in the selected item's group that maps
// by vertex, or any paint when none does but one exists.
func (m *Model) siblingVertexPaint() map[string]any {
	n, ok := m.SelectedShapeNode()
	d := m.StageClipDoc()
	if !ok || d == nil {
		return nil
	}
	var items []any
	if len(n.path) == 1 {
		items, _ = d.layerShapes(n.layer)
	} else if group, found := d.shapeItem(n.layer, n.path[:len(n.path)-1]); found {
		items, _ = group["it"].([]any)
	}
	var any1 map[string]any
	for _, iv := range items {
		im, ok := iv.(map[string]any)
		if !ok {
			continue
		}
		tex, ok := im[lottietexture.MemberTex].(map[string]any)
		if !ok {
			continue
		}
		if mapping, _ := tex["mapping"].(string); mapping == string(lottietexture.MappingVertex) {
			return tex
		}
		if any1 == nil {
			any1 = tex
		}
	}
	return any1
}

// SelectedUVVert is the UV point the pane has selected, -1 for none.
func (m *Model) SelectedUVVert() int { return m.selUVVert }

// SelectUVVert selects a UV point, and the same vertex on the stage.
func (m *Model) SelectUVVert(i int) {
	m.selUVVert = i
	if i >= 0 {
		m.selShapeVert = i
	}
	m.generation++
}

// editUV runs one write against the selected path's UV set.
func (m *Model) editUV(fn func(item map[string]any, uv [][2]float64) ([][2]float64, bool)) {
	if m.blockEdit() {
		return
	}
	n, ok := m.SelectedShapeNode()
	if !ok || n.ty != "sh" {
		return
	}
	item, ok := m.SelectedShapeItem()
	if !ok {
		return
	}
	uv, _ := m.ShapeUVs()
	pushed := m.snapshotClip()
	next, changed := fn(item, uv)
	if !changed {
		if pushed {
			m.dropLastSnapshot()
		}
		return
	}
	writeUV(item, next)
	m.touchClipDoc()
}

// SetShapeUV writes one UV point.
func (m *Model) SetShapeUV(i int, u, v float64) {
	m.editUV(func(_ map[string]any, uv [][2]float64) ([][2]float64, bool) {
		if i < 0 || i >= len(uv) {
			return uv, false
		}
		next := slices.Clone(uv)
		next[i] = [2]float64{u, v}
		return next, next[i] != uv[i]
	})
}

// MoveShapeUV drags one UV point, or every point when i is -1.
func (m *Model) MoveShapeUV(i int, du, dv float64) {
	m.editUV(func(_ map[string]any, uv [][2]float64) ([][2]float64, bool) {
		if len(uv) == 0 || (du == 0 && dv == 0) {
			return uv, false
		}
		next := slices.Clone(uv)
		for k := range next {
			if i >= 0 && k != i {
				continue
			}
			next[k] = [2]float64{next[k][0] + du, next[k][1] + dv}
		}
		return next, true
	})
}

// ScaleShapeUV scales the whole set about its centroid.
func (m *Model) ScaleShapeUV(factor float64) {
	m.editUV(func(_ map[string]any, uv [][2]float64) ([][2]float64, bool) {
		if len(uv) == 0 || factor <= 0 || factor == 1 {
			return uv, false
		}
		var cu, cv float64
		for _, p := range uv {
			cu, cv = cu+p[0], cv+p[1]
		}
		cu, cv = cu/float64(len(uv)), cv/float64(len(uv))
		next := make([][2]float64, len(uv))
		for k, p := range uv {
			next[k] = [2]float64{cu + (p[0]-cu)*factor, cv + (p[1]-cv)*factor}
		}
		return next, true
	})
}

// SeedShapeUV projects the selected path's vertices onto its own bounding
// box, the layout bbox mapping shows — the starting point every UV edit
// begins from, and what Reset returns to.
func (m *Model) SeedShapeUV() {
	m.editUV(func(item map[string]any, _ [][2]float64) ([][2]float64, bool) {
		uv := seedUVFor(item, m.stageFrame())
		return uv, uv != nil
	})
}

// ClearShapeUV drops the selected path's UV set.
func (m *Model) ClearShapeUV() {
	m.editUV(func(_ map[string]any, uv [][2]float64) ([][2]float64, bool) {
		return nil, uv != nil
	})
}

// seedUVFor computes the bbox projection of a path's vertices at a frame.
func seedUVFor(item map[string]any, frame float64) [][2]float64 {
	p, ok := pathAt(item, frame, true)
	if !ok || len(p.v) == 0 {
		return nil
	}
	lo := [2]float64{math.Inf(1), math.Inf(1)}
	hi := [2]float64{math.Inf(-1), math.Inf(-1)}
	for _, v := range p.v {
		lo[0], lo[1] = math.Min(lo[0], v[0]), math.Min(lo[1], v[1])
		hi[0], hi[1] = math.Max(hi[0], v[0]), math.Max(hi[1], v[1])
	}
	w, h := hi[0]-lo[0], hi[1]-lo[1]
	if w <= 0 {
		w = 1
	}
	if h <= 0 {
		h = 1
	}
	uv := make([][2]float64, len(p.v))
	for i, v := range p.v {
		uv[i] = [2]float64{(v[0] - lo[0]) / w, (v[1] - lo[1]) / h}
	}
	return uv
}

// seedSiblingUVs gives every path in the selected item's group a UV set if
// it has none, when a paint there starts mapping by vertex.
func (m *Model) seedSiblingUVs() {
	n, ok := m.SelectedShapeNode()
	d := m.StageClipDoc()
	if !ok || d == nil {
		return
	}
	var items []any
	if len(n.path) == 1 {
		items, _ = d.layerShapes(n.layer)
	} else if group, found := d.shapeItem(n.layer, n.path[:len(n.path)-1]); found {
		items, _ = group["it"].([]any)
	}
	for _, iv := range items {
		im, ok := iv.(map[string]any)
		if !ok {
			continue
		}
		if ty, _ := im["ty"].(string); ty != "sh" {
			continue
		}
		if _, has := im[lottietexture.MemberUV]; has {
			continue
		}
		if uv := seedUVFor(im, m.stageFrame()); uv != nil {
			writeUV(im, uv)
		}
	}
}

// insertUVVertex keeps a path's UV set in step with a vertex inserted at
// index at by splitting segment seg at t (insertPathVertex's arguments).
func insertUVVertex(item map[string]any, seg int, t float64) {
	uv := readUV(item)
	n := len(uv)
	if n == 0 || seg < 0 || seg >= n {
		return
	}
	a, b := uv[seg], uv[(seg+1)%n]
	t = min(max(t, 0.01), 0.99)
	mid := [2]float64{a[0] + (b[0]-a[0])*t, a[1] + (b[1]-a[1])*t}
	writeUV(item, slices.Insert(slices.Clone(uv), seg+1, mid))
}

// deleteUVVertex keeps a path's UV set in step with a deleted vertex.
func deleteUVVertex(item map[string]any, idx int) {
	uv := readUV(item)
	if idx < 0 || idx >= len(uv) {
		return
	}
	writeUV(item, slices.Delete(slices.Clone(uv), idx, idx+1))
}

// ---- images: the bundle's shared images as clip assets ----

// TextureChoices lists the image assets a paint can name: every image the
// bundle holds, by asset id, with the file each stands for.
type textureChoice struct {
	ID, File string
}

// TextureChoices lists the stage clip's image assets plus the bundle images
// no asset names yet, so any bundle image is one pick away.
func (m *Model) TextureChoices() []textureChoice {
	d := m.StageClipDoc()
	if d == nil {
		return nil
	}
	var out []textureChoice
	named := map[string]bool{}
	assets, _ := d.root["assets"].([]any)
	for _, av := range assets {
		am, ok := av.(map[string]any)
		if !ok {
			continue
		}
		if _, isComp := am["layers"]; isComp {
			continue
		}
		id, _ := am["id"].(string)
		file, _ := am["p"].(string)
		if id == "" || strings.HasPrefix(file, "data:") {
			continue
		}
		out = append(out, textureChoice{ID: id, File: file})
		named[filepath.Base(file)] = true
	}
	for _, name := range m.bundle.ImageNames() {
		if !named[name] {
			out = append(out, textureChoice{ID: "", File: name})
		}
	}
	return out
}

// TextureFile names the file behind an asset id, for display.
func (m *Model) TextureFile(assetID string) string {
	for _, c := range m.TextureChoices() {
		if c.ID == assetID {
			return c.File
		}
	}
	return assetID
}

// BindTextureFile binds a bundle image to the selected fill or stroke,
// adding the image asset the clip needs to reach it when there is none.
func (m *Model) BindTextureFile(file string) {
	if m.blockEdit() || !m.ShapeCanTexture() {
		return
	}
	d := m.StageClipDoc()
	item, ok := m.SelectedShapeItem()
	if d == nil || !ok {
		return
	}
	data, ok := m.bundle.Image(file)
	if !ok {
		m.setStatus("no image %q in the bundle", file)
		m.generation++
		return
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		m.setStatus("image %q: %v", file, err)
		m.generation++
		return
	}
	m.snapshotClip()
	id := ensureImageAsset(d, file, cfg.Width, cfg.Height)
	tex, ok := item[lottietexture.MemberTex].(map[string]any)
	if !ok {
		tex = map[string]any{}
		item[lottietexture.MemberTex] = tex
	}
	tex["texture"] = id
	m.touchClipDoc()
}

// ensureImageAsset returns the id of the clip's image asset for a bundle
// image, adding one when the clip has none.
func ensureImageAsset(d *clipDoc, file string, w, h int) string {
	assets, _ := d.root["assets"].([]any)
	var ids []string
	for _, av := range assets {
		am, ok := av.(map[string]any)
		if !ok {
			continue
		}
		id, _ := am["id"].(string)
		ids = append(ids, id)
		if p, _ := am["p"].(string); p == file {
			if _, isComp := am["layers"]; !isComp && id != "" {
				return id
			}
		}
	}
	base := strings.TrimSuffix(file, filepath.Ext(file))
	if base == "" {
		base = "image"
	}
	id := uniqueID(base, ids)
	d.root["assets"] = append(assets, any(map[string]any{
		"id": id, "w": w, "h": h, "u": "", "p": file, "e": 0,
	}))
	return id
}

var imageFilters = zenity.FileFilters{
	{Name: "Image", Patterns: []string{"*.png", "*.jpg", "*.jpeg"}, CaseFold: true},
}

// BrowseTextureImage asks for an image file to add to the bundle and bind
// to the selected fill or stroke.
func (m *Model) BrowseTextureImage() {
	m.start(dialogImportTexture, func() ([]string, error) {
		p, err := zenity.SelectFile(zenity.Title("Import texture"), imageFilters)
		return []string{p}, err
	})
}

// ImportTextureImage adds an image file to the bundle and binds it.
func (m *Model) ImportTextureImage(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		m.setStatus("cannot read %q: %v", path, err)
		m.generation++
		return
	}
	if _, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil {
		m.setStatus("%q is not an image this editor reads: %v", filepath.Base(path), err)
		m.generation++
		return
	}
	name := filepath.Base(path)
	m.bundle.SetImage(name, data)
	delete(m.texImages, name)
	m.docGen++
	m.BindTextureFile(name)
}

// TextureImage returns the bundle image behind an asset id decoded for the
// UV pane, cached by file name until the bundle is replaced.
func (m *Model) TextureImage(assetID string) *ebiten.Image {
	file := m.TextureFile(assetID)
	if img, ok := m.texImages[file]; ok {
		return img
	}
	if m.texImages == nil {
		m.texImages = map[string]*ebiten.Image{}
	}
	var img *ebiten.Image
	if data, ok := m.bundle.Image(file); ok {
		if src, _, err := image.Decode(bytes.NewReader(data)); err == nil {
			img = ebiten.NewImageFromImage(src)
		}
	}
	m.texImages[file] = img
	return img
}

// UVPaneImage is the texture the UV pane draws behind the selected path's
// UV set: whichever sibling paint maps by vertex.
func (m *Model) UVPaneImage() *ebiten.Image {
	tex := m.siblingVertexPaint()
	if tex == nil {
		return nil
	}
	id, _ := tex["texture"].(string)
	if id == "" {
		return nil
	}
	return m.TextureImage(id)
}
