package main

// Where each part comes from, in its sheet's coordinates.
//
// Two sources, because neither has everything. The parts sheet draws the
// cape, the arms, a leg, the sword, the sheath and the faces on their
// own, which is the only way to get them clean: on the front view those
// pieces lie across one another and the drawing contains no line where
// they would have to be cut apart. What the parts sheet does not draw —
// the torso under its armour, and the twin-tails — comes off the front
// view, where nothing overlaps them.
//
// Boxes are read by eye against the overlays -grid writes. They may be
// sloppy: the cut intersects them with the figure's own alpha, and parts
// claim pixels front to back so no pixel lands in two parts.
type region struct {
	name string
	poly [][2]int
}

func box(x0, y0, x1, y1 int) [][2]int {
	return [][2]int{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
}

// From the front view: back to front.
var frontRegions = []region{
	{"tail-far", box(426, 108, 536, 306)},
	{"torso", box(280, 280, 430, 508)},
}

// From the parts sheet. Nothing here overlaps, so the order is only the
// order they get written in.
var partRegions = []region{
	{"cape", box(50, 150, 515, 625)},
	{"arm-near", box(505, 170, 705, 330)},
	{"arm-far", box(770, 165, 980, 360)},
	{"leg", box(585, 445, 705, 690)},
	{"sword", box(80, 595, 460, 690)},
	{"sheath", box(840, 440, 930, 690)},
}

// The faces sit inside drawn panels, each with its own pale ground, so
// they are lifted panel by panel rather than cut out of the sheet.
var faceRegions = []region{
	{"face-smile", box(1042, 172, 1178, 318)},
	{"face-resolve", box(1226, 172, 1362, 318)},
	{"face-blush", box(1030, 335, 1196, 485)},
	{"face-shock", box(1214, 335, 1380, 485)},
	{"face-wink", box(1030, 495, 1196, 645)},
	{"face-sad", box(1214, 495, 1380, 645)},
}

// The head comes apart on its own sheet: a bald base, a face, three
// pieces of hair and the veil's hood. Layering them is what lets the
// expression be swapped without redrawing the hair over it — the mesh
// has no interior vertices, so a face cannot be deformed into another
// expression, only replaced.
//
// These sit in drawn panels like the faces do, so they are lifted panel
// by panel rather than cut out of the sheet.
var headRegions = []region{
	{"head-base", box(74, 434, 228, 682)},
	{"face-3q", box(288, 148, 382, 248)},
	{"hair-bangs", box(244, 434, 352, 566)},
	{"hair-side", box(368, 434, 476, 566)},
	{"hair-pigtails", box(494, 578, 602, 688)},
	{"veil-hood", box(908, 432, 1032, 572)},
}
