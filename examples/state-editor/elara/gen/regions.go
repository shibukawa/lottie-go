package main

// Where each part is cut from the front view, in sheet coordinates, and
// in what order they stack. The list runs BACK to FRONT, which is both
// the draw order and the order the cut resolves overlaps in: a part
// claims only what nothing in front of it already took.
//
// The polygons are placed by eye against work/front-grid.png and may be
// sloppy where they fall outside the figure — the cut intersects them
// with the figure's own alpha. What they really decide is where one part
// ENDS and the next begins, along seams the illustration does not draw:
// the shoulder, the hip, the hairline.
//
// The count is driven by depth, not by joints. A mesh bends, so a thigh
// and a shin are one part; a second part is needed only where something
// passes in front of something else during a clip. That is why there are
// ten of these and not the rig's usual sixteen.
type region struct {
	name string
	poly [][2]int
}

func box(x0, y0, x1, y1 int) [][2]int {
	return [][2]int{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
}

var regions = []region{
	{"cape", box(232, 240, 510, 660)},
	{"scabbard", box(414, 394, 476, 592)},
	{"arm-far", box(396, 284, 480, 444)},
	{"leg-far", box(350, 492, 428, 710)},
	{"tail-far", box(426, 108, 536, 306)},
	{"torso", box(280, 280, 430, 508)},
	{"leg-near", box(278, 492, 352, 710)},
	{"head", box(270, 90, 440, 294)},
	{"arm-near", box(250, 284, 324, 444)},
	{"sword", box(190, 396, 286, 608)},
}
