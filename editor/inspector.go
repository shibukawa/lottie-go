package main

import (
	"fmt"
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

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
	typeCombo, animCombo, segCombo  basicwidget.Combobox
	modeCombo                       basicwidget.Combobox
	speedInput                      basicwidget.TextInput
	loopCheck, autoplayCheck        basicwidget.Checkbox
	form                            basicwidget.Form

	transTitle  basicwidget.Text
	transList   basicwidget.List[int]
	transTarget basicwidget.Combobox
	addTrans    basicwidget.Button
	upTrans     basicwidget.Button
	downTrans   basicwidget.Button
	delTrans    basicwidget.Button

	guardTitle basicwidget.Text
	guardList  basicwidget.List[int]
	addGuard   basicwidget.Button
	delGuard   basicwidget.Button
	guardType  basicwidget.Combobox
	guardInput basicwidget.Combobox
	guardCond  basicwidget.Combobox
	guardValue basicwidget.TextInput
	guardForm  basicwidget.Form

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

	// Socket pane.
	sockTitle      basicwidget.Text
	sockNameLabel  basicwidget.Text
	sockLayerLabel basicwidget.Text
	sockLayerValue basicwidget.Text
	sockNameInput  basicwidget.TextInput
	sockZBtn       basicwidget.Button
	sockForm       basicwidget.Form
	sockFormItems  []basicwidget.FormItem

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
		c.buildSocketPane(context, m)
	default:
		for _, w := range []guigui.Widget{
			&c.stateTitle, &c.addState, &c.delState, &c.setInitial, &c.form,
			&c.transTitle, &c.transList, &c.transTarget, &c.addTrans,
			&c.upTrans, &c.downTrans, &c.delTrans,
			&c.guardTitle, &c.guardList, &c.addGuard, &c.delGuard, &c.guardForm,
		} {
			adder.AddWidget(w)
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
		context.SetEnabled(w, current != "")
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
		context.SetEnabled(w, b != nil)
	}
	for _, w := range []guigui.Widget{&c.hbFromInput, &c.hbToInput, &c.hbDelSpan} {
		context.SetEnabled(w, sp != nil)
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
		context.SetEnabled(w, s != nil)
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
	c.sockLayerValue.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	if s != nil {
		c.sockNameInput.SetValue(s.Name)
		c.sockLayerValue.SetValue(s.LayerName())
	} else {
		c.sockNameInput.SetValue("")
		c.sockLayerValue.SetValue("")
	}
	c.sockNameInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed {
			m.RenameSocket(text)
		}
	})

	zText := "z: front"
	if s != nil && s.Z == lottiesockets.ZBehind {
		zText = "z: behind"
	}
	c.sockZBtn.SetText(zText)
	c.sockZBtn.OnDown(func(context *guigui.Context) { m.ToggleSocketZ() })

	for _, w := range []guigui.Widget{&c.sockNameInput, &c.sockZBtn} {
		context.SetEnabled(w, s != nil)
	}

	c.sockFormItems = slices.Delete(c.sockFormItems, 0, len(c.sockFormItems))
	c.sockFormItems = append(c.sockFormItems,
		basicwidget.FormItem{PrimaryWidget: &c.sockNameLabel, SecondaryWidget: &c.sockNameInput},
		basicwidget.FormItem{PrimaryWidget: &c.sockLayerLabel, SecondaryWidget: &c.sockLayerValue},
	)
	c.sockForm.SetItems(c.sockFormItems)
}

func (c *inspectorContent) buildStateForm(context *guigui.Context, m *Model, st *lottie.State) {
	c.addState.SetText("Add state")
	c.addState.OnDown(func(context *guigui.Context) { m.AddState() })
	c.delState.SetText("Delete")
	c.delState.OnDown(func(context *guigui.Context) { m.DeleteState(m.SelectedStateName()) })
	c.setInitial.SetText("Set initial")
	c.setInitial.OnDown(func(context *guigui.Context) { m.SetInitial(m.SelectedStateName()) })
	context.SetEnabled(&c.delState, st != nil)
	context.SetEnabled(&c.setInitial, st != nil)

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
		&c.nameInput, &c.typeCombo, &c.animCombo, &c.segCombo,
		&c.modeCombo, &c.speedInput, &c.loopCheck, &c.autoplayCheck,
	} {
		context.SetEnabled(w, st != nil)
	}
	for _, w := range []guigui.Widget{&c.animCombo, &c.segCombo, &c.modeCombo, &c.speedInput, &c.loopCheck, &c.autoplayCheck} {
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

	c.typeCombo.SetItems([]string{string(lottie.StatePlayback), string(lottie.StateGlobal)})
	c.typeCombo.SetValue(string(st.Type))
	c.typeCombo.OnValueChanged(func(context *guigui.Context, value string, committed bool) {
		if committed && value != string(st.Type) {
			st.Type = lottie.StateType(value)
			m.Touch()
		}
	})

	c.animCombo.SetItems(m.AnimationIDs())
	c.animCombo.SetValue(st.Animation)
	c.animCombo.OnValueChanged(func(context *guigui.Context, value string, committed bool) {
		if committed && value != st.Animation {
			st.Animation = value
			st.Segment = ""
			m.Touch()
		}
	})

	c.segCombo.SetItems(append([]string{""}, m.Markers(st.Animation)...))
	c.segCombo.SetValue(st.Segment)
	c.segCombo.OnValueChanged(func(context *guigui.Context, value string, committed bool) {
		if committed && value != st.Segment {
			st.Segment = value
			m.Touch()
		}
	})

	c.modeCombo.SetItems([]string{string(lottie.PlayForward), string(lottie.PlayReverse)})
	c.modeCombo.SetValue(string(st.PlaybackMode()))
	c.modeCombo.OnValueChanged(func(context *guigui.Context, value string, committed bool) {
		if committed && value != string(st.Mode) {
			st.Mode = lottie.PlayMode(value)
			m.Touch()
		}
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
		basicwidget.FormItem{PrimaryWidget: &c.typeLabel, SecondaryWidget: &c.typeCombo},
		basicwidget.FormItem{PrimaryWidget: &c.animLabel, SecondaryWidget: &c.animCombo},
		basicwidget.FormItem{PrimaryWidget: &c.segLabel, SecondaryWidget: &c.segCombo},
		basicwidget.FormItem{PrimaryWidget: &c.modeLabel, SecondaryWidget: &c.modeCombo},
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

	c.transTarget.SetItems(m.StateNames())
	c.addTrans.SetText("Add →")
	c.addTrans.OnDown(func(context *guigui.Context) {
		to := c.transTarget.Value()
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

	var g *lottie.Guard
	if tr != nil && c.selectedGuard >= 0 && c.selectedGuard < len(tr.Guards) {
		g = &tr.Guards[c.selectedGuard]
	}
	for _, w := range []guigui.Widget{&c.guardType, &c.guardInput, &c.guardCond, &c.guardValue} {
		context.SetEnabled(w, g != nil)
	}

	c.guardType.SetItems([]string{
		string(lottie.GuardEvent), string(lottie.GuardBoolean),
		string(lottie.GuardNumeric), string(lottie.GuardString),
	})
	c.guardCond.SetItems([]string{
		string(lottie.ConditionEqual), string(lottie.ConditionNotEqual),
		string(lottie.ConditionGreaterThan), string(lottie.ConditionGreaterThanOrEqual),
		string(lottie.ConditionLessThan), string(lottie.ConditionLessThanOrEqual),
	})
	c.guardInput.SetItems(m.InputNames())

	if g == nil {
		c.guardType.SetValue("")
		c.guardInput.SetValue("")
		c.guardCond.SetValue("")
		c.guardValue.SetValue("")
	} else {
		c.guardType.SetValue(string(g.Type))
		c.guardType.OnValueChanged(func(context *guigui.Context, value string, committed bool) {
			if committed {
				g.Type = lottie.GuardType(value)
				m.Touch()
			}
		})
		c.guardInput.SetValue(g.InputName)
		c.guardInput.OnValueChanged(func(context *guigui.Context, value string, committed bool) {
			if committed {
				g.InputName = value
				m.Touch()
			}
		})
		c.guardCond.SetValue(string(g.ConditionType))
		c.guardCond.OnValueChanged(func(context *guigui.Context, value string, committed bool) {
			if committed {
				g.ConditionType = lottie.ConditionType(value)
				m.Touch()
			}
		})
		c.guardValue.SetValue(string(g.CompareTo))
		c.guardValue.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
			if committed {
				g.CompareTo = parseCompareTo(g.Type, text)
				m.Touch()
			}
		})
		// Event guards have no comparison at all.
		isEvent := g.Type == lottie.GuardEvent
		context.SetEnabled(&c.guardCond, !isEvent)
		context.SetEnabled(&c.guardValue, !isEvent)
	}

	c.guardFormItems = slices.Delete(c.guardFormItems, 0, len(c.guardFormItems))
	c.guardFormItems = append(c.guardFormItems,
		basicwidget.FormItem{PrimaryWidget: &c.guardType, SecondaryWidget: &c.guardInput},
		basicwidget.FormItem{PrimaryWidget: &c.guardCond, SecondaryWidget: &c.guardValue},
	)
	c.guardForm.SetItems(c.guardFormItems)
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
// guard sections.
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
	return append(items,
		guigui.LinearLayoutItem{Widget: &c.stateTitle, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &c.stateBtnRow},
		guigui.LinearLayoutItem{Widget: &c.form},
		guigui.LinearLayoutItem{Widget: &c.transTitle, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &c.transList, Size: guigui.FixedSize(5 * u)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &c.transBtnRow},
		guigui.LinearLayoutItem{Widget: &c.guardTitle, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &c.guardList, Size: guigui.FixedSize(4 * u)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &c.guardBtnRow},
		guigui.LinearLayoutItem{Widget: &c.guardForm},
	)
}

func (c *inspectorContent) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	c.layout(context).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (c *inspectorContent) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return c.layout(context).Measure(context, constraints)
}
