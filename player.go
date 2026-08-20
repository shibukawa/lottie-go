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

// SetLoop makes playback restart from the beginning after the last frame.
func (p *Player) SetLoop(loop bool) { p.loop = loop }

// SetSpeed sets the playback rate. 1.0 is normal speed; negative values are
// clamped to 0.
func (p *Player) SetSpeed(s float64) {
	if s < 0 {
		s = 0
	}
	p.speed = s
}

// Play resumes playback.
func (p *Player) Play() { p.playing = true }

// Pause stops advancing time; Draw keeps rendering the current frame.
func (p *Player) Pause() { p.playing = false }

// IsPlaying reports whether Update advances the animation.
func (p *Player) IsPlaying() bool { return p.playing }

// Seek jumps to the given time from the start of the animation.
func (p *Player) Seek(t time.Duration) {
	f := p.anim.inPoint + t.Seconds()*p.anim.frameRate
	p.frame = p.clampFrame(f)
}

// Position returns the current playback position from the start.
func (p *Player) Position() time.Duration {
	return time.Duration((p.frame - p.anim.inPoint) / p.anim.frameRate * float64(time.Second))
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
	p.frame += p.anim.frameRate / tps * p.speed
	p.frame = p.clampFrame(p.frame)
}

func (p *Player) clampFrame(f float64) float64 {
	in, out := p.anim.inPoint, p.anim.outPoint
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
	if f >= p.anim.outPoint {
		f = p.anim.outPoint - 1e-6
	}
	p.r.render(dst, p.anim, f, root, cs, antialias)
}
