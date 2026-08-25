---
id: decision:align-dotlottie-state-machine
type: decision
title: Align With dotLottie 2.0 State Machines
---

Adopt the dotLottie 2.0 state machine schema instead of inventing a format. It already standardizes multi-clip bundling plus state machines stored in `s/` (data:bundle-layout, data:state-machine).

```yaml
rationale:
  - interoperable with LottieFiles web and flutter runtimes
  - avoids a private format only lottie-go can read
  - bundle stays a plain ZIP readable by archive/zip (decision:no-cgo)
cost:
  - schema is web-interaction oriented; games need a subset (decision:game-oriented-sm-subset)
  - current reader is v1-only (requirement:player-state-machine)
reference: https://dotlottie.io/spec/2.0/
```

Unknown schema fields are preserved on load and written back unchanged, so a bundle edited here stays valid for other runtimes (policy:robustness).
