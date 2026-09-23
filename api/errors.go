package canon

import (
	"errors"
	"fmt"
)

// Sentinel errors (API.md §15). Every error type below wraps exactly one of them; use errors.Is.
var (
	ErrNoProject          = errors.New("no project.canon found")
	ErrProject            = errors.New("invalid project")
	ErrUnsupportedVersion = errors.New("unsupported language version")
	ErrUnknownPackage     = errors.New("unknown package")
	ErrUnknownLayer       = errors.New("unknown layer")
	ErrBadPath            = errors.New("invalid path")
	ErrNoPath             = errors.New("no value at path")
	ErrAmbiguousPath      = errors.New("ambiguous path")
	ErrNoValue            = errors.New("value could not be computed")
	ErrBadOp              = errors.New("operation not valid here")
	ErrBadValue           = errors.New("value does not fit the type")
	ErrKeyExists          = errors.New("key already exists")
	ErrStableKey          = errors.New("stable id cannot be removed, renamed or un-retired")
	ErrNotEditable        = errors.New("value is not editable")
	ErrStale              = errors.New("sources changed since the base revision")
	ErrRejected           = errors.New("edit rejected: it produces errors")
	ErrNotCanonical       = errors.New("file is not in canonical layout")
	ErrPathCollision      = errors.New("file already exists")
	ErrOverlay            = errors.New("file has an unsaved overlay")
	ErrSyntax             = errors.New("syntax error")
	ErrClosed             = errors.New("project is closed")
	ErrInternal           = errors.New("internal compiler error")
)

// errUnimplemented is what a stub with an error result returns until M4 (rule X2).
func errUnimplemented() error { return &InternalError{Msg: msgUnimplemented} }

func opPrefix(op int) string {
	if op < 0 {
		return ""
	}
	return fmt.Sprintf(fmtOpPrefix, op)
}

// errText is the text of rule X1: "<op N: >path: <sentinel text>: <detail>".
func errText(op int, path string, sentinel error, detail string) string {
	s := opPrefix(op)
	if path != "" {
		s += path + textSep
	}
	s += sentinel.Error()
	if detail != "" {
		s += textSep + detail
	}
	return s
}

// ProjectError reports errors in project.canon or in Options (rule O3). Err is ErrProject or
// ErrUnsupportedVersion.
type ProjectError struct {
	Err      error
	Findings []Finding
}

func (e *ProjectError) Error() string {
	if len(e.Findings) > 0 {
		return e.Err.Error() + textSep + e.Findings[0].Message
	}
	return e.Err.Error()
}

func (e *ProjectError) Unwrap() error { return e.Err }

// PathError reports a problem with a path or an operation (API.md §15).
type PathError struct {
	Op         int // index in Edit.Ops, or -1
	Path       string
	Err        error
	Detail     string
	Findings   []Finding // explain ErrNoValue
	Candidates []string  // qualified roots, for ErrAmbiguousPath
}

func (e *PathError) Error() string { return errText(e.Op, e.Path, e.Err, e.Detail) }

func (e *PathError) Unwrap() error { return e.Err }

// ValueError reports an operation value that does not fit the expected type (rule V1).
type ValueError struct {
	Op       int
	Path     string
	Expected string // canonical type text
	Got      string // description of the given value
	Detail   string
}

func (e *ValueError) Error() string {
	d := textExpected + e.Expected + textGot + e.Got
	if e.Detail != "" {
		d += textSep + e.Detail
	}
	return errText(e.Op, e.Path, ErrBadValue, d)
}

func (e *ValueError) Unwrap() error { return ErrBadValue }

// NotEditableError reports an edit of a value that has no editable source (rule W5).
type NotEditableError struct {
	Op     int
	Path   string
	Reason Reason
	Origin string // ReasonComputed: canonical path of the nearest editable source, or ""
	Layer  string // ReasonLayered
	Detail string
}

func (e *NotEditableError) Error() string {
	d := string(e.Reason)
	if e.Detail != "" {
		d += textSep + e.Detail
	}
	return errText(e.Op, e.Path, ErrNotEditable, d)
}

func (e *NotEditableError) Unwrap() error { return ErrNotEditable }

// StaleError reports files that changed since the base revision (rules S5, N9).
type StaleError struct {
	Files []string
}

func (e *StaleError) Error() string { return errText(-1, "", ErrStale, fmt.Sprint(e.Files)) }

func (e *StaleError) Unwrap() error { return ErrStale }

// RejectedError reports an edit refused because the result has error findings (rule E19).
type RejectedError struct {
	Findings []Finding // the error findings, rule F2 order
}

func (e *RejectedError) Error() string {
	return errText(-1, "", ErrRejected, fmt.Sprintf(fmtErrorCount, len(e.Findings)))
}

func (e *RejectedError) Unwrap() error { return ErrRejected }

// NotCanonicalError lists files an edit would write that are not in canonical layout (rule M9).
type NotCanonicalError struct {
	Files []string
}

func (e *NotCanonicalError) Error() string {
	return errText(-1, "", ErrNotCanonical, fmt.Sprint(e.Files))
}

func (e *NotCanonicalError) Unwrap() error { return ErrNotCanonical }

// SyntaxError reports input that does not parse, from Format or FormatJSONSource.
type SyntaxError struct {
	Findings []Finding
}

func (e *SyntaxError) Error() string {
	if len(e.Findings) > 0 {
		f := e.Findings[0]
		return fmt.Sprintf(fmtSyntaxAt, f.File, f.Line, f.Col, f.Message)
	}
	return ErrSyntax.Error()
}

func (e *SyntaxError) Unwrap() error { return ErrSyntax }

// InternalError is a compiler bug recovered at the API boundary (rule X2).
type InternalError struct {
	Msg   string
	Stack string // Go stack trace
}

func (e *InternalError) Error() string { return ErrInternal.Error() + textSep + e.Msg }

func (e *InternalError) Unwrap() error { return ErrInternal }
