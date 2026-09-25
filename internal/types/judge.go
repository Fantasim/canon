package types

import "slices"

// Identical is static identity (TYPES.md §6.1); an application is its type function's union (TYP-18).
func Identical(a, b Type) bool {
	a, b = erasedApp(expandUnion(a.Base())), erasedApp(expandUnion(b.Base()))
	if a.Kind() != b.Kind() {
		return false
	}
	return identityOf(a.Kind())(a, b)
}

// Assignable is S ≤ E on types (TYPES.md §6.2); the rules about literals belong to the checker.
func Assignable(from, to Type) bool {
	f, t := from.Base(), expandUnion(to.Base())
	switch {
	case f.Kind() == Error || t.Kind() == Error || f.Kind() == Never:
		return true
	case f.Kind() == DepUnion || f.Kind() == TypeApp:
		return t.Kind() == Any || storesDependent(f, t) || Identical(f, t)
	case t.Kind() == Any || t.Kind() == DepUnion || t.Kind() == TypeApp:
		return true
	case Identical(f, t):
		return true
	case t.Kind() == Optional:
		return assignOptional(f, t.(*OptionalType))
	case f.Kind() == Optional || f.Kind() == None:
		return false
	case f.Kind() == Ref && t.Kind() != Ref && Identical(f.(*RefType).Target.Elem, t):
		return true
	}
	rule := assignRuleOf(t.Kind())
	return rule != nil && rule(f, t)
}

// storesDependent reports a dependent value stored as the same type function, optional or not, or in a literal union over it (TYPES.md §11.4).
func storesDependent(f, t Type) bool {
	if o, ok := t.(*OptionalType); ok {
		t = expandUnion(o.Elem.Base())
	}
	if u, ok := t.(*LitUnionType); ok {
		t = u.Of.Base()
	}
	fn := typeFuncOf(f)
	return fn != nil && fn == typeFuncOf(t)
}

// erasedApp is F(*) for an application F(args), arguments being erased statically; else t.
func erasedApp(t Type) Type {
	if app, ok := t.(*TypeAppType); ok {
		return &DepUnionType{Fn: app.Fn}
	}
	return t
}

// expandUnion is the literal union an application of a type function whose body is one stands for (TYPES.md §13.2).
func expandUnion(t Type) Type {
	u := unionBody(typeFuncOf(t))
	if u == nil || unionCycles(typeFuncOf(t)) {
		return t
	}
	return u
}

// unionBody is the literal union a type function's body is, nil for any other body.
func unionBody(fn *TypeFunc) *LitUnionType {
	if fn == nil || fn.Body == nil {
		return nil
	}
	u, _ := fn.Body.Base().(*LitUnionType)
	return u
}

// unionCycles reports a chain of union bodies, each over the next type function, that comes
// back to one already met (the checker reports it, E3021); expanding it would never end.
func unionCycles(fn *TypeFunc) bool {
	seen := map[*TypeFunc]bool{}
	for u := unionBody(fn); u != nil; u = unionBody(fn) {
		if seen[fn] {
			return true
		}
		seen[fn] = true
		fn = typeFuncOf(u.Of.Base())
	}
	return false
}

// Join is the least common type of two branches (TYPES.md §6.4, TYP-05).
func Join(a, b Type) (Type, bool) {
	a, b = a.Base(), b.Base()
	switch {
	case a.Kind() == Error:
		return b, true
	case b.Kind() == Error:
		return a, true
	case a.Kind() == None || b.Kind() == None || a.Kind() == Optional || b.Kind() == Optional:
		return joinOptional(a, b)
	case a.Kind() == List && b.Kind() == List:
		return joinLists(a.(*ListType), b.(*ListType))
	case a.Kind() == Map && b.Kind() == Map:
		return joinMaps(a.(*MapType), b.(*MapType))
	case Identical(a, b):
		return a, true
	}
	if t, ok := joinRef(a, b); ok {
		return t, true
	}
	if t, ok := joinRef(b, a); ok {
		return t, true
	}
	if va, vb := variantOf(a), variantOf(b); va != nil && va == vb {
		return va, true
	}
	return nil, false
}

// identityOf compares two types of kind k (TYPES.md §6.1).
func identityOf(k Kind) func(a, b Type) bool {
	switch k {
	case Enum, Variant, Case, Define:
		return samePointer
	case Record:
		return sameRecord
	case VariantKind:
		return sameVariantKind
	case Ref:
		return sameTarget
	case Optional:
		return sameOptional
	case List:
		return sameList
	case Map:
		return sameMap
	case DepMap:
		return sameDepMap
	case Table:
		return sameTable
	case LitUnion:
		return sameLitUnion
	case Func:
		return sameFunc
	case Pair:
		return samePair
	case TypeApp, DepUnion:
		return sameTypeFunc
	default:
		return sameKind
	}
}

func sameKind(Type, Type) bool { return true }

func samePointer(a, b Type) bool { return a == b }

func sameRecord(a, b Type) bool { return recordOf(a) == recordOf(b) }

// recordOf is the declaration of a record type, applied or not (TYP-18).
func recordOf(t Type) *RecordType {
	if a, ok := t.(*AppliedRecord); ok {
		return a.Rec
	}
	r, _ := t.(*RecordType)
	return r
}

func sameVariantKind(a, b Type) bool {
	return a.(*VariantKindType).Variant == b.(*VariantKindType).Variant
}

func sameTarget(a, b Type) bool { return a.(*RefType).Target == b.(*RefType).Target }

func sameOptional(a, b Type) bool {
	return Identical(a.(*OptionalType).Elem, b.(*OptionalType).Elem)
}

func sameList(a, b Type) bool {
	x, y := a.(*ListType), b.(*ListType)
	return x.KeyedBy == y.KeyedBy && Identical(x.Elem, y.Elem)
}

func sameMap(a, b Type) bool {
	x, y := a.(*MapType), b.(*MapType)
	return Identical(x.Key, y.Key) && Identical(x.Value, y.Value)
}

func sameDepMap(a, b Type) bool {
	x, y := a.(*DepMapType), b.(*DepMapType)
	return x.Coll == y.Coll && Identical(x.Value, y.Value)
}

func sameTable(a, b Type) bool { return Identical(a.(*TableType).Elem, b.(*TableType).Elem) }

func sameLitUnion(a, b Type) bool {
	x, y := a.(*LitUnionType), b.(*LitUnionType)
	if !Identical(x.Of, y.Of) || len(x.Literals) != len(y.Literals) {
		return false
	}
	for _, l := range x.Literals {
		if !slices.Contains(y.Literals, l) {
			return false
		}
	}
	return true
}

func sameFunc(a, b Type) bool {
	x, y := a.(*FuncType), b.(*FuncType)
	return slices.EqualFunc(x.Params, y.Params, Identical) && Identical(x.Result, y.Result)
}

func samePair(a, b Type) bool {
	x, y := a.(*PairType), b.(*PairType)
	return Identical(x.A, y.A) && Identical(x.B, y.B)
}

// sameTypeFunc compares dependent types by their type function: arguments are erased (TYP-18).
func sameTypeFunc(a, b Type) bool { return typeFuncOf(a) == typeFuncOf(b) }

// typeFuncOf is the type function of a dependent type, nil for any other type.
func typeFuncOf(t Type) *TypeFunc {
	switch x := t.(type) {
	case *DepUnionType:
		return x.Fn
	case *TypeAppType:
		return x.Fn
	default:
		return nil
	}
}

// assignRuleOf is the row of TYPES.md §6.2 for an expected type of kind k, or nil.
func assignRuleOf(k Kind) func(f, t Type) bool {
	switch k {
	case Ref:
		return assignToRef
	case Variant:
		return assignToVariant
	case List:
		return assignToList
	case Map:
		return assignToMap
	case DepMap:
		return assignToDepMap
	case Pair:
		return assignToPair
	case LitUnion:
		return assignToLitUnion
	case Func:
		return assignToFunc
	default:
		return nil
	}
}

func assignOptional(f Type, t *OptionalType) bool {
	switch {
	case f.Kind() == None:
		return true
	case f.Kind() == Optional:
		return Assignable(f.(*OptionalType).Elem, t.Elem)
	}
	return Assignable(f, t.Elem)
}

// assignToRef is T ≤ ref T (TYP-02); a ref of another collection never converts (E3002).
func assignToRef(f, t Type) bool {
	return f.Kind() != Ref && Identical(f, t.(*RefType).Target.Elem)
}

func assignToLitUnion(f, t Type) bool { return Assignable(f, t.(*LitUnionType).Of) }

func assignToVariant(f, t Type) bool {
	c, ok := f.(*CaseType)
	return ok && c.Variant == t
}

// assignToList is `[S] ≤ [T]`, a keyed list to a plain list, `[S] ≤ [T] keyed by f`
// element-wise, and `table T ≤ [T]` with entries keeping their identity.
func assignToList(f, t Type) bool {
	to := t.(*ListType)
	switch from := f.(type) {
	case *ListType:
		return Assignable(from.Elem, to.Elem)
	case *TableType:
		return to.KeyedBy == nil && Identical(from.Elem, to.Elem)
	}
	return false
}

func assignToMap(f, t Type) bool {
	from, ok := f.(*MapType)
	to := t.(*MapType)
	return ok && Assignable(from.Key, to.Key) && Assignable(from.Value, to.Value)
}

// assignToDepMap is `{K: V} ≤` a dependent map over `ref C` when `K ≤ ref C`; each value is
// checked at evaluation (E3802).
func assignToDepMap(f, t Type) bool {
	from, ok := f.(*MapType)
	return ok && Assignable(from.Key, &RefType{Target: t.(*DepMapType).Coll})
}

func assignToPair(f, t Type) bool {
	from, ok := f.(*PairType)
	to := t.(*PairType)
	return ok && Assignable(from.A, to.A) && Assignable(from.B, to.B)
}

func assignToFunc(f, t Type) bool {
	from, ok := f.(*FuncType)
	to := t.(*FuncType)
	return ok && slices.EqualFunc(from.Params, to.Params, Identical) && Assignable(from.Result, to.Result)
}

// joinOptional joins when either side may be none: None and T, or T? and T, give T?.
func joinOptional(a, b Type) (Type, bool) {
	x, y := optionalElem(a), optionalElem(b)
	switch {
	case x == nil && y == nil:
		return NoneType, true
	case x == nil:
		return &OptionalType{Elem: y.Base()}, true
	case y == nil:
		return &OptionalType{Elem: x.Base()}, true
	}
	j, ok := Join(x, y)
	if !ok {
		return nil, false
	}
	return &OptionalType{Elem: j}, true
}

// optionalElem is T for T? and T itself, nil for None.
func optionalElem(t Type) Type {
	switch t.Kind() {
	case None:
		return nil
	case Optional:
		return t.(*OptionalType).Elem
	default:
	}
	return t
}

func joinLists(a, b *ListType) (Type, bool) {
	j, ok := Join(a.Elem, b.Elem)
	if !ok {
		return nil, false
	}
	l := &ListType{Elem: j}
	if a.KeyedBy == b.KeyedBy {
		l.KeyedBy = a.KeyedBy
	}
	return l, true
}

func joinMaps(a, b *MapType) (Type, bool) {
	k, ok := Join(a.Key, b.Key)
	if !ok {
		return nil, false
	}
	v, ok := Join(a.Value, b.Value)
	if !ok {
		return nil, false
	}
	return &MapType{Key: k, Value: v}, true
}

// joinRef is `T ⊔ ref T = T` with a the ref.
func joinRef(a, b Type) (Type, bool) {
	r, ok := a.(*RefType)
	if !ok || b.Kind() == Ref || !Identical(r.Target.Elem, b) {
		return nil, false
	}
	return b, true
}

// variantOf is the variant of a variant or case type, else nil.
func variantOf(t Type) *VariantType {
	switch x := t.(type) {
	case *VariantType:
		return x
	case *CaseType:
		return x.Variant
	}
	return nil
}
