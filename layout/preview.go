package main

import (
	"image"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	lottie "github.com/shibukawa/lottie-go"
)

// sourcePreview plays the source selected in the palette, looped, so a
// clip can be judged before it is placed — names alone do not say what a
// clip looks like.
type sourcePreview struct {
	guigui.DefaultWidget

	key     sourceRef
	keySet  bool
	player  *lottie.Player
	machine *lottie.StateMachinePlayer
	img     *ebiten.Image
}

func (s *sourcePreview) model(context *guigui.Context) *Model {
	v, ok := context.Env(s, envKeyModel)
	if !ok {
		return nil
	}
	m, _ := v.(*Model)
	return m
}

// sync rebuilds the playback when the palette selection moved.
func (s *sourcePreview) sync(m *Model) {
	ref, ok := m.SelectedSourceRef()
	if !ok {
		s.keySet, s.player, s.machine, s.img = false, nil, nil, nil
		return
	}
	if s.keySet && ref == s.key {
		return
	}
	s.key, s.keySet = ref, true
	s.player, s.machine, s.img = nil, nil, nil
	if ref.Kind == sourceImage {
		s.img, _ = m.Image(ref.Alias)
		return
	}
	b, ok := m.Bundle(ref.Alias)
	if !ok {
		return
	}
	// A bundle previews what a placed node would first show: its initial
	// machine, or its first clip looped.
	kind, id, okc := initialContent(b)
	if !okc {
		return
	}
	switch kind {
	case lottie.SceneNodeAnimation:
		anim, err := b.Animation(id)
		if err != nil {
			return
		}
		p := anim.NewPlayer()
		p.SetLoop(true)
		p.Play()
		s.player = p
	case lottie.SceneNodeMachine:
		if sm, err := b.NewStateMachinePlayer(id); err == nil {
			s.machine = sm
		}
	}
}

func (s *sourcePreview) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	m := s.model(context)
	if m == nil {
		return nil
	}
	s.sync(m)
	switch {
	case s.machine != nil:
		s.machine.Update()
	case s.player != nil:
		s.player.Update()
	}
	guigui.RequestRedraw(s)
	return nil
}

// anim is the animation currently showing, for sizing the drawing.
func (s *sourcePreview) anim() *lottie.Animation {
	switch {
	case s.machine != nil && s.machine.Player() != nil:
		return s.machine.Player().Animation()
	case s.player != nil:
		return s.player.Animation()
	}
	return nil
}

func (s *sourcePreview) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	pal := paletteFor(context)
	b := widgetBounds.Bounds()
	vector.StrokeRect(dst, float32(b.Min.X)+0.5, float32(b.Min.Y)+0.5,
		float32(b.Dx())-1, float32(b.Dy())-1, 1, pal.frame, false)
	aw, ah := 0, 0
	if s.img != nil {
		ib := s.img.Bounds()
		aw, ah = ib.Dx(), ib.Dy()
	} else if anim := s.anim(); anim != nil {
		aw, ah = anim.Size()
	}
	if aw <= 0 || ah <= 0 {
		return
	}
	pad := basicwidget.UnitSize(context) / 4
	inner := b.Inset(pad)
	scale := min(float64(inner.Dx())/float64(aw), float64(inner.Dy())/float64(ah))
	var op lottie.DrawOptions
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(
		float64(inner.Min.X)+(float64(inner.Dx())-float64(aw)*scale)/2,
		float64(inner.Min.Y)+(float64(inner.Dy())-float64(ah)*scale)/2,
	)
	switch {
	case s.img != nil:
		var iop ebiten.DrawImageOptions
		iop.GeoM = op.GeoM
		iop.Filter = ebiten.FilterLinear
		dst.DrawImage(s.img, &iop)
	case s.machine != nil:
		s.machine.Draw(dst, &op)
	case s.player != nil:
		s.player.Draw(dst, &op)
	}
}

func (s *sourcePreview) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	return image.Pt(6*u, 5*u)
}
