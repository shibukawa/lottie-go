package main

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// The prompts of skills/lottie-character-forge/references/prompts.md,
// filled from the spec. Kept as text: a model that needs other wording
// gets an edit here, not a code change (decision:image-gen-by-prompt).

const modelPrompt = `# Model sheet prompt (P1)

Run this first, text to image. Save the result as sheets/model.png.

` + "```" + `
Character design reference for a 2D game sprite, to be cut into animation parts.

Subject: {{.Description}}.
Style: {{.Style}}. {{.Proportions}}.

Pose: standing still, relaxed A-pose. Arms hang straight down, slightly away from the body so they do not touch the torso. Legs straight and slightly apart, both feet flat on the same line, toes pointing to the RIGHT. Head level, looking straight ahead.
View: three-quarter view facing RIGHT — both eyes visible, the front of the torso visible, the character's own right side is the far side.
Background: one flat, uniform {{.KeyName}} ({{.Key}}) color filling the whole image. No gradient, no floor line, no cast shadow, no drop shadow, no vignette, no frame, no border.
Rendering: crisp edges with no glow, no outer blur, no soft shadow around the character, no motion lines, no sparkles, no text, no labels, no watermark, no extra objects or props other than the ones described.
Layout: the character centered, about 85% of the image height, nothing cropped.
` + "```" + `

Check before going on: view, facing, feet, background, cropping, and that
every part the spec names is there (a held prop counts).

## Turnaround (optional, P1b)

Only when the back of the design matters (a cape, a tail, a pattern).

` + "```" + `
Same character as the attached reference, same style and colors, same A-pose.
Three views side by side, same scale, on one flat {{.KeyName}} ({{.Key}}) background: (1) front three-quarter view facing right, (2) rear three-quarter view facing right — mostly the back of the head and torso, one eye just visible at the leading edge, (3) full back view.
No shadow, no text, no labels, nothing else.
` + "```" + `
`

const partPrompt = `# {{.Sheet}} sheet prompt (P2)

Attach sheets/model.png as (1) and sheets/{{.Sheet}}.template.png as (2).
Save the result as sheets/{{.Sheet}}.png.

` + "```" + `
Attached: (1) the character reference, (2) a grid template.

Task: redraw the character from (1) as SEPARATE BODY PARTS, one part per cell of the grid in (2), like an exploded cutout-puppet sheet for 2D animation.
Keep the template exactly as it is — its flat {{.KeyName}} ({{.Key}}) background, the black cell borders and the label text under each cell. Draw nothing on the borders or in the label bands. Draw each part entirely inside its own cell, with clear space between the part and the cell border.

Cells, row by row:
{{.CellList}}

Rules for every part:
- Same character, same colors, same outline weight, same rendering as the reference. Do not restyle.
- Draw each part COMPLETE, including what other parts normally cover: the whole upper arm even where the torso would overlap it, the whole neck stump under the head, the whole thigh up to the hip.
- Limb segments hang STRAIGHT DOWN, vertical, with the joint at the TOP center of the cell. Upper arm = shoulder to elbow. Forearm = elbow to fingertips, hand included. Thigh = hip to knee. Shin = knee to toes, foot included, foot pointing RIGHT.
- Every limb segment ends in a ROUNDED CAP that extends a little past the joint at BOTH ends, so neighbouring segments overlap when a joint bends. No flat cut-off ends.
- The torso is drawn from the neck stump at the top to the hips at the bottom, hip at the bottom center of the cell, without arms and without legs.
- The head is drawn with all of its hair and a short neck stump, neck at the bottom center of the cell.
- A cell that says WITHOUT something: draw that part complete as if the item were not worn (the hips and belt line under a skirt, the scalp under a ponytail); the item has its own cell. A cell that says AS WORN: include those items in that view.
- Hanging items (hair locks, ribbons, tails, skirts, sleeves, capes) are drawn complete, hanging STRAIGHT DOWN, their root or waistband at the TOP center of the cell, slightly wider at the root than where they attach so they overlap the part they hang from.
- Each part fills its cell from top to bottom; the cells are already sized so the parts keep the reference's proportions — do not enlarge a small part to fill a wide cell.
- Flat {{.KeyName}} background inside every cell. No shadow, no glow, no soft edges, no text, no labels inside the cells, no arrows, no numbers, no extra parts.
` + "```" + `

## Without a template image (P2-text)

For a model that ignores attached grids. Then cut with ` + "`lottieforge cut -free`" + `
and confirm the slot order against contact.png.

` + "```" + `
Attached: the character reference.
Redraw the character as SEPARATE BODY PARTS on one flat {{.KeyName}} ({{.Key}}) background, arranged in {{.Rows}} rows, left to right, top to bottom, with wide empty space between parts so that no two parts touch:
{{.RowList}}
(then the rules block above, unchanged)
` + "```" + `
`

const fixPrompt = `# Fix one cell (P3)

Attach the sheet that failed as the image to edit. Fill in the cell,
the slot and the instruction from report.json's status:

` + "```" + `
Edit the attached sheet. Change ONLY cell {{"{{cell}}"}} ({{"{{slot}}"}}): {{"{{reason}}"}}. Redraw that part {{"{{instruction}}"}}, entirely inside its cell with space to the border, on the flat {{.KeyName}} background. Leave every other cell, the borders and the labels exactly as they are.
` + "```" + `

| status | reason | instruction |
|--------|--------|-------------|
| empty  | the cell is empty | as listed for that cell |
| border | the drawing touches or crosses the cell border | smaller, centered, with clear space all round |
| multi  | it is drawn as several pieces | as ONE connected piece (hand attached to the forearm, no separate details) |
| halo   | it has a soft shadow or glow around it | with crisp edges and no shadow or glow |
| (fit)  | the joint end is cut flat / the part is diagonal | hanging vertically with rounded caps past both joints |

## One item on its own (P4)

` + "```" + `
Same character and style as the attached reference. Draw only {{"{{part}}"}}: {{"{{cell line}}"}}. One flat {{.KeyName}} ({{.Key}}) background, the object centered and filling about 80% of the image height, crisp edges, no shadow, no text.
` + "```" + `
`

type promptData struct {
	Description, Style, Proportions, Key, KeyName string
	Sheet, CellList, RowList                      string
	Rows                                          int
}

func writePrompts(work string, spec *Spec, m *manifest, key color.NRGBA) error {
	base := promptData{Description: spec.Description, Style: spec.Style, Proportions: spec.Proportions,
		Key: strings.ToUpper(spec.Key), KeyName: colorName(key)}
	if base.Description == "" {
		base.Description = spec.Name
	}
	write := func(name, tmpl string, data promptData) error {
		t, err := template.New(name).Parse(tmpl)
		if err != nil {
			return err
		}
		var sb strings.Builder
		if err := t.Execute(&sb, data); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(work, "prompts", name+".md"), []byte(sb.String()), 0o644)
	}
	if err := write("model", modelPrompt, base); err != nil {
		return err
	}
	for _, sh := range m.Sheets {
		if len(sh.Cells) == 0 {
			continue
		}
		d := base
		d.Sheet = sh.ID
		var lines, rows []string
		var row []string
		lastY := -1
		for _, c := range sh.Cells {
			lines = append(lines, fmt.Sprintf("%s %s — %s", c.ID, c.Slot, c.Line))
			if c.Rect[1] != lastY && len(row) > 0 {
				rows = append(rows, "Row "+string(rune('A'+len(rows)))+": "+strings.Join(row, ", "))
				row = nil
			}
			lastY = c.Rect[1]
			row = append(row, c.Slot+" ("+c.Line+")")
		}
		if len(row) > 0 {
			rows = append(rows, "Row "+string(rune('A'+len(rows)))+": "+strings.Join(row, ", "))
		}
		d.CellList = strings.Join(lines, "\n")
		d.RowList = strings.Join(rows, "\n")
		d.Rows = len(rows)
		if err := write(sh.ID, partPrompt, d); err != nil {
			return err
		}
	}
	return write("fix-cell", fixPrompt, base)
}
