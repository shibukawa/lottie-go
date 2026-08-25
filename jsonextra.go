package lottie

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"
)

// extraFields holds object members that the enclosing Go struct does not
// model. The dotLottie state machine schema is still growing, so a document
// this package rewrites must carry members it does not understand through
// untouched; otherwise editing a bundle here would quietly strip features
// other runtimes rely on.
type extraFields map[string]json.RawMessage

var knownKeysCache sync.Map // reflect.Type -> map[string]struct{}

// knownKeys returns the JSON member names covered by t's own fields.
func knownKeys(t reflect.Type) map[string]struct{} {
	if v, ok := knownKeysCache.Load(t); ok {
		return v.(map[string]struct{})
	}
	keys := make(map[string]struct{}, t.NumField())
	for i := range t.NumField() {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		keys[name] = struct{}{}
	}
	knownKeysCache.Store(t, keys)
	return keys
}

// decodeExtra returns the members of data that v's fields do not cover. v
// must be a struct value.
func decodeExtra(data []byte, v any) (extraFields, error) {
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, err
	}
	for k := range knownKeys(reflect.TypeOf(v)) {
		delete(all, k)
	}
	if len(all) == 0 {
		return nil, nil
	}
	return all, nil
}

// encodeExtra marshals v and merges extra back into the object. Modeled
// fields win, so a stale extra member can never shadow one written here.
func encodeExtra(v any, extra extraFields) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(extra) == 0 {
		return data, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	for k, raw := range extra {
		if _, ok := m[k]; !ok {
			m[k] = raw
		}
	}
	return json.Marshal(m)
}
