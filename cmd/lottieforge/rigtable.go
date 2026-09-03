package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	lottie "github.com/shibukawa/lottie-go"
)

// slot is one part of the base rig as its reference clip declares it:
// the contract a drawing must honor (skills/lottie-character-preset
// references/rig.md), read from the bundle instead of a table so any
// preset works.
type slot struct {
	name     string
	w, h     int
	anchor   [2]float64
	attach   [2]float64
	parent   string
	ind      int
	joint    string // "top" or "bottom": where the pivot sits in the drawing
	category string // head, body, limb, prop, shadow
	viewOf   string // head-side -> head
	nearOf   string // upper-arm-far -> upper-arm-near
	file     string // image file in i/
	order    int    // position in the reference clip's layer array (0 = front)
}

// cell reports whether the slot needs a drawing of its own.
func (s *slot) cell() bool {
	return s.category != "shadow" && s.nearOf == ""
}

// cellName is the drawing's name: near limbs drop their suffix.
func (s *slot) cellName() string { return strings.TrimSuffix(s.name, "-near") }

type rig struct {
	path    string
	bundle  *lottie.Bundle
	slots   []*slot
	byName  map[string]*slot
	clipIDs []string
	refClip string
	w, h    int
	fps     float64
}

func loadRig(path string) (*rig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	b, err := lottie.DecodeBundle(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	r := &rig{path: path, bundle: b, byName: map[string]*slot{}}
	r.clipIDs = b.AnimationIDs()
	sort.Strings(r.clipIDs)
	if len(r.clipIDs) == 0 {
		return nil, fmt.Errorf("%s: no clips", path)
	}
	r.refClip = r.clipIDs[0]
	for _, id := range r.clipIDs {
		if id == "idle-anim" {
			r.refClip = id
		}
	}
	raw, _ := b.AnimationJSON(r.refClip)
	var clip obj
	if err := json.Unmarshal(raw, &clip); err != nil {
		return nil, err
	}
	r.w, r.h, r.fps = int(num(clip["w"])), int(num(clip["h"])), num(clip["fr"])
	assets := map[string]obj{}
	if arr, ok := clip["assets"].([]any); ok {
		for _, a := range arr {
			if m, ok := a.(obj); ok {
				if id, _ := m["id"].(string); id != "" {
					assets[id] = m
				}
			}
		}
	}
	layers := layersOf(clip)
	byInd := map[int]obj{}
	for _, l := range layers {
		byInd[layerInd(l)] = l
	}
	for i, l := range layers {
		if num(l["ty"]) != 2 {
			continue
		}
		ref, _ := l["refId"].(string)
		name, _ := l["nm"].(string)
		if name == "" {
			name = ref
		}
		as, ok := assets[ref]
		if !ok {
			continue
		}
		ks, _ := l["ks"].(obj)
		s := &slot{name: name, w: int(num(as["w"])), h: int(num(as["h"])), ind: layerInd(l), order: i}
		s.file, _ = as["p"].(string)
		if a := propValue(ks["a"]); len(a) >= 2 {
			s.anchor = [2]float64{a[0], a[1]}
		}
		if p := propValue(ks["p"]); len(p) >= 2 {
			s.attach = [2]float64{p[0], p[1]}
		}
		if pl, ok := byInd[int(num(l["parent"]))]; ok && l["parent"] != nil {
			s.parent, _ = pl["nm"].(string)
		}
		categorize(s)
		r.slots = append(r.slots, s)
		r.byName[name] = s
	}
	if len(r.slots) == 0 {
		return nil, fmt.Errorf("%s: %s has no image layers; the base must be a raster cutout preset", path, r.refClip)
	}
	return r, nil
}

func categorize(s *slot) {
	base := s.name
	if strings.HasSuffix(base, "-far") {
		s.nearOf = strings.TrimSuffix(base, "-far") + "-near"
		base = strings.TrimSuffix(base, "-far")
	}
	base = strings.TrimSuffix(base, "-near")
	for _, v := range []string{"-side", "-back"} {
		if strings.HasSuffix(base, v) {
			s.viewOf = strings.TrimSuffix(base, v)
			base = s.viewOf
		}
	}
	switch {
	case base == "head":
		s.category, s.joint = "head", "bottom"
	case base == "body":
		s.category, s.joint = "body", "bottom"
	case base == "shadow":
		s.category, s.joint = "shadow", "top"
	case base == "upper-arm" || base == "forearm" || base == "thigh" || base == "shin":
		s.category, s.joint = "limb", "top"
	default:
		s.category, s.joint = "prop", "top"
	}
}

// hostSlot resolves a spec host name ("upper-arm", "head") to a slot,
// preferring the near variant for paired limbs.
func (r *rig) hostSlot(name string, far bool) *slot {
	if far {
		if s, ok := r.byName[name+"-far"]; ok {
			return s
		}
	}
	if s, ok := r.byName[name]; ok {
		return s
	}
	if s, ok := r.byName[name+"-near"]; ok {
		return s
	}
	return nil
}

// views lists the view-swap slots of a host (head-side, head-back).
func (r *rig) views(host string) []*slot {
	var out []*slot
	for _, s := range r.slots {
		if s.viewOf == host {
			out = append(out, s)
		}
	}
	return out
}

// hasSlot reports whether name exists in the rig by any spelling.
func (r *rig) hasSlot(name string) bool { return r.hostSlot(name, false) != nil }
