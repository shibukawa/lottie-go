// Package lottietexture is the static plugin for textured fills and
// strokes: an image painted through a vector path, with the UV given per
// path vertex so a deforming shape carries deforming art. Nothing in Lottie
// expresses this, so the data lives beside the clip, at
// extensions/texture/<animID>.json, and the clip itself stays plain Lottie —
// a player without this plugin draws the fills' solid colors.
//
// The core renders the paint and exposes typed hooks
// (lottie.Player.SetTexturePaint, SetVertexUV, SetTexture); this package
// owns the document and its file. A game loads the document and applies it
// to a player:
//
//	anim, _ := b.Animation(id)
//	p := anim.NewPlayer()
//	if doc, err := lottietexture.Load(b, id); err == nil && doc != nil {
//		doc.Apply(p)
//	}
//
// An editor works the other way round. It weaves the document into the
// clip's JSON tree, where every entry sits on the item it belongs to as an
// x-tex or x-uv member, edits it there — so vertex insertion, reordering,
// copy and paste carry the data for free — and unweaves it back into a
// fresh document whenever the clip is stored: addresses are regenerated
// from the tree, never edited, so they cannot drift. No x- member ever
// reaches a stored clip.
package lottietexture

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	lottie "github.com/shibukawa/lottie-go"
)

// Dir is the bundle subtree this plugin claims; File names one clip's
// document inside it.
const Dir = "extensions/texture/"

// File returns the bundle member holding the document of one clip.
func File(animID string) string { return Dir + animID + ".json" }

// Mapping says how a paint finds its UV; see lottie.TextureMapping.
type Mapping string

const (
	MappingBBox   Mapping = "bbox" // the empty default
	MappingVertex Mapping = "vertex"
	MappingStroke Mapping = "stroke"
)

// Wrap says what a sample outside the texture reads.
type Wrap string

const (
	WrapClamp  Wrap = "clamp" // the empty default
	WrapRepeat Wrap = "repeat"
	WrapMirror Wrap = "mirror"
)

// Filter picks the sampling filter.
type Filter string

const (
	FilterLinear  Filter = "linear" // the empty default
	FilterNearest Filter = "nearest"
)

// Paint binds an image to one fill or stroke item. Asset, Layer and Item
// are the item's address (lottie.ShapeRef): the precomp asset holding the
// layer (empty for the root), the layer's ind, and the index path through
// the shapes and it arrays. They are regenerated on every store and never
// edited by hand.
type Paint struct {
	Asset string `json:"asset,omitempty"`
	Layer int    `json:"layer"`
	Item  []int  `json:"item"`

	// Texture is an image asset refId of the clip, or a name the game binds
	// at runtime with lottie.Player.SetTexture.
	Texture string  `json:"texture"`
	Mapping Mapping `json:"mapping,omitempty"`
	Wrap    Wrap    `json:"wrap,omitempty"`
	Filter  Filter  `json:"filter,omitempty"`
	// Tint multiplies the texture by the item's color and opacity; nil
	// reads as true, so the item's color swatch stays meaningful.
	Tint *bool `json:"tint,omitempty"`
	// Transform is a Lottie transform object placing the texture in UV
	// space (p offset, s scale percent, r degrees, a anchor), static or
	// keyframed; see lottie.TexturePaint.Transform.
	Transform json.RawMessage `json:"transform,omitempty"`

	Extra lottie.ExtraFields `json:"-"`
}

func (p Paint) MarshalJSON() ([]byte, error) {
	type alias Paint
	return lottie.MarshalWithExtra(alias(p), p.Extra)
}

func (p *Paint) UnmarshalJSON(data []byte) error {
	type alias Paint
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := lottie.UnmarshalExtra(data, a)
	if err != nil {
		return err
	}
	*p = Paint(a)
	p.Extra = extra
	return nil
}

// Tinted applies the nil-means-true default of Tint.
func (p *Paint) Tinted() bool { return p.Tint == nil || *p.Tint }

// Ref is the paint's address as the core takes it.
func (p *Paint) Ref() lottie.ShapeRef {
	return lottie.ShapeRef{Asset: p.Asset, Layer: p.Layer, Item: p.Item}
}

// Runtime converts the paint into the core's form.
func (p *Paint) Runtime() *lottie.TexturePaint {
	tp := &lottie.TexturePaint{Texture: p.Texture, Tint: p.Tinted(), Transform: p.Transform}
	switch p.Mapping {
	case MappingVertex:
		tp.Mapping = lottie.MappingVertex
	case MappingStroke:
		tp.Mapping = lottie.MappingStroke
	}
	switch p.Wrap {
	case WrapRepeat:
		tp.Wrap = lottie.WrapRepeat
	case WrapMirror:
		tp.Wrap = lottie.WrapMirror
	}
	if p.Filter == FilterNearest {
		tp.Filter = lottie.FilterNearest
	}
	return tp
}

// UV gives one path item a normalized UV per vertex, parallel to the
// path's v array; the count must match, which Lottie keeps constant across
// a path's keys. Static: motion is the paint transform's job.
type UV struct {
	Asset string `json:"asset,omitempty"`
	Layer int    `json:"layer"`
	Item  []int  `json:"item"`

	V [][2]float64 `json:"v"`

	Extra lottie.ExtraFields `json:"-"`
}

func (u UV) MarshalJSON() ([]byte, error) {
	type alias UV
	return lottie.MarshalWithExtra(alias(u), u.Extra)
}

func (u *UV) UnmarshalJSON(data []byte) error {
	type alias UV
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := lottie.UnmarshalExtra(data, a)
	if err != nil {
		return err
	}
	*u = UV(a)
	u.Extra = extra
	return nil
}

// Ref is the entry's address as the core takes it.
func (u *UV) Ref() lottie.ShapeRef {
	return lottie.ShapeRef{Asset: u.Asset, Layer: u.Layer, Item: u.Item}
}

// Runtime converts the UV list into the core's form.
func (u *UV) Runtime() [][2]float32 {
	out := make([][2]float32, len(u.V))
	for i, v := range u.V {
		out[i] = [2]float32{float32(v[0]), float32(v[1])}
	}
	return out
}

// Doc is one clip's texture document.
type Doc struct {
	Paints []Paint `json:"paints"`
	UVs    []UV    `json:"uvs"`

	Extra lottie.ExtraFields `json:"-"`
}

func (d Doc) MarshalJSON() ([]byte, error) {
	type alias Doc
	return lottie.MarshalWithExtra(alias(d), d.Extra)
}

func (d *Doc) UnmarshalJSON(data []byte) error {
	type alias Doc
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := lottie.UnmarshalExtra(data, a)
	if err != nil {
		return err
	}
	*d = Doc(a)
	d.Extra = extra
	return nil
}

// Empty reports whether the document binds nothing.
func (d *Doc) Empty() bool {
	return d == nil || len(d.Paints) == 0 && len(d.UVs) == 0
}

// Append adds another document's entries to this one — how an editor puts
// the entries Weave could not place back beside the ones it unwove.
func (d *Doc) Append(o *Doc) {
	if o == nil {
		return
	}
	d.Paints = append(d.Paints, o.Paints...)
	d.UVs = append(d.UVs, o.UVs...)
}

// Apply binds every entry to the player. Entries that do not resolve are
// reported together and skipped; the rest still apply, so one stale entry
// does not undress the whole clip.
func (d *Doc) Apply(p *lottie.Player) error {
	var errs []error
	for i := range d.Paints {
		pt := &d.Paints[i]
		if err := p.SetTexturePaint(pt.Ref(), pt.Runtime()); err != nil {
			errs = append(errs, err)
		}
	}
	for i := range d.UVs {
		uv := &d.UVs[i]
		if err := p.SetVertexUV(uv.Ref(), uv.Runtime()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Parse decodes one document.
func Parse(data []byte) (*Doc, error) {
	var d Doc
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("lottietexture: %w", err)
	}
	return &d, nil
}

// Load parses the document of one clip. A clip without one yields nil, nil.
func Load(b *lottie.Bundle, animID string) (*Doc, error) {
	data, ok := b.ExtensionFile(File(animID))
	if !ok {
		return nil, nil
	}
	return Parse(data)
}

// Store writes the document into the bundle, where Encode carries it — with
// or without this plugin imported at that point. Entries keep the order
// they were given; json.Marshal makes the encoding canonical, so a document
// this package wrote re-encodes byte for byte.
func Store(b *lottie.Bundle, animID string, d *Doc) error {
	data, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("lottietexture: %w", err)
	}
	return b.SetExtensionFile(File(animID), data)
}

// Remove drops the document of one clip. The core leaves extension files
// alone when a clip goes, so this is the writer's job.
func Remove(b *lottie.Bundle, animID string) {
	b.RemoveExtensionFile(File(animID))
}

// ---- weaving: the document as members of the clip tree ----

// The woven form puts each entry on its item: x-tex on the fill or stroke,
// holding the paint's members minus the address; x-uv on the path, holding
// {v}. Only these two are read back, but Unweave strips every x- member, so
// a working member can never leak into a stored clip.

const (
	// MemberTex is the woven form of a Paint, on an fl or st item.
	MemberTex = "x-tex"
	// MemberUV is the woven form of a UV, on an sh item.
	MemberUV = "x-uv"
)

// Weave attaches the document's entries to the decoded clip tree (the
// generic map form of the JSON, as encoding/json produces it). Entries
// whose address does not resolve come back as a document of their own, so
// the caller can carry them and write them back unchanged; nil when every
// entry found its item.
func Weave(clip map[string]any, d *Doc) *Doc {
	if d == nil {
		return nil
	}
	var left Doc
	for i := range d.Paints {
		p := &d.Paints[i]
		item, ok := itemAt(clip, p.Asset, p.Layer, p.Item)
		if !ok {
			left.Paints = append(left.Paints, *p)
			continue
		}
		item[MemberTex] = p.woven()
	}
	for i := range d.UVs {
		u := &d.UVs[i]
		item, ok := itemAt(clip, u.Asset, u.Layer, u.Item)
		if !ok {
			left.UVs = append(left.UVs, *u)
			continue
		}
		item[MemberUV] = u.woven()
	}
	if left.Empty() {
		return nil
	}
	return &left
}

// Unweave strips every x- member from the clip tree's shape items into a
// fresh document, addresses computed from the tree as it stands, in
// document order: root layers first, then each precomp asset's layers.
// The tree is left plain Lottie. A tree that held nothing yields nil.
func Unweave(clip map[string]any) *Doc {
	var d Doc
	visit := func(asset string, layers []any) {
		for _, lv := range layers {
			lm, ok := lv.(map[string]any)
			if !ok {
				continue
			}
			ind, ok := num(lm["ind"])
			shapes, isArr := lm["shapes"].([]any)
			if !isArr {
				continue
			}
			walkItems(shapes, nil, func(item map[string]any, path []int) {
				if raw, has := item[MemberTex]; has {
					if ok {
						d.Paints = append(d.Paints, paintFromWoven(raw, asset, int(ind), path))
					}
				}
				if raw, has := item[MemberUV]; has {
					if ok {
						d.UVs = append(d.UVs, uvFromWoven(raw, asset, int(ind), path))
					}
				}
				for k := range item {
					if strings.HasPrefix(k, "x-") {
						delete(item, k)
					}
				}
			})
		}
	}
	if layers, ok := clip["layers"].([]any); ok {
		visit("", layers)
	}
	if assets, ok := clip["assets"].([]any); ok {
		for _, av := range assets {
			am, ok := av.(map[string]any)
			if !ok {
				continue
			}
			id, _ := am["id"].(string)
			if layers, ok := am["layers"].([]any); ok && id != "" {
				visit(id, layers)
			}
		}
	}
	if d.Empty() {
		return nil
	}
	return &d
}

// WeaveJSON is Weave over the clip's bytes. The clip comes back
// re-encoded compactly; an editor that keeps its own tree should call Weave
// on that tree instead.
func WeaveJSON(clip []byte, d *Doc) (woven []byte, unplaced *Doc, err error) {
	tree, err := decodeTree(clip)
	if err != nil {
		return nil, nil, err
	}
	unplaced = Weave(tree, d)
	out, err := json.Marshal(tree)
	if err != nil {
		return nil, nil, fmt.Errorf("lottietexture: %w", err)
	}
	return out, unplaced, nil
}

// UnweaveJSON is Unweave over the clip's bytes; see WeaveJSON.
func UnweaveJSON(clip []byte) (pure []byte, d *Doc, err error) {
	tree, err := decodeTree(clip)
	if err != nil {
		return nil, nil, err
	}
	d = Unweave(tree)
	out, err := json.Marshal(tree)
	if err != nil {
		return nil, nil, fmt.Errorf("lottietexture: %w", err)
	}
	return out, d, nil
}

func decodeTree(clip []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(clip))
	dec.UseNumber()
	var tree map[string]any
	if err := dec.Decode(&tree); err != nil {
		return nil, fmt.Errorf("lottietexture: %w", err)
	}
	return tree, nil
}

// woven is the paint as an x-tex member: its own members minus the
// address, unknown ones included so they round-trip.
func (p *Paint) woven() map[string]any {
	m := map[string]any{"texture": p.Texture}
	if p.Mapping != "" {
		m["mapping"] = string(p.Mapping)
	}
	if p.Wrap != "" {
		m["wrap"] = string(p.Wrap)
	}
	if p.Filter != "" {
		m["filter"] = string(p.Filter)
	}
	if p.Tint != nil {
		m["tint"] = *p.Tint
	}
	if len(p.Transform) > 0 {
		if tr, err := rawToTree(p.Transform); err == nil {
			m["transform"] = tr
		}
	}
	for k, raw := range p.Extra {
		if _, taken := m[k]; !taken {
			if v, err := rawToTree(raw); err == nil {
				m[k] = v
			}
		}
	}
	return m
}

func paintFromWoven(raw any, asset string, layer int, path []int) Paint {
	p := Paint{Asset: asset, Layer: layer, Item: append([]int(nil), path...)}
	m, ok := raw.(map[string]any)
	if !ok {
		return p
	}
	for k, v := range m {
		switch k {
		case "texture":
			p.Texture, _ = v.(string)
		case "mapping":
			s, _ := v.(string)
			p.Mapping = Mapping(s)
		case "wrap":
			s, _ := v.(string)
			p.Wrap = Wrap(s)
		case "filter":
			s, _ := v.(string)
			p.Filter = Filter(s)
		case "tint":
			if b, ok := v.(bool); ok {
				p.Tint = &b
			}
		case "transform":
			if data, err := json.Marshal(v); err == nil {
				p.Transform = data
			}
		default:
			if data, err := json.Marshal(v); err == nil {
				if p.Extra == nil {
					p.Extra = lottie.ExtraFields{}
				}
				p.Extra[k] = data
			}
		}
	}
	return p
}

func (u *UV) woven() map[string]any {
	v := make([]any, len(u.V))
	for i, uv := range u.V {
		v[i] = []any{uv[0], uv[1]}
	}
	m := map[string]any{"v": v}
	for k, raw := range u.Extra {
		if _, taken := m[k]; !taken {
			if val, err := rawToTree(raw); err == nil {
				m[k] = val
			}
		}
	}
	return m
}

func uvFromWoven(raw any, asset string, layer int, path []int) UV {
	u := UV{Asset: asset, Layer: layer, Item: append([]int(nil), path...)}
	m, ok := raw.(map[string]any)
	if !ok {
		return u
	}
	for k, v := range m {
		if k != "v" {
			if data, err := json.Marshal(v); err == nil {
				if u.Extra == nil {
					u.Extra = lottie.ExtraFields{}
				}
				u.Extra[k] = data
			}
			continue
		}
		arr, _ := v.([]any)
		u.V = make([][2]float64, 0, len(arr))
		for _, e := range arr {
			pair, _ := e.([]any)
			var uv [2]float64
			for c := 0; c < 2 && c < len(pair); c++ {
				uv[c], _ = num(pair[c])
			}
			u.V = append(u.V, uv)
		}
	}
	return u
}

func rawToTree(raw json.RawMessage) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// itemAt resolves an address in the tree.
func itemAt(clip map[string]any, asset string, layer int, path []int) (map[string]any, bool) {
	var layers []any
	if asset == "" {
		layers, _ = clip["layers"].([]any)
	} else {
		assets, _ := clip["assets"].([]any)
		for _, av := range assets {
			am, ok := av.(map[string]any)
			if !ok {
				continue
			}
			if id, _ := am["id"].(string); id == asset {
				layers, _ = am["layers"].([]any)
				break
			}
		}
	}
	var items []any
	for _, lv := range layers {
		lm, ok := lv.(map[string]any)
		if !ok {
			continue
		}
		if ind, ok := num(lm["ind"]); ok && int(ind) == layer {
			items, _ = lm["shapes"].([]any)
			break
		}
	}
	if items == nil || len(path) == 0 {
		return nil, false
	}
	var item map[string]any
	for step, idx := range path {
		if idx < 0 || idx >= len(items) {
			return nil, false
		}
		var ok bool
		item, ok = items[idx].(map[string]any)
		if !ok {
			return nil, false
		}
		if step == len(path)-1 {
			return item, true
		}
		if ty, _ := item["ty"].(string); ty != "gr" {
			return nil, false
		}
		items, _ = item["it"].([]any)
	}
	return nil, false
}

// walkItems visits every item of a shapes tree depth-first in document
// order, with the index path that reaches it.
func walkItems(items []any, prefix []int, fn func(item map[string]any, path []int)) {
	for i, iv := range items {
		im, ok := iv.(map[string]any)
		if !ok {
			continue
		}
		path := append(append([]int(nil), prefix...), i)
		fn(im, path)
		if ty, _ := im["ty"].(string); ty == "gr" {
			if it, ok := im["it"].([]any); ok {
				walkItems(it, path, fn)
			}
		}
	}
}

// num accepts the number forms a decoded tree can hold.
func num(v any) (float64, bool) {
	switch n := v.(type) {
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case float64:
		return n, true
	case int:
		return float64(n), true
	}
	return 0, false
}

// Attach dresses every clip player a state machine creates with the
// bundle's texture documents, so a machine-driven character shows its
// textures without the game ever touching the players the machine owns.
// Entries that do not resolve are skipped: a game cannot act on them here,
// and the editor reports them where they can be fixed.
func Attach(sm *lottie.StateMachinePlayer, b *lottie.Bundle) {
	sm.OnPlayer(func(animID string, p *lottie.Player) {
		doc, err := Load(b, animID)
		if err != nil || doc == nil {
			return
		}
		_ = doc.Apply(p)
	})
}
