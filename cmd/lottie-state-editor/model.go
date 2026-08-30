package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	lottie "github.com/shibukawa/lottie-go"
	lottiecp "github.com/shibukawa/lottie-go/plugin/physics/cp"
	lottieresolv "github.com/shibukawa/lottie-go/plugin/physics/resolv"
	lottiesockets "github.com/shibukawa/lottie-go/plugin/sockets"
)

// inspectTarget names what kind of thing the inspector pane edits. The
// selection itself lives in the fields that already track it (selectedState,
// selBox, ...); this only says which of them is current.
type inspectTarget int

const (
	inspectState inspectTarget = iota
	inspectMachine
	inspectHitbox
	inspectCPShape
	inspectSocket
	inspectPose
	inspectShape
	inspectConfig
)

// editorExtraKey names the member this editor stashes in a State's extra
// fields. The dotLottie schema has nowhere to record graph layout, so node
// positions ride along there: other runtimes ignore the member and
// lottie-go writes it back untouched.
const editorExtraKey = "x-lottie-go-editor"

type nodeMeta struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Model is the editor's document and selection. It knows nothing about
// widgets, so the same operations back the menus, the graph, and the
// inspector.
type Model struct {
	bundle    *lottie.Bundle
	path      string
	machineID string
	machine   *lottie.StateMachine

	// sources is every disk file the document was built from — the opened
	// bundle or clip plus later imports — in load order, so viewer mode
	// can watch them and Reload can replay them.
	sources []string
	// viewer marks watch-and-reload mode: the disk is the source of truth,
	// saving is disabled, and edits only last until the next reload.
	viewer bool

	// autoPlay starts clips playing the moment they appear on stage — a
	// selected clip, the machine's initial state, every transition. Off,
	// they arrive paused on their first frame so an animation can be
	// inspected from the instant an event fired, one +1 at a time.
	autoPlay bool

	selectedState string
	selectedTrans int

	preview    *lottie.StateMachinePlayer
	previewGen int
	previewErr error

	// The preview shows either the machine or one clip on its own.
	previewClip    clipRef
	clipPlayer     *lottie.Player
	previewClipSet bool

	// How often each marker has been passed since the stage last changed.
	// Markers are the machine's outgoing side, so the count is what shows
	// they are actually firing. Keyed by file as well as name: two clips
	// may each carry a marker called "hit-seg" and they are not the same
	// signal.
	markerHits map[markerKey]int
	// markerGen counts every emission, so a state key can notice one
	// without walking the bundle.
	markerGen int

	// Which input is selected, so the graph can highlight the transitions
	// that depend on it.
	selectedInput int

	// What the inspector edits: the most recently selected thing. Every
	// Select*/Add* records itself here, so the right pane always shows the
	// parameters of what was last touched.
	inspect inspectTarget

	// Collision editing (see hitbox.go). The active tab is also what
	// decides which overlay group shows on the stage. The caches hold
	// parsed plugin documents; the overlay reads them every frame.
	colTab        colTab
	selBox        int
	selCPShape    int
	selSocket     int
	trackCache    map[string]*lottieresolv.Track
	cpBody        *lottiecp.Body
	cpLoaded      bool
	socketSet     *lottiesockets.Set
	socketsLoaded bool

	// Pose editing (see pose.go). clipDocs holds the clips parsed for
	// editing; the selection is one keyframe, plus the part whose numbers
	// the inspector shows. poseLayer is -1 while the selected tick is a
	// whole-body pose rather than one layer's key.
	clipDocs  map[string]*clipDoc
	poseFrame float64
	poseSet   bool
	poseLayer int
	posePart  int

	// Shape editing (see shape.go). The selection is a layer plus the index
	// path of one item in its tree; the tool is the Shapes tab's active
	// gesture, and the pen accumulates points until it commits.
	selShapeLayer int
	selShapePath  []int
	selShapeVert  int
	selGradStop   int
	shapeTool     shapeTool
	penPts        [][2]float64
	penActive     bool
	// shapeClipboard holds a deep copy of one copied item, so it can be
	// pasted into any group of any clip for as long as the editor runs.
	shapeClipboard map[string]any

	// Clip edits are undoable on their own stack; a drag writes on every
	// mouse move, so it collapses into one step between Begin and End.
	clipUndo       []clipSnapshot
	poseDragOpen   bool
	poseDragPushed bool

	// How the stage is being looked at (see stageview.go): the zoom over
	// fit-to-pane, the pan in screen pixels, and how much of the window the
	// preview takes from the graph.
	stageZoom            float64
	stagePanX, stagePanY float64
	previewH             int
	onionSkin            bool
	jointKeepsArt        bool
	showRig              bool

	// generation counts every change the UI must redraw for, including
	// selection; widgets hash it in WriteStateKey instead of the whole
	// document. docGen counts document edits only, so merely selecting
	// something never reports the running preview as out of date.
	generation int
	docGen     int
	status     string

	problemsCache []string
	problemsGen   int

	// Native file dialogs run on their own goroutine; see dialog.go.
	dialog     chan dialogResult
	dialogOpen bool
}

func NewModel() *Model {
	m := &Model{
		bundle:        lottie.NewBundle(),
		selectedTrans: -1,
		selectedInput: -1,
		selBox:        -1,
		selCPShape:    -1,
		selSocket:     -1,
		poseLayer:     -1,
		posePart:      -1,
		selShapeLayer: -1,
		selShapeVert:  -1,
		selGradStop:   -1,
		trackCache:    map[string]*lottieresolv.Track{},
		clipDocs:      map[string]*clipDoc{},
		dialog:        make(chan dialogResult, 1),
	}
	m.autoPlay = true
	m.status = "New bundle. Import a clip to begin."
	return m
}

func (m *Model) Generation() int               { return m.generation }
func (m *Model) Status() string                { return m.status }
func (m *Model) Path() string                  { return m.path }
func (m *Model) MachineID() string             { return m.machineID }
func (m *Model) Machine() *lottie.StateMachine { return m.machine }
func (m *Model) Bundle() *lottie.Bundle        { return m.bundle }

func (m *Model) setStatus(format string, args ...any) {
	m.status = fmt.Sprintf(format, args...)
}

// Touch records an edit made directly against the document, which the
// inspector does when it writes into a State or Guard in place.
func (m *Model) Touch() {
	if m.blockEdit() {
		return
	}
	m.touch()
}

// Redraw requests a repaint without recording a document edit — for
// widget-local state (a checkbox, a picker) that changes what the panes
// show but not the document itself.
func (m *Model) Redraw() { m.generation++ }

// blockEdit refuses document mutation in viewer mode, where the disk owns
// the document and any edit would be silently thrown away by the next
// auto-reload. Guarding here, in the model, covers every path at once —
// buttons, inspector fields, stage drags, graph node moves.
func (m *Model) blockEdit() bool {
	if !m.viewer {
		return false
	}
	m.setStatus("viewer mode: read-only")
	m.generation++
	return true
}

// syncMachine writes the machine back into the bundle so a save, a
// validation, or a preview restart sees the current edits. It does not mark
// anything changed, so read-only callers can use it freely.
func (m *Model) syncMachine() {
	if m.machine != nil && m.machineID != "" {
		if err := m.bundle.SetStateMachine(m.machineID, m.machine); err != nil {
			m.setStatus("cannot serialize machine: %v", err)
		}
	}
}

// touch records a document edit: it syncs and bumps both counters.
func (m *Model) touch() {
	m.syncMachine()
	m.docGen++
	m.generation++
}

// CreateBundle materializes the New… choice at path and opens it in a
// separate editor window, leaving this one untouched. A template is
// written out first; an empty bundle cannot exist as a file (the format
// requires at least one animation), so the new window starts blank and
// remembers the path for its first save.
func (m *Model) CreateBundle(template, path string) {
	if template == newEmptyChoice {
		if err := spawnEditor("-new", path); err != nil {
			m.setStatus("cannot open new window: %v", err)
		} else {
			m.setStatus("new empty bundle in a new window; it will save to %s", filepath.Base(path))
		}
		m.generation++
		return
	}
	data, err := templateBytes(template)
	if err != nil {
		m.setStatus("template %q: %v", template, err)
		m.generation++
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		m.setStatus("cannot write %s: %v", path, err)
		m.generation++
		return
	}
	if err := spawnEditor(path); err != nil {
		m.setStatus("wrote %s but cannot open new window: %v", path, err)
	} else {
		m.setStatus("created %s from %s in a new window", filepath.Base(path), template)
	}
	m.generation++
}

// StartNewAt begins an empty document whose first save goes to path,
// which is how a window spawned by New… > Empty bundle starts.
func (m *Model) StartNewAt(path string) {
	m.path = path
	m.sources = nil
	m.setStatus("new bundle; Save writes %s", path)
	m.generation++
}

// spawnEditor launches another instance of this editor.
func spawnEditor(args ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, args...)
	return cmd.Start()
}

// Open loads a .lottie bundle, or a bare .json clip into a fresh bundle.
func (m *Model) Open(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		m.setStatus("no path given")
		m.generation++
		return
	}
	// The clip preview holds a player decoded from the old bundle; drop
	// it, and re-show the same clip from the new bundle when it survives
	// the reload — how viewer mode follows edits to a previewed clip.
	prevClip := m.previewClip
	m.previewClip, m.clipPlayer = clipRef{}, nil
	if strings.EqualFold(filepath.Ext(path), ".json") {
		m.bundle = lottie.NewBundle()
		m.path = ""
		m.machineID, m.machine = "", nil
		m.sources = nil
		m.resetCollisionSelection()
		m.resetCollisionCache()
		m.resetClipDocCache()
		m.ImportClip(path)
		m.reshowClip(prevClip)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		m.setStatus("open failed: %v", err)
		m.generation++
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		m.setStatus("stat failed: %v", err)
		m.generation++
		return
	}
	b, err := lottie.DecodeBundle(f, st.Size())
	if err != nil {
		m.setStatus("load failed: %v", err)
		m.generation++
		return
	}
	m.bundle = b
	m.path = path
	m.sources = []string{path}
	m.selectedState, m.selectedTrans = "", -1
	m.machineID, m.machine = "", nil
	m.resetCollisionSelection()
	m.resetCollisionCache()
	m.resetClipDocCache()
	m.generation++
	if ids := b.StateMachineIDs(); len(ids) > 0 {
		m.SelectMachine(ids[0])
	}
	m.reshowClip(prevClip)
	m.setStatus("loaded %s (%d clips, %d machines)", path,
		len(b.AnimationIDs()), len(b.StateMachineIDs()))
}

// reshowClip restores a clip preview across Open when the new bundle still
// carries the clip; otherwise the preview stays on the machine.
func (m *Model) reshowClip(c clipRef) {
	if c.Anim == "" || !slices.Contains(m.bundle.AnimationIDs(), c.Anim) {
		return
	}
	m.ShowClip(c)
}

// Save writes the bundle as dotLottie v2.
func (m *Model) Save(path string) {
	if m.blockEdit() {
		return
	}
	path = strings.TrimSpace(path)
	if path == "" {
		path = m.path
	}
	if path == "" {
		m.setStatus("no save path given")
		m.generation++
		return
	}
	m.touch()
	var buf bytes.Buffer
	if err := m.bundle.Encode(&buf); err != nil {
		m.setStatus("save failed: %v", err)
		m.generation++
		return
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		m.setStatus("write failed: %v", err)
		m.generation++
		return
	}
	m.path = path
	m.setStatus("saved %s (%d bytes)", path, buf.Len())
	m.generation++
}

// ImportClip adds a Lottie JSON file to the bundle under its file name.
func (m *Model) ImportClip(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		m.setStatus("no clip path given")
		m.generation++
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		m.setStatus("read failed: %v", err)
		m.generation++
		return
	}
	id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if err := m.bundle.SetAnimation(id, data); err != nil {
		m.setStatus("import failed: %v", err)
		m.generation++
		return
	}
	if !slices.Contains(m.sources, path) {
		m.sources = append(m.sources, path)
	}
	// An import replaces whatever that id held, so a document parsed from
	// the old bytes is stale.
	delete(m.clipDocs, id)
	m.setStatus("imported clip %q", id)
	m.generation++
}

// Sources lists the disk files behind the current document, oldest first.
func (m *Model) Sources() []string { return m.sources }

// SetViewer switches watch-and-reload mode on; Viewer reports it.
func (m *Model) SetViewer(v bool) { m.viewer = v }
func (m *Model) Viewer() bool     { return m.viewer }

// Reload rebuilds the document from its source files, replaying the open
// and every import, and keeps the selected machine when it still exists.
// It is how viewer mode follows edits made outside the editor.
func (m *Model) Reload() {
	if len(m.sources) == 0 {
		return
	}
	srcs := slices.Clone(m.sources)
	prevMachine := m.machineID
	m.Open(srcs[0])
	for _, p := range srcs[1:] {
		m.ImportClip(p)
	}
	if prevMachine != "" && slices.Contains(m.bundle.StateMachineIDs(), prevMachine) {
		m.SelectMachine(prevMachine)
	}
	m.setStatus("auto-reloaded %s", filepath.Base(srcs[0]))
	m.generation++
}

func (m *Model) RemoveClip(id string) {
	if m.blockEdit() {
		return
	}
	m.bundle.RemoveAnimation(id)
	// The clip's hitbox track goes with it; the core leaves extension
	// files alone, so the cleanup is this editor's job.
	lottieresolv.Remove(m.bundle, id)
	delete(m.trackCache, id)
	delete(m.clipDocs, id)
	m.setStatus("removed clip %q", id)
	m.generation++
}

func (m *Model) AnimationIDs() []string { return m.bundle.AnimationIDs() }

// ClipSummary describes a clip for the clips list.
func (m *Model) ClipSummary(id string) string {
	anim, err := m.bundle.Animation(id)
	if err != nil {
		return "unreadable"
	}
	// Kept short: the summary shares its row with the clip's name, and the
	// name is the part you are looking for.
	w, h := anim.Size()
	s := fmt.Sprintf("%.2fs %d×%d", anim.Duration().Seconds(), w, h)
	if n := len(anim.Markers()); n > 0 {
		s += fmt.Sprintf(" ▾%d", n)
	}
	return s
}

// Markers lists the marker names of a clip, for the segment picker.
func (m *Model) Markers(animID string) []string {
	anim, err := m.bundle.Animation(animID)
	if err != nil {
		return nil
	}
	var out []string
	for _, mk := range anim.Markers() {
		if mk.Name != "" {
			out = append(out, mk.Name)
		}
	}
	return out
}

func (m *Model) MachineIDs() []string { return m.bundle.StateMachineIDs() }

// InspectTarget reports what kind of thing the inspector should edit.
func (m *Model) InspectTarget() inspectTarget { return m.inspect }

func (m *Model) setInspect(t inspectTarget) { m.inspect = t }

// ShowConfigPane opens the bundle configuration in the inspector; the
// config is just another selectable thing.
func (m *Model) ShowConfigPane() {
	m.setInspect(inspectConfig)
	m.generation++
}

// ---- bundle editor config ----

// editorConfig is what this editor stores at bundle level, in the
// manifest's x-lottie-go-editor extra member: the same channel as state
// node positions, traveling with the bundle and ignored elsewhere.
type editorConfig struct {
	// Physics picks which collision tooling the editor shows: "cp",
	// "resolv", "both" (the empty default), or "none". It gates editor
	// UI, not runtime behavior — a game chooses engines by importing
	// plugins.
	Physics string `json:"physics,omitempty"`
}

func (m *Model) editorConfigValue() editorConfig {
	var cfg editorConfig
	if man := m.bundle.Manifest(); man.Extra != nil {
		if raw, ok := man.Extra[editorExtraKey]; ok {
			_ = json.Unmarshal(raw, &cfg)
		}
	}
	return cfg
}

func (m *Model) setEditorConfig(cfg editorConfig) {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return
	}
	man := m.bundle.Manifest()
	if man.Extra == nil {
		man.Extra = lottie.ExtraFields{}
	}
	man.Extra[editorExtraKey] = raw
}

// PhysicsBackend is the configured collision tooling: cp, resolv, both,
// or none. The empty value reads as both, so old bundles show everything.
func (m *Model) PhysicsBackend() string {
	switch v := m.editorConfigValue().Physics; v {
	case "cp", "resolv", "none":
		return v
	}
	return "both"
}

func (m *Model) SetPhysicsBackend(v string) {
	if m.blockEdit() {
		return
	}
	cfg := m.editorConfigValue()
	cfg.Physics = v
	m.setEditorConfig(cfg)
	m.setStatus("physics tooling: %s", v)
	m.generation++
}

// ResolvEnabled reports whether hitbox-track tooling should show.
func (m *Model) ResolvEnabled() bool {
	v := m.PhysicsBackend()
	return v == "resolv" || v == "both"
}

// CPEnabled reports whether rigid-body tooling should show.
func (m *Model) CPEnabled() bool {
	v := m.PhysicsBackend()
	return v == "cp" || v == "both"
}

// ---- transport ----

// AutoPlay reports whether the stage starts clips playing by itself.
func (m *Model) AutoPlay() bool { return m.autoPlay }

// SetAutoPlay switches automatic playback; it only affects what enters
// the stage from now on, so pausing mid-clip stays where it is.
func (m *Model) SetAutoPlay(v bool) {
	m.autoPlay = v
	m.generation++
}

// PreviewPlaying reports whether the stage is advancing.
func (m *Model) PreviewPlaying() bool {
	p := m.PreviewPlayer()
	return p != nil && p.IsPlaying()
}

// PausePreview halts playback where it is. Chart edits call this first:
// placing a span under a moving playhead is near impossible.
func (m *Model) PausePreview() {
	if p := m.PreviewPlayer(); p != nil && p.IsPlaying() {
		p.Pause()
		m.generation++
	}
}

// TogglePreviewPlaying is the chart's play/pause button.
func (m *Model) TogglePreviewPlaying() {
	p := m.PreviewPlayer()
	if p == nil {
		return
	}
	if p.IsPlaying() {
		p.Pause()
	} else {
		// Playing ends the park: the key under the playhead is about to
		// stop being the key under the playhead, and a clip preview loops.
		m.clearPoseSelection()
		if m.clipPlayer != nil {
			p.SetLoop(true)
		}
		p.Play()
	}
	m.generation++
}

// StepPreviewFrame pauses and nudges the playhead by whole frames.
func (m *Model) StepPreviewFrame(delta float64) {
	p := m.PreviewPlayer()
	if p == nil {
		return
	}
	p.Pause()
	p.SetFrame(p.Frame() + delta)
	m.generation++
}

func (m *Model) SelectMachine(id string) {
	sm, err := m.bundle.StateMachine(id)
	if err != nil {
		m.setStatus("cannot open machine %q: %v", id, err)
		m.generation++
		return
	}
	m.machineID, m.machine = id, sm
	m.selectedState, m.selectedTrans = sm.Initial, -1
	m.setInspect(inspectMachine)
	m.generation++
	m.restartPreview()
}

// NewMachine creates an empty machine and selects it.
func (m *Model) NewMachine() {
	if m.blockEdit() {
		return
	}
	id := uniqueID("machine", m.bundle.StateMachineIDs())
	sm := &lottie.StateMachine{}
	if err := m.bundle.SetStateMachine(id, sm); err != nil {
		m.setStatus("cannot create machine: %v", err)
		m.generation++
		return
	}
	m.machineID, m.machine = id, sm
	m.selectedState, m.selectedTrans = "", -1
	m.setInspect(inspectMachine)
	m.setStatus("created machine %q", id)
	m.generation++
}

// InitialMachine is the machine the manifest names for a player that asks
// for none. It is what NewStateMachinePlayer("") loads.
func (m *Model) InitialMachine() string {
	if in := m.bundle.Manifest().Initial; in != nil {
		return in.StateMachine
	}
	return ""
}

// SetInitialMachine records which machine a player loads by default, or
// clears the choice when id is empty so the first listed wins again.
func (m *Model) SetInitialMachine(id string) {
	if m.blockEdit() {
		return
	}
	man := m.bundle.Manifest()
	if man.Initial == nil {
		if id == "" {
			return
		}
		// Keep any initial animation the bundle already named.
		man.Initial = &lottie.ManifestInitial{}
	}
	man.Initial.StateMachine = id
	if id == "" {
		m.setStatus("cleared the default machine")
	} else {
		m.setStatus("machine %q is now the default", id)
	}
	m.docGen++
	m.generation++
}

// RenameMachine changes a machine's id, which is also the name of its file
// under s/.
func (m *Model) RenameMachine(old, name string) {
	if m.blockEdit() {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" || name == old {
		return
	}
	if slices.Contains(m.bundle.StateMachineIDs(), name) {
		m.setStatus("a machine named %q already exists", name)
		m.generation++
		return
	}
	sm, err := m.bundle.StateMachine(old)
	if err != nil {
		m.setStatus("cannot rename %q: %v", old, err)
		m.generation++
		return
	}
	// Note this before removing: dropping the machine reconciles the
	// manifest, which clears a pointer at an id that no longer exists.
	wasInitial := m.InitialMachine() == old
	if err := m.bundle.SetStateMachine(name, sm); err != nil {
		m.setStatus("cannot rename %q: %v", old, err)
		m.generation++
		return
	}
	m.bundle.RemoveStateMachine(old)
	if wasInitial {
		man := m.bundle.Manifest()
		if man.Initial == nil {
			man.Initial = &lottie.ManifestInitial{}
		}
		man.Initial.StateMachine = name
	}
	if m.machineID == old {
		m.machineID = name
	}
	m.setStatus("renamed machine %q to %q", old, name)
	m.docGen++
	m.generation++
}

// DeleteMachine drops a machine and moves to whatever is left.
func (m *Model) DeleteMachine(id string) {
	if m.blockEdit() {
		return
	}
	if id == "" {
		return
	}
	m.bundle.RemoveStateMachine(id)
	m.setStatus("deleted machine %q", id)
	m.docGen++
	if m.machineID != id {
		m.generation++
		return
	}
	m.machineID, m.machine = "", nil
	m.selectedState, m.selectedTrans = "", -1
	if ids := m.bundle.StateMachineIDs(); len(ids) > 0 {
		m.generation++
		m.SelectMachine(ids[0])
		return
	}
	// Nothing left to run.
	m.preview, m.previewErr = nil, nil
	m.generation++
}

func uniqueID(base string, taken []string) string {
	used := map[string]bool{}
	for _, t := range taken {
		used[t] = true
	}
	if !used[base] {
		return base
	}
	for i := 2; ; i++ {
		id := fmt.Sprintf("%s%d", base, i)
		if !used[id] {
			return id
		}
	}
}

// ---- states ----

func (m *Model) SelectedState() *lottie.State {
	if m.machine == nil || m.selectedState == "" {
		return nil
	}
	st, _ := m.machine.State(m.selectedState)
	return st
}

func (m *Model) SelectState(name string) {
	m.selectedState = name
	m.selectedTrans = -1
	m.setInspect(inspectState)
	m.generation++
}

func (m *Model) SelectedStateName() string { return m.selectedState }

// AddState appends a playback state wired to the first available clip.
func (m *Model) AddState() {
	if m.blockEdit() {
		return
	}
	if m.machine == nil {
		m.NewMachine()
	}
	names := make([]string, 0, len(m.machine.States))
	for _, s := range m.machine.States {
		names = append(names, s.Name)
	}
	name := uniqueID("state", names)
	anim := ""
	if ids := m.bundle.AnimationIDs(); len(ids) > 0 {
		anim = ids[0]
	}
	st := lottie.State{
		Name: name, Type: lottie.StatePlayback, Animation: anim,
		Autoplay: true, Loop: true,
	}
	setNodePos(&st, gridPos(len(m.machine.States)))
	m.machine.States = append(m.machine.States, st)
	if m.machine.Initial == "" {
		m.machine.Initial = name
	}
	m.selectedState, m.selectedTrans = name, -1
	m.setInspect(inspectState)
	m.setStatus("added state %q", name)
	m.touch()
}

func (m *Model) DeleteState(name string) {
	if m.blockEdit() {
		return
	}
	if m.machine == nil {
		return
	}
	for i, s := range m.machine.States {
		if s.Name == name {
			m.machine.States = append(m.machine.States[:i], m.machine.States[i+1:]...)
			break
		}
	}
	// Transitions pointing at it would dangle; drop them too. If the
	// selected state's list shrinks, the selected index would silently
	// name a different transition, so clear it.
	for i := range m.machine.States {
		trs := m.machine.States[i].Transitions[:0]
		for _, tr := range m.machine.States[i].Transitions {
			if tr.ToState != name {
				trs = append(trs, tr)
			}
		}
		if m.machine.States[i].Name == m.selectedState &&
			len(trs) != len(m.machine.States[i].Transitions) {
			m.selectedTrans = -1
		}
		m.machine.States[i].Transitions = trs
	}
	if m.machine.Initial == name {
		m.machine.Initial = ""
		if len(m.machine.States) > 0 {
			m.machine.Initial = m.machine.States[0].Name
		}
	}
	if m.selectedState == name {
		m.selectedState, m.selectedTrans = m.machine.Initial, -1
	}
	m.setStatus("deleted state %q", name)
	m.touch()
}

// RenameState renames a state and repoints every transition to it.
func (m *Model) RenameState(old, name string) {
	if m.blockEdit() {
		return
	}
	name = strings.TrimSpace(name)
	if m.machine == nil || name == "" || name == old {
		return
	}
	if _, exists := m.machine.State(name); exists {
		m.setStatus("a state named %q already exists", name)
		m.generation++
		return
	}
	for i := range m.machine.States {
		if m.machine.States[i].Name == old {
			m.machine.States[i].Name = name
		}
		for j := range m.machine.States[i].Transitions {
			if m.machine.States[i].Transitions[j].ToState == old {
				m.machine.States[i].Transitions[j].ToState = name
			}
		}
	}
	if m.machine.Initial == old {
		m.machine.Initial = name
	}
	if m.selectedState == old {
		m.selectedState = name
	}
	m.touch()
}

func (m *Model) SetInitial(name string) {
	if m.blockEdit() {
		return
	}
	if m.machine == nil {
		return
	}
	m.machine.Initial = name
	m.setStatus("initial state is now %q", name)
	m.touch()
}

// ---- transitions ----

func (m *Model) SelectedTransition() *lottie.Transition {
	st := m.SelectedState()
	if st == nil || m.selectedTrans < 0 || m.selectedTrans >= len(st.Transitions) {
		return nil
	}
	return &st.Transitions[m.selectedTrans]
}

func (m *Model) SelectTransition(i int) {
	m.selectedTrans = i
	m.generation++
}

func (m *Model) SelectedTransitionIndex() int { return m.selectedTrans }

func (m *Model) AddTransition(to string) {
	if m.blockEdit() {
		return
	}
	st := m.SelectedState()
	if st == nil {
		return
	}
	st.Transitions = append(st.Transitions, lottie.Transition{
		Type: lottie.TransitionImmediate, ToState: to,
	})
	m.selectedTrans = len(st.Transitions) - 1
	m.touch()
}

func (m *Model) DeleteTransition(i int) {
	if m.blockEdit() {
		return
	}
	st := m.SelectedState()
	if st == nil || i < 0 || i >= len(st.Transitions) {
		return
	}
	st.Transitions = append(st.Transitions[:i], st.Transitions[i+1:]...)
	m.selectedTrans = -1
	m.touch()
}

// MoveTransition reorders a transition. Order decides which one wins, so it
// is a semantic edit, not cosmetic.
func (m *Model) MoveTransition(i, delta int) {
	if m.blockEdit() {
		return
	}
	st := m.SelectedState()
	if st == nil {
		return
	}
	j := i + delta
	if i < 0 || i >= len(st.Transitions) || j < 0 || j >= len(st.Transitions) {
		return
	}
	st.Transitions[i], st.Transitions[j] = st.Transitions[j], st.Transitions[i]
	m.selectedTrans = j
	m.touch()
}

// ---- guards ----

func (m *Model) AddGuard() {
	if m.blockEdit() {
		return
	}
	tr := m.SelectedTransition()
	if tr == nil {
		return
	}
	name := ""
	if m.machine != nil {
		for _, in := range m.machine.Inputs {
			if in.Type == lottie.InputEvent {
				name = in.Name
				break
			}
		}
	}
	tr.Guards = append(tr.Guards, lottie.Guard{Type: lottie.GuardEvent, InputName: name})
	m.touch()
}

func (m *Model) DeleteGuard(i int) {
	if m.blockEdit() {
		return
	}
	tr := m.SelectedTransition()
	if tr == nil || i < 0 || i >= len(tr.Guards) {
		return
	}
	tr.Guards = append(tr.Guards[:i], tr.Guards[i+1:]...)
	m.touch()
}

// ---- inputs ----

func (m *Model) AddInput(t lottie.InputType) {
	if m.blockEdit() {
		return
	}
	if m.machine == nil {
		m.NewMachine()
	}
	names := make([]string, 0, len(m.machine.Inputs))
	for _, in := range m.machine.Inputs {
		names = append(names, in.Name)
	}
	base := "trigger"
	switch t {
	case lottie.InputBoolean:
		base = "flag"
	case lottie.InputNumeric:
		base = "value"
	case lottie.InputString:
		base = "text"
	}
	m.machine.Inputs = append(m.machine.Inputs, lottie.Input{Type: t, Name: uniqueID(base, names)})
	m.touch()
}

func (m *Model) DeleteInput(i int) {
	if m.blockEdit() {
		return
	}
	if m.machine == nil || i < 0 || i >= len(m.machine.Inputs) {
		return
	}
	m.machine.Inputs = append(m.machine.Inputs[:i], m.machine.Inputs[i+1:]...)
	m.touch()
}

func (m *Model) RenameInput(i int, name string) {
	if m.blockEdit() {
		return
	}
	name = strings.TrimSpace(name)
	if m.machine == nil || i < 0 || i >= len(m.machine.Inputs) || name == "" {
		return
	}
	old := m.machine.Inputs[i].Name
	if old == name {
		return
	}
	// Inputs are addressed by name (guards, the values tab), so a
	// duplicate would make every lookup hit whichever comes first.
	for j := range m.machine.Inputs {
		if j != i && m.machine.Inputs[j].Name == name {
			m.setStatus("an input named %q already exists", name)
			m.generation++
			return
		}
	}
	m.machine.Inputs[i].Name = name
	// Guards refer to inputs by name; keep them pointing at this one.
	for si := range m.machine.States {
		for ti := range m.machine.States[si].Transitions {
			for gi := range m.machine.States[si].Transitions[ti].Guards {
				g := &m.machine.States[si].Transitions[ti].Guards[gi]
				if g.InputName == old {
					g.InputName = name
				}
			}
		}
	}
	m.touch()
}

// EventInputs lists the Event input names, which are the game-facing
// triggers the preview offers as buttons.
func (m *Model) EventInputs() []string {
	if m.machine == nil {
		return nil
	}
	var out []string
	for _, in := range m.machine.Inputs {
		if in.Type == lottie.InputEvent {
			out = append(out, in.Name)
		}
	}
	return out
}

func (m *Model) InputNames() []string {
	if m.machine == nil {
		return nil
	}
	out := make([]string, 0, len(m.machine.Inputs))
	for _, in := range m.machine.Inputs {
		out = append(out, in.Name)
	}
	return out
}

func (m *Model) StateNames() []string {
	if m.machine == nil {
		return nil
	}
	out := make([]string, 0, len(m.machine.States))
	for _, s := range m.machine.States {
		out = append(out, s.Name)
	}
	return out
}

// ---- validation ----

func (m *Model) Problems() []string {
	if m.machine == nil {
		return nil
	}
	// Validation is called from Build, so it must neither mark the document
	// changed nor revalidate on every tick.
	if m.problemsGen == m.generation+1 {
		return m.problemsCache
	}
	m.syncMachine()
	var out []string
	for _, p := range m.bundle.Validate() {
		out = append(out, p.Error())
	}
	sort.Strings(out)
	m.problemsCache, m.problemsGen = out, m.generation+1
	return out
}

// ---- input selection ----

// SelectInput records which input the graph should trace. Selecting one
// highlights every transition guarded on it.
func (m *Model) SelectInput(i int) {
	m.selectedInput = i
	m.generation++
}

func (m *Model) SelectedInputIndex() int { return m.selectedInput }

// SelectedInputName returns the highlighted input's name, or "" for none.
func (m *Model) SelectedInputName() string {
	if m.machine == nil || m.selectedInput < 0 || m.selectedInput >= len(m.machine.Inputs) {
		return ""
	}
	return m.machine.Inputs[m.selectedInput].Name
}

// TransitionUsesInput reports whether any of a transition's guards read the
// given input.
func TransitionUsesInput(tr lottie.Transition, name string) bool {
	if name == "" {
		return false
	}
	for _, g := range tr.Guards {
		if g.InputName == name {
			return true
		}
	}
	return false
}

// Fire raises an event on the running machine, which is what the try button
// beside each event input does.
func (m *Model) Fire(name string) {
	if m.preview != nil {
		m.preview.Fire(name)
	}
}

// SetInputValue writes a value input on the running machine.
func SetInputValue[T lottie.InputValue](m *Model, name string, v T) {
	if m.preview != nil {
		m.preview.Set(name, v)
	}
}

// ---- preview ----

func (m *Model) Preview() *lottie.StateMachinePlayer { return m.preview }
func (m *Model) PreviewErr() error                   { return m.previewErr }

// clipRef is one playable unit: a file, optionally narrowed to one of its
// markers. A document carrying three markers is three clips as far as this
// tool is concerned, which is how they are listed.
type clipRef struct {
	Anim    string
	Segment string
}

// Label is how the unit reads in a list: the segment leads when there is
// one, since that is the part being played.
func (c clipRef) Label() string {
	if c.Segment != "" {
		return c.Segment
	}
	return c.Anim
}

// markerRef is a marker together with the file it lives in.
type markerRef struct {
	Anim  string
	Name  string
	Start float64
	End   float64
}

// ClipRefs lists every playable unit in the bundle: one row per marker, or
// the whole file when it carries none. The same file name repeats across
// its segments, which is the point.
func (m *Model) ClipRefs() []clipRef {
	var out []clipRef
	for _, id := range m.bundle.AnimationIDs() {
		anim, err := m.bundle.Animation(id)
		if err != nil {
			out = append(out, clipRef{Anim: id})
			continue
		}
		var named int
		for _, mk := range anim.Markers() {
			if mk.Name == "" {
				continue
			}
			named++
			out = append(out, clipRef{Anim: id, Segment: mk.Name})
		}
		if named == 0 {
			out = append(out, clipRef{Anim: id})
		}
	}
	return out
}

// MarkerRefs lists every marker in the bundle. They are the machine's
// outgoing interface: a game reacts to them through OnMarker.
func (m *Model) MarkerRefs() []markerRef {
	var out []markerRef
	for _, id := range m.bundle.AnimationIDs() {
		anim, err := m.bundle.Animation(id)
		if err != nil {
			continue
		}
		for _, mk := range anim.Markers() {
			if mk.Name == "" {
				continue
			}
			out = append(out, markerRef{Anim: id, Name: mk.Name, Start: mk.Start, End: mk.End})
		}
	}
	return out
}

type markerKey struct{ Anim, Name string }

// MarkerHits reports how often one file's marker has fired since the stage
// last changed.
func (m *Model) MarkerHits(anim, name string) int {
	return m.markerHits[markerKey{anim, name}]
}

// MarkerGeneration counts every marker emission. Widgets hash it instead of
// re-reading every marker in the bundle on each state-key check.
func (m *Model) MarkerGeneration() int { return m.markerGen }

// ClipSummaryRef describes one playable unit for the clips list.
func (m *Model) ClipSummaryRef(c clipRef) string {
	anim, err := m.bundle.Animation(c.Anim)
	if err != nil {
		return "unreadable"
	}
	if c.Segment == "" {
		w, h := anim.Size()
		return fmt.Sprintf("%.2fs %d×%d", anim.Duration().Seconds(), w, h)
	}
	mk, ok := anim.Marker(c.Segment)
	if !ok {
		return c.Anim + " (missing)"
	}
	fps := anim.FrameRate()
	if fps <= 0 {
		fps = 60
	}
	return fmt.Sprintf("%s %.2fs", c.Anim, (mk.End-mk.Start)/fps)
}

// PreviewClip returns the unit being previewed on its own; Anim is empty
// when the preview is running the machine.
func (m *Model) PreviewClip() clipRef { return m.previewClip }

// ShowClip switches the preview to one playable unit, looped so it can be
// judged without wiring it into a machine first.
func (m *Model) ShowClip(c clipRef) {
	anim, err := m.bundle.Animation(c.Anim)
	if err != nil {
		m.setStatus("cannot play clip %q: %v", c.Anim, err)
		m.generation++
		return
	}
	p := anim.NewPlayer()
	p.SetLoop(true)
	if c.Segment != "" && !p.SetMarkerRange(c.Segment) {
		m.setStatus("clip %q has no marker %q", c.Anim, c.Segment)
		m.generation++
		return
	}
	p.Rewind()
	if !m.autoPlay {
		p.Pause()
	}
	p.OnMarker(func(mk lottie.Marker) { m.noteMarker(c.Anim, mk) })
	// The counts describe what is on the stage, so switching clips starts
	// them over rather than carrying the last clip's tally forward.
	m.resetMarkerHits()
	// The hitbox selection indexed the previous stage's track, and the pose
	// and shape selections indexed its layers and key times. The shape
	// selection survives when the same document stays on stage — parking a
	// key re-shows the clip a machine was already playing, and clearing
	// there left every later stage click picking against no layer at all.
	if m.StageAnimID() != c.Anim {
		m.clearShapeSelection()
	}
	m.previewClip, m.clipPlayer = c, p
	m.selBox = -1
	m.clearPoseSelection()
	m.setStatus("previewing %s", c.Label())
	m.generation++
}

// ShowMachine returns the preview to the state machine.
func (m *Model) ShowMachine() {
	if m.previewClip.Anim == "" {
		return
	}
	m.previewClip, m.clipPlayer = clipRef{}, nil
	m.resetMarkerHits()
	m.selBox = -1
	m.clearPoseSelection()
	m.clearShapeSelection()
	m.generation++
}

func (m *Model) noteMarker(anim string, mk lottie.Marker) {
	if m.markerHits == nil {
		m.markerHits = map[markerKey]int{}
	}
	m.markerHits[markerKey{anim, mk.Name}]++
	m.markerGen++
}

// animForState resolves which file a state plays, so a marker the machine
// reports can be attributed to the clip it came from.
func (m *Model) animForState(name string) string {
	if m.machine == nil {
		return ""
	}
	st, ok := m.machine.State(name)
	if !ok {
		return ""
	}
	return st.Animation
}

// resetMarkerHits starts the counts over. They describe the run currently on
// the stage, so swapping what is playing clears them.
func (m *Model) resetMarkerHits() {
	clear(m.markerHits)
	m.markerGen++
}

// ActiveState is the state the running machine is in, or "" when a clip is
// being previewed on its own.
func (m *Model) ActiveState() string {
	if m.previewClip.Anim != "" || m.preview == nil {
		return ""
	}
	return m.preview.State()
}

// PreviewLabel describes what the stage is showing.
func (m *Model) PreviewLabel() string {
	switch {
	case m.previewClip.Anim != "":
		return "clip: " + m.previewClip.Label()
	case m.previewErr != nil:
		return "preview error"
	case m.preview == nil:
		return "no preview"
	default:
		return "state: " + m.preview.State()
	}
}

// PreviewUpdate advances whichever player the stage is showing.
func (m *Model) PreviewUpdate() {
	if m.clipPlayer != nil {
		m.clipPlayer.Update()
		return
	}
	if m.preview != nil {
		m.preview.Update()
	}
}

// PreviewDraw renders whichever player the stage is showing.
func (m *Model) PreviewDraw(dst *ebiten.Image, opts *lottie.DrawOptions) {
	if m.clipPlayer != nil {
		m.clipPlayer.Draw(dst, opts)
		return
	}
	if m.preview != nil {
		m.preview.Draw(dst, opts)
	}
}

// PreviewPlayer is the player behind the stage, whichever is showing. The
// timeline reads its position and range through this.
func (m *Model) PreviewPlayer() *lottie.Player {
	if m.clipPlayer != nil {
		return m.clipPlayer
	}
	if m.preview == nil {
		return nil
	}
	return m.preview.Player()
}

// PreviewMarkers lists the markers of the animation on the stage. They are
// what a state's segment names, so the timeline shows where they fall.
func (m *Model) PreviewMarkers() []lottie.Marker {
	anim := m.PreviewAnimation()
	if anim == nil {
		return nil
	}
	return anim.Markers()
}

// PreviewSeek scrubs the stage to an absolute frame.
func (m *Model) PreviewSeek(frame float64) {
	p := m.PreviewPlayer()
	if p == nil {
		return
	}
	p.SetFrame(frame)
	m.generation++
}

// PreviewAnimation is the animation on the stage, for sizing the drawing.
func (m *Model) PreviewAnimation() *lottie.Animation {
	if m.clipPlayer != nil {
		return m.clipPlayer.Animation()
	}
	if m.preview == nil || m.preview.Player() == nil {
		return nil
	}
	return m.preview.Player().Animation()
}

// PreviewStale reports whether the document was edited since the running
// preview was built. Selection and playback do not count.
func (m *Model) PreviewStale() bool {
	return m.preview != nil && m.previewGen != m.docGen
}

func (m *Model) RestartPreview() {
	m.previewClip, m.clipPlayer = clipRef{}, nil
	m.generation++
	m.restartPreview()
}

// restartPreview rebuilds the running player and records the document
// revision it was built from.
func (m *Model) restartPreview() {
	m.preview, m.previewErr = nil, nil
	if m.machine == nil || m.machineID == "" {
		return
	}
	if err := m.bundle.SetStateMachine(m.machineID, m.machine); err != nil {
		m.previewErr = err
		return
	}
	p, err := m.bundle.NewStateMachinePlayer(m.machineID)
	if err != nil {
		m.previewErr = err
		return
	}
	m.resetMarkerHits()
	p.OnMarker(func(state string, mk lottie.Marker) {
		m.noteMarker(m.animForState(state), mk)
	})
	// With auto-play off, every state's clip arrives paused on its first
	// frame — transitions still take (guards need no playback), so firing
	// an event lands the new clip ready to be stepped through.
	p.OnStateChanged(func(from, to string) {
		if !m.autoPlay {
			if pl := p.Player(); pl != nil {
				pl.Pause()
			}
		}
	})
	if !m.autoPlay {
		if pl := p.Player(); pl != nil {
			pl.Pause()
		}
	}
	m.preview = p
	m.previewGen = m.docGen
}

// ---- node positions ----

func nodePos(st *lottie.State) (image.Point, bool) {
	if st == nil || st.Extra == nil {
		return image.Point{}, false
	}
	raw, ok := st.Extra[editorExtraKey]
	if !ok {
		return image.Point{}, false
	}
	var n nodeMeta
	if json.Unmarshal(raw, &n) != nil {
		return image.Point{}, false
	}
	return image.Pt(n.X, n.Y), true
}

func setNodePos(st *lottie.State, p image.Point) {
	if st == nil {
		return
	}
	raw, err := json.Marshal(nodeMeta{X: p.X, Y: p.Y})
	if err != nil {
		return
	}
	if st.Extra == nil {
		st.Extra = lottie.ExtraFields{}
	}
	st.Extra[editorExtraKey] = raw
}

// NodePos returns a state's stored position, falling back to a grid slot so
// a machine authored elsewhere still lays out sensibly.
func (m *Model) NodePos(index int) image.Point {
	if m.machine == nil || index < 0 || index >= len(m.machine.States) {
		return image.Point{}
	}
	if p, ok := nodePos(&m.machine.States[index]); ok {
		return p
	}
	return gridPos(index)
}

// gridPos lays nodes out in columns wide enough that they do not overlap.
func gridPos(index int) image.Point {
	return image.Pt(30+200*(index%3), 30+110*(index/3))
}

func (m *Model) SetNodePos(index int, p image.Point) {
	if m.blockEdit() {
		return
	}
	if m.machine == nil || index < 0 || index >= len(m.machine.States) {
		return
	}
	if p.X < 0 {
		p.X = 0
	}
	if p.Y < 0 {
		p.Y = 0
	}
	setNodePos(&m.machine.States[index], p)
	// Positions do not affect playback, so skip the reserialization touch
	// does; Save and Problems write the machine back before they need it.
	m.generation++
}
