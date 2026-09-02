package lottie

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"testing"
)

func almost(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestSceneCameraGeoM(t *testing.T) {
	apply := func(c SceneCamera, depth, x, y float64) (float64, float64) {
		g := c.GeoM(400, 400, depth)
		return g.Apply(x, y)
	}
	// Moving the camera right/down shifts the content left/up, scaled by
	// the node's depth.
	cam := SceneCamera{X: 100, Y: 50}
	x, y := apply(cam, 1, 200, 200)
	if !almost(x, 100) || !almost(y, 150) {
		t.Errorf("depth 1 pan: (%v, %v), want (100, 150)", x, y)
	}
	x, y = apply(cam, 0.5, 200, 200)
	if !almost(x, 150) || !almost(y, 175) {
		t.Errorf("depth 0.5 pan: (%v, %v), want (150, 175)", x, y)
	}
	x, y = apply(cam, 0, 200, 200)
	if !almost(x, 200) || !almost(y, 200) {
		t.Errorf("depth 0 must be identity, got (%v, %v)", x, y)
	}

	// Zoom raises to the depth's power around the design center: zoom 4 at
	// depth 0.5 magnifies 2x.
	x, y = apply(SceneCamera{Zoom: 4}, 0.5, 300, 200)
	if !almost(x, 400) || !almost(y, 200) {
		t.Errorf("zoom 4 depth 0.5: (%v, %v), want (400, 200)", x, y)
	}

	// A clockwise camera tilt shows the content rotated the other way: a
	// point right of center moves up (y-down coordinates).
	x, y = apply(SceneCamera{Rotation: 90}, 1, 300, 200)
	if !almost(x, 200) || !almost(y, 100) {
		t.Errorf("rotation 90 depth 1: (%v, %v), want (200, 100)", x, y)
	}
}

func TestSceneCameraPhaseAndOverride(t *testing.T) {
	s, err := ParseScene([]byte(`{
		"size": {"w": 400, "h": 400},
		"camera": {"x": 10},
		"bundles": [{"alias": "ui", "path": "ui.lottie"}],
		"phases": [
			{"name": "a"},
			{"name": "b", "camera": {"x": 100, "zoom": 2}}
		],
		"nodes": [
			{"name": "logo", "kind": "animation", "source": {"bundle": "ui", "id": "logo"},
			 "playback": {"loop": true, "autoplay": true}}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if errs := s.Validate(); len(errs) > 0 {
		t.Fatalf("valid scene reported %v", errs)
	}
	b := sceneTestBundle(t)
	sp, err := s.NewScenePlayer(func(string) (*Bundle, error) { return b, nil })
	if err != nil {
		t.Fatal(err)
	}
	if got := sp.Camera(); got.X != 10 {
		t.Fatalf("phase a camera = %+v, want the scene's x=10", got)
	}
	sp.SetPhase("b")
	if got := sp.Camera(); got.X != 100 || got.ZoomFactor() != 2 {
		t.Fatalf("phase b camera = %+v, want the override x=100 zoom=2", got)
	}
	sp.SetCamera(SceneCamera{X: 5})
	if got := sp.Camera(); got.X != 5 {
		t.Fatalf("SetCamera override = %+v, want x=5", got)
	}
	sp.SetPhase("a")
	if got := sp.Camera(); got.X != 10 {
		t.Fatalf("re-entering a phase must resolve the document camera, got %+v", got)
	}
	sp.SetCamera(SceneCamera{X: 5})
	sp.Restart()
	if got := sp.Camera(); got.X != 10 {
		t.Fatalf("Restart must resolve the document camera, got %+v", got)
	}
}

func TestScenePointerUnderCameraWithDepth(t *testing.T) {
	// Camera x=100 shifts depth-1 nodes 100 left; the depth-0 HUD stays.
	s, err := ParseScene([]byte(`{
		"size": {"w": 400, "h": 400},
		"camera": {"x": 100},
		"bundles": [{"alias": "ui", "path": "ui.lottie"}],
		"nodes": [
			{"name": "world", "kind": "machine", "source": {"bundle": "ui", "id": "btn"},
			 "transform": {"x": 50, "y": 150}},
			{"name": "hud", "kind": "machine", "source": {"bundle": "ui", "id": "btn"},
			 "transform": {"x": 50, "y": 280}, "depth": 0}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	b := sceneTestBundle(t)
	sp, err := s.NewScenePlayer(func(path string) (*Bundle, error) {
		if path != "ui.lottie" {
			return nil, fmt.Errorf("unexpected path %q", path)
		}
		return b, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// world's 100x100 box moved from (50,150) to (-50,150); its on-screen
	// remnant is hit, its old spot is empty.
	if n, ok := sp.NodeAt(10, 160); !ok || n.Name() != "world" {
		t.Errorf("NodeAt(10,160) = %v, want world at its camera-shifted spot", n)
	}
	if _, ok := sp.NodeAt(120, 160); ok {
		t.Error("world still hit at its unshifted spot")
	}
	// hud pins to the screen: hit where the document places it.
	if n, ok := sp.NodeAt(60, 300); !ok || n.Name() != "hud" {
		t.Errorf("NodeAt(60,300) = %v, want the depth-0 hud unmoved", n)
	}
}

func TestSceneNodeDepthRoundTrip(t *testing.T) {
	s, err := ParseScene([]byte(`{
		"size": {"w": 100, "h": 100},
		"nodes": [
			{"name": "a", "kind": "text", "text": {"font": "f"}},
			{"name": "b", "kind": "text", "text": {"font": "f"}, "depth": 0},
			{"name": "c", "kind": "text", "text": {"font": "f"}, "depth": 0.25}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]float64{"a": 1, "b": 0, "c": 0.25} {
		n, _ := s.Node(name)
		if got := n.ParallaxDepth(); got != want {
			t.Errorf("node %s depth = %v, want %v", name, got, want)
		}
	}
	var buf bytes.Buffer
	if err := s.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	// depth 0 is meaningful and must survive a save; absent must stay
	// absent.
	out := buf.String()
	if !strings.Contains(out, `"depth": 0`) {
		t.Error("depth 0 dropped on encode")
	}
	if strings.Count(out, `"depth"`) != 2 {
		t.Errorf("want exactly two depth members, got:\n%s", out)
	}
}

func TestSceneValidateCameraZoom(t *testing.T) {
	s, err := ParseScene([]byte(`{
		"size": {"w": 100, "h": 100},
		"camera": {"zoom": -1},
		"phases": [{"name": "a", "camera": {"zoom": -2}}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	errs := s.Validate()
	var scene, phase bool
	for _, e := range errs {
		if strings.Contains(e.Error(), "scene camera has negative zoom") {
			scene = true
		}
		if strings.Contains(e.Error(), `phase "a" camera has negative zoom`) {
			phase = true
		}
	}
	if !scene || !phase {
		t.Errorf("negative zooms not reported: %v", errs)
	}
}

// A negative zoom raised to a fractional depth is NaN; SetCamera takes it
// as absent so the nodes keep drawing at finite positions.
func TestSetCameraNegativeZoomStaysFinite(t *testing.T) {
	_, sp := menuScene(t)
	sp.SetCamera(SceneCamera{X: 10, Zoom: -2})
	if got := sp.Camera(); got.Zoom != 0 || got.X != 10 {
		t.Errorf("Camera() = %+v; want zoom dropped, pan kept", got)
	}
	g := sp.cameraGeoM(0.5)
	x, y := g.Apply(200, 200)
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
		t.Errorf("depth 0.5 under a negative zoom mapped (200, 200) to (%v, %v)", x, y)
	}
	sp.SetCamera(SceneCamera{X: math.NaN(), Zoom: math.Inf(1), Rotation: math.NaN()})
	if got := sp.Camera(); !got.isIdentity() {
		t.Errorf("non-finite camera = %+v; want identity", got)
	}
}
