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
	{part: "tail-far", x: 322, y: 176, scale: 0.78, points: 26},
	{part: "arm-far", x: 300, y: 286, scale: 0.60, points: 34},
	{part: "leg", x: 282, y: 372, scale: 0.66, points: 34},
	{part: "sheath", x: 300, y: 350, scale: 0.62, points: 18, rot: 14},
	{part: "torso", x: 256, y: 300, scale: 0.66, points: 30},
	{part: "leg", x: 232, y: 372, scale: 0.66, points: 34, flipX: true},
	{part: "face-resolve", x: 256, y: 168, scale: 0.72, points: 24},
	{part: "arm-near", x: 212, y: 286, scale: 0.60, points: 34, flipX: true},
	{part: "sword", x: 214, y: 372, scale: 0.62, points: 22, rot: -108},
}
