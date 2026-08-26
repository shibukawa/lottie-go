package main

import (
	"fmt"
	"image"
	"image/color"
	"slices"
	"strconv"
	"strings"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	lottiecp "github.com/shibukawa/lottie-go/plugin/physics/cp"
	lottieresolv "github.com/shibukawa/lottie-go/plugin/physics/resolv"
)

// Collision overlay and its panel. The overlay paints the stage's live
// hitboxes and the bundle's cp body over the animation; the panel under the
// timeline adds, tags, and times them. Dragging happens on the stage
// itself, in preview.go.

// tagColor maps a box's meaning to the fighting-game convention: attacks
// red, vulnerability green, movement collision amber. The first recognized
// tag wins; an untagged box is gray so it visibly still needs a meaning.
func tagColor(tags []string) color.NRGBA {
	for _, t := range tags {
		switch t {
		case "hit", "attack":
			return color.NRGBA{0xe5, 0x48, 0x4d, 0xff}
		case "hurt":
			return color.NRGBA{0x30, 0xa4, 0x6c, 0xff}
		case "push", "collision", "body":
			return color.NRGBA{0xf5, 0xa5, 0x24, 0xff}
		}
	}
	return color.NRGBA{0x8a, 0x93, 0xa6, 0xff}
}

// cpShapeColor marks the rigid body silhouette apart from any hitbox tag.
var cpShapeColor = color.NRGBA{0x8e, 0x4e, 0xc6, 0xff}

func withAlpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}

// stageTransform maps animation coordinates onto the stage rectangle the
// same way previewStage.Draw scales the drawing, so overlays and hit tests
// land exactly on the rendered pixels.
type stageTransform struct {
	scale  float64
	ox, oy float64
}

func (t stageTransform) toScreen(x, y float64) (float32, float32) {
	return float32(t.ox + x*t.scale), float32(t.oy + y*t.scale)
}

func (t stageTransform) toAnim(x, y int) (float64, float64) {
	return (float64(x) - t.ox) / t.scale, (float64(y) - t.oy) / t.scale
}

// ---- overlay drawing ----

// drawCollisionOverlay paints the cp body first and the live hitboxes over
// it: frame-stepped judgement is what this stage is for, so it wins ties.
func drawCollisionOverlay(dst *ebiten.Image, m *Model, tr stageTransform, u float32) {
	stroke := max(1, u/12)
	for i, s := range m.CPBodyShapes() {
		drawCPShape(dst, tr, s, stroke, i == m.SelectedCPShapeIndex(), u)
	}
	track := m.StageTrack()
	if track == nil {
		return
	}
	frame := m.stageFrame()
	for _, ab := range track.At(frame) {
		drawActiveBox(dst, tr, ab, stroke, ab.Index == m.SelectedHitboxIndex(), u)
	}
}

func drawActiveBox(dst *ebiten.Image, tr stageTransform, ab lottieresolv.ActiveBox, stroke float32, selected bool, u float32) {
	clr := tagColor(ab.Tags)
	fill := withAlpha(clr, 0x40)
	edge := withAlpha(clr, 0xe6)
	if selected {
		stroke *= 2
	}
	if ab.Kind == lottieresolv.KindCircle {
		cx, cy := tr.toScreen(ab.X, ab.Y)
		r := float32(ab.R * tr.scale)
		vector.DrawFilledCircle(dst, cx, cy, r, fill, true)
		vector.StrokeCircle(dst, cx, cy, r, stroke, edge, true)
		if selected {
			drawHandle(dst, cx+r, cy, u)
		}
		return
	}
	x, y := tr.toScreen(ab.X, ab.Y)
	w := float32(ab.W * tr.scale)
	h := float32(ab.H * tr.scale)
	vector.DrawFilledRect(dst, x, y, w, h, fill, true)
	vector.StrokeRect(dst, x, y, w, h, stroke, edge, true)
	if selected {
		drawHandle(dst, x+w, y+h, u)
	}
}

func drawCPShape(dst *ebiten.Image, tr stageTransform, s lottiecp.Shape, stroke float32, selected bool, u float32) {
	fill := withAlpha(cpShapeColor, 0x30)
	edge := withAlpha(cpShapeColor, 0xd0)
	if selected {
		stroke *= 2
	}
	switch s.Type {
	case lottiecp.ShapeCircle:
		cx, cy := tr.toScreen(s.Center.X, s.Center.Y)
		r := float32(s.Radius * tr.scale)
		vector.DrawFilledCircle(dst, cx, cy, r, fill, true)
		vector.StrokeCircle(dst, cx, cy, r, stroke, edge, true)
		if selected {
			drawHandle(dst, cx+r, cy, u)
		}
	case lottiecp.ShapeBox:
		x, y := tr.toScreen(s.Center.X-s.Width/2, s.Center.Y-s.Height/2)
		w := float32(s.Width * tr.scale)
		h := float32(s.Height * tr.scale)
		vector.DrawFilledRect(dst, x, y, w, h, fill, true)
		vector.StrokeRect(dst, x, y, w, h, stroke, edge, true)
		if selected {
			drawHandle(dst, x+w, y+h, u)
		}
	case lottiecp.ShapePolygon:
		// The editor never authors polygons, but a hand-written one must
		// still show; an outline is enough for that.
		n := len(s.Vertices)
		for i := range n {
			x0, y0 := tr.toScreen(s.Vertices[i].X, s.Vertices[i].Y)
			x1, y1 := tr.toScreen(s.Vertices[(i+1)%n].X, s.Vertices[(i+1)%n].Y)
			vector.StrokeLine(dst, x0, y0, x1, y1, stroke, edge, true)
		}
	}
}

// drawHandle marks the resize grip of the selected shape.
func drawHandle(dst *ebiten.Image, x, y float32, u float32) {
	s := handleSize(u)
	vector.DrawFilledRect(dst, x-s/2, y-s/2, s, s, color.NRGBA{0xff, 0xff, 0xff, 0xff}, true)
	vector.StrokeRect(dst, x-s/2, y-s/2, s, s, max(1, u/16), color.NRGBA{0x20, 0x20, 0x28, 0xff}, true)
}

func handleSize(u float32) float32 { return max(6, u/3) }

// ---- hit testing (screen coordinates) ----

// handleAt returns the selected shape's resize grip position, if any shape
// is selected.
func handleAt(m *Model, tr stageTransform) (float32, float32, bool) {
	if b := m.SelectedHitbox(); b != nil {
		if sp := m.SelectedSpan(); sp != nil {
			if b.Kind == lottieresolv.KindCircle {
				cx, cy := tr.toScreen(sp.X, sp.Y)
				return cx + float32(sp.R*tr.scale), cy, true
			}
			x, y := tr.toScreen(sp.X+sp.W, sp.Y+sp.H)
			return x, y, true
		}
	}
	if s := m.SelectedCPShape(); s != nil {
		switch s.Type {
		case lottiecp.ShapeCircle:
			cx, cy := tr.toScreen(s.Center.X, s.Center.Y)
			return cx + float32(s.Radius*tr.scale), cy, true
		case lottiecp.ShapeBox:
			x, y := tr.toScreen(s.Center.X+s.Width/2, s.Center.Y+s.Height/2)
			return x, y, true
		}
	}
	return 0, 0, false
}

// hitTestBoxes returns the topmost live hitbox under an animation-space
// point. Later boxes draw later, so they win.
func hitTestBoxes(m *Model, ax, ay float64) (int, bool) {
	track := m.StageTrack()
	if track == nil {
		return 0, false
	}
	live := track.At(m.stageFrame())
	for i := len(live) - 1; i >= 0; i-- {
		ab := live[i]
		if ab.Kind == lottieresolv.KindCircle {
			if (ax-ab.X)*(ax-ab.X)+(ay-ab.Y)*(ay-ab.Y) <= ab.R*ab.R {
				return ab.Index, true
			}
			continue
		}
		if ax >= ab.X && ax < ab.X+ab.W && ay >= ab.Y && ay < ab.Y+ab.H {
			return ab.Index, true
		}
	}
	return 0, false
}

// hitTestCPShapes returns the topmost body shape under an animation-space
// point.
func hitTestCPShapes(m *Model, ax, ay float64) (int, bool) {
	shapes := m.CPBodyShapes()
	for i := len(shapes) - 1; i >= 0; i-- {
		s := shapes[i]
		switch s.Type {
		case lottiecp.ShapeCircle:
			dx, dy := ax-s.Center.X, ay-s.Center.Y
			if dx*dx+dy*dy <= s.Radius*s.Radius {
				return i, true
			}
		case lottiecp.ShapeBox:
			if ax >= s.Center.X-s.Width/2 && ax <= s.Center.X+s.Width/2 &&
				ay >= s.Center.Y-s.Height/2 && ay <= s.Center.Y+s.Height/2 {
				return i, true
			}
		case lottiecp.ShapePolygon:
			if pointInConvex(s.Vertices, ax, ay) {
				return i, true
			}
		}
	}
	return 0, false
}

func pointInConvex(vs []lottiecp.Point, x, y float64) bool {
	n := len(vs)
	if n < 3 {
		return false
	}
	sign := 0.0
	for i := range n {
		a, b := vs[i], vs[(i+1)%n]
		cross := (b.X-a.X)*(y-a.Y) - (b.Y-a.Y)*(x-a.X)
		if cross == 0 {
			continue
		}
		if sign == 0 {
			sign = cross
		} else if (cross > 0) != (sign > 0) {
			return false
		}
	}
	return true
}

// ---- panel ----

// collisionPanel is the strip under the timeline: hitbox row (per-clip,
// frame-stepped) and body row (bundle-wide, fixed).
type collisionPanel struct {
	guigui.DefaultWidget

	showLabel basicwidget.Text
	showCheck basicwidget.Checkbox
	boxCombo  basicwidget.Combobox
	addRect   basicwidget.Button
	addCircle basicwidget.Button
	delBox    basicwidget.Button
	nameInput basicwidget.TextInput
	tagsInput basicwidget.TextInput

	spanLabel basicwidget.Text
	fromInput basicwidget.TextInput
	toInput   basicwidget.TextInput
	addSpan   basicwidget.Button
	delSpan   basicwidget.Button
	bodyLabel basicwidget.Text
	addCPCirc basicwidget.Button
	addCPBox  basicwidget.Button
	delCP     basicwidget.Button

	rowA      guigui.LinearLayout
	rowAItems []guigui.LinearLayoutItem
	rowB      guigui.LinearLayout
	rowBItems []guigui.LinearLayoutItem
	items     []guigui.LinearLayoutItem
}

func (c *collisionPanel) model(context *guigui.Context) *Model {
	v, ok := context.Env(c, envKeyModel)
	if !ok {
		return nil
	}
	m, _ := v.(*Model)
	return m
}

func (c *collisionPanel) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m := c.model(context)
	if m == nil {
		return nil
	}
	for _, w := range []guigui.Widget{
		&c.showLabel, &c.showCheck, &c.boxCombo, &c.addRect, &c.addCircle,
		&c.delBox, &c.nameInput, &c.tagsInput,
		&c.spanLabel, &c.fromInput, &c.toInput, &c.addSpan, &c.delSpan,
		&c.bodyLabel, &c.addCPCirc, &c.addCPBox, &c.delCP,
	} {
		adder.AddWidget(w)
	}

	onStage := m.StageAnimID() != ""

	label(&c.showLabel, "hitboxes")
	c.showCheck.SetValue(m.ShowCollision())
	c.showCheck.OnValueChanged(func(context *guigui.Context, v bool) {
		m.SetShowCollision(v)
	})

	var labels []string
	if t := m.StageTrack(); t != nil {
		for i, b := range t.Boxes {
			labels = append(labels, HitboxLabel(i, b))
		}
	}
	c.boxCombo.SetItems(labels)
	if i := m.SelectedHitboxIndex(); i >= 0 && i < len(labels) {
		c.boxCombo.SetValue(labels[i])
	} else {
		c.boxCombo.SetValue("")
	}
	c.boxCombo.OnValueChanged(func(context *guigui.Context, value string, committed bool) {
		if !committed {
			return
		}
		if n, _, ok := strings.Cut(value, ":"); ok {
			if i, err := strconv.Atoi(n); err == nil {
				m.SelectHitbox(i - 1)
			}
		}
	})

	c.addRect.SetText("+Rect")
	c.addRect.OnDown(func(context *guigui.Context) { m.AddHitbox(lottieresolv.KindRect) })
	c.addCircle.SetText("+Circle")
	c.addCircle.OnDown(func(context *guigui.Context) { m.AddHitbox(lottieresolv.KindCircle) })
	c.delBox.SetText("Del")
	c.delBox.OnDown(func(context *guigui.Context) { m.DeleteHitbox() })

	sel := m.SelectedHitbox()
	if sel != nil {
		c.nameInput.SetValue(sel.Name)
	} else {
		c.nameInput.SetValue("")
	}
	c.nameInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed {
			m.RenameHitbox(text)
		}
	})
	c.tagsInput.SetValue(m.HitboxTagsCSV())
	c.tagsInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed {
			m.SetHitboxTagsCSV(text)
		}
	})

	label(&c.spanLabel, "span")
	sp := m.SelectedSpan()
	if sp != nil {
		c.fromInput.SetValue(strconv.FormatFloat(sp.From, 'g', -1, 64))
		c.toInput.SetValue(strconv.FormatFloat(sp.To, 'g', -1, 64))
	} else {
		c.fromInput.SetValue("")
		c.toInput.SetValue("")
	}
	commitRange := func(context *guigui.Context) {
		from, err1 := strconv.ParseFloat(strings.TrimSpace(c.fromInput.Value()), 64)
		to, err2 := strconv.ParseFloat(strings.TrimSpace(c.toInput.Value()), 64)
		if err1 == nil && err2 == nil {
			m.SetSpanRange(from, to)
		}
	}
	c.fromInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed {
			commitRange(context)
		}
	})
	c.toInput.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed {
			commitRange(context)
		}
	})
	c.addSpan.SetText("+Span")
	c.addSpan.OnDown(func(context *guigui.Context) { m.AddHitboxSpan() })
	c.delSpan.SetText("Del span")
	c.delSpan.OnDown(func(context *guigui.Context) { m.DeleteHitboxSpan() })

	label(&c.bodyLabel, fmt.Sprintf("body (%d)", len(m.CPBodyShapes())))
	c.addCPCirc.SetText("+Circle")
	c.addCPCirc.OnDown(func(context *guigui.Context) { m.AddCPShape(lottiecp.ShapeCircle) })
	c.addCPBox.SetText("+Box")
	c.addCPBox.OnDown(func(context *guigui.Context) { m.AddCPShape(lottiecp.ShapeBox) })
	c.delCP.SetText("Del")
	c.delCP.OnDown(func(context *guigui.Context) { m.DeleteCPShape() })

	for _, w := range []guigui.Widget{&c.boxCombo, &c.addRect, &c.addCircle} {
		context.SetEnabled(w, onStage)
	}
	for _, w := range []guigui.Widget{&c.delBox, &c.nameInput, &c.tagsInput, &c.addSpan} {
		context.SetEnabled(w, sel != nil)
	}
	for _, w := range []guigui.Widget{&c.fromInput, &c.toInput, &c.delSpan} {
		context.SetEnabled(w, sp != nil)
	}
	for _, w := range []guigui.Widget{&c.addCPCirc, &c.addCPBox} {
		context.SetEnabled(w, onStage)
	}
	context.SetEnabled(&c.delCP, m.SelectedCPShape() != nil)
	return nil
}

func (c *collisionPanel) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	if m := c.model(context); m != nil {
		w.WriteInt(m.Generation())
	}
}

func (c *collisionPanel) layout(context *guigui.Context) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)

	c.rowAItems = slices.Delete(c.rowAItems, 0, len(c.rowAItems))
	c.rowAItems = append(c.rowAItems,
		guigui.LinearLayoutItem{Widget: &c.showLabel, Size: guigui.FixedSize(5 * u / 2)},
		guigui.LinearLayoutItem{Widget: &c.showCheck, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &c.boxCombo, Size: guigui.FlexibleSize(3)},
		guigui.LinearLayoutItem{Widget: &c.addRect, Size: guigui.FixedSize(2 * u)},
		guigui.LinearLayoutItem{Widget: &c.addCircle, Size: guigui.FixedSize(5 * u / 2)},
		guigui.LinearLayoutItem{Widget: &c.delBox, Size: guigui.FixedSize(3 * u / 2)},
		guigui.LinearLayoutItem{Widget: &c.nameInput, Size: guigui.FlexibleSize(3)},
		guigui.LinearLayoutItem{Widget: &c.tagsInput, Size: guigui.FlexibleSize(4)},
	)
	c.rowA = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal, Items: c.rowAItems, Gap: u / 4,
	}

	c.rowBItems = slices.Delete(c.rowBItems, 0, len(c.rowBItems))
	c.rowBItems = append(c.rowBItems,
		guigui.LinearLayoutItem{Widget: &c.spanLabel, Size: guigui.FixedSize(3 * u / 2)},
		guigui.LinearLayoutItem{Widget: &c.fromInput, Size: guigui.FlexibleSize(2)},
		guigui.LinearLayoutItem{Widget: &c.toInput, Size: guigui.FlexibleSize(2)},
		guigui.LinearLayoutItem{Widget: &c.addSpan, Size: guigui.FixedSize(2 * u)},
		guigui.LinearLayoutItem{Widget: &c.delSpan, Size: guigui.FixedSize(5 * u / 2)},
		guigui.LinearLayoutItem{Widget: &c.bodyLabel, Size: guigui.FixedSize(3 * u)},
		guigui.LinearLayoutItem{Widget: &c.addCPCirc, Size: guigui.FixedSize(5 * u / 2)},
		guigui.LinearLayoutItem{Widget: &c.addCPBox, Size: guigui.FixedSize(2 * u)},
		guigui.LinearLayoutItem{Widget: &c.delCP, Size: guigui.FixedSize(3 * u / 2)},
	)
	c.rowB = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal, Items: c.rowBItems, Gap: u / 4,
	}

	c.items = slices.Delete(c.items, 0, len(c.items))
	c.items = append(c.items,
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &c.rowA},
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &c.rowB},
	)
	return guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical, Items: c.items, Gap: u / 4,
	}
}

func (c *collisionPanel) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	c.layout(context).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (c *collisionPanel) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return c.layout(context).Measure(context, constraints)
}
