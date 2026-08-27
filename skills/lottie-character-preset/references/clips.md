# Clips and the state machine

## The clip vocabulary

Every preset ships the same base set, so games can swap characters
without rewiring. Loops repeat until an input; one-shots return by
themselves; transitions bridge two loops.

- Loops: `idle`, `walk`, `run`, `fall-loop`, `guard`
- Bridges: `idle-turn`, `walk-turn`, `run-turn` (see below),
  `run-to-idle` (braking stop), `fall` (into fall-loop)
- One-shots: `slide`, `jump` (chains to fall), `hurt`, `death` (holds
  its last frame, terminal)
- Attacks (unarmed set): `punch`, `punch-2` (combo follow-up), `kick`,
  `jump-kick`; defense: `guard` (loop while `guarding` is true),
  `guard-hit` (flinch, returns to guard)

There is deliberately no dash clip: a dash is run played faster (a
state's `speed` or the game's `SetSpeed`), so adding one would just
duplicate run.

Turn clips actually turn rather than morphing: at the midpoint the limb
chains trade sides and the head and shoes x-mirror, switched instantly
on hold keyframes (`"h": 1`). A turn clip therefore ENDS facing the
other way — the game flips its Mirrored flag when the turn completes,
and the mirrored idle matches the end pose. Follow the same pattern when
authoring turns for new gaits: smooth rotations, hold-keyed side swap.

Attacks lead with the far-side limb — the one attached on the leading
(+x) edge, so a strike reaches the enemy in front instead of crossing
the body; the combo follow-up swings the trailing (near) limb through.
Kicks chamber first (knee fully folded) and snap the knee straight only
on the strike frame.

Clip ids end in `-anim`. Keep new clips in this style: a new attack is
`spin-kick-anim`, its state `spin-kick-state`, its trigger `spinKick` or
`spin` (inputs are the bare verbs games call `Fire` with).

## Clip JSON anatomy

Each dumped clip is one Lottie document:

```jsonc
{
  "fr": 60, "ip": 0, "op": 48,          // 60fps, frames 0..48
  "w": 256, "h": 256,
  "assets": [ {"id": "head", "p": "chibi-male-head.png", "u": "i/", "w": 72, "h": 60}, ... ],
  "layers": [
    { "ty": 2, "nm": "forearm-near", "ind": 1, "parent": 2,
      "refId": "forearm-near", "ip": 0, "op": 48, "st": 0,
      "ks": {
        "a": {"a": 0, "k": [7, 4]},      // anchor (static)
        "p": {"a": 0, "k": [7, 23]},     // position in parent space
        "s": {"a": 0, "k": [100, 100]},
        "r": {"a": 1, "k": [             // animated rotation
          {"t": 0,  "s": [-12], "i": {...}, "o": {...}},
          {"t": 24, "s": [-20], ...},
          {"t": 48, "s": [-12], ...}
        ]},
        "o": {"a": 0, "k": 100}
      } }
  ]
}
```

What matters when editing:

- A property is static (`"a": 0`, `k` = value) or keyframed (`"a": 1`,
  `k` = keyframe list). Promote freely between the two.
- A keyframe is `{"t": frame, "s": [values...]}` plus easing handles
  `i`/`o` (`{"x":[0.4],"y":[1]}` style). Reuse the handles already in
  the file: the preset uses eased handles for organic moves and
  `x: 0.5` linear handles for cyclic gaits. Keyframe `t`s must be
  ascending.
- Rotations are degrees, clockwise, `0` = hanging straight down (limbs)
  or upright (body). Negative swings a limb toward +x (forward). Elbows
  bend forward (negative), knees backward (positive).
- Layer order in the file is front-to-back. `parent` refers to `ind`.
  Do not renumber `ind` casually — parents and mattes point at it.
- The last keyframe should restate the first for a loop clip, and `op`
  should equal that last `t`. One-shots end on their resting pose (or
  hold, for death).

## Timing patterns that read well

- **Gait cycles** alternate contact and passing poses: contact at 0 and
  midpoint (legs split, body low), passing at the quarters (legs
  together, body high). Walk ~48 frames, run ~24.
- **Attacks** are windup (few frames, pulls back), strike (2-4 frames,
  linear easing — snap matters), hold, recover. Making an attack feel
  stronger is usually a longer windup and a shorter strike, not bigger
  angles.
- **Retiming a whole clip**: scale every keyframe `t` and `op` by the
  same factor. But prefer the state machine's per-state `speed` or the
  game's `SetSpeed` when the request is "faster/slower overall" —
  keyframe surgery is for changing the *shape* of the timing.

## State machine JSON

`work/machines/<id>.json` after a dump:

```jsonc
{
  "initial": "idle-state",
  "states": [
    { "name": "run-state", "animation": "run-anim",
      "autoplay": true, "loop": true,
      "transitions": [
        { "type": "Transition", "toState": "jump-state",
          "guards": [ {"type": "Event", "inputName": "jump"},
                      {"type": "Boolean", "inputName": "grounded",
                       "conditionType": "Equal", "compareTo": true} ] }
      ] },
    { "name": "kick-state", "animation": "kick-anim", "autoplay": true,
      "transitions": [ { "type": "Transition", "toState": "idle-state",
                         "guards": [ {"type": "Event", "inputName": "clipDone"} ] } ] },
    { "type": "GlobalState", "name": "anywhere-state",
      "transitions": [ ... ] }
  ],
  "inputs": [ {"type": "Event", "name": "jump"},
              {"type": "Boolean", "name": "grounded", "value": true} ],
  "interactions": [ { "type": "OnComplete",
                      "actions": [ {"type": "Fire", "inputName": "clipDone"} ] } ]
}
```

Conventions to preserve when wiring new states:

- **One-shots return via `clipDone`**: the `OnComplete` interaction fires
  it when any clip ends, so a one-shot state just declares
  `clipDone -> wherever`. Never add game-side timers.
- **Transition order is priority.** Within a state, earlier transitions
  win; the preset lists `jump` first in every grounded state so it beats
  a simultaneous move request. A state's own transitions also beat the
  GlobalState's — that is why a hurt event during guard plays guard-hit
  (guard-state declares it) instead of the global hurt.
- **GlobalState is "from anywhere"**: hurt and death live there. A
  terminal state (death) simply has no transitions.
- **Booleans gate, events trigger.** `grounded` guards jump and brings
  `fall-loop` home; `guarding` enters/exits guard. Multiple guards on
  one transition are ANDed.
- New event inputs must be declared in `inputs` or `Fire` does nothing.

After editing a machine, repack and run the editor
(`cd editor && go run . yourfile.lottie`) if a human wants to try the
wiring interactively; `lottiecheck` verifies references (a state naming a
missing animation fails the bundle validation).

## Verifying motion edits

`lottiecheck -render` samples each clip at spaced frames. For a motion
edit, look at the frames around your changed keyframes specifically —
e.g. after sharpening a punch, read the strike-frame PNG and confirm the
arm is extended there, not one frame later. If a preset's generator is
available (in-repo work), `editor/genpresets` regenerates presets from
pose data; for dumped bundles the JSON is the source of truth.
