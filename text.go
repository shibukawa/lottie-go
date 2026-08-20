package lottie

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

// FontResolver supplies font sources for text layers. family and style come
// from the animation's font list (e.g. "Roboto", "Bold"); return nil to
// skip rendering text set in that font.
type FontResolver func(family, style string) *text.GoTextFaceSource

// textResolverNote is reported until a FontResolver is installed.
const textResolverNote = "text layer (set a font with SetFontResolver)"

// SetFontResolver installs the font lookup used by text layers. Without a
// resolver, text layers are skipped and reported as unsupported.
func (a *Animation) SetFontResolver(r FontResolver) {
	a.fontResolver = r
	if r != nil {
		delete(a.unsupported, textResolverNote)
	}
}

// textDoc is one evaluated text document.
type textDoc struct {
	text       string
	family     string
	style      string
	size       float64
	color      [3]float64
	justify    int // 0 left, 1 right, 2 center
	lineHeight float64
}

// textNode is the compiled form of a text layer's document keyframes.
// Document changes are hold-style: each keyframe applies until the next.
type textNode struct {
	keys []struct {
		t   float64
		doc textDoc
	}
}

func (tn *textNode) docAt(f float64) *textDoc {
	if len(tn.keys) == 0 {
		return nil
	}
	best := 0
	for i := range tn.keys {
		if tn.keys[i].t <= f {
			best = i
		}
	}
	return &tn.keys[best].doc
}

// buildText compiles a text layer's document track.
func (b *builder) buildText(raw *rawTextData) *textNode {
	if raw == nil || raw.D == nil || len(raw.D.K) == 0 {
		return nil
	}
	if len(raw.A) > 0 && string(raw.A) != "[]" && string(raw.A) != "null" {
		b.anim.note("text animators")
	}
	tn := &textNode{}
	for _, k := range raw.D.K {
		v := k.S
		doc := textDoc{
			text:       v.Text,
			family:     v.Font,
			size:       v.Size,
			justify:    v.Justify,
			lineHeight: v.LineHeight,
		}
		if f, ok := b.fonts[v.Font]; ok {
			doc.family = f.Family
			doc.style = f.Style
		}
		if len(v.FillColor) >= 3 {
			doc.color = [3]float64{v.FillColor[0], v.FillColor[1], v.FillColor[2]}
		}
		tn.keys = append(tn.keys, struct {
			t   float64
			doc textDoc
		}{k.T, doc})
	}
	if len(tn.keys) == 0 {
		return nil
	}
	return tn
}

// renderText draws a text layer body.
func (r *renderer) renderText(dst *ebiten.Image, l *layerNode, lt float64, mat matrix, opacity float64, cs ebiten.ColorScale, blend ebiten.Blend) {
	if l.text == nil || r.anim == nil || r.anim.fontResolver == nil {
		return
	}
	doc := l.text.docAt(lt)
	if doc == nil || doc.text == "" || doc.size <= 0 {
		return
	}
	src := r.anim.fontResolver(doc.family, doc.style)
	if src == nil {
		return
	}
	face := &text.GoTextFace{Source: src, Size: doc.size}
	ascent := face.Metrics().HAscent
	lh := doc.lineHeight
	if lh <= 0 {
		lh = doc.size * 1.2
	}
	lines := splitTextLines(doc.text)
	for i, line := range lines {
		if line == "" {
			continue
		}
		w, _ := text.Measure(line, face, 0)
		var xoff float64
		switch doc.justify {
		case 1:
			xoff = -w
		case 2:
			xoff = -w / 2
		}
		local := identityMatrix.translate(xoff, float64(i)*lh-ascent)
		var op text.DrawOptions
		op.GeoM = mat.mul(local).toGeoM()
		a := opacity
		op.ColorScale.Scale(
			float32(doc.color[0]*a), float32(doc.color[1]*a), float32(doc.color[2]*a), float32(a))
		op.ColorScale.ScaleWithColorScale(cs)
		op.Blend = blend
		op.Filter = ebiten.FilterLinear
		text.Draw(dst, line, face, &op)
	}
}

// splitTextLines splits on the line separators AE exports (\r, \3) as well
// as plain \n.
func splitTextLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\u0003", "\n")
	return strings.Split(s, "\n")
}
