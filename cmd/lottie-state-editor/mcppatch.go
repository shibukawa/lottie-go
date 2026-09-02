package main

// A small RFC 6902 JSON Patch, enough for an agent to change a clip or a
// machine document without resending it whole: add, remove, replace, move,
// copy, test. Pointers follow RFC 6901.

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

type patchOp struct {
	Op    string `json:"op" jsonschema:"add|remove|replace|move|copy|test"`
	Path  string `json:"path"`
	From  string `json:"from,omitempty"`
	Value any    `json:"value,omitempty"`
}

func applyPatch(doc any, ops []patchOp) (any, error) {
	for i, op := range ops {
		var err error
		switch op.Op {
		case "add":
			doc, err = ptrSet(doc, op.Path, op.Value, true)
		case "replace":
			if _, gerr := ptrGet(doc, op.Path); gerr != nil {
				err = gerr
				break
			}
			doc, err = ptrSet(doc, op.Path, op.Value, false)
		case "remove":
			doc, err = ptrRemove(doc, op.Path)
		case "move", "copy":
			var v any
			v, err = ptrGet(doc, op.From)
			if err != nil {
				break
			}
			v = deepCopyJSON(v)
			if op.Op == "move" {
				doc, err = ptrRemove(doc, op.From)
				if err != nil {
					break
				}
			}
			doc, err = ptrSet(doc, op.Path, v, true)
		case "test":
			var v any
			v, err = ptrGet(doc, op.Path)
			if err == nil && !reflect.DeepEqual(normalize(v), normalize(op.Value)) {
				err = fmt.Errorf("test failed at %s", op.Path)
			}
		default:
			err = fmt.Errorf("unknown op %q", op.Op)
		}
		if err != nil {
			return nil, fmt.Errorf("patch %d (%s %s): %w", i, op.Op, op.Path, err)
		}
	}
	return doc, nil
}

// normalize re-encodes so json.Number and float64 compare equal.
func normalize(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	json.Unmarshal(b, &out)
	return out
}

func splitPointer(p string) ([]string, error) {
	if p == "" {
		return nil, nil
	}
	if !strings.HasPrefix(p, "/") {
		return nil, fmt.Errorf("pointer %q must start with /", p)
	}
	parts := strings.Split(p[1:], "/")
	for i, s := range parts {
		s = strings.ReplaceAll(s, "~1", "/")
		parts[i] = strings.ReplaceAll(s, "~0", "~")
	}
	return parts, nil
}

func ptrGet(doc any, p string) (any, error) {
	parts, err := splitPointer(p)
	if err != nil {
		return nil, err
	}
	cur := doc
	for _, key := range parts {
		switch c := cur.(type) {
		case map[string]any:
			v, ok := c[key]
			if !ok {
				return nil, fmt.Errorf("no member %q", key)
			}
			cur = v
		case []any:
			i, err := strconv.Atoi(key)
			if err != nil || i < 0 || i >= len(c) {
				return nil, fmt.Errorf("index %q out of range", key)
			}
			cur = c[i]
		default:
			return nil, fmt.Errorf("cannot descend into %q", key)
		}
	}
	return cur, nil
}

// ptrSet writes v at p. With insert, an array index inserts (and "-"
// appends); otherwise it replaces.
func ptrSet(doc any, p string, v any, insert bool) (any, error) {
	parts, err := splitPointer(p)
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 {
		return v, nil
	}
	parent, err := ptrGet(doc, "/"+strings.Join(escapeAll(parts[:len(parts)-1]), "/"))
	if len(parts) == 1 {
		parent, err = doc, nil
	}
	if err != nil {
		return nil, err
	}
	last := parts[len(parts)-1]
	switch c := parent.(type) {
	case map[string]any:
		c[last] = v
		return doc, nil
	case []any:
		var i int
		if last == "-" {
			i = len(c)
		} else if i, err = strconv.Atoi(last); err != nil || i < 0 || i > len(c) || (!insert && i == len(c)) {
			return nil, fmt.Errorf("index %q out of range", last)
		}
		var next []any
		if insert {
			next = append(append(append([]any{}, c[:i]...), v), c[i:]...)
		} else {
			next = append([]any{}, c...)
			next[i] = v
		}
		return ptrReplaceArray(doc, parts[:len(parts)-1], next)
	}
	return nil, fmt.Errorf("cannot set member of %T", parent)
}

func ptrRemove(doc any, p string) (any, error) {
	parts, err := splitPointer(p)
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("cannot remove the document")
	}
	parent, err := ptrGet(doc, "/"+strings.Join(escapeAll(parts[:len(parts)-1]), "/"))
	if len(parts) == 1 {
		parent, err = doc, nil
	}
	if err != nil {
		return nil, err
	}
	last := parts[len(parts)-1]
	switch c := parent.(type) {
	case map[string]any:
		if _, ok := c[last]; !ok {
			return nil, fmt.Errorf("no member %q", last)
		}
		delete(c, last)
		return doc, nil
	case []any:
		i, err := strconv.Atoi(last)
		if err != nil || i < 0 || i >= len(c) {
			return nil, fmt.Errorf("index %q out of range", last)
		}
		next := append(append([]any{}, c[:i]...), c[i+1:]...)
		return ptrReplaceArray(doc, parts[:len(parts)-1], next)
	}
	return nil, fmt.Errorf("cannot remove member of %T", parent)
}

// ptrReplaceArray stores a rebuilt array back into its parent, since a
// slice header cannot be changed in place.
func ptrReplaceArray(doc any, parts []string, arr []any) (any, error) {
	if len(parts) == 0 {
		return arr, nil
	}
	parent, err := ptrGet(doc, "/"+strings.Join(escapeAll(parts[:len(parts)-1]), "/"))
	if len(parts) == 1 {
		parent, err = doc, nil
	}
	if err != nil {
		return nil, err
	}
	last := parts[len(parts)-1]
	switch c := parent.(type) {
	case map[string]any:
		c[last] = arr
		return doc, nil
	case []any:
		i, err := strconv.Atoi(last)
		if err != nil || i < 0 || i >= len(c) {
			return nil, fmt.Errorf("index %q out of range", last)
		}
		c[i] = arr
		return doc, nil
	}
	return nil, fmt.Errorf("cannot store array into %T", parent)
}

func escapeAll(parts []string) []string {
	out := make([]string, len(parts))
	for i, s := range parts {
		s = strings.ReplaceAll(s, "~", "~0")
		out[i] = strings.ReplaceAll(s, "/", "~1")
	}
	return out
}
