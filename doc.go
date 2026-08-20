// Package lottie plays a practical subset of Lottie animations through
// Ebitengine's vector package, with no cgo and no dependencies beyond
// Ebitengine itself.
//
// The target is UI motion: icons, loaders, HUD elements, and transitions
// authored in Lottie editors such as Lottie Creator, Lottielab, SVGator,
// and Glaxnimate. Unsupported features are skipped without failing;
// Animation.UnsupportedFeatures reports what was skipped.
//
// Typical use:
//
//	anim, err := lottie.Decode(file)
//	if err != nil { ... }
//	player := anim.NewPlayer()
//	player.SetLoop(true)
//
//	// in ebiten.Game:
//	func (g *Game) Update() error { player.Update(); return nil }
//	func (g *Game) Draw(screen *ebiten.Image) { player.Draw(screen, nil) }
package lottie
