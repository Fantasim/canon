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
	errWatch = errors.New("cannot watch the project's files")
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
