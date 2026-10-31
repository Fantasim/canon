package eval

import "errors"

// errInconsistent is a state the checked program excludes, met during evaluation: a compiler
// bug (API.md ErrInternal, exit 3), never a finding of the program.
var errInconsistent = errors.New("evaluation met a state the checker excludes")

// ErrNotFresh is TestCalls on an evaluator that has evaluated: tests force values afresh (CONFORMANCE.md §6.1).
var ErrNotFresh = errors.New("eval: test calls need a fresh evaluator")

// ErrNoPackage is TestCalls on a package the program does not hold.
var ErrNoPackage = errors.New("eval: no such package")

// errNoWorld is an `expect package` whose world could not be built: the test's Builder is no
// PackageBuilder, its host serves no world, or a value holds a node no let holds.
var errNoWorld = errors.New("eval: expect package built no world")
