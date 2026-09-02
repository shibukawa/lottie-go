# elara — a UV-mesh character (sample)

エララ, the knight nun, built from AI-generated character sheets as a
**texture-mesh** rig rather than a cutout one. Each part is a contour
traced off its own silhouette, painted with a slice of the sheet through
per-vertex UV. A part *bends* instead of pivoting, so a leg is one mesh
that folds at the knee rather than a thigh and a shin that rotate — and
the drawing bends with it, armour plates and piping and all.

This is sample data, **not a template**. The templates the editor's New…
dialog offers stay `chibi-male` and `chibi-sword`.

## Where it stands

Assembly and preview work: `elara.lottie` holds the standing figure as
sixteen textured meshes with a one-state machine, and opens in the editor or
renders with `lottiecheck`. The skinning that drives the meshes from the
rig's joint angles is next; today the outlines are static.

```bash
go run ./cmd/lottiecheck -render out/ examples/state-editor/elara/elara.lottie
cd cmd/lottie-state-editor && go run . ../../examples/state-editor/elara/elara.lottie
```

A player without the texture extension shows the fills' flat colours
instead of the art — that is the format working as intended, not a
failure. `lottiecheck` loads the extension, so its renders show what a
game gets.

## The sheets, and why there are four

`sheets/` holds the source art, generated with Gemini and so free of any
third-party material — but unlike the rest of this repository's samples
it is not *drawn by code*, so it is committed as pixels rather than
regenerated.

The front view alone cannot supply parts. On it the cape, both arms and
the sword lie across one another, and the drawing holds no line where
they would have to be cut apart; boxes drawn over it caught four parts
cleanly and mangled six. The dedicated part sheets draw each piece on its
own, and cutting those needs nothing but a rough box round each.

- `character-sheet.jpg` — front and back views. Supplies the torso and
  the twin-tails, which the part sheets do not draw.
- `expressions-and-angles.jpg` — six expressions and four view angles.
- `parts.jpg` — every piece on its own, plus six faces.
- `parts-angles.jpg` — the same pieces from several angles, which is
  where the turn clips' side and back views will come from.
- `head-parts.jpg` — the head taken apart: a bald base, ten 3/4 faces,
  bangs, side hair, pigtails and the veil's hood.

The head needs that last sheet because a mesh has no interior vertices.
A face cannot be deformed into another expression, only replaced — so
the face has to be a separate plate laid on a bald base, with everything
that overlaps it stacked on top rather than drawn into it. Six layers
for a head, and swapping one of them changes the expression.

## Lifting the art off the paper

The figures are drawn on printed graph paper, and no distance to one
background colour separates them from it: the grid lines run from bright
beige down to a muted grey, and a tolerance wide enough to cross them
also swallows the character's whites. What separates them is position.
Everything unsaturated and not dark is *candidate* paper, and only the
part of it the crop's border can reach is actually paper — the coif, the
ribbons and the underskirt are enclosed by the drawing's own linework, so
connectivity protects them.

Three things needed more than that, each for its own reason:

- the printed **guide dashes** are a mid grey, so the brightness floor
  comes down far enough to take them;
- the **face panels** have their own pale ground, walled off from the
  sheet's border by a frame, so they are lifted panel by panel instead;
- the **frame** is as dark as the linework and touches the face, so
  neither colour nor connectivity tells them apart — size does, keeping
  only the largest run of pixels in the panel. The same measure clears
  the strays that would otherwise derail a silhouette trace, which starts
  at the topmost opaque pixel and would happily walk around a speck.

## Rebuilding

```bash
go run ./gen -grid   # coordinate overlays, which the cut boxes are read off
go run ./gen         # cut the parts and build elara.lottie
```

`parts/` and `work/` are intermediates and are not committed.
