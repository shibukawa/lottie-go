---
id: api:editor-mcp-tools
type: api
title: Editor MCP Tool Set
---

Implemented in cmd/lottie-state-editor/mcptools.go (2026-09-02). Tool surface of requirement:editor-mcp: fifteen tools over concept:editor-focus, grouped read / edit / document / session. The inspector form is the schema, so tool parameters stay small and stable. Transport and placement: decision:editor-mcp-server.

```yaml
design:
  principle: few tools, self-describing replies — the agent learns fields from inspect, not from the tool list
  scoping: edit tools take an optional target address; absent means the focus
  reply: every tool returns {focus, generation, changed, problems} plus its payload as compact JSON text; render returns image content
  refusal: "{error, hint, valid: [...]}; valid lists field names, enum values or candidate addresses — never a bare failure"
  busy: a mutation while DialogOpen or a stage drag is in progress returns busy; the agent retries
read:
  describe:
    params: {scope: bundle|machine|clip|focus, detail: brief|full}
    returns: clips (id, length, layers, markers), machines (id, initial, states, inputs), config, problems; full adds per-state transitions and per-clip pose times
  select:
    params: {target: address, frame?: number}
    returns: focus plus form; performs the click's side effects (park, stage switch, tab)
  inspect:
    params: {target?: address}
    returns: form — fields with value, options, keyed, writable, problem; adjacent-key values for keyed fields. On a clip: its parts list (name, joint, hidden, order) and shape layers; on a shape layer: the item tree (path, kind, name, hidden, inert for unmodeled kinds) — list.parts and list.shapes as data
  render:
    params: {what: stage|sheet|window, frame?: number, overlays?: bool, samples?: int, width?: int}
    returns: PNG; stage draws through PreviewDraw over a checkerboard with the rig, pose, shape or collision overlay when overlays is true; sheet tiles sampled frames of the stage clip (lottiecheck -render in-session); window is the next drawn frame of the whole window, graph included
  validate:
    params: {}
    returns: Problems() plus UnsupportedFeatures per clip and name-guard findings
edit:
  set:
    params: {target?: address, fields: {name: value}}
    covers: every Set* / Rename* / Toggle* Model method — state form, transition target, guard, input, pose p/r/s/o, shape members (fill and stroke color, opacity, width, cap, join, miter, dashes; rect / ellipse / polystar parameters; group tr), gradient type and stop color, texture binding (asset, tint, wrap, filter, placement p/r/s/a), hitbox geometry and tags, span range, socket flags, config, clip length, ease
    atomic: fields apply together per call; one unknown field refuses the whole call with valid
  add:
    params: {kind: state|transition|guard|input|clip|machine|pose|shape_layer|shape_item|vertex|stop|texture|hitbox|span|body|socket, in?: address, from?: address, ...kind params}
    shape_item: "{item: group|path|rect|ellipse|polystar|fill|stroke|gradient_fill|gradient_stroke|trim|round|pucker|zigzag|offset|merge|repeater} into the selected group, or {from: shape address} to duplicate or paste across layers and clips (the tree clipboard)"
    texture: "{path} imports an image into the bundle and binds it to the selected fill or stroke; {asset: id} binds an existing one"
    clip: "{path} import | {template: blank} | {from: clip id} duplicate | {json} raw document, validated as an import"
    returns: the new address, selected
  remove:
    params: {target?: address}
  move:
    params: {target?: address, to?: number|address, delta?: number}
    covers: transition order, part draw order, shape item order, key retime (whole column or one layer), span edge and shift, gradient stop position
  pose:
    params: {verb: insert|insert_from|swap|delete|reparent|jump, frame?, from?: {clip, frame}, swap?: bool, parent?: layer-name, dir?: -1|1}
    why: column operations rewrite several layers at once (requirement:keyframe-timeline); they are not field writes
  path:
    params: {verb: move_vertex|move_handle|insert_vertex|delete_vertex|smooth|corner|close|open|pen|primitive|move_geometry|resize|set_uv|move_uv|scale_uv|seed_uv|clear_uv, target?: address, ...}
    pen: "{points: [[x, y]], closed: bool} commits one stroke as one path; coordinates in the layer's space, as the stage tool records them"
    primitive: "{kind: rect|ellipse|polystar, at: [x, y], size: [w, h]} drops the item where a click would; parameters edit through set afterwards"
    geometry: "{verb: move_geometry, delta} and {verb: resize, corner, delta} act on the selected item's box, the stage's box gizmo as a call"
    uv: "set_uv {vertex, uv: [u, v]}, move_uv, scale_uv {factor}, seed_uv and clear_uv act on the selected item's texture (requirement:texture-mapping vertex mapping)"
    why: path topology rewrites every key of the property (requirement:vector-editing); pen and primitive make geometry from points, not fields; UV rides the vertex list and is kept in step with vertex insert and delete
  undo:
    params: {what?: clip|machine}
    scope: two stacks — clip edits (pose, shape, texture) and machine edits (states, transitions, guards, inputs); omitted picks machine while the focus is on the machine, clip otherwise
document:
  raw:
    params: {op: get|put|patch, target: clip:<id>|machine:<id>|manifest, json?: object, patch?: rfc6902 list}
    why: the whole-clip path — an agent that already writes Lottie JSON (decision:ai-skills-workflow) keeps doing so; the editor validates, re-decodes, shows, and keeps the previous document on refusal
  file:
    params: {op: new|open|save|save_as|import|reload, path?: string, template?: string}
    viewer: save refused in viewer mode, as the button is
session:
  preview:
    params: {verb: play|pause|step|seek|fire|set_value|restart|show, frame?, delta?, input?, value?, target?: clip:<id>|machine:<id>}
    covers: the transport row and the interface tabs' try buttons; fire and set_value drive api:state-machine-runtime
resources:
  - lottie://focus
  - lottie://problems
  - lottie://clip/{id}.json
  - lottie://machine/{id}.json
count: 15 tools over ~300 Model methods, grouped by the noun they act on and the verb families the inspector already uses
```

The grouping mirrors requirement:selection-driven-ui: lists carry add and remove, the pane carries set, and the few operations that rewrite several things at once — pose columns, path topology, reorder — get a verb tool each.
