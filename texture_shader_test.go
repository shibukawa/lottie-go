package lottie

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// The texture shader is Kage source compiled at first use; a typo there
// would otherwise surface as a panic in the first textured frame.
func TestTextureShaderCompiles(t *testing.T) {
	if _, err := ebiten.NewShader([]byte(texShaderSrc)); err != nil {
		t.Fatalf("texture shader: %v", err)
	}
	if _, err := ebiten.NewShader([]byte(gradShaderSrc)); err != nil {
		t.Fatalf("gradient shader: %v", err)
	}
}
