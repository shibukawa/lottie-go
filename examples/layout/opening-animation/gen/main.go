// Command gen writes the opening sample's assets: the bundle of clips and
// the scene that choreographs them. Everything is generated rather than
// downloaded so the licensing is unambiguous.
//
//	go run ./examples/layout/opening-animation/gen
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
	out := flag.String("out", filepath.Join("examples", "layout", "opening-animation", "assets"), "output directory")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(out string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	// One bundle per actor, not one archive of parts: each layer of the
	// scene is its own file, so swapping the logo is replacing
	// logo.lottie and nothing else.
	bundles := []struct {
		file, id string
		clip     obj
	}{
		{"backdrop.lottie", "bg-dark-anim", darkBackdropClip()},
		{"sky.lottie", "bg-sky-anim", skyClip()},
		{"logo.lottie", "corp-logo-anim", corpLogoClip()},
		{"ocean.lottie", "ocean-anim", oceanClip()},
		{"title.lottie", "title-anim", titleClip()},
	}
	// The all-in-one archive of an earlier layout is stale if it is
	// still on disk.
	_ = os.Remove(filepath.Join(out, "opening.lottie"))
	for _, bd := range bundles {
		b := lottie.NewBundle()
		data, err := json.Marshal(bd.clip)
		if err != nil {
			return fmt.Errorf("%s: %w", bd.id, err)
		}
		if err := b.SetAnimation(bd.id, data); err != nil {
			return fmt.Errorf("%s: %w", bd.id, err)
		}
		b.Manifest().Generator = "lottie-go opening-animation gen"
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
	scenePath := filepath.Join(out, "opening.scene.json")
	if err := os.WriteFile(scenePath, sb.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d nodes, %d phases)\n", scenePath, len(s.Nodes), len(s.Phases))
	return nil
}

// scene choreographs the opening: the vanity card runs (or is skipped by
// a click or the confirm button), then the wave crashes over the sky, the
// title slams in as the sea settles, and PRESS START waits for the
// player.
func scene() *lottie.Scene {
	loopStep := func(segment string) []lottie.ScenePlayStep {
		return []lottie.ScenePlayStep{{Segment: segment, Loop: true}}
	}
	return &lottie.Scene{
		Name: "opening",
		Size: lottie.SceneSize{W: 1280, H: 720},
		// One bundle per layer, so any actor can be swapped by replacing
		// its file.
		Bundles: []lottie.SceneBundle{
			{Alias: "backdrop", Path: "backdrop.lottie"},
			{Alias: "sky", Path: "sky.lottie"},
			{Alias: "logo", Path: "logo.lottie"},
			{Alias: "ocean", Path: "ocean.lottie"},
			{Alias: "title", Path: "title.lottie"},
		},
		Fonts: []lottie.SceneAsset{
			// The stopwatch sample's CC-licensed display font, shared
			// rather than duplicated.
			{Alias: "display", Path: "../../../lottie/stopwatch/assets/LuckiestGuy-Regular.ttf"},
		},
		Phases: []lottie.ScenePhase{
			// The card holds exactly as long as its clip, then the
			// opening proper begins.
			{Name: "logo", Duration: 2, Next: "opening"},
			{Name: "opening"},
		},
		Options: lottie.SceneOptions{InitialFocus: "corp-logo"},
		Nodes: []lottie.SceneNode{
			{
				Name: "backdrop-dark", Kind: lottie.SceneNodeAnimation,
				Source:   lottie.SceneSource{Bundle: "backdrop", ID: "bg-dark-anim"},
				Phase:    "logo",
				Playback: lottie.ScenePlayback{Loop: true, Autoplay: true},
			},
			{
				Name: "corp-logo", Kind: lottie.SceneNodeAnimation,
				Source:    lottie.SceneSource{Bundle: "logo", ID: "corp-logo-anim"},
				Phase:     "logo",
				Transform: lottie.SceneTransform{X: 440, Y: 160},
				Playback:  lottie.ScenePlayback{Autoplay: true},
				Focus:     lottie.SceneFocus{Focusable: true},
				// Click or confirm skips straight to the opening.
				Bindings: []lottie.SceneBinding{
					{On: lottie.SceneActivate, Do: lottie.ScenePhaseAction, Arg: "opening"},
				},
			},
			{
				Name: "sky", Kind: lottie.SceneNodeAnimation,
				Source:   lottie.SceneSource{Bundle: "sky", ID: "bg-sky-anim"},
				Phase:    "opening",
				Playback: lottie.ScenePlayback{Loop: true, Autoplay: true},
			},
			{
				Name: "ocean", Kind: lottie.SceneNodeAnimation,
				Source: lottie.SceneSource{Bundle: "ocean", ID: "ocean-anim"},
				Phase:  "opening",
				// The big wave once, then the calm sea forever.
				Playback: lottie.ScenePlayback{
					Segment: "wave-seg", Autoplay: true,
					Then: loopStep("sea-seg"),
				},
			},
			{
				Name: "title-badge", Kind: lottie.SceneNodeAnimation,
				Source:    lottie.SceneSource{Bundle: "title", ID: "title-anim"},
				Phase:     "opening",
				Start:     3, // as the wave clears
				Transform: lottie.SceneTransform{X: 290, Y: 60},
				Playback: lottie.ScenePlayback{
					Segment: "in-seg", Autoplay: true,
					Then: loopStep("idle-seg"),
				},
			},
			{
				Name: "game-title", Kind: lottie.SceneNodeText,
				Phase: "opening", Start: 3.3,
				Transform: lottie.SceneTransform{X: 640, Y: 238},
				Text: lottie.SceneText{
					Value: "WAVE QUEST", Font: "display", Size: 84,
					Align: lottie.AlignCenter, AnchorX: lottie.AlignCenter,
					AnchorY: lottie.AlignMiddle, Color: "#fff3d6",
				},
			},
			{
				Name: "press-start", Kind: lottie.SceneNodeText,
				Phase: "opening", Start: 4.6,
				Transform: lottie.SceneTransform{X: 640, Y: 636},
				Text: lottie.SceneText{
					// Navy, since it sits on the swell's cream foam.
					Value: "PRESS START", Font: "display", Size: 44,
					Align: lottie.AlignCenter, AnchorX: lottie.AlignCenter,
					AnchorY: lottie.AlignMiddle, Color: "#1c3a5f",
				},
				Focus: lottie.SceneFocus{Focusable: true},
				Bindings: []lottie.SceneBinding{
					{On: lottie.SceneActivate, Do: lottie.SceneCallback, Arg: "start-game"},
				},
			},
		},
	}
}
