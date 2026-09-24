package eval

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// evalLink is a link of a postfix chain: `.f`, `?.f`, `[i]`, a call or `!` (TYPES.md §6.5).
func evalLink(r *run, e syntax.Expr, _ *vpath) value.Value {
	switch x := e.(type) {
	case *syntax.SelectorExpr:
		return r.selector(x)
	case *syntax.IndexExpr:
		return r.index(x)
	case *syntax.CallExpr:
		return r.call(x)
	case *syntax.ForceExpr:
		return r.forceLink(x)
	}
	return nil
}

// recv is a link's receiver: a link of the same chain keeps its `?.` state (TYPES.md §6.5).
func (r *run) recv(x syntax.Expr) value.Value {
	switch x.(type) {
	case nil:
		if r.fr.short == nil || !r.step(nil) {
			r.bug(nil)
			return nil
		}
		return r.fr.short
	case *syntax.SelectorExpr, *syntax.IndexExpr, *syntax.CallExpr, *syntax.ForceExpr:
		return r.node(x, nil)
	}
	return r.eval(x)
}

// unwrap applies `?.`: on none the rest of the chain is skipped; false when the link stops.
func (r *run) unwrap(v value.Value, optional bool) bool {
	if r.chainNone || v == nil {
		return false
	}
	if optional && isNone(v) {
		r.chainNone = true
		return false
	}
	return true
}

// selector is `x.name`: a field, entry or member, or a qualified form (TYPES.md §3.5).
func (r *run) selector(x *syntax.SelectorExpr) value.Value {
	sel := r.ev.info.Selections[x]
	if sel == nil {
		if obj := r.ev.info.NameUses[x.Name]; obj != nil {
			return r.objectValue(obj, x)
		}
		r.bug(x)
		return nil
	}
	v := r.recv(x.X)
	if !r.unwrap(v, x.Optional) {
		return nil
	}
	if sel.Deref {
		if v = r.deref(v, x); v == nil {
			return nil
		}
	}
	name := x.Name.Name
	switch sel.Kind {
	case check.SelField:
		return r.field(v, name)
	case check.SelEntry:
		return r.lookupKey(v, &value.Str{V: name, T: types.StringType}, x)
	case check.SelBuiltinMember:
		return r.builtinMember(v, name, x)
	default:
	}
	r.bug(x)
	return nil
}

// field is field name of a record or case value.
func (r *run) field(v value.Value, name string) value.Value {
	rec, ok := v.(*value.Record)
	if !ok {
		r.bug(nil)
		return nil
	}
	i := fieldIndex(rec.T, name)
	if i < 0 || rec.Fields[i] == nil {
		r.bug(nil)
		return nil
	}
	return r.read(rec.Fields[i])
}

// builtinMember is a member of STDLIB.md §3.
func (r *run) builtinMember(v value.Value, name string, x syntax.Expr) value.Value {
	p := r.prov(x, value.ProvComputed)
	switch v := v.(type) {
	case *value.Record:
		return r.recordMember(v, name, x, p)
	case *value.Member:
		m := v.Enum.Members[v.Index]
		return memberField(m, name, p)
	case *value.Ref:
		if name == memberID {
			return &value.Str{V: v.Key.Text(), T: types.StringType, P: p}
		}
	case *value.Range:
		return r.rangeMember(v, name, x, p)
	}
	r.bug(x)
	return nil
}

// recordMember is `.id` and `.retired` of an entry, `.kind` of a variant value.
func (r *run) recordMember(v *value.Record, name string, x syntax.Expr, p *value.Prov) value.Value {
	switch name {
	case memberID:
		if v.Ident != nil {
			return &value.Str{V: v.Ident.Key.Text(), T: types.StringType, P: p}
		}
	case memberRetired:
		return &value.Bool{V: v.Ident != nil && v.Ident.Retired, P: p}
	case memberKind:
		ct, ok := v.T.Base().(*types.CaseType)
		kt, isKind := r.typeOf(x).Base().(*types.VariantKindType)
		if ok && isKind {
			return &value.CaseKind{T: kt, Index: ct.Index, P: p}
		}
	}
	r.bug(x)
	return nil
}

func memberField(m *types.Member, name string, p *value.Prov) value.Value {
	switch name {
	case memberName:
		return &value.Str{V: m.Name, T: types.StringType, P: p}
	case memberWire:
		return &value.Str{V: m.Wire, T: types.StringType, P: p}
	case memberIndex:
		return &value.Int{V: int64(m.Index), T: types.IntType, P: p}
	}
	return &value.Int{V: m.Code, T: types.IntType, P: p}
}

// rangeMember is `.start`, and `.end`: E4002 on an open range.
func (r *run) rangeMember(v *value.Range, name string, x syntax.Expr, p *value.Prov) value.Value {
	if name == memberStart {
		return &value.Int{V: v.Start, T: types.IntType, P: p}
	}
	if !v.HasEnd {
		r.fail(diag.E4002.AtOpen(r.span(x)))
		return nil
	}
	return &value.Int{V: v.End, T: types.IntType, P: p}
}

// forceLink is `x!`: E4001 at the `!` when x is none (TYPES.md §6.5).
func (r *run) forceLink(x *syntax.ForceExpr) value.Value {
	v := r.recv(x.X)
	if !r.unwrap(v, false) {
		return nil
	}
	if isNone(v) {
		r.fail(diag.E4001.At(r.span(x), r.span(x.X)))
		return nil
	}
	return v
}

// lookupKey is the entry of a key in a table or keyed list, or the value of a key in a map:
// E4002 when it is missing.
func (r *run) lookupKey(coll, key value.Value, x syntax.Expr) value.Value {
	if m, ok := coll.(*value.Map); ok {
		r.site = r.span(x)
		i, ok := std.MapIndex(r.host(), m, key)
		if !ok {
			return nil
		}
		if i >= 0 {
			return r.read(m.Vals[i])
		}
	} else if k, ok := std.KeyOf(key); ok {
		if e, found := r.ev.entry(coll, k); found {
			return r.read(e)
		}
	}
	r.fail(diag.E4002.AtKey(r.span(x), key, r.receiverName(x)))
	return nil
}

// receiverName names the collection of a lookup in E4002: its source when it is a name or a
// path of names, else its type.
func (r *run) receiverName(x syntax.Expr) string {
	var recv syntax.Expr
	switch y := x.(type) {
	case *syntax.SelectorExpr:
		recv = y.X
	case *syntax.IndexExpr:
		recv = y.X
	}
	if isPath(recv) {
		sp := r.span(recv)
		return string(r.fr.file.Src.Content[sp.Start:sp.End])
	}
	return r.typeOf(recv).String()
}

// isPath reports a name followed by `.name` segments.
func isPath(x syntax.Expr) bool {
	for {
		switch y := x.(type) {
		case *syntax.IdentExpr:
			return true
		case *syntax.SelectorExpr:
			if y.X == nil || y.Optional {
				return false
			}
			x = y.X
		default:
			return false
		}
	}
}
