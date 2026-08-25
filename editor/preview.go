package main

import (
	"image"
	"slices"
	"strconv"
	"strings"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"

	lottie "github.com/shibukawa/lottie-go"
)

// previewPane runs the machine with the real interpreter, so what it shows
// is what a game gets.
type previewPane struct {
	guigui.DefaultWidget

	stage      previewStage
	stateLabel basicwidget.Text
	hint       basicwidget.Text
	restart    basicwidget.Button
	triggers   guigui.WidgetSlice[*basicwidget.Button]

	// Value inputs need controls too, or a machine guarded on a boolean
	// cannot be exercised here at all.
	valueForm   basicwidget.Form
	valueLabels guigui.WidgetSlice[*basicwidget.Text]
	valueChecks guigui.WidgetSlice[*basicwidget.Checkbox]
	valueTexts  guigui.WidgetSlice[*basicwidget.TextInput]
	valueItems  []basicwidget.FormItem

	items        []guigui.LinearLayoutItem
	triggerRow   guigui.LinearLayout
	triggerItems []guigui.LinearLayoutItem
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
	adder.AddWidget(&p.restart)

	names := m.EventInputs()
	p.triggers.SetLen(len(names))
	for i := range names {
		adder.AddWidget(p.triggers.At(i))
	}
	for i, name := range names {
		b := p.triggers.At(i)
		b.SetText(name)
		b.OnDown(func(context *guigui.Context) {
			if sm := m.Preview(); sm != nil {
				sm.Fire(name)
			}
		})
		context.SetEnabled(b, m.Preview() != nil)
	}

	adder.AddWidget(&p.valueForm)
	p.buildValueInputs(context, m)

	switch {
	case m.PreviewErr() != nil:
		p.stateLabel.SetValue("preview error")
		p.hint.SetValue(m.PreviewErr().Error())
	case m.Preview() == nil:
		p.stateLabel.SetValue("no preview")
		p.hint.SetValue("Select or create a state machine.")
	default:
		p.stateLabel.SetValue("state: " + m.Preview().State())
		hint := ""
		if m.PreviewStale() {
			hint = "edited — restart to apply"
		}
		if u := m.Preview().UnsupportedFeatures(); len(u) > 0 {
			if hint != "" {
				hint += "   "
			}
			hint += "skipped: " + u[0]
		}
		p.hint.SetValue(hint)
	}
	p.stateLabel.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	p.hint.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	p.hint.SetScale(0.85)

	p.restart.SetText("Restart")
	p.restart.OnDown(func(context *guigui.Context) {
		m.RestartPreview()
	})
	return nil
}

// buildValueInputs gives every non-event input a control: a checkbox for a
// boolean, a field for a number or string.
func (p *previewPane) buildValueInputs(context *guigui.Context, m *Model) {
	var inputs []lottie.Input
	if sm := m.Machine(); sm != nil {
		for _, in := range sm.Inputs {
			if in.Type != lottie.InputEvent {
				inputs = append(inputs, in)
			}
		}
	}
	var bools, texts int
	for _, in := range inputs {
		if in.Type == lottie.InputBoolean {
			bools++
		} else {
			texts++
		}
	}
	p.valueLabels.SetLen(len(inputs))
	p.valueChecks.SetLen(bools)
	p.valueTexts.SetLen(texts)

	p.valueItems = slices.Delete(p.valueItems, 0, len(p.valueItems))
	sm := m.Preview()
	bi, ti := 0, 0
	for i, in := range inputs {
		label := p.valueLabels.At(i)
		adderLabel(label, in.Name)
		var control guigui.Widget
		switch in.Type {
		case lottie.InputBoolean:
			c := p.valueChecks.At(bi)
			bi++
			if sm != nil {
				v, _ := sm.Get[bool](in.Name)
				c.SetValue(v)
			}
			c.OnValueChanged(func(context *guigui.Context, value bool) {
				if sm != nil {
					sm.Set(in.Name, value)
				}
			})
			control = c
		default:
			t := p.valueTexts.At(ti)
			ti++
			if sm != nil {
				if in.Type == lottie.InputNumeric {
					v, _ := sm.Get[float64](in.Name)
					t.SetValue(strconv.FormatFloat(v, 'g', -1, 64))
				} else {
					v, _ := sm.Get[string](in.Name)
					t.SetValue(v)
				}
			}
			kind := in.Type
			t.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
				if !committed || sm == nil {
					return
				}
				if kind == lottie.InputNumeric {
					if v, err := strconv.ParseFloat(strings.TrimSpace(text), 64); err == nil {
						sm.Set(in.Name, v)
					}
					return
				}
				sm.Set(in.Name, text)
			})
			control = t
		}
		context.SetEnabled(control, sm != nil)
		p.valueItems = append(p.valueItems, basicwidget.FormItem{
			PrimaryWidget: label, SecondaryWidget: control,
		})
	}
	p.valueForm.SetItems(p.valueItems)
}

func adderLabel(t *basicwidget.Text, s string) {
	t.SetValue(s)
	t.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	t.SetScale(0.85)
}

// WriteStateKey rebuilds when the machine moves to another state, so the
// label follows playback. The per-frame animation is a redraw, not a
// rebuild; previewStage requests that itself.
func (p *previewPane) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	m := p.model(context)
	if m == nil {
		return
	}
	w.WriteInt(m.Generation())
	if sm := m.Preview(); sm != nil {
		w.WriteString(sm.State())
	}
}

func (p *previewPane) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)

	p.triggerItems = slices.Delete(p.triggerItems, 0, len(p.triggerItems))
	p.triggerItems = append(p.triggerItems, guigui.LinearLayoutItem{
		Widget: &p.restart, Size: guigui.FixedSize(4 * u),
	})
	for i := range p.triggers.Len() {
		p.triggerItems = append(p.triggerItems, guigui.LinearLayoutItem{
			Widget: p.triggers.At(i), Size: guigui.FixedSize(4 * u),
		})
	}
	p.triggerItems = append(p.triggerItems, guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)})
	p.triggerRow = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     p.triggerItems,
		Gap:       u / 4,
	}

	p.items = slices.Delete(p.items, 0, len(p.items))
	p.items = append(p.items,
		guigui.LinearLayoutItem{Widget: &p.stage, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &p.stateLabel, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &p.hint, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &p.valueForm},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &p.triggerRow},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     p.items,
		Gap:       u / 4,
		Padding:   guigui.Padding{Start: u / 2, Top: u / 2, End: u / 2, Bottom: u / 2},
	}).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

// previewStage renders the running machine, scaled to fit.
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
	sm := m.Preview()
	if sm == nil {
		return nil
	}
	sm.Update()
	guigui.RequestRedraw(s)
	return nil
}

func (s *previewStage) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	m := s.model(context)
	if m == nil {
		return
	}
	sm := m.Preview()
	if sm == nil {
		return
	}
	pl := sm.Player()
	if pl == nil {
		return
	}
	aw, ah := pl.Animation().Size()
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
	sm.Draw(dst, &op)
}

func (s *previewStage) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	return image.Pt(8*u, 6*u)
}
