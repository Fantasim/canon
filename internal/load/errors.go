package load

import "errors"

// ErrUnsupported is a form, option, format or default this milestone does not read (DECISIONS 196).
var ErrUnsupported = errors.New("load: not supported yet")

// errNotDir is a glob base or load.dir path naming a file where a directory is needed.
var errNotDir = errors.New("not a directory")

// UnsupportedError is ErrUnsupported naming its cause (DECISIONS 196).
type UnsupportedError struct {
	Cause string
}

func (e *UnsupportedError) Error() string { return ErrUnsupported.Error() + ": " + e.Cause }

func (e *UnsupportedError) Unwrap() error { return ErrUnsupported }

// unsupported is ErrUnsupported carrying cause (DECISIONS 196).
func unsupported(cause string) error { return &UnsupportedError{Cause: cause} }
