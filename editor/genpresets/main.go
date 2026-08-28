// Command genpresets writes the character animation presets under
// testdata/presets. Presets are the templates AI-assisted workflows start
// from (see .knowledge requirement:animation-presets): raster cutout rigs
// whose part images are the contract a design swap must honor, with every
// clip and the state machine wired so a customized copy is game-ready.
// Everything is generated in-repository, so licensing is unambiguous.
//
//	go run ./genpresets
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	lottie "github.com/shibukawa/lottie-go"
)

func main() {
	out := flag.String("out", filepath.Join("..", "testdata", "presets"), "output directory")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(out string) error {
	// The default output is relative to the editor directory. Running from
	// elsewhere (genpresets/ itself is the classic slip) would silently
	// create a second testdata tree, so refuse unless the target exists.
	if _, err := os.Stat(out); err != nil {
		return fmt.Errorf("output directory %s not found: run from the editor directory (go run ./genpresets) or pass -out", out)
	}
	for _, p := range presets() {
		if err := writePreset(filepath.Join(out, p.name), p); err != nil {
			return err
		}
	}
	return nil
}

// preset is one character template: which rig slots it draws, which
// clips it plays, and how the machine wires them.
type preset struct {
	name    string
	parts   []part
	sword   bool
	defs    []clipDef
	machine *lottie.StateMachine
	readme  string
}

func presets() []preset {
	return []preset{
		{
			name: "chibi-male", parts: baseParts,
			defs: chibiMaleDefs(), machine: chibiMachine(false), readme: maleReadme,
		},
		{
			name: "chibi-sword", parts: swordParts, sword: true,
			defs: chibiSwordDefs(), machine: chibiMachine(true), readme: swordReadme,
		},
	}
}

func writePreset(dir string, p preset) error {
	// Everything downstream — part file names, the asset table, whether
	// clips carry a weapon layer — reads the active rig, so set it first.
	partPrefix, allParts, swordRig = p.name, p.parts, p.sword
	name := p.name
	if err := os.MkdirAll(filepath.Join(dir, "parts"), 0o755); err != nil {
		return err
	}
	clips := clipsOf(p.defs)
	if err := removeStaleClips(dir, clips); err != nil {
		return err
	}
	if err := removeStaleParts(filepath.Join(dir, "parts")); err != nil {
		return err
	}

	b := lottie.NewBundle()
	for _, p := range allParts {
		data, err := p.pngBytes()
		if err != nil {
			return fmt.Errorf("%s: %w", p.name, err)
		}
		// The loose copy is for inspection and for redrawing a part
		// without unzipping the bundle; the bundle copy is what plays.
		if err := os.WriteFile(filepath.Join(dir, "parts", p.file()), data, 0o644); err != nil {
			return err
		}
		b.SetImage(p.file(), data)
	}
	// Sorted so the manifest order is stable run to run.
	for _, id := range slices.Sorted(maps.Keys(clips)) {
		data, err := json.MarshalIndent(clips[id], "", " ")
		if err != nil {
			return fmt.Errorf("%s/%s: %w", name, id, err)
		}
		if err := os.WriteFile(filepath.Join(dir, id+".json"), data, 0o644); err != nil {
			return err
		}
		if err := b.SetAnimation(id, data); err != nil {
			return fmt.Errorf("%s/%s: %w", name, id, err)
		}
	}
	if err := b.SetStateMachine(name, p.machine); err != nil {
		return err
	}
	b.Manifest().Generator = "lottie-go/editor genpresets"
	if problems := b.Validate(); len(problems) > 0 {
		return fmt.Errorf("%s: %v", name, problems)
	}
	var buf bytes.Buffer
	if err := b.Encode(&buf); err != nil {
		return err
	}
	path := filepath.Join(dir, name+".lottie")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return err
	}
	if err := verify(buf.Bytes()); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	sheet, err := previewSheet(p.defs, 6)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "preview.png"), sheet, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(p.readme), 0o644); err != nil {
		return err
	}
	// The editor embeds the preset as a "New…" template, so keep its
	// copy in sync with every regeneration.
	if err := os.MkdirAll("templates", 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join("templates", name+".lottie"), buf.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d clips, %d parts, %d bytes)\n", path, len(clips), len(allParts), buf.Len())
	return nil
}

// verify re-reads the written bundle the way a game would and rejects the
// run if any clip fails to decode clean: presets must load with zero
// unsupported features, or the AI workflow's own validation baseline lies.
func verify(data []byte) error {
	b, err := lottie.DecodeBundle(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, id := range b.AnimationIDs() {
		a, err := b.Animation(id)
		if err != nil {
			return err
		}
		if notes := a.UnsupportedFeatures(); len(notes) > 0 {
			return fmt.Errorf("%s: unsupported features: %v", id, notes)
		}
	}
	return nil
}

// removeStaleParts deletes part images a rig change no longer produces,
// so an old slot's file does not look like part of the contract.
func removeStaleParts(dir string) error {
	entries, err := filepath.Glob(filepath.Join(dir, "*.png"))
	if err != nil {
		return err
	}
	current := map[string]bool{}
	for _, p := range allParts {
		current[p.file()] = true
	}
	for _, path := range entries {
		if current[filepath.Base(path)] {
			continue
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		fmt.Printf("removed stale %s\n", path)
	}
	return nil
}

// removeStaleClips deletes the .json clips in dir that are not being
// written, so a renamed clip does not leave its old name looking like
// part of the preset.
func removeStaleClips(dir string, clips map[string]obj) error {
	entries, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}
	for _, path := range entries {
		id := strings.TrimSuffix(filepath.Base(path), ".json")
		if _, ok := clips[id]; ok {
			continue
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		fmt.Printf("removed stale %s\n", path)
	}
	return nil
}
