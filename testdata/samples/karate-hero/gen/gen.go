// Command gen draws the karate-hero part sprites: a gi-clad martial artist
// with spiky hair and a red headband, built as a design swap over the
// chibi-male preset. It only replaces images — sizes, anchors, and file
// names stay exactly on the rig contract (skills/lottie-character-preset/
// references/rig.md), so every chibi-male clip plays unchanged.
//
// Rebuild the sample:
//
//	go run ./gen -out parts
//	go run github.com/shibukawa/lottie-go/cmd/lottierepack -dump -dir work karate-hero.lottie
//	cp parts/*.png work/parts/
//	go run github.com/shibukawa/lottie-go/cmd/lottierepack -dir work -out karate-hero.lottie
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

const pxScale = 3

var palette = map[byte]color.NRGBA{
	'O': {38, 32, 43, 255},    // outline
	'K': {46, 38, 38, 255},    // hair (near-black brown)
	'P': {245, 205, 160, 255}, // skin
	'e': {38, 32, 43, 255},    // eye / mouth
	'W': {242, 240, 232, 255}, // gi white
	'w': {206, 202, 190, 255}, // gi shade / lapel line
	'V': {212, 208, 198, 255}, // gi seen from behind (dimmer, keeps turns readable)
	'v': {182, 178, 168, 255}, // back-gi shade
	'R': {200, 52, 44, 255},   // headband + belt red
	'r': {150, 38, 34, 255},   // red shade (knot, tails)
	'D': {0, 0, 0, 255},       // shadow
}

// Three-quarter face: spiky hair on top, red headband above the eyes,
// hair trailing down the back (-x) edge, band tails hanging behind.
var headArt = []string{
	"....O.....O.....O.......",
	"...OKO...OKO...OKO......",
	"..OKKKO.OKKKO.OKKKO.....",
	".OKKKKKKKKKKKKKKKKKO....",
	".OKKKKKKKKKKKKKKKKKKKO..",
	".ORRRRRRRRRRRRRRRRRRRRO.",
	"OKKKPPPPPPPPPPPPPPPPPPPO",
	"OKRKPPPPPPPPPPPPPPPPPPPO",
	"OKRKPPPPPPeePPPPPeePPPPO",
	"OKRKPPPPPPeePPPPPeePPPPO",
	"OKKKPPPPPPeePPPPPeePPPPO",
	"OKKKPPPPPPPPPPPPPPPPPPPO",
	"OKKPPPPPPPPPPPPPPPPPPPPO",
	".OKPPPPPPPPPPPPPPPPPPPO.",
	".OPPPPPPPPPPPeePPPPPPPO.",
	"..OPPPPPPPPPPPPPPPPPPO..",
	"...OPPPPPPPPPPPPPPPPO...",
	"....OOPPPPPPPPPPPPOO....",
	"......OOOOOOOOOOOO......",
	"........................",
}

// Rear-quarter head for turns and spin attacks: back-of-head hair with
// just the leading eye visible, band tails at the back center.
var headSideArt = []string{
	"....O.....O.....O.......",
	"...OKO...OKO...OKO......",
	"..OKKKO.OKKKO.OKKKO.....",
	".OKKKKKKKKKKKKKKKKKO....",
	".OKKKKKKKKKKKKKKKKKKKO..",
	".ORRRRRRRRRRRRRRRRRRRRO.",
	"OKKKKKKKKKKKKKKKPPPPPPPO",
	"OKKKKKKKKRRKKKKKPPPPPPPO",
	"OKKKKKKKKRRKKKKKPPeePPPO",
	"OKKKKKKKKRRKKKKKPPeePPPO",
	"OKKKKKKKKKKKKKKKPPeePPPO",
	"OKKKKKKKKKKKKKKKPPPPPPPO",
	"OKKKKKKKKKKKKKKKPPPPPPPO",
	".OKKKKKKKKKKKKKKPPPPPPO.",
	".OKKKKKKKKKKKKKKPPPPPPO.",
	"..OKKKKKKKKKKKKKPPPPPO..",
	"...OKKKKKKKKKKKKKPPPO...",
	"....OOKKKKKKKKKKPPOO....",
	"......OOOOOOOOOOOO......",
	"........................",
}

// Fully turned-away head: all hair, band knot and tails at center back.
var headBackArt = []string{
	"....O.....O.....O.......",
	"...OKO...OKO...OKO......",
	"..OKKKO.OKKKO.OKKKO.....",
	".OKKKKKKKKKKKKKKKKKO....",
	".OKKKKKKKKKKKKKKKKKKKO..",
	".ORRRRRRRRRRRRRRRRRRRRO.",
	"OKKKKKKKKKKrrKKKKKKKKKKO",
	"OKKKKKKKKKKRRKKKKKKKKKKO",
	"OKKKKKKKKKKRRKKKKKKKKKKO",
	"OKKKKKKKKKKRRKKKKKKKKKKO",
	"OKKKKKKKKKKKKKKKKKKKKKKO",
	"OKKKKKKKKKKKKKKKKKKKKKKO",
	"OKKKKKKKKKKKKKKKKKKKKKKO",
	".OKKKKKKKKKKKKKKKKKKKKO.",
	".OKKKKKKKKKKKKKKKKKKKKO.",
	"..OKKKKKKKKKKKKKKKKKKO..",
	"...OKKKKKKKKKKKKKKKKO...",
	"....OOKKKKKKKKKKKKOO....",
	"......OOOOOOOOOOOO......",
	"........................",
}

// Gi jacket: skin V at the collar, lapel line running down toward the
// belt, red belt with the knot on the leading side, gi skirt below.
var bodyArt = []string{
	"...OOOOOOOOOO...",
	"..OWWWWPPWWWWO..",
	".OWWWWwPPwWWWWO.",
	"OWWWWWWwPwWWWWWO",
	"OWWWWWWWwWWWWWWO",
	"OWWWWWWwWWWWWWWO",
	"OWWWWWWwWWWWWWWO",
	"OWWWWWwWWWWWWWWO",
	"OWWWWWwWWWWWWWWO",
	"OWWWWwWWWWWWWWWO",
	"OWWWWwWWWWWWWWWO",
	"OwwwwwwwwwwwwwwO",
	"ORRRRRRRRRRRRRRO",
	"ORRRRRRRRRrrRRRO",
	"OWWWWWWWWWrrWWWO",
	".OOOOOOOOOOOOOO.",
}

// Mid-turn torso: the gi thinned to the side silhouette.
var bodySideArt = []string{
	"....OOOOOOOO....",
	"...OWWWWWWWWO...",
	"...OWWWWWWWWO...",
	"...OWWWWWWWWO...",
	"...OWWWWWWWWO...",
	"...OWWWWWWWWO...",
	"...OWWWWWWWWO...",
	"...OWWWWWWWWO...",
	"...OWWWWWWWWO...",
	"...OWWWWWWWWO...",
	"...OWWWWWWWWO...",
	"...OwwwwwwwwO...",
	"...ORRRRRRRRO...",
	"...ORRRRRRRRO...",
	"...OWWWWWWWWO...",
	"....OOOOOOOO....",
}

// Torso from behind: plain dimmer gi, no lapels, flat belt — the tone
// drop is what makes a turned-away pose read at a glance.
var bodyBackArt = []string{
	"...OOOOOOOOOO...",
	"..OVVVVVVVVVVO..",
	".OVVVVVVVVVVVVO.",
	"OVVVVVVVVVVVVVVO",
	"OVVVVVVVVVVVVVVO",
	"OVVVVVVVVVVVVVVO",
	"OVVVVVVVVVVVVVVO",
	"OVVVVVVVVVVVVVVO",
	"OVVVVVVVVVVVVVVO",
	"OVVVVVVVVVVVVVVO",
	"OVVVVVVVVVVVVVVO",
	"OvvvvvvvvvvvvvvO",
	"OrrrrrrrrrrrrrrO",
	"OrrrrrrrrrrrrrrO",
	"OVVVVVVVVVVVVVVO",
	".OOOOOOOOOOOOOO.",
}

// Gi sleeve, hemmed at the elbow.
var upperArmArt = []string{
	".OOO.",
	"OWWWO",
	"OWWWO",
	"OWWWO",
	"OWWWO",
	"OWWWO",
	"OWWWO",
	"OwwwO",
	".OOO.",
}

// Sleeve end, bare forearm, red wrist wrap, bare fist.
var forearmArt = []string{
	".OOO.",
	"OWWWO",
	"OWWWO",
	"OwwwO",
	"OPPPO",
	"OPPPO",
	"ORRRO",
	"ORRRO",
	"OPPPO",
	".OOO.",
}

// Gi trousers.
var thighArt = []string{
	".OOOO.",
	"OWWWWO",
	"OWWWWO",
	"OWWWWO",
	"OWWWWO",
	"OWWWWO",
	"OWWWWO",
	"OWWWWO",
	"OWWWWO",
	"OwwwwO",
	".OOOO.",
}

// Trouser hem, bare shin, bare foot with the toe pointing +x.
var shinArt = []string{
	".OOOO..",
	"OWWWWO.",
	"OWWWWO.",
	"OWWWWO.",
	"OwwwwO.",
	"OPPPPO.",
	"OPPPPO.",
	"OPPPPO.",
	"OPPPPO.",
	"OPPPPPO",
	"OPPPPPO",
	".OOOOO.",
}

var shadowArt = []string{
	"........DDDDDDDDDDDD........",
	"....DDDDDDDDDDDDDDDDDDDD....",
	"..DDDDDDDDDDDDDDDDDDDDDDDD..",
	"....DDDDDDDDDDDDDDDDDDDD....",
	"........DDDDDDDDDDDD........",
}

type part struct {
	file string
	art  []string
	dark bool // far-side depth cue: colors x0.72, matching the preset
	w, h int  // rig-contract PNG size; the render must match exactly
}

var parts = []part{
	{"chibi-male-head.png", headArt, false, 72, 60},
	{"chibi-male-head-side.png", headSideArt, false, 72, 60},
	{"chibi-male-head-back.png", headBackArt, false, 72, 60},
	{"chibi-male-body.png", bodyArt, false, 48, 48},
	{"chibi-male-body-side.png", bodySideArt, false, 48, 48},
	{"chibi-male-body-back.png", bodyBackArt, false, 48, 48},
	{"chibi-male-upper-arm-near.png", upperArmArt, false, 15, 27},
	{"chibi-male-upper-arm-far.png", upperArmArt, true, 15, 27},
	{"chibi-male-forearm-near.png", forearmArt, false, 15, 30},
	{"chibi-male-forearm-far.png", forearmArt, true, 15, 30},
	{"chibi-male-thigh-near.png", thighArt, false, 18, 33},
	{"chibi-male-thigh-far.png", thighArt, true, 18, 33},
	{"chibi-male-shin-near.png", shinArt, false, 21, 36},
	{"chibi-male-shin-far.png", shinArt, true, 21, 36},
	{"chibi-male-shadow.png", shadowArt, false, 84, 15},
}

func render(art []string, dark bool) (*image.NRGBA, error) {
	w := len(art[0])
	for _, row := range art {
		if len(row) != w {
			return nil, fmt.Errorf("ragged art row %q", row)
		}
	}
	img := image.NewNRGBA(image.Rect(0, 0, w*pxScale, len(art)*pxScale))
	for y, row := range art {
		for x := range w {
			ch := row[x]
			if ch == '.' {
				continue
			}
			c, ok := palette[ch]
			if !ok {
				return nil, fmt.Errorf("unknown art pixel %q", ch)
			}
			if dark {
				c = color.NRGBA{
					uint8(float64(c.R) * 0.72),
					uint8(float64(c.G) * 0.72),
					uint8(float64(c.B) * 0.72), c.A}
			}
			for dy := range pxScale {
				for dx := range pxScale {
					img.SetNRGBA(x*pxScale+dx, y*pxScale+dy, c)
				}
			}
		}
	}
	return img, nil
}

func run(out string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	for _, p := range parts {
		img, err := render(p.art, p.dark)
		if err != nil {
			return fmt.Errorf("%s: %w", p.file, err)
		}
		if b := img.Bounds(); b.Dx() != p.w || b.Dy() != p.h {
			return fmt.Errorf("%s: drew %dx%d; rig contract wants %dx%d",
				p.file, b.Dx(), b.Dy(), p.w, p.h)
		}
		f, err := os.Create(filepath.Join(out, p.file))
		if err != nil {
			return err
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	fmt.Printf("wrote %d parts to %s\n", len(parts), out)
	return nil
}

func main() {
	out := flag.String("out", "parts", "output directory for the part PNGs")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
