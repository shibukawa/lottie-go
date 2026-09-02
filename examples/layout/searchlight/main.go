// Command searchlight runs the layout tool's camera sample as a game
// would: a dark kitchen at night, a searchlight (a screen-pinned mask at
// parallax depth 0) sweeping the room as the scene camera moves from
// waypoint to waypoint, and finally landing on the mouse eating the
// cheese. The waypoints are the scene's phases; this program only eases
// the camera between them.
//
//	go run ./examples/layout/searchlight            # from the repo root
//	go run ./cmd/lottie-layout examples/layout/searchlight/assets/searchlight.scene.json
//
// The scene, its bundles, and the font are embedded, so the sample runs
// from any directory; -scene points it at a scene file on disk instead
// (its bundles and assets resolve beside that file).
//
// Enter, Space, or a click skips to the next waypoint; Esc restarts.
package main

import (
	"embed"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	lottie "github.com/shibukawa/lottie-go"
)

//go:embed assets/*.lottie assets/*.ttf assets/*.scene.json
var assets embed.FS

const sceneFile = "searchlight.scene.json"

type game struct {
	sp   *lottie.ScenePlayer
	w, h int
	// cam is the camera actually shown, chasing the running phase's
	// camera. Entering a phase snaps the player's camera to the phase's;
	// setting this one after every Update is what makes the light glide.
	cam lottie.SceneCamera
}

// chase is the per-tick fraction of the remaining distance the camera
// covers: about 95% of a move in three quarters of a second at 60 TPS.
const chase = 0.065

func (g *game) Update() error {
	sp := g.sp
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter), inpututil.IsKeyJustPressed(ebiten.KeySpace),
		inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft):
		if ph, ok := sp.Scene().Phase(sp.Phase()); ok && ph.Next != "" {
			sp.SetPhase(ph.Next)
		}
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		sp.Restart()
		g.cam = sp.Camera()
	}
	sp.Update()
	target := sp.Camera() // the running phase's camera (or where a switch just snapped it)
	g.cam = lottie.SceneCamera{
		X:        g.cam.X + (target.X-g.cam.X)*chase,
		Y:        g.cam.Y + (target.Y-g.cam.Y)*chase,
		Zoom:     g.cam.ZoomFactor() + (target.ZoomFactor()-g.cam.ZoomFactor())*chase,
		Rotation: g.cam.Rotation + (target.Rotation-g.cam.Rotation)*chase,
	}
	sp.SetCamera(g.cam)
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
	scenePath := flag.String("scene", "", "scene file to run instead of the embedded one")
	startPhase := flag.String("phase", "", "start in this phase instead of the first")
	flag.Parse()

	// Everything resolves relative to the scene: the embedded assets
	// directory by default, the given file's directory with -scene.
	var readRel func(path string) ([]byte, error)
	if *scenePath == "" {
		sub, err := fs.Sub(assets, "assets")
		if err != nil {
			log.Fatal(err)
		}
		readRel = func(path string) ([]byte, error) { return fs.ReadFile(sub, path) }
		*scenePath = sceneFile
	} else {
		dir := filepath.Dir(*scenePath)
		readRel = func(path string) ([]byte, error) {
			return os.ReadFile(filepath.Join(dir, filepath.FromSlash(path)))
		}
		*scenePath = filepath.Base(*scenePath)
	}
	data, err := readRel(*scenePath)
	if err != nil {
		log.Fatal(err)
	}
	scene, err := lottie.ParseScene(data)
	if err != nil {
		log.Fatal(err)
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
	if *startPhase != "" && !sp.SetPhase(*startPhase) {
		log.Fatalf("no phase %q", *startPhase)
	}
	sp.OnPhaseChanged(func(from, to string) { log.Printf("phase: %s -> %s", from, to) })

	g := &game{sp: sp, cam: sp.Camera()}
	ebiten.SetWindowTitle("Searchlight (lottie-go scene camera)")
	ebiten.SetWindowSize(1280, 720)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if err := runWithOptionalScreenshot(g, func() {
		fmt.Fprintf(os.Stderr, "shot state: phase=%q time=%.2fs camera=%+v\n", sp.Phase(), sp.Time(), sp.Camera())
	}); err != nil {
		log.Fatal(err)
	}
}

// byteReaderAt adapts a byte slice to io.ReaderAt for DecodeBundle.
type byteReaderAt struct{ data []byte }

func newByteReaderAt(data []byte) *byteReaderAt { return &byteReaderAt{data} }

func (r *byteReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(r.data)) {
		return 0, fmt.Errorf("EOF")
	}
	n := copy(p, r.data[off:])
	return n, nil
}

// Screenshot support, the repository's usual headless check: setting
// SEARCHLIGHT_SCREENSHOT renders for SEARCHLIGHT_SCREENSHOT_TICKS ticks,
// writes that frame as a PNG, and exits.
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
	path := os.Getenv("SEARCHLIGHT_SCREENSHOT")
	if path == "" {
		return ebiten.RunGame(g)
	}
	after := 30
	if v, err := strconv.Atoi(os.Getenv("SEARCHLIGHT_SCREENSHOT_TICKS")); err == nil && v > 0 {
		after = v
	}
	return ebiten.RunGame(&screenshotGame{Game: g, path: path, after: after, onShot: onShot})
}
