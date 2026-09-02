---
id: decision:editor-mcp-server
type: decision
title: Editor Exposes Its Model Over MCP
---

Implemented 2026-09-02 (cmd/lottie-state-editor -mcp flag). Reverses the MCP rejection in decision:ai-skills-workflow (2026-09-02): cmd/lottie-state-editor serves its Model as an MCP server, so a coding agent edits the open document while the author watches the same window. Requirement: requirement:editor-mcp. Tool design: api:editor-mcp-tools.

```yaml
why_now:
  - the editor grew surfaces with no file-level equivalent — pose columns, shape trees, gradient ramps, sockets, hitbox spans (requirement:pose-editing, requirement:vector-editing, requirement:collision-authoring). Editing them as raw JSON through lottierepack means re-deriving every invariant the Model already enforces: promotion, path topology, name guards, reparent correction
  - viewer mode shows a finished write; it cannot show a selection, a parked key, or the form being filled in. Pair-editing needs the agent's cursor on screen
  - the Model is widget-free by design ("the same operations back the menus, the graph, and the inspector"); MCP is a third front end over it, not a new editing path
  - render-as-tool-result closes the self-check loop in-session: the agent sees the stage the human sees instead of re-running lottiecheck on a re-packed file
chosen:
  server: in-process in the editor binary, enabled by a flag; off by default
  transport: streamable HTTP on 127.0.0.1, port from the flag (0 = OS-assigned, shown in the status bar), plus stdio when the flag says so. The human starts the editor and the agent connects; the editor outlives agent sessions
  auth: none; loopback is the boundary (user 2026-09-02)
  protocol: target revision 2026-07-28 (stateless, no session id, server/discover, per-request _meta version) while also answering the 2025-11-25 handshake from the same endpoint; system:mcp-go-sdk v1.7.0 does both
  sdk: system:mcp-go-sdk (pure Go — decision:no-cgo)
  setup: an init subcommand (cmd/lottie-state-editor/mcpinit.go) writes the client half — .mcp.json for Claude Code, .vscode/mcp.json for VS Code, the codex mcp add line for Codex, whose servers live in ~/.codex/config.toml — on one fixed port (7391) and prints the matching launch command; -transport stdio records a launcher on the bundle instead (user 2026-09-02)
  surface: few tools over one shared selection cursor (concept:editor-focus) plus a raw-JSON escape hatch; not one tool per Model method
rejected:
  stdio_only: the agent would own the window's lifecycle, and a second agent session could not reach an open document. Kept as the second transport for agent-launched runs, not the only one
  headless_server: removes the human from the loop, which is the point; lottierepack + lottiecheck already cover unattended editing (decision:ai-skills-workflow)
  tool_per_method: ~300 Model methods; a tool listing is paid on every agent turn, and the agent would still need the inspector's field list to call them
  one_generic_tool: a single {op, args} tool hides every parameter behind free text; validation and hints degrade to prose
compatibility:  # client state as of 2026-09-02; the dual-revision server makes the answer "works" either way, this records which revision each client will pick
  claude_code: v2.1.232+ runs on TypeScript SDK 2.0; HTTP servers are asked for 2026-07-28 automatically, stdio servers only with MCP_PROTOCOL_NEGOTIATION=auto, otherwise the 2025-11-25 handshake — so HTTP is the path that gets the new revision by default
  codex_cli: v0.147.0 (2026-08) ships MCP SDK 3.0.0 with 2026-07-28; per-server protocol_version setting; falls back to the previous revision for older servers
  vs_code_copilot: stdio and http, tools / resources / prompts / MCP Apps; its 2026-07-28 status was not confirmed in release notes through v1.127 — it reaches the server on the older handshake until it is
  spec_features_not_used: MRTR, tasks, apps, subscriptions/listen; the editor is request/response with image results, which every revision carries
kept: skills/lottie-character-preset and the lottierepack/lottiecheck loop stay for batch, CI and no-editor contexts; MCP is the interactive path
```

The reversal condition in decision:ai-skills-workflow was "demonstrated demand". The demand is the editor's own growth: its invariants now live in the Model, and a file-level agent cannot reuse them.
