package canon

import (
	"errors"
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/edit"
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
	ErrInputField         = errors.New("input field has no value at build time")
	ErrBadOp              = errors.New("operation not valid here")
	ErrBadValue           = errors.New("value does not fit the type")
	ErrKeyExists          = errors.New("key already exists")
	ErrStableKey          = errors.New("stable id cannot be removed, renamed or un-retired")
	ErrNameClash          = errors.New("rename would change what another name refers to")
	ErrNotEditable        = errors.New("value is not editable")
	ErrStale              = errors.New("sources changed since the base revision")
	ErrRejected           = errors.New("edit rejected: it produces errors")
	ErrNotCanonical       = errors.New("file is not in canonical layout")
	ErrPathCollision      = errors.New("file already exists")
	ErrOverlay            = errors.New("file has an unsaved overlay")
	ErrSyntax             = errors.New("syntax error")
	ErrClosed             = errors.New("project is closed")
	ErrInternal           = errors.New("internal compiler error")

	errSeverity   = errors.New("unknown severity")
	errOpKind     = errors.New("unknown operation")
	errDecodeNull = errors.New("null where the view model admits none")
	errDecodeCase = errors.New("member name matches a field only by case")
)

// sentinelMap is one of edit's path refusals and the API sentinel it is (API.md §15).
type sentinelMap struct {
	from error
	to   error
}

// pathSentinels maps edit's path refusals to the API's (rules R5, R6).
var pathSentinels = [...]sentinelMap{
	{from: edit.ErrBadPath, to: ErrBadPath},
	{from: edit.ErrNoPath, to: ErrNoPath},
	{from: edit.ErrAmbiguousPath, to: ErrAmbiguousPath},
	{from: edit.ErrNoValue, to: ErrNoValue},
}

func opPrefix(op int) string {
	if op < 0 {
		return ""
	}
	return fmt.Sprintf(fmtOpPrefix, op)
}

// errText is the one text of rule X1: "<op N: ><path: ><sentinel text><: detail>".
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

func (e *ProjectError) Error() string { return errText(-1, "", e.Err, firstFinding(e.Findings)) }

// firstFinding is the detail of rule X1 for findings: the first one, located when it has a file.
func firstFinding(fs []Finding) string {
	if len(fs) == 0 {
		return ""
	}
	f := fs[0]
	if f.File == "" {
		return f.Message
	}
	return fmt.Sprintf(fmtFindingAt, f.File, f.Line, f.Col, f.Message)
}

// fileList is the detail of rule X1 for a list of files.
func fileList(files []string) string { return strings.Join(files, textListSep) }

func (e *ProjectError) Unwrap() error { return e.Err }

// PathError reports a problem with a path or an operation (API.md §15).
type PathError struct {
	Op         int // index in Edit.Ops, or -1
	Path       string
	Err        error
	Detail     string
	Findings   []Finding // explain ErrNoValue
	Candidates []string  // qualified roots, or a rename's positions, for ErrAmbiguousPath
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

func (e *StaleError) Error() string { return errText(-1, "", ErrStale, fileList(e.Files)) }

func (e *StaleError) Unwrap() error { return ErrStale }

// RejectedError reports an edit refused because the result has error findings (rule E19).
type RejectedError struct {
	Findings []Finding // the error findings, rule F2 order
}

func (e *RejectedError) Error() string {
	noun := textErrors
	if len(e.Findings) == 1 {
		noun = textError
	}
	return errText(-1, "", ErrRejected, fmt.Sprintf(fmtCount, len(e.Findings), noun))
}

func (e *RejectedError) Unwrap() error { return ErrRejected }

// NotCanonicalError lists files an edit would write that are not in canonical layout (rule M9).
type NotCanonicalError struct {
	Files []string
}

func (e *NotCanonicalError) Error() string {
	return errText(-1, "", ErrNotCanonical, fileList(e.Files))
}

func (e *NotCanonicalError) Unwrap() error { return ErrNotCanonical }

// SyntaxError reports input that does not parse, from Format or FormatJSONSource.
type SyntaxError struct {
	Findings []Finding
}

func (e *SyntaxError) Error() string { return errText(-1, "", ErrSyntax, firstFinding(e.Findings)) }

func (e *SyntaxError) Unwrap() error { return ErrSyntax }

// InternalError is a compiler bug recovered at the API boundary (rule X2).
type InternalError struct {
	Msg   string
	Stack string // Go stack trace
}

func (e *InternalError) Error() string { return errText(-1, "", ErrInternal, e.Msg) }

func (e *InternalError) Unwrap() error { return ErrInternal }
