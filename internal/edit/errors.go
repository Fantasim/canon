package edit

import (
	"errors"
	"fmt"

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
