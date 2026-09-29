package check_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// boundedSource declares a record whose bounds the check folds.
const boundedSource = "package p\n\n/// A thing sold.\nrecord Item {\n  /// Its price.\n  price: Int(0..=100)\n}\n"

// silentFolder fails every fold without a finding, as eval's does once the step budget is spent.
type silentFolder struct{}

func (silentFolder) Fold(context.Context, check.Object, syntax.Expr, *check.Info) (value.Value, bool) {
	return nil, false
}

// DECISIONS 150, TYPES.md §15 (log-2026-09-29 M4 B2-r): a fold failing without a finding is E3015, the budget spent or not.
func TestFoldFailingSilently(t *testing.T) {
	for _, session := range []bool{false, true} {
		prog, bags := checkBounded(t, silentFolder{}, session)
		n := 0
		for _, f := range bags["p"].Findings() {
			if f.Code == diag.E3015.Def().Code {
				n++
			}
		}
		if n == 0 {
			t.Errorf("session %t: no %s", session, diag.E3015.Def().Code)
		}
		if item := prog.Packages[0].Decls[0]; item.Name() != "Item" || !prog.Info.Broken[item] {
			t.Errorf("session %t: %s: want the record broken", session, item.Name())
		}
	}
}

// checkBounded checks boundedSource with fold, through Check or CheckSession.
func checkBounded(t *testing.T, fold check.Folder, session bool) (*check.Program, check.Bags) {
	t.Helper()
	fs := &source.FileSet{}
	src, err := fs.Add("p/p.canon", "/p/p.canon", []byte(boundedSource))
	if err != nil {
		t.Fatal(err)
	}
	files := []*syntax.File{syntax.Parse(src, syntax.FileSource, diag.NewBag(fs, ""))}
	bags := check.Bags{}
	if session {
		prog, _ := check.CheckSession(context.Background(), exampleProject(), files, bags, fold)
		return prog, bags
	}
	return check.Check(context.Background(), exampleProject(), files, bags, fold), bags
}
