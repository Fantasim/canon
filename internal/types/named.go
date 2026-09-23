package types

import "github.com/fantasim/canonlang/internal/syntax"

// EnumType is a declared enum (TYPES.md §8.1).
type EnumType struct {
	Pkg, Name, Doc string
	Ordered        bool
	Codes          *Basic // the @codes type; nil without @codes
	WireCodes      bool   // @json(codes): the wire holds the code (WIR-04)
	Members        []*Member
	Decl           *syntax.EnumDecl
}

func (e *EnumType) Kind() Kind { return Enum }

func (e *EnumType) String() string { return qualify(e.Pkg, e.Name) }

func (e *EnumType) Underlying() Type { return e }

func (e *EnumType) Base() Type { return e }

// Member is an enum member; Index counts retired members too.
type Member struct {
	Name, Wire, Doc string
	Index           int
	Code            int64
	HasCode         bool
	Retired         bool
	Deprecated      *Deprecation
}

// Deprecation is a @deprecated annotation; Why is its optional reason (TYP-22).
type Deprecation struct {
	Why string
}

// RecordType is a declared record, possibly with value parameters (TYPES.md §11.1).
type RecordType struct {
	Pkg, Name, Doc string
	Params         []*Param
	Fields         []*Field
	Methods        []*Method
	Checks         []*syntax.CheckDecl
	Decl           *syntax.RecordDecl
	define         bool
}

func (r *RecordType) Kind() Kind {
	if r.define {
		return Define
	}
	return Record
}

func (r *RecordType) String() string { return qualify(r.Pkg, r.Name) }

func (r *RecordType) Underlying() Type { return r }

func (r *RecordType) Base() Type { return r }

// Param is a value parameter of a record or a type function: a record type or a ref.
type Param struct {
	Name  string
	Index int
	Type  Type
}

// Field is a field of a record or case, with its wire mapping resolved (WIRE.md §4, §5).
type Field struct {
	Name        string
	Index       int
	Type        Type
	Wire        string   // the key; with @json(path:), the last segment
	WirePath    []string // the whole key path; one element without @json(path:)
	Inline      bool     // @json(inline) on a variant field
	NoneWire    []byte   // compact JSON of @json(none:); nil means null
	Unit        Unit
	Enc         Enc
	Pairs       *Pairs
	DependsOn   []int  // earlier fields its type uses (TYPES.md §11)
	Input       *Input // a runtime input: absent from values (TYP-17)
	Stable      bool   // @stable (LCK-02)
	Deprecated  *Deprecation
	Doc         string
	Default     syntax.Expr // nil when the field has no default
	Annotations []*syntax.Annotation
}

// Pairs is @json(pairs: [k, v]): two slot key templates and the slot count (WIRE.md §5.14).
type Pairs struct {
	Keys  [pairKeys]string
	Slots int
}

// Input is `input T from env NAME` (EVALUATION.md §11).
type Input struct {
	Env string
}

// Method is a fn of a record or case body; Type excludes self.
type Method struct {
	Name   string
	Export bool
	Type   *FuncType
}

// VariantType is a declared variant; Tag is the wire tag key, "kind" by default.
type VariantType struct {
	Pkg, Name, Doc, Tag string
	Cases               []*CaseType
	Decl                *syntax.VariantDecl
}

func (v *VariantType) Kind() Kind { return Variant }

func (v *VariantType) String() string { return qualify(v.Pkg, v.Name) }

func (v *VariantType) Underlying() Type { return v }

func (v *VariantType) Base() Type { return v }

// CaseType is one case of a variant, and the type of a value narrowed to it (TYPES.md §8.2).
type CaseType struct {
	Variant         *VariantType
	Name, Wire, Doc string
	Index           int
	Fields          []*Field
	Methods         []*Method
	Checks          []*syntax.CheckDecl
	Retired         bool
	Deprecated      *Deprecation
}

func (c *CaseType) Kind() Kind { return Case }

func (c *CaseType) String() string { return c.Variant.String() + textDot + c.Name }

func (c *CaseType) Underlying() Type { return c }

func (c *CaseType) Base() Type { return c }

// VariantKindType is the type of v.kind: an enum whose members are the cases (TYPES.md §8.3).
type VariantKindType struct {
	Variant *VariantType
}

func (k *VariantKindType) Kind() Kind { return VariantKind }

func (k *VariantKindType) String() string { return textKind + k.Variant.String() + ")" }

func (k *VariantKindType) Underlying() Type { return k }

func (k *VariantKindType) Base() Type { return k }
