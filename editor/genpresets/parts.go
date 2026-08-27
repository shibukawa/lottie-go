package main

// Part sprites are authored as string art: one byte per art pixel, scaled
// up by pxScale so the PNGs land at the size the clips reference. String
// art keeps the character reviewable in a diff, which matters because
// these images are the contract AI design swaps must honor.
//
// The character is a deliberate placeholder: every slot gets its own flat
// color so a customizer can see exactly which image drives which limb.
// The view is three-quarter, not profile — both eyes show and both arm
// and leg chains flank the torso — because that is how game characters
// usually read. "Male" is carried by the motion data, not the drawing.

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
)

// pxScale is how many PNG pixels one art pixel covers.
const pxScale = 3

var palette = map[byte]color.NRGBA{
	'O': {38, 32, 43, 255},    // outline
	'T': {111, 203, 214, 255}, // head teal
	'e': {38, 32, 43, 255},    // eye
	'G': {95, 168, 90, 255},   // torso green
	'g': {76, 138, 73, 255},   // torso shade
	'B': {58, 53, 80, 255},    // belt
	'A': {224, 112, 158, 255}, // upper arm pink
	'F': {238, 154, 184, 255}, // forearm light pink
	'L': {74, 111, 165, 255},  // thigh blue
	'S': {110, 147, 201, 255}, // shin light blue
	'W': {58, 53, 80, 255},    // shoe
	'D': {0, 0, 0, 255},       // shadow
}

// Three-quarter head: round, both eyes visible, the near eye closer to
// the leading (+x) edge.
var headArt = []string{
	"......OOOOOOOOOOOO......",
	"....OOTTTTTTTTTTTTOO....",
	"...OTTTTTTTTTTTTTTTTO...",
	"..OTTTTTTTTTTTTTTTTTTO..",
	".OTTTTTTTTTTTTTTTTTTTTO.",
	".OTTTTTTTTTTTTTTTTTTTTO.",
	"OTTTTTTTTTTTTTTTTTTTTTTO",
	"OTTTTTTTTTTTTTTTTTTTTTTO",
	"OTTTTTTTTTeeTTTTTeeTTTTO",
	"OTTTTTTTTTeeTTTTTeeTTTTO",
	"OTTTTTTTTTeeTTTTTeeTTTTO",
	"OTTTTTTTTTTTTTTTTTTTTTTO",
	"OTTTTTTTTTTTTTTTTTTTTTTO",
	".OTTTTTTTTTTTTTTTTTTTTO.",
	".OTTTTTTTTTTTTTTTTTTTTO.",
	"..OTTTTTTTTTTTTTTTTTTO..",
	"...OTTTTTTTTTTTTTTTTO...",
	"....OOTTTTTTTTTTTTOO....",
	"......OOOOOOOOOOOO......",
	"........................",
}

// headBackArt is the head seen from behind: the same silhouette with no
// face. punch-2 switches to it while winding up turned away.
var headBackArt = func() []string {
	rows := make([]string, len(headArt))
	for i, row := range headArt {
		rows[i] = strings.ReplaceAll(row, "e", "T")
	}
	return rows
}()

// headSideArt is the rear-quarter view a turning head passes through:
// mostly the back of the head with just the leading eye visible. Same
// silhouette, the trailing eye removed.
var headSideArt = func() []string {
	rows := make([]string, len(headArt))
	for i, row := range headArt {
		rows[i] = strings.Replace(row, "ee", "TT", 1)
	}
	return rows
}()

// bodySideArt is the torso mid-turn: the same canvas with a narrower
// silhouette, so the body visibly thins as it rotates instead of
// mirroring in place.
var bodySideArt = []string{
	"....OOOOOOOO....",
	"...OGGGGGGGGO...",
	"...OGGGGGGGGO...",
	"...OGGGGGGGGO...",
	"...OGGGGGGGGO...",
	"...OGGGGGGGGO...",
	"...OGGGGGGGGO...",
	"...OGGGGGGGGO...",
	"...OGGGGGGGGO...",
	"...OGGGGGGGGO...",
	"...OGGGGGGGGO...",
	"...OGGGGGGGGO...",
	"...OggggggggO...",
	"...OBBBBBBBBO...",
	"...OBBBBBBBBO...",
	"....OOOOOOOO....",
}

// The base torso stays undecorated on purpose: collars, plackets and
// buckles belong to customized variants, not the template. Front-ness is
// carried by the limb layout instead (see the near/far attach points).
var bodyArt = []string{
	"...OOOOOOOOOO...",
	"..OGGGGGGGGGGO..",
	".OGGGGGGGGGGGGO.",
	"OGGGGGGGGGGGGGGO",
	"OGGGGGGGGGGGGGGO",
	"OGGGGGGGGGGGGGGO",
	"OGGGGGGGGGGGGGGO",
	"OGGGGGGGGGGGGGGO",
	"OGGGGGGGGGGGGGGO",
	"OGGGGGGGGGGGGGGO",
	"OGGGGGGGGGGGGGGO",
	"OGGGGGGGGGGGGGGO",
	"OggggggggggggggO",
	"OBBBBBBBBBBBBBBO",
	"OBBBBBBBBBBBBBBO",
	".OOOOOOOOOOOOOO.",
}

var upperArmArt = []string{
	".OOO.",
	"OAAAO",
	"OAAAO",
	"OAAAO",
	"OAAAO",
	"OAAAO",
	"OAAAO",
	"OAAAO",
	".OOO.",
}

var forearmArt = []string{
	".OOO.",
	"OFFFO",
	"OFFFO",
	"OFFFO",
	"OFFFO",
	"OFFFO",
	"OFFFO",
	"OFFFO",
	"OFFFO",
	".OOO.",
}

var thighArt = []string{
	".OOOO.",
	"OLLLLO",
	"OLLLLO",
	"OLLLLO",
	"OLLLLO",
	"OLLLLO",
	"OLLLLO",
	"OLLLLO",
	"OLLLLO",
	"OLLLLO",
	".OOOO.",
}

// The shoe toe points forward (+x).
var shinArt = []string{
	".OOOO..",
	"OSSSSO.",
	"OSSSSO.",
	"OSSSSO.",
	"OSSSSO.",
	"OSSSSO.",
	"OSSSSO.",
	"OSSSSO.",
	"OWWWWO.",
	"OWWWWWO",
	"OWWWWWO",
	".OOOOO.",
}

var shadowArt = []string{
	"........DDDDDDDDDDDD........",
	"....DDDDDDDDDDDDDDDDDDDD....",
	"..DDDDDDDDDDDDDDDDDDDDDDDD..",
	"....DDDDDDDDDDDDDDDDDDDD....",
	"........DDDDDDDDDDDD........",
}

// renderArt turns string art into an image. dark dims the palette for
// far-side limbs so depth reads without a second drawing.
func renderArt(art []string, dark bool) *image.NRGBA {
	w := len(art[0])
	for _, row := range art {
		if len(row) != w {
			panic(fmt.Sprintf("ragged art row %q", row))
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
				panic(fmt.Sprintf("unknown art pixel %q", ch))
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
	return img
}

// part ties one slot to its sprite and rig placement. Anchor is the joint
// the part rotates around, in part image pixels; pos is where that anchor
// attaches in the parent's space — the body layer for head and limb
// roots, the parent limb segment for forearms and shins, the canvas for
// body and shadow.
type part struct {
	name   string
	art    []string
	dark   bool
	anchor [2]float64
	pos    [2]float64
}

func (p part) w() float64 { return float64(len(p.art[0]) * pxScale) }
func (p part) h() float64 { return float64(len(p.art) * pxScale) }

func (p part) file() string { return "chibi-male-" + p.name + ".png" }

func (p part) render() *image.NRGBA { return renderArt(p.art, p.dark) }

func (p part) pngBytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, p.render()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// The rig. Canvas coordinates are PNG pixels; the character stands on
// groundY with the body anchored at the hip. Hip height is the leg chain
// below its anchors: thigh (29-4) plus shin (36-4) = 57px.
const (
	canvas  = 256.0
	groundY = 236.0
	restX   = 128.0
	restY   = 179.0
)

var (
	headPart     = part{name: "head", art: headArt, anchor: [2]float64{36, 56}, pos: [2]float64{24, 3}}
	headSidePart = part{name: "head-side", art: headSideArt, anchor: [2]float64{36, 56}, pos: [2]float64{24, 3}}
	headBackPart = part{name: "head-back", art: headBackArt, anchor: [2]float64{36, 56}, pos: [2]float64{24, 3}}
	bodyPart     = part{name: "body", art: bodyArt, anchor: [2]float64{24, 45}, pos: [2]float64{restX, restY}}
	bodySidePart = part{name: "body-side", art: bodySideArt, anchor: [2]float64{24, 45}, pos: [2]float64{24, 45}}
	// Facing right in three-quarter view, the camera-side (near) limbs are
	// the character's left side, which trails on screen (-x); the far limbs
	// lead (+x) and peek out from behind the torso. Getting this backwards
	// reads as a back view.
	upperArmN    = part{name: "upper-arm-near", art: upperArmArt, anchor: [2]float64{7, 4}, pos: [2]float64{3, 8}}
	upperArmF    = part{name: "upper-arm-far", art: upperArmArt, dark: true, anchor: [2]float64{7, 4}, pos: [2]float64{45, 8}}
	forearmN     = part{name: "forearm-near", art: forearmArt, anchor: [2]float64{7, 4}, pos: [2]float64{7, 23}}
	forearmF     = part{name: "forearm-far", art: forearmArt, dark: true, anchor: [2]float64{7, 4}, pos: [2]float64{7, 23}}
	thighN       = part{name: "thigh-near", art: thighArt, anchor: [2]float64{9, 4}, pos: [2]float64{16, 45}}
	thighF       = part{name: "thigh-far", art: thighArt, dark: true, anchor: [2]float64{9, 4}, pos: [2]float64{32, 45}}
	shinNearPart = part{name: "shin-near", art: shinArt, anchor: [2]float64{9, 4}, pos: [2]float64{9, 29}}
	shinFarPart  = part{name: "shin-far", art: shinArt, dark: true, anchor: [2]float64{9, 4}, pos: [2]float64{9, 29}}
	shadowPart   = part{name: "shadow", art: shadowArt, anchor: [2]float64{42, 7}, pos: [2]float64{restX, groundY}}
)

var allParts = []part{
	headPart, headSidePart, headBackPart, bodyPart, bodySidePart,
	upperArmN, forearmN, upperArmF, forearmF,
	thighN, shinNearPart, thighF, shinFarPart,
	shadowPart,
}
