package rules_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/rules"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"golang.org/x/tools/txtar"
)

const treeSource = `package teamboard

record Deck {
  maxHidden: Int
  columns: [Column]

  check maxHidden < columns.len() else "too many hidden"
}

record Column {
  label: String

  check self.label != "" else "empty label"
}

let deck: Deck = { maxHidden: 1, columns: [{ label: "a" }, { label: "" }] }
let again: Deck = deck
`

type tree struct {
	fx            *fixture
	deck, column  *types.RecordType
	hidden, empty *syntax.CheckDecl
	value         *value.Record
	cols          []*value.Record
}

func newTree(t *testing.T) *tree {
	fx := newFixture(t, "teamboard/deck.canon", []byte(treeSource))
	tr := &tree{fx: fx, hidden: fx.check("check maxHidden"), empty: fx.check("check self")}
	tr.column = record("Column", field("label", types.StringType))
	tr.column.Checks = []*syntax.CheckDecl{tr.empty}
	tr.deck = record("Deck", field("maxHidden", types.IntType), field("columns", &types.ListType{Elem: tr.column}))
	tr.deck.Checks = []*syntax.CheckDecl{tr.hidden}
	for _, label := range []string{`"a"`, `""`} {
		tr.cols = append(tr.cols, &value.Record{T: tr.column, Fields: []value.Value{str(label, fx.lit(label, "let deck"))}, P: fx.lit("{ label: " + label)})
	}
	one := &value.Int{V: 1, T: types.IntType, P: fx.lit("1", "let deck")}
	cols := &value.List{Elems: []value.Value{tr.cols[0], tr.cols[1]}, P: fx.lit("[", "let deck")}
	tr.value = &value.Record{T: tr.deck, Fields: []value.Value{one, cols}, P: fx.lit("{", "let deck")}
	fx.let("deck", tr.value)
	fx.let("again", tr.value)
	return tr
}

// EVALUATION.md §8.1: once per instance, pre-order; `again` shares deck's instance.
func TestInstancesOnceInPreOrder(t *testing.T) {
	tr := newTree(t)
	tr.fx.run()
	want := []*syntax.CheckDecl{tr.hidden, tr.empty, tr.empty}
	if !slices.Equal(tr.fx.ev.calls, want) {
		t.Errorf("runs %v, want deck, then each column", tr.fx.ev.calls)
	}
}

// EVALUATION.md §7.3: an invalid value skips the instances above it, not those beside it.
func TestInvalidSubtreeSkipsChecks(t *testing.T) {
	tr := newTree(t)
	tr.fx.ev.invalid[tr.cols[1].Fields[0]] = true
	tr.fx.run()
	if want := []*syntax.CheckDecl{tr.empty}; !slices.Equal(tr.fx.ev.calls, want) {
		t.Errorf("runs %v, want the first column's only", tr.fx.ev.calls)
	}
}

// VIEWMODEL.md J15: a one-line check without `at` reads the fields its condition names.
func TestReads(t *testing.T) {
	tr := newTree(t)
	syntax.Inspect(tr.hidden.Cond, func(n syntax.Node) bool {
		if id, ok := n.(*syntax.IdentExpr); ok {
			tr.fx.prog.Info.Uses[id] = &object{kind: check.ObjField, name: id.Name}
		}
		return true
	})
	syntax.Inspect(tr.empty.Cond, func(n syntax.Node) bool {
		if sel, ok := n.(*syntax.SelectorExpr); ok {
			tr.fx.prog.Info.Selections[sel] = &check.Selection{Kind: check.SelField, Obj: &object{kind: check.ObjField, name: sel.Name.Name}}
		}
		return true
	})
	tr.fx.ev.scripts[tr.hidden] = fails("too many hidden")
	tr.fx.ev.scripts[tr.empty] = failsFor("empty label", tr.cols[1])
	tr.fx.run()
	got := tr.fx.bag.Findings()
	if len(got) != 2 || !slices.Equal(got[0].Reads, []string{"maxHidden", "columns"}) || !slices.Equal(got[1].Reads, []string{"label"}) {
		t.Errorf("findings %+v", got)
	}
}

// A cancelled context runs no check.
func TestCancelledRunsNothing(t *testing.T) {
	tr := newTree(t)
	r := rules.New(tr.fx.ev, tr.fx.prog, map[string]*diag.Bag{pkg: tr.fx.bag})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := errors.Join(r.Instances(ctx, eval.Root{Pkg: pkg, Name: "deck"}, tr.value), r.Package(ctx, tr.fx.prog.Packages[0]))
	if err != nil || len(tr.fx.ev.calls) != 0 {
		t.Errorf("runs %v, err %v", tr.fx.ev.calls, err)
	}
}

// A package without a bag is a Go error, never a panic.
func TestNoBag(t *testing.T) {
	tr := newTree(t)
	r := rules.New(tr.fx.ev, tr.fx.prog, nil)
	for _, err := range []error{
		r.Instances(context.Background(), eval.Root{Pkg: pkg, Name: "deck"}, tr.value),
		r.Package(context.Background(), tr.fx.prog.Packages[0]),
		r.Names(tr.fx.prog.Packages[0]),
	} {
		if !errors.Is(err, rules.ErrNoBag) {
			t.Errorf("err %v", err)
		}
	}
}

// TYPES.md §1, EVALUATION.md §1: the checks of a broken record never run.
func TestBrokenRecord(t *testing.T) {
	tr := newTree(t)
	tr.fx.prog.Info.Broken[tr.fx.typeName(tr.column)] = true
	tr.fx.typeName(tr.deck)
	tr.fx.run()
	if want := []*syntax.CheckDecl{tr.hidden}; !slices.Equal(tr.fx.ev.calls, want) {
		t.Errorf("runs %v, want Deck's only", tr.fx.ev.calls)
	}
}

// EVALUATION.md §8.3: fail(at, …) with no value to locate is reported without a location.
func TestFailWithoutValue(t *testing.T) {
	tr := newTree(t)
	tr.fx.ev.scripts[tr.hidden] = func(value.Value) rules.Run {
		return rules.Run{Reports: []rules.Report{{Message: "nowhere"}}}
	}
	tr.fx.run()
	got := tr.fx.bag.Findings()
	if len(got) != 1 || got[0].Span.File != 0 || got[0].Path != "" {
		t.Errorf("findings %+v", got)
	}
}

// EVALUATION.md §8.1, TYPES.md §12.1: on a case value the variant-level checks run first, then the case's.
func TestVariantLevelChecksFirst(t *testing.T) {
	a, err := txtar.ParseFile("testdata/findings/E5001_2.txtar")
	if err != nil {
		t.Fatal(err)
	}
	fx := fromArchive(t, a)
	variantLevelCase(fx)
	fx.run()
	spent, capped := fx.check("check spent"), fx.check("check capped")
	if want := []*syntax.CheckDecl{spent, capped, spent}; !slices.Equal(fx.ev.calls, want) {
		t.Errorf("runs %v, want spent, capped on gold, then spent on nothing", fx.ev.calls)
	}
}
