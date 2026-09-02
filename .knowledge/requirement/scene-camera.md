---
id: requirement:scene-camera
type: requirement
title: Scene Camera
---

2D camera for scenes: pan, zoom, and rotate the whole scene, with
per-node parallax depth, authored in the layout tool. Status: implemented
(scene.go / sceneplayer.go / cmd/lottie-layout).

```yaml
model: # data:scene-document
  - "scene camera {x, y, zoom, rotation}; zero value = identity, zoom 0 resolves to 1; phases override it (requirement:scene-phases); Validate rejects negative zoom"
  - "node depth = parallax factor: absent = 1 (full tracking), 0 = screen-pinned HUD, 0..1 = slower background, >1 = leading foreground; pointer field since 0 is meaningful"
runtime: # api:scene-runtime
  - camera GeoM per node between node transform and screen mapping; depth scales every component (translation ×depth, zoom^depth, rotation ×depth) so depth 0 is exactly identity
  - zoom/rotation pivot on design box center; camera motion shifts content the opposite way
  - hit tests (Pointer/NodeAt) and directional focus geometry apply the camera, matching the drawn positions
  - entering a phase / Restart resolves the document camera; SetCamera overrides per frame for game-driven moves
  - "Lottie 3D camera layers (ty: 13) stay excluded by policy:out-of-scope"
editor: # ui:layout-shell
  - Camera section in the scene pane; per-phase camera override checkbox + fields in the phase form; depth input in the node Transform pane
  - edit mode arranges in plain scene coords (runtime camera neutralized) and draws the camera framing as an overlay; preview mode plays the document camera
sample: examples/layout/searchlight — searchlight = screen-pinned depth-0 mask; the room's layers at depths 0.5/0.7/1/1.4; each search stop is a phase camera, the game eases between them (SetCamera per tick); lookAt(p, depth) = (p - center) / depth
resolved_tension:
  - keyframed camera moves stay out (requirement:scene-timeline wont); a static per-phase camera plus game-side SetCamera interpolation covers moves
```
