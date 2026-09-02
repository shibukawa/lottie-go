package main

// Where each part sits on the canvas, and in what order.
//
// The parts come off two sheets drawn at different sizes, so each one
// carries its own scale as well as its position — the cape is drawn half
// again as large as the torso. Everything here was set by eye against the
// rendered result; there is no measurement in the sheets to derive it
// from, because the sheets never drew the pieces together.
//
// Order runs back to front, and it is the only thing deciding overlap.

const (
	canvasW = 512
	canvasH = 512
	// Ground, and the hip the rig hangs from.
	groundY = 470.0
	hipX    = 256.0
	hipY    = 300.0
)

type placement struct {
	part  string // which cut part
	x, y  float64
	scale float64
	// points is how finely the silhouette is traced. A piece that only
	// travels needs a handful; one that bends at a joint needs enough of
	// them near the bend for the outline not to crease.
	points int
	flipX  bool
	// rot turns the part about its own centre, in degrees clockwise. The
	// sheets draw each piece at whatever angle read best on the page — the
	// sword lies flat across it — so the rig has to set them upright.
	rot float64
}

var layout = []placement{
	{part: "cape", x: 256, y: 250, scale: 0.62, points: 40},
	{part: "arm-far", x: 300, y: 286, scale: 0.60, points: 34},
	{part: "leg", x: 282, y: 372, scale: 0.66, points: 34},
	{part: "sheath", x: 300, y: 350, scale: 0.62, points: 18, rot: 14},
	{part: "torso", x: 256, y: 292, scale: 0.66, points: 30},
	{part: "leg", x: 232, y: 372, scale: 0.66, points: 34, flipX: true},
	// The head is six layers, not one. Stacking it this way is what lets
	// the expression be swapped for another of the sheet's ten without
	// touching the hair or the veil: the face is a plate laid on a bald
	// base, and everything that overlaps it — side hair, bangs — sits on
	// top rather than being drawn into it.
	// Scales differ within the head because the sheet draws its pieces at
	// whatever size fitted its panel: the face plate is drawn smaller than
	// the base it lies on, so it needs the larger factor to fill the
	// opening. Nothing in the sheet says what the ratio should be — these
	// are read off the render.
	{part: "hair-pigtails", x: 322, y: 176, scale: 0.95, points: 22},
	{part: "hair-pigtails", x: 190, y: 176, scale: 0.95, points: 22, flipX: true},
	{part: "head-base", x: 256, y: 146, scale: 0.85, points: 26},
	// The veil is a solid shape with no opening cut in it, so it goes
	// between the bald base and the face: behind it, the crown shows above
	// the bangs; in front of the face, it covers the face entirely.
	{part: "veil-hood", x: 256, y: 112, scale: 1.18, points: 26},
	{part: "face-3q", x: 259, y: 140, scale: 1.04, points: 20},
	{part: "hair-side", x: 214, y: 148, scale: 0.85, points: 18, flipX: true},
	{part: "hair-side", x: 300, y: 148, scale: 0.85, points: 18},
	{part: "hair-bangs", x: 256, y: 114, scale: 0.85, points: 26},
	{part: "arm-near", x: 212, y: 286, scale: 0.60, points: 34, flipX: true},
	{part: "sword", x: 214, y: 372, scale: 0.62, points: 22, rot: -108},
}
