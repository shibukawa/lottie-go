package lottie

import (
	"bytes"
	"math"
	"testing"
	"time"
)

// drawFramePlayer builds a playing clip player with a controllable clock.
// clipAnimation runs at 60fps with TPS 60, so one tick is one frame.
func drawFramePlayer(t *testing.T, frames int) (*Player, *time.Time) {
	t.Helper()
	anim, err := Decode(bytes.NewReader(clipAnimation(frames, "")))
	if err != nil {
		t.Fatal(err)
	}
	p := anim.NewPlayer()
	now := time.Unix(1000, 0)
	p.now = func() time.Time { return now }
	return p, &now
}

// A Draw between ticks sees the fraction of the tick that has elapsed —
// the smooth read a 144Hz display needs.
func TestDrawFrameReadsIntoTheTick(t *testing.T) {
	p, now := drawFramePlayer(t, 60)
	p.Update() // frame 1
	*now = now.Add(8 * time.Millisecond)
	got := p.DrawFrame()
	want := 1 + 0.008*60
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("DrawFrame = %v; want %v", got, want)
	}
	if p.Frame() != 1 {
		t.Errorf("Frame moved to %v; DrawFrame must not advance the cursor", p.Frame())
	}
}

// When Update stalls, the read stops one tick ahead instead of running on.
func TestDrawFrameCapsAtOneTick(t *testing.T) {
	p, now := drawFramePlayer(t, 60)
	p.Update() // frame 1
	*now = now.Add(300 * time.Millisecond)
	if got := p.DrawFrame(); got != 2 {
		t.Errorf("DrawFrame = %v; want 2 (one tick ahead)", got)
	}
}

// Paused playback has nothing to smooth.
func TestDrawFrameHoldsWhilePaused(t *testing.T) {
	p, now := drawFramePlayer(t, 60)
	p.Update()
	p.Pause()
	*now = now.Add(8 * time.Millisecond)
	if got := p.DrawFrame(); got != p.Frame() {
		t.Errorf("DrawFrame = %v while paused; want Frame %v", got, p.Frame())
	}
}

// Before the first Update there is no tick to read into.
func TestDrawFrameBeforeFirstUpdate(t *testing.T) {
	p, _ := drawFramePlayer(t, 60)
	if got := p.DrawFrame(); got != p.Frame() {
		t.Errorf("DrawFrame = %v before any Update; want Frame %v", got, p.Frame())
	}
}

// Without a loop the read holds at the range end, where advance will park.
func TestDrawFrameHoldsAtTheEnd(t *testing.T) {
	p, now := drawFramePlayer(t, 2)
	p.Update() // frame 1, one frame short of the end
	*now = now.Add(300 * time.Millisecond)
	if got := p.DrawFrame(); got != 2 {
		t.Errorf("DrawFrame = %v; want to hold at out 2", got)
	}
}

// A looping read wraps ahead of Update, the way advance will.
func TestDrawFrameWrapsWhileLooping(t *testing.T) {
	p, now := drawFramePlayer(t, 2)
	p.SetLoop(true)
	p.Update() // frame 1
	*now = now.Add(300 * time.Millisecond)
	if got := p.DrawFrame(); got != 0 {
		t.Errorf("DrawFrame = %v; want wrap to 0", got)
	}
}

// On the final counted pass advance finishes at the boundary; the read
// must not show the start of a pass that will never play.
func TestDrawFrameHoldsOnFinalCountedPass(t *testing.T) {
	p, now := drawFramePlayer(t, 2)
	p.SetLoop(true)
	p.SetLoopCount(1)
	p.Update() // frame 1, last pass
	*now = now.Add(300 * time.Millisecond)
	if got := p.DrawFrame(); got != 2 {
		t.Errorf("DrawFrame = %v; want to hold at out 2", got)
	}
}

// Reverse playback reads backwards into the tick.
func TestDrawFrameReverse(t *testing.T) {
	p, now := drawFramePlayer(t, 60)
	p.SetReverse(true)
	p.Rewind() // frame 60
	p.Update() // frame 59
	*now = now.Add(8 * time.Millisecond)
	got := p.DrawFrame()
	want := 59 - 0.008*60
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("DrawFrame = %v; want %v", got, want)
	}
}

// Speed scales the read the same way it scales the tick.
func TestDrawFrameFollowsSpeed(t *testing.T) {
	p, now := drawFramePlayer(t, 60)
	p.SetSpeed(2)
	p.Update() // frame 2
	*now = now.Add(8 * time.Millisecond)
	got := p.DrawFrame()
	want := 2 + 0.008*60*2
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("DrawFrame = %v; want %v", got, want)
	}
}
