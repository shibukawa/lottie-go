package lottie

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strings"
)

// dotLottieManifest is the manifest.json of a dotLottie archive.
type dotLottieManifest struct {
	Animations []struct {
		ID string `json:"id"`
	} `json:"animations"`
}

// DecodeDotLottie reads a dotLottie (.lottie) archive and decodes the first
// animation listed in its manifest. Image assets referenced by the
// animation are resolved from the archive.
func DecodeDotLottie(r io.ReaderAt, size int64) (*Animation, error) {
	return DecodeDotLottieAnimation(r, size, "")
}

// DecodeDotLottieAnimation decodes the animation with the given manifest id
// from a dotLottie archive. An empty id selects the first animation.
func DecodeDotLottieAnimation(r io.ReaderAt, size int64, id string) (*Animation, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("lottie: open dotLottie: %w", err)
	}
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[path.Clean(f.Name)] = f
	}
	readFile := func(name string) ([]byte, error) {
		f, ok := files[path.Clean(name)]
		if !ok {
			return nil, fmt.Errorf("lottie: %s not found in archive", name)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}

	if id == "" {
		manifest, err := readFile("manifest.json")
		if err != nil {
			return nil, fmt.Errorf("lottie: dotLottie manifest: %w", err)
		}
		var m dotLottieManifest
		if err := json.Unmarshal(manifest, &m); err != nil {
			return nil, fmt.Errorf("lottie: dotLottie manifest: %w", err)
		}
		if len(m.Animations) == 0 {
			return nil, fmt.Errorf("lottie: dotLottie manifest lists no animations")
		}
		id = m.Animations[0].ID
	}

	data, err := readFile("animations/" + id + ".json")
	if err != nil {
		return nil, err
	}
	resolver := func(dir, name string) ([]byte, error) {
		// Asset dirs are archive-relative like "/images/" or "images/".
		p := path.Join(strings.TrimPrefix(dir, "/"), name)
		if data, err := readFile(p); err == nil {
			return data, nil
		}
		return readFile(path.Join("images", name))
	}
	return decodeJSON(data, resolver)
}
