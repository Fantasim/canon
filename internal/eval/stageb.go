package eval

import (
	"context"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// StageB serves the verification of one top-level value; each step it spends is charged to that value's root (EVALUATION.md §5).
type StageB struct {
	e       *Evaluator
	r       *run
	root    Root
	emitted []*diag.Builder
}

// Verifying is the evaluator serving the verification of root into bag, one run for all it spends.
func (e *Evaluator) Verifying(ctx context.Context, root Root, bag *diag.Bag) *StageB {
	s := &StageB{e: e, root: root}
	s.r = e.newRun(ctx, charge{pkg: root.Pkg, name: root.Name}, nil)
	s.r.fr.pkg, s.r.sink, s.r.emitted = root.Pkg, bag, &s.emitted
	s.r.free = e.late.free // phase 8's verifications spend no budget (EVALUATION.md §2.1)
	return s
}

// Emitted is every finding the evaluator reported for this verification, in order.
func (s *StageB) Emitted() []*diag.Builder {
	return s.emitted
}

// Charge spends n steps of a type function; false once the budget ran out, E4401 at at (EVALUATION.md §12.1).
func (s *StageB) Charge(n int, at source.Span) bool {
	return s.r.spend(n, func() source.Span { return at })
}

// Store converts v to t as any storage point does, its findings related to rel and at path; false: a hard error (TYPES.md §6.2).
func (s *StageB) Store(v value.Value, t types.Type, rel source.Span, path string) (value.Value, bool) {
	r, at := s.r, rootPath(path)
	if rt, isRef := unwrapOptional(t).Base().(*types.RefType); isRef {
		v = r.entryToRef(v, rt, at, nil)
	} else {
		v = r.coerce(v, t, nil)
	}
	if v == nil || r.failed {
		return nil, false
	}
	v = r.store(v, t, site{decl: rel, has: rel != source.Span{}}, at)
	return v, v != nil && !r.failed
}

// Params is the arguments applied record instance rec was built with, nil for none (TYPES.md §11.1).
func (s *StageB) Params(rec *value.Record) map[*types.Param]value.Value {
	return s.e.boundParams(rec)
}

// Bare fills rec, a case a symbol names bare, with its defaults: the required field it lacks, nil for none; false: a hard error.
func (s *StageB) Bare(rec *value.Record) (*types.Field, bool) {
	return s.r.bare(rec)
}

// Written reports a value kept as written: a literal given to a dependent field, or loaded into one (TYPES.md §11.4).
func (s *StageB) Written(v value.Value) bool {
	return s.e.isWritten(v)
}

// Refine checks t's outer refinements on v, a container at a dependent branch, its first where run charged (EVALUATION.md §12.1).
func (s *StageB) Refine(v value.Value, t types.Type, rel source.Span, path string) bool {
	r, at := s.r, rootPath(path)
	for t != nil && v != nil && !r.failed {
		switch x := t.(type) {
		case *types.Alias:
			t = x.Def
		case *types.Refined:
			v, t = r.refine(v, x, site{decl: rel, has: rel != source.Span{}}, at), x.Of
		default:
			return v != nil && !r.failed
		}
	}
	return v != nil && !r.failed
}

// Verified is what verification remembered of instance rec, its conversion done once for every root reaching it (EVALUATION.md §4.2).
func (s *StageB) Verified(rec *value.Record) (any, bool) {
	for e := s.e; e != nil; e = e.parent {
		if v, ok := e.verified[rec]; ok {
			return v, true
		}
	}
	return nil, false
}

// Remember records what verification made of instance rec, and of its copy cp.
func (s *StageB) Remember(rec, cp *value.Record, memo any) {
	s.e.verified[rec] = memo
	s.e.verified[cp] = memo
}

// isWritten reports a value kept as written, marked by this evaluator or a vector's parent, whose stages A and B are done.
func (e *Evaluator) isWritten(v value.Value) bool {
	for ; e != nil; e = e.parent {
		if e.written[v] {
			return true
		}
	}
	return false
}

// loadedPart is a part of a loaded value and the type the load decoded it as.
type loadedPart struct {
	v value.Value
	t types.Type
}

// markLoaded marks each part of v, loaded as t, that the decoder wrote into a dependent position (TYPES.md §11.4).
func (e *Evaluator) markLoaded(v value.Value, t types.Type) {
	stack := []loadedPart{{v: v, t: t}}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if p.v == nil || isNone(p.v) || !e.dependent(p.t) {
			continue
		}
		if std.IsDependent(branchBase(p.t)) {
			e.written[p.v], e.gens.written = true, e.gens.written+1
			continue
		}
		stack = append(stack, loadedParts(p.v, p.t)...)
	}
}

// loadedParts are the parts of v with the types they were decoded as.
func loadedParts(v value.Value, t types.Type) []loadedPart {
	var out []loadedPart
	switch x := v.(type) {
	case *value.Record:
		for i, f := range fieldsOf(x.T) {
			if i < len(x.Fields) {
				out = append(out, loadedPart{v: x.Fields[i], t: f.Type})
			}
		}
	case *value.List:
		for _, el := range x.Elems {
			out = append(out, loadedPart{v: el, t: elemOf(t)})
		}
	case *value.Map:
		out = mapParts(x, branchBase(t))
	case *value.Table:
		for _, en := range x.Entries {
			out = append(out, loadedPart{v: en, t: en.T})
		}
	}
	return out
}

// mapParts are a loaded map's keys and values with their types.
func mapParts(m *value.Map, t types.Type) []loadedPart {
	var kt, vt types.Type
	switch x := t.(type) {
	case *types.MapType:
		kt, vt = x.Key, x.Value
	case *types.DepMapType:
		vt = x.Value
	}
	out := make([]loadedPart, 0, len(m.Keys)+len(m.Vals))
	for i, k := range m.Keys {
		out = append(out, loadedPart{v: k, t: kt})
		if i < len(m.Vals) {
			out = append(out, loadedPart{v: m.Vals[i], t: vt})
		}
	}
	return out
}

// Moved records that to, a copy verification built, replaces from: its marks, and a record's collections and refs (EVALUATION.md §3.4).
func (s *StageB) Moved(from, to value.Value) value.Value {
	e := s.e
	e.carry(from, to)
	rec, isRec := from.(*value.Record)
	if cp, ok := to.(*value.Record); ok && isRec && rec != cp {
		e.copiedFrom(rec, cp)
		e.transfer(rec, cp, nil)
	}
	return to
}

// Replace gives the root the value verification converted (TYPES.md §11.6); a vector keeps its parent's.
func (s *StageB) Replace(v value.Value) {
	e := s.e
	st := e.rootState(s.root)
	if st == nil || st.status != done || e.parent != nil && e.parent.roots[s.root] == st {
		return
	}
	st.v = v
}

// bare fills rec, a case written bare, with its defaults (TYPES.md §8.2): the required field it lacks, nil for none.
func (r *run) bare(rec *value.Record) (*types.Field, bool) {
	c, _ := rec.T.(*types.CaseType)
	for i, f := range c.Fields {
		_, optional := f.Type.Base().(*types.OptionalType)
		switch {
		case f.Input != nil:
			continue
		case f.Default == nil && !optional:
			return f, true
		case f.Default == nil:
			rec.Fields[i] = &value.None{T: f.Type, P: rec.P}
			continue
		}
		if rec.Fields[i] = r.defaultValue(rec, f, nil, rec.P); rec.Fields[i] == nil {
			return nil, false
		}
	}
	return nil, true
}

// literal marks v, a literal token evaluated for a dependent field, kept as written (TYPES.md §11.4).
func (r *run) literal(v value.Value) value.Value {
	if v != nil && r.dep != nil && r.ev.dependent(r.dep.field) {
		r.ev.written[v], r.ev.gens.written = true, r.ev.gens.written+1
	}
	return v
}

// dependent reports a type holding a dependent type, through optionals, unions, lists, maps and pairs.
func (e *Evaluator) dependent(t types.Type) bool {
	if t == nil {
		return false
	}
	if d, ok := e.deps[t]; ok {
		return d
	}
	d := std.IsDependent(t)
	switch x := t.Base().(type) {
	case *types.OptionalType:
		d = e.dependent(x.Elem)
	case *types.LitUnionType:
		d = e.dependent(x.Of)
	case *types.ListType:
		d = e.dependent(x.Elem)
	case *types.MapType:
		d = e.dependent(x.Key) || e.dependent(x.Value)
	case *types.DepMapType:
		d = e.dependent(x.Value)
	case *types.PairType:
		d = e.dependent(x.A) || e.dependent(x.B)
	}
	e.deps[t] = d
	return d
}
