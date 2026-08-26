package lottieresolv

import (
	"sync"

	"github.com/solarlune/resolv"
)

// A Tracker owns one animated character's boxes. Each game frame, after
// advancing the lottie player, call SetOffset with the character's world
// position and Sync with the player's frame; the tracker inserts, moves,
// and removes resolv shapes to match what the track says is live. Queries
// then run through resolv as usual, or through Shapes for one character's
// boxes by tag.

// tag bits are global to resolv (64 exist in total) and resolv.NewTag
// allocates a fresh bit on every call, so the mapping from tag names must
// be deduplicated here.
var (
	tagMu   sync.Mutex
	tagBits = map[string]resolv.Tags{}
)

// Tag returns the resolv tag bit for a track tag name, allocating it on
// first use. Use it to query shapes this package inserted: a shape tagged
// "hurt" in the editor answers space queries for Tag("hurt").
func Tag(name string) resolv.Tags {
	tagMu.Lock()
	defer tagMu.Unlock()
	if t, ok := tagBits[name]; ok {
		return t
	}
	t := resolv.NewTag(name)
	tagBits[name] = t
	return t
}

// BoxData is stored as each inserted shape's Data, so a collision callback
// can tell which box of which character it hit.
type BoxData struct {
	// Tracker identifies the owner, for telling two characters apart.
	Tracker *Tracker
	// Index and Name are the box's position and name in the track.
	Index int
	Name  string
}

// Tracker keeps one track's live boxes present in a space.
type Tracker struct {
	space *resolv.Space
	track *Track

	offX, offY float64
	boxes      []trackedBox
}

type trackedBox struct {
	span  int // index of the span currently on stage, -1 when none
	shape resolv.IShape
	x, y  float64 // world anchor the shape was last placed at
}

// NewTracker prepares a tracker; nothing enters the space until Sync.
func NewTracker(space *resolv.Space, track *Track) *Tracker {
	return &Tracker{
		space: space,
		track: track,
		boxes: make([]trackedBox, len(track.Boxes)),
	}
}

// SetOffset places the animation's origin in world coordinates — the
// character's position. Takes effect on the next Sync.
func (t *Tracker) SetOffset(x, y float64) {
	t.offX, t.offY = x, y
}

// Sync makes the space match the track at the given frame, typically
// player.Frame(). Shapes are rebuilt only when their box steps to another
// span; a moving character just slides them.
func (t *Tracker) Sync(frame float64) {
	for i := range t.track.Boxes {
		b := &t.track.Boxes[i]
		tb := &t.boxes[i]
		span, live := b.SpanAt(frame)
		if !live {
			if tb.shape != nil {
				t.space.Remove(tb.shape)
				tb.shape = nil
			}
			tb.span = -1
			continue
		}
		sp := &b.Spans[span]
		wx, wy := t.offX+sp.X, t.offY+sp.Y
		if tb.shape != nil && tb.span == span {
			if wx != tb.x || wy != tb.y {
				tb.shape.Move(wx-tb.x, wy-tb.y)
				tb.x, tb.y = wx, wy
			}
			continue
		}
		if tb.shape != nil {
			t.space.Remove(tb.shape)
		}
		var shape resolv.IShape
		if b.Kind == KindCircle {
			shape = resolv.NewCircle(wx, wy, sp.R)
		} else {
			shape = resolv.NewRectangleFromTopLeft(wx, wy, sp.W, sp.H)
		}
		for _, name := range b.Tags {
			shape.Tags().Set(Tag(name))
		}
		shape.SetData(BoxData{Tracker: t, Index: i, Name: b.Name})
		t.space.Add(shape)
		tb.shape, tb.span, tb.x, tb.y = shape, span, wx, wy
	}
}

// Shapes returns this tracker's live shapes, keeping only those carrying
// at least one of the named tags; with none, every live shape returns.
func (t *Tracker) Shapes(tags ...string) []resolv.IShape {
	var mask resolv.Tags
	for _, name := range tags {
		mask |= Tag(name)
	}
	var out []resolv.IShape
	for i := range t.boxes {
		s := t.boxes[i].shape
		if s == nil {
			continue
		}
		if mask != 0 && !s.Tags().Has(mask) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// Remove pulls every live shape out of the space, for a character leaving
// play. The tracker stays usable: the next Sync re-inserts.
func (t *Tracker) Remove() {
	for i := range t.boxes {
		if t.boxes[i].shape != nil {
			t.space.Remove(t.boxes[i].shape)
			t.boxes[i].shape = nil
			t.boxes[i].span = -1
		}
	}
}
