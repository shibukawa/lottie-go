---
id: requirement:editor-config
type: requirement
title: Bundle Editor Config
---

Status: implemented — Config toolbar button opens the pane (inspectConfig target); physics select (both/cp/resolv/none, empty reads both) gates the collision strip groups, the chart rows, the stage overlay, and stage hit-testing; stored as {physics} in manifest Extra x-lottie-go-editor and survives save/reopen; unknown stored values read as both. A per-bundle configuration pane: a Config button beside
Save As opens config editing in the selection pane
(requirement:selection-driven-ui — config is just another selectable).

```yaml
settings:
  physics_backend:
    values: [cp, resolv, both, none]
    meaning: which collision authoring UI the editor shows and validates; NOT runtime behavior — a game chooses by importing plugins (decision:collision-static-plugins)
    effect: none hides collision tooling; cp hides hitbox tracks; resolv hides body shapes
  future: default facing axis (rule:facing-mirror), chart preferences
storage: manifest Extra under the existing x-lottie-go-editor member — travels with the bundle, ignored by other runtimes, same channel as state node positions
ui: {kind: button, label: Config, place: toolbar beside Save As, action: selection pane shows the config form}
```

Gates which parts of requirement:collision-timeline appear.
