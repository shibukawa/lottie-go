package main

import (
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	lottie "github.com/shibukawa/lottie-go"
)

// clipsPane lists the animations in the bundle and the machine's inputs.
// Event input names are what a game fires, so they are edited here next to
// the clips they drive.
type clipsPane struct {
	guigui.DefaultWidget

	clipsTitle basicwidget.Text
	clipList   basicwidget.List[string]
	importBtn  basicwidget.Button
	removeBtn  basicwidget.Button

	inputsTitle basicwidget.Text
	inputList   basicwidget.List[int]
	inputName   basicwidget.TextInput
	addEvent    basicwidget.Button
	addBool     basicwidget.Button
	addNumber   basicwidget.Button
	removeInput basicwidget.Button

	selectedClip  string
	selectedInput int

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
	adder.AddWidget(&c.clipsTitle)
	adder.AddWidget(&c.clipList)
	adder.AddWidget(&c.importBtn)
	adder.AddWidget(&c.removeBtn)
	adder.AddWidget(&c.inputsTitle)
	adder.AddWidget(&c.inputList)
	adder.AddWidget(&c.inputName)
	adder.AddWidget(&c.addEvent)
	adder.AddWidget(&c.addBool)
	adder.AddWidget(&c.addNumber)
	adder.AddWidget(&c.removeInput)

	setBold(&c.clipsTitle, "Clips")

	ids := m.AnimationIDs()
	c.clipItems = slices.Delete(c.clipItems, 0, len(c.clipItems))
	for _, id := range ids {
		c.clipItems = append(c.clipItems, basicwidget.ListItem[string]{
			Text:    id + "   " + m.ClipSummary(id),
			Value:   id,
			KeyText: "",
		})
	}
	c.clipList.SetItems(c.clipItems)
	if c.selectedClip != "" && slices.Contains(ids, c.selectedClip) {
		c.clipList.SelectItemByValue(c.selectedClip)
	}
	c.clipList.OnItemSelected(func(context *guigui.Context, index int) {
		if index >= 0 && index < len(ids) {
			c.selectedClip = ids[index]
		} else {
			c.selectedClip = ""
		}
	})

	c.importBtn.SetText("Import…")
	c.importBtn.OnDown(func(context *guigui.Context) { m.BrowseImport() })
	context.SetEnabled(&c.importBtn, !m.DialogOpen())
	c.removeBtn.SetText("Remove")
	c.removeBtn.OnDown(func(context *guigui.Context) {
		if c.selectedClip != "" {
			m.RemoveClip(c.selectedClip)
			c.selectedClip = ""
		}
	})
	context.SetEnabled(&c.removeBtn, c.selectedClip != "")

	// ---- inputs ----
	setBold(&c.inputsTitle, "Inputs")

	var inputs []lottie.Input
	if sm := m.Machine(); sm != nil {
		inputs = sm.Inputs
	}
	if c.selectedInput >= len(inputs) {
		c.selectedInput = -1
	}
	c.inputItems = slices.Delete(c.inputItems, 0, len(c.inputItems))
	for i, in := range inputs {
		c.inputItems = append(c.inputItems, basicwidget.ListItem[int]{
			Text:    in.Name,
			KeyText: string(in.Type),
			Value:   i,
		})
	}
	c.inputList.SetItems(c.inputItems)
	if c.selectedInput >= 0 {
		c.inputList.SelectItemByValue(c.selectedInput)
	}
	c.inputList.OnItemSelected(func(context *guigui.Context, index int) {
		c.selectedInput = index
	})

	if c.selectedInput >= 0 && c.selectedInput < len(inputs) {
		c.inputName.SetValue(inputs[c.selectedInput].Name)
	} else {
		c.inputName.SetValue("")
	}
	c.inputName.SetPlaceholder("input name")
	idx := c.selectedInput
	c.inputName.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed {
			m.RenameInput(idx, text)
		}
	})
	context.SetEnabled(&c.inputName, c.selectedInput >= 0)

	c.addEvent.SetText("+Event")
	c.addEvent.OnDown(func(context *guigui.Context) { m.AddInput(lottie.InputEvent) })
	c.addBool.SetText("+Bool")
	c.addBool.OnDown(func(context *guigui.Context) { m.AddInput(lottie.InputBoolean) })
	c.addNumber.SetText("+Num")
	c.addNumber.OnDown(func(context *guigui.Context) { m.AddInput(lottie.InputNumeric) })
	c.removeInput.SetText("Remove")
	c.removeInput.OnDown(func(context *guigui.Context) {
		m.DeleteInput(c.selectedInput)
		c.selectedInput = -1
	})
	context.SetEnabled(&c.removeInput, c.selectedInput >= 0)
	return nil
}

func (c *clipsPane) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	if m := c.model(context); m != nil {
		w.WriteInt(m.Generation())
	}
	w.WriteString(c.selectedClip)
	w.WriteInt(c.selectedInput)
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
		guigui.LinearLayoutItem{Widget: &c.clipList, Size: guigui.FlexibleSize(3)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &c.importRow},
		guigui.LinearLayoutItem{Widget: &c.inputsTitle, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &c.inputList, Size: guigui.FlexibleSize(2)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &c.nameRow},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &c.addRow},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     c.items, Gap: u / 4,
		Padding: guigui.Padding{Start: u / 2, Top: u / 2, End: u / 2, Bottom: u / 2},
	}).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}
