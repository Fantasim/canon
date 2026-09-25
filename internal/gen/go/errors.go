package gogen

import (
	"errors"
	"fmt"

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
	unionFormat          = "%s: a literal union over a ref or dependent type, not generated yet"
	unionMalformedFormat = "%s: a literal union whose other arm is not string-wired"
	inlineFormat         = "%s: an optional or non-variant @json(inline) field, in data mode"
	foldFormat           = "%s: an inline variant key equal to another key of its parent but for letter case, in data mode"
	noneMarkerFormat     = "%s: the none marker %s, a non-empty object or array, in data mode"
	dataCollisionFormat  = "%s declares %s twice in data mode"
)

// Dependent types (CODEGEN.md §5.6): what the ir plan does not resolve yet is ErrUnsupported, each its own text (decision 194).
const (
	dependentDiscKindFormat     = "%s: a dependent type whose discriminant is not an enum, in data mode"
	dependentDiscOptionalFormat = "%s: a dependent type whose discriminant field is optional, in data mode"
	dependentArgFormat          = "%s: a dependent type argument from other than one earlier field, in data mode"
	dependentScrutineeFormat    = "%s: a dependent type whose match reads further than its parameter, in data mode"
	dependentNestedFormat       = "%s: a dependent type inside a list, optional or pairs field, in data mode"
	dependentValueFormat        = "%s: a ref into a load.defines table's define value, in a dependent type, not generated yet"
	dependentLiteralFormat      = "%s: a baked or embedded literal of a dependent type, not generated yet"
)

// The kinds stage E refuses (E8019) where gen/go writes a type, a value literal, a value it reads or a constant: meeting one there is ErrMalformed (CODEGEN.md §5.6).
var (
	typeRefused  = map[types.Kind]bool{types.Optional: true, types.Table: true}
	readRefused  = map[types.Kind]bool{types.Optional: true, types.Table: true, types.Map: true, types.DepMap: true, types.Case: true}
	constRefused = map[types.Kind]bool{types.Record: true, types.Variant: true, types.Case: true}
)
