// Package lottiesockets is the static plugin for attachment sockets: named
// transforms — a hand, a muzzle, the character's feet — that gameplay
// attaches things to. A socket binds a stable game-facing name to a layer
// (typically a null) the animator drives in their own tool, so the
// attachment interpolates exactly like the artwork; the transform itself
// comes from lottie.Animation.LayerPlacement.
//
// The mapping document lives in the bundle at extensions/sockets.json,
// carried verbatim by the core; importing this package is what gives it a
// schema. It is bundle-level: socket names are a convention across every
// clip of a character, each clip resolving them against its own layers.
//
// Root motion is a socket named however the game likes (commonly "root"):
// Displacement reports how far it has traveled between two frames, so a
// lunge or a dodge roll moves the character exactly as drawn.
package lottiesockets

import (
	"encoding/json"
	"fmt"
	"math"

	lottie "github.com/shibukawa/lottie-go"
)

// File is the bundle member this plugin claims.
const File = "extensions/sockets.json"

// Z says which side of the character an attached item draws on.
type Z string

const (
	ZFront  Z = "front"
	ZBehind Z = "behind"
)

// Rotate says whether the socket's angle follows the bound layer.
type Rotate string

const (
	// RotateFollow (the empty default) hands the layer's rotation on.
	RotateFollow Rotate = ""
	// RotateNone pins the angle regardless of the layer — a health bar
	// over the head stays level however the head tilts.
	RotateNone Rotate = "none"
)

// Socket binds one game-facing name to a layer.
type Socket struct {
	Name string `json:"name"`
	// Layer is the bound layer's name; empty means "same as Name".
	Layer string `json:"layer,omitempty"`
	// Z hints where the attached item draws; empty reads as front.
	Z Z `json:"z,omitempty"`
	// DX and DY nudge the socket in the layer's local space — a grip
	// adjustment that rotates and scales with the hand. The layer stays
	// the position's source of truth; this is the editor-side trim for
	// when re-authoring the animation is not worth it.
	DX float64 `json:"dx,omitempty"`
	DY float64 `json:"dy,omitempty"`
	// DR trims the returned angle, in degrees: added onto the layer's
	// rotation, or the whole angle when Rotate is none.
	DR float64 `json:"dr,omitempty"`
	// Rotate picks whether the angle follows the layer at all.
	Rotate Rotate `json:"rotate,omitempty"`

	Extra lottie.ExtraFields `json:"-"`
}

func (s Socket) MarshalJSON() ([]byte, error) {
	type alias Socket
	return lottie.MarshalWithExtra(alias(s), s.Extra)
}

func (s *Socket) UnmarshalJSON(data []byte) error {
	type alias Socket
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := lottie.UnmarshalExtra(data, a)
	if err != nil {
		return err
	}
	*s = Socket(a)
	s.Extra = extra
	return nil
}

// LayerName is the layer this socket reads, applying the empty-Layer
// default.
func (s *Socket) LayerName() string {
	if s.Layer != "" {
		return s.Layer
	}
	return s.Name
}

// Set is the bundle's socket table.
type Set struct {
	Sockets []Socket `json:"sockets"`

	Extra lottie.ExtraFields `json:"-"`
}

func (s Set) MarshalJSON() ([]byte, error) {
	type alias Set
	return lottie.MarshalWithExtra(alias(s), s.Extra)
}

func (s *Set) UnmarshalJSON(data []byte) error {
	type alias Set
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	extra, err := lottie.UnmarshalExtra(data, a)
	if err != nil {
		return err
	}
	*s = Set(a)
	s.Extra = extra
	return nil
}

// Find returns the socket with the given name.
func (s *Set) Find(name string) (*Socket, bool) {
	for i := range s.Sockets {
		if s.Sockets[i].Name == name {
			return &s.Sockets[i], true
		}
	}
	return nil, false
}

// ParseSet decodes one socket table document.
func ParseSet(data []byte) (*Set, error) {
	var s Set
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("lottiesockets: %w", err)
	}
	return &s, nil
}

// Load parses the bundle's socket table. Each call parses afresh; a caller
// editing in place keeps the pointer and Stores it back.
func Load(b *lottie.Bundle) (*Set, error) {
	data, ok := b.ExtensionFile(File)
	if !ok {
		return nil, fmt.Errorf("lottiesockets: no socket table in bundle")
	}
	return ParseSet(data)
}

// Store writes the socket table into the bundle, where Encode carries it —
// with or without this plugin imported at that point.
func Store(b *lottie.Bundle, s *Set) error {
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("lottiesockets: %w", err)
	}
	return b.SetExtensionFile(File, data)
}

// Remove drops the socket table from the bundle.
func Remove(b *lottie.Bundle) {
	b.RemoveExtensionFile(File)
}

// Placed is one socket resolved against an animation at a frame.
type Placed struct {
	Socket
	lottie.LayerPlacement
}

// Mirrored reflects the placement across x = axis (rule: position mirrors,
// angle negates, Z stays — front is still front).
func (p Placed) Mirrored(axis float64) Placed {
	p.LayerPlacement = p.LayerPlacement.Mirrored(axis)
	return p
}

// At resolves one socket by name. It reports false when the set has no
// such socket or the animation no such layer.
func (s *Set) At(a *lottie.Animation, frame float64, name string) (Placed, bool) {
	sock, ok := s.Find(name)
	if !ok {
		return Placed{}, false
	}
	pl, ok := a.LayerPlacement(sock.LayerName(), frame)
	if !ok {
		return Placed{}, false
	}
	return place(*sock, pl), true
}

// All resolves every socket whose layer the animation has, in table order.
// A clip missing some layer simply lacks that socket, which is normal
// across a character's clips.
func (s *Set) All(a *lottie.Animation, frame float64) []Placed {
	var out []Placed
	for i := range s.Sockets {
		sock := &s.Sockets[i]
		pl, ok := a.LayerPlacement(sock.LayerName(), frame)
		if !ok {
			continue
		}
		out = append(out, place(*sock, pl))
	}
	return out
}

// place applies the socket's trims through the layer transform: the
// positional nudge rides the layer's rotation and scale like anything
// attached (whatever Rotate says — the attachment point is on the hand
// either way), and the angle then follows the layer or stays pinned.
func place(sock Socket, pl lottie.LayerPlacement) Placed {
	if sock.DX != 0 || sock.DY != 0 {
		sin, cos := math.Sincos(pl.Angle)
		lx, ly := pl.ScaleX*sock.DX, pl.ScaleY*sock.DY
		pl.X += cos*lx - sin*ly
		pl.Y += sin*lx + cos*ly
	}
	dr := sock.DR * math.Pi / 180
	if sock.Rotate == RotateNone {
		pl.Angle = dr
	} else {
		pl.Angle += dr
	}
	return Placed{Socket: sock, LayerPlacement: pl}
}

// Displacement reports how far the named layer moved between two frames of
// the animation — the root-motion query. Games keep the last frame they
// applied and diff each Update; across a loop wrap, apply the remainder to
// the clip end and re-base at the start. It reports false when the
// animation has no such layer.
func Displacement(a *lottie.Animation, layer string, from, to float64) (dx, dy float64, ok bool) {
	p0, ok0 := a.LayerPlacement(layer, from)
	p1, ok1 := a.LayerPlacement(layer, to)
	if !ok0 || !ok1 {
		return 0, 0, false
	}
	return p1.X - p0.X, p1.Y - p0.Y, true
}
