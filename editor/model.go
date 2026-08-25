package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sort"
	"strings"

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

	// generation counts every change that the UI must redraw for. Widgets
	// hash it in WriteStateKey instead of the whole document.
	generation int
	status     string

	problemsCache []string
	problemsGen   int
}

func NewModel() *Model {
	m := &Model{bundle: lottie.NewBundle(), selectedTrans: -1}
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

// touch records a change: it syncs and bumps the generation.
func (m *Model) touch() {
	m.syncMachine()
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
	// Bump before selecting: SelectMachine starts the preview and records
	// the generation it was built at, so nothing may bump it afterwards.
	m.generation++
	if ids := b.StateMachineIDs(); len(ids) > 0 {
		m.SelectMachine(ids[0])
	}
	m.setStatus("loaded %s (%d clips, %d machines)", filepath.Base(path),
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
	m.setStatus("saved %s (%d bytes)", filepath.Base(path), buf.Len())
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
	w, h := anim.Size()
	s := fmt.Sprintf("%dx%d  %.2fs", w, h, anim.Duration().Seconds())
	if n := len(anim.Markers()); n > 0 {
		s += fmt.Sprintf("  %d markers", n)
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

// ---- preview ----

func (m *Model) Preview() *lottie.StateMachinePlayer { return m.preview }
func (m *Model) PreviewErr() error                   { return m.previewErr }

// PreviewStale reports whether the document changed since the running
// preview was built.
func (m *Model) PreviewStale() bool {
	return m.preview != nil && m.previewGen != m.generation
}

func (m *Model) RestartPreview() {
	m.generation++
	m.restartPreview()
}

// restartPreview rebuilds the running player. The generation must already be
// at its final value, since previewGen is what PreviewStale compares against.
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
	m.preview = p
	m.previewGen = m.generation
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
