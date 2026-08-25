// Command gpuprobe inspects what the renderer costs on the GPU and guards
// the compositing path against pixel regressions.
//
// Built with the ebitenginedebug tag, Ebitengine dumps every graphics command
// and every internal texture once per frame to stdout. That log is the only
// way to see draw-call merging and texture allocation from outside the
// engine, so the probe mode simply renders a few frames and lets the log
// speak:
//
//	go run -tags ebitenginedebug ./examples/gpuprobe -copies 20 | go run ./examples/gpuprobe -summarize
//
// The golden modes render fixed frames to PNG so that changes to offscreen
// allocation, masking, or matte composition can be shown to be pixel-neutral:
//
//	go run ./examples/gpuprobe -golden /tmp/base       # record a baseline
//	go run ./examples/gpuprobe -compare /tmp/base      # verify against it
//
// Usage:
//
//	go run ./examples/gpuprobe [-files a.json,b.json] [-copies n] [-frames n]
//	                           [-golden dir] [-compare dir] [-tolerance n]
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	lottie "github.com/shibukawa/lottie-go"
)

var (
	files     = flag.String("files", "", "comma separated lottie JSON paths (default: every gallery asset)")
	copies    = flag.Int("copies", 1, "copies of each animation to play at once")
	frames    = flag.Int("frames", 6, "frames to render (probe mode) or samples per animation (golden modes)")
	golden    = flag.String("golden", "", "write reference PNGs into this directory")
	compare   = flag.String("compare", "", "compare rendered output against reference PNGs in this directory")
	tolerance = flag.Int("tolerance", 0, "maximum allowed per-channel difference when comparing")
	summarize = flag.Bool("summarize", false, "read an ebitenginedebug log on stdin and summarize its last frame")
	paused    = flag.Bool("paused", false, "probe mode: do not advance the animations, so the idle snapshot cache engages")
	nosnap    = flag.Bool("nosnap", false, "disable the idle snapshot cache on every player")
)

const defaultAssetDir = "examples/gallery/assets"

type entry struct {
	name string
	anim *lottie.Animation
	play *lottie.Player
}

func main() {
	flag.Parse()

	if *summarize {
		if err := summarizeLog(os.Stdin, os.Stdout); err != nil {
			log.Fatal(err)
		}
		return
	}

	paths, err := resolveFiles()
	if err != nil {
		log.Fatal(err)
	}
	var entries []entry
	for _, p := range paths {
		anim, err := decode(p)
		if err != nil {
			log.Fatalf("%s: %v", p, err)
		}
		for i := 0; i < *copies; i++ {
			play := anim.NewPlayer()
			play.SetSnapshotCache(!*nosnap)
			entries = append(entries, entry{
				name: strings.TrimSuffix(filepath.Base(p), ".json"),
				anim: anim,
				play: play,
			})
		}
	}
	if len(entries) == 0 {
		log.Fatal("no animations to render")
	}

	g := &game{entries: entries}
	if *golden != "" || *compare != "" {
		g.mode = modeGolden
		if err := os.MkdirAll(*golden, 0o755); *golden != "" && err != nil {
			log.Fatal(err)
		}
	}
	ebiten.SetWindowSize(512, 512)
	ebiten.SetVsyncEnabled(false)
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
	if g.failures > 0 {
		os.Exit(1)
	}
}

func resolveFiles() ([]string, error) {
	if *files != "" {
		return strings.Split(*files, ","), nil
	}
	paths, err := filepath.Glob(filepath.Join(defaultAssetDir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func decode(path string) (*lottie.Animation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return lottie.Decode(f)
}

type mode int

const (
	modeProbe mode = iota
	modeGolden
)

type game struct {
	entries  []entry
	mode     mode
	n        int
	done     bool
	failures int
	checked  int
}

func (g *game) Update() error {
	if g.done || (g.mode == modeProbe && g.n >= *frames) {
		return ebiten.Termination
	}
	if g.mode == modeProbe && !*paused {
		for i := range g.entries {
			g.entries[i].play.Update()
		}
	}
	g.n++
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	if g.mode == modeProbe {
		for i := range g.entries {
			g.entries[i].play.Draw(screen, nil)
		}
		return
	}
	// One pass: render every animation at evenly spaced frames into its own
	// offscreen and record or verify the pixels.
	for i := range g.entries {
		e := &g.entries[i]
		w, h := e.anim.Size()
		off := ebiten.NewImage(w, h)
		start, end := e.play.Range()
		for s := 0; s < *frames; s++ {
			f := start + (end-start)*float64(s)/float64(*frames)
			e.play.SetFrame(f)
			// Draw twice: the first draw records the snapshot key and
			// renders directly, the second is served by the idle snapshot
			// cache. Comparing the second verifies both paths, since the
			// cache bakes with the same renderer the first draw used.
			off.Clear()
			e.play.Draw(off, nil)
			off.Clear()
			e.play.Draw(off, nil)
			name := fmt.Sprintf("%s-%02d.png", e.name, s)
			if err := g.record(name, off, w, h); err != nil {
				log.Fatal(err)
			}
		}
		off.Deallocate()
	}
	g.report()
	g.done = true
}

// record writes or verifies one rendered frame.
//
// ReadPixels hands back premultiplied alpha, but PNG stores straight alpha.
// The buffer is therefore wrapped in an NRGBA header purely so that encoding
// and decoding round-trip the bytes untouched; a converting wrap would lose
// precision wherever alpha is below 255 and turn exact comparisons into
// approximate ones. Semi-transparent areas consequently look brighter than
// the real render when the file is opened in a viewer.
func (g *game) record(name string, img *ebiten.Image, w, h int) error {
	pix := make([]byte, 4*w*h)
	img.ReadPixels(pix)

	if *golden != "" {
		var buf bytes.Buffer
		raw := &image.NRGBA{Pix: pix, Stride: 4 * w, Rect: image.Rect(0, 0, w, h)}
		if err := png.Encode(&buf, raw); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(*golden, name), buf.Bytes(), 0o644); err != nil {
			return err
		}
	}
	if *compare == "" {
		return nil
	}

	want, err := readPNG(filepath.Join(*compare, name), w, h)
	if err != nil {
		return err
	}
	g.checked++
	worst := 0
	differing := 0
	for i := range pix {
		d := int(pix[i]) - int(want[i])
		if d < 0 {
			d = -d
		}
		if d > worst {
			worst = d
		}
		if d > *tolerance {
			differing++
		}
	}
	if differing > 0 {
		g.failures++
		fmt.Printf("MISMATCH %-56s worst-channel-delta=%-4d differing-bytes=%d/%d\n",
			name, worst, differing, len(pix))
	}
	return nil
}

func readPNG(path string, w, h int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	raw, ok := src.(*image.NRGBA)
	if !ok || raw.Rect.Dx() != w || raw.Rect.Dy() != h || raw.Stride != 4*w {
		return nil, fmt.Errorf("%s: unexpected reference image format or size", path)
	}
	return raw.Pix, nil
}

func (g *game) report() {
	switch {
	case *compare != "" && g.failures == 0:
		fmt.Printf("OK: %d frames match the reference in %s\n", g.checked, *compare)
	case *compare != "":
		fmt.Printf("FAIL: %d of %d frames differ from the reference in %s\n", g.failures, g.checked, *compare)
	case *golden != "":
		fmt.Printf("wrote reference frames to %s\n", *golden)
	}
}

func (g *game) Layout(w, h int) (int, int) { return 512, 512 }
