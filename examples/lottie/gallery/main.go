// Command gallery plays the bundled CC0 sample animations from
// LottieFiles/test-files in a grid.
//
// Controls:
//   - click a tile: zoom into that animation (click again to go back)
//   - space: pause / resume
//   - R: restart all animations
//
// Usage:
//
//	go run ./examples/lottie/gallery
package main

import (
	"embed"
	"fmt"
	"image/color"
	"log"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	lottie "github.com/shibukawa/lottie-go"
)

//go:embed assets/*.json
var assets embed.FS

const (
	cols      = 6
	cellW     = 130
	cellH     = 150
	tileSize  = 120
	labelPadY = 126
)

type item struct {
	name   string
	anim   *lottie.Animation
	player *lottie.Player
	unsup  []string
}

type game struct {
	items  []*item
	rows   int
	zoom   int // -1 = grid view
	paused bool
	zoomP  *lottie.Player // fresh player for zoomed view
}

func (g *game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		g.paused = !g.paused
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		for _, it := range g.items {
			it.player.Seek(0)
			it.player.Play()
		}
		if g.zoomP != nil {
			g.zoomP.Seek(0)
			g.zoomP.Play()
		}
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if g.zoom >= 0 {
			g.zoom = -1
			g.zoomP = nil
		} else {
			x, y := ebiten.CursorPosition()
			col, row := x/cellW, y/cellH
			idx := row*cols + col
			if col >= 0 && col < cols && idx >= 0 && idx < len(g.items) {
				g.zoom = idx
				g.zoomP = g.items[idx].anim.NewPlayer()
				g.zoomP.SetLoop(true)
			}
		}
	}
	if g.paused {
		return nil
	}
	for _, it := range g.items {
		it.player.Update()
	}
	if g.zoomP != nil {
		g.zoomP.Update()
	}
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{0x23, 0x26, 0x2b, 0xff})
	if g.zoom >= 0 {
		g.drawZoom(screen)
		return
	}
	for i, it := range g.items {
		col, row := i%cols, i/cols
		ox, oy := float64(col*cellW), float64(row*cellH)
		g.drawTile(screen, it, it.player, ox, oy, tileSize)
		label := it.name
		// Cut on runes, not bytes: a byte cut through a multi-byte name
		// leaves a broken sequence at the end.
		if r := []rune(label); len(r) > 20 {
			label = string(r[:20])
		}
		ebitenutil.DebugPrintAt(screen, label, col*cellW+2, row*cellH+labelPadY)
		if len(it.unsup) > 0 {
			ebitenutil.DebugPrintAt(screen, "!", col*cellW+tileSize-10, row*cellH+2)
		}
	}
}

func (g *game) drawTile(screen *ebiten.Image, it *item, p *lottie.Player, ox, oy float64, size float64) {
	w, h := it.anim.Size()
	scale := size / float64(w)
	if s := size / float64(h); s < scale {
		scale = s
	}
	var opts lottie.DrawOptions
	opts.GeoM.Scale(scale, scale)
	opts.GeoM.Translate(ox+(size-float64(w)*scale)/2, oy+(size-float64(h)*scale)/2)
	p.Draw(screen, &opts)
}

func (g *game) drawZoom(screen *ebiten.Image) {
	it := g.items[g.zoom]
	sw := screen.Bounds().Dx()
	size := float64(sw) - 160
	g.drawTile(screen, it, g.zoomP, 80, 40, size)
	info := fmt.Sprintf("%s\n%v  pos %.2fs\nclick: back / space: pause / R: restart",
		it.name, it.anim.Duration(), g.zoomP.Position().Seconds())
	if len(it.unsup) > 0 {
		info += "\nunsupported: " + strings.Join(it.unsup, ", ")
	}
	ebitenutil.DebugPrintAt(screen, info, 8, screen.Bounds().Dy()-64)
}

func (g *game) Layout(w, h int) (int, int) {
	return cols * cellW, g.rows * cellH
}

func main() {
	entries, err := assets.ReadDir("assets")
	if err != nil {
		log.Fatal(err)
	}
	var g game
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		f, err := assets.Open("assets/" + e.Name())
		if err != nil {
			log.Fatal(err)
		}
		anim, err := lottie.Decode(f)
		f.Close()
		if err != nil {
			log.Printf("skip %s: %v", e.Name(), err)
			continue
		}
		p := anim.NewPlayer()
		p.SetLoop(true)
		g.items = append(g.items, &item{
			name:   strings.TrimSuffix(e.Name(), ".json"),
			anim:   anim,
			player: p,
			unsup:  anim.UnsupportedFeatures(),
		})
	}
	if len(g.items) == 0 {
		log.Fatal("no animations loaded")
	}
	sort.Slice(g.items, func(i, j int) bool { return g.items[i].name < g.items[j].name })
	g.zoom = -1
	g.rows = (len(g.items) + cols - 1) / cols
	ebiten.SetWindowSize(cols*cellW, g.rows*cellH)
	ebiten.SetWindowTitle("lottie-go gallery (CC0 samples from LottieFiles/test-files)")
	if err := ebiten.RunGame(&g); err != nil {
		log.Fatal(err)
	}
}
