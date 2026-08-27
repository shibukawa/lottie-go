// Command cutscene plays the bundled motion-comic sample. The JSON is
// embedded, so it runs from anywhere:
//
//	go run github.com/shibukawa/lottie-go/examples/cutscene@latest
//
// Controls:
//   - space: pause / resume
//   - R: restart
//
// Regenerate the animation itself with `go run ./examples/cutscene/gen`;
// see the README next to this file for the cut list and rig notes.
package main

import (
	"bytes"
	_ "embed"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	lottie "github.com/shibukawa/lottie-go"
)

//go:embed cutscene.json
var cutsceneJSON []byte

type game struct {
	anim   *lottie.Animation
	player *lottie.Player
}

func (g *game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		if g.player.IsPlaying() {
			g.player.Pause()
		} else {
			g.player.Play()
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		g.player.Rewind()
		g.player.Play()
	}
	g.player.Update()
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	g.player.Draw(screen, nil)
}

func (g *game) Layout(w, h int) (int, int) {
	return g.anim.Size()
}

func main() {
	anim, err := lottie.Decode(bytes.NewReader(cutsceneJSON))
	if err != nil {
		log.Fatal(err)
	}
	p := anim.NewPlayer()
	p.SetLoop(true)
	w, h := anim.Size()
	ebiten.SetWindowSize(w*2, h*2)
	ebiten.SetWindowTitle("lottie-go: cutscene")
	if err := ebiten.RunGame(&game{anim: anim, player: p}); err != nil {
		log.Fatal(err)
	}
}
