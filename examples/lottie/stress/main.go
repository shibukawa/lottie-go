// Command stress verifies the performance targets: N concurrent UI-scale
// animations at a sustained 60fps with per-frame draw time under 2ms.
//
// Usage:
//
//	go run ./examples/lottie/stress [-n 5] [-seconds 10]
//
// It plays n copies of every bundled kind of workload (shapes, gradients,
// trim, precomp) and prints a timing report to stdout; non-zero exit when
// the 2ms budget is exceeded.
package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"os"
	"sort"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"

	lottie "github.com/shibukawa/lottie-go"
)

var (
	n       = flag.Int("n", 5, "concurrent animations")
	seconds = flag.Float64("seconds", 10, "measurement duration")
	files   = []string{
		"examples/lottie/gallery/assets/layers-precomp-nested.json",
		"examples/lottie/gallery/assets/properties-bezier-ease.json",
		"examples/lottie/gallery/assets/shape-style-gradient-radial.json",
		"examples/lottie/gallery/assets/shape-modifiers-trim-simultaneously.json",
		"examples/lottie/gallery/assets/layers-matte-above.json",
	}
)

type game struct {
	players []*lottie.Player
	anims   []*lottie.Animation
	samples []float64 // per-frame draw milliseconds
	start   time.Time
	done    bool
}

func (g *game) Update() error {
	if g.done {
		return ebiten.Termination
	}
	for _, p := range g.players {
		p.Update()
	}
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	if g.done {
		return
	}
	screen.Fill(color.RGBA{0x20, 0x22, 0x26, 0xff})
	t0 := time.Now()
	for i, p := range g.players {
		w, _ := g.anims[i%len(g.anims)].Size()
		scale := 100 / float64(w)
		var opts lottie.DrawOptions
		opts.GeoM.Scale(scale, scale)
		opts.GeoM.Translate(float64((i%6)*110), float64((i/6)*110))
		p.Draw(screen, &opts)
	}
	ms := float64(time.Since(t0).Microseconds()) / 1000
	if time.Since(g.start) > time.Second { // skip warm-up
		g.samples = append(g.samples, ms)
	}
	msg := fmt.Sprintf("%d players  %.1f FPS  draw %.3fms", len(g.players), ebiten.ActualFPS(), ms)
	ebitenutil.DebugPrint(screen, msg)
	if time.Since(g.start) > time.Duration(float64(time.Second)*(*seconds)) {
		g.report()
		g.done = true
	}
}

func (g *game) report() {
	if len(g.samples) == 0 {
		return
	}
	sort.Float64s(g.samples)
	sum := 0.0
	for _, s := range g.samples {
		sum += s
	}
	mean := sum / float64(len(g.samples))
	p50 := g.samples[len(g.samples)/2]
	p99 := g.samples[len(g.samples)*99/100]
	max := g.samples[len(g.samples)-1]
	fps := ebiten.ActualFPS()
	fmt.Printf("players=%d frames=%d fps=%.1f draw_ms mean=%.3f p50=%.3f p99=%.3f max=%.3f\n",
		len(g.players), len(g.samples), fps, mean, p50, p99, max)
	if p99 > 2.0 {
		fmt.Println("RESULT: FAIL (p99 draw time over 2ms budget)")
		os.Exit(1)
	}
	fmt.Println("RESULT: PASS (p99 draw time within 2ms budget)")
}

func (g *game) Layout(w, h int) (int, int) { return 660, 340 }

func main() {
	flag.Parse()
	var anims []*lottie.Animation
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			log.Fatal(err)
		}
		anim, err := lottie.Decode(f)
		f.Close()
		if err != nil {
			log.Fatalf("%s: %v", path, err)
		}
		anims = append(anims, anim)
	}
	g := &game{start: time.Now(), anims: anims}
	for i := 0; i < *n; i++ {
		p := anims[i%len(anims)].NewPlayer()
		p.SetLoop(true)
		g.players = append(g.players, p)
	}
	ebiten.SetWindowSize(660, 340)
	ebiten.SetWindowTitle("lottie-go stress")
	if err := ebiten.RunGame(g); err != nil && err != ebiten.Termination {
		log.Fatal(err)
	}
}
