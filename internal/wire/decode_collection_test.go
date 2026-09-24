package wire_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

var collectionCases = []struct {
	rule string
	typ  types.Type
	json string
	want string
}{
	{"§5.7 list", listOf(types.IntType), `[3, 1, 2]`, "[3, 1, 2]"},
	{"§5.7 list kind", listOf(types.IntType), `{"a": 1}`, at(diag.E7110, "1:1", "")},
	{"§5.7 every element", listOf(types.IntType), `[1, "a", true]`, findings(at(diag.E7110, "1:5", "/1"), at(diag.E7110, "1:10", "/2"))},
	{"§5.7 keyed list", keyed(intNode, 0), `[{"id": 2}, {"id": 1}]`, "[Node{id: 2}, Node{id: 1}]"},
	{"§5.7 keyed list kind", keyed(intNode, 0), `{"2": {"id": 2}}`, at(diag.E7110, "1:1", "")},
	{"§5.7 table", &types.TableType{Elem: status}, `{"open": {"label": "O"}, "taken": {"$retired": true, "label": "T"}}`,
		`{open: Status{label: "O"}, taken: Status{label: "T"}}`},
	{"§5.7 table key", &types.TableType{Elem: status}, `{"not-id": {"label": "O"}, "_": {"label": "U"}}`, findings(at(diag.E7114, "1:2", "/not-id"), at(diag.E7114, "1:28", "/_"))},
	{"§5.12 $schema at the root", &types.TableType{Elem: status}, `{"$schema": "flow.Status@b330a789", "a": {"label": "A"}}`, `{a: Status{label: "A"}}`},
	{"§5.7 reserved word key", &types.TableType{Elem: status}, `{"record": {"label": "R"}}`, `{record: Status{label: "R"}}`},
	{"§5.7 $retired false", &types.TableType{Elem: status}, `{"a": {"$retired": false, "label": "A"}}`, at(diag.E7110, "1:20", "/a/$retired")},
	{"§5.7 $retired kind", &types.TableType{Elem: status}, `{"a": {"$retired": {}, "label": "A"}}`, at(diag.E7110, "1:20", "/a/$retired")},
	{"§5.7 table kind", &types.TableType{Elem: status}, `[]`, at(diag.E7110, "1:1", "")},
	{"§5.7 $retired outside rows", status, `{"$retired": true, "label": "A"}`, at(diag.E3301, "1:2", "/$retired")},
	{"§5.8 string keys", &types.MapType{Key: types.StringType, Value: types.IntType}, `{"b": 1, "a": 2}`, `{"b": 1, "a": 2}`},
	{"§5.8 int keys", &types.MapType{Key: types.IntType, Value: types.IntType}, `{"-5": 1, "0": 2, "42": 3}`, "{-5: 1, 0: 2, 42: 3}"},
	{"§5.8 int key not canonical", &types.MapType{Key: types.IntType, Value: types.IntType}, `{"05": 1, "+1": 2, "1.0": 3}`,
		findings(at(diag.E7103, "1:2", "/05"), at(diag.E7103, "1:11", "/+1"), at(diag.E7103, "1:20", "/1.0"))},
	{"§5.8 int key range", &types.MapType{Key: types.UInt8Type, Value: types.IntType}, `{"256": 1}`, at(diag.E3201, "1:2", "/256")},
	{"§5.8 one key text twice", &types.MapType{Key: types.IntType, Value: types.IntType}, `{"0": 1, "-0": 2}`, at(diag.E3317, "1:10", "/-0")},
	{"§5.8 enum keys", &types.MapType{Key: tone, Value: types.IntType}, `{"series-1": 1}`, "{series_1: 1}"},
	{"§5.8 enum key unknown", &types.MapType{Key: tone, Value: types.IntType}, `{"series_1": 1}`, at(diag.E7111, "1:2", "/series_1")},
	{"§5.8 code keys", &types.MapType{Key: element, Value: types.FloatType}, `{"1": 0.5, "2": 1.25}`, "{FIRE: 0.5, WATER: 1.25}"},
	{"§5.8 code key", &types.MapType{Key: element, Value: types.FloatType}, `{"FIRE": 0.5}`, at(diag.E7103, "1:2", "/FIRE")},
	{"§5.8 ref keys", &types.MapType{Key: &types.RefType{Target: intKeyed}, Value: types.IntType}, `{"7": 1}`, "{7: 1}"},
	{"§5.8 enum-keyed ref keys", &types.MapType{Key: &types.RefType{Target: toneKeyed}, Value: types.IntType}, `{"series-1": 1}`, "{series_1: 1}"},
	{"§5.8 literal key wins", &types.MapType{Key: &types.LitUnionType{Of: tone, Literals: []string{"info"}}, Value: types.IntType},
		`{"info": 1, "series-1": 2}`, `{"info": 1, series_1: 2}`},
	{"§5.8 map kind", &types.MapType{Key: types.StringType, Value: types.IntType}, `[1]`, at(diag.E7110, "1:1", "")},
	{"§5.8 key and value", &types.MapType{Key: types.IntType, Value: types.IntType}, `{"x": "y"}`, findings(at(diag.E7103, "1:2", "/x"), at(diag.E7110, "1:7", "/x"))},
}

// WIRE.md §5.7, §5.8: lists, keyed lists, source-wire tables and maps, their keys and codes.
func TestDecodeCollections(t *testing.T) {
	for _, c := range collectionCases {
		if got := decodeJSON(t, wire.Decoder{}, c.json, c.typ).text(); got != c.want {
			t.Errorf("%s: %s = %q, want %q", c.rule, c.json, got, c.want)
		}
	}
}

// TYPES.md §6.3, §10.2 (RES-03): entries carry identities; a level-1 ref binds its instance.
func TestDecodeIdentities(t *testing.T) {
	tree, coll := talentFixture()
	got := decodeJSON(t, wire.Decoder{}, `{"nodes": [{"id": 0, "parent": -1}, {"id": 1, "parent": 0}, {"id": 2, "parent": -1.0}]}`, tree)
	if !got.ok {
		t.Fatalf("tree: %s", got.text())
	}
	root := got.v.(*value.Record)
	nodes := root.Fields[0].(*value.List).Elems
	first, second := nodes[0].(*value.Record), nodes[1].(*value.Record)
	parent := second.Fields[1].(*value.Ref)
	if first.Ident == nil || first.Ident.Coll != coll || first.Ident.Owner != root || first.Ident.Key != (value.Key{IsInt: true}) {
		t.Errorf("node identity %+v", first.Ident)
	}
	if parent.Owner != root || !value.Equal(parent, first) {
		t.Errorf("parent %+v is not node 0", parent)
	}
	if _, isNone := nodes[2].(*value.Record).Fields[1].(*value.None); !isNone {
		t.Errorf("-1.0 is the marker -1: %s", nodes[2].CanonText())
	}
	whole := &types.Collection{Kind: types.CollLet, Pkg: "p", Name: "nodes"}
	got = decodeJSON(t, wire.Decoder{Coll: whole}, `[{"id": 3}]`, keyed(intNode, 0))
	if id := got.v.(*value.List).Elems[0].(*value.Record).Ident; id.Coll != whole || id.Key.I != 3 {
		t.Errorf("a whole keyed list is the decoder's collection: %+v", id)
	}
	got = decodeJSON(t, wire.Decoder{Coll: whole}, `{"a": {"label": "A"}, "b": {"$retired": true, "label": "B"}}`, &types.TableType{Elem: status})
	entries := got.v.(*value.Table).Entries
	if entries[0].Ident.Coll != whole || entries[0].Ident.Retired || !entries[1].Ident.Retired || entries[1].Ident.Key.S != "b" {
		t.Errorf("table identities %+v %+v", entries[0].Ident, entries[1].Ident)
	}
}

// WIRE.md §6.3: a `*` over an object is a map of its keys; into a record, the caller's error.
func TestDecodeSelection(t *testing.T) {
	fs := &source.FileSet{}
	f, _ := fs.Add("etc.json", "/p/etc.json", []byte(`{"JOB_VAGRANT": {"us": "Vagrant", "fr": "Vagabond"}, "JOB_KNIGHT": {"us": "Knight"}}`))
	bag := diag.NewBag(fs, "p")
	root, err := jsonsrc.Parse(f, bag)
	if err != nil {
		t.Fatal(err)
	}
	sel := wire.Selection{Node: root, Star: true}
	for _, m := range root.Members {
		sel.Items = append(sel.Items, wire.Selection{Node: m.Value.Members[0].Value})
	}
	dec := wire.Decoder{Bag: bag, Pkg: "p"}
	names := &types.MapType{Key: types.StringType, Value: types.StringType}
	v, ok, err := dec.Decode(context.Background(), sel, names)
	if err != nil || !ok || v.CanonText() != `{"JOB_VAGRANT": "Vagrant", "JOB_KNIGHT": "Knight"}` {
		t.Errorf("at: \"*.us\" = %v, %v, %v", v, ok, err)
	}
	if _, _, err := dec.Decode(context.Background(), sel, status); !errors.Is(err, wire.ErrStar) {
		t.Errorf("a `*` into a record: %v, want ErrStar", err)
	}
	if _, _, err := dec.Decode(context.Background(), wire.Selection{Node: root}, types.RangeType); !errors.Is(err, wire.ErrNoWireType) {
		t.Errorf("Range: %v, want ErrNoWireType", err)
	}
}

// WIRE.md §6.5: load.dir's elements, or entries keyed by stem (E7114, E3102) and retired.
func TestDecodeDir(t *testing.T) {
	fs := &source.FileSet{}
	bag := diag.NewBag(fs, "p")
	files := []wire.File{}
	for _, name := range []string{"open", "open", "bad-stem", "gone"} {
		text := `{"label": "` + name + `"}`
		if name == "gone" {
			text = `{"$retired": true, "label": "G"}`
		}
		f, _ := fs.Add(name+".json", "/p/"+name+".json", []byte(text))
		root, err := jsonsrc.Parse(f, bag)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, wire.File{Sel: wire.Selection{Node: root}, Stem: name, At: source.Span{File: f.ID}})
	}
	dec := wire.Decoder{Bag: bag, Pkg: "p"}
	if _, ok, err := dec.Dir(context.Background(), files, &types.TableType{Elem: status}); ok || err != nil {
		t.Fatalf("bad stems decode: %v %v", ok, err)
	}
	if got := short(fs, bag.Findings()); len(got) != 2 || got[0] != at(diag.E7114, "1:1", "") || got[1] != at(diag.E3102, "1:1", "") {
		t.Errorf("stem findings %q", got)
	}
	v, ok, err := dec.Dir(context.Background(), []wire.File{files[0], files[3]}, &types.TableType{Elem: status})
	if !ok || err != nil || v.CanonText() != `{open: Status{label: "open"}, gone: Status{label: "G"}}` || !v.(*value.Table).Entries[1].Ident.Retired {
		t.Errorf("table from files: %v %v %v", v, ok, err)
	}
	v, ok, _ = dec.Dir(context.Background(), []wire.File{files[0], files[2]}, listOf(status))
	if !ok || v.CanonText() != `[Status{label: "open"}, Status{label: "bad-stem"}]` {
		t.Errorf("list from files: %v", v)
	}
}
