package lottietexture

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	lottie "github.com/shibukawa/lottie-go"
)

const testClip = `{
  "v": "5.7.1", "fr": 30, "ip": 0, "op": 60, "w": 200, "h": 200,
  "assets": [
    {"id": "comp_0", "layers": [
      {"ty": 4, "ind": 1, "ip": 0, "op": 60, "st": 0, "ks": {},
       "shapes": [
         {"ty": "sh", "ks": {"a": 0, "k": {"c": true, "v": [[0,0],[10,0],[10,10]], "i": [[0,0],[0,0],[0,0]], "o": [[0,0],[0,0],[0,0]]}}},
         {"ty": "fl", "c": {"a": 0, "k": [1,0,0,1]}, "o": {"a": 0, "k": 100}}
       ]}
    ]}
  ],
  "layers": [
    {"ty": 4, "ind": 1, "nm": "shapes", "ip": 0, "op": 60, "st": 0, "ks": {},
     "shapes": [
       {"ty": "gr", "it": [
         {"ty": "sh", "ks": {"a": 0, "k": {"c": true, "v": [[0,0],[100,0],[100,100],[0,100]], "i": [[0,0],[0,0],[0,0],[0,0]], "o": [[0,0],[0,0],[0,0],[0,0]]}}},
         {"ty": "fl", "c": {"a": 0, "k": [1,1,1,1]}, "o": {"a": 0, "k": 100}},
         {"ty": "tr", "p": {"a": 0, "k": [0,0]}, "a": {"a": 0, "k": [0,0]}, "s": {"a": 0, "k": [100,100]}, "r": {"a": 0, "k": 0}, "o": {"a": 0, "k": 100}}
       ]},
       {"ty": "st", "c": {"a": 0, "k": [0,0,0,1]}, "o": {"a": 0, "k": 100}, "w": {"a": 0, "k": 4}}
     ]},
    {"ty": 0, "ind": 2, "refId": "comp_0", "w": 200, "h": 200, "ip": 0, "op": 60, "st": 0, "ks": {}}
  ]
}`

func testDoc() *Doc {
	off := false
	return &Doc{
		Paints: []Paint{
			{Layer: 1, Item: []int{0, 1}, Texture: "skin", Mapping: MappingVertex, Wrap: WrapRepeat,
				Transform: json.RawMessage(`{"p":{"a":0,"k":[0.5,0.5]},"s":{"a":0,"k":[200,200]}}`),
				Extra:     lottie.ExtraFields{"note": json.RawMessage(`"hand"`)}},
			{Layer: 1, Item: []int{1}, Texture: "brush", Mapping: MappingStroke, Tint: &off},
			{Asset: "comp_0", Layer: 1, Item: []int{1}, Texture: "skin"},
		},
		UVs: []UV{
			{Layer: 1, Item: []int{0, 0}, V: [][2]float64{{0, 0}, {1, 0}, {1, 1}, {0, 1}}},
		},
		Extra: lottie.ExtraFields{"generator": json.RawMessage(`"test"`)},
	}
}

func parseTree(t *testing.T, data string) map[string]any {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(data))
	dec.UseNumber()
	var tree map[string]any
	if err := dec.Decode(&tree); err != nil {
		t.Fatal(err)
	}
	return tree
}

func TestWeaveThenUnweaveRoundTrips(t *testing.T) {
	tree := parseTree(t, testClip)
	d := testDoc()
	if left := Weave(tree, d); left != nil {
		t.Fatalf("entries left unplaced: %+v", left)
	}
	fill := tree["layers"].([]any)[0].(map[string]any)["shapes"].([]any)[0].(map[string]any)["it"].([]any)[1].(map[string]any)
	tex, ok := fill[MemberTex].(map[string]any)
	if !ok {
		t.Fatalf("fill has no %s member: %v", MemberTex, fill)
	}
	if tex["texture"] != "skin" || tex["mapping"] != "vertex" || tex["wrap"] != "repeat" {
		t.Fatalf("woven paint = %v", tex)
	}
	if _, has := tex["asset"]; has {
		t.Fatal("the address leaked into the woven form")
	}
	if tex["note"] != "hand" {
		t.Fatalf("unknown member did not weave: %v", tex["note"])
	}
	back := Unweave(tree)
	if back == nil {
		t.Fatal("nothing unwove")
	}
	// Unweave walks in document order: root layers, then assets.
	if len(back.Paints) != 3 || len(back.UVs) != 1 {
		t.Fatalf("unwove %d paints %d uvs", len(back.Paints), len(back.UVs))
	}
	want, _ := json.Marshal(d.Paints)
	got, _ := json.Marshal(back.Paints)
	if !bytes.Equal(want, got) {
		t.Fatalf("paints changed across the round trip:\n%s\n%s", want, got)
	}
	want, _ = json.Marshal(d.UVs)
	got, _ = json.Marshal(back.UVs)
	if !bytes.Equal(want, got) {
		t.Fatalf("uvs changed across the round trip:\n%s\n%s", want, got)
	}
	// The tree is plain Lottie again.
	out, _ := json.Marshal(tree)
	if bytes.Contains(out, []byte(`"x-`)) {
		t.Fatalf("an x- member survived Unweave: %s", out)
	}
	if Unweave(tree) != nil {
		t.Fatal("a plain tree unwove something")
	}
}

func TestUnplacedEntriesComeBack(t *testing.T) {
	tree := parseTree(t, testClip)
	d := &Doc{Paints: []Paint{
		{Layer: 1, Item: []int{0, 1}, Texture: "ok"},
		{Layer: 9, Item: []int{0}, Texture: "gone-layer"},
		{Layer: 1, Item: []int{0, 7}, Texture: "gone-item"},
		{Asset: "missing", Layer: 1, Item: []int{1}, Texture: "gone-asset"},
	}, UVs: []UV{{Layer: 1, Item: []int{0, 0, 1}, V: nil}}}
	left := Weave(tree, d)
	if left == nil || len(left.Paints) != 3 || len(left.UVs) != 1 {
		t.Fatalf("unplaced = %+v", left)
	}
	if left.Paints[0].Texture != "gone-layer" || left.Paints[2].Texture != "gone-asset" {
		t.Fatalf("unplaced order changed: %+v", left.Paints)
	}
	if back := Unweave(tree); back == nil || len(back.Paints) != 1 || back.Paints[0].Texture != "ok" {
		t.Fatalf("placed entry did not unweave: %+v", back)
	}
}

func TestUnweaveStripsEveryWorkingMember(t *testing.T) {
	tree := parseTree(t, testClip)
	fill := tree["layers"].([]any)[0].(map[string]any)["shapes"].([]any)[0].(map[string]any)["it"].([]any)[1].(map[string]any)
	fill["x-future"] = map[string]any{"a": 1}
	fill[MemberTex] = map[string]any{"texture": "t", "tint": false}
	d := Unweave(tree)
	if _, has := fill["x-future"]; has {
		t.Fatal("an unknown x- member was kept")
	}
	if d == nil || len(d.Paints) != 1 || d.Paints[0].Tinted() {
		t.Fatalf("paint = %+v", d)
	}
}

func TestJSONFormsAndBundleIO(t *testing.T) {
	woven, left, err := WeaveJSON([]byte(testClip), testDoc())
	if err != nil || left != nil {
		t.Fatalf("WeaveJSON: %v %+v", err, left)
	}
	if !bytes.Contains(woven, []byte(`"x-tex"`)) {
		t.Fatal("WeaveJSON produced no x-tex")
	}
	pure, d, err := UnweaveJSON(woven)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(pure, []byte(`"x-`)) {
		t.Fatal("UnweaveJSON left an x- member")
	}
	if d == nil || len(d.Paints) != 3 {
		t.Fatalf("doc = %+v", d)
	}

	b := lottie.NewBundle()
	if err := b.SetAnimation("clip", []byte(testClip)); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(b, "clip"); err != nil || got != nil {
		t.Fatalf("Load of an absent document = %v, %v", got, err)
	}
	d.Extra = lottie.ExtraFields{"generator": json.RawMessage(`"test"`)}
	if err := Store(b, "clip", d); err != nil {
		t.Fatal(err)
	}
	raw, ok := b.ExtensionFile(File("clip"))
	if !ok {
		t.Fatal("Store wrote nothing")
	}
	got, err := Load(b, "clip")
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Extra["generator"]) != `"test"` {
		t.Fatalf("doc extra lost: %v", got.Extra)
	}
	// A document this package wrote re-encodes byte for byte.
	again, _ := json.Marshal(got)
	if !bytes.Equal(raw, again) {
		t.Fatalf("stored document is not canonical:\n%s\n%s", raw, again)
	}
	Remove(b, "clip")
	if _, ok := b.ExtensionFile(File("clip")); ok {
		t.Fatal("Remove left the document")
	}
}

func TestApplyBindsWhatResolves(t *testing.T) {
	b := lottie.NewBundle()
	if err := b.SetAnimation("clip", []byte(testClip)); err != nil {
		t.Fatal(err)
	}
	anim, err := b.Animation("clip")
	if err != nil {
		t.Fatal(err)
	}
	p := anim.NewPlayer()
	d := testDoc()
	d.Paints = append(d.Paints, Paint{Layer: 4, Item: []int{0}, Texture: "stale"})
	err = d.Apply(p)
	if err == nil || !strings.Contains(err.Error(), "layer 4") {
		t.Fatalf("stale entry not reported: %v", err)
	}
	if got := strings.Join(p.TextureNames(), ","); got != "brush,skin" {
		t.Fatalf("bound textures = %q; the stale entry must not undress the rest", got)
	}
}
