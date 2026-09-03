package lottiespine

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// The JSON export of Spine 4.x, read permissively: members this importer
// does not use are ignored, and the handful of 3.x spellings that cost
// nothing to accept (skins as a map, "transform" for inherit, "color" slot
// timelines, top-level "deform") are accepted too.

// Skeleton is one Spine JSON export.
type Skeleton struct {
	Info       SkeletonInfo          `json:"skeleton"`
	Bones      []BoneData            `json:"bones"`
	Slots      []SlotData            `json:"slots"`
	IK         []IKData              `json:"ik"`
	Transform  []TransformData       `json:"transform"`
	Path       []json.RawMessage     `json:"path"`
	Physics    []json.RawMessage     `json:"physics"`
	Skins      []Skin                `json:"-"`
	Events     map[string]any        `json:"events"`
	Animations map[string]*Animation `json:"animations"`
}

// SkeletonInfo is the "skeleton" header.
type SkeletonInfo struct {
	Hash   string  `json:"hash"`
	Spine  string  `json:"spine"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Images string  `json:"images"`
	FPS    float64 `json:"fps"`
}

// Major returns the major version of the exporting Spine, 4 when unknown.
func (s SkeletonInfo) Major() int {
	v := strings.TrimSpace(s.Spine)
	if i := strings.IndexByte(v, '.'); i > 0 {
		v = v[:i]
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 4
	}
	return n
}

// BoneData is one entry of "bones".
type BoneData struct {
	Name     string   `json:"name"`
	Parent   string   `json:"parent"`
	Length   float64  `json:"length"`
	X        float64  `json:"x"`
	Y        float64  `json:"y"`
	Rotation float64  `json:"rotation"`
	ScaleX   *float64 `json:"scaleX"`
	ScaleY   *float64 `json:"scaleY"`
	ShearX   float64  `json:"shearX"`
	ShearY   float64  `json:"shearY"`
	Inherit  string   `json:"inherit"`
	// Transform is the 3.x name of Inherit.
	Transform string `json:"transform"`
}

func (b *BoneData) scale() (float64, float64) { return ptrOr(b.ScaleX, 1), ptrOr(b.ScaleY, 1) }

func (b *BoneData) inherit() string {
	if b.Inherit != "" {
		return b.Inherit
	}
	if b.Transform != "" {
		return b.Transform
	}
	return "normal"
}

// SlotData is one entry of "slots".
type SlotData struct {
	Name       string `json:"name"`
	Bone       string `json:"bone"`
	Color      string `json:"color"`
	Dark       string `json:"dark"`
	Attachment string `json:"attachment"`
	Blend      string `json:"blend"`
}

// IKData is one IK constraint.
type IKData struct {
	Name         string   `json:"name"`
	Order        int      `json:"order"`
	SkinRequired bool     `json:"skin"`
	Bones        []string `json:"bones"`
	Target       string   `json:"target"`
	Mix          *float64 `json:"mix"`
	Softness     float64  `json:"softness"`
	BendPositive *bool    `json:"bendPositive"`
	Compress     bool     `json:"compress"`
	Stretch      bool     `json:"stretch"`
	Uniform      bool     `json:"uniform"`
}

// TransformData is one transform constraint.
type TransformData struct {
	Name         string   `json:"name"`
	Order        int      `json:"order"`
	SkinRequired bool     `json:"skin"`
	Bones        []string `json:"bones"`
	Target       string   `json:"target"`
	Local        bool     `json:"local"`
	Relative     bool     `json:"relative"`
	Rotation     float64  `json:"rotation"`
	X            float64  `json:"x"`
	Y            float64  `json:"y"`
	ScaleX       float64  `json:"scaleX"`
	ScaleY       float64  `json:"scaleY"`
	ShearY       float64  `json:"shearY"`
	MixRotate    *float64 `json:"mixRotate"`
	MixX         *float64 `json:"mixX"`
	MixY         *float64 `json:"mixY"`
	MixScaleX    *float64 `json:"mixScaleX"`
	MixScaleY    *float64 `json:"mixScaleY"`
	MixShearY    *float64 `json:"mixShearY"`
	// 3.x names.
	RotateMix    *float64 `json:"rotateMix"`
	TranslateMix *float64 `json:"translateMix"`
	ScaleMix     *float64 `json:"scaleMix"`
	ShearMix     *float64 `json:"shearMix"`
}

// mixes resolves the constraint's setup mixes with Spine's defaults: 1,
// with mixY following mixX and mixScaleY following mixScaleX.
func (t *TransformData) mixes() transformMix {
	var m transformMix
	m.rotate = ptrOr(t.MixRotate, ptrOr(t.RotateMix, 1))
	m.x = ptrOr(t.MixX, ptrOr(t.TranslateMix, 1))
	m.y = ptrOr(t.MixY, m.x)
	m.scaleX = ptrOr(t.MixScaleX, ptrOr(t.ScaleMix, 1))
	m.scaleY = ptrOr(t.MixScaleY, m.scaleX)
	m.shearY = ptrOr(t.MixShearY, ptrOr(t.ShearMix, 1))
	return m
}

type transformMix struct{ rotate, x, y, scaleX, scaleY, shearY float64 }

// Skin is one entry of "skins": attachments keyed by slot name, then by
// the name the slot shows them under.
type Skin struct {
	Name        string                            `json:"name"`
	Attachments map[string]map[string]*Attachment `json:"attachments"`
}

// Attachment is one skin entry. Type "" is a region.
type Attachment struct {
	Type     string          `json:"type"`
	Name     string          `json:"name"`
	Path     string          `json:"path"`
	X        float64         `json:"x"`
	Y        float64         `json:"y"`
	ScaleX   *float64        `json:"scaleX"`
	ScaleY   *float64        `json:"scaleY"`
	Rotation float64         `json:"rotation"`
	Width    float64         `json:"width"`
	Height   float64         `json:"height"`
	Color    string          `json:"color"`
	Sequence json.RawMessage `json:"sequence"`

	// Mesh.
	UVs       []float64 `json:"uvs"`
	Triangles []int     `json:"triangles"`
	Vertices  []float64 `json:"vertices"`
	Hull      int       `json:"hull"`
	Edges     []int     `json:"edges"`

	// Linked mesh.
	Skin      string `json:"skin"`
	Parent    string `json:"parent"`
	Timelines *bool  `json:"timelines"`
	Deform    *bool  `json:"deform"`

	// Bounding box, clipping, path.
	VertexCount int `json:"vertexCount"`
}

func (a *Attachment) kind() string {
	if a.Type == "" {
		return "region"
	}
	return a.Type
}

// imagePath is the texture region the attachment draws: its path, or the
// name it is stored under.
func (a *Attachment) imagePath(storedName string) string {
	if a.Path != "" {
		return a.Path
	}
	if a.Name != "" {
		return a.Name
	}
	return storedName
}

// Animation is one entry of "animations". Timeline keys are kept generic
// (Key) since every kind shares the time and curve members.
type Animation struct {
	Bones     map[string]map[string][]Key `json:"bones"`
	Slots     map[string]map[string][]Key `json:"slots"`
	IK        map[string][]Key            `json:"ik"`
	Transform map[string][]Key            `json:"transform"`
	// Attachments is the 4.2 form: skin, slot, attachment, timeline
	// ("deform" or "sequence").
	Attachments map[string]map[string]map[string]map[string][]Key `json:"attachments"`
	// Deform is the pre-4.2 form: skin, slot, attachment.
	Deform    map[string]map[string]map[string][]Key `json:"deform"`
	Events    []Key                                  `json:"events"`
	DrawOrder []json.RawMessage                      `json:"drawOrder"`
	Path      map[string]json.RawMessage             `json:"path"`
	Physics   map[string]json.RawMessage             `json:"physics"`
}

// Key is one keyframe of any timeline. Members not meaningful to a
// timeline stay nil, and the accessors apply Spine's per-timeline defaults.
type Key struct {
	Time  float64  `json:"time"`
	Value *float64 `json:"value"`
	Angle *float64 `json:"angle"` // 3.x rotate
	X     *float64 `json:"x"`
	Y     *float64 `json:"y"`
	Name  *string  `json:"name"`
	Color *string  `json:"color"`
	Light *string  `json:"light"`
	Dark  *string  `json:"dark"`
	// Inherit is the 4.2 inherit timeline's value.
	Inherit *string `json:"inherit"`

	Mix          *float64 `json:"mix"`
	Softness     *float64 `json:"softness"`
	BendPositive *bool    `json:"bendPositive"`
	Compress     *bool    `json:"compress"`
	Stretch      *bool    `json:"stretch"`
	MixRotate    *float64 `json:"mixRotate"`
	MixX         *float64 `json:"mixX"`
	MixY         *float64 `json:"mixY"`
	MixScaleX    *float64 `json:"mixScaleX"`
	MixScaleY    *float64 `json:"mixScaleY"`
	MixShearY    *float64 `json:"mixShearY"`
	RotateMix    *float64 `json:"rotateMix"`
	TranslateMix *float64 `json:"translateMix"`
	ScaleMix     *float64 `json:"scaleMix"`
	ShearMix     *float64 `json:"shearMix"`

	Offset   int       `json:"offset"`
	Vertices []float64 `json:"vertices"`

	Curve json.RawMessage `json:"curve"`
	C2    *float64        `json:"c2"`
	C3    *float64        `json:"c3"`
	C4    *float64        `json:"c4"`
}

func ptrOr(p *float64, def float64) float64 {
	if p == nil {
		return def
	}
	return *p
}

func boolOr(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

// Parse decodes a Spine JSON export.
func Parse(data []byte) (*Skeleton, error) {
	var raw struct {
		Skeleton
		Skins json.RawMessage `json:"skins"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("lottiespine: %w", err)
	}
	sk := raw.Skeleton
	if len(sk.Bones) == 0 {
		return nil, fmt.Errorf("lottiespine: no bones; not a Spine skeleton export")
	}
	skins, err := parseSkins(raw.Skins)
	if err != nil {
		return nil, err
	}
	sk.Skins = skins
	return &sk, nil
}

// parseSkins reads the 4.x array form or the 3.x map form of "skins".
func parseSkins(raw json.RawMessage) ([]Skin, error) {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	if raw[0] == '[' {
		var skins []Skin
		if err := json.Unmarshal(raw, &skins); err != nil {
			return nil, fmt.Errorf("lottiespine: skins: %w", err)
		}
		return skins, nil
	}
	var byName map[string]map[string]map[string]*Attachment
	if err := json.Unmarshal(raw, &byName); err != nil {
		return nil, fmt.Errorf("lottiespine: skins: %w", err)
	}
	var skins []Skin
	if def, ok := byName["default"]; ok {
		skins = append(skins, Skin{Name: "default", Attachments: def})
	}
	for name, atts := range byName {
		if name != "default" {
			skins = append(skins, Skin{Name: name, Attachments: atts})
		}
	}
	return skins, nil
}

// parseColor reads "rrggbbaa" or "rrggbb" into 0..1 components; alpha is
// 1 when absent. Anything unreadable is white.
func parseColor(s string) [4]float64 {
	c := [4]float64{1, 1, 1, 1}
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 && len(s) != 8 {
		return c
	}
	for i := 0; i*2 < len(s); i++ {
		v, err := strconv.ParseUint(s[i*2:i*2+2], 16, 8)
		if err != nil {
			return [4]float64{1, 1, 1, 1}
		}
		c[i] = float64(v) / 255
	}
	return c
}
