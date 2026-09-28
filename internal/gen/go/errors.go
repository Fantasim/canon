package gogen

import (
	"errors"
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

var (
	// ErrTarget is an emit whose target is not go.
	ErrTarget = errors.New("gogen: not a go emit")
	// ErrUnsupported is a mode or construct this generator does not emit yet.
	ErrUnsupported = errors.New("gogen: not supported")
	// ErrMalformed is an IR that stage E should not have produced.
	ErrMalformed = errors.New("gogen: malformed IR")
	// errNameCollision is two generated names equal in one Go scope: stage E reports it first (malformed IR).
	errNameCollision = fmt.Errorf("%w: generated name collision", ErrMalformed)
	errFormat        = errors.New("gogen: generated Go does not format")
)

// DetailError names, in Subject and Index, what a sentinel refused: a caller compares fields, never the message (go.md §3).
type DetailError struct {
	Subject string
	Index   int

	err     error
	message string
}

func newDetail(err error, subject, format string, args ...any) *DetailError {
	return &DetailError{Subject: subject, Index: -1, err: err, message: fmt.Sprintf(format, args...)}
}

func (e *DetailError) Error() string { return e.err.Error() + ": " + e.message }

func (e *DetailError) Unwrap() error { return e.err }

// What data mode refuses (ErrUnsupported, decision 124), finds malformed (ErrMalformed: stage E refuses it first) and the collision its name check finds.
const (
	packageFnFormat      = "package-level export fn %s in data mode (CODEGEN.md §5.10)"
	lookupParamFormat    = "%s: a finite parameter that is not an enum or a Bool, in data mode (CODEGEN.md §5.10)"
	lookupRefFormat      = "%s: a finite-parameter method whose result holds a ref resolved at load, in data mode"
	dataValueFormat      = "data value %s that is not a table, a keyed list or a record (CODEGEN.md §2.2)"
	foreignClassFormat   = "a record or variant of another package, %s, read by a loader at %s (its decoder is unexported there)"
	unionFormat          = "%s: a literal union over a ref, not generated yet"
	unionMalformedFormat = "%s: a literal union whose other arm is not string-wired"
	inlineFormat         = "%s: an optional or non-variant @json(inline) field, in data mode"
	foldFormat           = "%s: an inline variant key equal to another key of its parent but for letter case, in data mode"
	noneMarkerFormat     = "%s: the none marker %s, a non-empty object or array, in data mode"
	dataCollisionFormat  = "%s declares %s twice in data mode"
)

// The causes of the dependent-type refusals, each ErrMalformed: a caller tells them apart with errors.Is (go.md §3).
var (
	errDependentNoDisc   = fmt.Errorf("%w", ErrMalformed)
	errDependentDisc     = fmt.Errorf("%w", ErrMalformed)
	errDependentForeign  = fmt.Errorf("%w", ErrMalformed)
	errDependentNested   = fmt.Errorf("%w", ErrMalformed)
	errDependentValue    = fmt.Errorf("%w", ErrMalformed)
	errDependentNoBranch = fmt.Errorf("%w", ErrMalformed)
	errDependentNever    = fmt.Errorf("%w", ErrMalformed)
	errDependentUnion    = fmt.Errorf("%w", ErrMalformed)
)

// Dependent types (CODEGEN.md §5.6): what stage E refuses first (E8019 DependentType; E8012 for a define ref), so meeting one is ErrMalformed, each its own text (decision 194).
const (
	dependentNoDiscFormat   = "%s: a dependent type without its Bool or enum discriminant"
	dependentDiscFormat     = "%s: a dependent value whose discriminant is not read from earlier required fields of its record (ir.DiscFields)"
	dependentForeignFormat  = "%s: a dependent value of another package's type, whose decoder and fields are unexported there"
	dependentNestedFormat   = "%s: a dependent value outside a field of its record and that field's list elements"
	dependentValueFormat    = "%s: a ref into a load.defines table's define value, in a dependent type"
	dependentNoBranchFormat = "%s: a dependent type every arm of which is Never"
	dependentNeverFormat    = "%s: a dependent value whose discriminant selects a Never arm"
	dependentUnionFormat    = "%s: a literal union over a dependent type, in data mode"
)

// The kinds stage E refuses (E8019) where gen/go writes a type, a value literal, a value it reads or a constant: meeting one there is ErrMalformed (CODEGEN.md §5.6).
var (
	typeRefused  = map[types.Kind]bool{types.Optional: true, types.Table: true}
	readRefused  = map[types.Kind]bool{types.Optional: true, types.Table: true, types.Map: true, types.DepMap: true, types.Case: true}
	constRefused = map[types.Kind]bool{types.Record: true, types.Variant: true, types.Case: true}
)

// Messages of the generator's errors.
const (
	kindFormat      = "%s: a value of kind %s"
	unknownKind     = "an unknown kind"
	unknownMode     = "an unknown mode"
	defineRefFormat = "a ref into the load.defines table %s, whose define value getter and table are not generated yet"
	noKeyType       = "a ref without a key type"
	noEnumFormat    = "an enum type without its enum at %s"
)

// modeNames name the modes in messages (CODEGEN.md §2.1).
var modeNames = [...]string{ir.ModeNone: "none", ir.ModeBaked: "baked", ir.ModeEmbedded: "embedded", ir.ModeData: "data", ir.ModeTypes: "types"}

// kindNames name the type kinds in messages (TYPES.md §2).
var kindNames = [...]string{
	types.Bool: "Bool", types.Int: "Int", types.Float: "Float", types.String: "String",
	types.Duration: "Duration", types.Enum: "enum", types.Record: "record", types.Variant: "variant",
	types.Case: "case", types.VariantKind: "variant kind", types.Optional: "optional",
	types.List: "list", types.Map: "map", types.DepMap: "dependent map", types.Table: "table",
	types.Ref: "ref", types.LitUnion: "literal union", types.Never: "Never", types.Range: "Range",
	types.Func: "function", types.Pair: "pair", types.TypeApp: "dependent type",
	types.DepUnion: "dependent union", types.Define: "define", types.Any: "Any", types.None: "None",
	types.Error: "error",
}
