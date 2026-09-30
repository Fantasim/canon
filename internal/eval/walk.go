package eval

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// walkList stores each element; the list is rebuilt only when an element changed.
func (r *run) walkList(v value.Value, lt *types.ListType, s site, at *vpath) value.Value {
	l, ok := v.(*value.List)
	if !ok {
		return v
	}
	var out []value.Value
	for i, e := range l.Elems {
		eat := at.index(i)
		if k, keyed := std.KeyOf(e); keyed && lt.KeyedBy != nil {
			eat = at.key(k)
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

// walkMap stores each key and value, in insertion order.
func (r *run) walkMap(v value.Value, mt *types.MapType, s site, at *vpath) value.Value {
	m, ok := v.(*value.Map)
	if !ok {
		return v
	}
	out := &value.Map{T: m.T, Keys: make([]value.Value, len(m.Keys)), Vals: make([]value.Value, len(m.Vals)), P: m.P}
	for i, k := range m.Keys {
		out.Keys[i] = r.walk(k, mt.Key, s, at)
		out.Vals[i] = r.walk(m.Vals[i], mt.Value, s, at.mapKey(k, mt.Key))
		if out.Keys[i] == nil || out.Vals[i] == nil {
			return nil
		}
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
	if w == nil || w.prog == nil {
		return written{}
	}
	if !w.built {
		w.build()
	}
	return w.at[x]
}

// build indexes each refinement at its first place in the program.
func (w *writtenIndex) build() {
	w.built, w.at = true, map[*types.Refined]written{}
	for _, pkg := range w.prog.Packages {
		for _, f := range pkg.Files {
			w.merge(cachedFile(w.memo, filesRefined, f, refinedFiles.of))
		}
	}
}

// merge indexes the refinements of one file's types not written in an earlier place.
func (w *writtenIndex) merge(ts []typeAt) {
	for _, ta := range ts {
		x, ok := w.prog.Info.TypeExprs[ta.t].(*types.Refined)
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
