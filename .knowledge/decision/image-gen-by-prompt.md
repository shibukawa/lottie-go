---
id: decision:image-gen-by-prompt
type: decision
title: Image Models Reached by Prompt Files, Not by the Tool
---

For requirement:ai-character-forge the repository ships prompts and a grid
template, never an API client: the user pastes the prompt into Gemini or
Grok (or the agent calls an image tool its own stack has) and drops the
PNG into work/sheets/. Same reasoning as decision:ai-skills-workflow —
keys, billing and a changing vendor surface do not belong in a Go asset
tool — plus one more: the part split is a prompt-engineering problem whose
answer changes with every model release, and prompt files are the
artifact a user tunes without a rebuild. Decided 2026-09-03.

```yaml
chosen: prompt templates in skills/lottie-character-forge/references/prompts.md, filled by lottieforge grid; a template image generated to spec; a cut report that names the cell to redo
rejected:
  api_client: per-vendor auth and image APIs that change monthly; the same prompt through the vendor's app costs nothing extra
  mcp_image_server: worth having, but not here — any image MCP the user already has fits the contract (prompt + reference image in, PNG out), and the skill says where the result goes
  in_editor_generation: decision:ai-skills-workflow rejected embedded AI once; nothing changed
model_notes:
  gemini: takes a reference image plus a template image with the text and tends to keep the grid; the primary target for part sheets
  grok: strong on the model sheet; grid adherence varies, so the text-layout prompt and cut's connected-component fallback cover it
  any: the prompts ask for a flat key color, no shadow, no text in cells, neutral hang and joint caps; models drift on all four, which is why cut measures instead of trusting
consequences:
  - a human (or an image tool) sits between grid and cut; the skill states exactly which file to produce so the step is a copy, not a judgment
  - prompt files are documentation: a model needing other wording gets a section, not a code change
```
