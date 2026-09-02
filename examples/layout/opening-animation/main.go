// Command opening-animation runs the layout tool's sample scene as a
// game would: a vanity card (skippable with a click or Enter), a Toei-
// style wave that settles into a calm sea, the title slamming in, and a
// PRESS START prompt wired to a callback.
//
//	go run ./examples/layout/opening-animation           # from the repo root
//	go run ./cmd/lottie-layout examples/layout/opening-animation/assets/opening.scene.json
//
// The second command opens the same scene in the layout editor.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	lottie "github.com/shibukawa/lottie-go"
)

type game struct {
	sp      *lottie.ScenePlayer
	w, h    int
	started bool
}

func (g *game) Update() error {
	sp := g.sp
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyTab):
		if ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight) {
			sp.MoveFocus(lottie.FocusPrev)
		} else {
			sp.MoveFocus(lottie.FocusNext)
		}
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowUp):
		sp.MoveFocus(lottie.FocusUp)
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowDown):
		sp.MoveFocus(lottie.FocusDown)
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft):
		sp.MoveFocus(lottie.FocusLeft)
	case inpututil.IsKeyJustPressed(ebiten.KeyArrowRight):
		sp.MoveFocus(lottie.FocusRight)
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter), inpututil.IsKeyJustPressed(ebiten.KeySpace):
		sp.Activate()
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		// Cancel is unbound in the scene, so it surfaces as the scene
		// callback below; this demo replays the opening on it.
		sp.Cancel()
	}
	cx, cy := ebiten.CursorPosition()
	sp.Pointer(float64(cx), float64(cy), ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft))
	sp.Update()
	return nil
}

func (g *game) Draw(screen *ebiten.Image) { g.sp.Draw(screen, nil) }

func (g *game) Layout(w, h int) (int, int) {
	if w != g.w || h != g.h {
		g.w, g.h = w, h
		g.sp.SetScreenMapping(w, h, lottie.ScaleContain)
	}
	return w, h
}

func main() {
	scenePath := flag.String("scene",
		filepath.Join("examples", "layout", "opening-animation", "assets", "opening.scene.json"),
		"scene file to run")
	flag.Parse()

	f, err := os.Open(*scenePath)
	if err != nil {
		log.Fatal(err)
	}
	scene, err := lottie.DecodeScene(f)
	f.Close()
	if err != nil {
		log.Fatal(err)
	}
	dir := filepath.Dir(*scenePath)
	readRel := func(path string) ([]byte, error) {
		return os.ReadFile(filepath.Join(dir, filepath.FromSlash(path)))
	}
	sp, err := scene.NewScenePlayerWithLoader(lottie.SceneLoader{
		Bundle: func(path string) (*lottie.Bundle, error) {
			data, err := readRel(path)
			if err != nil {
				return nil, err
			}
			return lottie.DecodeBundle(newByteReaderAt(data), int64(len(data)))
		},
		File: readRel,
	})
	if err != nil {
		log.Fatal(err)
	}

	g := &game{sp: sp}
	sp.OnCallback(func(node, name string) {
		switch name {
		case "start-game":
			// Where a real game would leave the menu; the demo just
			// answers through the named text node.
			if !g.started {
				g.started = true
				if n, ok := sp.Node("press-start"); ok {
					n.SetText("GOOD LUCK!")
				}
				log.Printf("callback: %s (from %q)", name, node)
			}
		case "cancel":
			g.started = false
			if n, ok := sp.Node("press-start"); ok {
				n.SetText("PRESS START")
			}
			sp.Restart()
		}
	})
	sp.OnPhaseChanged(func(from, to string) { log.Printf("phase: %s -> %s", from, to) })

	ebiten.SetWindowTitle("Opening Animation (lottie-go scene)")
	ebiten.SetWindowSize(1280, 720)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if err := runWithOptionalScreenshot(g, func() {
		fmt.Fprintf(os.Stderr, "shot state: phase=%q time=%.2fs\n", sp.Phase(), sp.Time())
	}); err != nil {
		log.Fatal(err)
	}
}

// byteReaderAt adapts a byte slice to io.ReaderAt for DecodeBundle.
type byteReaderAt struct{ data []byte }

func newByteReaderAt(data []byte) *byteReaderAt { return &byteReaderAt{data} }

func (r *byteReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 || off >= int64(len(r.data)) {
		return 0, io.EOF
	}
	n := copy(p, r.data[off:])
	if n < len(p) {
		// io.ReaderAt: a short read must say why, and here it is only ever
		// the end of the data.
		return n, io.EOF
	}
	return n, nil
}

// Screenshot support, the repository's usual headless check: setting
// OPENING_SCREENSHOT renders for OPENING_SCREENSHOT_TICKS ticks, writes
// that frame as a PNG, and exits.
type screenshotGame struct {
	ebiten.Game
	path   string
	after  int
	ticks  int
	done   bool
	onShot func()
}

func (s *screenshotGame) Update() error {
	if err := s.Game.Update(); err != nil {
		return err
	}
	s.ticks++
	if s.done {
		return ebiten.Termination
	}
	return nil
}

func (s *screenshotGame) Draw(screen *ebiten.Image) {
	s.Game.Draw(screen)
	if s.done || s.ticks < s.after {
		return
	}
	b := screen.Bounds()
	img := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	screen.ReadPixels(img.Pix)
	f, err := os.Create(s.path)
	if err == nil {
		err = png.Encode(f, img)
		f.Close()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "screenshot:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "screenshot: wrote %s after %d ticks\n", s.path, s.ticks)
	if s.onShot != nil {
		s.onShot()
	}
	s.done = true
}

func runWithOptionalScreenshot(g ebiten.Game, onShot func()) error {
	path := os.Getenv("OPENING_SCREENSHOT")
	if path == "" {
		return ebiten.RunGame(g)
	}
	after := 30
	if v, err := strconv.Atoi(os.Getenv("OPENING_SCREENSHOT_TICKS")); err == nil && v > 0 {
		after = v
	}
	return ebiten.RunGame(&screenshotGame{Game: g, path: path, after: after, onShot: onShot})
}
