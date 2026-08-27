---
id: decision:ai-skills-workflow
type: decision
title: AI Customization via Agent Skills
---

Deliver AI-assisted character animation as agent skills over requirement:animation-presets, not as AI embedded in the editor and not (initially) as MCP. Users already pay for coding agents (Claude Code etc.); the agent edits Lottie JSON / .lottie bundles directly and the editor is the viewer.

```yaml
chosen: skills + presets + validation CLI
rejected:
  embedded-ai: API key/billing/chat UI burden in a Go desktop app; double-pays vs user's agent subscription
  mcp: live-editing benefit reproducible by file watch; adds protocol maintenance; editor must be running
components:
  - preset .lottie templates with rig naming convention (requirement:animation-presets)
  - skill doc: supported-subset rules, rig convention, part-image spec, edit recipes
  - validation CLI: load -> UnsupportedFeatures() empty -> render frame PNGs for agent self-check
  - editor auto-reload on file change (substitutes MCP live view)
tasks:
  - motion: edit keyframes/timing, author new clips from preset reference
  - design: regenerate WebP part images to slot spec (image-gen capability supplied by user's stack)
rules:
  - no expressions: bake to keyframes (ecosystem norm: native renderers skip them; lottie-web needs CSP unsafe-eval)
  - no effects: emulate via duplicated shapes / opacity
  - blend modes normal/multiply/screen only; masks add/subtract only (decision:practical-subset)
mcp: revisit only on demonstrated demand
```

Self-verification loop is the critical piece: agent edits JSON, runs CLI, inspects rendered PNG, corrects. `UnsupportedFeatures()` turns subset violations into machine-checkable errors.
