package main

// Renders for the agent's eyes: the stage drawn by the real renderer, with
// or without the editing overlays, a contact sheet of the stage clip, or
// the whole window as the author sees it.

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	lottie "github.com/shibukawa/lottie-go"
)

const (
	renderMaxWidth = 2048
	renderUnit     = 24.0 // overlay unit size, what basicwidget uses at scale 1
)

var (
	checkerLight = color.RGBA{0xe6, 0xe6, 0xe6, 0xff}
	checkerDark  = color.RGBA{0xcc, 0xcc, 0xcc, 0xff}
)

// renderStage draws the stage at width pixels wide. frame, when set,
// draws that frame and restores the playhead afterwards.
func (s *mcpServer) renderStage(width int, frame *float64, overlays bool) ([]byte, error) {
	m := s.model
	anim := m.PreviewAnimation()
	if anim == nil {
		return nil, refuse("nothing is on stage", "select a clip or a machine first")
	}
	aw, ah := anim.Size()
	if aw <= 0 || ah <= 0 {
		return nil, refuse("the stage clip has no size", "")
	}
	if width <= 0 {
		width = aw
	}
	width = min(width, renderMaxWidth)
	scale := float64(width) / float64(aw)
	height := max(1, int(math.Round(float64(ah)*scale)))
	p := m.PreviewPlayer()
	restore := -1.0
	if frame != nil && p != nil {
		restore = p.Frame()
		p.SetFrame(*frame)
	}
	dst := ebiten.NewImage(width, height)
	defer dst.Deallocate()
	drawChecker(dst)
	var op lottie.DrawOptions
	op.GeoM.Scale(scale, scale)
	m.PreviewDraw(dst, &op)
	if overlays {
		tr := stageTransform{scale: scale}
		u := float32(renderUnit * min(scale, 1))
		if m.ShowRig() {
			drawRigOverlay(dst, m, tr, u)
		}
		switch {
		case m.PosesVisible():
			drawPoseOverlay(dst, m, tr, u)
		case m.ShapesVisible():
			drawShapeOverlay(dst, m, tr, u)
		case m.OverlayVisible():
			drawCollisionOverlay(dst, m, tr, u)
		}
	}
	if restore >= 0 {
		p.SetFrame(restore)
	}
	return encodePNG(dst)
}

// renderSheet tiles samples frames of the stage across the played range.
func (s *mcpServer) renderSheet(width, samples int) ([]byte, error) {
	m := s.model
	anim := m.PreviewAnimation()
	p := m.PreviewPlayer()
	if anim == nil || p == nil {
		return nil, refuse("nothing is on stage", "select a clip or a machine first")
	}
	if samples <= 0 {
		samples = 4
	}
	samples = min(samples, 16)
	aw, ah := anim.Size()
	if width <= 0 {
		width = min(aw, 256)
	}
	cols := min(samples, 4)
	rows := (samples + cols - 1) / cols
	scale := float64(width) / float64(aw)
	th := max(1, int(math.Round(float64(ah)*scale)))
	total := ebiten.NewImage(width*cols, th*rows)
	defer total.Deallocate()
	drawChecker(total)
	start, end := p.Range()
	restore := p.Frame()
	for i := range samples {
		frame := start
		if samples > 1 {
			frame = start + (end-start)*float64(i)/float64(samples)
		}
		p.SetFrame(frame)
		var op lottie.DrawOptions
		op.GeoM.Scale(scale, scale)
		op.GeoM.Translate(float64((i%cols)*width), float64((i/cols)*th))
		m.PreviewDraw(total, &op)
	}
	p.SetFrame(restore)
	return encodePNG(total)
}

// drawChecker paints a light checkerboard so transparent regions read as
// transparent rather than as a color the clip might also use.
func drawChecker(dst *ebiten.Image) {
	b := dst.Bounds()
	dst.Fill(checkerLight)
	const cell = 16
	for y := 0; y < b.Dy(); y += cell {
		for x := 0; x < b.Dx(); x += cell {
			if ((x/cell)+(y/cell))%2 == 0 {
				continue
			}
			vector.FillRect(dst, float32(x), float32(y), cell, cell, checkerDark, false)
		}
	}
}

func encodePNG(src *ebiten.Image) ([]byte, error) {
	b := src.Bounds()
	img := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	src.ReadPixels(img.Pix)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ---- window capture ----

// captureReq asks the game wrapper for the next drawn frame.
type captureReq struct {
	done chan []byte
}

// serveCapture is called from the wrapper's Draw with the finished frame.
func (s *mcpServer) serveCapture(screen *ebiten.Image) {
	select {
	case req := <-s.capture:
		data, err := encodePNG(screen)
		if err != nil {
			data = nil
		}
		req.done <- data
	default:
	}
}

// windowCaptureTimeout bounds how long a capture waits for a frame. Draw
// stops coming while the window is minimized or the loop is stalled, and
// a call that never returns would hold the transport forever.
const windowCaptureTimeout = 5 * time.Second

// renderWindow waits for the next frame. It must not be called on the game
// loop: the frame is only drawn once the loop is free.
func (s *mcpServer) renderWindow(ctx context.Context) ([]byte, error) {
	req := &captureReq{done: make(chan []byte, 1)}
	select {
	case s.capture <- req:
	default:
		return nil, refuse("a window capture is already pending", "retry")
	}
	timer := time.NewTimer(windowCaptureTimeout)
	defer timer.Stop()
	select {
	case data := <-req.done:
		if data == nil {
			return nil, refuse("window capture failed", "")
		}
		return data, nil
	case <-ctx.Done():
	case <-timer.C:
	}
	// Withdraw the request if Draw has not taken it yet, so the next
	// capture is not refused as "already pending"; if it was taken, the
	// buffered done channel absorbs the late frame.
	select {
	case <-s.capture:
	default:
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return nil, refuse("no frame was drawn in time", "is the window minimized? retry")
}

// mcpGame wraps the app so a window capture can read the finished frame.
type mcpGame struct {
	ebiten.Game
	srv *mcpServer
}

func (g *mcpGame) Draw(screen *ebiten.Image) {
	g.Game.Draw(screen)
	g.srv.serveCapture(screen)
}

// LayoutF is forwarded for the same reason screenshotGame forwards it.
func (g *mcpGame) LayoutF(outsideWidth, outsideHeight float64) (float64, float64) {
	if lf, ok := g.Game.(interface {
		LayoutF(float64, float64) (float64, float64)
	}); ok {
		return lf.LayoutF(outsideWidth, outsideHeight)
	}
	w, h := g.Game.Layout(int(outsideWidth), int(outsideHeight))
	return float64(w), float64(h)
}
