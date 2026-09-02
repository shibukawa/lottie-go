package lottie

import (
	"image"
	"math"
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
	// Smooth renders at DrawFrame instead of the tick cursor, so on a
	// display faster than the tick rate each Draw lands on its own point
	// of the timeline.
	Smooth bool
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
	onFrameSpan    func(from, to float64)

	// Draw-side smoothing; see DrawFrame.
	tickAt time.Time
	now    func() time.Time // nil means time.Now; tests substitute

	// Idle snapshot cache; see drawSnapshot.
	snapKey      snapshotKey
	snapKeyValid bool
	snap         *ebiten.Image
	snapOffset   image.Point
	snapEmpty    bool
	snapOff      bool
}

// snapshotKey captures every input Draw depends on. While it repeats, the
// output repeats, so the previous frame's pixels can be reused.
type snapshotKey struct {
	frame      float64
	root       matrix
	cs         ebiten.ColorScale
	antialias  bool
	dst        image.Rectangle
	generation int
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

// maxLoopCallbacksPerUpdate bounds how many OnLoopComplete calls one Update
// makes. A runaway speed or a hair-thin range can cross the range end many
// thousands of times in a single tick; the loop count still advances by
// every crossing, but the callback fires at most this often, so a game's
// per-loop hook cannot hang the game loop. It mirrors the state machine
// player's maxTransitionsPerUpdate.
const maxLoopCallbacksPerUpdate = 16

// maxLoopCrossings bounds how many range crossings one Update accounts for,
// which keeps the loop arithmetic inside int for any finite delta.
const maxLoopCrossings = 1 << 30

// SetSpeed sets the playback rate. 1.0 is normal speed; negative values are
// clamped to 0. NaN and infinite rates are ignored — the speed stays as it
// was — since neither names a point on the timeline. Use SetReverse to play
// backwards.
func (p *Player) SetSpeed(s float64) {
	if math.IsNaN(s) || math.IsInf(s, 0) {
		return
	}
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

// OnFrameSpan reports every half-open span [from, to) the cursor sweeps
// during Update, including the partial spans of a loop wrap. Reverse
// playback passes spans the same way round as forward. It exists for
// extension packages that fire their own cues off frame positions the way
// markers fire (plugin/events); seeks and SetFrame jump without sweeping,
// exactly as they do for markers.
func (p *Player) OnFrameSpan(f func(from, to float64)) { p.onFrameSpan = f }

// emitMarkers reports the markers whose start lies in the half-open span
// [from, to) the cursor just moved through. Reverse playback passes the
// span the same way round, so a marker fires whichever way it is crossed.
// The span also feeds OnFrameSpan, keeping every cue mechanism on one
// definition of "moved through".
func (p *Player) emitMarkers(from, to float64) {
	if to <= from {
		return
	}
	if p.onFrameSpan != nil {
		p.onFrameSpan(from, to)
	}
	if p.onMarker == nil {
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
	p.tickAt = p.timeNow()
	if !p.playing {
		return
	}
	delta := p.anim.frameRate / tickTPS() * p.speed
	if p.reverse {
		delta = -delta
	}
	p.advance(delta)
}

func (p *Player) timeNow() time.Time {
	if p.now != nil {
		return p.now()
	}
	return time.Now()
}

// tickTPS is the tick rate Update advances by. With SyncWithFPS the
// configured rate is -1 and ticks follow the display, so the measured rate
// stands in; headless, with neither, 60.
func tickTPS() float64 {
	if tps := float64(ebiten.TPS()); tps > 0 {
		return tps
	}
	if tps := ebiten.ActualTPS(); tps > 0 {
		return tps
	}
	return 60
}

// DrawFrame is the frame for draw-side reads: the tick cursor plus however
// far into the current tick the wall clock is, so on a 144Hz display each
// Draw sees its own point of the timeline instead of the 60Hz staircase
// Frame gives. Because the animation is a pure function of the fractional
// frame, this samples the real curves — no linear approximation. Use it for
// what the eye tracks between ticks: rendering (DrawOptions.Smooth does the
// same for Draw itself) and attachment reads in Draw, such as feeding a
// particle emitter from a socket. Gameplay — hitboxes, root motion, cues —
// stays on Frame, the value Update's events are ordered against.
//
// It reads ahead of Frame by less than one tick and never past where Update
// will go: the range end holds and a loop wraps, exactly as advance will
// decide, but no callbacks fire — those belong to Update. While paused it
// returns Frame.
func (p *Player) DrawFrame() float64 {
	if !p.playing || p.tickAt.IsZero() {
		return p.frame
	}
	elapsed := p.timeNow().Sub(p.tickAt).Seconds()
	if elapsed < 0 {
		elapsed = 0
	}
	if tick := 1 / tickTPS(); elapsed > tick {
		elapsed = tick
	}
	delta := elapsed * p.anim.frameRate * p.speed
	if p.reverse {
		delta = -delta
	}
	return p.peekFrame(p.frame + delta)
}

// peekFrame confines f to the active range the way advance would move there
// — wrapping while looping, holding at the boundary otherwise — without
// touching playback state.
func (p *Player) peekFrame(f float64) float64 {
	in, out := p.bounds()
	if in <= f && f < out {
		return f
	}
	// On the final counted pass advance finishes at the boundary rather
	// than wrapping; hold there like it will.
	wraps := p.loop && (p.loopCount == 0 || p.loopsDone < p.loopCount-1)
	if wraps && out > in {
		return in + mod(f-in, out-in)
	}
	if f < in {
		return in
	}
	return out
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
	if math.IsNaN(f) || math.IsInf(f, 0) {
		// Nothing finite to sweep through: park at the end the value lies
		// beyond, the way finish does, and keep playing — the cursor must
		// never hold a NaN.
		if math.IsInf(f, 1) {
			p.frame = out
		} else {
			p.frame = in
		}
		return
	}
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
		crossed += int(math.Min((f-out)/span, maxLoopCrossings))
	} else {
		crossed += int(math.Min((in-f)/span, maxLoopCrossings))
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
	for range min(n, maxLoopCallbacksPerUpdate) {
		p.onLoopComplete()
	}
}

// clampFrame confines f to the active range, wrapping when looping. It does
// not report completion, nor pause: a seek that lands on the boundary holds
// there, and the next Update runs off the end and lets advance finish —
// otherwise a scrub to 100% would silently stop without OnComplete.
func (p *Player) clampFrame(f float64) float64 {
	in, out := p.bounds()
	if math.IsNaN(f) {
		return in
	}
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
		}
	}
	return f
}

func mod(a, b float64) float64 {
	m := math.Mod(a, b)
	if math.IsNaN(m) {
		return 0
	}
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
	f := p.frame
	if opts != nil {
		root = matrixFromGeoM(opts.GeoM)
		cs = opts.ColorScale
		antialias = !opts.DisableAntiAlias
		if opts.Smooth {
			f = p.DrawFrame()
		}
	}
	// The out point is exclusive; render the last covered frame instead.
	if _, out := p.bounds(); f >= out {
		f = out - 1e-6
	}
	if p.drawSnapshot(dst, f, root, cs, antialias) {
		return
	}
	p.r.render(dst, p.anim, f, root, cs, antialias)
}

// HitTest reports whether the point lies within a named layer's bounds at
// the current frame, in the animation's composition coordinates — the same
// space Draw's GeoM maps to the screen, so apply the inverse of that
// transform to a cursor position first. Nested precompositions are searched
// too. Layers whose extent cannot be known up front (text) never hit.
func (p *Player) HitTest(name string, x, y float64) bool {
	if name == "" {
		return false
	}
	pt := image.Pt(int(math.Floor(x)), int(math.Floor(y)))
	var walk func(layers []*layerNode, f float64, root matrix) bool
	walk = func(layers []*layerNode, f float64, root matrix) bool {
		for _, l := range layers {
			if l.hidden || l.matteOnly || f < l.ip || f >= l.op {
				continue
			}
			lt := l.localTime(f)
			mat := root.mul(layerMatrix(l, f, 0))
			if l.name == name {
				if b, ok := p.r.layerBounds(l, f, lt, mat); ok && pt.In(b) {
					return true
				}
			}
			if l.typ == 0 && len(l.comp) > 0 {
				if walk(l.comp, l.compTime(lt, p.anim.frameRate), mat) {
					return true
				}
			}
		}
		return false
	}
	p.r.anim = p.anim
	return walk(p.anim.layers, p.frame, identityMatrix)
}

// SetSnapshotCache toggles the idle snapshot cache (default on). While a
// player's draw inputs repeat — same frame, transform, color, and
// destination — the frame is baked once and reused, which reduces an idle
// player to a single texture draw and no per-frame evaluation.
func (p *Player) SetSnapshotCache(enabled bool) {
	p.snapOff = !enabled
	if !enabled {
		p.dropSnapshot()
		p.snapKeyValid = false
	}
}

// drawSnapshot serves Draw from the cache when the key repeats. The first
// frame with a new key renders directly and only records the key; the bake
// happens on the second frame, so a continuously animating player never
// pays for one. Each bake uses a fresh image: Ebitengine returns an image
// that is only ever read to its shared source atlas after ~10 frames, at
// which point the composites of every idle player merge into one draw call,
// and re-baking into a kept image would defer that rejoin exponentially.
func (p *Player) drawSnapshot(dst *ebiten.Image, f float64, root matrix, cs ebiten.ColorScale, antialias bool) bool {
	// A textured player is never snapshotted: a bound texture may be a
	// render target the game redraws between frames, which no key sees.
	if p.snapOff || !p.anim.snapshotOK || len(p.r.paints) > 0 {
		return false
	}
	key := snapshotKey{f, root, cs, antialias, dst.Bounds(), p.anim.generation}
	if !p.snapKeyValid || key != p.snapKey {
		p.snapKey = key
		p.snapKeyValid = true
		p.dropSnapshot()
		return false
	}
	if p.snap == nil && !p.snapEmpty {
		b, ok := p.r.animBounds(p.anim, f, root)
		if ok {
			b = b.Intersect(dst.Bounds())
		} else {
			b = dst.Bounds()
		}
		if b.Empty() {
			p.snapEmpty = true
			return true
		}
		img := ebiten.NewImage(b.Dx(), b.Dy())
		shift := identityMatrix.translate(-float64(b.Min.X), -float64(b.Min.Y))
		p.r.render(img, p.anim, f, shift.mul(root), cs, antialias)
		p.snap = img
		p.snapOffset = b.Min
	}
	if p.snapEmpty {
		return true
	}
	var op ebiten.DrawImageOptions
	op.GeoM.Translate(float64(p.snapOffset.X), float64(p.snapOffset.Y))
	dst.DrawImage(p.snap, &op)
	return true
}

func (p *Player) dropSnapshot() {
	if p.snap != nil {
		p.snap.Deallocate()
		p.snap = nil
	}
	p.snapEmpty = false
}
