package main

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	lottiecp "github.com/shibukawa/lottie-go/plugin/physics/cp"
	lottieresolv "github.com/shibukawa/lottie-go/plugin/physics/resolv"
	lottiesockets "github.com/shibukawa/lottie-go/plugin/sockets"
)

// Collision editing: resolv hitbox tracks ride with the clip on stage, the
// cp body is bundle-wide. Both are drawn over the stage and dragged there;
// this file is the model side of that.
//
// The documents live in the bundle as extension files that the core treats
// as opaque bytes, so this editor imports the two plugin packages and
// caches the parsed values — the overlay reads them every frame, and
// parsing per frame would be silly. Every edit Stores straight back, so
// the bundle bytes are always current and Save needs no extra sync.

// editorCPBodyID is the body this editor manages. The format allows many
// bodies per bundle; the editor keeps one, which covers a character bundle,
// and leaves hand-authored extras untouched.
const editorCPBodyID = "body"

// resetCollisionCache drops the parsed documents; the bundle they came
// from was replaced.
func (m *Model) resetCollisionCache() {
	m.trackCache = map[string]*lottieresolv.Track{}
	m.cpBody, m.cpLoaded = nil, false
	m.socketSet, m.socketsLoaded = nil, false
}

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

// StageTrack is the hitbox track of the animation on stage, or nil when
// the clip has none yet.
func (m *Model) StageTrack() *lottieresolv.Track {
	id := m.StageAnimID()
	if id == "" {
		return nil
	}
	if t, ok := m.trackCache[id]; ok {
		return t
	}
	t, err := lottieresolv.Load(m.bundle, id)
	if err != nil {
		// Absent or unreadable both read as "no track"; the first edit
		// starts a fresh one.
		t = nil
	}
	m.trackCache[id] = t
	return t
}

// ensureStageTrack returns the stage clip's track, creating an empty one on
// first use.
func (m *Model) ensureStageTrack() *lottieresolv.Track {
	id := m.StageAnimID()
	if id == "" {
		return nil
	}
	if t := m.StageTrack(); t != nil {
		return t
	}
	t := &lottieresolv.Track{}
	if err := lottieresolv.Store(m.bundle, id, t); err != nil {
		m.setStatus("cannot create hitbox track: %v", err)
		return nil
	}
	m.trackCache[id] = t
	return t
}

// touchTrack writes the stage track back into the bundle and asks for a
// rebuild. Hitbox edits do not change what the machine plays, so the
// preview-stale counter (docGen) stays put.
func (m *Model) touchTrack() {
	if id := m.StageAnimID(); id != "" {
		if t := m.StageTrack(); t != nil {
			if err := lottieresolv.Store(m.bundle, id, t); err != nil {
				m.setStatus("cannot serialize hitboxes: %v", err)
			}
		}
	}
	m.generation++
}

// CollisionTab is what the strip shows, clamped to what the physics
// config leaves standing. The zero value is the segment overview.
func (m *Model) CollisionTab() colTab {
	switch m.colTab {
	case colHitboxes:
		if m.ResolvEnabled() {
			return colHitboxes
		}
	case colBody:
		if m.CPEnabled() {
			return colBody
		}
	case colSockets:
		return colSockets
	}
	return colSegment
}

func (m *Model) SetCollisionTab(t colTab) {
	m.colTab = t
	m.generation++
}

// The active tab is what shows on the stage: each overlay group appears
// exactly while its tab is the working context, and the Segment tab is
// the clean, undecorated preview. No toggles to remember.

func (m *Model) HitboxesVisible() bool {
	return m.ResolvEnabled() && m.CollisionTab() == colHitboxes
}

func (m *Model) BodyVisible() bool {
	return m.CPEnabled() && m.CollisionTab() == colBody
}

func (m *Model) SocketsVisible() bool {
	return m.CollisionTab() == colSockets
}

// OverlayVisible reports whether any overlay group shows, which is what
// the stage checks before drawing or hit-testing at all.
func (m *Model) OverlayVisible() bool {
	return m.HitboxesVisible() || m.BodyVisible() || m.SocketsVisible()
}

// stageFrameLimit is the last meaningful frame of the stage animation;
// spans beyond it can never be live, so edits clamp against it.
func (m *Model) stageFrameLimit() float64 {
	anim := m.PreviewAnimation()
	if anim == nil {
		return math.MaxFloat64
	}
	total := anim.Duration().Seconds() * anim.FrameRate()
	for _, mk := range anim.Markers() {
		total = max(total, mk.End)
	}
	return total
}

// ---- hitbox selection ----

func (m *Model) SelectedHitboxIndex() int { return m.selBox }

func (m *Model) SelectHitbox(i int) {
	m.selBox = i
	if i >= 0 {
		m.selCPShape = -1
		m.setInspect(inspectHitbox)
	}
	m.generation++
}

func (m *Model) SelectedHitbox() *lottieresolv.Box {
	t := m.StageTrack()
	if t == nil || m.selBox < 0 || m.selBox >= len(t.Boxes) {
		return nil
	}
	return &t.Boxes[m.selBox]
}

// resetCollisionSelection drops every selection; the stage they indexed
// into is gone.
func (m *Model) resetCollisionSelection() {
	m.selBox, m.selCPShape, m.selSocket = -1, -1, -1
}

// ---- hitbox edits ----

// AddHitbox appends a box of the given kind with one span starting at the
// playhead, placed mid-stage so it is visible before the first drag.
func (m *Model) AddHitbox(kind lottieresolv.Kind) {
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
	span := lottieresolv.Span{From: frame, To: min(frame+10, max(m.stageFrameLimit(), frame+1))}
	switch kind {
	case lottieresolv.KindCircle:
		span.X, span.Y = float64(w)/2, float64(h)/2
		span.R = round2(float64(min(w, h)) / 8)
	case lottieresolv.KindWindow:
		// A pure timed flag; there is no geometry to place.
	default:
		span.W, span.H = round2(float64(w)/4), round2(float64(h)/4)
		span.X, span.Y = round2(float64(w)/2-span.W/2), round2(float64(h)/2-span.H/2)
	}
	names := make([]string, 0, len(t.Boxes))
	for _, b := range t.Boxes {
		names = append(names, b.Name)
	}
	t.Boxes = append(t.Boxes, lottieresolv.Box{
		Name: uniqueID("box", names), Kind: kind,
		Spans: []lottieresolv.Span{span},
	})
	m.selBox, m.selCPShape = len(t.Boxes)-1, -1
	m.setInspect(inspectHitbox)
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
	m.setInspect(inspectState)
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
func (m *Model) SelectedSpan() *lottieresolv.Span {
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
	span := lottieresolv.Span{From: frame, To: min(frame+10, max(m.stageFrameLimit(), frame+1))}
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
// inclusive, To exclusive, matching Span.
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

func sortSpans(b *lottieresolv.Box) {
	sort.SliceStable(b.Spans, func(i, j int) bool { return b.Spans[i].From < b.Spans[j].From })
}

// boxSpan resolves a chart position to the span it edits.
func (m *Model) boxSpan(box, span int) (*lottieresolv.Box, *lottieresolv.Span) {
	t := m.StageTrack()
	if t == nil || box < 0 || box >= len(t.Boxes) {
		return nil, nil
	}
	b := &t.Boxes[box]
	if span < 0 || span >= len(b.Spans) {
		return nil, nil
	}
	return b, &b.Spans[span]
}

// ShiftSpan slides one span whole, the chart's bar drag. Frames may go
// fractional during a drag; the chart snaps before calling. Spans are
// deliberately not re-sorted here — the chart addresses the dragged span
// by index across the whole drag — so NormalizeSpans runs on release.
func (m *Model) ShiftSpan(box, span int, delta float64) {
	_, sp := m.boxSpan(box, span)
	if sp == nil || delta == 0 {
		return
	}
	sp.From += delta
	sp.To += delta
	if sp.From < 0 {
		sp.To -= sp.From
		sp.From = 0
	}
	if limit := m.stageFrameLimit(); sp.To > limit {
		sp.From = max(0, sp.From-(sp.To-limit))
		sp.To = limit
	}
	m.touchTrack()
}

// SetSpanEdge moves one end of a span, the chart's bar-edge drag. The
// span keeps at least one frame so it cannot vanish under the cursor.
// Like ShiftSpan, ordering is restored by NormalizeSpans on release.
func (m *Model) SetSpanEdge(box, span int, right bool, frame float64) {
	_, sp := m.boxSpan(box, span)
	if sp == nil {
		return
	}
	if right {
		sp.To = min(max(frame, sp.From+1), max(m.stageFrameLimit(), sp.From+1))
	} else {
		sp.From = max(0, min(frame, sp.To-1))
	}
	m.touchTrack()
}

// NormalizeSpans restores frame order after a chart drag ends.
func (m *Model) NormalizeSpans(box int) {
	t := m.StageTrack()
	if t == nil || box < 0 || box >= len(t.Boxes) {
		return
	}
	sortSpans(&t.Boxes[box])
	m.touchTrack()
}

// round2 keeps dragged values to centipixel precision: cursor deltas
// divided by the stage scale otherwise leave fifteen-digit fractions in
// the saved JSON and the inspector fields.
func round2(v float64) float64 { return math.Round(v*100) / 100 }

// DragHitbox moves the current span by a delta in animation coordinates.
func (m *Model) DragHitbox(dx, dy float64) {
	sp := m.SelectedSpan()
	if sp == nil {
		return
	}
	sp.X = round2(sp.X + dx)
	sp.Y = round2(sp.Y + dy)
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
	if b.Kind == lottieresolv.KindCircle {
		sp.R = round2(max(1, sp.R+dx))
	} else {
		sp.W = round2(max(1, sp.W+dx))
		sp.H = round2(max(1, sp.H+dy))
	}
	m.touchTrack()
}

// ---- cp body ----

// loadCPBody returns the editor-managed body, or nil when the bundle has
// none yet. The parse is cached; edits go through the same pointer.
func (m *Model) loadCPBody() *lottiecp.Body {
	if !m.cpLoaded {
		body, err := lottiecp.Load(m.bundle, editorCPBodyID)
		if err != nil {
			body = nil
		}
		m.cpBody, m.cpLoaded = body, true
	}
	return m.cpBody
}

// CPBodyShapes lists the editor-managed body's shapes; empty when the
// bundle has no body yet.
func (m *Model) CPBodyShapes() []lottiecp.Shape {
	if body := m.loadCPBody(); body != nil {
		return body.Shapes
	}
	return nil
}

func (m *Model) editableCPBody() *lottiecp.Body {
	if body := m.loadCPBody(); body != nil {
		return body
	}
	body := &lottiecp.Body{Type: lottiecp.BodyDynamic}
	if err := lottiecp.Store(m.bundle, editorCPBodyID, body); err != nil {
		m.setStatus("cannot create body: %v", err)
		return nil
	}
	m.cpBody, m.cpLoaded = body, true
	return body
}

func (m *Model) touchCPBody() {
	if body := m.loadCPBody(); body != nil {
		if err := lottiecp.Store(m.bundle, editorCPBodyID, body); err != nil {
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
		m.setInspect(inspectCPShape)
	}
	m.generation++
}

func (m *Model) SelectedCPShape() *lottiecp.Shape {
	body := m.loadCPBody()
	if body == nil || m.selCPShape < 0 || m.selCPShape >= len(body.Shapes) {
		return nil
	}
	return &body.Shapes[m.selCPShape]
}

// AddCPShape places a fixed circle or box mid-stage on the bundle's body.
func (m *Model) AddCPShape(kind lottiecp.ShapeType) {
	body := m.editableCPBody()
	if body == nil {
		return
	}
	anim := m.PreviewAnimation()
	w, h := 100, 100
	if anim != nil {
		w, h = anim.Size()
	}
	s := lottiecp.Shape{
		Type:     kind,
		Center:   lottiecp.Point{X: float64(w) / 2, Y: float64(h) / 2},
		Friction: 0.7,
	}
	if kind == lottiecp.ShapeCircle {
		s.Radius = round2(float64(min(w, h)) / 6)
	} else {
		s.Width, s.Height = round2(float64(w)/4), round2(float64(h)/3)
	}
	body.Shapes = append(body.Shapes, s)
	m.selCPShape, m.selBox = len(body.Shapes)-1, -1
	m.setInspect(inspectCPShape)
	m.setStatus("added %s to body", kind)
	m.touchCPBody()
}

func (m *Model) DeleteCPShape() {
	body := m.loadCPBody()
	if body == nil || m.selCPShape < 0 || m.selCPShape >= len(body.Shapes) {
		return
	}
	body.Shapes = slices.Delete(body.Shapes, m.selCPShape, m.selCPShape+1)
	m.selCPShape = -1
	m.setInspect(inspectState)
	m.setStatus("deleted body shape")
	m.touchCPBody()
}

func (m *Model) DragCPShape(dx, dy float64) {
	s := m.SelectedCPShape()
	if s == nil {
		return
	}
	if s.Type == lottiecp.ShapePolygon {
		for i := range s.Vertices {
			s.Vertices[i].X = round2(s.Vertices[i].X + dx)
			s.Vertices[i].Y = round2(s.Vertices[i].Y + dy)
		}
	} else {
		s.Center.X = round2(s.Center.X + dx)
		s.Center.Y = round2(s.Center.Y + dy)
	}
	m.touchCPBody()
}

func (m *Model) DragCPShapeHandle(dx, dy float64) {
	s := m.SelectedCPShape()
	if s == nil {
		return
	}
	switch s.Type {
	case lottiecp.ShapeCircle:
		s.Radius = round2(max(1, s.Radius+dx))
	case lottiecp.ShapeBox:
		s.Width = round2(max(1, s.Width+dx*2))
		s.Height = round2(max(1, s.Height+dy*2))
	}
	m.touchCPBody()
}

// DragSocket nudges the selected socket by a delta in animation
// coordinates, stored as the socket's layer-local offset so the trim
// rides the layer's rotation and scale. The bound layer remains the
// position's source of truth.
func (m *Model) DragSocket(dx, dy float64) {
	s := m.SelectedSocket()
	anim := m.PreviewAnimation()
	if s == nil || anim == nil {
		return
	}
	pl, ok := anim.LayerPlacement(s.LayerName(), m.stageFrame())
	if !ok || pl.ScaleX == 0 || pl.ScaleY == 0 {
		return
	}
	// Inverse of the placement's rotate-and-scale, so the on-screen drag
	// lands exactly under the cursor.
	sin, cos := math.Sincos(pl.Angle)
	s.DX = round2(s.DX + (cos*dx+sin*dy)/pl.ScaleX)
	s.DY = round2(s.DY + (-sin*dx+cos*dy)/pl.ScaleY)
	m.touchSockets()
}

// HitboxLabel names one box for the picker; the index keeps duplicate
// names selectable, and a window is marked because it never shows on the
// stage.
func HitboxLabel(i int, b lottieresolv.Box) string {
	if b.Kind == lottieresolv.KindWindow {
		return fmt.Sprintf("%d: %s (win)", i+1, b.Name)
	}
	return fmt.Sprintf("%d: %s", i+1, b.Name)
}

// ---- sockets ----

// loadSockets returns the bundle's socket table, or nil when it has none
// yet. The parse is cached; edits go through the same pointer.
func (m *Model) loadSockets() *lottiesockets.Set {
	if !m.socketsLoaded {
		set, err := lottiesockets.Load(m.bundle)
		if err != nil {
			set = nil
		}
		m.socketSet, m.socketsLoaded = set, true
	}
	return m.socketSet
}

// Sockets lists the bundle's socket table; empty when there is none yet.
func (m *Model) Sockets() []lottiesockets.Socket {
	if set := m.loadSockets(); set != nil {
		return set.Sockets
	}
	return nil
}

func (m *Model) editableSockets() *lottiesockets.Set {
	if set := m.loadSockets(); set != nil {
		return set
	}
	set := &lottiesockets.Set{}
	if err := lottiesockets.Store(m.bundle, set); err != nil {
		m.setStatus("cannot create socket table: %v", err)
		return nil
	}
	m.socketSet, m.socketsLoaded = set, true
	return set
}

func (m *Model) touchSockets() {
	if set := m.loadSockets(); set != nil {
		if err := lottiesockets.Store(m.bundle, set); err != nil {
			m.setStatus("cannot serialize sockets: %v", err)
		}
	}
	m.generation++
}

func (m *Model) SelectedSocketIndex() int { return m.selSocket }

func (m *Model) SelectSocket(i int) {
	m.selSocket = i
	if i >= 0 {
		m.setInspect(inspectSocket)
	}
	m.generation++
}

// AddSocket binds a socket to the named layer, using the layer name as the
// socket name — the common case; hand-edit the table for a differing pair.
func (m *Model) AddSocket(layer string) {
	layer = strings.TrimSpace(layer)
	if layer == "" {
		return
	}
	set := m.editableSockets()
	if set == nil {
		return
	}
	if _, exists := set.Find(layer); exists {
		m.setStatus("a socket named %q already exists", layer)
		m.generation++
		return
	}
	set.Sockets = append(set.Sockets, lottiesockets.Socket{Name: layer})
	m.selSocket = len(set.Sockets) - 1
	m.setInspect(inspectSocket)
	m.setStatus("added socket %q", layer)
	m.touchSockets()
}

func (m *Model) DeleteSocket() {
	set := m.loadSockets()
	if set == nil || m.selSocket < 0 || m.selSocket >= len(set.Sockets) {
		return
	}
	name := set.Sockets[m.selSocket].Name
	set.Sockets = slices.Delete(set.Sockets, m.selSocket, m.selSocket+1)
	m.selSocket = -1
	m.setInspect(inspectState)
	m.setStatus("deleted socket %q", name)
	m.touchSockets()
}

// SelectedSocket returns the socket the inspector edits.
func (m *Model) SelectedSocket() *lottiesockets.Socket {
	set := m.loadSockets()
	if set == nil || m.selSocket < 0 || m.selSocket >= len(set.Sockets) {
		return nil
	}
	return &set.Sockets[m.selSocket]
}

// RenameSocket gives the selected socket a game-facing name of its own,
// pinning the layer binding first so the rename does not rebind it.
func (m *Model) RenameSocket(name string) {
	s := m.SelectedSocket()
	name = strings.TrimSpace(name)
	if s == nil || name == "" || s.Name == name {
		return
	}
	s.Layer = s.LayerName()
	s.Name = name
	m.touchSockets()
}

// ToggleSocketZ flips the selected socket between drawing an attached item
// in front of and behind the character.
func (m *Model) ToggleSocketZ() {
	set := m.loadSockets()
	if set == nil || m.selSocket < 0 || m.selSocket >= len(set.Sockets) {
		return
	}
	s := &set.Sockets[m.selSocket]
	if s.Z == lottiesockets.ZBehind {
		s.Z = lottiesockets.ZFront
	} else {
		s.Z = lottiesockets.ZBehind
	}
	m.touchSockets()
}

// StageLayerNames lists the layers of the animation on stage, for binding
// sockets.
func (m *Model) StageLayerNames() []string {
	anim := m.PreviewAnimation()
	if anim == nil {
		return nil
	}
	return anim.LayerNames()
}
