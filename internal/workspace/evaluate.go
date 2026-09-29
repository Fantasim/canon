package workspace

import (
	"context"
	"errors"
	"maps"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/live"
)

// Eval is one Evaluate without a draft (API.md 11): the snapshot's analysis of every package
// (calls sharing a key share one), the edit snapshot At was resolved in, and the language of
// its texts ("" for the source language).
type Eval struct {
	Analysis *build.Analysis
	Edit     *edit.Snapshot
	At       edit.Resolved
	Lang     string
}

// Evaluation is the live view state of a value (V5-V12), its package's findings at the value or
// below, which carry their reads (V10a), and the summary of the packages an edit there affects.
type Evaluation struct {
	State    *live.Result
	Package  string
	Findings []diag.Finding
	Files    diag.Files // what locates Findings: the analysis that found them, whoever joined
	Summary  diag.Summary
}

// Evaluate is e's live view state on s, shared by identical concurrent calls (S8), writing
// nothing (V13); the view evaluator's compiler failure, Analysis.ViewErr, is returned (X2).
// liveInput alone builds live's input; its method and bound-argument seams stay unset for now.
func Evaluate(ctx context.Context, s *Snapshot, e Eval) (*Evaluation, error) {
	// A draft must join this key when drafts land (API.md V13).
	key := Key(opEvaluate, nil, e.At.Canonical, e.Lang)
	return Share(ctx, s, key, func(ctx context.Context) (*Evaluation, error) { return evaluate(ctx, s, e) })
}

// EvalStale is a *StaleError when, since base, a file read by a package e touches changed (the
// path's package and its importers, API.md S5, E17), or any source or project.canon did: each
// can change the import graph (log-2026-09-29 M4 U5a-r2).
func EvalStale(s *Snapshot, base string, e Eval) error {
	at, err := edit.Parse(e.At.Canonical)
	if err != nil {
		return err
	}
	var reads []build.Read
	for _, pkg := range affected(e.Analysis.Program(), at.Package) {
		reads = append(reads, e.Analysis.Reads(pkg)...)
	}
	read := s.Stale(base, reads)
	sources := s.sourcesStale(base)
	var a, b *StaleError
	switch {
	case read != nil && !errors.As(read, &a):
		return read
	case sources != nil && !errors.As(sources, &b):
		return sources
	case a == nil:
		return sources
	case b == nil:
		return read
	}
	files := slices.Concat(a.Files, b.Files)
	slices.Sort(files)
	return &StaleError{Files: slices.Compact(files)}
}

// sourcesStale is a *StaleError naming project.canon and each source that changed since base,
// and each package directory that gained or lost one; nil for base "" (API.md S6).
func (s *Snapshot) sourcesStale(base string) error {
	if base == "" {
		return nil
	}
	names, err := project.Scan(s.b.FS(), s.b.Dir())
	if err != nil {
		return err
	}
	s.p.mu.Lock()
	i, ok := s.p.hist.find(base)
	var was map[name]sum
	if ok {
		was = s.p.hist.whole(i)
	}
	s.p.mu.Unlock()
	if !ok {
		return &StaleError{}
	}
	files := s.changedFiles(was, append([]string{project.FileName}, names...))
	files = append(files, s.changedDirs(was, names)...)
	if len(files) == 0 {
		return nil
	}
	slices.Sort(files)
	return &StaleError{Files: slices.Compact(files)}
}

// changedFiles is each of the project files names whose content differs from was's, or that
// was never held (a source added since).
func (s *Snapshot) changedFiles(was map[name]sum, names []string) []string {
	var out []string
	for _, rel := range names {
		n := name{kind: kindFile, abs: path.Join(s.b.Dir(), rel)}
		if old, held := was[n]; !held || old != s.fs.get(n, s.fs.readFile).sum {
			out = append(out, rel)
		}
	}
	return out
}

// changedDirs names, for each directory holding sources now or in was, a change of its sources:
// the sources was never held, else the directory (a source removed, a package deleted).
func (s *Snapshot) changedDirs(was map[name]sum, names []string) []string {
	dirs := map[string]bool{}
	for _, rel := range names {
		dirs[path.Join(s.b.Dir(), path.Dir(rel))] = true
	}
	empty := listSum(nil, nil, nil)
	//canon:unordered each directory is added to a set
	for n, v := range was {
		if n.kind == kindSources && v != empty {
			dirs[n.abs] = true
		}
	}
	var out []string
	for _, abs := range slices.Sorted(maps.Keys(dirs)) {
		e := s.fs.get(name{kind: kindDir, abs: abs}, s.fs.readDir)
		if old, held := was[name{kind: kindSources, abs: abs}]; held && old == e.src {
			continue
		}
		added := slices.DeleteFunc(s.fs.sources(abs, e.list), func(f string) bool {
			_, held := was[name{kind: kindFile, abs: f}]
			return held
		})
		if len(added) == 0 {
			added = []string{abs}
		}
		for _, f := range added {
			out = append(out, s.b.Display(f))
		}
	}
	return out
}

func evaluate(ctx context.Context, s *Snapshot, e Eval) (*Evaluation, error) {
	at, err := edit.Parse(e.At.Canonical)
	if err != nil {
		return nil, err
	}
	in := liveInput(e.Analysis)
	target, err := targetOf(e, at)
	if err != nil {
		return nil, err
	}
	res, err := live.Evaluate(ctx, in, target)
	if err != nil {
		return nil, err
	}
	if err := e.Analysis.ViewErr(); err != nil {
		return nil, err
	}
	out := &Evaluation{State: res, Package: at.Package, Files: e.Analysis.Files()}
	if bag := e.Analysis.Bag(at.Package); bag != nil {
		out.Findings = below(bag.Findings(), edit.Path{Root: at.Root, Segs: at.Segs}.String())
	}
	for _, pkg := range affected(e.Analysis.Program(), at.Package) {
		if bag := e.Analysis.Bag(pkg); bag != nil {
			out.Summary = out.Summary.Merge(bag.Summary())
		}
	}
	return out, nil
}

// liveInput is what live reads of a: its program, settled values, layout, view evaluator and
// methods, bound arguments, and the studio, languages and catalogues Analyze holds.
func liveInput(a *build.Analysis) live.Input {
	in := a.LiveInputs()
	var force encode.Force = func(pkg, name string) (value.Value, bool) { return a.Force(eval.Root{Pkg: pkg, Name: name}) }
	return live.Input{
		Program: a.Program(), Studio: in.Studio, I18N: in.I18N, Force: force, Layout: a.Layout(),
		Eval: a.ViewEvaluator(), Methods: a.ViewMethods(), Bound: a.ViewBound(), Languages: in.Languages,
	}
}

// targetOf is the value e resolved and what names it without a view (V7): a top-level value's
// name, a field's name, an entry's or keyed element's key, a map key, a plain element's `#<n>`;
// with its magic names (VIEWMODEL.md 3.4) and, for a field, the field and its record.
func targetOf(e Eval, at edit.Path) (live.Target, error) {
	out := live.Target{Value: e.At.Target, Name: at.Root, Lang: e.Lang}
	n := len(at.Segs)
	if n == 0 {
		return out, nil
	}
	parent, err := edit.Resolve(e.Edit, edit.Path{Package: at.Package, Root: at.Root, Segs: at.Segs[:n-1]})
	if err != nil {
		return out, err
	}
	last := at.Segs[n-1]
	switch c := parent.Target.(type) {
	case *value.Record:
		out.Name, out.Decl = last.Name, c.T
		out.Field = fieldNamed(c.T, last.Name)
	case *value.Table:
		out.Name = keyText(e.At.Target, out.Name)
	case *value.List:
		if i := elemIndex(e, at, c.Elems, plainIndex(c, last)); i >= 0 {
			out.Magic.Index = &value.Int{V: int64(i + 1), T: types.IntType}
			out.Name = keyText(e.At.Target, positionTag+strconv.Itoa(i+1))
		}
	case *value.Map:
		if i := elemIndex(e, at, c.Vals, -1); i >= 0 {
			out.Magic.Key, out.Name = c.Keys[i], c.Keys[i].CanonText()
		}
	}
	return out, nil
}

// fieldNamed is t's field name, nil for a pseudo-field (API.md P3).
func fieldNamed(t types.Type, name string) *types.Field {
	for _, f := range encode.FieldsOf(t) {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// keyText is the canonical text of v's key when v is an entry or a keyed element, else def.
func keyText(v value.Value, def string) string {
	if r, ok := v.(*value.Record); ok && r.Ident != nil {
		return r.Ident.Key.Text()
	}
	return def
}

// plainIndex is the index a plain list's canonical segment names (API.md P8), -1 in a keyed list.
func plainIndex(l *value.List, last edit.Seg) int {
	if lt, ok := l.T.Base().(*types.ListType); !ok || lt.KeyedBy != nil || last.Kind != edit.SegKey {
		return -1
	}
	return int(last.Key.Int)
}

// elemIndex is the position of e's value among elems, its container's: index when it is one;
// else the one element that is that value, or, when several are (a shared value), the one whose
// position resolves to e's path; -1 for none.
func elemIndex(e Eval, at edit.Path, elems []value.Value, index int) int {
	if index >= 0 {
		return index
	}
	var same []int
	for i, v := range elems {
		if v == e.At.Target {
			same = append(same, i)
		}
	}
	if len(same) == 1 {
		return same[0]
	}
	segs := slices.Clone(at.Segs)
	for _, i := range same {
		segs[len(segs)-1] = edit.Seg{Kind: edit.SegPos, Pos: i}
		r, err := edit.Resolve(e.Edit, edit.Path{Package: at.Package, Root: at.Root, Segs: segs})
		if err == nil && r.Canonical == e.At.Canonical {
			return i
		}
	}
	return -1
}

// below is the findings whose path is rel or a path inside it (API.md §11 Findings, F1).
func below(list []diag.Finding, rel string) []diag.Finding {
	var out []diag.Finding
	for _, f := range list {
		rest, ok := strings.CutPrefix(f.Path, rel)
		if ok && (rest == "" || strings.HasPrefix(rest, fieldMark) || strings.HasPrefix(rest, keyMark)) {
			out = append(out, f)
		}
	}
	return out
}

// affected is pkg and every package importing it, directly or not (API.md E17), in name order.
func affected(prog *check.Program, pkg string) []string {
	in := map[string]bool{pkg: true}
	for grown := true; grown; {
		grown = false
		for _, p := range prog.Packages {
			if !in[p.Path] && slices.ContainsFunc(p.Imports, func(i *check.Package) bool { return in[i.Path] }) {
				in[p.Path], grown = true, true
			}
		}
	}
	out := make([]string, 0, len(in))
	for _, p := range prog.Packages {
		if in[p.Path] {
			out = append(out, p.Path)
		}
	}
	slices.Sort(out)
	return out
}
