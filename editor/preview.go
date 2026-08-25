package main

import (
	"image"
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"

	lottie "github.com/shibukawa/lottie-go"
)

// previewPane shows either the machine, driven by the same interpreter a
// game uses, or one clip on its own. Everything that drives it lives in the
// Inputs table, so this pane is only the stage and its status.
type previewPane struct {
	guigui.DefaultWidget

	stage         previewStage
	stateLabel    basicwidget.Text
	hint          basicwidget.Text
	backToMachine basicwidget.Button

	items []guigui.LinearLayoutItem
}

func (p *previewPane) model(context *guigui.Context) *Model {
	v, ok := context.Env(p, envKeyModel)
	if !ok {
		return nil
	}
	m, _ := v.(*Model)
	return m
}

func (p *previewPane) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m := p.model(context)
	if m == nil {
		return nil
	}
	adder.AddWidget(&p.stage)
	adder.AddWidget(&p.stateLabel)
	adder.AddWidget(&p.hint)
	// Only offered while a clip has taken the stage.
	if m.PreviewClip() != "" {
		adder.AddWidget(&p.backToMachine)
	}

	p.stateLabel.SetValue(m.PreviewLabel())
	p.stateLabel.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	hint := ""
	switch {
	case m.PreviewClip() != "":
		hint = "playing one clip; the machine is paused"
	case m.PreviewErr() != nil:
		hint = m.PreviewErr().Error()
	case m.Preview() == nil:
		hint = "Select or create a state machine."
	default:
		if m.PreviewStale() {
			hint = "edited — Restart to apply"
		}
		if u := m.Preview().UnsupportedFeatures(); len(u) > 0 {
			if hint != "" {
				hint += "   "
			}
			hint += "skipped: " + u[0]
		}
	}
	p.hint.SetValue(hint)
	p.hint.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	p.hint.SetScale(0.85)

	p.backToMachine.SetText("Back to machine")
	p.backToMachine.OnDown(func(context *guigui.Context) { m.ShowMachine() })
	return nil
}

// WriteStateKey rebuilds when the machine moves to another state, so the
// label and the graph highlight follow playback. The per-frame animation is
// a redraw, not a rebuild; previewStage requests that itself.
func (p *previewPane) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	m := p.model(context)
	if m == nil {
		return
	}
	w.WriteInt(m.Generation())
	w.WriteString(m.ActiveState())
	w.WriteString(m.PreviewClip())
}

func (p *previewPane) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	m := p.model(context)

	p.items = slices.Delete(p.items, 0, len(p.items))
	p.items = append(p.items,
		guigui.LinearLayoutItem{Widget: &p.stage, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &p.stateLabel, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &p.hint, Size: guigui.FixedSize(u)},
	)
	if m != nil && m.PreviewClip() != "" {
		p.items = append(p.items, guigui.LinearLayoutItem{
			Widget: &p.backToMachine, Size: guigui.FixedSize(u)})
	}
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     p.items,
		Gap:       u / 4,
		Padding:   guigui.Padding{Start: u / 2, Top: u / 2, End: u / 2, Bottom: u / 2},
	}).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

// previewStage renders whatever the preview is showing, scaled to fit.
type previewStage struct {
	guigui.DefaultWidget
}

func (s *previewStage) model(context *guigui.Context) *Model {
	v, ok := context.Env(s, envKeyModel)
	if !ok {
		return nil
	}
	m, _ := v.(*Model)
	return m
}

// Tick advances playback. The frame changes without any tree change, so it
// asks for a redraw rather than a rebuild.
func (s *previewStage) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	m := s.model(context)
	if m == nil {
		return nil
	}
	m.PreviewUpdate()
	guigui.RequestRedraw(s)
	return nil
}

func (s *previewStage) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	m := s.model(context)
	if m == nil {
		return
	}
	anim := m.PreviewAnimation()
	if anim == nil {
		return
	}
	aw, ah := anim.Size()
	if aw <= 0 || ah <= 0 {
		return
	}
	b := widgetBounds.Bounds()
	scale := min(float64(b.Dx())/float64(aw), float64(b.Dy())/float64(ah))
	var op lottie.DrawOptions
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(
		float64(b.Min.X)+(float64(b.Dx())-float64(aw)*scale)/2,
		float64(b.Min.Y)+(float64(b.Dy())-float64(ah)*scale)/2,
	)
	m.PreviewDraw(dst, &op)
}

func (s *previewStage) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	return image.Pt(8*u, 6*u)
}
