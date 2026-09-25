package wire_test

import (
	"fmt"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// selfRef is `table Node`, `record Node { k: K  like: ref nodes  p: P(like)  items: [Item] }`,
// with `record Item { like: ref nodes  p: P(like) }`, `record Sub(s: Node) { lim: Int }` and `type
// P(e: Node) = match e.k { num => Int  col => Sub(e) }`; also its collection and Sub's parameter.
func selfRef() (*types.TableType, *types.Collection, *types.Param) {
	k := enum("p", "K", "num", "col")
	kField := field("k", k)
	nodes := &types.Collection{Kind: types.CollLet, Pkg: "p", Name: "nodes"}
	e, s := &types.Param{Name: "e"}, &types.Param{Name: "s"}
	sub := record("p", "Sub", field("lim", types.IntType))
	sub.Params = []*types.Param{s}
	fn := &types.TypeFunc{Pkg: "p", Name: "P", Params: []*types.Param{e},
		Scrutinee: &types.Scrutinee{Param: e, Path: []*types.Field{kField}, Type: k},
		Arms: []*types.TypeArm{
			{Members: []int{0}, Result: types.IntType},
			{Members: []int{1}, Result: &types.AppliedRecord{Rec: sub, Args: []*types.Arg{{Source: types.ArgParam, Param: e}}}},
		},
	}
	dep := func(like *types.Field) *types.Field {
		p := field("p", &types.TypeAppType{Fn: fn, Args: []*types.Arg{{Source: types.ArgField, Path: []*types.Field{like}}}})
		p.DependsOn = []int{like.Index}
		return p
	}
	itemLike := field("like", &types.RefType{Target: nodes})
	item := record("p", "Item", itemLike, dep(itemLike))
	like := field("like", &types.RefType{Target: nodes})
	like.Index = 1
	node := record("p", "Node", kField, like, dep(like), field("items", listOf(item)))
	nodes.Elem = node
	return &types.TableType{Elem: node}, nodes, s
}

// WIRE.md §5.9: entries decode dependent fields last, refs between them never reach Host.Deref; Bind gets arguments.
func TestDecodeEntriesLater(t *testing.T) {
	table, nodes, s := selfRef()
	h := newHost()
	src := `{"a": {"k": "num", "like": "b", "p": {"lim": 1}, "items": [{"like": "a", "p": 2}]},
	         "b": {"k": "col", "like": "a", "p": 3, "items": []}}`
	got := decodeJSON(t, wire.Decoder{Host: h, Coll: nodes}, src, table)
	want := "{a: Node{k: num, like: b, p: Sub{lim: 1}, items: [Item{like: a, p: 2}]}, b: Node{k: col, like: a, p: 3, items: []}}"
	if got.text() != want || h.misses != 0 {
		t.Fatalf("got %s after %d host misses, want %s and none", got.text(), h.misses, want)
	}
	a := got.v.(*value.Table).Entries[0]
	params := h.bound[a.Fields[2].(*value.Record)]
	if ref, ok := params[s].(*value.Ref); !ok || ref.Key.S != "b" {
		t.Errorf("Sub's bound arguments = %v, want s = the ref b", params)
	}
}

// loops is `table Loop`, `record Loop { like: ref loops  l: P(like)  k: K = num }` and `type
// P(e: Loop) = match e.k { _ => Int }`, with the host evaluating k's default; also k's field.
func loops(h *host) (*types.TableType, *types.Collection, *types.Field) {
	coll := &types.Collection{Kind: types.CollLet, Pkg: "p", Name: "loops"}
	k := enum("p", "K", "num")
	late := field("k", k)
	e := &types.Param{Name: "e"}
	fn := &types.TypeFunc{Pkg: "p", Name: "P", Params: []*types.Param{e},
		Scrutinee: &types.Scrutinee{Param: e, Path: []*types.Field{late}, Type: k},
		Arms:      []*types.TypeArm{{Wildcard: true, Result: types.IntType}},
	}
	like := field("like", &types.RefType{Target: coll})
	l := field("l", &types.TypeAppType{Fn: fn, Args: []*types.Arg{{Source: types.ArgField, Path: []*types.Field{like}}}})
	l.DependsOn = []int{0}
	coll.Elem = record("p", "Loop", like, l, late)
	h.withDefault(late, member(k, 0))
	return &types.TableType{Elem: coll.Elem}, coll, late
}

// EVALUATION.md §3.2: k = num does not wait, so the entries decode; a k reading l is a cycle at the ref.
func TestDecodeEntriesCycle(t *testing.T) {
	const src = `{"a": {"like": "b", "l": 1}, "b": {"like": "a", "l": 2}}`
	h := newHost()
	table, coll, _ := loops(h)
	if got := decodeJSON(t, wire.Decoder{Host: h, Coll: coll}, src, table); !got.ok || len(h.cycles) != 0 {
		t.Errorf("k = num: %s, cycles %v, want a decoded table", got.text(), h.cycles)
	}
	h = newHost()
	table, coll, k := loops(h)
	h.reads[k] = []int{1}
	got := decodeJSON(t, wire.Decoder{Host: h, Coll: coll}, src, table)
	if got.ok || len(got.findings) != 0 || len(h.cycles) != 1 || h.cycles[0].Key.S != "a" {
		t.Errorf("k reading l: ok = %v, findings %v, cycles %v, want one cycle at the ref a", got.ok, got.findings, h.cycles)
	}
}

// DECISIONS 195: a chain of entries each waiting for the next finishes on an explicit stack.
func TestDecodeEntriesDeepChain(t *testing.T) {
	const n, maxStack = 5000, 1 << 18
	h := newHost()
	table, coll, k := loops(h)
	h.reads[k] = []int{1}
	var sb strings.Builder
	sb.WriteString("{")
	for i := range n - 1 {
		fmt.Fprintf(&sb, `"e%d": {"like": "e%d", "l": %d}, `, i, i+1, i)
	}
	fmt.Fprintf(&sb, `"e%d": {"like": "e%d", "l": 0, "k": "num"}}`, n-1, n-1)
	defer debug.SetMaxStack(debug.SetMaxStack(maxStack))
	got := decodeJSON(t, wire.Decoder{Host: h, Coll: coll}, sb.String(), table)
	if !got.ok || len(got.v.(*value.Table).Entries) != n || len(h.cycles) != 0 {
		t.Errorf("ok = %v, findings %v, cycles %v, want %d entries", got.ok, got.findings, h.cycles, n)
	}
}

// WIRE.md §5.5.1, §5.6: an inline case's dependent field waits, its keys claimed: no E3301.
func TestDecodeInlineCaseWaits(t *testing.T) {
	table, coll, _ := selfRef()
	node := table.Elem.(*types.RecordType)
	fn := node.Fields[2].Type.(*types.TypeAppType).Fn
	on := field("on", &types.RefType{Target: coll})
	v := field("v", &types.TypeAppType{Fn: fn, Args: []*types.Arg{{Source: types.ArgField, Path: []*types.Field{on}}}})
	v.DependsOn = []int{0}
	shape := field("s", variantOf("p", "Shape", "shape", &types.CaseType{Name: "dot", Fields: []*types.Field{on, v}}))
	shape.Inline = true
	holder := record("p", "Holder", field("k", node.Fields[0].Type), shape)
	holders := &types.Collection{Kind: types.CollLet, Pkg: "p", Name: "holders", Elem: holder}
	h := newHost()
	h.entries[coll] = []*value.Record{{T: node, Fields: []value.Value{member(node.Fields[0].Type.(*types.EnumType), 0)}, Ident: &value.Identity{Coll: coll, Key: value.Key{S: "n"}}}}
	got := decodeJSON(t, wire.Decoder{Host: h, Coll: holders}, `{"a": {"k": "num", "shape": "dot", "on": "n", "v": 4}}`, &types.TableType{Elem: holder})
	if want := "{a: Holder{k: num, s: dot{on: n, v: 4}}}"; got.text() != want {
		t.Errorf("got %s, want %s", got.text(), want)
	}
}

// DECISIONS 195: an attempt collects every record it needs before it is undone, so a map naming n
// waiting entries is retried once, not n times: n+2 attempts, one undone.
func TestDecodeEntriesWideMap(t *testing.T) {
	const n = 4000
	h := newHost()
	table, coll, k := loops(h)
	h.reads[k] = []int{1}
	loop := table.Elem.(*types.RecordType)
	fn := loop.Fields[1].Type.(*types.TypeAppType).Fn
	dep := &types.DepMapType{Coll: coll, Binder: "x", Value: &types.TypeAppType{Fn: fn, Args: []*types.Arg{{Source: types.ArgKey, Binder: "x"}}}}
	m := field("m", opt(dep))
	m.Index = len(loop.Fields)
	loop.Fields = append(loop.Fields, m)
	var keys, entries strings.Builder
	for i := range n {
		fmt.Fprintf(&keys, `, "e%d": %d`, i, i)
		fmt.Fprintf(&entries, `, "e%d": {"like": "hub", "l": %d}`, i, i)
	}
	src := `{"hub": {"like": "hub", "l": 0, "k": "num", "m": {` + keys.String()[2:] + `}}` + entries.String() + `}`
	got := decodeJSON(t, wire.Decoder{Host: h, Coll: coll}, src, table)
	if !got.ok || h.attempts != n+2 || h.undone != 1 {
		t.Errorf("ok = %v %v, %d attempts, %d undone, want %d attempts, one undone", got.ok, got.findings, h.attempts, h.undone, n+2)
	}
}
