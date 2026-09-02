// Command gen writes the searchlight sample's assets: one bundle per
// actor and the scene that choreographs the camera. Everything is
// generated rather than downloaded so the licensing is unambiguous.
//
//	go run ./examples/layout/searchlight/gen
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	lottie "github.com/shibukawa/lottie-go"
)

func main() {
	out := flag.String("out", filepath.Join("examples", "layout", "searchlight", "assets"), "output directory")
	font := flag.String("font",
		filepath.Join("examples", "lottie", "stopwatch", "assets", "LuckiestGuy-Regular.ttf"),
		"display font to copy beside the scene")
	flag.Parse()
	if err := run(*out, *font); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(out, font string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	// The font sits beside the scene so the sample embeds a single
	// directory (go:embed cannot reach outside the package); the copy is
	// the stopwatch sample's Apache-2.0 typeface, kept in sync here.
	ttf, err := os.ReadFile(font)
	if err != nil {
		return fmt.Errorf("font: %w", err)
	}
	fontPath := filepath.Join(out, fontFile)
	if err := os.WriteFile(fontPath, ttf, 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d bytes)\n", fontPath, len(ttf))
	bundles := []struct {
		file, id string
		clip     obj
	}{
		{"wall.lottie", "wall-anim", wallClip()},
		{"shelf.lottie", "shelf-anim", shelfClip()},
		{"table.lottie", "table-anim", tableClip()},
		{"mouse.lottie", "mouse-anim", mouseClip()},
		{"plant.lottie", "plant-anim", plantClip()},
		{"mask.lottie", "mask-anim", maskClip()},
		{"alert.lottie", "alert-anim", alertClip()},
	}
	for _, bd := range bundles {
		b := lottie.NewBundle()
		data, err := json.Marshal(bd.clip)
		if err != nil {
			return fmt.Errorf("%s: %w", bd.id, err)
		}
		if err := b.SetAnimation(bd.id, data); err != nil {
			return fmt.Errorf("%s: %w", bd.id, err)
		}
		b.Manifest().Generator = "lottie-go searchlight gen"
		if problems := b.Validate(); len(problems) > 0 {
			return fmt.Errorf("%s: %v", bd.file, problems)
		}
		var buf bytes.Buffer
		if err := b.Encode(&buf); err != nil {
			return err
		}
		path := filepath.Join(out, bd.file)
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			return err
		}
		fmt.Printf("wrote %s (%d bytes)\n", path, buf.Len())
	}

	s := scene()
	if errs := s.Validate(); len(errs) > 0 {
		return fmt.Errorf("scene: %v", errs)
	}
	var sb bytes.Buffer
	if err := s.Encode(&sb); err != nil {
		return err
	}
	scenePath := filepath.Join(out, "searchlight.scene.json")
	if err := os.WriteFile(scenePath, sb.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d nodes, %d phases)\n", scenePath, len(s.Nodes), len(s.Phases))
	return nil
}

const sceneW, sceneH = 1280, 720

// fontFile is the display font's name beside the scene.
const fontFile = "LuckiestGuy-Regular.ttf"

// lookAt aims the camera so the scene point (px, py) of a node at the
// given parallax depth lands at the screen's center. The camera moves
// depth times further than the point for shallower nodes, which is the
// parallax working backwards.
func lookAt(px, py, depth, zoom, tilt float64) lottie.SceneCamera {
	return lottie.SceneCamera{
		X: round2((px - sceneW/2) / depth), Y: round2((py - sceneH/2) / depth),
		Zoom: zoom, Rotation: tilt,
	}
}

func round2(v float64) float64 { return float64(int(v*100+0.5)) / 100 }

func depth(d float64) *float64 { return &d }

// scene choreographs the search: each phase points the camera at one
// spot of the room — the picture, the clock, the window, the bookshelf,
// the plant, the floor — and the last one lands on the mouse at the
// table. The game eases the camera between phases (main.go); the phases
// hold the waypoints, so the whole search is authored in the layout
// tool.
//
// Parallax amplifies distance from the look-at point: at zoom 2 a
// depth-1.4 node scales 2.64x, so the plant is only in the beam when the
// beam looks straight at it — which is its own beat of the search.
func scene() *lottie.Scene {
	const search = 2.0 // zoom while searching; the beam sees a slice of the room
	look := func(name string, dur float64, next string, cam lottie.SceneCamera) lottie.ScenePhase {
		return lottie.ScenePhase{Name: name, Duration: dur, Next: next, Camera: &cam}
	}
	// Where things stand in the room (scene coordinates, at their depth).
	const (
		wallDepth  = 0.5
		shelfDepth = 0.7
		plantDepth = 1.4
	)
	mouseX, mouseY := 690.0, 300.0 // node origin; the mouse is 220x180
	return &lottie.Scene{
		Name:   "searchlight",
		Size:   lottie.SceneSize{W: sceneW, H: sceneH},
		Camera: lookAt(420, 200, wallDepth, search, 0),
		Bundles: []lottie.SceneBundle{
			{Alias: "wall", Path: "wall.lottie"},
			{Alias: "shelf", Path: "shelf.lottie"},
			{Alias: "table", Path: "table.lottie"},
			{Alias: "mouse", Path: "mouse.lottie"},
			{Alias: "plant", Path: "plant.lottie"},
			{Alias: "mask", Path: "mask.lottie"},
			{Alias: "alert", Path: "alert.lottie"},
		},
		Fonts: []lottie.SceneAsset{
			// Copied beside the scene by this generator (see run).
			{Alias: "display", Path: fontFile},
		},
		Phases: []lottie.ScenePhase{
			look("look-picture", 2.2, "look-clock", lookAt(420, 200, wallDepth, search, -3)),
			look("look-clock", 1.8, "look-window", lookAt(640, 190, wallDepth, search, 0)),
			look("look-window", 2.2, "look-shelf", lookAt(980, 230, wallDepth, search, 3)),
			look("look-shelf", 2.2, "look-plant", lookAt(130, 370, shelfDepth, search, -2)),
			// The foreground plant magnifies 2.64x at the search zoom; pulling
			// back to 1.2 fits its crown in the beam.
			look("look-plant", 2.0, "look-floor", lookAt(120, 460, plantDepth, 1.2, 2)),
			look("look-floor", 1.8, "found", lookAt(640, 660, 1, search, 0)),
			// Found: closer, and level.
			look("found", 0, "", lookAt(mouseX+110, mouseY+100, 1, 2.6, 0)),
		},
		Nodes: []lottie.SceneNode{
			{
				Name: "wall", Kind: lottie.SceneNodeAnimation,
				Source:   lottie.SceneSource{Bundle: "wall", ID: "wall-anim"},
				Depth:    depth(wallDepth),
				Playback: lottie.ScenePlayback{Loop: true, Autoplay: true},
			},
			{
				Name: "shelf", Kind: lottie.SceneNodeAnimation,
				Source:    lottie.SceneSource{Bundle: "shelf", ID: "shelf-anim"},
				Depth:     depth(shelfDepth),
				Transform: lottie.SceneTransform{X: 0, Y: 160},
				Playback:  lottie.ScenePlayback{Loop: true, Autoplay: true},
			},
			{
				Name: "table", Kind: lottie.SceneNodeAnimation,
				Source:    lottie.SceneSource{Bundle: "table", ID: "table-anim"},
				Transform: lottie.SceneTransform{X: 560, Y: 300},
				Playback:  lottie.ScenePlayback{Loop: true, Autoplay: true},
			},
			{
				// Nibbling all along; the light just has not reached it.
				Name: "mouse", Kind: lottie.SceneNodeAnimation,
				Source:    lottie.SceneSource{Bundle: "mouse", ID: "mouse-anim"},
				Transform: lottie.SceneTransform{X: mouseX, Y: mouseY},
				Playback:  lottie.ScenePlayback{Segment: "eat-seg", Loop: true, Autoplay: true},
			},
			{
				// Pops once the light lands; its completion startles the mouse.
				Name: "alert", Kind: lottie.SceneNodeAnimation,
				Source: lottie.SceneSource{Bundle: "alert", ID: "alert-anim"},
				Phase:  "found", Start: 0.5,
				Transform: lottie.SceneTransform{X: mouseX + 150, Y: mouseY - 120},
				Playback:  lottie.ScenePlayback{Segment: "pop-seg", Autoplay: true},
				Bindings: []lottie.SceneBinding{
					{On: lottie.SceneComplete, Do: lottie.ScenePlaySegment, Target: "mouse", Arg: "startled-seg"},
				},
			},
			{
				Name: "plant", Kind: lottie.SceneNodeAnimation,
				Source:    lottie.SceneSource{Bundle: "plant", ID: "plant-anim"},
				Depth:     depth(plantDepth),
				Transform: lottie.SceneTransform{X: -40, Y: 210},
				Playback:  lottie.ScenePlayback{Loop: true, Autoplay: true},
			},
			{
				// The beam: pinned to the screen while the camera roams.
				Name: "searchlight", Kind: lottie.SceneNodeAnimation,
				Source:   lottie.SceneSource{Bundle: "mask", ID: "mask-anim"},
				Depth:    depth(0),
				Playback: lottie.ScenePlayback{Loop: true, Autoplay: true},
			},
			{
				Name: "caption", Kind: lottie.SceneNodeText,
				Depth:     depth(0),
				Transform: lottie.SceneTransform{X: 640, Y: 46},
				Text: lottie.SceneText{
					Value: "Somebody is nibbling my cheese...", Font: "display", Size: 40,
					Align: lottie.AlignCenter, AnchorX: lottie.AlignCenter,
					AnchorY: lottie.AlignMiddle, Color: "#c9c2ad",
				},
			},
			{
				Name: "found-text", Kind: lottie.SceneNodeText,
				Phase: "found", Start: 1.0,
				Depth:     depth(0),
				Transform: lottie.SceneTransform{X: 640, Y: 650},
				Text: lottie.SceneText{
					Value: "FOUND YOU!", Font: "display", Size: 84,
					Align: lottie.AlignCenter, AnchorX: lottie.AlignCenter,
					AnchorY: lottie.AlignMiddle, Color: "#f9cc40",
				},
			},
		},
	}
}
