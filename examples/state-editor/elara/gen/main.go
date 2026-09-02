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
	// The parts sheet draws each piece on its own, clear of the others,
	// which is what makes a clean rig possible at all: on the front view
	// the cape, both arms and the sword lie across one another, and no cut
	// line through that overlap exists in the drawing to find.
	sheetParts = "sheets/parts.jpg"
	// The expanded sheet redraws every piece from several angles, which is
	// where the turn clips' side and back views come from.
	sheetAngles = "sheets/parts-angles.jpg"
	partsDir    = "parts"
	frontX      = 200
	frontY      = 90
	frontW      = 340
	frontH      = 620
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
		for name, path := range map[string]string{
			"parts-grid.png":  sheetParts,
			"angles-grid.png": sheetAngles,
		} {
			sh, err := loadSheet(path)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			b := sh.Bounds()
			if err := lift(sh, 0, 0, b.Dx(), b.Dy()).grid(filepath.Join("work", name)); err != nil {
				return err
			}
		}
		fmt.Println("wrote the coordinate overlays into work/")
		return nil
	}

	if err := os.MkdirAll(partsDir, 0o755); err != nil {
		return err
	}
	sheet2, err := loadSheet(sheetParts)
	if err != nil {
		return fmt.Errorf("%s: %w", sheetParts, err)
	}
	pb := sheet2.Bounds()
	loose := lift(sheet2, 0, 0, pb.Dx(), pb.Dy())

	parts, err := front.cutAll(frontRegions)
	if err != nil {
		return err
	}
	fromSheet, err := loose.cutAll(partRegions)
	if err != nil {
		return err
	}
	for k, v := range fromSheet {
		parts[k] = v
	}
	for _, r := range faceRegions {
		panel := liftBox(sheet2, r.poly)
		panel.eraseMargin(6)
		keepLargest(panel.img)
		p, err := panel.trim(r.name)
		if err != nil {
			return err
		}
		parts[r.name] = p
	}
	for _, p := range parts {
		keepLargest(p.img)
	}
	all := append(append([]region{}, frontRegions...), partRegions...)
	all = append(all, faceRegions...)
	for _, r := range all {
		p := parts[r.name]
		if err := writePNG(filepath.Join(partsDir, p.name+".png"), p.img); err != nil {
			return err
		}
		fmt.Printf("%-12s %3dx%-3d at (%d,%d)\n",
			p.name, p.img.Bounds().Dx(), p.img.Bounds().Dy(), p.x, p.y)
	}
	return buildBundle(parts, "elara.lottie")
}
