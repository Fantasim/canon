package build

import (
	"cmp"
	"context"
	"path"
	"slices"
	"sync"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
	"github.com/fantasim/canonlang/internal/wire"
)

// JSONSource is a JSON file the given sources load: Content is what loads decoded (CRLF normalized),
// Numbers its numbers Canon fields read that are not canonical; both nil when no load decoded it
// or two read different contents.
type JSONSource struct {
	Display string
	Abs     string // links resolved
	Content []byte
	Numbers []Number // in Content order
}

// Number is the canonical text of the number token at [Start, End) of a JSON source's Content.
type Number struct {
	Start, End source.Pos
	Text       string
}

// JSONSources is the JSON files the loads of sources (.canon display paths) read, in display order.
// It checks their packages and forces only the consts and lets holding a load, reporting nothing;
// only the readings of those forced loads make a number canonical, any other stays as written.
func (p *Project) JSONSources(ctx context.Context, sources []string) ([]JSONSource, error) {
	s, err := p.load(ctx)
	if err != nil {
		return nil, err
	}
	want := make(map[string]bool, len(sources))
	for _, name := range sources {
		want[name] = true
	}
	selected := slices.DeleteFunc(slices.Clone(s.units), func(u *project.Unit) bool {
		return !slices.ContainsFunc(u.Files, func(f *syntax.File) bool { return want[f.Src.Path] })
	})
	if len(selected) == 0 {
		return nil, nil
	}
	loaded := withStudio(s.units, imported(s.units, selected), s.proj.Studio.Path)
	r := &run{p: p, s: s, selected: selected, loaded: loaded, bags: s.bagsOf(loaded)}
	if err := interrupted(ctx, r.check(ctx)); err != nil {
		return nil, err
	}
	rec, err := r.recordLoads()
	if err != nil {
		return nil, err
	}
	r.forceLoads(ctx, want)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return r.jsonSources(want, rec.numbers(r)), nil
}

// loadRecorder is the run's host keeping every value a load decoded whole.
type loadRecorder struct {
	*evalHost
	mu     sync.Mutex
	values []value.Value
}

func (h *loadRecorder) Load(ctx context.Context, e *syntax.LoadExpr, expected types.Type) (value.Value, bool) {
	v, ok := h.evalHost.Load(ctx, e, expected)
	if ok {
		h.mu.Lock()
		h.values = append(h.values, v)
		h.mu.Unlock()
	}
	return v, ok
}

// recordLoads makes the run's host and evaluator, the evaluator loading through a recorder.
func (r *run) recordLoads() (*loadRecorder, error) {
	r.newHost(r.bags)
	rec := &loadRecorder{evalHost: r.host}
	r.ev = eval.New(r.prog, rec, r.bags, r.opt)
	if !r.ev.UseFolder(r.fold) { // DECISIONS 104
		return nil, internal(errTwoCounts)
	}
	r.host.ev = r.ev
	r.host.verifier = verify.NewShared(r.vix, r.ev, r.bags, r.assets)
	return rec, nil
}

// forceLoads forces each const and let of want's files whose declaration holds a load.
func (r *run) forceLoads(ctx context.Context, want map[string]bool) {
	sites := map[*syntax.File][]loadAt{}
	for _, cp := range r.prog.Packages {
		if !r.selects(cp.Path) {
			continue
		}
		for _, obj := range cp.Decls {
			if holdsLoad(obj, want, sites) {
				r.ev.Force(ctx, eval.Root{Pkg: cp.Path, Name: obj.Name()})
			}
		}
	}
}

// holdsLoad reports a const or let of want's files whose declaration holds a load; sites keeps
// each file's loads, walked once.
func holdsLoad(obj check.Object, want map[string]bool, sites map[*syntax.File][]loadAt) bool {
	f := obj.File()
	if (obj.Kind() != check.ObjConst && obj.Kind() != check.ObjLet) || f == nil || !want[f.Src.Path] {
		return false
	}
	if _, ok := sites[f]; !ok {
		sites[f] = walkLoads(f)
	}
	decl := f.Span(obj.Decl())
	return slices.ContainsFunc(sites[f], func(at loadAt) bool { return decl.Start <= at.span.Start && at.span.End <= decl.End })
}

// jsonSources is the JSON files of want's load expressions, by load's own resolver, each with
// what the loads read of it.
func (r *run) jsonSources(want map[string]bool, read map[string]*readFile) []JSONSource {
	var out []JSONSource
	for _, u := range r.selected {
		for _, f := range u.Files {
			if want[f.Src.Path] {
				out = append(out, r.jsonFiles(f)...)
			}
		}
	}
	slices.SortFunc(out, func(a, b JSONSource) int { return cmp.Or(cmp.Compare(a.Display, b.Display), cmp.Compare(a.Abs, b.Abs)) })
	out = slices.CompactFunc(out, func(a, b JSONSource) bool { return a.Display == b.Display && a.Abs == b.Abs })
	for i := range out {
		if rf := read[r.real(out[i].Abs)]; rf != nil && !rf.mixed {
			out[i].Content, out[i].Numbers = rf.content, rf.changed()
		}
	}
	return out
}

// jsonFiles is the JSON files f's load expressions read, whatever they decode to (WIRE.md §6.1, §6.5).
func (r *run) jsonFiles(f *syntax.File) []JSONSource {
	scratch := diag.NewBag(nil, "")
	var out []JSONSource
	for _, at := range walkLoads(f) {
		for _, lf := range load.JSONFiles(r.p.fs, r.s.layout, path.Dir(f.Src.Path), at.e, scratch) {
			out = append(out, JSONSource{Display: lf.Display, Abs: lf.Abs})
		}
	}
	return out
}

// real is abs with its links resolved, abs itself when they leave the roots.
func (r *run) real(abs string) string {
	if real, ok := load.Inside(r.p.fs, r.s.layout, abs); ok {
		return real
	}
	return abs
}

// readFile is what the loads read of one file: its content, and the text each number a field
// read takes, by its span in the file; mixed when two loads read different contents.
type readFile struct {
	content []byte
	mixed   bool
	texts   map[source.Span]string
}

// note keeps text for the number at span, its token when two fields want different texts.
func (rf *readFile) note(span source.Span, text string) {
	key := source.Span{Start: span.Start, End: span.End}
	if old, ok := rf.texts[key]; ok && old != text {
		text = string(rf.content[key.Start:key.End])
	}
	rf.texts[key] = text
}

// changed is the numbers whose text differs from their token, in content order.
func (rf *readFile) changed() []Number {
	var out []Number
	//canon:unordered sorted below
	for key, text := range rf.texts {
		if text != string(rf.content[key.Start:key.End]) {
			out = append(out, Number{Start: key.Start, End: key.End, Text: text})
		}
	}
	slices.SortFunc(out, func(a, b Number) int { return cmp.Compare(a.Start, b.Start) })
	return out
}

// numbers is, by resolved path, what the recorded loads read of each file.
func (h *loadRecorder) numbers(r *run) map[string]*readFile {
	byID := map[source.FileID]*readFile{}
	byAbs := map[string]*readFile{}
	stack := slices.Clone(h.values)
	for len(stack) > 0 {
		v := stack[len(stack)-1]
		stack = append(stack[:len(stack)-1], parts(v)...)
		p := v.Prov()
		if p == nil || p.Kind != value.ProvJSON {
			continue
		}
		rf, ok := byID[p.Span.File]
		if !ok {
			rf = fileRead(r, p.Span.File, byAbs)
			byID[p.Span.File] = rf
		}
		if rf == nil || !isNumber(rf.content, p.Span) {
			continue
		}
		if text, ok := wire.NumberText(v, string(rf.content[p.Span.Start:p.Span.End])); ok {
			rf.note(p.Span, text)
		}
	}
	return byAbs
}

// fileRead is the readFile of the set's file id, shared by every load of its resolved path.
func fileRead(r *run, id source.FileID, byAbs map[string]*readFile) *readFile {
	f := r.s.set.File(id)
	if f == nil {
		return nil
	}
	abs := r.real(f.Abs)
	rf := byAbs[abs]
	switch {
	case rf == nil:
		rf = &readFile{content: f.Content, texts: map[source.Span]string{}}
		byAbs[abs] = rf
	case string(rf.content) != string(f.Content):
		rf.mixed = true
	}
	return rf
}

// parts is the values v holds; a ref's target and a record's identity are not parts.
func parts(v value.Value) []value.Value {
	var out []value.Value
	switch x := v.(type) {
	case *value.Record:
		out = x.Fields
	case *value.List:
		out = x.Elems
	case *value.Map:
		out = x.Vals
	case *value.Table:
		for _, e := range x.Entries {
			out = append(out, e)
		}
	case *value.Pair:
		out = []value.Value{x.A, x.B}
	}
	return slices.DeleteFunc(slices.Clone(out), func(p value.Value) bool { return p == nil })
}

// isNumber reports a JSON number token at span of content.
func isNumber(content []byte, span source.Span) bool {
	if span.Start < 0 || span.End <= span.Start || int(span.End) > len(content) {
		return false
	}
	c := content[span.Start]
	return c == '-' || '0' <= c && c <= '9'
}
