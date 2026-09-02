package main

// The fifteen tools. Read: describe, select, inspect, render, validate.
// Edit: set, add, remove, move, pose, path, undo. Document: raw, file.
// Session: preview. Parameters stay small; the form inspect returns is
// where the per-selection detail lives.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	lottie "github.com/shibukawa/lottie-go"
	lottiecp "github.com/shibukawa/lottie-go/plugin/physics/cp"
	lottieresolv "github.com/shibukawa/lottie-go/plugin/physics/resolv"
)

func (s *mcpServer) registerTools() {
	addTool(s, "describe", "Overview of the open bundle, one machine, one clip, or the focus. Start here.", false, s.describe)
	addTool(s, "select", "Move the shared focus to an address, with the side effects a click has (stage switch, parked key, opened tab). Returns the form of what is now selected.", true, s.selectTool)
	addTool(s, "inspect", "The selection as a form: fields with value, options, keyed-ness and writability. Learn what set accepts from this, not from the tool list.", false, s.inspect)
	s.registerRender()
	addTool(s, "validate", "Problems, unsupported features per clip, and name-guard findings.", false, s.validate)
	addTool(s, "set", "Write fields of the selection (or of target). Field names come from inspect; keyed fields write at the parked key.", true, s.set)
	addTool(s, "add", "Create something and select it: state, transition, guard, input, machine, clip, pose, shape_layer, shape_item, vertex, stop, texture, hitbox, span, body, socket.", true, s.add)
	addTool(s, "remove", "Delete the selection (or target). what=clip removes the stage clip, what=span the span at the playhead, what=texture unbinds the paint's texture.", true, s.remove)
	addTool(s, "move", "Reorder or retime the selection: transition order, part draw order, shape item order, key retime (what=key), span edge or shift, gradient stop position.", true, s.move)
	addTool(s, "pose", "Pose column operations on the stage clip: insert, insert_from, swap, delete, reparent, jump.", true, s.pose)
	addTool(s, "path", "Vector geometry on the selected shape item: move_vertex, set_vertex, move_handle, insert_vertex, delete_vertex, smooth, corner, close, open, pen, primitive, move_geometry, resize, set_uv, move_uv, scale_uv, seed_uv, clear_uv.", true, s.path)
	addTool(s, "undo", "Undo the last edit: what=clip (pose, shape, texture — one step per tool call) or what=machine (states, transitions, guards, inputs). Omitted: machine while the focus is on the machine, clip otherwise.", true, s.undo)
	addTool(s, "raw", "Get, put or patch (RFC 6902) the raw JSON of clip:<id>, machine:<id> or the manifest. A put that fails to decode or uses unsupported features is refused and nothing changes.", true, s.raw)
	addTool(s, "file", "new, open, save, save_as, import (a clip), reload.", true, s.file)
	addTool(s, "preview", "Drive the stage: play, pause, step, seek, fire, set_value, restart, show, autoplay.", true, s.preview)
}

// ---- describe ----

type describeArgs struct {
	Scope  string `json:"scope,omitempty" jsonschema:"bundle (default) | machine | clip | focus"`
	ID     string `json:"id,omitempty" jsonschema:"machine or clip id; defaults to the current one"`
	Detail string `json:"detail,omitempty" jsonschema:"brief (default) | full"`
}

func (s *mcpServer) describe(in describeArgs) (toolReply, error) {
	m := s.model
	full := in.Detail == "full"
	switch in.Scope {
	case "", "bundle":
		var clips []map[string]any
		for _, id := range m.AnimationIDs() {
			clips = append(clips, s.clipInfo(id, full))
		}
		var machines []map[string]any
		for _, id := range m.MachineIDs() {
			machines = append(machines, s.machineInfo(id, full))
		}
		return toolReply{payload: map[string]any{
			"path":     m.Path(),
			"viewer":   m.Viewer(),
			"clips":    clips,
			"machines": machines,
			"images":   m.Bundle().ImageNames(),
			"config":   map[string]any{"physics": m.PhysicsBackend()},
			"sockets":  s.socketRows(),
			"body":     len(m.CPBodyShapes()),
		}}, nil
	case "machine":
		id := in.ID
		if id == "" {
			id = m.MachineID()
		}
		if !hasString(m.MachineIDs(), id) {
			return toolReply{}, refuse(fmt.Sprintf("no machine %q", id), "", m.MachineIDs()...)
		}
		return toolReply{payload: map[string]any{"machine": s.machineInfo(id, true)}}, nil
	case "clip":
		id := in.ID
		if id == "" {
			id = s.stageClip()
		}
		if !hasString(m.AnimationIDs(), id) {
			return toolReply{}, refuse(fmt.Sprintf("no clip %q", id), "", m.AnimationIDs()...)
		}
		return toolReply{payload: map[string]any{"clip": s.clipInfo(id, true)}}, nil
	case "focus":
		return toolReply{payload: map[string]any{"form": s.formPayload()}}, nil
	}
	return toolReply{}, refuse(fmt.Sprintf("unknown scope %q", in.Scope), "", "bundle", "machine", "clip", "focus")
}

func (s *mcpServer) clipInfo(id string, full bool) map[string]any {
	m := s.model
	info := map[string]any{"id": id}
	anim, err := m.Bundle().Animation(id)
	if err != nil {
		info["error"] = err.Error()
		return info
	}
	w, h := anim.Size()
	fps := anim.FrameRate()
	info["size"] = []int{w, h}
	info["fps"] = fps
	info["frames"] = round2(anim.Duration().Seconds() * fps)
	markers := []map[string]any{}
	for _, mk := range anim.Markers() {
		markers = append(markers, map[string]any{"name": mk.Name, "start": mk.Start, "end": mk.End})
	}
	info["markers"] = markers
	if uf := anim.UnsupportedFeatures(); len(uf) > 0 {
		info["unsupported"] = uf
	}
	d := s.docFor(id)
	if d == nil {
		return info
	}
	info["layers"] = len(d.layers)
	info["posed"] = d.posed
	if !full {
		return info
	}
	info["pose_times"] = d.times
	var layers []map[string]any
	for i := range d.layers {
		l := &d.layers[i]
		row := map[string]any{"index": i, "name": l.name, "type": layerTypeName(l.ty)}
		if l.hasParent {
			row["parent"] = l.parent
		}
		if len(l.keyed) > 0 {
			row["keyed"] = sortedKeys(l.keyed)
		}
		layers = append(layers, row)
	}
	info["layers_list"] = layers
	var shapes []map[string]any
	for _, li := range d.shapeLayerIndices() {
		var tree []map[string]any
		for _, n := range d.shapeTree(li) {
			row := map[string]any{"path": strings.Join(pathSegs(n.path), "/"), "kind": n.ty}
			if n.name != "" {
				row["name"] = n.name
			}
			tree = append(tree, row)
		}
		shapes = append(shapes, map[string]any{"layer": li, "name": d.layers[li].name, "tree": tree})
	}
	info["shape_layers"] = shapes
	if tr, err := lottieresolv.Load(m.Bundle(), id); err == nil && tr != nil {
		var boxes []map[string]any
		for _, b := range tr.Boxes {
			boxes = append(boxes, map[string]any{"name": b.Name, "kind": string(b.Kind), "tags": b.Tags, "spans": len(b.Spans)})
		}
		info["hitboxes"] = boxes
	}
	return info
}

// docFor parses a clip for reading. The stage clip's live document is
// used so the numbers match what the panes show.
func (s *mcpServer) docFor(id string) *clipDoc {
	m := s.model
	if id == s.stageClip() {
		return m.StageClipDoc()
	}
	if d, ok := m.clipDocs[id]; ok {
		return d
	}
	data, ok := m.Bundle().AnimationJSON(id)
	if !ok {
		return nil
	}
	d, err := newClipDoc(id, data)
	if err != nil {
		return nil
	}
	return d
}

func layerTypeName(ty float64) string {
	switch ty {
	case 0:
		return "precomp"
	case 1:
		return "solid"
	case 2:
		return "image"
	case 3:
		return "null"
	case 4:
		return "shape"
	case 5:
		return "text"
	}
	return fmt.Sprint(ty)
}

func (s *mcpServer) machineInfo(id string, full bool) map[string]any {
	m := s.model
	m.syncMachine()
	sm, err := m.Bundle().StateMachine(id)
	if err != nil {
		return map[string]any{"id": id, "error": err.Error()}
	}
	info := map[string]any{"id": id, "initial": sm.Initial, "default": m.InitialMachine() == id}
	var inputs []map[string]any
	for _, in := range sm.Inputs {
		row := map[string]any{"name": in.Name, "type": string(in.Type)}
		if len(in.Value) > 0 {
			row["value"] = json.RawMessage(in.Value)
		}
		inputs = append(inputs, row)
	}
	info["inputs"] = inputs
	if !full {
		var names []string
		for _, st := range sm.States {
			names = append(names, st.Name)
		}
		info["states"] = names
		return info
	}
	var states []map[string]any
	for _, st := range sm.States {
		row := map[string]any{"name": st.Name, "type": string(st.Type)}
		if st.Type != lottie.StateGlobal {
			row["animation"] = st.Animation
			if st.Segment != "" {
				row["segment"] = st.Segment
			}
			row["loop"] = st.Loop
			row["mode"] = string(st.PlaybackMode())
			row["speed"] = st.PlaybackSpeed()
		}
		row["transitions"] = transitionRows(&st)
		states = append(states, row)
	}
	info["states"] = states
	return info
}

func (s *mcpServer) socketRows() []map[string]any {
	var out []map[string]any
	for _, sk := range s.model.Sockets() {
		out = append(out, map[string]any{"name": sk.Name, "layer": sk.LayerName()})
	}
	return out
}

// ---- select, inspect, validate ----

type selectArgs struct {
	Target string   `json:"target" jsonschema:"address, e.g. state:walk, part:forearm-near, key:12, shape:#0/0/1"`
	Frame  *float64 `json:"frame,omitempty" jsonschema:"seek the playhead to this frame after selecting"`
}

func (s *mcpServer) selectTool(in selectArgs) (toolReply, error) {
	if in.Target == "" {
		return toolReply{}, refuse("target is required", "", addressKinds...)
	}
	if err := s.selectAddress(in.Target); err != nil {
		return toolReply{}, err
	}
	if in.Frame != nil {
		s.model.PreviewSeek(*in.Frame)
	}
	return toolReply{payload: map[string]any{"form": s.formPayload()}}, nil
}

type inspectArgs struct {
	Target string `json:"target,omitempty" jsonschema:"address to inspect; selects it first. Omitted: the focus"`
}

func (s *mcpServer) inspect(in inspectArgs) (toolReply, error) {
	if in.Target != "" {
		if err := s.selectAddress(in.Target); err != nil {
			return toolReply{}, err
		}
	}
	return toolReply{payload: map[string]any{"form": s.formPayload()}}, nil
}

type validateArgs struct{}

func (s *mcpServer) validate(validateArgs) (toolReply, error) {
	return toolReply{payload: s.validation()}, nil
}

func (s *mcpServer) validation() map[string]any {
	m := s.model
	out := map[string]any{"problems": m.Problems()}
	unsupported := map[string][]string{}
	for _, id := range m.AnimationIDs() {
		if anim, err := m.Bundle().Animation(id); err != nil {
			unsupported[id] = []string{"decode: " + err.Error()}
		} else if uf := anim.UnsupportedFeatures(); len(uf) > 0 {
			unsupported[id] = uf
		}
	}
	if len(unsupported) > 0 {
		out["unsupported"] = unsupported
	}
	var guards []string
	if p := m.PosePartNameProblem(); p != "" {
		guards = append(guards, "part: "+p)
	}
	if p := m.ShapeLayerNameProblem(); p != "" {
		guards = append(guards, "shape layer: "+p)
	}
	if len(guards) > 0 {
		out["name_guards"] = guards
	}
	if errs := m.Bundle().Validate(); len(errs) > 0 {
		var msgs []string
		for _, e := range errs {
			msgs = append(msgs, e.Error())
		}
		out["bundle"] = msgs
	}
	out["ok"] = len(m.Problems()) == 0 && len(unsupported) == 0 && len(guards) == 0
	return out
}

// ---- render ----

type renderArgs struct {
	What     string   `json:"what,omitempty" jsonschema:"stage (default) | sheet | window"`
	Frame    *float64 `json:"frame,omitempty" jsonschema:"stage: draw this frame instead of the playhead"`
	Overlays bool     `json:"overlays,omitempty" jsonschema:"stage: draw the rig, pose, shape or collision overlay as the window does"`
	Samples  int      `json:"samples,omitempty" jsonschema:"sheet: frames to tile (default 4, max 16)"`
	Width    int      `json:"width,omitempty" jsonschema:"pixel width of the stage (default: the clip's own)"`
}

func (s *mcpServer) registerRender() {
	t := &mcp.Tool{
		Name:        "render",
		Description: "A PNG of the stage as the real renderer draws it (what=stage, optional frame and overlays), a contact sheet of the stage clip (what=sheet), or the whole window as the author sees it (what=window).",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}
	mcp.AddTool(s.server, t, func(ctx context.Context, _ *mcp.CallToolRequest, in renderArgs) (*mcp.CallToolResult, any, error) {
		var (
			png  []byte
			herr error
		)
		switch in.What {
		case "window":
			png, herr = s.renderWindow(ctx)
		case "", "stage":
			if err := s.call(ctx, func() { png, herr = s.renderStage(in.Width, in.Frame, in.Overlays) }); err != nil {
				return refusalResult(err), nil, nil
			}
		case "sheet":
			if err := s.call(ctx, func() { png, herr = s.renderSheet(in.Width, in.Samples) }); err != nil {
				return refusalResult(err), nil, nil
			}
		default:
			herr = refuse(fmt.Sprintf("unknown render %q", in.What), "", "stage", "sheet", "window")
		}
		if herr != nil {
			return refusalResult(herr), nil, nil
		}
		var env map[string]any
		if err := s.call(ctx, func() { env = s.envelope(map[string]any{"what": in.What}, nil) }); err != nil {
			return refusalResult(err), nil, nil
		}
		res := jsonText(env)
		res.Content = append(res.Content, &mcp.ImageContent{Data: png, MIMEType: "image/png"})
		return res, nil, nil
	})
}

// ---- set ----

type setArgs struct {
	editArgs
	Fields map[string]any `json:"fields" jsonschema:"field name to value, names as inspect lists them"`
}

func (s *mcpServer) set(in setArgs) (toolReply, error) {
	if len(in.Fields) == 0 {
		return toolReply{}, refuse("fields is empty", "inspect lists the writable fields")
	}
	changed, err := s.setFields(in.Fields)
	if err != nil {
		return toolReply{}, err
	}
	return toolReply{payload: map[string]any{"form": s.formPayload()}, changed: changed}, nil
}

// ---- add ----

type addArgs struct {
	editArgs
	Kind     string         `json:"kind" jsonschema:"state | transition | guard | input | machine | clip | pose | shape_layer | shape_item | vertex | stop | texture | hitbox | span | body | socket"`
	In       string         `json:"in,omitempty" jsonschema:"where to add: a state for transition/guard, a clip for pose/hitbox, a layer or group for shape_item"`
	Name     string         `json:"name,omitempty" jsonschema:"name or id for the new thing, where it has one"`
	Type     string         `json:"type,omitempty" jsonschema:"input: Event|Boolean|Numeric|String; hitbox: rect|circle|window; body: circle|box|polygon"`
	To       string         `json:"to,omitempty" jsonschema:"transition: target state"`
	Path     string         `json:"path,omitempty" jsonschema:"clip: file to import; texture: image file to import and bind"`
	Template string         `json:"template,omitempty" jsonschema:"clip: blank"`
	From     string         `json:"from,omitempty" jsonschema:"clip: id to duplicate; shape_item: shape address to copy; pose: clip id to borrow the pose from (with frame)"`
	JSON     any            `json:"json,omitempty" jsonschema:"clip: a whole Lottie document, validated as an import"`
	Item     string         `json:"item,omitempty" jsonschema:"shape_item: gr | sh | fl | st | gf | rc | el | sr | tm | rd"`
	Frame    *float64       `json:"frame,omitempty" jsonschema:"pose: insert at this frame (default playhead); with from, the source frame; span: start frame"`
	Swap     bool           `json:"swap,omitempty" jsonschema:"pose: trade near/far limbs as it is inserted"`
	Seg      *int           `json:"seg,omitempty" jsonschema:"vertex: segment index to split"`
	T        *float64       `json:"t,omitempty" jsonschema:"vertex: position along the segment 0..1 (default 0.5)"`
	Pos      *float64       `json:"pos,omitempty" jsonschema:"stop: ramp position 0..1"`
	Asset    string         `json:"asset,omitempty" jsonschema:"texture: bundle image to bind instead of importing"`
	Layer    string         `json:"layer,omitempty" jsonschema:"socket: layer name to bind"`
	Fields   map[string]any `json:"fields,omitempty" jsonschema:"fields to set on the new thing right away"`
}

var shapeItemKinds = []string{"gr", "sh", "fl", "st", "gf", "rc", "el", "sr", "tm", "rd"}

func (s *mcpServer) add(in addArgs) (toolReply, error) {
	m := s.model
	if in.In != "" {
		if err := s.selectAddress(in.In); err != nil {
			return toolReply{}, err
		}
	}
	before := m.DocGeneration()
	switch in.Kind {
	case "state":
		if m.Machine() == nil {
			return toolReply{}, refuse("no machine is open", "add kind=machine first")
		}
		m.AddState()
		if in.Name != "" {
			m.RenameState(m.SelectedStateName(), in.Name)
		}
	case "transition":
		if m.SelectedState() == nil {
			return toolReply{}, refuse("select a state first", "in=state:<name>", m.StateNames()...)
		}
		if in.To == "" || !hasString(m.StateNames(), in.To) {
			return toolReply{}, refuse("to must name a state", "", m.StateNames()...)
		}
		m.AddTransition(in.To)
	case "guard":
		tr := m.SelectedTransition()
		if tr == nil {
			return toolReply{}, refuse("select a transition first", "in=transition:<state>/<n>")
		}
		m.AddGuard()
		s.guard = len(m.SelectedTransition().Guards) - 1
	case "input":
		if m.Machine() == nil {
			return toolReply{}, refuse("no machine is open", "")
		}
		if !hasString(inputTypes, in.Type) {
			return toolReply{}, refuse("type is required", "", inputTypes...)
		}
		m.AddInput(lottie.InputType(in.Type))
		n := len(m.Machine().Inputs) - 1
		if in.Name != "" {
			m.RenameInput(n, in.Name)
		}
		m.SelectInput(n)
		m.setInspect(inspectState)
		s.selInput = true
	case "machine":
		m.NewMachine()
		if in.Name != "" {
			m.RenameMachine(m.MachineID(), in.Name)
		}
	case "clip":
		if err := s.addClip(in); err != nil {
			return toolReply{}, err
		}
	case "pose":
		if m.StageClipDoc() == nil {
			return toolReply{}, refuse("put a clip on stage first", "in=clip:<id>")
		}
		if in.From != "" {
			if in.Frame == nil {
				return toolReply{}, refuse("frame is the source frame when from is given", "", s.sourceKeys(in.From)...)
			}
			m.InsertPoseFrom(in.From, *in.Frame, in.Swap)
		} else {
			if in.Frame != nil {
				m.PreviewSeek(*in.Frame)
			}
			if !m.CanInsertPose() {
				return toolReply{}, refuse("cannot insert a pose here", "the playhead must be off an existing key and inside the clip", s.keyTimes(-1)...)
			}
			m.InsertPose(in.Swap)
		}
	case "shape_layer":
		if m.StageClipDoc() == nil {
			return toolReply{}, refuse("put a clip on stage first", "in=clip:<id>")
		}
		m.SetCollisionTab(colShapes)
		m.AddShapeLayerAction()
	case "shape_item":
		if m.SelectedShapeLayer() < 0 {
			return toolReply{}, refuse("select a shape layer or group first", "in=layer:<name> or in=shape:<layer>/<path>")
		}
		if in.From != "" {
			keep := s.selection()
			if err := s.selectAddress(in.From); err != nil {
				return toolReply{}, err
			}
			m.CopyShapeItem()
			if err := s.selectAddress(keep); err != nil {
				return toolReply{}, err
			}
			m.PasteShapeItem()
		} else {
			if !hasString(shapeItemKinds, in.Item) {
				return toolReply{}, refuse("item is required", "", shapeItemKinds...)
			}
			m.AddShapeItemAction(in.Item)
		}
	case "vertex":
		if in.Seg == nil {
			return toolReply{}, refuse("seg is required", "the segment index to split; t defaults to 0.5")
		}
		t := 0.5
		if in.T != nil {
			t = *in.T
		}
		m.InsertShapeVertex(*in.Seg, t)
	case "stop":
		if in.Pos == nil {
			return toolReply{}, refuse("pos is required", "ramp position 0..1")
		}
		m.AddGradStopAt(*in.Pos)
	case "texture":
		if !m.ShapeCanTexture() {
			return toolReply{}, refuse("select a fill or stroke first", "in=shape:<layer>/<path>")
		}
		switch {
		case in.Path != "":
			m.ImportTextureImage(in.Path)
			m.BindTextureFile(filepath.Base(in.Path))
		case in.Asset != "":
			m.SetShapeTexture(in.Asset)
		default:
			var ids []string
			for _, c := range m.TextureChoices() {
				ids = append(ids, c.ID)
			}
			return toolReply{}, refuse("path or asset is required", "", ids...)
		}
	case "hitbox":
		if m.StageClipDoc() == nil {
			return toolReply{}, refuse("put a clip on stage first", "in=clip:<id>")
		}
		kinds := []string{"rect", "circle", "window"}
		if !hasString(kinds, in.Type) {
			return toolReply{}, refuse("type is required", "", kinds...)
		}
		m.SetCollisionTab(colHitboxes)
		m.AddHitbox(lottieresolv.Kind(in.Type))
		if in.Name != "" {
			m.RenameHitbox(in.Name)
		}
	case "span":
		if m.SelectedHitbox() == nil {
			return toolReply{}, refuse("select a hitbox first", "in=hitbox:<name>")
		}
		if in.Frame != nil {
			m.PreviewSeek(*in.Frame)
		}
		m.AddHitboxSpan()
	case "body":
		kinds := []string{"circle", "box", "polygon"}
		if !hasString(kinds, in.Type) {
			return toolReply{}, refuse("type is required", "", kinds...)
		}
		m.SetCollisionTab(colBody)
		m.AddCPShape(lottiecp.ShapeType(in.Type))
	case "socket":
		if in.Layer == "" {
			return toolReply{}, refuse("layer is required", "", m.StageLayerNames()...)
		}
		m.SetCollisionTab(colSockets)
		m.AddSocket(in.Layer)
		if in.Name != "" {
			m.RenameSocket(in.Name)
		}
	default:
		return toolReply{}, refuse(fmt.Sprintf("unknown kind %q", in.Kind), "",
			"state", "transition", "guard", "input", "machine", "clip", "pose", "shape_layer", "shape_item", "vertex", "stop", "texture", "hitbox", "span", "body", "socket")
	}
	if m.DocGeneration() == before && in.Kind != "clip" {
		return toolReply{}, refuse("nothing was added", m.Status())
	}
	var changed []string
	if len(in.Fields) > 0 {
		c, err := s.setFields(in.Fields)
		changed = append(changed, c...)
		if err != nil {
			return toolReply{payload: map[string]any{"form": s.formPayload()}, changed: changed}, err
		}
	}
	return toolReply{payload: map[string]any{"added": s.selection(), "form": s.formPayload()}, changed: changed}, nil
}

func (s *mcpServer) sourceKeys(clip string) []string {
	var out []string
	for _, t := range s.model.PoseSourceKeys(clip) {
		out = append(out, fmtFloat(t))
	}
	return out
}

func (s *mcpServer) addClip(in addArgs) error {
	m := s.model
	switch {
	case in.Path != "":
		before := len(m.AnimationIDs())
		m.ImportClip(in.Path)
		if len(m.AnimationIDs()) == before && !strings.HasPrefix(m.Status(), "imported") {
			return refuse("import failed", m.Status())
		}
		id := strings.TrimSuffix(filepath.Base(in.Path), filepath.Ext(in.Path))
		m.ShowClip(clipRef{Anim: id})
	case in.From != "":
		if !hasString(m.AnimationIDs(), in.From) {
			return refuse(fmt.Sprintf("no clip %q", in.From), "", m.AnimationIDs()...)
		}
		if m.DuplicateClip(in.From) == "" {
			return refuse("duplicate failed", m.Status())
		}
	case in.JSON != nil:
		id := in.Name
		if id == "" {
			id = uniqueID("clip", m.AnimationIDs())
		}
		if hasString(m.AnimationIDs(), id) {
			return refuse(fmt.Sprintf("clip %q exists", id), "use raw op=put to replace it, or another name")
		}
		data, err := json.MarshalIndent(in.JSON, "", " ")
		if err != nil {
			return refuse("json is not encodable", err.Error())
		}
		if err := s.putClip(id, data, false); err != nil {
			return err
		}
		m.ShowClip(clipRef{Anim: id})
	default:
		if in.Template != "" && in.Template != "blank" {
			return refuse(fmt.Sprintf("unknown template %q", in.Template), "", "blank")
		}
		if m.NewClip() == "" {
			return refuse("could not create a clip", m.Status())
		}
	}
	if in.Name != "" && in.JSON == nil && s.stageClip() != in.Name {
		m.RenameClip(s.stageClip(), in.Name)
	}
	return nil
}

// putClip stores a clip document, keeping the old one when the new one
// does not decode or uses features the renderer skips.
func (s *mcpServer) putClip(id string, data []byte, replacing bool) error {
	m := s.model
	if m.blockEdit() {
		return refuse("viewer mode is read-only", "set config viewer=false")
	}
	old, had := m.Bundle().AnimationJSON(id)
	if replacing && !had {
		return refuse(fmt.Sprintf("no clip %q", id), "", m.AnimationIDs()...)
	}
	if replacing && id == s.stageClip() {
		m.snapshotClip()
	}
	restore := func() {
		if had {
			m.Bundle().SetAnimation(id, old)
		} else {
			m.Bundle().RemoveAnimation(id)
		}
	}
	if err := m.Bundle().SetAnimation(id, data); err != nil {
		restore()
		return refuse("the document does not decode", err.Error())
	}
	anim, err := m.Bundle().Animation(id)
	if err != nil {
		restore()
		return refuse("the document does not decode", err.Error())
	}
	if uf := anim.UnsupportedFeatures(); len(uf) > 0 {
		restore()
		return refuse("the document uses features the renderer skips; nothing was changed", "remove them and retry", uf...)
	}
	delete(m.clipDocs, id)
	m.docGen++
	m.generation++
	if id == s.stageClip() {
		m.reshowClip(m.previewClip)
	} else {
		m.restartPreview()
	}
	m.setStatus("stored clip %q", id)
	return nil
}

// ---- remove ----

type removeArgs struct {
	editArgs
	What string `json:"what,omitempty" jsonschema:"override: clip (the stage clip) | span (at the playhead) | texture (unbind) | key (the selected pose column)"`
}

func (s *mcpServer) remove(in removeArgs) (toolReply, error) {
	m := s.model
	f := s.formPayload()
	was := s.selection()
	before := m.DocGeneration()
	kind := f.Kind
	if in.What != "" {
		kind = in.What
	}
	switch kind {
	case "clip":
		id := s.stageClip()
		if id == "" {
			return toolReply{}, refuse("no clip is on stage", "target=clip:<id>")
		}
		m.RemoveClip(id)
	case "span":
		m.DeleteHitboxSpan()
	case "texture":
		m.SetShapeTexture("")
	case "key":
		if !m.CanDeletePose() {
			return toolReply{}, refuse("cannot delete this pose", "the last pose stays; select a key first")
		}
		m.DeletePose()
	case "machine":
		m.DeleteMachine(m.MachineID())
	case "state":
		m.DeleteState(m.SelectedStateName())
	case "transition":
		m.DeleteTransition(m.SelectedTransitionIndex())
	case "guard":
		m.DeleteGuard(s.guard)
		s.guard = -1
	case "input":
		m.DeleteInput(m.SelectedInputIndex())
		s.selInput = false
	case "hitbox":
		m.DeleteHitbox()
	case "body":
		m.DeleteCPShape()
	case "socket":
		m.DeleteSocket()
	case "pose":
		if m.SelectedPosePart() >= 0 {
			return toolReply{}, refuse("parts are not deletable from the pose tab", "what=key deletes the selected pose column; edit the clip JSON to drop a layer")
		}
		if !m.CanDeletePose() {
			return toolReply{}, refuse("cannot delete this pose", "the last pose stays; select a key first")
		}
		m.DeletePose()
	case "shape":
		m.DeleteShapeItemAction()
	case "layer":
		m.DeleteShapeLayerAction()
	case "vertex":
		m.DeleteShapeVertex()
	case "stop":
		m.DeleteGradStop(m.SelectedGradStop())
	case "uv":
		m.ClearShapeUV()
	default:
		return toolReply{}, refuse(fmt.Sprintf("%s cannot be removed", kind), "select something deletable, or pass what")
	}
	if m.DocGeneration() == before {
		return toolReply{}, refuse("nothing was removed", m.Status())
	}
	return toolReply{payload: map[string]any{"removed": was, "form": s.formPayload()}, changed: []string{was}}, nil
}

// ---- move ----

type moveArgs struct {
	editArgs
	What  string   `json:"what,omitempty" jsonschema:"override: key (retime the selected pose column or key) | part (draw order) | span"`
	To    *float64 `json:"to,omitempty" jsonschema:"absolute: new order index, frame, or ramp position"`
	Delta *float64 `json:"delta,omitempty" jsonschema:"relative: steps, frames, or ramp distance"`
	Edge  string   `json:"edge,omitempty" jsonschema:"span: from | to — which edge to set (with to); omitted shifts the whole span (with delta)"`
}

func (s *mcpServer) move(in moveArgs) (toolReply, error) {
	m := s.model
	f := s.formPayload()
	if in.To == nil && in.Delta == nil {
		return toolReply{}, refuse("to or delta is required", "")
	}
	before := m.DocGeneration()
	kind := f.Kind
	if in.What != "" {
		kind = in.What
	}
	delta := func(cur float64) int {
		if in.Delta != nil {
			return int(*in.Delta)
		}
		return int(*in.To - cur)
	}
	switch kind {
	case "transition":
		i := m.SelectedTransitionIndex()
		m.MoveTransition(i, delta(float64(i)))
	case "part":
		parts := m.PoseParts()
		pos := -1
		for i, l := range parts {
			if l == m.SelectedPosePart() {
				pos = i
			}
		}
		if pos < 0 {
			return toolReply{}, refuse("select a part first", "")
		}
		if in.To != nil {
			m.ReorderPosePartTo(pos, int(*in.To))
		} else {
			m.ReorderPosePart(int(*in.Delta))
		}
	case "pose", "key":
		frame, ok := m.SelectedPoseKey()
		if !ok {
			return toolReply{}, refuse("select a key first", "key:<frame>")
		}
		if kind == "pose" && m.SelectedPosePart() >= 0 && in.What == "" {
			// A part is selected: reorder it, as the Parts list would.
			return s.move(moveArgs{editArgs: in.editArgs, What: "part", To: in.To, Delta: in.Delta})
		}
		to := frame
		if in.To != nil {
			to = *in.To
		} else {
			to += *in.Delta
		}
		m.RetimePoseKey(frame, to, m.SelectedPoseRow())
	case "shape":
		n, _ := m.SelectedShapeNode()
		cur := 0
		if len(n.path) > 0 {
			cur = n.path[len(n.path)-1]
		}
		m.MoveShapeItemAction(delta(float64(cur)))
	case "hitbox", "span":
		b := m.SelectedHitbox()
		if b == nil {
			return toolReply{}, refuse("select a hitbox first", "")
		}
		span, ok := b.SpanAt(m.stageFrame())
		if !ok {
			return toolReply{}, refuse("no span at the playhead", "seek into one")
		}
		box := m.SelectedHitboxIndex()
		switch {
		case in.Edge != "" && in.To != nil:
			m.SetSpanEdge(box, span, in.Edge == "to", *in.To)
		case in.Delta != nil:
			m.ShiftSpan(box, span, *in.Delta)
		default:
			return toolReply{}, refuse("a span moves by delta, or by edge=from|to with to", "")
		}
		m.NormalizeSpans(box)
	case "stop":
		i := m.SelectedGradStop()
		stops := m.ShapeGradientStops()
		if i < 0 || i >= len(stops) {
			return toolReply{}, refuse("select a stop first", "")
		}
		pos := stops[i].pos
		if in.To != nil {
			pos = *in.To
		} else {
			pos += *in.Delta
		}
		m.SetGradStopPos(i, pos)
	default:
		return toolReply{}, refuse(fmt.Sprintf("%s cannot be moved", kind), "", "transition", "part", "key", "shape", "span", "stop")
	}
	if m.DocGeneration() == before {
		return toolReply{}, refuse("nothing moved", m.Status())
	}
	return toolReply{payload: map[string]any{"form": s.formPayload()}, changed: []string{s.selection()}}, nil
}

// ---- pose ----

type poseArgs struct {
	editArgs
	Verb   string   `json:"verb" jsonschema:"insert | insert_from | swap | delete | reparent | jump"`
	Frame  *float64 `json:"frame,omitempty" jsonschema:"insert: where (default playhead); insert_from: the source frame"`
	From   string   `json:"from,omitempty" jsonschema:"insert_from: source clip id"`
	Swap   bool     `json:"swap,omitempty" jsonschema:"insert/insert_from: trade near/far limbs"`
	Parent string   `json:"parent,omitempty" jsonschema:"reparent: layer name or #index; empty detaches"`
	Dir    int      `json:"dir,omitempty" jsonschema:"jump: -1 previous key, 1 next key"`
}

func (s *mcpServer) pose(in poseArgs) (toolReply, error) {
	m := s.model
	if m.StageClipDoc() == nil {
		return toolReply{}, refuse("put a clip on stage first", "select clip:<id>")
	}
	before := m.DocGeneration()
	switch in.Verb {
	case "insert":
		return s.add(addArgs{editArgs: in.editArgs, Kind: "pose", Frame: in.Frame, Swap: in.Swap})
	case "insert_from":
		return s.add(addArgs{editArgs: in.editArgs, Kind: "pose", From: in.From, Frame: in.Frame, Swap: in.Swap})
	case "swap":
		m.SwapPose()
	case "delete":
		if !m.CanDeletePose() {
			return toolReply{}, refuse("cannot delete this pose", "select a key first; the last pose stays")
		}
		m.DeletePose()
	case "reparent":
		if m.SelectedPosePart() < 0 {
			return toolReply{}, refuse("select a part first", "part:<layer>")
		}
		parent := -1
		if in.Parent != "" {
			i, err := s.layerArg(in.Parent, nil)
			if err != nil {
				return toolReply{}, err
			}
			parent = i
		}
		m.SetPosePartParent(parent)
	case "jump":
		if in.Dir == 0 {
			in.Dir = 1
		}
		m.JumpToKey(in.Dir)
		return toolReply{payload: map[string]any{"form": s.formPayload()}}, nil
	default:
		return toolReply{}, refuse(fmt.Sprintf("unknown verb %q", in.Verb), "", "insert", "insert_from", "swap", "delete", "reparent", "jump")
	}
	if m.DocGeneration() == before {
		return toolReply{}, refuse("nothing changed", m.Status())
	}
	return toolReply{payload: map[string]any{"form": s.formPayload()}, changed: []string{s.selection()}}, nil
}

// ---- path ----

type pathArgs struct {
	editArgs
	Verb   string       `json:"verb" jsonschema:"move_vertex | set_vertex | move_handle | insert_vertex | delete_vertex | smooth | corner | close | open | pen | primitive | move_geometry | resize | set_uv | move_uv | scale_uv | seed_uv | clear_uv"`
	Vertex *int         `json:"vertex,omitempty" jsonschema:"vertex index (default: the selected one)"`
	Delta  *[2]float64  `json:"delta,omitempty" jsonschema:"move_vertex, move_handle, move_geometry, resize, move_uv: offset"`
	Point  *[2]float64  `json:"point,omitempty" jsonschema:"set_vertex: absolute position in the layer's space"`
	Out    bool         `json:"out,omitempty" jsonschema:"move_handle: the out handle (default in)"`
	Seg    *int         `json:"seg,omitempty" jsonschema:"insert_vertex: segment to split"`
	T      *float64     `json:"t,omitempty" jsonschema:"insert_vertex: 0..1 along the segment (default 0.5)"`
	Points [][2]float64 `json:"points,omitempty" jsonschema:"pen: the vertices of a new path, in the shape layer's space"`
	Closed *bool        `json:"closed,omitempty" jsonschema:"pen: close the path (default true)"`
	Kind   string       `json:"kind,omitempty" jsonschema:"primitive: rect | ellipse | polystar"`
	At     *[2]float64  `json:"at,omitempty" jsonschema:"primitive: center in the layer's space (default origin)"`
	Size   *[2]float64  `json:"size,omitempty" jsonschema:"primitive: width and height (polystar: outer diameter)"`
	Corner *int         `json:"corner,omitempty" jsonschema:"resize: 0 top-left, 1 top-right, 2 bottom-right, 3 bottom-left"`
	Factor *float64     `json:"factor,omitempty" jsonschema:"scale_uv"`
	UV     *[2]float64  `json:"uv,omitempty" jsonschema:"set_uv: the texture coordinate"`
}

func (s *mcpServer) path(in pathArgs) (toolReply, error) {
	m := s.model
	if m.SelectedShapeLayer() < 0 {
		return toolReply{}, refuse("select a shape layer or item first", "layer:<name> or shape:<layer>/<path>")
	}
	vertex := func() (int, error) {
		if in.Vertex != nil {
			m.SelectShapeVert(*in.Vertex)
			return *in.Vertex, nil
		}
		if v := m.SelectedShapeVert(); v >= 0 {
			return v, nil
		}
		return 0, refuse("vertex is required", "an index, or select vertex:<...> first")
	}
	delta := func() ([2]float64, error) {
		if in.Delta == nil {
			return [2]float64{}, refuse("delta is required", "")
		}
		return *in.Delta, nil
	}
	needPath := func() error {
		n, ok := m.SelectedShapeNode()
		if !ok || n.ty != "sh" {
			return refuse("select a path (sh) item first", "shape:<layer>/<path>")
		}
		if !m.ShapePathWritable() {
			return refuse("the path is not writable now", "park on a key (select key:<frame>) so the edit lands there")
		}
		return nil
	}
	before := m.DocGeneration()
	m.BeginPoseEdit()
	defer m.EndPoseEdit()
	switch in.Verb {
	case "move_vertex":
		if err := needPath(); err != nil {
			return toolReply{}, err
		}
		v, err := vertex()
		if err != nil {
			return toolReply{}, err
		}
		d, err := delta()
		if err != nil {
			return toolReply{}, err
		}
		m.MoveShapeVertex(v, d[0], d[1])
	case "set_vertex":
		if err := needPath(); err != nil {
			return toolReply{}, err
		}
		if _, err := vertex(); err != nil {
			return toolReply{}, err
		}
		if in.Point == nil {
			return toolReply{}, refuse("point is required", "")
		}
		m.SetShapeVertexValue(0, in.Point[0])
		m.SetShapeVertexValue(1, in.Point[1])
	case "move_handle":
		if err := needPath(); err != nil {
			return toolReply{}, err
		}
		v, err := vertex()
		if err != nil {
			return toolReply{}, err
		}
		d, err := delta()
		if err != nil {
			return toolReply{}, err
		}
		m.MoveShapeHandle(v, in.Out, d[0], d[1])
	case "insert_vertex":
		if err := needPath(); err != nil {
			return toolReply{}, err
		}
		if in.Seg == nil {
			return toolReply{}, refuse("seg is required", "")
		}
		t := 0.5
		if in.T != nil {
			t = *in.T
		}
		m.InsertShapeVertex(*in.Seg, t)
	case "delete_vertex":
		if err := needPath(); err != nil {
			return toolReply{}, err
		}
		if _, err := vertex(); err != nil {
			return toolReply{}, err
		}
		m.DeleteShapeVertex()
	case "smooth", "corner":
		if err := needPath(); err != nil {
			return toolReply{}, err
		}
		v, err := vertex()
		if err != nil {
			return toolReply{}, err
		}
		if in.Verb == "smooth" {
			m.SmoothShapeVertex(v)
		} else {
			m.SetShapeVertexTangents(v, false)
		}
	case "close", "open":
		if err := needPath(); err != nil {
			return toolReply{}, err
		}
		m.SetShapePathClosed(in.Verb == "close")
	case "pen":
		if len(in.Points) < 2 {
			return toolReply{}, refuse("pen needs at least two points", "")
		}
		closed := true
		if in.Closed != nil {
			closed = *in.Closed
		}
		m.penPts = append([][2]float64{}, in.Points...)
		m.penActive = true
		m.CommitPen(closed)
	case "primitive":
		tool, ok := map[string]shapeTool{"rect": toolRect, "ellipse": toolEllipse, "polystar": toolStar, "star": toolStar}[in.Kind]
		if !ok {
			return toolReply{}, refuse("kind is required", "", "rect", "ellipse", "polystar")
		}
		g, okG := m.penTargetMatrix()
		if !okG {
			return toolReply{}, refuse("the shape layer cannot be placed", m.ShapeLayerNameProblem())
		}
		x, y := 0.0, 0.0
		if in.At != nil {
			x, y = in.At[0], in.At[1]
		}
		ax, ay := g.Apply(x, y)
		m.DropShapePrimitive(tool, ax, ay)
		if in.Size != nil {
			if tool == toolStar {
				m.SetShapeMemberValue("or", []float64{in.Size[0] / 2})
			} else {
				m.SetShapeMemberValue("s", []float64{in.Size[0], in.Size[1]})
			}
		}
	case "move_geometry":
		d, err := delta()
		if err != nil {
			return toolReply{}, err
		}
		m.MoveShapeGeometry(d[0], d[1])
	case "resize":
		d, err := delta()
		if err != nil {
			return toolReply{}, err
		}
		corner := 2
		if in.Corner != nil {
			corner = *in.Corner
		}
		m.ResizeShapeGeometry(corner, d[0], d[1])
	case "set_uv":
		v, err := vertex()
		if err != nil {
			return toolReply{}, err
		}
		if in.UV == nil {
			return toolReply{}, refuse("uv is required", "")
		}
		m.SetShapeUV(v, in.UV[0], in.UV[1])
	case "move_uv":
		d, err := delta()
		if err != nil {
			return toolReply{}, err
		}
		v := -1
		if in.Vertex != nil {
			v = *in.Vertex
		}
		m.MoveShapeUV(v, d[0], d[1])
	case "scale_uv":
		if in.Factor == nil {
			return toolReply{}, refuse("factor is required", "")
		}
		m.ScaleShapeUV(*in.Factor)
	case "seed_uv":
		m.SeedShapeUV()
	case "clear_uv":
		m.ClearShapeUV()
	default:
		return toolReply{}, refuse(fmt.Sprintf("unknown verb %q", in.Verb), "",
			"move_vertex", "set_vertex", "move_handle", "insert_vertex", "delete_vertex", "smooth", "corner", "close", "open", "pen", "primitive", "move_geometry", "resize", "set_uv", "move_uv", "scale_uv", "seed_uv", "clear_uv")
	}
	if m.DocGeneration() == before {
		return toolReply{}, refuse("nothing changed", m.Status())
	}
	return toolReply{payload: map[string]any{"form": s.formPayload()}, changed: []string{s.selection()}}, nil
}

// ---- undo ----

type undoArgs struct {
	What string `json:"what,omitempty" jsonschema:"clip | machine; omitted picks by the focus"`
}

func (s *mcpServer) undo(in undoArgs) (toolReply, error) {
	m := s.model
	what := in.What
	if what == "" {
		what = "clip"
		switch m.InspectTarget() {
		case inspectMachine, inspectState:
			what = "machine"
		}
	}
	switch what {
	case "clip":
		if !m.CanUndoClipEdit() {
			return toolReply{}, refuse("no clip edit to undo", "what=machine undoes machine edits", "clip", "machine")
		}
		m.UndoClipEdit()
		return toolReply{payload: map[string]any{"undid": "clip", "form": s.formPayload()}, changed: []string{addr("clip", s.stageClip())}}, nil
	case "machine":
		if !m.CanUndoMachineEdit() {
			return toolReply{}, refuse("no machine edit to undo", "what=clip undoes pose, shape and texture edits", "clip", "machine")
		}
		m.UndoMachineEdit()
		s.guard = -1
		return toolReply{payload: map[string]any{"undid": "machine", "form": s.formPayload()}, changed: []string{addr("machine", m.MachineID())}}, nil
	}
	return toolReply{}, refuse(fmt.Sprintf("unknown what %q", in.What), "", "clip", "machine")
}

// ---- raw ----

type rawArgs struct {
	Op     string    `json:"op" jsonschema:"get | put | patch"`
	Target string    `json:"target" jsonschema:"clip:<id> | machine:<id> | manifest"`
	JSON   any       `json:"json,omitempty" jsonschema:"put: the whole document"`
	Patch  []patchOp `json:"patch,omitempty" jsonschema:"patch: RFC 6902 operations"`
	Expect *int      `json:"expect_generation,omitempty"`
}

func (s *mcpServer) raw(in rawArgs) (toolReply, error) {
	m := s.model
	if in.Expect != nil && *in.Expect != m.DocGeneration() {
		return toolReply{}, refuse(fmt.Sprintf("document generation is %d, not %d", m.DocGeneration(), *in.Expect), "read again before writing")
	}
	kind, id, _ := strings.Cut(in.Target, ":")
	var current []byte
	switch kind {
	case "clip":
		data, ok := m.Bundle().AnimationJSON(id)
		if !ok {
			return toolReply{}, refuse(fmt.Sprintf("no clip %q", id), "", m.AnimationIDs()...)
		}
		current = data
	case "machine":
		m.syncMachine()
		sm, err := m.Bundle().StateMachine(id)
		if err != nil {
			return toolReply{}, refuse(fmt.Sprintf("no machine %q", id), "", m.MachineIDs()...)
		}
		current, _ = json.Marshal(sm)
	case "manifest":
		current, _ = json.Marshal(m.Bundle().Manifest())
	default:
		return toolReply{}, refuse(fmt.Sprintf("unknown target %q", in.Target), "", "clip:<id>", "machine:<id>", "manifest")
	}
	if in.Op == "get" {
		return toolReply{payload: map[string]any{"document": json.RawMessage(current)}}, nil
	}
	if kind == "manifest" {
		return toolReply{}, refuse("the manifest is read-only here", "edit machines and clips through their own targets")
	}
	var next []byte
	switch in.Op {
	case "put":
		if in.JSON == nil {
			return toolReply{}, refuse("json is required", "")
		}
		b, err := json.MarshalIndent(in.JSON, "", " ")
		if err != nil {
			return toolReply{}, refuse("json is not encodable", err.Error())
		}
		next = b
	case "patch":
		if len(in.Patch) == 0 {
			return toolReply{}, refuse("patch is required", "")
		}
		var doc any
		dec := json.NewDecoder(strings.NewReader(string(current)))
		dec.UseNumber()
		if err := dec.Decode(&doc); err != nil {
			return toolReply{}, refuse("the current document does not parse", err.Error())
		}
		patched, err := applyPatch(doc, in.Patch)
		if err != nil {
			return toolReply{}, refuse("patch failed; nothing changed", err.Error())
		}
		b, err := json.MarshalIndent(patched, "", " ")
		if err != nil {
			return toolReply{}, refuse("patched document is not encodable", err.Error())
		}
		next = b
	default:
		return toolReply{}, refuse(fmt.Sprintf("unknown op %q", in.Op), "", "get", "put", "patch")
	}
	switch kind {
	case "clip":
		if err := s.putClip(id, next, true); err != nil {
			return toolReply{}, err
		}
	case "machine":
		if m.blockEdit() {
			return toolReply{}, refuse("viewer mode is read-only", "")
		}
		sm, err := lottie.ParseStateMachine(next)
		if err != nil {
			return toolReply{}, refuse("the machine does not parse; nothing changed", err.Error())
		}
		if uf := sm.UnsupportedFeatures(); len(uf) > 0 {
			return toolReply{}, refuse("the machine uses unsupported features; nothing changed", "", uf...)
		}
		if err := m.Bundle().SetStateMachine(id, sm); err != nil {
			return toolReply{}, refuse("cannot store the machine", err.Error())
		}
		if id == m.MachineID() {
			m.SelectMachine(id)
		}
		m.docGen++
		m.generation++
		m.restartPreview()
	}
	return toolReply{payload: map[string]any{"stored": in.Target}, changed: []string{in.Target}}, nil
}

// ---- file ----

type fileArgs struct {
	Op       string `json:"op" jsonschema:"new | open | save | save_as | import | reload"`
	Path     string `json:"path,omitempty" jsonschema:"file path; save uses the open path when omitted"`
	Template string `json:"template,omitempty" jsonschema:"new: a preset template name, or empty for a blank bundle"`
}

func (s *mcpServer) file(in fileArgs) (toolReply, error) {
	m := s.model
	switch in.Op {
	case "new":
		if in.Path == "" {
			return toolReply{}, refuse("path is required", "where the new bundle will be saved")
		}
		if in.Template != "" {
			data, err := templateBytes(in.Template)
			if err != nil {
				return toolReply{}, refuse(fmt.Sprintf("unknown template %q", in.Template), "", templateNames()...)
			}
			if err := os.WriteFile(in.Path, data, 0o644); err != nil {
				return toolReply{}, refuse("cannot write the template", err.Error())
			}
			m.Open(in.Path)
		} else {
			m.StartNewAt(in.Path)
		}
	case "open":
		if in.Path == "" {
			return toolReply{}, refuse("path is required", "")
		}
		m.Open(in.Path)
		if m.Path() != in.Path && !strings.EqualFold(filepath.Ext(in.Path), ".json") {
			return toolReply{}, refuse("open failed", m.Status())
		}
	case "save", "save_as":
		if m.Viewer() {
			return toolReply{}, refuse("viewer mode: saving is disabled", "set config viewer=false")
		}
		path := in.Path
		if path == "" {
			path = m.Path()
		}
		if path == "" {
			return toolReply{}, refuse("path is required for an unsaved bundle", "")
		}
		m.Save(path)
		if !strings.HasPrefix(m.Status(), "saved") {
			return toolReply{}, refuse("save failed", m.Status())
		}
	case "import":
		return s.add(addArgs{Kind: "clip", Path: in.Path})
	case "reload":
		m.Reload()
	default:
		return toolReply{}, refuse(fmt.Sprintf("unknown op %q", in.Op), "", "new", "open", "save", "save_as", "import", "reload")
	}
	return toolReply{payload: map[string]any{"path": m.Path()}, changed: []string{"file"}}, nil
}

// ---- preview ----

type previewArgs struct {
	Verb   string   `json:"verb" jsonschema:"play | pause | step | seek | fire | set_value | restart | show | autoplay"`
	Frame  *float64 `json:"frame,omitempty" jsonschema:"seek: absolute frame"`
	Delta  *float64 `json:"delta,omitempty" jsonschema:"step: frames (default 1)"`
	Input  string   `json:"input,omitempty" jsonschema:"fire / set_value: input name"`
	Value  any      `json:"value,omitempty" jsonschema:"set_value: bool, number or string per the input's type"`
	Target string   `json:"target,omitempty" jsonschema:"show: clip:<id> or machine:<id>"`
	On     *bool    `json:"on,omitempty" jsonschema:"autoplay: the new setting"`
}

func (s *mcpServer) preview(in previewArgs) (toolReply, error) {
	m := s.model
	switch in.Verb {
	case "play":
		if !m.PreviewPlaying() {
			m.TogglePreviewPlaying()
		}
	case "pause":
		m.PausePreview()
	case "step":
		d := 1.0
		if in.Delta != nil {
			d = *in.Delta
		}
		m.StepPreviewFrame(d)
	case "seek":
		if in.Frame == nil {
			return toolReply{}, refuse("frame is required", "")
		}
		m.PreviewSeek(*in.Frame)
	case "fire":
		if !hasString(m.EventInputs(), in.Input) {
			return toolReply{}, refuse("input must name an Event input", "", m.EventInputs()...)
		}
		if m.Preview() == nil || s.stageClip() != "" {
			return toolReply{}, refuse("the machine is not on stage", "preview verb=show target=machine:<id>")
		}
		m.Fire(in.Input)
		m.PreviewUpdate()
	case "set_value":
		sm := m.Machine()
		if sm == nil {
			return toolReply{}, refuse("no machine is open", "")
		}
		def, ok := sm.Input(in.Input)
		if !ok || def.Type == lottie.InputEvent {
			return toolReply{}, refuse("input must name a Boolean, Numeric or String input", "", m.InputNames()...)
		}
		switch def.Type {
		case "Boolean":
			SetInputValue(m, in.Input, toBool(in.Value))
		case "Numeric":
			SetInputValue(m, in.Input, toFloat(in.Value))
		default:
			SetInputValue(m, in.Input, toString(in.Value))
		}
		m.generation++
	case "restart":
		m.RestartPreview()
	case "show":
		kind, id, _ := strings.Cut(in.Target, ":")
		switch kind {
		case "clip":
			if _, err := s.clipArg(address{segs: []string{id}}, 0, true); err != nil {
				return toolReply{}, err
			}
			m.ShowClip(clipRef{Anim: id})
		case "machine":
			if id != "" && id != m.MachineID() {
				if _, err := s.machineArg(address{segs: []string{id}}, 0); err != nil {
					return toolReply{}, err
				}
				m.SelectMachine(id)
			}
			m.ShowMachine()
		default:
			return toolReply{}, refuse("target must be clip:<id> or machine:<id>", "")
		}
	case "autoplay":
		if in.On == nil {
			return toolReply{}, refuse("on is required", "")
		}
		m.SetAutoPlay(*in.On)
	default:
		return toolReply{}, refuse(fmt.Sprintf("unknown verb %q", in.Verb), "", "play", "pause", "step", "seek", "fire", "set_value", "restart", "show", "autoplay")
	}
	payload := map[string]any{}
	if p := m.Preview(); p != nil && s.stageClip() == "" {
		payload["state"] = m.ActiveState()
	}
	return toolReply{payload: payload}, nil
}

// unused guard against import pruning while the tool set grows.
var _ = strconv.Itoa
