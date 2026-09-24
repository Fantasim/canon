package eval

import "errors"

// errInconsistent is a state the checked program excludes, met during evaluation: a compiler
// bug (API.md ErrInternal, exit 3), never a finding of the program.
var errInconsistent = errors.New("evaluation met a state the checker excludes")

// ErrNotFresh is TestCalls on an evaluator that has evaluated: tests force values afresh (CONFORMANCE.md §6.1).
var ErrNotFresh = errors.New("eval: test calls need a fresh evaluator")

// ErrNoPackage is TestCalls on a package the program does not hold.
var ErrNoPackage = errors.New("eval: no such package")
