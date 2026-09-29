package workspace

import (
	"errors"
	"strings"
)

var (
	// ErrClosed is a call after Close (API.md O6).
	ErrClosed = errors.New("project is closed")
	// ErrBadPath is an overlay's file that is neither absolute nor a display path of the project.
	ErrBadPath = errors.New("invalid path")
	// ErrStale is a base revision that is not remembered, or whose read set changed (API.md S4, S5).
	ErrStale = errors.New("sources changed since the base revision")
	// ErrRejected is an edit whose result has errors, without AllowErrors (API.md E19).
	ErrRejected     = errors.New("edit rejected: it produces errors")
	errOverlay      = errors.New("file has an unsaved overlay")
	errNotCanonical = errors.New("file is not in canonical layout")
	errWatch        = errors.New("cannot watch the project's files")
	errUnowned      = errors.New("changed files that no package owns")
)

// StaleError is ErrStale with the display paths of the files that changed, in byte order; none
// when the base revision is not one this project remembers (API.md S4, S5).
type StaleError struct {
	Files []string
}

func (e *StaleError) Error() string {
	if len(e.Files) == 0 {
		return ErrStale.Error()
	}
	return ErrStale.Error() + textSep + strings.Join(e.Files, listSep)
}

func (e *StaleError) Unwrap() error { return ErrStale }

// OverlayError is errOverlay with the display path of the file (API.md S12).
type OverlayError struct {
	File string
}

func (e *OverlayError) Error() string { return e.File + textSep + errOverlay.Error() }

func (e *OverlayError) Unwrap() error { return errOverlay }

// NotCanonicalError is errNotCanonical with the files' display paths, in byte order (API.md M9).
type NotCanonicalError struct {
	Files []string
}

func (e *NotCanonicalError) Error() string {
	return errNotCanonical.Error() + textSep + strings.Join(e.Files, listSep)
}

func (e *NotCanonicalError) Unwrap() error { return errNotCanonical }
