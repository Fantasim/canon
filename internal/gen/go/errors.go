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
