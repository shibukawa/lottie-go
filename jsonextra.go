package lottie

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"
)

// ExtraFields holds object members that the enclosing Go struct does not
// model. It is exported so a tool rewriting a document can stash its own
// data in one — an editor's node positions, say — and have it survive. The dotLottie state machine schema is still growing, so a document
// this package rewrites must carry members it does not understand through
// untouched; otherwise editing a bundle here would quietly strip features
// other runtimes rely on.
type ExtraFields map[string]json.RawMessage

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

// MarshalWithExtra marshals v and merges extra back into the object, the
// way this package's own types round-trip unknown members. It is exported
// for plugin packages that store their own documents in a bundle — the
// collision plugins under plugin/ — so their types can carry foreign
// members the same way. Modeled fields win over stale extra members.
func MarshalWithExtra(v any, extra ExtraFields) ([]byte, error) {
	return encodeExtra(v, extra)
}

// UnmarshalExtra returns the members of data that v's fields do not cover;
// the counterpart of MarshalWithExtra for decoding. v must be the struct
// value data was just unmarshaled into.
func UnmarshalExtra(data []byte, v any) (ExtraFields, error) {
	return decodeExtra(data, v)
}

// decodeExtra returns the members of data that v's fields do not cover. v
// must be a struct value.
func decodeExtra(data []byte, v any) (ExtraFields, error) {
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
func encodeExtra(v any, extra ExtraFields) ([]byte, error) {
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
