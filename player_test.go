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

// A NaN or infinite rate names no point on the timeline; SetSpeed leaves
// the speed alone rather than poisoning every later frame.
func TestSetSpeedIgnoresNonFinite(t *testing.T) {
	p, _ := drawFramePlayer(t, 60)
	p.SetSpeed(2)
	for _, s := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		p.SetSpeed(s)
		if p.speed != 2 {
			t.Errorf("SetSpeed(%v) changed the speed to %v", s, p.speed)
		}
	}
	p.SetSpeed(-1)
	if p.speed != 0 {
		t.Errorf("negative speed = %v; want 0", p.speed)
	}
}

// A hair-thin range crosses its end many times per tick. The loop count
// keeps every crossing, but the callback fires a bounded number of times,
// so a per-loop hook cannot stall the tick.
func TestLoopCallbacksCappedPerUpdate(t *testing.T) {
	p, _ := drawFramePlayer(t, 60)
	p.SetLoop(true)
	p.SetRange(10, 10.001)
	calls := 0
	p.OnLoopComplete(func() { calls++ })
	p.Update() // one frame of delta across a 0.001-frame range
	if calls != maxLoopCallbacksPerUpdate {
		t.Errorf("OnLoopComplete ran %d times; want the cap of %d", calls, maxLoopCallbacksPerUpdate)
	}
	if p.loopsDone < 900 {
		t.Errorf("loopsDone = %d; every crossing must still count", p.loopsDone)
	}
	if f := p.Frame(); f < 10 || f >= 10.001 {
		t.Errorf("Frame = %v; want inside the range", f)
	}
}

// The cursor must never hold a NaN: a seek with one lands at the range
// start, and an advance that produces one parks at the boundary.
func TestFrameNeverHoldsNaN(t *testing.T) {
	p, _ := drawFramePlayer(t, 60)
	p.SetFrame(math.NaN())
	if p.Frame() != 0 {
		t.Errorf("SetFrame(NaN): Frame = %v; want 0", p.Frame())
	}
	p.SetProgress(math.NaN())
	if p.Frame() != 0 {
		t.Errorf("SetProgress(NaN): Frame = %v; want 0", p.Frame())
	}
	p.frame = math.Inf(1)
	p.Update()
	if f := p.Frame(); math.IsNaN(f) || math.IsInf(f, 0) {
		t.Errorf("Update from +Inf left Frame = %v", f)
	}
	p.SetLoop(true)
	p.frame = math.Inf(-1)
	p.Update()
	if f := p.Frame(); f != 0 {
		t.Errorf("Update from -Inf while looping left Frame = %v; want 0", f)
	}
}
