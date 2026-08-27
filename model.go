package lottie

import (
	"encoding/json"
	"fmt"
)

// Raw JSON model. Fields not needed for the supported subset are either
// absent (encoding/json ignores unknown keys) or kept as RawMessage so that
// unsupported features can be detected and reported.

type rawAnimation struct {
	Version string       `json:"v"`
	IP      float64      `json:"ip"`
	OP      float64      `json:"op"`
	FR      float64      `json:"fr"`
	W       int          `json:"w"`
	H       int          `json:"h"`
	Name    string       `json:"nm"`
	DDD     int          `json:"ddd"`
	Layers  []rawLayer   `json:"layers"`
	Assets  []rawAsset   `json:"assets"`
	Fonts   *rawFontList `json:"fonts"`
	Markers []rawMarker  `json:"markers"`
}

// rawMarker is one entry of the document-level "markers" array. dotLottie
// state machines name a marker to play only that part of an animation.
type rawMarker struct {
	Time     float64 `json:"tm"`
	Comment  string  `json:"cm"`
	Duration float64 `json:"dr"`
}

type rawFontList struct {
	List []rawFont `json:"list"`
}

type rawFont struct {
	Name   string `json:"fName"`
	Family string `json:"fFamily"`
	Style  string `json:"fStyle"`
}

// rawTextData is the "t" object of a text layer.
type rawTextData struct {
	D *rawTextDoc     `json:"d"`
	A json.RawMessage `json:"a"` // text animators; parsed by buildText
}

// rawTextAnimator is one entry of a text layer's animator list.
type rawTextAnimator struct {
	Name string            `json:"nm"`
	S    *rawTextSelector  `json:"s"`
	A    *rawTextAnimProps `json:"a"`
}

// rawTextSelector is an animator's range selector.
type rawTextSelector struct {
	B  int      `json:"b"`  // based on: 1 chars, 2 chars excl. spaces, 3 words, 4 lines
	Sh int      `json:"sh"` // shape: 1 square, 2 ramp up, 3 ramp down, 4 triangle, 5 round, 6 smooth
	M  int      `json:"m"`  // mode: 1 add
	R  int      `json:"r"`  // range units: 1 percent, 2 index
	A  *rawProp `json:"a"`  // max amount, percent
	S  *rawProp `json:"s"`  // range start
	E  *rawProp `json:"e"`  // range end
	O  *rawProp `json:"o"`  // range offset
}

// rawTextAnimProps is the set of properties an animator drives.
type rawTextAnimProps struct {
	P  *rawProp `json:"p"`  // position
	S  *rawProp `json:"s"`  // scale, percent
	R  *rawProp `json:"r"`  // rotation, degrees
	O  *rawProp `json:"o"`  // opacity, 0..100
	FC *rawProp `json:"fc"` // fill color
	T  *rawProp `json:"t"`  // tracking, thousandths of an em
}

type rawTextDoc struct {
	K []rawTextDocKey `json:"k"`
}

type rawTextDocKey struct {
	T float64         `json:"t"`
	S rawTextDocValue `json:"s"`
}

type rawTextDocValue struct {
	Text       string    `json:"t"`
	Font       string    `json:"f"`
	Size       float64   `json:"s"`
	FillColor  []float64 `json:"fc"`
	Justify    int       `json:"j"`
	LineHeight float64   `json:"lh"`
	Tracking   float64   `json:"tr"`
}

// rawAsset is either a precomposition (Layers != nil) or an image asset.
type rawAsset struct {
	ID     string     `json:"id"`
	Name   string     `json:"nm"`
	Layers []rawLayer `json:"layers"`
	// Image asset fields.
	W        float64         `json:"w"`
	H        float64         `json:"h"`
	Path     string          `json:"u"` // directory
	FileName string          `json:"p"` // file name or data: URI
	Embedded json.RawMessage `json:"e"` // 1 when p is a data URI
}

type rawLayer struct {
	Type        int             `json:"ty"`
	Name        string          `json:"nm"`
	Index       *int            `json:"ind"`
	Parent      *int            `json:"parent"`
	Hidden      bool            `json:"hd"`
	IP          float64         `json:"ip"`
	OP          float64         `json:"op"`
	ST          float64         `json:"st"`
	SR          *float64        `json:"sr"`
	AO          int             `json:"ao"`
	KS          rawTransform    `json:"ks"`
	Shapes      []rawShapeItem  `json:"shapes"`
	Masks       []rawMask       `json:"masksProperties"`
	Matte       *int            `json:"tt"`
	MatteParent *int            `json:"tp"` // explicit matte source layer index
	MatteSource int             `json:"td"` // 1: this layer is a matte source
	Blend       int             `json:"bm"`
	Effects     json.RawMessage `json:"ef"`
	DDD         int             `json:"ddd"`
	TM          *rawProp        `json:"tm"` // time remap (precomp), in seconds

	// Precomposition / image layer (ty: 0 / 2).
	RefID string  `json:"refId"`
	W     float64 `json:"w"` // precomp clip width
	H     float64 `json:"h"` // precomp clip height

	// Text layer (ty: 5).
	Text *rawTextData `json:"t"`

	// Solid layer (ty: 1).
	SolidW     float64 `json:"sw"`
	SolidH     float64 `json:"sh"`
	SolidColor string  `json:"sc"` // "#rrggbb"
}

// rawEffect is one entry of a layer's "ef" array.
type rawEffect struct {
	Type    int             `json:"ty"`
	Name    string          `json:"nm"`
	Enabled *int            `json:"en"`
	Values  []rawEffectItem `json:"ef"`
}

// rawEffectItem is one parameter of an effect, in schema order.
type rawEffectItem struct {
	Type int      `json:"ty"`
	Name string   `json:"nm"`
	V    *rawProp `json:"v"`
}

// rawMask is one entry of masksProperties.
type rawMask struct {
	Mode      string          `json:"mode"` // n none, a add, s subtract, i intersect, ...
	Inverted  bool            `json:"inv"`
	Points    *rawProp        `json:"pt"` // bezier shape
	Opacity   *rawProp        `json:"o"`  // 0..100
	Expansion json.RawMessage `json:"x"`
}

type rawTransform struct {
	A  *rawProp `json:"a"`
	P  *rawProp `json:"p"`
	S  *rawProp `json:"s"`
	R  *rawProp `json:"r"`
	O  *rawProp `json:"o"`
	SK *rawProp `json:"sk"`
	SA *rawProp `json:"sa"`
}

// rawShapeItem is the union of all shape item kinds, distinguished by Type.
type rawShapeItem struct {
	Type   string         `json:"ty"`
	Name   string         `json:"nm"`
	Hidden bool           `json:"hd"`
	Items  []rawShapeItem `json:"it"` // gr

	KS *rawProp `json:"ks"` // sh: bezier path

	P *rawProp `json:"p"` // rc, el: center / tr: position
	S *rawProp `json:"s"` // rc, el: size / tr: scale
	R *rawProp `json:"r"` // rc: roundness / tr: rotation / fl,gf: fill rule (int)

	C *rawProp `json:"c"` // fl, st: color / rp: copies
	O *rawProp `json:"o"` // fl, st, tr: opacity / tm, rp: offset
	W *rawProp `json:"w"` // st: width

	E  *rawProp          `json:"e"`  // gf, gs: end point / tm: end
	T  int               `json:"t"`  // gf, gs: 1 linear, 2 radial
	G  *rawGradientStops `json:"g"`  // gf, gs: color stops
	M  int               `json:"m"`  // tm: 1 simultaneously, 2 individually
	MM int               `json:"mm"` // mm: 1 merge, 2 add, 3 subtract, 4 intersect, 5 exclude
	H  *rawProp          `json:"h"`  // gf, gs: radial highlight length

	A  *rawProp `json:"a"`  // tr: anchor
	SK *rawProp `json:"sk"` // tr: skew
	SA *rawProp `json:"sa"` // tr: skew axis

	LineCap    int     `json:"lc"`
	LineJoin   int     `json:"lj"`
	MiterLimit float64 `json:"ml"`

	// "d" is polymorphic: a direction number on shapes, a dash array on
	// strokes. Decoded on demand via dashes().
	D json.RawMessage `json:"d"`

	// Polystar (sr).
	Points     *rawProp `json:"pt"`
	OuterR     *rawProp `json:"or"`
	InnerR     *rawProp `json:"ir"`
	OuterRound *rawProp `json:"os"`
	InnerRound *rawProp `json:"is"`
	StarType   int      `json:"sy"` // 1 star, 2 polygon

	// Repeater (rp). "tr" only appears as a field on rp items; group
	// transforms are separate "tr"-typed items, not fields.
	RepeaterT *rawRepeaterTransform `json:"tr"`
}

// rawRepeaterTransform is a repeater's per-copy transform, which adds
// start/end opacity to the usual transform fields.
type rawRepeaterTransform struct {
	A  *rawProp `json:"a"`
	P  *rawProp `json:"p"`
	S  *rawProp `json:"s"`
	R  *rawProp `json:"r"`
	SO *rawProp `json:"so"` // opacity of copy 0, 0..100
	EO *rawProp `json:"eo"` // opacity of the last copy, 0..100
}

// rawDashElem is one element of a stroke's dash array.
type rawDashElem struct {
	Name  string   `json:"n"` // "d" dash, "g" gap, "o" offset
	Value *rawProp `json:"v"`
}

// dashes decodes the "d" field as a dash array; nil when it is a plain
// direction number or absent.
func (it *rawShapeItem) dashes() []rawDashElem {
	if len(it.D) == 0 {
		return nil
	}
	var elems []rawDashElem
	if err := json.Unmarshal(it.D, &elems); err != nil {
		return nil
	}
	return elems
}

// rawGradientStops is the "g" object of gradient fills and strokes.
type rawGradientStops struct {
	Count int      `json:"p"`
	K     *rawProp `json:"k"`
}

// rawProp is an animatable property: {a, k, ...} where k is polymorphic.
// Some exporters write bare values (e.g. a fill rule "r": 1) where others
// write a property object, so both forms are accepted.
type rawProp struct {
	K json.RawMessage `json:"k"`
	X json.RawMessage `json:"x"` // expression (string) or split-position sub-prop
	Y json.RawMessage `json:"y"` // split-position sub-prop
	S json.RawMessage `json:"s"` // true when position is split into x/y
}

func (p *rawProp) UnmarshalJSON(data []byte) error {
	for _, c := range data {
		switch c {
		case ' ', '\t', '\n', '\r':
			continue
		case '{':
			type alias rawProp
			return json.Unmarshal(data, (*alias)(p))
		default:
			// Bare value: treat it as the static k.
			p.K = json.RawMessage(data)
			return nil
		}
	}
	return nil
}

// splitPosition reports whether the property is a split x/y position and
// returns the sub-properties.
func (p *rawProp) splitPosition() (x, y *rawProp, ok bool) {
	var split bool
	if len(p.S) > 0 && json.Unmarshal(p.S, &split) == nil && split {
		var px, py rawProp
		if len(p.X) > 0 && json.Unmarshal(p.X, &px) == nil &&
			len(p.Y) > 0 && json.Unmarshal(p.Y, &py) == nil {
			return &px, &py, true
		}
	}
	return nil, nil, false
}

// hasExpression reports whether the property carries an expression string.
func (p *rawProp) hasExpression() bool {
	if len(p.X) == 0 {
		return false
	}
	var s string
	return json.Unmarshal(p.X, &s) == nil && s != ""
}

type rawKeyframe struct {
	T  float64         `json:"t"`
	S  json.RawMessage `json:"s"`
	E  json.RawMessage `json:"e"` // legacy end value
	O  json.RawMessage `json:"o"`
	I  json.RawMessage `json:"i"`
	H  int             `json:"h"`
	TI []float64       `json:"ti"`
	TO []float64       `json:"to"`
}

type rawEase struct {
	X json.RawMessage `json:"x"`
	Y json.RawMessage `json:"y"`
}

type rawShapeValue struct {
	Closed bool         `json:"c"`
	V      [][2]float64 `json:"v"`
	I      [][2]float64 `json:"i"`
	O      [][2]float64 `json:"o"`
}

// numbers decodes a JSON value that is either a number or an array of
// numbers into a slice.
func numbers(raw json.RawMessage) ([]float64, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty value")
	}
	var arr []float64
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		return []float64{n}, nil
	}
	return nil, fmt.Errorf("value is neither number nor number array: %s", truncateJSON(raw))
}

// easeComponent extracts the idx-th component (or the sole value) from an
// easing control coordinate.
func easeComponent(raw json.RawMessage, def float64) float64 {
	vals, err := numbers(raw)
	if err != nil || len(vals) == 0 {
		return def
	}
	return vals[0]
}

func parseEasing(k *rawKeyframe) easing {
	if len(k.O) == 0 && len(k.I) == 0 {
		return linearEasing
	}
	var o, in rawEase
	if len(k.O) > 0 {
		if err := json.Unmarshal(k.O, &o); err != nil {
			return linearEasing
		}
	}
	if len(k.I) > 0 {
		if err := json.Unmarshal(k.I, &in); err != nil {
			return linearEasing
		}
	}
	e := easing{
		outX: clamp01(easeComponent(o.X, 0)),
		outY: easeComponent(o.Y, 0),
		inX:  clamp01(easeComponent(in.X, 1)),
		inY:  easeComponent(in.Y, 1),
	}
	if e.outX == 0 && e.outY == 0 && e.inX == 1 && e.inY == 1 {
		e.linear = true
	}
	return e
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func truncateJSON(raw json.RawMessage) string {
	s := string(raw)
	if len(s) > 80 {
		s = s[:80] + "..."
	}
	return s
}

// isKeyframeArray reports whether k holds an array of keyframe objects
// (as opposed to a static number array).
func isKeyframeArray(k json.RawMessage) bool {
	var probe []json.RawMessage
	if err := json.Unmarshal(k, &probe); err != nil || len(probe) == 0 {
		return false
	}
	for _, c := range probe[0] {
		switch c {
		case ' ', '\t', '\n', '\r':
			continue
		}
		return c == '{'
	}
	return false
}

// parseVectorProp parses a property into a vectorTrack. dim hints are not
// required; the track keeps whatever dimensionality the file provides.
func parseVectorProp(p *rawProp, def []float64) (*vectorTrack, error) {
	if p == nil || len(p.K) == 0 {
		return &vectorTrack{static: def}, nil
	}
	if !isKeyframeArray(p.K) {
		vals, err := numbers(p.K)
		if err != nil {
			return nil, err
		}
		return &vectorTrack{static: vals}, nil
	}
	var raws []rawKeyframe
	if err := json.Unmarshal(p.K, &raws); err != nil {
		return nil, fmt.Errorf("keyframes: %w", err)
	}
	keys := make([]vectorKey, 0, len(raws))
	for idx, rk := range raws {
		key := vectorKey{
			t:    rk.T,
			hold: rk.H == 1,
			ease: parseEasing(&rk),
			ti:   rk.TI,
			to:   rk.TO,
		}
		if len(rk.S) > 0 {
			vals, err := numbers(rk.S)
			if err != nil {
				return nil, fmt.Errorf("keyframe %d: %w", idx, err)
			}
			key.value = vals
		} else if idx > 0 {
			// Trailing {t} keyframe: reuse previous end value.
			prev := &keys[len(keys)-1]
			if v := prev.legacyEnd; v != nil {
				key.value = v
			} else {
				key.value = prev.value
			}
		}
		// Legacy format: explicit end value on the previous keyframe.
		if len(rk.E) > 0 {
			if vals, err := numbers(rk.E); err == nil {
				key.legacyEnd = vals
			}
		}
		keys = append(keys, key)
	}
	// Resolve legacy end values: segment i interpolates keys[i].value ->
	// keys[i].legacyEnd (if set) else keys[i+1].value. Normalize by
	// rewriting the next key's value when it is missing.
	for i := 0; i < len(keys)-1; i++ {
		if keys[i+1].value == nil && keys[i].legacyEnd != nil {
			keys[i+1].value = keys[i].legacyEnd
		}
	}
	if len(keys) > 0 && keys[len(keys)-1].value == nil && len(keys) > 1 {
		keys[len(keys)-1].value = keys[len(keys)-2].value
	}
	if len(keys) == 0 {
		return &vectorTrack{static: def}, nil
	}
	if len(keys) == 1 {
		return &vectorTrack{static: keys[0].value}, nil
	}
	return &vectorTrack{keys: keys}, nil
}

// parseShapeProp parses a bezier path property into a shapeTrack.
func parseShapeProp(p *rawProp) (*shapeTrack, error) {
	if p == nil || len(p.K) == 0 {
		return nil, fmt.Errorf("missing shape property")
	}
	if !isKeyframeArray(p.K) {
		var sv rawShapeValue
		if err := json.Unmarshal(p.K, &sv); err != nil {
			return nil, fmt.Errorf("shape value: %w", err)
		}
		return &shapeTrack{static: bezierShape{Closed: sv.Closed, V: sv.V, I: sv.I, O: sv.O}}, nil
	}
	var raws []rawKeyframe
	if err := json.Unmarshal(p.K, &raws); err != nil {
		return nil, fmt.Errorf("shape keyframes: %w", err)
	}
	keys := make([]shapeKey, 0, len(raws))
	for idx, rk := range raws {
		key := shapeKey{t: rk.T, hold: rk.H == 1, ease: parseEasing(&rk)}
		if len(rk.S) > 0 {
			var svs []rawShapeValue
			if err := json.Unmarshal(rk.S, &svs); err != nil {
				// Some exporters write s as a bare object.
				var sv rawShapeValue
				if err2 := json.Unmarshal(rk.S, &sv); err2 != nil {
					return nil, fmt.Errorf("shape keyframe %d: %w", idx, err)
				}
				svs = []rawShapeValue{sv}
			}
			if len(svs) > 0 {
				sv := svs[0]
				key.value = bezierShape{Closed: sv.Closed, V: sv.V, I: sv.I, O: sv.O}
			}
		} else if idx > 0 {
			key.value = keys[len(keys)-1].value
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("empty shape keyframes")
	}
	if len(keys) == 1 {
		return &shapeTrack{static: keys[0].value}, nil
	}
	return &shapeTrack{keys: keys}, nil
}
