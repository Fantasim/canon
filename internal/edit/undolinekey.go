package edit

import (
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// basePrinter is an applier printing values typed in the edit's base: its defaults are computed by
// the base's host, whose types those values carry (API.md M7).
func (c *undoCheck) basePrinter() *applier {
	return &applier{ctx: c.a.ctx, env: c.a.env, snap: c.a.base, base: c.a.base, host: c.a.baseHost, baseHost: c.a.baseHost, marks: c.a.marks}
}

// partKeyer writes a line no Source states as verification compares it (API.md E22; DECISIONS 462):
// literal parts by the values they type to in the base, loads and expressions by their text, fields
// in declaration order, one at a default literalDefault computes left out (E6).
type partKeyer struct {
	c   *undoCheck
	s   *Snapshot // the state holding the line
	f   *syntax.File
	pkg string
	pa  *applier
}

// lineParts is the line at path, whose value node n s states in f, as partKeyer writes it; false
// where a part is no record, table, map or list literal (a spread, a case literal).
func (c *undoCheck) lineParts(s *Snapshot, path string, f *syntax.File, n syntax.Node) (string, bool) {
	t, pkg, ok := c.a.base.lineType(path)
	if !ok {
		return "", false
	}
	k := &partKeyer{c: c, s: s, f: f, pkg: pkg, pa: c.basePrinter()}
	return k.key(n, t)
}

// text is n's text with no comment, its gaps single (lineKey).
func (k *partKeyer) text(n syntax.Node) string {
	sp := k.f.Span(n)
	return lineKey(string(k.f.Src.Content[sp.Start:sp.End]))
}

// typed is n, a literal part, typed as a Source of t in the base.
func (k *partKeyer) typed(n syntax.Node, t types.Type) (value.Value, bool) {
	ty := Typer{Host: k.c.a.baseHost, Pkg: k.pkg, marks: k.c.a.marks}
	v, err := ty.Value(k.c.a.ctx, Source(k.text(n)), t)
	return v, err == nil
}

// key is node n of declared type t as partKeyer writes it.
func (k *partKeyer) key(n syntax.Node, t types.Type) (string, bool) {
	switch {
	case k.s.opaqueIn(n) == 0:
		v, ok := k.typed(n, t)
		if !ok {
			return "", false
		}
		text, err := k.pa.canonText(v)
		return text, err == nil
	case k.s.nodeForm(n) != shape.FormLiteral:
		return opaqueOpen + k.text(n) + opaqueClose, true
	}
	if l, ok := n.(*syntax.ListLit); ok {
		return k.list(l, elemType(present(t)))
	}
	lit, ok := n.(*syntax.BraceLit)
	if !ok || hasSpread(lit) {
		return "", false
	}
	if tt, isTable := baseOf(present(t)).(*types.TableType); isTable {
		return k.table(lit, tt.Elem)
	}
	if _, vt, isMap := mapTypes(present(t)); isMap {
		return k.mapItems(lit, vt)
	}
	return k.record(lit, present(t))
}

// record is a record literal's stated fields, in declaration order.
func (k *partKeyer) record(lit *syntax.BraceLit, rt types.Type) (string, bool) {
	fields := fieldsOf(rt)
	if len(fields) == 0 {
		return "", false
	}
	var b strings.Builder
	for i, f := range fields {
		fi := fieldItem(lit, f.Name)
		if fi == nil {
			continue
		}
		v := syntax.Unparen(fi.Value)
		if k.atDefault(v, rt, fields, i) {
			continue
		}
		part, ok := k.key(v, f.Type)
		if !ok {
			return "", false
		}
		b.WriteString(f.Name + colonSp + part + listSep)
	}
	return braceOpenSp + b.String() + braceCloseSp, true
}

// atDefault reports v, a literal stating field i of a record of type rt, equal to a default
// literalDefault computes: leaving the field out gives the same value (API.md E6).
func (k *partKeyer) atDefault(v syntax.Node, rt types.Type, fields []*types.Field, i int) bool {
	if k.s.opaqueIn(v) != 0 || !k.s.literalDefault(rt, fields[i].Name) {
		return false
	}
	tv, ok := k.typed(v, fields[i].Type)
	if !ok {
		return false
	}
	rec := &value.Record{T: rt, Fields: make([]value.Value, len(fields)), Set: make([]bool, len(fields))}
	rec.Fields[i], rec.Set[i] = tv, true
	return k.pa.isDefault(rec, i)
}

// table is a table literal's entries, in order.
func (k *partKeyer) table(lit *syntax.BraceLit, elem types.Type) (string, bool) {
	var b strings.Builder
	for _, it := range lit.Items {
		ei, ok := it.(*syntax.EntryItem)
		if !ok || ei.Mods != nil || len(ei.Annotations) > 0 || ei.Value == nil {
			return "", false
		}
		part, ok := k.key(ei.Value, elem)
		if !ok {
			return "", false
		}
		b.WriteString(ei.Key.Name + space + part + listSep)
	}
	return braceOpenSp + b.String() + braceCloseSp, true
}

// mapItems is a map literal's items, in order, each key by its text.
func (k *partKeyer) mapItems(lit *syntax.BraceLit, vt types.Type) (string, bool) {
	var b strings.Builder
	for _, it := range lit.Items {
		var key string
		var val syntax.Expr
		switch x := it.(type) {
		case *syntax.MapItem:
			key, val = k.text(x.Key), x.Value
		case *syntax.FieldItem:
			key, val = x.Name.Name, x.Value
		default:
			return "", false
		}
		part, ok := k.key(syntax.Unparen(val), vt)
		if !ok {
			return "", false
		}
		b.WriteString(key + colonSp + part + listSep)
	}
	return braceOpenSp + b.String() + braceCloseSp, true
}

// list is a list literal's elements, in order.
func (k *partKeyer) list(l *syntax.ListLit, et types.Type) (string, bool) {
	var b strings.Builder
	for _, e := range l.Elems {
		part, ok := k.key(syntax.Unparen(e), et)
		if !ok {
			return "", false
		}
		b.WriteString(part + listSep)
	}
	return string(bracketOpen) + b.String() + bracketClose, true
}
