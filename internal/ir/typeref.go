package ir

import "github.com/fantasim/canonlang/internal/types"

// TypeRef is a type as generators and the fingerprint see it (GEN-02). Named is the IR type
// (a *Dependent for TypeApp); Elem is also a map's value and a union's base; Key a ref's key.
type TypeRef struct {
	Kind     types.Kind
	Bits     int
	Signed   bool
	Named    Type
	Case     *Case
	Elem     *TypeRef
	Key      *TypeRef
	KeyedBy  *KeyField
	Ref      *RefTarget
	Literals []string
	Args     []*Source
}

// KeyField is the key field of a keyed list.
type KeyField struct {
	Name     string
	WirePath []string
}

// RefTarget is the collection a ref targets (CG-03); Value is "" for an enclosing field.
type RefTarget struct {
	Coll   types.CollKind
	Pkg    string
	Value  string
	Path   []string
	Elem   Type
	Keyed  bool
	Local  bool
	BigInt bool // the key field is @ts(bigint): TypeScript keys the ref as a bigint (DECISIONS 278)
}

// Source is where a type argument is read from (FINGERPRINT.md §4.4).
type Source struct {
	From     types.ArgSource
	Param    int
	WirePath []string
}
