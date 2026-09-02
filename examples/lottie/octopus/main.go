// Command octopus plays the textured octopus sample: a soft-bodied
// character whose skin, suckers and background kelp are images painted
// through vector paths with lottie-go's texture extension (plugin/texture).
// The clip itself is plain Lottie; press T to see what a player without the
// extension draws — the same shapes in their fallback solid colors.
//
//	go run ./examples/lottie/octopus
//
// Controls:
//   - space: pause / resume
//   - R: restart
//   - T: toggle the textures
//
// Setting LOTTIE_OCTOPUS_SCREENSHOT to a path writes one frame there and
// exits; LOTTIE_OCTOPUS_FRAME picks the frame (default 20).
//
// Regenerate the art and the clip with `go run ./examples/lottie/octopus/gen`.
package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	lottie "github.com/shibukawa/lottie-go"
	lottietexture "github.com/shibukawa/lottie-go/plugin/texture"
)

//go:embed octopus.lottie
var bundleData []byte

type game struct {
	bundle   *lottie.Bundle
	anim     *lottie.Animation
	player   *lottie.Player
	textured bool

	shotPath string
	shotDone bool
}

// newPlayer builds the stage player, dressed from the texture document
// unless textured is off.
func (g *game) newPlayer(textured bool, frame float64) {
	p := g.anim.NewPlayer()
	p.SetLoop(true)
	if textured {
		doc, err := lottietexture.Load(g.bundle, "swim")
		if err != nil {
			log.Fatal(err)
		}
		if err := doc.Apply(p); err != nil {
			log.Fatal(err)
		}
	}
	p.SetFrame(frame)
	if g.player != nil && !g.player.IsPlaying() {
		p.Pause()
	}
	g.player, g.textured = p, textured
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
	if inpututil.IsKeyJustPressed(ebiten.KeyT) {
		g.newPlayer(!g.textured, g.player.Frame())
	}
	if g.shotDone {
		return ebiten.Termination
	}
	g.player.Update()
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	g.player.Draw(screen, nil)
	if g.shotPath != "" && !g.shotDone {
		if err := writePNG(screen, g.shotPath); err != nil {
			log.Fatal(err)
		}
		fmt.Fprintln(os.Stderr, "wrote", g.shotPath)
		g.shotDone = true
	}
}

func (g *game) Layout(w, h int) (int, int) { return g.anim.Size() }

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

func main() {
	b, err := lottie.DecodeBundle(bytes.NewReader(bundleData), int64(len(bundleData)))
	if err != nil {
		log.Fatal(err)
	}
	anim, err := b.Animation("swim")
	if err != nil {
		log.Fatal(err)
	}
	g := &game{bundle: b, anim: anim}
	frame := 20.0
	if v, err := strconv.ParseFloat(os.Getenv("LOTTIE_OCTOPUS_FRAME"), 64); err == nil {
		frame = v
	}
	g.newPlayer(os.Getenv("LOTTIE_OCTOPUS_UNTEXTURED") == "", frame)
	g.shotPath = os.Getenv("LOTTIE_OCTOPUS_SCREENSHOT")
	if g.shotPath != "" {
		g.player.Pause()
	}
	w, h := anim.Size()
	ebiten.SetWindowSize(w*2, h*2)
	ebiten.SetWindowTitle("lottie-go — textured octopus")
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
