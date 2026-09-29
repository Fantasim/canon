package project

import "errors"

// Errors of loading a project; language errors are findings, these say which kind stopped it.
var (
	ErrNoProject          = errors.New("no project.canon found")
	ErrInvalid            = errors.New("invalid project")
	ErrUnsupportedVersion = errors.New("unsupported language version")
	ErrUnknownPackage     = errors.New("unknown package")
	ErrMixedDirectory     = errors.New("its files declare several packages")
	ErrReuseSet           = errors.New("the reuse store belongs to another file set")
)

// errNoLinks is EvalSymlinks on an FS that resolves no symbolic link (WIRE.md §6.5).
var errNoLinks = errors.New("the file system resolves no symbolic link")

// ErrSymlinkLoop is a link chain past MaxSymlinkHops; load reports it as W7115 looping.
var ErrSymlinkLoop = errors.New("too many levels of symbolic links")

// UnknownError names what a call selected that the project does not have: a package selector (API.md R1) or a layer (API.md O4).
type UnknownError struct {
	Err  error
	Name string
}

func (e *UnknownError) Error() string { return e.Err.Error() + unknownSep + e.Name }

func (e *UnknownError) Unwrap() error { return e.Err }
