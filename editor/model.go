package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	lottie "github.com/shibukawa/lottie-go"
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
		dialog:        make(chan dialogResult, 1),
	}
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
func (m *Model) Touch() { m.touch() }

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

// Open loads a .lottie bundle, or a bare .json clip into a fresh bundle.
func (m *Model) Open(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		m.setStatus("no path given")
		m.generation++
		return
	}
	if strings.EqualFold(filepath.Ext(path), ".json") {
		m.bundle = lottie.NewBundle()
		m.path = ""
		m.machineID, m.machine = "", nil
		m.ImportClip(path)
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
	m.selectedState, m.selectedTrans = "", -1
	m.machineID, m.machine = "", nil
	m.generation++
	if ids := b.StateMachineIDs(); len(ids) > 0 {
		m.SelectMachine(ids[0])
	}
	m.setStatus("loaded %s (%d clips, %d machines)", path,
		len(b.AnimationIDs()), len(b.StateMachineIDs()))
}

// Save writes the bundle as dotLottie v2.
func (m *Model) Save(path string) {
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
	m.setStatus("imported clip %q", id)
	m.generation++
}

func (m *Model) RemoveClip(id string) {
	m.bundle.RemoveAnimation(id)
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

func (m *Model) SelectMachine(id string) {
	sm, err := m.bundle.StateMachine(id)
	if err != nil {
		m.setStatus("cannot open machine %q: %v", id, err)
		m.generation++
		return
	}
	m.machineID, m.machine = id, sm
	m.selectedState, m.selectedTrans = sm.Initial, -1
	m.generation++
	m.restartPreview()
}

// NewMachine creates an empty machine and selects it.
func (m *Model) NewMachine() {
	id := uniqueID("machine", m.bundle.StateMachineIDs())
	sm := &lottie.StateMachine{}
	if err := m.bundle.SetStateMachine(id, sm); err != nil {
		m.setStatus("cannot create machine: %v", err)
		m.generation++
		return
	}
	m.machineID, m.machine = id, sm
	m.selectedState, m.selectedTrans = "", -1
	m.setStatus("created machine %q", id)
	m.generation++
}

// RenameMachine changes a machine's id, which is also the name of its file
// under s/.
func (m *Model) RenameMachine(old, name string) {
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
	if err := m.bundle.SetStateMachine(name, sm); err != nil {
		m.setStatus("cannot rename %q: %v", old, err)
		m.generation++
		return
	}
	m.bundle.RemoveStateMachine(old)
	if m.machineID == old {
		m.machineID = name
	}
	m.setStatus("renamed machine %q to %q", old, name)
	m.docGen++
	m.generation++
}

// DeleteMachine drops a machine and moves to whatever is left.
func (m *Model) DeleteMachine(id string) {
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
	m.generation++
}

func (m *Model) SelectedStateName() string { return m.selectedState }

// AddState appends a playback state wired to the first available clip.
func (m *Model) AddState() {
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
	m.setStatus("added state %q", name)
	m.touch()
}

func (m *Model) DeleteState(name string) {
	if m.machine == nil {
		return
	}
	for i, s := range m.machine.States {
		if s.Name == name {
			m.machine.States = append(m.machine.States[:i], m.machine.States[i+1:]...)
			break
		}
	}
	// Transitions pointing at it would dangle; drop them too.
	for i := range m.machine.States {
		trs := m.machine.States[i].Transitions[:0]
		for _, tr := range m.machine.States[i].Transitions {
			if tr.ToState != name {
				trs = append(trs, tr)
			}
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
	tr := m.SelectedTransition()
	if tr == nil || i < 0 || i >= len(tr.Guards) {
		return
	}
	tr.Guards = append(tr.Guards[:i], tr.Guards[i+1:]...)
	m.touch()
}

// ---- inputs ----

func (m *Model) AddInput(t lottie.InputType) {
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
	if m.machine == nil || i < 0 || i >= len(m.machine.Inputs) {
		return
	}
	m.machine.Inputs = append(m.machine.Inputs[:i], m.machine.Inputs[i+1:]...)
	m.touch()
}

func (m *Model) RenameInput(i int, name string) {
	name = strings.TrimSpace(name)
	if m.machine == nil || i < 0 || i >= len(m.machine.Inputs) || name == "" {
		return
	}
	old := m.machine.Inputs[i].Name
	if old == name {
		return
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
	p.OnMarker(func(mk lottie.Marker) { m.noteMarker(c.Anim, mk) })
	// The counts describe what is on the stage, so switching clips starts
	// them over rather than carrying the last clip's tally forward.
	m.resetMarkerHits()
	m.previewClip, m.clipPlayer = c, p
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
