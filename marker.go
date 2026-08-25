package lottie

import "time"

// Marker is a named frame range declared in a Lottie document's "markers"
// array. Editors use markers to label sections of a single animation, and
// dotLottie state machines name one to play just that section.
//
// End is exclusive, matching the animation's own out point. A marker with no
// duration is a point marker: End equals Start.
type Marker struct {
	Name  string
	Start float64 // first frame
	End   float64 // one past the last frame
}

// Markers returns the animation's markers in document order.
func (a *Animation) Markers() []Marker {
	return append([]Marker(nil), a.markers...)
}

// Marker looks up a marker by name. Names are not required to be unique; the
// first match in document order wins.
func (a *Animation) Marker(name string) (Marker, bool) {
	for _, m := range a.markers {
		if m.Name == name {
			return m, true
		}
	}
	return Marker{}, false
}

// Duration returns the marker's length. It is zero for a point marker.
func (m Marker) Duration(a *Animation) time.Duration {
	return time.Duration((m.End - m.Start) / a.frameRate * float64(time.Second))
}

func buildMarkers(raw []rawMarker) []Marker {
	if len(raw) == 0 {
		return nil
	}
	out := make([]Marker, 0, len(raw))
	for _, r := range raw {
		// A marker without a name cannot be referenced, but keeping it
		// preserves index-based document order for round-tripping.
		d := r.Duration
		if d < 0 {
			d = 0
		}
		out = append(out, Marker{Name: r.Comment, Start: r.Time, End: r.Time + d})
	}
	return out
}
