package main

import (
	"slices"
	"strconv"
	"strings"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	lottie "github.com/shibukawa/lottie-go"
)

// restartRowIndex is the pseudo-input pinned to the top of the Inputs table.
// Restarting is not something the document declares, but it belongs with the
// other things you can trigger, so it sits in the same table and is the one
// row that cannot be renamed or removed.
const restartRowIndex = 0

// clipsPane lists the animations in the bundle and the machine's inputs.
// Each input carries its own control, so an event is fired where it is
// defined rather than from a separate strip of buttons.
type clipsPane struct {
	guigui.DefaultWidget

	clipsTitle basicwidget.Text
	clipList   basicwidget.List[string]
	importBtn  basicwidget.Button
	removeBtn  basicwidget.Button

	inputsTitle basicwidget.Text
	inputList   basicwidget.List[int]
	inputRows   guigui.WidgetSlice[*inputRow]
	inputName   basicwidget.TextInput
	addEvent    basicwidget.Button
	addBool     basicwidget.Button
	addNumber   basicwidget.Button
	removeInput basicwidget.Button

	selectedClip string
	selectedRow  int

	clipItems  []basicwidget.ListItem[string]
	inputItems []basicwidget.ListItem[int]

	importRow      guigui.LinearLayout
	importRowItems []guigui.LinearLayoutItem
	addRow         guigui.LinearLayout
	addRowItems    []guigui.LinearLayoutItem
	nameRow        guigui.LinearLayout
	nameRowItems   []guigui.LinearLayoutItem
	items          []guigui.LinearLayoutItem
}

func (c *clipsPane) model(context *guigui.Context) *Model {
	v, ok := context.Env(c, envKeyModel)
	if !ok {
		return nil
	}
	m, _ := v.(*Model)
	return m
}

func (c *clipsPane) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m := c.model(context)
	if m == nil {
		return nil
	}
	for _, w := range []guigui.Widget{
		&c.clipsTitle, &c.clipList, &c.importBtn, &c.removeBtn,
		&c.inputsTitle, &c.inputList, &c.inputName,
		&c.addEvent, &c.addBool, &c.addNumber, &c.removeInput,
	} {
		adder.AddWidget(w)
	}
	c.buildClips(context, m)
	c.buildInputs(context, m)
	return nil
}

func (c *clipsPane) buildClips(context *guigui.Context, m *Model) {
	setBold(&c.clipsTitle, "Clips")

	ids := m.AnimationIDs()
	c.clipItems = slices.Delete(c.clipItems, 0, len(c.clipItems))
	for _, id := range ids {
		c.clipItems = append(c.clipItems, basicwidget.ListItem[string]{
			Text:    id,
			KeyText: m.ClipSummary(id),
			Value:   id,
		})
	}
	c.clipList.SetItems(c.clipItems)
	if c.selectedClip != "" && slices.Contains(ids, c.selectedClip) {
		c.clipList.SelectItemByValue(c.selectedClip)
	}
	// Selecting a clip plays it on its own, so a clip can be judged before
	// it is wired into any state.
	c.clipList.OnItemSelected(func(context *guigui.Context, index int) {
		if index >= 0 && index < len(ids) {
			c.selectedClip = ids[index]
			m.ShowClip(c.selectedClip)
			return
		}
		c.selectedClip = ""
	})

	c.importBtn.SetText("Import…")
	c.importBtn.OnDown(func(context *guigui.Context) { m.BrowseImport() })
	context.SetEnabled(&c.importBtn, !m.DialogOpen())
	c.removeBtn.SetText("Remove")
	c.removeBtn.OnDown(func(context *guigui.Context) {
		if c.selectedClip != "" {
			m.RemoveClip(c.selectedClip)
			c.selectedClip = ""
			m.ShowMachine()
		}
	})
	context.SetEnabled(&c.removeBtn, c.selectedClip != "")
}

func (c *clipsPane) buildInputs(context *guigui.Context, m *Model) {
	setBold(&c.inputsTitle, "Inputs")

	var inputs []lottie.Input
	if sm := m.Machine(); sm != nil {
		inputs = sm.Inputs
	}
	live := m.Preview() != nil && m.PreviewClip() == ""

	// Row 0 is Restart; the declared inputs follow.
	total := len(inputs) + 1
	if c.selectedRow >= total {
		c.selectedRow = restartRowIndex
	}
	c.inputRows.SetLen(total)
	c.inputItems = slices.Delete(c.inputItems, 0, len(c.inputItems))
	for i := range total {
		c.inputItems = append(c.inputItems, basicwidget.ListItem[int]{
			Content: c.inputRows.At(i),
			Value:   i,
		})
	}
	c.inputList.SetItems(c.inputItems)
	c.inputList.SetItemHeight(inputRowHeight(context))
	c.inputList.SelectItemByValue(c.selectedRow)
	c.inputList.OnItemSelected(func(context *guigui.Context, index int) {
		c.selectedRow = index
		// Highlight the transitions that read the selected input.
		m.SelectInput(index - 1)
	})

	restart := c.inputRows.At(restartRowIndex)
	restart.SetFire("Restart", "action", "▶", m.Machine() != nil)
	restart.OnFired(func(context *guigui.Context) { m.RestartPreview() })

	for i, in := range inputs {
		row := c.inputRows.At(i + 1)
		switch in.Type {
		case lottie.InputBoolean:
			v := false
			if sm := m.Preview(); sm != nil {
				v, _ = sm.Get[bool](in.Name)
			}
			row.SetBool(in.Name, string(in.Type), v, live)
			row.OnBoolSet(func(context *guigui.Context, value bool) {
				SetInputValue(m, in.Name, value)
			})
		case lottie.InputNumeric:
			s := ""
			if sm := m.Preview(); sm != nil {
				if v, ok := sm.Get[float64](in.Name); ok {
					s = strconv.FormatFloat(v, 'g', -1, 64)
				}
			}
			row.SetText(in.Name, string(in.Type), s, live)
			row.OnTextSet(func(context *guigui.Context, value string) {
				if v, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
					SetInputValue(m, in.Name, v)
				}
			})
		case lottie.InputString:
			s := ""
			if sm := m.Preview(); sm != nil {
				s, _ = sm.Get[string](in.Name)
			}
			row.SetText(in.Name, string(in.Type), s, live)
			row.OnTextSet(func(context *guigui.Context, value string) {
				SetInputValue(m, in.Name, value)
			})
		default:
			row.SetFire(in.Name, string(in.Type), "Try", live)
			row.OnFired(func(context *guigui.Context) { m.Fire(in.Name) })
		}
	}

	// Restart is not a declared input, so it cannot be renamed or removed.
	editable := c.selectedRow > restartRowIndex && c.selectedRow-1 < len(inputs)
	if editable {
		c.inputName.SetValue(inputs[c.selectedRow-1].Name)
	} else {
		c.inputName.SetValue("")
	}
	c.inputName.SetPlaceholder("input name")
	idx := c.selectedRow - 1
	c.inputName.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed {
			m.RenameInput(idx, text)
		}
	})
	context.SetEnabled(&c.inputName, editable)

	c.addEvent.SetText("+Event")
	c.addEvent.OnDown(func(context *guigui.Context) { m.AddInput(lottie.InputEvent) })
	c.addBool.SetText("+Bool")
	c.addBool.OnDown(func(context *guigui.Context) { m.AddInput(lottie.InputBoolean) })
	c.addNumber.SetText("+Num")
	c.addNumber.OnDown(func(context *guigui.Context) { m.AddInput(lottie.InputNumeric) })
	c.removeInput.SetText("Remove")
	c.removeInput.OnDown(func(context *guigui.Context) {
		m.DeleteInput(c.selectedRow - 1)
		c.selectedRow = restartRowIndex
		m.SelectInput(-1)
	})
	context.SetEnabled(&c.removeInput, editable)
}

func (c *clipsPane) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	m := c.model(context)
	if m != nil {
		w.WriteInt(m.Generation())
		// The live values shown in the rows follow playback.
		w.WriteString(m.ActiveState())
	}
	w.WriteString(c.selectedClip)
	w.WriteInt(c.selectedRow)
}

func (c *clipsPane) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)

	c.importRowItems = slices.Delete(c.importRowItems, 0, len(c.importRowItems))
	c.importRowItems = append(c.importRowItems,
		guigui.LinearLayoutItem{Widget: &c.importBtn, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &c.removeBtn, Size: guigui.FlexibleSize(1)},
	)
	c.importRow = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     c.importRowItems, Gap: u / 4,
	}

	c.nameRowItems = slices.Delete(c.nameRowItems, 0, len(c.nameRowItems))
	c.nameRowItems = append(c.nameRowItems,
		guigui.LinearLayoutItem{Widget: &c.inputName, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &c.removeInput, Size: guigui.FixedSize(3 * u)},
	)
	c.nameRow = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     c.nameRowItems, Gap: u / 4,
	}

	c.addRowItems = slices.Delete(c.addRowItems, 0, len(c.addRowItems))
	c.addRowItems = append(c.addRowItems,
		guigui.LinearLayoutItem{Widget: &c.addEvent, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &c.addBool, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &c.addNumber, Size: guigui.FlexibleSize(1)},
	)
	c.addRow = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     c.addRowItems, Gap: u / 4,
	}

	c.items = slices.Delete(c.items, 0, len(c.items))
	c.items = append(c.items,
		guigui.LinearLayoutItem{Widget: &c.clipsTitle, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &c.clipList, Size: guigui.FlexibleSize(2)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &c.importRow},
		guigui.LinearLayoutItem{Widget: &c.inputsTitle, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &c.inputList, Size: guigui.FlexibleSize(3)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &c.nameRow},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &c.addRow},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     c.items, Gap: u / 4,
		Padding: guigui.Padding{Start: u / 2, Top: u / 2, End: u / 2, Bottom: u / 2},
	}).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}
