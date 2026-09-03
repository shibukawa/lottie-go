// Command lottierepack explodes a dotLottie bundle into an editable
// directory and rebuilds it, which makes bundles scriptable: an automated
// (AI) workflow dumps, edits the loose JSON and images with ordinary
// file tools, repacks, and then verifies with cmd/lottiecheck.
//
//	go run github.com/shibukawa/lottie-go/cmd/lottierepack -dump -dir work character.lottie
//	go run github.com/shibukawa/lottie-go/cmd/lottierepack -dir work -out character.lottie
//
// The directory layout mirrors what the pieces are:
//
//	work/<id>.json            one animation per file, named by its id
//	work/parts/<name>.png     shared images (the bundle's i/ or images/ dir)
//	work/machines/<id>.json   state machines
//	work/extensions/<path>    plugin payloads, the bundle's extensions/ subtree
//
// Repacking starts from the dumped bundle's manifest when -base points at
// one (or when the output already exists), so fields the directory does
// not carry survive the round trip, and JSON is stored compact however the
// directory indents it. Animations present in the bundle but
// missing from the directory are removed — deleting a clip's file deletes
// the clip. The same holds for extension files once work/extensions/
// exists; a directory without one leaves the base's payloads untouched.
//
// It also imports a Spine skeleton (JSON export, 4.x) into the same layout:
//
//	go run github.com/shibukawa/lottie-go/cmd/lottierepack -import-spine hero.json -dir work -out hero.lottie
//
// Every animation becomes a clip in which each slot is a shape layer and
// each region or mesh attachment a keyframed path painted through the
// texture extension (plugin/texture), so the meshes keep deforming with
// their art. The atlas (hero.atlas beside the JSON, or -atlas) supplies the
// images; without one, the loose images under the skeleton's images
// directory (or -images) do. -skin shows a skin over the default one, -fps
// and -scale set the sampling, -mesh triangles keeps every mesh triangle
// (exact inner deformation, about five times the size of the default hull),
// -tolerance sets how far a baked frame may stray from the straight line
// between keys before it is kept as a key (1 px by default, 0 keeps every
// frame) and -timing-tolerance how far along that line the easing fitted
// to each key may put it early or late (3 px; 0 for linear keys),
// -bounds skeleton keeps the declared size instead of growing it to what
// the animations reach, and -bones adds a null layer per bone. What the
// importer skipped is printed as notes.
package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	lottie "github.com/shibukawa/lottie-go"
)

func main() {
	dump := flag.Bool("dump", false, "explode the bundle into -dir instead of building one")
	dir := flag.String("dir", "", "directory holding the exploded bundle")
	out := flag.String("out", "", "bundle to write when repacking (default: the input of -dump, or -base)")
	base := flag.String("base", "", "existing bundle whose manifest and stray files carry over on repack")
	spineSrc := flag.String("import-spine", "", "Spine skeleton JSON to import into -dir (and -out when given)")
	spineAtlas := flag.String("atlas", "", "Spine atlas file (default: the skeleton's name with .atlas)")
	spineImages := flag.String("images", "", "directory of loose attachment images when there is no atlas (default: the skeleton's images path)")
	spineSkin := flag.String("skin", "", "comma-separated skins to show over the default skin")
	spineFPS := flag.Float64("fps", 30, "frames per second to bake the Spine animations at")
	spineScale := flag.Float64("scale", 1, "scale applied to every Spine coordinate")
	spineMesh := flag.String("mesh", "hull", "how a Spine mesh becomes paths: hull (the outline, light) or triangles (every triangle, exact)")
	spineBones := flag.Bool("bones", false, "add a null layer per Spine bone with its baked transform")
	spineTol := flag.Float64("tolerance", 1, "pixels a baked frame may stray from the straight line between keys before it becomes a key; 0 keeps every frame")
	spineTTol := flag.Float64("timing-tolerance", 3, "pixels along that line a frame may run early or late under the easing fitted to the keys; 0 writes linear keys only")
	spineBounds := flag.String("bounds", "union", "composition size: union (the skeleton's bounds widened to every animation's reach) or skeleton (the declared bounds only)")
	spineMachine := flag.Bool("machine", true, "generate a state machine with one state and one event per animation")
	flag.Parse()
	if *dir == "" {
		fmt.Fprintln(os.Stderr, "usage: lottierepack -dump -dir work file.lottie | lottierepack -dir work [-base file.lottie] [-out file.lottie] | lottierepack -import-spine skeleton.json -dir work [-out file.lottie]")
		os.Exit(2)
	}
	var err error
	switch {
	case *spineSrc != "":
		err = importSpine(*spineSrc, *dir, spineOptions{
			atlas: *spineAtlas, images: *spineImages, skins: *spineSkin,
			fps: *spineFPS, scale: *spineScale, mesh: *spineMesh, bones: *spineBones, machine: *spineMachine,
			bounds: *spineBounds, tolerance: *spineTol, timing: *spineTTol,
		})
		if err == nil && *out != "" {
			err = repack(*dir, *base, *out)
		}
	case *dump:
		if flag.NArg() != 1 {
			fmt.Fprintln(os.Stderr, "lottierepack -dump needs exactly one .lottie argument")
			os.Exit(2)
		}
		err = dumpBundle(flag.Arg(0), *dir)
	default:
		err = repack(*dir, *base, *out)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func dumpBundle(src, dir string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	b, err := lottie.DecodeBundle(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, sub := range []string{"parts", "machines", "extensions"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return err
		}
	}
	for _, id := range b.AnimationIDs() {
		if !safeComponent(id) {
			return fmt.Errorf("lottierepack: refusing unsafe animation id %q", id)
		}
		raw, _ := b.AnimationJSON(id)
		if err := writeIndented(filepath.Join(dir, id+".json"), raw); err != nil {
			return err
		}
	}
	for _, id := range b.StateMachineIDs() {
		if !safeComponent(id) {
			return fmt.Errorf("lottierepack: refusing unsafe machine id %q", id)
		}
		sm, err := b.StateMachine(id)
		if err != nil {
			return err
		}
		raw, err := json.MarshalIndent(sm, "", " ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "machines", id+".json"), raw, 0o644); err != nil {
			return err
		}
	}
	// Extension payloads keep their subtree: plugins claim directories
	// (extensions/physics/cp/, extensions/texture/) and some a single file
	// at the root (extensions/sockets.json). JSON is pretty-printed like
	// the clips; anything else is written verbatim.
	for _, name := range b.ExtensionFiles("") {
		rel, err := extensionRelPath(name)
		if err != nil {
			return err
		}
		raw, _ := b.ExtensionFile(name)
		dst := filepath.Join(dir, "extensions", rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := writeIndented(dst, raw); err != nil {
			return err
		}
	}
	// Images come from the archive itself: the Bundle API plays them but
	// does not enumerate them. Both archive layouts are covered.
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "i/") && !strings.HasPrefix(f.Name, "images/") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		img := make([]byte, 0, f.UncompressedSize64)
		buf := bytes.NewBuffer(img)
		if _, err := buf.ReadFrom(rc); err != nil {
			rc.Close()
			return err
		}
		rc.Close()
		name := path.Base(f.Name)
		if !safeComponent(name) {
			return fmt.Errorf("lottierepack: refusing unsafe image name %q", f.Name)
		}
		if err := os.WriteFile(filepath.Join(dir, "parts", name), buf.Bytes(), 0o644); err != nil {
			return err
		}
	}
	// Remember where this came from so a bare repack can find its base.
	if err := os.WriteFile(filepath.Join(dir, ".source"), []byte(src), 0o644); err != nil {
		return err
	}
	fmt.Printf("dumped %s into %s (%d clips, %d machines, %d extension files)\n", src, dir, len(b.AnimationIDs()), len(b.StateMachineIDs()), len(b.ExtensionFiles("")))
	return nil
}

// extensionRelPath turns a bundle member name under extensions/ into a
// path relative to work/extensions, checking every component: member
// names come from zip entries, which may try to climb out of -dir.
func extensionRelPath(name string) (string, error) {
	rel, ok := strings.CutPrefix(name, "extensions/")
	if !ok || rel == "" {
		return "", fmt.Errorf("lottierepack: refusing extension member %q outside extensions/", name)
	}
	parts := strings.Split(rel, "/")
	for _, p := range parts {
		if !safeComponent(p) {
			return "", fmt.Errorf("lottierepack: refusing unsafe extension member %q", name)
		}
	}
	return filepath.Join(parts...), nil
}

// syncExtensions makes work/extensions the authority over the bundle's
// extensions/ subtree, mirroring the clip rule: a file present is set, a
// member absent from the directory is removed. A directory that does not
// exist means the caller never dumped one (an older dump, a hand-built
// tree), and the base's payloads pass through as before.
func syncExtensions(b *lottie.Bundle, dir string) error {
	root := filepath.Join(dir, "extensions")
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	present := map[string]bool{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		name := "extensions/" + filepath.ToSlash(rel)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := b.SetExtensionFile(name, compactJSON(data)); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		present[name] = true
		return nil
	})
	if err != nil {
		return err
	}
	for _, name := range b.ExtensionFiles("") {
		if !present[name] {
			b.RemoveExtensionFile(name)
		}
	}
	return nil
}

func repack(dir, base, out string) error {
	explicitBase := base != ""
	if base == "" {
		if src, err := os.ReadFile(filepath.Join(dir, ".source")); err == nil {
			base = strings.TrimSpace(string(src))
		}
	}
	if out == "" {
		out = base
	}
	if out == "" {
		return fmt.Errorf("lottierepack: nowhere to write; pass -out (or -base, or dump first)")
	}
	if base == "" {
		// The doc's promise: an already-existing output serves as the base.
		if _, err := os.Stat(out); err == nil {
			base = out
		}
	}
	b := lottie.NewBundle()
	if base != "" {
		data, err := os.ReadFile(base)
		switch {
		case err != nil && explicitBase:
			// A -base that cannot be read must not be silently ignored:
			// the output would quietly lose the manifest metadata, fonts,
			// themes and stray files the base was meant to carry over.
			return fmt.Errorf("lottierepack: base %s: %w", base, err)
		case err != nil:
			fmt.Fprintf(os.Stderr, "lottierepack: recorded base %s unreadable (%v); repacking from the directory alone — manifest fields and stray files will not carry over\n", base, err)
		default:
			if b, err = lottie.DecodeBundle(bytes.NewReader(data), int64(len(data))); err != nil {
				return fmt.Errorf("lottierepack: base %s: %w", base, err)
			}
		}
	}

	clipFiles, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}
	present := map[string]bool{}
	for _, path := range clipFiles {
		id := strings.TrimSuffix(filepath.Base(path), ".json")
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := b.SetAnimation(id, compactJSON(data)); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		present[id] = true
	}
	for _, id := range b.AnimationIDs() {
		if !present[id] {
			b.RemoveAnimation(id)
		}
	}

	machineFiles, err := filepath.Glob(filepath.Join(dir, "machines", "*.json"))
	if err != nil {
		return err
	}
	for _, path := range machineFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var sm lottie.StateMachine
		if err := json.Unmarshal(data, &sm); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		id := strings.TrimSuffix(filepath.Base(path), ".json")
		if err := b.SetStateMachine(id, &sm); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}

	partFiles, err := filepath.Glob(filepath.Join(dir, "parts", "*"))
	if err != nil {
		return err
	}
	for _, path := range partFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		b.SetImage(filepath.Base(path), data)
	}

	if err := syncExtensions(b, dir); err != nil {
		return err
	}

	if problems := b.Validate(); len(problems) > 0 {
		return fmt.Errorf("lottierepack: %v", problems)
	}
	var buf bytes.Buffer
	if err := b.Encode(&buf); err != nil {
		return err
	}
	if err := os.WriteFile(out, buf.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d clips, %d bytes)\n", out, len(b.AnimationIDs()), buf.Len())
	return nil
}

// safeComponent reports whether a bundle-supplied name is safe to use as a
// single file-name component. Bundle ids come from zip entry names, and on
// Windows a '\' in one would escape -dir (path.Base does not split on it).
func safeComponent(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, `/\`)
}

// compactJSON strips the indentation the directory carries for editing:
// the bundle is what a game loads, and a pretty-printed clip is several
// times the bytes to store and parse. Anything that is not JSON passes
// through untouched.
func compactJSON(raw []byte) []byte {
	if !json.Valid(raw) {
		return raw
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return raw
	}
	return buf.Bytes()
}

func writeIndented(path string, raw []byte) error {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return os.WriteFile(path, raw, 0o644)
	}
	pretty, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return os.WriteFile(path, raw, 0o644)
	}
	return os.WriteFile(path, pretty, 0o644)
}
