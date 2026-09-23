package ir

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// FnKind is how an export fn is emitted (SPEC §9.4).
type FnKind uint8

// ExportFn is an export fn, in Record.Methods, Case.Methods or Package.Fns. A method keeps
// Instances; a package-level fn keeps Value or Table; a translated one keeps Body onwards.
type ExportFn struct {
	Name, Doc   string
	Kind        FnKind
	Params      []*Param
	Result      TypeRef
	ResultRange *types.Bound
	Instances   []*Instance // one per receiver, in traversal order (EVL-02)
	Value       value.Value
	Table       *LookupTable
	Body        PExpr
	Reads       []*Read
	Vectors     []*Vector
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

// Read is a path of self a translated method reads; Name joins Path with `_`.
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
