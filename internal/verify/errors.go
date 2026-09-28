package verify

import "errors"

// ErrNoBag is a top-level value of a package Check was given no bag for.
var ErrNoBag = errors.New("no bag for the package")

// ErrUnjudged is a dependent value verification cannot judge, never handed on unconverted (TYPES.md §11.6).
var ErrUnjudged = errors.New("dependent value left unjudged")
