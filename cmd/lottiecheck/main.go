// Command lottiecheck validates Lottie files against what lottie-go can
// actually play, which makes it the feedback loop for automated (AI)
// editing: after changing a clip or a bundle, run lottiecheck and read
// the verdict instead of guessing whether the change decodes.
//
//	go run github.com/shibukawa/lottie-go/cmd/lottiecheck file.lottie
//	go run github.com/shibukawa/lottie-go/cmd/lottiecheck -render out/ file.lottie
//
// It exits non-zero when anything fails to decode, references a missing
// asset, or uses a feature lottie-go skips, and names the animation and
// the feature so the fix is obvious. With -render it also writes sampled
// frames of every animation as PNGs — the visual half of the check — by
// running an Ebitengine frame offscreen; a window may flash briefly.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	lottie "github.com/shibukawa/lottie-go"

	// Part images may be WebP; the library decodes via image.Decode.
	_ "golang.org/x/image/webp"
)

func main() {
	render := flag.String("render", "", "directory to write sampled frames of every animation as PNGs")
	samples := flag.Int("samples", 4, "frames to render per animation")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: lottiecheck [-render dir] [-samples n] file.lottie|file.json ...")
		os.Exit(2)
	}
	failed := false
	var jobs []renderJob
	for _, path := range flag.Args() {
		anims, ok := check(path)
		if !ok {
			failed = true
		}
		jobs = append(jobs, anims...)
	}
	if failed {
		os.Exit(1)
	}
	if *render != "" {
		if err := renderFrames(*render, *samples, jobs); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

type renderJob struct {
	name string
	anim *lottie.Animation
}

// check loads one file and reports per animation. It returns the decoded
// animations so a later -render pass reuses them.
func check(path string) ([]renderJob, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".lottie":
		return checkBundle(path)
	default:
		return checkJSON(path)
	}
}

func checkBundle(path string) ([]renderJob, bool) {
	f, err := os.Open(path)
	if err != nil {
		fmt.Printf("NG %s: %v\n", path, err)
		return nil, false
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		fmt.Printf("NG %s: %v\n", path, err)
		return nil, false
	}
	b, err := lottie.DecodeBundle(f, st.Size())
	if err != nil {
		fmt.Printf("NG %s: %v\n", path, err)
		return nil, false
	}
	ok := true
	for _, problem := range b.Validate() {
		fmt.Printf("NG %s: %v\n", path, problem)
		ok = false
	}
	var jobs []renderJob
	for _, id := range b.AnimationIDs() {
		a, err := b.Animation(id)
		if err != nil {
			fmt.Printf("NG %s %s: %v\n", path, id, err)
			ok = false
			continue
		}
		if !report(path, id, a) {
			ok = false
			continue
		}
		// Prefix the bundle's name: two bundles both holding a "main"
		// clip must not overwrite each other's renders.
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		jobs = append(jobs, renderJob{name: base + "-" + id, anim: a})
	}
	for _, id := range b.StateMachineIDs() {
		if _, err := b.StateMachine(id); err != nil {
			fmt.Printf("NG %s machine %s: %v\n", path, id, err)
			ok = false
		}
	}
	return jobs, ok
}

func checkJSON(path string) ([]renderJob, bool) {
	f, err := os.Open(path)
	if err != nil {
		fmt.Printf("NG %s: %v\n", path, err)
		return nil, false
	}
	defer f.Close()
	// External assets resolve relative to the file, so a loose clip next
	// to its parts/ or i/ directory checks the same way the bundle does.
	dir := filepath.Dir(path)
	a, err := lottie.DecodeWithAssets(f, func(u, name string) ([]byte, error) {
		return os.ReadFile(filepath.Join(dir, filepath.FromSlash(u), name))
	})
	if err != nil {
		fmt.Printf("NG %s: %v\n", path, err)
		return nil, false
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if !report(path, name, a) {
		return nil, false
	}
	return []renderJob{{name: name, anim: a}}, true
}

func report(path, id string, a *lottie.Animation) bool {
	if notes := a.UnsupportedFeatures(); len(notes) > 0 {
		fmt.Printf("NG %s %s: unsupported: %s\n", path, id, strings.Join(notes, "; "))
		return false
	}
	fmt.Printf("ok %s %s\n", path, id)
	return true
}

// renderFrames draws sampled frames of every animation inside one
// short-lived Ebitengine run, the same trick the editor uses for
// screenshots: draw, save, terminate.
func renderFrames(dir string, samples int, jobs []renderJob) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	g := &renderGame{dir: dir, samples: samples, jobs: jobs}
	ebiten.SetWindowSize(64, 64)
	ebiten.SetWindowTitle("lottiecheck")
	op := &ebiten.RunGameOptions{InitUnfocused: true}
	if err := ebiten.RunGameWithOptions(g, op); err != nil {
		return err
	}
	return g.err
}

type renderGame struct {
	dir     string
	samples int
	jobs    []renderJob
	done    bool
	err     error
}

func (g *renderGame) Update() error {
	if !g.done {
		g.done = true
		g.err = writeAll(g.dir, g.samples, g.jobs)
	}
	return ebiten.Termination
}

func (g *renderGame) Draw(screen *ebiten.Image) {}

func (g *renderGame) Layout(w, h int) (int, int) { return w, h }

func writeAll(dir string, samples int, jobs []renderJob) error {
	for _, j := range jobs {
		w, h := j.anim.Size()
		dst := ebiten.NewImage(w, h)
		p := j.anim.NewPlayer()
		in, out := p.Range()
		last := out - 1
		if last < in {
			last = in
		}
		for i := range samples {
			frame := in + (last-in)*float64(i)/float64(max(samples-1, 1))
			p.SetFrame(frame)
			dst.Clear()
			p.Draw(dst, nil)
			// The sample index keeps names unique: on a short clip several
			// fractional frames truncate to the same integer, and silently
			// writing fewer PNGs than asked defeats the visual check.
			name := fmt.Sprintf("%s-%02d-f%.1f.png", j.name, i, frame)
			if err := writePNG(dst, filepath.Join(dir, name)); err != nil {
				return err
			}
		}
	}
	return nil
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
