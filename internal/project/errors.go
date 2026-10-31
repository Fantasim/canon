package project

import (
	"errors"
	"fmt"
	"io/fs"
	"syscall"
)

// Errors of loading a project; language errors are findings, these say which kind stopped it.
var (
	ErrNoProject          = errors.New("no project.canon found")
	ErrInvalid            = errors.New("invalid project")
	ErrUnsupportedVersion = errors.New("unsupported language version")
	ErrUnknownPackage     = errors.New("unknown package")
	ErrMixedDirectory     = errors.New("its files declare several packages")
	ErrReuseSet           = errors.New("the reuse store belongs to another file set")
)

// ErrNoLinks is EvalSymlinks on an FS that resolves no symbolic link: the caller decides without links (WIRE.md §6.5).
var ErrNoLinks = errors.New("the file system resolves no symbolic link")

// ErrDeviceTarget is a link whose target is a device path: dangling, naming no path (WIRE.md §6.5, DECISIONS 419).
var ErrDeviceTarget = fmt.Errorf("a link's target is a device path: %w", fs.ErrNotExist)

// notLinkErrs are readlink's answers for an entry that is no link, any other failure an error (DECISIONS 419).
var notLinkErrs = [...]error{fs.ErrNotExist, syscall.EINVAL}

// ErrSymlinkLoop is a link chain past MaxSymlinkHops; load reports it as W7115 looping.
var ErrSymlinkLoop = errors.New("too many levels of symbolic links")

// UnknownError names what a call selected that the project does not have: a package selector (API.md R1) or a layer (API.md O4).
type UnknownError struct {
	Err  error
	Name string
}

func (e *UnknownError) Error() string { return e.Err.Error() + unknownSep + e.Name }

func (e *UnknownError) Unwrap() error { return e.Err }
