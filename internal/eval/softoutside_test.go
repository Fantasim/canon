package eval_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
	"golang.org/x/tools/txtar"
)

// softOutsideCase holds the tests whose own statements produce findings outside every expect.
const softOutsideCase = "testdata/tests/soft_outside.txtar"

// verdict is a test's outcome by its title: whether it failed, and whether it stopped.
type verdict struct {
	title           string
	failed, stopped bool
}

// EVALUATION.md §10.4, DECISIONS 448: an error outside every expect fails the test unstopped; a warning there, or one inside an expect, does not (pins).
func TestErrorOutsideExpectsFailsTest(t *testing.T) {
	a, err := txtar.ParseFile(softOutsideCase)
	if err != nil {
		t.Fatal(err)
	}
	b := runBuild(t, fromArchive(t, a), eval.Options{})
	want := []verdict{
		{`"a let"`, true, false},
		{`"a call, twice"`, true, false},
		{`"a key given twice"`, true, false},
		{`"an error, then a stop"`, true, true},
		{`"a warning"`, false, false},
		{`"inside an expect"`, true, false},
	}
	got := verdicts(b)
	if len(got) != len(want) {
		t.Fatalf("%d tests, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("test %s: failed %t, stopped %t; want failed %t, stopped %t", w.title, got[i].failed, got[i].stopped, w.failed, w.stopped)
		}
	}
}

// verdicts runs every test of b's packages in order and keeps each one's outcome.
func verdicts(b *build) []verdict {
	var out []verdict
	for _, pkg := range b.selected(nil) {
		for _, obj := range pkg.Decls {
			d, ok := obj.Decl().(*syntax.TestDecl)
			if !ok || obj.Kind() != check.ObjTest {
				continue
			}
			res := b.ev.Test(context.Background(), d, subjects{b: b, pkg: pkg.Path})
			out = append(out, verdict{title: testTitle(obj.File(), d), failed: res.Failed, stopped: res.Stopped})
		}
	}
	return out
}
