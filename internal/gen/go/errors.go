package gogen

import (
	"errors"
	"fmt"
)

var (
	// ErrTarget is an emit whose target is not go.
	ErrTarget = errors.New("gogen: not a go emit")
	// ErrUnsupported is a mode or construct this generator does not emit yet.
	ErrUnsupported = errors.New("gogen: not supported")
	// ErrMalformed is an IR that stage E should not have produced.
	ErrMalformed = errors.New("gogen: malformed IR")
	// ErrName is a generated name that is no Go identifier, or an unexported @go(name:).
	ErrName = errors.New("gogen: not a Go identifier")
	// ErrNameCollision is two generated names equal in one Go scope (CODEGEN.md §3.5).
	ErrNameCollision = errors.New("gogen: generated name collision")
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

// What data mode refuses (ErrUnsupported, decision 124) and the collision its name check finds.
const (
	packageFnFormat      = "package-level export fn %s in data mode (CODEGEN.md §5.10)"
	lookupParamFormat    = "%s: a finite parameter that is not an enum or a Bool, in data mode (CODEGEN.md §5.10)"
	lookupRefFormat      = "%s: a finite-parameter method whose result holds a ref resolved at load, in data mode"
	severalHoldersFormat = "%s: a ref that could resolve, in a record several emitted values hold"
	dataValueFormat      = "data value %s that is not a table, a keyed list or a record (CODEGEN.md §2.2)"
	foreignClassFormat   = "a record or variant of another package, %s, read by a loader at %s (its decoder is unexported there)"
	unionFormat          = "%s: a literal union whose other arm is not written as a string, in data mode"
	inlineFormat         = "%s: an optional or non-variant @json(inline) field, in data mode"
	foldFormat           = "%s: an inline variant key equal to another key of its parent but for letter case, in data mode"
	noneMarkerFormat     = "%s: the none marker %s, a non-empty object or array, in data mode"
	dataCollisionFormat  = "%s declares %s twice in data mode"
)
