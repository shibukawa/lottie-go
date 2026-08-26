// Command player is a standalone viewer for arbitrary Lottie JSON files.
//
// Open a file by passing it as an argument or by dropping it onto the
// window:
//
//	go run ./examples/player [file.json]
//
// Controls:
//   - space: play / pause
//   - left / right: seek -/+ 0.1s (hold shift for a single frame)
//   - up / down: playback speed +/- 0.25x
//   - L: toggle loop
//   - R: restart
//   - B: toggle background (checker / dark / light)
package main

import (
	"bytes"
	"fmt"
	"image/color"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	lottie "github.com/shibukawa/lottie-go"

	// The library only registers PNG; registering WebP here lets the
	// player open samples with WebP image assets (examples/motioncomic).
	_ "golang.org/x/image/webp"
)

const (
	initialW = 640
	initialH = 480
)

type viewer struct {
	anim   *lottie.Animation
	player *lottie.Player
	name   string
	errMsg string

	loop    bool
	speed   float64
	bg      int // 0 checker, 1 dark, 2 light
	checker *ebiten.Image
}

func newViewer() *viewer {
	v := &viewer{loop: true, speed: 1}
	v.checker = ebiten.NewImage(16, 16)
	v.checker.Fill(color.RGBA{0x50, 0x50, 0x58, 0xff})
	half := ebiten.NewImage(8, 8)
	half.Fill(color.RGBA{0x68, 0x68, 0x72, 0xff})
	var op ebiten.DrawImageOptions
	v.checker.DrawImage(half, &op)
	op.GeoM.Translate(8, 8)
	v.checker.DrawImage(half, &op)
	return v
}

func (v *viewer) load(name string, data []byte) {
	var anim *lottie.Animation
	var err error
	if strings.HasSuffix(strings.ToLower(name), ".lottie") {
		anim, err = lottie.DecodeDotLottie(bytes.NewReader(data), int64(len(data)))
	} else {
		anim, err = lottie.Decode(bytes.NewReader(data))
	}
	if err != nil {
		v.errMsg = fmt.Sprintf("%s: %v", name, err)
		return
	}
	v.errMsg = ""
	v.anim = anim
	v.name = name
	v.player = anim.NewPlayer()
	v.player.SetLoop(v.loop)
	v.player.SetSpeed(v.speed)
}

func (v *viewer) loadPath(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		v.errMsg = err.Error()
		return
	}
	v.load(filepath.Base(path), data)
}

// pollDroppedFiles loads the first dropped *.json file, walking dropped
// directories too.
func (v *viewer) pollDroppedFiles() {
	dropped := ebiten.DroppedFiles()
	if dropped == nil {
		return
	}
	_ = fs.WalkDir(dropped, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		lower := strings.ToLower(path)
		if !strings.HasSuffix(lower, ".json") && !strings.HasSuffix(lower, ".lottie") {
			return nil
		}
		data, err := fs.ReadFile(dropped, path)
		if err != nil {
			v.errMsg = err.Error()
			return fs.SkipAll
		}
		v.load(filepath.Base(path), data)
		return fs.SkipAll
	})
}

func (v *viewer) Update() error {
	v.pollDroppedFiles()
	if v.player == nil {
		return nil
	}
	p := v.player

	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		if p.IsPlaying() {
			p.Pause()
		} else {
			if p.Position() >= v.anim.Duration() {
				p.Seek(0)
			}
			p.Play()
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		p.Seek(0)
		p.Play()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyL) {
		v.loop = !v.loop
		p.SetLoop(v.loop)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyB) {
		v.bg = (v.bg + 1) % 3
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		v.speed += 0.25
		if v.speed > 4 {
			v.speed = 4
		}
		p.SetSpeed(v.speed)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		v.speed -= 0.25
		if v.speed < 0.25 {
			v.speed = 0.25
		}
		p.SetSpeed(v.speed)
	}
	seekStep := func(dir time.Duration) {
		step := 100 * time.Millisecond
		if ebiten.IsKeyPressed(ebiten.KeyShift) {
			step = time.Duration(float64(time.Second) / v.anim.FrameRate())
		}
		p.Pause()
		p.Seek(p.Position() + dir*step)
	}
	if keyRepeated(ebiten.KeyLeft) {
		seekStep(-1)
	}
	if keyRepeated(ebiten.KeyRight) {
		seekStep(1)
	}

	p.Update()
	return nil
}

// keyRepeated fires on press and then every 6 ticks while held.
func keyRepeated(key ebiten.Key) bool {
	d := inpututil.KeyPressDuration(key)
	return d == 1 || (d >= 20 && d%6 == 0)
}

func (v *viewer) Draw(screen *ebiten.Image) {
	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	switch v.bg {
	case 0:
		var op ebiten.DrawImageOptions
		for y := 0; y < sh; y += 16 {
			for x := 0; x < sw; x += 16 {
				op.GeoM.Reset()
				op.GeoM.Translate(float64(x), float64(y))
				screen.DrawImage(v.checker, &op)
			}
		}
	case 1:
		screen.Fill(color.RGBA{0x20, 0x22, 0x26, 0xff})
	case 2:
		screen.Fill(color.RGBA{0xf2, 0xf2, 0xf2, 0xff})
	}

	if v.player == nil {
		msg := "drop a Lottie .json / .lottie file here\n(or: go run ./examples/player file.json)"
		if v.errMsg != "" {
			msg += "\n\nerror: " + v.errMsg
		}
		ebitenutil.DebugPrintAt(screen, msg, sw/2-140, sh/2-16)
		return
	}

	// Fit the animation into the window, keeping some margin for the HUD.
	aw, ah := v.anim.Size()
	avail := float64(sh - 72)
	scale := float64(sw) / float64(aw)
	if s := avail / float64(ah); s < scale {
		scale = s
	}
	var opts lottie.DrawOptions
	opts.GeoM.Scale(scale, scale)
	opts.GeoM.Translate((float64(sw)-float64(aw)*scale)/2, (avail-float64(ah)*scale)/2)
	v.player.Draw(screen, &opts)

	v.drawHUD(screen, sw, sh)
}

func (v *viewer) drawHUD(screen *ebiten.Image, sw, sh int) {
	// Progress bar.
	dur := v.anim.Duration()
	progress := 0.0
	if dur > 0 {
		progress = float64(v.player.Position()) / float64(dur)
	}
	barY := float32(sh - 56)
	vector.FillRect(screen, 8, barY, float32(sw-16), 4, color.RGBA{0x40, 0x40, 0x48, 0xff}, false)
	vector.FillRect(screen, 8, barY, float32(float64(sw-16)*progress), 4, color.RGBA{0x4c, 0xaf, 0xef, 0xff}, false)

	state := "playing"
	if !v.player.IsPlaying() {
		state = "paused"
	}
	w, h := v.anim.Size()
	info := fmt.Sprintf("%s  %dx%d  %.0ffps  %.2fs / %.2fs  x%.2f  loop:%v  [%s]",
		v.name, w, h, v.anim.FrameRate(),
		v.player.Position().Seconds(), dur.Seconds(), v.speed, v.loop, state)
	if unsup := v.anim.UnsupportedFeatures(); len(unsup) > 0 {
		info += "\nunsupported: " + strings.Join(unsup, ", ")
	}
	if v.errMsg != "" {
		info += "\nerror: " + v.errMsg
	}
	ebitenutil.DebugPrintAt(screen, info, 8, sh-44)
}

func (v *viewer) Layout(w, h int) (int, int) { return w, h }

func main() {
	v := newViewer()
	if len(os.Args) > 1 {
		v.loadPath(os.Args[1])
		if v.errMsg != "" {
			log.Fatal(v.errMsg)
		}
	}
	ebiten.SetWindowSize(initialW, initialH)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle("lottie-go player")
	if err := ebiten.RunGame(v); err != nil {
		log.Fatal(err)
	}
}
