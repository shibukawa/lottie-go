// Package lottieresolv is the static plugin for frame-stepped hitboxes —
// the fighting-game kind: named boxes carrying free-form tags (hit, hurt,
// push), each active over frame spans with constant geometry per span.
// Tracks are stored in a dotLottie bundle under extensions/physics/resolv/
// and mirrored into a SolarLune/resolv Space by a Tracker.
//
// The lottie-go core knows nothing about this payload — it only carries
// extensions/ files through a rewrite verbatim. Importing this package is
// what gives the data a schema, readers, and writers; a program that never
// imports it never links this code or the resolv engine.
package lottieresolv

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	lottie "github.com/shibukawa/lottie-go"
)

// Dir is the bundle subtree this plugin claims; one JSON document per
// animation, named by the animation's id.
const Dir = "extensions/physics/resolv/"

// Kind names a hitbox's shape.
type Kind string

const (
	KindRect   Kind = "rect"
	KindCircle Kind = "circle"
)

// Track holds the hitboxes of one animation. It is keyed by the
// animation's id in the bundle, because the boxes only mean anything
// against that clip's frames. Coordinates are animation coordinates.
type Track struct {
	Boxes []Box `json:"boxes"`

	Extra lottie.ExtraFields `json:"-"`
}

func (t Track) MarshalJSON() ([]byte, error) {
	type alias Track
	return lottie.MarshalWithExtra(alias(t), t.Extra)
}

func (t *Track) UnmarshalJSON(data []byte) error {
	type alias Track
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := lottie.UnmarshalExtra(data, a)
	if err != nil {
		return err
	}
	*t = Track(a)
	t.Extra = extra
	return nil
}

// Box is one named hitbox. Tags say what the box means to the game —
// "hit", "hurt", "push" — and are how a game pulls out one judgement kind
// without knowing box names; a box may carry several.
type Box struct {
	Name string   `json:"name"`
	Kind Kind     `json:"kind"`
	Tags []string `json:"tags,omitempty"`
	// Spans are kept in frame order by the editor but nothing here requires
	// it; At takes the first span covering the frame.
	Spans []Span `json:"spans"`

	Extra lottie.ExtraFields `json:"-"`
}

func (b Box) MarshalJSON() ([]byte, error) {
	type alias Box
	return lottie.MarshalWithExtra(alias(b), b.Extra)
}

func (b *Box) UnmarshalJSON(data []byte) error {
	type alias Box
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := lottie.UnmarshalExtra(data, a)
	if err != nil {
		return err
	}
	*b = Box(a)
	b.Extra = extra
	return nil
}

// HasTag reports whether the box carries the tag.
func (b *Box) HasTag(tag string) bool {
	return slices.Contains(b.Tags, tag)
}

// Span makes its box active on frames [From, To) with constant geometry.
// Steps, not tweens: a fighting-game hitbox snaps between poses, so each
// span is one pose and adjacent spans express movement.
type Span struct {
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

	Extra lottie.ExtraFields `json:"-"`
}

func (s Span) MarshalJSON() ([]byte, error) {
	type alias Span
	return lottie.MarshalWithExtra(alias(s), s.Extra)
}

func (s *Span) UnmarshalJSON(data []byte) error {
	type alias Span
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := lottie.UnmarshalExtra(data, a)
	if err != nil {
		return err
	}
	*s = Span(a)
	s.Extra = extra
	return nil
}

// Covers reports whether the span is live at the frame.
func (s *Span) Covers(frame float64) bool {
	return frame >= s.From && frame < s.To
}

// SpanAt returns the index of the first span covering the frame.
func (b *Box) SpanAt(frame float64) (int, bool) {
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
	Kind  Kind
	Tags  []string
	X, Y  float64
	W, H  float64
	R     float64
}

// At returns the boxes live at the frame. Passing tags keeps only boxes
// carrying at least one of them, which is how a game asks for just its
// hurtboxes; with none, every live box returns.
func (t *Track) At(frame float64, tags ...string) []ActiveBox {
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

// ParseTrack decodes one track document.
func ParseTrack(data []byte) (*Track, error) {
	var t Track
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("lottieresolv: track: %w", err)
	}
	return &t, nil
}

// ---- bundle storage ----

func fileName(animID string) string { return Dir + animID + ".json" }

// IDs returns the animation ids carrying a track in the bundle, sorted.
func IDs(b *lottie.Bundle) []string {
	var out []string
	for _, name := range b.ExtensionFiles(Dir) {
		out = append(out, strings.TrimSuffix(strings.TrimPrefix(name, Dir), ".json"))
	}
	return out
}

// Load parses the given animation's track out of the bundle. Each call
// parses afresh; a caller editing in place keeps the pointer and Stores it
// back.
func Load(b *lottie.Bundle, animID string) (*Track, error) {
	data, ok := b.ExtensionFile(fileName(animID))
	if !ok {
		return nil, fmt.Errorf("lottieresolv: no track for animation %q in bundle", animID)
	}
	t, err := ParseTrack(data)
	if err != nil {
		return nil, fmt.Errorf("lottieresolv: track %q: %w", animID, err)
	}
	return t, nil
}

// Store writes an animation's track into the bundle, where Encode carries
// it under extensions/physics/resolv/ — with or without this plugin
// imported at that point. The animation need not exist yet; a track can be
// authored ahead of its clip. Removing a clip does not remove its track —
// the core cannot know they belong together — so an editor dropping a clip
// calls Remove too.
func Store(b *lottie.Bundle, animID string, t *Track) error {
	if animID == "" {
		return fmt.Errorf("lottieresolv: track animation id must not be empty")
	}
	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("lottieresolv: track %q: %w", animID, err)
	}
	return b.SetExtensionFile(fileName(animID), data)
}

// Remove drops an animation's track from the bundle.
func Remove(b *lottie.Bundle, animID string) {
	b.RemoveExtensionFile(fileName(animID))
}
