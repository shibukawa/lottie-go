---
id: requirement:selection-driven-ui
type: requirement
title: Selection-Driven Editor UI
---

Status: implemented — inspector switches on Model.InspectTarget (state | machine | hitbox/window | body shape | socket), machine rename and the initial toggle moved into the machine pane, list footers slimmed to add/delete with delete gated on selection, interface add buttons scoped per tab; body-shape material (friction/elasticity/sensor) and socket rename became editable for the first time via their panes. The Config pane (requirement:editor-config) and the collision strip replacement (requirement:collision-timeline) remain separate work. Restructure ui:editor-shell so available actions follow
the current selection instead of every action being visible at once.

```yaml
principle: one selection at a time drives one context pane; lists carry only add/remove
inspector:
  becomes: parameter pane for whatever is selected (replaces the fixed state-edit panel)
  by_selection:
    state: state form (name, type, animation, segment, mode, speed, loop, autoplay) + transitions/guards
    machine: machine params; "Set initial" — or "Unset initial" when it already is
    collision_item: geometry/tags/span params (requirement:collision-timeline right-pane editing)
lists:
  footer_buttons: add and delete only (import counts as add)
  delete_enabled: only while an item in that list is selected
interface_tabs:
  now: +Event / +Bool / +Num always visible under the tabs
  target: +Event only on the Events tab; +Bool and +Num only on the Values tab
motivation: current UI shows many always-on buttons (three per list, three add-input, nine collision); actions detached from selection read as clutter and mis-afford
```

Serves requirement:editor-mvp usability; collision rows move out entirely
per requirement:collision-timeline.
