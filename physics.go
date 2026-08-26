package lottie

import (
	"encoding/json"
	"fmt"
	"slices"
)

// Collision extensions. A dotLottie archive says nothing about physics, so
// this package defines two tool-specific payloads under extensions/physics/
// in the bundle. They are named after the engines they feed rather than a
// neutral schema: a body under extensions/physics/cp/ is shaped for
// jakecoffman/cp, a track under extensions/physics/resolv/ for
// SolarLune/resolv. The core stays dependency-free — the adapter modules
// under physics/ do the actual engine wiring.
//
// All coordinates are animation coordinates: the space Animation.Size
// describes, y growing downward.

// PhysPoint is a 2D point in animation coordinates.
type PhysPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// ---- cp: rigid body silhouettes ----

// CPBodyType selects how jakecoffman/cp integrates the body.
type CPBodyType string

const (
	CPBodyDynamic   CPBodyType = "dynamic"
	CPBodyKinematic CPBodyType = "kinematic"
	CPBodyStatic    CPBodyType = "static"
)

// CPBody is one rigid body definition: the fixed collision silhouette of a
// character or prop. It lives at bundle level, not per animation — the same
// capsule serves idle and run alike, which is why it is not keyframed.
type CPBody struct {
	// Type defaults to dynamic when empty.
	Type CPBodyType `json:"type,omitempty"`
	// Mass applies to dynamic bodies; zero or negative reads as 1.
	Mass float64 `json:"mass,omitempty"`
	// Moment is the moment of inertia; zero means "derive it from the
	// shapes", which is what the cp adapter does.
	Moment float64   `json:"moment,omitempty"`
	Shapes []CPShape `json:"shapes"`

	Extra ExtraFields `json:"-"`
}

func (b CPBody) MarshalJSON() ([]byte, error) {
	type alias CPBody
	return encodeExtra(alias(b), b.Extra)
}

func (b *CPBody) UnmarshalJSON(data []byte) error {
	type alias CPBody
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := decodeExtra(data, a)
	if err != nil {
		return err
	}
	*b = CPBody(a)
	b.Extra = extra
	return nil
}

// CPShapeType names the cp shape a CPShape builds.
type CPShapeType string

const (
	CPShapeCircle  CPShapeType = "circle"
	CPShapeBox     CPShapeType = "box"
	CPShapePolygon CPShapeType = "polygon"
)

// CPShape is one collision shape attached to a CPBody, in body-local
// animation coordinates.
type CPShape struct {
	Type CPShapeType `json:"type"`
	// Center places a circle or a box.
	Center PhysPoint `json:"center,omitzero"`
	// Radius is the circle's radius; for a box or polygon it is the corner
	// rounding cp applies.
	Radius float64 `json:"radius,omitempty"`
	// Width and Height size a box.
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
	// Vertices are a convex polygon's corners.
	Vertices []PhysPoint `json:"vertices,omitempty"`

	Friction   float64 `json:"friction,omitempty"`
	Elasticity float64 `json:"elasticity,omitempty"`
	// Sensor shapes report contacts without colliding.
	Sensor bool `json:"sensor,omitempty"`

	Extra ExtraFields `json:"-"`
}

func (s CPShape) MarshalJSON() ([]byte, error) {
	type alias CPShape
	return encodeExtra(alias(s), s.Extra)
}

func (s *CPShape) UnmarshalJSON(data []byte) error {
	type alias CPShape
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := decodeExtra(data, a)
	if err != nil {
		return err
	}
	*s = CPShape(a)
	s.Extra = extra
	return nil
}

// ParseCPBody decodes one extensions/physics/cp/ document.
func ParseCPBody(data []byte) (*CPBody, error) {
	var b CPBody
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("lottie: cp body: %w", err)
	}
	return &b, nil
}

// ---- resolv: frame-stepped hitboxes ----

// ResolvBoxKind names a hitbox's shape.
type ResolvBoxKind string

const (
	ResolvRect   ResolvBoxKind = "rect"
	ResolvCircle ResolvBoxKind = "circle"
)

// ResolvTrack holds the hitboxes of one animation. It is keyed by the
// animation's id in the bundle, because the boxes only mean anything against
// that clip's frames.
type ResolvTrack struct {
	Boxes []ResolvBox `json:"boxes"`

	Extra ExtraFields `json:"-"`
}

func (t ResolvTrack) MarshalJSON() ([]byte, error) {
	type alias ResolvTrack
	return encodeExtra(alias(t), t.Extra)
}

func (t *ResolvTrack) UnmarshalJSON(data []byte) error {
	type alias ResolvTrack
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := decodeExtra(data, a)
	if err != nil {
		return err
	}
	*t = ResolvTrack(a)
	t.Extra = extra
	return nil
}

// ResolvBox is one named hitbox. Tags say what the box means to the game —
// "hit", "hurt", "push" — and are how a game pulls out one judgement kind
// without knowing box names; a box may carry several.
type ResolvBox struct {
	Name string        `json:"name"`
	Kind ResolvBoxKind `json:"kind"`
	Tags []string      `json:"tags,omitempty"`
	// Spans are kept in frame order by the editor but nothing here requires
	// it; At takes the first span covering the frame.
	Spans []ResolvSpan `json:"spans"`

	Extra ExtraFields `json:"-"`
}

func (b ResolvBox) MarshalJSON() ([]byte, error) {
	type alias ResolvBox
	return encodeExtra(alias(b), b.Extra)
}

func (b *ResolvBox) UnmarshalJSON(data []byte) error {
	type alias ResolvBox
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := decodeExtra(data, a)
	if err != nil {
		return err
	}
	*b = ResolvBox(a)
	b.Extra = extra
	return nil
}

// HasTag reports whether the box carries the tag.
func (b *ResolvBox) HasTag(tag string) bool {
	return slices.Contains(b.Tags, tag)
}

// ResolvSpan makes its box active on frames [From, To) with constant
// geometry. Steps, not tweens: a fighting-game hitbox snaps between poses,
// so each span is one pose and adjacent spans express movement.
type ResolvSpan struct {
	From float64 `json:"from"`
	To   float64 `json:"to"`
	// X, Y are a rect's top-left corner, or a circle's center.
	X float64 `json:"x"`
	Y float64 `json:"y"`
	// W, H size a rect.
	W float64 `json:"w,omitempty"`
	H float64 `json:"h,omitempty"`
	// R is a circle's radius.
	R float64 `json:"r,omitempty"`

	Extra ExtraFields `json:"-"`
}

func (s ResolvSpan) MarshalJSON() ([]byte, error) {
	type alias ResolvSpan
	return encodeExtra(alias(s), s.Extra)
}

func (s *ResolvSpan) UnmarshalJSON(data []byte) error {
	type alias ResolvSpan
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := decodeExtra(data, a)
	if err != nil {
		return err
	}
	*s = ResolvSpan(a)
	s.Extra = extra
	return nil
}

// Covers reports whether the span is live at the frame.
func (s *ResolvSpan) Covers(frame float64) bool {
	return frame >= s.From && frame < s.To
}

// SpanAt returns the index of the first span covering the frame.
func (b *ResolvBox) SpanAt(frame float64) (int, bool) {
	for i := range b.Spans {
		if b.Spans[i].Covers(frame) {
			return i, true
		}
	}
	return 0, false
}

// ActiveBox is one box live at a queried frame with its geometry resolved.
// Index points back into the track's Boxes, so an editor can trace a query
// result to the box it edits.
type ActiveBox struct {
	Index int
	Name  string
	Kind  ResolvBoxKind
	Tags  []string
	X, Y  float64
	W, H  float64
	R     float64
}

// At returns the boxes live at the frame. Passing tags keeps only boxes
// carrying at least one of them, which is how a game asks for just its
// hurtboxes; with none, every live box returns.
func (t *ResolvTrack) At(frame float64, tags ...string) []ActiveBox {
	if t == nil {
		return nil
	}
	var out []ActiveBox
	for i := range t.Boxes {
		b := &t.Boxes[i]
		if len(tags) > 0 && !slices.ContainsFunc(tags, b.HasTag) {
			continue
		}
		si, ok := b.SpanAt(frame)
		if !ok {
			continue
		}
		sp := &b.Spans[si]
		out = append(out, ActiveBox{
			Index: i, Name: b.Name, Kind: b.Kind, Tags: b.Tags,
			X: sp.X, Y: sp.Y, W: sp.W, H: sp.H, R: sp.R,
		})
	}
	return out
}

// ParseResolvTrack decodes one extensions/physics/resolv/ document.
func ParseResolvTrack(data []byte) (*ResolvTrack, error) {
	var t ResolvTrack
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("lottie: resolv track: %w", err)
	}
	return &t, nil
}
