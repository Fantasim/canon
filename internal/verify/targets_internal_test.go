package verify

import (
	"context"
	"reflect"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	targetsTable = "rewards"
	targetsPick  = "pick"
	targetsKept  = "gold"
	targetsGone  = "gem"
	targetsRuns  = 3
	targetsTree  = "Tree"
	targetsNodes = "nodes"
)

// targetsEval forces each root to what roots holds.
type targetsEval struct {
	roots map[eval.Root]value.Value
}

func (e *targetsEval) Force(_ context.Context, r eval.Root) (value.Value, bool) {
	v, ok := e.roots[r]
	return v, ok
}

func (*targetsEval) MarkInvalid(value.Value) {}

func (*targetsEval) Invalid(value.Value) bool { return false }

func (*targetsEval) Where(context.Context, *types.Predicate, value.Value, string) (bool, bool) {
	return true, true
}

// rewards is a table of reward entries, one per key, and the ref type into it.
func rewards(keys ...string) (*value.Table, *types.RefType) {
	rt := &types.RecordType{Pkg: cachePkg, Name: "Reward"}
	coll := &types.Collection{Kind: types.CollLet, Pkg: cachePkg, Name: targetsTable, Elem: rt}
	tt := &types.TableType{Elem: rt}
	tbl := &value.Table{T: tt}
	for _, k := range keys {
		tbl.Entries = append(tbl.Entries, &value.Record{T: rt, Ident: &value.Identity{Coll: coll, Key: value.Key{S: k}}})
	}
	return tbl, &types.RefType{Target: coll}
}

// pick verifies a ref to key of tbl through a verifier of its own over ix, and is its findings.
func pick(t *testing.T, ix *Index, tbl *value.Table, ref *types.RefType, key string) []diag.Finding {
	t.Helper()
	return verifyRef(t, ix, tbl, &value.Ref{T: ref, Key: value.Key{S: key}, P: &value.Prov{Kind: value.ProvLiteral}})
}

// verifyRef verifies r through a verifier of its own over ix, tbl the top-level table, and is its findings.
func verifyRef(t *testing.T, ix *Index, tbl *value.Table, r *value.Ref) []diag.Finding {
	t.Helper()
	ev := &targetsEval{roots: map[eval.Root]value.Value{{Pkg: cachePkg, Name: targetsTable}: tbl}}
	bag := diag.NewBag(&source.FileSet{}, cachePkg)
	if _, err := NewShared(ix, ev, map[string]*diag.Bag{cachePkg: bag}, nil).Check(context.Background(), eval.Root{Pkg: cachePkg, Name: targetsPick}, r); err != nil {
		t.Fatal(err)
	}
	return bag.Findings()
}

// EVALUATION.md §3.4; DECISIONS 515: a ref into its instance's own collection leaves the Index's targets alone.
func TestTargetsLeaveInstanceCollections(t *testing.T) {
	ix := NewIndex(nil)
	nodes, _ := rewards(targetsKept)
	tree := &types.RecordType{Pkg: cachePkg, Name: targetsTree, Fields: []*types.Field{{Name: targetsNodes, Type: nodes.T}}}
	coll := &types.Collection{Kind: types.CollField, Owner: tree, FieldPath: []string{targetsNodes}}
	owner := &value.Record{T: tree, Fields: []value.Value{nodes}}
	for i := range targetsRuns {
		r := &value.Ref{T: &types.RefType{Target: coll}, Key: value.Key{S: targetsKept}, Owner: owner, P: &value.Prov{Kind: value.ProvLiteral}}
		if found := verifyRef(t, ix, nodes, r); len(found) != 0 || ix.Indexed() != 0 {
			t.Fatalf("verifier %d: findings %v, %d shared targets; want none, none", i, found, ix.Indexed())
		}
	}
}

// indexed is the index ix's verifiers keep of coll, and whether they keep one.
func indexed(ix *Index, coll value.Value) (uintptr, bool) {
	ix.targets.mu.Lock()
	defer ix.targets.mu.Unlock()
	idx, ok := ix.targets.byColl[coll]
	return reflect.ValueOf(idx).Pointer(), ok
}

// EVALUATION.md §5; DECISIONS 446, 515: verifiers made over one Index index a ref target once.
func TestTargetsSharedByVerifiers(t *testing.T) {
	ix := NewIndex(nil)
	tbl, ref := rewards(targetsKept, targetsGone)
	var first uintptr
	for i := range targetsRuns {
		if found := pick(t, ix, tbl, ref, targetsGone); len(found) != 0 {
			t.Fatalf("verifier %d: findings %v, want none", i, found)
		}
		at, ok := indexed(ix, tbl)
		switch {
		case !ok || len(ix.targets.byColl) != 1:
			t.Fatalf("verifier %d: %d collections indexed, want the table", i, len(ix.targets.byColl))
		case i == 0:
			first = at
		case at != first:
			t.Fatalf("verifier %d indexed the table again", i)
		}
	}
}

// EVALUATION.md §5; DECISIONS 446, 515: ref targets die with their Index, an Apart one's too.
func TestTargetsDieWithTheirIndex(t *testing.T) {
	cache := &IndexCache{}
	prog, _ := checkedFiles(t, map[string]string{cacheKept: keptSource, cacheEdited: editedBefore}, nil)
	ix := cache.Index(prog)
	tbl, ref := rewards(targetsKept, targetsGone)
	apart := ix.Apart()
	if apart.src != ix.src || apart.targets == ix.targets {
		t.Fatal("an Apart index must share the sources and keep its own targets")
	}
	if found := pick(t, apart, tbl, ref, targetsGone); len(found) != 0 {
		t.Fatalf("apart: findings %v, want none", found)
	}
	if _, ok := indexed(ix, tbl); ok {
		t.Fatal("an Apart index's verifier indexed into the run's targets")
	}
	pick(t, ix, tbl, ref, targetsGone)
	edited, _ := rewards(targetsKept)
	next := cache.Index(prog)
	if _, ok := indexed(next, tbl); ok {
		t.Fatal("the next run's Index holds the last run's targets")
	}
	found := pick(t, next, edited, ref, targetsGone)
	if len(found) != 1 || found[0].Code != diag.E3501.Def().Code {
		t.Fatalf("after the edit: findings %v, want one %s", found, diag.E3501.Def().Code)
	}
}
