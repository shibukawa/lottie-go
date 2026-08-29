package lottie

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"unicode"

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
	a.generation++ // invalidate idle snapshots that may include text
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
	tracking   float64 // thousandths of an em
}

// textNode is the compiled form of a text layer's document keyframes and
// animators. Document changes are hold-style: each keyframe applies until
// the next.
type textNode struct {
	keys []struct {
		t   float64
		doc textDoc
	}
	animators []textAnimator
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

// textAnimator is one compiled animator: a range selector plus the
// properties it drives on the selected characters.
type textAnimator struct {
	basedOn int // 1 chars, 2 chars excl. spaces, 3 words, 4 lines
	shape   int // 1 square, 2 ramp up, 3 ramp down, 4 triangle, 5 round, 6 smooth
	units   int // 1 percent, 2 index
	amount  *vectorTrack
	start   *vectorTrack
	end     *vectorTrack
	offset  *vectorTrack

	pos      *vectorTrack
	scale    *vectorTrack
	rotation *vectorTrack
	opacity  *vectorTrack
	fill     *vectorTrack
	tracking *vectorTrack
}

// factorAt returns the animator's influence on unit i of n at layer-local
// frame f. The selector shape maps the unit's position within the start/end
// range to 0..1, then the amount scales it.
func (ta *textAnimator) factorAt(f float64, i, n int) float64 {
	if n <= 0 {
		return 0
	}
	lo := 0.0
	if ta.start != nil {
		lo = ta.start.scalarAt(f, 0)
	}
	hi := 100.0
	if ta.end != nil {
		hi = ta.end.scalarAt(f, 100)
	}
	off := 0.0
	if ta.offset != nil {
		off = ta.offset.scalarAt(f, 0)
	}
	lo += off
	hi += off
	if ta.units == 2 {
		// Index units count characters rather than percent.
		lo = lo / float64(n) * 100
		hi = hi / float64(n) * 100
	}
	if hi < lo {
		lo, hi = hi, lo
	}
	pos := (float64(i) + 0.5) / float64(n) * 100
	span := hi - lo
	u := 0.0
	if span > 0 {
		u = clamp01((pos - lo) / span)
	} else if pos >= lo {
		u = 1
	}
	var v float64
	switch ta.shape {
	case 2: // ramp up
		v = u
	case 3: // ramp down
		v = 1 - u
	case 4: // triangle
		v = 1 - math.Abs(u*2-1)
	case 5: // round
		w := u*2 - 1
		v = math.Sqrt(math.Max(0, 1-w*w))
	case 6: // smooth
		v = (1 - math.Cos(u*2*math.Pi)) / 2
	default: // square
		if pos >= lo && pos < hi {
			v = 1
		}
	}
	if ta.amount != nil {
		v *= ta.amount.scalarAt(f, 100) / 100
	}
	return v
}

// buildText compiles a text layer's document track and animators.
func (b *builder) buildText(raw *rawTextData) *textNode {
	if raw == nil || raw.D == nil || len(raw.D.K) == 0 {
		return nil
	}
	tn := &textNode{}
	tn.animators = b.buildTextAnimators(raw.A)
	for _, k := range raw.D.K {
		v := k.S
		doc := textDoc{
			text:       v.Text,
			family:     v.Font,
			size:       v.Size,
			justify:    v.Justify,
			lineHeight: v.LineHeight,
			tracking:   v.Tracking,
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

func (b *builder) buildTextAnimators(raw json.RawMessage) []textAnimator {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "[]" {
		return nil
	}
	var raws []rawTextAnimator
	if err := json.Unmarshal(raw, &raws); err != nil {
		b.anim.note("unparsable text animators")
		return nil
	}
	var out []textAnimator
	for i := range raws {
		ra := &raws[i]
		if ra.A == nil {
			continue
		}
		ta := textAnimator{basedOn: 1, shape: 1, units: 1}
		if s := ra.S; s != nil {
			if s.B != 0 {
				ta.basedOn = s.B
			}
			if s.Sh != 0 {
				ta.shape = s.Sh
			}
			if s.R != 0 {
				ta.units = s.R
			}
			if s.M > 1 {
				b.anim.note(fmt.Sprintf("text selector mode %d", s.M))
			}
			track := func(p *rawProp, what string) *vectorTrack {
				if p == nil {
					return nil
				}
				return b.vectorProp(p, what, nil)
			}
			ta.amount = track(s.A, "text selector amount")
			ta.start = track(s.S, "text selector start")
			ta.end = track(s.E, "text selector end")
			ta.offset = track(s.O, "text selector offset")
		}
		track := func(p *rawProp, what string) *vectorTrack {
			if p == nil {
				return nil
			}
			return b.vectorProp(p, what, nil)
		}
		ta.pos = track(ra.A.P, "text animator position")
		ta.scale = track(ra.A.S, "text animator scale")
		ta.rotation = track(ra.A.R, "text animator rotation")
		ta.opacity = track(ra.A.O, "text animator opacity")
		ta.fill = track(ra.A.FC, "text animator fill color")
		ta.tracking = track(ra.A.T, "text animator tracking")
		if ta.pos == nil && ta.scale == nil && ta.rotation == nil &&
			ta.opacity == nil && ta.fill == nil && ta.tracking == nil {
			continue
		}
		out = append(out, ta)
	}
	return out
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
	lh := doc.lineHeight
	if lh <= 0 {
		lh = doc.size * 1.2
	}
	lines := splitTextLines(doc.text)
	if len(l.text.animators) > 0 || doc.tracking != 0 {
		r.renderGlyphText(dst, l.text, doc, face, lines, lh, lt, mat, opacity, cs, blend)
		return
	}
	ascent := face.Metrics().HAscent
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

// glyphUnit carries the selector indices of one glyph: which character,
// non-space character, word, and line it belongs to.
type glyphUnit struct {
	char, charNS, word, line int
	space                    bool
}

// renderGlyphText draws the text one glyph at a time so animators and
// tracking can position and style each character on its own.
func (r *renderer) renderGlyphText(dst *ebiten.Image, tn *textNode, doc *textDoc, face *text.GoTextFace, lines []string, lh, lt float64, mat matrix, opacity float64, cs ebiten.ColorScale, blend ebiten.Blend) {
	// Count the selector units of the whole text block first: ranges span
	// every line, not each line separately.
	var totals glyphUnit
	counts := make([][]glyphUnit, len(lines))
	for li, line := range lines {
		runes := []rune(line)
		units := make([]glyphUnit, len(runes))
		inWord := false
		for ri, rn := range runes {
			space := unicode.IsSpace(rn)
			if !space && !inWord {
				totals.word++
			}
			inWord = !space
			units[ri] = glyphUnit{
				char:   totals.char,
				charNS: totals.charNS,
				word:   totals.word - 1,
				line:   li,
				space:  space,
			}
			totals.char++
			if !space {
				totals.charNS++
			}
		}
		counts[li] = units
	}
	totals.line = len(lines)

	baseTracking := doc.tracking / 1000 * doc.size
	// AppendGlyphs positions the first baseline at +HAscent, so pull each
	// line up by the ascent — the same correction the plain-text path
	// applies — or the two paths disagree by a whole ascent.
	ascent := face.Metrics().HAscent
	var glyphs []text.Glyph
	for li, line := range lines {
		if line == "" {
			continue
		}
		units := counts[li]
		glyphs = text.AppendGlyphs(glyphs[:0], line, face, nil)

		// First pass: per-glyph factors decide tracking, which shifts both
		// the glyph positions and the line width used for justification.
		type glyphState struct {
			shift  float64 // accumulated tracking before this glyph
			track  float64 // tracking this glyph adds after itself
			factor []float64
		}
		states := make([]glyphState, len(glyphs))
		shift := 0.0
		for gi := range glyphs {
			g := &glyphs[gi]
			u := unitFor(units, line, g.StartIndexInBytes)
			states[gi].shift = shift
			track := baseTracking
			factors := make([]float64, len(tn.animators))
			for ai := range tn.animators {
				ta := &tn.animators[ai]
				f := ta.factorAt(lt, unitIndex(ta.basedOn, u), unitTotal(ta.basedOn, &totals))
				factors[ai] = f
				if ta.tracking != nil {
					track += ta.tracking.scalarAt(lt, 0) / 1000 * doc.size * f
				}
			}
			states[gi].factor = factors
			states[gi].track = track
			shift += track
		}
		lineAdvance := text.Advance(line, face)
		w, _ := text.Measure(line, face, 0)
		if len(glyphs) > 0 {
			w += shift - states[len(glyphs)-1].track
		}
		var xoff float64
		switch doc.justify {
		case 1:
			xoff = -w
		case 2:
			xoff = -w / 2
		}
		local := mat.mul(identityMatrix.translate(xoff, float64(li)*lh-ascent))

		for gi := range glyphs {
			g := &glyphs[gi]
			if g.Image == nil {
				continue
			}
			dx, dy, rot := states[gi].shift, 0.0, 0.0
			sx, sy := 1.0, 1.0
			alpha := opacity
			col := doc.color
			for ai := range tn.animators {
				ta := &tn.animators[ai]
				f := states[gi].factor[ai]
				if f == 0 {
					continue
				}
				if ta.pos != nil {
					p := ta.pos.at(lt, nil)
					dx += at(p, 0) * f
					dy += at(p, 1) * f
				}
				if ta.rotation != nil {
					rot += ta.rotation.scalarAt(lt, 0) * f
				}
				if ta.scale != nil {
					s := ta.scale.at(lt, nil)
					ssx := at(s, 0) / 100
					ssy := ssx
					if len(s) > 1 {
						ssy = s[1] / 100
					}
					sx *= 1 + (ssx-1)*f
					sy *= 1 + (ssy-1)*f
				}
				if ta.opacity != nil {
					o := clamp01(ta.opacity.scalarAt(lt, 100) / 100)
					alpha *= 1 + (o-1)*clamp01(f)
				}
				if ta.fill != nil {
					c := ta.fill.at(lt, nil)
					ff := clamp01(f)
					for d := 0; d < 3; d++ {
						col[d] += (at(c, d) - col[d]) * ff
					}
				}
			}
			gm := identityMatrix.translate(dx, dy)
			if rot != 0 || sx != 1 || sy != 1 {
				// Rotate and scale around the glyph's baseline center. The
				// advance is the gap to the next glyph's origin, or to the
				// line's end for the last one.
				adv := lineAdvance - g.OriginX
				if gi+1 < len(glyphs) {
					adv = glyphs[gi+1].OriginX - g.OriginX
				}
				ax := g.OriginX + g.OriginOffsetX + adv/2
				ay := g.OriginY + g.OriginOffsetY
				gm = gm.translate(ax, ay)
				if rot != 0 {
					gm = gm.rotate(rot * math.Pi / 180)
				}
				if sx != 1 || sy != 1 {
					gm = gm.scale(sx, sy)
				}
				gm = gm.translate(-ax, -ay)
			}
			var op ebiten.DrawImageOptions
			op.GeoM = local.mul(gm).translate(g.X, g.Y).toGeoM()
			op.ColorScale.Scale(
				float32(col[0]*alpha), float32(col[1]*alpha), float32(col[2]*alpha), float32(alpha))
			op.ColorScale.ScaleWithColorScale(cs)
			op.Blend = blend
			op.Filter = ebiten.FilterLinear
			dst.DrawImage(g.Image, &op)
		}
	}
}

// unitFor maps a glyph's byte offset to its rune's selector indices.
func unitFor(units []glyphUnit, line string, byteIdx int) glyphUnit {
	ri := 0
	for i := range line {
		if i >= byteIdx {
			break
		}
		ri++
	}
	if ri >= len(units) {
		if len(units) == 0 {
			return glyphUnit{}
		}
		ri = len(units) - 1
	}
	return units[ri]
}

func unitIndex(basedOn int, u glyphUnit) int {
	switch basedOn {
	case 2:
		return u.charNS
	case 3:
		return u.word
	case 4:
		return u.line
	}
	return u.char
}

func unitTotal(basedOn int, totals *glyphUnit) int {
	switch basedOn {
	case 2:
		return totals.charNS
	case 3:
		return totals.word
	case 4:
		return totals.line
	}
	return totals.char
}

// splitTextLines splits on the line separators AE exports (\r, \3) as well
// as plain \n.
func splitTextLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\u0003", "\n")
	return strings.Split(s, "\n")
}
