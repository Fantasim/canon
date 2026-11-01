package check_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
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

// TYPES.md §1, §7.4, §15, DECISIONS 150, EVALUATION.md §1: a fold building a record or case reads its defaults checked: the program's own findings, never an internal error.
func TestFoldBuildsUncheckedRecord(t *testing.T) {
	for _, tc := range foldRecordCases {
		t.Run(tc.name, func(t *testing.T) {
			var want []diag.Code
			for _, d := range tc.want {
				want = append(want, d.Code)
			}
			if got := errorCodes(buildUnder(t, shopProject, tc.text).List); !slices.Equal(got, sortedCodes(want)) {
				t.Errorf("a build reports %v, want %v (HEAD: %s)", got, want, tc.head)
			}
		})
	}
}

// TYPES.md §1: a bound building a record its default broke is broken, with no finding of its own.
func TestFoldBuildsBrokenRecord(t *testing.T) {
	prog, out := checkShopEval(t, brokenDefault)
	if !brokenNamed(prog, "T") || !brokenNamed(prog, "n") {
		t.Errorf("T and n are not both broken:\n%s", out)
	}
}

// TYPES.md §15, DECISIONS 412: a record's defaults checked early for a fold are not checked again: the default folds once, its finding is reported once.
func TestFoldBuildsChecksDefaultsOnce(t *testing.T) {
	const text = "package shop\n\nconst C = %d\n\nlocal let n: Int(0..(T {}).y) = 1\n\nlocal record T {\n  x: Int(1..) = C\n  y: Int = 4\n}\n"
	for _, tc := range []struct {
		name         string
		c, folds, rf int
	}{
		{"a valid default: n's two ends, x's bound, x's default", 1, 4, 0},
		{"a default outside its refinement breaks T: n's low end, x's bound, x's default", 0, 3, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			set := &source.FileSet{}
			parse := diag.NewBag(set, "")
			src, err := set.Add(shopFile, "/"+shopFile, fmt.Appendf(nil, text, tc.c))
			if err != nil {
				t.Fatal(err)
			}
			f := syntax.Parse(src, syntax.FileSource, parse)
			bags := check.Bags{}
			fold := &countingFolder{inner: eval.NewFolder(bags, eval.Options{})}
			check.Check(context.Background(), exampleProject(), []*syntax.File{f}, bags, fold)
			out := render(t, set, parse, bags)
			if fold.calls != tc.folds {
				t.Errorf("%d folds, want %d", fold.calls, tc.folds)
			}
			if got := strings.Count(out, "["+string(diag.E3204.Def().Code)+"]"); got != tc.rf {
				t.Errorf("%d range findings, want %d:\n%s", got, tc.rf, out)
			}
		})
	}
}

// foldRecordWorld folds a bound building a later record whose nested default breaks it.
var foldRecordWorld = map[string]string{
	"q/q.canon": "package q\n\nlocal let n: Int(0..(T {}).u.y) = 1\n\nlocal record T {\n  u: U = {}\n}\n\nlocal record U {\n  y: Int(1..) = 0\n}\n" +
		"\nlocal record Row {\n  n: Int\n}\n\nlocal let rows: table Row = {}\n",
	"q/x.canon": "package q\n\nentry rows.x { n: 1 }\n",
}

// IMPLEMENTATION-PLAN §7.6 (NFR-02), TYPES.md §1: after a fold checked a record's defaults first, a Recheck reports what a cold Check does.
func TestFoldBuildsRecheck(t *testing.T) {
	valid := maps.Clone(foldRecordWorld)
	valid["q/q.canon"] = strings.Replace(valid["q/q.canon"], "Int(1..) = 0", "Int(1..) = 4", 1)
	for _, tc := range []struct {
		name  string
		srcs  map[string]string
		steps []step
	}{
		{"its default broken", foldRecordWorld, []step{{"q/x.canon", "n: 1", "n: 2", true}, {"q/x.canon", "n: 2", "n: \"x\"", true}}},
		{"its default valid", valid, []step{{"q/x.canon", "n: 1", "n: 2", true}, {"q/x.canon", "n: 2", "n: \"x\"", true}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(t, tc.srcs)
			_, s, _ := w.session()
			for _, st := range tc.steps {
				s = w.step(s, st)
			}
		})
	}
}

// TYPES.md §15, EVALUATION.md §3.2: a build emitting a value that reads a const cycle through a default is its E4301, never a value missing.
func TestFoldBuildsConstCycleBuild(t *testing.T) {
	const emit = "\nemit json { out: \"out/v.json\", values: [v] }\n"
	for _, tc := range []struct{ name, text string }{
		{"a let reading the const", cycleK + "\nlet v: Int = K\n" + emit},
		{"a let of the record", cycleK + "\nlet v: T = {}\n" + emit},
		{"two consts", "package a\n\nconst K = K2 + 1\n\nconst K2 = (T {}).a\n\nrecord T {\n  a: Int = K\n}\n\nlet v: Int = 1\n" + emit},
		{"a default folding a const that builds the cycle's record", "package a\n" + cycleZ + "\nlet v: Int = Y\n" + emit},
		{"a default read beside the cycle", "package a\n\nconst K = (T {}).b\n\nrecord T {\n  a: Int = K\n  b: Int = 2\n}\n\nlet v: Int = K\n" + emit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fsys := shopFS{shopDir + "/" + projectFile: {Data: []byte(shopProject)}, shopDir + "/" + builtFile: {Data: []byte(tc.text)}}
			p, err := build.Open(fsys, "/"+shopDir, build.Options{})
			if err != nil {
				t.Fatal(err)
			}
			res, err := p.Build(context.Background(), build.BuildOptions{Packages: []string{builtPackage}, Check: true})
			if err != nil {
				t.Fatalf("the build failed: %v", err)
			}
			if got := errorCodes(res.List); !slices.Equal(got, []diag.Code{diag.E4301.Def().Code}) {
				t.Errorf("a build reports %v, want exactly [%s]", got, diag.E4301.Def().Code)
			}
		})
	}
}

// EVALUATION.md §3.2, DECISIONS 187: however a fold or no fold reaches a cycle, its E4301 is one finding, at the read of its first const, with the path of the const before it.
func TestFoldBuildsCycleIsOneFinding(t *testing.T) {
	const want = "error[%s]  shop/shop.canon:%d:12\n  K2: cycle between values: K1 -> K2 -> K1\n\n"
	code := diag.E4301.Def().Code
	for _, tc := range []struct {
		name, lead string
		line       int
	}{
		{"no fold", "", 12},
		{"a bound reading K2", "\nlocal let n: Int(0..K2) = 1\n", 14},
		{"a bound reading K1", "\nlocal let n: Int(0..K1) = 1\n", 14},
		{"a bound building T2", "\nlocal let n: Int(0..(T2 {}).b) = 1\n", 14},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, out := checkShopEval(t, "package shop\n"+tc.lead+cycleK1K2)
			finding := fmt.Sprintf(want, code, tc.line)
			if !strings.Contains(out, finding) || strings.Count(out, "["+string(code)+"]") != 1 {
				t.Errorf("got\n%s\nwant once\n%s", out, finding)
			}
		})
	}
}

// TYPES.md §1, §3.1, §15: a bound building another package's record reads its defaults checked.
func TestFoldBuildsImportedRecord(t *testing.T) {
	fsys := shopFS{
		shopDir + "/" + projectFile: {Data: []byte(shopProject)},
		shopDir + "/b/b.canon":      {Data: []byte(importedPkg)},
		shopDir + "/" + builtFile:   {Data: []byte("package a\n\nimport b\n\nlocal let n: Int(0..(b.T {}).u.y) = 1\n")},
	}
	p, err := build.Open(fsys, "/"+shopDir, build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Check(context.Background(), []string{builtPackage})
	if errors.Is(err, build.ErrInternal) {
		t.Fatalf("the evaluator met a state the checker excludes: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	if got := errorCodes(res.List); len(got) != 0 {
		t.Errorf("a build reports %v, want none", got)
	}
}
