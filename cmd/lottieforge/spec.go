package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Spec is work/character.json — the one file the agent writes
// (.knowledge data:character-forge-spec).
type Spec struct {
	Name        string              `json:"name"`
	Base        string              `json:"base"`
	Description string              `json:"description"`
	Style       string              `json:"style"`
	Proportions string              `json:"proportions"`
	Key         string              `json:"key"`
	SheetSize   []int               `json:"sheet_size"`
	Parts       map[string]PartSpec `json:"parts"`
	Attachments []*Attachment       `json:"attachments"`
	Props       []Prop              `json:"props"`
	Morph       []MorphSpec         `json:"morph"`
	Clips       ClipsSpec           `json:"clips"`
	Raster      bool                `json:"raster"`
}

// PartSpec overrides one rig slot.
type PartSpec struct {
	Fit      float64 `json:"fit"`
	Vertices int     `json:"vertices"`
	Mapping  string  `json:"mapping"`
	Blobs    int     `json:"blobs"`
}

// Attachment is one worn or grown thing beyond the rig's slots
// (.knowledge concept:attachment-kinds).
type Attachment struct {
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	Host      string    `json:"host"`
	Attach    []float64 `json:"attach"`
	Anchor    []float64 `json:"anchor"`
	Size      []float64 `json:"size"`
	Order     string    `json:"order"`
	Paired    bool      `json:"paired"`
	Views     string    `json:"views"`
	Drivers   []string  `json:"drivers"`
	Weight    float64   `json:"weight"`
	Panels    int       `json:"panels"`
	Segments  int       `json:"segments"`
	Damping   float64   `json:"damping"`
	Stiffness float64   `json:"stiffness"`
	Sway      *Sway     `json:"sway"`
	Vertices  int       `json:"vertices"`
	Fit       float64   `json:"fit"`
}

// Sway is a slow sine on a rigid attachment's rotation.
type Sway struct {
	Amount float64 `json:"amount"`
	Period float64 `json:"period"`
}

// Prop renames or resizes a held-item slot of the base rig.
type Prop struct {
	Slot   string  `json:"slot"`
	Name   string  `json:"name"`
	Length float64 `json:"length"`
}

// MorphSpec is one generator application (references/motion.md).
type MorphSpec struct {
	Generator string   `json:"generator"`
	Parts     names    `json:"parts"`
	Clips     names    `json:"clips"`
	Amount    float64  `json:"amount"`
	Period    float64  `json:"period"`
	At        float64  `json:"at"`
	Recover   float64  `json:"recover"`
	From      float64  `json:"from"`
	To        float64  `json:"to"`
	Threshold float64  `json:"threshold"`
	Reach     float64  `json:"reach"`
	Lag       float64  `json:"lag"`
	Weight    float64  `json:"weight"`
	Drivers   []string `json:"drivers"`
}

// ClipsSpec adds clips and states on top of the base's.
type ClipsSpec struct {
	Add     []ClipAdd    `json:"add"`
	Machine []MachineAdd `json:"machine"`
}

type ClipAdd struct {
	Name string `json:"name"`
	From string `json:"from"`
}

type MachineAdd struct {
	State     string   `json:"state"`
	Animation string   `json:"animation"`
	Event     string   `json:"event"`
	From      []string `json:"from"`
	Returns   string   `json:"returns"`
}

// names is a list of names, or the word "all".
type names struct {
	All  bool
	List []string
}

func (n *names) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		n.All = s == "all" || s == "*"
		if !n.All && s != "" {
			n.List = []string{s}
		}
		return nil
	}
	return json.Unmarshal(b, &n.List)
}

func (n names) has(name string) bool {
	if n.All {
		return true
	}
	for _, x := range n.List {
		if x == name {
			return true
		}
	}
	return false
}

func loadSpec(work string) (*Spec, error) {
	data, err := os.ReadFile(filepath.Join(work, "character.json"))
	if err != nil {
		return nil, err
	}
	var s Spec
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("character.json: %w", err)
	}
	if s.Name == "" {
		s.Name = "character"
	}
	if s.Base == "" {
		s.Base = "chibi-male"
	}
	if s.Key == "" {
		s.Key = "#FF00FF"
	}
	if len(s.SheetSize) != 2 || s.SheetSize[0] <= 0 || s.SheetSize[1] <= 0 {
		s.SheetSize = []int{1024, 1024}
	}
	if s.Proportions == "" {
		if strings.HasPrefix(s.Base, "chibi") {
			s.Proportions = "chibi proportions, about 2.5 heads tall, big head, short limbs"
		} else {
			s.Proportions = "standard proportions, about 6 heads tall"
		}
	}
	if s.Style == "" {
		s.Style = "clean anime cel shading, thick dark outline, flat colors, no gradients"
	}
	if s.Parts == nil {
		s.Parts = map[string]PartSpec{}
	}
	for i, a := range s.Attachments {
		if a.Name == "" {
			return nil, fmt.Errorf("attachments[%d]: name is required", i)
		}
		a.defaults()
	}
	return &s, nil
}

func (a *Attachment) defaults() {
	switch a.Kind {
	case "rigid", "swing", "drape", "lock":
	default:
		a.Kind = "rigid"
	}
	if a.Host == "" {
		if a.Kind == "drape" {
			a.Host = "body"
		} else {
			a.Host = "head"
		}
	}
	if len(a.Size) != 2 {
		switch a.Kind {
		case "drape":
			a.Size = []float64{56, 40}
		case "rigid":
			a.Size = []float64{32, 24}
		default:
			a.Size = []float64{16, 44}
		}
	}
	if a.Views == "" {
		a.Views = "baked"
	}
	if a.Weight == 0 {
		if a.Kind == "drape" {
			a.Weight = 0.6
		} else {
			a.Weight = 0.4
		}
	}
	if a.Damping == 0 {
		a.Damping = 0.8
	}
	if a.Stiffness == 0 {
		a.Stiffness = 1
	}
	if a.Segments < 1 {
		a.Segments = 1
	}
	if a.Fit == 0 {
		a.Fit = 1
	}
	if a.Kind == "drape" && len(a.Drivers) == 0 {
		switch strings.TrimSuffix(strings.TrimSuffix(a.Host, "-near"), "-far") {
		case "body":
			a.Drivers = []string{"thigh-near", "thigh-far"}
		case "upper-arm":
			a.Drivers = []string{"forearm"}
		case "thigh":
			a.Drivers = []string{"shin"}
		}
	}
}

// partSpec returns the override for a slot, defaults filled.
func (s *Spec) partSpec(slot string) PartSpec {
	p := s.Parts[slot]
	if p.Fit == 0 {
		p.Fit = 1
	}
	return p
}

// propName is the display name of a held-item slot.
func (s *Spec) propName(slot string) string {
	for _, p := range s.Props {
		if p.Slot == slot && p.Name != "" {
			return p.Name
		}
	}
	return slot
}

func (s *Spec) propLength(slot string) float64 {
	for _, p := range s.Props {
		if p.Slot == slot {
			return p.Length
		}
	}
	return 0
}

// resolveBase finds the base preset's bundle: a path as given, or a
// preset name looked up under -presets and then under
// examples/state-editor/presets of this checkout or any parent.
func resolveBase(base, presets string) (string, error) {
	if strings.HasSuffix(base, ".lottie") {
		if _, err := os.Stat(base); err == nil {
			return base, nil
		}
		return "", fmt.Errorf("base bundle %s not found", base)
	}
	rel := filepath.Join(base, base+".lottie")
	var dirs []string
	if presets != "" {
		dirs = append(dirs, presets)
	}
	if wd, err := os.Getwd(); err == nil {
		for d := wd; ; d = filepath.Dir(d) {
			dirs = append(dirs, filepath.Join(d, "examples", "state-editor", "presets"))
			if filepath.Dir(d) == d {
				break
			}
		}
	}
	for _, d := range dirs {
		p := filepath.Join(d, rel)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("preset %q not found; pass -presets DIR or a .lottie path as base", base)
}
