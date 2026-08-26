package main

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	lottie "github.com/shibukawa/lottie-go"
)

// Collision editing: resolv hitbox tracks ride with the clip on stage, the
// cp body is bundle-wide. Both are drawn over the stage and dragged there;
// this file is the model side of that.

// editorCPBodyID is the body this editor manages. The format allows many
// bodies per bundle; the editor keeps one, which covers a character bundle,
// and leaves hand-authored extras untouched.
const editorCPBodyID = "body"

// StageAnimID is the id of the animation on stage: the previewed clip, or
// whatever the running machine's state plays.
func (m *Model) StageAnimID() string {
	if m.previewClip.Anim != "" {
		return m.previewClip.Anim
	}
	if m.preview != nil {
		return m.animForState(m.preview.State())
	}
	return ""
}

// StageTrack is the hitbox track of the animation on stage, or nil when the
// clip has none yet.
func (m *Model) StageTrack() *lottie.ResolvTrack {
	id := m.StageAnimID()
	if id == "" {
		return nil
	}
	t, err := m.bundle.ResolvTrack(id)
	if err != nil {
		return nil
	}
	return t
}

// ensureStageTrack returns the stage clip's track, creating an empty one on
// first use.
func (m *Model) ensureStageTrack() *lottie.ResolvTrack {
	id := m.StageAnimID()
	if id == "" {
		return nil
	}
	if t := m.StageTrack(); t != nil {
		return t
	}
	t := &lottie.ResolvTrack{}
	if err := m.bundle.SetResolvTrack(id, t); err != nil {
		m.setStatus("cannot create hitbox track: %v", err)
		return nil
	}
	return t
}

// touchTrack writes the stage track back into the bundle and asks for a
// rebuild. Hitbox edits do not change what the machine plays, so the
// preview-stale counter (docGen) stays put.
func (m *Model) touchTrack() {
	if id := m.StageAnimID(); id != "" {
		if t := m.StageTrack(); t != nil {
			if err := m.bundle.SetResolvTrack(id, t); err != nil {
				m.setStatus("cannot serialize hitboxes: %v", err)
			}
		}
	}
	m.generation++
}

func (m *Model) ShowCollision() bool { return !m.hideCollision }

func (m *Model) SetShowCollision(v bool) {
	m.hideCollision = !v
	m.generation++
}

// ---- hitbox selection ----

func (m *Model) SelectedHitboxIndex() int { return m.selBox }

func (m *Model) SelectHitbox(i int) {
	m.selBox = i
	if i >= 0 {
		m.selCPShape = -1
	}
	m.generation++
}

func (m *Model) SelectedHitbox() *lottie.ResolvBox {
	t := m.StageTrack()
	if t == nil || m.selBox < 0 || m.selBox >= len(t.Boxes) {
		return nil
	}
	return &t.Boxes[m.selBox]
}

// resetCollisionSelection drops both selections; the stage they indexed
// into is gone.
func (m *Model) resetCollisionSelection() {
	m.selBox, m.selCPShape = -1, -1
}

// ---- hitbox edits ----

// AddHitbox appends a box of the given kind with one span starting at the
// playhead, placed mid-stage so it is visible before the first drag.
func (m *Model) AddHitbox(kind lottie.ResolvBoxKind) {
	t := m.ensureStageTrack()
	if t == nil {
		m.setStatus("no clip on stage to attach a hitbox to")
		m.generation++
		return
	}
	anim := m.PreviewAnimation()
	w, h := 100, 100
	if anim != nil {
		w, h = anim.Size()
	}
	frame := 0.0
	if p := m.PreviewPlayer(); p != nil {
		frame = p.Frame()
	}
	span := lottie.ResolvSpan{From: frame, To: frame + 10}
	switch kind {
	case lottie.ResolvCircle:
		span.X, span.Y = float64(w)/2, float64(h)/2
		span.R = float64(min(w, h)) / 8
	default:
		span.W, span.H = float64(w)/4, float64(h)/4
		span.X, span.Y = float64(w)/2-span.W/2, float64(h)/2-span.H/2
	}
	names := make([]string, 0, len(t.Boxes))
	for _, b := range t.Boxes {
		names = append(names, b.Name)
	}
	t.Boxes = append(t.Boxes, lottie.ResolvBox{
		Name: uniqueID("box", names), Kind: kind,
		Spans: []lottie.ResolvSpan{span},
	})
	m.selBox, m.selCPShape = len(t.Boxes)-1, -1
	m.setStatus("added hitbox %q", t.Boxes[m.selBox].Name)
	m.touchTrack()
}

func (m *Model) DeleteHitbox() {
	t := m.StageTrack()
	if t == nil || m.selBox < 0 || m.selBox >= len(t.Boxes) {
		return
	}
	name := t.Boxes[m.selBox].Name
	t.Boxes = slices.Delete(t.Boxes, m.selBox, m.selBox+1)
	m.selBox = -1
	m.setStatus("deleted hitbox %q", name)
	m.touchTrack()
}

func (m *Model) RenameHitbox(name string) {
	b := m.SelectedHitbox()
	name = strings.TrimSpace(name)
	if b == nil || name == "" || b.Name == name {
		return
	}
	b.Name = name
	m.touchTrack()
}

// HitboxTagsCSV is the selected box's tags as an editable line.
func (m *Model) HitboxTagsCSV() string {
	b := m.SelectedHitbox()
	if b == nil {
		return ""
	}
	return strings.Join(b.Tags, ", ")
}

// SetHitboxTagsCSV parses "hit, hurt" into the box's tags. Tags are the
// game-facing meaning of a box, so they are free-form here.
func (m *Model) SetHitboxTagsCSV(csv string) {
	b := m.SelectedHitbox()
	if b == nil {
		return
	}
	var tags []string
	for _, t := range strings.Split(csv, ",") {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}
	if slices.Equal(b.Tags, tags) {
		return
	}
	b.Tags = tags
	m.touchTrack()
}

// ---- spans ----

// stageFrame is the playhead, which span edits key on.
func (m *Model) stageFrame() float64 {
	if p := m.PreviewPlayer(); p != nil {
		return p.Frame()
	}
	return 0
}

// SelectedSpan is the selected box's span under the playhead. Geometry
// drags land here: what you grab is what you see at this frame.
func (m *Model) SelectedSpan() *lottie.ResolvSpan {
	b := m.SelectedHitbox()
	if b == nil {
		return nil
	}
	i, ok := b.SpanAt(m.stageFrame())
	if !ok {
		return nil
	}
	return &b.Spans[i]
}

// AddHitboxSpan starts a new span at the playhead, copying the geometry of
// the nearest earlier span so the box steps rather than jumps somewhere
// arbitrary.
func (m *Model) AddHitboxSpan() {
	b := m.SelectedHitbox()
	if b == nil {
		return
	}
	frame := m.stageFrame()
	if _, ok := b.SpanAt(frame); ok {
		m.setStatus("a span already covers frame %.0f", frame)
		m.generation++
		return
	}
	span := lottie.ResolvSpan{From: frame, To: frame + 10}
	src := -1
	for i := range b.Spans {
		if b.Spans[i].From <= frame && (src < 0 || b.Spans[i].From > b.Spans[src].From) {
			src = i
		}
	}
	if src < 0 && len(b.Spans) > 0 {
		src = 0
	}
	if src >= 0 {
		g := b.Spans[src]
		span.X, span.Y, span.W, span.H, span.R = g.X, g.Y, g.W, g.H, g.R
	} else {
		span.W, span.H = 50, 50
	}
	// A fresh span must not run into the next one.
	for i := range b.Spans {
		if b.Spans[i].From > frame {
			span.To = min(span.To, b.Spans[i].From)
		}
	}
	b.Spans = append(b.Spans, span)
	sortSpans(b)
	m.touchTrack()
}

func (m *Model) DeleteHitboxSpan() {
	b := m.SelectedHitbox()
	if b == nil {
		return
	}
	i, ok := b.SpanAt(m.stageFrame())
	if !ok {
		return
	}
	b.Spans = slices.Delete(b.Spans, i, i+1)
	m.touchTrack()
}

// SetSpanRange rewrites the current span's frame interval. From is
// inclusive, To exclusive, matching ResolvSpan.
func (m *Model) SetSpanRange(from, to float64) {
	sp := m.SelectedSpan()
	if sp == nil || to <= from {
		return
	}
	sp.From, sp.To = from, to
	if b := m.SelectedHitbox(); b != nil {
		sortSpans(b)
	}
	m.touchTrack()
}

func sortSpans(b *lottie.ResolvBox) {
	sort.SliceStable(b.Spans, func(i, j int) bool { return b.Spans[i].From < b.Spans[j].From })
}

// DragHitbox moves the current span by a delta in animation coordinates.
func (m *Model) DragHitbox(dx, dy float64) {
	sp := m.SelectedSpan()
	if sp == nil {
		return
	}
	sp.X += dx
	sp.Y += dy
	m.touchTrack()
}

// DragHitboxHandle resizes the current span: a rect grows by the delta, a
// circle's radius follows the x component.
func (m *Model) DragHitboxHandle(dx, dy float64) {
	b := m.SelectedHitbox()
	sp := m.SelectedSpan()
	if b == nil || sp == nil {
		return
	}
	if b.Kind == lottie.ResolvCircle {
		sp.R = max(1, sp.R+dx)
	} else {
		sp.W = max(1, sp.W+dx)
		sp.H = max(1, sp.H+dy)
	}
	m.touchTrack()
}

// ---- cp body ----

// CPBodyShapes lists the editor-managed body's shapes; empty when the
// bundle has no body yet.
func (m *Model) CPBodyShapes() []lottie.CPShape {
	body, err := m.bundle.CPBody(editorCPBodyID)
	if err != nil {
		return nil
	}
	return body.Shapes
}

func (m *Model) editableCPBody() *lottie.CPBody {
	body, err := m.bundle.CPBody(editorCPBodyID)
	if err == nil {
		return body
	}
	body = &lottie.CPBody{Type: lottie.CPBodyDynamic}
	if err := m.bundle.SetCPBody(editorCPBodyID, body); err != nil {
		m.setStatus("cannot create body: %v", err)
		return nil
	}
	return body
}

func (m *Model) touchCPBody() {
	if body, err := m.bundle.CPBody(editorCPBodyID); err == nil {
		if err := m.bundle.SetCPBody(editorCPBodyID, body); err != nil {
			m.setStatus("cannot serialize body: %v", err)
		}
	}
	m.generation++
}

func (m *Model) SelectedCPShapeIndex() int { return m.selCPShape }

func (m *Model) SelectCPShape(i int) {
	m.selCPShape = i
	if i >= 0 {
		m.selBox = -1
	}
	m.generation++
}

func (m *Model) SelectedCPShape() *lottie.CPShape {
	body, err := m.bundle.CPBody(editorCPBodyID)
	if err != nil || m.selCPShape < 0 || m.selCPShape >= len(body.Shapes) {
		return nil
	}
	return &body.Shapes[m.selCPShape]
}

// AddCPShape places a fixed circle or box mid-stage on the bundle's body.
func (m *Model) AddCPShape(kind lottie.CPShapeType) {
	body := m.editableCPBody()
	if body == nil {
		return
	}
	anim := m.PreviewAnimation()
	w, h := 100, 100
	if anim != nil {
		w, h = anim.Size()
	}
	s := lottie.CPShape{
		Type:     kind,
		Center:   lottie.PhysPoint{X: float64(w) / 2, Y: float64(h) / 2},
		Friction: 0.7,
	}
	if kind == lottie.CPShapeCircle {
		s.Radius = float64(min(w, h)) / 6
	} else {
		s.Width, s.Height = float64(w)/4, float64(h)/3
	}
	body.Shapes = append(body.Shapes, s)
	m.selCPShape, m.selBox = len(body.Shapes)-1, -1
	m.setStatus("added %s to body", kind)
	m.touchCPBody()
}

func (m *Model) DeleteCPShape() {
	body, err := m.bundle.CPBody(editorCPBodyID)
	if err != nil || m.selCPShape < 0 || m.selCPShape >= len(body.Shapes) {
		return
	}
	body.Shapes = slices.Delete(body.Shapes, m.selCPShape, m.selCPShape+1)
	m.selCPShape = -1
	m.setStatus("deleted body shape")
	m.touchCPBody()
}

func (m *Model) DragCPShape(dx, dy float64) {
	s := m.SelectedCPShape()
	if s == nil {
		return
	}
	if s.Type == lottie.CPShapePolygon {
		for i := range s.Vertices {
			s.Vertices[i].X += dx
			s.Vertices[i].Y += dy
		}
	} else {
		s.Center.X += dx
		s.Center.Y += dy
	}
	m.touchCPBody()
}

func (m *Model) DragCPShapeHandle(dx, dy float64) {
	s := m.SelectedCPShape()
	if s == nil {
		return
	}
	switch s.Type {
	case lottie.CPShapeCircle:
		s.Radius = max(1, s.Radius+dx)
	case lottie.CPShapeBox:
		s.Width = max(1, s.Width+dx*2)
		s.Height = max(1, s.Height+dy*2)
	}
	m.touchCPBody()
}

// HitboxLabel names one box for the picker; the index keeps duplicate
// names selectable.
func HitboxLabel(i int, b lottie.ResolvBox) string {
	return fmt.Sprintf("%d: %s", i+1, b.Name)
}
