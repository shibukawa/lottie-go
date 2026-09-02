package main

// Addresses name what a click selects: the editor's one selection as a
// string an agent can say. Parsing an address into the Model's Select*
// calls performs the click's side effects — stage switch, parked key,
// opened tab — so the human sees where the agent went.

import (
	"fmt"
	"strconv"
	"strings"
)

type address struct {
	kind string
	segs []string
}

func parseAddress(s string) (address, error) {
	s = strings.TrimSpace(s)
	kind, rest, ok := strings.Cut(s, ":")
	if !ok {
		if s == "config" {
			return address{kind: "config"}, nil
		}
		return address{}, refuse(fmt.Sprintf("address %q has no kind", s),
			"write <kind>:<segments>", addressKinds...)
	}
	a := address{kind: strings.ToLower(kind)}
	if rest != "" {
		a.segs = strings.Split(rest, "/")
	}
	for _, k := range addressKinds {
		if k == a.kind {
			return a, nil
		}
	}
	return address{}, refuse(fmt.Sprintf("unknown address kind %q", kind), "", addressKinds...)
}

var addressKinds = []string{
	"clip", "machine", "state", "transition", "guard", "input",
	"part", "key", "layer", "shape", "vertex", "stop", "uv",
	"hitbox", "span", "body", "socket", "config",
}

func (a address) String() string {
	if len(a.segs) == 0 {
		return a.kind
	}
	return a.kind + ":" + strings.Join(a.segs, "/")
}

func addr(kind string, segs ...string) string {
	return address{kind: kind, segs: segs}.String()
}

// ---- focus ----

type focusInfo struct {
	Stage     string   `json:"stage,omitempty"`
	Frame     float64  `json:"frame"`
	Key       *float64 `json:"key,omitempty"`
	Playing   bool     `json:"playing"`
	Selection string   `json:"selection,omitempty"`
	Tab       string   `json:"tab,omitempty"`
}

var tabNames = map[colTab]string{
	colSegment: "segment", colPoses: "poses", colShapes: "shapes",
	colHitboxes: "hitboxes", colBody: "body", colSockets: "sockets",
}

func (s *mcpServer) stageClip() string {
	return s.model.PreviewClip().Anim
}

func (s *mcpServer) focus() focusInfo {
	m := s.model
	f := focusInfo{Tab: tabNames[m.CollisionTab()]}
	if id := s.stageClip(); id != "" {
		f.Stage = addr("clip", id)
	} else if m.MachineID() != "" {
		f.Stage = addr("machine", m.MachineID())
	}
	if p := m.PreviewPlayer(); p != nil {
		f.Frame = p.Frame()
		f.Playing = p.IsPlaying()
	}
	if frame, ok := m.SelectedPoseKey(); ok {
		f.Key = &frame
	}
	f.Selection = s.selection()
	return f
}

// selection formats what the inspector edits as an address.
func (s *mcpServer) selection() string {
	m := s.model
	clip := s.stageClip()
	switch m.InspectTarget() {
	case inspectMachine:
		if m.MachineID() != "" {
			return addr("machine", m.MachineID())
		}
	case inspectState:
		if s.selInput && m.SelectedInputName() != "" {
			return addr("input", m.MachineID(), m.SelectedInputName())
		}
		if st := m.SelectedStateName(); st != "" {
			if i := m.SelectedTransitionIndex(); i >= 0 {
				if s.guard >= 0 {
					return addr("guard", m.MachineID(), st, strconv.Itoa(i), strconv.Itoa(s.guard))
				}
				return addr("transition", m.MachineID(), st, strconv.Itoa(i))
			}
			return addr("state", m.MachineID(), st)
		}
		if m.MachineID() != "" {
			return addr("machine", m.MachineID())
		}
	case inspectHitbox:
		if b := m.SelectedHitbox(); b != nil {
			return addr("hitbox", clip, b.Name)
		}
	case inspectCPShape:
		if i := m.SelectedCPShapeIndex(); i >= 0 {
			return addr("body", strconv.Itoa(i))
		}
	case inspectSocket:
		if sk := m.SelectedSocket(); sk != nil {
			return addr("socket", sk.Name)
		}
	case inspectPose:
		if part := m.SelectedPosePart(); part >= 0 {
			return addr("part", clip, s.layerRef(part))
		}
		if frame, ok := m.SelectedPoseKey(); ok {
			if row := m.SelectedPoseRow(); row >= 0 {
				return addr("key", clip, s.layerRef(row), fmtFloat(frame))
			}
			return addr("key", clip, fmtFloat(frame))
		}
	case inspectShape:
		layer := m.SelectedShapeLayer()
		if layer < 0 {
			return ""
		}
		n, ok := m.SelectedShapeNode()
		if !ok {
			return addr("layer", clip, s.layerRef(layer))
		}
		segs := append([]string{clip, s.layerRef(layer)}, pathSegs(n.path)...)
		if v := m.SelectedShapeVert(); v >= 0 {
			return addr("vertex", append(segs, strconv.Itoa(v))...)
		}
		if st := m.SelectedGradStop(); st >= 0 {
			return addr("stop", append(segs, strconv.Itoa(st))...)
		}
		if v := m.SelectedUVVert(); v >= 0 {
			return addr("uv", append(segs, strconv.Itoa(v))...)
		}
		return addr("shape", segs...)
	case inspectConfig:
		return "config"
	}
	return ""
}

// layerRef names a layer for an address: its name when that is usable,
// #index otherwise.
func (s *mcpServer) layerRef(layer int) string {
	d := s.model.StageClipDoc()
	if d == nil || layer < 0 || layer >= len(d.layers) {
		return "#" + strconv.Itoa(layer)
	}
	name := d.layers[layer].name
	if name == "" || strings.Contains(name, "/") || s.layerNameCount(d, name) != 1 {
		return "#" + strconv.Itoa(layer)
	}
	return name
}

func (s *mcpServer) layerNameCount(d *clipDoc, name string) int {
	n := 0
	for i := range d.layers {
		if d.layers[i].name == name {
			n++
		}
	}
	return n
}

func pathSegs(path []int) []string {
	out := make([]string, len(path))
	for i, p := range path {
		out[i] = strconv.Itoa(p)
	}
	return out
}

func fmtFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// ---- resolution ----

// selectAddress moves the focus to a, with the side effects a click has.
func (s *mcpServer) selectAddress(text string) error {
	a, err := parseAddress(text)
	if err != nil {
		return err
	}
	m := s.model
	s.guard, s.selInput = -1, false
	switch a.kind {
	case "config":
		m.ShowConfigPane()
		return nil
	case "clip":
		id, err := s.clipArg(a, 0, true)
		if err != nil {
			return err
		}
		if s.stageClip() != id {
			m.ShowClip(clipRef{Anim: id})
		}
		return nil
	case "machine":
		id, err := s.machineArg(a, 0)
		if err != nil {
			return err
		}
		if m.MachineID() != id {
			m.SelectMachine(id)
		} else {
			m.setInspect(inspectMachine)
			m.generation++
		}
		return nil
	case "state", "transition", "guard":
		want := map[string]int{"state": 2, "transition": 3, "guard": 4}[a.kind]
		segs, err := s.withMachine(a, want)
		if err != nil {
			return err
		}
		if m.MachineID() != segs[0] {
			m.SelectMachine(segs[0])
		}
		if _, ok := m.Machine().State(segs[1]); !ok {
			return refuse(fmt.Sprintf("machine %q has no state %q", segs[0], segs[1]), "", m.StateNames()...)
		}
		m.SelectState(segs[1])
		if a.kind == "state" {
			return nil
		}
		st := m.SelectedState()
		n, err := indexArg(segs[2], len(st.Transitions), "transition")
		if err != nil {
			return err
		}
		m.SelectTransition(n)
		if a.kind == "guard" {
			g, err := indexArg(segs[3], len(st.Transitions[n].Guards), "guard")
			if err != nil {
				return err
			}
			s.guard = g
		}
		return nil
	case "input":
		segs, err := s.withMachine(a, 2)
		if err != nil {
			return err
		}
		if m.MachineID() != segs[0] {
			m.SelectMachine(segs[0])
		}
		for i, in := range m.Machine().Inputs {
			if in.Name == segs[1] {
				m.SelectInput(i)
				m.setInspect(inspectState)
				s.selInput = true
				m.generation++
				return nil
			}
		}
		return refuse(fmt.Sprintf("no input %q", segs[1]), "", m.InputNames()...)
	case "part":
		clip, rest, err := s.withClip(a, 1)
		if err != nil {
			return err
		}
		s.stage(clip)
		layer, err := s.layerArg(rest[0], m.PoseParts())
		if err != nil {
			return err
		}
		m.SetCollisionTab(colPoses)
		m.SelectPosePart(layer)
		return nil
	case "key":
		clip, rest, err := s.withClip(a, 1)
		if err != nil {
			return err
		}
		s.stage(clip)
		layer := -1
		frameText := rest[0]
		if len(rest) >= 2 {
			frameText = rest[1]
			layer, err = s.layerArg(rest[0], nil)
			if err != nil {
				return err
			}
		}
		frame, err := strconv.ParseFloat(frameText, 64)
		if err != nil {
			return refuse(fmt.Sprintf("frame %q is not a number", frameText), "", s.keyTimes(layer)...)
		}
		if !hasFloat(s.keyTimesF(layer), frame) {
			return refuse(fmt.Sprintf("no key at frame %s", frameText), "keys are the only frames an edit can land on", s.keyTimes(layer)...)
		}
		if m.CollisionTab() != colShapes {
			m.SetCollisionTab(colPoses)
		}
		m.SelectPoseKey(frame, layer)
		return nil
	case "layer", "shape", "vertex", "stop", "uv":
		clip, rest, err := s.withClip(a, 1)
		if err != nil {
			return err
		}
		s.stage(clip)
		layer, err := s.layerArg(rest[0], m.ShapeLayers())
		if err != nil {
			return err
		}
		m.SetCollisionTab(colShapes)
		m.SelectShapeLayer(layer)
		if a.kind == "layer" {
			return nil
		}
		pathText := rest[1:]
		var tail string
		if a.kind != "shape" {
			if len(pathText) < 2 {
				return refusef("%s needs a shape path and an index", a.kind)
			}
			tail, pathText = pathText[len(pathText)-1], pathText[:len(pathText)-1]
		}
		if len(pathText) == 0 {
			return refuse("shape needs an item path", "", s.shapePaths(layer)...)
		}
		path := make([]int, len(pathText))
		for i, p := range pathText {
			v, err := strconv.Atoi(p)
			if err != nil {
				return refuse(fmt.Sprintf("shape path segment %q is not an index", p), "", s.shapePaths(layer)...)
			}
			path[i] = v
		}
		d := m.StageClipDoc()
		if _, ok := d.shapeItem(layer, path); !ok {
			return refuse(fmt.Sprintf("no shape item at %s", strings.Join(pathText, "/")), "", s.shapePaths(layer)...)
		}
		m.SelectShapeNode(path)
		if a.kind == "shape" {
			return nil
		}
		i, err := strconv.Atoi(tail)
		if err != nil {
			return refusef("%s index %q is not a number", a.kind, tail)
		}
		switch a.kind {
		case "vertex":
			p, ok := m.ShapePath()
			if !ok {
				return refuse("the selected item is not a path", "", "")
			}
			if i < 0 || i >= len(p.v) {
				return refusef("vertex %d out of range 0..%d", i, len(p.v)-1)
			}
			m.SelectShapeVert(i)
		case "stop":
			stops := m.ShapeGradientStops()
			if i < 0 || i >= len(stops) {
				return refusef("stop %d out of range 0..%d", i, len(stops)-1)
			}
			m.SelectGradStop(i)
		case "uv":
			uv, ok := m.ShapeUVs()
			if !ok || i < 0 || i >= len(uv) {
				return refusef("uv %d out of range (%d points)", i, len(uv))
			}
			m.SelectUVVert(i)
		}
		return nil
	case "hitbox", "span":
		clip, rest, err := s.withClip(a, 1)
		if err != nil {
			return err
		}
		s.stage(clip)
		m.SetCollisionTab(colHitboxes)
		tr := m.StageTrack()
		if tr == nil {
			return refuse("this clip has no hitbox track", "add kind=hitbox creates one")
		}
		var names []string
		for i, b := range tr.Boxes {
			names = append(names, b.Name)
			if b.Name == rest[0] {
				m.SelectHitbox(i)
				if a.kind == "span" {
					if len(rest) < 2 {
						return refusef("span needs an index")
					}
					n, err := indexArg(rest[1], len(b.Spans), "span")
					if err != nil {
						return err
					}
					m.PreviewSeek(b.Spans[n].From)
				}
				return nil
			}
		}
		return refuse(fmt.Sprintf("no hitbox %q", rest[0]), "", names...)
	case "body":
		if len(a.segs) != 1 {
			return refusef("body needs one index")
		}
		m.SetCollisionTab(colBody)
		n, err := indexArg(a.segs[0], len(m.CPBodyShapes()), "body shape")
		if err != nil {
			return err
		}
		m.SelectCPShape(n)
		return nil
	case "socket":
		if len(a.segs) != 1 {
			return refusef("socket needs one name")
		}
		m.SetCollisionTab(colSockets)
		var names []string
		for i, sk := range m.Sockets() {
			names = append(names, sk.Name)
			if sk.Name == a.segs[0] {
				m.SelectSocket(i)
				return nil
			}
		}
		return refuse(fmt.Sprintf("no socket %q", a.segs[0]), "", names...)
	}
	return refusef("unhandled address kind %q", a.kind)
}

// stage puts a clip on stage unless it already is.
func (s *mcpServer) stage(clip string) {
	if s.stageClip() != clip {
		s.model.ShowClip(clipRef{Anim: clip})
	}
}

func (s *mcpServer) clipArg(a address, i int, required bool) (string, error) {
	m := s.model
	if i >= len(a.segs) || a.segs[i] == "" {
		if id := s.stageClip(); id != "" && !required {
			return id, nil
		}
		return "", refuse("no clip named", "put the clip on stage or name it", m.AnimationIDs()...)
	}
	id := a.segs[i]
	for _, have := range m.AnimationIDs() {
		if have == id {
			return id, nil
		}
	}
	return "", refuse(fmt.Sprintf("no clip %q", id), "", m.AnimationIDs()...)
}

func (s *mcpServer) machineArg(a address, i int) (string, error) {
	m := s.model
	if i >= len(a.segs) || a.segs[i] == "" {
		if m.MachineID() != "" {
			return m.MachineID(), nil
		}
		return "", refuse("no machine selected", "", m.MachineIDs()...)
	}
	for _, have := range m.MachineIDs() {
		if have == a.segs[i] {
			return a.segs[i], nil
		}
	}
	return "", refuse(fmt.Sprintf("no machine %q", a.segs[i]), "", m.MachineIDs()...)
}

// withMachine returns want segments, prepending the current machine when
// the address left it out.
func (s *mcpServer) withMachine(a address, want int) ([]string, error) {
	segs := a.segs
	if len(segs) == want-1 {
		id, err := s.machineArg(address{}, 0)
		if err != nil {
			return nil, err
		}
		segs = append([]string{id}, segs...)
	}
	if len(segs) != want {
		return nil, refusef("%s needs %d segments", a.kind, want)
	}
	if _, err := s.machineArg(address{segs: segs}, 0); err != nil {
		return nil, err
	}
	return segs, nil
}

// withClip splits off the clip segment, defaulting to the stage clip, and
// returns the rest, which must hold at least min entries.
func (s *mcpServer) withClip(a address, min int) (string, []string, error) {
	segs := a.segs
	if len(segs) == min {
		id, err := s.clipArg(address{}, 0, false)
		if err != nil {
			return "", nil, err
		}
		return id, segs, nil
	}
	if len(segs) < min+1 {
		return "", nil, refusef("%s needs at least %d segments", a.kind, min+1)
	}
	id, err := s.clipArg(a, 0, true)
	if err != nil {
		return "", nil, err
	}
	return id, segs[1:], nil
}

// layerArg resolves a layer name or #index against the stage clip; among
// limits the answer to those indices when given.
func (s *mcpServer) layerArg(text string, among []int) (int, error) {
	d := s.model.StageClipDoc()
	if d == nil {
		return 0, refuse("the stage clip cannot be edited", "")
	}
	allowed := func(i int) bool {
		return among == nil || hasInt(among, i)
	}
	candidates := func() []string {
		var out []string
		for i := range d.layers {
			if allowed(i) {
				out = append(out, s.layerRef(i))
			}
		}
		return out
	}
	if strings.HasPrefix(text, "#") {
		i, err := strconv.Atoi(text[1:])
		if err != nil || i < 0 || i >= len(d.layers) || !allowed(i) {
			return 0, refuse(fmt.Sprintf("no layer %s", text), "", candidates()...)
		}
		return i, nil
	}
	found := -1
	for i := range d.layers {
		if d.layers[i].name == text && allowed(i) {
			if found >= 0 {
				return 0, refuse(fmt.Sprintf("layer name %q is not unique", text), "use #<index>", candidates()...)
			}
			found = i
		}
	}
	if found < 0 {
		return 0, refuse(fmt.Sprintf("no layer %q", text), "", candidates()...)
	}
	return found, nil
}

func (s *mcpServer) keyTimesF(layer int) []float64 {
	m := s.model
	if layer >= 0 {
		return m.PoseRowTimes(layer)
	}
	if t := m.PoseTimes(); t != nil {
		return t
	}
	// Not a pose sequence: the union of every row.
	var out []float64
	for _, l := range m.PoseRows() {
		for _, t := range m.PoseRowTimes(l) {
			if !hasFloat(out, t) {
				out = append(out, t)
			}
		}
	}
	return out
}

func (s *mcpServer) keyTimes(layer int) []string {
	var out []string
	for _, t := range s.keyTimesF(layer) {
		out = append(out, fmtFloat(t))
	}
	return out
}

func (s *mcpServer) shapePaths(layer int) []string {
	d := s.model.StageClipDoc()
	if d == nil {
		return nil
	}
	var out []string
	for _, n := range d.shapeTree(layer) {
		label := strings.Join(pathSegs(n.path), "/")
		if n.ty != "" {
			label += " (" + n.ty + ")"
		}
		out = append(out, label)
	}
	return out
}

func indexArg(text string, n int, what string) (int, error) {
	i, err := strconv.Atoi(text)
	if err != nil || i < 0 || i >= n {
		return 0, refuse(fmt.Sprintf("%s index %q out of range 0..%d", what, text, n-1), "")
	}
	return i, nil
}

func hasInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func hasFloat(xs []float64, v float64) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
