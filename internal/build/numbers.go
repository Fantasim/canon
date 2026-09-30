package build

import (
	"bytes"
	"cmp"
	"slices"
	"sync"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// readFile is what the loads read of one file: its content, and the text each number a field
// read takes and the base type it was read as, by its span in the file; mixed when two loads
// read different contents.
type readFile struct {
	content []byte
	mixed   bool
	texts   map[source.Span]string
	types   map[source.Span]string
}

func newReadFile(content []byte) *readFile {
	return &readFile{content: content, texts: map[source.Span]string{}, types: map[source.Span]string{}}
}

// reading notes v's reading of the number token at span, when v is a numeric reading of one
// (FORMATTER.md 14.1, WIRE.md 7.2).
func (rf *readFile) reading(v value.Value, span source.Span) {
	if !isNumber(rf.content, span) {
		return
	}
	if text, ok := wire.NumberText(v, string(rf.content[span.Start:span.End])); ok {
		rf.note(span, text, v.Type().Base().String())
	}
}

// note keeps text for the number at span, read as base type typ; the token as written when two
// loads read it as different types or texts (log-2026-09-29 M4 B11-r3).
func (rf *readFile) note(span source.Span, text, typ string) {
	key := source.Span{Start: span.Start, End: span.End}
	if old, ok := rf.types[key]; ok && (old != typ || rf.texts[key] != text) {
		text, typ = string(rf.content[key.Start:key.End]), readConflict
	}
	rf.texts[key], rf.types[key] = text, typ
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

// numberIndex is what an analysis's loads read as numbers, by JSON source, gathered on demand.
type numberIndex struct {
	mu     sync.Mutex
	rooted bool
	roots  []value.Value // the settled values of the roots whose declaration holds a load
	files  map[string]*readFile
}

// NumberTexts is, by span, the canonical text of each number token of the JSON source at display a
// load read into a Canon field, with the content read; nil when none or two contents (API.md M9,
// FORMATTER.md 14.1). Computed once per file, walking only the values that file states.
func (a *Analysis) NumberTexts(display string) ([]byte, map[source.Span]string) {
	ix := &a.numbers
	ix.mu.Lock()
	defer ix.mu.Unlock()
	rf, ok := ix.files[display]
	if !ok {
		if !ix.rooted {
			ix.roots, ix.rooted = a.loadedRoots(), true
		}
		rf = a.readNumbers(display, ix.roots)
		if ix.files == nil {
			ix.files = map[string]*readFile{}
		}
		ix.files[display] = rf
	}
	if rf.mixed {
		return nil, nil
	}
	return rf.content, rf.texts
}

// loadedRoots are the settled values of the consts and lets whose declaration holds a load.
func (a *Analysis) loadedRoots() []value.Value {
	var out []value.Value
	for _, cp := range a.r.prog.Packages {
		for _, obj := range cp.Decls {
			if obj.Kind() != check.ObjConst && obj.Kind() != check.ObjLet || !holdsLoadExpr(obj.Decl()) {
				continue
			}
			if v, ok := a.settled[eval.Root{Pkg: cp.Path, Name: obj.Name()}]; ok {
				out = append(out, v)
			}
		}
	}
	return out
}

// holdsLoadExpr reports a declaration holding a load expression.
func holdsLoadExpr(n syntax.Node) bool {
	found := false
	syntax.Inspect(n, func(c syntax.Node) bool {
		if _, ok := c.(*syntax.LoadExpr); ok {
			found = true
		}
		return !found
	})
	return found
}

// readNumbers is what roots read from the file the JSON source at display is, under any display
// path (log-2026-09-29 M4 B11-r5): only that file's values are walked.
func (a *Analysis) readNumbers(display string, roots []value.Value) *readFile {
	tops := jsonTops(roots, a.r.s.set)
	same := a.sameFile(display, tops)
	var rf *readFile
	for _, t := range tops {
		if same(t.f) {
			rf = claimed(rf, t.f.Content)
			readAll(rf, t.v)
		}
	}
	if rf == nil {
		return &readFile{mixed: true}
	}
	return rf
}

// jsonTop is a value a JSON source states nearest the roots, and the file it was read from.
type jsonTop struct {
	v value.Value
	f *source.File
}

// jsonTops are the values nearest the roots a JSON source states: the scan never enters one,
// whose parts that source states too.
func jsonTops(roots []value.Value, set *source.FileSet) []jsonTop {
	var out []jsonTop
	stack := slices.Clone(roots)
	for len(stack) > 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		p := v.Prov()
		if p == nil || p.Kind != value.ProvJSON {
			stack = append(stack, parts(v)...)
			continue
		}
		if f := set.File(p.Span.File); f != nil {
			out = append(out, jsonTop{v: v, f: f})
		}
	}
	return out
}

// sameFile reports a file read from the real file display is, as fmt groups them (fileRead):
// display itself, its absolute path under another root, or a link to it, which holds the same
// bytes, so only files holding them are resolved.
func (a *Analysis) sameFile(display string, tops []jsonTop) func(*source.File) bool {
	i := slices.IndexFunc(tops, func(t jsonTop) bool { return t.f.Path == display })
	if i < 0 {
		return func(*source.File) bool { return false }
	}
	target := tops[i].f
	real := a.r.real(target.Abs)
	seen := map[*source.File]bool{}
	return func(f *source.File) bool {
		switch {
		case f.Path == display || f.Abs == target.Abs:
			return true
		case len(f.Content) != len(target.Content):
			return false
		}
		in, ok := seen[f]
		if !ok {
			in = bytes.Equal(f.Content, target.Content) && a.r.real(f.Abs) == real
			seen[f] = in
		}
		return in
	}
}

// claimed is rf for content: made from it, or marked mixed when it read other bytes.
func claimed(rf *readFile, content []byte) *readFile {
	switch {
	case rf == nil:
		return newReadFile(content)
	case string(rf.content) != string(content):
		rf.mixed = true
	}
	return rf
}

// readAll notes every number reading of v and its parts, all stated by v's JSON source.
func readAll(rf *readFile, v value.Value) {
	stack := []value.Value{v}
	for len(stack) > 0 && !rf.mixed {
		x := stack[len(stack)-1]
		stack = append(stack[:len(stack)-1], parts(x)...)
		if p := x.Prov(); p != nil && p.Kind == value.ProvJSON {
			rf.reading(x, p.Span)
		}
	}
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
