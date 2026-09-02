package main

import (
	"fmt"
	"image"
	"image/color"
	"slices"
	"strconv"
	"strings"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// The shape pane of the inspector: the selected item's values, stored at
// the selected key or clip-wide when static, exactly as the pose pane
// shows transforms. Per kind the pane is mostly a table of numeric rows;
// fills and strokes add a color row, gradients add the stop ramp, and a
// path adds its selected vertex.

// shapeField is one editable number of an item kind.
type shapeField struct {
	label  string
	member string
	comp   int
	dim    int
}

var shapeFieldsByKind = map[string][]shapeField{
	"fl": {{"opacity %", "o", 0, 1}},
	"st": {{"opacity %", "o", 0, 1}, {"width", "w", 0, 1}},
	"gf": {
		{"opacity %", "o", 0, 1},
		{"start x", "s", 0, 2}, {"start y", "s", 1, 2},
		{"end x", "e", 0, 2}, {"end y", "e", 1, 2},
		{"highlight len", "h", 0, 1}, {"highlight °", "a", 0, 1},
	},
	"gs": {
		{"opacity %", "o", 0, 1}, {"width", "w", 0, 1},
		{"start x", "s", 0, 2}, {"start y", "s", 1, 2},
		{"end x", "e", 0, 2}, {"end y", "e", 1, 2},
	},
	"rc": {
		{"center x", "p", 0, 2}, {"center y", "p", 1, 2},
		{"width", "s", 0, 2}, {"height", "s", 1, 2},
		{"roundness", "r", 0, 1},
	},
	"el": {
		{"center x", "p", 0, 2}, {"center y", "p", 1, 2},
		{"width", "s", 0, 2}, {"height", "s", 1, 2},
	},
	"sr": {
		{"points", "pt", 0, 1},
		{"outer r", "or", 0, 1}, {"inner r", "ir", 0, 1},
		{"outer round", "os", 0, 1}, {"inner round", "is", 0, 1},
		{"rotation °", "r", 0, 1},
		{"center x", "p", 0, 2}, {"center y", "p", 1, 2},
	},
	"tr": {
		{"position x", "p", 0, 2}, {"position y", "p", 1, 2},
		{"anchor x", "a", 0, 2}, {"anchor y", "a", 1, 2},
		{"scale x %", "s", 0, 2}, {"scale y %", "s", 1, 2},
		{"rotation °", "r", 0, 1}, {"opacity %", "o", 0, 1},
	},
	"tm": {{"start %", "s", 0, 1}, {"end %", "e", 0, 1}, {"offset °", "o", 0, 1}},
	"rd": {{"radius", "r", 0, 1}},
	"rp": {{"copies", "c", 0, 1}, {"offset", "o", 0, 1}},
	"op": {{"amount", "a", 0, 1}},
	"pb": {{"amount", "a", 0, 1}},
	"zz": {{"size", "s", 0, 1}, {"ridges", "r", 0, 1}},
}

// vertex rows edit the selected vertex through SetShapeVertexValue.
var shapeVertexFields = []struct {
	label string
	comp  int
}{
	{"vertex x", 0}, {"vertex y", 1},
	{"in dx", 2}, {"in dy", 3},
	{"out dx", 4}, {"out dy", 5},
}

type shapeInspector struct {
	guigui.DefaultWidget

	// The layer picker and the item tree head the pane, like the Parts
	// list heads the pose pane: the strip under the stage has no height to
	// spare, and the tree is what most selection starts from.
	treeTitle     basicwidget.Text
	layerSel      basicwidget.Select[int]
	layerItems    []basicwidget.SelectItem[int]
	addLayerBtn   basicwidget.Button
	delLayerBtn   basicwidget.Button
	treeList      basicwidget.List[int]
	treeItems     []basicwidget.ListItem[int]
	treeShown     int // last row scrolled to, plus one
	addGrBtn      basicwidget.Button
	addFlBtn      basicwidget.Button
	addStBtn      basicwidget.Button
	addGfBtn      basicwidget.Button
	addTmBtn      basicwidget.Button
	addRdBtn      basicwidget.Button
	addShBtn      basicwidget.Button
	addRcBtn      basicwidget.Button
	addElBtn      basicwidget.Button
	addSrBtn      basicwidget.Button
	frontBtn      basicwidget.Button
	backBtn       basicwidget.Button
	delItemBtn    basicwidget.Button
	copyBtn       basicwidget.Button
	pasteBtn      basicwidget.Button
	dupBtn        basicwidget.Button
	clipRow       guigui.LinearLayout
	clipRowItems  []guigui.LinearLayoutItem
	layerRow      guigui.LinearLayout
	layerRowItems []guigui.LinearLayoutItem
	addRow1       guigui.LinearLayout
	addRow1Items  []guigui.LinearLayoutItem
	addRow2       guigui.LinearLayout
	addRow2Items  []guigui.LinearLayoutItem
	geomRow       guigui.LinearLayout
	geomRowItems  []guigui.LinearLayoutItem
	moveRow       guigui.LinearLayout
	moveRowItems  []guigui.LinearLayoutItem

	title      basicwidget.Text
	nameLabel  basicwidget.Text
	nameInput  basicwidget.TextInput
	frameLabel basicwidget.Text
	frameValue basicwidget.Text

	colorLabel basicwidget.Text
	colorRow   colorFieldRow

	gradTypeLabel basicwidget.Text
	gradTypeSel   basicwidget.Select[bool]
	gradItems     []basicwidget.SelectItem[bool]
	rampLabel     basicwidget.Text
	ramp          gradRampView
	stopColorLbl  basicwidget.Text
	stopColorRow  colorFieldRow
	stopDel       basicwidget.Button

	closedLabel basicwidget.Text
	closedCheck basicwidget.Checkbox

	// The vertex list edits a path from the sidebar: every vertex a row,
	// insertion and deletion beside it, the selected one's numbers below.
	vertList     basicwidget.List[int]
	vertItems    []basicwidget.ListItem[int]
	vertShown    int // last row scrolled to, plus one
	vertInsBtn   basicwidget.Button
	vertDelBtn   basicwidget.Button
	vertBtnRow   guigui.LinearLayout
	vertBtnItems []guigui.LinearLayoutItem
	showVerts    bool
	tangentLbl   basicwidget.Text
	tangentChk   basicwidget.Checkbox

	capLabel  basicwidget.Text
	capSel    basicwidget.Select[int]
	joinLabel basicwidget.Text
	joinSel   basicwidget.Select[int]
	capItems  []basicwidget.SelectItem[int]
	joinItems []basicwidget.SelectItem[int]

	// The texture section of a fill or stroke and the UV pane of a path
	// (textureui.go).
	texLabel       basicwidget.Text
	texSel         basicwidget.Select[string]
	texItems       []basicwidget.SelectItem[string]
	texMapLabel    basicwidget.Text
	texMapSel      basicwidget.Select[string]
	texMapItems    []basicwidget.SelectItem[string]
	texWrapLabel   basicwidget.Text
	texWrapSel     basicwidget.Select[string]
	texWrapItems   []basicwidget.SelectItem[string]
	texFilterLabel basicwidget.Text
	texFilterSel   basicwidget.Select[string]
	texFilterItems []basicwidget.SelectItem[string]
	texTintLabel   basicwidget.Text
	texTintChk     basicwidget.Checkbox
	texLabels      guigui.WidgetSlice[*basicwidget.Text]
	texRows        guigui.WidgetSlice[*poseFieldRow]
	uvPane         uvPaneView
	uvSeedBtn      basicwidget.Button
	uvClearBtn     basicwidget.Button
	uvBtnRow       guigui.LinearLayout
	uvBtnItems     []guigui.LinearLayoutItem
	showUV         bool
	unplacedLabel  basicwidget.Text
	unplacedText   basicwidget.Text
	unplacedDrop   basicwidget.Button

	numLabels  guigui.WidgetSlice[*basicwidget.Text]
	numRows    guigui.WidgetSlice[*poseFieldRow]
	vertLabels guigui.WidgetSlice[*basicwidget.Text]
	vertRows   guigui.WidgetSlice[*poseFieldRow]

	undoBtn basicwidget.Button
	hint    basicwidget.Text

	form      basicwidget.Form
	formItems []basicwidget.FormItem
	items     []guigui.LinearLayoutItem
	tabFields []*basicwidget.TextInput
}

func (p *shapeInspector) model(context *guigui.Context) *Model {
	v, ok := context.Env(p, envKeyModel)
	if !ok {
		return nil
	}
	m, _ := v.(*Model)
	return m
}

func (p *shapeInspector) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m := p.model(context)
	if m == nil {
		return nil
	}
	adder.AddWidget(&p.treeTitle)
	adder.AddWidget(&p.layerSel)
	adder.AddWidget(&p.addLayerBtn)
	adder.AddWidget(&p.delLayerBtn)
	adder.AddWidget(&p.treeList)
	for _, w := range []guigui.Widget{
		&p.addGrBtn, &p.addFlBtn, &p.addStBtn, &p.addGfBtn, &p.addTmBtn, &p.addRdBtn,
		&p.addShBtn, &p.addRcBtn, &p.addElBtn, &p.addSrBtn,
		&p.frontBtn, &p.backBtn, &p.delItemBtn,
		&p.copyBtn, &p.pasteBtn, &p.dupBtn,
	} {
		adder.AddWidget(w)
	}
	p.buildTreeSection(context, m)

	adder.AddWidget(&p.title)
	adder.AddWidget(&p.form)
	adder.AddWidget(&p.undoBtn)
	adder.AddWidget(&p.hint)

	n, hasSel := m.SelectedShapeNode()
	setBold(&p.title, "Shape")
	if hasSel {
		setBold(&p.title, "Shape · "+shapeItemLabel(n.ty))
	}

	p.formItems = slices.Delete(p.formItems, 0, len(p.formItems))
	p.tabFields = p.tabFields[:0]
	p.showVerts = false
	editable := hasSel && !m.Viewer()

	label(&p.nameLabel, "name")
	p.nameInput.SetValue(n.name)
	p.nameInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed {
			m.RenameShapeItem(text)
		}
	})
	context.SetEnabled(&p.nameInput, editable)
	p.tabFields = append(p.tabFields, &p.nameInput)

	label(&p.frameLabel, "frame")
	if frame, ok := m.SelectedPoseKey(); ok {
		p.frameValue.SetValue(strconv.FormatFloat(frame, 'f', -1, 64))
	} else {
		p.frameValue.SetValue("— (static edits only)")
	}
	p.frameValue.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	p.formItems = append(p.formItems,
		basicwidget.FormItem{PrimaryWidget: &p.nameLabel, SecondaryWidget: &p.nameInput},
		basicwidget.FormItem{PrimaryWidget: &p.frameLabel, SecondaryWidget: &p.frameValue},
	)

	p.showUV = false
	if hasSel {
		switch {
		case n.ty == "fl" || n.ty == "st":
			p.buildColorRow(context, m, adder, editable)
			p.buildTextureRows(context, m, adder, editable)
		case n.ty == "gf" || n.ty == "gs":
			p.buildGradientRows(context, m, adder, editable)
		}
		if n.ty == "st" || n.ty == "gs" {
			p.buildStrokeStyleRows(context, m, adder, editable)
		}
		if n.ty == "sh" {
			p.buildPathRows(context, m, adder, editable)
			p.buildUVRows(context, m, adder, editable)
		}
		p.buildNumericRows(context, m, adder, n.ty, editable)
	}
	p.buildUnplacedRows(context, m, adder, !m.Viewer())

	p.undoBtn.SetText("Undo shape edit")
	p.undoBtn.OnDown(func(context *guigui.Context) { m.UndoClipEdit() })
	context.SetEnabled(&p.undoBtn, m.CanUndoClipEdit() && !m.Viewer())

	p.hint.SetValue(p.hintText(m))
	p.hint.SetMultiline(true)
	p.hint.SetWrapMode(basicwidget.WrapModeNormal)
	p.hint.SetScale(0.85)

	tabOrder(context, p.tabFields...)
	p.form.SetItems(p.formItems)
	return nil
}

// buildTreeSection wires the layer picker, the item tree and the
// structure buttons that head the pane.
func (p *shapeInspector) buildTreeSection(context *guigui.Context, m *Model) {
	setBold(&p.treeTitle, "Shapes")
	editable := m.StageClipDoc() != nil && !m.Viewer()

	layers := m.ShapeLayers()
	p.layerItems = p.layerItems[:0]
	d := m.StageClipDoc()
	for _, i := range layers {
		name := ""
		if d != nil {
			name = d.layers[i].name
		}
		if name == "" {
			name = "(unnamed layer)"
		}
		p.layerItems = append(p.layerItems, basicwidget.SelectItem[int]{Text: name, Value: i})
	}
	p.layerSel.SetItems(p.layerItems)
	if sel := m.SelectedShapeLayer(); sel >= 0 {
		p.layerSel.SelectItemByValue(sel)
	}
	p.layerSel.OnItemSelected(func(context *guigui.Context, index int) {
		if it, ok := p.layerSel.ItemByIndex(index); ok && it.Value != m.SelectedShapeLayer() {
			m.SelectShapeLayer(it.Value)
		}
	})
	context.SetEnabled(&p.layerSel, len(layers) > 0)

	p.addLayerBtn.SetText("+Layer")
	p.addLayerBtn.OnDown(func(context *guigui.Context) { m.AddShapeLayerAction() })
	context.SetEnabled(&p.addLayerBtn, editable)
	p.delLayerBtn.SetText("−Layer")
	p.delLayerBtn.OnDown(func(context *guigui.Context) { m.DeleteShapeLayerAction() })
	context.SetEnabled(&p.delLayerBtn, editable && m.SelectedShapeLayer() >= 0)

	// The tree, indented by depth, selection shared with the stage both
	// ways.
	nodes := m.ShapeNodes()
	p.treeItems = p.treeItems[:0]
	selRow := -1
	sel, hasSel := m.SelectedShapeNode()
	for i, n := range nodes {
		indent := ""
		for range n.depth {
			indent += "   "
		}
		text := shapeItemLabel(n.ty)
		if n.name != "" {
			text = n.name + " · " + text
		}
		p.treeItems = append(p.treeItems, basicwidget.ListItem[int]{
			Text: indent + text, Value: i,
		})
		if hasSel && slicesEqualInt(n.path, sel.path) {
			selRow = i
		}
	}
	p.treeList.SetItems(p.treeItems)
	if selRow >= 0 {
		p.treeList.SelectItemByValue(selRow)
		if p.treeShown != selRow+1 {
			p.treeList.EnsureItemVisibleByIndex(selRow)
			p.treeShown = selRow + 1
		}
	} else {
		p.treeShown = 0
	}
	p.treeList.OnItemSelected(func(context *guigui.Context, index int) {
		if index < 0 || index >= len(nodes) {
			return
		}
		if sel, ok := m.SelectedShapeNode(); ok && slicesEqualInt(nodes[index].path, sel.path) {
			return
		}
		m.SelectShapeNode(nodes[index].path)
	})

	// New style items join the selected item's group; geometry comes from
	// the stage tools, where it lands where it is clicked.
	add := func(btn *basicwidget.Button, text, kind string) {
		btn.SetText(text)
		btn.OnDown(func(context *guigui.Context) { m.AddShapeItemAction(kind) })
		context.SetEnabled(btn, editable && m.SelectedShapeLayer() >= 0)
	}
	add(&p.addGrBtn, "+Group", "gr")
	add(&p.addFlBtn, "+Fill", "fl")
	add(&p.addStBtn, "+Stroke", "st")
	add(&p.addGfBtn, "+Grad", "gf")
	add(&p.addTmBtn, "+Trim", "tm")
	add(&p.addRdBtn, "+Round", "rd")
	// Geometry inserts here too: pick the place in the tree and drop the
	// shape into it, then move and resize it on the stage. The stage tools
	// remain for placing by click.
	add(&p.addShBtn, "+Path", "sh")
	add(&p.addRcBtn, "+Rect", "rc")
	add(&p.addElBtn, "+Ellipse", "el")
	add(&p.addSrBtn, "+Star", "sr")

	p.frontBtn.SetText("▲ Front")
	p.frontBtn.OnDown(func(context *guigui.Context) { m.MoveShapeItemAction(-1) })
	p.backBtn.SetText("▼ Back")
	p.backBtn.OnDown(func(context *guigui.Context) { m.MoveShapeItemAction(1) })
	p.delItemBtn.SetText("Delete")
	p.delItemBtn.OnDown(func(context *guigui.Context) { m.DeleteShapeItemAction() })
	for _, w := range []guigui.Widget{&p.frontBtn, &p.backBtn, &p.delItemBtn} {
		context.SetEnabled(w, editable && hasSel)
	}

	// The clipboard is the editor's own and lives for the session, so a
	// copied group pastes into another layer — or another clip.
	p.copyBtn.SetText("Copy")
	p.copyBtn.OnDown(func(context *guigui.Context) { m.CopyShapeItem() })
	context.SetEnabled(&p.copyBtn, hasSel)
	p.pasteBtn.SetText("Paste")
	p.pasteBtn.OnDown(func(context *guigui.Context) { m.PasteShapeItem() })
	context.SetEnabled(&p.pasteBtn, editable && m.CanPasteShapeItem() && m.SelectedShapeLayer() >= 0)
	p.dupBtn.SetText("Duplicate")
	p.dupBtn.OnDown(func(context *guigui.Context) { m.DuplicateShapeItem() })
	context.SetEnabled(&p.dupBtn, editable && hasSel)
}

func (p *shapeInspector) hintText(m *Model) string {
	switch {
	case m.StageClipDoc() == nil:
		return "No clip on stage."
	case m.SelectedShapeLayer() < 0:
		return "Pick a shape layer above the tree, or +Layer to start one."
	case m.ShapeLayerNameProblem() != "":
		return "Cannot edit on the stage: " + m.ShapeLayerNameProblem() +
			". Rename the layer; typed values still work."
	}
	if m.PenActive() {
		return "Click to add points. Click the first point or right-click to " +
			"close the path; Finish commits it open."
	}
	if _, ok := m.SelectedShapeNode(); !ok {
		return "Click a shape on the stage or a row in the tree. The Pen and " +
			"primitive tools draw into the selected layer."
	}
	if !shapeEditLive(m) {
		return "This value animates: click a key on the chart to park on it; " +
			"values are only written at a key."
	}
	if m.ShapeItemIsGradient() {
		return "Drag the square (start), circle (end) or diamond (both) on the " +
			"stage. Click under the ramp to add a stop; drag a stop off the bar to delete it."
	}
	if m.ShapeTexGizmoActive() {
		return "Drag the square (texture origin) or the circle (its u axis: " +
			"rotation and scale) on the stage; the rows below type the same placement."
	}
	if m.ShapeUVEditable() {
		return "Drag a UV point in the pane, or empty space to move them all; " +
			"the wheel scales the set. Seed projects the path's box onto the texture."
	}
	return "Drag inside the shape to move it, a box corner to resize it. On a " +
		"path, vertices drag too; the selected one shows its bezier handles, " +
		"and Smooth/Corner toggles the tangents."
}

// buildColorRow is the solid color of a fill or stroke.
func (p *shapeInspector) buildColorRow(context *guigui.Context, m *Model, adder *guigui.ChildAdder, editable bool) {
	adder.AddWidget(&p.colorRow)
	label(&p.colorLabel, "color")
	hex, _ := m.ShapeColorHex()
	p.colorRow.set(hex, editable && m.ShapeMemberWritable("c"), func(context *guigui.Context, text string) {
		m.SetShapeColorHex(text)
	})
	context.SetEnabled(&p.colorRow, editable && m.ShapeMemberWritable("c"))
	p.tabFields = append(p.tabFields, &p.colorRow.text)
	p.formItems = append(p.formItems,
		basicwidget.FormItem{PrimaryWidget: &p.colorLabel, SecondaryWidget: &p.colorRow})
}

// buildGradientRows is the Flash pair: type, ramp, and the selected stop.
func (p *shapeInspector) buildGradientRows(context *guigui.Context, m *Model, adder *guigui.ChildAdder, editable bool) {
	adder.AddWidget(&p.gradTypeSel)
	adder.AddWidget(&p.ramp)
	adder.AddWidget(&p.stopColorRow)
	adder.AddWidget(&p.stopDel)

	label(&p.gradTypeLabel, "type")
	p.gradItems = p.gradItems[:0]
	p.gradItems = append(p.gradItems,
		basicwidget.SelectItem[bool]{Text: "linear", Value: false},
		basicwidget.SelectItem[bool]{Text: "radial", Value: true},
	)
	p.gradTypeSel.SetItems(p.gradItems)
	p.gradTypeSel.SelectItemByValue(m.ShapeGradientRadial())
	p.gradTypeSel.OnItemSelected(func(context *guigui.Context, index int) {
		if it, ok := p.gradTypeSel.ItemByIndex(index); ok {
			m.SetShapeGradientRadial(it.Value)
		}
	})
	context.SetEnabled(&p.gradTypeSel, editable)

	label(&p.rampLabel, "ramp")
	context.SetEnabled(&p.ramp, editable && m.ShapeMemberWritable("o"))

	label(&p.stopColorLbl, "stop color")
	hex, hasStop := m.GradStopColorHex()
	p.stopColorRow.set(hex, editable && hasStop, func(context *guigui.Context, text string) {
		m.SetGradStopColorHex(text)
	})
	context.SetEnabled(&p.stopColorRow, editable && hasStop)
	p.tabFields = append(p.tabFields, &p.stopColorRow.text)

	p.stopDel.SetText("Delete stop")
	p.stopDel.OnDown(func(context *guigui.Context) { m.DeleteGradStop(m.SelectedGradStop()) })
	context.SetEnabled(&p.stopDel, editable && hasStop && len(m.ShapeGradientStops()) > 2)

	p.formItems = append(p.formItems,
		basicwidget.FormItem{PrimaryWidget: &p.gradTypeLabel, SecondaryWidget: &p.gradTypeSel},
		basicwidget.FormItem{PrimaryWidget: &p.rampLabel, SecondaryWidget: &p.ramp},
		basicwidget.FormItem{PrimaryWidget: &p.stopColorLbl, SecondaryWidget: &p.stopColorRow},
		basicwidget.FormItem{SecondaryWidget: &p.stopDel},
	)
}

// buildStrokeStyleRows are the cap and join pickers of st and gs.
func (p *shapeInspector) buildStrokeStyleRows(context *guigui.Context, m *Model, adder *guigui.ChildAdder, editable bool) {
	adder.AddWidget(&p.capSel)
	adder.AddWidget(&p.joinSel)
	label(&p.capLabel, "cap")
	label(&p.joinLabel, "join")

	p.capItems = p.capItems[:0]
	p.capItems = append(p.capItems,
		basicwidget.SelectItem[int]{Text: "butt", Value: 1},
		basicwidget.SelectItem[int]{Text: "round", Value: 2},
		basicwidget.SelectItem[int]{Text: "square", Value: 3},
	)
	p.capSel.SetItems(p.capItems)
	if v, ok := m.ShapePlainInt("lc"); ok {
		p.capSel.SelectItemByValue(v)
	}
	p.capSel.OnItemSelected(func(context *guigui.Context, index int) {
		if it, ok := p.capSel.ItemByIndex(index); ok {
			m.SetShapePlainInt("lc", it.Value)
		}
	})

	p.joinItems = p.joinItems[:0]
	p.joinItems = append(p.joinItems,
		basicwidget.SelectItem[int]{Text: "miter", Value: 1},
		basicwidget.SelectItem[int]{Text: "round", Value: 2},
		basicwidget.SelectItem[int]{Text: "bevel", Value: 3},
	)
	p.joinSel.SetItems(p.joinItems)
	if v, ok := m.ShapePlainInt("lj"); ok {
		p.joinSel.SelectItemByValue(v)
	}
	p.joinSel.OnItemSelected(func(context *guigui.Context, index int) {
		if it, ok := p.joinSel.ItemByIndex(index); ok {
			m.SetShapePlainInt("lj", it.Value)
		}
	})
	for _, w := range []guigui.Widget{&p.capSel, &p.joinSel} {
		context.SetEnabled(w, editable)
	}
	p.formItems = append(p.formItems,
		basicwidget.FormItem{PrimaryWidget: &p.capLabel, SecondaryWidget: &p.capSel},
		basicwidget.FormItem{PrimaryWidget: &p.joinLabel, SecondaryWidget: &p.joinSel},
	)
}

// buildPathRows edit a path from the sidebar: the closure, the vertex
// list with insert and delete beside it, and the selected vertex's
// numbers — coordinates always, the handle vectors while handles are on.
func (p *shapeInspector) buildPathRows(context *guigui.Context, m *Model, adder *guigui.ChildAdder, editable bool) {
	adder.AddWidget(&p.closedCheck)
	adder.AddWidget(&p.vertList)
	adder.AddWidget(&p.vertInsBtn)
	adder.AddWidget(&p.vertDelBtn)
	adder.AddWidget(&p.tangentChk)
	p.showVerts = true

	label(&p.closedLabel, "closed")
	p.closedCheck.SetValue(m.ShapePathClosed())
	p.closedCheck.OnValueChanged(func(context *guigui.Context, v bool) {
		m.SetShapePathClosed(v)
	})
	context.SetEnabled(&p.closedCheck, editable)

	writable := m.ShapePathWritable() && !m.Viewer()
	sel := m.SelectedShapeVert()
	pd, okPath := m.ShapePath()
	hasVert := sel >= 0 && okPath && sel < len(pd.v)

	// Every vertex is a row, selected in step with the stage both ways.
	p.vertItems = p.vertItems[:0]
	if okPath {
		for i, v := range pd.v {
			kind := "corner"
			if pd.i[i] != [2]float64{} || pd.o[i] != [2]float64{} {
				kind = "smooth"
			}
			p.vertItems = append(p.vertItems, basicwidget.ListItem[int]{
				Text: fmt.Sprintf("%d  (%s, %s)", i+1,
					strconv.FormatFloat(v[0], 'g', -1, 64),
					strconv.FormatFloat(v[1], 'g', -1, 64)),
				KeyText: kind, Value: i,
			})
		}
	}
	p.vertList.SetItems(p.vertItems)
	if hasVert {
		p.vertList.SelectItemByValue(sel)
		if p.vertShown != sel+1 {
			p.vertList.EnsureItemVisibleByIndex(sel)
			p.vertShown = sel + 1
		}
	} else {
		p.vertShown = 0
	}
	p.vertList.OnItemSelected(func(context *guigui.Context, index int) {
		if index >= 0 && index < len(p.vertItems) && index != m.SelectedShapeVert() {
			m.SelectShapeVert(index)
		}
	})

	// Insert splits the segment leaving the selected vertex at its
	// midpoint, on every key at once, so the shape looks unchanged until
	// the new vertex is dragged.
	segs := 0
	if okPath {
		segs = len(pd.v) - 1
		if pd.closed {
			segs = len(pd.v)
		}
	}
	p.vertInsBtn.SetText("+Insert after")
	p.vertInsBtn.OnDown(func(context *guigui.Context) {
		m.InsertShapeVertex(m.SelectedShapeVert(), 0.5)
	})
	context.SetEnabled(&p.vertInsBtn, editable && hasVert && sel < segs)
	p.vertDelBtn.SetText("Delete")
	p.vertDelBtn.OnDown(func(context *guigui.Context) { m.DeleteShapeVertex() })
	context.SetEnabled(&p.vertDelBtn, editable && hasVert && okPath && len(pd.v) > 2)

	// Handles on is a smooth vertex, off a corner; the vector fields below
	// follow the switch.
	label(&p.tangentLbl, "handles")
	tangents := hasVert && m.ShapeVertexHasTangents(sel)
	p.tangentChk.SetValue(tangents)
	p.tangentChk.OnValueChanged(func(context *guigui.Context, v bool) {
		m.SetShapeVertexTangents(m.SelectedShapeVert(), v)
	})
	context.SetEnabled(&p.tangentChk, writable && hasVert)

	p.formItems = append(p.formItems,
		basicwidget.FormItem{PrimaryWidget: &p.closedLabel, SecondaryWidget: &p.closedCheck},
		basicwidget.FormItem{PrimaryWidget: &p.tangentLbl, SecondaryWidget: &p.tangentChk},
	)

	p.vertLabels.SetLen(len(shapeVertexFields))
	p.vertRows.SetLen(len(shapeVertexFields))
	for i, f := range shapeVertexFields {
		lb, row := p.vertLabels.At(i), p.vertRows.At(i)
		adder.AddWidget(lb)
		adder.AddWidget(row)
		label(lb, f.label)
		in := &row.text
		val, cur := "", 0.0
		if hasVert {
			cur, _ = vertexComp(pd, sel, f.comp)
			val = strconv.FormatFloat(cur, 'g', -1, 64)
		}
		in.SetValue(val)
		in.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
			if !committed {
				return
			}
			if v, err := strconv.ParseFloat(strings.TrimSpace(text), 64); err == nil {
				m.SetShapeVertexValue(f.comp, v)
			}
		})
		// The handle vectors only mean something while handles are on.
		rowOn := writable && hasVert && (f.comp < 2 || tangents)
		context.SetEnabled(in, rowOn)
		p.tabFields = append(p.tabFields, in)
		wireAdjacentCopy(context, row, rowOn, cur,
			func(dir int) (float64, float64, bool) { return m.ShapeVertexAdjacentValue(f.comp, dir) },
			func(dir int) { m.CopyShapeVertexFromAdjacent(f.comp, dir) })
		p.formItems = append(p.formItems, basicwidget.FormItem{PrimaryWidget: lb, SecondaryWidget: row})
	}
}

// wireAdjacentCopy hooks one row's prev/next buttons to a neighbour-key
// source, the way the pose fields copy — enabled exactly where the frame
// differs from that neighbour.
func wireAdjacentCopy(context *guigui.Context, row *poseFieldRow, writable bool, cur float64,
	read func(dir int) (float64, float64, bool), copyFn func(dir int)) {
	for dir, btn := range map[int]*basicwidget.Button{-1: &row.prev, +1: &row.next} {
		tip := &row.prevTip
		side := "previous"
		if dir > 0 {
			tip = &row.nextTip
			side = "next"
		}
		adj, at, adjOK := read(dir)
		btn.OnDown(func(context *guigui.Context) { copyFn(dir) })
		context.SetEnabled(btn, writable && adjOK && adj != cur)
		if adjOK {
			tip.SetText(fmt.Sprintf("Copy from the %s key (frame %s)",
				side, strconv.FormatFloat(at, 'f', -1, 64)))
		} else {
			tip.SetText("No " + side + " key to copy from")
		}
	}
}

// buildNumericRows are the generic {a, k} members of the item kind. Each
// row carries the pose pane's neighbour-copy buttons: a shape parameter,
// like a pose, is mostly its value at the key next door with a nudge.
func (p *shapeInspector) buildNumericRows(context *guigui.Context, m *Model, adder *guigui.ChildAdder, kind string, editable bool) {
	fields := shapeFieldsByKind[kind]
	p.numLabels.SetLen(len(fields))
	p.numRows.SetLen(len(fields))
	for i, f := range fields {
		lb, row := p.numLabels.At(i), p.numRows.At(i)
		adder.AddWidget(lb)
		adder.AddWidget(row)
		label(lb, f.label)
		in := &row.text
		v, ok := m.ShapeMemberValue(f.member)
		writable := editable && ok && len(v) >= f.dim && m.ShapeMemberWritable(f.member)
		cur := 0.0
		if ok && len(v) > f.comp {
			cur = v[f.comp]
			in.SetValue(strconv.FormatFloat(cur, 'g', -1, 64))
		} else {
			in.SetValue("")
		}
		in.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
			if !committed {
				return
			}
			if n, err := strconv.ParseFloat(strings.TrimSpace(text), 64); err == nil {
				m.SetShapeMemberComponent(f.member, f.comp, n)
			}
		})
		context.SetEnabled(in, writable)
		p.tabFields = append(p.tabFields, in)
		wireAdjacentCopy(context, row, writable, cur,
			func(dir int) (float64, float64, bool) {
				adj, at, ok := m.ShapeAdjacentValue(f.member, dir)
				if !ok || len(adj) <= f.comp {
					return 0, 0, false
				}
				return adj[f.comp], at, true
			},
			func(dir int) { m.CopyShapeValueFromAdjacent(f.member, f.comp, dir) })
		p.formItems = append(p.formItems, basicwidget.FormItem{PrimaryWidget: lb, SecondaryWidget: row})
	}
}

func (p *shapeInspector) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	if m := p.model(context); m != nil {
		w.WriteInt(m.Generation())
	}
}

func (p *shapeInspector) layout(context *guigui.Context) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)

	p.layerRowItems = slices.Delete(p.layerRowItems, 0, len(p.layerRowItems))
	p.layerRowItems = append(p.layerRowItems,
		guigui.LinearLayoutItem{Widget: &p.layerSel, Size: guigui.FlexibleSize(2)},
		guigui.LinearLayoutItem{Widget: &p.addLayerBtn, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &p.delLayerBtn, Size: guigui.FlexibleSize(1)},
	)
	p.layerRow = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal, Items: p.layerRowItems, Gap: u / 4,
	}

	p.addRow1Items = slices.Delete(p.addRow1Items, 0, len(p.addRow1Items))
	p.addRow1Items = append(p.addRow1Items,
		guigui.LinearLayoutItem{Widget: &p.addGrBtn, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &p.addFlBtn, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &p.addStBtn, Size: guigui.FlexibleSize(1)},
	)
	p.addRow1 = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal, Items: p.addRow1Items, Gap: u / 4,
	}
	p.addRow2Items = slices.Delete(p.addRow2Items, 0, len(p.addRow2Items))
	p.addRow2Items = append(p.addRow2Items,
		guigui.LinearLayoutItem{Widget: &p.addGfBtn, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &p.addTmBtn, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &p.addRdBtn, Size: guigui.FlexibleSize(1)},
	)
	p.addRow2 = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal, Items: p.addRow2Items, Gap: u / 4,
	}
	p.geomRowItems = slices.Delete(p.geomRowItems, 0, len(p.geomRowItems))
	p.geomRowItems = append(p.geomRowItems,
		guigui.LinearLayoutItem{Widget: &p.addShBtn, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &p.addRcBtn, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &p.addElBtn, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &p.addSrBtn, Size: guigui.FlexibleSize(1)},
	)
	p.geomRow = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal, Items: p.geomRowItems, Gap: u / 4,
	}

	p.moveRowItems = slices.Delete(p.moveRowItems, 0, len(p.moveRowItems))
	p.moveRowItems = append(p.moveRowItems,
		guigui.LinearLayoutItem{Widget: &p.frontBtn, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &p.backBtn, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &p.delItemBtn, Size: guigui.FlexibleSize(1)},
	)
	p.moveRow = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal, Items: p.moveRowItems, Gap: u / 4,
	}

	p.clipRowItems = slices.Delete(p.clipRowItems, 0, len(p.clipRowItems))
	p.clipRowItems = append(p.clipRowItems,
		guigui.LinearLayoutItem{Widget: &p.copyBtn, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &p.pasteBtn, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &p.dupBtn, Size: guigui.FlexibleSize(1)},
	)
	p.clipRow = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal, Items: p.clipRowItems, Gap: u / 4,
	}

	p.items = slices.Delete(p.items, 0, len(p.items))
	p.items = append(p.items,
		guigui.LinearLayoutItem{Widget: &p.treeTitle, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &p.layerRow},
		guigui.LinearLayoutItem{Widget: &p.treeList, Size: guigui.FixedSize(6 * u)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &p.moveRow},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &p.clipRow},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &p.geomRow},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &p.addRow1},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &p.addRow2},
		guigui.LinearLayoutItem{Widget: &p.title, Size: guigui.FixedSize(u)},
	)
	if p.showVerts {
		p.vertBtnItems = slices.Delete(p.vertBtnItems, 0, len(p.vertBtnItems))
		p.vertBtnItems = append(p.vertBtnItems,
			guigui.LinearLayoutItem{Widget: &p.vertInsBtn, Size: guigui.FlexibleSize(1)},
			guigui.LinearLayoutItem{Widget: &p.vertDelBtn, Size: guigui.FlexibleSize(1)},
		)
		p.vertBtnRow = guigui.LinearLayout{
			Direction: guigui.LayoutDirectionHorizontal, Items: p.vertBtnItems, Gap: u / 4,
		}
		p.items = append(p.items,
			guigui.LinearLayoutItem{Widget: &p.vertList, Size: guigui.FixedSize(4 * u)},
			guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &p.vertBtnRow},
		)
	}
	if p.showUV {
		p.uvBtnItems = slices.Delete(p.uvBtnItems, 0, len(p.uvBtnItems))
		p.uvBtnItems = append(p.uvBtnItems,
			guigui.LinearLayoutItem{Widget: &p.uvSeedBtn, Size: guigui.FlexibleSize(1)},
			guigui.LinearLayoutItem{Widget: &p.uvClearBtn, Size: guigui.FlexibleSize(1)},
		)
		p.uvBtnRow = guigui.LinearLayout{
			Direction: guigui.LayoutDirectionHorizontal, Items: p.uvBtnItems, Gap: u / 4,
		}
		p.items = append(p.items,
			guigui.LinearLayoutItem{Widget: &p.uvPane, Size: guigui.FixedSize(8 * u)},
			guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &p.uvBtnRow},
		)
	}
	p.items = append(p.items,
		guigui.LinearLayoutItem{Widget: &p.form},
		guigui.LinearLayoutItem{Widget: &p.undoBtn, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &p.hint, Size: guigui.FixedSize(3 * u)},
	)
	return guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical, Items: p.items, Gap: u / 4,
	}
}

func (p *shapeInspector) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	p.layout(context).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (p *shapeInspector) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return p.layout(context).Measure(context, constraints)
}

// ---- color row: hex input plus a live swatch ----

// colorFieldRow pairs the hex field with a swatch that shows the color it
// names, so a typo reads as the wrong color and not just the wrong text.
type colorFieldRow struct {
	guigui.DefaultWidget

	text    basicwidget.TextInput
	hex     string
	onEdit  func(*guigui.Context, string)
	items   []guigui.LinearLayoutItem
	swatch  colorSwatch
	enabled bool
}

func (r *colorFieldRow) set(hex string, enabled bool, onEdit func(*guigui.Context, string)) {
	r.hex, r.enabled, r.onEdit = hex, enabled, onEdit
	r.swatch.hex = hex
}

func (r *colorFieldRow) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&r.text)
	adder.AddWidget(&r.swatch)
	r.text.SetValue(r.hex)
	r.text.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed && r.onEdit != nil {
			r.onEdit(context, text)
		}
	})
	context.SetEnabled(&r.text, r.enabled)
	return nil
}

func (r *colorFieldRow) layout(context *guigui.Context) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)
	r.items = slices.Delete(r.items, 0, len(r.items))
	r.items = append(r.items,
		guigui.LinearLayoutItem{Widget: &r.text, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &r.swatch, Size: guigui.FixedSize(3 * u / 2)},
	)
	return guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal, Items: r.items, Gap: u / 8,
	}
}

func (r *colorFieldRow) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	r.layout(context).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (r *colorFieldRow) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return r.text.Measure(context, constraints)
}

type colorSwatch struct {
	guigui.DefaultWidget
	hex string
}

func (s *colorSwatch) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	r, g, bl, ok := hexToRGB(s.hex)
	if !ok {
		return
	}
	c := color.NRGBA{uint8(r * 255), uint8(g * 255), uint8(bl * 255), 0xff}
	vector.DrawFilledRect(dst, float32(b.Min.X), float32(b.Min.Y)+2,
		float32(b.Dx()), float32(b.Dy())-4, c, false)
	vector.StrokeRect(dst, float32(b.Min.X), float32(b.Min.Y)+2,
		float32(b.Dx()), float32(b.Dy())-4, 1, color.NRGBA{0x40, 0x40, 0x48, 0xff}, false)
}

// ---- the gradient ramp ----

// gradRampView is the Flash color-panel bar: the gradient drawn across it,
// stop pointers under it. Click a pointer to select, drag it to move,
// drag it well off the bar to delete, click an empty spot under the bar to
// add a stop carrying the ramp's own color there.
type gradRampView struct {
	guigui.DefaultWidget

	// dragPlus1 is the stop being dragged plus one, so the zero value
	// means none.
	dragPlus1 int
}

func (v *gradRampView) model(context *guigui.Context) *Model {
	val, ok := context.Env(v, envKeyModel)
	if !ok {
		return nil
	}
	m, _ := val.(*Model)
	return m
}

// barRect is where the gradient paints; the strip below it belongs to the
// pointers.
func (v *gradRampView) barRect(context *guigui.Context, b image.Rectangle) image.Rectangle {
	u := basicwidget.UnitSize(context)
	return image.Rect(b.Min.X+u/4, b.Min.Y, b.Max.X-u/4, b.Max.Y-u/2)
}

func (v *gradRampView) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w := 6 * u
	if cw, ok := constraints.FixedWidth(); ok {
		w = cw
	}
	return image.Pt(w, 3*u/2)
}

func (v *gradRampView) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	m := v.model(context)
	if m == nil {
		return
	}
	stops := m.ShapeGradientStops()
	if len(stops) == 0 {
		return
	}
	b := widgetBounds.Bounds()
	bar := v.barRect(context, b)
	u := float32(basicwidget.UnitSize(context))
	// The gradient itself, sampled in strips: exact at every stop, close
	// enough between them.
	const strips = 64
	w := float32(bar.Dx()) / strips
	for i := range strips {
		pos := (float64(i) + 0.5) / strips
		r, g, bl := gradColorAt(stops, pos)
		c := color.NRGBA{uint8(r * 255), uint8(g * 255), uint8(bl * 255), 0xff}
		vector.DrawFilledRect(dst, float32(bar.Min.X)+float32(i)*w, float32(bar.Min.Y),
			w+1, float32(bar.Dy()), c, false)
	}
	vector.StrokeRect(dst, float32(bar.Min.X), float32(bar.Min.Y),
		float32(bar.Dx()), float32(bar.Dy()), 1, color.NRGBA{0x40, 0x40, 0x48, 0xff}, false)
	// The pointers: a triangle per stop, the selected one filled dark.
	sel := m.SelectedGradStop()
	for i, s := range stops {
		x := float32(bar.Min.X) + float32(s.pos)*float32(bar.Dx())
		y := float32(bar.Max.Y)
		var pth vector.Path
		pth.MoveTo(x, y)
		pth.LineTo(x-u/4, y+u/2-2)
		pth.LineTo(x+u/4, y+u/2-2)
		pth.Close()
		fill := color.NRGBA{uint8(s.r * 255), uint8(s.g * 255), uint8(s.b * 255), 0xff}
		op := pathColor(fill)
		vector.FillPath(dst, &pth, &vector.FillOptions{FillRule: vector.FillRuleNonZero}, &op)
		edge := color.NRGBA{0x60, 0x60, 0x68, 0xff}
		width := float32(1)
		if i == sel {
			edge = color.NRGBA{0x10, 0x10, 0x18, 0xff}
			width = 2
		}
		vector.StrokeLine(dst, x, y, x-u/4, y+u/2-2, width, edge, true)
		vector.StrokeLine(dst, x-u/4, y+u/2-2, x+u/4, y+u/2-2, width, edge, true)
		vector.StrokeLine(dst, x+u/4, y+u/2-2, x, y, width, edge, true)
	}
}

func (v *gradRampView) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	m := v.model(context)
	if m == nil || !context.IsEnabled(v) {
		return guigui.HandleInputResult{}
	}
	b := widgetBounds.Bounds()
	bar := v.barRect(context, b)
	u := basicwidget.UnitSize(context)
	cx, cy := ebiten.CursorPosition()
	posOf := func() float64 {
		if bar.Dx() <= 0 {
			return 0
		}
		return min(max(float64(cx-bar.Min.X)/float64(bar.Dx()), 0), 1)
	}
	if v.dragPlus1 > 0 {
		if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			// Released well below the bar: the Flash delete gesture.
			if cy > bar.Max.Y+u && len(m.ShapeGradientStops()) > 2 {
				m.DeleteGradStop(v.dragPlus1 - 1)
			}
			v.dragPlus1 = 0
			m.EndPoseEdit()
			return guigui.HandleInputByWidget(v)
		}
		m.SetGradStopPos(v.dragPlus1-1, posOf())
		// The write re-sorts; keep dragging whichever stop is selected now.
		if s := m.SelectedGradStop(); s >= 0 {
			v.dragPlus1 = s + 1
		}
		return guigui.HandleInputByWidget(v)
	}
	if !widgetBounds.IsHitAtCursor() || !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return guigui.HandleInputResult{}
	}
	stops := m.ShapeGradientStops()
	// A press near a pointer grabs it; anywhere else on the strip adds.
	for i, s := range stops {
		x := bar.Min.X + int(s.pos*float64(bar.Dx()))
		if abs32(float32(cx-x)) <= float32(u)/3 && cy >= bar.Min.Y {
			m.SelectGradStop(i)
			v.dragPlus1 = i + 1
			m.BeginPoseEdit()
			return guigui.HandleInputByWidget(v)
		}
	}
	if cy >= bar.Min.Y {
		m.AddGradStopAt(posOf())
		if s := m.SelectedGradStop(); s >= 0 {
			v.dragPlus1 = s + 1
			m.BeginPoseEdit()
		}
		return guigui.HandleInputByWidget(v)
	}
	return guigui.HandleInputResult{}
}

func (v *gradRampView) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	if m := v.model(context); m != nil {
		w.WriteInt(m.Generation())
	}
	w.WriteInt(v.dragPlus1)
}
