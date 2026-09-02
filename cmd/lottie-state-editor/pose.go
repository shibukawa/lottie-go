package main

import (
	"bytes"
	lottietexture "github.com/shibukawa/lottie-go/plugin/texture"
	"math"
	"slices"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/shibukawa/lottie-go"
)

// Pose editing is the model side of the Poses tab: where a clip's keyframes
// are, which one is selected, and what a drag on the stage writes into it.
//
// Everything here edits the clip on stage through clipdoc.go and stores back
// with Bundle.SetAnimation, so the preview is redrawn by the real renderer
// rather than by an editor-side approximation of it. Edits land on a
// selected key and nowhere else: preset clips keep every animated property
// on one shared set of times, and auto-keying at an arbitrary frame would
// quietly break that.

// StageClipDoc is the editable document of the clip on stage, parsed once
// and kept until the bundle underneath it is replaced.
func (m *Model) StageClipDoc() *clipDoc {
	id := m.StageAnimID()
	if id == "" {
		return nil
	}
	if d, ok := m.clipDocs[id]; ok {
		return d
	}
	data, ok := m.bundle.AnimationJSON(id)
	if !ok {
		return nil
	}
	d, err := newClipDoc(id, data)
	if err != nil {
		// An unparseable clip is one the pose tab simply cannot offer; the
		// rest of the editor still works on it.
		d = nil
	}
	m.weaveTextures(d)
	if m.clipDocs == nil {
		m.clipDocs = map[string]*clipDoc{}
	}
	m.clipDocs[id] = d
	return d
}

// resetClipDocCache drops the parsed clips; the bundle they came from was
// replaced.
func (m *Model) resetClipDocCache() {
	m.clipDocs = map[string]*clipDoc{}
	m.clearPoseSelection()
	m.clearShapeSelection()
}

func (m *Model) clearPoseSelection() {
	m.poseSet = false
	m.poseFrame = 0
	m.poseLayer = -1
	m.posePart = -1
}

// PosesVisible reports whether the pose overlay and its hit testing are
// live, which — like every other overlay group — is exactly while its tab is
// the working context.
func (m *Model) PosesVisible() bool {
	return m.CollisionTab() == colPoses
}

// PoseTimes are the tick positions of the pose row: the times every animated
// property of the clip agrees on. It is empty when they do not agree, and
// the timeline shows a row per layer instead.
func (m *Model) PoseTimes() []float64 {
	d := m.StageClipDoc()
	if d == nil || !d.posed {
		return nil
	}
	return d.times
}

// PoseRows are the layers of the fallback timeline, one row each. It is
// empty while the clip is a clean pose sequence, because then there is a
// single row and it is the pose row.
func (m *Model) PoseRows() []int {
	d := m.StageClipDoc()
	if d == nil || d.posed {
		return nil
	}
	return d.animatedLayers()
}

// PoseRowTimes are the ticks of one fallback row.
func (m *Model) PoseRowTimes(layer int) []float64 {
	d := m.StageClipDoc()
	if d == nil {
		return nil
	}
	l := d.layer(layer)
	if l == nil {
		return nil
	}
	return l.layerTimes()
}

// PoseLayerName names a row.
func (m *Model) PoseLayerName(layer int) string {
	d := m.StageClipDoc()
	if d == nil {
		return ""
	}
	if l := d.layer(layer); l != nil {
		return l.name
	}
	return ""
}

// PoseKeyIsHold reports whether a tick is a hold: a value that switches
// rather than travels. The timeline draws those square, because a limb
// trading sides is a swap and there is nothing in it to nudge.
func (m *Model) PoseKeyIsHold(frame float64, layer int) bool {
	d := m.StageClipDoc()
	if d == nil {
		return false
	}
	if layer < 0 {
		return d.poseIsHold(frame)
	}
	l := d.layer(layer)
	if l == nil {
		return false
	}
	for p := range l.keyed {
		if slices.Contains(l.keyed[p], frame) && !d.isHold(layer, p, frame) {
			return false
		}
	}
	return true
}

// SelectPoseKey parks the playhead on a key and makes it the edit target.
// layer is -1 for a whole-pose tick and a layer index on the fallback rows,
// where selecting a tick also selects the layer it belongs to.
func (m *Model) SelectPoseKey(frame float64, layer int) {
	m.takeClipOnStage()
	m.parkPreview(frame)
	m.poseFrame, m.poseSet, m.poseLayer = frame, true, layer
	if layer >= 0 {
		m.posePart = layer
	}
	// On the Shapes tab a key is parked to edit shape values, so the pane
	// that must stay up is the shape one — and the panel must keep offering
	// a layer, even if switching the stage clip dropped the old selection.
	if m.CollisionTab() == colShapes {
		if m.SelectedShapeLayer() < 0 {
			if ls := m.ShapeLayers(); len(ls) > 0 {
				m.selShapeLayer = ls[0]
			}
		}
		m.setInspect(inspectShape)
	} else {
		m.setInspect(inspectPose)
	}
	m.generation++
}

// takeClipOnStage makes the edited clip the thing the stage is playing.
//
// A running state machine owns a player per state and keeps the Animation it
// decoded when it started, so an edit reaches the bundle but never the
// picture: the drag looks like it did nothing. Restarting the machine on
// every mouse move is worse than useless, so posing switches the stage to
// the clip itself, which is the right context anyway — a parked playhead and
// a running machine are not compatible. "Back to machine" returns.
func (m *Model) takeClipOnStage() {
	if m.clipPlayer != nil {
		return
	}
	id := m.StageAnimID()
	if id == "" {
		return
	}
	m.ShowClip(clipRef{Anim: id})
}

// parkPreview holds the playhead on a frame. Looping is switched off first:
// a clip's last key sits at its out point, and a looping player wraps that
// straight back to the start, which would make the final pose of every clip
// the one pose that cannot be selected or edited.
func (m *Model) parkPreview(frame float64) {
	p := m.PreviewPlayer()
	if p == nil {
		return
	}
	p.Pause()
	p.SetLoop(false)
	p.SetFrame(frame)
	m.generation++
}

// poseParked reports whether a key is selected, which is what keeps the
// stage player from looping past the frame being edited.
func (m *Model) poseParked() bool { return m.poseSet }

// SelectedPoseKey is the key being edited, if any.
func (m *Model) SelectedPoseKey() (float64, bool) {
	if !m.poseSet {
		return 0, false
	}
	// Scrubbing away from a key leaves edit mode rather than writing an
	// interpolated frame into the document.
	if p := m.PreviewPlayer(); p == nil || p.Frame() != m.poseFrame {
		return 0, false
	}
	// The key has to still be there. An undo can take one away under a
	// selection that was made before it.
	if !slices.Contains(m.poseTimesFor(m.poseLayer), m.poseFrame) {
		return 0, false
	}
	return m.poseFrame, true
}

// poseTimesFor is the tick set of one row, whichever kind it is.
func (m *Model) poseTimesFor(layer int) []float64 {
	if layer < 0 {
		return m.PoseTimes()
	}
	return m.PoseRowTimes(layer)
}

// SelectedPoseRow is the row the selected tick sits on: -1 for the pose row.
func (m *Model) SelectedPoseRow() int { return m.poseLayer }

// SelectedPosePart is the layer whose parameters the inspector edits.
func (m *Model) SelectedPosePart() int { return m.posePart }

// SelectPosePart picks the layer a stage click landed on.
func (m *Model) SelectPosePart(i int) {
	m.posePart = i
	if i >= 0 {
		m.setInspect(inspectPose)
	}
	m.generation++
}

// PoseParts are the layers the pose editor can select, in the order the
// document lists them, which is front to back. Every image layer is here
// whether or not it can be reached on the stage: a rig hides parts behind
// each other and swaps others out by opacity, and those are exactly the ones
// a click cannot find.
func (m *Model) PoseParts() []int {
	d := m.StageClipDoc()
	if d == nil {
		return nil
	}
	var out []int
	for i := range d.layers {
		if d.layers[i].ty == 2 {
			out = append(out, i)
		}
	}
	return out
}

// PosePartHidden reports whether a part is invisible at the current frame,
// which for these rigs means its opacity has been switched off — a head
// drawn from another angle waiting for the character to turn. Listing it
// without saying so would look like the list was lying.
func (m *Model) PosePartHidden(layer int) bool {
	d := m.StageClipDoc()
	if d == nil {
		return false
	}
	o, ok := d.valueNear(layer, "o", m.stageFrame())
	return ok && len(o) > 0 && o[0] <= 0
}

// PosePartIndex finds a part by layer name. Names index the clip document,
// which is not the same list as Animation.LayerNames: that one dedups and
// descends into precomps, while a pose edit addresses a row of this
// document.
func (m *Model) PosePartIndex(name string) (int, bool) {
	d := m.StageClipDoc()
	if d == nil {
		return 0, false
	}
	for i := range d.layers {
		if d.layers[i].name == name {
			return i, true
		}
	}
	return 0, false
}

// SelectedPosePartName names the selected part for the inspector heading.
func (m *Model) SelectedPosePartName() string {
	return m.PoseLayerName(m.posePart)
}

// PoseValue reads one transform member of the selected part at the selected
// key. Static members read too: they are just as editable, they simply apply
// to the whole clip until an edit keys them.
func (m *Model) PoseValue(prop string) ([]float64, bool) {
	d := m.StageClipDoc()
	frame, ok := m.SelectedPoseKey()
	if d == nil || !ok || m.posePart < 0 {
		return nil, false
	}
	return d.value(m.posePart, prop, frame)
}

// PoseValueIsKeyed reports whether the shown value is stored at this key or
// is the clip-wide static one, which is what tells the inspector to offer
// keying it.
func (m *Model) PoseValueIsKeyed(prop string) bool {
	d := m.StageClipDoc()
	frame, ok := m.SelectedPoseKey()
	if d == nil || !ok || m.posePart < 0 {
		return false
	}
	_, keyed := d.valueAt(m.posePart, prop, frame)
	return keyed
}

// PoseAdjacentValue reads the value a transform member holds at its own key
// on one side of the selected one — dir < 0 is the key before, the pose the
// part is arriving from; dir > 0 the key after, the pose it is heading to —
// along with the frame that key sits at, which is what the copy buttons'
// tooltips name. Reading the property's key times rather than the pose
// columns keeps it exact when a member is keyed more sparsely than the poses
// are. A static member has no keys and reports nothing, which is right: it
// already equals its neighbours.
func (m *Model) PoseAdjacentValue(prop string, dir int) ([]float64, float64, bool) {
	d := m.StageClipDoc()
	frame, ok := m.SelectedPoseKey()
	if d == nil || !ok || m.posePart < 0 {
		return nil, 0, false
	}
	adj, found := 0.0, false
	for _, t := range d.keyTimesOf(m.posePart, prop) {
		if dir < 0 {
			if t < frame && (!found || t > adj) {
				adj, found = t, true
			}
		} else {
			if t > frame && (!found || t < adj) {
				adj, found = t, true
			}
		}
	}
	if !found {
		return nil, 0, false
	}
	v, ok := d.valueAt(m.posePart, prop, adj)
	return v, adj, ok
}

// CopyPoseValueFromAdjacent sets one component of a transform member to what
// it is at the member's neighbouring key, before (dir < 0) or after (dir > 0).
// A pose mostly is its neighbour with a few numbers nudged, so "same as the
// key next door" is the edit the pane's per-field copy buttons make in one
// click instead of a retyped number.
func (m *Model) CopyPoseValueFromAdjacent(prop string, comp, dir int) {
	adj, _, okAdj := m.PoseAdjacentValue(prop, dir)
	cur, okCur := m.PoseValue(prop)
	if !okAdj || !okCur || comp >= len(adj) || comp >= len(cur) {
		return
	}
	if adj[comp] == cur[comp] {
		return
	}
	next := slices.Clone(cur)
	next[comp] = adj[comp]
	m.SetPoseValue(prop, next)
}

// SetPoseValue writes a transform member at the selected key, keying a
// static member first when the clip has a pose set to key it against.
func (m *Model) SetPoseValue(prop string, v []float64) {
	if m.blockEdit() {
		return
	}
	d := m.StageClipDoc()
	frame, ok := m.SelectedPoseKey()
	if d == nil || !ok || m.posePart < 0 {
		return
	}
	if !finite(v...) {
		m.rejectValue("pose " + prop)
		return
	}
	pushed := m.snapshotClip()
	if !d.setValue(m.posePart, prop, frame, v) {
		if pushed {
			m.dropLastSnapshot()
		}
		return
	}
	m.touchClipDoc()
}

// RetimePoseKey drags a key along the frame axis. On a pose clip the whole
// column moves together, which is what keeps a tick meaning a pose.
func (m *Model) RetimePoseKey(from, to float64, layer int) {
	if m.blockEdit() {
		return
	}
	d := m.StageClipDoc()
	if d == nil {
		return
	}
	pushed := m.snapshotClip()
	landed, moved := d.retime(from, to, layer)
	if !moved {
		if pushed {
			m.dropLastSnapshot()
		}
		return
	}
	if m.poseSet && m.poseFrame == from {
		m.poseFrame = landed
		m.parkPreview(landed)
	}
	m.touchClipDoc()
}

// touchClipDoc writes the edited clip back into the bundle and rebuilds what
// is on stage from it, so the frame under the playhead is drawn by the real
// renderer. SetAnimation decodes before it accepts, so an edit that would
// produce an unreadable clip is rejected here rather than at save time.
func (m *Model) touchClipDoc() {
	d := m.StageClipDoc()
	if d == nil {
		return
	}
	// The clip is stored pure, its texture document beside it (texture.go).
	if err := m.storeClipDoc(d); err != nil {
		// The document in memory still holds the refused edit; the status
		// has to repaint, or the refusal is invisible until something else
		// rebuilds. Undo restores the stored bytes and drops the document.
		m.setStatus("clip edit rejected: %v", err)
		m.generation++
		return
	}
	m.rebuildStageClip(d.id)
	m.docGen++
	m.generation++
}

// rebuildStageClip re-creates the stage player over the freshly decoded
// clip, holding the frame and staying paused. A running machine preview is
// not rebuilt: it owns players for several clips and restarting it would
// throw away the state the edit is being judged in, so it reads as stale
// until it is restarted, which is what every other document edit does.
func (m *Model) rebuildStageClip(id string) {
	if m.clipPlayer == nil || m.previewClip.Anim != id {
		return
	}
	anim, err := m.bundle.Animation(id)
	if err != nil {
		m.setStatus("cannot reload clip %q: %v", id, err)
		return
	}
	frame := m.clipPlayer.Frame()
	c := m.previewClip
	p := anim.NewPlayer()
	m.applyTextures(id, p)
	// Looping stays off while a key is parked, or restoring the frame would
	// wrap an out-point key back to the start (parkPreview).
	p.SetLoop(!m.poseParked())
	if c.Segment != "" && !p.SetMarkerRange(c.Segment) {
		m.setStatus("clip %q has no marker %q", c.Anim, c.Segment)
		return
	}
	p.OnMarker(func(mk lottie.Marker) { m.noteMarker(c.Anim, mk) })
	p.SetFrame(frame)
	p.Pause()
	m.clipPlayer = p
}

// ---- stage geometry ----

// poseQuad is a part's local box mapped into animation coordinates, plus the
// point it rotates about. The corners come from the exact layer matrix, not
// from LayerPlacement: a mirrored part decomposes into a half turn and its
// outline would be drawn reflected.
type poseQuad struct {
	pts   [4][2]float64 // local (0,0) (w,0) (w,h) (0,h)
	pivot [2]float64
}

// PosePartQuad is where a part sits on stage at the current frame.
func (m *Model) PosePartQuad(layer int) (poseQuad, bool) {
	d := m.StageClipDoc()
	anim := m.PreviewAnimation()
	if d == nil || anim == nil {
		return poseQuad{}, false
	}
	l := d.layer(layer)
	if l == nil || l.name == "" {
		return poseQuad{}, false
	}
	w, h, ok := d.layerSize(layer)
	if !ok {
		return poseQuad{}, false
	}
	g, ok := anim.LayerTransform(l.name, m.stageFrame())
	if !ok {
		return poseQuad{}, false
	}
	var q poseQuad
	corners := [4][2]float64{{0, 0}, {w, 0}, {w, h}, {0, h}}
	for i, c := range corners {
		x, y := g.Apply(c[0], c[1])
		q.pts[i] = [2]float64{x, y}
	}
	a, ok := d.value(layer, "a", m.stageFrame())
	if !ok || len(a) < 2 {
		a = []float64{0, 0}
	}
	px, py := g.Apply(a[0], a[1])
	q.pivot = [2]float64{px, py}
	return q, true
}

// PosePartAt returns the part under an animation-space point: the topmost
// one, which is the first in layer order because a Lottie document lists its
// layers front to back.
func (m *Model) PosePartAt(ax, ay float64) (int, bool) {
	d := m.StageClipDoc()
	if d == nil {
		return 0, false
	}
	for i := range d.layers {
		if d.layers[i].ty != 2 {
			continue
		}
		// A part switched off by opacity is not on the stage, so a click
		// must fall through it to whatever is actually drawn underneath.
		// The rig stacks alternate drawings of a slot over each other —
		// body-side and body-back sit in front of body — and without this
		// the torso could never be picked at all. The parts list is how a
		// hidden part is reached.
		if m.PosePartHidden(i) {
			continue
		}
		q, ok := m.PosePartQuad(i)
		if !ok {
			continue
		}
		if pointInQuad(q.pts, ax, ay) {
			return i, true
		}
	}
	return 0, false
}

// pointInQuad tests a convex quad by keeping every cross product on one
// side. The quad stays convex under any affine transform, mirrored or not.
func pointInQuad(pts [4][2]float64, x, y float64) bool {
	var pos, neg bool
	for i := range pts {
		a, b := pts[i], pts[(i+1)%4]
		cr := (b[0]-a[0])*(y-a[1]) - (b[1]-a[1])*(x-a[0])
		if cr > 0 {
			pos = true
		} else if cr < 0 {
			neg = true
		}
	}
	return !(pos && neg)
}

// parentGeoM is the matrix the selected part's own transform is written
// against: its parent layer's, or the identity when it rides on the
// animation itself.
func (m *Model) parentGeoM(layer int) ebiten.GeoM {
	var g ebiten.GeoM
	d := m.StageClipDoc()
	anim := m.PreviewAnimation()
	if d == nil || anim == nil {
		return g
	}
	name, ok := d.parentName(layer)
	if !ok {
		return g
	}
	if pg, ok := anim.LayerTransform(name, m.stageFrame()); ok {
		return pg
	}
	return g
}

// RotatePosePart turns the selected part to follow the cursor. The angle is
// measured about the part's own pivot in animation space and then handed
// back into the parent's space, because that is where the rotation member is
// written: under a mirrored ancestor, turning the cursor clockwise must
// write a counter-clockwise number.
func (m *Model) RotatePosePart(fromX, fromY, toX, toY float64) {
	if m.blockEdit() {
		return
	}
	frame, ok := m.SelectedPoseKey()
	if !ok || m.posePart < 0 {
		return
	}
	q, ok := m.PosePartQuad(m.posePart)
	if !ok {
		return
	}
	a0 := math.Atan2(fromY-q.pivot[1], fromX-q.pivot[0])
	a1 := math.Atan2(toY-q.pivot[1], toX-q.pivot[0])
	delta := normalizeAngle(a1-a0) * 180 / math.Pi
	if delta == 0 {
		return
	}
	if det(m.parentGeoM(m.posePart)) < 0 {
		delta = -delta
	}
	d := m.StageClipDoc()
	cur, ok := d.value(m.posePart, "r", frame)
	if !ok || len(cur) == 0 {
		cur = []float64{0}
	}
	m.SetPoseValue("r", []float64{round2(cur[0] + delta)})
}

// MovePosePart drags the selected part's joint, converting the on-screen
// movement into the parent's space so the joint lands under the cursor.
//
// What follows the joint depends on the mode. By default the part does,
// which is how an attach point in the wrong place is corrected and how the
// character itself is moved (the body's joint is its position in the
// composition). With JointDragKeepsArt the artwork stays where it is and
// only the point it turns about moves, which is how a limb rotating about
// the wrong pixel is fixed.
func (m *Model) MovePosePart(dx, dy float64) {
	if m.blockEdit() {
		return
	}
	frame, ok := m.SelectedPoseKey()
	if !ok || m.posePart < 0 {
		return
	}
	g := m.parentGeoM(m.posePart)
	if det(g) == 0 {
		return
	}
	// Only the linear part matters: this is a delta, not a point.
	inv := g
	inv.SetElement(0, 2, 0)
	inv.SetElement(1, 2, 0)
	inv.Invert()
	ldx, ldy := inv.Apply(dx, dy)
	d := m.StageClipDoc()
	cur, ok := d.value(m.posePart, "p", frame)
	if !ok || len(cur) < 2 {
		return
	}
	next := slices.Clone(cur)
	next[0] = round2(next[0] + ldx)
	next[1] = round2(next[1] + ldy)
	if !m.jointKeepsArt {
		m.SetPoseValue("p", next)
		return
	}
	// Holding the artwork still means moving the anchor by the same step,
	// measured in the part's own space: the composition is
	// translate(p) . rotate . scale . translate(-a), so a shift of p is
	// cancelled by a shift of a through the part's own rotate-and-scale.
	a, ok := d.value(m.posePart, "a", frame)
	if !ok || len(a) < 2 {
		return
	}
	l := m.partLinear(frame)
	if det(l) == 0 {
		return
	}
	l.Invert()
	adx, ady := l.Apply(next[0]-cur[0], next[1]-cur[1])
	na := slices.Clone(a)
	na[0] = round2(na[0] + adx)
	na[1] = round2(na[1] + ady)
	m.SetPoseValue("p", next)
	m.SetPoseValue("a", na)
}

// partLinear is the selected part's own rotate-and-scale, without the
// translations either side of it.
func (m *Model) partLinear(frame float64) ebiten.GeoM {
	var l ebiten.GeoM
	d := m.StageClipDoc()
	if d == nil {
		return l
	}
	sx, sy := 1.0, 1.0
	if s, ok := d.value(m.posePart, "s", frame); ok && len(s) >= 2 {
		sx, sy = s[0]/100, s[1]/100
	}
	l.Scale(sx, sy)
	if r, ok := d.value(m.posePart, "r", frame); ok && len(r) > 0 {
		l.Rotate(r[0] * math.Pi / 180)
	}
	return l
}

// normalizeAngle folds a difference into (-pi, pi] so a drag that crosses
// the negative x-axis does not read as most of a turn the other way.
func normalizeAngle(a float64) float64 {
	for a > math.Pi {
		a -= 2 * math.Pi
	}
	for a <= -math.Pi {
		a += 2 * math.Pi
	}
	return a
}

func det(g ebiten.GeoM) float64 {
	return g.Element(0, 0)*g.Element(1, 1) - g.Element(0, 1)*g.Element(1, 0)
}

// ---- generator joint names ----

// poseJoints maps a rig slot to the field it is called in editor/genpresets
// clips.go. The editor is a probe for the repository's own presets: a pose
// found by dragging is transcribed back into the generator by hand, and
// hunting for which field a layer corresponds to is most of that work.
//
// A clip whose layers are not this rig simply shows no joint name.
var poseJoints = map[string]string{
	"body":           "at / lean / squash",
	"head":           "nod",
	"upper-arm-near": "arms(near)",
	"upper-arm-far":  "arms(far)",
	"forearm-near":   "elbows(near)",
	"forearm-far":    "elbows(far)",
	"thigh-near":     "legs(near)",
	"thigh-far":      "legs(far)",
	"shin-near":      "knees(near)",
	"shin-far":       "knees(far)",
	"sword":          "blades",
	"shadow":         "shade",
}

// PoseJointName names the generator field the selected part is driven by,
// or "" when the clip is not one of the presets.
func (m *Model) PoseJointName() string {
	return poseJoints[m.SelectedPosePartName()]
}

// ---- undo ----

// A pose drag writes numbers straight into the document on every mouse
// move, so it needs a way back. Undo here is scoped to clip edits — the
// editor has no undo for the machine graph yet (requirement:editor-mvp) and
// this feature is the one that cannot wait for it.
//
// Snapshots are whole encoded clips. They are a few tens of kilobytes each
// and the alternative — a typed diff per property — buys nothing at this
// size while being much easier to get subtly wrong.

const clipUndoLimit = 64

type clipSnapshot struct {
	id   string
	data []byte
	// The clip's texture document travels with it (nil with hasTex set
	// means the clip had none), so undoing a texture edit is one step too.
	tex    []byte
	hasTex bool
}

// BeginPoseEdit opens a drag. Every write until EndPoseEdit collapses into
// one undo step, so a swing that crossed forty frames of mouse movement is
// one thing to take back, not forty.
func (m *Model) BeginPoseEdit() {
	m.poseDragOpen, m.poseDragPushed = true, false
}

func (m *Model) EndPoseEdit() {
	m.poseDragOpen, m.poseDragPushed = false, false
}

// snapshotClip records the clip as it is now, before the caller changes it.
// It reports whether it actually pushed: a step in the middle of a drag
// rides on the snapshot the drag's first step took, and must not take back
// that one if its own write turns out to change nothing.
func (m *Model) snapshotClip() bool {
	if m.poseDragOpen && m.poseDragPushed {
		return false
	}
	d := m.StageClipDoc()
	if d == nil {
		return false
	}
	data, ok := m.bundle.AnimationJSON(d.id)
	if !ok {
		return false
	}
	snap := clipSnapshot{id: d.id, data: bytes.Clone(data)}
	if tex, ok := m.bundle.ExtensionFile(lottietexture.File(d.id)); ok {
		snap.tex, snap.hasTex = bytes.Clone(tex), true
	}
	m.clipUndo = append(m.clipUndo, snap)
	if n := len(m.clipUndo); n > clipUndoLimit {
		m.clipUndo = append(m.clipUndo[:0], m.clipUndo[n-clipUndoLimit:]...)
	}
	m.poseDragPushed = m.poseDragOpen
	return true
}

// dropLastSnapshot takes back a snapshot whose edit turned out to be a
// no-op — a drag step that rounded to the same frame, a typed value that
// did not change. Only the caller that pushed it may drop it, or a later
// no-op would discard the state an earlier edit needs to return to.
func (m *Model) dropLastSnapshot() {
	if n := len(m.clipUndo); n > 0 {
		m.clipUndo = m.clipUndo[:n-1]
		m.poseDragPushed = false
	}
}

// CanUndoClipEdit reports whether there is a pose edit to take back.
func (m *Model) CanUndoClipEdit() bool { return len(m.clipUndo) > 0 }

// UndoClipEdit restores the clip as it was before the last edit.
func (m *Model) UndoClipEdit() {
	if m.blockEdit() || len(m.clipUndo) == 0 {
		return
	}
	snap := m.clipUndo[len(m.clipUndo)-1]
	m.clipUndo = m.clipUndo[:len(m.clipUndo)-1]
	if err := m.bundle.SetAnimation(snap.id, snap.data); err != nil {
		m.setStatus("cannot undo: %v", err)
		return
	}
	if snap.hasTex {
		if err := m.bundle.SetExtensionFile(lottietexture.File(snap.id), snap.tex); err != nil {
			m.setStatus("cannot undo textures: %v", err)
		}
	} else {
		lottietexture.Remove(m.bundle, snap.id)
	}
	// The parsed document is the edited one; drop it so the restored bytes
	// are what the next question is answered from.
	delete(m.clipDocs, snap.id)
	m.rebuildStageClip(snap.id)
	m.docGen++
	m.generation++
	m.setStatus("undid an edit to %s", snap.id)
}

// ---- draw order and visibility ----

// CanReorderParts reports whether the overlap can be rearranged. Track
// mattes make the layer array mean more than draw order — a matte without an
// explicit source takes the layer before it — so a clip using them is left
// alone rather than silently retargeted.
func (m *Model) CanReorderParts() bool {
	d := m.StageClipDoc()
	return d != nil && !m.Viewer() && !d.usesTrackMatte()
}

// ReorderPosePart moves the selected part one place through the draw order:
// -1 towards the front, +1 towards the back. The rig's overlap is authored
// per clip — a gripping forearm belongs in front of the torso during a swing
// and behind it at rest — so this is a clip edit like any other.
func (m *Model) ReorderPosePart(delta int) {
	parts := m.PoseParts()
	i := slices.Index(parts, m.posePart)
	if i < 0 || i+delta < 0 || i+delta >= len(parts) {
		return
	}
	m.movePartToLayer(m.posePart, parts[i+delta])
}

// ReorderPosePartTo is the drag form: both ends are rows of the parts list.
func (m *Model) ReorderPosePartTo(from, to int) {
	parts := m.PoseParts()
	if from < 0 || from >= len(parts) {
		return
	}
	// The list reports where the row should land in the pre-move list, so a
	// downward drag has to account for the row leaving its old place first.
	if to > from {
		to--
	}
	if to < 0 || to >= len(parts) {
		return
	}
	m.movePartToLayer(parts[from], parts[to])
}

// movePartToLayer rewrites the draw order and carries the selection with it:
// the indices name positions in the layer array, and the move changes them.
func (m *Model) movePartToLayer(from, to int) {
	if m.blockEdit() || !m.CanReorderParts() {
		return
	}
	d := m.StageClipDoc()
	if d == nil {
		return
	}
	pushed := m.snapshotClip()
	if !d.moveLayer(from, to) {
		if pushed {
			m.dropLastSnapshot()
		}
		return
	}
	m.posePart = shiftIndex(m.posePart, from, to)
	if m.poseLayer >= 0 {
		m.poseLayer = shiftIndex(m.poseLayer, from, to)
	}
	m.touchClipDoc()
}

// TogglePosePartHidden switches the selected part's opacity between off and
// full at the selected key. The rigs swap drawings this way — a head seen
// from another angle waits at zero until the character turns — so which one
// is showing is a pose decision, not a property of the part.
func (m *Model) TogglePosePartHidden() {
	if m.posePart < 0 {
		return
	}
	if m.PosePartHidden(m.posePart) {
		m.SetPoseValue("o", []float64{100})
		return
	}
	m.SetPoseValue("o", []float64{0})
}

// ---- poses: insert, delete, navigate, ease, length ----

// InsertPose adds a pose at the playhead, copying the one before it. A new
// pose starts as the one it follows and is then changed, so the clip looks
// unchanged until the first edit.
//
// swap trades the paired limbs as it goes, which is what the second half of
// a walk cycle is: the first half with the legs the other way round.
func (m *Model) InsertPose(swap bool) {
	m.insertPoseAt(m.stageFrame(), "", 0, swap)
}

// InsertPoseFrom adds a pose at the playhead and fills it from a key of
// another clip, matched layer by layer by name. The clips of a preset share
// one rig, so a guard stance or a rest pose is worth borrowing rather than
// dialling in again.
func (m *Model) InsertPoseFrom(clip string, frame float64, swap bool) {
	m.insertPoseAt(m.stageFrame(), clip, frame, swap)
}

func (m *Model) insertPoseAt(frame float64, srcClip string, srcFrame float64, swap bool) {
	if m.blockEdit() {
		return
	}
	d := m.StageClipDoc()
	if d == nil {
		return
	}
	frame = math.Round(frame)
	pushed := m.snapshotClip()
	if !d.insertPose(frame) {
		if pushed {
			m.dropLastSnapshot()
		}
		return
	}
	if srcClip != "" {
		m.copyPoseInto(d, frame, srcClip, srcFrame)
	}
	// The swap comes last, so it trades whatever the pose ended up holding
	// rather than what it started as.
	if swap {
		d.swapPose(frame)
	}
	m.poseFrame, m.poseSet, m.poseLayer = frame, true, -1
	m.touchClipDoc()
	m.parkPreview(frame)
}

// copyPoseInto writes another clip's pose into the column just inserted.
// Layers are matched by name — the rig's slot names are the contract that
// makes the clips of a preset interchangeable — and a slot the source lacks
// keeps the value the insert copied from the previous pose.
func (m *Model) copyPoseInto(d *clipDoc, frame float64, srcClip string, srcFrame float64) {
	src := m.clipDocFor(srcClip)
	if src == nil {
		return
	}
	for li := range d.layers {
		name := d.layers[li].name
		if name == "" {
			continue
		}
		si := -1
		for i := range src.layers {
			if src.layers[i].name == name {
				si = i
				break
			}
		}
		if si < 0 {
			continue
		}
		for _, prop := range poseProps {
			v, ok := src.valueNear(si, prop, srcFrame)
			if !ok || len(v) == 0 {
				continue
			}
			d.setValue(li, prop, frame, v)
		}
	}
}

// clipDocFor parses any clip of the bundle, not just the one on stage, so a
// pose can be read out of a clip nobody is looking at.
func (m *Model) clipDocFor(id string) *clipDoc {
	if id == "" {
		return nil
	}
	if d, ok := m.clipDocs[id]; ok {
		return d
	}
	data, ok := m.bundle.AnimationJSON(id)
	if !ok {
		return nil
	}
	d, err := newClipDoc(id, data)
	if err != nil {
		d = nil
	}
	m.weaveTextures(d)
	if m.clipDocs == nil {
		m.clipDocs = map[string]*clipDoc{}
	}
	m.clipDocs[id] = d
	return d
}

// PoseSourceKeys are the key times of a clip a pose could be copied from.
func (m *Model) PoseSourceKeys(id string) []float64 {
	d := m.clipDocFor(id)
	if d == nil {
		return nil
	}
	return d.times
}

// CanInsertPose reports whether the playhead is somewhere a pose could go:
// inside the clip and not on a key already.
func (m *Model) CanInsertPose() bool {
	d := m.StageClipDoc()
	if d == nil || m.Viewer() {
		return false
	}
	f := math.Round(m.stageFrame())
	return f >= d.inPoint() && f <= d.outPoint() && !slices.Contains(d.times, f)
}

// DeletePose removes the selected pose. The last one is kept: a track with
// no keys is a different kind of clip, not an empty one.
func (m *Model) DeletePose() {
	if m.blockEdit() {
		return
	}
	d := m.StageClipDoc()
	frame, ok := m.SelectedPoseKey()
	if d == nil || !ok {
		return
	}
	pushed := m.snapshotClip()
	if !d.deletePose(frame) {
		if pushed {
			m.dropLastSnapshot()
		}
		return
	}
	m.clearPoseSelection()
	m.touchClipDoc()
}

// CanDeletePose reports whether there is a pose to remove and more than one
// to remove it from.
func (m *Model) CanDeletePose() bool {
	d := m.StageClipDoc()
	if d == nil || m.Viewer() {
		return false
	}
	_, ok := m.SelectedPoseKey()
	return ok && len(m.poseTimesFor(m.poseLayer)) > 1
}

// JumpToKey parks on the next key in a direction, which is how a pose
// sequence is stepped through: the frames between keys hold nothing to edit.
func (m *Model) JumpToKey(dir int) {
	times := m.poseTimesFor(m.poseLayer)
	if len(times) == 0 {
		times = m.PoseTimes()
	}
	if len(times) == 0 {
		return
	}
	f := m.stageFrame()
	target, found := 0.0, false
	if dir < 0 {
		for i := len(times) - 1; i >= 0; i-- {
			if times[i] < f {
				target, found = times[i], true
				break
			}
		}
	} else {
		for _, t := range times {
			if t > f {
				target, found = t, true
				break
			}
		}
	}
	if !found {
		return
	}
	m.SelectPoseKey(target, m.poseLayer)
}

// SwapPose trades the paired limbs of the selected pose in place, for when
// the pose is already there and it is the wrong way round.
func (m *Model) SwapPose() {
	if m.blockEdit() {
		return
	}
	d := m.StageClipDoc()
	frame, ok := m.SelectedPoseKey()
	if d == nil || !ok {
		return
	}
	pushed := m.snapshotClip()
	if !d.swapPose(frame) {
		if pushed {
			m.dropLastSnapshot()
		}
		return
	}
	m.touchClipDoc()
}

// PoseEased reports whether the selected pose arrives on a curve.
func (m *Model) PoseEased() bool {
	d := m.StageClipDoc()
	frame, ok := m.SelectedPoseKey()
	return d != nil && ok && d.poseEased(frame)
}

// SetPoseEase gives the selected pose a curve, or takes it away.
func (m *Model) SetPoseEase(eased bool) {
	if m.blockEdit() {
		return
	}
	d := m.StageClipDoc()
	frame, ok := m.SelectedPoseKey()
	if d == nil || !ok || eased == d.poseEased(frame) {
		return
	}
	pushed := m.snapshotClip()
	if !d.setPoseEase(frame, eased) {
		if pushed {
			m.dropLastSnapshot()
		}
		return
	}
	m.touchClipDoc()
}

// ClipLength is how many frames the stage clip runs for.
func (m *Model) ClipLength() float64 {
	d := m.StageClipDoc()
	if d == nil {
		return 0
	}
	return d.outPoint()
}

// SetClipLength retimes the end of the clip. It refuses to cut a pose off:
// delete the pose first, which says plainly what is being lost.
func (m *Model) SetClipLength(op float64) {
	if m.blockEdit() {
		return
	}
	d := m.StageClipDoc()
	if d == nil {
		return
	}
	if !finite(op) {
		m.rejectValue("clip length")
		return
	}
	pushed := m.snapshotClip()
	if !d.setLength(math.Round(op)) {
		if pushed {
			m.dropLastSnapshot()
		}
		m.setStatus("clip length must be past the last pose")
		m.generation++
		return
	}
	m.touchClipDoc()
}

// ---- parenting and the joint ----

// The joint a part hangs from is one point named twice: ks.a says which
// pixel of the part it is, ks.p says where that pixel sits in the parent's
// space. Both belong to the child; the parent knows nothing. Everything
// below keeps that pair honest while the structure around it changes.

// PoseParentCandidates are the layers the selected part may be attached to,
// its own descendants excluded — a cycle is not something to report, it is
// something the picker should not be able to say.
func (m *Model) PoseParentCandidates() []int {
	d := m.StageClipDoc()
	if d == nil || m.posePart < 0 {
		return nil
	}
	return d.parentCandidates(m.posePart)
}

// PosePartParent is the selected part's parent, or false when it rides on
// the composition itself.
func (m *Model) PosePartParent() (int, bool) {
	d := m.StageClipDoc()
	if d == nil || m.posePart < 0 {
		return 0, false
	}
	return d.parentOf(m.posePart)
}

// parentMatrixOf is a layer's parent's world transform, or the identity when
// it has none.
func (m *Model) parentMatrixOf(layer int) ebiten.GeoM {
	var g ebiten.GeoM
	d := m.StageClipDoc()
	anim := m.PreviewAnimation()
	if d == nil || anim == nil {
		return g
	}
	p, ok := d.parentOf(layer)
	if !ok {
		return g
	}
	name := d.layers[p].name
	if name == "" {
		return g
	}
	if pg, ok := anim.LayerTransform(name, m.stageFrame()); ok {
		return pg
	}
	return g
}

// SetPosePartParent attaches the selected part to another layer, or to the
// composition when parent is negative, and rewrites its transform so it does
// not move. Position, rotation and scale are all in the parent's terms, so
// all three are restated in the new parent's.
//
// A value keeps the form it had. A static one is corrected once and stays
// static, because static means "rigidly attached here" and that is still
// true of the new parent. A keyed one is corrected at every key, since the
// correction is not the same at every frame — two parents that move
// differently need a different answer at each.
func (m *Model) SetPosePartParent(parent int) {
	if m.blockEdit() {
		return
	}
	d := m.StageClipDoc()
	if d == nil || m.posePart < 0 {
		return
	}
	if _, onKey := m.SelectedPoseKey(); !onKey {
		m.setStatus("park on a key before re-parenting: the transform is rewritten there")
		return
	}
	oldParent, ok := d.parentOf(m.posePart)
	if !d.layerHasParent(m.posePart) {
		oldParent = -1
	} else if !ok {
		// The layer names a parent that is not in this clip; corrections
		// measured against layer 0 in its place would be garbage.
		m.setStatus("cannot re-parent: the current parent is not in this clip")
		return
	}
	// Every correction is measured against the clip as it is now, so they
	// are all worked out before the link changes.
	type fix struct {
		prop   string
		frame  float64
		static bool
		value  []float64
	}
	var fixes []fix
	for _, prop := range []string{"p", "r", "s"} {
		frames := d.keyTimesOf(m.posePart, prop)
		if len(frames) == 0 {
			v, ok := d.staticValue(m.posePart, prop)
			if !ok {
				continue
			}
			f := m.stageFrame()
			fixes = append(fixes, fix{prop, f, true, m.correct(prop, v, oldParent, parent, f)})
			continue
		}
		for _, f := range frames {
			v, ok := d.valueAt(m.posePart, prop, f)
			if !ok {
				continue
			}
			fixes = append(fixes, fix{prop, f, false, m.correct(prop, v, oldParent, parent, f)})
		}
	}

	pushed := m.snapshotClip()
	if !d.setParent(m.posePart, parent) {
		if pushed {
			m.dropLastSnapshot()
		}
		return
	}
	for _, fx := range fixes {
		if fx.value == nil {
			continue
		}
		if fx.static {
			d.setStatic(m.posePart, fx.prop, fx.value)
		} else {
			d.setValue(m.posePart, fx.prop, fx.frame, fx.value)
		}
	}
	m.touchClipDoc()
}

// correct restates one transform member in the new parent's terms. The
// carrier is X = inverse(new) . old, the matrix that takes a point from the
// old parent's space into the new one.
func (m *Model) correct(prop string, v []float64, oldParent, newParent int, frame float64) []float64 {
	x := m.reparentX(oldParent, newParent, frame)
	switch prop {
	case "p":
		if len(v) < 2 {
			return nil
		}
		px, py := x.Apply(v[0], v[1])
		return []float64{round2(px), round2(py)}
	case "r":
		if len(v) == 0 {
			return nil
		}
		return []float64{round2(v[0] + geoMAngle(x)*180/math.Pi)}
	case "s":
		if len(v) < 2 {
			return nil
		}
		sx, sy := geoMScale(x)
		return []float64{round2(v[0] * sx), round2(v[1] * sy)}
	}
	return nil
}

// reparentX is the matrix carrying a point from one parent's space to
// another's, at one frame. GeoM.Concat applies its argument after the
// receiver, so the inverse is the one concatenated on.
func (m *Model) reparentX(oldParent, newParent int, frame float64) ebiten.GeoM {
	old := m.layerMatrixAt(oldParent, frame)
	inv := m.layerMatrixAt(newParent, frame)
	if det(inv) == 0 {
		inv = ebiten.GeoM{}
	}
	inv.Invert()
	x := old
	x.Concat(inv)
	return x
}

// layerMatrixAt is a layer's world transform, or the identity for the
// composition itself.
func (m *Model) layerMatrixAt(layer int, frame float64) ebiten.GeoM {
	var g ebiten.GeoM
	d := m.StageClipDoc()
	anim := m.PreviewAnimation()
	if layer < 0 || d == nil || anim == nil {
		return g
	}
	l := d.layer(layer)
	if l == nil || l.name == "" {
		return g
	}
	if lg, ok := anim.LayerTransform(l.name, frame); ok {
		return lg
	}
	return g
}

// geoMAngle and geoMScale decompose the correction. They lose a mirror the
// way api:layer-placement's own decomposition does, so a re-parent across
// chains of opposite handedness lands the part in the right place and may
// still want a nudge.
func geoMAngle(g ebiten.GeoM) float64 {
	return math.Atan2(g.Element(1, 0), g.Element(0, 0))
}

func geoMScale(g ebiten.GeoM) (sx, sy float64) {
	sx = math.Hypot(g.Element(0, 0), g.Element(1, 0))
	sy = math.Hypot(g.Element(0, 1), g.Element(1, 1))
	if sx == 0 {
		sx = 1
	}
	if sy == 0 {
		sy = 1
	}
	return sx, sy
}

// JointDragKeepsArt reports which of the two things a joint drag does.
func (m *Model) JointDragKeepsArt() bool { return m.jointKeepsArt }

// SetJointDragKeepsArt picks between moving the part by its joint and moving
// the joint through the part. Both are wanted: an attach point in the wrong
// place needs the first, a limb rotating about the wrong pixel the second.
func (m *Model) SetJointDragKeepsArt(v bool) {
	m.jointKeepsArt = v
	m.generation++
}

// ---- rig overlay ----

// The rig is a graph the artwork hides: a joint per part and a bone from
// each to its parent's. Drawing it says at a glance what the parts list and
// the parent picker say one row at a time — which chain a part is on, and
// where it actually pivots. Over the onion ghosts it also answers the
// question the ghosts are for, more sharply than the drawings do: a joint
// that travelled is a line between two dots, where two overlapping heads at
// a third of an alpha are a smudge.

// rigJoint is one part's pivot in animation coordinates, with the joint it
// hangs from.
type rigJoint struct {
	at     [2]float64
	parent int // index into the same slice; -1 for a part riding the composition
	layer  int
}

func (m *Model) ShowRig() bool { return m.showRig }

func (m *Model) SetShowRig(v bool) {
	m.showRig = v
	m.generation++
}

// RigJoints is the skeleton at one frame. Parts switched off by opacity are
// left out: their joints sit on top of the drawing that replaced them, and
// two dots in one place say less than one.
func (m *Model) RigJoints(frame float64) []rigJoint {
	d := m.StageClipDoc()
	anim := m.PreviewAnimation()
	if d == nil || anim == nil {
		return nil
	}
	slot := make(map[int]int, len(d.layers)) // layer index -> position in out
	var out []rigJoint
	for i := range d.layers {
		l := &d.layers[i]
		if l.ty != 2 || l.name == "" {
			continue
		}
		if o, ok := d.valueNear(i, "o", frame); ok && len(o) > 0 && o[0] <= 0 {
			continue
		}
		g, ok := anim.LayerTransform(l.name, frame)
		if !ok {
			continue
		}
		a, ok := d.value(i, "a", frame)
		if !ok || len(a) < 2 {
			a = []float64{0, 0}
		}
		x, y := g.Apply(a[0], a[1])
		slot[i] = len(out)
		out = append(out, rigJoint{at: [2]float64{x, y}, parent: -1, layer: i})
	}
	// Second pass: the parent may be listed after the child, and a hidden
	// one is not listed at all.
	for i := range out {
		if p, ok := d.parentOf(out[i].layer); ok {
			if j, ok := slot[p]; ok {
				out[i].parent = j
			}
		}
	}
	return out
}

// ---- names ----

// PosePartNameProblem reports why the selected part cannot be posed on the
// stage, or "" when it can. Everything the stage does — outlining a part,
// turning a drag into the parent's terms — asks the core for a layer by
// name, and the core takes the first match. An unnamed or duplicated layer
// therefore gets the wrong matrix rather than none, which would mean a drag
// that writes plausible numbers into the wrong space. Better to refuse.
func (m *Model) PosePartNameProblem() string {
	d := m.StageClipDoc()
	if d == nil || m.posePart < 0 {
		return ""
	}
	if why := d.nameProblem(m.posePart); why != "" {
		return why
	}
	if p, ok := d.parentOf(m.posePart); ok {
		if why := d.nameProblem(p); why != "" {
			return "its parent cannot be addressed: " + why
		}
	} else if d.layerHasParent(m.posePart) {
		return "its parent is not in this clip"
	}
	return ""
}

// PosePartDraggable reports whether the stage may edit the selected part.
func (m *Model) PosePartDraggable() bool {
	return m.posePart >= 0 && m.PosePartNameProblem() == ""
}

// RenamePosePart renames the selected layer. Names are how a socket binds a
// layer, how a pose is copied between clips, and how the near/far pairs are
// found, so a rename is a real edit and not a label change.
func (m *Model) RenamePosePart(name string) {
	if m.blockEdit() {
		return
	}
	d := m.StageClipDoc()
	if d == nil || m.posePart < 0 {
		return
	}
	name = strings.TrimSpace(name)
	old := d.layers[m.posePart].name
	pushed := m.snapshotClip()
	if !d.setName(m.posePart, name) {
		if pushed {
			m.dropLastSnapshot()
		}
		if name != "" && name != old {
			m.setStatus("a layer is already called %q", name)
			m.generation++
		}
		return
	}
	// A socket binds a layer by name across every clip, so renaming here
	// unbinds it here and nowhere else. Saying so beats finding out later.
	for _, s := range m.Sockets() {
		if s.LayerName() == old {
			m.setStatus("renamed %q to %q; socket %q still binds the old name",
				old, name, s.Name)
			break
		}
	}
	m.touchClipDoc()
}
