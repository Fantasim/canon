package edit

import (
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// textKeyValue is a key written as text, of key type kt; nil when kt's keys are not text. A
// path key reads an enum's text as a Canon name, then as a wire value (API.md E25, P1, P2); a
// Key is never an enum member (V4).
func textKeyValue(s string, kt types.Type, asPath bool) value.Value {
	switch b := kt.Base().(type) {
	case types.Basic:
		if b.K == types.String {
			return &value.Str{V: s, T: kt}
		}
	case *types.EnumType:
		if i := namedIndex(b.Members, s, memberNaming); asPath && i >= 0 {
			return &value.Member{Enum: b, Index: i}
		}
	case *types.VariantKindType:
		if i := namedIndex(b.Variant.Cases, s, caseNaming); asPath && i >= 0 {
			return &value.CaseKind{T: b, Index: i}
		}
	case *types.RefType:
		if k := textKeyValue(s, refKeyType(b), asPath); k != nil {
			return &value.Ref{T: kt, Key: keyOfValue(k)}
		}
	case *types.LitUnionType:
		if slices.Contains(b.Literals, s) {
			return &value.Str{V: s, T: kt}
		}
		return textKeyValue(s, b.Of, asPath)
	}
	return nil
}

// intKeyValue is an integer key of key type kt; nil when kt's keys are not integers.
func intKeyValue(n int64, kt types.Type) value.Value {
	switch b := kt.Base().(type) {
	case types.Basic:
		if b.K == types.Int {
			return &value.Int{V: n, T: kt}
		}
	case *types.RefType:
		if k := intKeyValue(n, refKeyType(b)); k != nil {
			return &value.Ref{T: kt, Key: keyOfValue(k)}
		}
	case *types.LitUnionType:
		return intKeyValue(n, b.Of)
	}
	return nil
}

func memberNaming(m *types.Member) (name, wire string) { return m.Name, m.Wire }

func caseNaming(c *types.CaseType) (name, wire string) { return c.Name, c.Wire }

// namedIndex is the index of the item named s by its Canon name, else by its wire value; -1 for none.
func namedIndex[T any](items []T, s string, naming func(T) (name, wire string)) int {
	if i := slices.IndexFunc(items, func(it T) bool { n, _ := naming(it); return n == s }); i >= 0 {
		return i
	}
	return slices.IndexFunc(items, func(it T) bool { _, w := naming(it); return w == s })
}

// list is a List as a plain or keyed list of t.
func (tc *typing) list(elems List, t types.Type) (value.Value, error) {
	lt, ok := t.Base().(*types.ListType)
	if !ok {
		return nil, tc.mismatch(elems, t, "")
	}
	out := &value.List{T: t, Elems: make([]value.Value, len(elems))}
	for i, e := range elems {
		v, err := tc.inside(intSeg(int64(i)), func() (value.Value, error) { return tc.value(e, lt.Elem) })
		if err != nil {
			return nil, err
		}
		out.Elems[i] = v
	}
	return out, nil
}

// obj is an Obj as a record, an applied record or the fields of the case t is.
func (tc *typing) obj(obj Obj, t types.Type) (value.Value, error) {
	switch t.Base().(type) {
	case *types.RecordType, *types.AppliedRecord, *types.CaseType:
		return tc.record(obj, t, fieldsOf(t))
	}
	return nil, tc.mismatch(obj, t, "")
}

// caseLit is a Case as a case of the variant t, or as the case t is.
func (tc *typing) caseLit(c Case, t types.Type) (value.Value, error) {
	var ct *types.CaseType
	switch b := t.Base().(type) {
	case *types.VariantType:
		if i := slices.IndexFunc(b.Cases, func(x *types.CaseType) bool { return x.Name == c.Name }); i >= 0 {
			ct = b.Cases[i]
		}
	case *types.CaseType:
		if b.Name == c.Name {
			ct = b
		}
	}
	if ct == nil {
		return nil, tc.mismatch(c, t, "")
	}
	return tc.record(c.Fields, ct, ct.Fields)
}

// record types the fields obj writes; a field it leaves out is nil and not Set (API.md V3).
func (tc *typing) record(obj Obj, t types.Type, fields []*types.Field) (value.Value, error) {
	rec := &value.Record{T: t, Fields: make([]value.Value, len(fields)), Set: make([]bool, len(fields))}
	for _, name := range slices.Sorted(maps.Keys(obj)) {
		i := fieldIndex(fields, name)
		if detail := fieldRefusal(fields, i, name); detail != "" {
			return nil, tc.mismatch(Obj(nil), t, detail)
		}
		v, err := tc.inside(Seg{Kind: SegField, Name: name}, func() (value.Value, error) { return tc.value(obj[name], fields[i].Type) })
		if err != nil {
			return nil, err
		}
		rec.Fields[i], rec.Set[i] = v, true
	}
	return rec, nil
}

// fieldRefusal is why field i, written as name, cannot be given: none such, or an input
// field, which has no value (reason `input`); "" when it can.
func fieldRefusal(fields []*types.Field, i int, name string) string {
	switch {
	case i < 0:
		return detailField + name
	case fields[i].Input != nil:
		return detailInput + name
	}
	return ""
}

// mapLit is a Map as a map of t, a dependent map's keys being refs (TYPES.md §11.5).
func (tc *typing) mapLit(entries Map, t types.Type) (value.Value, error) {
	kt, vt, ok := mapTypes(t)
	if !ok {
		return nil, tc.mismatch(entries, t, "")
	}
	out := &value.Map{T: t, Keys: make([]value.Value, len(entries)), Vals: make([]value.Value, len(entries))}
	for i, kv := range entries {
		k, err := tc.inside(Seg{Kind: SegPos, Pos: i}, func() (value.Value, error) { return tc.key(kv.Key, kt) })
		if err != nil {
			return nil, err
		}
		v, err := tc.inside(keySeg(k, kt), func() (value.Value, error) { return tc.value(kv.Value, vt) })
		if err != nil {
			return nil, err
		}
		out.Keys[i], out.Vals[i] = k, v
	}
	return out, nil
}

// inside runs typeIt one segment deeper, for the detail of a refusal below.
func (tc *typing) inside(seg Seg, typeIt func() (value.Value, error)) (value.Value, error) {
	tc.at = append(tc.at, seg)
	defer func() { tc.at = tc.at[:len(tc.at)-1] }()
	return typeIt()
}

// mismatch is the V1 refusal of lit where t is expected, at the current place in the value.
func (tc *typing) mismatch(lit Lit, t types.Type, detail string) *ValueError {
	return tc.refuse(t, describe(lit), detail)
}

// refuse is a ValueError: what was expected and given, where in the value, and why.
func (tc *typing) refuse(t types.Type, got, detail string) *ValueError {
	e := &ValueError{Expected: t.String(), Got: got, Detail: detail}
	if len(tc.at) > 0 {
		where := detailAt + Path{Segs: tc.at}.String()
		if detail != "" {
			where += detailSep + detail
		}
		e.Detail = where
	}
	return e
}
