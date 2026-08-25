---
id: concept:phase-compositing
type: concept
title: Phase Compositing over Scratch Atlases
---

Implemented 2026-08-26 (render.go planPhases/preparePhases/emitPhase).
Within one layer list, every masked or matted layer's offscreen work runs
as shared phases over two process-wide scratch atlases, so fills batch
across layers and combines/composites merge into single draws
(concept:ebitengine-draw-batching). Batching is per animation instance;
draws stay linear across separately drawn players — idle players are
concept:idle-snapshot-cache's job.

```yaml
gating:
  decode: layerNode.phaseOK = (masks or matte) and not text; animations
    with phaseNodes == 0 never touch the machinery
  render: plans with < 2 active nodes use the pooled path (batching only
    pays from 2 nodes); atlas full or concurrent Draw -> per-node fallback
phases: [clear plan rows (1 draw per atlas), contents -> atlas A,
  coverage + matte sources -> atlas B, combines A<-B (merge),
  z-order emit (merges)]
nesting: no ping-pong needed — precomp surfaces are separate pooled
  images, so every combine crosses an image boundary; plans recurse
  through precomp layer lists (tier 2 works via the same code)
pitfalls_found:
  - per-region Clear never merges (builtin shader uniforms differ per
    region) -> each plan takes a private shelf span, cleared full-width
    in one draw per atlas
  - atlas images sized 1022 so Ebitengine's 1px padding still fits a
    1024 backend; 1024 would spill onto 2048 and quadruple the cost
  - kage dst.xy is a position on the internal texture; the gradient
    shader needed imageDstOrigin() (latent bug, fixed)
measured:
  synthetic_16_masked_layers: {draws: 197 -> 14, pixels: identical}
  nested_masked_precomp: {draws: 204 -> 24, pixels: identical}
  vram: engine stencil atlas high-water ~30-40MiB while such content
    plays; constant in player count
debug: LOTTIE_DISABLE_PHASES=1 forces the pooled path
```
