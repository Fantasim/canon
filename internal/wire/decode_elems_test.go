package wire_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// keptElems is a wire.Elems serving the records a test keeps, by selection, and noting each
// element offered.
type keptElems struct {
	kept    map[*jsonsrc.Node]keptElem
	offered []keptElem
}

// keptElem is an element kept or offered: its record, whether retired, whether decoded purely.
type keptElem struct {
	rec           *value.Record
	retired, pure bool
}

func (k *keptElems) Kept(sel wire.Selection) (*value.Record, bool, bool) {
	e, ok := k.kept[sel.Node]
	return e.rec, e.retired, ok
}

func (k *keptElems) Start(wire.Selection) func(*value.Record, bool, bool) {
	return func(rec *value.Record, retired, pure bool) {
		k.offered = append(k.offered, keptElem{rec: rec, retired: retired, pure: pure})
	}
}

// dirOf parses each stem's text as a load.dir file.
func dirOf(t *testing.T, fs *source.FileSet, bag *diag.Bag, stems, texts []string) []wire.File {
	t.Helper()
	out := make([]wire.File, len(stems))
	for i, stem := range stems {
		f, err := fs.Add(stem+".json", "/p/"+stem+".json", []byte(texts[i]))
		if err != nil {
			t.Fatal(err)
		}
		root, err := jsonsrc.Parse(f, bag)
		if err != nil {
			t.Fatal(err)
		}
		out[i] = wire.File{Sel: wire.Selection{Node: root}, Stem: stem, At: source.Span{File: f.ID}}
	}
	return out
}

// WIRE.md §6.5, §5.5.1, IMPLEMENTATION-PLAN §7.6: a finding kept, a default failed or a field waiting is not pure.
func TestDirElemsImpure(t *testing.T) {
	failing := field("n", types.IntType)
	failing.Default = &syntax.IntLit{} // the host serves no value for it
	waiting := field("m", types.IntType)
	waiting.DependsOn = []int{0} // decoded in the second pass (TYPES.md §11)
	for _, c := range []struct {
		name string
		keep bool
		typ  *types.RecordType
		json string
	}{
		{"a finding kept (API.md V2)", true, status, `{}`},
		{"a default failed", false, record("flow", "Failing", failing), `{}`},
		{"a field waiting", false, record("flow", "Waiting", field("n", types.IntType), waiting), `{"n": 1, "m": 2}`},
	} {
		fs := &source.FileSet{}
		bag := diag.NewBag(fs, "p")
		k := &keptElems{kept: map[*jsonsrc.Node]keptElem{}}
		dec := wire.Decoder{Bag: bag, Pkg: "p", Host: newHost(), Elems: k, Keep: c.keep}
		_, _, err := dec.Dir(context.Background(), dirOf(t, fs, bag, []string{"x"}, []string{c.json}), listOf(c.typ))
		if err != nil || len(k.offered) != 1 || k.offered[0].pure {
			t.Errorf("%s: %v, offered %+v: want one element, impure", c.name, err, k.offered)
		}
	}
}

// WIRE.md §6.5, IMPLEMENTATION-PLAN §7.6 NFR-02: kept elements served, the others decoded and offered.
func TestDirElems(t *testing.T) {
	ctx := context.Background()
	fs := &source.FileSet{}
	bag := diag.NewBag(fs, "p")
	files := dirOf(t, fs, bag, []string{"a", "b", "c"}, []string{`{"label": "a"}`, `{"label": 1}`, `{"$retired": true, "label": "c"}`})
	k := &keptElems{kept: map[*jsonsrc.Node]keptElem{}}
	dec := wire.Decoder{Bag: bag, Pkg: "p", Host: newHost(), Elems: k}
	if _, ok, err := dec.Dir(ctx, files[:2], listOf(status)); ok || err != nil {
		t.Fatalf("a list with a bad element: %v %v", ok, err)
	}
	if len(k.offered) != 2 || !k.offered[0].pure || k.offered[0].rec == nil || k.offered[1].pure || k.offered[1].rec != nil {
		t.Fatalf("offered %+v: want a pure, then b impure and failed", k.offered)
	}
	a := &value.Record{T: status, Fields: []value.Value{&value.Str{V: "kept", T: types.StringType}}, Set: []bool{true}}
	k.kept[files[0].Sel.Node], k.offered = keptElem{rec: a}, nil
	v, ok, err := dec.Dir(ctx, files[:1], listOf(status))
	if !ok || err != nil || v.(*value.List).Elems[0] != a || len(k.offered) != 0 {
		t.Errorf("a kept element: %v %v %v, %d offered: want the kept record, none offered", v, ok, err, len(k.offered))
	}
	c := &value.Record{T: status, Fields: []value.Value{&value.Str{V: "c", T: types.StringType}}, Set: []bool{true}}
	k.kept, k.offered = map[*jsonsrc.Node]keptElem{files[2].Sel.Node: {rec: c, retired: true}}, nil
	v, ok, err = dec.Dir(ctx, []wire.File{files[0], files[2]}, &types.TableType{Elem: status})
	entries := v.(*value.Table).Entries
	if !ok || err != nil || entries[1] != c || !c.Ident.Retired || c.Ident.Key.S != "c" || len(k.offered) != 1 || !k.offered[0].pure {
		t.Errorf("a kept row: %v %v %v, offered %+v: want the kept row retired, keyed by its stem", v, ok, err, k.offered)
	}
	plain := wire.Decoder{Bag: bag, Pkg: "p", Elems: k}
	k.offered = nil
	if _, _, err := plain.Dir(ctx, files[:1], listOf(status)); err != nil || len(k.offered) != 0 {
		t.Errorf("no host: %v, %d offered: want Elems unasked", err, len(k.offered))
	}
}
