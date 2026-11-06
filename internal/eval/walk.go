package eval

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// conversion is a storage conversion in progress and the containers it converted cleanly (DECISIONS 513).
type conversion struct {
	root value.Value          // the value being stored, never met again inside itself
	done map[convKey]convDone // nil until a container is kept
	busy int                  // the findings reported and where predicates run so far
}

// convKey is a container and the declared type it is converted to.
type convKey struct {
	v value.Value
	t types.Type
}

// convDone is a clean visit's result and the evaluator's marks once it was converted.
type convDone struct {
	w     value.Value
	marks int
}

// convertOnce is a container converted once per conversion at a type, unless it found, spent or a mark came since (DECISIONS 513).
func (r *run) convertOnce(v value.Value, t types.Type, s site, at *vpath) value.Value {
	k := convKey{v: v, t: t}
	if d, ok := r.conv.done[k]; ok && d.marks == r.ev.marksGen() {
		return d.w
	}
	busy, spent := r.conv.busy, r.spent()
	w := r.container(v, t, s, at)
	if w == nil || r.conv.busy != busy || r.spent() != spent || v == r.conv.root {
		return w
	}
	if r.conv.done == nil {
		r.conv.done = map[convKey]convDone{}
	}
	r.conv.done[k] = convDone{w: w, marks: r.ev.marksGen()}
	return w
}

// spent is the steps the run has spent, its package's or its free ones.
func (r *run) spent() int64 {
	return r.ev.steps + r.freeSteps
}

// container converts a list, map or pair against its declared type.
func (r *run) container(v value.Value, t types.Type, s site, at *vpath) value.Value {
	switch x := t.(type) {
	case *types.ListType:
		return r.walkList(v, x, s, at)
	case *types.MapType:
		return r.walkMap(v, x, s, at)
	case *types.PairType:
		return r.walkPair(v, x, s, at)
	}
	return v
}

// walkList stores each element; the list is rebuilt only when an element changed.
func (r *run) walkList(v value.Value, lt *types.ListType, s site, at *vpath) value.Value {
	l, ok := v.(*value.List)
	if !ok {
		return v
	}
	var out []value.Value
	for i, e := range l.Elems {
		var eat *vpath
		if k, keyed := std.KeyOf(e); keyed && lt.KeyedBy != nil {
			eat = at.key(k)
		} else {
			eat = at.index(i)
		}
		w := r.walk(e, lt.Elem, s, eat)
		if w == nil {
			return nil
		}
		if w != e && out == nil {
			out = append([]value.Value(nil), l.Elems...)
		}
		if out != nil {
			out[i] = w
		}
	}
	if out == nil {
		return v
	}
	return r.ev.carry(v, &value.List{T: l.T, Elems: out, P: l.P})
}

// walkMap stores each key and value, in insertion order; the map is rebuilt only when one changed.
func (r *run) walkMap(v value.Value, mt *types.MapType, s site, at *vpath) value.Value {
	m, ok := v.(*value.Map)
	if !ok {
		return v
	}
	var out *value.Map
	for i, k := range m.Keys {
		nk := r.walk(k, mt.Key, s, at)
		nv := r.walk(m.Vals[i], mt.Value, s, at.mapKey(k, mt.Key))
		if nk == nil || nv == nil {
			return nil
		}
		if out == nil && (nk != k || nv != m.Vals[i]) {
			out = &value.Map{T: m.T, Keys: slices.Clone(m.Keys), Vals: slices.Clone(m.Vals), P: m.P}
		}
		if out != nil {
			out.Keys[i], out.Vals[i] = nk, nv
		}
	}
	if out == nil {
		return v
	}
	return r.ev.carry(v, out)
}

func (r *run) walkPair(v value.Value, pt *types.PairType, s site, at *vpath) value.Value {
	p, ok := v.(*value.Pair)
	if !ok {
		return v
	}
	a, b := r.walk(p.A, pt.A, s, at), r.walk(p.B, pt.B, s, at)
	if a == nil || b == nil {
		return nil
	}
	if a == p.A && b == p.B {
		return v
	}
	return r.ev.carry(v, &value.Pair{T: p.T, A: a, B: b, P: p.P})
}

// needsCheck reports a type a stored value must be walked against: a refinement, a sized
// integer, a Duration or a Float32 somewhere in its optionals, lists, maps and pairs.
func (e *Evaluator) needsCheck(t types.Type) bool {
	switch x := t.(type) {
	case *types.Alias:
		return e.needsCheck(x.Def)
	case *types.Refined:
		return true
	case types.Basic:
		switch x.K {
		case types.Duration:
			return true
		case types.Int:
			return x != types.IntType
		case types.Float:
			return x.Bits == types.Float32Type.Bits
		default:
		}
		return false
	case *types.OptionalType:
		return e.needsCheck(x.Elem)
	case *types.LitUnionType:
		return e.needsCheck(x.Of)
	case *types.ListType:
		return e.needsCheck(x.Elem)
	case *types.MapType:
		return e.needsCheck(x.Key) || e.needsCheck(x.Value)
	case *types.PairType:
		return e.needsCheck(x.A) || e.needsCheck(x.B)
	}
	return false
}

// writtenIndex locates each refinement where it is written: the declaration it belongs to
// (a field or let with its type, else the type itself) and its bound or predicate.
type writtenIndex struct {
	prog  *check.Program
	built bool
	at    map[*types.Refined]written
	memo  *memoUse // keeps each file's types across the evaluators of an epoch

	met  *index      // a fold's evaluator, with no Program: the files it met, typed by info
	info *check.Info // the fold's Info
	seen int         // the files of met indexed so far
}

// foldWritten is the writtenIndex of a fold's evaluator, whose index is x: its storage findings quote what stage A's do, so the bag merges the two (API.md F2).
func foldWritten(x *index, info *check.Info) *writtenIndex {
	return &writtenIndex{met: x, info: info, at: map[*types.Refined]written{}}
}

type written struct {
	decl, arg source.Span
	has       bool
}

// typeAt is a type a file writes, where it is declared and what E3204 quotes; syntax only, so
// a memo may keep it across programs.
type typeAt struct {
	t         syntax.Type
	decl, arg source.Span
}

func (w *writtenIndex) of(x *types.Refined) written {
	switch {
	case w == nil:
		return written{}
	case w.prog != nil && !w.built:
		w.build()
	case w.met != nil && w.info != nil:
		w.meet()
	}
	return w.at[x]
}

// build indexes each refinement at its first place in the program.
func (w *writtenIndex) build() {
	w.built, w.at = true, map[*types.Refined]written{}
	for _, pkg := range w.prog.Packages {
		for _, f := range pkg.Files {
			w.merge(w.prog.Info, cachedFile(w.memo, filesRefined, f, refinedFiles.of))
		}
	}
}

// meet indexes the files a fold's evaluator met since the last lookup, in the order it met them.
func (w *writtenIndex) meet() {
	for ; w.seen < len(w.met.files); w.seen++ {
		w.merge(w.info, cachedFile(w.memo, filesRefined, w.met.files[w.seen], refinedFiles.of))
	}
}

// merge indexes the refinements of one file's types not written in an earlier place.
func (w *writtenIndex) merge(info *check.Info, ts []typeAt) {
	for _, ta := range ts {
		x, ok := info.TypeExprs[ta.t].(*types.Refined)
		if _, seen := w.at[x]; ok && !seen {
			w.at[x] = written{decl: ta.decl, arg: ta.arg, has: true}
		}
	}
}

// typesIn is the types f writes, in order, each with the field or let declaring it, else itself.
func typesIn(f *syntax.File) []typeAt {
	owner := map[syntax.Type]source.Span{}
	var out []typeAt
	syntax.Inspect(f, func(n syntax.Node) bool {
		switch d := n.(type) {
		case *syntax.FieldDecl:
			owner[d.Type] = f.Span(d.Name).Cover(f.Span(d.Type))
		case *syntax.LetDecl:
			if d.Type != nil {
				owner[d.Type] = f.Span(d.Name).Cover(f.Span(d.Type))
			}
		case syntax.Type:
			decl, named := owner[d]
			if !named {
				decl = f.Span(d)
			}
			out = append(out, typeAt{t: d, decl: decl, arg: f.Span(refinementArg(d))})
		}
		return true
	})
	return out
}

// refinementArg is what E3204 and E3206 quote: the predicate of a where, else the first
// argument of the type that is not a regex.
func refinementArg(t syntax.Type) syntax.Node {
	var args *syntax.TypeArgs
	switch x := t.(type) {
	case *syntax.WhereType:
		return x.Pred
	case *syntax.NamedType:
		args = x.Args
	case *syntax.ListType:
		args = x.Args
	case *syntax.MapType:
		args = x.Args
	}
	if args == nil {
		return t
	}
	for _, a := range args.Args {
		if _, isRegex := a.(*syntax.RegexLit); !isRegex {
			return a
		}
	}
	return t
}
