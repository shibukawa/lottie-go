package main

import (
	"image"
	"runtime"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// How big the stage is, and which part of it you are looking at.
//
// Fitting the whole clip in the pane is the right default for watching a
// machine run, and the wrong one for posing: a chibi rig lands about a
// hundred pixels tall, and a forearm is a few pixels wide at that size. Two
// separate things make it bigger — a zoom on the stage itself, and a
// splitter that gives the whole preview more of the window.

// Zoom is a multiple of the fit scale, so 1 is "the whole clip, centred",
// whatever the pane happens to be. Below 1 is allowed but only a little:
// zooming out past the fit is almost never what someone meant.
const (
	stageZoomMin  = 0.5
	stageZoomMax  = 20
	stageZoomStep = 1.1
)

// adjustedWheel normalizes the wheel the way basicwidget's own scrolling
// does: a macOS trackpad reports far smaller steps than a wheel elsewhere,
// so an unadjusted delta zooms at wildly different speeds per platform.
func adjustedWheel() (float64, float64) {
	x, y := ebiten.Wheel()
	switch runtime.GOOS {
	case "darwin":
		x *= 2
		y *= 2
	case "js":
	default:
		x *= 10
		y *= 10
	}
	return x, y
}

// StageZoom is the current magnification, 1 being fit-to-pane.
func (m *Model) StageZoom() float64 {
	if m.stageZoom == 0 {
		return 1
	}
	return m.stageZoom
}

// StagePan is how far the view has been dragged, in screen pixels.
func (m *Model) StagePan() (x, y float64) { return m.stagePanX, m.stagePanY }

// SetStageView records a zoom and pan the stage worked out for itself. The
// geometry lives in the widget, which is the only thing that knows where the
// clip is being drawn; the model just remembers the answer.
func (m *Model) SetStageView(zoom, panX, panY float64) {
	m.stageZoom = min(max(zoom, stageZoomMin), stageZoomMax)
	m.stagePanX, m.stagePanY = panX, panY
	m.generation++
}

// StageViewChanged reports whether the view has moved off fit-and-centre,
// which is what makes the reset worth offering.
func (m *Model) StageViewChanged() bool {
	return m.StageZoom() != 1 || m.stagePanX != 0 || m.stagePanY != 0
}

// ResetStageView returns to the whole clip, centred.
func (m *Model) ResetStageView() {
	m.stageZoom, m.stagePanX, m.stagePanY = 1, 0, 0
	m.generation++
}

// InitPreviewHeight seeds the split the first time the window is laid out.
// The default has to come from the real height or it would mean something
// different on every screen, and the layout is the only place that knows it.
func (m *Model) InitPreviewHeight(def int) {
	if m.previewH == 0 {
		m.previewH = def
	}
}

// PreviewHeight is how tall the preview pane is, in pixels.
func (m *Model) PreviewHeight() int { return m.previewH }

// SetPreviewHeight moves the splitter. The caller clamps against the window,
// which is the only place the available height is known.
func (m *Model) SetPreviewHeight(h int) {
	if h == m.previewH {
		return
	}
	m.previewH = h
	m.generation++
}

// splitterView is the draggable boundary between the state graph and the
// preview under it. Dragging it up hands the stage the room the graph was
// using, which is how a rig gets big enough to pose without zooming.
type splitterView struct {
	guigui.DefaultWidget

	dragging bool
	lastY    int
	limit    int // the tallest the preview may be, set by the layout
}

func (s *splitterView) model(context *guigui.Context) *Model {
	v, ok := context.Env(s, envKeyModel)
	if !ok {
		return nil
	}
	m, _ := v.(*Model)
	return m
}

func (s *splitterView) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	w := 8 * u
	if cw, ok := constraints.FixedWidth(); ok {
		w = cw
	}
	// Wide enough to grab without hunting; the line drawn inside it is thin.
	return image.Pt(w, u/2)
}

func (s *splitterView) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	pal := paletteFor(context)
	y := float32(b.Min.Y+b.Max.Y) / 2
	clr := pal.frame
	if s.dragging || widgetBounds.IsHitAtCursor() {
		clr = pal.playhead
	}
	// A short grip in the middle says the whole strip is draggable without
	// drawing a full-width bar that would read as a border.
	u := float32(basicwidget.UnitSize(context))
	cx := float32(b.Min.X+b.Max.X) / 2
	vector.StrokeLine(dst, float32(b.Min.X), y, float32(b.Max.X), y, 1, pal.frame, false)
	vector.StrokeLine(dst, cx-u, y, cx+u, y, max(2, u/8), clr, true)
}

func (s *splitterView) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if s.dragging || widgetBounds.IsHitAtCursor() {
		return ebiten.CursorShapeNSResize, true
	}
	return 0, false
}

func (s *splitterView) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	m := s.model(context)
	if m == nil {
		return guigui.HandleInputResult{}
	}
	u := basicwidget.UnitSize(context)
	if s.dragging {
		if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			s.dragging = false
			return guigui.HandleInputByWidget(s)
		}
		_, cy := ebiten.CursorPosition()
		if dy := cy - s.lastY; dy != 0 {
			s.lastY = cy
			// The preview is below the splitter, so dragging up grows it.
			h := m.PreviewHeight() - dy
			m.SetPreviewHeight(min(max(h, 4*u), max(s.limit, 4*u)))
		}
		return guigui.HandleInputByWidget(s)
	}
	if !widgetBounds.IsHitAtCursor() || !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return guigui.HandleInputResult{}
	}
	_, s.lastY = ebiten.CursorPosition()
	s.dragging = true
	return guigui.HandleInputByWidget(s)
}

func (s *splitterView) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	if m := s.model(context); m != nil {
		w.WriteInt(m.Generation())
	}
}

// stageZoomButtonStep is a coarser step than the wheel's: a button press is
// one deliberate act, where a wheel notch is one of many.
const stageZoomButtonStep = 1.5

// ZoomStage magnifies about the middle of whatever is on stage, which is
// what a button press can mean — the wheel zooms about the cursor instead,
// because there the cursor is the thing being pointed at.
func (m *Model) ZoomStage(factor float64) {
	zoom := min(max(m.StageZoom()*factor, stageZoomMin), stageZoomMax)
	if zoom == m.StageZoom() {
		return
	}
	// The pan is in screen pixels at the old scale, so it has to grow with
	// it or the view would drift back towards the centre on every press.
	k := zoom / m.StageZoom()
	m.SetStageView(zoom, m.stagePanX*k, m.stagePanY*k)
}

// ---- onion skin ----

// Onion skin draws the keyframes either side of the playhead faintly under
// the current one. Posing is mostly a question of how far a limb travels
// between two poses, and answering it by stepping back and forth means
// holding the previous pose in your head; drawing it instead is the trick
// animators have used on paper for a century.
//
// The neighbours are tinted rather than merely faded: which side a ghost is
// on is the thing being read, and two identical grey silhouettes do not say
// it.

// onionAlpha is faint enough to read as background and solid enough to see
// against the checkerless white stage.
const onionAlpha = 0.35

// onionGhost is one neighbouring keyframe to draw behind the current one.
type onionGhost struct {
	frame float64
	next  bool // the key after the playhead, rather than the one before it
}

func (m *Model) OnionSkin() bool { return m.onionSkin }

func (m *Model) SetOnionSkin(v bool) {
	m.onionSkin = v
	m.generation++
}

// OnionGhosts are the keys to draw behind the current frame: the last one
// before the playhead and the first one after it. Between two keys that is
// the pair bracketing the playhead, which is as useful while scrubbing as it
// is while parked on a key.
//
// Nothing is drawn while the clip is playing: the pair would change several
// times a second and strobe.
func (m *Model) OnionGhosts() []onionGhost {
	if !m.onionSkin || m.PreviewPlaying() {
		return nil
	}
	times := m.poseTimesFor(m.poseLayer)
	if len(times) == 0 {
		// No row is selected, or the clip's properties disagree on their
		// times so there is no pose row to read. The union of every
		// animated track is what "the next drawing" means either way.
		if d := m.StageClipDoc(); d != nil {
			times = d.times
		}
	}
	if len(times) == 0 {
		return nil
	}
	f := m.stageFrame()
	var out []onionGhost
	for i := len(times) - 1; i >= 0; i-- {
		if times[i] < f {
			out = append(out, onionGhost{frame: times[i]})
			break
		}
	}
	for _, t := range times {
		if t > f {
			out = append(out, onionGhost{frame: t, next: true})
			break
		}
	}
	return out
}
