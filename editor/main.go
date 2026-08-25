// Command editor authors dotLottie state machines: it bundles Lottie clips,
// wires the states a game drives by name, and previews the result with the
// same interpreter the game will run.
package main

import (
	"fmt"
	"image"
	"os"
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	_ "github.com/guigui-gui/guigui/basicwidget/cjkfont"
)

// envKeyModel publishes the document to every pane, so none of them needs a
// pointer threaded through its parent.
var envKeyModel = guigui.GenerateEnvKey()

type Root struct {
	guigui.DefaultWidget

	background basicwidget.Background

	pathInput    basicwidget.TextInput
	openBtn      basicwidget.Button
	saveBtn      basicwidget.Button
	machineCombo basicwidget.Combobox
	newMachine   basicwidget.Button

	clips      clipsPane
	graph      graphView
	graphPanel basicwidget.Panel
	preview    previewPane
	inspector  inspectorPane
	status     basicwidget.Text

	model *Model

	toolbar      guigui.LinearLayout
	toolbarItems []guigui.LinearLayoutItem
	middle       guigui.LinearLayout
	middleItems  []guigui.LinearLayoutItem
	center       guigui.LinearLayout
	centerItems  []guigui.LinearLayoutItem
	items        []guigui.LinearLayoutItem
}

func (r *Root) Env(context *guigui.Context, key guigui.EnvKey, source *guigui.EnvSource) (any, bool) {
	if key == envKeyModel {
		return r.model, true
	}
	return nil, false
}

func (r *Root) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&r.background)
	adder.AddWidget(&r.pathInput)
	adder.AddWidget(&r.openBtn)
	adder.AddWidget(&r.saveBtn)
	adder.AddWidget(&r.machineCombo)
	adder.AddWidget(&r.newMachine)
	adder.AddWidget(&r.clips)
	adder.AddWidget(&r.graphPanel)
	adder.AddWidget(&r.preview)
	adder.AddWidget(&r.inspector)
	adder.AddWidget(&r.status)

	m := r.model
	r.pathInput.SetPlaceholder("path to .lottie (or .json clip)")
	r.pathInput.SetValue(m.Path())
	r.openBtn.SetText("Open")
	r.openBtn.OnDown(func(context *guigui.Context) { m.Open(r.pathInput.Value()) })
	r.saveBtn.SetText("Save")
	r.saveBtn.OnDown(func(context *guigui.Context) { m.Save(r.pathInput.Value()) })

	ids := m.MachineIDs()
	r.machineCombo.SetItems(ids)
	r.machineCombo.SetValue(m.MachineID())
	r.machineCombo.OnValueChanged(func(context *guigui.Context, value string, committed bool) {
		if committed && value != "" && value != m.MachineID() {
			m.SelectMachine(value)
		}
	})
	context.SetEnabled(&r.machineCombo, len(ids) > 0)
	r.newMachine.SetText("New machine")
	r.newMachine.OnDown(func(context *guigui.Context) { m.NewMachine() })

	r.graphPanel.SetContent(&r.graph)
	r.graphPanel.SetAutoBorder(true)

	r.status.SetValue(m.Status())
	r.status.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	r.status.SetScale(0.9)
	return nil
}

func (r *Root) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteInt(r.model.Generation())
}

func (r *Root) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&r.background, widgetBounds.Bounds())
	u := basicwidget.UnitSize(context)

	r.toolbarItems = slices.Delete(r.toolbarItems, 0, len(r.toolbarItems))
	r.toolbarItems = append(r.toolbarItems,
		guigui.LinearLayoutItem{Widget: &r.pathInput, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &r.openBtn, Size: guigui.FixedSize(3 * u)},
		guigui.LinearLayoutItem{Widget: &r.saveBtn, Size: guigui.FixedSize(3 * u)},
		guigui.LinearLayoutItem{Widget: &r.machineCombo, Size: guigui.FixedSize(6 * u)},
		guigui.LinearLayoutItem{Widget: &r.newMachine, Size: guigui.FixedSize(5 * u)},
	)
	r.toolbar = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal, Items: r.toolbarItems, Gap: u / 4,
	}

	// The graph and the preview share the middle column: you watch the
	// machine run right under the graph you are editing.
	r.centerItems = slices.Delete(r.centerItems, 0, len(r.centerItems))
	r.centerItems = append(r.centerItems,
		guigui.LinearLayoutItem{Widget: &r.graphPanel, Size: guigui.FlexibleSize(3)},
		guigui.LinearLayoutItem{Widget: &r.preview, Size: guigui.FlexibleSize(2)},
	)
	r.center = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical, Items: r.centerItems, Gap: u / 4,
	}

	r.middleItems = slices.Delete(r.middleItems, 0, len(r.middleItems))
	r.middleItems = append(r.middleItems,
		guigui.LinearLayoutItem{Widget: &r.clips, Size: guigui.FlexibleSize(2)},
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(5), Layout: &r.center},
		guigui.LinearLayoutItem{Widget: &r.inspector, Size: guigui.FlexibleSize(3)},
	)
	r.middle = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal, Items: r.middleItems, Gap: u / 4,
	}

	r.items = slices.Delete(r.items, 0, len(r.items))
	r.items = append(r.items,
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &r.toolbar},
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1), Layout: &r.middle},
		guigui.LinearLayoutItem{Widget: &r.status, Size: guigui.FixedSize(u)},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     r.items, Gap: u / 4,
		Padding: guigui.Padding{Start: u / 2, Top: u / 2, End: u / 2, Bottom: u / 2},
	}).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func main() {
	root := &Root{model: NewModel()}
	if len(os.Args) > 1 {
		root.model.Open(os.Args[1])
	}
	op := &guigui.RunOptions{
		Title:         "Lottie State Machine Editor",
		WindowSize:    image.Pt(1280, 800),
		WindowMinSize: image.Pt(900, 600),
	}
	if err := runWithOptionalScreenshot(root, op); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
