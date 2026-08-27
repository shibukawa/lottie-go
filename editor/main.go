// Command editor authors dotLottie state machines: it bundles Lottie clips,
// wires the states a game drives by name, and previews the result with the
// same interpreter the game will run.
package main

import (
	"flag"
	"fmt"
	"image"
	"os"
	"path/filepath"
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

	pathLabel basicwidget.Text
	openBtn   basicwidget.Button
	saveBtn   basicwidget.Button
	saveAsBtn basicwidget.Button
	configBtn basicwidget.Button

	clips        clipsPane
	graph        graphView
	graphPanel   basicwidget.Panel
	graphFrame   framedPane
	preview      previewPane
	previewFrame framedPane
	inspector    inspectorPane
	status       basicwidget.Text

	model *Model
	watch *watcher

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
	adder.AddWidget(&r.pathLabel)
	adder.AddWidget(&r.openBtn)
	adder.AddWidget(&r.saveBtn)
	adder.AddWidget(&r.saveAsBtn)
	adder.AddWidget(&r.configBtn)
	adder.AddWidget(&r.clips)
	adder.AddWidget(&r.graphFrame)
	adder.AddWidget(&r.previewFrame)
	adder.AddWidget(&r.inspector)
	adder.AddWidget(&r.status)

	m := r.model
	// The dialogs own picking files, so the toolbar only has to say which
	// document is open. A full absolute path spent most of the bar's width
	// on something nothing here acts on.
	name := "(unsaved bundle)"
	if p := m.Path(); p != "" {
		name = filepath.Base(p)
	}
	if m.Viewer() {
		name += "  [viewer]"
	}
	r.pathLabel.SetValue(name)
	r.pathLabel.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	r.openBtn.SetText("Open…")
	r.openBtn.OnDown(func(context *guigui.Context) { m.BrowseOpen() })
	r.saveBtn.SetText("Save")
	r.saveBtn.OnDown(func(context *guigui.Context) {
		// Saving a bundle that has no path yet has to ask where to put it.
		if m.Path() == "" {
			m.BrowseSaveAs()
			return
		}
		m.Save(m.Path())
	})
	r.saveAsBtn.SetText("Save As…")
	r.saveAsBtn.OnDown(func(context *guigui.Context) { m.BrowseSaveAs() })
	r.configBtn.SetText("Config")
	r.configBtn.OnDown(func(context *guigui.Context) { m.ShowConfigPane() })

	// A second dialog while one is already up would be ignored anyway; grey
	// the buttons so that is visible. Viewer mode also greys saving: the
	// disk is the source of truth there, and saving over a file another
	// tool is writing would fight it.
	context.SetEnabled(&r.openBtn, !m.DialogOpen())
	context.SetEnabled(&r.configBtn, !m.DialogOpen())
	for _, w := range []guigui.Widget{&r.saveBtn, &r.saveAsBtn} {
		context.SetEnabled(w, !m.DialogOpen() && !m.Viewer())
	}

	r.graphPanel.SetContent(&r.graph)
	// The graph and the stage are the two working surfaces; outline both so
	// they read as areas rather than as loose content between control panes.
	r.graphFrame.SetContent(&r.graphPanel)
	r.previewFrame.SetContent(&r.preview)

	r.status.SetValue(m.Status())
	r.status.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	r.status.SetScale(0.9)
	return nil
}

// Tick applies whatever a file dialog finished with. The dialog runs on its
// own goroutine, so the result lands outside any handler; applying it bumps
// the generation, and the state key below turns that into a rebuild.
func (r *Root) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	r.model.PumpDialogs()
	// Viewer mode can be toggled at runtime (Config pane) as well as by
	// the -viewer flag, so the watcher follows the model here.
	if r.model.Viewer() && r.watch == nil {
		r.watch = newWatcher(r.model)
	}
	if !r.model.Viewer() {
		r.watch = nil
	}
	if r.watch != nil {
		r.watch.tick()
	}
	return nil
}

func (r *Root) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteInt(r.model.Generation())
	w.WriteBool(r.model.DialogOpen())
}

func (r *Root) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&r.background, widgetBounds.Bounds())
	u := basicwidget.UnitSize(context)

	r.toolbarItems = slices.Delete(r.toolbarItems, 0, len(r.toolbarItems))
	r.toolbarItems = append(r.toolbarItems,
		guigui.LinearLayoutItem{Widget: &r.pathLabel, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &r.openBtn, Size: guigui.FixedSize(4 * u)},
		guigui.LinearLayoutItem{Widget: &r.saveBtn, Size: guigui.FixedSize(3 * u)},
		guigui.LinearLayoutItem{Widget: &r.saveAsBtn, Size: guigui.FixedSize(4 * u)},
		guigui.LinearLayoutItem{Widget: &r.configBtn, Size: guigui.FixedSize(4 * u)},
	)
	r.toolbar = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal, Items: r.toolbarItems, Gap: u / 4,
	}

	// The graph and the preview share the middle column: you watch the
	// machine run right under the graph you are editing.
	r.centerItems = slices.Delete(r.centerItems, 0, len(r.centerItems))
	r.centerItems = append(r.centerItems,
		guigui.LinearLayoutItem{Widget: &r.graphFrame, Size: guigui.FlexibleSize(3)},
		guigui.LinearLayoutItem{Widget: &r.previewFrame, Size: guigui.FlexibleSize(2)},
	)
	r.center = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical, Items: r.centerItems, Gap: u / 4,
	}

	r.middleItems = slices.Delete(r.middleItems, 0, len(r.middleItems))
	r.middleItems = append(r.middleItems,
		// The clips pane carries three columns now — name, source, control —
		// so it needs more than the fifth of the width it had.
		guigui.LinearLayoutItem{Widget: &r.clips, Size: guigui.FlexibleSize(4)},
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(7), Layout: &r.center},
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
	viewer := flag.Bool("viewer", false,
		"viewer mode: watch every loaded file (.lottie, .json, images) and auto-reload on change; saving is disabled")
	flag.Parse()
	root := &Root{model: NewModel()}
	root.model.SetViewer(*viewer)
	if flag.NArg() > 0 {
		root.model.Open(flag.Arg(0))
	}
	title := "Lottie State Machine Editor"
	if *viewer {
		title += " (viewer)"
	}
	op := &guigui.RunOptions{
		Title: title,
		// Four columns of content: clips and machines, the graph, the
		// stage, and the inspector. The old 1280 default left the
		// inspector too narrow to show a transition's target.
		WindowSize:    image.Pt(1520, 920),
		WindowMinSize: image.Pt(1180, 700),
	}
	if err := runWithOptionalScreenshot(root, op); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
