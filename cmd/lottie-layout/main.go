// Command layout arranges Lottie clips and state machines into scenes —
// game screens and GUI menus — and previews them with the same runtime a
// game uses. It is the third layer over lottie-go: the player plays one
// clip, the state machine editor sequences clips into one actor, and this
// tool arranges many actors into one screen.
package main

import (
	"fmt"
	"image"
	// Scene image references decode through image.Decode; the formats
	// come from these imports, per the library's asset convention.
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"slices"

	_ "golang.org/x/image/webp"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	_ "github.com/guigui-gui/guigui/basicwidget/cjkfont"
)

// envKeyModel publishes the document to every pane, so none of them needs
// a pointer threaded through its parent.
var envKeyModel = guigui.GenerateEnvKey()

type Root struct {
	guigui.DefaultWidget

	background basicwidget.Background

	pathLabel  basicwidget.Text
	openBtn    basicwidget.Button
	saveBtn    basicwidget.Button
	saveAsBtn  basicwidget.Button
	sceneBtn   basicwidget.Button
	previewBtn basicwidget.Button

	palette       palettePane
	canvas        canvasView
	canvasFrame   framedPane
	timeline      timelinePane
	timelineFrame framedPane
	inspector     inspectorPane
	status        basicwidget.Text

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
	adder.AddWidget(&r.pathLabel)
	adder.AddWidget(&r.openBtn)
	adder.AddWidget(&r.saveBtn)
	adder.AddWidget(&r.saveAsBtn)
	adder.AddWidget(&r.sceneBtn)
	adder.AddWidget(&r.previewBtn)
	adder.AddWidget(&r.palette)
	adder.AddWidget(&r.canvasFrame)
	adder.AddWidget(&r.timelineFrame)
	adder.AddWidget(&r.inspector)
	adder.AddWidget(&r.status)

	m := r.model
	name := "(unsaved scene)"
	if p := m.Path(); p != "" {
		name = filepath.Base(p)
	}
	r.pathLabel.SetValue(name)
	r.pathLabel.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	r.openBtn.SetText("Open…")
	r.openBtn.OnDown(func(context *guigui.Context) { m.BrowseOpen() })
	r.saveBtn.SetText("Save")
	r.saveBtn.OnDown(func(context *guigui.Context) {
		if m.Path() == "" {
			m.BrowseSaveAs()
			return
		}
		m.Save(m.Path())
	})
	r.saveAsBtn.SetText("Save As…")
	r.saveAsBtn.OnDown(func(context *guigui.Context) { m.BrowseSaveAs() })
	r.sceneBtn.SetText("Scene")
	r.sceneBtn.OnDown(func(context *guigui.Context) { m.ShowScenePane() })
	if m.PreviewMode() {
		r.previewBtn.SetText("Edit")
	} else {
		r.previewBtn.SetText("Preview")
	}
	r.previewBtn.OnDown(func(context *guigui.Context) { m.TogglePreview() })

	for _, w := range []guigui.Widget{&r.openBtn, &r.saveBtn, &r.saveAsBtn} {
		context.SetEnabled(w, !m.DialogOpen())
	}

	r.canvasFrame.SetContent(&r.canvas)
	r.timelineFrame.SetContent(&r.timeline)

	r.status.SetValue(m.Status())
	r.status.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	r.status.SetScale(0.9)
	return nil
}

// Tick applies whatever a file dialog finished with; the dialog runs on
// its own goroutine, so the result lands outside any handler.
func (r *Root) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	r.model.PumpDialogs()
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
		guigui.LinearLayoutItem{Widget: &r.sceneBtn, Size: guigui.FixedSize(3 * u)},
		guigui.LinearLayoutItem{Widget: &r.previewBtn, Size: guigui.FixedSize(4 * u)},
	)
	r.toolbar = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal, Items: r.toolbarItems, Gap: u / 4,
	}

	// The canvas and the timeline share the middle column: you watch the
	// choreography run right under the scene you are arranging.
	r.centerItems = slices.Delete(r.centerItems, 0, len(r.centerItems))
	r.centerItems = append(r.centerItems,
		guigui.LinearLayoutItem{Widget: &r.canvasFrame, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &r.timelineFrame, Size: guigui.FixedSize(7 * u)},
	)
	r.center = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical, Items: r.centerItems, Gap: u / 4,
	}

	r.middleItems = slices.Delete(r.middleItems, 0, len(r.middleItems))
	r.middleItems = append(r.middleItems,
		guigui.LinearLayoutItem{Widget: &r.palette, Size: guigui.FlexibleSize(3)},
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(8), Layout: &r.center},
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
		Title:         "Lottie Scene Layout",
		WindowSize:    image.Pt(1520, 920),
		WindowMinSize: image.Pt(1180, 700),
	}
	if err := runWithOptionalScreenshot(root, op); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
