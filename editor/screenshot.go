package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"strconv"

	"github.com/guigui-gui/guigui"
	"github.com/hajimehoshi/ebiten/v2"
)

// Screenshot support. Setting LSM_EDITOR_SCREENSHOT to a path renders the
// app for a few ticks, writes that frame as a PNG, and exits. It needs no
// display permissions, so it is how the UI is checked without a human
// looking at the window.
const (
	screenshotPathEnv  = "LSM_EDITOR_SCREENSHOT"
	screenshotTicksEnv = "LSM_EDITOR_SCREENSHOT_TICKS"
)

type screenshotGame struct {
	ebiten.Game

	path  string
	after int
	ticks int
	done  bool
}

func (g *screenshotGame) Update() error {
	if err := g.Game.Update(); err != nil {
		return err
	}
	g.ticks++
	if g.done {
		return ebiten.Termination
	}
	return nil
}

// Draw captures after delegating, so the frame holds the whole widget tree
// rather than just the root's own painting.
func (g *screenshotGame) Draw(screen *ebiten.Image) {
	g.Game.Draw(screen)
	if g.done || g.ticks < g.after {
		return
	}
	if err := writePNG(screen, g.path); err != nil {
		fmt.Fprintln(os.Stderr, "screenshot:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "screenshot: wrote %s after %d ticks\n", g.path, g.ticks)
	g.done = true
}

// LayoutF must be forwarded explicitly. Embedding ebiten.Game exposes only
// Layout, and guigui's game panics there because it expects the float
// variant to be used instead.
func (g *screenshotGame) LayoutF(outsideWidth, outsideHeight float64) (float64, float64) {
	if lf, ok := g.Game.(interface {
		LayoutF(float64, float64) (float64, float64)
	}); ok {
		return lf.LayoutF(outsideWidth, outsideHeight)
	}
	w, h := g.Game.Layout(int(outsideWidth), int(outsideHeight))
	return float64(w), float64(h)
}

func writePNG(src *ebiten.Image, path string) error {
	b := src.Bounds()
	img := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	src.ReadPixels(img.Pix)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// runWithOptionalScreenshot runs the app, wrapping it when a screenshot was
// requested.
func runWithOptionalScreenshot(root guigui.Widget, op *guigui.RunOptions) error {
	path := os.Getenv(screenshotPathEnv)
	if path == "" {
		return guigui.Run(root, op)
	}
	after := 30
	if v, err := strconv.Atoi(os.Getenv(screenshotTicksEnv)); err == nil && v > 0 {
		after = v
	}
	return guigui.RunWithCustomFunc(root, op, func(game ebiten.Game, options *ebiten.RunGameOptions) error {
		return ebiten.RunGameWithOptions(&screenshotGame{Game: game, path: path, after: after}, options)
	})
}
