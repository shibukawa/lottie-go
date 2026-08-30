// Command gen renders the shoujo-manga raster art, packs it into WebP data
// URIs, and writes ../motioncomic.json plus the loose .webp files under
// ../assets for inspection. Everything is generated in this repository, so
// there is no third-party art or font licensing to track.
//
//	go run ./examples/lottie/motioncomic/gen
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	"github.com/HugoSmits86/nativewebp"
	lottie "github.com/shibukawa/lottie-go"
	_ "golang.org/x/image/webp" // decode support for the validation pass
)

type rasterAsset struct {
	id  string
	img image.Image
}

// renderAll draws every raster the timeline references, in a stable order
// so the committed JSON does not churn.
func renderAll() []rasterAsset {
	return []rasterAsset{
		{"img-pig-run", pigRunSprite()},
		{"img-squirrel-run", squirrelRunSprite()},
		{"img-crash", crashSprite()},
		{"img-pig-sit", pigSitSprite()},
		{"img-squirrel-sit", squirrelSitSprite()},
		{"img-toast", toastSprite()},
		{"img-sparkle", sparkleSprite()},
		{"img-petal", petalSprite()},
		{"img-drop", dropSprite()},
		{"img-lines", speedLines(920, 480)},
		{"img-p1bg", p1bg(880, 480)},
		{"img-p2bg", p2bg(880, 480)},
		{"img-p3bg", p3bg(1720, 520)},
		{"img-p4bg", p4bg(1000, 480)},
		// Panels 1 and 2 share a frame: same size, same art.
		{"img-frame-r", rectFrame(880, 560)},
		{"img-frame3", slashFrame(1720, 560, 120)},
		{"img-frame4", rectFrame(1080, 560)},
		{"img-sfx", sfxDon()},
	}
}

func run(out string) error {
	assetDir := filepath.Join(out, "assets")
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		return err
	}
	d := dims{}
	var assets []obj
	total := 0
	for _, ra := range renderAll() {
		var buf bytes.Buffer
		if err := nativewebp.Encode(&buf, ra.img, nil); err != nil {
			return fmt.Errorf("%s: %w", ra.id, err)
		}
		if err := os.WriteFile(filepath.Join(assetDir, ra.id+".webp"), buf.Bytes(), 0o644); err != nil {
			return err
		}
		b := ra.img.Bounds()
		d[ra.id] = image.Pt(b.Dx(), b.Dy())
		uri := "data:image/webp;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
		assets = append(assets, imageAsset(ra.id, b.Dx(), b.Dy(), uri))
		total += buf.Len()
	}

	data, err := json.MarshalIndent(buildDoc(d, assets), "", " ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	// The sample has to load clean in this repository's own player before
	// it is worth committing.
	anim, err := lottie.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("generated JSON does not decode: %w", err)
	}
	if unsup := anim.UnsupportedFeatures(); len(unsup) > 0 {
		return fmt.Errorf("generated JSON uses unsupported features: %s",
			strings.Join(unsup, ", "))
	}

	path := filepath.Join(out, "motioncomic.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	w, h := anim.Size()
	fmt.Printf("wrote %s (%dx%d, %.1fs, %d bytes, %d bytes of webp)\n",
		path, w, h, anim.Duration().Seconds(), len(data), total)
	return nil
}

func main() {
	out := flag.String("out", "examples/lottie/motioncomic", "output directory")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
