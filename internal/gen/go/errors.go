package gogen

import "errors"

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
	// errFormat is generated source that go/format refuses: a compiler bug (CODEGEN.md §2.7).
	errFormat = errors.New("gogen: generated Go does not format")
)
