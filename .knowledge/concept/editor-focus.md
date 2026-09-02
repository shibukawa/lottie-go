---
id: concept:editor-focus
type: concept
title: Editor Focus Cursor
---

The editor's single selection (requirement:selection-driven-ui) exposed as a named cursor: what is on stage, where the playhead is parked, and which thing the inspector edits. Humans move it by clicking; agents move it by address through api:editor-mcp-tools select. Serves requirement:editor-mcp.

```yaml
focus:
  stage: clip:<id> or machine:<id> — what PreviewDraw draws
  frame: playhead; parked on a key while a key is selected (requirement:pose-editing park)
  selection: one address from the grammar below, or none
  tab: the strip tab owning the selection (poses | shapes | hitboxes | body | sockets); derived, not set
address:  # <kind>:<segments>; segments are ids and names, never list indices where a name exists
  clip: clip:<id>
  machine: machine:<id>
  state: state:<machine>/<name>
  transition: transition:<machine>/<state>/<n>; n is the order, because order is semantic (data:state-machine)
  guard: guard:<machine>/<state>/<n>/<g>
  input: input:<machine>/<name>
  part: part:<clip>/<layer-name>; image layer, name guards apply (requirement:pose-editing)
  key: key:<clip>/<frame> for a pose column, key:<clip>/<layer-name>/<frame> for one layer's key (requirement:keyframe-timeline)
  layer: layer:<clip>/<layer-name>; a shape layer as a whole — the tree root the Shapes tab picks
  shape: shape:<clip>/<layer-name>/<i>/<j>/...; item path in the gr tree (data:clip-edit-document shapes)
  vertex: vertex:<shape address>/<v>
  stop: stop:<shape address>/<n>; gradient ramp stop
  uv: uv:<shape address>/<v>; the texture UV of vertex v (requirement:texture-mapping)
  hitbox: hitbox:<clip>/<name>; span: span:<clip>/<name>/<n>
  body: body:<n>; socket: socket:<name>; config: config
resolution:
  relative: an address without its clip or machine segment (part:forearm-near, key:12) resolves against the current stage; the reply always echoes the absolute form
  side_effects: selecting a key parks the playhead and switches the stage to that clip; selecting a part or shape opens its tab — the side effects a click has, so the human sees the agent's move
  ambiguity: a name matching several layers is refused with the candidates, as the stage refuses to drag such a part
form:
  what: inspect returns the selection as fields {name, type, value, options, keyed, writable, problem} — the inspector pane as data
  why: the tool list stays small because the field list is delivered per selection at runtime, not per tool at listing time; a new inspector field is a new form entry, not a new tool
  keyed: a keyed field reads the value at the parked key and writes there (promotion on first keying); off-key it reports the interpolated value and refuses writes, as the UI does
shared:
  one_cursor: agent and human move the same focus; every reply carries the focus after the call, so the agent notices when the human moved it
  drift: an edit tool with an explicit target selects it first (implemented that way — the Model only addresses its selection), so the focus follows the agent; one without a target edits the focus and names what that was. Either way the reply carries the focus after the call
```

Implemented in cmd/lottie-state-editor/mcpaddr.go. Adds almost no state to the Model: it names what is already there (inspectTarget plus the per-kind selected index) so an address parses into the existing Select* calls and formats back from them.
