package lottie

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// DrawOptions configures Player.Draw. The zero value draws at the origin
// with no color modification and anti-aliasing enabled.
type DrawOptions struct {
	// GeoM is applied on top of the animation's own transforms.
	GeoM ebiten.GeoM
	// ColorScale is applied to every drawn element.
	ColorScale ebiten.ColorScale
	// DisableAntiAlias turns off anti-aliased path rendering.
	DisableAntiAlias bool
}

// Player is a playback instance of an Animation. A Player is not safe for
// concurrent use; drive it from the game's Update/Draw loop.
type Player struct {
	anim    *Animation
	r       renderer
	frame   float64
	playing bool
	loop    bool
	speed   float64

	// Playback limited to a frame range, typically a Marker. Inactive while
	// rangeEnd is not greater than rangeStart.
	rangeStart float64
	rangeEnd   float64

	reverse   bool
	loopCount int // 0: unlimited while loop is set
	loopsDone int

	onComplete     func()
	onLoopComplete func()
	onMarker       func(Marker)
}

// NewPlayer creates an independent playback instance.
func (a *Animation) NewPlayer() *Player {
	return &Player{
		anim:    a,
		frame:   a.inPoint,
		playing: true,
		speed:   1,
	}
}

// Animation returns the animation this player renders. It is needed to size
// or place the drawing, since Draw applies whatever transform it is given.
func (p *Player) Animation() *Animation { return p.anim }

// SetLoop makes playback restart from the beginning after the last frame.
func (p *Player) SetLoop(loop bool) { p.loop = loop }

// SetSpeed sets the playback rate. 1.0 is normal speed; negative values are
// clamped to 0. Use SetReverse to play backwards.
func (p *Player) SetSpeed(s float64) {
	if s < 0 {
		s = 0
	}
	p.speed = s
}

// SetReverse plays from the end of the range towards its start.
func (p *Player) SetReverse(r bool) { p.reverse = r }

// IsReverse reports whether playback runs backwards.
func (p *Player) IsReverse() bool { return p.reverse }

// SetLoopCount stops playback after n passes. Zero, the default, loops
// without limit while SetLoop is set. It has no effect otherwise.
func (p *Player) SetLoopCount(n int) {
	if n < 0 {
		n = 0
	}
	p.loopCount = n
	p.loopsDone = 0
}

// SetRange limits playback to the frames in [start, end), which is how a
// Marker names a segment of a longer animation. An empty or inverted range
// clears the limit.
func (p *Player) SetRange(start, end float64) {
	if end <= start {
		p.rangeStart, p.rangeEnd = 0, 0
	} else {
		p.rangeStart, p.rangeEnd = start, end
	}
	p.loopsDone = 0
	p.frame = p.clampFrame(p.frame)
}

// SetMarkerRange limits playback to a named marker of the player's
// animation. It reports false when the animation has no such marker.
func (p *Player) SetMarkerRange(name string) bool {
	m, ok := p.anim.Marker(name)
	if !ok || m.End <= m.Start {
		return false
	}
	p.SetRange(m.Start, m.End)
	return true
}

// ClearRange restores playback over the whole animation.
func (p *Player) ClearRange() { p.SetRange(0, 0) }

// Range returns the frames playback is limited to. Without a limit it
// returns the animation's own in and out points.
func (p *Player) Range() (start, end float64) { return p.bounds() }

// bounds returns the active frame range.
func (p *Player) bounds() (in, out float64) {
	if p.rangeEnd > p.rangeStart {
		return p.rangeStart, p.rangeEnd
	}
	return p.anim.inPoint, p.anim.outPoint
}

// OnComplete registers a function to run when playback reaches the end of
// its final pass. It runs during Update; keep it short and do not drive the
// player from inside it.
func (p *Player) OnComplete(f func()) { p.onComplete = f }

// OnLoopComplete registers a function to run at the end of each looping
// pass. It runs during Update.
func (p *Player) OnLoopComplete(f func()) { p.onLoopComplete = f }

// OnMarker registers a function to run when playback passes a marker's
// start frame, which is how a game hangs a footstep, a hit frame, or any
// other cue off the animation itself rather than off a timer. Unnamed
// markers are skipped. It runs during Update; keep it short and do not
// drive the player from inside it.
func (p *Player) OnMarker(f func(Marker)) { p.onMarker = f }

// emitMarkers reports the markers whose start lies in the half-open span
// [from, to) the cursor just moved through. Reverse playback passes the
// span the same way round, so a marker fires whichever way it is crossed.
func (p *Player) emitMarkers(from, to float64) {
	if p.onMarker == nil || to <= from {
		return
	}
	for _, m := range p.anim.markers {
		if m.Name == "" {
			continue
		}
		if from <= m.Start && m.Start < to {
			p.onMarker(m)
		}
	}
}

// Play resumes playback.
func (p *Player) Play() { p.playing = true }

// Pause stops advancing time; Draw keeps rendering the current frame.
func (p *Player) Pause() { p.playing = false }

// IsPlaying reports whether Update advances the animation.
func (p *Player) IsPlaying() bool { return p.playing }

// Rewind returns the cursor to where playback starts: the range start, or
// its end when playing in reverse. It also resets the loop count.
func (p *Player) Rewind() {
	in, out := p.bounds()
	if p.reverse {
		p.frame = out
	} else {
		p.frame = in
	}
	p.loopsDone = 0
}

// Seek jumps to the given time from the start of the range.
func (p *Player) Seek(t time.Duration) {
	in, _ := p.bounds()
	p.frame = p.clampFrame(in + t.Seconds()*p.anim.frameRate)
}

// Position returns the current playback position from the start of the
// range.
func (p *Player) Position() time.Duration {
	in, _ := p.bounds()
	return time.Duration((p.frame - in) / p.anim.frameRate * float64(time.Second))
}

// SetFrame jumps to an absolute frame of the animation, clamped to the
// active range.
func (p *Player) SetFrame(f float64) { p.frame = p.clampFrame(f) }

// Frame returns the current absolute frame.
func (p *Player) Frame() float64 { return p.frame }

// SetProgress jumps to a fraction of the active range, from 0 to 1.
func (p *Player) SetProgress(v float64) {
	in, out := p.bounds()
	p.frame = p.clampFrame(in + v*(out-in))
}

// Progress returns how far through the active range playback is, from 0
// to 1.
func (p *Player) Progress() float64 {
	in, out := p.bounds()
	if out <= in {
		return 0
	}
	return (p.frame - in) / (out - in)
}

// Duration returns the length of the active range.
func (p *Player) Duration() time.Duration {
	in, out := p.bounds()
	return time.Duration((out - in) / p.anim.frameRate * float64(time.Second))
}

// Update advances the animation by one tick (1/TPS seconds, scaled by
// speed). Call it from ebiten.Game's Update.
func (p *Player) Update() {
	if !p.playing {
		return
	}
	tps := float64(ebiten.TPS())
	if tps <= 0 {
		tps = 60
	}
	delta := p.anim.frameRate / tps * p.speed
	if p.reverse {
		delta = -delta
	}
	p.advance(delta)
}

// advance moves the cursor, wrapping at the range boundary while passes
// remain and otherwise stopping there and reporting completion.
func (p *Player) advance(delta float64) {
	in, out := p.bounds()
	span := out - in
	if span <= 0 {
		p.frame = in
		return
	}
	prev := p.frame
	f := p.frame + delta
	if f >= in && f < out {
		p.frame = f
		p.emitMarkers(min(prev, f), max(prev, f))
		return
	}
	// The cursor left the range: it swept to the boundary it ran past, and
	// on a wrap it continues from the other end.
	if f >= out {
		p.emitMarkers(prev, out)
	} else {
		p.emitMarkers(in, prev)
	}
	// How many times the cursor ran off an end. Normally one; a very large
	// delta or a very short range can cross several at once.
	crossed := 1
	if f >= out {
		crossed += int((f - out) / span)
	} else {
		crossed += int((in - f) / span)
	}

	switch {
	case !p.loop:
		p.finish(f, in, out)
	case p.loopCount > 0 && crossed >= p.loopCount-p.loopsDone:
		p.fireLoops(p.loopCount - p.loopsDone)
		p.loopsDone = p.loopCount
		p.finish(f, in, out)
	default:
		p.fireLoops(crossed)
		p.loopsDone += crossed
		p.frame = in + mod(f-in, span)
		if delta >= 0 {
			p.emitMarkers(in, p.frame)
		} else {
			p.emitMarkers(p.frame, out)
		}
	}
}

// finish parks the cursor on the end it ran past and stops.
func (p *Player) finish(f, in, out float64) {
	if f >= out {
		p.frame = out
	} else {
		p.frame = in
	}
	p.playing = false
	if p.onComplete != nil {
		p.onComplete()
	}
}

func (p *Player) fireLoops(n int) {
	if p.onLoopComplete == nil {
		return
	}
	for range n {
		p.onLoopComplete()
	}
}

// clampFrame confines f to the active range, wrapping when looping. It does
// not report completion; that belongs to advance.
func (p *Player) clampFrame(f float64) float64 {
	in, out := p.bounds()
	if f < in {
		return in
	}
	if f >= out {
		if p.loop && out > in {
			span := out - in
			f = in + mod(f-in, span)
		} else {
			// Hold on the final renderable moment.
			f = out
			p.playing = false
		}
	}
	return f
}

func mod(a, b float64) float64 {
	m := a - float64(int(a/b))*b
	if m < 0 {
		m += b
	}
	return m
}

// Draw renders the current frame. opts may be nil.
func (p *Player) Draw(dst *ebiten.Image, opts *DrawOptions) {
	root := identityMatrix
	var cs ebiten.ColorScale
	antialias := true
	if opts != nil {
		root = matrixFromGeoM(opts.GeoM)
		cs = opts.ColorScale
		antialias = !opts.DisableAntiAlias
	}
	f := p.frame
	// The out point is exclusive; render the last covered frame instead.
	if _, out := p.bounds(); f >= out {
		f = out - 1e-6
	}
	p.r.render(dst, p.anim, f, root, cs, antialias)
}
