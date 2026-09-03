package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"math"
	"os"
	"path/filepath"
	"strings"

	lottie "github.com/shibukawa/lottie-go"
	lottietexture "github.com/shibukawa/lottie-go/plugin/texture"
)

func drawImage(dst draw.Image, r image.Rectangle, src image.Image) {
	draw.Draw(dst, r, src, src.Bounds().Min, draw.Over)
}

// runRig builds the character bundle (concept:uv-morph-rig).
func runRig(args []string) error {
	fs := flag.NewFlagSet("rig", flag.ContinueOnError)
	presets := fs.String("presets", "", "directory holding the base presets")
	out := fs.String("o", "", "bundle to write (default work/<name>.lottie)")
	raster := fs.Bool("raster", false, "write the raster cutout rig instead of textured paths")
	if err := fs.Parse(args); err != nil {
		return err
	}
	work := "."
	if fs.NArg() > 0 {
		work = fs.Arg(0)
	}
	spec, err := loadSpec(work)
	if err != nil {
		return err
	}
	if *raster {
		spec.Raster = true
	}
	basePath, err := resolveBase(spec.Base, *presets)
	if err != nil {
		return err
	}
	r, err := loadRig(basePath)
	if err != nil {
		return err
	}
	atts, err := expandAttachments(spec, r)
	if err != nil {
		return err
	}
	f := &forge{spec: spec, rig: r, work: work, atts: atts, images: map[string][]byte{}, geoms: map[string]*partGeom{},
		files: map[string]string{}, sizes: map[string][2]int{}}
	if err := f.loadParts(); err != nil {
		return err
	}
	b, err := f.build()
	if err != nil {
		return err
	}
	if *out == "" {
		*out = filepath.Join(work, spec.Name+".lottie")
	}
	var buf bytes.Buffer
	if err := b.Encode(&buf); err != nil {
		return err
	}
	if err := os.WriteFile(*out, buf.Bytes(), 0o644); err != nil {
		return err
	}
	for _, w := range f.warnings {
		fmt.Println("warning:", w)
	}
	fmt.Printf("wrote %s (%d clips, %d parts, %d attachment layers, %d bytes)\n", *out, len(b.AnimationIDs()), len(f.geoms), len(atts), buf.Len())
	return nil
}

type forge struct {
	spec     *Spec
	rig      *rig
	work     string
	atts     []*attLayer
	images   map[string][]byte    // file name -> PNG bytes for i/
	geoms    map[string]*partGeom // layer name -> geometry (textured mode)
	files    map[string]string    // layer name -> image file
	sizes    map[string][2]int    // layer name -> image size
	warnings []string
}

func (f *forge) partPath(name string) string { return filepath.Join(f.work, "parts", name+".png") }

// loadPart finds a slot's drawing: cut output, a derived far copy, or the
// base preset's own image so a partial sheet still rigs.
func (f *forge) loadPart(s *slot) (*image.NRGBA, error) {
	if img, err := loadImage(f.partPath(s.name)); err == nil {
		return img, nil
	}
	if s.nearOf != "" {
		if near := f.rig.byName[s.nearOf]; near != nil {
			if img, err := loadImage(f.partPath(near.cellName())); err == nil {
				return darken(img, 0.72), nil
			}
		}
	} else if img, err := loadImage(f.partPath(s.cellName())); err == nil {
		return img, nil
	}
	if data, ok := f.rig.bundle.Image(s.file); ok {
		f.warnings = append(f.warnings, s.name+": no part drawn, using the base preset's image")
		return decodeImage(data)
	}
	return nil, fmt.Errorf("%s: no drawing in parts/ and none in the base", s.name)
}

func (f *forge) loadParts() error {
	for _, s := range f.rig.slots {
		img, err := f.loadPart(s)
		if err != nil {
			return err
		}
		file := f.spec.Name + "-" + s.name + ".png"
		if s.category == "shadow" || f.spec.Raster {
			if s.category != "shadow" {
				img = fitRaster(img, s.w, s.h, s.anchor, s.joint, f.spec.partSpec(s.name).Fit)
			}
			f.images[file] = encodePNG(img)
			f.files[s.name] = file
			f.sizes[s.name] = [2]int{img.Rect.Dx(), img.Rect.Dy()}
			continue
		}
		ps := f.spec.partSpec(s.name)
		budget := ps.Vertices
		if budget == 0 {
			budget = 12
			if s.category == "head" || s.category == "body" {
				budget = 16
			}
		}
		fit := ps.Fit
		if s.category == "prop" {
			if l := f.spec.propLength(s.name); l > 0 {
				fit *= l / float64(s.h)
			}
		}
		g, err := buildGeom(s.name, img, float64(s.w), float64(s.h), s.anchor, s.joint, fit, budget)
		if err != nil {
			return err
		}
		f.geoms[s.name] = g
		f.images[file] = encodePNG(img)
		f.files[s.name] = file
		f.sizes[s.name] = [2]int{img.Rect.Dx(), img.Rect.Dy()}
	}
	for _, al := range f.atts {
		img, err := loadImage(f.partPath(al.cellName))
		if err != nil {
			if al.viewOf != "" {
				// A missing view drawing falls back to the front one.
				img, err = loadImage(f.partPath(al.att.Name))
			}
			if err != nil {
				return fmt.Errorf("attachment %s: parts/%s.png missing (cut it, or draw it)", al.name, al.cellName)
			}
		}
		if al.crop != nil {
			img = cropImage(img, alphaBounds(cropFrac(img, *al.crop)).Add(cropOrigin(img, *al.crop)))
		}
		if al.dark != 1 {
			img = darken(img, al.dark)
		}
		file := f.spec.Name + "-" + al.name + ".png"
		if f.spec.Raster {
			img = fitRaster(img, int(math.Round(al.w)), int(math.Round(al.h)), al.anchor, "top", al.att.Fit)
			f.images[file] = encodePNG(img)
			f.files[al.name] = file
			f.sizes[al.name] = [2]int{img.Rect.Dx(), img.Rect.Dy()}
			continue
		}
		budget := al.att.Vertices
		if budget == 0 {
			budget = 12
		}
		g, err := buildGeom(al.name, img, al.w, al.h, al.anchor, "top", al.att.Fit, budget)
		if err != nil {
			return err
		}
		f.geoms[al.name] = g
		f.images[file] = encodePNG(img)
		f.files[al.name] = file
		f.sizes[al.name] = [2]int{img.Rect.Dx(), img.Rect.Dy()}
	}
	return nil
}

func cropOrigin(img *image.NRGBA, fr [4]float64) image.Point {
	return image.Pt(int(fr[0]*float64(img.Rect.Dx())), int(fr[1]*float64(img.Rect.Dy())))
}

// fitRaster resizes a drawing into a slot canvas with the same fit rule
// the textured path uses, so the raster tier and the morph tier agree.
func fitRaster(img *image.NRGBA, w, h int, anchor pt, joint string, fit float64) *image.NRGBA {
	box := alphaBounds(img)
	if box.Empty() {
		return image.NewNRGBA(image.Rect(0, 0, w, h))
	}
	fm := computeFit(float64(w), float64(h), anchor, joint, box, fit)
	sw, sh := max(int(math.Round(float64(box.Dx())*fm.s)), 1), max(int(math.Round(float64(box.Dy())*fm.s)), 1)
	scaled := resizeTo(cropImage(img, box), sw, sh, false)
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	origin := fm.apply(pt{float64(box.Min.X), float64(box.Min.Y)})
	drawImage(out, image.Rect(int(math.Round(origin[0])), int(math.Round(origin[1])), int(math.Round(origin[0]))+sw, int(math.Round(origin[1]))+sh), scaled)
	return out
}

func (f *forge) build() (*lottie.Bundle, error) {
	b := lottie.NewBundle()
	for _, id := range f.rig.clipIDs {
		raw, _ := f.rig.bundle.AnimationJSON(id)
		var clip obj
		if err := json.Unmarshal(raw, &clip); err != nil {
			return nil, fmt.Errorf("%s: %w", id, err)
		}
		doc, err := f.rewriteClip(clip)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", id, err)
		}
		out, err := json.Marshal(clip)
		if err != nil {
			return nil, err
		}
		if err := b.SetAnimation(id, out); err != nil {
			return nil, fmt.Errorf("%s: %w", id, err)
		}
		if doc != nil && !doc.Empty() {
			if err := lottietexture.Store(b, id, doc); err != nil {
				return nil, err
			}
		}
	}
	for name, data := range f.images {
		b.SetImage(name, data)
	}
	for _, id := range f.rig.bundle.StateMachineIDs() {
		sm, err := f.rig.bundle.StateMachine(id)
		if err != nil {
			return nil, err
		}
		if err := b.SetStateMachine(id, sm); err != nil {
			return nil, err
		}
	}
	for _, name := range f.rig.bundle.ExtensionFiles("") {
		if strings.HasPrefix(name, lottietexture.Dir) || strings.HasPrefix(name, forgeDir) {
			continue
		}
		data, _ := f.rig.bundle.ExtensionFile(name)
		b.SetExtensionFile(name, data)
	}
	for name, g := range f.geoms {
		if err := b.SetExtensionFile(forgeFile(name), g.marshal()); err != nil {
			return nil, err
		}
	}
	if err := addClips(b, f.spec); err != nil {
		return nil, err
	}
	if problems := b.Validate(); len(problems) > 0 {
		return nil, fmt.Errorf("bundle invalid: %v", problems)
	}
	return b, nil
}

const forgeDir = "extensions/forge/"

func forgeFile(layer string) string { return forgeDir + layer + ".json" }

// rewriteClip converts the base's image layers to textured shape layers
// and inserts the attachment layers; it returns the clip's texture doc.
func (f *forge) rewriteClip(clip obj) (*lottietexture.Doc, error) {
	doc := &lottietexture.Doc{}
	off := false
	// Assets: repoint every part at its new file and size.
	assets, _ := clip["assets"].([]any)
	seen := map[string]bool{}
	for _, a := range assets {
		m, ok := a.(obj)
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		if file, ok := f.files[id]; ok {
			m["p"], m["u"] = file, "i/"
			m["w"], m["h"] = float64(f.sizes[id][0]), float64(f.sizes[id][1])
			seen[id] = true
		}
	}
	for _, al := range f.atts {
		if seen[al.name] {
			continue
		}
		assets = append(assets, obj{"id": al.name, "p": f.files[al.name], "u": "i/", "e": 0,
			"w": float64(f.sizes[al.name][0]), "h": float64(f.sizes[al.name][1])})
	}
	clip["assets"] = assets

	layers := layersOf(clip)
	byName := map[string]obj{}
	maxInd := 0
	for _, l := range layers {
		if name, _ := l["nm"].(string); name != "" {
			byName[name] = l
		}
		maxInd = max(maxInd, layerInd(l))
	}
	for _, l := range layers {
		name, _ := l["nm"].(string)
		g, ok := f.geoms[name]
		if !ok || num(l["ty"]) != 2 {
			continue
		}
		convertLayer(l, g, doc, &off)
	}
	// Attachments, parents first.
	pending := append([]*attLayer{}, f.atts...)
	for len(pending) > 0 {
		progress := false
		var rest []*attLayer
		for _, al := range pending {
			parent, ok := byName[al.parentName]
			if !ok {
				rest = append(rest, al)
				continue
			}
			host := byName[al.host.name]
			if host == nil {
				host = parent
			}
			maxInd++
			l := f.attachmentLayer(al, parent, host, maxInd, doc, &off)
			before, target := orderTarget(al.order)
			idx := len(layers)
			for i, x := range layers {
				if n, _ := x["nm"].(string); n == target {
					idx = i
					if !before {
						idx = i + 1
					}
					break
				}
			}
			layers = append(layers[:idx], append([]obj{l}, layers[idx:]...)...)
			byName[al.name] = l
			progress = true
		}
		if !progress {
			return nil, fmt.Errorf("attachment %s: parent %q not found", rest[0].name, rest[0].parentName)
		}
		pending = rest
	}
	setLayers(clip, layers)
	return doc, nil
}

// convertLayer turns an image layer into a shape layer painted with the
// same image through its traced contours.
func convertLayer(l obj, g *partGeom, doc *lottietexture.Doc, off *bool) {
	items, fillIdx := g.shapeItems(nil)
	name, _ := l["nm"].(string)
	delete(l, "refId")
	l["ty"] = 4
	l["shapes"] = []any{obj{"ty": "gr", "nm": name, "it": toAny(items)}}
	ind := layerInd(l)
	doc.Paints = append(doc.Paints, lottietexture.Paint{Layer: ind, Item: []int{0, fillIdx}, Texture: g.Slot,
		Mapping: lottietexture.MappingVertex, Tint: off})
	for i, p := range g.Pieces {
		uv := make([][2]float64, len(p.UV))
		copy(uv, p.UV)
		doc.UVs = append(doc.UVs, lottietexture.UV{Layer: ind, Item: []int{0, i}, V: uv})
	}
}

func toAny(items []obj) []any {
	out := make([]any, len(items))
	for i, it := range items {
		out[i] = it
	}
	return out
}

func (f *forge) attachmentLayer(al *attLayer, parent, host obj, ind int, doc *lottietexture.Doc, off *bool) obj {
	hostKS, _ := host["ks"].(obj)
	l := obj{"nm": al.name, "ind": ind, "parent": layerInd(parent),
		"ip": parent["ip"], "op": parent["op"], "st": parent["st"],
		"ks": obj{
			"a": static(vec2(al.anchor)), "p": static(vec2(al.attach)),
			"s": static([]float64{100, 100}), "r": static(0.0), "o": deepCopy(hostKS["o"]),
		}}
	if parent["ip"] == nil {
		l["ip"], l["op"], l["st"] = 0.0, 1e6, 0.0
	}
	if g, ok := f.geoms[al.name]; ok {
		l["ty"] = 4
		convertLayer(l, g, doc, off)
	} else {
		l["ty"] = 2
		l["refId"] = al.name
	}
	return l
}

// addClips copies the spec's new clips and wires their states.
func addClips(b *lottie.Bundle, spec *Spec) error {
	for _, c := range spec.Clips.Add {
		raw, ok := b.AnimationJSON(c.From)
		if !ok {
			return fmt.Errorf("clips.add %s: no clip %q to copy", c.Name, c.From)
		}
		var clip obj
		if err := json.Unmarshal(raw, &clip); err != nil {
			return err
		}
		clip["nm"] = c.Name
		out, _ := json.Marshal(clip)
		if err := b.SetAnimation(c.Name, out); err != nil {
			return err
		}
		if doc, err := lottietexture.Load(b, c.From); err == nil && doc != nil {
			if err := lottietexture.Store(b, c.Name, doc); err != nil {
				return err
			}
		}
	}
	if len(spec.Clips.Machine) == 0 {
		return nil
	}
	ids := b.StateMachineIDs()
	if len(ids) == 0 {
		return fmt.Errorf("clips.machine: the base has no state machine to extend")
	}
	sm, err := b.StateMachine(ids[0])
	if err != nil {
		return err
	}
	for _, m := range spec.Clips.Machine {
		if m.State == "" || m.Animation == "" {
			return fmt.Errorf("clips.machine: state and animation are required")
		}
		if _, ok := b.AnimationJSON(m.Animation); !ok {
			return fmt.Errorf("clips.machine %s: no clip %q", m.State, m.Animation)
		}
		returns := m.Returns
		if returns == "" {
			returns = sm.Initial
		}
		st := lottie.State{Name: m.State, Type: lottie.StatePlayback, Animation: m.Animation, Autoplay: true,
			Transitions: []lottie.Transition{{Type: lottie.TransitionType("Transition"), ToState: returns,
				Guards: []lottie.Guard{{Type: lottie.GuardEvent, InputName: "clipDone"}}}}}
		replaced := false
		for i := range sm.States {
			if sm.States[i].Name == m.State {
				sm.States[i], replaced = st, true
			}
		}
		if !replaced {
			sm.States = append(sm.States, st)
		}
		if m.Event != "" {
			have := false
			for _, in := range sm.Inputs {
				have = have || in.Name == m.Event
			}
			if !have {
				sm.Inputs = append(sm.Inputs, lottie.Input{Type: lottie.InputEvent, Name: m.Event})
			}
			for _, from := range m.From {
				for i := range sm.States {
					if sm.States[i].Name != from {
						continue
					}
					sm.States[i].Transitions = append(sm.States[i].Transitions, lottie.Transition{
						Type: lottie.TransitionType("Transition"), ToState: m.State,
						Guards: []lottie.Guard{{Type: lottie.GuardEvent, InputName: m.Event}}})
				}
			}
		}
	}
	return b.SetStateMachine(ids[0], sm)
}
