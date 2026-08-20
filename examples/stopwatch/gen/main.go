// Command gen writes the stopwatch's Lottie assets: ten cartoon digit
// transitions (digit-N.json shows N at frame 0 and plays an exaggerated
// bounce morph to N+1), a squishy blinking colon, and push buttons that
// burst particles when pressed.
//
// Text uses the "Luckiest Guy" font (Apache-2.0); the app supplies it via
// SetFontResolver.
//
// Regenerate with:
//
//	go run ./examples/stopwatch/gen
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
)

type obj = map[string]any

func static(v any) obj { return obj{"a": 0, "k": v} }

func keys(frames ...obj) obj { return obj{"a": 1, "k": frames} }

// key builds a keyframe with a gentle default ease.
func key(t float64, v ...float64) obj {
	return keyE(t, 0.4, 0, 0.6, 1, v...)
}

// keyE builds a keyframe with explicit cubic-bezier easing handles.
func keyE(t, ox, oy, ix, iy float64, v ...float64) obj {
	return obj{
		"t": t, "s": v,
		"o": obj{"x": ox, "y": oy}, "i": obj{"x": ix, "y": iy},
	}
}

// keyHold holds its value until the next keyframe.
func keyHold(t float64, v ...float64) obj {
	return obj{"t": t, "s": v, "h": 1}
}

func end(t float64, v ...float64) obj { return obj{"t": t, "s": v} }

func animation(name string, w, h, op float64, layers []obj) obj {
	a := obj{
		"v": "5.9.0", "nm": name, "fr": 60.0,
		"ip": 0.0, "op": op, "w": w, "h": h,
		"layers": layers,
	}
	a["fonts"] = obj{"list": []obj{
		{"fName": "LuckiestGuy-Regular", "fFamily": "Luckiest Guy", "fStyle": "Regular"},
	}}
	return a
}

// Palette: Simpsons-style yellow on deep sky blue.
var (
	yellow = []float64{1, 0.851, 0.059}   // #FFD90F
	navy   = []float64{0.078, 0.11, 0.29} // dark shadow ink
	white  = []float64{1, 1, 1}
	sky    = []float64{0.31, 0.62, 0.94} // particle accent
	orange = []float64{1, 0.58, 0.17}    // particle accent
)

const (
	digitW  = 160.0
	digitH  = 220.0
	baseY   = 185.0 // text baseline at rest
	textSz  = 170.0
	digitOP = 22.0 // 0.37s transition
)

// textLayer builds a text layer with transform keyframes. Pass nil tracks
// for static defaults.
func textLayer(name string, ind int, op float64, docText string, size float64, fill []float64, pos, scale, rot, opacity obj) obj {
	if pos == nil {
		pos = static([]float64{digitW / 2, baseY})
	}
	if scale == nil {
		scale = static([]float64{100, 100})
	}
	if rot == nil {
		rot = static(0.0)
	}
	if opacity == nil {
		opacity = static(100.0)
	}
	return obj{
		"ty": 5, "nm": name, "ind": ind,
		"ip": 0.0, "op": op, "st": 0.0,
		"ks": obj{
			"a": static([]float64{0, 0}),
			"p": pos, "s": scale, "r": rot, "o": opacity,
		},
		"t": obj{"d": obj{"k": []obj{{
			"t": 0,
			"s": obj{
				"t": docText, "f": "LuckiestGuy-Regular", "s": size,
				"fc": fill, "j": 2, "lh": size * 1.2,
			},
		}}}},
	}
}

// digitAnimation: digit N rests at frame 0; playing it squashes N, launches
// it upward, and drops N+1 in from above with an overshoot bounce.
func digitAnimation(n int) obj {
	cur := fmt.Sprintf("%d", n)
	next := fmt.Sprintf("%d", (n+1)%10)
	cx := digitW / 2

	// Outgoing digit: anticipation squash, then launch up and away.
	outPos := keys(
		key(0, cx, baseY),
		keyE(4, 0.5, 0, 0.8, 0.2, cx, baseY+14),
		end(12, cx, -70),
	)
	outScale := keys(
		key(0, 100, 100),
		key(4, 128, 62),  // squash down before launch
		end(12, 60, 130), // stretch on the way out
	)
	outRot := keys(key(4, 0), end(12, 24))
	outOpacity := keys(key(8, 100), end(12, 0))

	// Incoming digit: falls from above, overshoots, squashes, settles.
	inPos := keys(
		keyHold(0, cx, -80),
		keyE(5, 0.7, 0, 0.9, 0.4, cx, -80),
		keyE(13, 0.3, 0.6, 0.3, 1, cx, baseY+26), // slam past the baseline
		keyE(17, 0.4, 0, 0.6, 1, cx, baseY-10),   // rebound
		end(digitOP, cx, baseY),
	)
	inScale := keys(
		keyHold(0, 100, 100),
		key(5, 62, 138),  // stretched while falling
		key(13, 142, 58), // big squash on impact
		key(17, 90, 112),
		end(digitOP, 100, 100),
	)
	inRot := keys(key(5, -14), key(13, 7), end(digitOP, 0))
	inOpacity := keys(keyHold(0, 0), end(5, 100))

	// Shadows repeat the same motion offset down-right.
	off := func(track obj, dx, dy float64) obj { return offsetPos(track, dx, dy) }

	layers := []obj{
		textLayer("in", 1, digitOP, next, textSz, yellow, inPos, inScale, inRot, inOpacity),
		textLayer("in-shadow", 2, digitOP, next, textSz, navy, off(inPos, 6, 7), inScale, inRot, inOpacity),
		textLayer("out", 3, digitOP, cur, textSz, yellow, outPos, outScale, outRot, outOpacity),
		textLayer("out-shadow", 4, digitOP, cur, textSz, navy, off(outPos, 6, 7), outScale, outRot, outOpacity),
	}
	return animation("digit-"+cur, digitW, digitH, digitOP, layers)
}

// offsetPos clones a position track shifting every keyframe by (dx, dy).
func offsetPos(track obj, dx, dy float64) obj {
	if track["a"] == 0 {
		v := track["k"].([]float64)
		return static([]float64{v[0] + dx, v[1] + dy})
	}
	var out []obj
	for _, kf := range track["k"].([]obj) {
		c := obj{}
		for k, v := range kf {
			c[k] = v
		}
		s := kf["s"].([]float64)
		c["s"] = []float64{s[0] + dx, s[1] + dy}
		out = append(out, c)
	}
	return obj{"a": 1, "k": out}
}

func groupTransform() obj {
	return obj{
		"ty": "tr",
		"a":  static([]float64{0, 0}),
		"p":  static([]float64{0, 0}),
		"s":  static([]float64{100, 100}),
		"r":  static(0.0),
		"o":  static(100.0),
	}
}

func shapeLayer(name string, ind int, op float64, shapes []obj, ks obj) obj {
	if ks == nil {
		ks = obj{}
	}
	return obj{
		"ty": 4, "nm": name, "ind": ind,
		"ip": 0.0, "op": op, "st": 0.0,
		"ks": ks, "shapes": shapes,
	}
}

func circleGroup(cx, cy, d float64, fill []float64, opacity, transform obj) obj {
	if transform == nil {
		transform = groupTransform()
	}
	if opacity == nil {
		opacity = static(100.0)
	}
	return obj{
		"ty": "gr", "nm": "dot",
		"it": []obj{
			{"ty": "el", "p": static([]float64{cx, cy}), "s": static([]float64{d, d})},
			{"ty": "fl", "c": static(fill), "o": opacity, "r": 1},
			transform,
		},
	}
}

// colonAnimation: two dots doing a squishy pulse once per second.
func colonAnimation() obj {
	pulse := func(cx, cy float64) obj {
		return obj{
			"ty": "tr",
			"a":  static([]float64{cx, cy}),
			"p":  static([]float64{cx, cy}),
			"s": keys(
				key(0, 100, 100),
				key(24, 128, 66),
				key(34, 84, 116),
				end(60, 100, 100),
			),
			"r": static(0.0),
			"o": keys(key(0, 100), key(28, 45), end(60, 100)),
		}
	}
	var shapes []obj
	for _, cy := range []float64{95, 155} {
		shapes = append(shapes,
			circleGroup(36, cy+7, 26, navy, nil, pulse(36, cy+7)),
			circleGroup(30, cy, 26, yellow, nil, pulse(30, cy)),
		)
	}
	return animation("colon", 60, digitH, 60, []obj{shapeLayer("colon", 1, 60, shapes, nil)})
}

// jitter is a cheap deterministic pseudo-random in [0,1) per particle and
// channel, so regenerated assets stay stable.
func jitter(i, ch int) float64 {
	x := float64(i*374761393 + ch*668265263)
	s := math.Sin(x) * 43758.5453
	return s - math.Floor(s)
}

// particle emits one five-point star: it appears on press, shoots outward
// in its own direction with a slight arc of gravity, spins, shrinks, and
// fades out. Angles, distances, sizes, and launch delays are jittered so
// the burst reads as a shower flying every which way.
func particle(i, total int, fill []float64, w, h, op float64) obj {
	ang := 2*math.Pi*float64(i)/float64(total) + (jitter(i, 0)-0.5)*0.7
	dist := 100 + jitter(i, 1)*80 // 100..180
	cx, cy := w/2, h/2
	tx := cx + math.Cos(ang)*dist
	ty := cy + math.Sin(ang)*dist*0.85
	size := 16 + jitter(i, 2)*18 // 16..34
	delay := math.Floor(jitter(i, 3) * 4)
	spin := 160 + jitter(i, 4)*220
	if i%2 == 0 {
		spin = -spin
	}
	fall := 10 + jitter(i, 5)*16 // gravity droop at the end of flight

	star := obj{
		"ty": "sr", "sy": 1,
		"p":  static([]float64{0, 0}),
		"pt": static(5.0),
		"r":  static(jitter(i, 6) * 72),
		"or": static(size / 2),
		"ir": static(size / 5),
		"os": static(0.0),
		"is": static(0.0),
	}
	mid := op * 0.55
	return obj{
		"ty": "gr", "nm": fmt.Sprintf("particle-%d", i),
		"it": []obj{
			star,
			obj{"ty": "fl", "c": static(fill), "o": static(100.0), "r": 1},
			obj{
				"ty": "tr",
				"a":  static([]float64{0, 0}),
				"p": keys(
					keyHold(0, cx, cy),
					keyE(1+delay, 0.15, 0.75, 0.4, 1, cx, cy),
					keyE(mid, 0.3, 0.3, 0.5, 1,
						cx+(tx-cx)*0.75, cy+(ty-cy)*0.75),
					end(op, tx, ty+fall),
				),
				"s": keys(key(1+delay, 115, 115), end(op, 20, 20)),
				"r": keys(key(1+delay, 0), end(op, spin)),
				"o": keys(
					keyHold(0, 0),
					key(1+delay, 100),
					key(op-7, 100),
					end(op, 0),
				),
			},
		},
	}
}

// buttonAnimation: a pill with a label; playing it runs a press pop, a
// white flash, and a particle burst.
func buttonAnimation(label string, base []float64) obj {
	const w, h, op = 240, 90, 24
	press := obj{
		"ty": 3, "nm": "press", "ind": 3,
		"ip": 0.0, "op": float64(op), "st": 0.0,
		"ks": obj{
			"a": static([]float64{w / 2, h / 2}),
			"p": static([]float64{w / 2, h / 2}),
			"s": keys(
				key(0, 100, 100),
				key(4, 84, 84),
				keyE(10, 0.3, 0.4, 0.5, 1, 108, 108),
				end(16, 100, 100),
			),
		},
	}
	pill := obj{
		"ty": "gr", "nm": "pill",
		"it": []obj{
			{
				"ty": "rc",
				"p":  static([]float64{w / 2, h / 2}),
				"s":  static([]float64{w - 16, h - 16}),
				"r":  static((h - 16.0) / 2),
			},
			{"ty": "fl", "c": static(base), "o": static(100.0), "r": 1},
			groupTransform(),
		},
	}
	flash := obj{
		"ty": "gr", "nm": "flash",
		"it": []obj{
			{
				"ty": "rc",
				"p":  static([]float64{w / 2, h / 2}),
				"s":  static([]float64{w - 16, h - 16}),
				"r":  static((h - 16.0) / 2),
			},
			{
				"ty": "fl", "c": static(white),
				"o": keys(keyHold(0, 0), key(2, 70), end(9, 0)),
				"r": 1,
			},
			groupTransform(),
		},
	}
	var burst []obj
	colors := [][]float64{yellow, white, sky, orange}
	for i := 0; i < 18; i++ {
		burst = append(burst, particle(i, 18, colors[i%4], w, h, op))
	}
	particles := shapeLayer("particles", 4, op, burst, nil)
	body := shapeLayer("body", 1, op, []obj{flash, pill}, nil)
	body["parent"] = 3
	label9 := textLayer("label", 2, op, label, 36, white,
		static([]float64{w / 2, h/2 + 13}), nil, nil, nil)
	label9["parent"] = 3
	a := animation("button-"+label, w, h, op, []obj{particles, label9, body, press})
	return a
}

func main() {
	outDir := "examples/stopwatch/assets"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatal(err)
	}
	write := func(name string, a obj) {
		data, err := json.MarshalIndent(a, "", " ")
		if err != nil {
			log.Fatal(err)
		}
		path := filepath.Join(outDir, name)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Println("wrote", path)
	}
	for n := 0; n < 10; n++ {
		write(fmt.Sprintf("digit-%d.json", n), digitAnimation(n))
	}
	write("colon.json", colonAnimation())
	write("button-START.json", buttonAnimation("START", []float64{0.15, 0.68, 0.42}))
	write("button-STOP.json", buttonAnimation("STOP", []float64{0.85, 0.19, 0.21}))
	write("button-RESET.json", buttonAnimation("RESET", []float64{0.33, 0.36, 0.47}))
	os.Remove(filepath.Join(outDir, "dot.json"))
}
