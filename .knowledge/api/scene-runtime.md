---
id: api:scene-runtime
type: api
title: Scene Runtime API
---

Go API for playing data:scene-document in a game and in the vision:scene-editor preview (decision:runtime-package-first). Extends api:state-machine-runtime. Implemented in core scene.go / sceneplayer.go.

```yaml
loading: # scene is a standalone file referencing bundles + assets (decision:scene-references-bundles)
  - "lottie.ParseScene([]byte) / lottie.DecodeScene(io.Reader) (*Scene, error)"
  - "scene.NewScenePlayer(resolve func(path string) (*Bundle, error))  // bundles only"
  - "scene.NewScenePlayerWithLoader(SceneLoader{Bundle, File})  // File serves image/font references; image formats come from consumer blank imports"
  - "scene.Validate() []error  // broken references are findings, not parse errors"
phases: # requirement:scene-phases
  - "sp.SetPhase(name) bool / sp.Phase() string  // re-entering replays"
  - "sp.OnPhaseChanged(func(from, to string)) / sp.OnPhaseEnd(func(phase string))  // OnPhaseEnd: a timed phase's duration elapsed, before any auto-advance"
game_loop:
  - "sp.Update() in ebiten.Game.Update  // advances the scene clock; entrances happen here (requirement:scene-timeline)"
  - "sp.Draw(screen, *lottie.DrawOptions)  // GeoM composes after the screen mapping"
chaining: # data:scene-document playback.then
  - a completed pass advances the node's chain (entrance clip -> idle loop) and delivers the complete event to its bindings
  - a looping link never completes, so the chain parks there
timeline:
  - "sp.Time() float64  // seconds since the scene (re)started"
  - "sp.Restart()       // replay entrances from zero; focus returns to the initial choice"
  - "n.Started() bool   // whether the node's entrance happened; editors keep unentered nodes arrangeable"
camera: # requirement:scene-camera — 2D camera with per-node parallax depth
  - "sp.Camera() SceneCamera / sp.SetCamera(c)  // runtime override, not persisted; entering a phase or Restart re-resolves from the document (scene camera, or the phase's override)"
  - "easing toward a phase camera: target = sp.Scene().CameraFor(sp.Phase()), never sp.Camera() — after SetCamera that is the override itself (pitfall hit in the searchlight sample)"
  - "c.GeoM(w, h, depth) ebiten.GeoM  // scene-to-view transform; depth scales every component: translation ×depth, zoom^depth, rotation ×depth — depth 0 is exactly identity (screen-pinned HUD)"
  - Draw composes per node camera(depth) -> screen mapping; Pointer, NodeAt, and directional focus geometry apply the same chain, so hits and focus match what is on screen
  - zoom/rotation pivot on the design box center; moving the camera shifts content the opposite way
screen_mapping: # design size (data:scene-document size) onto the real screen
  - "sp.SetScreenMapping(w, h int, mode ScaleMode)  // recompute on resize; default identity"
  - "modes: ScaleContain (letterbox, centered) | ScaleCover (fill, side crop) | ScaleStretch | ScaleCenter (1:1)"
  - Draw applies it; Pointer inverts it, so the game passes raw screen coords; sp.ScreenGeoM() for overlays
input: # pushed by the game each frame; the core reads no device, so keys stay rebindable
  - "sp.MoveFocus(dir)  // FocusUp Down Left Right Next Prev"
  - "sp.Activate()      // confirm button, delivered to the focused node"
  - "sp.Cancel()        // focused node first; unhandled surfaces as OnCallback(\"\", \"cancel\")"
  - "sp.Pointer(x, y float64, pressed bool)  // hover/press/activate from changes between calls; press focuses focusable nodes"
focus:
  - "sp.Focus(node string) bool / sp.Focused() string"
  - "sp.OnFocusChanged(func(from, to string))"
callbacks:
  - "sp.OnCallback(func(node, name string))  // requirement:scene-interactions callback actions"
actions: # binding execution details (requirement:scene-interactions)
  - fireEvent / playSegment resolve Target (empty = self) and force-start an unentered target
  - playSegment with an empty arg plays the whole clip from the top
  - the focus action moves focus to the node its arg names
nodes:
  - "sp.Node(name) (*SceneNodePlayer, bool) / sp.Nodes() / sp.NodeAt(x, y)  // NodeAt: topmost hit, editor picking"
  - "n.Fire(event) / n.Set(name, v) / n.Get[T](name)  // machine nodes, delegates to StateMachinePlayer"
  - "n.Player() *Player       // the clip playing: the animation node's own, or the machine's current"
  - "n.Machine() *StateMachinePlayer / n.Definition() *SceneNode"
  - "n.SetVisible(bool) / n.SetTransform(SceneTransform)  // runtime overrides, not persisted"
  - "n.SetText(s) / n.Text()  // text nodes: overwrite content by name (a score, a nickname); survives Restart and phase switches"
  - "n.LocalRect() (x, y, w, h)  // hit box in node coords; text shifts by its anchor — editors outline with this"
support:
  - "m.EnterState(name) bool on StateMachinePlayer — host convenience behind the node entry override"
```

Scenes are flat (vision:scene-editor): a sub-menu or dialog is another scene the game draws as a second ScenePlayer on top, routing input to the topmost one.

Values flow through the same generic Set/Get as api:state-machine-runtime (decision:generic-input-accessors): a HP bar in a menu is a machine node the game feeds with `n.Set("hp", v)`. Unknown node names report false, never panic (policy:robustness). ScaleX/ScaleY/Opacity resolve a zero value to 1 the way State.Speed does, so zero-value transforms still show.
