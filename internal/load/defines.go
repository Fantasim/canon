package load

import (
	"bytes"
	"cmp"
	"context"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// headerFile is one #define header's classification, cached and shared by every call that names it (WIRE.md §6.8).
type headerFile struct {
	defs     []headerDefine
	skipped  []skippedDefine
	ok       bool
	path     string
	req      Request
	prefixes map[string]bool
}

// defines runs `load.defines(path[, prefix:])` against a C header (WIRE.md §6.8, LOD-01).
func (l *Loader) defines(_ context.Context, req Request, e *syntax.LoadExpr, t types.Type) (value.Value, bool, error) {
	c, ok := parseCall(e)
	if !ok {
		return nil, false, notLiteral(formDefines)
	}
	if !checkOptions(formDefines, c, types.FormatUnknown, req) {
		return nil, false, nil
	}
	p, ok := l.resolveFile(c.path, req)
	if !ok {
		return nil, false, nil
	}
	prefix := ""
	if c.prefix != nil {
		prefix = *c.prefix
	}
	if req.Scratch { // read for this call alone: its findings go to a bag nothing reads
		hf, ok := l.readHeader(p, req)
		if !ok {
			return nil, false, nil
		}
		return definesTable(hf.defs, prefix, t), hf.ok, nil
	}
	hf, ok := l.headerFileAt(p, req)
	if !ok {
		return nil, false, nil
	}
	l.mu.Lock()
	hf.prefixes[prefix] = true
	l.mu.Unlock()
	return definesTable(hf.defs, prefix, t), hf.ok, nil
}

// headerFileAt is p's classification for req's package, read once per package, its findings in
// that package's bag: E7102 once per package, and never depending on which packages a run loads.
func (l *Loader) headerFileAt(p project.Path, req Request) (*headerFile, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	key := headerKey{pkg: req.Pkg, abs: p.Abs}
	if hf, ok := l.headers[key]; ok {
		if l.Reused != nil {
			l.Reused(p.Abs)
		}
		return hf, true
	}
	hf, ok := l.readHeader(p, req)
	if !ok {
		return nil, false
	}
	if l.headers == nil {
		l.headers = map[headerKey]*headerFile{}
	}
	l.headers[key] = hf
	return hf, true
}

// headerKey is a header read by a package: the package, then the header's resolved absolute path.
type headerKey struct {
	pkg, abs string
}

// compareHeaderKeys orders headers by package, then path, as FinishDefines reports them.
func compareHeaderKeys(a, b headerKey) int {
	return cmp.Or(cmp.Compare(a.pkg, b.pkg), cmp.Compare(a.abs, b.abs))
}

// readHeader reads and classifies p's #defines, reporting into req's bag (WIRE.md §6.8).
func (l *Loader) readHeader(p project.Path, req Request) (*headerFile, bool) {
	src, _, ok := l.takeSource(p.Display, p.Abs, req) // Latin-1 bytes decode as themselves
	if !ok {
		return nil, false
	}
	c := l.Headers.classify(src, req)
	return &headerFile{defs: c.defs, skipped: c.skipped, ok: c.ok, path: src.Path, req: req, prefixes: map[string]bool{}}, true
}

// reportHeaderSkips is W7101, counting every skip matching a call's prefix given (WIRE.md §6.8).
func reportHeaderSkips(hf *headerFile) {
	all := hf.prefixes[""]
	var first source.Span
	var firstName string
	n := int64(0)
	for _, s := range hf.skipped {
		if !all && !matchesAnyPrefix(s.name, hf.prefixes) {
			continue
		}
		if n == 0 {
			first, firstName = s.at, s.name
		}
		n++
	}
	switch {
	case n == 1:
		diag.W7101.AtOne(first, hf.path, firstName).Report(hf.req.Bag)
	case n > 1:
		diag.W7101.AtMany(first, n, hf.path, firstName).Report(hf.req.Bag)
	}
}

// matchesAnyPrefix is whether name starts with one of prefixes' non-empty keys.
func matchesAnyPrefix(name string, prefixes map[string]bool) bool {
	for p := range prefixes { //canon:unordered name either matches some prefix or none, order-free
		if p != "" && strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// headerDefine is one accepted #define, in file order of its first definition (WIRE.md §6.8).
type headerDefine struct {
	name string
	val  int64
	at   source.Span
}

// skippedDefine is one function-like macro or unparseable expression, counted by W7101.
type skippedDefine struct {
	name string
	at   source.Span
}

// definesTable is defs, filtered to prefix, as `table Define` (WIRE.md §6.8).
func definesTable(defs []headerDefine, prefix string, t types.Type) value.Value {
	coll := &types.Collection{Kind: types.CollDefines, Elem: types.DefineType}
	tv := &value.Table{T: t}
	for _, d := range defs {
		if prefix != "" && !strings.HasPrefix(d.name, prefix) {
			continue
		}
		p := &value.Prov{Kind: value.ProvDefines, Span: d.at}
		rec := &value.Record{
			T:      types.DefineType,
			Fields: []value.Value{&value.Int{V: d.val, T: types.IntType, P: p}},
			Set:    []bool{true},
			Ident:  &value.Identity{Coll: coll, Key: value.Key{S: d.name}},
			P:      p,
		}
		tv.Entries = append(tv.Entries, rec)
	}
	return tv
}

// defineScan is one file's classification pass, WIRE.md §6.8's rules applied line by line.
type defineScan struct {
	pos      []int // logical byte k's offset in the file's raw content, for provenance
	file     source.FileID
	req      Request
	accepted map[string]int64
	firstAt  map[string]source.Span
	defs     []headerDefine
	skipped  []skippedDefine
}

// readDefines classifies every #define of src, in file order; ok is false after an E7102 (WIRE.md §6.8).
func readDefines(src *source.File, req Request) ([]headerDefine, []skippedDefine, bool) {
	logical, pos := canonicalizeHeader(src.Content)
	ds := &defineScan{pos: pos, file: src.ID, req: req, accepted: map[string]int64{}, firstAt: map[string]source.Span{}}
	ok, lineStart := true, 0
	for i := 0; i <= len(logical); i++ {
		if i < len(logical) && logical[i] != '\n' {
			continue
		}
		ok = ds.line(string(logical[lineStart:i]), lineStart) && ok
		lineStart = i + 1
	}
	return ds.defs, ds.skipped, ok
}

// line matches and classifies one logical line (WIRE.md §6.8); ok is false only after an E7102.
func (ds *defineScan) line(text string, lineStart int) bool {
	m := defineLineRe.FindStringSubmatchIndex(text)
	if m == nil {
		return true
	}
	name, rest := text[m[reNameStart]:m[reNameEnd]], text[m[reRestStart]:m[reRestEnd]]
	at := oneByteSpan(ds.file, ds.pos[lineStart+m[reHashEnd]])
	switch {
	case strings.HasPrefix(rest, "("): // a function-like macro (WIRE.md §6.8)
		ds.skipped = append(ds.skipped, skippedDefine{name: name, at: at})
		return true
	case strings.TrimSpace(rest) == "": // an include guard: skipped silently (WIRE.md §6.8)
		return true
	}
	val, ok := evalDefineExpr(strings.TrimSpace(rest), ds.accepted)
	if !ok {
		ds.skipped = append(ds.skipped, skippedDefine{name: name, at: at})
		return true
	}
	return ds.accept(name, val, at)
}

// accept is WIRE.md §6.8's duplicate rule: the same value is ignored, a different one is E7102.
func (ds *defineScan) accept(name string, val int64, at source.Span) bool {
	prev, dup := ds.accepted[name]
	switch {
	case !dup:
		ds.accepted[name], ds.firstAt[name] = val, at
		ds.defs = append(ds.defs, headerDefine{name: name, val: val, at: at})
		return true
	case prev == val:
		return true
	default:
		diag.E7102.At(at, name, val, prev, ds.firstAt[name]).Report(ds.req.Bag)
		return false
	}
}

func oneByteSpan(file source.FileID, at int) source.Span {
	return source.Span{File: file, Start: posOf(at), End: posOf(at + 1)}
}

// canonicalizeHeader is content spliced, then comment-stripped outside literals, pos its offsets (WIRE.md §6.8).
func canonicalizeHeader(content []byte) (logical []byte, pos []int) {
	text, at := spliceLines(content)
	logical, pos = make([]byte, 0, len(text)), make([]int, 0, len(text))
	for i := 0; i < len(text); {
		if end, isComment := commentSpan(text, i); isComment {
			logical, pos = append(logical, ' '), append(pos, at[i])
			i = end
			continue
		}
		end := i + 1
		if text[i] == '"' || text[i] == '\'' {
			end = literalEnd(text, i)
		}
		for ; i < end; i++ {
			logical, pos = append(logical, text[i]), append(pos, at[i])
		}
	}
	return logical, pos
}

// spliceLines removes every `\` LF in one pass as C does, at each kept byte's offset (WIRE.md §6.8 step 2).
func spliceLines(content []byte) (text []byte, at []int) {
	text, at = make([]byte, 0, len(content)), make([]int, 0, len(content))
	for i := 0; i < len(content); i++ {
		if content[i] == '\\' && i+1 < len(content) && content[i+1] == '\n' {
			i++
			continue
		}
		text, at = append(text, content[i]), append(at, i)
	}
	return text, at
}

// literalEnd is past the literal text[i] opens, `\` escaping, or at the next unspliced LF (WIRE.md §6.8).
func literalEnd(text []byte, i int) int {
	quote := text[i]
	for i++; i < len(text); i++ {
		switch {
		case text[i] == '\n':
			return i
		case text[i] == quote:
			return i + 1
		case text[i] == '\\' && i+1 < len(text) && text[i+1] != '\n':
			i++
		}
	}
	return i
}

// commentSpan is the end of the "//" or "/* */" comment starting at i (WIRE.md §6.8 step 3).
func commentSpan(text []byte, i int) (int, bool) {
	if i+1 >= len(text) || text[i] != '/' {
		return i, false
	}
	switch text[i+1] {
	case '/':
		return lineCommentEnd(text, i+twoBytes), true
	case '*':
		return blockCommentEnd(text, i+twoBytes), true
	default:
		return i, false
	}
}

// lineCommentEnd is the LF ending a "//" comment, kept as the line's end.
func lineCommentEnd(text []byte, i int) int {
	if n := bytes.IndexByte(text[i:], '\n'); n >= 0 {
		return i + n
	}
	return len(text)
}

// blockCommentEnd is past the "*/" closing a block comment, or the text's end.
func blockCommentEnd(text []byte, i int) int {
	if n := bytes.Index(text[i:], []byte(blockCommentClose)); n >= 0 {
		return i + n + len(blockCommentClose)
	}
	return len(text)
}
