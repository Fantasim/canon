package edit

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// listLit is `[…]` as a plain or keyed list.
func (st *srcTyping) listLit(e syntax.Expr, t types.Type) (value.Value, error) {
	lt, ok := t.Base().(*types.ListType)
	if !ok {
		return nil, st.wrong(e, t, "")
	}
	elems := e.(*syntax.ListLit).Elems
	out := &value.List{T: t, Elems: make([]value.Value, len(elems))}
	for i, x := range elems {
		v, err := st.inside(intSeg(int64(i)), func() (value.Value, error) { return st.expr(x, lt.Elem) })
		if err != nil {
			return nil, err
		}
		out.Elems[i] = v
	}
	return out, nil
}

// brace is `{…}` as what the expected type makes it: a record, a map or a table (SPEC §6.1).
func (st *srcTyping) brace(e syntax.Expr, t types.Type) (value.Value, error) {
	lit := e.(*syntax.BraceLit)
	if len(lit.Clauses) > 0 {
		return nil, st.wrong(e, t, detailNotLiteral)
	}
	switch b := t.Base().(type) {
	case *types.RecordType, *types.AppliedRecord, *types.CaseType:
		return st.recordItems(lit, t)
	case *types.MapType, *types.DepMapType:
		return st.mapItems(lit, t)
	case *types.TableType:
		return st.tableItems(lit, t, b.Elem)
	}
	return nil, st.wrong(e, t, "")
}

// typedLit is `case { … }`, a case of the expected variant by its bare name (SPEC §5.5).
func (st *srcTyping) typedLit(e syntax.Expr, t types.Type) (value.Value, error) {
	x := e.(*syntax.TypedLit)
	if len(x.Type.Parts) != 1 {
		return nil, st.wrong(e, t, detailNotContextual)
	}
	rec, ok := nameValue(x.Type.Parts[0].Name, t)
	c, isCase := rec.(*value.Record)
	if !ok || !isCase {
		return nil, st.wrong(e, t, "")
	}
	return st.recordItems(x.Lit, c.T)
}

// recordItems is a record literal: `name: value` items only, each field at most once.
func (st *srcTyping) recordItems(lit *syntax.BraceLit, t types.Type) (value.Value, error) {
	fields := fieldsOf(t)
	rec := &value.Record{T: t, Fields: make([]value.Value, len(fields)), Set: make([]bool, len(fields))}
	for _, it := range lit.Items {
		fi, ok := it.(*syntax.FieldItem)
		if !ok {
			return nil, st.wrong(it, t, detailNotLiteral)
		}
		i := fieldIndex(fields, fi.Name.Name)
		if detail := fieldRefusal(fields, i, fi.Name.Name); detail != "" {
			return nil, st.wrong(fi, t, detail)
		}
		if rec.Set[i] {
			return nil, st.wrong(fi, t, detailTwice+fi.Name.Name)
		}
		v, err := st.inside(Seg{Kind: SegField, Name: fi.Name.Name}, func() (value.Value, error) { return st.expr(fi.Value, fields[i].Type) })
		if err != nil {
			return nil, err
		}
		rec.Fields[i], rec.Set[i] = v, true
	}
	return rec, nil
}

// mapItems is a map literal; a bare name keys only an enum or a ref key type (SPEC §6.1).
func (st *srcTyping) mapItems(lit *syntax.BraceLit, t types.Type) (value.Value, error) {
	kt, vt, _ := mapTypes(t)
	out := &value.Map{T: t}
	for i, it := range lit.Items {
		k, x, err := st.mapKey(it, kt, i)
		if err != nil {
			return nil, err
		}
		v, err := st.inside(keySeg(k, kt), func() (value.Value, error) { return st.expr(x, vt) })
		if err != nil {
			return nil, err
		}
		out.Keys, out.Vals = append(out.Keys, k), append(out.Vals, v)
	}
	return out, nil
}

// mapKey is item i's key as a key of kt, and its value expression.
func (st *srcTyping) mapKey(it syntax.BraceItem, kt types.Type, i int) (value.Value, syntax.Expr, error) {
	switch x := it.(type) {
	case *syntax.MapItem:
		k, err := st.inside(Seg{Kind: SegPos, Pos: i}, func() (value.Value, error) { return st.expr(x.Key, kt) })
		return k, x.Value, err
	case *syntax.FieldItem:
		if k, ok := keyByName(x.Name.Name, kt); ok {
			return k, x.Value, nil
		}
		return nil, nil, st.wrong(x.Name, kt, detailNameKey)
	}
	return nil, nil, st.wrong(it, kt, detailNotLiteral)
}

// keyByName is a bare name as a map key: of an enum, a Kind or a ref key type only.
func keyByName(name string, kt types.Type) (value.Value, bool) {
	switch b := kt.Base().(type) {
	case *types.EnumType, *types.VariantKindType, *types.RefType:
		return nameValue(name, kt)
	case *types.LitUnionType:
		return keyByName(name, b.Of)
	}
	return nil, false
}

// tableItems is a table literal: `[retired] key { … }` entries, without annotations.
func (st *srcTyping) tableItems(lit *syntax.BraceLit, t, elem types.Type) (value.Value, error) {
	out := &value.Table{T: t}
	for _, it := range lit.Items {
		ent, ok := it.(*syntax.EntryItem)
		if !ok || len(ent.Annotations) > 0 {
			return nil, st.wrong(it, t, detailNotLiteral)
		}
		key := ent.Key.Name
		v, err := st.inside(entrySeg(value.Key{S: key}), func() (value.Value, error) { return st.recordItems(ent.Value, elem) })
		if err != nil {
			return nil, err
		}
		rec := v.(*value.Record)
		rec.Ident = &value.Identity{Key: value.Key{S: key}, Retired: ent.Mods != nil && ent.Mods.Retired.Valid()}
		out.Entries = append(out.Entries, rec)
	}
	return out, nil
}
