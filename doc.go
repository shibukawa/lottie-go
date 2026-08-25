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
//
// A dotLottie archive holding several clips is opened with DecodeBundle,
// which reads both the version 2 layout (a/ i/ s/ t/ f/) and the older
// version 1 one, and writes version 2.
//
// A bundle can also carry state machines, which let a game drive playback by
// name rather than by frame range:
//
//	sm, err := bundle.NewStateMachinePlayer("character")
//	if err != nil { ... }
//	sm.Fire("jump")
//
//	// in ebiten.Game:
//	func (g *Game) Update() error { sm.Update(); return nil }
//	func (g *Game) Draw(screen *ebiten.Image) { sm.Draw(screen, nil) }
//
// See StateMachine for the document model and StateMachinePlayer for how it
// runs.
package lottie
