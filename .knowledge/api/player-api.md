---
id: api:player-api
type: api
title: Public API Surface
---

Proposed Go API:

```yaml
api:
  loading:
    - "lottie.Decode(r io.Reader) (*Animation, error)"
    - "lottie.DecodeDotLottie(r io.ReaderAt, size int64) (*Animation, error)"
  player:
    - "anim.NewPlayer() *Player"
    - "p.SetLoop(bool) / p.SetSpeed(float64)"
    - "p.Play() / p.Pause() / p.Seek(time.Duration)"
  game_loop:
    - "p.Update() in ebiten.Game.Update"
    - "p.Draw(screen, &lottie.DrawOptions{GeoM, ColorScale}) in Draw"
  introspection:
    - "anim.Size() (w, h int)"
    - "anim.Duration() time.Duration"
    - "anim.UnsupportedFeatures() []string"
```

Draw executes flow:render-frame. UnsupportedFeatures supports policy:robustness debug mode.
