---
id: flow:ai-character-forge
type: flow
title: Forge a Character from a Description
---

```yaml
flow:
  trigger: the user asks for a new character ("a fox-eared shrine maiden with a naginata, chibi, cel-shaded") in a project using lottie-go, with an image model at hand
  actors: [user, agent running skills/lottie-character-forge, image model (Gemini / Grok — driven by the user, or by an image tool the agent has), cmd/lottieforge, cmd/lottiecheck, cmd/lottie-state-editor (optional)]
  steps:
    - id: spec
      action: the agent writes work/character.json (data:character-forge-spec) — base preset chosen by whether the character holds something, description, style, key color, props, extra slots for hair or tail; nothing the spec can default is asked
    - id: grid
      action: lottieforge grid work/character.json -> work/sheets.json, work/sheets/*.template.png, work/prompts/{model,heads,torsos,limbs}.md filled with the base rig's slot list
    - id: model-sheet
      action: the user (or the agent's image tool) runs prompts/model.md; the image is saved as work/sheets/model.png. The agent checks it — three-quarter, facing right, neutral hang, key background, nothing cropped — or issues the fix-up prompt
    - id: part-sheets
      action: same with prompts/heads.md, torsos.md, limbs.md, each attaching model.png and the sheet's template; outputs saved as work/sheets/<sheet>.png
    - id: cut
      action: lottieforge cut work -> work/parts/, report.json, contact.png; the agent reads the report and looks at the contact sheet; a flagged cell (empty, border, multi, halo) goes back to part-sheets with the cell fix-up prompt — one cell, not the sheet
    - id: rig
      action: lottieforge rig work -o work/<name>.lottie; also writes faithful.png (t1 against t0 at idle frame 0) and refuses a contour the fan cannot texture, naming the part and the vertices
    - id: check
      action: lottiecheck -render work/preview work/<name>.lottie; the agent reads idle, the walk contact pose, an attack's strike frame and a turn's midpoint. A detached joint means a fit factor in the spec, then rig again; a wedge means a split slot
    - id: motion
      action: lottieforge morph work for the spec's morph list; new clips copied from their nearest template and reshaped per skills/lottie-character-preset; states wired; check again
    - id: review
      action: optional — the editor on the bundle, -viewer to watch or -mcp to pair; a human nudges a vertex or a UV point in the Shapes tab, the agent re-renders (flow:agent-edit-session)
    - id: deliver
      action: the bundle goes to the game; work/ stays as the source of the next variant (recolor, new outfit, another prop)
  variants:
    recolor_only: no images — recolor parts/ programmatically, rig again; the preset skill's lowest-risk swap, now at texture resolution
    raster_target: lottieforge rig -raster for a pixel-art look or a player without plugin/texture
    grid_ignored: a model that will not keep the template's grid gets the text-layout prompt; cut falls back to connected components ordered top-to-bottom, left-to-right and asks the agent to confirm the slot order against contact.png
    own_image_tool: an agent with an image tool runs model-sheet and part-sheets itself; the prompt files are the same
    variant_from_forge: a second character from the same work/ — new description, same base — reuses sheets.json and the fit factors
  failure:
    parts_not_split: the model drew one figure — re-prompt with the exploded-view wording and the template attached; last resort, ask for the model sheet with gaps between all limbs and cut it by connected components
    style_drift: cells in another rendering — attach model.png again with "same character, same rendering, same outline weight"; regenerate the one sheet
    detached_limb: a cap-to-cap length did not fit; report names the slot — set parts.<slot>.fit, or redraw the cell with the joint caps
    untextured_wedge: contour not star-shaped; rig names the part — split the slot (hair -> hair-front, hair-back) or set mapping bbox on it
    unsupported: lottiecheck names it; rig emits only what the renderer draws, so this points at a hand edit made afterwards
```

The loop is the preset skill's loop — edit, render, look — with the
image model as one more editor whose output is measured by cut before it
is trusted.
