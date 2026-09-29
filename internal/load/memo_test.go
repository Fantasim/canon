package load_test

import (
	"bytes"
	"context"
	"path"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"github.com/fantasim/canonlang/internal/types"
)

// memoTree is the files the replay cases start from.
var memoTree = map[string]string{
	"data/a.json":  `{"label": "a"}`,
	"data/b.json":  `{"label": "b"}`,
	"data/c.txt":   "not matched",
	"other/x.json": `{"label": "x"}`,
	"t.txt":        "text",
	"r.csv":        "a,b\n",
}

// recordedOver records e's load over memoTree: its loader, file tree and inputs.
func recordedOver(t *testing.T, e *syntax.LoadExpr, typ types.Type) (*load.Loader, fstest.MapFS, *load.Inputs) {
	t.Helper()
	l, req := loaderFor(t, memoTree)
	m := l.FS.(memFS).m
	got, err := l.Recorded(context.Background(), req, e, typ)
	if err != nil || !got.OK || got.Inputs == nil {
		t.Fatalf("ok=%v err=%v inputs=%v findings=%+v", got.OK, err, got.Inputs != nil, req.Bag.Findings())
	}
	return l, m, got.Inputs
}

// IMPLEMENTATION-PLAN §7.6, WIRE.md §6.1, §6.5-§6.7: a replay answers alike only while all it read is the same.
func TestReplaySeesEveryChange(t *testing.T) {
	file := func(name string) string { return strings.TrimPrefix(path.Join(projectDir, name), "/") }
	put := func(name, data string) func(fstest.MapFS) {
		return func(m fstest.MapFS) { m[file(name)] = &fstest.MapFile{Data: []byte(data)} }
	}
	drop := func(name string) func(fstest.MapFS) { return func(m fstest.MapFS) { delete(m, file(name)) } }
	dir, text, csv := dirExpr("data/*.json"), textExpr("t.txt"), csvExpr("r.csv")
	strs := &types.ListType{Elem: &types.ListType{Elem: types.StringType}}
	for _, c := range []struct {
		name   string
		e      *syntax.LoadExpr
		typ    types.Type
		change func(fstest.MapFS)
		same   bool
	}{
		{"dir unchanged", dir, rowType(), func(fstest.MapFS) {}, true},
		{"dir file edited", dir, rowType(), put("data/a.json", `{"label": "A"}`), false},
		{"dir file added", dir, rowType(), put("data/d.json", `{"label": "d"}`), false},
		{"dir file removed", dir, rowType(), drop("data/b.json"), false},
		{"dir unmatched file added", dir, rowType(), put("data/e.txt", "x"), false},
		{"other directory edited", dir, rowType(), put("other/x.json", "{}"), true},
		{"text edited", text, types.StringType, put("t.txt", "text\n"), false},
		{"text removed", text, types.StringType, drop("t.txt"), false},
		{"csv edited", csv, strs, put("r.csv", "a,c\n"), false},
		{"csv unchanged, dir edited", csv, strs, put("data/a.json", "{}"), true},
	} {
		l, m, in := recordedOver(t, c.e, c.typ)
		c.change(m)
		if got := l.Replay(in); got != c.same {
			t.Errorf("%s: replay answered alike %t, want %t", c.name, got, c.same)
		}
	}
}

// IMPLEMENTATION-PLAN §7.6, WIRE.md §2.2: a replay against other root places is refused.
func TestReplayRefusesOtherPlaces(t *testing.T) {
	l, _, in := recordedOver(t, dirExpr("data/*.json"), rowType())
	proj := &project.Project{Roots: []project.Root{{Name: "r", Path: "data"}}}
	layout, ok := project.NewLayout(proj, projectDir, nil, diag.NewBag(nil, ""))
	if !ok {
		t.Fatal("layout")
	}
	other := &load.Loader{FS: l.FS, Layout: layout, Set: l.Set}
	if other.Replay(in) {
		t.Error("a replay under other roots answered alike")
	}
}

// IMPLEMENTATION-PLAN §7.6, API.md S3: a replay adds each file it reads under the load's display.
func TestReplayAddsAsTheLoad(t *testing.T) {
	var added, again []string
	l, req := loaderFor(t, memoTree)
	l.Add = func(display, abs string, data []byte) (*source.File, error) {
		added = append(added, display)
		return l.Set.Add(display, abs, data)
	}
	got, err := l.Recorded(context.Background(), req, dirExpr("data/*.json"), rowType())
	if err != nil || got.Inputs == nil {
		t.Fatalf("err=%v inputs=%v", err, got.Inputs != nil)
	}
	added, again = nil, added
	if !l.Replay(got.Inputs) || !slices.Equal(added, again) || len(added) != 2 {
		t.Errorf("replay added %q, the load %q", added, again)
	}
}

// IMPLEMENTATION-PLAN §7.6, WIRE.md §6.8: failed, scratch and load.defines loads keep no inputs.
func TestRecordedRefuses(t *testing.T) {
	for _, c := range []struct {
		name    string
		e       *syntax.LoadExpr
		typ     types.Type
		scratch bool
	}{
		{"defines", definesExpr("h.h", ""), definesTableType(), false},
		{"scratch", dirExpr("data/*.json"), rowType(), true},
		{"failed", bareExpr("missing.json"), types.StringType, false},
	} {
		l, req := loaderFor(t, map[string]string{"h.h": "#define A 1\n", "data/a.json": `{"label": "a"}`})
		req.Scratch = c.scratch
		if got, err := l.Recorded(context.Background(), req, c.e, c.typ); err != nil || got.Inputs != nil {
			t.Errorf("%s: err=%v, inputs kept %t", c.name, err, got.Inputs != nil)
		}
	}
}

// WIRE.md §6.1, §6.5: a recorded load keeps every finding of a load that holds, and nothing after an error.
func TestRecordedKeepsItsFindings(t *testing.T) {
	cases, err := golden.Load("testdata/findings/*.txtar")
	if err != nil {
		t.Fatal(err)
	}
	warned := 0
	for _, c := range cases {
		set := &source.FileSet{}
		bag, again := diag.NewBag(set, "p"), diag.NewBag(set, "p")
		l, req, call := caseLoad(t, c, set, bag)
		got, err := l.Recorded(context.Background(), req, call.e, call.typ)
		if err != nil {
			t.Fatalf("%s: %v", c.Path, err)
		}
		if got.Inputs == nil {
			if got.OK && (call.e.Method == nil || call.e.Method.Name != "defines") {
				t.Errorf("%s: a load that did not fail kept no inputs", c.Path)
			}
			continue
		}
		got.Inputs.Report(again)
		s := bag.Summary()
		if !got.OK || s.Errors > 0 || render(t, set, bag) != render(t, set, again) {
			t.Errorf("%s: kept inputs with ok=%t, %d errors; replay reports\n%s\nthe load\n%s", c.Path, got.OK, s.Errors, render(t, set, again), render(t, set, bag))
		}
		warned += s.Warnings
	}
	if warned == 0 {
		t.Error("no recorded case reported a warning: the kept findings are not exercised")
	}
}

// caseCall is a findings case's load expression and expected type.
type caseCall struct {
	e   *syntax.LoadExpr
	typ types.Type
}

// caseLoad is a findings case's load: its pattern's load.dir, else its call, against its type.
func caseLoad(t *testing.T, c golden.Case, set *source.FileSet, bag *diag.Bag) (*load.Loader, load.Request, caseCall) {
	t.Helper()
	layout, ok := project.NewLayout(&project.Project{}, projectDir, nil, bag)
	if !ok {
		t.Fatalf("%s: layout", c.Path)
	}
	l := &load.Loader{FS: newMemFS(c.Archive), Layout: layout, Set: set}
	req := load.Request{Pkg: "p", Bag: bag}
	if data, ok := archiveFile(c.Archive, patternFile); ok {
		return l, req, caseCall{dirExpr(strings.TrimSuffix(string(data), "\n")), rowType()}
	}
	data, _ := archiveFile(c.Archive, callFile)
	kind, _ := archiveFile(c.Archive, typeFile)
	return l, req, caseCall{loadCall(t, strings.TrimSuffix(string(data), "\n")), callType(string(kind))}
}

// render is bag's findings as text.
func render(t *testing.T, set *source.FileSet, bag *diag.Bag) string {
	t.Helper()
	var buf bytes.Buffer
	if err := diag.Render(&buf, set, bag.Findings(), diag.RenderOptions{Golden: true}); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}
