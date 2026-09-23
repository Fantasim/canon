package verify

import "errors"

// ErrNoBag is a top-level value of a package Check was given no bag for.
var ErrNoBag = errors.New("no bag for the package")
