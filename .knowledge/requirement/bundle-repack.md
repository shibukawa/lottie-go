---
id: requirement:bundle-repack
type: requirement
title: Bundle Dump and Repack Tool
---

cmd/lottierepack explodes a .lottie bundle (data:bundle-layout) into a directory of loose files and rebuilds it, so agents in decision:ai-skills-workflow edit clips, images, state machines, and plugin payloads with plain file tools and verify with cmd/lottiecheck.

```yaml
commands:
  dump: "lottierepack -dump -dir work file.lottie"
  repack: "lottierepack -dir work [-base file.lottie] [-out file.lottie]"
layout:
  "<id>.json": one animation per file, pretty-printed
  "parts/<name>": shared images (archive i/ or images/)
  "machines/<id>.json": state machines
  "extensions/<path>": extensions/ members with subtree kept (api:bundle-extension-files); JSON pretty-printed, other bytes verbatim
  ".source": dump origin; implicit -base for a bare repack
repack:
  base: -base > .source > existing -out; manifest, fonts, themes, stray files carry over from it
  clips: directory is authoritative; a clip in base but not in dir is removed
  extensions:
    sync_when: work/extensions/ exists
    rule: directory is authoritative like clips; missing file removes the base member
    absent_dir: base members pass through untouched (dumps from older tool versions, hand-built dirs)
  validate: Bundle.Validate must pass before writing
safety:
  - bundle-supplied ids and member paths are checked per component: no empty, ".", "..", "/" or "\\"
  - extension names are cleaned and must stay under extensions/ (SetExtensionFile enforces)
acceptance:
  - dump -> repack of a bundle with plugin payloads (plugin dirs of decision:collision-static-plugins, sockets.json at the root) yields the same extension members
  - editing or deleting a file under work/extensions/ changes or removes that member
  - repack of a dir without extensions/ keeps the base's extension members
```

Docs: README, skills/lottie-character-preset/SKILL.md, and the package comment describe the layout identically.
