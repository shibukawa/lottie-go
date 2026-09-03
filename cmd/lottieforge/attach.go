package main

import (
	"fmt"
	"strings"
)

// attLayer is one layer an attachment becomes: the base drawing, its far
// copy, a panel, a view variant or a second segment
// (concept:attachment-kinds).
type attLayer struct {
	name       string // layer name and texture name
	att        *Attachment
	parentName string // a rig slot or another attachment layer
	host       *slot  // the rig slot whose opacity it copies
	cellName   string // the drawing it is cut from
	crop       *[4]float64
	dark       float64 // color multiplier for derived copies
	w, h       float64 // slot-space size
	anchor     pt
	attach     pt
	order      string // "in-front-of-<layer>" or "behind-<layer>"
	viewOf     string // the host view slot it follows, for views separate
	segment    int    // 1 or 2
}

// expandAttachments turns the spec's attachments into layers.
func expandAttachments(spec *Spec, r *rig) ([]*attLayer, error) {
	var out []*attLayer
	for _, a := range spec.Attachments {
		host := r.hostSlot(a.Host, false)
		if host == nil {
			return nil, fmt.Errorf("attachment %s: host %q is not a slot of %s", a.Name, a.Host, spec.Base)
		}
		w, h := a.Size[0], a.Size[1]
		anchor := pt{w / 2, 0}
		if len(a.Anchor) == 2 {
			anchor = pt{a.Anchor[0], a.Anchor[1]}
		}
		attach := defaultAttach(a, host)
		order := a.Order
		if order == "" {
			order = "in-front-of-" + host.name
			if a.Kind == "drape" && host.category == "body" && r.hasSlot("thigh-near") {
				order = "in-front-of-thigh-near"
			}
		}
		base := &attLayer{name: a.Name, att: a, parentName: host.name, host: host, cellName: a.Name,
			w: w, h: h, anchor: anchor, attach: attach, order: order, dark: 1, segment: 1}
		variants := []*attLayer{base}
		if a.Paired {
			far := *base
			far.name = a.Name + "-far"
			far.dark = 0.72
			if fh := r.hostSlot(a.Host, true); fh != nil && fh != host {
				far.parentName, far.host = fh.name, fh
			} else {
				// Same host: mirror across the host's anchor and go behind.
				far.attach = pt{2*host.anchor[0] - attach[0], attach[1]}
			}
			far.order = "behind-" + far.host.name
			variants = append(variants, &far)
		}
		if a.Kind == "drape" && a.Panels >= 2 {
			var withPanels []*attLayer
			for _, v := range variants {
				front := *v
				front.name = v.name + "-front"
				front.crop = &[4]float64{0, 0, 0.55, 1}
				back := *v
				back.name = v.name + "-back"
				back.dark = v.dark * 0.9
				back.order = "behind-" + v.host.name
				if r.hasSlot("thigh-far") && v.host.category == "body" {
					back.order = "behind-thigh-far"
				}
				withPanels = append(withPanels, &front, &back)
			}
			variants = withPanels
		}
		if (a.Kind == "swing" || a.Kind == "lock") && a.Segments >= 2 {
			var withSegs []*attLayer
			for _, v := range variants {
				top := *v
				top.h = v.h / 2
				top.crop = &[4]float64{0, 0, 1, 0.5}
				bottom := *v
				bottom.name = v.name + "-2"
				bottom.h = v.h / 2
				bottom.crop = &[4]float64{0, 0.5, 1, 1}
				bottom.parentName = top.name
				bottom.attach = pt{top.anchor[0], top.h}
				bottom.order = "in-front-of-" + top.name
				bottom.segment = 2
				withSegs = append(withSegs, &top, &bottom)
			}
			variants = withSegs
		}
		out = append(out, variants...)
		if a.Views == "separate" {
			for _, v := range r.views(host.name) {
				suffix := v.name[strings.LastIndex(v.name, "-"):]
				vl := *base
				vl.name = a.Name + suffix
				vl.cellName = a.Name + suffix
				vl.parentName, vl.host, vl.viewOf = v.name, v, v.name
				vl.order = strings.Replace(order, host.name, v.name, 1)
				out = append(out, &vl)
			}
		}
	}
	return out, nil
}

func defaultAttach(a *Attachment, host *slot) pt {
	if len(a.Attach) == 2 {
		return pt{a.Attach[0], a.Attach[1]}
	}
	switch {
	case a.Kind == "drape" && host.category == "body":
		return pt{float64(host.w) / 2, float64(host.h) * 0.65}
	case host.category == "head":
		return pt{float64(host.w) / 2, 0}
	case host.category == "body":
		return pt{float64(host.w) / 2, float64(host.h) * 0.2}
	}
	return host.anchor
}

// orderTarget splits an order string into (before, layer name).
func orderTarget(order string) (before bool, target string) {
	if strings.HasPrefix(order, "in-front-of-") {
		return true, strings.TrimPrefix(order, "in-front-of-")
	}
	if strings.HasPrefix(order, "behind-") {
		return false, strings.TrimPrefix(order, "behind-")
	}
	return true, order
}
