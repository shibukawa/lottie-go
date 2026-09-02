---
id: flow:agent-edit-session
type: flow
title: Agent Edits Through the Editor
---

```yaml
flow:
  trigger: author starts cmd/lottie-state-editor with -mcp and hands the URL to a coding agent (decision:editor-mcp-server)
  steps:
    - id: orient
      action: describe scope=bundle; learn clips, machines, inputs, problems
    - id: locate
      action: select the address the task names (concept:editor-focus); the window follows — stage switch, parked key, opened tab
    - id: learn-fields
      action: inspect; read the form — field names, options, keyed-ness, current values
    - id: edit
      action: set / add / remove / move / pose / path on the focus (api:editor-mcp-tools); each reply carries focus, changed addresses, generation, problems
    - id: look
      action: render what=stage, and graph for machine work; judge the picture, not the numbers
    - id: check
      action: validate; fix what it names
    - id: save
      action: file op=save
  variants:
    whole_clip: raw op=put target=clip:<id> json=<lottie document>, or add kind=clip json — then look and check; the agent authors a clip from scratch or from a preset reference
    vector_from_scratch: add kind=clip template=blank; add kind=shape_layer; path verb=pen points=[...] closed=true; add kind=shape_item item=gradient_fill and set its stops; select key:<clip>/<frame> then path verb=move_vertex to pose it; add kind=texture path=... and path verb=seed_uv — every step answered with a render, so the agent draws by looking (requirement:vector-editing). The raw put stays available for an agent that would rather write the shape JSON whole
    human_intervenes: the author drags a part or clicks elsewhere; the next reply shows a moved focus or a generation refusal, and the agent re-inspects before writing
  failure:
    refused: reply lists valid fields, values or targets; retry once with the correction
    busy: dialog open or drag in progress; wait and retry
    unsupported: raw put decodes to UnsupportedFeatures; document unchanged, features named (decision:practical-subset)
```

Same loop as skills/lottie-character-preset — edit, render, check — with the render and the check answered by the running editor instead of a re-pack (requirement:editor-mcp).
