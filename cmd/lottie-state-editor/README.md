# Lottie State Machine Editor

A desktop tool for bundling short Lottie clips into one dotLottie v2 archive
and wiring the state machine a game drives by name — `sm.Fire("jump")`
rather than tracking frame ranges.

Built with [Guigui](https://github.com/guigui-gui/guigui). It is a separate
Go module, so the `lottie-go` library itself never pulls in a GUI toolkit.

```bash
cd cmd/lottie-state-editor && go run . ../../examples/state-editor/character/character.lottie
```

The path argument is optional; **New…**, **Open…**, **Save As…**, and
**Import…** open native file dialogs through
[zenity](https://github.com/ncruces/zenity), which is pure Go and needs no
cgo. **New…** asks what to start from — an empty bundle, or one of the
embedded preset templates (chibi-male today) — then where to put it, and
opens the result in a NEW editor window, leaving the current one alone. A
template is written to the chosen path immediately; an empty bundle
cannot exist as a file (the format wants at least one animation), so that
window starts blank and its first Save writes the chosen path. Templates
are embedded at build time from `cmd/lottie-state-editor/templates/`, which
`go run ./genpresets` keeps in sync with the presets.

Every tab carries the same transport — play/pause, −1, +1 — and the row
above the tab bar holds what describes the stage rather than any one tab:
an **autoplay** toggle (on by default), an **onion skin** toggle, a zoom
readout, **−** / **+**, and **Fit**.

**rig** draws the skeleton the artwork hides: a dot at every joint and a
bone wherever a chain continues. The torso has five limbs hanging off it at
five different points, so joining each to its single hip joint would draw a
starburst that says nothing — a hub is drawn as a hub, which is to say as
its joint alone. With onion skin on, the neighbouring keys get their joints
too, in the same cool and warm — dots only, since three skeletons of lines
over one drawing is a thicket. A joint that travelled is then a line
between two dots rather than two overlapping drawings at a third of an
alpha.

Onion skin draws the keyframes either side of the playhead faintly under
the current one — the previous tinted cool, the next warm, so which way a
limb is travelling reads at a glance. Between two keys it shows the pair
bracketing the playhead, which is as useful while scrubbing as while parked
on a key; it stays out of the way while the clip is playing, where the pair
would change several times a second.

The stage fits the whole clip by default, which is right for watching a
machine run and far too small to pose a rig in — a chibi forearm is a few
pixels wide at that size. The mouse wheel zooms about the cursor, the
buttons zoom about the middle of the pane, dragging empty stage pans, and
**Fit** puts it back. Dragging the boundary between the state graph and the
preview gives the whole stage more of the window; that split is remembered
for the session. With autoplay off, every clip
that reaches the stage arrives paused on its first frame: firing an event
still transitions, so an attack can be inspected from the exact frame it
started, one +1 at a time.

The **Poses** tab marks where the clip's keyframes actually are. A Lottie
clip stores a handful of keys and interpolates between them, so a 20-frame
punch may hold only five — and those are the only frames an edit can land
on. The preset rigs keep every animated property on one set of times, so
the tab shows a single **Poses** row whose ticks are whole-body poses;
when a clip's properties disagree, it falls back to a row per animated
layer. Diamonds interpolate, squares hold. Click a tick to park the
playhead on it, drag one to retime it (it stops one frame short of its
neighbours).

Choosing the Poses tab opens the **Parts** list at the top of the right
pane: every part of the rig, front to back, with the `genpresets` joint it
belongs to. Selection runs both ways — clicking a row picks the part on the
stage, and picking one on the stage highlights (and scrolls to) its row.
The list is there because the stage cannot offer every part: a rig layers
them over each other, and switches others off by opacity, which the rows
mark as `hidden`. A hidden part is not on the stage, so a click falls
through it to whatever is drawn underneath — the list is the only way to
reach one.

The list order *is* the draw order, so dragging a row — or **▲ Front** /
**▼ Back** — changes the overlap: the gripping forearm in front of the
torso during a swing, behind it at rest. Parent links are by `ind`, not by
position, so the rig survives being rearranged; clips using track mattes
refuse the whole operation, because there the layer before a matte is its
source. **Hide** / **Show** switches the selected part's opacity at the
current key, which is how a slot's alternate drawings take turns.

The Poses tab's own footer changes which poses the clip has. **+Pose**
inserts one at the playhead as a copy of the pose before it — a new pose
starts as the one it follows and is then changed — and **Delete** removes
the selected column. **|◀** and **▶|** step key to key, since the frames
between poses hold nothing to edit, and **length** sets the clip's own out
point (it refuses to cut past the last pose; delete that pose instead). The
second row borrows a pose from another clip: a preset's clips share one
rig, so a rest stance or a guard is worth copying rather than dialling in
again.

**swap near/far** trades the paired limbs as a pose is inserted — the near
arm takes the far arm's angles and back — because half a walk cycle is the
other half with the legs the other way round. **Swap** does the same to a
pose that is already there. Only properties keyed on both sides trade: a
limb's attach point is rig spec that puts it on its own side of the torso,
and trading those would detach the pair rather than swap it. Which limb
draws in front is a separate edit, in the Parts list.

The Pose form's **parent** row changes what a part hangs from — a sword
passing from one hand to the other — and rewrites its transform so the part
does not jump. A value keeps the form it had: a static attach point is
corrected once and stays static, because static means "rigidly here on my
parent" and that is still true of the new one; a keyed rotation is
corrected at every key, so the limb keeps its poses. Position can only
match at the frame the link was changed — two parents that move differently
cannot both be followed by one point, which is the reason for re-parenting
rather than a shortcoming of it. The candidate list leaves out the part's
own descendants, so a cycle is not something to warn about: it cannot be
chosen.

**joint drag** picks what the joint mark does. `moves part` is the plain
reading — the part follows its attach point, which is also how the
character itself is moved, since the body's joint is its position in the
composition. `keeps art` moves the point the part turns about and leaves
the drawing exactly where it is, which is how a limb rotating about the
wrong pixel gets fixed.

**Tab** moves between the fields of a form and **Shift-Tab** back, stepping
over any that are disabled. A text input commits when it loses focus, so
tabbing out of a field is the same as pressing Enter in it.

The **part** row is the layer's name and is editable. Names are how a
socket binds a layer, how a pose is copied between clips, and how the
near/far pairs are found, so a rename is a real edit — blanks and
duplicates are refused, and a socket still bound to the old name is called
out in the status bar. A part whose name (or whose parent's name) is blank
or duplicated cannot be dragged on the stage at all: the core resolves a
layer by name and takes the first match, so a drag would write plausible
numbers into the wrong space. The pane says which, and the numeric fields
keep working.

With a key selected, click a part on the stage or in the list, then drag
inside its outline to swing it about its joint, or drag the joint mark to
move it. In the Pose form the **joint** row is a picker as well as a
readout, naming the parts the way the rig does, and **ease** switches the
whole pose between the two curves `genpresets` writes. The Pose pane on the right shows the numbers stored at that key —
rotation in degrees, and the `genpresets` joint the part belongs to
(`upper-arm-near` → `arms(near)`), so a pose found by dragging can be
transcribed back into the generator. **Undo pose edit** takes back the
last change, counting a whole drag as one step. Editing is only possible
while the playhead sits on a key; scrubbing away ends it rather than
writing a value at a frame the other tracks have no key at.

The **Shapes** tab edits vector artwork: the shape layers of the clip on
stage — imported UI assets, the generated samples, or layers drawn from
scratch. Choosing the tab opens the **Shapes** section at the top of the
right pane, the way Poses opens the Parts list: a layer picker (with
**+Layer** to start an empty one and **−Layer** to remove it) over the
layer's item tree, indented the way the document nests it — groups,
paths, primitives, fills, strokes, gradients, and modifiers, in paint
order. Clicking a shape on the stage selects it in the tree and back,
switching the layer with it when the click landed on another layer's
artwork. Unknown item kinds are listed but left inert — they survive
saving untouched. The strip under the stage keeps only the key chart and
the tool row, so the stage keeps its height.

The tool row picks the gesture. **Select** drags what is already there:
the selected geometry shows its box, and dragging inside moves the whole
shape while a box corner resizes it about the opposite corner — the press
that picks a shape starts carrying it, so select-and-move is one gesture.
A path adds the finer controls: square vertices drag individually (the
selected one carries its bezier handle pins — drag them to bend the
curve, **Smooth / Corner** toggles the tangents), and **Pen** on the
outline of the selected path splits the segment under the click.
Elsewhere **Pen** draws a new path click by click — clicking the first
vertex again or **right-clicking** closes it (the last point joins the
first), **Finish** commits it open — and **Rect** / **Ellipse** /
**Star** drop a primitive where clicked.

Geometry also inserts from the tree, without touching the stage tools:
**+Path**, **+Rect**, **+Ellipse** and **+Star** drop into the selected
item's group at its origin, ready to drag into place. New geometry from
the stage tools arrives in its own group with a grey fill, selected and
ready to restyle. **+Fill**, **+Stroke**, **+Grad**, **+Trim**,
**+Round** and **+Group** add items to the selected item's group the same
way; **▲ Front / ▼ Back** move an item within its group (the tree is the
paint order), **Delete** removes it, subtree included.

**Copy** / **Paste** / **Duplicate** multiply what is already there.
Copy takes the selected item, subtree and keyframes included, into the
editor's own clipboard; Paste drops a fresh copy into the current group —
of any layer, or any clip, since the clipboard lives as long as the
editor — and Duplicate is the one-click twin, landing right on top of its
source in the same group, selected and ready to drag aside. The grips
follow the park rule everywhere: an animated shape shows its corner
markers only while the playhead sits on one of its keys, so a drag is
never offered where the write would be refused.

Gradients edit the Flash way. On the stage the selected gradient shows
its transform gizmo — drag the square (start / center), the circle (end:
rotation and length in one handle) or the diamond (the whole gradient).
In the pane, the **ramp** is the color bar: click under it to add a stop
carrying the color the ramp already shows there, drag a stop to move it,
drag it well off the bar to delete it, and recolor the selected stop in
the hex field beside its swatch. **type** switches linear and radial.

Editing follows the pose rules exactly. A static value is writable
anywhere and applies to the whole clip; an animated one is written only
with the playhead parked on one of its keys — click a tick on the chart —
and on a preset-style pose clip, touching a static value keys it at every
pose first. Topology is stricter still: Lottie interpolates a path
vertex-wise, so inserting or deleting a vertex rewrites **every** key of
that path at once, keeping them in step. Shape keys ride the pose
columns: retiming, +Pose, Delete and ease all carry them along.
**Undo shape edit** shares the clip-edit stack with the pose tab.

`-viewer` starts **viewer mode**: the editor watches every file the
document was loaded from — the `.lottie` bundle (whose images travel
inside it), or a loose clip `.json` plus anything imported after — and
reloads automatically when any of them changes on disk. A change only
triggers once the file stats hold still for a poll interval, so a tool
rewriting several files (a `lottierepack`, an agent editing clips) is
picked up when it finishes, not mid-write. The disk is the source of
truth in this mode, so the document is read-only: saving, machine
new/delete, input and state editing, hitbox / body / socket authoring
and graph-node dragging are all refused (the model itself rejects
mutations, so stage drags are covered too, not just buttons). Driving
the preview — Try buttons, value inputs, tabs, clip selection — stays
live, since that reads the document rather than changing it. Viewer
mode can also be toggled at runtime from the Config pane, which stays
clickable for exactly that reason. It is the intended way to watch AI
edits land live:

```bash
cd cmd/lottie-state-editor && go run . -viewer ../../examples/state-editor/presets/chibi-male/chibi-male.lottie
```

The editor is its own module and depends on a released `lottie-go`, so it
installs like any other command:

```bash
go run github.com/shibukawa/lottie-go/cmd/lottie-state-editor@latest
```

The repository root carries a `go.work` that points the editor at the
library in this checkout, so working on both at once needs no `replace`
directive. Set `GOWORK=off` to build the editor against the released
library instead — worth doing before a release, to check the library
actually exports everything the editor uses.

Right now that build fails: the Markers tab needs `Player.OnMarker` and
`StateMachinePlayer.OnMarker`, which are newer than v0.5.2. The editor
builds from this checkout until the next library release.

## Samples

`examples/state-editor/` holds three generated bundles — a platformer character,
a marker-segmented spritesheet, and a combo whose clips chain end to end.
See its README for what each one demonstrates. Regenerate them with:

```bash
cd cmd/lottie-state-editor && go run ./gensamples
```

They are generated rather than downloaded so there is no third-party
licensing to track: every clip is authored in this repository.

Their names carry what kind of thing they are — `-anim` for clips, `-state`
for states, `-seg` for markers, nothing for inputs. dotLottie keeps those in
four separate namespaces but does nothing to tell them apart, so a machine
where the clip, the state, the marker and the event are all called `jump`
reads fine on screen and is unreadable as data. Inputs stay bare because
they are what a game passes to `Fire`.

## Layout

- **Clips** — one row per playable unit, which is a file narrowed to one of
  its markers. A document carrying three markers is three clips here, so the
  same file name repeats down the source column. Selecting one plays it on
  its own, so it can be judged before being wired into any state.
- **Machines** — beside the clips. A bundle may hold several state machines,
  each its own file under `s/`, sharing the bundle's clips but not each
  other's states. Create, rename and delete them here; selecting one opens
  it in the graph. `▶` marks the one the manifest names as default — what
  `NewStateMachinePlayer("")` loads — and *Set initial* names it; naming the
  one already marked clears the choice, putting "the first listed" back. Nothing in the format switches between machines at
  runtime, so they are alternative entry points a game picks between: it
  makes a `StateMachinePlayer` per machine and drives whichever it wants.
- **Interface** — the machine's external surface, split by direction:
  - *Events* come in from the game. Each carries a *Try* button. `Restart`
    leads the tab as a pseudo-event: triggerable like the rest, but not
    renamable or removable, since the document does not declare it.
  - *Values* come in too, each with a checkbox or a field.
  - *Markers* go the other way: the animation passes them and the game
    reacts. They are not declared by the machine and cannot be driven from
    here, so the tab reports where each one lives and how often it has
    fired this run.

  Selecting an event or value traces the transitions that read it, in
  orange, on the graph.
- **State graph** — states as nodes, transitions as arrows, outlined as its
  own working area. Click to select, drag to move. `▶` marks the initial
  state, and the state the preview is currently in is filled green, so the
  machine can be watched running.
- **Inspector** — the selected state's playback fields, its transitions in
  the order that decides which one wins, and the guards on the selected
  transition. Validation problems are listed at the bottom.
- **Preview** — the stage, outlined to match the graph. It runs the machine
  through the same interpreter a game uses, or plays a single clip.
  Everything that drives it lives in the Inputs table.
- **Timeline** — under the stage: the whole document as a track, the range
  actually playing as a lighter band, every marker as a labelled tick, and
  the playhead. A state that names a segment shows as a band covering only
  part of the document, which is how you tell the segment was cut where you
  meant. Drag to scrub. The selected hitbox's active spans draw on the same
  ruler, so when it comes out and disappears reads against the playhead.
- **Collision** — the strip under the timeline, editing the two physics
  extensions (see the library README). The hitbox row works on the clip on
  stage: `+Rect` / `+Circle` add a frame-stepped box, the name and tag
  fields describe it, and tags color it by meaning — `hit` red, `hurt`
  green, `push` amber, untagged gray. The span row times the current box:
  from/to fields edit the interval under the playhead, `+Span` starts a new
  pose at the playhead copying the previous one. The body row places the
  bundle-wide rigid silhouette (violet). All shapes are dragged on the
  stage itself: inside moves, the white grip resizes, empty stage
  deselects. The checkbox hides the whole overlay.

## Notes

A native dialog blocks for as long as it is on screen, so dialogs run on
their own goroutine and their result is applied from `Tick`. The window
keeps rendering while one is open, and every `Model` field stays owned by
the main goroutine. Only one dialog runs at a time; the buttons grey out
while one is up.

Transitions are created and edited in the inspector; the graph draws them.
Drag-to-connect is not implemented.

Node positions are stored in each state's extra fields under
`x-lottie-go-editor`. The dotLottie schema has nowhere to record graph
layout, and `lottie-go` writes members it does not model back unchanged, so
positions survive a save without making the file invalid for other runtimes.

Setting `LSM_EDITOR_SCREENSHOT` to a path renders a frame to a PNG and
exits, which is how the UI is checked without a display:

```bash
LSM_EDITOR_SCREENSHOT=/tmp/shot.png LSM_EDITOR_SCREENSHOT_TICKS=45 \
  go run . path/to/character.lottie
```
