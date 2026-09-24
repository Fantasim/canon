package ir

import (
	"github.com/fantasim/canonlang/internal/types"
)

// refOf converts a type by the kind of its base (DECISIONS 26); a nil entry keeps only the kind.
var refOf [types.Error + 1]func(*stage, types.Type) TypeRef

func init() {
	for _, k := range [...]types.Kind{types.Bool, types.Int, types.Float, types.String, types.Duration} {
		refOf[k] = (*stage).basicRef
	}
	refOf[types.Enum], refOf[types.Record], refOf[types.Variant] = (*stage).enumRef, (*stage).recordRef, (*stage).variantRef
	refOf[types.Case], refOf[types.Optional], refOf[types.List] = (*stage).caseRef, (*stage).optionalRef, (*stage).listRef
	refOf[types.Map], refOf[types.DepMap], refOf[types.Table] = (*stage).mapRef, (*stage).depMapRef, (*stage).tableRef
	refOf[types.Ref], refOf[types.LitUnion], refOf[types.TypeApp] = (*stage).refRef, (*stage).unionRef, (*stage).appRef
}

// ref is t as generators and the fingerprint see it (GEN-02): aliases expanded, refinements dropped (FINGERPRINT.md §4.4).
func (s *stage) ref(t types.Type) TypeRef {
	if t == nil {
		return TypeRef{Kind: types.Error}
	}
	b := t.Base()
	k := b.Kind()
	if int(k) < len(refOf) && refOf[k] != nil {
		return refOf[k](s, b)
	}
	return TypeRef{Kind: k}
}

func (s *stage) refPtr(t types.Type) *TypeRef {
	r := s.ref(t)
	return &r
}

func (s *stage) basicRef(t types.Type) TypeRef {
	b, _ := t.(types.Basic)
	return TypeRef{Kind: b.K, Bits: b.Bits, Signed: b.Signed}
}

func (s *stage) enumRef(t types.Type) TypeRef {
	return TypeRef{Kind: types.Enum, Named: s.enum(t.(*types.EnumType))}
}

// recordRef is a record, or R(args) with its bindings (FINGERPRINT.md §4.4).
func (s *stage) recordRef(t types.Type) TypeRef {
	switch x := t.(type) {
	case *types.AppliedRecord:
		return TypeRef{Kind: types.Record, Named: s.record(x.Rec), Args: s.sources(x.Args)}
	case *types.RecordType:
		return TypeRef{Kind: types.Record, Named: s.record(x)}
	}
	return TypeRef{Kind: types.Error}
}

func (s *stage) variantRef(t types.Type) TypeRef {
	return TypeRef{Kind: types.Variant, Named: s.variant(t.(*types.VariantType))}
}

// caseRef is a type narrowed to one case: its variant, and the case.
func (s *stage) caseRef(t types.Type) TypeRef {
	c := t.(*types.CaseType)
	v := s.variant(c.Variant)
	return TypeRef{Kind: types.Case, Named: v, Case: v.Cases[c.Index]}
}

func (s *stage) optionalRef(t types.Type) TypeRef {
	return TypeRef{Kind: types.Optional, Elem: s.refPtr(t.(*types.OptionalType).Elem)}
}

// listRef is [T], or [T] keyed by f with f's wire path (FINGERPRINT.md §4.4).
func (s *stage) listRef(t types.Type) TypeRef {
	l := t.(*types.ListType)
	r := TypeRef{Kind: types.List, Elem: s.refPtr(l.Elem)}
	if l.KeyedBy != nil {
		r.KeyedBy = &KeyField{Name: l.KeyedBy.Name, WirePath: l.KeyedBy.WirePath}
	}
	return r
}

func (s *stage) mapRef(t types.Type) TypeRef {
	m := t.(*types.MapType)
	return TypeRef{Kind: types.Map, Key: s.refPtr(m.Key), Elem: s.refPtr(m.Value)}
}

// depMapRef is {e in c: T(e)}: its key a ref into c (decision 118), its value bound to `key`.
func (s *stage) depMapRef(t types.Type) TypeRef {
	d := t.(*types.DepMapType)
	key := s.collRef(d.Coll)
	return TypeRef{Kind: types.DepMap, Key: &key, Elem: s.refPtr(d.Value)}
}

func (s *stage) tableRef(t types.Type) TypeRef {
	return TypeRef{Kind: types.Table, Elem: s.refPtr(t.(*types.TableType).Elem)}
}

func (s *stage) refRef(t types.Type) TypeRef {
	return s.collRef(t.(*types.RefType).Target)
}

// collRef is `ref c`: its key type and its target (FINGERPRINT.md §4.4, CODEGEN.md §5.8).
func (s *stage) collRef(c *types.Collection) TypeRef {
	if c == nil {
		return TypeRef{Kind: types.Ref, Key: &TypeRef{Kind: types.Error}}
	}
	key := TypeRef{Kind: types.String}
	if c.KeyedBy != nil {
		key = s.ref(c.KeyedBy.Type)
	}
	target := &RefTarget{Coll: c.Kind, Pkg: c.Pkg, Path: c.FieldPath, Keyed: c.KeyedBy != nil, Local: c.Local}
	if c.Kind != types.CollField {
		target.Value = c.Name
	}
	if e := s.ref(c.Elem); e.Named != nil {
		target.Elem = e.Named
	}
	return TypeRef{Kind: types.Ref, Key: &key, Ref: target}
}

func (s *stage) unionRef(t types.Type) TypeRef {
	u := t.(*types.LitUnionType)
	return TypeRef{Kind: types.LitUnion, Elem: s.refPtr(u.Of), Literals: u.Literals}
}

// appRef is a dependent type F(args); a type function without a match is its body.
func (s *stage) appRef(t types.Type) TypeRef {
	a := t.(*types.TypeAppType)
	if a.Fn.Scrutinee == nil {
		return s.ref(a.Fn.Body)
	}
	return TypeRef{Kind: types.TypeApp, Named: s.dependent(a.Fn), Args: s.sources(a.Args)}
}

// sources is where each type argument is read from (FINGERPRINT.md §4.4): its wire path.
func (s *stage) sources(args []*types.Arg) []*Source {
	out := make([]*Source, len(args))
	for i, a := range args {
		src := &Source{From: a.Source, WirePath: wirePath(a.Path)}
		if a.Param != nil {
			src.Param = a.Param.Index
		}
		out[i] = src
	}
	return out
}

// wirePath joins the wire paths of a field path.
func wirePath(fields []*types.Field) []string {
	var out []string
	for _, f := range fields {
		out = append(out, f.WirePath...)
	}
	return out
}
