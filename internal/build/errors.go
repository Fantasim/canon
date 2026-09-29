package build

import "errors"

var (
	// ErrUnknownLayer is a layer name no loaded package has a file for (API.md O4).
	ErrUnknownLayer = errors.New("unknown layer")
	ErrNoGenerator  = errors.New("no generator for this emit target yet")
	// ErrLoad is a load form or option the load package does not read yet (DECISIONS 196).
	ErrLoad = errors.New("load is not supported by this compiler yet")
	// ErrInternal is a compiler bug met by a build: the evaluator's, a stage's or a placement's.
	ErrInternal    = errors.New("internal compiler error")
	ErrNotSelected = errors.New("a view model of a package the analysis did not select")
	// ErrReadOnly is a build that must write through a file system without WriteFS's methods.
	ErrReadOnly = errors.New("the project's file system cannot be written")

	errNoProgram  = errors.New("the checker returned no program")
	errLinkLoop   = errors.New("too many levels of symbolic links")
	errNoLoadSite = errors.New("a forced load expression is in no file of the program")
	errTwoCounts  = errors.New("the evaluator cannot spend the folder's step counter")
)

// internalError is a build's ErrInternal, its text the cause's alone (API.md §15 X1).
type internalError struct {
	cause error
}

func (e *internalError) Error() string { return e.cause.Error() }

func (e *internalError) Unwrap() []error { return []error{ErrInternal, e.cause} }

func internal(cause error) error {
	return &internalError{cause: cause}
}
