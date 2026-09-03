package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// cellDef is one drawing the image model is asked for
// (.knowledge data:character-forge-spec sheets.json).
type cellDef struct {
	ID    string `json:"id"`
	Slot  string `json:"slot"`
	View  string `json:"view,omitempty"`
	Rect  [4]int `json:"rect"`
	Joint string `json:"joint"`
	Blobs int    `json:"blobs"`
	Line  string `json:"line"`
}

type sheetDef struct {
	ID      string    `json:"id"`
	Size    [2]int    `json:"size"`
	Purpose string    `json:"purpose,omitempty"`
	Cells   []cellDef `json:"cells"`
}

type manifest struct {
	Key    string     `json:"key"`
	Margin int        `json:"margin"`
	Band   int        `json:"band"`
	Sheets []sheetDef `json:"sheets"`
}

const (
	cellMargin = 8
	labelBand  = 32
	cellGutter = 16
)

// cellReq is a cell before layout.
type cellReq struct {
	name, view, line, joint string
	w, h                    float64
	blobs                   int
}

func loadManifest(work string) (*manifest, error) {
	data, err := os.ReadFile(filepath.Join(work, "sheets.json"))
	if err != nil {
		return nil, fmt.Errorf("sheets.json: %w (run lottieforge grid first)", err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("sheets.json: %w", err)
	}
	if m.Margin == 0 {
		m.Margin = cellMargin
	}
	return &m, nil
}

func runGrid(args []string) error {
	fs := flag.NewFlagSet("grid", flag.ContinueOnError)
	presets := fs.String("presets", "", "directory holding the base presets")
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
	basePath, err := resolveBase(spec.Base, *presets)
	if err != nil {
		return err
	}
	r, err := loadRig(basePath)
	if err != nil {
		return err
	}
	key, err := parseHex(spec.Key)
	if err != nil {
		return err
	}
	m := buildManifest(spec, r)
	for _, d := range []string{"sheets", "prompts", "parts"} {
		if err := os.MkdirAll(filepath.Join(work, d), 0o755); err != nil {
			return err
		}
	}
	raw, _ := json.MarshalIndent(m, "", " ")
	if err := os.WriteFile(filepath.Join(work, "sheets.json"), raw, 0o644); err != nil {
		return err
	}
	for _, sh := range m.Sheets {
		if len(sh.Cells) == 0 {
			continue
		}
		img := templateImage(sh, key, m)
		if err := writePNG(filepath.Join(work, "sheets", sh.ID+".template.png"), img); err != nil {
			return err
		}
	}
	if err := writePrompts(work, spec, m, key); err != nil {
		return err
	}
	fmt.Printf("wrote sheets.json (%d sheets), sheets/*.template.png and prompts/*.md in %s\n", len(m.Sheets), work)
	return nil
}

// buildManifest lays every drawing the rig and the spec need onto sheets.
func buildManifest(spec *Spec, r *rig) *manifest {
	m := &manifest{Key: strings.ToUpper(spec.Key), Margin: cellMargin, Band: labelBand}
	size := [2]int{spec.SheetSize[0], spec.SheetSize[1]}
	m.Sheets = append(m.Sheets, sheetDef{ID: "model", Size: size,
		Purpose: "full body reference: three-quarter view facing right, relaxed A-pose, flat key background"})

	groups := map[string][]cellReq{}
	for _, s := range sortedSlots(r) {
		if !s.cell() {
			continue
		}
		req := cellReq{name: s.cellName(), joint: s.joint, w: float64(s.w), h: float64(s.h), blobs: 1}
		req.line = slotLine(spec, r, s)
		switch s.category {
		case "head":
			req.view = viewOf(s.name)
			groups["heads"] = append(groups["heads"], req)
		case "body":
			req.view = viewOf(s.name)
			groups["torsos"] = append(groups["torsos"], req)
		case "limb":
			groups["limbs"] = append(groups["limbs"], req)
		case "prop":
			groups["limbs"] = append(groups["limbs"], req)
		}
	}
	for _, a := range spec.Attachments {
		req := cellReq{name: a.Name, joint: "top", w: a.Size[0], h: a.Size[1], blobs: 1, line: attachmentLine(a, "")}
		groups["attachments"] = append(groups["attachments"], req)
		if a.Views == "separate" {
			for _, v := range []string{"side", "back"} {
				groups["attachments"] = append(groups["attachments"], cellReq{name: a.Name + "-" + v, view: v,
					joint: "top", w: a.Size[0], h: a.Size[1], blobs: 1, line: attachmentLine(a, v)})
			}
		}
	}
	for _, id := range []string{"heads", "torsos", "limbs", "attachments"} {
		cells := groups[id]
		if len(cells) == 0 {
			continue
		}
		sh := sheetDef{ID: id, Size: size}
		_, rects := layoutCells(cells, size[0], size[1], m.Margin, m.Band, cellGutter)
		for i, c := range cells {
			sh.Cells = append(sh.Cells, cellDef{ID: cellID(rects, i), Slot: c.name, View: c.view,
				Rect: rects[i], Joint: c.joint, Blobs: c.blobs, Line: c.line})
		}
		m.Sheets = append(m.Sheets, sh)
	}
	return m
}

// sortedSlots orders drawings the way the prompts list them: fronts
// before views, then limbs in chain order, then props.
func sortedSlots(r *rig) []*slot {
	rank := func(s *slot) int {
		base := strings.TrimSuffix(s.cellName(), "-side")
		base = strings.TrimSuffix(base, "-back")
		order := map[string]int{"head": 0, "body": 10, "upper-arm": 20, "forearm": 21, "thigh": 22, "shin": 23}
		v := 30
		if o, ok := order[base]; ok {
			v = o
		}
		switch {
		case strings.HasSuffix(s.name, "-side"):
			v++
		case strings.HasSuffix(s.name, "-back"):
			v += 2
		}
		return v
	}
	out := append([]*slot{}, r.slots...)
	sort.SliceStable(out, func(i, j int) bool { return rank(out[i]) < rank(out[j]) })
	return out
}

func viewOf(name string) string {
	switch {
	case strings.HasSuffix(name, "-side"):
		return "rear-quarter"
	case strings.HasSuffix(name, "-back"):
		return "back"
	}
	return "front"
}

// slotLine is the prompt line for a rig drawing, with the attachment
// wording a host needs.
func slotLine(spec *Spec, r *rig, s *slot) string {
	host := s.name
	if s.viewOf != "" {
		host = s.viewOf
	}
	var baked, separate []string
	for _, a := range spec.Attachments {
		if strings.TrimSuffix(strings.TrimSuffix(a.Host, "-near"), "-far") != host {
			continue
		}
		if a.Views == "separate" {
			separate = append(separate, a.Name)
		} else {
			baked = append(baked, a.Name)
		}
	}
	without := func(names []string) string {
		if len(names) == 0 {
			return ""
		}
		return " WITHOUT " + joinNames(names) + " (drawn in their own cells; draw the part complete where they would attach)"
	}
	worn := func() string {
		if len(baked) == 0 {
			return ""
		}
		return " AS WORN with its " + joinNames(baked) + " seen from that angle"
	}
	switch s.category {
	case "head":
		switch viewOf(s.name) {
		case "front":
			return "the whole head with its hair and a short neck stump" + without(append(baked, separate...)) + ", front three-quarter view facing right, both eyes visible, neck at the bottom center"
		case "rear-quarter":
			return "the same head from the rear three-quarter view" + worn() + without(separate) + ": mostly the back of the head, one eye just visible at the leading (right) edge, neck at the bottom center"
		default:
			return "the same head from directly behind" + worn() + without(separate) + ", no face visible, neck at the bottom center"
		}
	case "body":
		switch viewOf(s.name) {
		case "front":
			return "the torso from the neck stump at the top to the hips at the bottom" + without(append(baked, separate...)) + ", without arms and without legs, hip at the bottom center"
		case "rear-quarter":
			return "the same torso from the rear three-quarter view" + worn() + without(separate) + ", without arms and legs, hip at the bottom center"
		default:
			return "the same torso from directly behind" + worn() + without(separate) + ", without arms and legs, hip at the bottom center"
		}
	case "limb":
		switch s.cellName() {
		case "upper-arm":
			return "one upper arm, shoulder to elbow, hanging straight down, shoulder cap at the top center, elbow cap at the bottom"
		case "forearm":
			return "one forearm with the hand, elbow to fingertips, hanging straight down, elbow cap at the top center, hand relaxed and open at the bottom"
		case "thigh":
			return "one thigh, hip to knee, hanging straight down, hip cap at the top center, knee cap at the bottom"
		case "shin":
			return "one lower leg with the foot, knee to toes, knee cap at the top center, the foot flat at the bottom pointing RIGHT"
		}
	}
	return "the " + spec.propName(s.name) + " alone, drawn vertical, grip at the top center of the cell, pointing down, full brightness, symmetric so it reads the same mirrored"
}

func attachmentLine(a *Attachment, view string) string {
	host := strings.TrimSuffix(strings.TrimSuffix(a.Host, "-near"), "-far")
	var line string
	switch a.Kind {
	case "rigid":
		line = "the " + a.Name + " alone as worn on the " + host + ", upright, its base at the bottom center of the cell"
	case "drape":
		line = "the whole " + a.Name + " from its top edge at the top center of the cell to the hem, hanging straight and relaxed, a little wider at the top than the " + host + " so it overlaps it, nothing inside it"
	default:
		line = "the " + a.Name + " alone, from its root at the top center of the cell to the tip, hanging straight down, the root drawn slightly wider so it overlaps the " + host
	}
	if a.Paired {
		line += " (one of the pair)"
	}
	switch view {
	case "side":
		line = "the same " + a.Name + " seen from the rear three-quarter view, root at the top center"
	case "back":
		line = "the same " + a.Name + " seen from directly behind, root at the top center"
	}
	return line
}

func joinNames(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return "the " + names[0]
	}
	return "the " + strings.Join(names[:len(names)-1], ", the ") + " and the " + names[len(names)-1]
}

// layoutCells packs cells into rows at the largest scale that fits.
func layoutCells(cells []cellReq, W, H, margin, band, gutter int) (float64, [][4]int) {
	lo, hi := 0.25, 64.0
	var best [][4]int
	bestScale := lo
	for range 40 {
		mid := (lo + hi) / 2
		if rects, ok := packCells(cells, mid, W, H, margin, band, gutter); ok {
			best, bestScale, lo = rects, mid, mid
		} else {
			hi = mid
		}
	}
	if best == nil {
		best, _ = packCells(cells, lo, W, H, margin, band, gutter)
	}
	return bestScale, best
}

func packCells(cells []cellReq, k float64, W, H, margin, band, gutter int) ([][4]int, bool) {
	rects := make([][4]int, len(cells))
	x, y, rowH := gutter, gutter, 0
	for i, c := range cells {
		cw := int(math.Ceil(c.w*k)) + 2*margin
		ch := int(math.Ceil(c.h*k)) + 2*margin
		if x+cw+gutter > W {
			y += rowH + band + gutter
			x, rowH = gutter, 0
		}
		if x+cw+gutter > W || y+ch+band+gutter > H {
			return nil, false
		}
		rects[i] = [4]int{x, y, x + cw, y + ch}
		x += cw + gutter
		if ch > rowH {
			rowH = ch
		}
	}
	return rects, true
}

// cellID names a cell by its row letter and column number.
func cellID(rects [][4]int, i int) string {
	row, col := 0, 1
	for j := 1; j <= i; j++ {
		if rects[j][1] != rects[j-1][1] {
			row++
			col = 1
		} else {
			col++
		}
	}
	return fmt.Sprintf("%c%d", 'A'+row, col)
}

func templateImage(sh sheetDef, key color.NRGBA, m *manifest) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, sh.Size[0], sh.Size[1]))
	fillRect(img, img.Rect, key)
	for _, c := range sh.Cells {
		r := image.Rect(c.Rect[0], c.Rect[1], c.Rect[2], c.Rect[3])
		strokeRect(img, r, 2, color.Black)
		label := c.ID + " " + c.Slot
		scale := 2
		if textWidth(label, scale) > r.Dx()+cellGutter {
			scale = 1
		}
		drawText(img, r.Min.X, r.Max.Y+4, label, scale, color.Black)
	}
	return img
}
