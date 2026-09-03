package lottiespine

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// Atlas is a parsed Spine texture atlas (.atlas): the pages it packs and
// the regions on them, in both the 4.x ("bounds:", "offsets:") and the
// older ("xy:", "size:", "orig:", "offset:") spellings.
type Atlas struct {
	Pages []*AtlasPage
}

// AtlasPage is one packed image.
type AtlasPage struct {
	Name          string // the image file name, relative to the atlas
	Width, Height int    // from the size line; 0 when absent, then read from the image
	PMA           bool
	Regions       []*AtlasRegion
}

// AtlasRegion is one packed image on a page. Width and Height are the
// packed size before rotation; a Rotate of 90 means the region occupies
// Height by Width pixels on the page, rotated clockwise. OrigWidth,
// OrigHeight and OffsetX, OffsetY describe the whitespace stripped when
// packing, OffsetY measured from the bottom as Spine does.
type AtlasRegion struct {
	Name                  string
	Page                  *AtlasPage
	X, Y, Width, Height   float64
	Rotate                int
	OrigWidth, OrigHeight float64
	OffsetX, OffsetY      float64
	Index                 int
}

// ParseAtlas reads an atlas file.
func ParseAtlas(data []byte) (*Atlas, error) {
	atlas := &Atlas{}
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	var page *AtlasPage
	var region *AtlasRegion
	expectPage := true
	line := 0
	for sc.Scan() {
		line++
		text := sc.Text()
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			expectPage = true
			continue
		}
		key, values, isEntry := splitEntry(trimmed)
		// Settings are indented under their page or region; a bare line
		// names the next one. Unindented files are tolerated by treating a
		// known setting key as a setting.
		setting := isEntry && (text != trimmed || settingKey(key))
		switch {
		case !setting && expectPage:
			page = &AtlasPage{Name: trimmed}
			atlas.Pages = append(atlas.Pages, page)
			region = nil
			expectPage = false
		case page == nil:
			return nil, fmt.Errorf("lottiespine: atlas line %d: region before any page", line)
		case !setting:
			region = &AtlasRegion{Name: trimmed, Page: page, Index: -1}
			page.Regions = append(page.Regions, region)
		case region == nil:
			switch key {
			case "size":
				if len(values) == 2 {
					page.Width, _ = strconv.Atoi(values[0])
					page.Height, _ = strconv.Atoi(values[1])
				}
			case "pma":
				page.PMA = values[0] == "true"
			}
		default:
			nums := make([]float64, len(values))
			for i, v := range values {
				nums[i], _ = strconv.ParseFloat(v, 64)
			}
			switch key {
			case "bounds":
				if len(nums) == 4 {
					region.X, region.Y, region.Width, region.Height = nums[0], nums[1], nums[2], nums[3]
				}
			case "xy":
				if len(nums) == 2 {
					region.X, region.Y = nums[0], nums[1]
				}
			case "size":
				if len(nums) == 2 {
					region.Width, region.Height = nums[0], nums[1]
				}
			case "offsets":
				if len(nums) == 4 {
					region.OffsetX, region.OffsetY, region.OrigWidth, region.OrigHeight = nums[0], nums[1], nums[2], nums[3]
				}
			case "offset":
				if len(nums) == 2 {
					region.OffsetX, region.OffsetY = nums[0], nums[1]
				}
			case "orig":
				if len(nums) == 2 {
					region.OrigWidth, region.OrigHeight = nums[0], nums[1]
				}
			case "rotate":
				switch values[0] {
				case "true":
					region.Rotate = 90
				case "false":
					region.Rotate = 0
				default:
					region.Rotate, _ = strconv.Atoi(values[0])
				}
			case "index":
				region.Index, _ = strconv.Atoi(values[0])
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("lottiespine: atlas: %w", err)
	}
	for _, p := range atlas.Pages {
		for _, r := range p.Regions {
			if r.OrigWidth == 0 {
				r.OrigWidth = r.Width
			}
			if r.OrigHeight == 0 {
				r.OrigHeight = r.Height
			}
		}
	}
	if len(atlas.Pages) == 0 {
		return nil, fmt.Errorf("lottiespine: atlas has no pages")
	}
	return atlas, nil
}

func settingKey(key string) bool {
	switch key {
	case "size", "format", "filter", "repeat", "pma", "scale",
		"bounds", "xy", "offsets", "offset", "orig", "rotate", "index", "split", "pad":
		return true
	}
	return false
}

// splitEntry splits "key: a, b" into its key and trimmed values.
func splitEntry(s string) (key string, values []string, ok bool) {
	i := strings.Index(s, ":")
	if i < 0 {
		return "", nil, false
	}
	key = strings.TrimSpace(s[:i])
	for _, v := range strings.Split(s[i+1:], ",") {
		values = append(values, strings.TrimSpace(v))
	}
	return key, values, true
}

// Find returns the region with the given name (an attachment's path). Spine
// also matches a path with its extension stripped.
func (a *Atlas) Find(name string) *AtlasRegion {
	if a == nil {
		return nil
	}
	alt := strings.TrimSuffix(name, ".png")
	for _, p := range a.Pages {
		for _, r := range p.Regions {
			if r.Name == name || r.Name == alt {
				return r
			}
		}
	}
	return nil
}

// pageUV maps a point of the attachment's original (unstripped) image,
// given normalized with v downward, to normalized page coordinates. This
// is the mapping Spine applies to a mesh's uvs and a region's corners:
// the offsets undo the whitespace stripping and a rotated region turns
// through 90 degrees clockwise.
func (r *AtlasRegion) pageUV(u, v float64) (float64, float64) {
	pw, ph := float64(r.Page.Width), float64(r.Page.Height)
	px, py := u*r.OrigWidth, v*r.OrigHeight // original-image pixels, from the top left
	var x, y float64
	if r.Rotate == 90 {
		x = r.X + py - (r.OrigHeight - r.OffsetY - r.Height)
		y = r.Y + r.Width - (px - r.OffsetX)
	} else {
		x = r.X + px - r.OffsetX
		y = r.Y + py - (r.OrigHeight - r.OffsetY - r.Height)
	}
	return x / pw, y / ph
}

// packedRect is the region's non-stripped area in normalized original-image
// coordinates (u right, v down): what a region attachment actually draws.
func (r *AtlasRegion) packedRect() (u0, v0, u1, v1 float64) {
	u0 = r.OffsetX / r.OrigWidth
	v0 = (r.OrigHeight - r.OffsetY - r.Height) / r.OrigHeight
	return u0, v0, u0 + r.Width/r.OrigWidth, v0 + r.Height/r.OrigHeight
}
