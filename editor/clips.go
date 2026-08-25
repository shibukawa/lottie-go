package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	lottie "github.com/shibukawa/lottie-go"
)

// restartRowIndex is the pseudo-input pinned to the top of the events tab.
// Restarting is not something the document declares, but it is one of the
// things you can trigger, so it sits with them and is the one row that
// cannot be renamed or removed.
const restartRowIndex = 0

// ioTab splits the machine's external interface by direction. Values and
// events come in from the game; markers go out to it. Mixing them in one
// list hid that difference.
type ioTab int

const (
	tabEvents ioTab = iota
	tabValues
	tabMarkers
)

type clipsPane struct {
	guigui.DefaultWidget

	clipsTitle basicwidget.Text
	clipList   basicwidget.List[clipRef]
	importBtn  basicwidget.Button
	removeBtn  basicwidget.Button

	ioTitle basicwidget.Text
	tabs    basicwidget.SegmentedControl[ioTab]

	eventList basicwidget.List[int]
	eventRows guigui.WidgetSlice[*inputRow]

	valueList basicwidget.List[int]
	valueRows guigui.WidgetSlice[*inputRow]

	markerList basicwidget.List[int]
	markerRows guigui.WidgetSlice[*inputRow]

	inputName   basicwidget.TextInput
	addEvent    basicwidget.Button
	addBool     basicwidget.Button
	addNumber   basicwidget.Button
	removeInput basicwidget.Button

	tab          ioTab
	selectedClip clipRef
	selectedRow  int

	clipItems   []basicwidget.ListItem[clipRef]
	eventItems  []basicwidget.ListItem[int]
	valueItems  []basicwidget.ListItem[int]
	markerItems []basicwidget.ListItem[int]
	tabItems    []basicwidget.SegmentedControlItem[ioTab]

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
		&c.ioTitle, &c.tabs,
	} {
		adder.AddWidget(w)
	}
	c.buildClips(context, m)
	c.buildTabs(context)

	// Only the visible tab is added: an unbuilt list is not laid out, drawn
	// or sent input.
	switch c.tab {
	case tabEvents:
		adder.AddWidget(&c.eventList)
		c.buildEvents(context, m, adder)
	case tabValues:
		adder.AddWidget(&c.valueList)
		c.buildValues(context, m, adder)
	case tabMarkers:
		adder.AddWidget(&c.markerList)
		c.buildMarkers(context, m, adder)
	}

	// Editing the declaration is only meaningful for the incoming side.
	if c.tab != tabMarkers {
		adder.AddWidget(&c.inputName)
		adder.AddWidget(&c.removeInput)
		adder.AddWidget(&c.addEvent)
		adder.AddWidget(&c.addBool)
		adder.AddWidget(&c.addNumber)
	}
	c.buildEditors(context, m)
	return nil
}

func (c *clipsPane) buildClips(context *guigui.Context, m *Model) {
	setBold(&c.clipsTitle, "Clips")

	// One row per playable unit: a marker, or the whole file when it has
	// none. A file with three markers is three clips here, so the same file
	// name repeats down the list.
	refs := m.ClipRefs()
	c.clipItems = slices.Delete(c.clipItems, 0, len(c.clipItems))
	for _, r := range refs {
		c.clipItems = append(c.clipItems, basicwidget.ListItem[clipRef]{
			Text:    r.Label(),
			KeyText: m.ClipSummaryRef(r),
			Value:   r,
		})
	}
	c.clipList.SetItems(c.clipItems)
	if slices.Contains(refs, c.selectedClip) {
		c.clipList.SelectItemByValue(c.selectedClip)
	}
	c.clipList.OnItemSelected(func(context *guigui.Context, index int) {
		if index >= 0 && index < len(refs) {
			c.selectedClip = refs[index]
			m.ShowClip(c.selectedClip)
			return
		}
		c.selectedClip = clipRef{}
	})

	c.importBtn.SetText("Import…")
	c.importBtn.OnDown(func(context *guigui.Context) { m.BrowseImport() })
	context.SetEnabled(&c.importBtn, !m.DialogOpen())
	c.removeBtn.SetText("Remove")
	c.removeBtn.OnDown(func(context *guigui.Context) {
		if c.selectedClip.Anim != "" {
			m.RemoveClip(c.selectedClip.Anim)
			c.selectedClip = clipRef{}
			m.ShowMachine()
		}
	})
	context.SetEnabled(&c.removeBtn, c.selectedClip.Anim != "")
}

func (c *clipsPane) buildTabs(context *guigui.Context) {
	setBold(&c.ioTitle, "Interface")
	c.tabItems = slices.Delete(c.tabItems, 0, len(c.tabItems))
	c.tabItems = append(c.tabItems,
		basicwidget.SegmentedControlItem[ioTab]{Text: "Events", Value: tabEvents},
		basicwidget.SegmentedControlItem[ioTab]{Text: "Values", Value: tabValues},
		basicwidget.SegmentedControlItem[ioTab]{Text: "Markers", Value: tabMarkers},
	)
	c.tabs.SetItems(c.tabItems)
	c.tabs.SelectItemByValue(c.tab)
	c.tabs.OnItemSelected(func(context *guigui.Context, index int) {
		if item, ok := c.tabs.ItemByIndex(index); ok {
			c.tab = item.Value
			c.selectedRow = restartRowIndex
		}
	})
}

// eventInputs and valueInputs split the declared inputs by direction.
func eventInputs(m *Model) []lottie.Input {
	var out []lottie.Input
	if sm := m.Machine(); sm != nil {
		for _, in := range sm.Inputs {
			if in.Type == lottie.InputEvent {
				out = append(out, in)
			}
		}
	}
	return out
}

func valueInputs(m *Model) []lottie.Input {
	var out []lottie.Input
	if sm := m.Machine(); sm != nil {
		for _, in := range sm.Inputs {
			if in.Type != lottie.InputEvent {
				out = append(out, in)
			}
		}
	}
	return out
}

// inputIndex maps a position within one tab back to the document's own
// input list, which is what rename and delete address.
func inputIndex(m *Model, want lottie.Input) int {
	if sm := m.Machine(); sm != nil {
		for i, in := range sm.Inputs {
			if in.Name == want.Name {
				return i
			}
		}
	}
	return -1
}

func (c *clipsPane) buildEvents(context *guigui.Context, m *Model, adder *guigui.ChildAdder) {
	events := eventInputs(m)
	live := m.Preview() != nil && m.PreviewClip().Anim == ""

	total := len(events) + 1
	if c.selectedRow >= total {
		c.selectedRow = restartRowIndex
	}
	c.eventRows.SetLen(total)
	c.eventItems = slices.Delete(c.eventItems, 0, len(c.eventItems))
	for i := range total {
		adder.AddWidget(c.eventRows.At(i))
		c.eventItems = append(c.eventItems, basicwidget.ListItem[int]{
			Content: c.eventRows.At(i), Value: i,
		})
	}
	c.eventList.SetItems(c.eventItems)
	c.eventList.SetItemHeight(inputRowHeight(context))
	c.eventList.SelectItemByValue(c.selectedRow)
	c.eventList.OnItemSelected(func(context *guigui.Context, index int) {
		c.selectedRow = index
		if index > restartRowIndex && index-1 < len(events) {
			m.SelectInput(inputIndex(m, events[index-1]))
			return
		}
		m.SelectInput(-1)
	})

	restart := c.eventRows.At(restartRowIndex)
	restart.SetFire("Restart", "action", "▶", m.Machine() != nil)
	restart.OnFired(func(context *guigui.Context) { m.RestartPreview() })

	for i, in := range events {
		row := c.eventRows.At(i + 1)
		row.SetFire(in.Name, string(in.Type), "Try", live)
		row.OnFired(func(context *guigui.Context) { m.Fire(in.Name) })
	}
}

func (c *clipsPane) buildValues(context *guigui.Context, m *Model, adder *guigui.ChildAdder) {
	values := valueInputs(m)
	live := m.Preview() != nil && m.PreviewClip().Anim == ""

	if c.selectedRow >= len(values) {
		c.selectedRow = 0
	}
	c.valueRows.SetLen(len(values))
	c.valueItems = slices.Delete(c.valueItems, 0, len(c.valueItems))
	for i := range values {
		adder.AddWidget(c.valueRows.At(i))
		c.valueItems = append(c.valueItems, basicwidget.ListItem[int]{
			Content: c.valueRows.At(i), Value: i,
		})
	}
	c.valueList.SetItems(c.valueItems)
	c.valueList.SetItemHeight(inputRowHeight(context))
	c.valueList.SelectItemByValue(c.selectedRow)
	c.valueList.OnItemSelected(func(context *guigui.Context, index int) {
		c.selectedRow = index
		if index >= 0 && index < len(values) {
			m.SelectInput(inputIndex(m, values[index]))
			return
		}
		m.SelectInput(-1)
	})

	for i, in := range values {
		row := c.valueRows.At(i)
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
		default:
			s := ""
			if sm := m.Preview(); sm != nil {
				s, _ = sm.Get[string](in.Name)
			}
			row.SetText(in.Name, string(in.Type), s, live)
			row.OnTextSet(func(context *guigui.Context, value string) {
				SetInputValue(m, in.Name, value)
			})
		}
	}
}

// buildMarkers lists the outgoing side. Markers are not declared by the
// machine and cannot be driven from here; they fire as the animation passes
// them, and the count is what shows that happening.
func (c *clipsPane) buildMarkers(context *guigui.Context, m *Model, adder *guigui.ChildAdder) {
	markers := m.MarkerRefs()
	if c.selectedRow >= len(markers) {
		c.selectedRow = 0
	}
	c.markerRows.SetLen(len(markers))
	c.markerItems = slices.Delete(c.markerItems, 0, len(c.markerItems))
	for i := range markers {
		adder.AddWidget(c.markerRows.At(i))
		c.markerItems = append(c.markerItems, basicwidget.ListItem[int]{
			Content: c.markerRows.At(i), Value: i,
		})
	}
	c.markerList.SetItems(c.markerItems)
	c.markerList.SetItemHeight(inputRowHeight(context))
	c.markerList.SelectItemByValue(c.selectedRow)
	c.markerList.OnItemSelected(func(context *guigui.Context, index int) {
		c.selectedRow = index
	})

	for i, mk := range markers {
		hits := m.MarkerHits(mk.Name)
		fired := "—"
		if hits > 0 {
			fired = fmt.Sprintf("×%d", hits)
		}
		c.markerRows.At(i).SetReadout(mk.Name, mk.Anim, fired)
	}
}

func (c *clipsPane) buildEditors(context *guigui.Context, m *Model) {
	var inputs []lottie.Input
	idx := -1
	switch c.tab {
	case tabEvents:
		inputs = eventInputs(m)
		if c.selectedRow > restartRowIndex && c.selectedRow-1 < len(inputs) {
			idx = inputIndex(m, inputs[c.selectedRow-1])
		}
	case tabValues:
		inputs = valueInputs(m)
		if c.selectedRow >= 0 && c.selectedRow < len(inputs) {
			idx = inputIndex(m, inputs[c.selectedRow])
		}
	}
	editable := idx >= 0

	if editable {
		c.inputName.SetValue(m.Machine().Inputs[idx].Name)
	} else {
		c.inputName.SetValue("")
	}
	c.inputName.SetPlaceholder("input name")
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
		m.DeleteInput(idx)
		c.selectedRow = restartRowIndex
		m.SelectInput(-1)
	})
	context.SetEnabled(&c.removeInput, editable)
}

func (c *clipsPane) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	m := c.model(context)
	if m != nil {
		w.WriteInt(m.Generation())
		w.WriteString(m.ActiveState())
		// Marker counts change as the animation plays.
		for _, mk := range m.MarkerRefs() {
			w.WriteInt(m.MarkerHits(mk.Name))
		}
	}
	w.WriteString(c.selectedClip.Anim + "/" + c.selectedClip.Segment)
	w.WriteInt(c.selectedRow)
	w.WriteInt(int(c.tab))
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

	var list guigui.Widget = &c.eventList
	switch c.tab {
	case tabValues:
		list = &c.valueList
	case tabMarkers:
		list = &c.markerList
	}

	c.items = slices.Delete(c.items, 0, len(c.items))
	c.items = append(c.items,
		guigui.LinearLayoutItem{Widget: &c.clipsTitle, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &c.clipList, Size: guigui.FlexibleSize(2)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &c.importRow},
		guigui.LinearLayoutItem{Widget: &c.ioTitle, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &c.tabs, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: list, Size: guigui.FlexibleSize(3)},
	)
	if c.tab != tabMarkers {
		c.items = append(c.items,
			guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &c.nameRow},
			guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &c.addRow},
		)
	}
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     c.items, Gap: u / 4,
		Padding: guigui.Padding{Start: u / 2, Top: u / 2, End: u / 2, Bottom: u / 2},
	}).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}
