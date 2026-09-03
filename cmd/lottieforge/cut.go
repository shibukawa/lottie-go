package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// cellReport is cut's verdict on one cell
// (.knowledge data:character-forge-spec report.json).
type cellReport struct {
	Sheet  string `json:"sheet"`
	ID     string `json:"id"`
	Slot   string `json:"slot"`
	Status string `json:"status"`
	Blobs  int    `json:"blobs"`
	BBox   [4]int `json:"bbox,omitempty"`
	Note   string `json:"note,omitempty"`
	File   string `json:"file,omitempty"`
}

type cutReport struct {
	Cells   []cellReport `json:"cells"`
	Derived []string     `json:"derived"`
	Missing []string     `json:"missing_sheets,omitempty"`
}

func runCut(args []string) error {
	fs := flag.NewFlagSet("cut", flag.ContinueOnError)
	presets := fs.String("presets", "", "directory holding the base presets")
	free := fs.Bool("free", false, "ignore the grid: take every blob of a sheet and assign them to its cells top-to-bottom, left-to-right")
	order := fs.String("order", "", "with -free: comma-separated slot order to assign blobs to")
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
	m, err := loadManifest(work)
	if err != nil {
		return err
	}
	key, err := parseHex(m.Key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(work, "parts"), 0o755); err != nil {
		return err
	}
	rep := &cutReport{}
	for _, sh := range m.Sheets {
		if len(sh.Cells) == 0 {
			continue
		}
		img, path := findSheet(work, sh.ID)
		if img == nil {
			rep.Missing = append(rep.Missing, sh.ID)
			continue
		}
		sx := float64(img.Rect.Dx()) / float64(sh.Size[0])
		sy := float64(img.Rect.Dy()) / float64(sh.Size[1])
		if *free {
			cutFree(work, sh, img, key, strings.Split(*order, ","), rep)
			continue
		}
		for _, c := range sh.Cells {
			r := image.Rect(int(float64(c.Rect[0])*sx), int(float64(c.Rect[1])*sy), int(float64(c.Rect[2])*sx), int(float64(c.Rect[3])*sy))
			margin := int(float64(m.Margin) * sx)
			cr := cutCell(work, sh.ID, c, img, r, margin, key)
			rep.Cells = append(rep.Cells, cr)
		}
		_ = path
	}
	if err := derive(work, spec, *presets, rep); err != nil {
		return err
	}
	if err := contactSheet(work); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(rep, "", " ")
	if err := os.WriteFile(filepath.Join(work, "report.json"), raw, 0o644); err != nil {
		return err
	}
	bad := 0
	for _, c := range rep.Cells {
		if c.Status != "ok" {
			bad++
			fmt.Printf("%s %s %s: %s %s\n", c.Sheet, c.ID, c.Slot, c.Status, c.Note)
		}
	}
	fmt.Printf("cut %d cells (%d flagged), %d derived; report.json and contact.png written\n", len(rep.Cells), bad, len(rep.Derived))
	for _, s := range rep.Missing {
		fmt.Printf("sheet %s: no sheets/%s.png yet\n", s, s)
	}
	return nil
}

func findSheet(work, id string) (*image.NRGBA, string) {
	for _, ext := range []string{".png", ".webp", ".jpg", ".jpeg"} {
		p := filepath.Join(work, "sheets", id+ext)
		if _, err := os.Stat(p); err == nil {
			img, err := loadImage(p)
			if err == nil {
				return img, p
			}
		}
	}
	return nil, ""
}

// cutCell keys one cell, judges it, and writes its largest blob.
func cutCell(work, sheet string, c cellDef, img *image.NRGBA, r image.Rectangle, margin int, key color.NRGBA) cellReport {
	rep := cellReport{Sheet: sheet, ID: c.ID, Slot: c.Slot, Status: "ok"}
	r = r.Intersect(img.Rect)
	if r.Empty() {
		rep.Status, rep.Note = "empty", "cell outside the image"
		return rep
	}
	inner := r.Inset(margin + 2) // the border line itself is 2 px inside the rect
	sub := keyOut(cropImage(img, inner), key)
	w, h := sub.Rect.Dx(), sub.Rect.Dy()
	mask := alphaMask(sub, 128)
	comps := components(mask, w, h)
	minArea := w * h / 500
	var kept []component
	for _, cp := range comps {
		if cp.area >= minArea {
			kept = append(kept, cp)
		}
	}
	rep.Blobs = len(kept)
	if len(kept) == 0 {
		rep.Status, rep.Note = "empty", "nothing but the key color inside the cell"
		return rep
	}
	main := kept[0]
	// Border: alpha in the outermost rows and columns of the drawable area.
	b := main.box
	if b.Min.X <= 1 || b.Min.Y <= 1 || b.Max.X >= w-1 || b.Max.Y >= h-1 {
		rep.Status, rep.Note = "border", "the drawing touches the cell border"
	}
	if len(kept) > 1 && c.Blobs <= 1 {
		total := 0
		for _, cp := range kept {
			total += cp.area
		}
		if float64(kept[1].area) > 0.03*float64(total) {
			if rep.Status == "ok" {
				rep.Status = "multi"
			}
			rep.Note = strings.TrimSpace(rep.Note + fmt.Sprintf(" %d pieces; kept the largest", len(kept)))
		}
	}
	part := keepComponent(sub, main)
	if c.Blobs > 1 || c.Blobs == 0 && len(kept) > 1 {
		part = sub
	}
	soft, any := 0, 0
	for i := 3; i < len(part.Pix); i += 4 {
		if part.Pix[i] > 10 {
			any++
			if part.Pix[i] < 245 {
				soft++
			}
		}
	}
	if any > 0 && float64(soft)/float64(any) > 0.3 && rep.Status == "ok" {
		rep.Status, rep.Note = "halo", "soft edges or a glow around the drawing"
	}
	box := alphaBounds(part)
	part = cropImage(part, box)
	rep.BBox = [4]int{box.Min.X + inner.Min.X, box.Min.Y + inner.Min.Y, box.Max.X + inner.Min.X, box.Max.Y + inner.Min.Y}
	rep.File = "parts/" + c.Slot + ".png"
	if err := writePNG(filepath.Join(work, "parts", c.Slot+".png"), part); err != nil {
		rep.Status, rep.Note = "error", err.Error()
	}
	return rep
}

// cutFree assigns a sheet's blobs to its cells in reading order.
func cutFree(work string, sh sheetDef, img *image.NRGBA, key color.NRGBA, order []string, rep *cutReport) {
	keyed := keyOut(img, key)
	w, h := keyed.Rect.Dx(), keyed.Rect.Dy()
	comps := components(alphaMask(keyed, 128), w, h)
	var kept []component
	for _, c := range comps {
		if c.area >= w*h/2000 {
			kept = append(kept, c)
		}
	}
	// Reading order: rows of roughly a sixth of the sheet, then x.
	band := h / 6
	sort.SliceStable(kept, func(i, j int) bool {
		ri, rj := kept[i].box.Min.Y/band, kept[j].box.Min.Y/band
		if ri != rj {
			return ri < rj
		}
		return kept[i].box.Min.X < kept[j].box.Min.X
	})
	slots := make([]string, 0, len(sh.Cells))
	if len(order) > 0 && order[0] != "" {
		slots = order
	} else {
		for _, c := range sh.Cells {
			slots = append(slots, c.Slot)
		}
	}
	for i, slotName := range slots {
		cr := cellReport{Sheet: sh.ID, ID: fmt.Sprintf("free%d", i+1), Slot: slotName, Status: "ok", Blobs: 1}
		if i >= len(kept) {
			cr.Status, cr.Note = "empty", "fewer blobs than slots"
			rep.Cells = append(rep.Cells, cr)
			continue
		}
		part := cropImage(keepComponent(keyed, kept[i]), kept[i].box)
		cr.BBox = [4]int{kept[i].box.Min.X, kept[i].box.Min.Y, kept[i].box.Max.X, kept[i].box.Max.Y}
		cr.File = "parts/" + slotName + ".png"
		cr.Note = "assigned by reading order; confirm on contact.png"
		if err := writePNG(filepath.Join(work, "parts", slotName+".png"), part); err != nil {
			cr.Status, cr.Note = "error", err.Error()
		}
		rep.Cells = append(rep.Cells, cr)
	}
	fmt.Printf("sheet %s: %d blobs for %d slots\n", sh.ID, len(kept), len(slots))
}

// derive writes the parts nobody draws: far-side copies and the shadow.
func derive(work string, spec *Spec, presets string, rep *cutReport) error {
	basePath, err := resolveBase(spec.Base, presets)
	if err != nil {
		return err
	}
	r, err := loadRig(basePath)
	if err != nil {
		return err
	}
	partPath := func(name string) string { return filepath.Join(work, "parts", name+".png") }
	for _, s := range r.slots {
		switch {
		case s.nearOf != "":
			near := r.byName[s.nearOf]
			if near == nil {
				continue
			}
			img, err := loadImage(partPath(near.cellName()))
			if err != nil {
				continue
			}
			if err := writePNG(partPath(s.name), darken(img, 0.72)); err != nil {
				return err
			}
			rep.Derived = append(rep.Derived, s.name)
		case s.category == "shadow":
			if data, ok := r.bundle.Image(s.file); ok {
				if err := os.WriteFile(partPath(s.name), data, 0o644); err != nil {
					return err
				}
				rep.Derived = append(rep.Derived, s.name)
			}
		}
	}
	for _, a := range spec.Attachments {
		if !a.Paired {
			continue
		}
		img, err := loadImage(partPath(a.Name))
		if err != nil {
			continue
		}
		if err := writePNG(partPath(a.Name+"-far"), darken(img, 0.72)); err != nil {
			return err
		}
		rep.Derived = append(rep.Derived, a.Name+"-far")
	}
	return nil
}

// contactSheet tiles every part with its name, for the agent to look at.
func contactSheet(work string) error {
	files, _ := filepath.Glob(filepath.Join(work, "parts", "*.png"))
	sort.Strings(files)
	if len(files) == 0 {
		return nil
	}
	const tile, cols = 160, 6
	rows := (len(files) + cols - 1) / cols
	img := image.NewRGBA(image.Rect(0, 0, cols*tile, rows*tile))
	fillRect(img, img.Rect, color.NRGBA{40, 40, 48, 255})
	for i, f := range files {
		part, err := loadImage(f)
		if err != nil {
			continue
		}
		x, y := (i%cols)*tile, (i/cols)*tile
		w, h := part.Rect.Dx(), part.Rect.Dy()
		k := float64(tile-40) / float64(max(w, h))
		if k > 1 {
			k = 1
		}
		tw, th := max(int(float64(w)*k), 1), max(int(float64(h)*k), 1)
		scaled := resizeTo(part, tw, th, false)
		ox, oy := x+(tile-tw)/2, y+8+(tile-32-th)/2
		drawOver(img, scaled, ox, oy)
		name := strings.TrimSuffix(filepath.Base(f), ".png")
		drawText(img, x+4, y+tile-18, name, 1, color.White)
	}
	return writePNG(filepath.Join(work, "contact.png"), img)
}

func drawOver(dst *image.RGBA, src *image.NRGBA, x, y int) {
	r := image.Rect(x, y, x+src.Rect.Dx(), y+src.Rect.Dy())
	drawImage(dst, r, src)
}
