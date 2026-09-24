package build

import "errors"

var (
	// ErrUnknownLayer is a layer name no loaded package has a file for (API.md O4).
	ErrUnknownLayer = errors.New("unknown layer")
	// ErrNoGenerator is an emit whose target has no generator in this compiler yet.
	ErrNoGenerator = errors.New("no generator for this emit target yet")
	// ErrLoad is a load form or option the load package does not read yet (DECISIONS 196).
	ErrLoad = errors.New("load is not supported by this compiler yet")
	// ErrInternal is a compiler bug met by a build: the evaluator's, a stage's or a placement's.
	ErrInternal = errors.New("internal compiler error")
	// ErrReadOnly is a build that must write through a file system without WriteFS's methods.
	ErrReadOnly = errors.New("the project's file system cannot be written")
)
