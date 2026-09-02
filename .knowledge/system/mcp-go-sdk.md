---
id: system:mcp-go-sdk
type: system
title: MCP Go SDK
---

github.com/modelcontextprotocol/go-sdk — the official Go implementation of the Model Context Protocol. Serves decision:editor-mcp-server.

```yaml
use: server side only — tools with JSON-schema params, ImageContent in CallToolResult.Content, resources, StreamableHTTPHandler and StdioTransport side by side in one process
version: v1.7.0 (2026-07-28), the version cmd/lottie-state-editor pins; serves protocol revision 2026-07-28 by default and answers the 2025-11-25 initialize handshake from the same handler; StreamableServerTransport.SupportsProtocolVersion reports the set
fit:
  - pure Go (decision:no-cgo)
  - dependency of cmd/lottie-state-editor only; the library module stays GUI- and protocol-free (decision:runtime-package-first)
watch:
  - pin v1.7.0 or later; 2026-07-28 removed sessions, so any per-agent state (none planned — the focus is document state shared with the human) must ride in tool arguments
  - the transport handler runs on its own goroutines; every Model call is marshalled onto the game loop (requirement:editor-mcp)
```
