package eval

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// evalIdent is a name: a key or a symbol the checker kept, else what it names (TYPES.md §3.3).
func evalIdent(r *run, e syntax.Expr, _ *vpath) value.Value {
	x := e.(*syntax.IdentExpr)
	info := r.ev.info
	if coll := info.Keys[x]; coll != nil {
		return r.keyValue(x, value.Key{S: x.Name})
	}
	if info.Symbols[x] {
		return &value.Symbol{Name: x.Name, T: r.typeOf(x), P: r.prov(x, value.ProvLiteral)}
	}
	obj := info.Uses[x]
	if obj == nil {
		r.bug(x)
		return nil
	}
	return r.objectValue(obj, x)
}

// keyValue is a key written in source: a ref, or the key itself (TYPES.md §4.1).
func (r *run) keyValue(e syntax.Expr, k value.Key) value.Value {
	t, p := r.typeOf(e), r.prov(e, value.ProvLiteral)
	if _, isRef := t.Base().(*types.RefType); isRef {
		return &value.Ref{T: t, Key: k, P: p}
	}
	if k.IsInt {
		return &value.Int{V: k.I, T: types.IntType, P: p}
	}
	return &value.Str{V: k.S, T: types.StringType, P: p}
}

// literalKey is the key a string or integer literal checked against a ref names (TYPES.md §4.1).
func (r *run) literalKey(e syntax.Expr) (value.Key, bool) {
	switch x := e.(type) {
	case *syntax.IntLit:
		return value.Key{I: x.Value.Int64(), IsInt: true}, x.Value.IsInt64()
	case *syntax.RawStringLit:
		return value.Key{S: x.Value}, true
	case *syntax.StringLit:
		if len(x.Parts) == 1 && x.Parts[0].Interp == nil {
			return value.Key{S: x.Parts[0].Text}, true
		}
		if len(x.Parts) == 0 {
			return value.Key{}, true
		}
	}
	return value.Key{}, false
}

// objectValue is the value a name denotes.
func (r *run) objectValue(obj check.Object, at syntax.Expr) value.Value {
	switch obj.Kind() {
	case check.ObjLocal, check.ObjParam:
		return r.local(obj)
	case check.ObjField:
		return r.selfField(obj.Name())
	case check.ObjConst, check.ObjLet:
		return r.global(obj, at)
	case check.ObjFn:
		return &closure{fn: obj, t: obj.Type(), p: r.prov(at, value.ProvLiteral)}
	case check.ObjMember:
		return r.member(obj, at)
	case check.ObjCase:
		return r.caseValue(obj, at)
	case check.ObjEntry:
		return &value.Ref{T: r.typeOf(at), Key: value.Key{S: obj.Name()}, P: r.prov(at, value.ProvLiteral)}
	default:
	}
	r.bug(at)
	return nil
}

// local is a local, a parameter, or `it` in a where predicate.
func (r *run) local(obj check.Object) value.Value {
	if v, ok := r.fr.vars[obj]; ok {
		return r.read(v)
	}
	if obj.Name() == itWord && r.fr.it != nil {
		return r.read(r.fr.it)
	}
	r.bug(nil)
	return nil
}

// selfField is a field named in its record body: `self.f` (TYPES.md §3.3 step 3).
func (r *run) selfField(name string) value.Value {
	rec, ok := r.fr.self.(*value.Record)
	if !ok {
		r.bug(nil)
		return nil
	}
	if i := fieldIndex(rec.T, name); i >= 0 && rec.Fields[i] != nil {
		return r.read(rec.Fields[i])
	}
	r.bug(nil)
	return nil
}

// global forces a const or let; a poisoned one aborts the reader silently (EVALUATION.md §7.2).
func (r *run) global(obj check.Object, at syntax.Expr) value.Value {
	if obj.Kind() == check.ObjLet && r.nonConstant() {
		return nil
	}
	v, ok := r.ev.force(r.ctx, r.ev.state(obj), r, at)
	if !ok {
		r.readPoisoned(obj.Pkg(), obj.Name(), at)
		return nil
	}
	return r.read(v)
}

// nonConstant aborts a fold, silently, at what is not a constant expression; false outside a fold (TYPES.md §15, DECISIONS 150).
func (r *run) nonConstant() bool {
	if r.ev.constant {
		r.stop()
	}
	return r.ev.constant
}

// readPoisoned aborts the run at a read of poisoned pkg.name, which a test names (EVALUATION.md §7.2).
func (r *run) readPoisoned(pkg, name string, at syntax.Node) {
	if !r.failed {
		r.poisonAt, r.poisonRoot, r.poisonSpan = r.qualified(pkg, name), Root{Pkg: pkg, Name: name}, r.span(at)
		r.ev.notePoisonedRead(r, r.poisonRoot)
	}
	r.stop()
}

// member is an enum member, found by name in its enum.
func (r *run) member(obj check.Object, at syntax.Expr) value.Value {
	e, ok := obj.Type().Base().(*types.EnumType)
	if !ok {
		e, ok = r.typeOf(at).Base().(*types.EnumType)
	}
	if !ok {
		r.bug(at)
		return nil
	}
	for i, m := range e.Members {
		if m.Name == obj.Name() {
			return &value.Member{Enum: e, Index: i, P: r.prov(at, value.ProvLiteral)}
		}
	}
	r.bug(at)
	return nil
}

// caseValue is a case as a Kind(V) value, or a case written without fields (TYPES.md §8.2).
func (r *run) caseValue(obj check.Object, at syntax.Expr) value.Value {
	ct, ok := obj.Type().(*types.CaseType)
	if !ok {
		r.bug(at)
		return nil
	}
	if k, isKind := r.typeOf(at).Base().(*types.VariantKindType); isKind {
		return &value.CaseKind{T: k, Index: ct.Index, P: r.prov(at, value.ProvLiteral)}
	}
	return r.bareCase(ct, at)
}

// fieldIndex is the index of field name of a record or case type, -1 when absent.
func fieldIndex(t types.Type, name string) int {
	for i, f := range fieldsOf(t) {
		if f.Name == name {
			return i
		}
	}
	return -1
}

// fieldsOf are the fields of a record, applied record or case type.
func fieldsOf(t types.Type) []*types.Field {
	switch x := t.Base().(type) {
	case *types.RecordType:
		return x.Fields
	case *types.AppliedRecord:
		return x.Rec.Fields
	case *types.CaseType:
		return x.Fields
	}
	return nil
}
