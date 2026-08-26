package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	lottie "github.com/shibukawa/lottie-go"
	lottiecp "github.com/shibukawa/lottie-go/plugin/physics/cp"
	lottieresolv "github.com/shibukawa/lottie-go/plugin/physics/resolv"
	lottiesockets "github.com/shibukawa/lottie-go/plugin/sockets"
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

// socketColor marks attachment sockets; they are points, not judgement
// volumes, so they sit outside the tag palette.
var socketColor = color.NRGBA{0x14, 0xb0, 0xc8, 0xff}

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

// drawCollisionOverlay paints the cp body first, the live hitboxes over
// it (frame-stepped judgement is what this stage is for, so it wins
// ties), and the attachment sockets on top of everything: they are the
// smallest marks and must not drown.
func drawCollisionOverlay(dst *ebiten.Image, m *Model, tr stageTransform, u float32) {
	stroke := max(1, u/12)
	if m.BodyVisible() {
		for i, s := range m.CPBodyShapes() {
			drawCPShape(dst, tr, s, stroke, i == m.SelectedCPShapeIndex(), u)
		}
	}
	frame := m.stageFrame()
	if track := m.StageTrack(); track != nil && m.HitboxesVisible() {
		for _, ab := range track.At(frame) {
			drawActiveBox(dst, tr, ab, stroke, ab.Index == m.SelectedHitboxIndex(), u)
		}
	}
	if anim := m.PreviewAnimation(); anim != nil && m.SocketsVisible() {
		set := m.loadSockets()
		if set != nil {
			// Resolved by name so the layer-local offset applies; names
			// are unique (AddSocket refuses duplicates).
			for i := range set.Sockets {
				pl, ok := set.At(anim, frame, set.Sockets[i].Name)
				if !ok {
					continue
				}
				drawSocket(dst, tr, pl.LayerPlacement, stroke, i == m.SelectedSocketIndex(), u)
			}
		}
	}
}

// drawSocket marks an attachment point: a cross at the position and a tick
// along its x-axis so the bound layer's rotation is visible.
func drawSocket(dst *ebiten.Image, tr stageTransform, pl lottie.LayerPlacement, stroke float32, selected bool, u float32) {
	if selected {
		stroke *= 2
	}
	clr := socketColor
	if !pl.Visible {
		clr = withAlpha(clr, 0x60)
	}
	x, y := tr.toScreen(pl.X, pl.Y)
	s := max(4, u/3)
	vector.StrokeLine(dst, x-s, y, x+s, y, stroke, clr, true)
	vector.StrokeLine(dst, x, y-s, x, y+s, stroke, clr, true)
	tick := float64(s) * 1.6
	vector.StrokeLine(dst, x, y,
		x+float32(tick*math.Cos(pl.Angle)), y+float32(tick*math.Sin(pl.Angle)),
		stroke, clr, true)
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
	if b := m.SelectedHitbox(); b != nil && b.Kind != lottieresolv.KindWindow {
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
	if track == nil || !m.HitboxesVisible() {
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
	if !m.BodyVisible() {
		return 0, false
	}
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

// hitTestSockets returns the socket whose cross sits within radius of an
// animation-space point.
func hitTestSockets(m *Model, ax, ay, radius float64) (int, bool) {
	if !m.SocketsVisible() {
		return 0, false
	}
	anim := m.PreviewAnimation()
	set := m.loadSockets()
	if anim == nil || set == nil {
		return 0, false
	}
	frame := m.stageFrame()
	for i := len(set.Sockets) - 1; i >= 0; i-- {
		pl, ok := set.At(anim, frame, set.Sockets[i].Name)
		if !ok {
			continue
		}
		dx, dy := ax-pl.X, ay-pl.Y
		if dx*dx+dy*dy <= radius*radius {
			return i, true
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

// colTab picks what the strip under the stage shows: the whole-clip
// segment overview, or one annotation group. The physics config decides
// which annotation tabs exist at all.
type colTab int

const (
	colSegment colTab = iota
	colHitboxes
	colBody
	colSockets
)

// collisionPanel is the tabbed strip under the timeline: the hitbox tab
// carries the span chart and add buttons, the body and socket tabs carry
// their lists. Parameters of the selected item are edited in the
// inspector; selection here is by chart row or list row.
type collisionPanel struct {
	guigui.DefaultWidget

	tabs     basicwidget.SegmentedControl[colTab]
	timeline timelineView
	chart    chartView

	addRect   basicwidget.Button
	addCircle basicwidget.Button
	addWin    basicwidget.Button
	delBox    basicwidget.Button

	bodyList  basicwidget.List[int]
	addCPCirc basicwidget.Button
	addCPBox  basicwidget.Button
	delCP     basicwidget.Button

	sockList   basicwidget.List[int]
	layerCombo basicwidget.Select[string]
	addSock    basicwidget.Button
	delSock    basicwidget.Button

	tabItems  []basicwidget.SegmentedControlItem[colTab]
	bodyItems []basicwidget.ListItem[int]
	sockItems []basicwidget.ListItem[int]

	btnRow      guigui.LinearLayout
	btnRowItems []guigui.LinearLayoutItem
	items       []guigui.LinearLayoutItem
}

func (c *collisionPanel) model(context *guigui.Context) *Model {
	v, ok := context.Env(c, envKeyModel)
	if !ok {
		return nil
	}
	m, _ := v.(*Model)
	return m
}

// availableTabs is the tab set the physics config leaves standing. The
// segment overview and sockets are always there.
func availableTabs(m *Model) []colTab {
	out := []colTab{colSegment}
	if m.ResolvEnabled() {
		out = append(out, colHitboxes)
	}
	if m.CPEnabled() {
		out = append(out, colBody)
	}
	return append(out, colSockets)
}

func (c *collisionPanel) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m := c.model(context)
	if m == nil {
		return nil
	}
	avail := availableTabs(m)
	tab := m.CollisionTab()

	adder.AddWidget(&c.tabs)
	c.tabItems = slices.Delete(c.tabItems, 0, len(c.tabItems))
	for _, t := range avail {
		// The engine names say where each group's data goes: hitboxes
		// feed resolv detection, the body feeds a cp space.
		text := map[colTab]string{
			colSegment:  "Segment",
			colHitboxes: "Hitbox (resolv)",
			colBody:     "Body (cp)",
			colSockets:  "Sockets",
		}[t]
		c.tabItems = append(c.tabItems, basicwidget.SegmentedControlItem[colTab]{Text: text, Value: t})
	}
	c.tabs.SetItems(c.tabItems)
	c.tabs.SelectItemByValue(tab)
	c.tabs.OnItemSelected(func(context *guigui.Context, index int) {
		// The model holds the tab: switching to Body is also what shows
		// the silhouette on the stage.
		if item, ok := c.tabs.ItemByIndex(index); ok && item.Value != m.CollisionTab() {
			m.SetCollisionTab(item.Value)
		}
	})

	onStage := m.StageAnimID() != ""
	switch tab {
	case colSegment:
		// The whole-clip overview: where the played range (a state's
		// segment) sits inside the full timeline, markers included.
		adder.AddWidget(&c.timeline)
	case colHitboxes:
		adder.AddWidget(&c.chart)
		for _, w := range []guigui.Widget{&c.addRect, &c.addCircle, &c.addWin, &c.delBox} {
			adder.AddWidget(w)
		}
		c.addRect.SetText("+Rect")
		c.addRect.OnDown(func(context *guigui.Context) { m.AddHitbox(lottieresolv.KindRect) })
		c.addCircle.SetText("+Circle")
		c.addCircle.OnDown(func(context *guigui.Context) { m.AddHitbox(lottieresolv.KindCircle) })
		// A window is a geometry-less timed flag (cancelable, invincible);
		// it shares the tag and span editing but never draws on the stage.
		c.addWin.SetText("+Window")
		c.addWin.OnDown(func(context *guigui.Context) { m.AddHitbox(lottieresolv.KindWindow) })
		c.delBox.SetText("Delete")
		c.delBox.OnDown(func(context *guigui.Context) { m.DeleteHitbox() })
		for _, w := range []guigui.Widget{&c.addRect, &c.addCircle, &c.addWin} {
			context.SetEnabled(w, onStage)
		}
		context.SetEnabled(&c.delBox, m.SelectedHitbox() != nil)
	case colBody:
		adder.AddWidget(&c.bodyList)
		for _, w := range []guigui.Widget{&c.addCPCirc, &c.addCPBox, &c.delCP} {
			adder.AddWidget(w)
		}
		c.buildBodyList(context, m)
		c.addCPCirc.SetText("+Circle")
		c.addCPCirc.OnDown(func(context *guigui.Context) { m.AddCPShape(lottiecp.ShapeCircle) })
		c.addCPBox.SetText("+Box")
		c.addCPBox.OnDown(func(context *guigui.Context) { m.AddCPShape(lottiecp.ShapeBox) })
		c.delCP.SetText("Delete")
		c.delCP.OnDown(func(context *guigui.Context) { m.DeleteCPShape() })
		for _, w := range []guigui.Widget{&c.addCPCirc, &c.addCPBox} {
			context.SetEnabled(w, onStage)
		}
		context.SetEnabled(&c.delCP, m.SelectedCPShape() != nil)
	case colSockets:
		adder.AddWidget(&c.sockList)
		adder.AddWidget(&c.layerCombo)
		adder.AddWidget(&c.addSock)
		adder.AddWidget(&c.delSock)
		c.buildSocketList(context, m)
		setOptions(&c.layerCombo, m.StageLayerNames()...)
		c.addSock.SetText("+Socket")
		c.addSock.OnDown(func(context *guigui.Context) { m.AddSocket(selectedValue(&c.layerCombo)) })
		c.delSock.SetText("Delete")
		c.delSock.OnDown(func(context *guigui.Context) { m.DeleteSocket() })
		context.SetEnabled(&c.layerCombo, onStage)
		context.SetEnabled(&c.addSock, onStage && selectedValue(&c.layerCombo) != "")
		context.SetEnabled(&c.delSock, m.SelectedSocket() != nil)
	}
	return nil
}

// buildBodyList lists the body's shapes; they have no names, so the rows
// describe them.
func (c *collisionPanel) buildBodyList(context *guigui.Context, m *Model) {
	shapes := m.CPBodyShapes()
	c.bodyItems = slices.Delete(c.bodyItems, 0, len(c.bodyItems))
	for i, s := range shapes {
		var desc string
		switch s.Type {
		case lottiecp.ShapeCircle:
			desc = fmt.Sprintf("circle r=%.0f at (%.0f, %.0f)", s.Radius, s.Center.X, s.Center.Y)
		case lottiecp.ShapeBox:
			desc = fmt.Sprintf("box %.0f×%.0f at (%.0f, %.0f)", s.Width, s.Height, s.Center.X, s.Center.Y)
		default:
			desc = fmt.Sprintf("%s (%d points)", s.Type, len(s.Vertices))
		}
		c.bodyItems = append(c.bodyItems, basicwidget.ListItem[int]{
			Text: fmt.Sprintf("%d: %s", i+1, desc), Value: i,
		})
	}
	c.bodyList.SetItems(c.bodyItems)
	if i := m.SelectedCPShapeIndex(); i >= 0 && i < len(shapes) {
		c.bodyList.SelectItemByValue(i)
	}
	c.bodyList.OnItemSelected(func(context *guigui.Context, index int) {
		if index >= 0 && index < len(shapes) && index != m.SelectedCPShapeIndex() {
			m.SelectCPShape(index)
		}
	})
}

// buildSocketList lists the socket table with each binding spelled out.
func (c *collisionPanel) buildSocketList(context *guigui.Context, m *Model) {
	socks := m.Sockets()
	c.sockItems = slices.Delete(c.sockItems, 0, len(c.sockItems))
	for i, s := range socks {
		z := "front"
		if s.Z == lottiesockets.ZBehind {
			z = "behind"
		}
		c.sockItems = append(c.sockItems, basicwidget.ListItem[int]{
			Text: s.Name, KeyText: fmt.Sprintf("layer %s · %s", s.LayerName(), z), Value: i,
		})
	}
	c.sockList.SetItems(c.sockItems)
	if i := m.SelectedSocketIndex(); i >= 0 && i < len(socks) {
		c.sockList.SelectItemByValue(i)
	}
	c.sockList.OnItemSelected(func(context *guigui.Context, index int) {
		if index >= 0 && index < len(socks) && index != m.SelectedSocketIndex() {
			m.SelectSocket(index)
		}
	})
}

func (c *collisionPanel) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	if m := c.model(context); m != nil {
		w.WriteInt(m.Generation())
		w.WriteString(m.StageAnimID())
	}
}

func (c *collisionPanel) layout(context *guigui.Context) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)

	tab := colSegment
	if m := c.model(context); m != nil {
		tab = m.CollisionTab()
	}
	c.btnRowItems = slices.Delete(c.btnRowItems, 0, len(c.btnRowItems))
	switch tab {
	case colHitboxes:
		c.btnRowItems = append(c.btnRowItems,
			guigui.LinearLayoutItem{Widget: &c.addRect, Size: guigui.FixedSize(2 * u)},
			guigui.LinearLayoutItem{Widget: &c.addCircle, Size: guigui.FixedSize(5 * u / 2)},
			guigui.LinearLayoutItem{Widget: &c.addWin, Size: guigui.FixedSize(3 * u)},
			guigui.LinearLayoutItem{Widget: &c.delBox, Size: guigui.FixedSize(5 * u / 2)},
			guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)},
		)
	case colBody:
		c.btnRowItems = append(c.btnRowItems,
			guigui.LinearLayoutItem{Widget: &c.addCPCirc, Size: guigui.FixedSize(5 * u / 2)},
			guigui.LinearLayoutItem{Widget: &c.addCPBox, Size: guigui.FixedSize(2 * u)},
			guigui.LinearLayoutItem{Widget: &c.delCP, Size: guigui.FixedSize(5 * u / 2)},
			guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)},
		)
	case colSockets:
		c.btnRowItems = append(c.btnRowItems,
			guigui.LinearLayoutItem{Widget: &c.layerCombo, Size: guigui.FlexibleSize(1)},
			guigui.LinearLayoutItem{Widget: &c.addSock, Size: guigui.FixedSize(3 * u)},
			guigui.LinearLayoutItem{Widget: &c.delSock, Size: guigui.FixedSize(5 * u / 2)},
		)
	}
	c.btnRow = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal, Items: c.btnRowItems, Gap: u / 4,
	}

	c.items = slices.Delete(c.items, 0, len(c.items))
	c.items = append(c.items,
		guigui.LinearLayoutItem{Widget: &c.tabs, Size: guigui.FixedSize(u)},
	)
	switch tab {
	case colSegment:
		c.items = append(c.items, guigui.LinearLayoutItem{Widget: &c.timeline})
	case colHitboxes:
		c.items = append(c.items, guigui.LinearLayoutItem{Widget: &c.chart})
	case colBody:
		c.items = append(c.items, guigui.LinearLayoutItem{Widget: &c.bodyList, Size: guigui.FixedSize(3 * u)})
	case colSockets:
		c.items = append(c.items, guigui.LinearLayoutItem{Widget: &c.sockList, Size: guigui.FixedSize(3 * u)})
	}
	// The segment overview is display plus scrubbing; it has no buttons.
	if tab != colSegment {
		c.items = append(c.items,
			guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &c.btnRow},
		)
	}
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
