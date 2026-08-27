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
)

// inspectorPane is the parameter pane for whatever is selected: a node or
// the scene itself. It scrolls, since the node sections together are
// taller than most panes.
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

	// Scene pane.
	scTitle      basicwidget.Text
	scNameLabel  basicwidget.Text
	scWLabel     basicwidget.Text
	scHLabel     basicwidget.Text
	scHoverLabel basicwidget.Text
	scFocusLabel basicwidget.Text
	scNameInput  basicwidget.TextInput
	scWInput     basicwidget.TextInput
	scHInput     basicwidget.TextInput
	scHoverCheck basicwidget.Checkbox
	scFocusSel   basicwidget.Select[string]
	scForm       basicwidget.Form
	scFormItems  []basicwidget.FormItem

	// Phase editing, inside the scene pane.
	phTitle     basicwidget.Text
	phList      basicwidget.List[int]
	phAdd       basicwidget.Button
	phDel       basicwidget.Button
	phNameLabel basicwidget.Text
	phDurLabel  basicwidget.Text
	phNextLabel basicwidget.Text
	phNameInput basicwidget.TextInput
	phDurInput  basicwidget.TextInput
	phNextSel   basicwidget.Select[string]
	phForm      basicwidget.Form
	phFormItems []basicwidget.FormItem
	phItems     []basicwidget.ListItem[int]
	phBtnRow    guigui.LinearLayout
	phBtnItems  []guigui.LinearLayoutItem

	// Node pane.
	ndTitle       basicwidget.Text
	ndNameLabel   basicwidget.Text
	ndSourceLabel basicwidget.Text
	ndPlaysLabel  basicwidget.Text
	ndStartLabel  basicwidget.Text
	ndPhaseLabel  basicwidget.Text
	ndNameInput   basicwidget.TextInput
	ndSourceValue basicwidget.Text
	ndPlaysSel    basicwidget.Select[string]
	ndStartInput  basicwidget.TextInput
	ndPhaseSel    basicwidget.Select[string]
	ndForm        basicwidget.Form
	ndFormItems   []basicwidget.FormItem

	// Playback chain (animation nodes): what plays after the clip ends.
	thTitle     basicwidget.Text
	thList      basicwidget.List[int]
	thAdd       basicwidget.Button
	thDel       basicwidget.Button
	tsAnimLabel basicwidget.Text
	tsSegLabel  basicwidget.Text
	tsLoopLabel basicwidget.Text
	tsAnimSel   basicwidget.Select[string]
	tsSegSel    basicwidget.Select[string]
	tsLoopCheck basicwidget.Checkbox
	tsForm      basicwidget.Form
	tsFormItems []basicwidget.FormItem
	thItems     []basicwidget.ListItem[int]
	thBtnRow    guigui.LinearLayout
	thBtnItems  []guigui.LinearLayoutItem

	// Text pane (text nodes).
	txTitle      basicwidget.Text
	txValueLabel basicwidget.Text
	txFontLabel  basicwidget.Text
	txSizeLabel  basicwidget.Text
	txAlignLabel basicwidget.Text
	txAxLabel    basicwidget.Text
	txAyLabel    basicwidget.Text
	txColorLabel basicwidget.Text
	txValueInput basicwidget.TextInput
	txFontSel    basicwidget.Select[string]
	txSizeInput  basicwidget.TextInput
	txAlignSel   basicwidget.Select[string]
	txAxSel      basicwidget.Select[string]
	txAySel      basicwidget.Select[string]
	txColorInput basicwidget.TextInput
	txForm       basicwidget.Form
	txFormItems  []basicwidget.FormItem

	tfTitle     basicwidget.Text
	tfLabels    [6]basicwidget.Text
	tfInputs    [6]basicwidget.TextInput
	tfForm      basicwidget.Form
	tfFormItems []basicwidget.FormItem

	// Playback (animation nodes).
	pbTitle      basicwidget.Text
	pbSegLabel   basicwidget.Text
	pbLoopLabel  basicwidget.Text
	pbCountLabel basicwidget.Text
	pbSpeedLabel basicwidget.Text
	pbModeLabel  basicwidget.Text
	pbAutoLabel  basicwidget.Text
	pbSegSel     basicwidget.Select[string]
	pbLoopCheck  basicwidget.Checkbox
	pbCountInput basicwidget.TextInput
	pbSpeedInput basicwidget.TextInput
	pbModeSel    basicwidget.Select[string]
	pbAutoCheck  basicwidget.Checkbox
	pbForm       basicwidget.Form
	pbFormItems  []basicwidget.FormItem

	// Entry (machine nodes).
	enLabel basicwidget.Text
	enSel   basicwidget.Select[string]

	// Focus.
	fcTitle     basicwidget.Text
	fcAbleLabel basicwidget.Text
	fcTabLabel  basicwidget.Text
	fcDirLabels [4]basicwidget.Text
	fcAbleCheck basicwidget.Checkbox
	fcTabInput  basicwidget.TextInput
	fcDirSels   [4]basicwidget.Select[string]
	fcForm      basicwidget.Form
	fcFormItems []basicwidget.FormItem

	// Bindings.
	bdTitle       basicwidget.Text
	bdList        basicwidget.List[int]
	bdAdd         basicwidget.Button
	bdDel         basicwidget.Button
	bdOnLabel     basicwidget.Text
	bdDoLabel     basicwidget.Text
	bdTargetLabel basicwidget.Text
	bdArgLabel    basicwidget.Text
	bdOnSel       basicwidget.Select[string]
	bdDoSel       basicwidget.Select[string]
	bdTargetSel   basicwidget.Select[string]
	bdArgSel      basicwidget.Select[string]
	bdArgInput    basicwidget.TextInput
	bdForm        basicwidget.Form
	bdFormItems   []basicwidget.FormItem
	bdItems       []basicwidget.ListItem[int]

	problemsTitle basicwidget.Text
	problems      basicwidget.Text

	bdBtnRow   guigui.LinearLayout
	bdBtnItems []guigui.LinearLayoutItem
	items      []guigui.LinearLayoutItem
}

func (c *inspectorContent) model(context *guigui.Context) *Model {
	v, ok := context.Env(c, envKeyModel)
	if !ok {
		return nil
	}
	m, _ := v.(*Model)
	return m
}

// bindingArgIsPicker reports whether the binding's argument comes from a
// closed set: machine events or clip markers. Callback names are free
// text.
func bindingArgIsPicker(b *lottie.SceneBinding) bool {
	return b != nil && b.Do != lottie.SceneCallback
}

func (c *inspectorContent) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m := c.model(context)
	if m == nil {
		return nil
	}
	switch m.InspectTarget() {
	case inspectScene:
		for _, w := range []guigui.Widget{
			&c.scTitle, &c.scForm, &c.phTitle, &c.phList, &c.phAdd, &c.phDel,
		} {
			adder.AddWidget(w)
		}
		if m.SelectedPhase() != nil {
			adder.AddWidget(&c.phForm)
		}
		c.buildScenePane(context, m)
		c.buildPhases(context, m)
	default:
		for _, w := range []guigui.Widget{
			&c.ndTitle, &c.ndForm, &c.tfTitle, &c.tfForm,
		} {
			adder.AddWidget(w)
		}
		n := m.SelectedNode()
		if n != nil && n.Kind == lottie.SceneNodeAnimation {
			adder.AddWidget(&c.pbTitle)
			adder.AddWidget(&c.pbForm)
			adder.AddWidget(&c.thTitle)
			adder.AddWidget(&c.thList)
			adder.AddWidget(&c.thAdd)
			adder.AddWidget(&c.thDel)
			if m.SelectedStep() != nil {
				adder.AddWidget(&c.tsForm)
			}
			c.buildChain(context, m, n)
		}
		if n != nil && n.Kind == lottie.SceneNodeText {
			adder.AddWidget(&c.txTitle)
			adder.AddWidget(&c.txForm)
			c.buildTextPane(context, m, n)
		}
		for _, w := range []guigui.Widget{
			&c.fcTitle, &c.fcForm,
			&c.bdTitle, &c.bdList, &c.bdAdd, &c.bdDel,
		} {
			adder.AddWidget(w)
		}
		if m.SelectedBinding() != nil {
			adder.AddWidget(&c.bdForm)
		}
		c.buildNodePane(context, m, n)
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

func (c *inspectorContent) buildScenePane(context *guigui.Context, m *Model) {
	setBold(&c.scTitle, "Scene")
	s := m.Scene()

	label(&c.scNameLabel, "name")
	label(&c.scWLabel, "width")
	label(&c.scHLabel, "height")
	label(&c.scHoverLabel, "hover moves focus")
	label(&c.scFocusLabel, "initial focus")

	c.scNameInput.SetValue(s.Name)
	c.scNameInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed {
			m.scene.Name = strings.TrimSpace(text)
			m.touchLight()
		}
	})
	c.scWInput.SetValue(strconv.Itoa(s.Size.W))
	c.scHInput.SetValue(strconv.Itoa(s.Size.H))
	commitSize := func() {
		w, err1 := strconv.Atoi(strings.TrimSpace(c.scWInput.Value()))
		h, err2 := strconv.Atoi(strings.TrimSpace(c.scHInput.Value()))
		if err1 == nil && err2 == nil && w > 0 && h > 0 {
			m.scene.Size = lottie.SceneSize{W: w, H: h}
			m.touch()
		}
	}
	c.scWInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed {
			commitSize()
		}
	})
	c.scHInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed {
			commitSize()
		}
	})

	c.scHoverCheck.SetValue(s.Options.HoverMovesFocus == nil || *s.Options.HoverMovesFocus)
	c.scHoverCheck.OnValueChanged(func(context *guigui.Context, value bool) {
		v := value
		m.scene.Options.HoverMovesFocus = &v
		m.touch()
	})

	setOptionsWithDefault(&c.scFocusSel, "(lowest tab index)", m.FocusableNodeNames()...)
	c.scFocusSel.SelectItemByValue(s.Options.InitialFocus)
	c.scFocusSel.OnItemSelected(func(context *guigui.Context, index int) {
		it, ok := c.scFocusSel.ItemByIndex(index)
		if !ok || it.Value == m.scene.Options.InitialFocus {
			return
		}
		m.scene.Options.InitialFocus = it.Value
		m.touch()
	})

	c.scFormItems = slices.Delete(c.scFormItems, 0, len(c.scFormItems))
	c.scFormItems = append(c.scFormItems,
		basicwidget.FormItem{PrimaryWidget: &c.scNameLabel, SecondaryWidget: &c.scNameInput},
		basicwidget.FormItem{PrimaryWidget: &c.scWLabel, SecondaryWidget: &c.scWInput},
		basicwidget.FormItem{PrimaryWidget: &c.scHLabel, SecondaryWidget: &c.scHInput},
		basicwidget.FormItem{PrimaryWidget: &c.scHoverLabel, SecondaryWidget: &c.scHoverCheck},
		basicwidget.FormItem{PrimaryWidget: &c.scFocusLabel, SecondaryWidget: &c.scFocusSel},
	)
	c.scForm.SetItems(c.scFormItems)
}

// buildPhases edits the scene's phase list: an intro, the screen, an
// outro, with durations and automatic advances.
func (c *inspectorContent) buildPhases(context *guigui.Context, m *Model) {
	setBold(&c.phTitle, "Phases  (first is where the scene starts)")
	phases := m.Scene().Phases
	c.phItems = slices.Delete(c.phItems, 0, len(c.phItems))
	for i, p := range phases {
		key := ""
		if p.Duration > 0 {
			key = fmt.Sprintf("%.2fs", p.Duration)
			if p.Next != "" {
				key += " → " + p.Next
			}
		}
		c.phItems = append(c.phItems, basicwidget.ListItem[int]{Text: p.Name, KeyText: key, Value: i})
	}
	c.phList.SetItems(c.phItems)
	if sel := m.SelectedPhaseIndex(); sel >= 0 && sel < len(phases) {
		c.phList.SelectItemByValue(sel)
	}
	c.phList.OnItemSelected(func(context *guigui.Context, index int) {
		m.SelectPhase(index)
	})
	c.phAdd.SetText("Add phase")
	c.phAdd.OnDown(func(context *guigui.Context) { m.AddPhase() })
	c.phDel.SetText("Delete")
	c.phDel.OnDown(func(context *guigui.Context) { m.DeletePhase(m.SelectedPhaseIndex()) })
	context.SetEnabled(&c.phDel, m.SelectedPhase() != nil)

	p := m.SelectedPhase()
	if p == nil {
		return
	}
	label(&c.phNameLabel, "name")
	label(&c.phDurLabel, "duration (s)")
	label(&c.phNextLabel, "then")
	idx := m.SelectedPhaseIndex()
	c.phNameInput.SetValue(p.Name)
	c.phNameInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed {
			m.RenamePhase(idx, text)
		}
	})
	c.phDurInput.SetValue(strconv.FormatFloat(p.Duration, 'g', -1, 64))
	c.phDurInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if !committed {
			return
		}
		if v, err := strconv.ParseFloat(strings.TrimSpace(text), 64); err == nil && v >= 0 {
			if p := m.SelectedPhase(); p != nil {
				p.Duration = v
				m.touch()
			}
		}
	})
	setOptionsWithDefault(&c.phNextSel, "(stay)", m.PhaseNames()...)
	c.phNextSel.SelectItemByValue(p.Next)
	c.phNextSel.OnItemSelected(func(context *guigui.Context, index int) {
		it, ok := c.phNextSel.ItemByIndex(index)
		p := m.SelectedPhase()
		if !ok || p == nil || it.Value == p.Next {
			return
		}
		p.Next = it.Value
		m.touch()
	})

	c.phFormItems = slices.Delete(c.phFormItems, 0, len(c.phFormItems))
	c.phFormItems = append(c.phFormItems,
		basicwidget.FormItem{PrimaryWidget: &c.phNameLabel, SecondaryWidget: &c.phNameInput},
		basicwidget.FormItem{PrimaryWidget: &c.phDurLabel, SecondaryWidget: &c.phDurInput},
		basicwidget.FormItem{PrimaryWidget: &c.phNextLabel, SecondaryWidget: &c.phNextSel},
	)
	c.phForm.SetItems(c.phFormItems)
}

// buildTextPane styles a text node: content, font, size, alignment, and
// the anchor its transform positions.
func (c *inspectorContent) buildTextPane(context *guigui.Context, m *Model, n *lottie.SceneNode) {
	setBold(&c.txTitle, "Text")
	label(&c.txValueLabel, "value")
	label(&c.txFontLabel, "font")
	label(&c.txSizeLabel, "size")
	label(&c.txAlignLabel, "align")
	label(&c.txAxLabel, "anchor x")
	label(&c.txAyLabel, "anchor y")
	label(&c.txColorLabel, "color")

	c.txValueInput.SetValue(n.Text.Value)
	c.txValueInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if !committed {
			return
		}
		if n := m.SelectedNode(); n != nil {
			n.Text.Value = text
			m.touch()
		}
	})
	setOptions(&c.txFontSel, m.FontAliases()...)
	c.txFontSel.SelectItemByValue(n.Text.Font)
	c.txFontSel.OnItemSelected(func(context *guigui.Context, index int) {
		it, ok := c.txFontSel.ItemByIndex(index)
		n := m.SelectedNode()
		if !ok || n == nil || it.Value == n.Text.Font {
			return
		}
		n.Text.Font = it.Value
		m.touch()
	})
	c.txSizeInput.SetValue(strconv.FormatFloat(n.Text.Size, 'g', -1, 64))
	c.txSizeInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if !committed {
			return
		}
		if v, err := strconv.ParseFloat(strings.TrimSpace(text), 64); err == nil && v > 0 {
			if n := m.SelectedNode(); n != nil {
				n.Text.Size = v
				m.touch()
			}
		}
	})
	alignSel := func(s *basicwidget.Select[string], get func(*lottie.SceneNode) string, set func(*lottie.SceneNode, string), opts ...string) {
		setOptions(s, opts...)
		s.SelectItemByValue(get(n))
		s.OnItemSelected(func(context *guigui.Context, index int) {
			it, ok := s.ItemByIndex(index)
			n := m.SelectedNode()
			if !ok || n == nil || it.Value == get(n) {
				return
			}
			set(n, it.Value)
			m.touch()
		})
	}
	left, center, right := string(lottie.AlignLeft), string(lottie.AlignCenter), string(lottie.AlignRight)
	alignSel(&c.txAlignSel,
		func(n *lottie.SceneNode) string { return string(orDefault(n.Text.Align, lottie.AlignLeft)) },
		func(n *lottie.SceneNode, v string) { n.Text.Align = lottie.SceneAlign(v) },
		left, center, right)
	alignSel(&c.txAxSel,
		func(n *lottie.SceneNode) string { return string(orDefault(n.Text.AnchorX, lottie.AlignLeft)) },
		func(n *lottie.SceneNode, v string) { n.Text.AnchorX = lottie.SceneAlign(v) },
		left, center, right)
	alignSel(&c.txAySel,
		func(n *lottie.SceneNode) string { return string(orDefault(n.Text.AnchorY, lottie.AlignTop)) },
		func(n *lottie.SceneNode, v string) { n.Text.AnchorY = lottie.SceneVAlign(v) },
		string(lottie.AlignTop), string(lottie.AlignMiddle), string(lottie.AlignBottom))
	c.txColorInput.SetValue(n.Text.Color)
	c.txColorInput.SetPlaceholder("#ffffff")
	c.txColorInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if !committed {
			return
		}
		text = strings.TrimSpace(text)
		if _, ok := lottie.ParseSceneColor(text); ok || text == "" {
			if n := m.SelectedNode(); n != nil {
				n.Text.Color = text
				m.touch()
			}
		}
	})

	c.txFormItems = slices.Delete(c.txFormItems, 0, len(c.txFormItems))
	c.txFormItems = append(c.txFormItems,
		basicwidget.FormItem{PrimaryWidget: &c.txValueLabel, SecondaryWidget: &c.txValueInput},
		basicwidget.FormItem{PrimaryWidget: &c.txFontLabel, SecondaryWidget: &c.txFontSel},
		basicwidget.FormItem{PrimaryWidget: &c.txSizeLabel, SecondaryWidget: &c.txSizeInput},
		basicwidget.FormItem{PrimaryWidget: &c.txAlignLabel, SecondaryWidget: &c.txAlignSel},
		basicwidget.FormItem{PrimaryWidget: &c.txAxLabel, SecondaryWidget: &c.txAxSel},
		basicwidget.FormItem{PrimaryWidget: &c.txAyLabel, SecondaryWidget: &c.txAySel},
		basicwidget.FormItem{PrimaryWidget: &c.txColorLabel, SecondaryWidget: &c.txColorInput},
	)
	c.txForm.SetItems(c.txFormItems)
}

// orDefault resolves an absent enum value to its default for display.
func orDefault[T ~string](v, def T) T {
	if v == "" {
		return def
	}
	return v
}

func (c *inspectorContent) buildNodePane(context *guigui.Context, m *Model, n *lottie.SceneNode) {
	title := "Node"
	if n != nil {
		title = "Node — " + string(n.Kind)
	}
	setBold(&c.ndTitle, title)

	label(&c.ndNameLabel, "name")
	label(&c.ndSourceLabel, "source")
	label(&c.ndPlaysLabel, "plays")
	label(&c.ndStartLabel, "start (s)")
	label(&c.ndPhaseLabel, "phase")
	bundleNode := n != nil && (n.Kind == lottie.SceneNodeAnimation || n.Kind == lottie.SceneNodeMachine)
	if n != nil {
		c.ndNameInput.SetValue(n.Name)
		switch n.Kind {
		case lottie.SceneNodeImage:
			c.ndSourceValue.SetValue("image / " + n.Source.Image)
		case lottie.SceneNodeText:
			c.ndSourceValue.SetValue("(text)")
		default:
			c.ndSourceValue.SetValue("bundle / " + n.Source.Bundle)
		}
		c.ndStartInput.SetValue(strconv.FormatFloat(n.Start, 'g', -1, 64))
	} else {
		c.ndNameInput.SetValue("")
		c.ndSourceValue.SetValue("")
		c.ndStartInput.SetValue("")
	}
	// What the one bundle node shows: any machine or any clip of its
	// bundle, one dropdown.
	if bundleNode {
		items := []basicwidget.SelectItem[string]{}
		for _, id := range m.BundleMachines(n) {
			items = append(items, basicwidget.SelectItem[string]{Text: "machine: " + id, Value: "m\x00" + id})
		}
		for _, id := range m.BundleAnimations(n) {
			items = append(items, basicwidget.SelectItem[string]{Text: "clip: " + id, Value: "a\x00" + id})
		}
		c.ndPlaysSel.SetItems(items)
		cur := "a\x00" + n.Source.ID
		if n.Kind == lottie.SceneNodeMachine {
			cur = "m\x00" + n.Source.ID
		}
		c.ndPlaysSel.SelectItemByValue(cur)
		idx := m.SelectedNodeIndex()
		c.ndPlaysSel.OnItemSelected(func(context *guigui.Context, index int) {
			it, ok := c.ndPlaysSel.ItemByIndex(index)
			if !ok {
				return
			}
			kindTag, id, found := strings.Cut(it.Value, "\x00")
			if !found {
				return
			}
			kind := lottie.SceneNodeAnimation
			if kindTag == "m" {
				kind = lottie.SceneNodeMachine
			}
			m.SetNodeContent(idx, kind, id)
		})
	}
	c.ndSourceValue.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	idx := m.SelectedNodeIndex()
	c.ndNameInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed {
			m.RenameNode(idx, text)
		}
	})
	c.ndStartInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if !committed {
			return
		}
		if v, err := strconv.ParseFloat(strings.TrimSpace(text), 64); err == nil && v >= 0 {
			m.SetNodeStart(idx, v)
			m.CommitNodeStart()
		}
	})

	c.ndFormItems = slices.Delete(c.ndFormItems, 0, len(c.ndFormItems))
	c.ndFormItems = append(c.ndFormItems,
		basicwidget.FormItem{PrimaryWidget: &c.ndNameLabel, SecondaryWidget: &c.ndNameInput},
		basicwidget.FormItem{PrimaryWidget: &c.ndSourceLabel, SecondaryWidget: &c.ndSourceValue},
	)
	if bundleNode {
		c.ndFormItems = append(c.ndFormItems,
			basicwidget.FormItem{PrimaryWidget: &c.ndPlaysLabel, SecondaryWidget: &c.ndPlaysSel})
	}
	c.ndFormItems = append(c.ndFormItems,
		basicwidget.FormItem{PrimaryWidget: &c.ndStartLabel, SecondaryWidget: &c.ndStartInput},
	)
	if len(m.PhaseNames()) > 0 {
		setOptionsWithDefault(&c.ndPhaseSel, "(every phase)", m.PhaseNames()...)
		if n != nil {
			c.ndPhaseSel.SelectItemByValue(n.Phase)
		}
		c.ndPhaseSel.OnItemSelected(func(context *guigui.Context, index int) {
			it, ok := c.ndPhaseSel.ItemByIndex(index)
			n := m.SelectedNode()
			if !ok || n == nil || it.Value == n.Phase {
				return
			}
			n.Phase = it.Value
			m.touch()
		})
		c.ndFormItems = append(c.ndFormItems,
			basicwidget.FormItem{PrimaryWidget: &c.ndPhaseLabel, SecondaryWidget: &c.ndPhaseSel})
	}
	if n != nil && n.Kind == lottie.SceneNodeMachine {
		label(&c.enLabel, "entry state")
		setOptionsWithDefault(&c.enSel, "(machine default)", m.MachineStates(n)...)
		c.enSel.SelectItemByValue(n.Entry)
		c.enSel.OnItemSelected(func(context *guigui.Context, index int) {
			it, ok := c.enSel.ItemByIndex(index)
			n := m.SelectedNode()
			if !ok || n == nil || it.Value == n.Entry {
				return
			}
			n.Entry = it.Value
			m.touch()
		})
		c.ndFormItems = append(c.ndFormItems,
			basicwidget.FormItem{PrimaryWidget: &c.enLabel, SecondaryWidget: &c.enSel})
	}
	c.ndForm.SetItems(c.ndFormItems)

	c.buildTransform(context, m, n)
	if n != nil && n.Kind == lottie.SceneNodeAnimation {
		c.buildPlayback(context, m, n)
	}
	c.buildFocus(context, m, n)
	c.buildBindings(context, m, n)

	enabled := n != nil
	context.SetEnabled(&c.ndNameInput, enabled)
	context.SetEnabled(&c.ndStartInput, enabled)
	for i := range c.tfInputs {
		context.SetEnabled(&c.tfInputs[i], enabled)
	}
}

// buildTransform wires the six placement fields. They patch the live node
// too, so typing a coordinate moves the canvas without restarting
// playback.
func (c *inspectorContent) buildTransform(context *guigui.Context, m *Model, n *lottie.SceneNode) {
	setBold(&c.tfTitle, "Transform")
	names := [6]string{"x", "y", "scale x", "scale y", "rotation °", "opacity"}
	get := func(tf lottie.SceneTransform) [6]float64 {
		return [6]float64{tf.X, tf.Y, tf.ScaleX, tf.ScaleY, tf.Rotation, tf.Opacity}
	}
	for i := range names {
		label(&c.tfLabels[i], names[i])
		if n != nil {
			c.tfInputs[i].SetValue(strconv.FormatFloat(get(n.Transform)[i], 'g', -1, 64))
		} else {
			c.tfInputs[i].SetValue("")
		}
	}
	idx := m.SelectedNodeIndex()
	commit := func() {
		n := m.SelectedNode()
		if n == nil {
			return
		}
		var vals [6]float64
		for i := range c.tfInputs {
			v, err := strconv.ParseFloat(strings.TrimSpace(c.tfInputs[i].Value()), 64)
			if err != nil {
				return
			}
			vals[i] = v
		}
		m.SetNodeTransform(idx, lottie.SceneTransform{
			X: vals[0], Y: vals[1], ScaleX: vals[2], ScaleY: vals[3],
			Rotation: vals[4], Opacity: vals[5],
		})
	}
	for i := range c.tfInputs {
		c.tfInputs[i].OnValueChanged(func(context *guigui.Context, text string, committed bool) {
			if committed {
				commit()
			}
		})
	}
	c.tfFormItems = slices.Delete(c.tfFormItems, 0, len(c.tfFormItems))
	for i := range names {
		c.tfFormItems = append(c.tfFormItems,
			basicwidget.FormItem{PrimaryWidget: &c.tfLabels[i], SecondaryWidget: &c.tfInputs[i]})
	}
	c.tfForm.SetItems(c.tfFormItems)
}

func (c *inspectorContent) buildPlayback(context *guigui.Context, m *Model, n *lottie.SceneNode) {
	setBold(&c.pbTitle, "Playback")
	label(&c.pbSegLabel, "segment")
	label(&c.pbLoopLabel, "loop")
	label(&c.pbCountLabel, "loop count")
	label(&c.pbSpeedLabel, "speed")
	label(&c.pbModeLabel, "mode")
	label(&c.pbAutoLabel, "autoplay")

	setOptionsWithDefault(&c.pbSegSel, "(whole clip)", m.Markers(n)...)
	c.pbSegSel.SelectItemByValue(n.Playback.Segment)
	c.pbSegSel.OnItemSelected(func(context *guigui.Context, index int) {
		it, ok := c.pbSegSel.ItemByIndex(index)
		n := m.SelectedNode()
		if !ok || n == nil || it.Value == n.Playback.Segment {
			return
		}
		n.Playback.Segment = it.Value
		m.touch()
	})
	c.pbLoopCheck.SetValue(n.Playback.Loop)
	c.pbLoopCheck.OnValueChanged(func(context *guigui.Context, value bool) {
		if n := m.SelectedNode(); n != nil {
			n.Playback.Loop = value
			m.touch()
		}
	})
	c.pbCountInput.SetValue(strconv.Itoa(n.Playback.LoopCount))
	c.pbCountInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if !committed {
			return
		}
		if v, err := strconv.Atoi(strings.TrimSpace(text)); err == nil && v >= 0 {
			if n := m.SelectedNode(); n != nil {
				n.Playback.LoopCount = v
				m.touch()
			}
		}
	})
	c.pbSpeedInput.SetValue(strconv.FormatFloat(n.Playback.PlaybackSpeed(), 'g', -1, 64))
	c.pbSpeedInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if !committed {
			return
		}
		if v, err := strconv.ParseFloat(strings.TrimSpace(text), 64); err == nil && v >= 0 {
			if n := m.SelectedNode(); n != nil {
				n.Playback.Speed = v
				m.touch()
			}
		}
	})
	setOptions(&c.pbModeSel, string(lottie.PlayForward), string(lottie.PlayReverse))
	c.pbModeSel.SelectItemByValue(string(n.Playback.PlaybackMode()))
	c.pbModeSel.OnItemSelected(func(context *guigui.Context, index int) {
		it, ok := c.pbModeSel.ItemByIndex(index)
		n := m.SelectedNode()
		if !ok || n == nil || it.Value == string(n.Playback.PlaybackMode()) {
			return
		}
		n.Playback.Mode = lottie.PlayMode(it.Value)
		m.touch()
	})
	c.pbAutoCheck.SetValue(n.Playback.Autoplay)
	c.pbAutoCheck.OnValueChanged(func(context *guigui.Context, value bool) {
		if n := m.SelectedNode(); n != nil {
			n.Playback.Autoplay = value
			m.touch()
		}
	})

	c.pbFormItems = slices.Delete(c.pbFormItems, 0, len(c.pbFormItems))
	c.pbFormItems = append(c.pbFormItems,
		basicwidget.FormItem{PrimaryWidget: &c.pbSegLabel, SecondaryWidget: &c.pbSegSel},
		basicwidget.FormItem{PrimaryWidget: &c.pbLoopLabel, SecondaryWidget: &c.pbLoopCheck},
		basicwidget.FormItem{PrimaryWidget: &c.pbCountLabel, SecondaryWidget: &c.pbCountInput},
		basicwidget.FormItem{PrimaryWidget: &c.pbSpeedLabel, SecondaryWidget: &c.pbSpeedInput},
		basicwidget.FormItem{PrimaryWidget: &c.pbModeLabel, SecondaryWidget: &c.pbModeSel},
		basicwidget.FormItem{PrimaryWidget: &c.pbAutoLabel, SecondaryWidget: &c.pbAutoCheck},
	)
	c.pbForm.SetItems(c.pbFormItems)
}

// buildChain edits what plays after the first clip completes: the
// entrance-then-idle-loop pattern, one row per link.
func (c *inspectorContent) buildChain(context *guigui.Context, m *Model, n *lottie.SceneNode) {
	setBold(&c.thTitle, "Then  (after the clip completes)")
	steps := n.Playback.Then
	c.thItems = slices.Delete(c.thItems, 0, len(c.thItems))
	for i, st := range steps {
		txt := st.Animation
		if txt == "" {
			txt = "(same clip)"
		}
		if st.Segment != "" {
			txt += " / " + st.Segment
		}
		key := "once"
		if st.Loop {
			key = "loop"
		}
		c.thItems = append(c.thItems, basicwidget.ListItem[int]{
			Text: fmt.Sprintf("%d. %s", i+1, txt), KeyText: key, Value: i,
		})
	}
	c.thList.SetItems(c.thItems)
	if sel := m.SelectedStepIndex(); sel >= 0 && sel < len(steps) {
		c.thList.SelectItemByValue(sel)
	}
	c.thList.OnItemSelected(func(context *guigui.Context, index int) {
		m.SelectStep(index)
	})
	c.thAdd.SetText("Add step")
	c.thAdd.OnDown(func(context *guigui.Context) { m.AddStep() })
	c.thDel.SetText("Delete")
	c.thDel.OnDown(func(context *guigui.Context) { m.DeleteStep(m.SelectedStepIndex()) })
	context.SetEnabled(&c.thDel, m.SelectedStep() != nil)

	st := m.SelectedStep()
	if st == nil {
		return
	}
	label(&c.tsAnimLabel, "clip")
	label(&c.tsSegLabel, "segment")
	label(&c.tsLoopLabel, "loop")
	stepIdx := m.SelectedStepIndex()

	setOptionsWithDefault(&c.tsAnimSel, "(same clip)", m.BundleAnimations(n)...)
	c.tsAnimSel.SelectItemByValue(st.Animation)
	c.tsAnimSel.OnItemSelected(func(context *guigui.Context, index int) {
		it, ok := c.tsAnimSel.ItemByIndex(index)
		st := m.SelectedStep()
		if !ok || st == nil || it.Value == st.Animation {
			return
		}
		st.Animation = it.Value
		st.Segment = ""
		m.touch()
	})
	setOptionsWithDefault(&c.tsSegSel, "(whole clip)", m.StepMarkers(n, stepIdx)...)
	c.tsSegSel.SelectItemByValue(st.Segment)
	c.tsSegSel.OnItemSelected(func(context *guigui.Context, index int) {
		it, ok := c.tsSegSel.ItemByIndex(index)
		st := m.SelectedStep()
		if !ok || st == nil || it.Value == st.Segment {
			return
		}
		st.Segment = it.Value
		m.touch()
	})
	c.tsLoopCheck.SetValue(st.Loop)
	c.tsLoopCheck.OnValueChanged(func(context *guigui.Context, value bool) {
		if st := m.SelectedStep(); st != nil {
			st.Loop = value
			m.touch()
		}
	})

	c.tsFormItems = slices.Delete(c.tsFormItems, 0, len(c.tsFormItems))
	c.tsFormItems = append(c.tsFormItems,
		basicwidget.FormItem{PrimaryWidget: &c.tsAnimLabel, SecondaryWidget: &c.tsAnimSel},
		basicwidget.FormItem{PrimaryWidget: &c.tsSegLabel, SecondaryWidget: &c.tsSegSel},
		basicwidget.FormItem{PrimaryWidget: &c.tsLoopLabel, SecondaryWidget: &c.tsLoopCheck},
	)
	c.tsForm.SetItems(c.tsFormItems)
}

func (c *inspectorContent) buildFocus(context *guigui.Context, m *Model, n *lottie.SceneNode) {
	setBold(&c.fcTitle, "Focus")
	label(&c.fcAbleLabel, "focusable")
	label(&c.fcTabLabel, "tab index")
	dirNames := [4]string{"up", "down", "left", "right"}
	for i := range dirNames {
		label(&c.fcDirLabels[i], dirNames[i])
	}

	focusable := n != nil && n.Focus.Focusable
	if n != nil {
		c.fcAbleCheck.SetValue(n.Focus.Focusable)
		c.fcTabInput.SetValue(strconv.Itoa(n.Focus.TabIndex))
	} else {
		c.fcAbleCheck.SetValue(false)
		c.fcTabInput.SetValue("")
	}
	c.fcAbleCheck.OnValueChanged(func(context *guigui.Context, value bool) {
		if n := m.SelectedNode(); n != nil {
			n.Focus.Focusable = value
			m.touch()
		}
	})
	c.fcTabInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if !committed {
			return
		}
		if v, err := strconv.Atoi(strings.TrimSpace(text)); err == nil {
			if n := m.SelectedNode(); n != nil {
				n.Focus.TabIndex = v
				m.touch()
			}
		}
	})

	// Neighbor links pick from every focusable node; linking to oneself is
	// legal and pins that direction in place.
	targets := m.FocusableNodeNames()
	dirOf := func(nb *lottie.SceneNeighbors, i int) *string {
		switch i {
		case 0:
			return &nb.Up
		case 1:
			return &nb.Down
		case 2:
			return &nb.Left
		}
		return &nb.Right
	}
	for i := range c.fcDirSels {
		setOptionsWithDefault(&c.fcDirSels[i], "(nearest)", targets...)
		if n != nil {
			c.fcDirSels[i].SelectItemByValue(*dirOf(&n.Focus.Neighbors, i))
		}
		c.fcDirSels[i].OnItemSelected(func(context *guigui.Context, index int) {
			it, ok := c.fcDirSels[i].ItemByIndex(index)
			n := m.SelectedNode()
			if !ok || n == nil {
				return
			}
			dst := dirOf(&n.Focus.Neighbors, i)
			if it.Value == *dst {
				return
			}
			*dst = it.Value
			m.touch()
		})
	}

	context.SetEnabled(&c.fcAbleCheck, n != nil)
	context.SetEnabled(&c.fcTabInput, focusable)
	for i := range c.fcDirSels {
		context.SetEnabled(&c.fcDirSels[i], focusable)
	}

	c.fcFormItems = slices.Delete(c.fcFormItems, 0, len(c.fcFormItems))
	c.fcFormItems = append(c.fcFormItems,
		basicwidget.FormItem{PrimaryWidget: &c.fcAbleLabel, SecondaryWidget: &c.fcAbleCheck},
		basicwidget.FormItem{PrimaryWidget: &c.fcTabLabel, SecondaryWidget: &c.fcTabInput},
	)
	for i := range c.fcDirSels {
		c.fcFormItems = append(c.fcFormItems,
			basicwidget.FormItem{PrimaryWidget: &c.fcDirLabels[i], SecondaryWidget: &c.fcDirSels[i]})
	}
	c.fcForm.SetItems(c.fcFormItems)
}

func (c *inspectorContent) buildBindings(context *guigui.Context, m *Model, n *lottie.SceneNode) {
	setBold(&c.bdTitle, "Bindings")

	var bindings []lottie.SceneBinding
	if n != nil {
		bindings = n.Bindings
	}
	c.bdItems = slices.Delete(c.bdItems, 0, len(c.bdItems))
	for i, b := range bindings {
		key := b.Arg
		if b.Target != "" {
			key = fmt.Sprintf("@%s %s", b.Target, b.Arg)
		}
		c.bdItems = append(c.bdItems, basicwidget.ListItem[int]{
			Text:    fmt.Sprintf("%s → %s", b.On, b.Do),
			KeyText: key,
			Value:   i,
		})
	}
	c.bdList.SetItems(c.bdItems)
	if sel := m.SelectedBindingIndex(); sel >= 0 && sel < len(bindings) {
		c.bdList.SelectItemByValue(sel)
	}
	c.bdList.OnItemSelected(func(context *guigui.Context, index int) {
		m.SelectBinding(index)
	})
	c.bdAdd.SetText("Add binding")
	c.bdAdd.OnDown(func(context *guigui.Context) { m.AddBinding() })
	c.bdDel.SetText("Delete")
	c.bdDel.OnDown(func(context *guigui.Context) { m.DeleteBinding(m.SelectedBindingIndex()) })
	context.SetEnabled(&c.bdAdd, n != nil)
	context.SetEnabled(&c.bdDel, m.SelectedBinding() != nil)

	b := m.SelectedBinding()
	if b == nil {
		return
	}
	label(&c.bdOnLabel, "on")
	label(&c.bdDoLabel, "do")
	label(&c.bdTargetLabel, "target")
	label(&c.bdArgLabel, "arg")

	events := lottie.SceneEvents()
	names := make([]string, len(events))
	for i, e := range events {
		names[i] = string(e)
	}
	setOptions(&c.bdOnSel, names...)
	c.bdOnSel.SelectItemByValue(string(b.On))
	c.bdOnSel.OnItemSelected(func(context *guigui.Context, index int) {
		it, ok := c.bdOnSel.ItemByIndex(index)
		b := m.SelectedBinding()
		if !ok || b == nil || it.Value == string(b.On) {
			return
		}
		b.On = lottie.SceneEvent(it.Value)
		m.touch()
	})

	// Every action is offered: fireEvent and playSegment can aim at any
	// node via target, and the validator flags a kind mismatch.
	actions := []string{
		string(lottie.SceneFireEvent), string(lottie.ScenePlaySegment),
		string(lottie.SceneCallback), string(lottie.SceneFocusAction),
	}
	if len(m.PhaseNames()) > 0 {
		actions = append(actions, string(lottie.ScenePhaseAction))
	}
	setOptions(&c.bdDoSel, actions...)
	c.bdDoSel.SelectItemByValue(string(b.Do))
	c.bdDoSel.OnItemSelected(func(context *guigui.Context, index int) {
		it, ok := c.bdDoSel.ItemByIndex(index)
		b := m.SelectedBinding()
		if !ok || b == nil || it.Value == string(b.Do) {
			return
		}
		b.Do = lottie.SceneActionType(it.Value)
		b.Arg, b.Target = "", ""
		m.touch()
	})

	targeted := b.Do == lottie.SceneFireEvent || b.Do == lottie.ScenePlaySegment
	if targeted {
		setOptionsWithDefault(&c.bdTargetSel, "(this node)", m.NodeNames()...)
		c.bdTargetSel.SelectItemByValue(b.Target)
		c.bdTargetSel.OnItemSelected(func(context *guigui.Context, index int) {
			it, ok := c.bdTargetSel.ItemByIndex(index)
			b := m.SelectedBinding()
			if !ok || b == nil || it.Value == b.Target {
				return
			}
			b.Target = it.Value
			b.Arg = ""
			m.touch()
		})
	}

	// What the arg picks depends on the action and on the node it acts
	// on: the target when one is named, else this node.
	tgt := n
	if b.Target != "" {
		if resolved, ok := m.Scene().Node(b.Target); ok {
			tgt = resolved
		}
	}
	switch b.Do {
	case lottie.SceneFireEvent:
		setOptionsWithDefault(&c.bdArgSel, "(pick event)", m.MachineEvents(tgt)...)
	case lottie.ScenePlaySegment:
		setOptionsWithDefault(&c.bdArgSel, "(whole clip)", m.Markers(tgt)...)
	case lottie.SceneFocusAction:
		setOptionsWithDefault(&c.bdArgSel, "(pick node)", m.FocusableNodeNames()...)
	case lottie.ScenePhaseAction:
		setOptionsWithDefault(&c.bdArgSel, "(pick phase)", m.PhaseNames()...)
	}
	if bindingArgIsPicker(b) {
		c.bdArgSel.SelectItemByValue(b.Arg)
		c.bdArgSel.OnItemSelected(func(context *guigui.Context, index int) {
			it, ok := c.bdArgSel.ItemByIndex(index)
			b := m.SelectedBinding()
			if !ok || b == nil || it.Value == b.Arg {
				return
			}
			b.Arg = it.Value
			m.touch()
		})
	} else {
		c.bdArgInput.SetValue(b.Arg)
		c.bdArgInput.SetPlaceholder("callback name")
		c.bdArgInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
			b := m.SelectedBinding()
			if committed && b != nil {
				b.Arg = strings.TrimSpace(text)
				m.touch()
			}
		})
	}

	c.bdFormItems = slices.Delete(c.bdFormItems, 0, len(c.bdFormItems))
	c.bdFormItems = append(c.bdFormItems,
		basicwidget.FormItem{PrimaryWidget: &c.bdOnLabel, SecondaryWidget: &c.bdOnSel},
		basicwidget.FormItem{PrimaryWidget: &c.bdDoLabel, SecondaryWidget: &c.bdDoSel},
	)
	if targeted {
		c.bdFormItems = append(c.bdFormItems,
			basicwidget.FormItem{PrimaryWidget: &c.bdTargetLabel, SecondaryWidget: &c.bdTargetSel})
	}
	if bindingArgIsPicker(b) {
		c.bdFormItems = append(c.bdFormItems,
			basicwidget.FormItem{PrimaryWidget: &c.bdArgLabel, SecondaryWidget: &c.bdArgSel})
	} else {
		c.bdFormItems = append(c.bdFormItems,
			basicwidget.FormItem{PrimaryWidget: &c.bdArgLabel, SecondaryWidget: &c.bdArgInput})
	}
	c.bdForm.SetItems(c.bdFormItems)
}

func (c *inspectorContent) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	if m := c.model(context); m != nil {
		w.WriteInt(m.Generation())
		w.WriteInt(int(m.InspectTarget()))
		w.WriteInt(m.SelectedNodeIndex())
		w.WriteInt(m.SelectedBindingIndex())
		w.WriteInt(m.SelectedPhaseIndex())
		w.WriteInt(m.SelectedStepIndex())
	}
}

func (c *inspectorContent) layout(context *guigui.Context) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)
	m := c.model(context)

	c.items = slices.Delete(c.items, 0, len(c.items))
	if m != nil && m.InspectTarget() == inspectScene {
		c.phBtnItems = slices.Delete(c.phBtnItems, 0, len(c.phBtnItems))
		c.phBtnItems = append(c.phBtnItems,
			guigui.LinearLayoutItem{Widget: &c.phAdd, Size: guigui.FlexibleSize(1)},
			guigui.LinearLayoutItem{Widget: &c.phDel, Size: guigui.FlexibleSize(1)},
		)
		c.phBtnRow = guigui.LinearLayout{
			Direction: guigui.LayoutDirectionHorizontal, Items: c.phBtnItems, Gap: u / 4,
		}
		c.items = append(c.items,
			guigui.LinearLayoutItem{Widget: &c.scTitle, Size: guigui.FixedSize(u)},
			guigui.LinearLayoutItem{Widget: &c.scForm},
			guigui.LinearLayoutItem{Widget: &c.phTitle, Size: guigui.FixedSize(u)},
			guigui.LinearLayoutItem{Widget: &c.phList, Size: guigui.FixedSize(4 * u)},
			guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &c.phBtnRow},
		)
		if m.SelectedPhase() != nil {
			c.items = append(c.items, guigui.LinearLayoutItem{Widget: &c.phForm})
		}
	} else {
		c.bdBtnItems = slices.Delete(c.bdBtnItems, 0, len(c.bdBtnItems))
		c.bdBtnItems = append(c.bdBtnItems,
			guigui.LinearLayoutItem{Widget: &c.bdAdd, Size: guigui.FlexibleSize(1)},
			guigui.LinearLayoutItem{Widget: &c.bdDel, Size: guigui.FlexibleSize(1)},
		)
		c.bdBtnRow = guigui.LinearLayout{
			Direction: guigui.LayoutDirectionHorizontal, Items: c.bdBtnItems, Gap: u / 4,
		}
		c.items = append(c.items,
			guigui.LinearLayoutItem{Widget: &c.ndTitle, Size: guigui.FixedSize(u)},
			guigui.LinearLayoutItem{Widget: &c.ndForm},
			guigui.LinearLayoutItem{Widget: &c.tfTitle, Size: guigui.FixedSize(u)},
			guigui.LinearLayoutItem{Widget: &c.tfForm},
		)
		if m != nil {
			if n := m.SelectedNode(); n != nil && n.Kind == lottie.SceneNodeAnimation {
				c.thBtnItems = slices.Delete(c.thBtnItems, 0, len(c.thBtnItems))
				c.thBtnItems = append(c.thBtnItems,
					guigui.LinearLayoutItem{Widget: &c.thAdd, Size: guigui.FlexibleSize(1)},
					guigui.LinearLayoutItem{Widget: &c.thDel, Size: guigui.FlexibleSize(1)},
				)
				c.thBtnRow = guigui.LinearLayout{
					Direction: guigui.LayoutDirectionHorizontal, Items: c.thBtnItems, Gap: u / 4,
				}
				c.items = append(c.items,
					guigui.LinearLayoutItem{Widget: &c.pbTitle, Size: guigui.FixedSize(u)},
					guigui.LinearLayoutItem{Widget: &c.pbForm},
					guigui.LinearLayoutItem{Widget: &c.thTitle, Size: guigui.FixedSize(u)},
					guigui.LinearLayoutItem{Widget: &c.thList, Size: guigui.FixedSize(3 * u)},
					guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &c.thBtnRow},
				)
				if m.SelectedStep() != nil {
					c.items = append(c.items, guigui.LinearLayoutItem{Widget: &c.tsForm})
				}
			}
			if n := m.SelectedNode(); n != nil && n.Kind == lottie.SceneNodeText {
				c.items = append(c.items,
					guigui.LinearLayoutItem{Widget: &c.txTitle, Size: guigui.FixedSize(u)},
					guigui.LinearLayoutItem{Widget: &c.txForm},
				)
			}
		}
		c.items = append(c.items,
			guigui.LinearLayoutItem{Widget: &c.fcTitle, Size: guigui.FixedSize(u)},
			guigui.LinearLayoutItem{Widget: &c.fcForm},
			guigui.LinearLayoutItem{Widget: &c.bdTitle, Size: guigui.FixedSize(u)},
			guigui.LinearLayoutItem{Widget: &c.bdList, Size: guigui.FixedSize(4 * u)},
			guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &c.bdBtnRow},
		)
		if m != nil && m.SelectedBinding() != nil {
			c.items = append(c.items, guigui.LinearLayoutItem{Widget: &c.bdForm})
		}
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

func (c *inspectorContent) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	c.layout(context).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (c *inspectorContent) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return c.layout(context).Measure(context, constraints)
}
