package load

import "errors"

// ErrUnsupported is a load form, option, format or default this milestone does not read yet;
// the caller (build) falls back to its own refusal until the form's milestone (DECISIONS 196).
var ErrUnsupported = errors.New("load: not supported yet")

// errNotDir is a glob's base, or a literal load.dir path, naming a file where a directory is
// needed (E7004 cause "not a directory", meta/decisions/log-2026-09-24.md "load.dir review").
var errNotDir = errors.New("not a directory")

// UnsupportedError is ErrUnsupported naming its cause, which the caller prints (DECISIONS 196:
// every refusal names its cause).
type UnsupportedError struct {
	Cause string
}

func (e *UnsupportedError) Error() string { return ErrUnsupported.Error() + ": " + e.Cause }

func (e *UnsupportedError) Unwrap() error { return ErrUnsupported }

// unsupported is ErrUnsupported carrying cause (DECISIONS 196).
func unsupported(cause string) error { return &UnsupportedError{Cause: cause} }
