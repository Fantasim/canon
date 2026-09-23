package catalog

import "errors"

var (
	errTable     = errors.New("malformed table")
	errPlan      = errors.New("malformed package table")
	errCode      = errors.New("invalid code")
	errSeverity  = errors.New("invalid severity")
	errPackage   = errors.New("invalid package")
	errMessage   = errors.New("invalid message row")
	errArgs      = errors.New("invalid arguments")
	errTemplate  = errors.New("invalid template")
	errKind      = errors.New("invalid kind")
	errCount     = errors.New("count sentence disagrees with the tables")
	errRuntime   = errors.New("invalid runtime text")
	errGenerate  = errors.New("cannot generate")
	errLoneBrace = errors.New("a brace that is neither an escape nor a placeholder")
	errEscape    = errors.New(`a \ not followed by \ or n`)
	errMarkdown  = errors.New("a backtick or a | in a template")
)
