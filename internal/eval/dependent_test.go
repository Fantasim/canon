package eval_test

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"golang.org/x/tools/txtar"
)

// TYPES.md §11 (DEP-02, DECISIONS 147): each case prints its values and findings.
func TestDependent(t *testing.T) {
	golden.Run(t, "testdata/dependent/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		b := runBuild(t, fromArchive(t, c.Archive), eval.Options{})
		return []byte(b.dump() + "\n" + b.findings(t))
	}, golden.Expected(dumpFile))
}

// TYPES.md §11.6: stage A keeps kept.txtar's values as written; the root stage B hands back holds them converted.
func TestDependentKept(t *testing.T) {
	a, err := txtar.ParseFile("testdata/dependent/kept.txtar")
	if err != nil {
		t.Fatal(err)
	}
	b := runBuild(t, fromArchive(t, a), eval.Options{})
	rs, ok := b.values[eval.Root{Pkg: "a", Name: "rs"}].(*value.List)
	if !ok {
		t.Fatal("rs is not evaluated")
	}
	field := func(i, f int) value.Value { return rs.Elems[i].(*value.Record).Fields[f] }
	cases := []struct {
		name string
		v    value.Value
		want string
	}{
		{"an integer", field(0, 1), "*value.Int 3"},
		{"a symbol on a ref branch", field(1, 1), "*value.Ref one"},
		{"a string under a literal union", field(1, 2), "*value.Ref two"},
		{"a string in a dependent list", field(1, 4).(*value.List).Elems[0], "*value.Ref one"},
		{"a symbol in a dependent list", field(1, 4).(*value.List).Elems[1], "*value.Ref two"},
		{"a string on a ref branch", field(2, 1), "*value.Ref two"},
		{"a symbol on an enum branch", field(3, 1), "*value.Member red"},
		{"a symbol key", field(3, 3).(*value.Map).Keys[0], "*value.Member red"},
		{"a string key", field(3, 3).(*value.Map).Keys[1], "*value.Str red"},
		{"a case name on a variant branch", field(4, 1), "*value.Symbol box"},
	}
	for _, c := range cases {
		if got := fmt.Sprintf("%T %s", c.v, c.v.CanonText()); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
}

// DECISIONS 173, EVALUATION.md §13: Default and Deref serve a decoder outside a load too.
func TestDecodeHost(t *testing.T) {
	a, err := txtar.ParseFile("testdata/dependent/params.txtar")
	if err != nil {
		t.Fatal(err)
	}
	b := runBuild(t, fromArchive(t, a), eval.Options{})
	ctx := context.Background()
	l := namedRecord(b.checked, "L")
	ev := b.values[eval.Root{Pkg: "a", Name: "evs"}].(*value.Table).Entries[1]
	via := &value.Prov{Kind: value.ProvJSON}
	self := &value.Record{T: l, Fields: make([]value.Value, len(l.Fields)), Set: make([]bool, len(l.Fields)), P: via}
	lim, ok := b.ev.Default(ctx, l.Fields[0], self, map[*types.Param]value.Value{l.Params[0]: ev}, via)
	if !ok || lim.CanonText() != "7" || lim.Prov().Kind != value.ProvDefault || lim.Prov().Via != via {
		t.Fatalf("lim = %v, %v, want 7, default via the object", lim, ok)
	}
	if got := b.prog.fs.Locate(lim.Prov().Span); got.Line != 16 || got.Col != 14 {
		t.Errorf("lim at %d:%d, want its default expression at 16:14", got.Line, got.Col)
	}
	evs := &types.RefType{Target: &types.Collection{Kind: types.CollLet, Pkg: "a", Name: "evs", Elem: ev.T}}
	if rec, ok := b.ev.Deref(ctx, &value.Ref{T: evs, Key: value.Key{S: "b"}}); !ok || rec != ev {
		t.Errorf("Deref(b) = %v, %v, want the entry b", rec, ok)
	}
}

// EVALUATION.md §3.2, §3.4, §5: a decode's E3501, E3505 and E4301 are at the loaded ref, with its pointer.
func TestDecodeHostFindings(t *testing.T) {
	a, err := txtar.ParseFile("testdata/dependent/params.txtar")
	if err != nil {
		t.Fatal(err)
	}
	a.Files = append(a.Files, txtar.File{Name: "a/load.canon", Data: []byte(loadingDecl)})
	span := source.Span{File: 1, Start: 3, End: 7}
	var evs, perR *types.Collection
	loaded := func(c *types.Collection, key string) *value.Ref {
		return &value.Ref{T: &types.RefType{Target: c}, Key: value.Key{S: key}, P: &value.Prov{Kind: value.ProvJSON, Span: span, Pointer: "/0/ev"}}
	}
	var oks []bool
	serve := func(ev *eval.Evaluator, _ *syntax.LoadExpr, _ types.Type) (value.Value, bool) {
		ctx := context.Background()
		switch len(oks) {
		case 0:
			_, ok := ev.Deref(ctx, loaded(evs, "zz"))
			oks = append(oks, ok)
		case 1:
			_, ok := ev.Deref(ctx, loaded(perR, "a"))
			oks = append(oks, ok)
		default:
			ev.Cycle(ctx, loaded(evs, "a"))
		}
		return nil, false
	}
	p := fromArchive(t, a)
	probe := runBuild(t, p, eval.Options{})
	ev := namedRecord(probe.checked, "Ev")
	evs = &types.Collection{Kind: types.CollLet, Pkg: "a", Name: "evs", Elem: ev}
	perR = &types.Collection{Kind: types.CollField, Pkg: "a", Owner: namedRecord(probe.checked, "R"), Elem: ev}
	b := runBuildWith(t, fromArchive(t, a), eval.Options{}, serve)
	if !slices.Equal(oks, []bool{false, false}) {
		t.Fatalf("Deref of a missing key, then of an unbound ref: ok = %v, want false twice", oks)
	}
	for _, code := range []diag.Code{diag.E3501.Def().Code, diag.E3505.Def().Code, diag.E4301.Def().Code} {
		if f := findingOf(b.bags, code); f == nil || f.Span != span || f.Pointer != "/0/ev" {
			t.Errorf("%s: %+v, want it at the ref, pointer /0/ev", code, f)
		}
	}
}

// loadingDecl is a load for TestDecodeHostFindings's host to decode.
const loadingDecl = `package a

/// Its host meets a missing key.
let missing: [Ev] = load("@resource/x.json")

/// Its host meets an unbound ref.
let unbound: [Ev] = load("@resource/x.json")

/// Its host meets a cycle.
let cyclic: [Ev] = load("@resource/x.json")
`

// namedRecord is the record named name.
func namedRecord(prog *check.Program, name string) *types.RecordType {
	for _, pkg := range prog.Packages {
		for _, obj := range pkg.Decls {
			if rt, ok := obj.Type().(*types.RecordType); ok && obj.Name() == name {
				return rt
			}
		}
	}
	return nil
}

// findingOf is the first finding of code in any bag, in package order; nil for none.
func findingOf(bags check.Bags, code diag.Code) *diag.Finding {
	for _, pkg := range slices.Sorted(maps.Keys(bags)) {
		fs := bags[pkg].Findings()
		for i := range fs {
			if fs[i].Code == code {
				return &fs[i]
			}
		}
	}
	return nil
}
