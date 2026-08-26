// Package lottieevents is the static plugin for frame events: one-shot
// cues at exact frames carrying a free-form payload — a footstep sound
// with its volume, a screen shake with its magnitude, a spawn naming a
// socket. Lottie markers already fire through Player.OnMarker; this track
// exists for what markers cannot say: a JSON payload, and repeated
// same-name cues, without touching the Lottie document itself.
//
// One document per animation lives in the bundle under extensions/events/,
// carried verbatim by the core; importing this package is what gives it a
// schema and an emitter.
package lottieevents

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	lottie "github.com/shibukawa/lottie-go"
)

// Dir is the bundle subtree this plugin claims; one JSON document per
// animation, named by the animation's id.
const Dir = "extensions/events/"

// Event is one cue.
type Event struct {
	Frame float64 `json:"frame"`
	Name  string  `json:"name"`
	// Payload is whatever the game wants to hear: {"sound":"step","vol":0.4}.
	Payload json.RawMessage `json:"payload,omitempty"`

	Extra lottie.ExtraFields `json:"-"`
}

func (e Event) MarshalJSON() ([]byte, error) {
	type alias Event
	return lottie.MarshalWithExtra(alias(e), e.Extra)
}

func (e *Event) UnmarshalJSON(data []byte) error {
	type alias Event
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := lottie.UnmarshalExtra(data, a)
	if err != nil {
		return err
	}
	*e = Event(a)
	e.Extra = extra
	return nil
}

// Track holds one animation's events.
type Track struct {
	Events []Event `json:"events"`

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
	*t = Track(a)
	extra, err := lottie.UnmarshalExtra(data, a)
	if err != nil {
		return err
	}
	t.Extra = extra
	return nil
}

// In returns the events whose frame lies in the half-open span [from, to),
// in frame order — the same crossing rule markers use, so a cue fires
// exactly once however the cursor sweeps past it.
func (t *Track) In(from, to float64) []Event {
	if t == nil || to <= from {
		return nil
	}
	var out []Event
	for i := range t.Events {
		if f := t.Events[i].Frame; from <= f && f < to {
			out = append(out, t.Events[i])
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Frame < out[j].Frame })
	return out
}

// ParseTrack decodes one event track document.
func ParseTrack(data []byte) (*Track, error) {
	var t Track
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("lottieevents: track: %w", err)
	}
	return &t, nil
}

// ---- bundle storage ----

func fileName(animID string) string { return Dir + animID + ".json" }

// IDs returns the animation ids carrying an event track, sorted.
func IDs(b *lottie.Bundle) []string {
	var out []string
	for _, name := range b.ExtensionFiles(Dir) {
		out = append(out, strings.TrimSuffix(strings.TrimPrefix(name, Dir), ".json"))
	}
	return out
}

// Load parses the given animation's event track out of the bundle.
func Load(b *lottie.Bundle, animID string) (*Track, error) {
	data, ok := b.ExtensionFile(fileName(animID))
	if !ok {
		return nil, fmt.Errorf("lottieevents: no event track for animation %q in bundle", animID)
	}
	t, err := ParseTrack(data)
	if err != nil {
		return nil, fmt.Errorf("lottieevents: track %q: %w", animID, err)
	}
	return t, nil
}

// Store writes an animation's event track into the bundle.
func Store(b *lottie.Bundle, animID string, t *Track) error {
	if animID == "" {
		return fmt.Errorf("lottieevents: track animation id must not be empty")
	}
	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("lottieevents: track %q: %w", animID, err)
	}
	return b.SetExtensionFile(fileName(animID), data)
}

// Remove drops an animation's event track from the bundle.
func Remove(b *lottie.Bundle, animID string) {
	b.RemoveExtensionFile(fileName(animID))
}

// Cue installs the track's emitter on the player: fn fires for every event
// the cursor sweeps past during Update, loop wraps and reverse included,
// in frame order within each swept span. It claims the player's single
// OnFrameSpan slot — a program that needs its own span handler too should
// call Track.In from that handler instead. Seeks and SetFrame jump without
// sweeping, exactly as markers do.
func Cue(p *lottie.Player, t *Track, fn func(Event)) {
	p.OnFrameSpan(func(from, to float64) {
		for _, e := range t.In(from, to) {
			fn(e)
		}
	})
}
