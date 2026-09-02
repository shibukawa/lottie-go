// Command gen draws the knight-nun part sprites: Elara, a nun in plate,
// built as a design swap over the chibi-sword preset. It only replaces
// images — sizes, anchors, and file names stay exactly on the rig
// contract (skills/lottie-character-preset/references/rig.md), so every
// chibi-sword clip, the sword included, plays unchanged.
//
// The character sheet asks for 3.5 heads tall; the rig is 2.5 and its
// proportions are the contract, so the design is carried over at the
// rig's build. Everything else from the sheet is here: navy veil over a
// white coif with a red cross, brown twin-tails, steel pauldrons and
// breastplate, pleated miniskirt, thigh-highs under armored greaves,
// and a gold-hilted holy sword.
//
// Rebuild the sample from this directory:
//
//	go run ./gen -out parts
//	go run github.com/shibukawa/lottie-go/cmd/lottierepack -dump -dir work knight-nun.lottie
//	go run ./gen -out work/parts
//	go run github.com/shibukawa/lottie-go/cmd/lottierepack -dir work -out knight-nun.lottie
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
	'N': {46, 56, 96, 255},    // navy habit
	'n': {32, 40, 72, 255},    // navy shade
	'V': {38, 46, 80, 255},    // habit seen from behind (dimmer)
	'v': {26, 33, 60, 255},    // back-habit shade
	'W': {243, 241, 235, 255}, // white coif and trim
	'w': {206, 203, 195, 255}, // white shade
	'S': {156, 164, 176, 255}, // steel plate (pauldrons, greaves)
	's': {112, 120, 134, 255}, // steel shade
	'C': {96, 104, 120, 255},  // cuirass, darker than the pauldrons
	'c': {68, 75, 90, 255},    // cuirass shade
	'P': {248, 214, 184, 255}, // skin
	'p': {216, 176, 146, 255}, // skin shade
	'H': {152, 88, 52, 255},   // hair
	'h': {118, 66, 40, 255},   // hair shade
	'e': {58, 40, 36, 255},    // eye outline and mouth
	'E': {146, 88, 52, 255},   // iris
	'R': {188, 54, 50, 255},   // red cross
	'K': {42, 40, 50, 255},    // thigh-high and boot
	'k': {30, 28, 38, 255},    // boot shade
	'B': {110, 74, 48, 255},   // belt and scabbard leather
	'b': {80, 54, 36, 255},    // leather shade
	'G': {200, 162, 76, 255},  // gold buckle, guard, pommel
	'M': {222, 228, 238, 255}, // blade steel
	'm': {162, 174, 192, 255}, // blade bevel
	'D': {0, 0, 0, 255},       // shadow
}

// Three-quarter face: navy veil, a white coif band across the brow with
// the red cross, big eyes, and the twin-tails filling the sides.
var headArt = []string{
	".......OOOOOOOO.........",
	".....OONNNNNNNNOO.......",
	"...OONNNNNNNNNNNNOO.....",
	"..ONNNNNNNNNNNNNNNNO....",
	".ONNNNNNWWWWRRWWWWNNNO..",
	".ONNNNNWWWRRRRRRWWWNNO..",
	"OHNNNNWWWWWRRWWWWWWNNO..",
	"OHHHNNWWWPPPPPPPPPPWWNO.",
	"OHHHHNWPPhhhhPPPPhhhPPO.",
	"OHHHHHHPPPPPPPPPPPPPPNO.",
	"OHHHHHHPPPeeeePPPeeePPO.",
	"OHHHHHHPPPeWEePPPeWEPPO.",
	"OHHHHHHHPPeEEePPPeEEPPO.",
	"OHHHHHHHPPeEEePPPeEEPPO.",
	"OHHHHHHHPPeeeePPPeeePPO.",
	".OHHHHHHPPPPPPPPPPPPPpO.",
	".OHHHHHHPPPPPPPeePPPPpO.",
	"..OHHHHHPPPPPPPPPPPPpO..",
	"...OHHHHHPPPPPPPPPPPO...",
	"....OOOOOOOOOOOOOO......",
}

// Rear-quarter head for turns and the spin attacks: mostly veil and the
// back of the twin-tails, with only the leading eye still showing.
var headSideArt = []string{
	".......OOOOOOOO.........",
	".....OONNNNNNNNOO.......",
	"...OONNNNNNNNNNNNOO.....",
	"..ONNNNNNNNNNNNNNNNO....",
	".ONNNNNNNNNWWWRRWWWNO...",
	".ONNNNNNNNWWWRRRRWWWNO..",
	"OHNNNNNNNWWWWWRRWWWWNO..",
	"OHHNNNNNNNNWWWPPPPPWWNO.",
	"OHHHNNNNNNNWWPPPPhhhPPO.",
	"OHHHHHHHHHHHPPPPPPPPPNO.",
	"OHHHHHHHHHHHPPPPPeeePPO.",
	"OHHHHHHHHHHHPPPPPeWEPPO.",
	"OHHHHHHHHHHHPPPPPeEEPPO.",
	"OHHHHHHHHHHHPPPPPeEEPPO.",
	"OHHHHHHHHHHHPPPPPeeePPO.",
	".OHHHHHHHHHHPPPPPPPPPpO.",
	".OHHHHHHHHHHPPPPPPPeepO.",
	"..OHHHHHHHHHHPPPPPPPpO..",
	"...OHHHHHHHHHHPPPPPPO...",
	"....OOOOOOOOOOOOOO......",
}

// Fully turned away: veil, the cross on the back of the coif, and the
// twin-tails hanging down either side.
var headBackArt = []string{
	".......OOOOOOOO.........",
	".....OONNNNNNNNOO.......",
	"...OONNNNNNNNNNNNOO.....",
	"..ONNNNNNNNNNNNNNNNO....",
	".ONNNNNNNNNNNNNNNNNNO...",
	".ONNNNNNNNNRRNNNNNNNNO..",
	"OHNNNNNNNNNRRNNNNNNNNHO.",
	"OHHNNNNNNRRRRRRNNNNNNHHO",
	"OHHNNNNNNNNRRNNNNNNNNHHO",
	"OHHHHHHHHHHRRHHHHHHHHHHO",
	"OHHHHHHHHHHHHHHHHHHHHHHO",
	"OHHHHHHHHHHHHHHHHHHHHHHO",
	"OHHHHHHHHHHHHHHHHHHHHHHO",
	".OHHHHHHHHHHHHHHHHHHHHO.",
	".OHHHHHHHHHHHHHHHHHHHHO.",
	"..OHHHHHHHHHHHHHHHHHHO..",
	"...OHHHHHHHHHHHHHHHHO...",
	"....OOHHHHHHHHHHHHOO....",
	"......OOHHHHHHHHOO......",
	"........OOOOOOOO........",
}

// Breastplate over the habit, belt at the waist, pleated miniskirt with
// the white underskirt showing at the hem.
var bodyArt = []string{
	"...OOOOOOOOOO...",
	"..OWWWWWWWWWWO..",
	".OCCCCCCCCCCCCO.",
	"OSCCCCCCCCCCCCSO",
	"OSCCCCCcCCCCCCSO",
	"OSCCCCcCcCCCCCSO",
	"OSCCCCCCCCCCCCSO",
	"OSccccccccccccSO",
	"OBBBBBBGGBBBBBBO",
	"ONNNNNNNNNNNNNNO",
	"ONNnNNNNnNNNNnNO",
	"ONNnNNNNnNNNNnNO",
	"ONNnNNNNnNNNNnNO",
	"OWWWWWWWWWWWWWWO",
	"OWWWWWWWWWWWWWWO",
	".OOOOOOOOOOOOOO.",
}

// Mid-turn torso: the same kit on a narrower silhouette.
var bodySideArt = []string{
	"....OOOOOOOO....",
	"...OWWWWWWWWO...",
	"...OCCCCCCCCO...",
	"...OCCCCCCCCO...",
	"...OCCCcCCCCO...",
	"...OCCCcCCCCO...",
	"...OCCCCCCCCO...",
	"...OccccccccO...",
	"...OBBBGGBBBO...",
	"...ONNNNNNNNO...",
	"...ONNnNNnNNO...",
	"...ONNnNNnNNO...",
	"...ONNnNNnNNO...",
	"...OWWWWWWWWO...",
	"...OWWWWWWWWO...",
	"....OOOOOOOO....",
}

// From behind: the habit's steel cross, no breastplate, and a dimmer
// navy so a turned-away pose reads at a glance.
var bodyBackArt = []string{
	"...OOOOOOOOOO...",
	"..OWWWWWWWWWWO..",
	".OVVVVVSSVVVVVO.",
	"OVVVVVVSSVVVVVVO",
	"OVVVVSSSSSSVVVVO",
	"OVVVVVVSSVVVVVVO",
	"OVVVVVVSSVVVVVVO",
	"OvvvvvvvvvvvvvvO",
	"OBBBBBBBBBBBBBBO",
	"OVVVVVVVVVVVVVVO",
	"OVVvVVVVvVVVVvVO",
	"OVVvVVVVvVVVVvVO",
	"OVVvVVVVvVVVVvVO",
	"OWWWWWWWWWWWWWWO",
	"OWWWWWWWWWWWWWWO",
	".OOOOOOOOOOOOOO.",
}

// Pauldron over a navy sleeve.
var upperArmArt = []string{
	".OOO.",
	"OSSSO",
	"OSSSO",
	"OsssO",
	"ONNNO",
	"ONNNO",
	"ONNNO",
	"OnnnO",
	".OOO.",
}

// Sleeve into a steel gauntlet.
var forearmArt = []string{
	".OOO.",
	"ONNNO",
	"ONNNO",
	"OnnnO",
	"OSSSO",
	"OSSSO",
	"OSSSO",
	"OsssO",
	"OSSSO",
	".OOO.",
}

// Thigh-high.
var thighArt = []string{
	".OOOO.",
	"OWWWWO",
	"OKKKKO",
	"OKKKKO",
	"OKKKKO",
	"OKKKKO",
	"OKKKKO",
	"OKKKKO",
	"OKKKKO",
	"OkkkkO",
	".OOOO.",
}

// Armored greave over the boot; the toe points +x.
var shinArt = []string{
	".OOOO..",
	"OKKKKO.",
	"OSSSSO.",
	"OSSSSO.",
	"OSSSSO.",
	"OssssO.",
	"OKKKKO.",
	"OKKKKO.",
	"OKKKKOO",
	"OKKKKKO",
	"OkkkkkO",
	".OOOOO.",
}

// The holy sword: gold pommel and crossguard, leather grip, long blade.
var swordArt = []string{
	"...O...",
	"..OGO..",
	"..OBO..",
	"..OBO..",
	".OGGGO.",
	"OGGGGGO",
	".OMMmO.",
	".OMMmO.",
	".OMMmO.",
	".OMMmO.",
	".OMMmO.",
	".OMMmO.",
	".OMMmO.",
	".OMMmO.",
	".OMMmO.",
	".OMMmO.",
	".OMMmO.",
	".OMMmO.",
	".OMMmO.",
	".OMMmO.",
	".OMMmO.",
	".OMMmO.",
	".OMMmO.",
	".OMMmO.",
	"..OMO..",
	"...O...",
}

// --- Slots the base rig does not have ---
//
// The character sheet carries a scabbard, a hanging veil, twin-tails and
// a pleated skirt, and none of those work as paint on an existing part:
// the skirt has to stay put while the thighs swing under it, the tails
// have to follow the head rather than the body, and the veil has to hang
// behind everything. So they are added slots, each parented to the part
// it hangs off and given a static transform — parenting alone carries
// them, which is why no clip needed a new keyframe.

// The pleated miniskirt, hung from the hip so the legs swing under it.
var skirtArt = []string{
	"...OOOOOOOOOOOO...",
	"..ONNNNNNNNNNNNO..",
	".ONNNnNNNNNNnNNNO.",
	".ONNNnNNNNNNnNNNO.",
	"ONNNNnNNNNNNnNNNNO",
	"ONNNNnNNNNNNnNNNNO",
	"OnnnnnnnnnnnnnnnnO",
	"OWWWWWWWWWWWWWWWWO",
	".OOOOOOOOOOOOOOOO.",
}

// The veil's tail, hanging down the back behind everything else.
var capeArt = []string{
	"..OOOOOO..",
	".ONNNNNNO.",
	"ONNNNNNNNO",
	"ONNNNNNNNO",
	"ONNNNNNNNO",
	"ONNNNNNNNO",
	"ONNNRRNNNO",
	"ONNRRRRNNO",
	"ONNNRRNNNO",
	"ONNNNNNNNO",
	"ONNNNNNNNO",
	"ONNNNNNNNO",
	"ONNNNNNNNO",
	"ONNNNNNNNO",
	"ONNNNNNNNO",
	"ONNNNNNNNO",
	"ONNNNNNNNO",
	"OnnnnnnnnO",
	"OWWWWWWWWO",
	".OWWWWWWO.",
	"..OWWWWO..",
	"...OOOO...",
}

// The scabbard on the trailing hip, gold-banded leather.
var scabbardArt = []string{
	".OO.",
	"OGGO",
	"OBBO",
	"OBBO",
	"OGGO",
	"OBBO",
	"OBBO",
	"OBBO",
	"OGGO",
	"OBBO",
	"OBBO",
	"OBBO",
	"OGGO",
	"OBBO",
	"OBBO",
	"ObbO",
	".OO.",
}

// One twin-tail, ribbon at the root. Drawn once and mirrored for the
// other side by the layer's scale.
var tailArt = []string{
	"..OOOO..",
	".OWWWWO.",
	"OWWWWWWO",
	".OHHHHO.",
	"OHHHHHHO",
	"OHHHHHHO",
	"OHHHHHHO",
	"OHHHHHHO",
	"OHHHHHHO",
	"OHHHHHHO",
	"OHHHHHHO",
	"OhHHHHhO",
	"OhHHHHhO",
	"OhHHHHhO",
	".OhHHhO.",
	".OhHHhO.",
	".OhHHhO.",
	"..OhhO..",
	"...OO...",
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
	{"chibi-sword-head.png", headArt, false, 72, 60},
	{"chibi-sword-head-side.png", headSideArt, false, 72, 60},
	{"chibi-sword-head-back.png", headBackArt, false, 72, 60},
	{"chibi-sword-body.png", bodyArt, false, 48, 48},
	{"chibi-sword-body-side.png", bodySideArt, false, 48, 48},
	{"chibi-sword-body-back.png", bodyBackArt, false, 48, 48},
	{"chibi-sword-upper-arm-near.png", upperArmArt, false, 15, 27},
	{"chibi-sword-upper-arm-far.png", upperArmArt, true, 15, 27},
	{"chibi-sword-forearm-near.png", forearmArt, false, 15, 30},
	{"chibi-sword-forearm-far.png", forearmArt, true, 15, 30},
	{"chibi-sword-thigh-near.png", thighArt, false, 18, 33},
	{"chibi-sword-thigh-far.png", thighArt, true, 18, 33},
	{"chibi-sword-shin-near.png", shinArt, false, 21, 36},
	{"chibi-sword-shin-far.png", shinArt, true, 21, 36},
	// The sword is NOT dimmed even though it hangs off the far arm: it
	// would read as background, and it draws in front of the body anyway.
	{"chibi-sword-sword.png", swordArt, false, 21, 78},
	{"chibi-sword-shadow.png", shadowArt, false, 84, 15},
	// Added slots, not part of the base rig contract.
	{"knight-nun-skirt.png", skirtArt, false, 54, 27},
	{"knight-nun-cape.png", capeArt, false, 30, 66},
	{"knight-nun-scabbard.png", scabbardArt, false, 12, 51},
	{"knight-nun-tail.png", tailArt, false, 24, 57},
}

func render(art []string, dark bool) (*image.NRGBA, error) {
	w := len(art[0])
	for _, row := range art {
		if len(row) != w {
			return nil, fmt.Errorf("ragged art row %q (%d cells, want %d)", row, len(row), w)
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
			return fmt.Errorf("%s: drew %dx%d; the rig contract wants %dx%d",
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
	slotsIn := flag.String("slots", "", "dumped clip directory to add the decoration slots to")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *slotsIn != "" {
		if err := addSlots(*slotsIn); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
