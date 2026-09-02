// Command gen builds the Elara sample: a character sheet turned into a
// UV-mapped mesh rig.
//
// The parts are not rigid cutouts. Each is a contour traced off its own
// silhouette, painted with a slice of the sheet through per-vertex UV,
// and bent by skinning that contour to the rig's bones — so a leg is one
// mesh that folds at the knee rather than a thigh and a shin that pivot.
// The skinning is baked into path keyframes here, which is why the clips
// stay plain Lottie and the renderer needs no notion of bones.
//
//	go run ./gen -grid   # write the coordinate overlays the cuts are read from
//	go run ./gen         # cut the parts and build the bundle
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// The sheets, and where each figure sits on them. Read off the overlays
// that -grid writes.
const (
	sheetMain  = "sheets/character-sheet.jpg"
	sheetExtra = "sheets/expressions-and-angles.jpg"
	partsDir   = "parts"
	frontX     = 200
	frontY     = 90
	frontW     = 340
	frontH     = 620
)

func main() {
	grid := flag.Bool("grid", false, "write coordinate overlays instead of building")
	flag.Parse()
	if err := run(*grid); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(gridOnly bool) error {
	sheet, err := loadSheet(sheetMain)
	if err != nil {
		return fmt.Errorf("%s: %w (run from the sample directory)", sheetMain, err)
	}
	front := lift(sheet, frontX, frontY, frontW, frontH)

	if gridOnly {
		if err := os.MkdirAll("work", 0o755); err != nil {
			return err
		}
		if err := front.grid(filepath.Join("work", "front-grid.png")); err != nil {
			return err
		}
		fmt.Println("wrote work/front-grid.png")
		return nil
	}

	if err := os.MkdirAll(partsDir, 0o755); err != nil {
		return err
	}
	parts, err := front.cutAll(regions)
	if err != nil {
		return err
	}
	for _, r := range regions {
		p := parts[r.name]
		if err := writePNG(filepath.Join(partsDir, p.name+".png"), p.img); err != nil {
			return err
		}
		fmt.Printf("%-12s %3dx%-3d at (%d,%d)\n",
			p.name, p.img.Bounds().Dx(), p.img.Bounds().Dy(), p.x, p.y)
	}
	return nil
}
