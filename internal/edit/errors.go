package edit

import "errors"

var (
	// ErrBadPath is a path that does not follow API.md §6.1; the API wraps it as canon.ErrBadPath.
	ErrBadPath = errors.New("invalid path")

	errUnexpected = errors.New("unexpected character")
	errName       = errors.New("expected a name")
	errUnclosed   = errors.New("expected ]")
	errDigits     = errors.New("invalid digits")
	errRange      = errors.New("integer too large")
)
