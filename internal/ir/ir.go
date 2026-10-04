package ir

import (
	"regexp"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Package is the emit IR of one Canon package, every list in declaration order (CODEGEN.md §2.7).
type Package struct {
	Name    string // the Canon package path
	Dir     string
	Doc     string
	Imports []*PackageRef
	Types   []Type
	Consts  []*Const
	Values  []*Value
	Fns     []*ExportFn
	TextFns []*ExportFn
	Defines []*DefineTable
	Emits   []*Emit
}

// DefineTable is a load.defines table a ref targets, its entries sorted by name (CODEGEN.md §5.8).
type DefineTable struct {
	Pkg, Value string
	Names      []string
	Values     []int64
}

// PackageRef is an imported package this one references, with its emits (EMT-06).
type PackageRef struct {
	Name, Dir string
	Emits     []*Emit
}

// Type is a named IR type, of this package or another: *Record, *Enum, *Variant, *Dependent.
type Type interface {
	QName() string
}

// Record is a record type; a parameterized record is one erased class (CODEGEN.md §5.7).
type Record struct {
	Pkg, Name, Doc string
	Params         int
	Fields         []*Field
	Methods        []*ExportFn
	Cpp            CppOptions
	Go, TS         NameOptions
}

func (r *Record) QName() string { return r.Pkg + qnameSep + r.Name }

// Field is a field of a record or case with its wire mapping; Type excludes its outer `?`.
type Field struct {
	Name, Doc  string
	WirePath   []string // empty for an inline or pairs field
	Type       TypeRef
	Optional   bool
	NoneWire   []byte // compact JSON of @json(none:); nil is null
	Unit       types.Unit
	Enc        types.Enc
	Inline     bool
	Pairs      *types.Pairs
	Default    value.Value // a constant default
	Computed   bool        // the default reads earlier fields or parameters
	Deprecated bool
	Stable     bool
	Input      *types.Input
	Range      *types.Bound     // an input's own refinements, checked at run time
	Patterns   []*regexp.Regexp // idem: every distinct pattern of its alias chain, innermost first in alias-chain order (TYPES.md §7.4; DECISIONS 225)
	Cpp        CppFieldOptions
	Go, TS     NameOptions
	BigInt     bool
}

// Enum is an enum type (CODEGEN.md §5.2); Codes is nil without @codes.
type Enum struct {
	Pkg, Name, Doc string
	Members        []*EnumMember
	Codes          *TypeRef
	JSONCodes      bool
	Ordered        bool
	CppDefines     string
	Go, Cpp, TS    NameOptions
}

func (e *Enum) QName() string { return e.Pkg + qnameSep + e.Name }

// EnumMember is a member, retired ones included.
type EnumMember struct {
	Name, Wire, Doc string
	Index           int
	Code            int64
	Retired         bool
	Deprecated      bool
	Go, Cpp, TS     NameOptions
}

// Variant is a variant type (CODEGEN.md §5.5).
type Variant struct {
	Pkg, Name, Doc, Tag string
	Cases               []*Case
	Go, Cpp, TS         NameOptions
}

func (v *Variant) QName() string { return v.Pkg + qnameSep + v.Name }

// Case is a case of a variant, retired ones included.
type Case struct {
	Name, Wire, Doc string
	Fields          []*Field
	Methods         []*ExportFn
	Retired         bool
	Cpp             CppCaseOptions
	Go, TS          NameOptions
}

// Dependent is a dependent type, emitted as the union of its branches (DEP-03). ByMember
// maps each discriminant member (false, true for Bool) to its branch, or NoBranch for Never.
type Dependent struct {
	Pkg, Name, Doc string
	Params         int
	DiscParam      int
	DiscPath       []string // wire path from the parameter's value to the discriminant
	Disc           *TypeRef
	Branches       []*Branch
	ByMember       []int
	Go, Cpp, TS    NameOptions
}

func (d *Dependent) QName() string { return d.Pkg + qnameSep + d.Name }

// Branch is a non-Never arm, named by its first pattern.
type Branch struct {
	Name    string
	Members []int
	Type    TypeRef
}

// Const is a public constant.
type Const struct {
	Name, Doc   string
	Type        TypeRef
	V           value.Value
	Go, Cpp, TS NameOptions
}

// Value is an emitted value, evaluated and verified; IDs are its table keys (EMT-04).
type Value struct {
	Name, Doc   string
	Type        TypeRef
	V           value.Value
	Reload      bool
	IDs         []string
	Schema      string // the $schema identifier (FINGERPRINT.md §2)
	Go, Cpp, TS NameOptions
}

// Emit is one `emit` declaration with its typed options and its output resolved in stage E:
// a generator never reads the project or its roots, and computes the relative includes and
// imports between two emits from their Dirs.
type Emit struct {
	Target    Target
	Out       string // as written ("@sovcommon/teamboard"), for messages only
	From      string
	Dir       string // output directory (ts: its file's), project-relative by the declared roots
	FileName  string // a ts emit's file name in Dir; "" for the other targets
	GoImport  string // a go emit's import path of Dir; "" for the other targets
	Mode      Mode
	Values    []string
	GoPackage string
	Namespace string
}

// File is one output file of a generator; Path is relative to its emit's Dir.
type File struct {
	Path    string
	Content []byte
}

// Generator is a code generator: a pure function of the IR and one of its emits.
type Generator func(p *Package, e *Emit) ([]File, error)
