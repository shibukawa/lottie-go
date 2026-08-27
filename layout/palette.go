package main

import (
	"fmt"
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
)

// palettePane is the left column: what the referenced files offer for
// placing, with a live preview of the selection. Placed nodes live on the
// timeline — one list of nodes is enough, and the timeline already is
// one. Reference lists are not needed either: after Add there is nothing
// to do to a file, and a broken reference shows up in Problems.
type palettePane struct {
	guigui.DefaultWidget

	sourcesTitle basicwidget.Text
	addBundle    basicwidget.Button
	addImage     basicwidget.Button
	addFont      basicwidget.Button
	addText      basicwidget.Button
	sourcesList  basicwidget.List[int]
	preview      sourcePreview
	placeBtn     basicwidget.Button

	sourceItems []basicwidget.ListItem[int]

	sourceHdrRow   guigui.LinearLayout
	sourceHdrItems []guigui.LinearLayoutItem
	items          []guigui.LinearLayoutItem
}

func (p *palettePane) model(context *guigui.Context) *Model {
	v, ok := context.Env(p, envKeyModel)
	if !ok {
		return nil
	}
	m, _ := v.(*Model)
	return m
}

func (p *palettePane) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m := p.model(context)
	if m == nil {
		return nil
	}
	for _, w := range []guigui.Widget{
		&p.sourcesTitle, &p.addBundle, &p.addImage, &p.addFont, &p.addText,
		&p.sourcesList, &p.preview, &p.placeBtn,
	} {
		adder.AddWidget(w)
	}

	setBold(&p.sourcesTitle, "Sources")
	p.addBundle.SetText("+Bundle…")
	p.addBundle.OnDown(func(context *guigui.Context) { m.BrowseAddBundle() })
	p.addImage.SetText("+Image…")
	p.addImage.OnDown(func(context *guigui.Context) { m.BrowseAddImage() })
	p.addFont.SetText("+Font…")
	p.addFont.OnDown(func(context *guigui.Context) { m.BrowseAddFont() })
	p.addText.SetText("+Text")
	p.addText.OnDown(func(context *guigui.Context) { m.AddTextNode() })
	for _, w := range []guigui.Widget{&p.addBundle, &p.addImage, &p.addFont} {
		context.SetEnabled(w, !m.DialogOpen())
	}

	sources := m.Sources()
	// Something is always previewed once sources exist; an empty preview
	// box reads as broken.
	if m.SelectedSource() < 0 && len(sources) > 0 {
		m.selSource = 0
	}
	p.sourceItems = slices.Delete(p.sourceItems, 0, len(p.sourceItems))
	for i, s := range sources {
		key := "image"
		if s.Kind == sourceBundle {
			// Summarize what the one node will offer, so a bundle row
			// still says what is inside without exploding into parts.
			if b, ok := m.Bundle(s.Alias); ok {
				key = fmt.Sprintf("%d clips, %d machines", len(b.AnimationIDs()), len(b.StateMachineIDs()))
			} else {
				key = "bundle"
			}
		}
		p.sourceItems = append(p.sourceItems, basicwidget.ListItem[int]{
			Text: s.Alias, KeyText: key, Value: i,
		})
	}
	p.sourcesList.SetItems(p.sourceItems)
	if sel := m.SelectedSource(); sel >= 0 && sel < len(sources) {
		p.sourcesList.SelectItemByValue(sel)
	}
	p.sourcesList.OnItemSelected(func(context *guigui.Context, index int) {
		m.SelectSource(index)
	})
	p.placeBtn.SetText("Place")
	p.placeBtn.OnDown(func(context *guigui.Context) {
		if ref, ok := m.SelectedSourceRef(); ok {
			m.PlaceSource(ref)
		}
	})
	_, hasSource := m.SelectedSourceRef()
	context.SetEnabled(&p.placeBtn, hasSource)
	return nil
}

func (p *palettePane) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	if m := p.model(context); m != nil {
		w.WriteInt(m.Generation())
	}
}

func (p *palettePane) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)

	p.sourceHdrItems = slices.Delete(p.sourceHdrItems, 0, len(p.sourceHdrItems))
	p.sourceHdrItems = append(p.sourceHdrItems,
		guigui.LinearLayoutItem{Widget: &p.addBundle, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &p.addImage, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &p.addFont, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &p.addText, Size: guigui.FlexibleSize(1)},
	)
	p.sourceHdrRow = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal, Items: p.sourceHdrItems, Gap: u / 4,
	}

	p.items = slices.Delete(p.items, 0, len(p.items))
	p.items = append(p.items,
		guigui.LinearLayoutItem{Widget: &p.sourcesTitle, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &p.sourceHdrRow},
		guigui.LinearLayoutItem{Widget: &p.sourcesList, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &p.preview, Size: guigui.FixedSize(7 * u)},
		guigui.LinearLayoutItem{Widget: &p.placeBtn, Size: guigui.FixedSize(u)},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     p.items, Gap: u / 4,
		Padding: guigui.Padding{Start: u / 2, Top: u / 2, End: u / 2, Bottom: u / 2},
	}).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}
