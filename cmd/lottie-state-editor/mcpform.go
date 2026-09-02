package main

// The inspector pane as data. inspect returns the selection's fields —
// name, type, value, options, keyed, writable — so the agent learns what
// it can set from the reply rather than from the tool list, and set writes
// those fields back through the same Model calls the pane uses.

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	lottie "github.com/shibukawa/lottie-go"
	lottiesockets "github.com/shibukawa/lottie-go/plugin/sockets"
)

type field struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Value    any      `json:"value"`
	Options  []string `json:"options,omitempty"`
	Keyed    bool     `json:"keyed,omitempty"`
	Writable bool     `json:"writable"`
	Problem  string   `json:"problem,omitempty"`
}

type form struct {
	Kind    string         `json:"kind"`
	Address string         `json:"address,omitempty"`
	Fields  []field        `json:"fields"`
	Extra   map[string]any `json:"extra,omitempty"`
}

func (s *mcpServer) formPayload() form {
	m := s.model
	f := form{Address: s.selection(), Extra: map[string]any{}}
	ro := m.Viewer()
	w := func(b bool) bool { return b && !ro }
	switch m.InspectTarget() {
	case inspectMachine:
		s.machineForm(&f, w)
	case inspectState:
		switch {
		case s.selInput && m.SelectedInputIndex() >= 0:
			s.inputForm(&f, w)
		case m.SelectedState() == nil:
			s.machineForm(&f, w)
		case m.SelectedTransitionIndex() >= 0 && s.guard >= 0:
			s.guardForm(&f, w)
		case m.SelectedTransitionIndex() >= 0:
			s.transitionForm(&f, w)
		default:
			s.stateForm(&f, w)
		}
	case inspectHitbox:
		s.hitboxForm(&f, w)
	case inspectCPShape:
		s.bodyForm(&f, w)
	case inspectSocket:
		s.socketForm(&f, w)
	case inspectPose:
		s.poseForm(&f, w)
	case inspectShape:
		s.shapeForm(&f, w)
	case inspectConfig:
		s.configForm(&f)
	}
	if f.Kind == "" {
		f.Kind = "none"
	}
	if len(f.Extra) == 0 {
		f.Extra = nil
	}
	return f
}

func fld(name, typ string, value any, writable bool) field {
	return field{Name: name, Type: typ, Value: value, Writable: writable}
}

func enum(name string, value string, writable bool, options ...string) field {
	return field{Name: name, Type: "enum", Value: value, Options: options, Writable: writable}
}

// ---- machine, state, transition, guard, input ----

func (s *mcpServer) machineForm(f *form, w func(bool) bool) {
	m := s.model
	f.Kind = "machine"
	sm := m.Machine()
	if sm == nil {
		return
	}
	f.Fields = []field{
		fld("id", "string", m.MachineID(), w(true)),
		enum("initial", sm.Initial, w(true), m.StateNames()...),
		fld("default", "bool", m.InitialMachine() == m.MachineID(), w(true)),
	}
	f.Extra["states"] = m.StateNames()
	f.Extra["inputs"] = s.inputList()
}

func (s *mcpServer) inputList() []map[string]any {
	var out []map[string]any
	if sm := s.model.Machine(); sm != nil {
		for _, in := range sm.Inputs {
			row := map[string]any{"name": in.Name, "type": string(in.Type)}
			if len(in.Value) > 0 {
				row["value"] = json.RawMessage(in.Value)
			}
			out = append(out, row)
		}
	}
	return out
}

func (s *mcpServer) stateForm(f *form, w func(bool) bool) {
	m := s.model
	st := m.SelectedState()
	f.Kind = "state"
	if st == nil {
		return
	}
	playback := st.Type != lottie.StateGlobal
	segs := append([]string{""}, m.Markers(st.Animation)...)
	f.Fields = []field{
		fld("name", "string", st.Name, w(true)),
		enum("type", string(st.Type), w(true), string(lottie.StatePlayback), string(lottie.StateGlobal)),
		enum("animation", st.Animation, w(playback), m.AnimationIDs()...),
		enum("segment", st.Segment, w(playback), segs...),
		fld("loop", "bool", st.Loop, w(playback)),
		fld("loopCount", "number", st.LoopCount, w(playback)),
		fld("autoplay", "bool", st.Autoplay, w(playback)),
		enum("mode", string(st.PlaybackMode()), w(playback), string(lottie.PlayForward), string(lottie.PlayReverse)),
		fld("speed", "number", st.PlaybackSpeed(), w(playback)),
		fld("initial", "bool", m.Machine().Initial == st.Name, w(true)),
	}
	f.Extra["transitions"] = transitionRows(st)
}

func transitionRows(st *lottie.State) []map[string]any {
	out := []map[string]any{}
	for i, tr := range st.Transitions {
		out = append(out, map[string]any{
			"n": i, "toState": tr.ToState, "type": string(tr.Type), "guards": summarizeGuards(tr.Guards),
		})
	}
	return out
}

func (s *mcpServer) transitionForm(f *form, w func(bool) bool) {
	m := s.model
	tr := m.SelectedTransition()
	f.Kind = "transition"
	if tr == nil {
		return
	}
	tweened := string(tr.Type) == "Tweened"
	f.Fields = []field{
		enum("toState", tr.ToState, w(true), m.StateNames()...),
		enum("type", string(tr.Type), w(true), "Transition", "Tweened"),
		fld("duration", "number", tr.Duration, w(tweened)),
		fld("easing", "vec", tr.Easing, w(tweened)),
	}
	var guards []map[string]any
	for i, g := range tr.Guards {
		guards = append(guards, map[string]any{"g": i, "summary": describeGuard(g)})
	}
	f.Extra["guards"] = guards
	f.Extra["order"] = m.SelectedTransitionIndex()
}

var guardTypes = []string{"Event", "Boolean", "Numeric", "String"}
var conditionTypes = []string{"Equal", "NotEqual", "GreaterThan", "GreaterThanOrEqual", "LessThan", "LessThanOrEqual"}
var inputTypes = []string{"Event", "Boolean", "Numeric", "String"}

func (s *mcpServer) selectedGuard() *lottie.Guard {
	tr := s.model.SelectedTransition()
	if tr == nil || s.guard < 0 || s.guard >= len(tr.Guards) {
		return nil
	}
	return &tr.Guards[s.guard]
}

func (s *mcpServer) guardForm(f *form, w func(bool) bool) {
	m := s.model
	g := s.selectedGuard()
	f.Kind = "guard"
	if g == nil {
		return
	}
	compares := g.Type != lottie.GuardEvent
	var cmp any
	if len(g.CompareTo) > 0 {
		cmp = json.RawMessage(g.CompareTo)
	}
	f.Fields = []field{
		enum("type", string(g.Type), w(true), guardTypes...),
		enum("inputName", g.InputName, w(true), m.InputNames()...),
		enum("conditionType", string(g.ConditionType), w(compares), conditionTypes...),
		fld("compareTo", "json", cmp, w(compares)),
	}
}

func (s *mcpServer) inputForm(f *form, w func(bool) bool) {
	m := s.model
	f.Kind = "input"
	sm := m.Machine()
	i := m.SelectedInputIndex()
	if sm == nil || i < 0 || i >= len(sm.Inputs) {
		return
	}
	in := sm.Inputs[i]
	var val any
	if len(in.Value) > 0 {
		val = json.RawMessage(in.Value)
	}
	f.Fields = []field{
		fld("name", "string", in.Name, w(true)),
		enum("type", string(in.Type), false, inputTypes...),
		fld("value", "json", val, w(in.Type != lottie.InputEvent)),
	}
}

// ---- collision, sockets ----

func (s *mcpServer) hitboxForm(f *form, w func(bool) bool) {
	m := s.model
	b := m.SelectedHitbox()
	f.Kind = "hitbox"
	if b == nil {
		return
	}
	f.Fields = []field{
		fld("name", "string", b.Name, w(true)),
		fld("kind", "string", string(b.Kind), false),
		fld("tags", "list", b.Tags, w(true)),
	}
	if sp := m.SelectedSpan(); sp != nil {
		f.Fields = append(f.Fields,
			fld("from", "number", sp.From, w(true)),
			fld("to", "number", sp.To, w(true)),
			fld("x", "number", sp.X, w(true)),
			fld("y", "number", sp.Y, w(true)),
		)
		if b.Kind == "circle" {
			f.Fields = append(f.Fields, fld("r", "number", sp.R, w(true)))
		} else if b.Kind == "rect" {
			f.Fields = append(f.Fields, fld("w", "number", sp.W, w(true)), fld("h", "number", sp.H, w(true)))
		}
	} else {
		f.Extra["hint"] = "no span covers the playhead; seek into one (span address) or add kind=span"
	}
	var spans []map[string]any
	for i, sp := range b.Spans {
		spans = append(spans, map[string]any{"n": i, "from": sp.From, "to": sp.To, "x": sp.X, "y": sp.Y, "w": sp.W, "h": sp.H, "r": sp.R})
	}
	f.Extra["spans"] = spans
}

func (s *mcpServer) bodyForm(f *form, w func(bool) bool) {
	m := s.model
	sh := m.SelectedCPShape()
	f.Kind = "body"
	if sh == nil {
		return
	}
	f.Fields = []field{
		fld("type", "string", string(sh.Type), false),
		fld("center", "vec", []float64{sh.Center.X, sh.Center.Y}, w(true)),
		fld("radius", "number", sh.Radius, w(true)),
		fld("width", "number", sh.Width, w(true)),
		fld("height", "number", sh.Height, w(true)),
		fld("friction", "number", sh.Friction, w(true)),
		fld("elasticity", "number", sh.Elasticity, w(true)),
		fld("sensor", "bool", sh.Sensor, w(true)),
	}
	if len(sh.Vertices) > 0 {
		f.Fields = append(f.Fields, fld("vertices", "list", sh.Vertices, false))
	}
}

func (s *mcpServer) socketForm(f *form, w func(bool) bool) {
	m := s.model
	sk := m.SelectedSocket()
	f.Kind = "socket"
	if sk == nil {
		return
	}
	f.Fields = []field{
		fld("name", "string", sk.Name, w(true)),
		fld("layer", "string", sk.LayerName(), false),
		fld("dx", "number", sk.DX, w(true)),
		fld("dy", "number", sk.DY, w(true)),
		fld("dr", "number", sk.DR, w(true)),
		fld("rotate", "bool", sk.Rotate != lottiesockets.RotateNone, w(true)),
		fld("behind", "bool", sk.Z == lottiesockets.ZBehind, w(true)),
	}
}

// ---- pose ----

func (s *mcpServer) poseForm(f *form, w func(bool) bool) {
	m := s.model
	f.Kind = "pose"
	frame, onKey := m.SelectedPoseKey()
	part := m.SelectedPosePart()
	if onKey {
		f.Extra["frame"] = frame
		f.Fields = append(f.Fields,
			fld("ease", "bool", m.PoseEased(), w(true)),
		)
	} else {
		f.Extra["hint"] = "park on a key (select key:<frame>) before editing values"
	}
	f.Fields = append(f.Fields, fld("length", "number", m.ClipLength(), w(true)))
	if part >= 0 {
		var parents []string
		for _, c := range m.PoseParentCandidates() {
			parents = append(parents, s.layerRef(c))
		}
		parent := ""
		if p, ok := m.PosePartParent(); ok {
			parent = s.layerRef(p)
		}
		name := m.SelectedPosePartName()
		f.Fields = append(f.Fields,
			field{Name: "part", Type: "string", Value: name, Writable: w(true), Problem: m.PosePartNameProblem()},
			fld("joint", "string", m.PoseJointName(), false),
			enum("parent", parent, w(onKey), append([]string{""}, parents...)...),
			fld("hidden", "bool", m.PosePartHidden(part), w(onKey)),
		)
		d := m.StageClipDoc()
		for _, prop := range []string{"p", "a", "s", "r", "o"} {
			v, ok := m.PoseValue(prop)
			if !ok && d != nil {
				// Off a key the pane shows nothing; the agent still wants
				// to read where the part is, so report the nearest stored
				// value, read-only.
				v, ok = d.valueNear(part, prop, m.stageFrame())
			}
			if !ok {
				continue
			}
			typ := "vec"
			var val any = v
			if len(v) == 1 {
				typ, val = "number", v[0]
			}
			f.Fields = append(f.Fields, field{
				Name: prop, Type: typ, Value: val, Keyed: m.PoseValueIsKeyed(prop), Writable: w(onKey),
			})
		}
	}
	var parts []map[string]any
	for _, l := range m.PoseParts() {
		parts = append(parts, map[string]any{
			"layer": s.layerRef(l), "index": l, "name": m.PoseLayerName(l), "hidden": m.PosePartHidden(l),
		})
	}
	f.Extra["parts"] = parts
	f.Extra["pose_times"] = s.keyTimesF(-1)
}

// ---- shapes ----

// shapeMembers lists the numeric members the form offers per item kind.
// "c" on a fill or stroke is a color and is shown as hex.
var shapeMembers = map[string][]string{
	"fl": {"c", "o"},
	"st": {"c", "o", "w", "lc", "lj", "ml"},
	"gf": {"o", "s", "e", "h", "a"},
	"gs": {"o", "w", "lc", "lj", "ml", "s", "e", "h", "a"},
	"rc": {"p", "s", "r"},
	"el": {"p", "s"},
	"sr": {"p", "pt", "ir", "or", "is", "os", "r", "sy"},
	"tr": {"p", "a", "s", "r", "o", "sk", "sa"},
	"tm": {"s", "e", "o", "m"},
	"rd": {"r"},
	"pb": {"a"},
	"zz": {"r", "s", "pt"},
	"op": {"a", "lj", "ml"},
	"rp": {"c", "o"},
}

var shapeIntMembers = map[string]bool{"lc": true, "lj": true, "m": true, "sy": true}

var textureStrings = map[string][]string{
	"mapping": {"", "bbox", "vertex"},
	"wrap":    {"", "clamp", "repeat", "mirror"},
	"filter":  {"", "linear", "nearest"},
}

func (s *mcpServer) shapeForm(f *form, w func(bool) bool) {
	m := s.model
	f.Kind = "layer"
	layer := m.SelectedShapeLayer()
	d := m.StageClipDoc()
	if d == nil || layer < 0 {
		return
	}
	frame, onKey := m.SelectedPoseKey()
	if onKey {
		f.Extra["frame"] = frame
	}
	n, ok := m.SelectedShapeNode()
	if !ok {
		f.Fields = []field{
			field{Name: "name", Type: "string", Value: d.layers[layer].name, Writable: false, Problem: m.ShapeLayerNameProblem()},
		}
		f.Extra["tree"] = s.shapeTreeRows(layer)
		f.Extra["hint"] = "select shape:<layer>/<path> to edit an item, or add kind=shape_item"
		return
	}
	f.Kind = "shape"
	f.Fields = []field{
		fld("item", "string", n.ty, false),
		fld("name", "string", n.name, w(true)),
	}
	if n.ty == "gf" || n.ty == "gs" {
		f.Fields = append(f.Fields, fld("radial", "bool", m.ShapeGradientRadial(), w(m.ShapeMemberWritable("t"))))
	}
	for _, member := range shapeMembers[n.ty] {
		if member == "c" && (n.ty == "fl" || n.ty == "st") {
			hex, ok := m.ShapeColorHex()
			if ok {
				f.Fields = append(f.Fields, field{Name: "c", Type: "color", Value: hex, Keyed: shapeKeyed(m, "c"), Writable: w(m.ShapeMemberWritable("c"))})
			}
			continue
		}
		if shapeIntMembers[member] {
			if v, ok := m.ShapePlainInt(member); ok {
				f.Fields = append(f.Fields, fld(member, "number", v, w(true)))
			}
			continue
		}
		v, ok := m.ShapeMemberValue(member)
		if !ok {
			continue
		}
		typ := "vec"
		var val any = v
		if len(v) == 1 {
			typ, val = "number", v[0]
		}
		f.Fields = append(f.Fields, field{Name: member, Type: typ, Value: val, Keyed: shapeKeyed(m, member), Writable: w(m.ShapeMemberWritable(member))})
	}
	switch n.ty {
	case "gf", "gs":
		var stops []map[string]any
		for i, st := range m.ShapeGradientStops() {
			stops = append(stops, map[string]any{"n": i, "pos": st.pos, "color": rgbHex(st.r, st.g, st.b)})
		}
		f.Extra["stops"] = stops
		if st := m.SelectedGradStop(); st >= 0 {
			f.Kind = "stop"
			hex, _ := m.GradStopColorHex()
			stops := m.ShapeGradientStops()
			pos := 0.0
			if st < len(stops) {
				pos = stops[st].pos
			}
			f.Fields = []field{
				fld("pos", "number", pos, w(true)),
				fld("color", "color", hex, w(true)),
			}
		}
	case "sh":
		p, ok := m.ShapePath()
		if ok {
			f.Fields = append(f.Fields, fld("closed", "bool", p.closed, w(m.ShapePathWritable())))
			var verts []map[string]any
			for i := range p.v {
				verts = append(verts, map[string]any{"v": i, "point": p.v[i], "in": p.i[i], "out": p.o[i]})
			}
			f.Extra["vertices"] = verts
			f.Extra["path_writable"] = m.ShapePathWritable()
			if v := m.SelectedShapeVert(); v >= 0 && v < len(p.v) {
				f.Kind = "vertex"
				f.Fields = []field{
					fld("point", "vec", p.v[v], w(m.ShapePathWritable())),
					fld("in", "vec", p.i[v], w(m.ShapePathWritable())),
					fld("out", "vec", p.o[v], w(m.ShapePathWritable())),
					fld("tangents", "bool", m.ShapeVertexHasTangents(v), w(m.ShapePathWritable())),
				}
			}
		}
	}
	if m.ShapeCanTexture() {
		var choices []string
		choices = append(choices, "")
		for _, c := range m.TextureChoices() {
			choices = append(choices, c.ID)
		}
		f.Fields = append(f.Fields, enum("texture", m.ShapeTextureName(), w(true), choices...))
		if m.ShapeTextureName() != "" {
			f.Fields = append(f.Fields, fld("tint", "bool", m.ShapeTextureTint(), w(true)))
			for _, key := range []string{"mapping", "wrap", "filter"} {
				f.Fields = append(f.Fields, enum(key, m.ShapeTextureString(key), w(true), textureStrings[key]...))
			}
			for _, member := range []string{"p", "r", "s", "a"} {
				if v, ok := m.ShapeTexTransformValue(member); ok {
					var val any = v
					typ := "vec"
					if len(v) == 1 {
						typ, val = "number", v[0]
					}
					f.Fields = append(f.Fields, field{Name: "tex_" + member, Type: typ, Value: val, Writable: w(m.ShapeTexTransformWritable(member))})
				}
			}
			if uv, ok := m.ShapeUVs(); ok {
				f.Extra["uv"] = uv
				if i := m.SelectedUVVert(); i >= 0 && i < len(uv) {
					f.Kind = "uv"
					f.Fields = []field{fld("uv", "vec", uv[i], w(m.ShapeUVEditable()))}
				}
			}
		}
	}
	f.Extra["tree"] = s.shapeTreeRows(layer)
}

func shapeKeyed(m *Model, member string) bool {
	item, ok := m.SelectedShapeItem()
	return ok && propAnimatedObj(item, member)
}

func (s *mcpServer) shapeTreeRows(layer int) []map[string]any {
	d := s.model.StageClipDoc()
	rows := []map[string]any{}
	if d == nil {
		return rows
	}
	for _, n := range d.shapeTree(layer) {
		row := map[string]any{"path": strings.Join(pathSegs(n.path), "/"), "kind": n.ty, "depth": n.depth}
		if n.name != "" {
			row["name"] = n.name
		}
		rows = append(rows, row)
	}
	return rows
}

func rgbHex(r, g, b float64) string {
	c := func(v float64) int { return int(math.Round(min(max(v, 0), 1) * 255)) }
	return fmt.Sprintf("#%02x%02x%02x", c(r), c(g), c(b))
}

// ---- config ----

func (s *mcpServer) configForm(f *form) {
	m := s.model
	f.Kind = "config"
	joint := "moves_part"
	if m.JointDragKeepsArt() {
		joint = "keeps_art"
	}
	f.Fields = []field{
		enum("physics", m.PhysicsBackend(), true, "both", "cp", "resolv", "none"),
		fld("viewer", "bool", m.Viewer(), true),
		fld("autoplay", "bool", m.AutoPlay(), true),
		fld("onion_skin", "bool", m.OnionSkin(), true),
		fld("rig", "bool", m.ShowRig(), true),
		enum("joint_drag", joint, true, "moves_part", "keeps_art"),
		fld("mcp", "bool", m.MCPOn(), true),
		fld("mcp_url", "string", m.MCPURL(), false),
	}
}

// ---- set ----

// setFields writes fields into the selection. Every name is checked before
// anything is written, so one unknown field refuses the whole call.
func (s *mcpServer) setFields(in map[string]any) ([]string, error) {
	f := s.formPayload()
	if f.Kind == "none" {
		return nil, refuse("nothing is selected", "select an address first")
	}
	var valid []string
	byName := map[string]field{}
	for _, fd := range f.Fields {
		byName[fd.Name] = fd
		if fd.Writable {
			valid = append(valid, fd.Name)
		}
	}
	for name := range in {
		fd, ok := byName[name]
		if !ok {
			return nil, refuse(fmt.Sprintf("%s has no field %q", f.Kind, name), "", valid...)
		}
		if !fd.Writable {
			hint := "read-only here"
			if fd.Problem != "" {
				hint = fd.Problem
			} else if fd.Keyed || f.Kind == "pose" || f.Kind == "shape" {
				hint = "not writable now — park on a key (select key:<frame>) or check viewer mode"
			}
			return nil, refuse(fmt.Sprintf("field %q is not writable", name), hint, valid...)
		}
		if len(fd.Options) > 0 {
			if sv, ok := in[name].(string); ok && !hasString(fd.Options, sv) {
				return nil, refuse(fmt.Sprintf("%q is not a valid %s", sv, name), "", fd.Options...)
			}
		}
	}
	var changed []string
	for _, name := range sortedKeys(in) {
		if err := s.setField(f.Kind, name, in[name]); err != nil {
			return changed, err
		}
		changed = append(changed, name)
	}
	return changed, nil
}

func hasString(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func (s *mcpServer) setField(kind, name string, v any) error {
	m := s.model
	switch kind {
	case "machine":
		switch name {
		case "id":
			m.RenameMachine(m.MachineID(), toString(v))
		case "initial":
			m.SetInitial(toString(v))
		case "default":
			if toBool(v) {
				m.SetInitialMachine(m.MachineID())
			} else if m.InitialMachine() == m.MachineID() {
				m.SetInitialMachine("")
			}
		}
	case "state":
		st := m.SelectedState()
		switch name {
		case "name":
			m.RenameState(st.Name, toString(v))
			return nil
		case "initial":
			if toBool(v) {
				m.SetInitial(st.Name)
			}
			return nil
		case "type":
			st.Type = lottie.StateType(toString(v))
		case "animation":
			if a := toString(v); a != st.Animation {
				st.Animation, st.Segment = a, ""
			}
		case "segment":
			st.Segment = toString(v)
		case "loop":
			st.Loop = toBool(v)
		case "loopCount":
			st.LoopCount = int(toFloat(v))
		case "autoplay":
			st.Autoplay = toBool(v)
		case "mode":
			st.Mode = lottie.PlayMode(toString(v))
		case "speed":
			if sp := toFloat(v); sp >= 0 {
				st.Speed = sp
			}
		}
		m.Touch()
	case "transition":
		tr := m.SelectedTransition()
		switch name {
		case "toState":
			tr.ToState = toString(v)
		case "type":
			tr.Type = lottie.TransitionType(toString(v))
		case "duration":
			tr.Duration = toFloat(v)
		case "easing":
			tr.Easing = toFloats(v)
		}
		m.Touch()
	case "guard":
		g := s.selectedGuard()
		switch name {
		case "type":
			g.Type = lottie.GuardType(toString(v))
			if g.Type == lottie.GuardEvent {
				g.ConditionType, g.CompareTo = "", nil
			}
		case "inputName":
			g.InputName = toString(v)
		case "conditionType":
			g.ConditionType = lottie.ConditionType(toString(v))
		case "compareTo":
			g.CompareTo = lottie.JSONValue(v)
		}
		m.Touch()
	case "input":
		i := m.SelectedInputIndex()
		switch name {
		case "name":
			m.RenameInput(i, toString(v))
		case "value":
			m.Machine().Inputs[i].Value = lottie.JSONValue(v)
			m.Touch()
		}
	case "hitbox":
		b := m.SelectedHitbox()
		sp := m.SelectedSpan()
		switch name {
		case "name":
			m.RenameHitbox(toString(v))
		case "tags":
			m.SetHitboxTagsCSV(strings.Join(toStrings(v), ", "))
		case "from":
			m.SetSpanRange(toFloat(v), sp.To)
		case "to":
			m.SetSpanRange(sp.From, toFloat(v))
		case "x", "y", "w", "h", "r":
			val := round2(toFloat(v))
			switch name {
			case "x":
				sp.X = val
			case "y":
				sp.Y = val
			case "w":
				sp.W = max(val, 1)
			case "h":
				sp.H = max(val, 1)
			case "r":
				sp.R = max(val, 1)
			}
			_ = b
			m.touchTrack()
		}
	case "body":
		sh := m.SelectedCPShape()
		switch name {
		case "center":
			c := toFloats(v)
			if len(c) >= 2 {
				sh.Center.X, sh.Center.Y = round2(c[0]), round2(c[1])
			}
		case "radius":
			sh.Radius = round2(toFloat(v))
		case "width":
			sh.Width = round2(toFloat(v))
		case "height":
			sh.Height = round2(toFloat(v))
		case "friction":
			sh.Friction = toFloat(v)
		case "elasticity":
			sh.Elasticity = toFloat(v)
		case "sensor":
			sh.Sensor = toBool(v)
		}
		m.touchCPBody()
	case "socket":
		sk := m.SelectedSocket()
		switch name {
		case "name":
			m.RenameSocket(toString(v))
			return nil
		case "dx":
			sk.DX = round2(toFloat(v))
		case "dy":
			sk.DY = round2(toFloat(v))
		case "dr":
			sk.DR = round2(toFloat(v))
		case "rotate":
			if (sk.Rotate != lottiesockets.RotateNone) != toBool(v) {
				m.ToggleSocketRotate()
			}
			return nil
		case "behind":
			if (sk.Z == lottiesockets.ZBehind) != toBool(v) {
				m.ToggleSocketZ()
			}
			return nil
		}
		m.touchSockets()
	case "pose":
		switch name {
		case "ease":
			m.SetPoseEase(toBool(v))
		case "length":
			m.SetClipLength(toFloat(v))
		case "part":
			m.RenamePosePart(toString(v))
		case "parent":
			if toString(v) == "" {
				m.SetPosePartParent(-1)
				return nil
			}
			i, err := s.layerArg(toString(v), nil)
			if err != nil {
				return err
			}
			m.SetPosePartParent(i)
		case "hidden":
			if m.PosePartHidden(m.SelectedPosePart()) != toBool(v) {
				m.TogglePosePartHidden()
			}
		case "p", "a", "s", "r", "o":
			m.BeginPoseEdit()
			m.SetPoseValue(name, toFloats(v))
			m.EndPoseEdit()
		}
	case "shape", "layer":
		return s.setShapeField(name, v)
	case "stop":
		switch name {
		case "pos":
			m.SetGradStopPos(m.SelectedGradStop(), toFloat(v))
		case "color":
			m.SetGradStopColorHex(toString(v))
		}
	case "vertex":
		idx := m.SelectedShapeVert()
		p, _ := m.ShapePath()
		switch name {
		case "point":
			pt := toFloats(v)
			if len(pt) >= 2 && idx < len(p.v) {
				m.SetShapeVertexValue(0, pt[0])
				m.SetShapeVertexValue(1, pt[1])
			}
		case "in", "out":
			pt := toFloats(v)
			if len(pt) >= 2 && idx < len(p.v) {
				cur := p.i[idx]
				if name == "out" {
					cur = p.o[idx]
				}
				m.MoveShapeHandle(idx, name == "out", pt[0]-cur[0], pt[1]-cur[1])
			}
		case "tangents":
			m.SetShapeVertexTangents(idx, toBool(v))
		}
	case "uv":
		pt := toFloats(v)
		if len(pt) >= 2 {
			m.SetShapeUV(m.SelectedUVVert(), pt[0], pt[1])
		}
	case "config":
		switch name {
		case "physics":
			m.SetPhysicsBackend(toString(v))
		case "viewer":
			m.SetViewer(toBool(v))
		case "autoplay":
			m.SetAutoPlay(toBool(v))
		case "onion_skin":
			m.SetOnionSkin(toBool(v))
		case "rig":
			m.SetShowRig(toBool(v))
		case "joint_drag":
			m.SetJointDragKeepsArt(toString(v) == "keeps_art")
		case "mcp":
			m.SetMCPOn(toBool(v))
		}
	default:
		return refusef("%s fields cannot be set", kind)
	}
	return nil
}

func (s *mcpServer) setShapeField(name string, v any) error {
	m := s.model
	switch name {
	case "name":
		m.RenameShapeItem(toString(v))
	case "radial":
		m.SetShapeGradientRadial(toBool(v))
	case "c":
		m.SetShapeColorHex(toString(v))
	case "closed":
		m.SetShapePathClosed(toBool(v))
	case "texture":
		m.SetShapeTexture(toString(v))
	case "tint":
		m.SetShapeTextureTint(toBool(v))
	case "mapping", "wrap", "filter":
		m.SetShapeTextureString(name, toString(v))
	default:
		if strings.HasPrefix(name, "tex_") {
			m.SetShapeTexTransform(strings.TrimPrefix(name, "tex_"), toFloats(v))
			return nil
		}
		if shapeIntMembers[name] {
			m.SetShapePlainInt(name, int(toFloat(v)))
			return nil
		}
		m.SetShapeMemberValue(name, toFloats(v))
	}
	return nil
}

// ---- coercion ----

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	case json.RawMessage:
		var s string
		if json.Unmarshal(x, &s) == nil {
			return s
		}
		return string(x)
	}
	return fmt.Sprint(v)
}

func toBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		b, _ := strconv.ParseBool(x)
		return b
	case float64:
		return x != 0
	}
	return false
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f
	case []any:
		if len(x) > 0 {
			return toFloat(x[0])
		}
	}
	return 0
}

func toFloats(v any) []float64 {
	switch x := v.(type) {
	case []any:
		out := make([]float64, len(x))
		for i, e := range x {
			out[i] = toFloat(e)
		}
		return out
	case []float64:
		return x
	case [2]float64:
		return x[:]
	case nil:
		return nil
	}
	return []float64{toFloat(v)}
}

func toStrings(v any) []string {
	switch x := v.(type) {
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			if s := strings.TrimSpace(toString(e)); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return x
	case string:
		var out []string
		for _, s := range strings.Split(x, ",") {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
