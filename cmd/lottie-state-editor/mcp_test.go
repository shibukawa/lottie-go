package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcpHarness connects an in-memory client to a server over m. A goroutine
// stands in for Root.Tick, draining the call queue the way the game loop
// would.
type mcpHarness struct {
	t    *testing.T
	m    *Model
	s    *mcpServer
	cs   *mcp.ClientSession
	ctx  context.Context
	stop context.CancelFunc
}

func newMCPHarness(t *testing.T, m *Model) *mcpHarness {
	t.Helper()
	s := newMCPServer(m)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case f := <-s.queue:
				f()
			}
		}
	}()
	st, ct := mcp.NewInMemoryTransports()
	if _, err := s.server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	h := &mcpHarness{t: t, m: m, s: s, cs: cs, ctx: ctx, stop: cancel}
	t.Cleanup(func() { cs.Close(); cancel() })
	return h
}

// call invokes a tool and decodes the JSON text reply.
func (h *mcpHarness) call(name string, args map[string]any) (map[string]any, *mcp.CallToolResult) {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(h.ctx, 10*time.Second)
	defer cancel()
	res, err := h.cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		h.t.Fatalf("%s: protocol error: %v", name, err)
	}
	if len(res.Content) == 0 {
		h.t.Fatalf("%s: empty reply", name)
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		h.t.Fatalf("%s: first content is %T", name, res.Content[0])
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		h.t.Fatalf("%s: reply is not JSON: %v\n%s", name, err, text.Text)
	}
	return out, res
}

// ok calls a tool that must succeed.
func (h *mcpHarness) ok(name string, args map[string]any) map[string]any {
	h.t.Helper()
	out, res := h.call(name, args)
	if res.IsError {
		h.t.Fatalf("%s %v: refused: %v", name, args, out)
	}
	return out
}

// refused calls a tool that must be refused, returning the refusal.
func (h *mcpHarness) refused(name string, args map[string]any) map[string]any {
	h.t.Helper()
	out, res := h.call(name, args)
	if !res.IsError {
		h.t.Fatalf("%s %v: expected a refusal, got %v", name, args, out)
	}
	return out
}

func focusOf(out map[string]any) map[string]any {
	f, _ := out["focus"].(map[string]any)
	return f
}

func formOf(out map[string]any) map[string]any {
	f, _ := out["form"].(map[string]any)
	return f
}

func fieldOf(form map[string]any, name string) map[string]any {
	fields, _ := form["fields"].([]any)
	for _, f := range fields {
		fm, _ := f.(map[string]any)
		if fm["name"] == name {
			return fm
		}
	}
	return nil
}

func TestMCPToolSetIsFifteen(t *testing.T) {
	h := newMCPHarness(t, NewModel())
	res, err := h.cs.ListTools(h.ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	if len(names) != 15 {
		t.Fatalf("expected 15 tools, got %d: %v", len(names), names)
	}
	for _, want := range []string{"describe", "select", "inspect", "render", "validate", "set", "add", "remove", "move", "pose", "path", "undo", "raw", "file", "preview"} {
		if !hasString(names, want) {
			t.Errorf("missing tool %q in %v", want, names)
		}
	}
	// The set tool inlines editArgs, so target and expect_generation are
	// ordinary properties an agent can pass.
	for _, tool := range res.Tools {
		if tool.Name != "set" {
			continue
		}
		schema, _ := json.Marshal(tool.InputSchema)
		for _, prop := range []string{`"target"`, `"expect_generation"`, `"fields"`} {
			if !strings.Contains(string(schema), prop) {
				t.Errorf("set schema lacks %s: %s", prop, schema)
			}
		}
	}
}

func TestMCPDescribeSelectInspectSet(t *testing.T) {
	m := openSample(t, "character", "character.lottie")
	h := newMCPHarness(t, m)

	out := h.ok("describe", nil)
	if machines, _ := out["machines"].([]any); len(machines) != 1 {
		t.Fatalf("describe: machines = %v", out["machines"])
	}
	if clips, _ := out["clips"].([]any); len(clips) != 5 {
		t.Fatalf("describe: clips = %v", out["clips"])
	}

	out = h.ok("select", map[string]any{"target": "state:walk-state"})
	if got := focusOf(out)["selection"]; got != "state:character/walk-state" {
		t.Fatalf("focus after select = %v", got)
	}
	form := formOf(out)
	if form["kind"] != "state" {
		t.Fatalf("form kind = %v", form["kind"])
	}
	speed := fieldOf(form, "speed")
	if speed == nil || speed["writable"] != true {
		t.Fatalf("speed field = %v", speed)
	}

	out = h.ok("set", map[string]any{"fields": map[string]any{"speed": 1.5, "loop": true}})
	if st, _ := m.Machine().State("walk-state"); st == nil || st.Speed != 1.5 || !st.Loop {
		t.Fatalf("set did not reach the machine: %+v", st)
	}
	if changed, _ := out["changed"].([]any); len(changed) != 2 {
		t.Fatalf("changed = %v", out["changed"])
	}

	ref := h.refused("set", map[string]any{"fields": map[string]any{"nonsense": 1}})
	if valid, _ := ref["valid"].([]any); len(valid) == 0 {
		t.Fatalf("refusal carries no valid fields: %v", ref)
	}
	ref = h.refused("set", map[string]any{"fields": map[string]any{"mode": "Sideways"}})
	if !strings.Contains(ref["error"].(string), "Sideways") {
		t.Fatalf("enum refusal = %v", ref)
	}
}

func TestMCPAddsStatesTransitionsGuardsInputs(t *testing.T) {
	m := openSample(t, "character", "character.lottie")
	h := newMCPHarness(t, m)

	out := h.ok("add", map[string]any{"kind": "state", "name": "dash-state", "fields": map[string]any{"animation": "run-anim", "speed": 2}})
	if out["added"] != "state:character/dash-state" {
		t.Fatalf("added = %v", out["added"])
	}
	st, ok := m.Machine().State("dash-state")
	if !ok || st.Animation != "run-anim" || st.Speed != 2 {
		t.Fatalf("state not created as asked: %+v", st)
	}

	h.ok("add", map[string]any{"kind": "input", "type": "Event", "name": "dash"})
	if _, ok := m.Machine().Input("dash"); !ok {
		t.Fatalf("input not added: %v", m.InputNames())
	}

	out = h.ok("add", map[string]any{"kind": "transition", "in": "state:idle-state", "to": "dash-state"})
	if !strings.HasPrefix(out["added"].(string), "transition:character/idle-state/") {
		t.Fatalf("added = %v", out["added"])
	}
	out = h.ok("add", map[string]any{"kind": "guard", "fields": map[string]any{"type": "Event", "inputName": "dash"}})
	if !strings.HasPrefix(out["added"].(string), "guard:character/idle-state/") {
		t.Fatalf("added = %v", out["added"])
	}
	idle, _ := m.Machine().State("idle-state")
	last := idle.Transitions[len(idle.Transitions)-1]
	if last.ToState != "dash-state" || len(last.Guards) != 1 || last.Guards[0].InputName != "dash" {
		t.Fatalf("transition not wired: %+v", last)
	}

	// Order is semantic; move it to the front.
	n := len(idle.Transitions) - 1
	h.ok("move", map[string]any{"target": "transition:idle-state/" + itoa(n), "to": 0})
	if idle.Transitions[0].ToState != "dash-state" {
		t.Fatalf("move did not reorder: %v", idle.Transitions[0])
	}

	// The wiring works through the real interpreter.
	h.ok("preview", map[string]any{"verb": "show", "target": "machine:character"})
	h.ok("preview", map[string]any{"verb": "fire", "input": "dash"})
	fire(t, m, "dash", "dash-state", 5)

	h.ok("remove", map[string]any{"target": "state:dash-state"})
	if _, ok := m.Machine().State("dash-state"); ok {
		t.Fatalf("state not removed")
	}

	// Undo picks the machine stack while the focus is on the machine.
	out = h.ok("undo", nil)
	if out["undid"] != "machine" {
		t.Fatalf("undid = %v", out["undid"])
	}
	if _, ok := m.Machine().State("dash-state"); !ok {
		t.Fatalf("undo did not bring the state back")
	}
	h.ok("undo", map[string]any{"what": "machine"})
	h.refused("undo", map[string]any{"what": "clip"})
}

func itoa(i int) string { return fmtFloat(float64(i)) }

func TestMCPExpectGenerationRefusesAfterHumanEdit(t *testing.T) {
	m := openSample(t, "character", "character.lottie")
	h := newMCPHarness(t, m)
	out := h.ok("inspect", map[string]any{"target": "state:walk-state"})
	gen := int(out["generation"].(float64))

	// The author edits in between.
	m.AddState()

	ref := h.refused("set", map[string]any{"expect_generation": gen, "fields": map[string]any{"speed": 3}})
	if !strings.Contains(ref["error"].(string), "generation") {
		t.Fatalf("refusal = %v", ref)
	}
	st, _ := m.Machine().State("walk-state")
	if st.Speed == 3 {
		t.Fatalf("the refused write landed")
	}
	h.ok("set", map[string]any{"target": "state:walk-state", "expect_generation": m.DocGeneration(), "fields": map[string]any{"speed": 3}})
	if st.Speed != 3 {
		t.Fatalf("the accepted write did not land")
	}
}

func TestMCPRawPutRefusesBadDocumentsAndKeepsTheOld(t *testing.T) {
	m := openSample(t, "character", "character.lottie")
	h := newMCPHarness(t, m)
	before, _ := m.Bundle().AnimationJSON("idle-anim")

	ref := h.refused("raw", map[string]any{"op": "put", "target": "clip:idle-anim", "json": map[string]any{"v": "5.7.1"}})
	if ref["error"] == nil {
		t.Fatalf("no error text: %v", ref)
	}
	after, _ := m.Bundle().AnimationJSON("idle-anim")
	if string(before) != string(after) {
		t.Fatalf("a refused put changed the clip")
	}

	// A patch that adds a marker lands, and get reads it back.
	h.ok("raw", map[string]any{"op": "patch", "target": "clip:idle-anim", "patch": []map[string]any{
		{"op": "add", "path": "/markers", "value": []any{map[string]any{"cm": "half", "tm": 5, "dr": 5}}},
	}})
	out := h.ok("raw", map[string]any{"op": "get", "target": "clip:idle-anim"})
	doc, _ := out["document"].(map[string]any)
	if markers, _ := doc["markers"].([]any); len(markers) != 1 {
		t.Fatalf("patched marker missing: %v", doc["markers"])
	}
	if !hasString(m.Markers("idle-anim"), "half") {
		t.Fatalf("the model does not see the patched marker: %v", m.Markers("idle-anim"))
	}

	// Machines go the same way, and a broken one is refused whole.
	h.refused("raw", map[string]any{"op": "put", "target": "machine:character", "json": map[string]any{"initial": "nowhere", "states": "not a list"}})
	if m.Machine().Initial != "idle-state" {
		t.Fatalf("machine changed by a refused put")
	}
}

func TestMCPDrawsAVectorClipFromNothing(t *testing.T) {
	m := openSample(t, "character", "character.lottie")
	h := newMCPHarness(t, m)

	out := h.ok("add", map[string]any{"kind": "clip", "template": "blank", "name": "badge"})
	if !strings.HasPrefix(focusOf(out)["stage"].(string), "clip:badge") {
		t.Fatalf("blank clip not on stage: %v", focusOf(out))
	}
	form := formOf(out)
	if form["kind"] != "layer" {
		t.Fatalf("form after blank clip = %v", form["kind"])
	}

	out = h.ok("path", map[string]any{"verb": "pen", "points": [][2]float64{{-40, -40}, {40, -40}, {40, 40}, {-40, 40}}})
	sel := focusOf(out)["selection"].(string)
	if !strings.HasPrefix(sel, "shape:badge/") {
		t.Fatalf("pen did not select the new group: %v", sel)
	}
	tree, _ := formOf(out)["extra"].(map[string]any)["tree"].([]any)
	var kinds []string
	for _, row := range tree {
		kinds = append(kinds, row.(map[string]any)["kind"].(string))
	}
	if !hasString(kinds, "sh") || !hasString(kinds, "fl") {
		t.Fatalf("pen tree = %v", kinds)
	}

	// Recolor the fill: the fill is the second child of the pen's group.
	out = h.ok("select", map[string]any{"target": "shape:badge/#0/0/1"})
	if formOf(out)["kind"] != "shape" || fieldOf(formOf(out), "c") == nil {
		t.Fatalf("fill form = %v", formOf(out))
	}
	h.ok("set", map[string]any{"fields": map[string]any{"c": "#ff0000", "o": 80}})
	if hex, _ := m.ShapeColorHex(); hex != "#ff0000" {
		t.Fatalf("fill color = %v", hex)
	}

	// A primitive beside it, then a gradient stop on a fresh gradient fill.
	h.ok("path", map[string]any{"verb": "primitive", "kind": "ellipse", "at": [2]float64{60, 0}, "size": [2]float64{30, 30}})
	if v, ok := m.ShapeMemberValue("s"); !ok || len(v) < 2 || v[0] != 30 {
		t.Fatalf("ellipse size = %v", v)
	}
	out = h.ok("add", map[string]any{"kind": "shape_item", "in": "layer:badge/#0", "item": "gf"})
	stops, _ := formOf(out)["extra"].(map[string]any)["stops"].([]any)
	if len(stops) < 2 {
		t.Fatalf("gradient stops = %v", stops)
	}
	h.ok("add", map[string]any{"kind": "stop", "pos": 0.5})
	if len(m.ShapeGradientStops()) != len(stops)+1 {
		t.Fatalf("stop not added")
	}

	// Everything the agent did stays inside what the renderer draws.
	out = h.ok("validate", nil)
	if out["ok"] != true {
		t.Fatalf("validate = %v", out)
	}

	// Undo peels the last clip edit.
	h.ok("undo", nil)
	if len(m.ShapeGradientStops()) != len(stops) {
		t.Fatalf("undo did not remove the stop")
	}
}

func TestMCPPoseEditingParksOnAKey(t *testing.T) {
	m := NewModel()
	m.Open("../../examples/state-editor/presets/chibi-male/chibi-male.lottie")
	if m.Machine() == nil {
		t.Skipf("preset not loadable: %s", m.Status())
	}
	h := newMCPHarness(t, m)

	out := h.ok("describe", map[string]any{"scope": "clip", "id": "idle-anim", "detail": "full"})
	clip := out["clip"].(map[string]any)
	times, _ := clip["pose_times"].([]any)
	if len(times) < 2 {
		t.Fatalf("idle-anim pose_times = %v", times)
	}
	second := times[1].(float64)

	// Off a key, values are read-only; the refusal says how to park.
	out = h.ok("select", map[string]any{"target": "part:idle-anim/forearm-near"})
	if fieldOf(formOf(out), "r")["writable"] == true {
		t.Fatalf("rotation writable while not parked: %v", formOf(out))
	}
	ref := h.refused("set", map[string]any{"fields": map[string]any{"r": 12}})
	if !strings.Contains(ref["hint"].(string), "key") {
		t.Fatalf("hint = %v", ref["hint"])
	}

	out = h.ok("select", map[string]any{"target": "key:" + fmtFloat(second)})
	if k := focusOf(out)["key"]; k != second {
		t.Fatalf("focus key = %v, want %v", k, second)
	}
	h.ok("select", map[string]any{"target": "part:forearm-near"})
	out = h.ok("set", map[string]any{"fields": map[string]any{"r": 12}})
	if v, _ := m.PoseValue("r"); len(v) == 0 || v[0] != 12 {
		t.Fatalf("rotation after set = %v", v)
	}
	if fieldOf(formOf(out), "r")["keyed"] != true {
		t.Fatalf("rotation not reported keyed after the write")
	}

	// A pose column op through the pose tool.
	h.ok("pose", map[string]any{"verb": "jump", "dir": -1})
	if f, _ := m.SelectedPoseKey(); f != times[0].(float64) {
		t.Fatalf("jump landed on %v", f)
	}
}

func TestMCPResourcesAndFocus(t *testing.T) {
	m := openSample(t, "character", "character.lottie")
	h := newMCPHarness(t, m)
	h.ok("select", map[string]any{"target": "clip:walk-anim"})
	res, err := h.cs.ReadResource(h.ctx, &mcp.ReadResourceParams{URI: "lottie://focus"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Contents[0].Text, `"stage":"clip:walk-anim"`) {
		t.Fatalf("focus resource = %s", res.Contents[0].Text)
	}
	res, err = h.cs.ReadResource(h.ctx, &mcp.ReadResourceParams{URI: "lottie://clip/walk-anim.json"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Contents[0].Text, `"layers"`) {
		t.Fatalf("clip resource = %.80s", res.Contents[0].Text)
	}
	if _, err := h.cs.ReadResource(h.ctx, &mcp.ReadResourceParams{URI: "lottie://clip/nope.json"}); err == nil {
		t.Fatalf("missing clip read succeeded")
	}
}

func TestMCPAddressRefusalsCarryCandidates(t *testing.T) {
	m := openSample(t, "character", "character.lottie")
	h := newMCPHarness(t, m)
	ref := h.refused("select", map[string]any{"target": "state:flying"})
	valid, _ := ref["valid"].([]any)
	if len(valid) == 0 || !strings.Contains(ref["error"].(string), "flying") {
		t.Fatalf("refusal = %v", ref)
	}
	ref = h.refused("select", map[string]any{"target": "nonsense:x"})
	if valid, _ := ref["valid"].([]any); len(valid) != len(addressKinds) {
		t.Fatalf("kinds refusal = %v", ref)
	}
	ref = h.refused("select", map[string]any{"target": "key:walk-anim/999"})
	if valid, _ := ref["valid"].([]any); len(valid) == 0 {
		t.Fatalf("key refusal lists no key times: %v", ref)
	}
}

func TestMCPHitboxesAndSockets(t *testing.T) {
	m := openSample(t, "character", "character.lottie")
	h := newMCPHarness(t, m)
	out := h.ok("add", map[string]any{"kind": "hitbox", "in": "clip:idle-anim", "type": "rect", "name": "hurt"})
	if out["added"] != "hitbox:idle-anim/hurt" {
		t.Fatalf("added = %v", out["added"])
	}
	form := formOf(out)
	if fieldOf(form, "from") == nil {
		t.Fatalf("a new hitbox has no span at the playhead: %v", form)
	}
	h.ok("set", map[string]any{"fields": map[string]any{"tags": []string{"hurt", "push"}, "x": 10, "y": 20, "w": 30, "h": 40}})
	b := m.SelectedHitbox()
	sp := m.SelectedSpan()
	if b == nil || sp == nil || len(b.Tags) != 2 || sp.X != 10 || sp.W != 30 {
		t.Fatalf("hitbox after set: %+v %+v", b, sp)
	}

	layers := m.StageLayerNames()
	if len(layers) == 0 {
		t.Skip("sample has no named layers for a socket")
	}
	out = h.ok("add", map[string]any{"kind": "socket", "layer": layers[0], "name": "hand"})
	if out["added"] != "socket:hand" {
		t.Fatalf("added = %v", out["added"])
	}
	h.ok("set", map[string]any{"fields": map[string]any{"dx": 3, "behind": true}})
	if sk := m.SelectedSocket(); sk == nil || sk.DX != 3 || sk.Z != "behind" {
		t.Fatalf("socket after set: %+v", sk)
	}
}

func TestJSONPatchOps(t *testing.T) {
	doc := map[string]any{"a": []any{1.0, 2.0}, "b": map[string]any{"c": "x"}}
	out, err := applyPatch(doc, []patchOp{
		{Op: "add", Path: "/a/1", Value: 9.0},
		{Op: "replace", Path: "/b/c", Value: "y"},
		{Op: "copy", From: "/b/c", Path: "/d"},
		{Op: "move", From: "/a/0", Path: "/a/-"},
		{Op: "test", Path: "/d", Value: "y"},
		{Op: "remove", Path: "/b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(out)
	if string(b) != `{"a":[9,2,1],"d":"y"}` {
		t.Fatalf("patched = %s", b)
	}
	if _, err := applyPatch(doc, []patchOp{{Op: "test", Path: "/d", Value: "z"}}); err == nil {
		t.Fatalf("failed test op passed")
	}
}

// TestMCPLiveRender talks to a running editor (go run . -mcp 127.0.0.1:7391
// some.lottie) when LSM_MCP_URL is set, and checks that render returns an
// image — the one tool that needs the real game loop.
func TestMCPLiveRender(t *testing.T) {
	url := os.Getenv("LSM_MCP_URL")
	if url == "" {
		t.Skip("LSM_MCP_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "live-test", Version: "0"}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	for _, what := range []string{"stage", "sheet", "window"} {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "render", Arguments: map[string]any{"what": what, "overlays": true, "width": 300}})
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if res.IsError {
			t.Fatalf("%s refused: %v", what, res.Content[0])
		}
		var img *mcp.ImageContent
		for _, c := range res.Content {
			if ic, ok := c.(*mcp.ImageContent); ok {
				img = ic
			}
		}
		if img == nil || len(img.Data) < 100 || img.MIMEType != "image/png" {
			t.Fatalf("%s: no image in reply", what)
		}
		if err := os.WriteFile(os.Getenv("LSM_MCP_OUT")+"/render-"+what+".png", img.Data, 0o644); err != nil {
			t.Logf("could not save %s: %v", what, err)
		}
	}
}
