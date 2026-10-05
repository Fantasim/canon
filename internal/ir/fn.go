package ir

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// FnKind is how an export fn is emitted (SPEC §9.4).
type FnKind uint8

// ExportFn is an export fn, in Record.Methods, Case.Methods or Package.Fns: a method keeps Instances, a package fn Value or Table, a translated one Body onwards; File is its source file in Package.Dir (CODEGEN.md §2.5 T3), Order its rank among the package's export fns in declaration order (CONFORMANCE.md §7.2).
type ExportFn struct {
	Name, Doc   string
	File        string
	Order       int
	Kind        FnKind
	Params      []*Param
	Result      TypeRef
	ResultRange *types.Bound
	Instances   []*Instance // one per receiver, in traversal order (EVL-02)
	Value       value.Value
	Table       *LookupTable
	Domains     [][]value.Value // a lookup's parameter domains in CODEGEN.md §5.10 order, whatever receivers exist (a ref's: its table's entries); nil when one is not finite (log-2026-10-06 "U5 review FAIL" 2)
	Body        PExpr
	Reads       []*Read
	Vectors     []*Vector
	Err         error // ErrInternal: a translated fn stage E could not translate, or a stored fn whose results met their receiver past DECISIONS 284, with no error reported
	Go, Cpp, TS NameOptions
}

// Param is a parameter; Range is its range refinement, checked on entry of a translation.
type Param struct {
	Name  string
	Type  TypeRef
	Range *types.Bound
}

// Instance is a method's precomputed result for one receiver.
type Instance struct {
	Recv   *value.Record
	Result value.Value
	Table  *LookupTable
}

// LookupTable is a dense result table in domain order, the first parameter varying slowest.
type LookupTable struct {
	Domains [][]value.Value
	Cells   []value.Value
}

// Read is a path of self a translated method reads (fields, or one precomputed method of self), Name its Path joined with `_` (CONFORMANCE.md §2.3); Type excludes the `?` Optional keeps.
type Read struct {
	Name     string
	Path     []string
	Type     TypeRef
	Optional bool
}

// Vector is a conformance vector (CNF-01): a result or an error code, and TypeScript's.
type Vector struct {
	Recv   []value.Value
	Args   []value.Value
	Want   value.Value
	Code   diag.Code
	TSWant value.Value
	TSCode diag.Code
}
