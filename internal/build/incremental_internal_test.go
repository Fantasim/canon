package build

import (
	"context"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	viewgen "github.com/fantasim/canonlang/internal/gen/view"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/value"
)

// editFS is a file system whose files an edit replaces in memory, by absolute name.
type editFS struct {
	base project.FS
	mu   sync.Mutex
	over map[string][]byte
}

func newEditFS(base project.FS) *editFS {
	return &editFS{base: base, over: map[string][]byte{}}
}

func (f *editFS) ReadFile(name string) ([]byte, error) {
	f.mu.Lock()
	data, ok := f.over[name]
	f.mu.Unlock()
	if ok {
		return slices.Clone(data), nil
	}
	return f.base.ReadFile(name)
}

func (f *editFS) Stat(name string) (fs.FileInfo, error)      { return f.base.Stat(name) }
func (f *editFS) ReadDir(name string) ([]fs.DirEntry, error) { return f.base.ReadDir(name) }

// EvalSymlinks resolves links as the file system under the edits does.
func (f *editFS) EvalSymlinks(name string) (string, error) { return project.EvalSymlinks(f.base, name) }

// set replaces abs's content.
func (f *editFS) set(abs string, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.over[abs] = slices.Clone(data)
}

// analyzer opens one project twice over one file system: warm through a cache kept across
// calls, cold without one.
type analyzer struct {
	fs    *editFS
	dir   string
	opt   Options
	cache *Cache
	sel   []string // the packages pair selects; nil selects every one
}

// pair analyzes z.sel warm and cold; the warm analysis's epoch tells its lineage.
func (z *analyzer) pair(t *testing.T) (warm, cold *Analysis) {
	t.Helper()
	ctx := context.Background()
	p, err := Open(z.fs, z.dir, z.opt)
	if err != nil {
		t.Fatal(err)
	}
	if warm, err = p.WithCache(z.cache).Analyze(ctx, z.sel); err != nil {
		t.Fatal(err)
	}
	if cold, err = p.Analyze(ctx, z.sel); err != nil {
		t.Fatal(err)
	}
	return warm, cold
}

// same fails when the warm analysis differs from the cold one in anything observable.
func same(t *testing.T, name string, warm, cold *Analysis) {
	t.Helper()
	w, c := dumpAnalysis(t, warm), dumpAnalysis(t, cold)
	if w == c {
		return
	}
	wl, cl := strings.Split(w, "\n"), strings.Split(c, "\n")
	for i := range min(len(wl), len(cl)) {
		if wl[i] != cl[i] {
			t.Fatalf("%s: warm differs from cold at line %d:\nwarm: %s\ncold: %s", name, i+1, wl[i], cl[i])
		}
	}
	t.Fatalf("%s: warm has %d lines, cold %d", name, len(wl), len(cl))
}

// dumpAnalysis is everything observable of an analysis (log-2026-09-29 M4 "U11 review"): its
// findings as text and JSON, revision, each selected package's values in depth, causes, read
// sets and view model bytes. Spans are resolved, since file ids differ between file sets.
func dumpAnalysis(t *testing.T, a *Analysis) string {
	t.Helper()
	res := a.Result()
	var sb strings.Builder
	for _, format := range []diag.Format{diag.FormatText, diag.FormatJSON} {
		opt := diag.RenderOptions{Format: format, Summary: res.Summary, Golden: true}
		if err := diag.Render(&sb, res.Files, res.List, opt); err != nil {
			t.Fatal(err)
		}
	}
	fmt.Fprintf(&sb, "revision %s\n", res.Revision)
	sb.Write(a.Manifest())                             // WIRE.md §10: a warm run's inputs are a cold run's
	fmt.Fprintf(&sb, "charged %v\n", a.r.ev.Charged()) // EVALUATION.md §12.2: every root's steps, in first-charged order
	locks, err := a.LockUpdates()
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range locks {
		fmt.Fprintf(&sb, "lock %s %s %d %q\n%s", l.Package, l.Path, l.Status, l.Lines, l.Content)
	}
	d := &deepDump{t: t, a: a, ids: map[value.Value]int{}}
	for _, cp := range a.Program().Packages {
		if !a.r.selects(cp.Path) {
			continue
		}
		d.pkg(&sb, cp)
		for _, r := range a.Reads(cp.Path) {
			fmt.Fprintf(&sb, "read %s dir=%t sources=%t link=%t\n", r.Display, r.Dir, r.Sources, r.Link)
		}
		dumpView(t, &sb, a, cp)
	}
	return sb.String()
}

// dumpView writes cp's view model bytes when it emits one.
func dumpView(t *testing.T, sb *strings.Builder, a *Analysis, cp *check.Package) {
	t.Helper()
	if !hasEmitView(cp.Files) {
		return
	}
	m, err := a.ViewModel(context.Background(), cp.Path)
	if err != nil {
		fmt.Fprintf(sb, "view %s: %v\n", cp.Path, err)
		return
	}
	data, err := viewgen.Write(m)
	if err != nil {
		t.Fatal(err)
	}
	sb.Write(data)
}

// deepDump numbers each node of the values it dumps at its first reach.
type deepDump struct {
	t     *testing.T
	a     *Analysis
	ids   map[value.Value]int
	queue []value.Value
}

// pkg dumps each const and let of cp: its value in depth, else its cause.
func (d *deepDump) pkg(sb *strings.Builder, cp *check.Package) {
	for _, obj := range cp.Decls {
		if obj.Kind() != check.ObjConst && obj.Kind() != check.ObjLet {
			continue
		}
		root := eval.Root{Pkg: cp.Path, Name: obj.Name()}
		v, ok := d.a.Force(root)
		fmt.Fprintf(sb, "%s.%s ok=%t #%d produced=#%d history=%v\n", root.Pkg, root.Name, ok, d.id(v), d.id(d.produced(v)), d.history(v))
		for len(d.queue) > 0 {
			n := d.queue[0]
			d.queue = d.queue[1:]
			sb.WriteString(d.line(n))
		}
		causes, err := d.a.Cause(context.Background(), root)
		if err != nil {
			d.t.Fatal(err)
		}
		for _, f := range causes {
			fmt.Fprintf(sb, "  cause %s %s %s\n", f.Code, d.loc(f.Span.File, f.Span.Start), f.Message)
		}
	}
}

// history is the numbers of v's history, newest first (CLI.md §3.7).
func (d *deepDump) history(v value.Value) []int {
	if v == nil {
		return nil
	}
	var out []int
	for _, h := range d.a.History(v) {
		out = append(out, d.id(h))
	}
	return out
}

func (d *deepDump) produced(v value.Value) value.Value {
	if v == nil {
		return nil
	}
	return d.a.Produced(v)
}

// id is n's number, given and queued at its first reach; -1 for none.
func (d *deepDump) id(n value.Value) int {
	if n == nil {
		return -1
	}
	if id, ok := d.ids[n]; ok {
		return id
	}
	d.ids[n] = len(d.ids)
	d.queue = append(d.queue, n)
	return d.ids[n]
}

// line is one node: its kind, text, provenance, invalid mark, identity, then what it holds.
func (d *deepDump) line(n value.Value) string {
	s := fmt.Sprintf("#%d %T %q %s invalid=%t", d.ids[n], n, n.CanonText(), d.prov(n.Prov()), d.a.r.ev.Invalid(n))
	var parts []value.Value
	switch x := n.(type) {
	case *value.Record:
		s += fmt.Sprintf(" set=%v%s", x.Set, d.ident(x.Ident))
		parts = x.Fields
	case *value.List:
		parts = x.Elems
	case *value.Map:
		parts = append(slices.Clone(x.Keys), x.Vals...)
	case *value.Table:
		for _, en := range x.Entries {
			parts = append(parts, en)
		}
	case *value.Pair:
		parts = []value.Value{x.A, x.B}
	case *value.Ref:
		s += fmt.Sprintf(" owner=#%d", d.id(record(x.Owner)))
	}
	ids := make([]int, len(parts))
	for i, p := range parts {
		ids[i] = d.id(p)
	}
	return fmt.Sprintf("%s parts=%v\n", s, ids)
}

func record(r *value.Record) value.Value {
	if r == nil {
		return nil
	}
	return r
}

// ident is an identity: its collection, key, retirement and owner.
func (d *deepDump) ident(id *value.Identity) string {
	if id == nil {
		return ""
	}
	coll := "none"
	if c := id.Coll; c != nil {
		coll = fmt.Sprintf("%d:%s.%s%v", c.Kind, c.Pkg, c.Name, c.FieldPath)
	}
	return fmt.Sprintf(" ident=%s:%s retired=%t owner=#%d", coll, id.Key.Text(), id.Retired, d.id(record(id.Owner)))
}

// prov is a provenance and those it goes through, every span resolved (EVALUATION.md §13).
func (d *deepDump) prov(p *value.Prov) string {
	var sb strings.Builder
	for ; p != nil; p = p.Via {
		fmt.Fprintf(&sb, "<%d %s-%d %q %q", p.Kind, d.loc(p.Span.File, p.Span.Start), p.Span.End, p.Pointer, p.Layer)
		for _, fr := range p.Stack {
			fmt.Fprintf(&sb, " %s@%s", fr.Fn, d.loc(fr.Span.File, fr.Span.Start))
		}
		fmt.Fprintf(&sb, "+%d>", p.MoreFrames)
	}
	return sb.String()
}

// loc is a position as path:line:col in the analysis's files.
func (d *deepDump) loc(file source.FileID, at source.Pos) string {
	files := d.a.Files()
	line, col := files.Position(file, at)
	return fmt.Sprintf("%s:%d:%d", files.Path(file), line, col)
}
