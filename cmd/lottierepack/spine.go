package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lottiespine "github.com/shibukawa/lottie-go/plugin/spine"
	lottietexture "github.com/shibukawa/lottie-go/plugin/texture"
)

type spineOptions struct {
	atlas, images, skins string
	fps, scale           float64
	mesh, bounds         string
	bones, machine       bool
}

// importSpine converts a Spine skeleton into the exploded layout under dir:
// a clip per animation, the images under parts/, the texture documents
// under extensions/texture/, and a state machine under machines/.
func importSpine(src, dir string, o spineOptions) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	sk, err := lottiespine.Parse(data)
	if err != nil {
		return fmt.Errorf("%s: %w", src, err)
	}
	srcDir := filepath.Dir(src)
	base := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	opts := lottiespine.Options{
		FPS: o.fps, Scale: o.scale, Mesh: lottiespine.MeshMode(o.mesh), Bones: o.bones,
	}
	switch o.bounds {
	case "union", "":
	case "skeleton":
		opts.SkeletonBounds = true
	default:
		return fmt.Errorf("lottierepack: -bounds must be union or skeleton, not %q", o.bounds)
	}
	for _, s := range strings.Split(o.skins, ",") {
		if s = strings.TrimSpace(s); s != "" {
			opts.Skins = append(opts.Skins, s)
		}
	}
	if o.machine {
		opts.MachineID = base
	}

	atlasPath := o.atlas
	if atlasPath == "" {
		if p := filepath.Join(srcDir, base+".atlas"); fileExists(p) {
			atlasPath = p
		}
	}
	if atlasPath != "" {
		raw, err := os.ReadFile(atlasPath)
		if err != nil {
			return err
		}
		if opts.Atlas, err = lottiespine.ParseAtlas(raw); err != nil {
			return fmt.Errorf("%s: %w", atlasPath, err)
		}
		atlasDir := filepath.Dir(atlasPath)
		opts.ReadPage = func(name string) ([]byte, error) {
			return os.ReadFile(filepath.Join(atlasDir, filepath.FromSlash(name)))
		}
		fmt.Printf("atlas %s (%d pages)\n", atlasPath, len(opts.Atlas.Pages))
	} else {
		imagesDir := o.images
		if imagesDir == "" {
			imagesDir = filepath.Join(srcDir, filepath.FromSlash(sk.Info.Images))
			if sk.Info.Images == "" {
				imagesDir = filepath.Join(srcDir, "images")
			}
		}
		opts.ReadImage = func(path string) ([]byte, error) {
			for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp"} {
				if data, err := os.ReadFile(filepath.Join(imagesDir, filepath.FromSlash(path)+ext)); err == nil {
					return data, nil
				}
			}
			return nil, fmt.Errorf("no %s.png under %s", path, imagesDir)
		}
		fmt.Printf("no atlas; loose images from %s\n", imagesDir)
	}

	res, err := lottiespine.Convert(sk, opts)
	if err != nil {
		return err
	}
	for _, sub := range []string{"parts", "machines", filepath.FromSlash(lottietexture.Dir)} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return err
		}
	}
	for _, id := range res.ClipOrder {
		if !safeComponent(id) {
			return fmt.Errorf("lottierepack: refusing unsafe clip id %q", id)
		}
		if err := writeIndented(filepath.Join(dir, id+".json"), res.Clips[id]); err != nil {
			return err
		}
		docPath := filepath.Join(dir, filepath.FromSlash(lottietexture.File(id)))
		if doc := res.Docs[id]; !doc.Empty() {
			raw, err := json.Marshal(doc)
			if err != nil {
				return err
			}
			if err := writeIndented(docPath, raw); err != nil {
				return err
			}
		} else {
			os.Remove(docPath)
		}
	}
	for name, data := range res.Images {
		if err := os.WriteFile(filepath.Join(dir, "parts", name), data, 0o644); err != nil {
			return err
		}
	}
	if res.Machine != nil {
		raw, err := json.MarshalIndent(res.Machine, "", " ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "machines", res.MachineID+".json"), raw, 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("imported %s into %s (%d clips, %d images, %gx%g, origin at %g,%g)\n",
		src, dir, len(res.ClipOrder), len(res.Images), res.Width, res.Height, res.OriginX, res.OriginY)
	for _, n := range res.Notes {
		fmt.Printf("note: %s\n", n)
	}
	return nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
