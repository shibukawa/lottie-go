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
//	work/<id>.json          one animation per file, named by its id
//	work/parts/<name>.png   shared images (the bundle's i/ or images/ dir)
//	work/machines/<id>.json state machines
//
// Repacking starts from the dumped bundle's manifest when -base points at
// one (or when the output already exists), so fields the directory does
// not carry survive the round trip. Animations present in the bundle but
// missing from the directory are removed — deleting a clip's file deletes
// the clip.
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
	flag.Parse()
	if *dir == "" {
		fmt.Fprintln(os.Stderr, "usage: lottierepack -dump -dir work file.lottie | lottierepack -dir work [-base file.lottie] [-out file.lottie]")
		os.Exit(2)
	}
	var err error
	if *dump {
		if flag.NArg() != 1 {
			fmt.Fprintln(os.Stderr, "lottierepack -dump needs exactly one .lottie argument")
			os.Exit(2)
		}
		err = dumpBundle(flag.Arg(0), *dir)
	} else {
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
	for _, sub := range []string{"parts", "machines"} {
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
	fmt.Printf("dumped %s into %s (%d clips, %d machines)\n", src, dir, len(b.AnimationIDs()), len(b.StateMachineIDs()))
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
		if err := b.SetAnimation(id, data); err != nil {
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
