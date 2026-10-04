package types

// ListType is [Elem]; with KeyedBy set it is TYPES.md's keyed list (decision 46).
type ListType struct {
	Elem    Type
	KeyedBy *Field // the key field of Elem, a record
}

func (l *ListType) Kind() Kind { return List }

func (l *ListType) Underlying() Type { return l }

func (l *ListType) Base() Type { return l }

// MapType is {Key: Value}; maps keep insertion order (TYPES.md §9.2).
type MapType struct {
	Key, Value Type
}

func (m *MapType) Kind() Kind { return Map }

func (m *MapType) Underlying() Type { return m }

func (m *MapType) Base() Type { return m }

// DepMapType is {Binder in Coll: Value}, keyed by refs into Coll (TYPES.md §11.5).
type DepMapType struct {
	Binder string
	Coll   *Collection
	Value  Type
}

func (d *DepMapType) Kind() Kind { return DepMap }

func (d *DepMapType) Underlying() Type { return d }

func (d *DepMapType) Base() Type { return d }

// TableType is `table Elem` or `stable table Elem`; Elem is a record type (TYPES.md §9.3).
type TableType struct {
	Elem   Type
	Stable bool
}

func (t *TableType) Kind() Kind { return Table }

func (t *TableType) Underlying() Type { return t }

func (t *TableType) Base() Type { return t }

// RefType is `ref T`: the key of an entry of Target (TYPES.md §10).
type RefType struct {
	Target *Collection
}

func (r *RefType) Kind() Kind { return Ref }

func (r *RefType) Underlying() Type { return r }

func (r *RefType) Base() Type { return r }

// CollKind says whether a Collection is a let, a field of an enclosing record, or defines.
type CollKind uint8

// Collection is a table or keyed list a ref can target (RES-03). The checker interns them:
// two refs have the same target exactly when their Target pointers are equal.
type Collection struct {
	Kind      CollKind
	Pkg, Name string
	Owner     *RecordType // CollField: the record whose field holds it
	FieldPath []string
	Elem      Type
	KeyedBy   *Field // nil for a table
	Local     bool
}

// OptionalType is Elem?; Elem is never an OptionalType (TYPES.md §2).
type OptionalType struct {
	Elem Type
}

func (o *OptionalType) Kind() Kind { return Optional }

func (o *OptionalType) Underlying() Type { return o }

func (o *OptionalType) Base() Type { return o }

// LitUnionType is `Of | "lit" | …` (TYP-09); Literals keep source order.
type LitUnionType struct {
	Of       Type
	Literals []string
}

func (u *LitUnionType) Kind() Kind { return LitUnion }

func (u *LitUnionType) Underlying() Type { return u }

func (u *LitUnionType) Base() Type { return u }

// FuncType is `fn(Params) -> Result` (GRM-15).
type FuncType struct {
	Params []Type
	Result Type
}

func (f *FuncType) Kind() Kind { return Func }

func (f *FuncType) Underlying() Type { return f }

func (f *FuncType) Base() Type { return f }

// PairType is the unwritable Pair(A, B) of enumerate, zip, pairs and map iteration.
type PairType struct {
	A, B Type
}

func (p *PairType) Kind() Kind { return Pair }

func (p *PairType) Underlying() Type { return p }

func (p *PairType) Base() Type { return p }

// Infinite reports a function parameter type that has no finite key set: a ref into a keyed list or a `local let` table, or anything but Bool, an enum or a ref (CODEGEN.md §5.10, WIRE.md §5.11, DECISIONS 296).
func Infinite(t Type) bool {
	if r, ok := t.Base().(*RefType); ok {
		return r.Target == nil || r.Target.KeyedBy != nil || r.Target.Local && r.Target.Kind == CollLet
	}
	k := t.Base().Kind()
	return k != Bool && k != Enum
}
