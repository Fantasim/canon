package edit

import (
	"errors"
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/eval"
)

// The API wraps each exported sentinel as its own (API.md §15).
var (
	ErrBadPath       = errors.New("invalid path")
	ErrNoPath        = errors.New("no value at path")
	ErrAmbiguousPath = errors.New("ambiguous path")
	ErrNoValue       = errors.New("value not computed")
	ErrNotAnalyzed   = errors.New("package not analyzed") // a root of a package the analysis did not select
	ErrForeign       = errors.New("path resolved in another snapshot")

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

// PathError is a path Resolve refuses: Err at segment Seg, -1 for the root; Candidates are the
// roots an ambiguous path matches, sorted; Root is the poisoned root of ErrNoValue.
type PathError struct {
	Seg        int
	Candidates []string
	Root       eval.Root
	Err        error
}

func (e *PathError) Error() string {
	return fmt.Sprintf(fmtPathError, e.Err, e.Seg)
}

func (e *PathError) Unwrap() error { return e.Err }

// Refusals of operations and their values; the API wraps each as its own (API.md §15).
var (
	ErrBadValue = errors.New("value does not fit the type")
	ErrBadOp    = errors.New("operation not valid here")
	ErrNoHost   = errors.New("typer without a host")
)

// ValueError is an operation value that does not fit the type expected where it goes
// (API.md V1); it wraps ErrBadValue, and the API adds the op and the path.
type ValueError struct {
	Expected string // canonical type text
	Got      string // what was given
	Detail   string
}

func (e *ValueError) Error() string {
	text := fmt.Sprintf(fmtValueError, ErrBadValue, e.Expected, e.Got)
	if e.Detail != "" {
		text += detailSep + e.Detail
	}
	return text
}

func (e *ValueError) Unwrap() error { return ErrBadValue }

// The JSON form of an operation (API.md §8.8).
var (
	ErrOpJSON = errors.New("invalid operation JSON")
	ErrNoText = errors.New("value has no source text")

	errUnknownOp  = errors.New("unknown operation")
	errNotString  = errors.New("not a JSON string")
	errNotInteger = errors.New("not an integer")
	errEmptyCase  = errors.New("empty case name")
)

// The commit of an edit's files and its recovery (API.md §10.3, O5).
var (
	ErrStale    = errors.New("sources changed since the base revision")
	ErrRevision = errors.New("invalid revision")
	ErrJournal  = errors.New("unfinished edit journal")
	ErrChanges  = errors.New("invalid file changes")
)

// StaleError is a commit refused because files changed on disk since the snapshot the edit was
// planned on: their display paths, in byte order (API.md N9).
type StaleError struct {
	Files []string
}

func (e *StaleError) Error() string {
	return ErrStale.Error() + detailSep + strings.Join(e.Files, listSep)
}

// Unwrap is ErrStale.
func (e *StaleError) Unwrap() error { return ErrStale }
