package eval

import "errors"

// errInconsistent is a state the checked program excludes, met during evaluation: a compiler
// bug (API.md ErrInternal, exit 3), never a finding of the program.
var errInconsistent = errors.New("evaluation met a state the checker excludes")
