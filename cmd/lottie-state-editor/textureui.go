package main

import (
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	lottietexture "github.com/shibukawa/lottie-go/plugin/texture"
)

// The texture section of the shape inspector, the bbox gizmo on the stage
// and the UV pane. A paint is a property of a fill or stroke, UV of a path's
// vertices, so all of it lives inside the Shapes tab (ui:texture-mapping).

// texTransformFields are the placement transform's numeric rows.
var texTransformFields = []shapeField{
	{"tex offset u", "p", 0, 2}, {"tex offset v", "p", 1, 2},
	{"tex scale u %", "s", 0, 2}, {"tex scale v %", "s", 1, 2},
	{"tex rotation °", "r", 0, 1},
	{"tex anchor u", "a", 0, 2}, {"tex anchor v", "a", 1, 2},
}

// texImportChoice is the picker entry that opens the file dialog.
const texImportChoice = "\x00import"

// buildTextureRows is the texture section of an fl or st: the image
// picker, the mapping, wrap and filter pickers, the tint switch and the
// placement rows. Collapsed to the picker alone until a texture is bound.
func (p *shapeInspector) buildTextureRows(context *guigui.Context, m *Model, adder *guigui.ChildAdder, editable bool) {
	adder.AddWidget(&p.texSel)
	label(&p.texLabel, "texture")
	p.texItems = p.texItems[:0]
	p.texItems = append(p.texItems, basicwidget.SelectItem[string]{Text: "(none)", Value: ""})
	cur := m.ShapeTextureName()
	for _, c := range m.TextureChoices() {
		text := c.File
		if c.ID != "" && c.ID != c.File {
			text = c.File + " · " + c.ID
		}
		// Bundle images no asset names yet are picked by file; binding one
		// adds the asset.
		value := c.ID
		if value == "" {
			value = "file:" + c.File
		}
		p.texItems = append(p.texItems, basicwidget.SelectItem[string]{Text: text, Value: value})
	}
	p.texItems = append(p.texItems, basicwidget.SelectItem[string]{Text: "Import…", Value: texImportChoice})
	p.texSel.SetItems(p.texItems)
	p.texSel.SelectItemByValue(cur)
	p.texSel.OnItemSelected(func(context *guigui.Context, index int) {
		it, ok := p.texSel.ItemByIndex(index)
		if !ok || it.Value == m.ShapeTextureName() {
			return
		}
		switch {
		case it.Value == texImportChoice:
			m.BrowseTextureImage()
		case strings.HasPrefix(it.Value, "file:"):
			m.BindTextureFile(strings.TrimPrefix(it.Value, "file:"))
		default:
			m.SetShapeTexture(it.Value)
		}
	})
	context.SetEnabled(&p.texSel, editable)
	p.formItems = append(p.formItems,
		basicwidget.FormItem{PrimaryWidget: &p.texLabel, SecondaryWidget: &p.texSel})
	if _, bound := m.ShapeTexture(); !bound {
		return
	}

	adder.AddWidget(&p.texMapSel)
	adder.AddWidget(&p.texWrapSel)
	adder.AddWidget(&p.texFilterSel)
	adder.AddWidget(&p.texTintChk)
	label(&p.texMapLabel, "mapping")
	label(&p.texWrapLabel, "wrap")
	label(&p.texFilterLabel, "filter")
	label(&p.texTintLabel, "tint by color")

	n, _ := m.SelectedShapeNode()
	p.texMapItems = p.texMapItems[:0]
	p.texMapItems = append(p.texMapItems,
		basicwidget.SelectItem[string]{Text: "bounding box", Value: ""},
		basicwidget.SelectItem[string]{Text: "per vertex", Value: string(lottietexture.MappingVertex)},
	)
	if n.ty == "st" {
		p.texMapItems = append(p.texMapItems,
			basicwidget.SelectItem[string]{Text: "along stroke", Value: string(lottietexture.MappingStroke)})
	}
	wireTexPicker(context, &p.texMapSel, p.texMapItems, m.ShapeTextureString("mapping"), editable,
		func(v string) { m.SetShapeTextureString("mapping", v) })

	p.texWrapItems = p.texWrapItems[:0]
	p.texWrapItems = append(p.texWrapItems,
		basicwidget.SelectItem[string]{Text: "clamp", Value: ""},
		basicwidget.SelectItem[string]{Text: "repeat", Value: string(lottietexture.WrapRepeat)},
		basicwidget.SelectItem[string]{Text: "mirror", Value: string(lottietexture.WrapMirror)},
	)
	wireTexPicker(context, &p.texWrapSel, p.texWrapItems, m.ShapeTextureString("wrap"), editable,
		func(v string) { m.SetShapeTextureString("wrap", v) })

	p.texFilterItems = p.texFilterItems[:0]
	p.texFilterItems = append(p.texFilterItems,
		basicwidget.SelectItem[string]{Text: "linear", Value: ""},
		basicwidget.SelectItem[string]{Text: "nearest", Value: string(lottietexture.FilterNearest)},
	)
	wireTexPicker(context, &p.texFilterSel, p.texFilterItems, m.ShapeTextureString("filter"), editable,
		func(v string) { m.SetShapeTextureString("filter", v) })

	p.texTintChk.SetValue(m.ShapeTextureTint())
	p.texTintChk.OnValueChanged(func(context *guigui.Context, value bool) {
		if value != m.ShapeTextureTint() {
			m.SetShapeTextureTint(value)
		}
	})
	context.SetEnabled(&p.texTintChk, editable)

	p.formItems = append(p.formItems,
		basicwidget.FormItem{PrimaryWidget: &p.texMapLabel, SecondaryWidget: &p.texMapSel},
		basicwidget.FormItem{PrimaryWidget: &p.texWrapLabel, SecondaryWidget: &p.texWrapSel},
		basicwidget.FormItem{PrimaryWidget: &p.texFilterLabel, SecondaryWidget: &p.texFilterSel},
		basicwidget.FormItem{PrimaryWidget: &p.texTintLabel, SecondaryWidget: &p.texTintChk},
	)

	// The placement rows edit the transform like every other keyed member:
	// at the parked key, promoting a static one on its first keying.
	p.texLabels.SetLen(len(texTransformFields))
	p.texRows.SetLen(len(texTransformFields))
	for i, f := range texTransformFields {
		lb, row := p.texLabels.At(i), p.texRows.At(i)
		adder.AddWidget(lb)
		adder.AddWidget(row)
		label(lb, f.label)
		in := &row.text
		v, ok := m.ShapeTexTransformValue(f.member)
		writable := editable && ok && len(v) > f.comp && m.ShapeTexTransformWritable(f.member)
		cur := 0.0
		if ok && len(v) > f.comp {
			cur = v[f.comp]
			in.SetValue(strconv.FormatFloat(cur, 'g', -1, 64))
		} else {
			in.SetValue("")
		}
		in.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
			if !committed {
				return
			}
			if x, ok := parseFinite(text); ok {
				m.SetShapeTexTransformComponent(f.member, f.comp, x)
			} else {
				m.RejectNumber(text)
			}
		})
		context.SetEnabled(in, writable)
		p.tabFields = append(p.tabFields, in)
		wireAdjacentCopy(context, row, writable, cur,
			func(dir int) (float64, float64, bool) {
				adj, at, ok := m.ShapeTexTransformAdjacent(f.member, dir)
				if !ok || len(adj) <= f.comp {
					return 0, 0, false
				}
				return adj[f.comp], at, true
			},
			func(dir int) {
				if adj, _, ok := m.ShapeTexTransformAdjacent(f.member, dir); ok && len(adj) > f.comp {
					m.SetShapeTexTransformComponent(f.member, f.comp, adj[f.comp])
				}
			})
		p.formItems = append(p.formItems, basicwidget.FormItem{PrimaryWidget: lb, SecondaryWidget: row})
	}
}

// wireTexPicker fills one enumerated picker and routes its choice.
func wireTexPicker(context *guigui.Context, sel *basicwidget.Select[string], items []basicwidget.SelectItem[string], cur string, editable bool, set func(string)) {
	sel.SetItems(items)
	sel.SelectItemByValue(cur)
	sel.OnItemSelected(func(context *guigui.Context, index int) {
		if it, ok := sel.ItemByIndex(index); ok && it.Value != cur {
			set(it.Value)
		}
	})
	context.SetEnabled(sel, editable)
}

// buildUVRows adds the UV pane and its buttons for a path that takes a UV
// set; p.showUV tells the layout to make room.
func (p *shapeInspector) buildUVRows(context *guigui.Context, m *Model, adder *guigui.ChildAdder, editable bool) {
	if !m.ShapeUVEditable() {
		return
	}
	p.showUV = true
	adder.AddWidget(&p.uvPane)
	adder.AddWidget(&p.uvSeedBtn)
	adder.AddWidget(&p.uvClearBtn)
	_, has := m.ShapeUVs()
	context.SetEnabled(&p.uvPane, editable && has && m.ShapePathWritable())
	p.uvSeedBtn.SetText("Seed UV")
	if has {
		p.uvSeedBtn.SetText("Reset UV")
	}
	p.uvSeedBtn.OnDown(func(context *guigui.Context) { m.SeedShapeUV() })
	context.SetEnabled(&p.uvSeedBtn, editable)
	p.uvClearBtn.SetText("Clear UV")
	p.uvClearBtn.OnDown(func(context *guigui.Context) { m.ClearShapeUV() })
	context.SetEnabled(&p.uvClearBtn, editable && has)
}

// buildUnplacedRows lists the document entries whose item is gone, with
// the one button that lets them go.
func (p *shapeInspector) buildUnplacedRows(context *guigui.Context, m *Model, adder *guigui.ChildAdder, editable bool) {
	unplaced := m.UnplacedTextures()
	if len(unplaced) == 0 {
		return
	}
	adder.AddWidget(&p.unplacedText)
	adder.AddWidget(&p.unplacedDrop)
	label(&p.unplacedLabel, "unplaced textures")
	p.unplacedText.SetValue(strings.Join(unplaced, "\n"))
	p.unplacedText.SetMultiline(true)
	p.unplacedText.SetScale(0.85)
	p.unplacedDrop.SetText("Drop unplaced")
	p.unplacedDrop.OnDown(func(context *guigui.Context) { m.DropUnplacedTextures() })
	context.SetEnabled(&p.unplacedDrop, editable)
	p.formItems = append(p.formItems,
		basicwidget.FormItem{PrimaryWidget: &p.unplacedLabel, SecondaryWidget: &p.unplacedText},
		basicwidget.FormItem{SecondaryWidget: &p.unplacedDrop},
	)
}

// ---- the UV pane ----

// uvPaneView shows the texture a vertex-mapped paint uses with the selected
// path's UV polygon over it, points square like the stage's vertices. A
// point drags; empty space drags the whole set; the wheel scales the set
// about its centroid. Selection mirrors the stage's vertex both ways.
type uvPaneView struct {
	guigui.DefaultWidget

	// dragPlus2 is the point being dragged plus two: 0 none, 1 the whole
	// set, i+2 point i.
	dragPlus2  int
	lastCursor image.Point
}

func (v *uvPaneView) model(context *guigui.Context) *Model {
	val, ok := context.Env(v, envKeyModel)
	if !ok {
		return nil
	}
	m, _ := val.(*Model)
	return m
}

// imageRect fits the texture (or a unit square) into the widget with its
// aspect kept, leaving the frame the UV 0..1 square maps onto.
func (v *uvPaneView) imageRect(b image.Rectangle, img *ebiten.Image) image.Rectangle {
	w, h := 1.0, 1.0
	if img != nil {
		ib := img.Bounds()
		w, h = float64(ib.Dx()), float64(ib.Dy())
	}
	scale := math.Min(float64(b.Dx()-8)/w, float64(b.Dy()-8)/h)
	pw, ph := int(w*scale), int(h*scale)
	x := b.Min.X + (b.Dx()-pw)/2
	y := b.Min.Y + (b.Dy()-ph)/2
	return image.Rect(x, y, x+pw, y+ph)
}

func (v *uvPaneView) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w := 8 * u
	if cw, ok := constraints.FixedWidth(); ok {
		w = cw
	}
	return image.Pt(w, 8*u)
}

func (v *uvPaneView) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	m := v.model(context)
	if m == nil {
		return
	}
	b := widgetBounds.Bounds()
	vector.DrawFilledRect(dst, float32(b.Min.X), float32(b.Min.Y), float32(b.Dx()), float32(b.Dy()),
		color.NRGBA{0x30, 0x30, 0x36, 0xff}, false)
	img := m.UVPaneImage()
	fr := v.imageRect(b, img)
	if img != nil {
		clip := dst.SubImage(fr).(*ebiten.Image)
		var op ebiten.DrawImageOptions
		ib := img.Bounds()
		op.GeoM.Scale(float64(fr.Dx())/float64(ib.Dx()), float64(fr.Dy())/float64(ib.Dy()))
		op.GeoM.Translate(float64(fr.Min.X), float64(fr.Min.Y))
		op.Filter = ebiten.FilterLinear
		clip.DrawImage(img, &op)
	}
	vector.StrokeRect(dst, float32(fr.Min.X), float32(fr.Min.Y), float32(fr.Dx()), float32(fr.Dy()),
		1, color.NRGBA{0x80, 0x80, 0x88, 0xff}, false)
	uv, ok := m.ShapeUVs()
	if !ok {
		return
	}
	u := float32(basicwidget.UnitSize(context))
	live := context.IsEnabled(v)
	clr := shapeIdleColor
	stroke := max(1, u/24)
	if live {
		clr = shapeColor
		stroke = max(2, u/16)
	}
	pts := make([][2]float32, len(uv))
	for i, p := range uv {
		pts[i] = [2]float32{
			float32(fr.Min.X) + float32(p[0])*float32(fr.Dx()),
			float32(fr.Min.Y) + float32(p[1])*float32(fr.Dy()),
		}
	}
	closed := m.ShapePathClosed()
	for i := range pts {
		if i == len(pts)-1 && !closed {
			break
		}
		a, c := pts[i], pts[(i+1)%len(pts)]
		vector.StrokeLine(dst, a[0], a[1], c[0], c[1], stroke, clr, true)
	}
	r := handleSize(u) / 2
	sel := m.SelectedUVVert()
	for i, p := range pts {
		fill := shapeVertFill
		if i == sel {
			fill = clr
		}
		vector.DrawFilledRect(dst, p[0]-r, p[1]-r, r*2, r*2, fill, true)
		vector.StrokeRect(dst, p[0]-r, p[1]-r, r*2, r*2, stroke, clr, true)
	}
}

func (v *uvPaneView) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	m := v.model(context)
	if m == nil || !context.IsEnabled(v) {
		return guigui.HandleInputResult{}
	}
	b := widgetBounds.Bounds()
	fr := v.imageRect(b, m.UVPaneImage())
	if fr.Dx() <= 0 || fr.Dy() <= 0 {
		return guigui.HandleInputResult{}
	}
	cx, cy := ebiten.CursorPosition()
	if v.dragPlus2 > 0 {
		if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			v.dragPlus2 = 0
			m.EndPoseEdit()
			return guigui.HandleInputByWidget(v)
		}
		d := image.Pt(cx, cy).Sub(v.lastCursor)
		if d.X == 0 && d.Y == 0 {
			return guigui.HandleInputByWidget(v)
		}
		v.lastCursor = image.Pt(cx, cy)
		du, dv := float64(d.X)/float64(fr.Dx()), float64(d.Y)/float64(fr.Dy())
		m.MoveShapeUV(v.dragPlus2-2, round4(du), round4(dv))
		return guigui.HandleInputByWidget(v)
	}
	if !widgetBounds.IsHitAtCursor() {
		return guigui.HandleInputResult{}
	}
	if _, wy := adjustedWheel(); wy != 0 {
		m.ScaleShapeUV(math.Pow(1.05, wy))
		return guigui.HandleInputByWidget(v)
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return guigui.HandleInputResult{}
	}
	uv, ok := m.ShapeUVs()
	if !ok {
		return guigui.HandleInputResult{}
	}
	u := float32(basicwidget.UnitSize(context))
	half := handleSize(u)/2 + 2
	for i, p := range uv {
		px := float32(fr.Min.X) + float32(p[0])*float32(fr.Dx())
		py := float32(fr.Min.Y) + float32(p[1])*float32(fr.Dy())
		if abs32(float32(cx)-px) <= half && abs32(float32(cy)-py) <= half {
			m.SelectUVVert(i)
			v.dragPlus2 = i + 2
			v.lastCursor = image.Pt(cx, cy)
			m.BeginPoseEdit()
			return guigui.HandleInputByWidget(v)
		}
	}
	v.dragPlus2 = 1
	v.lastCursor = image.Pt(cx, cy)
	m.BeginPoseEdit()
	return guigui.HandleInputByWidget(v)
}

func (v *uvPaneView) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	if m := v.model(context); m != nil {
		w.WriteInt(m.Generation())
	}
	w.WriteInt(v.dragPlus2)
}

// ---- the stage gizmo ----

type texGizmoPts struct {
	origin, axis [2]float32
}

// texGizmoScreen maps the bbox gizmo into screen space.
func texGizmoScreen(m *Model, tr stageTransform) (texGizmoPts, bool) {
	n, ok := m.SelectedShapeNode()
	if !ok || !m.ShapeTexGizmoActive() {
		return texGizmoPts{}, false
	}
	o, a, ok := m.ShapeTexGizmoPoints()
	if !ok {
		return texGizmoPts{}, false
	}
	g, ok := m.shapeSpaceMatrix(n.layer, n.path, m.stageFrame())
	if !ok {
		return texGizmoPts{}, false
	}
	conv := func(p [2]float64) [2]float32 {
		ax, ay := g.Apply(p[0], p[1])
		sx, sy := tr.toScreen(ax, ay)
		return [2]float32{sx, sy}
	}
	return texGizmoPts{origin: conv(o), axis: conv(a)}, true
}

// drawTexGizmo paints the square at the texture's origin and the circle on
// its u axis, joined by a line — the gradient gizmo's vocabulary.
func drawTexGizmo(dst *ebiten.Image, m *Model, tr stageTransform, u float32, live bool) {
	pts, ok := texGizmoScreen(m, tr)
	if !ok {
		return
	}
	clr := shapeIdleColor
	if live {
		clr = shapeColor
	}
	stroke := max(1, u/16)
	vector.StrokeLine(dst, pts.origin[0], pts.origin[1], pts.axis[0], pts.axis[1], stroke, clr, true)
	r := handleSize(u) / 2
	vector.DrawFilledRect(dst, pts.origin[0]-r, pts.origin[1]-r, r*2, r*2, shapeVertFill, true)
	vector.StrokeRect(dst, pts.origin[0]-r, pts.origin[1]-r, r*2, r*2, stroke, clr, true)
	vector.DrawFilledCircle(dst, pts.axis[0], pts.axis[1], r, shapeVertFill, true)
	vector.StrokeCircle(dst, pts.axis[0], pts.axis[1], r, stroke, clr, true)
}

// texGripAt reports which gizmo handle sits under the cursor.
func texGripAt(m *Model, tr stageTransform, u float32, cx, cy int) (stageDragKind, bool) {
	pts, ok := texGizmoScreen(m, tr)
	if !ok {
		return 0, false
	}
	half := handleSize(u)/2 + 2
	hit := func(p [2]float32) bool {
		return abs32(float32(cx)-p[0]) <= half && abs32(float32(cy)-p[1]) <= half
	}
	switch {
	case hit(pts.axis):
		return dragShapeTexAxis, true
	case hit(pts.origin):
		return dragShapeTexOrigin, true
	}
	return 0, false
}
