// Command basic plays a Lottie JSON file in a window.
//
// Usage:
//
//	go run ./examples/basic [file.json]
//
// Without an argument it plays the bundled test animation.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"

	lottie "github.com/shibukawa/lottie-go"
)

type game struct {
	player *lottie.Player
	anim   *lottie.Animation
}

func (g *game) Update() error {
	g.player.Update()
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	g.player.Draw(screen, nil)
	msg := fmt.Sprintf("%.1f FPS", ebiten.ActualFPS())
	if unsup := g.anim.UnsupportedFeatures(); len(unsup) > 0 {
		msg += fmt.Sprintf("\nunsupported: %v", unsup)
	}
	ebitenutil.DebugPrint(screen, msg)
}

func (g *game) Layout(w, h int) (int, int) {
	return g.anim.Size()
}

func main() {
	path := "testdata/basic.json"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	anim, err := lottie.Decode(f)
	f.Close()
	if err != nil {
		log.Fatal(err)
	}
	p := anim.NewPlayer()
	p.SetLoop(true)
	w, h := anim.Size()
	ebiten.SetWindowSize(w*2, h*2)
	ebiten.SetWindowTitle("lottie-go: " + path)
	if err := ebiten.RunGame(&game{player: p, anim: anim}); err != nil {
		log.Fatal(err)
	}
}
