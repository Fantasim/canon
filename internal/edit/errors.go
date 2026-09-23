package edit

import (
	"errors"
	"fmt"
)

var (
	// ErrBadPath is a path that does not follow API.md §6.1; the API wraps it as canon.ErrBadPath.
	ErrBadPath = errors.New("invalid path")

	errUnexpected = errors.New("unexpected character")
	errName       = errors.New("expected a name")
	errUnclosed   = errors.New("expected ]")
	errDigits     = errors.New("invalid digits")
	errRange      = errors.New("integer too large")
)

// SyntaxError is a refused path: Reason at byte Offset. It wraps ErrBadPath and its Reason,
// so the API fills PathError.Detail without reading the text.
type SyntaxError struct {
	Offset int
	Reason error
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf(fmtSyntaxError, ErrBadPath, e.Reason, e.Offset)
}

// Unwrap is ErrBadPath and the reason, for errors.Is and errors.As.
func (e *SyntaxError) Unwrap() []error {
	return []error{ErrBadPath, e.Reason}
}
