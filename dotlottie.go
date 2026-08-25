package lottie

import (
	"fmt"
	"io"
)

// DecodeDotLottie reads a dotLottie (.lottie) archive and decodes the
// animation the manifest names as initial, or the first one listed. Image
// assets referenced by the animation are resolved from the archive.
//
// Use DecodeBundle to reach every animation in the archive along with its
// state machines.
func DecodeDotLottie(r io.ReaderAt, size int64) (*Animation, error) {
	b, err := DecodeBundle(r, size)
	if err != nil {
		return nil, err
	}
	return b.InitialAnimation()
}

// DecodeDotLottieAnimation decodes the animation with the given manifest id
// from a dotLottie archive. An empty id selects the initial animation.
func DecodeDotLottieAnimation(r io.ReaderAt, size int64, id string) (*Animation, error) {
	b, err := DecodeBundle(r, size)
	if err != nil {
		return nil, err
	}
	if id == "" {
		return b.InitialAnimation()
	}
	a, err := b.Animation(id)
	if err != nil {
		return nil, fmt.Errorf("lottie: dotLottie: %w", err)
	}
	return a, nil
}
