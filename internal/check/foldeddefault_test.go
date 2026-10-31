package check_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

const foldedHead = "package shop\n\nconst ZERO = 0\n\nlocal record R {\n  a: Int(1..) = %s\n}\n"

// TYPES.md §7.4, DECISIONS 412: the findings of a default naming a constant equal those of the literal it folds to, whether no value, one, several or one setting the field itself uses it.
func TestFoldedDefaultIsItsLiteral(t *testing.T) {
	for _, tc := range []struct{ name, values string }{
		{"no value", ""},
		{"one value", "\nlocal let one: R = R {}\n"},
		{"two values", "\nlocal let one: R = R {}\n\nlocal let two: R = R {}\n"},
		{"a value sets the field", "\nlocal let set: R = R { a: 5 }\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, folded := checkShop(t, fmt.Sprintf(foldedHead, "ZERO")+tc.values)
			_, _, literal := checkShop(t, fmt.Sprintf(foldedHead, "0")+tc.values)
			if !strings.Contains(literal, "["+string(diag.E3204.Def().Code)+"]") {
				t.Fatalf("the literal default is not reported:\n%s", literal)
			}
			if folded != literal {
				t.Errorf("= ZERO differs from = 0:\n%s\nwant\n%s", folded, literal)
			}
		})
	}
}

// checkShopEval is checkShop with the evaluator as the Folder, whose folds can fail.
func checkShopEval(t *testing.T, text string) (*check.Program, string) {
	t.Helper()
	set := &source.FileSet{}
	parse := diag.NewBag(set, "")
	src, err := set.Add(shopFile, "/"+shopFile, []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	f := syntax.Parse(src, syntax.FileSource, parse)
	bags := check.Bags{}
	prog := check.Check(context.Background(), exampleProject(), []*syntax.File{f}, bags, eval.NewFolder(bags, eval.Options{}))
	return prog, render(t, set, parse, bags)
}

const (
	overflowHead = "package shop\n\nconst MAXI = 9223372036854775807\n\nlocal record S {\n  b: Int = MAXI + 1\n}\n"
	failedConst  = "package shop\n\nconst MAXI = 9223372036854775807\n\nconst BAD = MAXI + 1\n\n" +
		"local record R {\n  a: Int = BAD\n}\n\nlocal fn f() -> Int {\n  let y: Int(0..BAD) = 1\n  return y\n}\n"
	spentConst = "package shop\n\nconst BIG = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12].map(x => x * x).sum()\n\n" +
		"local record R {\n  a: Int = BIG\n}\n"
	budgetProject = "project demo {\n  canon: \"0.1\"\n  budget: 10\n}\n"
)

// DECISIONS 150, TYPES.md §1 (log-2026-10-31, CHECK-OWED ruling (2) withdrawn): a default whose fold fails reports the fold's finding at check time and breaks its record, as a const's fold does; the evaluator does not report it again.
func TestFailingFoldIsReported(t *testing.T) {
	for _, tc := range []struct{ name, values string }{
		{"no value", ""},
		{"one value", "\nlocal let one: S = S {}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prog, out := checkShopEval(t, overflowHead+tc.values)
			if got := strings.Count(out, "["+string(diag.E4101.Def().Code)+"]"); got != 1 {
				t.Errorf("%d overflow findings at check time, want 1:\n%s", got, out)
			}
			if !brokenNamed(prog, "S") {
				t.Error("S is not broken by its failing default")
			}
			builtErrors(t, shopProject, overflowHead+tc.values, diag.E4101.Def().Code)
		})
	}
}

// DECISIONS 150, TYPES.md §1: a const whose fold failed, read by a default and then by a bound, or by two bounds, is reported once, at the const; a later fold reading it is no E3015.
func TestFailedConstIsReportedOnce(t *testing.T) {
	for _, tc := range []struct{ name, text string }{
		{"a default, then a bound", failedConst},
		{"two bounds", failedConst + "\nlocal fn g() -> Int {\n  let z: Int(0..BAD) = 1\n  return z\n}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			builtErrors(t, shopProject, tc.text, diag.E4101.Def().Code)
		})
	}
}

// DECISIONS 150, TYPES.md §1: a bound reading another package's const whose fold failed is no E3015; the const's finding is its package's.
func TestFailedImportedConstIsNoE3015(t *testing.T) {
	fsys := shopFS{
		shopDir + "/" + projectFile: {Data: []byte(shopProject)},
		shopDir + "/b/b.canon":      {Data: []byte("package b\n\nconst MAXI = 9223372036854775807\n\nconst BAD = MAXI + 1\n")},
		shopDir + "/" + builtFile: {Data: []byte("package a\n\nimport b\n\nlocal record R {\n  a: Int = b.BAD\n}\n\n" +
			"local fn f() -> Int {\n  let y: Int(0..b.BAD) = 1\n  return y\n}\n")},
	}
	p, err := build.Open(fsys, "/"+shopDir, build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Check(context.Background(), []string{builtPackage})
	if err != nil {
		t.Fatal(err)
	}
	if got := errorCodes(res.List); !slices.Equal(got, []diag.Code{diag.E4101.Def().Code}) {
		t.Errorf("a build reports %v, want exactly [%s]", got, diag.E4101.Def().Code)
	}
}

// DECISIONS 263, EVALUATION.md §12.2: a const whose fold spends the budget, read by a default and by a let, is the package's E4401, never lost.
func TestSpentFoldIsReported(t *testing.T) {
	for _, tc := range []struct{ name, values string }{
		{"a value takes the default", "\nlocal let r: R = R {}\n"},
		{"a let reads the const", "\nlocal let n: Int = BIG\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			builtErrors(t, budgetProject, spentConst+tc.values, diag.E4401.Def().Code)
		})
	}
}

// DECISIONS 263 (log-2026-10-31): once a bound's fold spent the package's budget, a default is not folded: E4401 alone, no E3015 budget at the default.
func TestSpentBudgetLeavesDefaults(t *testing.T) {
	const text = "package shop\n\nconst BIG = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12].map(x => x * x).sum()\n\nconst ONE = 1\n\n" +
		"local fn f() -> Int {\n  let y: Int(0..BIG) = 1\n  return y\n}\n\nlocal record R {\n  a: Int = ONE + 1\n}\n"
	builtErrors(t, budgetProject, text, diag.E4401.Def().Code)
}

// EVALUATION.md §1, TYPES.md §1, §13.1: a default building a record is never folded while checking: the record may be broken, or its body unchecked yet, so its own findings stay its own, once.
func TestRecordDefaultIsNotFolded(t *testing.T) {
	const later = "package shop\n\nlocal record A {\n  t: T = {}\n}\n\nlocal record T {\n  %s\n}\n"
	for _, tc := range []struct {
		name, text string
		want       *diag.Def
	}{
		{"its record holds itself", "package shop\n\nlocal record Config {\n  zzSelf: Config = {}\n}\n", diag.E3022.Def()},
		{"an unknown name in a later default", fmt.Sprintf(later, "x: Int = NOPE"), diag.E2102.Def()},
		{"a later default outside its refinement", fmt.Sprintf(later, "x: Int(1..) = 0"), diag.E3204.Def()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			builtErrors(t, shopProject, tc.text, tc.want.Code)
		})
	}
}

// brokenNamed reports a declaration of prog named name in Info.Broken.
func brokenNamed(prog *check.Program, name string) bool {
	for _, p := range prog.Packages {
		for _, o := range p.Decls {
			if o.Name() == name && prog.Info.Broken[o] {
				return true
			}
		}
	}
	return false
}

// builtErrors fails t unless a build of text, package shop renamed a, under proj reports exactly the one error want.
func builtErrors(t *testing.T, proj, text string, want diag.Code) {
	t.Helper()
	text = strings.Replace(text, "package "+shopPackage, "package "+builtPackage, 1)
	if got := errorCodes(buildUnder(t, proj, text).List); !slices.Equal(got, []diag.Code{want}) {
		t.Errorf("a build reports %v, want exactly [%s]", got, want)
	}
}

// TYPES.md §7.3: a Float32 value that overflows binary32 is storage's E3202, never compared as infinity.
func TestFloat32OverflowIsNotARangeFinding(t *testing.T) {
	_, _, out := checkShop(t, "package shop\n\nlocal let z: Float32(..=1e39) = 4e38\n")
	if strings.Contains(out, "["+string(diag.E3204.Def().Code)+"]") {
		t.Errorf("a Float32 overflow is reported as a range finding:\n%s", out)
	}
}

// TYPES.md §7.4, §15, DECISIONS 412: every constant form the Folder folds is checked as its value, at the default; a default reading a field is not one.
func TestFoldedDefaultForms(t *testing.T) {
	for _, tc := range []struct {
		name, field string
		want        int
	}{
		{"operator", "a: Int(1..) = ZERO + 0", 1},
		{"built-in", "a: Int(1..) = floor(0.5)", 1},
		{"method", "a: Int(..=4) = [2, 3].sum()", 1},
		{"map value", "a: {String: Int(1..)} = {\"k\": ZERO}", 1},
		{"map key", "a: {Int(1..): Int} = {(ZERO): 1}", 1},
		{"accepted", "a: Int(1..) = ZERO + 1", 0},
		{"a field read", "n: Int = 0\n  a: Int(1..) = n", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, out := checkShopEval(t, "package shop\n\nconst ZERO = 0\n\nlocal record R {\n  "+tc.field+"\n}\n")
			if got := strings.Count(out, "["+string(diag.E3204.Def().Code)+"]"); got != tc.want {
				t.Errorf("%d range findings, want %d:\n%s", got, tc.want, out)
			}
		})
	}
}
