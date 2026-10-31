package load_test

import (
	"context"
	"io/fs"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"golang.org/x/tools/txtar"
)

// dirRun is one `load.dir` run's keyed ids and every finding's message, in the bag's order.
type dirRun struct {
	ids, msgs []string
}

// runDirMessages runs `load.dir(pattern)` as `[Item] keyed by id` over fsys, placed by layout.
func runDirMessages(t *testing.T, fsys project.FS, layout *project.Layout, pattern string) dirRun {
	t.Helper()
	set := &source.FileSet{}
	bag := diag.NewBag(set, "p")
	l := &load.Loader{FS: fsys, Layout: layout, Set: set}
	item := itemType()
	keyed := &types.ListType{Elem: item, KeyedBy: item.Fields[0]}
	v, _, err := l.Load(context.Background(), load.Request{Pkg: "p", Bag: bag}, dirExpr(pattern), keyed)
	if err != nil {
		t.Fatalf("err=%v findings %+v", err, bag.Findings())
	}
	var run dirRun
	if list, ok := v.(*value.List); ok {
		for _, e := range list.Elems {
			run.ids = append(run.ids, e.(*value.Record).Ident.Key.S)
		}
	}
	for _, f := range bag.Findings() {
		run.msgs = append(run.msgs, f.Message)
	}
	return run
}

// junctionArchive is a project with data/a.json, the directory /p/real holding b.json, the
// directory /outside/o holding x.json, and the junctions lines given.
func junctionArchive(junctions string) *txtar.Archive {
	return &txtar.Archive{Files: []txtar.File{
		{Name: "data/a.json", Data: []byte(`{"id": "a", "name": "A"}`)},
		{Name: "real/b.json", Data: []byte(`{"id": "b", "name": "B"}`)},
		{Name: "/outside/o/x.json", Data: []byte(`{"id": "x", "name": "X"}`)},
		{Name: "junctions", Data: []byte(junctions)},
	}}
}

// junctionCase is one tree's junctions, the load.dir pattern run over it, and what it gives.
type junctionCase struct {
	name, junctions, pattern string
	ids, msgs                []string
}

// allJSON is the walk every junction case runs, the entry it names data/j.
const (
	allJSON   = "data/**/*.json"
	junctionJ = "data/j"
)

// messageOf is the message b reports, the registry's own text.
func messageOf(b *diag.Builder) string {
	bag := diag.NewBag(&source.FileSet{}, "p")
	b.Report(bag)
	return bag.Findings()[0].Message
}

// WIRE.md §6.5, DECISIONS 419: a junction is followed inside the bounds, else W7115; a placeholder is no link.
func TestLoadDirJunction(t *testing.T) {
	var at source.Span
	cases := []junctionCase{
		{"inside followed", "data/j /p/real\n", allJSON, []string{"a", "b"}, nil},
		{"outside skipped", "data/j /outside/o\n", allJSON, []string{"a"}, []string{messageOf(diag.W7115.AtOutsideRoots(at, junctionJ))}},
		{"dangling skipped", "data/j /p/ghost\n", allJSON, []string{"a"}, []string{messageOf(diag.W7115.AtDangling(at, junctionJ))}},
		{"file junction outside", "data/j.json /outside/o/x.json\n", "data/*.json", []string{"a"},
			[]string{messageOf(diag.W7115.AtOutsideRoots(at, "data/j.json"))}},
		{"placeholder", "data/ph.json\ndata/ph\n", allJSON, []string{"a"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			run := runDirMessages(t, newMemFS(junctionArchive(c.junctions)), layoutAt(t, projectDir), c.pattern)
			if !slices.Equal(run.ids, c.ids) {
				t.Errorf("entries = %v, want %v", run.ids, c.ids)
			}
			if !slices.Equal(run.msgs, c.msgs) {
				t.Errorf("findings = %q, want %q", run.msgs, c.msgs)
			}
		})
	}
}

// statlessFS fails every Stat of an irregular entry, as a cloud placeholder's may fail.
type statlessFS struct{ memFS }

func (f statlessFS) Stat(name string) (fs.FileInfo, error) {
	info, err := f.memFS.Stat(name)
	if err == nil && info.Mode()&fs.ModeIrregular != 0 {
		return nil, fs.ErrPermission
	}
	return info, err
}

// ADR-0023 §10.4 Decision 3: an unlinked entry (project.Unlinked) takes the non-link branch, never stat'd.
func TestLoadDirPlaceholderUnstated(t *testing.T) {
	// A pin: it fails when classify takes an unlinked entry's link branch, stat'ing it (W7115 statFailed).
	fsys := statlessFS{newMemFS(junctionArchive("data/ph.json\n"))}
	run := runDirMessages(t, fsys, layoutAt(t, projectDir), allJSON)
	if !slices.Equal(run.ids, []string{"a"}) || len(run.msgs) != 0 {
		t.Errorf("entries = %v, findings = %q; want [a] and none", run.ids, run.msgs)
	}
}

// WIRE.md §6.5, DECISIONS 419: a pin, a base or literal path through a junction (isMemLink) takes the bounds.
func TestLoadDirJunctionBase(t *testing.T) {
	// A pin: it fails when resolveBase or literalMatch stops following a junction (E7004 notDir, ReadMissing).
	var at source.Span
	const base = "data/j/*.json"
	cases := []junctionCase{
		{"base inside", "data/j /p/real\n", base, []string{"b"}, nil},
		{"literal inside", "data/j /p/real\n", "data/j/b.json", []string{"b"}, nil},
		{"base outside", "data/j /outside/o\n", base, nil,
			[]string{messageOf(diag.W7107.At(at, base)), messageOf(diag.W7115.AtOutsideRoots(at, "data/j/"))}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			run := runDirMessages(t, newMemFS(junctionArchive(c.junctions)), layoutAt(t, projectDir), c.pattern)
			if !slices.Equal(run.ids, c.ids) {
				t.Errorf("entries = %v, want %v", run.ids, c.ids)
			}
			if !slices.Equal(run.msgs, c.msgs) {
				t.Errorf("findings = %q, want %q", run.msgs, c.msgs)
			}
		})
	}
}
