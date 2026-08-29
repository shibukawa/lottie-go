package main

import (
	"fmt"
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	lottie "github.com/shibukawa/lottie-go"
	lottieresolv "github.com/shibukawa/lottie-go/plugin/physics/resolv"
	lottiesockets "github.com/shibukawa/lottie-go/plugin/sockets"
)

// setBold is the Text bold shorthand; guigui expresses it through a style.
func setBold(t *basicwidget.Text, s string) {
	t.SetValue(s)
	var style basicwidget.TextStyle
	style.SetBold(true)
	t.SetBaseStyle(&style)
}

func label(t *basicwidget.Text, s string) {
	t.SetValue(s)
	t.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
}

// setOptions fills a dropdown whose values are their own labels. Dropdowns
// replaced the editable comboboxes here: every one of these fields picks
// from a closed set, and typed text only got in the way of picking.
func setOptions(s *basicwidget.Select[string], values ...string) {
	items := make([]basicwidget.SelectItem[string], len(values))
	for i, v := range values {
		items[i] = basicwidget.SelectItem[string]{Text: v, Value: v}
	}
	s.SetItems(items)
}

// selectedValue is the dropdown's current value, or "" for none.
func selectedValue(s *basicwidget.Select[string]) string {
	if it, ok := s.SelectedItem(); ok {
		return it.Value
	}
	return ""
}

// inspectorPane is the parameter pane for whatever is selected: a state
// (with its transitions and guards), a machine, or one collision item.
// Model.InspectTarget says which; every Select records itself there. It
// scrolls, since the state sections together are taller than most panes.
type inspectorPane struct {
	guigui.DefaultWidget

	panel   basicwidget.Panel
	content inspectorContent
}

func (p *inspectorPane) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&p.panel)
	p.panel.SetContent(&p.content)
	p.panel.SetAutoBorder(true)
	p.panel.SetContentConstraints(basicwidget.PanelContentConstraintsFixedWidth)
	return nil
}

func (p *inspectorPane) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&p.panel, widgetBounds.Bounds())
}

type inspectorContent struct {
	guigui.DefaultWidget

	stateTitle basicwidget.Text
	addState   basicwidget.Button
	delState   basicwidget.Button
	setInitial basicwidget.Button

	nameLabel, typeLabel, animLabel basicwidget.Text
	segLabel, modeLabel, speedLabel basicwidget.Text
	loopLabel, autoplayLabel        basicwidget.Text
	nameInput                       basicwidget.TextInput
	typeSel, animSel, segSel        basicwidget.Select[string]
	modeSel                         basicwidget.Select[string]
	speedInput                      basicwidget.TextInput
	loopCheck, autoplayCheck        basicwidget.Checkbox
	form                            basicwidget.Form

	transTitle  basicwidget.Text
	transList   basicwidget.List[int]
	transTarget basicwidget.Select[string]
	addTrans    basicwidget.Button
	upTrans     basicwidget.Button
	downTrans   basicwidget.Button
	delTrans    basicwidget.Button

	guardTitle      basicwidget.Text
	guardList       basicwidget.List[int]
	addGuard        basicwidget.Button
	delGuard        basicwidget.Button
	guardTypeLabel  basicwidget.Text
	guardInputLabel basicwidget.Text
	guardCondLabel  basicwidget.Text
	guardValueLabel basicwidget.Text
	guardType       basicwidget.Select[string]
	guardInput      basicwidget.Select[string]
	guardCond       basicwidget.Select[string]
	guardValue      basicwidget.TextInput
	guardForm       basicwidget.Form

	problemsTitle basicwidget.Text
	problems      basicwidget.Text

	// Machine pane.
	machTitle     basicwidget.Text
	machNameLabel basicwidget.Text
	machNameInput basicwidget.TextInput
	machInitial   basicwidget.Button
	machAddState  basicwidget.Button
	machForm      basicwidget.Form
	machFormItems []basicwidget.FormItem

	// Hitbox pane.
	hbTitle     basicwidget.Text
	hbNameLabel basicwidget.Text
	hbKindLabel basicwidget.Text
	hbTagsLabel basicwidget.Text
	hbFromLabel basicwidget.Text
	hbToLabel   basicwidget.Text
	hbKindValue basicwidget.Text
	hbNameInput basicwidget.TextInput
	hbTagsInput basicwidget.TextInput
	hbFromInput basicwidget.TextInput
	hbToInput   basicwidget.TextInput
	hbAddSpan   basicwidget.Button
	hbDelSpan   basicwidget.Button
	hbForm      basicwidget.Form
	hbFormItems []basicwidget.FormItem

	// Body shape pane.
	cpTitle        basicwidget.Text
	cpTypeLabel    basicwidget.Text
	cpTypeValue    basicwidget.Text
	cpFrictLabel   basicwidget.Text
	cpElasticLabel basicwidget.Text
	cpSensorLabel  basicwidget.Text
	cpFrictInput   basicwidget.TextInput
	cpElasticInput basicwidget.TextInput
	cpSensorCheck  basicwidget.Checkbox
	cpForm         basicwidget.Form
	cpFormItems    []basicwidget.FormItem

	// Config pane.
	cfgTitle       basicwidget.Text
	cfgViewerLabel basicwidget.Text
	cfgViewer      basicwidget.Checkbox
	cfgPhysLabel   basicwidget.Text
	cfgPhysSel     basicwidget.Select[string]
	cfgNote        basicwidget.Text
	cfgForm        basicwidget.Form
	cfgFormItems   []basicwidget.FormItem

	// Socket pane.
	sockTitle      basicwidget.Text
	sockNameLabel  basicwidget.Text
	sockLayerLabel basicwidget.Text
	sockLayerValue basicwidget.Text
	sockDXLabel    basicwidget.Text
	sockDYLabel    basicwidget.Text
	sockDRLabel    basicwidget.Text
	sockNameInput  basicwidget.TextInput
	sockDXInput    basicwidget.TextInput
	sockDYInput    basicwidget.TextInput
	sockDRInput    basicwidget.TextInput
	sockZBtn       basicwidget.Button
	sockRotBtn     basicwidget.Button
	sockForm       basicwidget.Form
	sockFormItems  []basicwidget.FormItem

	// Pose pane. The numeric fields are built from poseFields, so the
	// labels and inputs are slices rather than one pair per member.
	poseTitle  basicwidget.Text
	partsTitle basicwidget.Text
	partsList  basicwidget.List[int]
	partsItems []basicwidget.ListItem[int]
	// The part the list was last scrolled to, plus one so the zero value
	// means "none". Scrolling on every rebuild would fight whoever is
	// scrolling the list by hand.
	partsShownPlus1 int
	partsFront      basicwidget.Button
	partsBack       basicwidget.Button
	partsHide       basicwidget.Button
	partsBtnRow     guigui.LinearLayout
	partsBtnItems   []guigui.LinearLayoutItem
	posePartLabel   basicwidget.Text
	posePartInput   basicwidget.TextInput
	poseJointLabel  basicwidget.Text
	poseJointSel    basicwidget.Select[int]
	poseEaseLabel   basicwidget.Text
	poseEaseSel     basicwidget.Select[bool]
	poseParentLabel basicwidget.Text
	poseParentSel   basicwidget.Select[int]
	poseJDragLabel  basicwidget.Text
	poseJDragSel    basicwidget.Select[bool]
	poseFrameLabel  basicwidget.Text
	poseFrameValue  basicwidget.Text
	poseLabels      guigui.WidgetSlice[*basicwidget.Text]
	poseInputs      guigui.WidgetSlice[*basicwidget.TextInput]
	poseHint        basicwidget.Text
	poseUndo        basicwidget.Button
	poseForm        basicwidget.Form
	poseFormItems   []basicwidget.FormItem
	jointItems      []basicwidget.SelectItem[int]
	easeItems       []basicwidget.SelectItem[bool]
	tabFields       []*basicwidget.TextInput
	parentItems     []basicwidget.SelectItem[int]
	jdragItems      []basicwidget.SelectItem[bool]

	selectedGuard int

	transItems     []basicwidget.ListItem[int]
	guardItems     []basicwidget.ListItem[int]
	formItems      []basicwidget.FormItem
	guardFormItems []basicwidget.FormItem

	stateBtnRow   guigui.LinearLayout
	stateBtnItems []guigui.LinearLayoutItem
	transBtnRow   guigui.LinearLayout
	transBtnItems []guigui.LinearLayoutItem
	guardBtnRow   guigui.LinearLayout
	guardBtnItems []guigui.LinearLayoutItem
	items         []guigui.LinearLayoutItem
}

func (c *inspectorContent) model(context *guigui.Context) *Model {
	v, ok := context.Env(c, envKeyModel)
	if !ok {
		return nil
	}
	m, _ := v.(*Model)
	return m
}

func (c *inspectorContent) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m := c.model(context)
	if m == nil {
		return nil
	}
	// The pane follows the last selection: only the widgets of the active
	// pane are added, so the others are neither drawn nor sent input.
	switch m.InspectTarget() {
	case inspectMachine:
		adder.AddWidget(&c.machTitle)
		adder.AddWidget(&c.machForm)
		adder.AddWidget(&c.machInitial)
		adder.AddWidget(&c.machAddState)
		c.buildMachinePane(context, m)
	case inspectHitbox:
		adder.AddWidget(&c.hbTitle)
		adder.AddWidget(&c.hbForm)
		adder.AddWidget(&c.hbAddSpan)
		adder.AddWidget(&c.hbDelSpan)
		c.buildHitboxPane(context, m)
	case inspectCPShape:
		adder.AddWidget(&c.cpTitle)
		adder.AddWidget(&c.cpForm)
		c.buildCPShapePane(context, m)
	case inspectSocket:
		adder.AddWidget(&c.sockTitle)
		adder.AddWidget(&c.sockForm)
		adder.AddWidget(&c.sockZBtn)
		adder.AddWidget(&c.sockRotBtn)
		c.buildSocketPane(context, m)
	case inspectPose:
		adder.AddWidget(&c.partsTitle)
		adder.AddWidget(&c.partsList)
		for _, w := range []guigui.Widget{&c.partsFront, &c.partsBack, &c.partsHide} {
			adder.AddWidget(w)
		}
		adder.AddWidget(&c.poseTitle)
		adder.AddWidget(&c.posePartInput)
		adder.AddWidget(&c.poseJointSel)
		adder.AddWidget(&c.poseEaseSel)
		adder.AddWidget(&c.poseParentSel)
		adder.AddWidget(&c.poseJDragSel)
		adder.AddWidget(&c.poseForm)
		adder.AddWidget(&c.poseUndo)
		adder.AddWidget(&c.poseHint)
		c.buildPosePane(context, m, adder)
	case inspectConfig:
		adder.AddWidget(&c.cfgTitle)
		adder.AddWidget(&c.cfgForm)
		adder.AddWidget(&c.cfgNote)
		c.buildConfigPane(context, m)
	default:
		for _, w := range []guigui.Widget{
			&c.stateTitle, &c.addState, &c.delState, &c.setInitial, &c.form,
			&c.transTitle, &c.transList, &c.transTarget, &c.addTrans,
			&c.upTrans, &c.downTrans, &c.delTrans,
			&c.guardTitle, &c.guardList, &c.addGuard, &c.delGuard,
		} {
			adder.AddWidget(w)
		}
		// The guard form appears only while a guard is selected; empty
		// unlabeled editors below the list read as mystery boxes.
		if c.selectedGuardOf(m) != nil {
			adder.AddWidget(&c.guardForm)
		}
		st := m.SelectedState()
		setBold(&c.stateTitle, "State")
		c.buildStateForm(context, m, st)
		c.buildTransitions(context, m, st)
		c.buildGuards(context, m)
	}

	adder.AddWidget(&c.problemsTitle)
	adder.AddWidget(&c.problems)
	setBold(&c.problemsTitle, "Problems")
	probs := m.Problems()
	if len(probs) == 0 {
		c.problems.SetValue("none")
	} else {
		c.problems.SetValue(strings.Join(probs, "\n"))
	}
	c.problems.SetMultiline(true)
	c.problems.SetScale(0.85)
	return nil
}

// buildMachinePane edits the machine itself: its id and whether it is the
// bundle's default. Adding a state lives here too — a state is added to
// the selected machine.
func (c *inspectorContent) buildMachinePane(context *guigui.Context, m *Model) {
	setBold(&c.machTitle, "Machine")
	current := m.MachineID()

	label(&c.machNameLabel, "id")
	c.machNameInput.SetValue(current)
	c.machNameInput.SetPlaceholder("machine id")
	c.machNameInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed {
			m.RenameMachine(current, text)
		}
	})

	// A toggle: naming the machine that is already default clears the
	// choice, which puts "the first listed" back.
	if current != "" && current == m.InitialMachine() {
		c.machInitial.SetText("Unset initial")
		c.machInitial.OnDown(func(context *guigui.Context) { m.SetInitialMachine("") })
	} else {
		c.machInitial.SetText("Set initial")
		c.machInitial.OnDown(func(context *guigui.Context) { m.SetInitialMachine(current) })
	}
	c.machAddState.SetText("Add state")
	c.machAddState.OnDown(func(context *guigui.Context) { m.AddState() })

	for _, w := range []guigui.Widget{&c.machNameInput, &c.machInitial, &c.machAddState} {
		context.SetEnabled(w, current != "" && !m.Viewer())
	}

	c.machFormItems = slices.Delete(c.machFormItems, 0, len(c.machFormItems))
	c.machFormItems = append(c.machFormItems,
		basicwidget.FormItem{PrimaryWidget: &c.machNameLabel, SecondaryWidget: &c.machNameInput},
	)
	c.machForm.SetItems(c.machFormItems)
}

// buildHitboxPane edits the selected hitbox: identity, tags, and the span
// under the playhead. Geometry is dragged on the stage.
func (c *inspectorContent) buildHitboxPane(context *guigui.Context, m *Model) {
	b := m.SelectedHitbox()
	title := "Hitbox"
	if b != nil && b.Kind == lottieresolv.KindWindow {
		title = "Window"
	}
	setBold(&c.hbTitle, title)

	label(&c.hbNameLabel, "name")
	label(&c.hbKindLabel, "kind")
	label(&c.hbTagsLabel, "tags")
	label(&c.hbFromLabel, "from")
	label(&c.hbToLabel, "to (excl.)")

	if b != nil {
		c.hbNameInput.SetValue(b.Name)
		c.hbKindValue.SetValue(string(b.Kind))
	} else {
		c.hbNameInput.SetValue("")
		c.hbKindValue.SetValue("")
	}
	c.hbKindValue.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	c.hbNameInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed {
			m.RenameHitbox(text)
		}
	})
	c.hbTagsInput.SetValue(m.HitboxTagsCSV())
	c.hbTagsInput.SetPlaceholder("hit, hurt, push, …")
	c.hbTagsInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed {
			m.SetHitboxTagsCSV(text)
		}
	})

	sp := m.SelectedSpan()
	if sp != nil {
		c.hbFromInput.SetValue(strconv.FormatFloat(sp.From, 'g', -1, 64))
		c.hbToInput.SetValue(strconv.FormatFloat(sp.To, 'g', -1, 64))
	} else {
		c.hbFromInput.SetValue("")
		c.hbToInput.SetValue("")
	}
	commitRange := func() {
		from, err1 := strconv.ParseFloat(strings.TrimSpace(c.hbFromInput.Value()), 64)
		to, err2 := strconv.ParseFloat(strings.TrimSpace(c.hbToInput.Value()), 64)
		if err1 == nil && err2 == nil {
			m.SetSpanRange(from, to)
		}
	}
	c.hbFromInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed {
			commitRange()
		}
	})
	c.hbToInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed {
			commitRange()
		}
	})
	c.hbAddSpan.SetText("+Span @ playhead")
	c.hbAddSpan.OnDown(func(context *guigui.Context) { m.AddHitboxSpan() })
	c.hbDelSpan.SetText("Delete span")
	c.hbDelSpan.OnDown(func(context *guigui.Context) { m.DeleteHitboxSpan() })

	for _, w := range []guigui.Widget{&c.hbNameInput, &c.hbTagsInput, &c.hbAddSpan} {
		context.SetEnabled(w, b != nil && !m.Viewer())
	}
	for _, w := range []guigui.Widget{&c.hbFromInput, &c.hbToInput, &c.hbDelSpan} {
		context.SetEnabled(w, sp != nil && !m.Viewer())
	}

	c.hbFormItems = slices.Delete(c.hbFormItems, 0, len(c.hbFormItems))
	c.hbFormItems = append(c.hbFormItems,
		basicwidget.FormItem{PrimaryWidget: &c.hbNameLabel, SecondaryWidget: &c.hbNameInput},
		basicwidget.FormItem{PrimaryWidget: &c.hbKindLabel, SecondaryWidget: &c.hbKindValue},
		basicwidget.FormItem{PrimaryWidget: &c.hbTagsLabel, SecondaryWidget: &c.hbTagsInput},
		basicwidget.FormItem{PrimaryWidget: &c.hbFromLabel, SecondaryWidget: &c.hbFromInput},
		basicwidget.FormItem{PrimaryWidget: &c.hbToLabel, SecondaryWidget: &c.hbToInput},
	)
	c.hbForm.SetItems(c.hbFormItems)
}

// buildCPShapePane edits the selected body shape's material; placement is
// dragged on the stage.
func (c *inspectorContent) buildCPShapePane(context *guigui.Context, m *Model) {
	setBold(&c.cpTitle, "Body shape")
	s := m.SelectedCPShape()

	label(&c.cpTypeLabel, "type")
	label(&c.cpFrictLabel, "friction")
	label(&c.cpElasticLabel, "elasticity")
	label(&c.cpSensorLabel, "sensor")
	c.cpTypeValue.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	if s != nil {
		c.cpTypeValue.SetValue(string(s.Type))
		c.cpFrictInput.SetValue(strconv.FormatFloat(s.Friction, 'g', -1, 64))
		c.cpElasticInput.SetValue(strconv.FormatFloat(s.Elasticity, 'g', -1, 64))
		c.cpSensorCheck.SetValue(s.Sensor)
	} else {
		c.cpTypeValue.SetValue("")
		c.cpFrictInput.SetValue("")
		c.cpElasticInput.SetValue("")
		c.cpSensorCheck.SetValue(false)
	}
	c.cpFrictInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if !committed {
			return
		}
		if v, err := strconv.ParseFloat(strings.TrimSpace(text), 64); err == nil && v >= 0 {
			if s := m.SelectedCPShape(); s != nil {
				s.Friction = v
				m.touchCPBody()
			}
		}
	})
	c.cpElasticInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if !committed {
			return
		}
		if v, err := strconv.ParseFloat(strings.TrimSpace(text), 64); err == nil && v >= 0 {
			if s := m.SelectedCPShape(); s != nil {
				s.Elasticity = v
				m.touchCPBody()
			}
		}
	})
	c.cpSensorCheck.OnValueChanged(func(context *guigui.Context, value bool) {
		if s := m.SelectedCPShape(); s != nil {
			s.Sensor = value
			m.touchCPBody()
		}
	})

	for _, w := range []guigui.Widget{&c.cpFrictInput, &c.cpElasticInput, &c.cpSensorCheck} {
		context.SetEnabled(w, s != nil && !m.Viewer())
	}

	c.cpFormItems = slices.Delete(c.cpFormItems, 0, len(c.cpFormItems))
	c.cpFormItems = append(c.cpFormItems,
		basicwidget.FormItem{PrimaryWidget: &c.cpTypeLabel, SecondaryWidget: &c.cpTypeValue},
		basicwidget.FormItem{PrimaryWidget: &c.cpFrictLabel, SecondaryWidget: &c.cpFrictInput},
		basicwidget.FormItem{PrimaryWidget: &c.cpElasticLabel, SecondaryWidget: &c.cpElasticInput},
		basicwidget.FormItem{PrimaryWidget: &c.cpSensorLabel, SecondaryWidget: &c.cpSensorCheck},
	)
	c.cpForm.SetItems(c.cpFormItems)
}

// buildSocketPane edits the selected socket: its game-facing name, the
// layer it reads (fixed at bind time), and which side an attached item
// draws on.
func (c *inspectorContent) buildSocketPane(context *guigui.Context, m *Model) {
	setBold(&c.sockTitle, "Socket")
	s := m.SelectedSocket()

	label(&c.sockNameLabel, "name")
	label(&c.sockLayerLabel, "layer")
	label(&c.sockDXLabel, "offset x")
	label(&c.sockDYLabel, "offset y")
	label(&c.sockDRLabel, "angle +°")
	c.sockLayerValue.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	if s != nil {
		c.sockNameInput.SetValue(s.Name)
		c.sockLayerValue.SetValue(s.LayerName())
		c.sockDXInput.SetValue(strconv.FormatFloat(s.DX, 'g', -1, 64))
		c.sockDYInput.SetValue(strconv.FormatFloat(s.DY, 'g', -1, 64))
		c.sockDRInput.SetValue(strconv.FormatFloat(s.DR, 'g', -1, 64))
	} else {
		c.sockNameInput.SetValue("")
		c.sockLayerValue.SetValue("")
		c.sockDXInput.SetValue("")
		c.sockDYInput.SetValue("")
		c.sockDRInput.SetValue("")
	}
	c.sockNameInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed {
			m.RenameSocket(text)
		}
	})
	// The offset is layer-local; dragging the cross on the stage writes
	// the same members.
	offset := func(dst func(*lottiesockets.Socket) *float64) func(*guigui.Context, string, bool) {
		return func(context *guigui.Context, text string, committed bool) {
			if !committed {
				return
			}
			if v, err := strconv.ParseFloat(strings.TrimSpace(text), 64); err == nil {
				if s := m.SelectedSocket(); s != nil {
					*dst(s) = v
					m.touchSockets()
				}
			}
		}
	}
	c.sockDXInput.OnValueChanged(offset(func(s *lottiesockets.Socket) *float64 { return &s.DX }))
	c.sockDYInput.OnValueChanged(offset(func(s *lottiesockets.Socket) *float64 { return &s.DY }))
	c.sockDRInput.OnValueChanged(offset(func(s *lottiesockets.Socket) *float64 { return &s.DR }))

	rotText := "rotate: follow"
	if s != nil && s.Rotate == lottiesockets.RotateNone {
		rotText = "rotate: none"
	}
	c.sockRotBtn.SetText(rotText)
	c.sockRotBtn.OnDown(func(context *guigui.Context) { m.ToggleSocketRotate() })

	zText := "z: front"
	if s != nil && s.Z == lottiesockets.ZBehind {
		zText = "z: behind"
	}
	c.sockZBtn.SetText(zText)
	c.sockZBtn.OnDown(func(context *guigui.Context) { m.ToggleSocketZ() })

	for _, w := range []guigui.Widget{&c.sockNameInput, &c.sockZBtn, &c.sockRotBtn, &c.sockDXInput, &c.sockDYInput, &c.sockDRInput} {
		context.SetEnabled(w, s != nil && !m.Viewer())
	}

	tabOrder(context, &c.sockNameInput, &c.sockDXInput, &c.sockDYInput, &c.sockDRInput)

	c.sockFormItems = slices.Delete(c.sockFormItems, 0, len(c.sockFormItems))
	c.sockFormItems = append(c.sockFormItems,
		basicwidget.FormItem{PrimaryWidget: &c.sockNameLabel, SecondaryWidget: &c.sockNameInput},
		basicwidget.FormItem{PrimaryWidget: &c.sockLayerLabel, SecondaryWidget: &c.sockLayerValue},
		basicwidget.FormItem{PrimaryWidget: &c.sockDXLabel, SecondaryWidget: &c.sockDXInput},
		basicwidget.FormItem{PrimaryWidget: &c.sockDYLabel, SecondaryWidget: &c.sockDYInput},
		basicwidget.FormItem{PrimaryWidget: &c.sockDRLabel, SecondaryWidget: &c.sockDRInput},
	)
	c.sockForm.SetItems(c.sockFormItems)
}

func (c *inspectorContent) buildStateForm(context *guigui.Context, m *Model, st *lottie.State) {
	c.addState.SetText("Add state")
	context.SetEnabled(&c.addState, !m.Viewer())
	c.addState.OnDown(func(context *guigui.Context) { m.AddState() })
	c.delState.SetText("Delete")
	c.delState.OnDown(func(context *guigui.Context) { m.DeleteState(m.SelectedStateName()) })
	c.setInitial.SetText("Set initial")
	c.setInitial.OnDown(func(context *guigui.Context) { m.SetInitial(m.SelectedStateName()) })
	context.SetEnabled(&c.delState, st != nil && !m.Viewer())
	context.SetEnabled(&c.setInitial, st != nil && !m.Viewer())

	label(&c.nameLabel, "name")
	label(&c.typeLabel, "type")
	label(&c.animLabel, "animation")
	label(&c.segLabel, "segment")
	label(&c.loopLabel, "loop")
	label(&c.autoplayLabel, "autoplay")
	label(&c.modeLabel, "mode")
	label(&c.speedLabel, "speed")

	playback := st != nil && st.Type != lottie.StateGlobal
	for _, w := range []guigui.Widget{
		&c.nameInput, &c.typeSel, &c.animSel, &c.segSel,
		&c.modeSel, &c.speedInput, &c.loopCheck, &c.autoplayCheck,
	} {
		context.SetEnabled(w, st != nil)
	}
	for _, w := range []guigui.Widget{&c.animSel, &c.segSel, &c.modeSel, &c.speedInput, &c.loopCheck, &c.autoplayCheck} {
		context.SetEnabled(w, playback)
	}

	if st == nil {
		c.nameInput.SetValue("")
		c.form.SetItems(c.buildFormItems())
		return
	}
	old := st.Name
	c.nameInput.SetValue(st.Name)
	c.nameInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed {
			m.RenameState(old, text)
		}
	})

	setOptions(&c.typeSel, string(lottie.StatePlayback), string(lottie.StateGlobal))
	c.typeSel.SelectItemByValue(string(st.Type))
	c.typeSel.OnItemSelected(func(context *guigui.Context, index int) {
		it, ok := c.typeSel.ItemByIndex(index)
		st := m.SelectedState()
		if !ok || st == nil || it.Value == string(st.Type) {
			return
		}
		st.Type = lottie.StateType(it.Value)
		m.Touch()
	})

	setOptions(&c.animSel, m.AnimationIDs()...)
	c.animSel.SelectItemByValue(st.Animation)
	c.animSel.OnItemSelected(func(context *guigui.Context, index int) {
		it, ok := c.animSel.ItemByIndex(index)
		st := m.SelectedState()
		if !ok || st == nil || it.Value == st.Animation {
			return
		}
		st.Animation = it.Value
		st.Segment = ""
		m.Touch()
	})

	// The empty segment means "the whole clip"; give it a readable label.
	segItems := []basicwidget.SelectItem[string]{{Text: "(whole clip)", Value: ""}}
	for _, name := range m.Markers(st.Animation) {
		segItems = append(segItems, basicwidget.SelectItem[string]{Text: name, Value: name})
	}
	c.segSel.SetItems(segItems)
	c.segSel.SelectItemByValue(st.Segment)
	c.segSel.OnItemSelected(func(context *guigui.Context, index int) {
		it, ok := c.segSel.ItemByIndex(index)
		st := m.SelectedState()
		if !ok || st == nil || it.Value == st.Segment {
			return
		}
		st.Segment = it.Value
		m.Touch()
	})

	setOptions(&c.modeSel, string(lottie.PlayForward), string(lottie.PlayReverse))
	c.modeSel.SelectItemByValue(string(st.PlaybackMode()))
	c.modeSel.OnItemSelected(func(context *guigui.Context, index int) {
		it, ok := c.modeSel.ItemByIndex(index)
		st := m.SelectedState()
		if !ok || st == nil || it.Value == string(st.PlaybackMode()) {
			return
		}
		st.Mode = lottie.PlayMode(it.Value)
		m.Touch()
	})

	c.speedInput.SetValue(strconv.FormatFloat(st.PlaybackSpeed(), 'g', -1, 64))
	c.speedInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if !committed {
			return
		}
		if v, err := strconv.ParseFloat(strings.TrimSpace(text), 64); err == nil && v >= 0 {
			st.Speed = v
			m.Touch()
		}
	})

	c.loopCheck.SetValue(st.Loop)
	c.loopCheck.OnValueChanged(func(context *guigui.Context, value bool) {
		st.Loop = value
		m.Touch()
	})
	c.autoplayCheck.SetValue(st.Autoplay)
	c.autoplayCheck.OnValueChanged(func(context *guigui.Context, value bool) {
		st.Autoplay = value
		m.Touch()
	})
	c.form.SetItems(c.buildFormItems())
}

func (c *inspectorContent) buildFormItems() []basicwidget.FormItem {
	c.formItems = slices.Delete(c.formItems, 0, len(c.formItems))
	c.formItems = append(c.formItems,
		basicwidget.FormItem{PrimaryWidget: &c.nameLabel, SecondaryWidget: &c.nameInput},
		basicwidget.FormItem{PrimaryWidget: &c.typeLabel, SecondaryWidget: &c.typeSel},
		basicwidget.FormItem{PrimaryWidget: &c.animLabel, SecondaryWidget: &c.animSel},
		basicwidget.FormItem{PrimaryWidget: &c.segLabel, SecondaryWidget: &c.segSel},
		basicwidget.FormItem{PrimaryWidget: &c.modeLabel, SecondaryWidget: &c.modeSel},
		basicwidget.FormItem{PrimaryWidget: &c.speedLabel, SecondaryWidget: &c.speedInput},
		basicwidget.FormItem{PrimaryWidget: &c.loopLabel, SecondaryWidget: &c.loopCheck},
		basicwidget.FormItem{PrimaryWidget: &c.autoplayLabel, SecondaryWidget: &c.autoplayCheck},
	)
	return c.formItems
}

func (c *inspectorContent) buildTransitions(context *guigui.Context, m *Model, st *lottie.State) {
	setBold(&c.transTitle, "Transitions  (order decides which one wins)")

	var trs []lottie.Transition
	if st != nil {
		trs = st.Transitions
	}
	c.transItems = slices.Delete(c.transItems, 0, len(c.transItems))
	for i, tr := range trs {
		c.transItems = append(c.transItems, basicwidget.ListItem[int]{
			Text:    fmt.Sprintf("%d. → %s", i+1, tr.ToState),
			KeyText: summarizeGuards(tr.Guards),
			Value:   i,
		})
	}
	c.transList.SetItems(c.transItems)
	if sel := m.SelectedTransitionIndex(); sel >= 0 && sel < len(trs) {
		c.transList.SelectItemByValue(sel)
	}
	c.transList.OnItemSelected(func(context *guigui.Context, index int) {
		m.SelectTransition(index)
		c.selectedGuard = -1
	})

	setOptions(&c.transTarget, m.StateNames()...)
	c.addTrans.SetText("Add →")
	c.addTrans.OnDown(func(context *guigui.Context) {
		to := selectedValue(&c.transTarget)
		if to == "" {
			if names := m.StateNames(); len(names) > 0 {
				to = names[0]
			}
		}
		m.AddTransition(to)
	})
	c.upTrans.SetText("Up")
	c.upTrans.OnDown(func(context *guigui.Context) { m.MoveTransition(m.SelectedTransitionIndex(), -1) })
	c.downTrans.SetText("Down")
	c.downTrans.OnDown(func(context *guigui.Context) { m.MoveTransition(m.SelectedTransitionIndex(), 1) })
	c.delTrans.SetText("Delete")
	c.delTrans.OnDown(func(context *guigui.Context) { m.DeleteTransition(m.SelectedTransitionIndex()) })

	hasTrans := m.SelectedTransition() != nil
	context.SetEnabled(&c.addTrans, st != nil)
	context.SetEnabled(&c.upTrans, hasTrans)
	context.SetEnabled(&c.downTrans, hasTrans)
	context.SetEnabled(&c.delTrans, hasTrans)
}

func summarizeGuards(gs []lottie.Guard) string {
	if len(gs) == 0 {
		return "always"
	}
	parts := make([]string, 0, len(gs))
	for _, g := range gs {
		parts = append(parts, describeGuard(g))
	}
	return strings.Join(parts, " & ")
}

func describeGuard(g lottie.Guard) string {
	if g.Type == lottie.GuardEvent {
		return "on " + g.InputName
	}
	cond := string(g.ConditionType)
	if cond == "" {
		cond = string(lottie.ConditionEqual)
	}
	return fmt.Sprintf("%s %s %s", g.InputName, cond, string(g.CompareTo))
}

func (c *inspectorContent) buildGuards(context *guigui.Context, m *Model) {
	setBold(&c.guardTitle, "Guards  (all must pass)")

	tr := m.SelectedTransition()
	var gs []lottie.Guard
	if tr != nil {
		gs = tr.Guards
	}
	if c.selectedGuard >= len(gs) {
		c.selectedGuard = -1
	}
	c.guardItems = slices.Delete(c.guardItems, 0, len(c.guardItems))
	for i, g := range gs {
		c.guardItems = append(c.guardItems, basicwidget.ListItem[int]{
			Text: describeGuard(g), KeyText: string(g.Type), Value: i,
		})
	}
	c.guardList.SetItems(c.guardItems)
	if c.selectedGuard >= 0 {
		c.guardList.SelectItemByValue(c.selectedGuard)
	}
	c.guardList.OnItemSelected(func(context *guigui.Context, index int) {
		c.selectedGuard = index
	})

	c.addGuard.SetText("Add guard")
	c.addGuard.OnDown(func(context *guigui.Context) { m.AddGuard() })
	c.delGuard.SetText("Delete")
	c.delGuard.OnDown(func(context *guigui.Context) {
		m.DeleteGuard(c.selectedGuard)
		c.selectedGuard = -1
	})
	context.SetEnabled(&c.addGuard, tr != nil)
	context.SetEnabled(&c.delGuard, c.selectedGuard >= 0)

	g := c.selectedGuardOf(m)
	if g == nil {
		// No guard selected: the form is not even shown (Build and layout
		// skip it), so there is nothing to wire.
		return
	}

	label(&c.guardTypeLabel, "kind")
	label(&c.guardInputLabel, "input")
	label(&c.guardCondLabel, "cond")
	label(&c.guardValueLabel, "value")

	setOptions(&c.guardType,
		string(lottie.GuardEvent), string(lottie.GuardBoolean),
		string(lottie.GuardNumeric), string(lottie.GuardString),
	)
	c.guardType.SelectItemByValue(string(g.Type))
	c.guardType.OnItemSelected(func(context *guigui.Context, index int) {
		it, ok := c.guardType.ItemByIndex(index)
		g := c.selectedGuardOf(m)
		if !ok || g == nil || it.Value == string(g.Type) {
			return
		}
		g.Type = lottie.GuardType(it.Value)
		m.Touch()
	})

	setOptions(&c.guardInput, m.InputNames()...)
	c.guardInput.SelectItemByValue(g.InputName)
	c.guardInput.OnItemSelected(func(context *guigui.Context, index int) {
		it, ok := c.guardInput.ItemByIndex(index)
		g := c.selectedGuardOf(m)
		if !ok || g == nil || it.Value == g.InputName {
			return
		}
		g.InputName = it.Value
		m.Touch()
	})

	setOptions(&c.guardCond,
		string(lottie.ConditionEqual), string(lottie.ConditionNotEqual),
		string(lottie.ConditionGreaterThan), string(lottie.ConditionGreaterThanOrEqual),
		string(lottie.ConditionLessThan), string(lottie.ConditionLessThanOrEqual),
	)
	c.guardCond.SelectItemByValue(string(g.ConditionType))
	c.guardCond.OnItemSelected(func(context *guigui.Context, index int) {
		it, ok := c.guardCond.ItemByIndex(index)
		g := c.selectedGuardOf(m)
		if !ok || g == nil || it.Value == string(g.ConditionType) {
			return
		}
		g.ConditionType = lottie.ConditionType(it.Value)
		m.Touch()
	})

	c.guardValue.SetValue(string(g.CompareTo))
	c.guardValue.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		g := c.selectedGuardOf(m)
		if committed && g != nil {
			g.CompareTo = parseCompareTo(g.Type, text)
			m.Touch()
		}
	})
	// Event guards have no comparison at all.
	isEvent := g.Type == lottie.GuardEvent
	context.SetEnabled(&c.guardCond, !isEvent)
	context.SetEnabled(&c.guardValue, !isEvent)

	c.guardFormItems = slices.Delete(c.guardFormItems, 0, len(c.guardFormItems))
	c.guardFormItems = append(c.guardFormItems,
		basicwidget.FormItem{PrimaryWidget: &c.guardTypeLabel, SecondaryWidget: &c.guardType},
		basicwidget.FormItem{PrimaryWidget: &c.guardInputLabel, SecondaryWidget: &c.guardInput},
		basicwidget.FormItem{PrimaryWidget: &c.guardCondLabel, SecondaryWidget: &c.guardCond},
		basicwidget.FormItem{PrimaryWidget: &c.guardValueLabel, SecondaryWidget: &c.guardValue},
	)
	c.guardForm.SetItems(c.guardFormItems)
}

// buildConfigPane edits the bundle-level editor configuration, stored in
// the manifest's extra member so it travels with the file.
func (c *inspectorContent) buildConfigPane(context *guigui.Context, m *Model) {
	setBold(&c.cfgTitle, "Config")
	label(&c.cfgPhysLabel, "physics")

	label(&c.cfgViewerLabel, "viewer mode")
	c.cfgViewer.SetValue(m.Viewer())
	c.cfgViewer.OnValueChanged(func(context *guigui.Context, v bool) {
		m.SetViewer(v)
	})

	setOptions(&c.cfgPhysSel, "both", "cp", "resolv", "none")
	context.SetEnabled(&c.cfgPhysSel, !m.Viewer())
	c.cfgPhysSel.SelectItemByValue(m.PhysicsBackend())
	c.cfgPhysSel.OnItemSelected(func(context *guigui.Context, index int) {
		it, ok := c.cfgPhysSel.ItemByIndex(index)
		if ok && it.Value != m.PhysicsBackend() {
			m.SetPhysicsBackend(it.Value)
		}
	})

	c.cfgNote.SetValue("Which collision tooling this editor shows: hitbox tracks need resolv, the rigid body needs cp. Games choose engines by importing plugins; this switch only declutters authoring.")
	c.cfgNote.SetMultiline(true)
	c.cfgNote.SetScale(0.85)

	c.cfgFormItems = slices.Delete(c.cfgFormItems, 0, len(c.cfgFormItems))
	c.cfgFormItems = append(c.cfgFormItems,
		basicwidget.FormItem{PrimaryWidget: &c.cfgViewerLabel, SecondaryWidget: &c.cfgViewer},
		basicwidget.FormItem{PrimaryWidget: &c.cfgPhysLabel, SecondaryWidget: &c.cfgPhysSel},
	)
	c.cfgForm.SetItems(c.cfgFormItems)
}

// selectedGuardOf resolves the guard the guard form edits, or nil.
func (c *inspectorContent) selectedGuardOf(m *Model) *lottie.Guard {
	tr := m.SelectedTransition()
	if tr == nil || c.selectedGuard < 0 || c.selectedGuard >= len(tr.Guards) {
		return nil
	}
	return &tr.Guards[c.selectedGuard]
}

// parseCompareTo encodes the typed-in comparison for the guard's kind.
func parseCompareTo(t lottie.GuardType, text string) []byte {
	text = strings.TrimSpace(text)
	switch t {
	case lottie.GuardBoolean:
		return lottie.JSONValue(text == "true")
	case lottie.GuardNumeric:
		v, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return nil
		}
		return lottie.JSONValue(v)
	case lottie.GuardString:
		s := strings.Trim(text, `"`)
		return lottie.JSONValue(s)
	}
	return nil
}

func (c *inspectorContent) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	if m := c.model(context); m != nil {
		w.WriteInt(m.Generation())
		w.WriteInt(int(m.InspectTarget()))
	}
	w.WriteInt(c.selectedGuard)
}

func (c *inspectorContent) layout(context *guigui.Context) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)

	c.partsBtnItems = slices.Delete(c.partsBtnItems, 0, len(c.partsBtnItems))
	c.partsBtnItems = append(c.partsBtnItems,
		guigui.LinearLayoutItem{Widget: &c.partsFront, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &c.partsBack, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &c.partsHide, Size: guigui.FlexibleSize(1)},
	)
	c.partsBtnRow = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal, Items: c.partsBtnItems, Gap: u / 4,
	}

	c.items = slices.Delete(c.items, 0, len(c.items))
	target := inspectState
	if m := c.model(context); m != nil {
		target = m.InspectTarget()
	}
	switch target {
	case inspectMachine:
		c.items = append(c.items,
			guigui.LinearLayoutItem{Widget: &c.machTitle, Size: guigui.FixedSize(u)},
			guigui.LinearLayoutItem{Widget: &c.machForm},
			guigui.LinearLayoutItem{Widget: &c.machInitial, Size: guigui.FixedSize(u)},
			guigui.LinearLayoutItem{Widget: &c.machAddState, Size: guigui.FixedSize(u)},
		)
	case inspectHitbox:
		c.items = append(c.items,
			guigui.LinearLayoutItem{Widget: &c.hbTitle, Size: guigui.FixedSize(u)},
			guigui.LinearLayoutItem{Widget: &c.hbForm},
			guigui.LinearLayoutItem{Widget: &c.hbAddSpan, Size: guigui.FixedSize(u)},
			guigui.LinearLayoutItem{Widget: &c.hbDelSpan, Size: guigui.FixedSize(u)},
		)
	case inspectCPShape:
		c.items = append(c.items,
			guigui.LinearLayoutItem{Widget: &c.cpTitle, Size: guigui.FixedSize(u)},
			guigui.LinearLayoutItem{Widget: &c.cpForm},
		)
	case inspectSocket:
		c.items = append(c.items,
			guigui.LinearLayoutItem{Widget: &c.sockTitle, Size: guigui.FixedSize(u)},
			guigui.LinearLayoutItem{Widget: &c.sockForm},
			guigui.LinearLayoutItem{Widget: &c.sockZBtn, Size: guigui.FixedSize(u)},
			guigui.LinearLayoutItem{Widget: &c.sockRotBtn, Size: guigui.FixedSize(u)},
		)
	case inspectPose:
		c.items = append(c.items,
			guigui.LinearLayoutItem{Widget: &c.partsTitle, Size: guigui.FixedSize(u)},
			guigui.LinearLayoutItem{Widget: &c.partsList, Size: guigui.FixedSize(6 * u)},
			guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &c.partsBtnRow},
			guigui.LinearLayoutItem{Widget: &c.poseTitle, Size: guigui.FixedSize(u)},
			guigui.LinearLayoutItem{Widget: &c.poseForm},
			guigui.LinearLayoutItem{Widget: &c.poseUndo, Size: guigui.FixedSize(u)},
			guigui.LinearLayoutItem{Widget: &c.poseHint, Size: guigui.FixedSize(2 * u)},
		)
	case inspectConfig:
		c.items = append(c.items,
			guigui.LinearLayoutItem{Widget: &c.cfgTitle, Size: guigui.FixedSize(u)},
			guigui.LinearLayoutItem{Widget: &c.cfgForm},
			guigui.LinearLayoutItem{Widget: &c.cfgNote, Size: guigui.FixedSize(3 * u)},
		)
	default:
		c.items = c.stateItems(context, u)
	}
	c.items = append(c.items,
		guigui.LinearLayoutItem{Widget: &c.problemsTitle, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &c.problems, Size: guigui.FixedSize(4 * u)},
	)
	return guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     c.items, Gap: u / 4,
		Padding: guigui.Padding{Start: u / 2, Top: u / 2, End: u / 2, Bottom: u / 2},
	}
}

// stateItems is the state pane's layout: the form plus the transition and
// guard sections. The guard form joins only while a guard is selected,
// mirroring Build.
func (c *inspectorContent) stateItems(context *guigui.Context, u int) []guigui.LinearLayoutItem {
	c.stateBtnItems = slices.Delete(c.stateBtnItems, 0, len(c.stateBtnItems))
	c.stateBtnItems = append(c.stateBtnItems,
		guigui.LinearLayoutItem{Widget: &c.addState, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &c.setInitial, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &c.delState, Size: guigui.FlexibleSize(1)},
	)
	c.stateBtnRow = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal, Items: c.stateBtnItems, Gap: u / 4,
	}

	c.transBtnItems = slices.Delete(c.transBtnItems, 0, len(c.transBtnItems))
	c.transBtnItems = append(c.transBtnItems,
		guigui.LinearLayoutItem{Widget: &c.transTarget, Size: guigui.FlexibleSize(2)},
		guigui.LinearLayoutItem{Widget: &c.addTrans, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &c.upTrans, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &c.downTrans, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &c.delTrans, Size: guigui.FlexibleSize(1)},
	)
	c.transBtnRow = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal, Items: c.transBtnItems, Gap: u / 4,
	}

	c.guardBtnItems = slices.Delete(c.guardBtnItems, 0, len(c.guardBtnItems))
	c.guardBtnItems = append(c.guardBtnItems,
		guigui.LinearLayoutItem{Widget: &c.addGuard, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &c.delGuard, Size: guigui.FlexibleSize(1)},
	)
	c.guardBtnRow = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal, Items: c.guardBtnItems, Gap: u / 4,
	}

	items := slices.Delete(c.items, 0, len(c.items))
	items = append(items,
		guigui.LinearLayoutItem{Widget: &c.stateTitle, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &c.stateBtnRow},
		guigui.LinearLayoutItem{Widget: &c.form},
		guigui.LinearLayoutItem{Widget: &c.transTitle, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &c.transList, Size: guigui.FixedSize(5 * u)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &c.transBtnRow},
		guigui.LinearLayoutItem{Widget: &c.guardTitle, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &c.guardList, Size: guigui.FixedSize(4 * u)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &c.guardBtnRow},
	)
	if m := c.model(context); m != nil && c.selectedGuardOf(m) != nil {
		items = append(items, guigui.LinearLayoutItem{Widget: &c.guardForm})
	}
	return items
}

func (c *inspectorContent) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	c.layout(context).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (c *inspectorContent) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return c.layout(context).Measure(context, constraints)
}

// poseField is one editable number of the pose pane. A rig is driven almost
// entirely by rotation, so that comes first; the rest are here because the
// value at a key should be readable and typeable even when nothing on the
// stage drags it.
type poseField struct {
	label string
	prop  string
	comp  int // which component of the property, for the vector members
	dim   int // how many components the property must have to be writable
}

var poseFields = []poseField{
	{"rotation °", "r", 0, 1},
	{"position x", "p", 0, 2},
	{"position y", "p", 1, 2},
	{"scale x %", "s", 0, 2},
	{"scale y %", "s", 1, 2},
	{"opacity %", "o", 0, 1},
	{"anchor x", "a", 0, 2},
	{"anchor y", "a", 1, 2},
}

// buildPosePane shows the values stored at the selected keyframe — not the
// interpolated ones, so what is typed is what lands in the file. This pane
// is also the way a pose found by dragging leaves the editor: for the
// repository's own presets the generator stays the source of truth, and
// these numbers are what gets transcribed back into it.
func (c *inspectorContent) buildPosePane(context *guigui.Context, m *Model, adder *guigui.ChildAdder) {
	c.buildPartsList(context, m)
	setBold(&c.poseTitle, "Pose")
	frame, onKey := m.SelectedPoseKey()
	part := m.SelectedPosePart()
	editable := onKey && part >= 0 && !m.Viewer()

	label(&c.posePartLabel, "part")
	label(&c.poseJointLabel, "joint")
	label(&c.poseEaseLabel, "ease")
	label(&c.poseParentLabel, "parent")
	label(&c.poseJDragLabel, "joint drag")
	label(&c.poseFrameLabel, "frame")
	c.poseFrameValue.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	// The part row is its name, and the name is what everything outside this
	// clip uses to find the layer, so it is editable here.
	c.posePartInput.SetValue(m.SelectedPosePartName())
	c.posePartInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed {
			m.RenamePosePart(text)
		}
	})
	context.SetEnabled(&c.posePartInput, part >= 0 && !m.Viewer())
	c.buildJointPicker(context, m)
	c.buildEasePicker(context, m, onKey)
	c.buildParentPicker(context, m, onKey)
	if onKey {
		c.poseFrameValue.SetValue(strconv.FormatFloat(frame, 'f', -1, 64))
	} else {
		c.poseFrameValue.SetValue("—")
	}

	c.poseLabels.SetLen(len(poseFields))
	c.poseInputs.SetLen(len(poseFields))
	for i, f := range poseFields {
		lb, in := c.poseLabels.At(i), c.poseInputs.At(i)
		adder.AddWidget(lb)
		adder.AddWidget(in)
		label(lb, f.label)

		v, ok := m.PoseValue(f.prop)
		writable := editable && ok && len(v) >= f.dim
		if writable {
			in.SetValue(strconv.FormatFloat(v[f.comp], 'g', -1, 64))
		} else {
			in.SetValue("")
		}
		in.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
			if !committed {
				return
			}
			n, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
			if err != nil {
				return
			}
			cur, ok := m.PoseValue(f.prop)
			if !ok || len(cur) <= f.comp {
				return
			}
			next := slices.Clone(cur)
			next[f.comp] = n
			m.SetPoseValue(f.prop, next)
		})
		context.SetEnabled(in, writable)
	}

	// A drag writes into the document on every mouse move, so the way back
	// is a button here: the editor has no keyboard shortcuts to hang it on.
	c.poseUndo.SetText("Undo pose edit")
	c.poseUndo.OnDown(func(context *guigui.Context) { m.UndoClipEdit() })
	context.SetEnabled(&c.poseUndo, m.CanUndoClipEdit() && !m.Viewer())

	// The hint carries the one rule that is not visible on the stage: an
	// edit needs a key under the playhead.
	switch {
	case m.StageClipDoc() == nil:
		c.poseHint.SetValue("No clip on stage to pose.")
	case part >= 0 && m.PosePartNameProblem() != "":
		// The stage finds a layer by name and takes the first match, so an
		// unnamed or duplicated one would be dragged in the wrong space.
		c.poseHint.SetValue("Cannot drag on the stage: " +
			m.PosePartNameProblem() + ". Rename it above; the numbers below still work.")
	case !onKey:
		c.poseHint.SetValue("Click a keyframe on the Poses row to edit; " +
			"values are only written at a key.")
	case part < 0:
		c.poseHint.SetValue("Click a part on the stage to pose it.")
	case !m.PoseValueIsKeyed("r"):
		c.poseHint.SetValue("Rotation is static across this clip; " +
			"editing it keys every pose.")
	default:
		c.poseHint.SetValue("Drag the part to swing it, the joint mark to move it.")
	}
	c.poseHint.SetScale(0.85)

	// Tab walks the form: name, then the numbers in the order they read.
	c.tabFields = append(c.tabFields[:0], &c.posePartInput)
	for i := range poseFields {
		c.tabFields = append(c.tabFields, c.poseInputs.At(i))
	}
	tabOrder(context, c.tabFields...)

	c.poseFormItems = slices.Delete(c.poseFormItems, 0, len(c.poseFormItems))
	c.poseFormItems = append(c.poseFormItems,
		basicwidget.FormItem{PrimaryWidget: &c.posePartLabel, SecondaryWidget: &c.posePartInput},
		basicwidget.FormItem{PrimaryWidget: &c.poseJointLabel, SecondaryWidget: &c.poseJointSel},
		basicwidget.FormItem{PrimaryWidget: &c.poseParentLabel, SecondaryWidget: &c.poseParentSel},
		basicwidget.FormItem{PrimaryWidget: &c.poseJDragLabel, SecondaryWidget: &c.poseJDragSel},
		basicwidget.FormItem{PrimaryWidget: &c.poseFrameLabel, SecondaryWidget: &c.poseFrameValue},
		basicwidget.FormItem{PrimaryWidget: &c.poseEaseLabel, SecondaryWidget: &c.poseEaseSel},
	)
	for i := range poseFields {
		c.poseFormItems = append(c.poseFormItems, basicwidget.FormItem{
			PrimaryWidget:   c.poseLabels.At(i),
			SecondaryWidget: c.poseInputs.At(i),
		})
	}
	c.poseForm.SetItems(c.poseFormItems)
}

// buildPartsList lists every part of the rig, selected either way: clicking
// a row picks the part on the stage, and picking one on the stage highlights
// its row.
//
// The list exists because the stage cannot offer every part. A rig layers
// parts over each other — the far arm sits behind the torso — and swaps
// others out by opacity, so a click can only ever reach what is on top and
// visible. Those are the parts that need posing least.
func (c *inspectorContent) buildPartsList(context *guigui.Context, m *Model) {
	setBold(&c.partsTitle, "Parts")
	parts := m.PoseParts()

	c.partsItems = slices.Delete(c.partsItems, 0, len(c.partsItems))
	for _, i := range parts {
		// The joint tells you what a part does; "hidden" tells you why it is
		// not on the stage to be clicked.
		note := poseJoints[m.PoseLayerName(i)]
		if m.PosePartHidden(i) {
			if note != "" {
				note += " · "
			}
			note += "hidden"
		}
		c.partsItems = append(c.partsItems, basicwidget.ListItem[int]{
			Text: m.PoseLayerName(i), KeyText: note, Value: i,
			Movable: m.CanReorderParts(),
		})
	}
	c.partsList.SetItems(c.partsItems)

	// The list order is the draw order, so dragging a row is how the overlap
	// changes: the gripping forearm in front of the torso during a swing,
	// behind it at rest. The footer does the same one place at a time, for
	// when a drag is more precision than the job needs.
	reorder := m.CanReorderParts()
	c.partsList.OnItemsMoved(func(context *guigui.Context, from, count, to int) {
		if count == 1 {
			m.ReorderPosePartTo(from, to)
		}
	})
	sel := m.SelectedPosePart()
	c.partsFront.SetText("▲ Front")
	c.partsFront.OnDown(func(context *guigui.Context) { m.ReorderPosePart(-1) })
	c.partsBack.SetText("▼ Back")
	c.partsBack.OnDown(func(context *guigui.Context) { m.ReorderPosePart(1) })
	i := slices.Index(parts, sel)
	context.SetEnabled(&c.partsFront, reorder && i > 0)
	context.SetEnabled(&c.partsBack, reorder && i >= 0 && i < len(parts)-1)

	// Which drawing of a slot is showing — a head seen from another angle,
	// say — is opacity, and opacity is per key like every other pose value.
	hideText := "Hide"
	if sel >= 0 && m.PosePartHidden(sel) {
		hideText = "Show"
	}
	c.partsHide.SetText(hideText)
	c.partsHide.OnDown(func(context *guigui.Context) { m.TogglePosePartHidden() })
	_, onKey := m.SelectedPoseKey()
	context.SetEnabled(&c.partsHide, sel >= 0 && onKey && !m.Viewer())

	if i >= 0 {
		c.partsList.SelectItemByValue(sel)
		// A part picked on the stage may be well down the list, and a
		// highlight nobody can see is not a highlight.
		if c.partsShownPlus1 != sel+1 {
			c.partsList.EnsureItemVisibleByIndex(i)
			c.partsShownPlus1 = sel + 1
		}
	} else {
		c.partsShownPlus1 = 0
	}
	c.partsList.OnItemSelected(func(context *guigui.Context, index int) {
		if index >= 0 && index < len(parts) && parts[index] != m.SelectedPosePart() {
			m.SelectPosePart(parts[index])
		}
	})
}

// buildJointPicker turns the joint readout into a way to change what is
// being posed. It names the parts the way the rig does — arms(near) rather
// than upper-arm-near — so a pose written in the generator's vocabulary can
// be followed here without translating it in your head. Parts with no joint
// of their own are listed by layer name so the picker is never a dead end.
func (c *inspectorContent) buildJointPicker(context *guigui.Context, m *Model) {
	parts := m.PoseParts()
	c.jointItems = slices.Delete(c.jointItems, 0, len(c.jointItems))
	for _, i := range parts {
		text := poseJoints[m.PoseLayerName(i)]
		if text == "" {
			text = m.PoseLayerName(i)
		}
		c.jointItems = append(c.jointItems, basicwidget.SelectItem[int]{Text: text, Value: i})
	}
	c.poseJointSel.SetItems(c.jointItems)
	if sel := m.SelectedPosePart(); slices.Contains(parts, sel) {
		c.poseJointSel.SelectItemByValue(sel)
	}
	c.poseJointSel.OnItemSelected(func(context *guigui.Context, index int) {
		if index >= 0 && index < len(parts) && parts[index] != m.SelectedPosePart() {
			m.SelectPosePart(parts[index])
		}
	})
	context.SetEnabled(&c.poseJointSel, len(parts) > 0)
}

// buildEasePicker sets whether the selected pose arrives on a curve. It is a
// property of the pose, not of one limb: a body easing in while its arm
// arrives linearly reads as a mistake, so the whole column moves together.
func (c *inspectorContent) buildEasePicker(context *guigui.Context, m *Model, onKey bool) {
	c.easeItems = slices.Delete(c.easeItems, 0, len(c.easeItems))
	c.easeItems = append(c.easeItems,
		basicwidget.SelectItem[bool]{Text: "linear", Value: false},
		basicwidget.SelectItem[bool]{Text: "eased", Value: true},
	)
	c.poseEaseSel.SetItems(c.easeItems)
	c.poseEaseSel.SelectItemByValue(m.PoseEased())
	c.poseEaseSel.OnItemSelected(func(context *guigui.Context, index int) {
		if item, ok := c.poseEaseSel.ItemByIndex(index); ok {
			m.SetPoseEase(item.Value)
		}
	})
	context.SetEnabled(&c.poseEaseSel, onKey && !m.Viewer())
}

// buildParentPicker changes what a part hangs from, and how its joint drags.
//
// The candidates leave out the part's own descendants, so a cycle is not
// something to warn about after the fact — it is not in the list. Attaching
// rewrites the transform so the part does not jump, which only makes sense
// at a key, since that is where the values live.
func (c *inspectorContent) buildParentPicker(context *guigui.Context, m *Model, onKey bool) {
	cands := m.PoseParentCandidates()
	c.parentItems = slices.Delete(c.parentItems, 0, len(c.parentItems))
	// -1 is the composition itself: the body and the shadow ride on it, and
	// anything detached joins them.
	c.parentItems = append(c.parentItems, basicwidget.SelectItem[int]{Text: "(none)", Value: -1})
	for _, i := range cands {
		c.parentItems = append(c.parentItems, basicwidget.SelectItem[int]{
			Text: m.PoseLayerName(i), Value: i,
		})
	}
	c.poseParentSel.SetItems(c.parentItems)
	cur := -1
	if p, ok := m.PosePartParent(); ok {
		cur = p
	}
	c.poseParentSel.SelectItemByValue(cur)
	c.poseParentSel.OnItemSelected(func(context *guigui.Context, index int) {
		if item, ok := c.poseParentSel.ItemByIndex(index); ok && item.Value != cur {
			m.SetPosePartParent(item.Value)
		}
	})
	context.SetEnabled(&c.poseParentSel, onKey && m.SelectedPosePart() >= 0 && !m.Viewer())

	// Both readings of a joint drag are wanted: an attach point in the wrong
	// place needs the part to follow, a limb turning about the wrong pixel
	// needs it to stay.
	c.jdragItems = slices.Delete(c.jdragItems, 0, len(c.jdragItems))
	c.jdragItems = append(c.jdragItems,
		basicwidget.SelectItem[bool]{Text: "moves part", Value: false},
		basicwidget.SelectItem[bool]{Text: "keeps art", Value: true},
	)
	c.poseJDragSel.SetItems(c.jdragItems)
	c.poseJDragSel.SelectItemByValue(m.JointDragKeepsArt())
	c.poseJDragSel.OnItemSelected(func(context *guigui.Context, index int) {
		if item, ok := c.poseJDragSel.ItemByIndex(index); ok {
			m.SetJointDragKeepsArt(item.Value)
		}
	})
}

// tabOrder wires Tab to walk a form's fields, and Shift-Tab to walk back.
//
// Moving on is also how the value lands: a text input commits when it loses
// focus, so tabbing out of a field is the same as pressing Enter in it.
// Nothing in guigui claims Tab, so a field would otherwise swallow it and
// the only way out of one would be the mouse.
//
// Disabled fields are stepped over rather than focused, or Tab would strand
// the caret somewhere that cannot be typed into.
func tabOrder(context *guigui.Context, fields ...*basicwidget.TextInput) {
	for i, in := range fields {
		step := func(dir int) *basicwidget.TextInput {
			for n := 1; n <= len(fields); n++ {
				cand := fields[((i+dir*n)%len(fields)+len(fields))%len(fields)]
				if context.IsEnabled(cand) {
					return cand
				}
			}
			return nil
		}
		in.OnHandleButtonInput(func(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
			if !inpututil.IsKeyJustPressed(ebiten.KeyTab) {
				return guigui.HandleInputResult{}
			}
			dir := 1
			if ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight) {
				dir = -1
			}
			target := step(dir)
			if target == nil {
				return guigui.HandleInputResult{}
			}
			context.SetFocused(target, true)
			return guigui.HandleInputByWidget(in)
		})
	}
}
