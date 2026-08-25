package main

import (
	"image"
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
)

// inputRow is one line of the Inputs table: the name, the kind, and the
// control that exercises it. Putting the control here rather than in a
// separate button strip means an input is fired where it is defined.
type inputRow struct {
	guigui.DefaultWidget

	name basicwidget.Text
	kind basicwidget.Text

	fire  basicwidget.Button
	check basicwidget.Checkbox
	text  basicwidget.TextInput

	control  inputControl
	label    string
	kindText string
	action   string
	boolVal  bool
	textVal  string
	live     bool

	items []guigui.LinearLayoutItem
}

type inputControl int

const (
	controlNone inputControl = iota
	controlFire
	controlBool
	controlText
)

var (
	eventInputFired   = guigui.GenerateEventKey()
	eventInputBoolSet = guigui.GenerateEventKey()
	eventInputTextSet = guigui.GenerateEventKey()
)

func (r *inputRow) OnFired(f func(context *guigui.Context)) {
	guigui.SetEventHandler(r, eventInputFired, f)
}

func (r *inputRow) OnBoolSet(f func(context *guigui.Context, v bool)) {
	guigui.SetEventHandler(r, eventInputBoolSet, f)
}

func (r *inputRow) OnTextSet(f func(context *guigui.Context, v string)) {
	guigui.SetEventHandler(r, eventInputTextSet, f)
}

// SetFire configures the row as an event, with a button that raises it.
// action is the button's label.
func (r *inputRow) SetFire(name, kind, action string, live bool) {
	r.label, r.kindText, r.action, r.live = name, kind, action, live
	r.control = controlFire
}

func (r *inputRow) SetBool(name, kind string, v, live bool) {
	r.label, r.kindText, r.boolVal, r.live = name, kind, v, live
	r.control = controlBool
}

func (r *inputRow) SetText(name, kind, v string, live bool) {
	r.label, r.kindText, r.textVal, r.live = name, kind, v, live
	r.control = controlText
}

func (r *inputRow) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&r.name)
	adder.AddWidget(&r.kind)
	switch r.control {
	case controlFire:
		adder.AddWidget(&r.fire)
	case controlBool:
		adder.AddWidget(&r.check)
	case controlText:
		adder.AddWidget(&r.text)
	}

	r.name.SetValue(r.label)
	r.name.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	r.kind.SetValue(r.kindText)
	r.kind.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	r.kind.SetHorizontalAlign(basicwidget.HorizontalAlignEnd)
	r.kind.SetScale(0.8)
	r.kind.SetOpacity(0.7)
	// The labels are decoration; let the click through so the list row
	// still selects.
	context.SetPassthrough(&r.name, true)
	context.SetPassthrough(&r.kind, true)

	switch r.control {
	case controlFire:
		r.fire.SetText(r.action)
		r.fire.OnDown(func(context *guigui.Context) {
			guigui.DispatchEvent(r, eventInputFired)
		})
		context.SetEnabled(&r.fire, r.live)
	case controlBool:
		r.check.SetValue(r.boolVal)
		r.check.OnValueChanged(func(context *guigui.Context, value bool) {
			guigui.DispatchEvent(r, eventInputBoolSet, value)
		})
		context.SetEnabled(&r.check, r.live)
	case controlText:
		r.text.SetValue(r.textVal)
		r.text.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
			if committed {
				guigui.DispatchEvent(r, eventInputTextSet, text)
			}
		})
		context.SetEnabled(&r.text, r.live)
	}
	return nil
}

func (r *inputRow) layout(context *guigui.Context) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)
	r.items = slices.Delete(r.items, 0, len(r.items))
	r.items = append(r.items,
		guigui.LinearLayoutItem{Widget: &r.name, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &r.kind, Size: guigui.FixedSize(5 * u / 2)},
	)
	switch r.control {
	case controlFire:
		r.items = append(r.items, guigui.LinearLayoutItem{
			Widget: &r.fire, Size: guigui.FixedSize(3 * u)})
	case controlBool:
		r.items = append(r.items, guigui.LinearLayoutItem{
			Widget: &r.check, Size: guigui.FixedSize(3 * u)})
	case controlText:
		r.items = append(r.items, guigui.LinearLayoutItem{
			Widget: &r.text, Size: guigui.FixedSize(3 * u)})
	}
	return guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     r.items,
		Gap:       u / 4,
		Padding:   guigui.Padding{Start: u / 4, End: u / 4},
	}
}

func (r *inputRow) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	r.layout(context).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (r *inputRow) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w := 12 * u
	if cw, ok := constraints.FixedWidth(); ok {
		w = cw
	}
	return image.Pt(w, inputRowHeight(context))
}

func inputRowHeight(context *guigui.Context) int {
	return 3 * basicwidget.UnitSize(context) / 2
}
