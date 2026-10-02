package edit_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// API.md E24, E25: the ops of §8.8's example, each member read as its rule says.
func TestOpJSONReads(t *testing.T) {
	cases := []struct {
		in   string
		want edit.Operation
	}{
		{
			`{"op": "set", "path": "farm.modelTypes[3].maxLevel", "value": 10}`,
			edit.Operation{Kind: edit.OpSet, Path: "farm.modelTypes[3].maxLevel", Value: edit.FromJSON("10")},
		},
		{
			`{"op": "add", "path": "levels", "value": {"level": 10, "modelName": "obj"}}`,
			edit.Operation{Kind: edit.OpAdd, Path: "levels", Value: edit.FromJSON(`{"level": 10, "modelName": "obj"}`)},
		},
		{
			`{"op": "addEntry", "path": "statuses", "key": "blocked", "source": "{ tone: danger }"}`,
			edit.Operation{Kind: edit.OpAddEntry, Path: "statuses", Key: edit.PathKey("blocked"), Value: edit.Source("{ tone: danger }")},
		},
		{
			`{"op": "setCase", "path": "e.kind", "case": "spawn_item", "source": "{ left: 0 }"}`,
			edit.Operation{Kind: edit.OpSetCase, Path: "e.kind", Case: "spawn_item", Value: edit.Source("{ left: 0 }")},
		},
		{`{"op": "setCase", "path": "e.kind", "case": "nothing"}`, edit.Operation{Kind: edit.OpSetCase, Path: "e.kind", Case: "nothing"}},
		{`{"op": "move", "path": "levels[2]", "index": 0}`, edit.Operation{Kind: edit.OpMove, Path: "levels[2]"}},
		{`{"op": "insert", "path": "xs", "index": 2, "source": "7"}`, edit.Operation{Kind: edit.OpInsert, Path: "xs", Index: 2, Value: edit.Source("7")}},
		{`{"op": "rename", "path": "statuses.blocked", "key": "on_hold"}`, edit.Operation{Kind: edit.OpRename, Path: "statuses.blocked", Key: edit.PathKey("on_hold")}},
		{`{"op": "rename", "path": "rows[7]", "key": -3}`, edit.Operation{Kind: edit.OpRename, Path: "rows[7]", Key: edit.IntKey(-3)}},
		{`{"op": "reset", "path": "config.server.port"}`, edit.Operation{Kind: edit.OpReset, Path: "config.server.port"}},
		{`{"op": "set", "path": "config.paths.iconDir", "value": null}`, edit.Operation{Kind: edit.OpSet, Path: "config.paths.iconDir", Value: edit.None{}}},
		{`{"path": "a", "op": "remove"}`, edit.Operation{Kind: edit.OpRemove, Path: "a"}},
		{`{"op": "retire", "path": "a"}`, edit.Operation{Kind: edit.OpRetire, Path: "a"}},
		{`{"op": "unretire", "path": "a"}`, edit.Operation{Kind: edit.OpUnretire, Path: "a"}},
	}
	for _, c := range cases {
		var op edit.Operation
		if err := json.Unmarshal([]byte(c.in), &op); err != nil {
			t.Errorf("%s: %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(op, c.want) {
			t.Errorf("%s = %#v, want %#v", c.in, op, c.want)
		}
	}
}

// API.md E24: `value` and `source` never both; a member no op has, a member given twice, a
// member the op does not take or needs and lacks, an unknown op, a key or index of the wrong
// kind are refused.
func TestOpJSONRefuses(t *testing.T) {
	for _, in := range []string{
		`{"op": "set", "path": "a", "value": 1, "source": "1"}`,
		`{"op": "set", "path": "a", "source": "1", "value": 1}`,
		`{"op": "set", "path": "a", "value": 1, "value": 2}`,
		`{"op": "set", "path": "a", "value": 1, "extra": 0}`,
		`{"op": "set", "path": "a"}`,
		`{"op": "set", "value": 1}`,
		`{"path": "a", "value": 1}`,
		`{"op": "set", "path": "a", "value": 1, "index": 0}`,
		`{"op": "remove", "path": "a", "key": "k"}`,
		`{"op": "delete", "path": "a"}`,
		`{"op": "rename", "path": "a", "key": 1.5}`,
		`{"op": "rename", "path": "a", "key": true}`,
		`{"op": "move", "path": "a", "index": "0"}`,
		`{"op": "move", "path": "a", "index": 1e2}`,
		`{"op": "add", "path": "a", "source": 1}`,
		`{"op": 1, "path": "a"}`,
		`{"op": "setCase", "path": "a", "source": "{}"}`,
		`{"op": "setCase", "path": "a", "case": ""}`,
		`["set"]`,
	} {
		var op edit.Operation
		if err := json.Unmarshal([]byte(in), &op); !errors.Is(err, edit.ErrOpJSON) {
			t.Errorf("%s: %v, want ErrOpJSON", in, err)
		}
	}
}

// API.md E26: a FromJSON is written as `value`, compacted; a Source as `source`, as given; a
// key as a JSON string or integer (E25); members in the order of the spec's example.
func TestOpJSONWrites(t *testing.T) {
	cases := []struct {
		op   edit.Operation
		want string
	}{
		{edit.Operation{Kind: edit.OpSet, Path: "a.b", Value: edit.FromJSON(`{ "x" : [1, 2] }`)}, `{"op":"set","path":"a.b","value":{"x":[1,2]}}`},
		{
			edit.Operation{Kind: edit.OpAddEntry, Path: "s", Key: edit.Key("k"), Value: edit.Source("{ tone: danger }")},
			`{"op":"addEntry","path":"s","key":"k","source":"{ tone: danger }"}`,
		},
		{edit.Operation{Kind: edit.OpAddEntry, Path: "m", Key: edit.Member("red"), Value: edit.None{}}, `{"op":"addEntry","path":"m","key":"red","source":"none"}`},
		{edit.Operation{Kind: edit.OpRename, Path: "r[7]", Key: edit.IntKey(8)}, `{"op":"rename","path":"r[7]","key":8}`},
		{edit.Operation{Kind: edit.OpMove, Path: "xs[1]"}, `{"op":"move","path":"xs[1]","index":0}`},
		{edit.Operation{Kind: edit.OpSetCase, Path: "e", Case: "dot"}, `{"op":"setCase","path":"e","case":"dot"}`},
	}
	for _, c := range cases {
		b, err := json.Marshal(c.op)
		if err != nil || string(b) != c.want {
			t.Errorf("%#v = %s, %v, want %s", c.op, b, err, c.want)
		}
	}
}

// API.md E26: a value the codec has no canonical text for, and an op carrying a member it does
// not take, are refused rather than written wrong.
func TestOpJSONWriteRefusals(t *testing.T) {
	for _, op := range []edit.Operation{
		{Kind: edit.OpSet, Path: "a"},
		{Kind: edit.OpRemove, Path: "a", Index: 2},
		{Kind: edit.OpRename, Path: "a", Key: edit.List{}},
		{Kind: edit.OpRenameName + 1, Path: "a"},
	} {
		if b, err := json.Marshal(op); err == nil {
			t.Errorf("%#v = %s, want an error", op, b)
		}
	}
	_, err := json.Marshal(edit.Operation{Kind: edit.OpSet, Path: "a", Value: edit.List{edit.FromJSON("1")}})
	if !errors.Is(err, edit.ErrNoText) {
		t.Errorf("a FromJSON inside a List: %v, want ErrNoText", err)
	}
}

// API.md E24, E25, E26: decode(encode(op)) == op for the forms the JSON form carries.
func TestOpJSONRoundTrip(t *testing.T) {
	for _, op := range roundTripOps {
		b, err := json.Marshal(op)
		if err != nil {
			t.Fatalf("%#v: %v", op, err)
		}
		var back edit.Operation
		if err := json.Unmarshal(b, &back); err != nil || !reflect.DeepEqual(back, op) {
			t.Errorf("%s = %#v, %v, want %#v", b, back, err, op)
		}
	}
}

var roundTripOps = []edit.Operation{
	{Kind: edit.OpSet, Path: "a.b", Value: edit.FromJSON(`{"x":[1,2]}`)},
	{Kind: edit.OpAdd, Path: "xs", Value: edit.Source("{ n: 1 }")},
	{Kind: edit.OpInsert, Path: "xs", Index: 3, Value: edit.FromJSON(`[1,"a"]`)},
	{Kind: edit.OpAddEntry, Path: "m", Key: edit.PathKey("k \"q\""), Value: edit.Source(`"é\n"`)},
	{Kind: edit.OpRename, Path: "r[7]", Key: edit.IntKey(-8)},
	{Kind: edit.OpSetCase, Path: "e", Case: "circle", Value: edit.FromJSON(`{"r":2}`)},
	{Kind: edit.OpReset, Path: `m["a b"]`},
	{Kind: edit.OpRetire, Path: "t.x"},
}

// API.md E24, E25, E26, fuzzed (IMPLEMENTATION-PLAN.md §7.7): decode(encode(op)) == op, stable bytes.
func FuzzOpJSON(f *testing.F) {
	for _, op := range roundTripOps {
		b, err := json.Marshal(op)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
	}
	f.Add([]byte(`{"op": "set", "path": "a", "value": null}`))
	f.Add([]byte(`{"op":"setCase","path":"a","case":""}`))
	f.Add([]byte(`{"op": "move", "path": "a", "index": 00}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		var op edit.Operation
		if json.Unmarshal(data, &op) != nil {
			return
		}
		b, err := json.Marshal(op)
		if err != nil {
			t.Fatalf("%s read as %#v, which does not write: %v", data, op, err)
		}
		var back edit.Operation
		if err := json.Unmarshal(b, &back); err != nil {
			t.Fatalf("%s wrote %s, which does not read: %v", data, b, err)
		}
		again, err := json.Marshal(back)
		if err != nil || string(again) != string(b) {
			t.Fatalf("%s wrote %s, then %s, %v", data, b, again, err)
		}
		var third edit.Operation
		if err := json.Unmarshal(again, &third); err != nil || !reflect.DeepEqual(third, back) {
			t.Fatalf("%s: %#v read back as %#v, %v", data, back, third, err)
		}
	})
}
