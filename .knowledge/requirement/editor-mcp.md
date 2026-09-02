---
id: requirement:editor-mcp
type: requirement
title: Editor as MCP Server
---

Status: implemented 2026-09-02 — cmd/lottie-state-editor mcp.go (server, queue, envelope), mcpaddr.go (addresses, focus), mcpform.go (inspect/set forms), mcptools.go (the fifteen tools), mcprender.go, mcppatch.go; must and should lists done. Let a coding agent drive vision:state-machine-editor through MCP while the author watches and intervenes in the same window. Decision and transport: decision:editor-mcp-server. Tools: api:editor-mcp-tools. Cursor model: concept:editor-focus. Session: flow:agent-edit-session.

```yaml
goals:
  - every operation the inspector offers is reachable by an agent without the mouse; edits land in the live document and redraw at once
  - agent and human share one selection, one stage clip and one playhead, so "here" means the same thing to both
  - the agent verifies by looking: stage and graph renders return as image content, drawn by the real renderer (decision:runtime-package-first, decision:json-level-animation-edit)
  - whole-document authoring stays possible: a clip or machine is created or replaced as raw JSON in one call, validated the way an import is
  - vector authoring is first-class (user 2026-09-02): an agent draws, styles, textures and keys shape layers with the same operations the Shapes tab offers (requirement:vector-editing, requirement:texture-mapping), so a vector clip can be made from nothing without touching JSON
coverage:  # every inspectTarget kind of the Model; MCP reaches all of them or it is not the editor
  machine: states, transitions and their order, guards, inputs, initial, node positions
  clips: import, blank, duplicate, rename, remove, length, raw JSON
  poses: part transforms at a key, draw order, visibility, reparent, columns (insert, borrow, swap, delete, retime, ease)
  shapes: layer and item tree (add, remove, reorder, copy, paste, rename), path geometry (vertices, handles, corner/smooth, insert, delete, open/close, pen, primitives), style (fill, stroke, dashes, gradient type and stops), keyed variants of all of these with promotion and path-topology rules
  texture: bind or import an image to a fill or stroke, tint, wrap and filter, bbox placement transform, per-vertex UV (set, move, scale, seed, clear) — data:texture-document
  collision: hitboxes, spans, body shapes, tags, material
  sockets: add, remove, rename, offset, rotate and z flags
  config: physics backend, viewer mode, MCP on/off
  preview: transport, seek, fire, values, show clip or machine
must:
  - "-mcp ADDR flag starts the server; the status bar shows the URL; the Config pane (requirement:editor-config) can stop and start it"
  - tool calls execute on the game loop — queued, drained in Tick like PumpDialogs — so the Model is never touched from two goroutines
  - every mutating tool returns the resulting focus, changed addresses, generation and problems; no follow-up read to learn what happened
  - a refusal carries the valid choices (field names, enum values, candidate targets) so the agent self-corrects in one retry
  - one tool call is one undo step where undo exists (clip edits); a raw-JSON put is one step
  - a raw-JSON put that fails to decode or leaves UnsupportedFeatures non-empty is refused and the document is unchanged (decision:practical-subset)
  - name guards, promotion, path topology and reparent correction apply exactly as from the UI — the agent goes through the same Model methods
  - a call while a dialog is open or a stage drag is in progress returns busy, never a partial edit
  - "expect_generation on every mutating tool: the call is refused with the current generation when the document changed since the agent last read it, so a human edit in between forces a re-inspect before the write (user 2026-09-02)"
  - stdio transport beside HTTP, for agent-launched editors; the window still opens (user 2026-09-02)
  - protocol revision 2026-07-28 served together with 2025-11-25 from the same server, so every client of decision:editor-mcp-server compatibility connects whichever revision it negotiates
should:
  - done: machine-graph undo (Model.UndoMachineEdit — touch snapshots the serialized machine, node drags refresh the snapshot without pushing, undo keeps the current layout), so the undo tool covers every edit tool with what=clip|machine
  - done: MCP resources lottie://focus, lottie://problems, lottie://clip/{id}.json, lottie://machine/{id}.json
wont:
  - headless or windowless server; unattended editing stays on lottierepack + lottiecheck (decision:ai-skills-workflow)
  - the editor's own exclusions and nothing more: raster pixel painting, text layer editing, cutting clips out of longer documents, theme and font editing (policy:editor-out-of-scope). Vector authoring is not among them — decision:vector-authoring-in-editor put it in scope, and this requirement keeps it there
  - a second cursor for the agent; one selection, shown on screen, is the feature
  - authentication: the editor and the agent run on the same machine as the same user; loopback bind is the whole boundary (user 2026-09-02)
  - vision:scene-editor in the first version; the same design applies later, its Model is a sibling
security:
  - loopback bind only, no token; the open document is writable by any local process that finds the port — accepted for a single-user desktop tool
  - file tool opens and saves only paths the agent names; no directory listing beyond the bundle's own sources
acceptance:
  - an agent with no prior knowledge of the editor adds a state, wires a transition, retimes a pose and recolors a shape using describe / select / inspect / set / add alone, guided by tool results
  - the author drags a part mid-session; the agent's next set is refused (generation) or lands on the author's new selection, and the reply says which
  - a render after each edit matches what the window shows
  - an agent starts a blank clip, draws a closed path with the pen, fills it with a two-stop gradient, keys a vertex at a second pose and binds a texture with vertex UV — the whole Shapes tab, through tools alone
```

```yaml
deviations:
  - an explicit target on an edit tool selects it first, so it does move the focus; the Model addresses everything through its selection and a silent side-channel would have meant a second cursor (which wont excludes). The reply's focus says where the agent ended up
  - render what=graph became what=window: the graph is a widget tree drawn with the toolkit's fonts, so the honest picture of it is the window as the author sees it, captured from the game wrapper's Draw
  - the guard selection lives in the MCP cursor, because the inspector keeps that index in its own widget rather than the Model
  - hitbox, body and socket edits now count in DocGeneration (a separate extGen beside docGen), or an agent adding a hitbox could not tell it had; the preview-stale check keeps reading docGen alone
  - remove and move take a what override (clip, span, texture, key, part) where the selection alone is ambiguous
  - file op=new with a template writes it and opens it in this window; the UI's New… spawns a second window instead
  - tests: mcp_test.go drives the tools over in-memory transports with a goroutine standing in for Tick; TestMCPLiveRender talks to a running editor (LSM_MCP_URL) for the render tool, which needs the real game loop
```

The measure of the tool design is the first acceptance item: inspect must describe a selection well enough that the tool list never has to.
